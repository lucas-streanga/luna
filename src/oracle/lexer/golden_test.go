// The golden corpus harness (lexer-testing-plan §2).
//
// Each testdata/*.lex file pairs Luna source with the token stream it must produce.
// The format is documented in testdata/FORMAT.md; this file parses it, runs the
// lexer, and compares.
//
// Run with -update to rewrite the expectations from what the lexer actually produced.
// That is a convenience, not an authority: a regenerated golden asserts only that the
// lexer still does what it did, bugs included. The discipline FORMAT.md states is that
// the resulting *diff* is read against lexer §0 before it is committed.
package lexer_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"luna/oracle/diagnostic"
	"luna/oracle/lexer"
	"luna/oracle/source"
)

var update = flag.Bool("update", false, "rewrite testdata/*.lex from actual lexer output")

// entry is one expectation line: a token, or a diagnostic when Code is set. Both
// carry a span, which is what lets them be compared in one ordered sequence — the
// order the scanner produced them, which is source order.
type entry struct {
	Code   diagnostic.Code // empty for a token line
	Name   string          // token name (§0) for a token line
	Lo, Hi int
	Lexeme string // token lines only
}

func (e entry) String() string {
	if e.Code != "" {
		return fmt.Sprintf("!%s %d..%d", e.Code, e.Lo, e.Hi)
	}
	return fmt.Sprintf("%s %d..%d %s", e.Name, e.Lo, e.Hi, strconv.Quote(e.Lexeme))
}

type golden struct {
	path   string
	input  string
	expect []entry
}

// parseGolden splits a .lex file at the --- separator and reads the expectations.
//
// Everything before the separator is the input, its trailing newline included: real
// files end with one, and the format represents that faithfully at the cost of being
// unable to express a file that does not (FORMAT.md).
func parseGolden(t *testing.T, path string) golden {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden: %v", err)
	}
	input, dump, found := strings.Cut(string(raw), "---\n")
	if !found {
		t.Fatalf("%s: no --- separator", path)
	}

	g := golden{path: path, input: input}
	for i, line := range strings.Split(strings.TrimRight(dump, "\n"), "\n") {
		if line == "" {
			continue
		}
		e, err := parseEntry(line)
		if err != nil {
			t.Fatalf("%s: expectation %d: %v", path, i+1, err)
		}
		g.expect = append(g.expect, e)
	}
	return g
}

func parseEntry(line string) (entry, error) {
	if rest, ok := strings.CutPrefix(line, "!"); ok {
		code, span, found := strings.Cut(rest, " ")
		if !found {
			return entry{}, fmt.Errorf("malformed diagnostic line %q", line)
		}
		lo, hi, err := parseSpan(span)
		if err != nil {
			return entry{}, err
		}
		return entry{Code: diagnostic.Code(code), Lo: lo, Hi: hi}, nil
	}

	f := strings.SplitN(line, " ", 3)
	if len(f) != 3 {
		return entry{}, fmt.Errorf("want `NAME lo..hi \"lexeme\"`, got %q", line)
	}
	lo, hi, err := parseSpan(f[1])
	if err != nil {
		return entry{}, err
	}
	lexeme, err := strconv.Unquote(f[2])
	if err != nil {
		return entry{}, fmt.Errorf("lexeme %q: %w", f[2], err)
	}
	return entry{Name: f[0], Lo: lo, Hi: hi, Lexeme: lexeme}, nil
}

func parseSpan(s string) (lo, hi int, err error) {
	before, after, found := strings.Cut(s, "..")
	if !found {
		return 0, 0, fmt.Errorf("malformed span %q", s)
	}
	if lo, err = strconv.Atoi(before); err != nil {
		return 0, 0, fmt.Errorf("malformed span %q: %w", s, err)
	}
	if hi, err = strconv.Atoi(after); err != nil {
		return 0, 0, fmt.Errorf("malformed span %q: %w", s, err)
	}
	return lo, hi, nil
}

