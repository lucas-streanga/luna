package parser

import (
	"testing"

	"luna/oracle/diagnostic"
	"luna/oracle/token"
)

// The cursor's specs. Every one is written against §4.5's invariant, pos being len(tokens) or
// an index that is never trivia, because that invariant is what the rest of the pass assumes and
// the only thing a cursor bug can break.

func TestCursorAtEndIsPosPastTheLastToken(t *testing.T) {
	expects(t, func(t *testing.T) {
		p := parserFor(t, "x;")
		if p.atEnd() {
			t.Error("atEnd on a file with tokens left")
		}
		p.pos = len(p.tokens)
		if !p.atEnd() {
			t.Error("not atEnd with pos past the last token")
		}
	})
}

// A file of nothing but trivia is at the end before it starts, which is what keeps
// `comment-only-file` from looping in File's own item loop.
func TestCursorAtEndOnATriviaOnlyFile(t *testing.T) {
	expects(t, func(t *testing.T) {
		if p := parserFor(t, "// just a comment\n"); !p.atEnd() {
			t.Errorf("a trivia-only file is not atEnd; pos is %d of %d", p.pos, len(p.tokens))
		}
	})
}

func TestCursorAtReadsTheCursorsKind(t *testing.T) {
	expects(t, func(t *testing.T) {
		p := parserFor(t, "let x;")
		if !p.at(token.KwLet) {
			t.Error("at(KW_LET) is false on `let x;`")
		}
		if p.at(token.Ident) {
			t.Error("at(IDENT) is true at the keyword")
		}
	})
}

// Every at is false at the end, which is what lets a production fail there in the ordinary way
// instead of needing an EOF terminal.
func TestCursorAtIsFalseAtTheEnd(t *testing.T) {
	expects(t, func(t *testing.T) {
		p := parserFor(t, "x")
		p.pos = len(p.tokens)
		for _, k := range []token.Kind{token.Ident, token.Semicolon, token.LBrace} {
			if p.at(k) {
				t.Errorf("at(%s) is true past the end", k)
			}
		}
	})
}

// nth counts in the **filtered** view: the whitespace between `x` and `;` is not a step.
func TestCursorNthSkipsTrivia(t *testing.T) {
	expects(t, func(t *testing.T) {
		p := parserFor(t, "x   /* c */  ;")
		if got := p.nth(0); got != token.Ident {
			t.Errorf("nth(0) is %s, want IDENT", got)
		}
		if got := p.nth(1); got != token.Semicolon {
			t.Errorf("nth(1) is %s, want SEMICOLON — nth counts non-trivia tokens", got)
		}
	})
}

func TestCursorNthIsUnsetPastTheEnd(t *testing.T) {
	expects(t, func(t *testing.T) {
		p := parserFor(t, "x;")
		if got := p.nth(2); got != token.Unset {
			t.Errorf("nth(2) is %s, want UNSET — no production names Unset, which is what makes "+
				"an EOF kind unnecessary", got)
		}
	})
}

func TestCursorLexemeReadsTheSpelling(t *testing.T) {
	expects(t, func(t *testing.T) {
		p := parserFor(t, "from m")
		if got := p.lexeme(0); got != "from" {
			t.Errorf("lexeme(0) is %q, want %q", got, "from")
		}
		if got := p.lexeme(1); got != "m" {
			t.Errorf("lexeme(1) is %q, want %q", got, "m")
		}
		if got := p.lexeme(2); got != "" {
			t.Errorf("lexeme past the end is %q, want empty", got)
		}
	})
}

// atWord is the whole spelling-match: the kind must be IDENT *and* the text must match, or `from`
// would be reserved rather than positional (R223).
func TestCursorAtWordMatchesKindAndText(t *testing.T) {
	expects(t, func(t *testing.T) {
		if p := parserFor(t, "from m"); !p.atWord("from") {
			t.Error("atWord(\"from\") is false on `from m`")
		}
		if p := parserFor(t, "unto m"); p.atWord("from") {
			t.Error("atWord(\"from\") matched a different identifier")
		}
		if p := parserFor(t, "let x"); p.atWord("let") {
			t.Error("atWord matched KW_LET; a keyword is not an IDENT, so no spelling-matched " +
				"terminal can ever be one")
		}
	})
}

// bump emits the cursor's own full-stream index (§2.2) and leaves pos on the next non-trivia
// token, which is what keeps splice's "no trivia index" precondition structural.
func TestCursorBumpEmitsAndSkipsTrailingTrivia(t *testing.T) {
	expects(t, func(t *testing.T) {
		p := parserFor(t, "x   ;")
		p.bump()
		if got, want := events(p.events), "token(0)"; got != want {
			t.Errorf("events are %q, want %q", got, want)
		}
		if want := indexOf(p.tokens, 1); p.pos != want {
			t.Errorf("pos is %d, want %d — bump advances past the trivia behind the token", p.pos, want)
		}
	})
}

