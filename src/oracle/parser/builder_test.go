// The builder (§4), driven by hand-written events — the seam §4.2 chose the stream for.
//
// The first test expects tree_test.go's arena, which was written to be it. The rest are what
// only the builder can get wrong, and what no golden can see.
package parser

import (
	"fmt"
	"strings"
	"testing"

	"luna/oracle/lexer"
	"luna/oracle/source"
	"luna/oracle/token"
)

// lexFixture uses the real lexer, so the indices an event carries are the ones the parser will
// be handed rather than ones a test invented.
func lexFixture(t *testing.T, name, src string) (*source.File, []token.Token) {
	t.Helper()
	f, err := source.New(name, src)
	if err != nil {
		t.Fatalf("building the source file: %v", err)
	}
	tokens, diags := lexer.Lex(f)
	if len(diags) > 0 {
		t.Fatalf("the fixture does not lex cleanly: %v", diags)
	}
	return f, tokens
}

// filtered returns the full-stream indices of the non-trivia tokens, which is how the parser
// sees them: it walks the filtered view and its token events carry real indices (§2.2).
func filtered(tokens []token.Token) []int {
	var out []int
	for i, tk := range tokens {
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
func dumpArena(tree *Tree) string {
	if tree == nil {
		return "<no tree>\n"
	}
	var b strings.Builder
	for id, d := range tree.nodes {
		fmt.Fprintf(&b, "%d %s parent=%d size=%d %d..%d\n",
			id, d.kind, d.parent, d.size, d.offset, d.end)
	}
	return b.String()
}

// TestBuildProducesTheHandTree: the trailing WHITESPACE is File's child rather than the
// Statement's because splice put its event after the close — §2.1, arriving here as nothing
// more than the order of the events.
func TestBuildProducesTheHandTree(t *testing.T) {
	f, tokens := lexFixture(t, "hand.luna", handSource)
	indices := filtered(tokens)

	got := build(f, tokens, eventStream{
		openEv(File),
		openEv(Statement),
		tokEv(indices[0]), // IDENT "x"
		tokEv(indices[1]), // SEMICOLON
		closeEv,
		tokEv(len(tokens) - 1), // WHITESPACE "\n", spliced in after the close
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
	f, tokens := lexFixture(t, "empty-node.luna", handSource)
	indices := filtered(tokens)

	got := build(f, tokens, eventStream{
		openEv(File),
		openEv(Statement),
		openEv(Modifier), // opened and closed with nothing between: it never existed
		closeEv,
		tokEv(indices[0]),
		tokEv(indices[1]),
		closeEv,
		tokEv(len(tokens) - 1),
		closeEv,
	})

	if want := dumpArena(handTree(t)); dumpArena(got) != want {
		t.Errorf("an empty node left a trace:\n%s\nwant\n%s", dumpArena(got), want)
	}
}

// TestBuildZeroWidthLeaf is absence: `x` with no terminator (§7.2 layer 1). Width zero is the
// one thing distinguishing the leaf from the empty node above.
func TestBuildZeroWidthLeaf(t *testing.T) {
	const src = "x"
	f, tokens := lexFixture(t, "missing.luna", src)
	indices := filtered(tokens)

	tree := build(f, tokens, eventStream{
		openEv(File),
		openEv(Statement),
		tokEv(indices[0]),
		missingEv(Kind(token.Semicolon)),
		closeEv,
		closeEv,
	})
	if tree == nil {
		t.Fatal("no tree for a file with a token in it")
	}

	stmt := tree.Root().Children()
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
	if got := leafText(tree); got != src {
		t.Errorf("the leaves reconstruct %q, want %q", got, src)
	}
}

// TestBuildEmptyFileHasNoTree is §6.1 at the root, and the case no golden can express, a golden's
// source section never being empty. The rule has no exception, which is what gives Parse's nil
// exactly one meaning.
func TestBuildEmptyFileHasNoTree(t *testing.T) {
	f, tokens := lexFixture(t, "empty.luna", "")
	if len(tokens) != 0 {
		t.Fatalf("the empty file lexed to %d tokens", len(tokens))
	}
	if tree := build(f, tokens, eventStream{openEv(File), closeEv}); tree != nil {
		t.Errorf("the empty file built a tree of %d nodes:\n%s", tree.Len(), dumpArena(tree))
	}
	if tree := build(f, tokens, nil); tree != nil {
		t.Errorf("an empty stream built a tree of %d nodes", tree.Len())
	}
}

// TestBuildKeepsANodeWhoseOnlyChildIsSynthesised pins the boundary §6.1 does not itself draw:
// a node holding only the zero-width Error survives, having a child.
//
// It is not the ambiguity the rule guards against — that one is between a synthesised leaf and an
// *empty* node, and the empty node is gone by now. What is left is the difference between a node
// the parser reached and found nothing in and one it never reached, which is worth keeping.
func TestBuildKeepsANodeWhoseOnlyChildIsSynthesised(t *testing.T) {
	const src = "let x = ;\n"
	f, tokens := lexFixture(t, "absent-construct.luna", src)

	// Splice's output for that file, written out: trivia flushes before each open, and the Error
	// lands at the cursor, the end of the space before `;`.
	tree := build(f, tokens, eventStream{
		openEv(File),
		openEv(Declaration),
		openEv(BindingDecl),
		tokEv(0),                          // KW_LET
		tokEv(1),                          // WHITESPACE
		openEv(Binder), tokEv(2), closeEv, // IDENT
		tokEv(3), // WHITESPACE
		tokEv(4), // ASSIGN
		tokEv(5), // WHITESPACE
		openEv(Initializer), missingEv(Error), closeEv,
		tokEv(6), // SEMICOLON
		closeEv,
		closeEv,
		tokEv(7), // WHITESPACE
		closeEv,
	})

	decl := tree.Root().Children()[0].Children()[0]
	if decl.Kind() != BindingDecl {
		t.Fatalf("expected a BindingDecl, got %s", decl.Kind())
	}
	var init Node
	for _, kid := range decl.Children() {
		if kid.Kind() == Initializer {
			init = kid
		}
	}
	if init.Kind() != Initializer {
		t.Fatalf("the Initializer was deleted; a node whose one child is synthesised has a child")
	}
	if o, e := init.Span(); o != 8 || e != 8 {
		t.Errorf("the Initializer spans %d..%d, want 8..8 at the insertion point", o, e)
	}
	kids := init.Children()
	if len(kids) != 1 || kids[0].Kind() != Error {
		t.Fatalf("the Initializer has %d children, want one Error", len(kids))
	}
	if o, e := kids[0].Span(); o != 8 || e != 8 {
		t.Errorf("the Error spans %d..%d, want 8..8", o, e)
	}
	if o, e := decl.Span(); o != 0 || e != 9 {
		t.Errorf("the BindingDecl spans %d..%d, want 0..9 — a zero-width child widens nothing", o, e)
	}
	if got := leafText(tree); got != src {
		t.Errorf("the leaves reconstruct %q, want %q", got, src)
	}
}

// TestBuildRejects is the panic contract, one case per precondition. A Go runtime error here
// would be a precondition nobody checked, which is the failure the fuzz targets will hunt.
func TestBuildRejects(t *testing.T) {
	f, tokens := lexFixture(t, "reject.luna", handSource) // IDENT, SEMICOLON, WHITESPACE

	tests := []struct {
		name   string
		events eventStream
		want   string
	}{
		{"a close with nothing open", eventStream{closeEv}, "closes a node that was never opened"},
		{"a leaf outside every node", eventStream{tokEv(0)}, "leaf outside every node"},
		{"a second root", eventStream{openEv(File), tokEv(0), closeEv, openEv(File), closeEv},
			"follows the root's close"},
		{"an event after the root closed", eventStream{openEv(File), closeEv, tokEv(0)},
			"follows the root's close"},
		{"a token index past the stream", eventStream{openEv(File), tokEv(3)},
			"token(3) of a stream of 3"},
		{"a negative token index", eventStream{openEv(File), tokEv(-1)}, "token(-1)"},
		{"tokens out of order", eventStream{openEv(File), tokEv(1), tokEv(0)}, "after token(1)"},
		{"a token consumed twice", eventStream{openEv(File), tokEv(1), tokEv(1)}, "after token(1)"},
		{"opening a token kind", eventStream{openEv(Kind(token.Semicolon))}, "not a node kind"},
		{"opening Unset", eventStream{openEv(Unset)}, "not a node kind"},
		{"opening past the last kind", eventStream{openEv(Error + 1)}, "not a node kind"},
		{"synthesising a node kind", eventStream{openEv(File), missingEv(Statement)},
			"not a terminal the parser could have expected"},
		{"synthesising trivia", eventStream{openEv(File), missingEv(Kind(token.Whitespace))},
			"not a terminal the parser could have expected"},
		{"synthesising Unset", eventStream{openEv(File), missingEv(Unset)},
			"not a terminal the parser could have expected"},
		{"a node left open", eventStream{openEv(File), tokEv(0)}, "ends inside File, 1 deep"},
		{"an event of no kind", eventStream{{kind: eventKind(42)}}, "has kind 42"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assertPanics(t, tc.want, func() { build(f, tokens, tc.events) })
		})
	}
}

// assertPanics requires a parser: message: a panic of any other type is a precondition nobody
// checked, which is what the contract forbids.
func assertPanics(t *testing.T, want string, f func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("no panic; want one containing %q", want)
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panicked with %T (%v); want a parser: string — anything else is a "+
				"precondition nobody checked", r, r)
		}
		if !strings.HasPrefix(msg, "parser: ") || !strings.Contains(msg, want) {
			t.Fatalf("panicked with %q, want a parser: message containing %q", msg, want)
		}
	}()
	f()
}

// leafText: the leaves, in order, are the file.
func leafText(tree *Tree) string {
	var b strings.Builder
	for id := range tree.Len() {
		if n := tree.At(NodeID(id)); len(n.Children()) == 0 {
			b.WriteString(n.Text())
		}
	}
	return b.String()
}
