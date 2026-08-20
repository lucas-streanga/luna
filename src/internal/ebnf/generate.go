package ebnf

import (
	"fmt"
	"sort"
	"strings"

	"luna/oracle/token"
)

// Bounded exhaustive generation: the ambiguity *search*, where Recognize is the ambiguity
// *test*.
//
// Running the corpus proves the grammar unambiguous over the inputs someone happened to write.
// That is evidence, and it is the weaker half: every ambiguity R264 fixed was found by running
// real Luna, none by imagining cases, which is exactly the blind spot: an ambiguous form
// nobody has written yet is invisible to it. This closes the other half. Enumerate every
// sentence the grammar derives up to a length and ask the recognizer about each one; within
// the bound, a clean answer is a proof rather than evidence.
//
// **The generator deliberately does not decide ambiguity itself.** Enumerating derivation
// *trees* and colliding their yields would answer the question directly, and would be a
// second implementation of the thing R264 records getting wrong once already, silently. Every
// verdict routes through Recognize instead, so the package has exactly one ambiguity oracle,
// and that buys a mutual check for free: a sentence this generator emits that the recognizer
// *rejects* means one of the two is wrong about the same grammar. Report.Unrecognized is where
// that lands, and it should always be empty.
//
// The bound is a length in tokens, not a derivation depth. Ambiguity is a property of a
// string, so the natural quantifier is "every string this short", and enumerating by length
// gives a termination argument that needs no depth cap: the set of strings of a fixed length
// over a finite alphabet is finite, so the per-length fixpoint below always converges.

// Bound is how far an enumeration goes.
type Bound struct {
	// Start is the nonterminal to enumerate from. Empty means the grammar's own start symbol.
	// Naming a smaller one (Type, Pattern, Expr) is the usual move: the interesting corners
	// are sub-languages, and File spends its budget on statement scaffolding before it reaches
	// them.
	Start string

	// MaxLen is the longest sentence to generate, in tokens.
	MaxLen int

	// MaxPerCell caps the distinct strings kept for one (nonterminal, length) pair. Zero means
	// no cap. A run that hits it is no longer exhaustive and says so; see Report.Exhaustive.
	MaxPerCell int

	// Spellings additionally emits, for every *unconstrained* terminal, each lexeme that some
	// production requires of that kind: the IDENT("from") / IDENT("get") / IDENT("type")
	// family. A spelling-matched terminal always emits its own lexeme, knob or not, so a
	// collision needing one such spelling is found either way; what this buys is every
	// sentence where an ordinary position happens to carry a reserved-ish word. That is the
	// `import { from } from a` shape, and the combinations across positions: two required
	// spellings in one sentence are unreachable without it, since neither production generates
	// the other's lexeme. It costs a factor of one-plus-the-spellings on every IDENT.
	Spellings bool

	// KeepSentences retains every sentence in Report.All. Off by default because a run's whole
	// point is that there are a great many of them; on, for eyeballing what a bound actually
	// searched, which is the only way to tell a narrow bound from a broken one.
	KeepSentences bool
}

// Sentence is one generated token string.
type Sentence []Token

func (s Sentence) String() string {
	parts := make([]string, len(s))
	for i, t := range s {
		if t.Text != "" {
			parts[i] = fmt.Sprintf("%s(%q)", t.Kind, t.Text)
		} else {
			parts[i] = t.Kind.String()
		}
	}
	return strings.Join(parts, " ")
}

// Truncation records one cell that hit MaxPerCell.
//
// It is reported rather than logged because a silent cap turns "found nothing" into "looked at
// a prefix of nothing" while the test still passes: the fail-open shape check.sh's own notes
// are written against.
type Truncation struct {
	Nonterminal string
	Length      int
	Kept        int
}

// Report is one enumeration's result.
type Report struct {
	Start        string
	MaxLen       int
	Sentences    int   // distinct sentences generated and checked
	ByLen        []int // sentences per length, index 0..MaxLen
	Ambiguous    []Sentence
	Unrecognized []Sentence // generated from the grammar, yet Recognize rejects: a tool bug
	Truncated    []Truncation
	All          []Sentence // every sentence checked, when Bound.KeepSentences is set

	// Cells and Stored describe the whole table, not just the start symbol's slice of it.
	// They are the tuning numbers: the cost of a bound is Stored, while Sentences is only what
	// came out, and the two differ by orders of magnitude wherever the start symbol sits above
	// a large sub-language.
	Cells  int
	Stored int
}

