package ebnf

import (
	"fmt"
	"strings"
)

// Parse reads grammar.md's metasyntax into productions.
//
// The notation is grammar.md's own, and small on purpose: `::=` defines, `|` alternates,
// `? * +` quantify, `( )` groups, UPPER_SNAKE is a token kind, UpperCamel is a nonterminal,
// IDENT("text") is a terminal that additionally matches a lexeme, and `!TERMINAL` is a guard,
// a zero-width assertion that the next token is not that terminal (R270).
//
// **The desugar to BNF is the one place this package can silently disagree with the spec.**
// Earley needs plain productions, so `A B?` has to become two alternatives and `A*` a fresh
// left-recursive pair. If that rewrite is wrong the tool tests a language grammar.md does not
// describe and every result afterwards is quietly worthless: the same fail-open shape
// lexer-testing-plan §1 guards for the spec reader. desugar_test.go therefore checks the
// rewrites against hand-computed languages before any corpus run is trusted.
//
// Synthetic nonterminals are named `LHS·n`; the interpunct cannot occur in a spec name, so a
// synthetic can never collide with or masquerade as one grammar.md defines.
func Parse(src string) ([]Prod, error) {
	rules, err := splitRules(src)
	if err != nil {
		return nil, err
	}
	var out []Prod
	for _, r := range rules {
		d := &desugarer{lhs: r.lhs}
		alts, err := d.parseAlts(newLexer(r.rhs))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", r.lhs, err)
		}
		for _, rhs := range alts {
			out = append(out, Prod{LHS: r.lhs, RHS: rhs})
		}
		out = append(out, d.extra...)
	}
	return out, nil
}

type rule struct{ lhs, rhs string }

// splitRules folds continuation lines into their rule. A rule starts at a line whose first
// token is a name followed by `::=`; everything up to the next such line belongs to it, which
// is what lets an alternative sit on its own `|` line as grammar.md writes them.
func splitRules(src string) ([]rule, error) {
	var rules []rule
	for _, line := range strings.Split(src, "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		if lhs, rhs, ok := splitDef(line); ok {
			rules = append(rules, rule{lhs: lhs, rhs: rhs})
			continue
		}
		if len(rules) == 0 {
			return nil, fmt.Errorf("continuation before any rule: %q", line)
		}
		rules[len(rules)-1].rhs += " " + strings.TrimSpace(line)
	}
	return rules, nil
}

func splitDef(line string) (lhs, rhs string, ok bool) {
	i := strings.Index(line, "::=")
	if i < 0 {
		return "", "", false
	}
	lhs = strings.TrimSpace(line[:i])
	if lhs == "" || strings.ContainsAny(lhs, " \t|()?*+") {
		return "", "", false
	}
	return lhs, strings.TrimSpace(line[i+3:]), true
}

// --- the metasyntax lexer -------------------------------------------------------------

type mlexer struct {
	src []rune
	pos int
}

func newLexer(s string) *mlexer { return &mlexer{src: []rune(s)} }

type mtok struct {
	kind rune // 'n' name, '|', '(', ')', '?', '*', '+', '!', 0 = end
	name string
	text string // the "text" of IDENT("text")
}

func (l *mlexer) next() (mtok, error) {
	for l.pos < len(l.src) && (l.src[l.pos] == ' ' || l.src[l.pos] == '\t') {
		l.pos++
	}
	if l.pos >= len(l.src) {
		return mtok{}, nil
	}
	c := l.src[l.pos]
	switch c {
	case '|', '(', ')', '?', '*', '+', '!':
		l.pos++
		return mtok{kind: c}, nil
	}
	if !isNameByte(c) {
		return mtok{}, fmt.Errorf("unexpected %q", string(c))
	}
	start := l.pos
	for l.pos < len(l.src) && isNameByte(l.src[l.pos]) {
		l.pos++
	}
	t := mtok{kind: 'n', name: string(l.src[start:l.pos])}

	// IDENT("text"): a terminal carrying a required lexeme.
	if l.pos < len(l.src) && l.src[l.pos] == '(' && l.pos+1 < len(l.src) && l.src[l.pos+1] == '"' {
		end := l.pos + 2
		for end < len(l.src) && l.src[end] != '"' {
			end++
		}
		if end >= len(l.src) || end+1 >= len(l.src) || l.src[end+1] != ')' {
			return mtok{}, fmt.Errorf("malformed %s(\"…\")", t.name)
		}
		t.text = string(l.src[l.pos+2 : end])
		l.pos = end + 2
	}
	return t, nil
}

func (l *mlexer) peek() (mtok, error) {
	save := l.pos
	t, err := l.next()
	l.pos = save
	return t, err
}

func isNameByte(c rune) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// --- the desugarer --------------------------------------------------------------------

type desugarer struct {
	lhs   string
	n     int
	extra []Prod
}

// fresh mints a synthetic nonterminal for a group or a repetition.
func (d *desugarer) fresh() string {
	d.n++
	return fmt.Sprintf("%s·%d", d.lhs, d.n)
}

