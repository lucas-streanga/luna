// The multi-line literal modes, `TRIPLE_DQ_STRING` and `TRIPLE_SQ_STRING` (R246).
//
// `"""` is `"…"` with more lines — the same escape table, the same interpolation — and
// the raw triple, spelled with three single quotes, has no escapes and no interpolation:
// every byte between the delimiters is its own.
// Both strip a **margin**, the closing delimiter's indentation, from each content line.
//
// The margin is emitted as a trivia token rather than removed, which is what makes
// stripping a *classification* instead of a transformation: nothing deletes anything, the
// decoder concatenates content and skips trivia, and §2's tiling needs no special case.
// The cooked-versus-raw difference then reduces to one bit — whether a line's trailing
// whitespace is trivia or content.
//
// Neither form has a single-token fast path. A triple always has margins to tokenize, so
// it always takes §6's delimited shape, which removes a decision rather than adding one.
package lexer

import (
	"strings"

	"luna/oracle/diagnostic"

	"luna/oracle/token"
)

const (
	tripleDq = `"""`
	tripleSq = `'''`
)

// lexTripleOpen scans an opener and enters its mode.
//
// The margin lives at the *end* of the literal, so the scanner must find the closing line
// before it can lex the body — one lookahead per literal, the same shape as F1's `${`
// probe. Finding no closer is not an error here: the mode is entered anyway and finish
// reports the unterminated literal at end of input, exactly as the single-line modes do.
func (s *Scanner) lexTripleOpen(delim string, kind modeKind, open token.Kind) token.Kind {
	start := s.pos
	i := s.pos + len(delim)

	// The opener owns the rest of its line (R246), which is what makes the first content
	// line unexceptional so the margin applies uniformly from line one.
	j := i
	for j < len(s.src) && isHorizontalSpace(s.src[j]) {
		j++
	}
	if j < len(s.src) && s.src[j] != '\n' {
		s.error(diagnostic.ContentAfterTripleOpen, j, 1,
			"a multi-line opener owns the rest of its line; move this to the next line")
	}
	if nl := strings.IndexByte(s.src[i:], '\n'); nl >= 0 {
		i += nl + 1
	} else {
		i = len(s.src)
	}

	s.push(mode{
		kind:    kind,
		open:    start,
		openLen: len(delim),
		margin:  findMargin(s.src, i, delim),
	})
	s.pos = i
	return open
}

// findMargin returns the indentation of the closing line: the first line at or after i
// whose leading whitespace is followed by the delimiter.
//
// That the margin comes from *punctuation* rather than from content is the whole of
// R246's rule. Java's minimum-indent alternative lets a content line set it, so editing
// one line silently changes the value of the others; the closer cannot do that, because
// nobody reorders it.
func findMargin(src string, i int, delim string) string {
	for i <= len(src) {
		j := i
		for j < len(src) && isHorizontalSpace(src[j]) {
			j++
		}
		if strings.HasPrefix(src[j:], delim) {
			return src[i:j]
		}
		nl := strings.IndexByte(src[i:], '\n')
		if nl < 0 {
			return ""
		}
		i += nl + 1
	}
	return ""
}

// lexTripleDqMode scans inside `"""`: escapes and interpolation as the single-line mode
// has them, plus
// the margin and the trailing-whitespace strip.
func (s *Scanner) lexTripleDqMode() token.Kind {
	if s.atLineStart() {
		if k := s.lexTripleLineStart(tripleDq, token.TripleDqClose); k != token.Unset {
			return k
		}
	}
	if s.src[s.pos] == '\n' {
		return s.lexLineBreak(tripleDq, token.TripleDqClose, token.DqText)
	}
	if s.src[s.pos] == '\\' {
		return s.lexEscape(formTripleDq)
	}
	if k := s.lexInterp(formTripleDq); k != token.Unset {
		return k
	}

	// A `"` is ordinary content here: the closer is recognized only at a line start after
	// the margin, so the run never competes with it (§0, R246).
	i := s.pos
	for i < len(s.src) && s.src[i] != '\\' && s.src[i] != '$' && s.src[i] != '\n' {
		i++
	}

	// Trailing whitespace is stripped in `"""` (R246), which here means classified as
	// trivia rather than removed by anyone. Back the run off it; if that empties the run,
	// this position *is* the whitespace.
	end := i
	if i >= len(s.src) || s.src[i] == '\n' {
		for end > s.pos && isHorizontalSpace(s.src[end-1]) {
			end--
		}
	}
	if end == s.pos {
		s.pos = i
		return token.Whitespace
	}
	s.pos = end
	return token.DqText
}

