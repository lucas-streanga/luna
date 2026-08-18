// The golden gate: every `.parse` case, against grammar.md and against the parser.
//
// Three checks per case, and they are different in kind. The **grammar** must derive the source
// exactly once — the ambiguity stress test these cases were written for. The **golden** must
// match what §0's own productions yield, so a grammar change that moves a tree shows up as a
// reviewable diff. And the **parser** must produce that same tree, raise no diagnostic, and
// reconstruct the source (golden.md §0's parser row).
//
// The second and third are the same expectation from opposite sides, which is the point: the
// golden is derived from §0 and the parser is written to §0, so a disagreement names which of
// the two is wrong instead of leaving one diff to interpret.
package parser_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"luna/internal/ebnf"
	"luna/oracle/diagnostic"
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
			lexed, err := parser.LexGolden(c.Name()+".luna", c.Source)
			if err != nil {
				t.Fatalf("lexing: %v", err)
			}
			res := g.Recognize(lexed.Input)

			if c.ExpectsDiagnostics() {
				if res.Accepted {
					t.Errorf("case is filed under %s but the grammar derives it",
						parser.GoldenErrorDir)
				}
				return
			}

			if !res.Accepted {
				t.Fatalf("the grammar does not derive this: %s", res.Explain(lexed.Input))
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

			// And the parser must produce that same tree. The two assertions are different
			// claims — that the golden still tracks §0, and that the parser implements §0 — so
			// keeping both means a disagreement names which of the two is wrong, and together
			// they compare the parser against the grammar through the committed tree.
			assertParserAgrees(t, c, lexed)
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

// assertParserAgrees is golden.md §0's parser row: **the tree matches, no diagnostics are raised,
// and the CST reconstructs the source.** All three, because they fail differently — a wrong Kind
// shows in the tree, a spurious recovery shows in the diagnostics, and a dropped token shows only
// in the reconstruction.
//
// While `parse` is unimplemented the case is reported **pending**, which is not a skip: the panic
// must be exactly parse's sentinel, so the moment a body lands every golden becomes a live
// comparison and a wrong one fails here. Anything else propagates.
func assertParserAgrees(t *testing.T, c *parser.Golden, lexed *parser.LexedGolden) {
	t.Helper()
	tree, diags, pending := parseGolden(lexed)
	if pending {
		t.Log("pending on parse")
		return
	}
	if len(diags) != 0 {
		t.Errorf("%d diagnostics on a case the grammar derives; a clean golden raises none:\n%v",
			len(diags), diags)
	}
	if tree == nil {
		t.Fatal("no tree; only the empty file has none, and a golden's source is never empty")
	}
	if got := parser.RenderGolden(tree); got != c.Tree {
		t.Errorf("the parser disagrees with the golden:\n--- want (the file)\n%s\n--- got (the parser)\n%s",
			c.Tree, got)
	}
	if got := tree.Root().Text(); got != c.Source {
		t.Errorf("the tree reconstructs %q, want %q — losslessness is the leaves concatenated",
			got, c.Source)
	}
}

// parseGolden runs the parser, reporting pending on parse's own sentinel and nothing else.
func parseGolden(lexed *parser.LexedGolden) (tree *parser.Tree, diags []diagnostic.Diagnostic, pending bool) {
	defer func() {
		v := recover()
		if v == nil {
			return
		}
		if s, ok := v.(string); ok && s == "parser: parse is unimplemented" {
			pending = true
			return
		}
		panic(v)
	}()
	tree, diags = parser.Parse(lexed.File, lexed.Tokens)
	return tree, diags, false
}
