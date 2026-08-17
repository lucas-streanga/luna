package ebnf

import "fmt"

// Derivation trees, reconstructed from the Earley chart.
//
// This exists so that a parse golden can be written **before the parser is**. The tree comes
// out of grammar.md's own productions rather than out of an implementation of them, which is
// the provenance a golden wants: `oracle/parser/testdata/golden.md` says a golden is trustworthy
// only when it was read against §0, and here §0 is what produced it.
//
// Derive refuses an ambiguous input rather than picking a derivation. A parse *forest* has no
// single tree to render, and quietly rendering one of them would turn the one defect this
// package exists to find into a golden that enshrines it.

// Node is one node of a derivation tree.
//
// Spans are half-open **token** indices, not byte offsets: this package never sees the source
// text. Whoever holds the lexed tokens converts, which is exactly the seam the golden format
// needs, since its spans are the lexer's.
type Node struct {
	Name     string // nonterminal name, or the token kind for a leaf
	Terminal bool
	From, To int // token index span, half-open
	Token    int // index of the token itself; leaves only
	Children []*Node
}

// Synthetic reports whether the node is a desugar artifact rather than a name grammar.md
// writes. `?`, `*` and groups mint `LHS·n` nonterminals (parse.go), and the interpunct cannot
// occur in a spec name, so the test is exact.
func (n *Node) Synthetic() bool {
	for _, r := range n.Name {
		if r == '·' {
			return true
		}
	}
	return false
}

// Derive returns the one derivation of toks from the start symbol.
func (g *Grammar) Derive(toks []Token) (*Node, error) {
	c := g.chart(toks)
	if len(c.roots) == 0 {
		r := Result{Furthest: c.furthest}
		return nil, fmt.Errorf("no derivation: %s", r.Explain(toks))
	}
	if len(c.roots) > 1 || anyAmbiguous(c.roots, c.causes) {
		return nil, fmt.Errorf("%d derivations: an ambiguous input has no single tree", len(c.roots))
	}
	return g.node(c, c.roots[0])
}

// node rebuilds the subtree for one completed item.
//
// The chart records how an item was reached, not what it contains, so the children come out by
// walking the dot backwards: each cause names the item one position earlier plus the thing that
// filled the gap — a scanned terminal, or a completed nonterminal. Walking from the end means
// they arrive in reverse, which is why the slice is turned at the finish.
func (g *Grammar) node(c *chart, k key) (*Node, error) {
	p := g.Prods[k.it.prod]
	out := &Node{Name: p.LHS, From: k.it.origin, To: k.set}

	cur := k
	for cur.it.pos > 0 {
		cs := c.causes[cur]
		if len(cs) != 1 {
			// Unreachable once Derive has ruled out ambiguity, and cheap enough to assert:
			// silently taking cs[0] here is precisely how a forest becomes a tree by accident.
			return nil, fmt.Errorf("%s at %d..%d was reached %d ways", p.LHS, k.it.origin, k.set, len(cs))
		}
		if cs[0].scan {
			at := cs[0].prev.set
			sym := g.Prods[cur.it.prod].RHS[cur.it.pos-1]
			// A guard advanced the dot without consuming, so it has a cause and no child: it is
			// an assertion about the input, not a part of the derivation (R270).
			if !sym.Negate {
				out.Children = append(out.Children, &Node{
					Name: sym.Name, Terminal: true, From: at, To: at + 1, Token: at,
				})
			}
		} else {
			child, err := g.node(c, cs[0].child)
			if err != nil {
				return nil, err
			}
			out.Children = append(out.Children, child)
		}
		cur = cs[0].prev
	}

	for i, j := 0, len(out.Children)-1; i < j; i, j = i+1, j-1 {
		out.Children[i], out.Children[j] = out.Children[j], out.Children[i]
	}
	return out, nil
}
