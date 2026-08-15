package parser

import (
	"fmt"
	"strings"

	"luna/internal/ebnf"
)

// Rendering a derivation into a golden's tree section.
//
// **The `internal/ebnf` dependency here is a scaffold, and it is load-bearing while it lasts.**
// A golden's expectation has to come from somewhere, and until this package produces trees the
// only source is grammar.md itself: `ebnf.Derive` reconstructs the one derivation from the
// Earley chart, and refuses an ambiguous input rather than choosing among its trees. That gives
// the goldens the provenance golden.md §3 asks for — read against §0, because §0 wrote them.
//
// When Parse lands and has its own node type, RenderGolden takes that instead and this import
// goes. The transforms below do not change: they are the format's, not the chart's.
//
// One span cannot come from the chart. A derivation runs over the trivia-filtered stream, so
// every span it offers is the extent of a node's non-trivia tokens, where a parsed node's is
// the extent of its children. §2.1 confines the difference to exactly one node — trivia is
// never the first or last child of anything but File — so File takes the whole file here, and
// everything beneath it keeps the chart's arithmetic, which the tree will agree with.

// A nonterminal collapses when a production passes through it with one child, and it does so
// for one of two reasons — golden.md §2, where the reasoning lives.
//
// **Pure alternations** are computed from the grammar, not listed (ebnf.PureAlternations): a
// name whose every production is one symbol is dispatch, and its child says what it said.
//
// **The precedence tiers** are listed, because they are not distinguishable by shape. The
// obvious blanket rule — "one child, no token of its own" — would also delete the five type
// tiers in a row, and then `let x: int = y;` prints two bare IDENTs with nothing recording
// which of them is in type position.
var goldenTiers = map[string]bool{
	// the expression tiers
	"Expr": true, "Assignment": true, "WordPrefix": true, "Conditional": true,
	"Coalesce": true, "Disjunction": true, "Conjunction": true, "Equality": true,
	"Comparison": true, "RangeExpr": true, "Additive": true, "Multiplicative": true,
	"PrefixExpr": true, "ApplyExpr": true, "PostfixExpr": true, "Primary": true,
	// the type tiers
	"UnionType": true, "IntersectType": true, "PostfixType": true, "PrimaryType": true,
	// the pattern tiers
	"Pattern": true, "AltPattern": true, "PrimaryPattern": true,
}

// goldenKeep overrides both rules. `Type ::= FnType | UnionType` is a pure alternation by shape
// and the one place that is wrong: it is the entry into a different sub-language, and eliding it
// leaves a bare IDENT indistinguishable from an expression's. Its name *is* the information —
// which is the whole hazard golden.md §2 records.
var goldenKeep = map[string]bool{"Type": true}

// goldenView is a node after the format's transforms: synthetics spliced away, absent optionals
// dropped, unit tiers collapsed.
type goldenView struct {
	name     string
	terminal bool
	tok      int
	from, to int // token indices
	kids     []*goldenView
}

// normalizeGolden rewrites one derivation node into the nodes it contributes to its parent —
// none, one, or several.
//
// Three transforms, in the order they have to happen. A **synthetic** splices: `*` and groups
// desugar to right-recursive `LHS·n` helpers, so the derivation nests where a hand-written
// parser loops, and the golden should show the loop. An **empty** node vanishes: an absent
// optional has nothing to pin, and printing one would make every golden a census of what is not
// there. A **unit tier** collapses last, once its children are known.
func normalizeGolden(n *ebnf.Node, pure map[string]bool) []*goldenView {
	if n.Terminal {
		return []*goldenView{{name: n.Name, terminal: true, tok: n.Token, from: n.From, to: n.To}}
	}
	var kids []*goldenView
	for _, c := range n.Children {
		kids = append(kids, normalizeGolden(c, pure)...)
	}
	if n.Synthetic() {
		return kids
	}
	if len(kids) == 0 {
		return nil
	}
	if len(kids) == 1 && !goldenKeep[n.Name] && (pure[n.Name] || goldenTiers[n.Name]) {
		return kids
	}
	return []*goldenView{{name: n.Name, from: n.From, to: n.To, kids: kids}}
}

// RenderGolden turns a derivation into a golden's tree section, byte spans and all. The root is
// printed rather than walked to because its span is the file's, not the chart's (see above);
// normalizeGolden yields one view for File, which never collapses, or none when every token in
// the file is trivia.
func RenderGolden(g *ebnf.Grammar, root *ebnf.Node, lx *LexedGolden) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s 0..%d\n", root.Name, len(lx.Source))
	for _, v := range normalizeGolden(root, g.PureAlternations()) {
		for _, kid := range v.kids {
			writeGolden(&b, kid, 1, lx)
		}
	}
	return b.String()
}

func writeGolden(b *strings.Builder, v *goldenView, depth int, lx *LexedGolden) {
	lo, hi := lx.Toks[v.from].Offset, lx.Toks[v.to-1].End()
	b.WriteString(strings.Repeat("  ", depth))
	if v.terminal {
		fmt.Fprintf(b, "%s %d..%d %q\n", v.name, lo, hi, lx.Source[lo:hi])
		return
	}
	fmt.Fprintf(b, "%s %d..%d\n", v.name, lo, hi)
	for _, k := range v.kids {
		writeGolden(b, k, depth+1, lx)
	}
}

// GoldenTree derives a case's source under the grammar and renders it. The error is the
// interesting half: a case that does not derive, or derives twice, has no golden to write.
func GoldenTree(g *ebnf.Grammar, c *Golden) (string, error) {
	lx, err := LexGolden(c.Name()+".luna", c.Source)
	if err != nil {
		return "", fmt.Errorf("lexing: %w", err)
	}
	root, err := g.Derive(lx.Input)
	if err != nil {
		return "", err
	}
	return RenderGolden(g, root, lx), nil
}
