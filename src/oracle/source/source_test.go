// Tests for the ingress and position layer (lexer-testing-plan §8, step 1).
//
// Written before the implementation. What is asserted here is the contract
// lexical-structure §1 and lexer §9 state, not a description of code that exists.
//
// One property is deliberately not tested: that the line index is built *lazily*.
// §9 states laziness as a performance property ("a compile that emits no diagnostic builds no
// table"), and asserting it means either white-box inspection or allocation
// counting, both of which pin an implementation strategy rather than a behaviour. If
// it regresses, nothing observable changes.
package source_test

import (
	"errors"
	"strings"
	"testing"

	"luna/oracle/diagnostic"
	"luna/oracle/source"
)

func mustNew(t *testing.T, src string) *source.File {
	t.Helper()
	f, err := source.New("test.luna", src)
	if err != nil {
		t.Fatalf("New(%q) failed: %v", src, err)
	}
	return f
}

func codeOf(t *testing.T, err error) diagnostic.Code {
	t.Helper()
	var e *source.Error
	if !errors.As(err, &e) {
		t.Fatalf("error %v is not a *source.Error", err)
	}
	return e.Code
}

// --- UTF-8 validation (L0001) -----------------------------------------------------

func TestAcceptsValidUTF8(t *testing.T) {
	// Non-ASCII is legal only inside literal and comment content (lexical-structure
	// §1), which is where these put it, but validation is byte-level and does not
	// know that, so the distinction is the lexer's, not this package's.
	cases := map[string][]byte{
		"empty":                []byte(""),
		"ascii":                []byte("let x = 1;\n"),
		"two-byte in comment":  []byte("// café\n"),
		"three-byte in string": []byte("let s = 'こんにちは';\n"),
		"four-byte in comment": []byte("// 🎉\n"),
		// U+FEFF is an ordinary codepoint anywhere but offset 0
		// (lexical-structure §1), so a BOM inside a comment is not an error here.
		"U+FEFF not at offset 0": append([]byte("// "), bom...),
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := source.New("t.luna", string(src)); err != nil {
				t.Errorf("rejected valid UTF-8: %v", err)
			}
		})
	}
}

// bom is U+FEFF encoded. Written as bytes because Go's own compiler rejects a
// literal byte-order mark in source, the same rule Luna adopts, for the same
// reason.
var bom = []byte{0xEF, 0xBB, 0xBF}

func TestRejectsInvalidUTF8(t *testing.T) {
	// Each case carries the offset of the first bad byte: the diagnostic must point
	// at where validity broke, not at the start of the file.
	cases := []struct {
		name       string
		src        []byte
		wantOffset int
	}{
		{"lone continuation byte", []byte{'a', 0x80}, 1},
		{"truncated three-byte sequence", []byte{'a', 0xE2, 0x82}, 1},
		{"truncated two-byte sequence at EOF", []byte{0xC3}, 0},
		{"overlong NUL", []byte{0xC0, 0x80}, 0},
		{"UTF-8-encoded surrogate", []byte{0xED, 0xA0, 0x80}, 0},
		{"above U+10FFFF", []byte{0xF5, 0x80, 0x80, 0x80}, 0},
		{"invalid byte 0xFF", []byte{'/', '/', ' ', 0xFF}, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := source.New("t.luna", string(tc.src))
			if err == nil {
				t.Fatal("accepted invalid UTF-8")
			}
			if got := codeOf(t, err); got != diagnostic.InvalidUTF8 {
				t.Errorf("code = %s, want %s", got, diagnostic.InvalidUTF8)
			}
			var e *source.Error
			errors.As(err, &e)
			if e.Offset != tc.wantOffset {
				t.Errorf("offset = %d, want %d", e.Offset, tc.wantOffset)
			}
		})
	}
}

// --- Byte-order mark (L0002) ------------------------------------------------------

