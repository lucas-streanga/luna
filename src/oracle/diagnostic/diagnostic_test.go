// Tests for the diagnostic vocabulary.
//
// The centrepiece is TestCodesMatchSpec: lexer §11 is the registry of lexical codes,
// and the constants here must mirror it exactly. Everything else pins the rules R240
// states about codes, spans, and what a well-formed diagnostic is.
package diagnostic_test

import (
	"testing"

	"luna/internal/spec"
	"luna/oracle/diagnostic"
)

func span(name string, off, length int) diagnostic.Span {
	return diagnostic.Span{Filename: name, Offset: off, Length: length}
}

// --- Code ---------------------------------------------------------------------------

func TestCodeValid(t *testing.T) {
	valid := []diagnostic.Code{
		"L0001", "L0012", "P0001", "S0143", "M0001",
		"C0001", "B0001", "F0001", "T0001", "I0001",
		"L9999",
	}
	for _, c := range valid {
		if !c.Valid() {
			t.Errorf("%q: Valid() = false, want true", c)
		}
	}

	invalid := map[string]diagnostic.Code{
		"empty":            "",
		"too short":        "L001",
		"too long":         "L00001",
		"unknown prefix":   "X0001",
		"lowercase prefix": "l0001",
		"non-digit":        "L000A",
		"no digits":        "LLLLL",
		"digits only":      "00001",
		// R240: 0000 is never allocated, too much tooling reading zero as "no error".
		"zero": "L0000",
	}
	for name, c := range invalid {
		if c.Valid() {
			t.Errorf("%s (%q): Valid() = true, want false", name, c)
		}
	}
}

func TestCodeStage(t *testing.T) {
	for _, tc := range []struct {
		code diagnostic.Code
		want diagnostic.Stage
	}{
		{"L0001", diagnostic.Lexical},
		{"P0001", diagnostic.Syntax},
		{"S0143", diagnostic.Semantic},
		{"I0001", diagnostic.Internal},
	} {
		if got := tc.code.Stage(); got != tc.want {
			t.Errorf("%q: Stage() = %q, want %q", tc.code, got, tc.want)
		}
	}

	// A malformed code has no stage, rather than reporting whatever byte is first.
	for _, c := range []diagnostic.Code{"", "X0001", "L0000", "l0001"} {
		if got := c.Stage(); got != 0 {
			t.Errorf("%q: Stage() = %q, want 0", c, got)
		}
	}
}

// TestCodesMatchSpec pins the lexical codes against lexer §11, which is the registry.
// A code allocated in Go but absent from §11 is a check nobody documented; one in §11
// but absent here is a check nobody implemented.
func TestCodesMatchSpec(t *testing.T) {
	inv, err := spec.Load()
	if err != nil {
		t.Fatalf("reading the lexer spec: %v", err)
	}
	if len(inv.Codes) == 0 {
		t.Fatal("no §11 rows parsed; the error-summary table changed shape")
	}

	for _, row := range inv.Codes {
		c := diagnostic.Code(row.Code)
		if !c.Valid() {
			t.Errorf("§11:%d: %q is not a well-formed code", row.Line, row.Code)
			continue
		}
		if got := c.Title(); got != row.Title {
			t.Errorf("§11:%d: %s title is %q in the spec, %q in Go", row.Line, c, row.Title, got)
		}
	}

	// And the other direction: every titled code must appear in §11. Without this the
	// test only catches deletions from Go, never additions to it.
	inSpec := map[string]bool{}
	for _, row := range inv.Codes {
		inSpec[row.Code] = true
	}
	for _, c := range lexicalCodes {
		if !inSpec[string(c)] {
			t.Errorf("%s is defined in Go but has no §11 row", c)
		}
	}
	if len(lexicalCodes) != len(inv.Codes) {
		t.Errorf("Go defines %d lexical codes, §11 lists %d", len(lexicalCodes), len(inv.Codes))
	}
}

