// Package lexer turns validated source into a token stream (compiler §1.1).
//
// It is a complete phase, not a service the parser calls into: §1.1 tokenizes each
// file before parsing begins and with no symbol knowledge, which is what lets files
// lex in parallel and what forces every ambiguity to be resolved from lexer-local
// state alone. R237 is why that is affordable — replacing bare /…/ with ~"…" removed
// the one construct that needed to know what came before it.
//
// Errors are collected, never fatal (§1.1): the scanner reports a lexical condition,
// makes progress, and keeps going, so one bad byte yields one diagnostic rather than
// a cascade. The compile aborts at the phase boundary, which is the driver's decision
// and not this package's.
//
// # Lexing does not pipeline into parsing, deliberately
//
// The obvious optimization — let the parser pull tokens as they are produced — is
// ruled out by §3: "a phase cannot meaningfully consume the broken output of the
// previous one." The argument is diagnostic quality, not speed. Pipelined, a lexical
// error at byte 5000 arrives after the parser has already consumed everything before
// it, so the cascade of syntax errors provoked by the lexical failure is already
// emitted. Completing the phase first is what makes "one bad byte, one diagnostic"
// true rather than aspirational.
//
// The concurrency this appears to give up is already had one level up: §2 lexes and
// parses every file "independently and simultaneously", so file A parses while file B
// lexes. That is coarser, needs no coordination, and saturates cores on any real
// project. Intra-file pipelining would only shorten latency on a single enormous file,
// bought with the property above.
//
// Scanner therefore exists for early termination, not streaming — see its doc.
//
// NOTE: declarations only. Bodies are unimplemented — the tests come first.
package lexer

import (
	"luna/oracle/diagnostic"
	"luna/oracle/source"
	"luna/oracle/token"
)

// Lex scans a whole file, returning its tokens and every lexical error found.
//
// The error list is an output, not a failure: a file with lexical errors still yields
// a token stream, because §1.1 requires the scan to complete. Callers decide what to
// do with a non-empty list; the batch driver aborts at the phase boundary, the
// language server does not (compiler §3).
func Lex(f *source.File) ([]token.Token, diagnostic.List) { panic("unimplemented") }

// Scanner produces tokens one at a time.
//
// Lex is the whole-file form and is what the compiler uses. This exists for the one
// consumer that must stop early: discovery (§1.0, R190) reads only a file's import
// prelude and halts at the first non-import declaration, making that stage O(file
// head) rather than O(file). Draining a whole file through Lex would defeat it.
type Scanner struct {
	f      *source.File
	errors diagnostic.List
	modes  []mode
}

// New starts a scanner over a validated file. The file must already have passed
// ingress: the patterns below match ASCII bytes on the assumption that the rest is
// well-formed UTF-8, which lexical-structure §1 establishes exactly once.
func New(f *source.File) *Scanner {
	return &Scanner{f: f, modes: []mode{{kind: modeDefault, depth: 0}}}
}

// Next returns the next token, or ok=false at end of input.
//
// There is no EOF token, deliberately: §0's inventory is the 126 lexemes the language
// has, and end-of-file is not one of them. Adding a 127th to make loops tidier would
// put the code out of step with the table it is pinned against.
//
// A lexical error does not stop the scan. Next records the diagnostic, advances by at
// least one byte, and returns the next token it can find — so a caller that ignores
// Errors still terminates and still sees a stream that tiles the input.
func (s *Scanner) Next() (token.Token, bool) { panic("unimplemented") }

// Errors returns the diagnostics collected so far. During a scan it grows; after the
// scan completes it is the file's full set.
func (s *Scanner) Errors() diagnostic.List { return s.errors }

// mode is one frame of the lexer's mode stack (§1).
//
// Three literal forms admit ${expr} whose body is a full Luna expression, including
// nested literals of the same kind, so those literals are not regular and no single
// RE2 pattern can tokenize them. The stack is what makes them tractable: a frame
// records which literal is open and, inside an interpolation, how deep the braces are.
type mode struct {
	kind  modeKind
	depth int // brace depth inside interpModeExpr; the } returning it to zero closes the splice
}

type modeKind uint8

const (
	modeDefault modeKind = iota
	modeDQString
	modeRegexBody
	modeCommand
	modeInterpExpr
)
