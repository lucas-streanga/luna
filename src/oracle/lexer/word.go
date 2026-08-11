// Identifiers, keywords, and the wildcard (§3, §7).
package lexer

import (
	"strings"

	"luna/oracle/token"
)

// lexWord scans `[A-Za-z_][A-Za-z0-9_]*` and then decides what it was: the wildcard, one
// of the two compound keywords, an ordinary keyword, or an identifier. This is §3's
// recommended implementation — one scan and a lookup, plus the peeks the compounds need —
// rather than 49 patterns attempted in order.
func (s *Scanner) lexWord() token.Kind {
	start := s.pos
	s.pos++
	for s.pos < len(s.src) && isIdentPart(s.src[s.pos]) {
		s.pos++
	}
	word := s.src[start:s.pos]

	switch {
	case word == "_":
		// `_\b`: identifier-shaped, but the discard (§7). The scan above is what enforces
		// the boundary, so `_foo` never reaches here.
		return token.Wildcard
	case word == "match" && s.peek(0) == '!':
		s.pos++
		return token.KwMatchBang
	case word == "yield":
		// `\byield[ \t\r\n]+from\b`. The fold consumes the run between the words, so no
		// WHITESPACE trivia is emitted inside the compound (§3, R223).
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
