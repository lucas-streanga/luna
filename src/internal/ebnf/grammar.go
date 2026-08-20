// Package ebnf reads grammar.md's productions and runs them.
//
// This is the **spec-literal parser** R253 commissions: an executable reading of
// specs/build/grammar.md, generated from the file rather than written beside it. It has no
// content of its own, which is the point: a hand-maintained second grammar would drift, and
// lexer-testing-plan §7 dropped the equivalent idea for the lexer precisely because a
// transcription there could only reach the RE2-expressible half. A parser's entire job is the
// CFG, so a transcription reaches all of it.
//
// What it answers, and nothing else: does this token stream derive from `File`, and how many
// ways. No CST, no recovery, no diagnostics, no semantic checks: everything grammar §9 lists
// as admitted-but-rejected parses here, deliberately.
package ebnf

import (
	"fmt"
	"sort"
	"strings"
)

// Sym is one symbol on the right-hand side of a production.
//
// A terminal names a lexer §0 token kind. A terminal with Text set additionally requires that
// lexeme, grammar.md's IDENT("from") form, the positional spelling-match Luna uses for `from`
// (R223), `get` / `set` (R232) and `identityEquality` (equality §4.4). It is not a keyword and
// does not reserve the word, so the match must be on the text and not on the kind.
//
// A **guard** (Negate) is grammar.md's `!TERMINAL`: a zero-width assertion that the next token
// is not that terminal (R270). It consumes nothing and yields nothing; it only forbids. The
// rules that need it share one shape, a leading token claimed by one reading: `{` opens a block
// wherever a block may appear (R268), and `: type` puts a binding's right-hand side in type
// position (R269). Restricting a nonterminal by its first token is an intersection with a
// regular set, so the grammar stays context-free and Earley stays exact; what it buys is that §0
// states the rule that was prose, so the parser's dispatch and the grammar's are one sentence.
type Sym struct {
	Name       string // token kind for a terminal, nonterminal name otherwise
	Text       string // required lexeme, terminals only; empty means any
	IsTerminal bool
	Negate     bool // a guard: this terminal may not appear here (terminals only)
}

func (s Sym) String() string {
	out := s.Name
	if s.Text != "" {
		out = fmt.Sprintf("%s(%q)", s.Name, s.Text)
	}
	if s.Negate {
		return "!" + out
	}
	return out
}

// carried is a right-hand side without its guards: the symbols that actually derive something.
// Anything counting a production's arity wants this, since a guard is an assertion rather than a
// child.
func carried(rhs []Sym) []Sym {
	for _, s := range rhs {
		if !s.Negate {
			continue
		}
		out := make([]Sym, 0, len(rhs)-1)
		for _, s := range rhs {
			if !s.Negate {
				out = append(out, s)
			}
		}
		return out
	}
	return rhs
}

// Prod is one alternative of one nonterminal: LHS ::= RHS.
//
// An empty RHS is the epsilon production, which the desugar introduces for `?` and `*` and
// which Earley handles directly, with no epsilon-elimination pass, since that would rewrite the
// grammar and cost the spec-literal property.
type Prod struct {
	LHS string
	RHS []Sym
}

// Grammar is the whole of §0, indexed for the recognizer.
type Grammar struct {
	Start string
	Prods []Prod
	byLHS map[string][]int // nonterminal -> indices into Prods
}

// New indexes a production list. The start symbol is fixed to File: grammar §10 asserts it is
// the only nonterminal appearing on no right-hand side, so there is nothing to infer.
func New(prods []Prod) *Grammar {
	g := &Grammar{Start: "File", Prods: prods, byLHS: map[string][]int{}}
	for i, p := range prods {
		g.byLHS[p.LHS] = append(g.byLHS[p.LHS], i)
	}
	return g
}

// Nonterminals returns every defined name, sorted.
func (g *Grammar) Nonterminals() []string {
	out := make([]string, 0, len(g.byLHS))
	for k := range g.byLHS {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Defines reports whether the name has at least one production.
func (g *Grammar) Defines(name string) bool { return len(g.byLHS[name]) > 0 }

// Undefined returns every nonterminal used on a right-hand side with no production of its own:
// a hole in the grammar, which grammar §10 makes a test failure rather than a review question.
func (g *Grammar) Undefined() []string {
	seen := map[string]bool{}
	for _, p := range g.Prods {
		for _, s := range p.RHS {
			if !s.IsTerminal && !g.Defines(s.Name) {
				seen[s.Name] = true
			}
		}
	}
	return sortedKeys(seen)
}

// Unreachable returns every nonterminal not reachable from the start symbol: dead grammar.
func (g *Grammar) Unreachable() []string {
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
	walk(g.Start)

	dead := map[string]bool{}
	for n := range g.byLHS {
		if !live[n] {
			dead[n] = true
		}
	}
	return sortedKeys(dead)
}

// Terminals returns every token kind named anywhere, sorted. The caller checks them against
// the real inventory: grammar §10 pins that no production may name a token that does not exist.
func (g *Grammar) Terminals() []string {
	seen := map[string]bool{}
	for _, p := range g.Prods {
		for _, s := range p.RHS {
			if s.IsTerminal {
				seen[s.Name] = true
			}
		}
	}
	return sortedKeys(seen)
}

// Alternatives returns the count of productions for a nonterminal, which is how the Keyword
// class is pinned against lexer §10's list.
func (g *Grammar) Alternatives(name string) int { return len(g.byLHS[name]) }

// PureAlternations returns every nonterminal whose productions are each a single symbol that is
// not synthetic: `AssignOp`, `Keyword`, `TopLevelItem`, `Literal` and their kin.
//
// Such a name is pure dispatch: it says which of several shapes follows, and the child it
// yields already says the same thing. Nothing else about the grammar distinguishes them, which
// is what makes this computable rather than a list somebody maintains: a new operator class
// added to grammar.md joins the set by being written, not by being remembered.
//
// **A synthetic symbol is excluded, and `Prelude ::= PreludeItem*` is why.** The desugar turns
// `*`, `?` and groups into `LHS·n` helpers, so a repetition is one symbol on the right yet
// stands for any number of children. Counting it as dispatch made this set claim a name that
// yields one `PreludeItem` per import. What the caller wants is "always passes through with one
// child", and a single synthetic is the one case where that does not follow from the arity.
//
// A guard is not a symbol here either: `FnBody ::= Block | !LBRACE Expr` still yields one child
// on both alternatives, and R268 must not cost `FnBody` its elision.
func (g *Grammar) PureAlternations() map[string]bool {
	out := map[string]bool{}
	for name, idxs := range g.byLHS {
		pure := true
		for _, i := range idxs {
			rhs := carried(g.Prods[i].RHS)
			if len(rhs) != 1 || strings.ContainsRune(rhs[0].Name, '·') {
				pure = false
				break
			}
		}
		if pure {
			out[name] = true
		}
	}
	return out
}

func (g *Grammar) String() string {
	var b strings.Builder
	for _, p := range g.Prods {
		b.WriteString(p.LHS)
		b.WriteString(" ::=")
		for _, s := range p.RHS {
			b.WriteByte(' ')
			b.WriteString(s.String())
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
