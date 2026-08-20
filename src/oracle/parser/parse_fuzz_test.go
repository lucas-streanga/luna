// The parser's fuzz target: bytes become source, and everything the pass promises of any input
// at all is asserted over them.
//
// The two targets in fuzz_test.go feed the builder a synthetic event stream, so neither has ever
// run the parser. This is the only instrument that can see a parser bug, which is why §7.8 names
// it: arbitrary bytes reach states no corpus contains, so an invariant that is merely
// conventional rather than structural is found here instead of by a user.
//
// It asserts structure, never meaning. Whether a tree is *right* is the goldens' question and
// needs an expectation written for it; whether it is well formed, covers the file, and comes with
// diagnostics a renderer can draw needs none, which is what lets this run on inputs nobody wrote.
// The reject-set invariant is deliberately absent: it would cost an Earley parse per exec, and
// the direction worth having is already free in TestSpecCorpusThroughParse.
//
// In-package because the unspliced stream is (§4.1).
package parser

import (
	"strings"
	"testing"

	"luna/oracle/diagnostic"
	"luna/oracle/lexer"
	"luna/oracle/source"
	"luna/oracle/token"
)

// parseRun keeps every stage, because splice's contract is a relation between two streams rather
// than a property of either, and a caller holding only the tree could not check it.
type parseRun struct {
	file      *source.File
	tokens    []token.Token
	unspliced eventStream
	events    eventStream
	tree      *Tree
	diags     []diagnostic.Diagnostic
}

// runParse drives one lexed source through parse, splice and build.
//
// **Nil is pending, and pending is not a skip.** The recovered value must be a scaffold sentinel
// whose name `scaffoldStubs` still agrees is unwritten, so a body that panics with a stale one
// fails loudly and every caller here becomes a live assertion the moment the table empties. The
// name is not pinned to `parse`, since Phase 2 lands bodies one nonterminal at a time and a
// target unusable until the last of them is a target nobody runs.
//
// Anything else propagates, which is what makes it a finding. A Bug carries a stack, and §7.8's
// claim is that no input reaches one, so catching it here would discard the report and answer the
// question in the same move.
func runParse(f *source.File, tokens []token.Token) (run *parseRun) {
	defer func() {
		v := recover()
		if v == nil {
			return
		}
		if name, ok := scaffoldPanic(v); ok && scaffoldStubs[name] {
			run = nil
			return
		}
		panic(v)
	}()

	events, diags := parse(f, tokens)
	spliced := splice(tokens, events)
	return &parseRun{
		file:      f,
		tokens:    tokens,
		unspliced: events,
		events:    spliced,
		tree:      build(f, tokens, spliced),
		diags:     diags,
	}
}

// checkParse is the property, shared by the fuzz target and the random driver so the two cannot
// drift. It reports whether the input reached the parser: ingress is source.New's contract and is
// fuzzed next door, and a helper that skipped here would skip its whole caller.
func checkParse(t *testing.T, src string) bool {
	t.Helper()

	f, err := source.New("fuzz.luna", src)
	if err != nil {
		return false
	}
	// A lexical failure is not a reason to stop. An INVALID token is a real token that tiles the
	// file, and §6.5, the parser running over a file that failed to lex, is reachable no other
	// way in this suite.
	tokens, _ := lexer.Lex(f)

	run := runParse(f, tokens)
	if run == nil {
		return true
	}

	reports := probe(func(r reporter) {
		assertSpliceInvariants(r, run.tokens, run.unspliced, run.events)
		assertTreeInvariants(r, run.tree, run.tokens, src)
		assertDiagnosticsAreRenderable(r, run.diags, src)
	})
	if len(reports) > 0 {
		t.Fatalf("source %q:\n%s\n--- events\n%s--- arena\n%s",
			src, strings.Join(reports, "\n"), run.events, dumpArena(run.tree))
	}
	return true
}

// assertDiagnosticsAreRenderable is the one property the battery does not hold, because the
// battery is about the tree and this is about the other return. A code with no title names no
// §11.2 row, a code from another stage means the parser reported what it does not own (§6.5), and
// a span outside the file is one no renderer can put a caret under.
//
// Zero length is not checked: §6.4 leaves the caret convention for an absent token to the
// diagnostic layer, and pinning it here would decide it by accident.
func assertDiagnosticsAreRenderable(t reporter, diags []diagnostic.Diagnostic, src string) {
	t.Helper()
	for i := range diags {
		d := &diags[i]
		if err := d.Validate(); err != nil {
			t.Errorf("diagnostic %d: %v", i, err)
			continue
		}
		if got := d.Code.Stage(); got != diagnostic.Parse {
			t.Errorf("diagnostic %d is %s, and the parser raises P codes alone", i, d.Code)
		}
		if d.Primary.Offset < 0 || d.Primary.End() > len(src) {
			t.Errorf("%s spans %d..%d, outside [0, %d]",
				d.Code, d.Primary.Offset, d.Primary.End(), len(src))
		}
	}
}

func FuzzParse(f *testing.F) {
	for _, src := range shapeSources(f) {
		f.Add(src)
	}
	f.Fuzz(func(t *testing.T, src string) {
		if !checkParse(t, src) {
			t.Skip() // it never reached the lexer, so it explored nothing here
		}
	})
}

// TestRandomParse is the only exploration a plain `go test` performs: a fuzz target sees its seed
// corpus and nothing else unless -fuzz is passed. Sharing checkParse is what keeps them together.
func TestRandomParse(t *testing.T) {
	r := rng(0x5eed) // fixed, so a reported failure reproduces by rerunning

	// While the scaffold stands, every case below is pending and reports nothing. Said out loud,
	// because a run that found nothing and a run that checked nothing read identically otherwise.
	if lexed, err := LexGolden("pending.luna", "x;"); err == nil {
		if runParse(lexed.File, lexed.Tokens) == nil {
			t.Logf("pending: %d stubs remain, so nothing below asserts yet", len(scaffoldStubs))
		}
	}

	sources := shapeSources(t)
	for _, src := range sources {
		if !checkParse(t, src) {
			t.Fatalf("%q did not reach the parser, and the corpus is all valid UTF-8", src)
		}
	}

	// Bytes rather than Luna, for what the corpus cannot reach: every seed above derives and
	// error_producing/ is empty, so this is the suite's only ungrammatical input.
	reached, garbage := 0, make([]byte, 32)
	for range randomGarbage {
		fillFrom(&r, garbage, garbageBytes)
		if checkParse(t, string(garbage)) {
			reached++
		}
	}

	// Reported rather than assumed: most uniform byte strings are not valid UTF-8, and a driver
	// that quietly stopped reaching the parser would still pass while exercising nothing.
	t.Logf("%d sources, and %d of %d random byte strings reached the parser",
		len(sources), reached, randomGarbage)
	if reached == 0 {
		t.Error("no random input reached the parser; this half of the test checked nothing")
	}
}
