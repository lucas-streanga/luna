// The splice pass (§2.2), tested events in, events out: the flattest comparison available, and
// why §2.1's rule is a pass rather than three conditions inside the builder.
//
// The four cases are the rule's four corners: trivia before anything is open, after everything
// has closed, between two siblings, and before a closing delimiter.
package parser

import (
	"fmt"
	"strings"
	"testing"

	"luna/oracle/token"
)

// spliceDump prints lexemes rather than indices, so an expectation reads as the file does.
// eventStream.String is deliberately not this: there flatness is the point, here legibility.
func spliceDump(tokens []token.Token, src string, events eventStream) string {
	var b strings.Builder
	for _, e := range events {
		switch e.kind {
		case evToken:
			tk := tokens[e.tok]
			fmt.Fprintf(&b, "token %q\n", src[tk.Offset:tk.End()])
		default:
			fmt.Fprintf(&b, "%s\n", e)
		}
	}
	return b.String()
}

func TestSplicePlacesTrivia(t *testing.T) {
	tests := []struct {
		name string
		src  string
		// Indices of the non-trivia tokens: the view the parser walks, numbered as it numbers them.
		events func(indices []int) eventStream
		want   string
	}{{
		name: "leading trivia lands inside File",
		src:  "// c\nx;",
		events: func(indices []int) eventStream {
			return eventStream{
				openEv(File), openEv(Statement), tokEv(indices[0]), tokEv(indices[1]), closeEv, closeEv,
			}
		},
		// Nothing is open when the comment is reached, so it flushes before the Statement instead,
		// File's first child being the one place §2.1 admits trivia at an edge.
		want: `open(File)
token "// c"
token "\n"
open(Statement)
token "x"
token ";"
close
close
`,
	}, {
		name: "trailing trivia stays in File",
		src:  "x;\n// end\n",
		events: func(indices []int) eventStream {
			return eventStream{
				openEv(File), openEv(Statement), tokEv(indices[0]), tokEv(indices[1]), closeEv, closeEv,
			}
		},
		// The Statement's close does not flush, so none of this lands inside it.
		want: `open(File)
open(Statement)
token "x"
token ";"
close
token "\n"
token "// end"
token "\n"
close
`,
	}, {
		name: "a comment between statements lands in the enclosing node",
		src:  "x; // c\ny;\n",
		events: func(indices []int) eventStream {
			return eventStream{
				openEv(File),
				openEv(Statement), tokEv(indices[0]), tokEv(indices[1]), closeEv,
				openEv(Statement), tokEv(indices[2]), tokEv(indices[3]), closeEv,
				closeEv,
			}
		},
		// Both directions meet here; either alone would put the comment inside a Statement.
		want: `open(File)
open(Statement)
token "x"
token ";"
close
token " "
token "// c"
token "\n"
open(Statement)
token "y"
token ";"
close
token "\n"
close
`,
	}, {
		name: "a comment before a closing brace stays in the block",
		src:  "{ x; /* c */ }\n",
		events: func(indices []int) eventStream {
			return eventStream{
				openEv(File),
				openEv(Statement),
				openEv(Block), tokEv(indices[0]),
				openEv(Statement), tokEv(indices[1]), tokEv(indices[2]), closeEv,
				tokEv(indices[3]), closeEv,
				closeEv,
				closeEv,
			}
		},
		// The closing brace flushes it, so it lands in the Block rather than trailing the statement.
		want: `open(File)
open(Statement)
open(Block)
token "{"
token " "
open(Statement)
token "x"
token ";"
close
token " "
token "/* c */"
token " "
token "}"
close
close
token "\n"
close
`,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, tokens := lexFixture(t, "splice.luna", tc.src)
			got := splice(tokens, tc.events(filtered(tokens)))
			if dump := spliceDump(tokens, tc.src, got); dump != tc.want {
				t.Errorf("spliced to\n%s\nwant\n%s", dump, tc.want)
			}
			assertIndexCoverage(t, tokens, got)
		})
	}
}

