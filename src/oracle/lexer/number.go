// Numeric literals (§4, ruled in full by R238).
package lexer

import (
	"luna/oracle/diagnostic"
	"luna/oracle/token"
)

// lexNumber scans a numeric literal. §8.5's order (DOUBLE first, then the radix prefixes,
// then the leading-zero error production, then INT_DEC) appears here as
// decisions about the byte following a leading `0`, plus a digit-after-the-point test.
// That test is what keeps `1..5` INT RANGE INT and `1.toDouble()` INT DOT IDENT with no
// lookahead, which RE2 has none of to offer (F6).
func (s *Scanner) lexNumber() token.Kind {
	start := s.pos

	if s.src[start] == '0' {
		switch c := s.peek(1); c {
		case 'x', 'b', 'o':
			if k := s.scanRadix(c); k != token.Unset {
				return k
			}
			// No digit after the prefix: `0x`, `0b2`, and `0x_FF`, which Go permits and
			// §4 does not. §0 has no error production for that, so `0` lexes as INT_DEC
			// here and the letters as an IDENT on the next call.
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

	// DOUBLE, both rows of §0, attempted before either error production (§8.5).
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

// scanRadix consumes `0x…`, `0b…`, or `0o…` with its digits. It returns Unset without
// moving when no valid digit follows the prefix, since §4 requires one immediately; a
// separator may not lead.
func (s *Scanner) scanRadix(prefix byte) token.Kind {
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
		return token.Unset
	}
	s.pos = scanDigitRun(s.src, i, digit)
	return kind
}

// scanExponent consumes `[eE][+-]?[0-9]+`, and only if it is complete: `1.5e` is
// DOUBLE(`1.5`) followed by IDENT(`e`). Plain digits, no separators inside an exponent
// and no hex-float form (§4).
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
// src[i] is a digit. One underscore, strictly between two digits, in any radix, so
// `1_000` runs and `1_`, `1__0`, `0x_FF` all stop where the separator rule does (R238).
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
