// Package escape is string §5.1's table: which backslash escapes are legal in which
// literal, and whether a given one is well formed.
//
// It is vocabulary rather than a phase, and it sits here rather than inside lexer because
// two consumers need it and neither should hold a private copy (R248). The lexer asks
// whether an escape is *legal*; whatever later turns a literal's span into a string value
// will ask what it *means*, of the same rows. Splitting the table would be the drift this
// project spends its process avoiding.
//
// The check is three staged questions, in order, each reachable only by passing the one
// before — which is what keeps the codes disjoint (R248):
//
//   - Is the character in this context's row?      no → L0005
//   - Is its shape well formed?                    no → L0013 (\u{…}), L0016 (\xNN)
//   - Is its value a real scalar?                  no → L0006
//
// So `\x` in a double-quoted string is L0005, `x` being absent from that row entirely,
// while `\xZZ` in a bytes literal is L0016, where `x` is legal and only the digits are
// wrong.
package escape

import (
	"strings"

	"luna/oracle/diagnostic"
)

// Context is the literal form an escape appears in — the key string §5.1's table is
// written on, and the one thing a later phase would have to re-derive, which is why the
// lexer is what runs this (R248).
type Context uint8

const (
	StringDq Context = iota // "…", and """…""" since R246
	StringSq                // '…'
	Bytes                   // b"…" and b'…'
	Command                 // `…`
	Regex                   // ~"…"
)

// allowed is string §5.1's table, one row per context: the characters that may follow a
// backslash.
//
// **Regex has no row**, deliberately — an empty one would read as "nothing is allowed
// there", the opposite of the truth. Check answers it before consulting this table; the
// array is sized to hold the index anyway, so a future edit that drops that guard reads a
// zero value rather than running off the end.
//
// Bytes carries both quote escapes. §5.1's row is written for the `b"…"` form, but bytes
// §7 rules that the quote style does not matter, and a literal must be able to escape
// whichever delimiter closes it — so `b'don\'t'` is legal for the same reason `b"say \""`
// is. That is an inference from two rules rather than a third rule stated outright.
var allowed = [Regex + 1]string{
	StringDq: "ntr\\\"$u",
	StringSq: "'\\",
	Bytes:    "ntr\\\"'x",
	Command:  "`\\$",
}

// passthrough reports whether the context has no escape language of its own. Only the
// regex does: RE2's escapes are passed through undecoded and Luna decodes exactly one,
// `\"` (string §5.1), so nothing there can be unknown.
func (c Context) passthrough() bool { return c == Regex }

// Allowed is the characters that may follow a backslash in ctx, or the empty string where
// the context has no table — the regex, whose escapes pass through undecoded.
//
// Exported for the grammar generator, which turns the same rows into the escape rules the
// editor grammars carry. Reading them from here rather than restating them is the point:
// §5.1's table is what the lexer validates against, so a grammar built from it flags
// exactly the escapes the lexer would raise L0005 for, and cannot slide out of step with
// a ruling that changes a row.
func Allowed(ctx Context) string {
	if ctx.passthrough() || int(ctx) >= len(allowed) {
		return ""
	}
	return allowed[ctx]
}

// Check examines the escape beginning at the backslash src[i], reporting how many bytes
// it spans and, when it is not legal here, the code that condemns it.
//
// A byte must follow the backslash; the caller establishes that. A backslash at end of
// input begins no token at all, which is L0012's condition rather than this table's
// (R248).
func Check(src string, i int, ctx Context) (n int, code diagnostic.Code) {
	switch c := src[i+1]; {
	case ctx.passthrough():
		return 2, ""
	case strings.IndexByte(allowed[ctx], c) < 0:
		// Stage 1. The span is the pair alone, so the caret sits under `\q` rather than
		// under the literal that contains it.
		return 2, diagnostic.UnknownEscape
	case c == 'u':
		return codepoint(src, i)
	case c == 'x':
		return hexByte(src, i)
	}
	return 2, ""
}

// codepoint validates `\u{H…}`: one to six hex digits in braces.
//
// A malformed one reports over the `\u` alone — the span R245 arranged deliberately, by
// making the long alternative fail so `\\.` takes two bytes, precisely so a caret has
// somewhere to sit. A well-formed one naming no scalar reports over the whole escape,
// because there the digits are the problem and the reader needs to see them.
func codepoint(src string, i int) (int, diagnostic.Code) {
	if byteAt(src, i+2) != '{' {
		return 2, diagnostic.MalformedCodepointEscape
	}
	v, j := uint32(0), i+3
	for digits := 0; digits < 6 && isHex(byteAt(src, j)); digits++ {
		v = v<<4 | uint32(hexValue(src[j]))
		j++
	}
	if j == i+3 || byteAt(src, j) != '}' {
		return 2, diagnostic.MalformedCodepointEscape
	}

	n := j + 1 - i
	// Surrogates are not scalar values: they exist only to encode astral planes in UTF-16
	// and have no UTF-8 form, so admitting them would break the validity lexical-structure
	// §1 establishes at ingress — which is the whole reason \u{…} exists and \xNN is
	// bytes-only (string §5.1).
	if v > 0x10FFFF || (v >= 0xD800 && v <= 0xDFFF) {
		return n, diagnostic.InvalidCodepointEscape
	}
	return n, ""
}

// hexByte validates `\xNN`: exactly two hex digits (bytes §7). Two, not "up to two" —
// `\x8` is a typo rather than a shorthand, and a raw octet has a fixed width.
func hexByte(src string, i int) (int, diagnostic.Code) {
	if !isHex(byteAt(src, i+2)) || !isHex(byteAt(src, i+3)) {
		return 2, diagnostic.MalformedByteEscape
	}
	return 4, ""
}

func byteAt(src string, i int) byte {
	if i < 0 || i >= len(src) {
		return 0
	}
	return src[i]
}

func isHex(c byte) bool {
	return ('0' <= c && c <= '9') || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F')
}

func hexValue(c byte) byte {
	switch {
	case c <= '9':
		return c - '0'
	case c <= 'F':
		return c - 'A' + 10
	}
	return c - 'a' + 10
}