// TestSpliceHoldsOpensUntilContent is §2.2's rule: an open that never gets content never existed,
// and no trivia is flushed on its behalf.
func TestSpliceHoldsOpensUntilContent(t *testing.T) {
	// What the four shapes below must all produce, three of them despite an empty node.
	const statementAndNewline = `open(File)
open(Statement)
token "x"
token ";"
close
token "\n"
close
`

	tests := []struct {
		name   string
		src    string
		events func(indices []int) eventStream
		want   string
	}{{
		// The shape the parse fuzzer found. Flushing at the open would have left the newline as
		// the Statement's last child, widening its span over a byte it does not own.
		name: "an empty node before a close",
		src:  "x;\n",
		events: func(indices []int) eventStream {
			return eventStream{
				openEv(File),
				openEv(Statement), tokEv(indices[0]), tokEv(indices[1]),
				openEv(Modifier), closeEv,
				closeEv,
				closeEv,
			}
		},
		want: statementAndNewline,
	}, {
		name: "empty nodes nested in each other",
		src:  "x;\n",
		events: func(indices []int) eventStream {
			return eventStream{
				openEv(File),
				openEv(Statement), tokEv(indices[0]), tokEv(indices[1]), closeEv,
				openEv(Block), openEv(Modifier), closeEv, closeEv,
				closeEv,
			}
		},
		want: statementAndNewline,
	}, {
		// A synthesised leaf is content: the node it lands in is real, and survives at zero width
		// (§6.1).
		name: "a node released by a synthesised leaf",
		src:  "x\n",
		events: func(indices []int) eventStream {
			return eventStream{
				openEv(File),
				openEv(Statement), tokEv(indices[0]), missingEv(Kind(token.Semicolon)), closeEv,
				closeEv,
			}
		},
		want: `open(File)
open(Statement)
token "x"
missing(SEMICOLON)
close
token "\n"
close
`,
	}, {
		// Trailing trivia is content with nowhere further out to go, so it releases File. Without
		// that, a comment-only file would vanish along with its comments.
		name:   "a file of nothing but comments",
		src:    "// c\n",
		events: func([]int) eventStream { return eventStream{openEv(File), closeEv} },
		want: `open(File)
token "// c"
token "\n"
close
`,
	}, {
		// §6.1's iff at the root: nothing releases File, so not one event is emitted and Parse
		// has no tree to return.
		name:   "the empty file",
		src:    "",
		events: func([]int) eventStream { return eventStream{openEv(File), closeEv} },
		want:   "",
	}, {
		// The same answer by a different path through the held stack: an empty node is not
		// content, so it cannot release the root any more than it releases itself.
		name: "a root holding nothing but an empty node",
		src:  "",
		events: func([]int) eventStream {
			return eventStream{openEv(File), openEv(Block), closeEv, closeEv}
		},
		want: "",
	}, {
		// The shebang is the trivia kind that can only ever be a file's first bytes, and the only
		// one no golden contains.
		name: "a file led by a shebang",
		src:  "#!/usr/bin/env luna\nx;\n",
		events: func(indices []int) eventStream {
			return eventStream{
				openEv(File),
				openEv(Statement), tokEv(indices[0]), tokEv(indices[1]), closeEv,
				closeEv,
			}
		},
		want: `open(File)
token "#!/usr/bin/env luna"
token "\n"
open(Statement)
token "x"
token ";"
close
token "\n"
close
`,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, tokens := lexFixture(t, "held.luna", tc.src)
			got := splice(tokens, tc.events(filtered(tokens)))
			if dump := spliceDump(tokens, tc.src, got); dump != tc.want {
				t.Errorf("spliced to\n%s\nwant\n%s", dump, tc.want)
			}
			assertIndexCoverage(t, tokens, got)
		})
	}
}

// TestSpliceRejects is the pass's half of the panic contract. Every case here would otherwise
// fail *silently*, which is why each is checked rather than documented.
func TestSpliceRejects(t *testing.T) {
	_, tokens := lexFixture(t, "reject.luna", handSource) // IDENT, SEMICOLON, WHITESPACE

	tests := []struct {
		name   string
		events eventStream
		want   string
	}{
		{"a token index past the stream", eventStream{openEv(File), tokEv(3), closeEv},
			"token(3) of a stream of 3"},
		{"a negative token index", eventStream{openEv(File), tokEv(-1), closeEv}, "token(-1)"},
		// Consuming 0 and 1 first is what isolates the violation: a stream that jumps straight to
		// token 1 is met by the skip check below, one event earlier.
		{"tokens out of order",
			eventStream{openEv(File), tokEv(0), tokEv(1), tokEv(0), closeEv},
			"after token(1)"},
		{"a close with nothing open", eventStream{closeEv}, "never opened"},
		{"a node left open", eventStream{openEv(File), tokEv(0)}, "ends at depth 1"},

		// The two halves of "the parser accounts for every token": a skipped one is still emitted
		// and still reconstructs, it is simply in a node nobody put it in.
		{"a token the parser stepped over",
			eventStream{openEv(File), tokEv(1), closeEv},
			"event 1 skips token 0 (IDENT)"},
		{"a token the parser never reached",
			eventStream{openEv(File), tokEv(0), closeEv},
			"ends with token 1 (SEMICOLON) in no event"},

		// Found by FuzzSpliceContract. A flush is bounded only by the run of trivia ending, so a
		// token event naming trivia consumes past itself and emits the index twice: coverage
		// broken with nothing raised.
		{"the parser consuming trivia",
			eventStream{openEv(File), tokEv(2), closeEv},
			"is token(2), which is WHITESPACE"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertPanics(t, tc.want, func() { splice(tokens, tc.events) })
		})
	}
}

// TestSpliceKeepsSynthesisedLeavesWhereTheyAre: a missing token consumes no index and covers no
// bytes, so splice passes it through without flushing, the same half of §2.1 that a close
// takes, and the only placement that keeps the zero-width leaf before the trivia it precedes in
// the file.
func TestSpliceKeepsSynthesisedLeavesWhereTheyAre(t *testing.T) {
	const src = "x // c\n"
	f, tokens := lexFixture(t, "missing.luna", src)
	indices := filtered(tokens)

	events := splice(tokens, eventStream{
		openEv(File),
		openEv(Statement), tokEv(indices[0]), missingEv(Kind(token.Semicolon)), closeEv,
		closeEv,
	})
	want := `open(File)
open(Statement)
token "x"
missing(SEMICOLON)
close
token " "
token "// c"
token "\n"
close
`
	if dump := spliceDump(tokens, src, events); dump != want {
		t.Errorf("spliced to\n%s\nwant\n%s", dump, want)
	}
	assertIndexCoverage(t, tokens, events)

	// The tree that follows is the point: the synthesised leaf sits at the end of the token it
	// followed, and the leaves still tile the file.
	tree := build(f, tokens, events)
	if got := leafText(tree); got != src {
		t.Errorf("the leaves reconstruct %q, want %q", got, src)
	}
	stmt := tree.Root().Children()[0]
	if o, e := stmt.Span(); o != 0 || e != 1 {
		t.Errorf("the Statement spans %d..%d, want 0..1: the comment is File's, not its", o, e)
	}
}
