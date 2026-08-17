// The two corpora through the machinery, rather than through the renderer alone: these are the
// assertions no golden can display (§2.3), run over every case rather than a chosen sample.
// In-package because the event stream is (§4.1).
package parser

import (
	"fmt"
	"testing"

	"luna/internal/spec"
)

// corpusFloor guards the reader, not the corpus: a walk that found nothing would make everything
// below vacuous.
const corpusFloor = 20

// Named here because golden_test.go's constant belongs to the other package.
const goldenDir = "testdata"

func TestGoldenCorpusInvariants(t *testing.T) {
	g := loadGrammar(t)
	cases, err := ReadGoldenDir(goldenDir)
	if err != nil {
		t.Fatalf("reading %s: %v", goldenDir, err)
	}
	if len(cases) < corpusFloor {
		t.Fatalf("found %d goldens, expected at least %d; the reader is not reaching them",
			len(cases), corpusFloor)
	}

	for _, c := range cases {
		if c.ExpectsDiagnostics() {
			continue // no derivation, so nothing to build from until the parser exists
		}
		t.Run(c.Name(), func(t *testing.T) {
			run, err := runGolden(g, c)
			if err != nil {
				t.Fatalf("deriving and building: %v", err)
			}
			assertSpliceInvariants(t, run.lexed.Tokens, run.unspliced, run.events)
			assertTreeInvariants(t, run.tree, run.lexed.Tokens, c.Source)
		})
	}
}

// specCorpusFloor is internal/ebnf's, for the same reason: a walk that found nothing would leave
// everything below vacuous.
const specCorpusFloor = 400

// TestSpecCorpusInvariants runs every Luna block in the spec through the machinery. It is the free
// coverage golden.md §0 promised — 431 files nobody wrote for the parser, against properties
// rather than shapes, which is what lets them be used without expectations.
//
// What it adds over the fuzz drivers, which already lex these same sources, is **grammatical**
// nesting: the elision golden.md §2 describes, applied to 431 files rather than the 30 hazard
// cases, and span arithmetic over the deep right-recursion the precedence tiers produce.
//
// A block that stops deriving is skipped rather than failed. The grammar's health is
// TestCorpusParses's assertion next door, and reporting it twice from two packages is noise; the
// count below is what keeps the skip from eating the test.
//
// When Parse lands this loses its Earley half and keeps everything else.
func TestSpecCorpusInvariants(t *testing.T) {
	g := loadGrammar(t)
	blocks, err := spec.LunaBlocks()
	if err != nil {
		t.Fatalf("reading the spec corpus: %v", err)
	}
	if len(blocks) < specCorpusFloor {
		t.Fatalf("found %d blocks, expected at least %d; the corpus walk is broken",
			len(blocks), specCorpusFloor)
	}

	derived, nodes := 0, 0
	for _, b := range blocks {
		run, err := runSource(g, "spec.luna", b.Source)
		if err != nil {
			continue // reported by internal/ebnf's TestCorpusParses
		}
		derived++
		if run.tree != nil {
			nodes += run.tree.Len()
		}
		t.Run(fmt.Sprintf("%s:%d", b.Path, b.Line), func(t *testing.T) {
			assertSpliceInvariants(t, run.lexed.Tokens, run.unspliced, run.events)
			assertTreeInvariants(t, run.tree, run.lexed.Tokens, b.Source)
		})
	}

	t.Logf("%d of %d blocks derived, %d nodes built", derived, len(blocks), nodes)
	if derived < specCorpusFloor {
		t.Errorf("only %d blocks derived: the machinery is being checked against almost nothing",
			derived)
	}
}
