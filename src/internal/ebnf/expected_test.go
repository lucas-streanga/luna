package ebnf_test

import (
	"strings"
	"testing"

	"luna/internal/ebnf"
	"luna/oracle/token"
)

// The frontier: what the grammar says may come next at the point a parse stopped. It is the
// raw material of a syntax diagnostic, and the reason it is tested against hand-computed
// grammars is that "what may follow" is easy to get subtly wrong at exactly the places that
// matter: after a nullable, and at end of input.

func TestExpectedNamesWhatMayFollow(t *testing.T) {
	g := build(t, "File ::= LPAREN IDENT RPAREN")
	r := g.Recognize(toks(token.LParen, token.Plus))
	if r.Accepted {
		t.Fatal("should not derive")
	}
	if got, want := strings.Join(r.Expected, " "), "IDENT"; got != want {
		t.Errorf("expected set: got %q, want %q", got, want)
	}
}

// A nullable in the middle means two things may follow, and both must appear: `A B? C` admits
// either B or C at position one. Missing the second is the classic Earley nullable bug wearing
// a different hat.
func TestExpectedSeesPastANullable(t *testing.T) {
	g := build(t, "File ::= LPAREN RPAREN? COMMA")
	r := g.Recognize(toks(token.LParen, token.Plus))
	if got, want := strings.Join(r.Expected, " "), "COMMA RPAREN"; got != want {
		t.Errorf("expected set: got %q, want %q", got, want)
	}
}

// At end of input the frontier is what would have completed the file, which is the "unexpected
// end of input" diagnostic's content.
func TestExpectedAtEndOfInput(t *testing.T) {
	g := build(t, "File ::= LPAREN IDENT RPAREN")
	r := g.Recognize(toks(token.LParen, token.Ident))
	if got, want := strings.Join(r.Expected, " "), "RPAREN"; got != want {
		t.Errorf("expected set: got %q, want %q", got, want)
	}
	if !strings.Contains(r.Explain(toks(token.LParen, token.Ident)), "expected RPAREN") {
		t.Errorf("Explain should carry a small frontier: %s", r.Explain(toks(token.LParen, token.Ident)))
	}
}

// An accepted input has no frontier: computing one for every parse would cost the generator
// dearly and answer a question nobody asked.
func TestExpectedIsEmptyOnSuccess(t *testing.T) {
	g := build(t, "File ::= IDENT")
	if r := g.Recognize(toks(token.Ident)); len(r.Expected) != 0 {
		t.Errorf("an accepted parse should report no frontier, got %v", r.Expected)
	}
}

// A spelling-matched terminal names its lexeme in the frontier, because "expected IDENT" and
// "expected the word `from`" are different messages.
func TestExpectedCarriesRequiredSpellings(t *testing.T) {
	g := build(t, `File ::= IDENT IDENT("from")`)
	r := g.Recognize(toks(token.Ident))
	if got, want := strings.Join(r.Expected, " "), `IDENT("from")`; got != want {
		t.Errorf("expected set: got %q, want %q", got, want)
	}
}

// Over the real grammar the frontier is never empty at a real failure: a parser that reported
// "unexpected token" with nothing to suggest would have nothing to say.
func TestExpectedIsNeverEmptyOnRealFailure(t *testing.T) {
	g := load(t)
	r := g.Recognize([]ebnf.Token{{Kind: token.Plus, Text: "+"}})
	if r.Accepted {
		t.Fatal("a bare `+` is not a file")
	}
	if len(r.Expected) == 0 {
		t.Error("no frontier at a failure the grammar certainly has an opinion about")
	}
}
