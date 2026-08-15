// The inventory pin (parser-implementation.md §5, lexer-testing-plan §1's "build it first").
//
// This is what let §1 choose a kind tag over a struct per construct, and what lets the
// constants be hand-written: the enum is checkable against grammar.md §0 both ways, where a
// struct per production would let a production added to §0 with nothing behind it pass
// unnoticed — the R232 defect class one level up.
//
// In-package, because the numeric half is about firstNode.
package parser

import (
	"strings"
	"testing"

	"luna/internal/ebnf"
	"luna/oracle/token"
)

// §0's arithmetic, which the tests below enforce rather than assume.
const (
	specNonterminals = 130                                     // grammar.md §10's total
	pureAlternations = 26                                      // never survive into a tree (§5)
	nodeKindCount    = specNonterminals - pureAlternations + 1 // + Type, kept back
)

func loadGrammar(t *testing.T) *ebnf.Grammar {
	t.Helper()
	g, err := ebnf.Load()
	if err != nil {
		t.Fatalf("loading grammar.md: %v", err)
	}
	return g
}

// survivingNonterminals applies §5's rule to the grammar itself. Computed rather than listed,
// so a new operator class joins the collapsing set by being written into §0; `Type` is the one
// override (R256).
func survivingNonterminals(t *testing.T, g *ebnf.Grammar) map[string]bool {
	t.Helper()
	pure := g.PureAlternations()
	out := map[string]bool{}
	for _, n := range g.Nonterminals() {
		if strings.ContainsRune(n, '·') {
			continue // a desugar artifact, not a name grammar.md writes
		}
		if pure[n] && n != "Type" {
			continue
		}
		out[n] = true
	}
	// Every check here compares against this set, so a reader that extracted nothing would make
	// them all pass while verifying nothing.
	if len(out) != nodeKindCount {
		t.Fatalf("§0 yields %d surviving nonterminals, want %d (%d defined − %d pure + Type); "+
			"either the grammar changed or the reader is broken, and the pin below is vacuous "+
			"until this is resolved",
			len(out), nodeKindCount, specNonterminals, pureAlternations)
	}
	return out
}

// TestKindsMatchGrammar is the pin, in both directions. A missing kind means a production §0
// derives that the tree cannot name; an extra one means a kind naming nothing the grammar
// derives.
func TestKindsMatchGrammar(t *testing.T) {
	want := survivingNonterminals(t, loadGrammar(t))

	got := map[string]bool{}
	for _, k := range AllNodes() {
		if k == Error {
			continue // the one parser-only kind, and the only exception this pin admits (§6.3)
		}
		name := k.String()
		if name == "" || name == "UNKNOWN" {
			t.Errorf("kind %d has no name in nodeNames", uint16(k))
			continue
		}
		if got[name] {
			t.Errorf("%s is the name of more than one kind", name)
		}
		got[name] = true
	}

	for n := range want {
		if !got[n] {
			t.Errorf("%s survives into a tree and has no Kind", n)
		}
	}
	for n := range got {
		if !want[n] {
			t.Errorf("Kind %s names no surviving §0 nonterminal — it was invented here, or §0 "+
				"dropped it, or it became a pure alternation and now always collapses", n)
		}
	}
}

// TestTokenRangeIsMirrored is the other half of §5's single kind space: Kind(tk.Kind) must be a
// conversion and not a translation. Asserted per token rather than by comparing two totals,
// since enums of equal size can still disagree value by value — and the teeth are in the name
// comparison, because a node kind inside the token range would take its name from nodeNames
// instead of delegating.
func TestTokenRangeIsMirrored(t *testing.T) {
	if got := len(token.All()) + 1; got != tokenValues {
		t.Fatalf("token.Kind occupies %d values with Unset, tokenValues says %d — lexer §0 "+
			"grew and the node kinds now start inside the token range", got, tokenValues)
	}
	if Unset != 0 {
		t.Errorf("Unset is %d, want 0: the zero value must name nothing in both enums", Unset)
	}
	for _, tk := range token.All() {
		k := Kind(tk)
		if !k.IsToken() {
			t.Errorf("Kind(%s) is %d, at or above firstNode (%d) — it collides with a node kind",
				tk, uint16(k), uint16(firstNode))
			continue
		}
		if k.String() != tk.String() {
			t.Errorf("Kind(%s).String() is %q, want %q", tk, k.String(), tk.String())
		}
	}
}

// TestNodeKindRange checks the arithmetic nothing else can see. A kind added without a
// nonterminal fails the pin above; a nonterminal added without a count to match fails here.
func TestNodeKindRange(t *testing.T) {
	if File != firstNode {
		t.Errorf("the node kinds start at %d, not at firstNode (%d)", uint16(File), uint16(firstNode))
	}
	if got := int(Error - firstNode); got != nodeKindCount {
		t.Errorf("%d node kinds are declared before Error, want %d", got, nodeKindCount)
	}
	if got := len(AllNodes()); got != nodeKindCount+1 {
		t.Errorf("AllNodes returned %d kinds, want %d — the §0 kinds and Error", got, nodeKindCount+1)
	}
	if Error.IsToken() {
		t.Error("Error reports as a token kind")
	}
}
