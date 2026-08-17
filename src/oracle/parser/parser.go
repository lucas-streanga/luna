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
//
// Its whole shape is three lines — construct, walk File, hand back the two sinks:
//
//	p := newParser(f, tokens)
//	p.file()
//	return p.events, p.diags
//
// There is no error return and no early exit. Error tolerance is a property of the pass (tooling
// §3), so every path ends with a stream that satisfies splice's preconditions: balanced, with
// File opened and closed here; ascending; never a trivia index; and every non-trivia token
// consumed, one that could not be placed having gone **into** an Error node rather than past it.
func parse(f *source.File, tokens []token.Token) (eventStream, []diagnostic.Diagnostic) {
	panic("parser: parse is unimplemented")
}

// parser is one file's parse in progress: a cursor, an event sink, and a diagnostic sink. Nothing
// else — the tree is the builder's, and the stack below is bookkeeping over the events rather than
// state of its own.
type parser struct {
	// f is carried for the spelling-matched terminals alone (§4.4): grammar.md's IDENT("from")
	// and its four siblings compare lexeme text, so the parser needs Slice and nothing more.
	f *source.File

	// tokens is the **full** stream, trivia included, because splice needs the rest of it. The
	// parser reaches only the filtered view of it, which cursor.go states as a rule about pos.
	tokens []token.Token

	// pos is the cursor: an index into tokens that is either len(tokens) or a non-trivia index,
	// never anything else. That invariant is what makes "the parser never sees trivia"
	// structural, and splice panics if it is ever broken.
	pos int

	// events is the output §4 chose: flat, ordered, and comparable against a hand-written
	// sequence with no tree in the way.
	events eventStream

	// stack is the markers of the nodes currently open, so complete can assert that the node
	// being closed is the one on top and precede can assert its own precondition.
	stack []marker

	// diags accumulates rather than aborting: the batch driver discards the tree at the phase
	// boundary and the LSP driver consumes it (tooling §3), and neither wants the parser
	// deciding which of them it is talking to.
	diags []diagnostic.Diagnostic
}

// newParser seeds a parse. The one thing it does beyond assignment is put the cursor on the first
// non-trivia token, since every later advance maintains that and nothing else establishes it — a
// file that opens with a shebang or a licence comment would otherwise start the parse on trivia.
func newParser(f *source.File, tokens []token.Token) *parser {
	panic("parser: newParser is unimplemented")
}
