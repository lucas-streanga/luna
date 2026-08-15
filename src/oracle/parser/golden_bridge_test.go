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
			assertIndexCoverage(t, run.lex.Toks, run.evs)
			assertReconstructs(t, run.tree, c.Source)
			assertTriviaIsNeverAtAnEdge(t, run.tree)
		})
	}
}

// assertReconstructs is tooling §2's losslessness, stated as one comparison. It holds only
// because trivia are nodes rather than attachments (§2), and it is what makes the File line's
// span real rather than a claim the renderer prints about itself.
func assertReconstructs(t *testing.T, tr *Tree, src string) {
	t.Helper()
	if tr == nil {
		t.Fatalf("no tree for %d bytes of source", len(src))
	}
	if got := leafText(tr); got != src {
		t.Errorf("the leaves reconstruct %q, want %q", got, src)
	}
	if o, e := tr.Root().Span(); o != 0 || e != len(src) {
		t.Errorf("the root spans %d..%d, want 0..%d", o, e, len(src))
	}
}

// assertTriviaIsNeverAtAnEdge is §2.3's other half, and the half index coverage cannot see: a
// comment placed in the wrong node still preserves order and still reconstructs.
//
// The invariant is what keeps inner spans tight. A node whose first child were a comment would
// start at that comment, and getting a tight span back would need Roslyn's Span/FullSpan split —
// two accessors on every node, forever. File is the exception because the file's own leading and
// trailing trivia have nowhere further out to go.
func assertTriviaIsNeverAtAnEdge(t *testing.T, tr *Tree) {
	t.Helper()
	for id := range tr.Len() {
		n := tr.At(NodeID(id))
		kids := n.Children()
		if len(kids) == 0 || n.Kind() == File {
			continue
		}
		for i, edge := range []Node{kids[0], kids[len(kids)-1]} {
			if !isTrivia(edge.Kind()) {
				continue
			}
			where := "first"
			if i == 1 {
				where = "last"
			}
			o, e := edge.Span()
			t.Errorf("%s at %d..%d is the %s child of %s: trivia belongs to the node that was "+
				"already open, not to the one it abuts", edge.Kind(), o, e, where, n.Kind())
		}
	}
}
