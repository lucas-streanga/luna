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
	"luna/oracle/diagnostic"
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
// drift. It reports whether the input reached the lexer: ingress is the lexer's contract and is
// fuzzed next door, and a helper that skipped here would skip its whole caller.
func checkShape(t *testing.T, src string, shape []byte) bool {
	t.Helper()

	f, err := source.New("fuzz.luna", src)
	if err != nil {
		return false
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
	return true
}

func FuzzSpliceBuild(f *testing.F) {
	for _, src := range shapeSources(f) {
		f.Add(src, []byte{0x03})
	}
	f.Fuzz(func(t *testing.T, src string, shape []byte) {
		if !checkShape(t, src, shape) {
			t.Skip() // it never reached the lexer, so it explored nothing here
		}
	})
}

// TestRandomShapes is the only exploration a plain `go test` performs: a fuzz target sees its seed
// corpus and nothing else unless -fuzz is passed. Sharing checkShape is what keeps them together.
func TestRandomShapes(t *testing.T) {
	r := rng(0x5eed) // fixed, so a reported failure reproduces by rerunning

	sources := shapeSources(t)
	for _, src := range sources {
		for range 2 {
			shape := drawShape(&r, src)
			if !checkShape(t, src, shape) {
				t.Fatalf("%q did not reach the lexer, and the corpus is all valid UTF-8", src)
			}
		}
	}

	// Bytes rather than Luna, for the path real source cannot reach: INVALID tokens, and a file
	// that is one long unterminated literal.
	lexed, garbage := 0, make([]byte, 32)
	for range randomGarbage {
		fillFrom(&r, garbage, garbageBytes)
		if checkShape(t, string(garbage), drawShape(&r, string(garbage))) {
			lexed++
		}
	}

	// Reported rather than assumed: most uniform byte strings are not valid UTF-8, and a suite
	// that quietly stopped reaching the lexer would still pass while exercising nothing.
	t.Logf("%d sources × 2 shapes, and %d of %d random byte strings reached the lexer",
		len(sources), lexed, randomGarbage)
	if lexed == 0 {
		t.Error("no random input reached the lexer; this half of the test checked nothing")
	}
}

// randomGarbage is how many byte strings each driver draws. Short and few: they are the cheap
// half, and the corpus above is where the structure is.
const randomGarbage = 200

// drawShape sizes the decision bytes to the source, because a fixed length quietly stops shaping
// anything. Two bytes are one decision and about five in eight of them consume a token, so 24
// bytes shape roughly seven — and 385 of the corpus's 431 blocks hold more tokens than that, with
// the rest force-consumed flat into whatever node was open. The fuzz target does not have this
// problem, since the fuzzer grows an input it gets coverage from; this driver has to be told.
func drawShape(r *rng, src string) []byte {
	shape := make([]byte, min(16+2*len(src), 2048))
	fill(r, shape)
	return shape
}

// --- the contract targets -----------------------------------------------------------------
//
// The other direction: bytes become an event stream with **no** well-formedness at all, so almost
// every input violates a precondition. What is asserted is therefore the contract rather than the
// tree — either an intentional `parser:` panic, or a value that holds up. A Go runtime error here
// is the finding: it means a precondition nobody checked.
//
// Only the invariants that survive an arbitrary stream are asserted. The tier that says the tree
// *is the file* is not: it answers to a parser having read one, and a decoder is not a parser.

// decodeEvents reads three bytes per event, spread so that every branch of the two contracts is
// reachable: an invalid event kind, a Kind that is a token's or Unset or past Error, and token
// indices that are negative, repeated, descending or past the end of the stream.
func decodeEvents(shape []byte) eventStream {
	out := make(eventStream, 0, len(shape)/3)
	for i := 0; i+2 < len(shape); i += 3 {
		out = append(out, event{
			kind: eventKind(shape[i] % 5), // 4 is no kind at all
			node: Kind(shape[i+1]),        // Unset, a token's, a node's, or past Error
			tok:  int(shape[i+2]) - 8,     // negative, in range, or past the end
		})
	}
	return out
}

// contractPanic reports whether f panicked as the contract requires. The type matters as much as
// the fact: an index-out-of-range here would be a precondition nobody wrote down.
func contractPanic(t *testing.T, f func()) (panicked bool) {
	t.Helper()
	defer func() {
		v := recover()
		if v == nil {
			return
		}
		bug, ok := v.(diagnostic.Bug)
		if !ok || !strings.HasPrefix(bug.Message, "parser: ") {
			t.Fatalf("panicked with %T (%v); a violation must be an intentional diagnostic.Bug",
				v, v)
		}
		panicked = true
	}()
	f()
	return
}

func checkBuildContract(t *testing.T, src string, shape []byte) bool {
	t.Helper()
	f, err := source.New("fuzz.luna", src)
	if err != nil {
		return false
	}
	tokens, _ := lexer.Lex(f)
	events := decodeEvents(shape)

	var tree *Tree
	if contractPanic(t, func() { tree = build(f, tokens, events) }) {
		return true
	}
	if reports := probe(func(r reporter) { assertTreeIsWellFormed(r, tree, src) }); len(reports) > 0 {
		t.Fatalf("source %q, shape %v:\n%s\n--- events\n%s--- arena\n%s",
			src, shape, strings.Join(reports, "\n"), events, dumpArena(tree))
	}
	return true
}

func checkSpliceContract(t *testing.T, src string, shape []byte) bool {
	t.Helper()
	f, err := source.New("fuzz.luna", src)
	if err != nil {
		return false
	}
	tokens, _ := lexer.Lex(f)
	events := decodeEvents(shape)

	var spliced eventStream
	if contractPanic(t, func() { spliced = splice(tokens, events) }) {
		return true
	}
	// Splice's own postconditions hold whatever it accepted. assertSpliceOnlyInserts is the one
	// left out: its oracle reads the first event as the root, which an arbitrary stream need not
	// make true, and the structured target checks it where that holds.
	reports := probe(func(r reporter) {
		assertIndexCoverage(r, tokens, spliced)
		assertTriviaIsNeverAtAnEventEdge(r, tokens, spliced)
	})

	// Accepting a stream is not vouching for it: splice never inspects kinds, and a second root
	// passes its balance check and fails the builder's.
	var tree *Tree
	if !contractPanic(t, func() { tree = build(f, tokens, spliced) }) {
		reports = append(reports,
			probe(func(r reporter) { assertTreeIsWellFormed(r, tree, src) })...)
	}
	if len(reports) > 0 {
		t.Fatalf("source %q, shape %v:\n%s\n--- events\n%s--- spliced\n%s--- arena\n%s",
			src, shape, strings.Join(reports, "\n"), events, spliced, dumpArena(tree))
	}
	return true
}

func FuzzBuildContract(f *testing.F) {
	for _, src := range shapeSources(f) {
		f.Add(src, []byte{0x00, 0x88, 0x08})
	}
	f.Fuzz(func(t *testing.T, src string, shape []byte) {
		if !checkBuildContract(t, src, shape) {
			t.Skip()
		}
	})
}

func FuzzSpliceContract(f *testing.F) {
	for _, src := range shapeSources(f) {
		f.Add(src, []byte{0x00, 0x88, 0x08})
	}
	f.Fuzz(func(t *testing.T, src string, shape []byte) {
		if !checkSpliceContract(t, src, shape) {
			t.Skip()
		}
	})
}

// TestRandomEvents drives both contracts on every `go test`, for the reason TestRandomShapes does.
func TestRandomEvents(t *testing.T) {
	r := rng(0xc0ffee)

	for _, src := range shapeSources(t) {
		shape := drawShape(&r, src)
		if !checkBuildContract(t, src, shape) || !checkSpliceContract(t, src, shape) {
			t.Fatalf("%q did not reach the lexer, and the corpus is all valid UTF-8", src)
		}
	}

	lexed, garbage := 0, make([]byte, 32)
	for range randomGarbage {
		fillFrom(&r, garbage, garbageBytes)
		shape := drawShape(&r, string(garbage))
		if checkBuildContract(t, string(garbage), shape) && checkSpliceContract(t, string(garbage), shape) {
			lexed++
		}
	}
	t.Logf("%d of %d random byte strings reached the lexer", lexed, randomGarbage)
	if lexed == 0 {
		t.Error("no random input reached the lexer; this half of the test checked nothing")
	}
}

// shapeSources is the corpus every driver draws on: the hazard cases, and every Luna block in the
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

// garbageBytes is the alphabet the random half draws its sources from. Uniform bytes would be
// worse than useless: almost none are valid UTF-8, so ingress rejects them and nothing here runs
// — which is what the count in each driver exists to catch. Drawing from ASCII the lexer branches
// on gets past ingress and into real token streams, INVALID ones included.
const garbageBytes = " \t\n;{}()[]<>=+-*/%!?.,:&|@#$\\\"'`~_0123456789abcxyz" +
	"letconstfnmatchimporttest"

func fillFrom(r *rng, buf []byte, alphabet string) {
	for i := range buf {
		buf[i] = alphabet[r.next()%uint64(len(alphabet))]
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