// Exhaustive reports whether the run actually saw every sentence within its bound. A report
// that is not exhaustive proves nothing by being clean.
func (r *Report) Exhaustive() bool { return len(r.Truncated) == 0 }

func (r *Report) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s, sentences up to %d tokens: %d checked (%d cells, %d stored)",
		r.Start, r.MaxLen, r.Sentences, r.Cells, r.Stored)
	if !r.Exhaustive() {
		fmt.Fprintf(&b, " (NOT exhaustive: %d cells truncated)", len(r.Truncated))
	}
	b.WriteByte('\n')
	for l, n := range r.ByLen {
		if n > 0 {
			fmt.Fprintf(&b, "  len %d: %d\n", l, n)
		}
	}
	for _, s := range r.Ambiguous {
		fmt.Fprintf(&b, "  AMBIGUOUS: %s\n", s)
	}
	for _, s := range r.Unrecognized {
		fmt.Fprintf(&b, "  UNRECOGNIZED: %s\n", s)
	}
	for _, t := range r.Truncated {
		fmt.Fprintf(&b, "  truncated: %s at length %d, kept %d\n", t.Nonterminal, t.Length, t.Kept)
	}
	return b.String()
}

// Enumerate generates every sentence the grammar derives within the bound and checks each one.
func (g *Grammar) Enumerate(b Bound) (*Report, error) {
	start := b.Start
	if start == "" {
		start = g.Start
	}
	if !g.Defines(start) {
		return nil, fmt.Errorf("start symbol %q has no production", start)
	}
	if b.MaxLen < 0 {
		return nil, fmt.Errorf("MaxLen %d is negative", b.MaxLen)
	}
	kinds, err := kindsByName()
	if err != nil {
		return nil, err
	}
	for _, name := range g.Terminals() {
		if _, ok := kinds[name]; !ok {
			return nil, fmt.Errorf("terminal %s is no lexer §0 token", name)
		}
	}

	e := &enumerator{
		g:     g,
		prods: g.reachableFrom(start),
		bound: b,
		spell: spellings(g, b.Spellings),
		min:   g.minLens(),
		cells: map[cellKey]*cell{},
	}
	e.run()

	rep := &Report{Start: start, MaxLen: b.MaxLen, ByLen: make([]int, b.MaxLen+1)}
	sub := *g
	sub.Start = start
	for l := 0; l <= b.MaxLen; l++ {
		c := e.cells[cellKey{start, l}]
		if c == nil {
			continue
		}
		rep.ByLen[l] = len(c.list)
		rep.Sentences += len(c.list)
		for _, enc := range c.list {
			toks, err := decodeSentence(enc, kinds)
			if err != nil {
				return nil, err
			}
			if b.KeepSentences {
				rep.All = append(rep.All, toks)
			}
			res := sub.Recognize(toks)
			if !res.Accepted {
				rep.Unrecognized = append(rep.Unrecognized, toks)
				continue
			}
			if res.Ambiguous {
				rep.Ambiguous = append(rep.Ambiguous, toks)
			}
		}
	}
	rep.Truncated = e.truncations()
	rep.Cells = len(e.cells)
	for _, c := range e.cells {
		rep.Stored += len(c.list)
	}
	return rep, nil
}

// --- the enumerator --------------------------------------------------------------------

type cellKey struct {
	name   string
	length int
}

// cell holds the distinct sentences one nonterminal derives at one length.
//
// The list is kept alongside the set purely for **determinism**: a capped run must keep the
// same sentences every time, and Go randomizes map iteration, so an unordered cell would make
// a truncated enumeration report different findings run to run.
type cell struct {
	seen map[string]bool
	list []string
	full bool
}

type enumerator struct {
	g     *Grammar
	prods []Prod // reachable from the start symbol only
	bound Bound
	spell map[string][]string
	min   map[string]int
	cells map[cellKey]*cell
}

// reachableFrom returns the productions a start symbol can use, in grammar order.
//
// Filtering is not an optimization detail, it is what makes a small start symbol mean
// anything: the fixpoint below fills a cell for every production it is given at every length,
// so enumerating `Type` over the whole grammar would cost exactly what enumerating `File`
// costs, and the point of naming a sub-language is to spend the budget inside it.
func (g *Grammar) reachableFrom(start string) []Prod {
	live := map[string]bool{}
	var walk func(string)
	walk = func(n string) {
		if live[n] {
			return
		}
		live[n] = true
		for _, i := range g.byLHS[n] {
			for _, s := range g.Prods[i].RHS {
				if !s.IsTerminal {
					walk(s.Name)
				}
			}
		}
	}
	walk(start)

	var out []Prod
	for _, p := range g.Prods {
		if live[p.LHS] {
			out = append(out, p)
		}
	}
	return out
}