// lexTripleSqMode scans the raw triple, the simplest literal in the language: after
// the margin, a line is one RAW_TEXT run. No escapes, no interpolation, and trailing
// whitespace preserved — which is what makes this the form for content that is sensitive
// to it (R246).
func (s *Scanner) lexTripleSqMode() token.Kind {
	if s.atLineStart() {
		if k := s.lexTripleLineStart(tripleSq, token.TripleSqClose); k != token.Unset {
			return k
		}
	}
	if s.src[s.pos] == '\n' {
		return s.lexLineBreak(tripleSq, token.TripleSqClose, token.RawText)
	}
	i := s.pos
	for i < len(s.src) && s.src[i] != '\n' {
		i++
	}
	s.pos = i
	return token.RawText
}

// lexLineBreak decides what a newline inside a triple is: the head of the closing
// delimiter, or content.
//
// When the following line is the closer, the token spans the newline *and* the margin
// *and* the delimiter. That is how R246's rule — the last newline belongs to the
// delimiter rather than to the value — becomes structural, settled by one token's span
// rather than by a special case wherever the value is built.
func (s *Scanner) lexLineBreak(delim string, close, content token.Kind) token.Kind {
	if n := s.closerLen(s.pos+1, delim); n >= 0 {
		s.pos += 1 + n
		s.pop()
		return close
	}
	s.pos++
	return content
}

// closerLen reports how many bytes the closing delimiter occupies at i — the margin
// followed by the delimiter — or -1 when the line at i is not the closing line.
//
// Checked at a line start as well as after a newline, because the closer may be the very
// first line after the opener: that is the empty multi-line string, whose newline the
// opener has already consumed, so there is none left for the closing token to span.
func (s *Scanner) closerLen(i int, delim string) int {
	m := s.modes[s.modeIndex()]
	if !strings.HasPrefix(s.src[i:], m.margin) {
		return -1
	}
	if !strings.HasPrefix(s.src[i+len(m.margin):], delim) {
		return -1
	}
	return len(m.margin) + len(delim)
}

// lexTripleLineStart handles what may begin a content line: the closing delimiter, or the
// margin. It returns Unset when neither produced a token, so the caller falls through to
// the line's content.
func (s *Scanner) lexTripleLineStart(delim string, close token.Kind) token.Kind {
	if n := s.closerLen(s.pos, delim); n >= 0 {
		s.pos += n
		s.pop()
		return close
	}
	return s.lexMargin()
}

// atLineStart is exact rather than heuristic: the opener consumes its own newline and
// every content newline is a token of its own, so the byte before the scanner is a
// newline precisely when a content line begins here.
func (s *Scanner) atLineStart() bool { return s.pos > 0 && s.src[s.pos-1] == '\n' }

// lexMargin consumes a content line's margin, returning Unset when it emitted nothing so
// the caller falls through to the line's content.
//
// The check is a **byte prefix**, never a column count: comparing columns would require a
// tab width, and §9 (R236) refuses to pick one. So it is stricter than "further left" — a
// four-space margin rejects a line indented with a tab, whose position is unknowable
// without the width nobody has chosen — and that strictness is the point, since the
// alternative is a literal whose value depends on an editor setting.
func (s *Scanner) lexMargin() token.Kind {
	m := s.modes[s.modeIndex()]
	if m.margin != "" && strings.HasPrefix(s.src[s.pos:], m.margin) {
		s.pos += len(m.margin)
		return token.Margin
	}
	if m.margin == "" || blankLine(s.src, s.pos) {
		// A blank line is exempt (R246): demanding the margin on a line that has nothing
		// on it would fight every editor that strips trailing whitespace on save.
		return token.Unset
	}
	s.error(diagnostic.InsufficientIndentation, s.pos, 1,
		"this line is not indented by the closing delimiter's margin (%d bytes)", len(m.margin))
	return token.Unset
}

// blankLine reports whether the line at i holds nothing but whitespace.
func blankLine(src string, i int) bool {
	for ; i < len(src); i++ {
		if src[i] == '\n' {
			return true
		}
		if !isHorizontalSpace(src[i]) {
			return false
		}
	}
	return true
}

// isLineSpace is horizontal whitespace: the margin and the trailing strip are both about
// what sits beside content on a line, so a newline is never one of them.
func isHorizontalSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\r' }
