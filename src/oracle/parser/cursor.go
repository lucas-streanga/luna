package parser

import "luna/oracle/token"

// The cursor over the trivia-filtered stream (§4.5). It is a full-stream index that never rests
// on trivia, so splice's "no trivia index" precondition holds by construction rather than by
// care, and a token event is the cursor's own index with no table to map it. Lookahead is a
// forward scan rather than a buffer; §4.7 bounds its depth at five.
//
// Terminals are token.Kind, not the tree's Kind, so a call site reads as §0 writes it. The one
// conversion that matters — expect's synthesised leaf — is a no-op by §5's alignment.
//
// **Three of these panic, and §7.8 is why**: no input may reach a panic, so each names a question
// no production asks and therefore a bug in the asking.

// atEnd is what stops an `Item*` loop on truncated input. Keyed only on a closing terminal it
// would spin, which is §6.4 in its simplest form.
func (p *parser) atEnd() bool {
	panic("parser: atEnd is unimplemented")
}

// at panics on token.Unset. Nothing in §0 names it, so asking is a bug and not a query — and the
// natural implementation would answer *true* at the end of input.
func (p *parser) at(k token.Kind) bool {
	panic("parser: at is unimplemented")
}

// nth returns token.Unset past the end. No production names Unset, so every production fails
// there in the ordinary way and no EOF kind has to exist; "unexpected end of input" is a §11.2
// diagnostic rather than a terminal. A negative offset panics: looking backwards is a bug.
func (p *parser) nth(n int) token.Kind {
	panic("parser: nth is unimplemented")
}

// lexeme serves grammar.md's spelling-matched terminals, and is the whole reason Parse is handed
// a *source.File it never lexes with (§4.4). It takes an offset because `COLON IDENT("type")
// ASSIGN` decides one token early (§4.7).
func (p *parser) lexeme(n int) string {
	panic("parser: lexeme is unimplemented")
}

func (p *parser) atWord(text string) bool {
	panic("parser: atWord is unimplemented")
}

// bump panics at the end of input. There is no token to consume there, so §7.2 layer 2's "consume
// one token into an Error" is unavailable and the only move is to unwind — a recovery loop wants
// atEnd in its condition. A bump that no-oped instead would turn a recovery bug into a **spin**,
// and a hang is a worse failure for a language server than a crash (§7.8).
func (p *parser) bump() {
	panic("parser: bump is unimplemented")
}

func (p *parser) eat(k token.Kind) bool {
	panic("parser: eat is unimplemented")
}

func (p *parser) eatWord(text string) bool {
	panic("parser: eatWord is unimplemented")
}

// expect is §7.2 layer 1, the only recovery needing no judgement: on a mismatch it synthesises a
// zero-width leaf of kind k (§6.1), reports Missing token subject to §7.6, and **consumes
// nothing**.
//
// Consuming nothing is what makes it safe to call anywhere and unsafe to loop on — a loop whose
// body reaches only an expect makes no progress (§6.4), which is why errorToken exists. It
// returns nothing so that call sites cannot invent their own repair; layers 2 to 4 live in one
// place.
//
// R267 allocates the P code when this body lands, not before.
func (p *parser) expect(k token.Kind) {
	panic("parser: expect is unimplemented")
}

// expectWord covers the one required spelling-matched terminal, `IDENT("from")`. The synthesised
// leaf spans no bytes, so the spelling is not recoverable from the tree and does not need to be.
func (p *parser) expectWord(text string) {
	panic("parser: expectWord is unimplemented")
}

// errorToken is the floor at a recursion site, where §7.2 layer 2 declines to guess. Layers 2 to
// 4 are Phase 3's; what is not deferred is **progress** — every error path consumes a real token,
// so no loop can spin on input it cannot parse (§6.4), and FuzzParse rests on it.
//
// A token nobody can place goes into an Error node and never past one: splice panics on a token
// that reached no event.
func (p *parser) errorToken() {
	panic("parser: errorToken is unimplemented")
}
