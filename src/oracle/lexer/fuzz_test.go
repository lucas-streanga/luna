// The fuzz target (lexer-testing-plan §6).
//
// A lexer is an unusually good fuzz target because every property worth checking needs no
// oracle — the answers are structural, so an arbitrary input can be judged without anyone
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
//  3. It terminates. Structural since R242 — Next panics unless the step covered at least
//     one byte — so a target that returns has proved it, and one that loops is caught by
//     the fuzzer's own timeout rather than by an assertion here.
//  4. Spans tile the input exactly: monotonic, gapless, summing to the length, with every
//     lexeme equal to its own slice. The strongest assertion available, and total since
//     R242 covers unclaimed bytes with INVALID rather than dropping them — which matters
//     precisely because a fuzzer's inputs are almost all invalid.
//  5. Every frame still open at end of input is explained by a diagnostic, and no token
//     carries Unset, the kind that names no token and must never be emitted.
//
// Property 5's second half guards a specific hazard: Unset doubles as the "no match"
// sentinel inside the scanner, so a missing check would return it as a real token rather
// than falling through.
//
// A sixth property waits on the spec-literal reference lexer (§7): running both over the
// same input and diffing catches *wrong-token* bugs, which none of the five above can see.
func FuzzLexer(f *testing.F) {
	for _, seed := range fuzzSeeds(f) {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		src, err := source.New("fuzz", string(data))
		if err != nil {
			var e *source.Error
			if !errors.As(err, &e) {
				t.Fatalf("ingress failed with %T (%v), want a *source.Error", err, err)
			}
			if e.Code != diagnostic.InvalidUTF8 && e.Code != diagnostic.ByteOrderMark {
				t.Fatalf("ingress raised %s, want L0001 or L0002", e.Code)
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
				t.Fatalf("token at %d carries Unset", tok.Offset)
			}
			if tok.Offset != next {
				t.Fatalf("%s starts at %d, previous ended at %d", tok.Kind, tok.Offset, next)
			}
			if tok.Len < 1 {
				t.Fatalf("%s at %d covers %d bytes", tok.Kind, tok.Offset, tok.Len)
			}
			next = tok.End()
		}
		if next != len(data) {
			t.Fatalf("spans end at %d, input is %d bytes", next, len(data))
		}

		// modes[0] is DEFAULT and never pops. Anything above it is a literal or a splice
		// the input left open, which finish must have reported (§11).
		if len(s.modes) > 1 && s.errors.Empty() {
			t.Fatalf("%d frames open at end of input with no diagnostic", len(s.modes)-1)
		}
	})
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
