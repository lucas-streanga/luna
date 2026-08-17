// Package parser is the oracle's recursive-descent parser (compiler §1.3): tokens in, a
// lossless CST and P diagnostics out.
//
// Three stages, one per file — parse the trivia-filtered tokens grammar.md §0 is defined over
// into a flat event stream, splice the trivia back in by index, build the arena. Splitting them
// is a test seam before it is anything else, so a failure names one of the three rather than
// leaving one diff over a nested structure (§4).
//
// parser-implementation.md is authoritative for how the tree is shaped and why — kind-tagged
// rather than a struct per construct, one arena, trivia as ordinary nodes, immutable once built
// — each with the alternative it beat. Neither it nor testdata/golden.md is a CHANGES.md
// ruling: only language decisions get those.
//
// Names belonging to the golden format are Golden-scoped, ReadGolden rather than Read, so that
// a bare Parse and Node sit beside them without collision.
package parser

import (
	"luna/oracle/diagnostic"
	"luna/oracle/source"
	"luna/oracle/token"
)

// Parse builds the tree for one file and returns every diagnostic it raised.
//
// **It does not lex** (§4.4): the driver owns the phase pipeline, and a pass that lexes is one
// that has absorbed another phase — the batch driver could no longer abort at the lex boundary,
// nor the LSP driver reuse a token stream it already has. The *source.File is needed anyway,
// for grammar.md's spelling-matched terminals: IDENT("from") and its kin make the parser
// compare lexeme text.
//
// tokens is the **full** stream — the parser walks the filtered view, and splice needs the rest.
//
// Damaged input still gets a tree, error tolerance being a property of the pass and aborting a
// property of the driver (tooling §3), and so does a file whose lexing failed: an INVALID token
// is excess in §6's sense, since no production names it, and raises no P code of its own
// because the lexer already raised an L (§6.5).
//
// **The one input with no tree is the empty one** (§6.1), which is therefore the only nil.
func Parse(f *source.File, tokens []token.Token) (*Tree, []diagnostic.Diagnostic) {
	events, diags := parse(f, tokens)
	return build(f, tokens, splice(tokens, events)), diags
}

// parse is the recursive descent itself: a function per grammar.md §0 nonterminal over the
// trivia-filtered view of tokens. On damaged input it follows §7 — synthesise at an expect-site,
// where exactly one terminal is required and the call site already holds the answer; close to
// the nearest ancestor that accepts the token at a recursion site, where guessing would be
// reckless; and never skip past the closer of the innermost open bracket.
func parse(f *source.File, tokens []token.Token) (eventStream, []diagnostic.Diagnostic) {
	panic("parser: parse is unimplemented")
}