// lexicalCodes is every code codes_lexical.go defines. Listed explicitly rather than
// derived, so that adding a constant without adding it here fails the count check
// above — a derived list would silently grow to match itself.
var lexicalCodes = []diagnostic.Code{
	diagnostic.InvalidUTF8,
	diagnostic.ByteOrderMark,
	diagnostic.LeadingZero,
	diagnostic.UppercaseRadixPrefix,
	diagnostic.UnknownEscape,
	diagnostic.InvalidCodepointEscape,
	diagnostic.UnexpectedHash,
	diagnostic.UnexpectedTilde,
	diagnostic.UnterminatedLiteral,
	diagnostic.UnterminatedBlockComment,
	diagnostic.UnterminatedInterpolation,
	diagnostic.UnexpectedCharacter,
	diagnostic.MalformedCodepointEscape,
	diagnostic.InsufficientIndentation,
	diagnostic.ContentAfterTripleOpen,
	diagnostic.MalformedByteEscape,
}

func TestLexicalCodesAreLexicalAndDistinct(t *testing.T) {
	seen := map[diagnostic.Code]bool{}
	for _, c := range lexicalCodes {
		if c.Stage() != diagnostic.Lexical {
			t.Errorf("%s: stage is %q, want L", c, c.Stage())
		}
		if seen[c] {
			t.Errorf("%s: allocated twice", c)
		}
		seen[c] = true
	}
}

func TestTitleOfUnregisteredCodeIsEmpty(t *testing.T) {
	// Well formed but never allocated. Empty rather than a placeholder, so Validate
	// can treat it as the drift it is.
	if got := diagnostic.Code("L9999").Title(); got != "" {
		t.Errorf("Title() = %q, want \"\"", got)
	}
}

// --- Span ---------------------------------------------------------------------------

func TestSpanEnd(t *testing.T) {
	for _, tc := range []struct {
		s    diagnostic.Span
		want int
	}{
		{span("a.luna", 0, 3), 3},
		{span("a.luna", 8, 4), 12},
		{span("a.luna", 5, 0), 5}, // empty span: a point, not a range
	} {
		if got := tc.s.End(); got != tc.want {
			t.Errorf("%+v: End() = %d, want %d", tc.s, got, tc.want)
		}
	}
}

// --- Diagnostic ---------------------------------------------------------------------

func TestNew(t *testing.T) {
	sp := span("t.luna", 8, 4)
	d := diagnostic.New(diagnostic.LeadingZero, sp, "`%s` has a leading zero", "0755")

	if d.Code != diagnostic.LeadingZero {
		t.Errorf("Code = %q, want %q", d.Code, diagnostic.LeadingZero)
	}
	if d.Primary != sp {
		t.Errorf("Primary = %+v, want %+v", d.Primary, sp)
	}
	if want := "`0755` has a leading zero"; d.Description != want {
		t.Errorf("Description = %q, want %q", d.Description, want)
	}
}

func TestTitleIsDerivedFromCode(t *testing.T) {
	// The title is not stored (R240): it cannot be set independently, so it cannot
	// contradict the code. Changing the code changes the title.
	d := diagnostic.New(diagnostic.LeadingZero, span("t.luna", 0, 1), "x")
	if got, want := d.Title(), "Leading zero"; got != want {
		t.Errorf("Title() = %q, want %q", got, want)
	}
	d.Code = diagnostic.UnexpectedTilde
	if got, want := d.Title(), "Unexpected `~`"; got != want {
		t.Errorf("after changing Code, Title() = %q, want %q", got, want)
	}
}

