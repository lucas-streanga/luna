// Tests for bounded exhaustive generation.
//
// The generator is checked against hand-computed grammars first, and in both directions: one
// whose ambiguity is known and must be found, one whose unambiguity is known and must not be
// disturbed. A finder tested only on ambiguous input passes while finding everything, and a
// finder tested only on clean input passes while finding nothing.
package ebnf_test

import (
	"testing"

	"luna/internal/ebnf"
)

// enumerate runs a bound over a small grammar and fails on the invariant every run shares:
// nothing this generator emits may be rejected by the recognizer, since both read the same
// productions.
func enumerate(t *testing.T, g *ebnf.Grammar, b ebnf.Bound) *ebnf.Report {
	t.Helper()
	rep, err := g.Enumerate(b)
	if err != nil {
		t.Fatalf("enumerating: %v", err)
	}
	for _, s := range rep.Unrecognized {
		t.Errorf("generated but not recognized: %s", s)
	}
	return rep
}

// TestGenerateFindsClassicAmbiguity: `E ::= E + E | a` is the textbook ambiguous grammar, and
// its shortest ambiguous sentence is `a + a + a`. Finding it at length 5 and nothing shorter
// is the generator's basic competence.
func TestGenerateFindsClassicAmbiguity(t *testing.T) {
	g := build(t, "File ::= File PLUS File | IDENT")

	short := enumerate(t, g, ebnf.Bound{MaxLen: 3})
	if len(short.Ambiguous) != 0 {
		t.Errorf("nothing up to 3 tokens is ambiguous, got %v", short.Ambiguous)
	}
	if short.Sentences != 2 { // `a` and `a + a`
		t.Errorf("sentences up to 3 tokens: got %d, want 2", short.Sentences)
	}

	long := enumerate(t, g, ebnf.Bound{MaxLen: 5})
	if len(long.Ambiguous) != 1 {
		t.Fatalf("exactly one ambiguous sentence up to 5 tokens, got %v", long.Ambiguous)
	}
	if got, want := long.Ambiguous[0].String(), "IDENT PLUS IDENT PLUS IDENT"; got != want {
		t.Errorf("ambiguous sentence: got %q, want %q", got, want)
	}
}

// TestGenerateLeavesLayeredGrammarAlone: the same language, stratified the way grammar.md
// stratifies its thirteen tiers, is unambiguous — and left-recursive, which the enumeration's
// per-length fixpoint has to handle.
func TestGenerateLeavesLayeredGrammarAlone(t *testing.T) {
	g := build(t, `
File ::= File PLUS Term | Term
Term ::= IDENT
`)
	rep := enumerate(t, g, ebnf.Bound{MaxLen: 7})
	if len(rep.Ambiguous) != 0 {
		t.Errorf("a stratified grammar has no ambiguity, got %v", rep.Ambiguous)
	}
	if rep.Sentences != 4 { // a, a+a, a+a+a, a+a+a+a
		t.Errorf("sentences up to 7 tokens: got %d, want 4", rep.Sentences)
	}
	if !rep.Exhaustive() {
		t.Errorf("uncapped run should be exhaustive: %s", rep)
	}
}

// TestGenerateFindsNullableUnderOptional pins the one desugar hazard `?` carries. `A B? C`
// expands to two alternatives rather than a synthetic nullable nonterminal (parse.go), which
// is unambiguous — unless B is itself nullable, in which case the empty B is reachable two
// ways and the *EBNF* was already ambiguous. grammar.md has no such pair: every nonterminal it
// writes under `?` requires at least one token. This is the test that would notice one arriving.
func TestGenerateFindsNullableUnderOptional(t *testing.T) {
	g := build(t, `
File  ::= LPAREN Maybe? RPAREN
Maybe ::= COMMA?
`)
	rep := enumerate(t, g, ebnf.Bound{MaxLen: 3})
	if len(rep.Ambiguous) != 1 {
		t.Fatalf("`( )` derives two ways, got %v", rep.Ambiguous)
	}
	if got, want := rep.Ambiguous[0].String(), "LPAREN RPAREN"; got != want {
		t.Errorf("ambiguous sentence: got %q, want %q", got, want)
	}
}

