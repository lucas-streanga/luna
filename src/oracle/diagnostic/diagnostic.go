// Package diagnostic is the vocabulary for compile errors: a code that names the
// check, spans that locate it, and prose that explains it.
//
// Compiler §3.1 (R240) fixes the shape. Every diagnostic carries a code — a
// one-letter stage prefix and four digits — because prose churns and tests must pin
// something stable. There is no severity: §3 rules "no warnings, ever", so a
// condition either stops the build or is not reported, and every code is an error.
//
// This package holds data only. It does not import source, and rendering lives above
// it: drawing a caret needs the source line, which would make a renderer depend on
// both, and a Span identifies its file by name so a diagnostic stays serializable —
// the language server consumes these, and ingress failures name a file that never
// became a *source.File at all.
package diagnostic

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
)

// Stage is the one-letter prefix naming the stage that *defined* a check — not
// necessarily the one that runs it. A check that migrates to an earlier phase keeps
// its original code (R240): stability beats taxonomy, and renumbering would collide
// with codes already allocated.
type Stage byte

const (
	Lexical  Stage = 'L' // §1.1
	Syntax   Stage = 'P' // §1.3
	Semantic Stage = 'S' // §1.4
	Modules  Stage = 'M' // §1.0 discovery, §1.2 import validation
	Comptime Stage = 'C' // §6
	Build    Stage = 'B' // §1.8, toolchain invocation, the incremental cache
	Format   Stage = 'F' // luna -f
	Tooling  Stage = 'T' // LSP, debugger, test runner
	Internal Stage = 'I' // an invariant violated: "this is a compiler bug"
)

// Code identifies a diagnostic, e.g. "L0003". Each stage numbers independently, so
// L0001 and P0001 are unrelated — which is what lets stages allocate without a
// central registry (R240).
//
// Codes are owned by the package that raises them and pinned against the spec's error
// summary (lexer §11) the way token kinds are pinned against §0. Numbering starts at
// 0001; 0000 is never allocated, too much tooling reading zero as "no error".
type Code string

// Stage reports the code's prefix. It returns 0 for a malformed code.
func (c Code) Stage() Stage {
	if !c.Valid() {
		return 0
	}
	return Stage(c[0])
}

// Valid reports whether the code is a known stage prefix followed by four digits.
// 0000 is not valid: it is never allocated, too much tooling reading zero as "no
// error" (R240).
func (c Code) Valid() bool {
	if len(c) != 5 || !stages[Stage(c[0])] {
		return false
	}
	zero := true
	for i := 1; i < 5; i++ {
		if c[i] < '0' || c[i] > '9' {
			return false
		}
		zero = zero && c[i] == '0'
	}
	return !zero
}

// Title is the phrase fixed to this code — what `luna explain` would print. It
// returns "" for a code with no registered title, which Validate treats as an error:
// a code nobody named is a code nobody documented.
// Each stage keeps its own registry, mirroring §11-and-§12's one-table-per-stage shape, and
// the switch is what makes an unregistered stage return "" rather than silently borrowing
// another's numbering — L0001 and M0001 are unrelated codes (R240).
func (c Code) Title() string {
	switch c.Stage() {
	case Lexical:
		return lexicalTitles[c]
	case Modules:
		return modulesTitles[c]
	}
	return ""
}

var stages = map[Stage]bool{
	Lexical: true, Syntax: true, Semantic: true, Modules: true, Comptime: true,
	Build: true, Format: true, Tooling: true, Internal: true,
}

// Span is a range of bytes in a named file.
//
// Filename is a name rather than a *source.File because a diagnostic must survive
// leaving the process (the language server), and because ingress failures — invalid
// UTF-8, a leading BOM — describe a file that never became a valid one. Resolving a
// name to text is the renderer's job.
type Span struct {
	Filename string
	Offset   int
	Length   int
}

// End is the offset one past the span, so a span is [Offset, End).
func (s Span) End() int { return s.Offset + s.Length }

// Label is a secondary span and the phrase that explains it — "declared here",
// "consumed here". Labels carry the narrative; the primary span carries the caret.
type Label struct {
	Span Span
	Text string
}

// Suggestion is a machine-applicable fix: replace the span's text with Replacement.
// Present so `luna -l` can offer code actions without every hint being rewritten
// later (R240); a hint without one is ordinary prose.
type Suggestion struct {
	Span        Span
	Replacement string
}

// Hint is advice, optionally carrying a fix a tool can apply.
type Hint struct {
	Text       string
	Suggestion *Suggestion
}

