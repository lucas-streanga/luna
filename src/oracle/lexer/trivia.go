// The trivia tokens (§2): whitespace, both comment forms, and the shebang.
//
// All are emitted rather than skipped (R236). They carry no meaning, so every consumer
// but the formatter drops them. The formatter cannot reproduce what the lexer discarded, and
// it is the only component that needs them.
package lexer

import (
	"strings"

	"luna/oracle/diagnostic"
	"luna/oracle/token"
)

// lexWhitespace consumes `[ \t\r\n]+` as one maximal run. Newlines fold into it: they are
// not significant, and the run's bytes stay recoverable from its span, so a per-newline
// token would double the trivia count for a fact no consumer reads (§9).
func (s *Scanner) lexWhitespace() token.Kind {
	for s.pos < len(s.src) && isSpace(s.src[s.pos]) {
		s.pos++
	}
	return token.Whitespace
}

// lexComment handles both forms. The caller has established that one of them starts here.
func (s *Scanner) lexComment() token.Kind {
	if s.has("//") {
		return s.lexLine(token.LineComment)
	}
	// `(?s)/\*.*?\*/`: the *first* `*/` closes it, block comments not nesting (F4),
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

// lexLine consumes through the byte before the next newline, the shape both `#!…` and
// `//…` have. The newline itself stays outside, to be picked up as WHITESPACE, which is
// what §0's two `[^\n]*`-tailed patterns mean and what §9 gives the reason for.
func (s *Scanner) lexLine(kind token.Kind) token.Kind {
	if i := strings.IndexByte(s.src[s.pos:], '\n'); i >= 0 {
		s.pos += i
	} else {
		s.pos = len(s.src)
	}
	return kind
}