func TestRejectsLeadingBOM(t *testing.T) {
	// Refused rather than stripped, so that byte offset 0 keeps meaning what the
	// shebang rule needs it to mean (lexical-structure §1).
	for _, tc := range []struct {
		name string
		src  []byte
	}{
		{"bom then code", append(append([]byte{}, bom...), []byte("let x = 1;\n")...)},
		{"bom alone", bom},
		{"bom then shebang", append(append([]byte{}, bom...), []byte("#!/usr/bin/env luna\n")...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := source.New("t.luna", string(tc.src))
			if err == nil {
				t.Fatal("accepted a leading BOM")
			}
			if got := codeOf(t, err); got != diagnostic.ByteOrderMark {
				t.Errorf("code = %s, want %s", got, diagnostic.ByteOrderMark)
			}
			var e *source.Error
			errors.As(err, &e)
			if e.Offset != 0 {
				t.Errorf("offset = %d, want 0", e.Offset)
			}
		})
	}
}

func TestBOMIsReportedBeforeLaterInvalidUTF8(t *testing.T) {
	// A file can be both. Reporting the earlier problem is what makes the offset in
	// the diagnostic the first thing a reader should look at.
	src := append(append([]byte{}, bom...), 0xFF)
	_, err := source.New("t.luna", string(src))
	if err == nil {
		t.Fatal("accepted a BOM followed by invalid UTF-8")
	}
	if got := codeOf(t, err); got != diagnostic.ByteOrderMark {
		t.Errorf("code = %s, want %s (the earlier of the two)", got, diagnostic.ByteOrderMark)
	}
}

// --- The pure-ASCII flag ----------------------------------------------------------

func TestASCIIFlag(t *testing.T) {
	cases := map[string]struct {
		src  string
		want bool
	}{
		"empty":                 {"", true},
		"ascii only":            {"let x = 1;\n", true},
		"tab and crlf":          {"let\tx = 1;\r\n", true},
		"del 0x7f is ascii":     {"// \x7f\n", true},
		"multi-byte in string":  {"let s = 'é';\n", false},
		"multi-byte in comment": {"// é\n", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := mustNew(t, tc.src).IsASCII(); got != tc.want {
				t.Errorf("IsASCII() = %v, want %v", got, tc.want)
			}
		})
	}
}

// --- Positions --------------------------------------------------------------------

func TestPosition(t *testing.T) {
	cases := []struct {
		name      string
		src       string
		offset    int
		line, col int
	}{
		{"empty file at 0", "", 0, 1, 1},
		{"first byte", "abc", 0, 1, 1},
		{"mid first line", "abc", 2, 1, 3},
		{"end of file, no trailing newline", "abc", 3, 1, 4},
		{"the newline itself belongs to its line", "ab\ncd", 2, 1, 3},
		{"first byte of second line", "ab\ncd", 3, 2, 1},
		{"mid second line", "ab\ncd", 4, 2, 2},
		{"after a trailing newline is an empty last line", "ab\n", 3, 2, 1},
		{"blank line between", "a\n\nb", 2, 2, 1},
		{"third line", "a\nb\nc", 4, 3, 1},
		// A tab is one column (R236). Rendering a caret under it is a renderer
		// concern; the reported location does not depend on a tab-width setting.
		{"tab counts as one column", "\tx", 1, 1, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mustNew(t, tc.src).Position(tc.offset)
			if got.Line != tc.line || got.Column != tc.col {
				t.Errorf("Position(%d) = %d:%d, want %d:%d",
					tc.offset, got.Line, got.Column, tc.line, tc.col)
			}
		})
	}
}

func TestPositionCRLF(t *testing.T) {
	// §9: "Line starts are the offsets just after each \n, with \r left as ordinary
	// content on the preceding line, so CRLF needs no special case." The visible
	// consequence is that a column at end-of-line counts the \r, which is what every
	// other toolchain does.
	f := mustNew(t, "ab\r\ncd")
	for _, tc := range []struct {
		offset    int
		line, col int
		what      string
	}{
		{2, 1, 3, "the \\r is column 3 of line 1"},
		{3, 1, 4, "the \\n is column 4 of line 1"},
		{4, 2, 1, "line 2 starts just after the \\n"},
	} {
		if got := f.Position(tc.offset); got.Line != tc.line || got.Column != tc.col {
			t.Errorf("Position(%d) = %d:%d, want %d:%d (%s)",
				tc.offset, got.Line, got.Column, tc.line, tc.col, tc.what)
		}
	}
}

