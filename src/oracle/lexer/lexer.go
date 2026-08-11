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
// # Layout
//
// One function per mode (§1), each consuming one token's bytes and reporting its kind.
// Next owns the span arithmetic and the progress check, so neither is repeated and
// neither can be got wrong in only one mode.
//
//	lexer.go        this file: the API, the scanner, the mode stack
//	core.go         the DEFAULT dispatch, which INTERP_EXPR shares; operators; braces
//	trivia.go       whitespace, comments, the shebang (§2)
//	word.go         identifiers, keywords, the wildcard (§3, §7)
//	number.go       numeric literals (§4)
//	form.go         literalForm: what each delimited literal admits
//	literal_fast.go the span-regex fast path — a literal with no `${` is one token
//	literal_mode.go the single-line literal modes, for one that has a `${`
//	triple.go       the multi-line literal modes (R246)
//	tables.go       the keyword and operator lookups
package lexer

import (
	"fmt"
	"slices"
	"strings"

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
func Lex(f *source.File) ([]token.Token, diagnostic.List) {
	s := New(f)
	// Real source runs a few bytes per token, trivia included. A starting point, not
	// a bound — append handles the file that disagrees.
	toks := make([]token.Token, 0, len(f.Text())/4+8)
	for {
		tok, ok := s.Next()
		if !ok {
			return toks, s.Errors()
		}
		toks = append(toks, tok)
	}
}

// Scanner produces tokens one at a time.
//
// Lex is the whole-file form and is what the compiler uses. This exists for the one
// consumer that must stop early: discovery (§1.0, R190) reads only a file's import
// prelude and halts at the first non-import declaration, making that stage O(file
// head) rather than O(file). Draining a whole file through Lex would defeat it.
type Scanner struct {
	f        *source.File
	src      string // f.Text(), hoisted: every step of the scan indexes it
	pos      int
	errors   diagnostic.List
	modes    []mode
	finished bool // finish has reported the frames left open at end of input
}

// New starts a scanner over a validated file. The file must already have passed
// ingress: the patterns below match ASCII bytes on the assumption that the rest is
// well-formed UTF-8, which lexical-structure §1 establishes exactly once.
func New(f *source.File) *Scanner {
	return &Scanner{f: f, src: f.Text(), modes: []mode{{kind: modeDefault, depth: 0}}}
}

// Next returns the next token, or ok=false at end of input.
//
// There is no EOF token, deliberately: §0's inventory is the lexemes the language has,
// and end-of-file is not one of them. Adding one to make loops tidier would put the code
// out of step with the table it is pinned against.
//
// A lexical error does not stop the scan. Next records the diagnostic, advances by at
// least one byte, and returns the next token it can find — so a caller that ignores
// Errors still terminates and still sees a stream that tiles the input.
func (s *Scanner) Next() (token.Token, bool) {
	if s.pos >= len(s.src) {
		s.finish()
		return token.Token{}, false
	}

	start := s.pos
	kind := s.lex()

	// Two invariants about what a mode may do to the position, checked in the one place
	// every mode passes through. Both are compiler bugs rather than conditions in the
	// program being compiled, so both panic.
	//
	// **Progress** is §11's rule — one token per step covering at least one byte — and is
	// the whole of what makes the scan terminate (R242).
	//
	// **Bounds** is the other half, and it is here because nothing else would notice.
	// Overrunning s.pos indexes nothing, so it raises no runtime error; the only symptom
	// would be a token claiming more bytes than the file has, surfacing as a tiling
	// failure at end of input rather than at the mode that caused it. Asserting here names
	// the culprit.
	switch {
	case s.pos <= start:
		panic(fmt.Sprintf("lexer: %s consumed no bytes at %d in %s", kind, start, s.f.Name()))
	case s.pos > len(s.src):
		panic(fmt.Sprintf("lexer: %s at %d ran to %d, past the end of %s (%d bytes)",
			kind, start, s.pos, s.f.Name(), len(s.src)))
	}
	return token.Token{Kind: kind, Offset: start, Len: s.pos - start}, true
}

// lex runs the current mode's scanner. It is separate from Next because a mode that ends
// without consuming anything — a raw newline closing a string (R244) — pops and calls
// this again, so the token comes from the mode underneath. That recursion is bounded by
// the stack depth, every step popping, and DEFAULT never pops.
func (s *Scanner) lex() token.Kind {
	switch k := s.modes[s.modeIndex()].kind; k {
	case modeDefault, modeInterp:
		return s.lexDefault()
	case modeDq:
		return s.lexDqMode()
	case modeRegex:
		return s.lexRegexMode()
	case modeCommand:
		return s.lexCommandMode()
	case modeTripleDq:
		return s.lexTripleDqMode()
	case modeTripleSq:
		return s.lexTripleSqMode()
	default:
		panic(fmt.Sprintf("lexer: unknown mode %d", k))
	}
}

// finish reports the literals and interpolations still open at end of input — one
// diagnostic per frame, outermost first, since source order is what a reader wants and
// each frame names a construct that really was left open.
//
// It runs once. Next may be called again after returning false, and a caller that does
// should not accumulate duplicates.
func (s *Scanner) finish() {
	if s.finished {
		return
	}
	s.finished = true
	for _, m := range s.modes[1:] {
		if m.kind == modeInterp {
			s.error(diagnostic.UnterminatedInterpolation, m.open, m.openLen,
				"unterminated interpolation")
			continue
		}
		s.error(diagnostic.UnterminatedLiteral, m.open, m.openLen,
			"unterminated %s literal", m.kind.noun())
	}
}

// Errors returns the diagnostics collected so far — a copy, so a caller cannot reorder or
// extend the scanner's own list. During a scan it grows; after the scan completes it is
// the file's full set.
//
// Note that a caller which stops early, as discovery does, never reaches the end of input
// and so is never told about a literal left open past where it stopped. That is the right
// answer for a consumer that chose not to read those bytes, and the reason finish runs
// only at end of input.
func (s *Scanner) Errors() diagnostic.List { return slices.Clone(s.errors) }

// mode is one frame of the lexer's mode stack (§1).
//
// Three literal forms admit ${expr} whose body is a full Luna expression, including
// nested literals of the same kind, so those literals are not regular and no single
// RE2 pattern can tokenize them. The stack is what makes them tractable: a frame
// records which literal is open and, inside an interpolation, how deep the braces are.
type mode struct {
	kind  modeKind
	depth int // brace depth inside modeInterpExpr; the } returning it to zero closes the splice

	// margin is the closing delimiter's indentation, in the triple modes (R246). Found by
	// a lookahead when the frame is pushed, because it lives at the *end* of the literal
	// and every content line before it needs to know what it is.
	margin string

	// Where the frame was opened. Kept because every diagnostic about an unclosed
	// construct wants its caret on the *opener* — that is what went unclosed — and by
	// the time the failure is known the scanner is somewhere else entirely, at a
	// newline or at end of input.
	open, openLen int
}

type modeKind uint8

const (
	modeDefault modeKind = iota
	modeInterp
	modeDq
	modeRegex
	modeCommand
	modeTripleDq
	modeTripleSq
)

// noun names the construct for an unterminated-literal message. §11 requires the
// description to say which kind of literal it was.
func (k modeKind) noun() string {
	switch k {
	case modeRegex:
		return "regex"
	case modeCommand:
		return "command"
	}
	return "string"
}

// modeIndex is the innermost frame's index.
//
// An index rather than a *mode: a pointer into the stack is invalidated by the next push,
// and a write through a stale one would land on a frame that is no longer innermost with
// nothing to say it had. Callers that only read may take a copy — mode is small and holds
// no pointers — but the mutable path goes through the slice.
func (s *Scanner) modeIndex() int { return len(s.modes) - 1 }

// push opens a frame. It takes the whole frame rather than its parts, so a field like the
// triples' margin is set where the frame is built instead of written into it afterwards.
func (s *Scanner) push(m mode) { s.modes = append(s.modes, m) }

func (s *Scanner) pop() { s.modes = s.modes[:len(s.modes)-1] }

// has reports whether the input at the current offset begins with lit.
func (s *Scanner) has(lit string) bool { return strings.HasPrefix(s.src[s.pos:], lit) }

// peek is the byte n past the current offset, or 0 past the end. The sentinel is
// unambiguous in practice: a real NUL satisfies no production's next-byte test, so it
// falls through to the same catch-all it would reach anyway.
func (s *Scanner) peek(n int) byte { return byteAt(s.src, s.pos+n) }

func byteAt(src string, i int) byte {
	if i < 0 || i >= len(src) {
		return 0
	}
	return src[i]
}

// error records a diagnostic. offset and length are the *caret* span, which is not
// always the span of the bytes consumed — see unterminated.
func (s *Scanner) error(code diagnostic.Code, offset, length int, format string, args ...any) {
	span := diagnostic.Span{Filename: s.f.Name(), Offset: offset, Length: length}
	s.errors.Add(diagnostic.New(code, span, format, args...))
}

// invalid consumes n bytes, records the diagnostic condemning them, and yields the
// INVALID token that covers them — R242's pairing, in one place so the two cannot
// drift apart. It is for the cases where the caret and the coverage coincide; where
// they do not, the caller writes both out.
func (s *Scanner) invalid(code diagnostic.Code, n int, format string, args ...any) token.Kind {
	s.error(code, s.pos, n, format, args...)
	s.pos += n
	return token.Invalid
}
