// The `DEFAULT` dispatch, which `INTERP_EXPR` shares (§1: "INTERP_EXPR lexes with the
// full DEFAULT rule set"). This is the whole language outside the delimited literals.
//
// §8 states the attempt order as a list of productions tried in turn. lexDefault
// dispatches on the leading byte instead, which is the same order realized: the
// productions that share a first byte are the ones §8's ordering constrains, and each
// such group is resolved in its own branch, in its own file, or by the longest-first
// table in tables.go. Where §8's order is load-bearing across groups — comments before
// division, `b"` before IDENT, the triples before their single-quote counterparts — the
// branch says so.
package lexer

import (
	"luna/oracle/diagnostic"
	"luna/oracle/token"
)

// lexDefault consumes one token's bytes and reports its kind. It never returns without
// advancing: every branch consumes at least one byte, which is what makes §11's totality
// claim structural rather than a property to be tested for (R242).
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
		// Before IDENT, or `b"…"` lexes as IDENT(b) followed by a string (§8.3, F6).
		return s.lexBytesFast()
	case c == '~':
		// `~` is a token in no other position (§8.4, R237), so this needs no ordering
		// against anything and consults no context.
		return s.lexRegexFast()
	case isDigit(c):
		return s.lexNumber()
	case isIdentStart(c):
		return s.lexWord()

	// Each triple before its single-quote counterpart (§8.2, R246), or `"""` lexes as an
	// empty string followed by a quote. The leading byte test guards the prefix scan, so
	// ordinary punctuation does not pay for it.
	case c == '"' && s.has(tripleDq):
		return s.lexTripleOpen(tripleDq, modeTripleDq, token.TripleDqOpen)
	case c == '\'' && s.has(tripleSq):
		return s.lexTripleOpen(tripleSq, modeTripleSq, token.TripleSqOpen)
	case c == '\'':
		return s.lexSqFast()
	case c == '"':
		return s.lexDqFast()
	case c == '`':
		return s.lexCommandFast()

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

	if k := s.lexOperator(); k != token.Unset {
		return k
	}

	// The catch-all (§0, §11). One byte, so a multi-byte rune yields one INVALID and one
	// L0012 per byte — the spec's wording, and the price of a rule stated over bytes
	// rather than runes.
	return s.invalid(diagnostic.UnexpectedCharacter, 1,
		"unexpected character %q", s.src[s.pos:s.pos+1])
}

// lexOperator matches the longest candidate sharing this first byte, returning Unset when
// none does. The table's within-group order is the maximal-munch chain, so first match
// wins is correct by construction.
func (s *Scanner) lexOperator() token.Kind {
	for _, e := range opsByByte[s.src[s.pos]] {
		if s.has(e.lit) {
			s.pos += len(e.lit)
			return e.kind
		}
	}
	return token.Unset
}

// lexBrace owns `{` and `}` because inside `INTERP_EXPR` their depth is what ends the
// splice (§6): the lexer counts them, and the `}` returning the count to zero is
// INTERP_CLOSE rather than RBRACE and pops back to the enclosing literal mode. In
// `DEFAULT` there is no depth to keep and both are ordinary punctuation.
func (s *Scanner) lexBrace() token.Kind {
	i := s.modeIndex()
	open := s.src[s.pos] == '{'
	s.pos++

	if s.modes[i].kind != modeInterp {
		if open {
			return token.LBrace
		}
		return token.RBrace
	}
	switch {
	case open:
		s.modes[i].depth++
		return token.LBrace
	case s.modes[i].depth == 0:
		s.pop()
		return token.InterpClose
	}
	s.modes[i].depth--
	return token.RBrace
}

// # Byte classes
//
// ASCII only, and matched as bytes: ingress has already proven the file is well-formed
// UTF-8 (lexical-structure §1), so a byte >= 0x80 can only be inside content these
// classes deliberately exclude.

// isSpace is any whitespace, newline included — §0's `[ \t\r\n]+` run. isHorizontalSpace
// in triple.go is the one that stops at a newline; the two are easy to confuse, so each
// says which it is.
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
