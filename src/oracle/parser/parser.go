// Package parser is the oracle's recursive-descent parser (compiler §1.3): tokens in, a
// lossless CST and P diagnostics out.
//
// Three stages, one per file: parse the trivia-filtered tokens grammar.md §0 is defined over
// into a flat event stream, splice the trivia back in by index, build the arena. Splitting them
// is a test seam before it is anything else, so a failure names one of the three rather than
// leaving one diff over a nested structure (§4).
//
// parser-implementation.md is authoritative for how the tree is shaped and why: kind-tagged
// rather than a struct per construct, one arena, trivia as ordinary nodes, immutable once
// built, each with the alternative it beat. Neither it nor testdata/golden.md is a CHANGES.md
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
// **It does not lex** (§4.4): a pass that lexes has absorbed another phase, and then the batch
// driver can no longer abort at the lex boundary nor the LSP driver reuse a stream it holds. The
// *source.File is still needed, for the spelling-matched terminals. tokens is the **full** stream,
// because splice needs the trivia the parser never sees.
//
// Damaged input still gets a tree, and so does a file whose lexing failed: an INVALID token is
// excess in §6's sense and raises no P code, the lexer having already raised an L (§6.5).
// **The empty file is the one input with no tree** (§6.1), and therefore the only nil.
func Parse(f *source.File, tokens []token.Token) (*Tree, []diagnostic.Diagnostic) {
	events, diags := parse(f, tokens)
	return build(f, tokens, splice(tokens, events)), diags
}

// parse is the recursive descent: a function per grammar.md §0 nonterminal, over the
// trivia-filtered view. Its shape is `newParser`, `p.file()`, return the two sinks.
//
// There is no error return and no early exit; error tolerance is a property of the pass (tooling
// §3). Every path therefore ends with a stream satisfying splice's preconditions: balanced with
// File opened and closed here, indices ascending and never trivia, and every non-trivia token
// consumed, one that could not be placed having gone **into** an Error node rather than past it.
func parse(f *source.File, tokens []token.Token) (eventStream, []diagnostic.Diagnostic) {
	panic("parser: parse is unimplemented")
}

// parser is one file's parse in progress. The tree is the builder's; stack is bookkeeping over
// the events rather than state of its own.
type parser struct {
	f      *source.File // for the spelling-matched terminals alone (§4.4)
	tokens []token.Token
	pos    int // §4.5: len(tokens), or an index that is never trivia
	events eventStream
	stack  []marker // open nodes, so complete and precede can check their arguments
	diags  []diagnostic.Diagnostic
}

// newParser does one thing beyond assignment: it puts the cursor on the first non-trivia token.
// Every later advance maintains pos's invariant and nothing else establishes it, so a file
// opening with a shebang or a licence comment would otherwise start the parse on trivia.
func newParser(f *source.File, tokens []token.Token) *parser {
	panic("parser: newParser is unimplemented")
}
