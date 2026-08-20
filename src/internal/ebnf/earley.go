package ebnf

import (
	"fmt"
	"strings"

	"luna/oracle/token"
)

// Earley is chosen over every generator, and the reason is the spec-literal property rather
// than convenience (R253, R260). An LL or LR generator demands the grammar be left-factored
// and its left recursion removed; those rewrites would make the running artifact structurally
// different from §0, which is the one thing this tool must not be. A PEG would be worse still:
// ordered choice *resolves* ambiguity silently, where detecting it is half the point.
//
// Earley takes any context-free grammar unchanged: left-recursive, ambiguous, epsilon-ridden.

// Token is the recognizer's input: a kind and the lexeme, since a terminal may require both
// (grammar.md's IDENT("from")).
type Token struct {
	Kind token.Kind
	Text string
}

// item is a production with a dot at Pos, begun at Origin.
type item struct {
	prod   int
	pos    int
	origin int
}

// key locates an item in the chart.
type key struct {
	set int
	it  item
}

// cause records *why* an item entered the chart, which is what makes ambiguity visible.
//
// Counting completed start items does not work, and the failure is silent: Earley
// deduplicates items, so two derivations that finish through the same production are one
// item and the count says "1". Only ambiguity at the very top production would ever show.
// Recording each item's causes turns the chart into a parse forest, where an item reached two
// different ways is an ambiguous node, which is the real question.
//
// A scan has exactly one cause and can never be ambiguous. A completion's cause names both
// halves: the item that was waiting, and the completed item that satisfied it.
type cause struct {
	prev  key // the item this advanced from
	child key // the completed item that licensed it; zero for a scan
	scan  bool
}

// Result is what a run reports.
type Result struct {
	Accepted  bool
	Ambiguous bool
	Furthest  int // the highest token index any item advanced to

	// Expected is the terminals that would have let the parse continue at Furthest, sorted.
	// It is the grammar's own answer to "what belongs here", and it is filled in only on a
	// rejection, since computing it for every accepted input would cost the generator dearly
	// and tell it nothing.
	Expected []string
}

// chart is one run's completed Earley state: the item sets in insertion order, the causes that
// put each item there, and the accepting items. Recognize reduces it to three booleans; Derive
// walks it back into a tree. Both come from one run so that neither can disagree with the other
// about what the grammar did.
type chart struct {
	order    [][]item
	causes   map[key][]cause
	roots    []key
	furthest int
}

// Recognize runs the grammar over toks from the start symbol.
func (g *Grammar) Recognize(toks []Token) Result {
	c := g.chart(toks)
	r := Result{
		Accepted:  len(c.roots) > 0,
		Ambiguous: len(c.roots) > 1 || anyAmbiguous(c.roots, c.causes),
		Furthest:  c.furthest,
	}
	if !r.Accepted {
		r.Expected = g.expectedAt(c, c.furthest)
	}
	return r
}

// expectedAt reads the frontier: every terminal sitting immediately after a dot in the item set
// the parse reached. That set *is* the grammar's answer to "what could come next here", and it
// is the raw material of a syntax diagnostic, though not the diagnostic itself: a long list of
// admissible tokens is a fact, not a sentence anyone wants to read.
func (g *Grammar) expectedAt(c *chart, at int) []string {
	if at < 0 || at >= len(c.order) {
		return nil
	}
	seen := map[string]bool{}
	for _, it := range c.order[at] {
		p := g.Prods[it.prod]
		// A guard is skipped: it expects nothing, it forbids. Where it does not fire, the item
		// past it is in this same set and speaks for the production.
		if it.pos < len(p.RHS) && p.RHS[it.pos].IsTerminal && !p.RHS[it.pos].Negate {
			seen[p.RHS[it.pos].String()] = true
		}
	}
	return sortedKeys(seen)
}