func TestCursorEatConsumesOnlyOnAMatch(t *testing.T) {
	expects(t, func(t *testing.T) {
		p := parserFor(t, ", x")
		if !p.eat(token.Comma) {
			t.Error("eat(COMMA) reported no match on `,`")
		}
		if len(p.events) != 1 {
			t.Errorf("a matching eat emitted %d events, want 1", len(p.events))
		}
		before := p.pos
		if p.eat(token.Comma) {
			t.Error("eat(COMMA) matched an IDENT")
		}
		if p.pos != before || len(p.events) != 1 {
			t.Error("a failed eat consumed something; an optional terminal that is absent must " +
				"leave the cursor where it was")
		}
	})
}

func TestCursorEatWordConsumesOnlyOnAMatch(t *testing.T) {
	expects(t, func(t *testing.T) {
		p := parserFor(t, "get set")
		if !p.eatWord("get") {
			t.Error("eatWord(\"get\") reported no match")
		}
		if p.eatWord("get") {
			t.Error("eatWord(\"get\") matched `set`")
		}
		if !p.eatWord("set") {
			t.Error("eatWord(\"set\") reported no match after `get` was consumed")
		}
	})
}

// expect on a match is bump: no diagnostic, no synthesised leaf.
func TestCursorExpectConsumesOnAMatch(t *testing.T) {
	expects(t, func(t *testing.T) {
		p := parserFor(t, ";")
		p.expect(token.Semicolon)
		if got, want := events(p.events), "token(0)"; got != want {
			t.Errorf("events are %q, want %q", got, want)
		}
		if len(p.diags) != 0 {
			t.Errorf("a satisfied expect raised %d diagnostics", len(p.diags))
		}
	})
}

// §7.2 layer 1: synthesise a zero-width leaf of the expected kind, report, and **consume
// nothing**: the token stays for the caller's own dispatch to meet.
func TestCursorExpectSynthesisesAndDoesNotConsume(t *testing.T) {
	expects(t, func(t *testing.T) {
		p := parserFor(t, "}")
		p.expect(token.Semicolon)
		if got, want := events(p.events), "missing(SEMICOLON)"; got != want {
			t.Errorf("events are %q, want %q — a missing token is a leaf of the kind that was "+
				"expected (§6.1)", got, want)
		}
		if want := indexOf(p.tokens, 0); p.pos != want {
			t.Errorf("expect consumed a token: pos is %d, want %d. Consuming nothing is what makes "+
				"it safe to call anywhere, and is why errorToken carries the progress guarantee",
				p.pos, want)
		}
		if len(p.diags) != 1 {
			t.Fatalf("a failed expect raised %d diagnostics, want 1", len(p.diags))
		}
		if got := p.diags[0].Code; got != diagnostic.MissingToken {
			t.Errorf("raised %s, want %s — `expect` is the only site that raises it, which is why "+
				"R267 let it take the first P number", got, diagnostic.MissingToken)
		}
	})
}

func TestCursorExpectWordSynthesisesAnIdent(t *testing.T) {
	expects(t, func(t *testing.T) {
		p := parserFor(t, "std")
		p.expectWord("from")
		if got, want := events(p.events), "missing(IDENT)"; got != want {
			t.Errorf("events are %q, want %q — the synthesised leaf carries the kind, the spelling "+
				"being unrecoverable from a zero-width span and named by the diagnostic instead",
				got, want)
		}
	})
}

// errorToken is §6.2's positive-width Error and §6.4's progress guarantee in one: the token goes
// **into** the node, never past it, and the cursor always moves.
func TestCursorErrorTokenWrapsExactlyOneTokenAndAdvances(t *testing.T) {
	expects(t, func(t *testing.T) {
		p := parserFor(t, "@@@ x")
		p.errorToken()
		if got, want := events(p.events), "open(Error) token(0) close"; got != want {
			t.Errorf("events are %q, want %q", got, want)
		}
		if want := indexOf(p.tokens, 1); p.pos != want {
			t.Errorf("errorToken left pos at %d, want %d: it consumes exactly one token, and "+
				"every error path consuming a real token is what keeps a recovery loop from "+
				"spinning (§6.4)", p.pos, want)
		}
	})
}

// §7.8's three input-independent panics. Each names a question no production asks, so reaching one
// is a bug in the caller and never something a user's file can cause.

func TestCursorBumpPanicsAtTheEnd(t *testing.T) {
	expectsPanic(t, "parser:", func() {
		p := parserFor(t, "x")
		p.pos = len(p.tokens)
		p.bump()
	})
}

func TestCursorNthPanicsOnANegativeOffset(t *testing.T) {
	expectsPanic(t, "parser:", func() {
		parserFor(t, "x").nth(-1)
	})
}

func TestCursorAtPanicsOnUnset(t *testing.T) {
	expectsPanic(t, "parser:", func() {
		parserFor(t, "x").at(token.Unset)
	})
}
