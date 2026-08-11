// `DEFAULT` mode, which `INTERP_EXPR` shares (§1: "INTERP_EXPR lexes with the full
// DEFAULT rule set"). This is the whole language outside the three interpolating
// literal forms.
//
// §8 states the attempt order as a list of productions tried in turn. lexDefault
// dispatches on the leading byte instead, which is the same order realized: the
// productions that share a first byte are the ones §8's ordering constrains, and each
// such group is resolved in its own branch or by the longest-first table in tables.go.
// Where §8's order is load-bearing across groups — comments before division, `b"`
// before IDENT, DOUBLE before the integer productions — the branch says so.
package lexer

import (
	"strings"

	"luna/oracle/diagnostic"
	"luna/oracle/token"
)

// lexDefault consumes one token's bytes and reports its kind. It never returns
// without advancing: every branch consumes at least one byte, which is what makes
// §11's totality claim structural rather than a property to be tested for (R242).
func (s *Scanner) lexDefault() token.Kind {
	c := s.src[s.pos]
	switch {
	case s.pos == 0 && s.has("#!"):
		// `\A#![^\n]*` — offset 0 only, and `INTERP_EXPR` can never be there (§2, R85).
		return s.lexLine(token.Shebang)
	case isSpace(c):
		return s.lexWhitespace()
	case c == '/' && (s.has("//") || s.has("/*")):
		// Comments before anything `/`-initial (§8.1). The *next* byte decides between
		// the four `/` forms; `/=` and `/` fall through to the operator table (§5).
		return s.lexComment()

	case c == 'b' && (s.peek(1) == '"' || s.peek(1) == '\''):
		// Before IDENT, or `b"…"` lexes as IDENT(b) followed by a string (§8.2, F6).
		return s.lexBytes()
	case c == '~':
		// `~` is a token in no other position (§8.3, R237), so this needs no ordering
		// against anything and consults no context.
		return s.lexRegex()
	case isDigit(c):
		return s.lexNumber()
	case isIdentStart(c):
		return s.lexWord()

	case c == '\'':
		return s.lexStringSq()
	case c == '"':
		return s.lexStringDq()
	case c == '`':
		return s.lexCommand()

	case c == '#':
		// `#` occurs only in `#[` and in a first-line `#!`, handled above (§5, R85).
		if s.has("#[") {
			s.pos += 2
			return token.AttrOpen
		}
		return s.invalid(diagnostic.UnexpectedHash, 1,
			"`#` opens an attribute only as `#[`, or a shebang as `#!` on the first line")
	case c == '{' || c == '}':
		return s.lexBrace()
	}

	if k, ok := s.lexOperator(); ok {
		return k
	}

	// The catch-all (§0, §11). One byte, so a multi-byte rune yields one INVALID and
	// one L0012 per byte — the spec's wording, and the price of a rule stated over
	// bytes rather than runes.
	return s.invalid(diagnostic.UnexpectedCharacter, 1,
		"unexpected character %q", s.src[s.pos:s.pos+1])
}

// # Trivia (§2)

// lexWhitespace consumes `[ \t\r\n]+` as one maximal run. Newlines fold into it: they
// are not significant, and the run's bytes stay recoverable from its span, so a
// per-newline token would double the trivia count for a fact no consumer reads (§9).
func (s *Scanner) lexWhitespace() token.Kind {
	for s.pos < len(s.src) && isSpace(s.src[s.pos]) {
		s.pos++
	}
	return token.Whitespace
}

// lexComment handles both forms. The caller has established that one of them starts
// here.
func (s *Scanner) lexComment() token.Kind {
	if s.has("//") {
		return s.lexLine(token.LineComment)
	}
	// `(?s)/\*.*?\*/` — the *first* `*/` closes it, block comments not nesting (F4),
	// which is exactly what the spec's lazy quantifier means.
	if i := strings.Index(s.src[s.pos+2:], "*/"); i >= 0 {
		s.pos += 2 + i + 2
		return token.BlockComment
	}
	// Unterminated: the caret goes on the `/*` that opened it, the token swallows the
	// rest of the file. Two different spans, deliberately (R242).
	s.error(diagnostic.UnterminatedBlockComment, s.pos, 2, "unterminated block comment")
	s.pos = len(s.src)
	return token.Invalid
}

// lexLine consumes through the byte before the next newline — the shape both `#!…`
// and `//…` have. The newline itself stays outside, to be picked up as WHITESPACE.
func (s *Scanner) lexLine(kind token.Kind) token.Kind {
	if i := strings.IndexByte(s.src[s.pos:], '\n'); i >= 0 {
		s.pos += i
	} else {
		s.pos = len(s.src)
	}
	return kind
}