// parseAlts reads `seq ( "|" seq )*` and returns one RHS per alternative.
func (d *desugarer) parseAlts(l *mlexer) ([][]Sym, error) {
	var alts [][]Sym
	for {
		seq, err := d.parseSeq(l)
		if err != nil {
			return nil, err
		}
		alts = append(alts, seq...)
		t, err := l.peek()
		if err != nil {
			return nil, err
		}
		if t.kind != '|' {
			return alts, nil
		}
		if _, err := l.next(); err != nil {
			return nil, err
		}
	}
}

// parseSeq reads one alternative. It returns a *set* of sequences because `?` multiplies the
// alternatives it appears in: `A B? C` is `A B C` and `A C`, and expanding here keeps the
// output free of synthetic names for the commonest quantifier.
func (d *desugarer) parseSeq(l *mlexer) ([][]Sym, error) {
	seqs := [][]Sym{{}}
	for {
		t, err := l.peek()
		if err != nil {
			return nil, err
		}
		if t.kind == 0 || t.kind == '|' || t.kind == ')' {
			return seqs, nil
		}
		if _, err := l.next(); err != nil {
			return nil, err
		}

		var atom []Sym // the atom as a symbol sequence, before its quantifier
		guard := false
		switch t.kind {
		case 'n':
			atom = []Sym{symbolFor(t)}
		case '!':
			// `!TERMINAL`: a zero-width guard (R270). It names a terminal because the
			// restriction it expresses is on the *next token*, which is what keeps it a regular
			// intersection and the grammar context-free; a guard over a nonterminal would be a
			// negation of a context-free language and is not admitted.
			n, err := l.next()
			if err != nil {
				return nil, err
			}
			if n.kind != 'n' {
				return nil, fmt.Errorf("! must be followed by a terminal")
			}
			s := symbolFor(n)
			if !s.IsTerminal {
				return nil, fmt.Errorf("!%s: a guard names a terminal, not a nonterminal", n.name)
			}
			s.Negate = true
			atom, guard = []Sym{s}, true
		case '(':
			alts, err := d.parseAlts(l)
			if err != nil {
				return nil, err
			}
			closing, err := l.next()
			if err != nil {
				return nil, err
			}
			if closing.kind != ')' {
				return nil, fmt.Errorf("unclosed (")
			}
			// A group with one alternative inlines; more than one needs a name to hold them.
			if len(alts) == 1 {
				atom = alts[0]
			} else {
				name := d.fresh()
				for _, a := range alts {
					d.extra = append(d.extra, Prod{LHS: name, RHS: a})
				}
				atom = []Sym{{Name: name}}
			}
		default:
			return nil, fmt.Errorf("unexpected %q", string(t.kind))
		}

		q, err := l.peek()
		if err != nil {
			return nil, err
		}
		if guard {
			// A guard matches nothing, so quantifying it says nothing, and a second one beside it
			// would mean two assertions at one position, where the generator carries one.
			// Both are refused rather than given a meaning nobody asked for.
			switch q.kind {
			case '?', '*', '+':
				return nil, fmt.Errorf("!%s takes no quantifier", atom[0].Name)
			case '!':
				return nil, fmt.Errorf("!%s: one guard to a position", atom[0].Name)
			}
		}
		switch q.kind {
		case '?':
			if _, err := l.next(); err != nil {
				return nil, err
			}
			// Multiply: every sequence so far branches into with-atom and without.
			var next [][]Sym
			for _, s := range seqs {
				next = append(next, append(append([]Sym{}, s...), atom...))
				next = append(next, append([]Sym{}, s...))
			}
			seqs = next
		case '*', '+':
			if _, err := l.next(); err != nil {
				return nil, err
			}
			// R ::= ε | R atom   (left recursion; Earley takes it unchanged, and it keeps
			// the repetition's tree left-leaning, which nothing here reads anyway)
			name := d.fresh()
			d.extra = append(d.extra,
				Prod{LHS: name, RHS: nil},
				Prod{LHS: name, RHS: append([]Sym{{Name: name}}, atom...)},
			)
			rep := []Sym{{Name: name}}
			if q.kind == '+' {
				rep = append(append([]Sym{}, atom...), rep...)
			}
			for i := range seqs {
				seqs[i] = append(seqs[i], rep...)
			}
		default:
			for i := range seqs {
				seqs[i] = append(seqs[i], atom...)
			}
		}
	}
}

// symbolFor classifies a name. UPPER_SNAKE is a token kind; anything else is a nonterminal.
// The split is by shape rather than by a lookup, so a typo'd terminal is reported by the
// inventory check as an unknown token rather than silently becoming an undefined nonterminal.
func symbolFor(t mtok) Sym {
	if isUpperSnake(t.name) {
		return Sym{Name: t.name, Text: t.text, IsTerminal: true}
	}
	return Sym{Name: t.name}
}

func isUpperSnake(s string) bool {
	for _, c := range s {
		if c != '_' && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return false
		}
	}
	return s != ""
}
