// The golden gate: every `.parse` case, against grammar.md.
//
// Two things are checked per case and they are different in kind. The **grammar** must derive
// the source exactly once — that is the ambiguity stress test, and it is what these cases were
// written for. The **tree** must match what grammar.md's own productions yield, which is a
// golden in the ordinary sense, and it exists now so that this package's parser has a target
// written from the spec rather than from itself.
package parser_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"luna/internal/ebnf"
	"luna/oracle/parser"
)

var update = flag.Bool("update", false, "rewrite each golden's tree section from grammar.md")

const testdataDir = "testdata"

// caseFloor guards the reader rather than the cases. A walk that silently found nothing would
// make every assertion below vacuous, which is the same fail-open internal/spec arms itself
// against; the number is a floor, so adding cases needs no edit here.
const caseFloor = 20

func TestGoldens(t *testing.T) {
	g, err := ebnf.Load()
	if err != nil {
		t.Fatalf("loading grammar.md: %v", err)
	}
	cases, err := parser.ReadGoldenDir(testdataDir)
	if err != nil {
		t.Fatalf("reading %s: %v", testdataDir, err)
	}
	if len(cases) < caseFloor {
		t.Fatalf("found %d goldens, expected at least %d; the reader is not reaching them",
			len(cases), caseFloor)
	}

	for _, c := range cases {
		t.Run(c.Name(), func(t *testing.T) {
			lx, err := parser.LexGolden(c.Name()+".luna", c.Source)
			if err != nil {
				t.Fatalf("lexing: %v", err)
			}
			res := g.Recognize(lx.Input)

			if c.ExpectsDiagnostics() {
				if res.Accepted {
					t.Errorf("case is filed under %s but the grammar derives it",
						parser.GoldenErrorDir)
				}
				return
			}

			if !res.Accepted {
				t.Fatalf("the grammar does not derive this: %s", res.Explain(lx.Input))
			}
			if res.Ambiguous {
				t.Fatalf("AMBIGUOUS: derives more than one way")
			}

			tree, err := parser.GoldenTree(g, c)
			if err != nil {
				t.Fatalf("deriving a tree: %v", err)
			}
			if *update {
				c.Tree, c.HasTree = tree, true
				if err := os.WriteFile(c.Path, c.Bytes(), 0o644); err != nil {
					t.Fatalf("writing: %v", err)
				}
				return
			}
			if !c.HasTree {
				t.Fatalf("no tree section; run `go test ./oracle/parser -update` and read the diff")
			}
			if tree != c.Tree {
				t.Errorf("tree does not match grammar.md:\n--- want (the file)\n%s\n--- got (the grammar)\n%s",
					c.Tree, tree)
			}
		})
	}
}

// TestGoldenRootSpansTheFile pins the one span a derivation cannot supply: File owns the file's
// leading and trailing trivia, so its span is the file (golden.md §1).
//
// It reads the committed tree section rather than the renderer's output, or it would be a check
// the renderer passes by construction. Once Parse produces these trees it catches a builder
// that gets the root's extent wrong, which no other line in a golden can see.
func TestGoldenRootSpansTheFile(t *testing.T) {
	cases, err := parser.ReadGoldenDir(testdataDir)
	if err != nil {
		t.Fatalf("reading %s: %v", testdataDir, err)
	}
	if len(cases) < caseFloor {
		t.Fatalf("found %d goldens, expected at least %d; the reader is not reaching them",
			len(cases), caseFloor)
	}
	for _, c := range cases {
		if !c.HasTree {
			continue
		}
		first, _, _ := strings.Cut(c.Tree, "\n")
		if want := fmt.Sprintf("File 0..%d", len(c.Source)); first != want {
			t.Errorf("%s: the tree starts %q, want %q", c.Name(), first, want)
		}
	}
}

// TestGoldenFilesRoundTrip: the reader and the writer must agree, or `-update` would rewrite
// every case the moment it touched one.
func TestGoldenFilesRoundTrip(t *testing.T) {
	cases, err := parser.ReadGoldenDir(testdataDir)
	if err != nil {
		t.Fatalf("reading %s: %v", testdataDir, err)
	}
	for _, c := range cases {
		raw, err := os.ReadFile(c.Path)
		if err != nil {
			t.Fatalf("reading %s: %v", c.Path, err)
		}
		if got := string(c.Bytes()); got != string(raw) {
			t.Errorf("%s does not round-trip through the reader", filepath.Base(c.Path))
		}
	}
}