// # Words (§3, §7)

// lexWord scans `[A-Za-z_][A-Za-z0-9_]*` and then decides what it was: the wildcard,
// one of the two compound keywords, an ordinary keyword, or an identifier. This is
// §3's recommended implementation — one scan and a lookup, plus the peeks the
// compounds need — rather than 49 patterns attempted in order.
func (s *Scanner) lexWord() token.Kind {
	start := s.pos
	s.pos++
	for s.pos < len(s.src) && isIdentPart(s.src[s.pos]) {
		s.pos++
	}
	word := s.src[start:s.pos]

	switch {
	case word == "_":
		// `_\b`: identifier-shaped, but the discard (§7). The scan above is what
		// enforces the boundary, so `_foo` never reaches here.
		return token.Wildcard
	case word == "match" && s.peek(0) == '!':
		s.pos++
		return token.KwMatchBang
	case word == "yield":
		// `\byield[ \t\r\n]+from\b`. The fold consumes the run between the words, so
		// no WHITESPACE trivia is emitted inside the compound (§3, R223).
		if end := s.foldYieldFrom(); end > 0 {
			s.pos = end
			return token.KwYieldFrom
		}
	}
	if k, ok := keywords[word]; ok {
		return k
	}
	return token.Ident
}

// foldYieldFrom reports where `yield from` ends, or 0 if this `yield` is a bare one.
// Whitespace only, by the regex in §0: a comment between the words defeats the fold,
// which is the same fact from the other side.
func (s *Scanner) foldYieldFrom() int {
	i := s.pos
	for i < len(s.src) && isSpace(s.src[i]) {
		i++
	}
	if i == s.pos || !strings.HasPrefix(s.src[i:], "from") {
		return 0
	}
	i += 4
	if i < len(s.src) && isIdentPart(s.src[i]) {
		return 0
	}
	return i
}

// # Numbers (§4, R238)

// lexNumber scans a numeric literal. §8.4's order — DOUBLE first, then the radix
// prefixes, then the leading-zero error production, then INT_DEC — appears here as
// decisions about the byte following a leading `0`, plus a digit-after-the-point test.
// That test is what keeps `1..5` INT RANGE INT and `1.toDouble()` INT DOT IDENT with
// no lookahead, which RE2 has none of to offer (F6).
func (s *Scanner) lexNumber() token.Kind {
	start := s.pos

	if s.src[start] == '0' {
		switch c := s.peek(1); c {
		case 'x', 'b', 'o':
			if k, ok := s.scanRadix(c); ok {
				return k
			}
			// No digit after the prefix — `0x`, `0b2`, and `0x_FF`, which Go permits
			// and §4 does not. §0 has no error production for that, so `0` lexes as
			// INT_DEC here and the letters as an IDENT on the next call.
		case 'X', 'B', 'O':
			return s.invalid(diagnostic.UppercaseRadixPrefix, 2,
				"`%s` — radix prefixes are lowercase: `0x`, `0b`, `0o`", s.src[start:start+2])
		}
	}

	// The integer part, `0|[1-9](?:_?[0-9])*`: a lone `0` is the whole of it, which is
	// what leaves `0755` to the error production below.
	if s.src[s.pos] == '0' {
		s.pos++
	} else {
		s.pos = scanDigitRun(s.src, s.pos, isDigit)
	}

	// DOUBLE, both rows of §0, attempted before either error production (§8.4).
	if s.peek(0) == '.' && isDigit(s.peek(1)) {
		s.pos = scanDigitRun(s.src, s.pos+1, isDigit)
		s.scanExponent()
		return token.Double
	}
	if s.scanExponent() {
		return token.Double
	}

	// `0[0-9_]+` (L0003). Reached only when the integer part was a lone `0`, since any
	// other integer part consumed these bytes already.
	if s.src[start] == '0' && isDigitOrSep(s.peek(0)) {
		end := s.pos
		for end < len(s.src) && isDigitOrSep(s.src[end]) {
			end++
		}
		s.error(diagnostic.LeadingZero, start, end-start,
			"`%s` is not decimal — write `0o…` for octal, or drop the zero", s.src[start:end])
		s.pos = end
		return token.Invalid
	}
	return token.IntDec
}

// scanRadix consumes `0x…`, `0b…`, or `0o…` with its digits. It reports false without
// moving when no valid digit follows the prefix, since §4 requires one immediately —
// a separator may not lead.
func (s *Scanner) scanRadix(prefix byte) (token.Kind, bool) {
	var kind token.Kind
	var digit func(byte) bool
	switch prefix {
	case 'x':
		kind, digit = token.IntHex, isHexDigit
	case 'b':
		kind, digit = token.IntBin, isBinDigit
	default:
		kind, digit = token.IntOct, isOctDigit
	}

	i := s.pos + 2
	if i >= len(s.src) || !digit(s.src[i]) {
		return token.Unset, false
	}
	s.pos = scanDigitRun(s.src, i, digit)
	return kind, true
}