func (g *Grammar) chart(toks []Token) *chart {
	n := len(toks)
	sets := make([]map[item]bool, n+1)
	order := make([][]item, n+1)
	causes := map[key][]cause{}
	for i := range sets {
		sets[i] = map[item]bool{}
	}

	add := func(k int, it item, c *cause) {
		if !sets[k][it] {
			sets[k][it] = true
			order[k] = append(order[k], it)
		}
		if c == nil {
			return
		}
		kk := key{set: k, it: it}
		for _, existing := range causes[kk] {
			if existing == *c {
				return
			}
		}
		causes[kk] = append(causes[kk], *c)
	}

	for _, pi := range g.byLHS[g.Start] {
		add(0, item{prod: pi}, nil)
	}

	furthest := 0
	for k := 0; k <= n; k++ {
		for i := 0; i < len(order[k]); i++ {
			it := order[k][i]
			p := g.Prods[it.prod]

			if it.pos == len(p.RHS) { // completion
				for _, src := range order[it.origin] {
					sp := g.Prods[src.prod]
					if src.pos < len(sp.RHS) && !sp.RHS[src.pos].IsTerminal &&
						sp.RHS[src.pos].Name == p.LHS {
						next := item{prod: src.prod, pos: src.pos + 1, origin: src.origin}
						add(k, next, &cause{
							prev:  key{set: it.origin, it: src},
							child: key{set: k, it: it},
						})
					}
				}
				continue
			}

			sym := p.RHS[it.pos]
			if sym.Negate { // a guard: zero-width, and it only forbids
				if k < n && matches(sym, toks[k]) {
					continue // the guard fires, so this production is not available here
				}
				next := item{prod: it.prod, pos: it.pos + 1, origin: it.origin}
				add(k, next, &cause{prev: key{set: k, it: it}, scan: true})
				continue
			}
			if !sym.IsTerminal { // prediction
				for _, pi := range g.byLHS[sym.Name] {
					add(k, item{prod: pi, pos: 0, origin: k}, nil)
				}
				// An epsilon production completes in the same set, so an item waiting on a
				// nullable nonterminal must be advanced here too: the classic Earley
				// nullable bug, and the reason `A B? C` parses at all.
				for _, pi := range g.byLHS[sym.Name] {
					if len(g.Prods[pi].RHS) == 0 {
						next := item{prod: it.prod, pos: it.pos + 1, origin: it.origin}
						add(k, next, &cause{
							prev:  key{set: k, it: it},
							child: key{set: k, it: item{prod: pi, pos: 0, origin: k}},
						})
					}
				}
				continue
			}
			if k < n && matches(sym, toks[k]) { // scan
				next := item{prod: it.prod, pos: it.pos + 1, origin: it.origin}
				add(k+1, next, &cause{prev: key{set: k, it: it}, scan: true})
				if k+1 > furthest {
					furthest = k + 1
				}
			}
		}
	}

	var roots []key
	for _, it := range order[n] {
		p := g.Prods[it.prod]
		if it.origin == 0 && it.pos == len(p.RHS) && p.LHS == g.Start {
			roots = append(roots, key{set: n, it: it})
		}
	}

	return &chart{order: order, causes: causes, roots: roots, furthest: furthest}
}

// anyAmbiguous walks back from the accepting items over the causes that actually contributed
// to a parse, and reports whether any of them was reached more than one way.
//
// The walk is restricted to reachable items on purpose. An item elsewhere in the chart may
// well have two causes without the input being ambiguous, since Earley explores derivations
// that go nowhere, so counting over the whole chart would report ambiguity that is not there.
func anyAmbiguous(roots []key, causes map[key][]cause) bool {
	seen := map[key]bool{}
	stack := append([]key{}, roots...)
	for len(stack) > 0 {
		k := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[k] {
			continue
		}
		seen[k] = true

		cs := causes[k]
		if len(cs) > 1 {
			return true
		}
		for _, c := range cs {
			stack = append(stack, c.prev)
			if !c.scan {
				stack = append(stack, c.child)
			}
		}
	}
	return false
}

func matches(s Sym, t Token) bool {
	if s.Name != t.Kind.String() {
		return false
	}
	return s.Text == "" || s.Text == t.Text
}

// Explain renders the furthest point reached, which is the only diagnostic this tool owes.
// Real syntax errors are the oracle parser's job and carry P codes (grammar §11); here the
// question is only whether the grammar derives the input, so "it stopped here" is enough to
// find the production at fault.
func (r Result) Explain(toks []Token) string {
	if r.Accepted {
		return "accepted"
	}
	where := "input ended early"
	if r.Furthest < len(toks) {
		where = fmt.Sprintf("no parse at token %d (%s %q)",
			r.Furthest, toks[r.Furthest].Kind, toks[r.Furthest].Text)
	}
	// Frontiers are bimodal over this grammar: a cluster of small ones, then a jump to sizes an
	// order larger. Below the gap the list *is* the message; above it, it is every way to start
	// an expression and says nothing. So the cut sits in the gap the data shows, not at a round
	// number picked for tidiness.
	if len(r.Expected) > 0 && len(r.Expected) <= 6 {
		where += ", expected " + strings.Join(r.Expected, " or ")
	}
	return where
}