func TestBuildersAccumulateAndChain(t *testing.T) {
	primary := span("t.luna", 10, 2)
	decl := span("t.luna", 0, 6)
	fix := span("t.luna", 10, 2)

	d := diagnostic.New(diagnostic.UnterminatedLiteral, primary, "unterminated string")
	got := d.
		Label(decl, "opened here").
		Note("string literals do not span statements").
		Hint("close it before the semicolon").
		Suggest("insert the closing quote", fix, `"`)

	// Each builder returns the receiver, so a chain decorates one diagnostic rather
	// than producing copies.
	if got != d {
		t.Error("builder chain returned a different diagnostic")
	}

	if len(d.Labels) != 1 || d.Labels[0].Span != decl || d.Labels[0].Text != "opened here" {
		t.Errorf("Labels = %+v", d.Labels)
	}
	if len(d.Notes) != 1 || d.Notes[0] != "string literals do not span statements" {
		t.Errorf("Notes = %+v", d.Notes)
	}
	if len(d.Hints) != 2 {
		t.Fatalf("Hints = %+v, want 2", d.Hints)
	}
	if d.Hints[0].Suggestion != nil {
		t.Error("Hint attached a suggestion; only Suggest should")
	}
	s := d.Hints[1].Suggestion
	if s == nil {
		t.Fatal("Suggest attached no suggestion")
	}
	if s.Span != fix || s.Replacement != `"` {
		t.Errorf("Suggestion = %+v", s)
	}
}

func TestValidate(t *testing.T) {
	ok := diagnostic.New(diagnostic.LeadingZero, span("t.luna", 0, 1), "x")
	if err := ok.Validate(); err != nil {
		t.Errorf("well-formed diagnostic rejected: %v", err)
	}

	for name, d := range map[string]*diagnostic.Diagnostic{
		"malformed code": {Code: "nope", Primary: span("t.luna", 0, 1)},
		// Well formed but not in §11: a code allocated without being documented.
		"untitled code": {Code: "L9999", Primary: span("t.luna", 0, 1)},
		"no filename":   {Code: diagnostic.LeadingZero, Primary: span("", 0, 1)},
	} {
		if err := d.Validate(); err == nil {
			t.Errorf("%s: Validate() = nil, want an error", name)
		}
	}
}

func TestString(t *testing.T) {
	d := diagnostic.New(diagnostic.LeadingZero, span("t.luna", 8, 4), "ignored here")
	// The byte offset, not a line and column: resolving those needs the source text
	// this package deliberately does not hold.
	if want := "L0003 t.luna@8: Leading zero"; d.String() != want {
		t.Errorf("String() = %q, want %q", d.String(), want)
	}
}

// --- List ---------------------------------------------------------------------------

func TestListAccumulates(t *testing.T) {
	var l diagnostic.List
	if !l.Empty() {
		t.Error("a zero List is not Empty")
	}

	l.Add(diagnostic.New(diagnostic.LeadingZero, span("t.luna", 0, 1), "a"))
	l.Add(diagnostic.New(diagnostic.UnexpectedTilde, span("t.luna", 4, 1), "b"))

	if l.Empty() {
		t.Error("Empty() after two Adds")
	}
	if len(l) != 2 {
		t.Errorf("len = %d, want 2", len(l))
	}
}

func TestListAddRejectsMalformed(t *testing.T) {
	// A malformed diagnostic is a compiler bug, not a condition in the program being
	// compiled, so it panics rather than being collected and reported to a user.
	defer func() {
		if recover() == nil {
			t.Error("Add of a malformed diagnostic did not panic")
		}
	}()
	var l diagnostic.List
	l.Add(&diagnostic.Diagnostic{Code: "nope"})
}

func TestListSorted(t *testing.T) {
	// §3 collects in discovery order, which is not reading order once a phase runs
	// files in parallel. Sorted gives the order a reader expects.
	var l diagnostic.List
	l.Add(diagnostic.New(diagnostic.LeadingZero, span("b.luna", 5, 1), "third"))
	l.Add(diagnostic.New(diagnostic.LeadingZero, span("a.luna", 9, 1), "second"))
	l.Add(diagnostic.New(diagnostic.LeadingZero, span("a.luna", 2, 1), "first"))

	got := l.Sorted()
	want := []string{"first", "second", "third"}
	if len(got) != len(want) {
		t.Fatalf("Sorted() returned %d, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Description != w {
			t.Errorf("Sorted()[%d] = %q, want %q", i, got[i].Description, w)
		}
	}

	// Sorting returns a copy: a phase that has already handed out its list should not
	// see it reordered underneath.
	if l[0].Description != "third" {
		t.Errorf("Sorted() reordered the receiver: l[0] = %q", l[0].Description)
	}
}