func TestPositionCountsRunesNotBytes(t *testing.T) {
	// The column is a rune count over the line's prefix (§9). "é" is two bytes, so a
	// byte-counting implementation reports one column too many, the exact bug this
	// case exists to catch.
	f := mustNew(t, "// é x\n")
	const offsetOfX = 6 // '/', '/', ' ', 0xC3, 0xA9, ' ', 'x'
	if got := f.Text()[offsetOfX]; got != 'x' {
		t.Fatalf("test is miscalibrated: byte %d is %q, not 'x'", offsetOfX, got)
	}
	got := f.Position(offsetOfX)
	if want := (source.Position{Line: 1, Column: 6}); got != want {
		t.Errorf("Position(%d) = %d:%d, want %d:%d",
			offsetOfX, got.Line, got.Column, want.Line, want.Column)
	}
}

func TestPositionOnLineWithMultiByteBefore(t *testing.T) {
	// The counting path only runs for a line that carries non-ASCII content (§9), so
	// a file that is mostly ASCII must still get the right answer on the one line
	// that is not.
	f := mustNew(t, "let a = 1;\nlet s = 'é';\nlet b = 2;\n")
	const offsetOfSemicolon = 11 + 12 // second line, after the closing quote
	if got := f.Text()[offsetOfSemicolon]; got != ';' {
		t.Fatalf("test is miscalibrated: byte %d is %q, not ';'", offsetOfSemicolon, got)
	}
	// Eleven runes precede the ';' on line 2 (l e t ␣ s ␣ = ␣ ' é '), so the column
	// is 12. Twelve *bytes* precede it, because é is two, and a byte-counting
	// implementation therefore reports 13. Counting the runes above is how this
	// expectation stays auditable: the first draft of this test asserted 13.
	got := f.Position(offsetOfSemicolon)
	if got.Line != 2 || got.Column != 12 {
		t.Errorf("Position(%d) = %d:%d, want 2:12", offsetOfSemicolon, got.Line, got.Column)
	}
}

// --- The buffer -------------------------------------------------------------------

func TestSliceRecoversTheLexeme(t *testing.T) {
	// Tokens carry spans, not copies (§9). This is the operation that makes that
	// work, and the one the formatter relies on to see a trivia token's exact bytes.
	src := "let x = 1;"
	f := mustNew(t, src)
	if got := f.Slice(4, 1); got != "x" {
		t.Errorf("Slice(4, 1) = %q, want %q", got, "x")
	}
	if got := f.Slice(0, 3); got != "let" {
		t.Errorf("Slice(0, 3) = %q, want %q", got, "let")
	}
	if got := f.Slice(0, len(src)); got != src {
		t.Errorf("Slice over the whole file = %q, want %q", got, src)
	}
}

func TestNamePreserved(t *testing.T) {
	// R240 makes file identity part of every span, because a secondary span
	// routinely lives in another module.
	f, err := source.New("pkg/thing.luna", "x")
	if err != nil {
		t.Fatal(err)
	}
	if got := f.Name(); got != "pkg/thing.luna" {
		t.Errorf("Name() = %q, want %q", got, "pkg/thing.luna")
	}
}

func TestLoadReportsAMissingFile(t *testing.T) {
	_, err := source.Load("does/not/exist.luna")
	if err == nil {
		t.Fatal("Load of a missing file succeeded")
	}
	// Not a diagnostic: a missing file is an IO failure, not a lexical one, so it
	// carries no L-code.
	var e *source.Error
	if errors.As(err, &e) {
		t.Errorf("missing file reported as a lexical diagnostic %s", e.Code)
	}
	if !strings.Contains(err.Error(), "exist.luna") {
		t.Errorf("error %q does not name the file", err)
	}
}
