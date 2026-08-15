// The builder (§4), against hand-written event streams — the seam §4.2 chose the event stream
// for, exercised in the direction that needs no parser.
//
// The first test's expectation is tree_test.go's arena, which was written to be it: what the
// builder must produce for "x;\n" is already pinned there by a tree somebody wrote by hand and
// read. The rest are what only the builder can get wrong, and what no golden can see.
package parser

import (
	"fmt"
	"strings"
	"testing"

	"luna/oracle/lexer"
	"luna/oracle/source"
	"luna/oracle/token"
)

// lexFixture lexes a test's source with the real lexer, so the token indices an event stream
// carries are the ones the parser will be handed rather than ones a test invented.
func lexFixture(t *testing.T, name, src string) (*source.File, []token.Token) {
	t.Helper()
	f, err := source.New(name, src)
	if err != nil {
		t.Fatalf("building the source file: %v", err)
	}
	toks, diags := lexer.Lex(f)
	if len(diags) > 0 {
		t.Fatalf("the fixture does not lex cleanly: %v", diags)
	}
	return f, toks
}

// filtered returns the full-stream indices of the non-trivia tokens, which is how the parser
// sees them: it walks the filtered view and its token events carry real indices (§2.2).
func filtered(toks []token.Token) []int {
	var out []int
	for i, tk := range toks {
		if !tk.IsTrivia() {
			out = append(out, i)
		}
	}
	return out
}

func openEv(k Kind) event    { return event{kind: evOpen, node: k} }
func tokEv(i int) event      { return event{kind: evToken, tok: i} }
func missingEv(k Kind) event { return event{kind: evMissing, node: k} }

var closeEv = event{kind: evClose}

// dumpArena renders the arena itself rather than a walk over it, because the arena is what is
// being compared: a size or a parent that is wrong shows here and nowhere in a tree dump.
func dumpArena(t *Tree) string {
	if t == nil {
		return "<no tree>\n"
	}
	var b strings.Builder
	for id, d := range t.nodes {
		fmt.Fprintf(&b, "%d %s parent=%d size=%d %d..%d\n",
			id, d.kind, d.parent, d.size, d.offset, d.end)
	}
	return b.String()
}

// TestBuildProducesTheHandTree is the whole of step 1: the event stream for tree_test.go's file,
// against tree_test.go's arena. The trailing WHITESPACE is File's child rather than the
// Statement's because splice put its event after the Statement's close — §2.1, arriving here as
// nothing more than the order of the events.
func TestBuildProducesTheHandTree(t *testing.T) {
	f, toks := lexFixture(t, "hand.luna", handSource)
	ix := filtered(toks)

	got := build(f, toks, eventStream{
		openEv(File),
		openEv(Statement),
		tokEv(ix[0]), // IDENT "x"
		tokEv(ix[1]), // SEMICOLON
		closeEv,
		tokEv(len(toks) - 1), // WHITESPACE "\n", spliced in after the close
		closeEv,
	})

	if want := dumpArena(handTree(t)); dumpArena(got) != want {
		t.Errorf("the builder produced\n%s\nwant\n%s", dumpArena(got), want)
	}
}

// TestBuildDropsEmptyInteriorNodes is §6.1's rule, which is what lets width alone distinguish a
// synthesised leaf from a real one: a zero-width Modifier surviving here would be
// indistinguishable from a missing token.
func TestBuildDropsEmptyInteriorNodes(t *testing.T) {
	f, toks := lexFixture(t, "empty-node.luna", handSource)
	ix := filtered(toks)

	got := build(f, toks, eventStream{
		openEv(File),
		openEv(Statement),
		openEv(Modifier), // opened and closed with nothing between: it never existed
		closeEv,
		tokEv(ix[0]),
		tokEv(ix[1]),
		closeEv,
		tokEv(len(toks) - 1),
		closeEv,
	})

	if want := dumpArena(handTree(t)); dumpArena(got) != want {
		t.Errorf("an empty node left a trace:\n%s\nwant\n%s", dumpArena(got), want)
	}
}

// TestBuildZeroWidthLeaf is absence: `x` with no terminator, where the parser synthesises the
// SEMICOLON it expected (§7.2 layer 1). The leaf survives with width zero, which is the one
// thing distinguishing it from the node above; the Statement's span stops at the real token, and
// the leaves still tile the file.
func TestBuildZeroWidthLeaf(t *testing.T) {
	const src = "x"
	f, toks := lexFixture(t, "missing.luna", src)
	ix := filtered(toks)

	tr := build(f, toks, eventStream{
		openEv(File),
		openEv(Statement),
		tokEv(ix[0]),
		missingEv(Kind(token.Semicolon)),
		closeEv,
		closeEv,
	})
	if tr == nil {
		t.Fatal("no tree for a file with a token in it")
	}

	stmt := tr.Root().Children()
	if len(stmt) != 1 {
		t.Fatalf("File has %d children, want one Statement", len(stmt))
	}
	if stmt[0].Kind() != Statement {
		t.Fatalf("File's child is a %s, want a Statement", stmt[0].Kind())
	}
	kids := stmt[0].Children()
	if len(kids) != 2 {
		t.Fatalf("the Statement has %d children, want IDENT and the synthesised SEMICOLON", len(kids))
	}
	if k := kids[1].Kind(); k != Kind(token.Semicolon) {
		t.Errorf("the synthesised leaf is %s, want SEMICOLON — a missing token keeps its kind", k)
	}
	if o, e := kids[1].Span(); o != 1 || e != 1 {
		t.Errorf("the synthesised leaf spans %d..%d, want 1..1 at the insertion point", o, e)
	}
	if got := kids[1].Text(); got != "" {
		t.Errorf("the synthesised leaf covers %q, want no bytes at all", got)
	}
	if len(kids[1].Children()) != 0 {
		t.Error("the synthesised leaf has children")
	}
	if o, e := stmt[0].Span(); o != 0 || e != 1 {
		t.Errorf("the Statement spans %d..%d, want 0..1 — a zero-width child widens nothing", o, e)
	}
	if got := leafText(tr); got != src {
		t.Errorf("the leaves reconstruct %q, want %q", got, src)
	}
}

// TestBuildEmptyFileHasNoTree is §6.1 at the root, and the case no golden can express, since a
// golden's source section is never empty. The rule has no exception: File opens and closes with
// nothing between it, so it is deleted like any other empty node and Parse's nil has exactly one
// meaning.
func TestBuildEmptyFileHasNoTree(t *testing.T) {
	f, toks := lexFixture(t, "empty.luna", "")
	if len(toks) != 0 {
		t.Fatalf("the empty file lexed to %d tokens", len(toks))
	}
	if tr := build(f, toks, eventStream{openEv(File), closeEv}); tr != nil {
		t.Errorf("the empty file built a tree of %d nodes:\n%s", tr.Len(), dumpArena(tr))
	}
	if tr := build(f, toks, nil); tr != nil {
		t.Errorf("an empty stream built a tree of %d nodes", tr.Len())
	}
}

// leafText is the reconstruction invariant's one line: the leaves, in order, are the file.
func leafText(t *Tree) string {
	var b strings.Builder
	for id := range t.Len() {
		if n := t.At(NodeID(id)); len(n.Children()) == 0 {
			b.WriteString(n.Text())
		}
	}
	return b.String()
}
