package ebnf_test

import (
	"fmt"
	"strings"
	"testing"

	"luna/internal/ebnf"
	"luna/oracle/token"
)

// sketch renders a tree as one line, `Name(child child)`, with token spans on the leaves. It
// is deliberately not the golden format; these tests check the *shape* the chart reconstructs,
// and the format is internal/golden's business.
func sketch(n *ebnf.Node) string {
	if n.Terminal {
		return fmt.Sprintf("%s@%d", n.Name, n.Token)
	}
	parts := make([]string, len(n.Children))
	for i, c := range n.Children {
		parts[i] = sketch(c)
	}
	return fmt.Sprintf("%s(%s)", n.Name, strings.Join(parts, " "))
}

func derive(t *testing.T, g *ebnf.Grammar, kinds ...token.Kind) *ebnf.Node {
	t.Helper()
	n, err := g.Derive(toks(kinds...))
	if err != nil {
		t.Fatalf("deriving: %v", err)
	}
	return n
}

func TestDeriveShape(t *testing.T) {
	g := build(t, `
File ::= File PLUS Term | Term
Term ::= IDENT
`)
	got := sketch(derive(t, g, token.Ident, token.Plus, token.Ident))
	want := "File(File(Term(IDENT@0)) PLUS@1 Term(IDENT@2))"
	if got != want {
		t.Errorf("tree:\n got %s\nwant %s", got, want)
	}
}

// TestDeriveSpans: every node covers exactly its children, and the root covers the input. The
// golden format's spans are read straight off these, so an off-by-one here is an off-by-one in
// every golden written afterwards.
func TestDeriveSpans(t *testing.T) {
	g := build(t, `
File ::= LPAREN Items RPAREN
Items ::= IDENT COMMA Items | IDENT
`)
	root := derive(t, g, token.LParen, token.Ident, token.Comma, token.Ident, token.RParen)
	if root.From != 0 || root.To != 5 {
		t.Errorf("root span: got %d..%d, want 0..5", root.From, root.To)
	}
	var walk func(*ebnf.Node)
	walk = func(n *ebnf.Node) {
		if n.Terminal || len(n.Children) == 0 {
			return
		}
		if n.Children[0].From != n.From {
			t.Errorf("%s starts at %d but its first child at %d", n.Name, n.From, n.Children[0].From)
		}
		last := n.Children[len(n.Children)-1]
		if last.To != n.To {
			t.Errorf("%s ends at %d but its last child at %d", n.Name, n.To, last.To)
		}
		for i := 1; i < len(n.Children); i++ {
			if n.Children[i-1].To != n.Children[i].From {
				t.Errorf("%s has a gap between children %d and %d", n.Name, i-1, i)
			}
			walk(n.Children[i])
		}
		walk(n.Children[0])
	}
	walk(root)
}

// TestDeriveHandlesRepetition: `*` desugars to a right-recursive synthetic, so the tree nests
// where a hand-written parser would loop. The golden renderer splices those away; this pins
// that they are there to splice, and that they are recognizable.
func TestDeriveHandlesRepetition(t *testing.T) {
	g := build(t, "File ::= IDENT*")
	root := derive(t, g, token.Ident, token.Ident)
	if root.Synthetic() {
		t.Errorf("File is a spec name, not a synthetic")
	}
	found := false
	var walk func(*ebnf.Node)
	walk = func(n *ebnf.Node) {
		if n.Synthetic() {
			found = true
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(root)
	if !found {
		t.Errorf("a `*` should leave a synthetic nonterminal in the tree: %s", sketch(root))
	}
}

func TestDeriveRefusesAmbiguity(t *testing.T) {
	g := build(t, "File ::= File PLUS File | IDENT")
	_, err := g.Derive(toks(token.Ident, token.Plus, token.Ident, token.Plus, token.Ident))
	if err == nil {
		t.Fatal("an ambiguous input has no single tree and must not silently get one")
	}
}

func TestDeriveRefusesNonDerivation(t *testing.T) {
	g := build(t, "File ::= IDENT")
	if _, err := g.Derive(toks(token.Plus)); err == nil {
		t.Fatal("input that does not derive should error")
	}
}
