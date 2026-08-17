// The corpus through the machinery, rather than through the renderer alone: these are the
// assertions no golden can display (§2.3), run over every case rather than a chosen sample.
// In-package because the event stream is (§4.1).
package parser

import "testing"

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