// run fills every cell up to MaxLen.
//
// Lengths ascend, and each length runs to a fixpoint before the next begins. The fixpoint is
// what makes unit productions and left recursion work: at length L a child may itself take all
// L tokens (`Additive ::= Multiplicative`), so the cell being built is one of its own inputs.
// It terminates because cells only grow and the strings of a fixed length are finite.
//
// Rounds after the first are driven by a worklist, and that is not a micro-optimization. Once
// lengths 0..L-1 are final, the *only* mutable input at length L is another length-L cell, so a
// production can produce something new only if one of its own nonterminals just grew. Without
// the worklist a naive fixpoint re-runs every production every round, and grammar.md's
// expression tiers need one round per tier to carry a single string from Primary up to Expr:
// a full pass over the whole grammar to add what the last one added.
func (e *enumerator) run() {
	sufs := make([][]int, len(e.prods))
	uses := make([][]string, len(e.prods))
	for i, p := range e.prods {
		sufs[i] = e.suffixMins(p)
		for _, s := range p.RHS {
			if !s.IsTerminal {
				uses[i] = append(uses[i], s.Name)
			}
		}
	}

	for l := 0; l <= e.bound.MaxLen; l++ {
		first := true
		dirty := map[string]bool{}
		for {
			grew := map[string]bool{}
			for i, p := range e.prods {
				if sufs[i][0] > l {
					continue // this production cannot be this short
				}
				if !first && !anyDirty(uses[i], dirty) {
					continue
				}
				e.build(p, sufs[i], 0, l, "", nil, func(s string) {
					if e.add(p.LHS, l, s) {
						grew[p.LHS] = true
					}
				})
			}
			if len(grew) == 0 {
				break
			}
			first, dirty = false, grew
		}
	}
}

func anyDirty(names []string, dirty map[string]bool) bool {
	for _, n := range names {
		if dirty[n] {
			return true
		}
	}
	return false
}

// build walks one production's right-hand side, handing every complete concatenation of
// exactly rem tokens to emit.
//
// guard is a `!TERMINAL` seen earlier in this right-hand side and not yet discharged (R270). It
// governs the **first token produced after it**, whichever symbol produces it, so it rides along
// until something emits one; a nullable nonterminal in between passes it on rather than
// consuming it. Generating and then filtering would be the alternative and is not available: an
// over-generated sentence is one the recognizer rejects, which lands in Report.Unrecognized and
// is meant to mean the two halves disagree about the grammar.
func (e *enumerator) build(p Prod, suf []int, i, rem int, acc string, guard *Sym, emit func(string)) {
	if i == len(p.RHS) {
		if rem == 0 {
			emit(acc)
		}
		return
	}
	if rem < suf[i] {
		return // the rest of the production cannot fit
	}
	s := p.RHS[i]
	if s.Negate {
		e.build(p, suf, i+1, rem, acc, &p.RHS[i], emit)
		return
	}
	if s.IsTerminal {
		for _, txt := range e.textsFor(s) {
			if guard != nil && guard.Name == s.Name && (guard.Text == "" || guard.Text == txt) {
				continue
			}
			e.build(p, suf, i+1, rem-1, acc+encodeToken(s.Name, txt), nil, emit)
		}
		return
	}
	lo := e.min[s.Name]
	for l := lo; l <= rem-suf[i+1]; l++ {
		c := e.cells[cellKey{s.Name, l}]
		if c == nil {
			continue
		}
		// Indexed rather than ranged: build may extend this very cell through emit, and the
		// additions are legitimate inputs to the same fixpoint round.
		for j := 0; j < len(c.list); j++ {
			next := guard
			if c.list[j] != "" {
				if guard != nil && guardBlocks(*guard, c.list[j]) {
					continue
				}
				next = nil
			}
			e.build(p, suf, i+1, rem-l, acc+c.list[j], next, emit)
		}
	}
}

// guardBlocks reports whether an encoded sentence begins with the token a guard forbids.
func guardBlocks(g Sym, enc string) bool {
	head, _, _ := strings.Cut(enc, tokenSep)
	name, text, _ := strings.Cut(head, fieldSep)
	return g.Name == name && (g.Text == "" || g.Text == text)
}

// add records a sentence, reporting whether it was new.
func (e *enumerator) add(name string, length int, s string) bool {
	k := cellKey{name, length}
	c := e.cells[k]
	if c == nil {
		c = &cell{seen: map[string]bool{}}
		e.cells[k] = c
	}
	if c.seen[s] {
		return false
	}
	if e.bound.MaxPerCell > 0 && len(c.list) >= e.bound.MaxPerCell {
		c.full = true
		return false
	}
	c.seen[s] = true
	c.list = append(c.list, s)
	return true
}

