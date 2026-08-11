// The literal modes: `DQ_STRING`, `REGEX_BODY`, `COMMAND` (§1, §6).
//
// These run when a literal actually interpolates. lexDefault's span-regex fast path
// handles the rest, and F1/F3 are why the split exists: `"${x ?? "none"}"` closes the
// span regex at the wrong quote, and nested delimiters plus nested braces need counting,
// which no RE2 pattern can do.
//
// §6 fixes the attempt order and the `$` chain is the load-bearing part: DOLLAR_TEXT's
// pattern would match at every `$` if tried first, and is correct only because both
// interpolation forms are attempted before it. lexInterp keeps all three in one place so
// the order cannot be got wrong in one mode and right in another.
package lexer

import (
	"fmt"
	"strings"

	"luna/oracle/diagnostic"
	"luna/oracle/escape"
	"luna/oracle/token"
)

// lexDQString scans inside `"…"`. It is the only mode with `$name` interpolation
// (string §5) and the only one where `\u{…}` is a legal escape (R245).
func (s *Scanner) lexDQString() token.Kind {
	switch c := s.src[s.pos]; c {
	case '"':
		s.pos++
		s.pop()
		return token.DqClose
	case '\n':
		return s.unterminatedMode("string")
	case '\\':
		return s.lexEscape(escape.StringDq, false)
	}
	if k, ok := s.lexInterp(true); ok {
		return k
	}
	return s.textRun(token.DqText, "\"\\$\n")
}

// lexRegexBody scans inside `~"…"`. It is the one mode a newline does not end (R244),
// the `x` flag existing to spell a pattern across lines, and its closer carries the
// flags (§0).
func (s *Scanner) lexRegexBody() token.Kind {
	switch c := s.src[s.pos]; c {
	case '"':
		s.pos++
		for s.pos < len(s.src) && isRegexFlag(s.src[s.pos]) {
			s.pos++
		}
		s.pop()
		return token.RegexClose
	case '\\':
		return s.lexEscape(escape.Regex, true)
	}
	if k, ok := s.lexInterp(false); ok {
		return k
	}
	return s.textRun(token.RegexText, "\"\\$")
}

// lexCommandBody scans inside a backtick literal. Escapes are R150's — the earlier
// no-escapes reading is retired (command §2.2) — and `$name` is not an interpolation
// here, so a `$` not followed by `{` is DOLLAR_TEXT.
func (s *Scanner) lexCommandBody() token.Kind {
	switch c := s.src[s.pos]; c {
	case '`':
		s.pos++
		s.pop()
		return token.CmdClose
	case '\n':
		return s.unterminatedMode("command")
	case '\\':
		return s.lexEscape(escape.Command, false)
	}
	if k, ok := s.lexInterp(false); ok {
		return k
	}
	return s.textRun(token.CmdText, "`\\$\n")
}

// lexInterp attempts §6's `$` chain in order: `${`, then `$name` where the mode has it,
// then a lone `$` as content. It reports false when the byte is not a `$` at all, so
// the caller falls through to its text run.
func (s *Scanner) lexInterp(ident bool) (token.Kind, bool) {
	if s.src[s.pos] != '$' {
		return token.Unset, false
	}
	switch {
	case s.peek(1) == '{':
		s.push(modeInterpExpr, s.pos, 2)
		s.pos += 2
		return token.InterpOpen, true
	case ident && isIdentStart(s.peek(1)):
		// Longest identifier wins (string §5): `"$abc"` splices `abc`, not `a`.
		s.pos++
		for s.pos < len(s.src) && isIdentPart(s.src[s.pos]) {
			s.pos++
		}
		return token.InterpIdent, true
	}
	s.pos++
	return token.DollarText, true
}

// lexEscape consumes one ESCAPE_PAIR and validates it against string §5.1 (R248).
//
// The token is emitted either way: an illegal escape is still a backslash pair, its bytes
// are claimed, and §2's tiling does not care whether they were legal — the diagnostic is
// the parallel channel R243 separated. So the span comes from escape.Check, which is what
// makes a whole `\u{1F600}` one token and a malformed `\u{` two bytes (R245).
//
// multiline cannot be derived from ctx, though it looks as if it could: `"""` shares
// `"…"`'s escape table (R246) while spanning lines, so the two facts came apart the
// moment the multi-line forms landed. Passing it keeps the table keyed on the literal
// *form* while line-spanning stays a property of the *mode*.
func (s *Scanner) lexEscape(ctx escape.Context, multiline bool) token.Kind {
	if !multiline && s.peek(1) == '\n' {
		// A trailing backslash cannot continue the literal (R244), so the pair does not
		// match and nothing else claims this byte. Consuming it as the INVALID that ends
		// the literal is what the span-regex path does too, and it keeps the newline
		// outside, for the enclosing mode to lex as WHITESPACE.
		m := s.mode()
		s.error(diagnostic.UnterminatedLiteral, m.open, m.openLen,
			"unterminated %s literal", m.kind.what())
		s.pos++
		s.pop()
		return token.Invalid
	}
	if s.pos+1 >= len(s.src) {
		// A backslash at end of input pairs with nothing, so it begins no token — which
		// is L0012's condition, and §11 names a bare `\` as its own example (R248 keeps
		// it there rather than calling it an unknown escape: there is no character to be
		// absent from a table row). finish reports the unterminated literal separately;
		// this byte is condemned on its own because nothing claims it.
		return s.invalid(diagnostic.UnexpectedCharacter, 1,
			"`\\` at end of input begins no escape")
	}

	n, code := escape.Check(s.src, s.pos, ctx)
	if code != "" {
		s.error(code, s.pos, n, "%s", describeEscape(code, s.src[s.pos:s.pos+n]))
	}
	s.pos += n
	return token.EscapePair
}

// describeEscape is the per-instance half of an escape diagnostic. The title is fixed to
// the code (R240), so this names the lexeme and what it wanted instead.
func describeEscape(code diagnostic.Code, lexeme string) string {
	switch code {
	case diagnostic.UnknownEscape:
		return fmt.Sprintf("`%s` is not an escape in this literal", lexeme)
	case diagnostic.MalformedCodepointEscape:
		return "`\\u` needs `{`, one to six hex digits, then `}`"
	case diagnostic.MalformedByteEscape:
		return "`\\x` needs exactly two hex digits"
	case diagnostic.InvalidCodepointEscape:
		return fmt.Sprintf("`%s` names no Unicode scalar value", lexeme)
	}
	return fmt.Sprintf("`%s` is not a valid escape", lexeme)
}

// textRun consumes the mode's text run: bytes up to the next one that could begin
// another token here. The caller has established the byte at pos is not one of them, so
// the run covers at least one byte.
func (s *Scanner) textRun(kind token.Kind, stop string) token.Kind {
	for s.pos < len(s.src) && strings.IndexByte(stop, s.src[s.pos]) < 0 {
		s.pos++
	}
	return kind
}

// unterminatedMode ends a literal that a raw newline closed (R244).
//
// The newline is not consumed: it is ordinary WHITESPACE in the mode underneath, which
// is what lets the next line lex as code rather than as more literal. So this consumes
// nothing itself and hands off to the mode it uncovered — and emits no INVALID, because
// every byte of the literal was already claimed by a token (R243). The caret goes on the
// opener, which is what went unclosed.
func (s *Scanner) unterminatedMode(what string) token.Kind {
	m := s.mode()
	s.error(diagnostic.UnterminatedLiteral, m.open, m.openLen, "unterminated %s literal", what)
	s.pop()
	return s.lex()
}
