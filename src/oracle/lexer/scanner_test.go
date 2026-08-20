// Scanner's exported surface (lexer-testing-plan §8, and the only API in the package
// that Lex does not exercise).
//
// Lex drains a whole file, which is what the compiler wants. Scanner exists for the one
// consumer that must stop early: discovery (§1.0, R190) reads a file's import prelude and
// halts at the first non-import declaration, making that stage O(file head) rather than
// O(file). Everything below is about that difference; nothing else in the package would
// notice if the incremental path broke.
package lexer_test

import (
	"testing"

	"luna/oracle/diagnostic"
	"luna/oracle/lexer"
	"luna/oracle/source"
	"luna/oracle/token"
)

// drain runs a Scanner to exhaustion and returns what it produced, so the incremental
// path and Lex can be compared on the same input.
func drain(t *testing.T, src string) ([]token.Token, diagnostic.List) {
	t.Helper()
	s := lexer.New(newFile(t, src))

	var toks []token.Token
	for {
		tok, ok := s.Next()
		if !ok {
			return toks, s.Errors()
		}
		toks = append(toks, tok)
	}
}

func newFile(t *testing.T, src string) *source.File {
	t.Helper()
	f, err := source.New("scanner", src)
	if err != nil {
		t.Fatalf("%q is not valid source: %v", src, err)
	}
	return f
}

// TestScannerMatchesLex pins the two entry points to the same answer. Lex is written in
// terms of Scanner today, but that is an implementation choice and this is the assertion
// that would notice if it stopped being true.
func TestScannerMatchesLex(t *testing.T) {
	for _, src := range []string{
		"",
		"let x = 1;\n",
		"let s = \"a${ `ls ${b}` }c\";\n",
		"let s = \"\"\"\n    a\n    \"\"\";\n",
		"let a = \"oops;\nlet b = 0755;\n",
	} {
		wantToks, wantErrs := lexer.Lex(newFile(t, src))
		gotToks, gotErrs := drain(t, src)

		if len(gotToks) != len(wantToks) {
			t.Errorf("%q: Scanner produced %d tokens, Lex %d", src, len(gotToks), len(wantToks))
			continue
		}
		for i := range gotToks {
			if gotToks[i] != wantToks[i] {
				t.Errorf("%q: token %d is %v from Scanner, %v from Lex", src, i, gotToks[i], wantToks[i])
			}
		}
		if len(gotErrs) != len(wantErrs) {
			t.Errorf("%q: Scanner raised %d diagnostics, Lex %d", src, len(gotErrs), len(wantErrs))
		}
	}
}

// TestScannerStopsEarly is discovery's case, and the reason this API exists.
//
// A caller that halts at the first non-import declaration must not be charged for what
// follows it, not in time and not in diagnostics. The broken literal below is real, and
// Lex reports it; a scanner that stopped before reaching it must not, because it never
// read those bytes and has no business having an opinion about them.
func TestScannerStopsEarly(t *testing.T) {
	const src = "import { a } from m;\nfn main() { let s = \"oops\n}\n"

	s := lexer.New(newFile(t, src))
	read := 0
	for {
		tok, ok := s.Next()
		if !ok {
			t.Fatal("reached end of input without meeting the first declaration")
		}
		read++
		if tok.Kind == token.KwFn {
			break
		}
	}

	if !s.Errors().Empty() {
		t.Errorf("stopping early raised %v; nothing before `fn` is malformed", s.Errors())
	}
	if read >= 20 {
		t.Errorf("read %d tokens to reach `fn`; the prelude is shorter than that", read)
	}

	// The contrast that makes the point: the same input, read to the end, does report it.
	if _, errs := lexer.Lex(newFile(t, src)); len(errs) != 1 {
		t.Errorf("Lex raised %d diagnostics over the whole file, want 1", len(errs))
	}
}

// TestScannerErrorsAccumulate checks that Errors grows during a scan rather than only at
// the end: a caller that stops early reads it mid-flight, which is the only way it ever
// sees anything.
func TestScannerErrorsAccumulate(t *testing.T) {
	s := lexer.New(newFile(t, "let a = 0755; let b = 0X1F;\n"))

	seen := 0
	for {
		if _, ok := s.Next(); !ok {
			break
		}
		if n := len(s.Errors()); n < seen {
			t.Fatalf("diagnostic count fell from %d to %d mid-scan", seen, n)
		} else if n > seen {
			seen = n
		}
	}
	if seen != 2 {
		t.Errorf("accumulated %d diagnostics, want 2 (L0003 then L0004)", seen)
	}
}

// TestScannerErrorsIsACopy pins what the doc promises: a caller cannot reorder or extend
// the scanner's own list by holding what Errors handed back.
func TestScannerErrorsIsACopy(t *testing.T) {
	s := lexer.New(newFile(t, "let a = 0755;\n"))
	for {
		if _, ok := s.Next(); !ok {
			break
		}
	}

	got := s.Errors()
	if len(got) != 1 {
		t.Fatalf("got %d diagnostics, want 1", len(got))
	}
	// Writing an element is the test that means something: it reaches the scanner exactly
	// when the backing array is shared. Appending would not, copy or no; it acts on the
	// caller's own slice header.
	got[0] = nil

	after := s.Errors()
	if len(after) != 1 || after[0] == nil {
		t.Errorf("mutating the returned list reached the scanner: %v", after)
	}
}

// TestScannerNextAfterEnd pins the idempotence finish's guard exists for. A caller that
// keeps calling past the end, a loop written the other way round say, must not
// accumulate a fresh unterminated-literal diagnostic on every call.
func TestScannerNextAfterEnd(t *testing.T) {
	s := lexer.New(newFile(t, "let s = \"a${x\n"))
	for {
		if _, ok := s.Next(); !ok {
			break
		}
	}

	want := len(s.Errors())
	if want == 0 {
		t.Fatal("expected the unterminated literal and splice to be reported")
	}
	for i := range 3 {
		if tok, ok := s.Next(); ok {
			t.Fatalf("call %d past the end returned %v", i, tok)
		}
	}
	if got := len(s.Errors()); got != want {
		t.Errorf("diagnostics grew from %d to %d by calling Next past the end", want, got)
	}
}
