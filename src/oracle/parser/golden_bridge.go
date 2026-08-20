package parser

import (
	"fmt"

	"luna/internal/ebnf"
	"luna/oracle/diagnostic"
)

// **This file is scaffolding and is meant to be deleted**, taking the package's only
// `internal/ebnf` import with it when Phase 2's parser emits its own events.
//
// What it buys until then: a golden's expectation came from grammar.md itself, so turning that
// derivation into the events the parser *will* emit exercises build, splice, the spans and the
// navigation API over every golden, on real files with real trivia. The first golden to fail once `parse`
// lands is then unambiguously the parser's fault.
//
// The elision therefore happens on the emit side, since these are the events the parser is
// expected to produce rather than a rendering convenience. The tables below die with the file,
// a recursive-descent parser opening a tier only when it fires.

// The tiers are listed where the pure alternations are computed, because no shape distinguishes
// them. golden.md §2 has the reasoning and the hazard.
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

// Type is a pure alternation by shape and the one that must survive anyway: eliding it leaves a
// bare IDENT in type position indistinguishable from an expression's (R256).
var goldenKeep = map[string]bool{"Type": true}

type goldenWalk struct {
	pure  map[string]bool
	kinds map[string]Kind
	index []int // filtered token position → full-stream index
}

// goldenKinds is built here rather than exported from kind.go because the bridge is the only
// caller there will ever be: the parser knows the kind at each call site.
func goldenKinds() map[string]Kind {
	m := make(map[string]Kind)
	for _, k := range AllNodes() {
		m[k.String()] = k
	}
	return m
}

// events returns the events a derivation node contributes and **how many nodes** they are, which
// the event count cannot say and the collapse rule needs.
//
// A synthetic splices, because `*` and groups desugar to `LHS·n` helpers and the tree should show
// the loop the parser writes. An empty node contributes nothing; dropping it here is what keeps
// it from counting as a child below. A unit tier collapses last, once its children are known.
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

func (w goldenWalk) wrap(name string, kids eventStream) eventStream {
	k, ok := w.kinds[name]
	if !ok {
		// Unreachable while kind_test.go's pin holds, and the one thing that pin exists to catch.
		panic(diagnostic.Bugf("parser: %s survives into a tree and has no Kind", name))
	}
	out := make(eventStream, 0, len(kids)+2)
	out = append(out, event{kind: evOpen, node: k})
	out = append(out, kids...)
	return append(out, event{kind: evClose})
}

// goldenRun keeps every stage, because §2.3's invariants are asserted on different ones and
// splice's contract is a relation between the two streams rather than a property of either.
type goldenRun struct {
	lexed     *LexedGolden
	unspliced eventStream
	events    eventStream
	tree      *Tree
}

func runGolden(g *ebnf.Grammar, c *Golden) (*goldenRun, error) {
	return runSource(g, c.Name()+".luna", c.Source)
}

// runSource derives one source and runs it through the parser's own stages. The error is the
// interesting half: a source that does not derive, or derives twice, has no tree to build.
//
// It takes bare source because the spec corpus is not a golden: blocks the parser must handle
// that nobody wrote expectations for.
func runSource(g *ebnf.Grammar, name, src string) (*goldenRun, error) {
	lexed, err := LexGolden(name, src)
	if err != nil {
		return nil, fmt.Errorf("lexing: %w", err)
	}
	root, err := g.Derive(lexed.Input)
	if err != nil {
		return nil, err
	}
	w := goldenWalk{pure: g.PureAlternations(), kinds: goldenKinds(), index: lexed.index}

	// The root is opened unconditionally where every other node earns its open: a file of only
	// comments derives no tokens, and File still has to be there for splice to fill (§6.1).
	var kids eventStream
	for _, child := range root.Children {
		ev, _ := w.events(child)
		kids = append(kids, ev...)
	}
	unspliced := w.wrap(root.Name, kids)
	events := splice(lexed.Tokens, unspliced)
	return &goldenRun{
		lexed:     lexed,
		unspliced: unspliced,
		events:    events,
		tree:      build(lexed.File, lexed.Tokens, events),
	}, nil
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
