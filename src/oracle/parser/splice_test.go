// The splice pass (§2.2), tested events in, events out — the flattest comparison available, and
// the reason §2.1's rule is a pass rather than three conditions inside the builder.
//
// The four cases are the rule's four corners: trivia before anything is open, trivia after
// everything has closed, trivia between two siblings, and trivia before a closing delimiter. In
// every one of them it lands in the innermost node that was *already* open when it occurred,
// which is what keeps inner spans tight and leaves File the only node with trivia at an edge.
package parser

import (
	"fmt"
	"strings"
	"testing"

	"luna/oracle/token"
)

// spliceDump prints a stream with lexemes rather than indices, so an expectation reads as the
// file does. eventStream.String is the debug dump §4.2 permits and is deliberately not this:
// there, flatness is the point; here, legibility is.
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
		// events takes the full-stream indices of the non-trivia tokens, which is the view the
		// parser walks: it never sees trivia, and its token events carry real indices (§2.2).
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
		// Nothing is open when the comment is reached, so it is not flushed before File; it is
		// flushed before the Statement opens, and File's first child is the one place §2.1's
		// invariant admits trivia at an edge.
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
		// The Statement's close does not flush, so none of this is inside it; the root's close
		// is the table's "end" row, and File's span is therefore the file's.
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
		// The two directions meet here: the first close does not flush, and the second open
		// defers until it has. Either alone would put the comment inside a Statement.
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
		// Trivia before a token is flushed by that token, so the comment lands in whatever the
		// closing brace belongs to — the Block — rather than trailing the statement it follows.
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

// TestSpliceRejects is the pass's half of the panic contract. It checks only what it depends on
// — indices and balance — because a bad kind is caught once, by the builder that reads it.
//
// Both cases here would otherwise fail *silently*, which is why they are checked rather than
// documented: an index out of order makes the flush emit the wrong run and coverage breaks with
// no symptom until reconstruction, and a stream whose depth never returns to zero drops the
// file's trailing trivia with none at all.
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

		// The two halves of "the parser accounts for every token". Both would otherwise pass
		// silently: the skipped token is still emitted, still tiles the file and still
		// reconstructs — it is simply in a node nobody put it in.
		{"a token the parser stepped over",
			eventStream{openEv(File), tokEv(1), closeEv},
			"event 1 skips token 0 (IDENT)"},
		{"a token the parser never reached",
			eventStream{openEv(File), tokEv(0), closeEv},
			"ends with token 1 (SEMICOLON) in no event"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertPanics(t, tc.want, func() { splice(tokens, tc.events) })
		})
	}
}

// TestSpliceKeepsSynthesisedLeavesWhereTheyAre: a missing token consumes no index and covers no
// bytes, so splice passes it through without flushing — the same half of §2.1 that a close
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
