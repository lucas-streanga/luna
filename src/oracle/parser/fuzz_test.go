// The structured fuzz target: bytes drive a generator that only ever emits a **well-formed**
// event stream, so every input reaches build and the assertion is the whole battery.
//
// It models the event contract rather than the grammar, deliberately. Recovery will emit shapes no
// production describes — empty nodes, Error over garbage, adjacent synthesised leaves — and those
// are what the builder is least tested on. Nothing here checks a tree is *right*; that is the
// goldens' job, and the division is what lets this one run on inputs nobody wrote.
//
// In-package because the event stream is (§4.1).
package parser

import (
	"strings"
	"testing"

	"luna/internal/spec"
	"luna/oracle/lexer"
	"luna/oracle/source"
	"luna/oracle/token"
)

// The generator's two menus. Error is in both: §6.2 opens it over tokens nobody could place, and
// §6.1 synthesises it where a construct is absent.
var (
	shapeNodes  = AllNodes()
	shapeLeaves = synthesisableKinds()
)

func synthesisableKinds() []Kind {
	out := []Kind{Error}
	for _, tk := range token.All() {
		if k := Kind(tk); isSynthesisable(k) {
			out = append(out, k)
		}
	}
	return out
}

// shaper hands out decision bytes, and zero once the input runs dry.
type shaper struct {
	data []byte
	pos  int
}

func (s *shaper) next() (byte, bool) {
	if s.pos >= len(s.data) {
		return 0, false
	}
	s.pos++
	return s.data[s.pos-1], true
}

// shapeMaxDepth keeps a blob of nothing but opens from eating memory.
const shapeMaxDepth = 32

// shapeEvents turns decision bytes into a stream the parser could plausibly have emitted:
// balanced, consuming every non-trivia token once and in order, with kinds valid for their event.
// Those are splice's preconditions, so anything this produces is in contract by construction and
// a panic from the passes below is a finding rather than a bad input.
//
// Consumption is weighted heavily; an action that cannot be taken falls through to another. Empty
// nodes and deep nesting arrive on their own out of the walk.
func shapeEvents(tokens []token.Token, shape []byte) eventStream {
	events := eventStream{openEv(File)}

	// §6.1: an empty file is File opened and closed with nothing between, so the parser has
	// nothing to synthesise into either. Modelling that keeps the iff assertable rather than
	// making this generator the one thing that violates it.
	if len(tokens) == 0 {
		return append(events, closeEv)
	}

	indices := filtered(tokens)
	s := shaper{data: shape}
	depth, next := 1, 0
	for {
		b, ok := s.next()
		if !ok {
			break
		}
		arg, _ := s.next() // the kind selector; zero at the end, where the loop stops anyway

		// Sequential rather than a switch: an action rewritten because it was unavailable has to
		// be re-examined, or a blocked close falls through to consuming a token that is not there.
		action := b % 8
		if action == 0 && depth >= shapeMaxDepth {
			action = 3 // too deep to open
		}
		if action == 1 && depth == 1 {
			action = 3 // nothing but the root is open
		}
		if action >= 3 && next == len(indices) {
			action = 2 // nothing left to consume, so synthesise
			if depth > 1 {
				action = 1
			}
		}

		switch action {
		case 0:
			events = append(events, openEv(shapeNodes[int(arg)%len(shapeNodes)]))
			depth++
		case 1:
			events = append(events, closeEv)
			depth--
		case 2:
			events = append(events, missingEv(shapeLeaves[int(arg)%len(shapeLeaves)]))
		default:
			events = append(events, tokEv(indices[next]))
			next++
		}
	}

	for ; next < len(indices); next++ {
		events = append(events, tokEv(indices[next]))
	}
	for ; depth > 1; depth-- {
		events = append(events, closeEv)
	}
	return append(events, closeEv)
}

// checkShape is the property, shared by the fuzz target and the random driver so the two cannot
// drift.
func checkShape(t *testing.T, src string, shape []byte) {
	t.Helper()

	f, err := source.New("fuzz.luna", src)
	if err != nil {
		t.Skip() // ingress is the lexer's contract, and fuzzed next door
	}
	// Lexical diagnostics are *not* a reason to stop. An INVALID token is a real token that tiles
	// the file, and §6.5 — the parser running on a file that failed to lex — is reachable no other
	// way in this suite.
	tokens, _ := lexer.Lex(f)

	generated := shapeEvents(tokens, shape)
	spliced := splice(tokens, generated)
	tree := build(f, tokens, spliced)

	reports := probe(func(r reporter) {
		assertSpliceInvariants(r, tokens, generated, spliced)
		assertTreeInvariants(r, tree, tokens, src)
	})
	if len(reports) > 0 {
		t.Fatalf("source %q, shape %v:\n%s\n--- events\n%s--- arena\n%s",
			src, shape, strings.Join(reports, "\n"), spliced, dumpArena(tree))
	}
}

func FuzzSpliceBuild(f *testing.F) {
	for _, src := range shapeSources(f) {
		f.Add(src, []byte{0x03})
	}
	f.Fuzz(checkShape)
}

// TestRandomShapes is the only exploration a plain `go test` performs: a fuzz target sees its seed
// corpus and nothing else unless -fuzz is passed. Sharing checkShape is what keeps them together.
func TestRandomShapes(t *testing.T) {
	r := rng(0x5eed) // fixed, so a reported failure reproduces by rerunning
	shape := make([]byte, 24)

	for _, src := range shapeSources(t) {
		for range 2 {
			fill(&r, shape)
			checkShape(t, src, shape)
		}
	}

	// Bytes rather than Luna, for the path real source cannot reach: INVALID tokens, and a file
	// that is one long unterminated literal.
	garbage := make([]byte, 32)
	for range 200 {
		fill(&r, garbage)
		fill(&r, shape)
		checkShape(t, string(garbage), shape)
	}
}

// shapeSources is the corpus both drivers draw on: the hazard cases, and every Luna block in the
// spec. Random bytes rarely lex to anything with structure, so the seeds are where the depth is.
func shapeSources(tb testing.TB) []string {
	tb.Helper()

	cases, err := ReadGoldenDir(goldenDir)
	if err != nil {
		tb.Fatalf("reading %s: %v", goldenDir, err)
	}
	blocks, err := spec.LunaBlocks()
	if err != nil {
		tb.Fatalf("reading the spec corpus: %v", err)
	}
	if len(cases) < corpusFloor || len(blocks) < 400 {
		tb.Fatalf("found %d goldens and %d blocks; the readers are not reaching them",
			len(cases), len(blocks))
	}

	out := make([]string, 0, len(cases)+len(blocks))
	for _, c := range cases {
		out = append(out, c.Source)
	}
	for _, b := range blocks {
		out = append(out, b.Source)
	}
	return out
}

func fill(r *rng, buf []byte) {
	for i := range buf {
		buf[i] = byte(r.next())
	}
}

// rng is splitmix64, for the reason oracle/lexer/random_test.go writes it out: the linter forbids
// math/rand in tests, and its sequence is not promised across Go releases.
type rng uint64

func (r *rng) next() uint64 {
	*r += 0x9e3779b97f4a7c15
	z := uint64(*r)
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}
