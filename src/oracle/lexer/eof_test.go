// The conditions a golden cannot express (lexer-testing-plan §2).
//
// A .lex file's input is everything before the `---` separator, trailing newline included,
// so a source that *ends without one* is unrepresentable — the same limitation txtar has,
// and FORMAT.md says such cases belong in a Go table test instead. This is that table.
//
// Two productions can only be reached that way. `lexLine`'s end-of-input branch, where a
// line comment or shebang runs off the end rather than meeting a newline; and, since R244
// bounded every single-byte-opener literal at its line, a literal that reaches end of
// input at all — which for those forms requires the file to stop mid-line.
//
// The fuzz suite covers both incidentally, on inputs nobody chose. These name them.
package lexer_test

import (
	"testing"

	"luna/oracle/diagnostic"
	"luna/oracle/lexer"
	"luna/oracle/token"
)

func TestEndOfInput(t *testing.T) {
	for _, c := range []struct {
		name  string
		src   string
		kinds []token.Kind
		codes []diagnostic.Code
	}{{
		name:  "line comment runs off the end",
		src:   "let x = 1; // no newline after this",
		kinds: []token.Kind{token.KwLet, token.Ident, token.Assign, token.IntDec, token.Semicolon, token.LineComment},
	}, {
		name:  "shebang is the whole file",
		src:   "#!/usr/bin/env luna",
		kinds: []token.Kind{token.Shebang},
	}, {
		// The R244 forms. Each stops at a newline normally, so end of input is reachable
		// only here, and each yields one INVALID over what it consumed with the caret on
		// its opener (R242, R247).
		name:  "double-quoted string",
		src:   `let a = "oops`,
		kinds: []token.Kind{token.KwLet, token.Ident, token.Assign, token.Invalid},
		codes: []diagnostic.Code{diagnostic.UnterminatedLiteral},
	}, {
		name:  "single-quoted string",
		src:   `let a = 'oops`,
		kinds: []token.Kind{token.KwLet, token.Ident, token.Assign, token.Invalid},
		codes: []diagnostic.Code{diagnostic.UnterminatedLiteral},
	}, {
		name:  "bytes literal",
		src:   `let a = b"oops`,
		kinds: []token.Kind{token.KwLet, token.Ident, token.Assign, token.Invalid},
		codes: []diagnostic.Code{diagnostic.UnterminatedLiteral},
	}, {
		name:  "command literal",
		src:   "let a = `oops",
		kinds: []token.Kind{token.KwLet, token.Ident, token.Assign, token.Invalid},
		codes: []diagnostic.Code{diagnostic.UnterminatedLiteral},
	}, {
		// A backslash at end of input pairs with nothing, so it begins no token: L0012's
		// condition, and §11 names a bare `\` as its own example (R248). The literal is
		// separately unterminated, reported by finish.
		name:  "backslash at end of input",
		src:   `let a = "ab\`,
		kinds: []token.Kind{token.KwLet, token.Ident, token.Assign, token.Invalid},
		codes: []diagnostic.Code{diagnostic.UnterminatedLiteral},
	}, {
		// The same backslash on the *mode* path, where lexEscape reaches it and condemns
		// the byte on its own. The fast path above raises only L0009, because it never
		// looks at an escape individually — the two paths differ, and R247 rules that
		// difference deliberate rather than an inconsistency to iron out.
		name: "backslash at end of input, mode path",
		src:  `let s = "a${x}b\`,
		kinds: []token.Kind{token.KwLet, token.Ident, token.Assign,
			token.DqOpen, token.DqText, token.InterpOpen, token.Ident, token.InterpClose,
			token.DqText, token.Invalid},
		codes: []diagnostic.Code{diagnostic.UnterminatedLiteral, diagnostic.UnexpectedCharacter},
	}, {
		// On the mode path, every byte is claimed by a real token, so there is no INVALID
		// at all — and finish reports one diagnostic per frame left open, outermost first.
		name: "splice open at end of input",
		src:  `let s = "a${x`,
		kinds: []token.Kind{token.KwLet, token.Ident, token.Assign,
			token.DqOpen, token.DqText, token.InterpOpen, token.Ident},
		codes: []diagnostic.Code{diagnostic.UnterminatedLiteral, diagnostic.UnterminatedInterpolation},
	}, {
		name:  "empty file",
		src:   "",
		kinds: nil,
	}} {
		t.Run(c.name, func(t *testing.T) {
			toks, errs := lexer.Lex(newFile(t, c.src))

			var got []token.Kind
			next := 0
			for _, tok := range toks {
				if tok.Offset != next {
					t.Fatalf("%s starts at %d, previous ended at %d", tok.Kind, tok.Offset, next)
				}
				next = tok.End()
				if !tok.IsTrivia() || tok.Kind != token.Whitespace {
					got = append(got, tok.Kind)
				}
			}
			if next != len(c.src) {
				t.Errorf("spans end at %d, input is %d bytes", next, len(c.src))
			}
			if !sameKinds(got, c.kinds) {
				t.Errorf("kinds %v, want %v", got, c.kinds)
			}

			var codes []diagnostic.Code
			for _, d := range errs.Sorted() {
				codes = append(codes, d.Code)
			}
			if !sameCodes(codes, c.codes) {
				t.Errorf("diagnostics %v, want %v", codes, c.codes)
			}
		})
	}
}

func sameKinds(a, b []token.Kind) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sameCodes(a, b []diagnostic.Code) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