func (e *enumerator) truncations() []Truncation {
	var out []Truncation
	for k, c := range e.cells {
		if c.full {
			out = append(out, Truncation{Nonterminal: k.name, Length: k.length, Kept: len(c.list)})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Nonterminal != out[j].Nonterminal {
			return out[i].Nonterminal < out[j].Nonterminal
		}
		return out[i].Length < out[j].Length
	})
	return out
}

// textsFor returns the lexemes to emit for one terminal. A spelling-matched terminal has
// exactly one; an unconstrained one has the empty lexeme, plus every required spelling of its
// kind when Bound.Spellings is set.
func (e *enumerator) textsFor(s Sym) []string {
	if s.Text != "" {
		return []string{s.Text}
	}
	return e.spell[s.Name]
}

// suffixMins[i] is the fewest tokens the symbols from i onward can yield, which is what prunes
// the composition search from combinatorial to tractable.
func (e *enumerator) suffixMins(p Prod) []int {
	out := make([]int, len(p.RHS)+1)
	for i := len(p.RHS) - 1; i >= 0; i-- {
		w := 1
		switch {
		case p.RHS[i].Negate:
			w = 0 // a guard yields no token
		case !p.RHS[i].IsTerminal:
			w = e.min[p.RHS[i].Name]
		}
		out[i] = saturatingAdd(out[i+1], w)
	}
	return out
}

// minLens is the fewest tokens each nonterminal can yield; unproductive ones stay at infinite.
func (g *Grammar) minLens() map[string]int {
	m := map[string]int{}
	for _, n := range g.Nonterminals() {
		m[n] = infinite
	}
	for {
		changed := false
		for _, p := range g.Prods {
			sum := 0
			for _, s := range p.RHS {
				switch {
				case s.Negate: // a guard yields no token
				case s.IsTerminal:
					sum = saturatingAdd(sum, 1)
				default:
					sum = saturatingAdd(sum, m[s.Name])
				}
			}
			if sum < m[p.LHS] {
				m[p.LHS] = sum
				changed = true
			}
		}
		if !changed {
			return m
		}
	}
}

const infinite = 1 << 20

func saturatingAdd(a, b int) int {
	if a >= infinite || b >= infinite {
		return infinite
	}
	return a + b
}

// spellings maps each terminal kind to the lexemes to emit for it.
func spellings(g *Grammar, withSpellings bool) map[string][]string {
	out := map[string][]string{}
	for _, name := range g.Terminals() {
		out[name] = []string{""}
	}
	if !withSpellings {
		return out
	}
	for _, p := range g.Prods {
		for _, s := range p.RHS {
			if !s.IsTerminal || s.Text == "" {
				continue
			}
			if !contains(out[s.Name], s.Text) {
				out[s.Name] = append(out[s.Name], s.Text)
			}
		}
	}
	return out
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// --- sentence encoding -----------------------------------------------------------------

// A sentence is carried as a string so that concatenating two of them is concatenating their
// encodings, which is what lets the composition search build with `+` instead of copying
// slices at every step. The separators are control bytes no token kind or lexeme contains.
const (
	fieldSep = "\x1f"
	tokenSep = "\x1e"
)

func encodeToken(name, text string) string { return name + fieldSep + text + tokenSep }

func decodeSentence(s string, kinds map[string]token.Kind) (Sentence, error) {
	if s == "" {
		return nil, nil
	}
	var out Sentence
	for _, part := range strings.Split(strings.TrimSuffix(s, tokenSep), tokenSep) {
		name, text, ok := strings.Cut(part, fieldSep)
		if !ok {
			return nil, fmt.Errorf("malformed encoded sentence %q", s)
		}
		k, ok := kinds[name]
		if !ok {
			return nil, fmt.Errorf("terminal %s is no lexer §0 token", name)
		}
		out = append(out, Token{Kind: k, Text: text})
	}
	return out, nil
}

// kindsByName inverts Kind.String(), which is the only bridge from a grammar terminal's name
// to the token it denotes.
func kindsByName() (map[string]token.Kind, error) {
	out := map[string]token.Kind{}
	for _, k := range token.All() {
		name := k.String()
		if _, dup := out[name]; dup {
			return nil, fmt.Errorf("two token kinds render as %s", name)
		}
		out[name] = k
	}
	return out, nil
}