// actual lexes the input and flattens tokens and diagnostics into one source-ordered
// sequence, which is the shape a golden file records.
func actual(t *testing.T, g golden) []entry {
	t.Helper()
	f, err := source.New(filepath.Base(g.path), g.input)
	if err != nil {
		t.Fatalf("%s: the input is not valid source: %v", g.path, err)
	}

	toks, errs := lexer.Lex(f)

	out := make([]entry, 0, len(toks)+len(errs))
	for _, tok := range toks {
		out = append(out, entry{
			Name:   tok.Kind.String(),
			Lo:     tok.Offset,
			Hi:     tok.End(),
			Lexeme: f.Slice(tok.Offset, tok.Len),
		})
	}
	for _, d := range errs {
		out = append(out, entry{
			Code: d.Code,
			Lo:   d.Primary.Offset,
			Hi:   d.Primary.End(),
		})
	}
	// Stable by start offset: tokens and diagnostics interleave in source order, which
	// is how a reader reads the file and how FORMAT.md records it.
	sortByOffset(out)
	return out
}

func sortByOffset(es []entry) {
	for i := 1; i < len(es); i++ {
		for j := i; j > 0 && es[j].Lo < es[j-1].Lo; j-- {
			es[j], es[j-1] = es[j-1], es[j]
		}
	}
}

func TestGolden(t *testing.T) {
	paths, err := filepath.Glob("testdata/*.lex")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no testdata/*.lex files; the corpus is the test")
	}

	for _, path := range paths {
		t.Run(strings.TrimSuffix(filepath.Base(path), ".lex"), func(t *testing.T) {
			g := parseGolden(t, path)
			got := actual(t, g)

			if *update {
				rewrite(t, g, got)
				return
			}
			compare(t, g, got)
			checkTiling(t, g, got)
		})
	}
}

func compare(t *testing.T, g golden, got []entry) {
	t.Helper()
	for i := range max(len(got), len(g.expect)) {
		switch {
		case i >= len(got):
			t.Errorf("missing at %d: want %s", i, g.expect[i])
		case i >= len(g.expect):
			t.Errorf("unexpected at %d: got %s", i, got[i])
		case got[i] != g.expect[i]:
			t.Errorf("at %d:\n  got  %s\n  want %s", i, got[i], g.expect[i])
		}
	}
}

// checkTiling asserts the invariant R236 made total: with trivia emitted, a file's
// tokens cover every byte, gaplessly and without overlap, so concatenating their
// lexemes reproduces the input.
//
// Only for cases the golden expects to lex cleanly. What an erroring scan covers is
// not yet ruled — §2 states the invariant over *token* spans, and a diagnostic's span
// is not a token — so asserting it here would invent a rule rather than check one.
func checkTiling(t *testing.T, g golden, got []entry) {
	t.Helper()
	for _, e := range got {
		if e.Code != "" {
			return
		}
	}

	var rebuilt strings.Builder
	next := 0
	for _, e := range got {
		if e.Lo != next {
			t.Errorf("span gap or overlap: %s starts at %d, previous ended at %d", e, e.Lo, next)
			return
		}
		rebuilt.WriteString(e.Lexeme)
		next = e.Hi
	}
	if next != len(g.input) {
		t.Errorf("spans end at %d, input is %d bytes", next, len(g.input))
	}
	if rebuilt.String() != g.input {
		t.Errorf("concatenated lexemes do not reproduce the input")
	}
}

func rewrite(t *testing.T, g golden, got []entry) {
	t.Helper()
	var b strings.Builder
	b.WriteString(g.input)
	b.WriteString("---\n")
	for _, e := range got {
		b.WriteString(e.String())
		b.WriteByte('\n')
	}
	if err := os.WriteFile(g.path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("rewriting golden: %v", err)
	}
	t.Logf("rewrote %s — read the diff against lexer §0 before committing", g.path)
}