// scanExponent consumes `[eE][+-]?[0-9]+`, and only if it is complete: `1.5e` is
// DOUBLE(`1.5`) followed by IDENT(`e`). Plain digits, no separators inside an
// exponent and no hex-float form (§4).
func (s *Scanner) scanExponent() bool {
	i := s.pos
	if c := s.peek(0); c != 'e' && c != 'E' {
		return false
	}
	i++
	if c := byteAt(s.src, i); c == '+' || c == '-' {
		i++
	}
	if !isDigit(byteAt(s.src, i)) {
		return false
	}
	for i < len(s.src) && isDigit(s.src[i]) {
		i++
	}
	s.pos = i
	return true
}

// scanDigitRun consumes `d(?:_?d)*` from i, where the caller has already checked that
// src[i] is a digit. One underscore, strictly between two digits, in any radix — so
// `1_000` runs and `1_`, `1__0`, `0x_FF` all stop where the separator rule does
// (R238).
func scanDigitRun(src string, i int, digit func(byte) bool) int {
	for i++; i < len(src); {
		j := i
		if src[j] == '_' {
			j++
		}
		if j >= len(src) || !digit(src[j]) {
			return i
		}
		i = j + 1
	}
	return i
}

// # Literals with delimiters (§4, §6)

// lexStringSq scans `'…'`. Single-quoted strings do not interpolate (string §5), so
// they are one span regex with no mode and a `$` inside is ordinary content.
func (s *Scanner) lexStringSq() token.Kind {
	end, _, ok := spanLiteral(s.src, s.pos+1, '\'', false, false)
	if !ok {
		return s.unterminated("string", 1, end)
	}
	s.pos = end
	return token.StringSq
}

// lexBytes scans `b"…"` or `b'…'`. Bytes literals do not interpolate either (bytes
// §7); the two rows of §0 differ only in their quote.
func (s *Scanner) lexBytes() token.Kind {
	end, _, ok := spanLiteral(s.src, s.pos+2, s.peek(1), false, false)
	if !ok {
		return s.unterminated("bytes", 2, end)
	}
	s.pos = end
	return token.Bytes
}

// lexStringDq scans `"…"`, taking §0's span pattern as the fast path F1 permits: it is
// valid only when the literal holds no `${`, and spanLiteral establishes that by
// stopping at whichever of the two comes first. A splice means the literal is not
// regular, so the delimited form is used instead and the mode stack takes over.
func (s *Scanner) lexStringDq() token.Kind {
	end, splice, ok := spanLiteral(s.src, s.pos+1, '"', true, false)
	switch {
	case !ok:
		return s.unterminated("string", 1, end)
	case splice:
		s.push(modeDQString)
		s.pos++
		return token.DqOpen
	}
	s.pos = end
	return token.StringDq
}

// lexRegex scans `~"…"` with its flags, or reports the bare `~` that opens nothing.
func (s *Scanner) lexRegex() token.Kind {
	if !s.has(`~"`) {
		return s.invalid(diagnostic.UnexpectedTilde, 1,
			"`~` opens a regex literal only as `~\"`")
	}
	end, splice, ok := spanLiteral(s.src, s.pos+2, '"', true, true)
	switch {
	case !ok:
		return s.unterminated("regex", 2, end)
	case splice:
		s.push(modeRegexBody)
		s.pos += 2
		return token.RegexOpen
	}
	// REGEX_CLOSE carries the flags (§0), so the whole-literal form covers them too.
	for end < len(s.src) && isRegexFlag(s.src[end]) {
		end++
	}
	s.pos = end
	return token.Regex
}

// lexCommand scans a backtick-delimited command literal, the third interpolating
// form (F3).
func (s *Scanner) lexCommand() token.Kind {
	end, splice, ok := spanLiteral(s.src, s.pos+1, '`', true, false)
	switch {
	case !ok:
		return s.unterminated("command", 1, end)
	case splice:
		s.push(modeCommand)
		s.pos++
		return token.CmdOpen
	}
	s.pos = end
	return token.Command
}

