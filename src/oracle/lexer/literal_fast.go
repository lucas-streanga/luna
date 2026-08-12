// The span-regex fast path: a delimited literal that holds no `${` is one token (§0, F1).
//
// Every function here is reached from the DEFAULT dispatch and produces a whole literal.
// Their mode-path counterparts live in literal_mode.go and triple.go, and R247 settled
// what separates them: the fast path is taken *precisely* when there is no interior
// structure worth emitting, so its single INVALID for an unterminated literal is a
// deliberate shape rather than a fallback waiting to be written.
package lexer

import (
	"luna/oracle/diagnostic"
	"luna/oracle/escape"
	"luna/oracle/token"
)

// lexSqFast scans `'…'`. Single-quoted strings do not interpolate (string §5), so there
// is no mode for them at all and a `$` inside is ordinary content.
func (s *Scanner) lexSqFast() token.Kind {
	p := probeLiteral(s.src, s.pos+1, '\'', formSq)
	s.reportEscapes(p.bad)
	if !p.ok {
		return s.unterminated(formSq, 1, p.end)
	}
	s.pos = p.end
	return token.StringSq
}

// lexBytesFast scans `b"…"` or `b'…'`. Bytes literals do not interpolate either (bytes
// §7); §0's two rows differ only in their quote.
func (s *Scanner) lexBytesFast() token.Kind {
	p := probeLiteral(s.src, s.pos+2, s.peek(1), formBytes)
	s.reportEscapes(p.bad)
	if !p.ok {
		return s.unterminated(formBytes, 2, p.end)
	}
	s.pos = p.end
	return token.Bytes
}

// lexDqFast scans `"…"`, taking §0's span pattern as the fast path F1 permits: valid only
// when the literal holds no `${`, which probeLiteral establishes by stopping at whichever
// of the two comes first. A splice means the literal is not regular, so the delimited
// form is used instead and the mode stack takes over.
func (s *Scanner) lexDqFast() token.Kind {
	p := probeLiteral(s.src, s.pos+1, '"', formDq)
	if p.splice {
		// The mode path walks these bytes again and validates as it tokenizes, so the
		// probe's findings are dropped rather than raised twice (R248).
		s.push(mode{kind: modeDq, open: s.pos, openLen: 1})
		s.pos++
		return token.DqOpen
	}
	s.reportEscapes(p.bad)
	if !p.ok {
		return s.unterminated(formDq, 1, p.end)
	}
	s.pos = p.end
	return token.StringDq
}

// lexRegexFast scans `~"…"` with its flags, or reports the bare `~` that opens nothing.
func (s *Scanner) lexRegexFast() token.Kind {
	if !s.has(`~"`) {
		return s.invalid(diagnostic.UnexpectedTilde, 1,
			"`~` opens a regex literal only as `~\"`")
	}
	p := probeLiteral(s.src, s.pos+2, '"', formRegex)
	if p.splice {
		s.push(mode{kind: modeRegex, open: s.pos, openLen: 2})
		s.pos += 2
		return token.RegexOpen
	}
	if !p.ok {
		return s.unterminated(formRegex, 2, p.end)
	}
	// REGEX_CLOSE carries the flags (§0), so the whole-literal form covers them too.
	end := p.end
	for end < len(s.src) && isRegexFlag(s.src[end]) {
		end++
	}
	s.pos = end
	return token.Regex
}

// lexCommandFast scans a backtick-delimited command literal, the third interpolating
// form (F3).
func (s *Scanner) lexCommandFast() token.Kind {
	p := probeLiteral(s.src, s.pos+1, '`', formCommand)
	if p.splice {
		s.push(mode{kind: modeCommand, open: s.pos, openLen: 1})
		s.pos++
		return token.CmdOpen
	}
	s.reportEscapes(p.bad)
	if !p.ok {
		return s.unterminated(formCommand, 1, p.end)
	}
	s.pos = p.end
	return token.Command
}

// probe is what probeLiteral found.
type probe struct {
	end    int  // just past the closing delimiter, or where the scan gave up
	splice bool // a `${` arrived first, so the literal is not regular (F1)
	ok     bool // the literal closed
	bad    []badEscape
}

// badEscape is an illegal escape the scan passed over.
//
// They are *carried* rather than reported, because probeLiteral doubles as the lookahead
// that chooses between §6's two shapes. Reporting during the probe would diagnose an
// escape twice whenever the mode path then walks the same bytes, so the caller reports
// only once it has committed to the single-token reading (R248).
type badEscape struct {
	offset, length int
	code           diagnostic.Code
}

// probeLiteral walks a delimited literal's body from i, one byte past its opener, and
// stops at the first of four things: the closing delimiter, a `${` when the form splices,
// a raw newline unless the form spans lines, or end of input.
//
// Stopping at whichever comes first is what makes the fast path exact rather than
// optimistic. If the closing delimiter arrives before any `${`, the literal provably
// holds no splice and §0's span pattern is the whole answer; if a `${` arrives first,
// that delimiter may well be inside the splice — F1's example, `"${x ?? "none"}"` — and
// no regex can settle it. Since R244 the whole question is line-local, so the lookahead
// is bounded by a line rather than by the file.
//
// Escapes are one pair, in every delimited form — command literals included since R150
// (command §2.2, closing G5) — but a backslash-newline is not one of them in a
// newline-bounded form, so a trailing `\` cannot continue the literal.
func probeLiteral(src string, i int, close byte, form literalForm) probe {
	// Read once. The loop would otherwise re-derive them on every byte of the literal.
	r := form.rules()

	var p probe
	for i < len(src) {
		switch {
		case !r.spansLines && src[i] == '\n':
			p.end = i
			return p
		case src[i] == '\\':
			if !r.spansLines && byteAt(src, i+1) == '\n' {
				p.end = i + 1
				return p
			}
			if i+1 >= len(src) {
				p.end = len(src)
				return p
			}
			n, code := escape.Check(src, i, r.escapes)
			if code != "" {
				p.bad = append(p.bad, badEscape{offset: i, length: n, code: code})
			}
			i += n
		case src[i] == close:
			p.end, p.ok = i+1, true
			return p
		case r.splices && src[i] == '$' && byteAt(src, i+1) == '{':
			p.end, p.splice, p.ok = i, true, true
			return p
		default:
			i++
		}
	}
	p.end = len(src)
	return p
}

// reportEscapes raises what the probe carried. Called on commit, never on the splice
// path, where the literal modes validate the same bytes as they tokenize them.
func (s *Scanner) reportEscapes(bad []badEscape) {
	for _, b := range bad {
		s.error(b.code, b.offset, b.length, "%s",
			describeEscape(b.code, s.src[b.offset:b.offset+b.length]))
	}
}

// unterminated reports L0009 and covers what the literal consumed as one INVALID — bytes
// to the newline that ended it, or to end of input for the file's last line and for a
// regex (R244).
//
// The two spans differ on purpose. The diagnostic's primary span is a caret position and
// belongs on the opening delimiter — that is what went unclosed — while the token records
// what the scanner consumed. Keeping those separate is what R242 bought, and it is why
// §2's tiling survives here.
func (s *Scanner) unterminated(form literalForm, openerLen, end int) token.Kind {
	s.error(diagnostic.UnterminatedLiteral, s.pos, openerLen,
		"unterminated %s literal", form.noun())
	s.pos = end
	return token.Invalid
}