// TestGenerateSpellingsKnob: the collision that needs a required lexeme in *two* positions is
// unreachable without the knob, because each production only ever emits its own — so neither
// generates the sentence that both accept. This is the case the knob exists for; a collision
// needing one spelling is found either way.
func TestGenerateSpellingsKnob(t *testing.T) {
	g := build(t, `
File ::= Lead | Trail
Lead  ::= IDENT("from") IDENT
Trail ::= IDENT IDENT("from")
`)
	blind := enumerate(t, g, ebnf.Bound{MaxLen: 2})
	if len(blind.Ambiguous) != 0 {
		t.Errorf("without Spellings the colliding sentence is never generated, got %v", blind.Ambiguous)
	}

	seeing := enumerate(t, g, ebnf.Bound{MaxLen: 2, Spellings: true})
	if len(seeing.Ambiguous) != 1 {
		t.Fatalf("both spellings at once matches both alternatives, got %v", seeing.Ambiguous)
	}
	if got, want := seeing.Ambiguous[0].String(), `IDENT("from") IDENT("from")`; got != want {
		t.Errorf("ambiguous sentence: got %q, want %q", got, want)
	}
}

// TestGenerateReportsTruncation: a capped run is not a proof, and must not read like one.
func TestGenerateReportsTruncation(t *testing.T) {
	g := build(t, "File ::= IDENT | INT_DEC | STRING_SQ")
	rep := enumerate(t, g, ebnf.Bound{MaxLen: 1, MaxPerCell: 2})
	if rep.Exhaustive() {
		t.Fatalf("a run that hit the cap claims exhaustiveness: %s", rep)
	}
	if rep.Sentences != 2 {
		t.Errorf("kept sentences: got %d, want 2", rep.Sentences)
	}
	if len(rep.Truncated) != 1 || rep.Truncated[0].Nonterminal != "File" {
		t.Errorf("truncation should name File, got %v", rep.Truncated)
	}
}

func TestGenerateRejectsUnknownStart(t *testing.T) {
	g := build(t, "File ::= IDENT")
	if _, err := g.Enumerate(ebnf.Bound{Start: "Nope", MaxLen: 1}); err == nil {
		t.Fatal("an undefined start symbol should be an error, not an empty report")
	}
}

// --- the real grammar ------------------------------------------------------------------

// TestGrammarUnambiguousAtGateDepth is the shallow end of the search, and it is deliberately
// the only part of it the gate pays for.
//
// The deep sweep is a **proof over a fixed grammar**, not a regression over changing code: its
// answer moves only when grammar.md does, so re-running it on every commit re-establishes what
// the last run established. It lives in `cmd/ambiguity -sweep`, behind `./check.sh --ambiguity`,
// with fuzzing and mutation. What stays here is three tokens — half a second under `-race` —
// which is enough to fail loudly if the grammar is broken outright, and enough to keep the
// generator wired to the real grammar so it cannot rot unnoticed.
//
// **The ceiling either way is structural, not a want of tuning.** Two facts set it. The thirteen
// expression tiers each store the whole expression language at every length, so the table
// carries the same strings a dozen times over. And the grammar is one connected component —
// `StringLit` admits interpolation, `Splice` holds an `Expr` — so even enumerating `Type` drags
// in every expression form there is: naming a sub-language narrows the *output* and almost never
// the *work*. The table is 227k strings at length 3 and 3.7M at length 4, sixteen times per
// token. So the 6-to-8-token corners grammar.md §11 flags — the parenthesized IIFE, `x->P.m`,
// `fn () => {}` against a variant literal — are past the ceiling of this instrument entirely,
// and are golden-file work (oracle/parser/testdata/golden.md).
func TestGrammarUnambiguousAtGateDepth(t *testing.T) {
	g := load(t)
	rep := enumerate(t, g, ebnf.Bound{Start: "File", MaxLen: 3})
	if !rep.Exhaustive() {
		t.Errorf("bound was not exhaustive, so a clean result proves nothing:\n%s", rep)
	}
	for _, s := range rep.Ambiguous {
		t.Errorf("derives more than one way: %s", s)
	}
	if rep.Sentences == 0 {
		t.Fatal("no sentences generated; this bound checks nothing")
	}
	t.Logf("%s", rep)
}