// spanLiteral walks a delimited literal's body from i, one byte past its opener, and
// stops at the first of four things: the closing delimiter, a `${` when the form
// interpolates, a raw newline unless the form may span lines, or end of input. It
// reports the offset just past the close, whether a splice stopped it, and whether the
// literal was closed at all.
//
// Stopping at whichever comes first is what makes the fast path exact rather than
// optimistic. If the closing delimiter arrives before any `${`, the literal provably
// holds no splice and §0's span pattern is the whole answer; if a `${` arrives first,
// that delimiter may well be inside the splice — F1's example, `"${x ?? "none"}"` —
// and no regex can settle it. Since R244 the whole question is line-local, so the
// lookahead this needs is bounded by a line rather than by the file.
//
// multiline is true only for `~"…"`, whose `x` flag exists to spell a pattern across
// lines (regex §4). Everywhere else a newline ends the literal (R244) and is left
// unconsumed, so it lexes as WHITESPACE and the next line lexes as code — which is the
// point: quotes pair greedily, so an unbounded literal reports its failure at the last
// unpaired quote in the file rather than at the typo.
//
// Escapes are one pair, in every delimited form — command literals included since R150
// (command §2.2, closing G5) — but a backslash-newline is not one of them in a
// newline-bounded form, so a trailing `\` cannot continue the literal. A trailing
// backslash at end of input consumes past the end and falls out unterminated, which is
// the correct reading of an unclosed literal.
func spanLiteral(src string, i int, close byte, interp, multiline bool) (end int, splice bool, ok bool) {
	for i < len(src) {
		switch {
		case !multiline && src[i] == '\n':
			return i, false, false
		case src[i] == '\\':
			if !multiline && byteAt(src, i+1) == '\n' {
				return i + 1, false, false
			}
			i += 2
		case src[i] == close:
			return i + 1, false, true
		case interp && src[i] == '$' && byteAt(src, i+1) == '{':
			return i, true, true
		default:
			i++
		}
	}
	return len(src), false, false
}

// unterminated reports L0009 and covers what the literal consumed as one INVALID —
// bytes to the newline that ended it, or to end of input for the file's last line and
// for a regex (R244).
//
// The two spans differ on purpose. The diagnostic's primary span is a caret position
// and belongs on the opening delimiter — that is what went unclosed — while the token
// records what the scanner consumed. Keeping those separate is what R242 bought, and
// it is why §2's tiling survives here.
func (s *Scanner) unterminated(what string, openerLen, end int) token.Kind {
	s.error(diagnostic.UnterminatedLiteral, s.pos, openerLen, "unterminated %s literal", what)
	s.pos = end
	return token.Invalid
}

// # Operators, punctuation, and braces (§5, §6)

// lexOperator matches the longest candidate sharing this first byte. The table's
// within-group order is the maximal-munch chain, so first match wins is correct by
// construction.
func (s *Scanner) lexOperator() (token.Kind, bool) {
	for _, e := range opsByByte[s.src[s.pos]] {
		if s.has(e.lit) {
			s.pos += len(e.lit)
			return e.kind, true
		}
	}
	return token.Unset, false
}

// lexBrace owns `{` and `}` because inside `INTERP_EXPR` their depth is what ends the
// splice (§6): the lexer counts them, and the `}` returning the count to zero is
// INTERP_CLOSE rather than RBRACE and pops back to the enclosing literal mode. In
// `DEFAULT` there is no depth to keep and both are ordinary punctuation.
func (s *Scanner) lexBrace() token.Kind {
	m := s.mode()
	open := s.src[s.pos] == '{'
	s.pos++

	if m.kind != modeInterpExpr {
		if open {
			return token.LBrace
		}
		return token.RBrace
	}
	switch {
	case open:
		m.depth++
		return token.LBrace
	case m.depth == 0:
		s.pop()
		return token.InterpClose
	}
	m.depth--
	return token.RBrace
}

// # Byte classes
//
// ASCII only, and matched as bytes: ingress has already proven the file is
// well-formed UTF-8 (lexical-structure §1), so a byte >= 0x80 can only be inside
// content these classes deliberately exclude.

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\r' || c == '\n' }

func isDigit(c byte) bool { return '0' <= c && c <= '9' }

func isDigitOrSep(c byte) bool { return isDigit(c) || c == '_' }

func isBinDigit(c byte) bool { return c == '0' || c == '1' }

func isOctDigit(c byte) bool { return '0' <= c && c <= '7' }

func isHexDigit(c byte) bool {
	return isDigit(c) || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F')
}

func isIdentStart(c byte) bool {
	return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || c == '_'
}

func isIdentPart(c byte) bool { return isIdentStart(c) || isDigit(c) }

// isRegexFlag covers exactly `i m s x b` (regex §3).
func isRegexFlag(c byte) bool {
	return c == 'i' || c == 'm' || c == 's' || c == 'x' || c == 'b'
}
