// The fuzz target (lexer-testing-plan §6).
//
// A lexer is an unusually good fuzz target because every property worth checking needs no
// oracle: the answers are structural, so an arbitrary input can be judged without anyone
// knowing what it should have lexed to.
//
// This is an *internal* test, deliberately: the mode-stack property reads s.modes, and
// growing the exported API to let a test see it would be the tail wagging the dog. The
// golden harness next door stays external, where it belongs.
package lexer

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"luna/internal/spec"
	"luna/oracle/diagnostic"
	"luna/oracle/source"
	"luna/oracle/token"
)

// FuzzLexer asserts what holds for every input, valid Luna or not.
//
//  1. Ingress rejects cleanly. Most of what a mutator produces is not valid UTF-8, and
//     that is not a wasted case: the rejection must be a *source.Error carrying L0001 or
//     L0002 (lexical-structure §1), never a panic and never some other code.
//  2. The scan never panics, on any byte sequence at all.
//  3. It terminates. Structural since R242, Next panicking unless the step covered at least
//     one byte, so a target that returns has proved it and one that loops is caught by
//     the fuzzer's own timeout rather than by an assertion here.
//  4. Spans tile the input exactly: monotonic, gapless, summing to the length.
//  5. Every frame still open at end of input is explained by a diagnostic, and no token
//     carries Unset, the kind that names no token and must never be emitted.
//
// Their strengths differ, and it is worth knowing which is which. **Tiling is now
// tautological**, and deliberately so: Next builds every span as start..s.pos and asserts
// that a mode both advanced and stayed in bounds, so tokens tile by construction and this
// check survives only as a guard on that arithmetic. Nor does it see a mode consuming the
// *wrong* number of bytes while staying in range: that still tiles perfectly and produces
// the wrong tokens. R242's direction was to convert tested properties into structural
// ones; the residue is the wrong-token class, which §7's differential was the only thing
// that would have caught.
//
// **Property 5 is the one with teeth.** Unset doubles as the "no match" sentinel inside the
// scanner, so a missing check returns it as a real token rather than falling through; and
// an unreported open frame is a file that ends mid-literal and compiles clean. Both are
// mutation-tested: emitting Unset from the catch-all, and making finish report nothing,
// each fail within a handful of random inputs.
//
// A sixth property waits on the spec-literal reference lexer (§7): running both over the
// same input and diffing catches *wrong-token* bugs, which none of the five above can see.
func FuzzLexer(f *testing.F) {
	for _, seed := range fuzzSeeds(f) {
		f.Add([]byte(seed))
	}

	f.Fuzz(checkProperties)
}

// checkProperties asserts the five properties over one input.
//
// Shared with TestRandomStreams rather than restated there, because the properties are one
// idea and the two callers differ only in where their bytes come from: coverage-guided
// mutation from real Luna here, blind uniform draws there. It also matters that this runs
// at all by default: a plain `go test` gives a fuzz target only its seed corpus, so
// without the random caller these would be checked on the curated seeds and on nothing
// else until somebody remembered to pass -fuzz.
func checkProperties(t *testing.T, data []byte) {
	src, err := source.New("fuzz", string(data))
	if err != nil {
		var e *source.Error
		if !errors.As(err, &e) {
			t.Fatalf("%s: ingress failed with %T (%v), want a *source.Error", brief(data), err, err)
		}
		if e.Code != diagnostic.InvalidUTF8 && e.Code != diagnostic.ByteOrderMark {
			t.Fatalf("%s: ingress raised %s, want L0001 or L0002", brief(data), e.Code)
		}
		return
	}

	s := New(src)
	next := 0
	for {
		tok, ok := s.Next()
		if !ok {
			break
		}
		if tok.Kind == token.Unset {
			t.Fatalf("%s: token at %d carries Unset", brief(data), tok.Offset)
		}
		if tok.Offset != next {
			t.Fatalf("%s: %s starts at %d, previous ended at %d",
				brief(data), tok.Kind, tok.Offset, next)
		}
		if tok.Len < 1 {
			t.Fatalf("%s: %s at %d covers %d bytes", brief(data), tok.Kind, tok.Offset, tok.Len)
		}
		next = tok.End()
	}
	if next != len(data) {
		t.Fatalf("%s: spans end at %d, input is %d bytes", brief(data), next, len(data))
	}

	// modes[0] is DEFAULT and never pops. Anything above it is a literal or a splice the
	// input left open, which finish must have reported (§11).
	if len(s.modes) > 1 && s.errors.Empty() {
		t.Fatalf("%s: %d frames open at end of input with no diagnostic",
			brief(data), len(s.modes)-1)
	}
}

// brief quotes an input for a failure message, truncated: a fuzz counterexample can be
// long, and the first bytes are what identify it.
func brief(data []byte) string {
	const max = 72
	if len(data) <= max {
		return strconv.Quote(string(data))
	}
	return strconv.Quote(string(data[:max])) + "…"
}

// fuzzSeeds is real Luna rather than generated noise, from two sources that complement
// each other: the golden corpus, whose inputs are deliberately malformed and so seed the
// error paths, and the spec's own `luna` blocks, which are valid and carry the realistic
// keyword and nesting vocabulary a mutator would take a long time to discover.
//
// Seeds also run on a plain `go test`, without -fuzz. So this doubles as the corpus gate
// (§9): every spec block is lexed on every run, and a change that breaks one fails here
// rather than waiting for someone to fuzz.
func fuzzSeeds(f *testing.F) []string {
	f.Helper()

	var seeds []string
	// Both levels, rather than naming error_producing: any subdirectory the corpus grows
	// later seeds the fuzzer without anyone remembering to add it here.
	for _, pattern := range []string{"testdata/*.lex", "testdata/*/*.lex"} {
		paths, err := filepath.Glob(pattern)
		if err != nil {
			f.Fatal(err)
		}
		for _, p := range paths {
			raw, err := os.ReadFile(p)
			if err != nil {
				f.Fatal(err)
			}
			input, _, found := strings.Cut(string(raw), "---\n")
			if !found {
				f.Fatalf("%s: no --- separator", p)
			}
			seeds = append(seeds, input)
		}
	}

	blocks, err := spec.LunaBlocks()
	if err != nil {
		f.Fatalf("reading the spec corpus: %v", err)
	}
	for _, b := range blocks {
		seeds = append(seeds, b.Source)
	}

	// Fail loud rather than fuzz from nothing: an empty seed set would still pass, having
	// checked one empty input, which is the shape of fail-open this project keeps finding.
	if len(seeds) < 400 {
		f.Fatalf("only %d seeds; expected the goldens plus ~436 spec blocks", len(seeds))
	}
	return seeds
}
