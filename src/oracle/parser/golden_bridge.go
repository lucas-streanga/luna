package parser

import (
	"fmt"

	"luna/internal/ebnf"
)

// **This file is scaffolding, and it is meant to be deleted.** It is the only thing in the
// package that reaches for `internal/ebnf`, and when Phase 2's parser emits its own events this
// file goes with the import — `golden_render.go`'s header has predicted that since before either
// existed.
//
// What it buys while it lasts is the reason to write it. A golden's expectation came from
// grammar.md itself (`ebnf.Derive` reconstructs the one derivation from the Earley chart), so
// turning that derivation into the events the parser *will* emit runs build, splice, the span
// arithmetic and the whole navigation API over thirty real files, with real trivia, before any
// parsing exists. The first golden that fails once `parse` lands is then unambiguously the
// parser's fault.
//
// So the elision golden.md §2 describes moves to the emit side here: the events this produces
// are the events the parser is expected to produce, not a rendering convenience. The tier
// tables below travel with the file for the same reason — a recursive-descent parser opens a
// tier only when it fires, so it never needs to be told which ones collapse.

// A nonterminal collapses when a production passes through it with one child, and it does so for
// one of two reasons — golden.md §2, where the reasoning lives.
//
// **Pure alternations** are computed from the grammar, not listed (ebnf.PureAlternations): a
// name whose every production is one symbol is dispatch, and its child says what it said.
//
// **The precedence tiers** are listed, because they are not distinguishable by shape. The
// obvious blanket rule — "one child, no token of its own" — would also delete the five type
// tiers in a row, and then `let x: int = y;` prints two bare IDENTs with nothing recording which
// of them is in type position.
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

// goldenWalk is one case's conversion state.
type goldenWalk struct {
	pure  map[string]bool
	kinds map[string]Kind
	index []int // filtered token position → full-stream index
}

// goldenKinds maps §0's names onto the enum. Built here rather than exported from kind.go
// because the bridge is the only caller there will ever be: the parser knows the kind at each
// call site, having a function per nonterminal.
func goldenKinds() map[string]Kind {
	m := make(map[string]Kind)
	for _, k := range AllNodes() {
		m[k.String()] = k
	}
	return m
}

// events turns one derivation node into the events it contributes, and reports how many nodes
// those are — which is what the collapse rule needs and what the event count cannot say, since a
// single child may itself be a whole subtree.
//
// Three transforms, in the order they have to happen. A **synthetic** splices: `*` and groups
// desugar to right-recursive `LHS·n` helpers, so the derivation nests where a hand-written
// parser loops, and the tree should show the loop. An **empty** node contributes nothing — an
// absent optional has no node behind it, and the builder would delete it anyway (§6.1); dropping
// it here is what keeps it from being counted as a child by the rule below. A **unit tier**
// collapses last, once its children are known.
func (w goldenWalk) events(n *ebnf.Node) (eventStream, int) {
	if n.Terminal {
		return eventStream{{kind: evToken, tok: w.index[n.Token]}}, 1
	}
	var kids eventStream
	nodes := 0
	for _, c := range n.Children {
		ev, count := w.events(c)
		kids = append(kids, ev...)
		nodes += count
	}
	if n.Synthetic() || nodes == 0 {
		return kids, nodes
	}
	if nodes == 1 && !goldenKeep[n.Name] && (w.pure[n.Name] || goldenTiers[n.Name]) {
		return kids, nodes
	}
	return w.wrap(n.Name, kids), 1
}

// wrap opens a node around events that are already its children's.
func (w goldenWalk) wrap(name string, kids eventStream) eventStream {
	k, ok := w.kinds[name]
	if !ok {
		// Unreachable while kind_test.go's pin holds, and worth saying out loud: a §0
		// nonterminal with no Kind is the one thing that pin exists to catch.
		panic(fmt.Sprintf("parser: %s survives into a tree and has no Kind", name))
	}
	out := make(eventStream, 0, len(kids)+2)
	out = append(out, event{kind: evOpen, node: k})
	out = append(out, kids...)
	return append(out, event{kind: evClose})
}

// goldenRun is one case taken all the way through the machine, with the intermediate kept: the
// invariants of §2.3 are asserted on different stages, index coverage on the events and the
// placement rule on the tree.
type goldenRun struct {
	lex  *LexedGolden
	evs  eventStream // spliced
	tree *Tree
}

// runGolden derives a case under the grammar and runs the derivation through the parser's own
// stages. The error is the interesting half: a case that does not derive, or derives twice, has
// no tree to build.
func runGolden(g *ebnf.Grammar, c *Golden) (*goldenRun, error) {
	lx, err := LexGolden(c.Name()+".luna", c.Source)
	if err != nil {
		return nil, fmt.Errorf("lexing: %w", err)
	}
	root, err := g.Derive(lx.Input)
	if err != nil {
		return nil, err
	}
	w := goldenWalk{pure: g.PureAlternations(), kinds: goldenKinds(), index: lx.index}

	// The root is opened unconditionally, where every other node earns its open. A file of
	// nothing but comments derives no tokens at all, and File still has to be there to hold
	// them — §6.1's iff is that File is empty exactly when the file is, and splice fills it on
	// the way past.
	var kids eventStream
	for _, child := range root.Children {
		ev, _ := w.events(child)
		kids = append(kids, ev...)
	}
	evs := splice(lx.Toks, w.wrap(root.Name, kids))
	return &goldenRun{lex: lx, evs: evs, tree: build(lx.File, lx.Toks, evs)}, nil
}

// GoldenTree renders the tree a case's source must produce, derived from grammar.md and built by
// this package's own builder.
func GoldenTree(g *ebnf.Grammar, c *Golden) (string, error) {
	run, err := runGolden(g, c)
	if err != nil {
		return "", err
	}
	return RenderGolden(run.tree), nil
}