// Diagnostic is one compile error.
//
// Description is the per-instance half of the prose: volatile, naming the binding or
// the lexeme that provoked this one. The other half, the title, is not a field —
// it is fixed per code (R240) and read back through Title, so it cannot drift from
// the code or be contradicted by a caller.
//
// Only the code and the primary span are pinned by tests (testing-strategy §2).
type Diagnostic struct {
	Code        Code
	Description string

	// Primary is where the caret goes, and is mandatory: exactly one, so "the
	// location" is never ambiguous and a test always has an unambiguous anchor.
	Primary Span

	Labels []Label
	Notes  []string
	Hints  []Hint
}

// New starts a diagnostic from the three parts that are never optional: which check
// fired, where, and what to say about this instance. Everything else is a decoration
// added by the builder methods, which return the receiver so a call site reads as one
// expression.
func New(code Code, primary Span, format string, args ...any) *Diagnostic {
	// Formatted unconditionally, as any format-taking function does. A caller passing
	// raw data where a format belongs is the same bug as fmt.Printf(userInput), and go
	// vet catches it at the call site.
	return &Diagnostic{
		Code:        code,
		Description: fmt.Sprintf(format, args...),
		Primary:     primary,
	}
}

// Title is the phrase fixed to the code — derived, never stored, so a diagnostic
// cannot claim a title its code does not have.
func (d *Diagnostic) Title() string { return d.Code.Title() }

// Label attaches a secondary span. Order is the order added; a renderer may present
// them in source order instead.
func (d *Diagnostic) Label(span Span, text string) *Diagnostic {
	d.Labels = append(d.Labels, Label{Span: span, Text: text})
	return d
}

// Note attaches context that is not tied to a span.
func (d *Diagnostic) Note(text string) *Diagnostic {
	d.Notes = append(d.Notes, text)
	return d
}

// Hint attaches advice with no machine-applicable fix.
func (d *Diagnostic) Hint(text string) *Diagnostic {
	d.Hints = append(d.Hints, Hint{Text: text})
	return d
}

// Suggest attaches advice with a fix a tool can apply.
func (d *Diagnostic) Suggest(text string, span Span, replacement string) *Diagnostic {
	d.Hints = append(d.Hints, Hint{
		Text:       text,
		Suggestion: &Suggestion{Span: span, Replacement: replacement},
	})
	return d
}

// Validate reports whether the diagnostic is well formed: a code that is valid and
// carries a title, and a primary span naming a file. It exists because the fields are
// exported and a caller may build one directly, bypassing New — and because an
// untitled code means one was allocated without being added to the spec's error
// summary, which is the drift this catches at runtime and the code pin catches
// statically.
func (d *Diagnostic) Validate() error {
	switch {
	case d == nil:
		return errors.New("diagnostic: nil")
	case !d.Code.Valid():
		return fmt.Errorf("diagnostic: malformed code %q", string(d.Code))
	case d.Code.Title() == "":
		return fmt.Errorf("diagnostic: %s has no title, so it is absent from the spec's error summary", d.Code)
	case d.Primary.Filename == "":
		return fmt.Errorf("diagnostic: %s has a primary span naming no file", d.Code)
	}
	return nil
}

// String is a one-line debug form: "L0003 t.luna@8: Leading zero". It reports the
// byte offset, not a line and column, because resolving those needs the source text
// this package deliberately does not hold.
func (d *Diagnostic) String() string {
	return fmt.Sprintf("%s %s@%d: %s", d.Code, d.Primary.Filename, d.Primary.Offset, d.Title())
}

// List accumulates the diagnostics of one phase.
//
// Compiler §3 requires a phase to run to completion collecting everything it finds
// and abort at the boundary, rather than stopping at the first error — so the unit a
// phase returns is a list, and "did anything fail" is asked once, at the end.
//
// A named slice rather than a wrapper: there is nothing to hide, and callers that
// want to range or append directly should not have to go through a method to do it.
type List []*Diagnostic

// Add appends a diagnostic. It panics if the diagnostic is malformed, since that is a
// compiler bug rather than a condition in the program being compiled.
func (l *List) Add(d *Diagnostic) {
	if err := d.Validate(); err != nil {
		panic(err)
	}
	*l = append(*l, d)
}

// Empty reports whether the phase found nothing — the question §3's phase boundary
// asks.
func (l List) Empty() bool { return len(l) == 0 }

// Sorted returns a copy ordered by file, then offset: the order a reader expects,
// independent of the order a phase happened to discover them in.
//
// A copy, because a phase may already have handed its list to something else, and
// reordering underneath it would be a surprise. Stable, because two diagnostics at
// one position should keep discovery order rather than an arbitrary one — compiler §8
// makes determinism a property this compiler owes, not one it hopes for.
func (l List) Sorted() List {
	out := slices.Clone(l)
	slices.SortStableFunc(out, func(a, b *Diagnostic) int {
		if c := cmp.Compare(a.Primary.Filename, b.Primary.Filename); c != 0 {
			return c
		}
		return cmp.Compare(a.Primary.Offset, b.Primary.Offset)
	})
	return out
}
