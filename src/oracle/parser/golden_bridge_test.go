// The corpus, run through the machinery rather than through the renderer alone.
//
// `golden_test.go` compares the rendered tree against each committed file, which is the
// expectation grammar.md wrote. These are the assertions no golden can display (§2.3): where the
// trivia went, and that the leaves still are the file. They run over every case rather than a
// chosen sample, and they are in-package because the event stream is (§4.1).
package parser

import "testing"

// corpusFloor guards the reader, not the corpus: a walk that silently found nothing would make
// everything below vacuous. golden_test.go's own floor says the same thing from the other
// package.
const corpusFloor = 20

// goldenDir is testdata, named here because golden_test.go's constant belongs to the other
// package.
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
