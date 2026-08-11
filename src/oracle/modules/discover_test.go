package modules_test

import (
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"luna/oracle/modules"
)

// External tests: these reach only Discover and the three result types, which is what a
// caller gets. Nothing here needs the prelude reader, so nothing here should see it.

// mark points at the byte where the prelude is expected to end. It is stripped before the
// source reaches Discover, so its index *is* the expected offset — which beats counting
// bytes by hand and keeps a case readable when its source changes.
//
// A source with no mark expects the prelude to run to end of file.
const mark = "‸"

func split(src string) (clean string, preludeEnd int) {
	i := strings.Index(src, mark)
	if i < 0 {
		return src, len(src)
	}
	return strings.Replace(src, mark, "", 1), i
}

// run discovers over an in-memory tree. Sources are passed through split, so any of them may
// carry a mark.
func run(t *testing.T, entry string, files map[string]string) modules.Result {
	t.Helper()
	fsys := fstest.MapFS{}
	for path, src := range files {
		clean, _ := split(src)
		fsys[path] = &fstest.MapFile{Data: []byte(clean)}
	}
	res, err := modules.Discover(fsys, entry)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	return res
}

func paths(r modules.Result) []string {
	out := make([]string, 0, len(r.Files))
	for _, f := range r.Files {
		out = append(out, f.Path)
	}
	return out
}

// edges renders the edge list. The root module's empty path prints as (root), so a failure
// does not show a bare arrow.
func edges(r modules.Result) []string {
	out := make([]string, 0, len(r.Edges))
	for _, e := range r.Edges {
		from := e.From
		if from == "" {
			from = "(root)"
		}
		out = append(out, from+"->"+e.To)
	}
	return out
}

func equal(t *testing.T, what string, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("%s:\n got %q\nwant %q", what, got, want)
	}
}

// TestImportForms covers §5's grid (R136) and the two spellings R250 added to the prelude.
// Every case must yield exactly one edge to `dep`, however it is written.
func TestImportForms(t *testing.T) {
	for _, tc := range []struct {
		name, src string
	}{
		{"bare", "import dep;"},
		{"selective", "import { a, b } from dep;"},
		{"selective alias", "import { a as b } from dep;"},
		{"selective trailing comma", "import { a, b, } from dep;"},
		// R223: `from` is contextual, not reserved, so it stays usable as a binding name.
		{"alias named from", "import { a as from } from dep;"},
		{"assigned", "const d = import dep;"},
		{"assigned selective", "const d = import { a } from dep;"},
		{"exported assigned", "export const d = import dep;"},
		{"exported assigned selective", "export const d = import { a } from dep;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := run(t, "app.luna", map[string]string{
				"app.luna": tc.src + "\n",
				"dep.luna": "",
			})
			equal(t, "edges", edges(res), []string{"(root)->dep"})
			equal(t, "files", paths(res), []string{"app.luna", "dep.luna"})
		})
	}
}

// TestPreludeEnd pins where discovery stops. The assigned forms are the reason this is a
// parse decision rather than a token test, so `const` appears on both sides of the line.
func TestPreludeEnd(t *testing.T) {
	for _, tc := range []struct {
		name, src string
	}{
		{"declaration ends it", "import a;\n‸let x = 1;\n"},
		{"const that is not an import ends it", "import a;\n‸const n = 5;\n"},
		{"const that is an import does not", "const d = import a;\n‸let x = 1;\n"},
		{"export const import does not", "export const d = import a;\n‸fn f() {}\n"},
		{"comments and blank lines do not", "import a;\n\n// note\n/* note */\n\n‸fn f() {}\n"},
		{"no imports at all", "‸fn main() {}\n"},
		{"empty file", "‸"},
		{"all imports, no marker", "import a;\n"},
		{"all imports with trailing trivia", "import a;\n\n// tail\n"},
		// A malformed import is not an import: the prelude ends where the item began, and
		// §1.3 raises the syntax error.
		{"missing path", "‸import ;\n"},
		{"missing semicolon", "‸import a\n"},
		{"missing from", "‸import { a } dep;\n"},
		{"unclosed brace", "‸import { a\n"},
		{"trailing dot", "‸import a.;\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, want := split(tc.src)
			res := run(t, "app.luna", map[string]string{"app.luna": tc.src, "a.luna": ""})
			if len(res.Files) == 0 {
				t.Fatal("entry was not discovered")
			}
			if got := res.Files[0].PreludeEnd; got != want {
				clean, _ := split(tc.src)
				t.Errorf("PreludeEnd = %d, want %d\nsource: %q\nstops before: %q",
					got, want, clean, clean[min(got, len(clean)):])
			}
		})
	}
}

// TestDecoysAreNotImports is R190's first soundness rule observed from outside: discovery
// shares the real lexer, so an `import` that is not a token never becomes an edge.
func TestDecoysAreNotImports(t *testing.T) {
	const src = `// import decoy.line;
import real.one;
/* import decoy.block;
   import decoy.block2; */
import real.two;
let s = "import decoy.string;";
let c = ` + "`import decoy.command;`" + `;
let r = ~"import decoy.regex;";
import decoy.late;
`
	res := run(t, "app.luna", map[string]string{
		"app.luna":      src,
		"real/one.luna": "",
		"real/two.luna": "",
	})
	// The comment decoys are skipped as trivia and the prelude continues past them; the rest
	// sit after the prelude, so discovery never reads them at all — including the late import,
	// which is §1.2's error to raise, not discovery's to find.
	equal(t, "edges", edges(res), []string{"(root)->real.one", "(root)->real.two"})
	equal(t, "files", paths(res), []string{"app.luna", "real/one.luna", "real/two.luna"})
}

func TestGraphShape(t *testing.T) {
	t.Run("transitive, breadth-first, entry first", func(t *testing.T) {
		res := run(t, "app.luna", map[string]string{
			"app.luna": "import b;\nimport c;\n",
			"b.luna":   "import d;\n",
			"c.luna":   "",
			"d.luna":   "",
		})
		equal(t, "files", paths(res), []string{"app.luna", "b.luna", "c.luna", "d.luna"})
	})

	t.Run("diamond reads the shared module once", func(t *testing.T) {
		res := run(t, "app.luna", map[string]string{
			"app.luna": "import b;\nimport c;\n",
			"b.luna":   "import d;\n",
			"c.luna":   "import d;\n",
			"d.luna":   "",
		})
		equal(t, "files", paths(res), []string{"app.luna", "b.luna", "c.luna", "d.luna"})
		// Both edges survive: §1.2 needs them to see the shape, even though d was read once.
		equal(t, "edges", edges(res), []string{"(root)->b", "(root)->c", "b->d", "c->d"})
	})

	t.Run("cycle terminates and keeps both edges", func(t *testing.T) {
		res := run(t, "app.luna", map[string]string{
			"app.luna": "import b;\n",
			"b.luna":   "import app;\n",
		})
		equal(t, "files", paths(res), []string{"app.luna", "b.luna"})
		equal(t, "edges", edges(res), []string{"(root)->b", "b->app"})
	})

	t.Run("self-import terminates", func(t *testing.T) {
		res := run(t, "app.luna", map[string]string{"app.luna": "import app;\n"})
		equal(t, "files", paths(res), []string{"app.luna"})
		equal(t, "edges", edges(res), []string{"(root)->app"})
	})
}

// TestModulePaths covers modules §3: a path maps to the filesystem, and the entry is the root
// module with the empty path.
func TestModulePaths(t *testing.T) {
	res := run(t, "app.luna", map[string]string{
		"app.luna":       "import one;\nimport a.b.c;\n",
		"one.luna":       "",
		"a/b/c.luna":     "",
		"unreached.luna": "",
	})

	want := []struct{ path, module string }{
		{"app.luna", ""},
		{"one.luna", "one"},
		{"a/b/c.luna", "a.b.c"},
	}
	if len(res.Files) != len(want) {
		t.Fatalf("files = %q, want %d", paths(res), len(want))
	}
	for i, w := range want {
		if got := res.Files[i]; got.Path != w.path || got.Module != w.module {
			t.Errorf("file %d = {%q %q}, want {%q %q}", i, got.Path, got.Module, w.path, w.module)
		}
	}
}

// TestReservedRoot pins modules §10. The `std/io.luna` file exists on purpose: it proves the
// edge is unfollowed by rule rather than merely unresolved.
func TestReservedRoot(t *testing.T) {
	res := run(t, "app.luna", map[string]string{
		"app.luna":      "import std.io;\nimport std;\nimport standard;\n",
		"std/io.luna":   "import should.not.be.read;\n",
		"standard.luna": "",
	})
	equal(t, "edges", edges(res), []string{"(root)->std.io", "(root)->std", "(root)->standard"})
	// `standard` is followed: the reservation is `std` and paths beneath it, not a prefix match
	// on the spelling.
	equal(t, "files", paths(res), []string{"app.luna", "standard.luna"})
}

// TestMissingImportIsSkipped is the no-diagnostics contract (R250): discovery loses the file
// but keeps the edge, so §1.2 still has what it needs to report the unresolved import.
func TestMissingImportIsSkipped(t *testing.T) {
	res := run(t, "app.luna", map[string]string{
		"app.luna":  "import ghost;\nimport real;\n",
		"real.luna": "",
	})
	equal(t, "edges", edges(res), []string{"(root)->ghost", "(root)->real"})
	equal(t, "files", paths(res), []string{"app.luna", "real.luna"})
}

// TestIngressRejectedFileIsSkipped covers the documented hole: a file the lexer's ingress
// refuses cannot have its imports read, so its dependencies go undiscovered. Sound because
// §1.1 reports the ingress error and the compile aborts at that boundary.
func TestIngressRejectedFileIsSkipped(t *testing.T) {
	for _, tc := range []struct{ name, bad string }{
		{"leading BOM", "\ufeffimport hidden;\n"},
		{"invalid UTF-8", "\xffimport hidden;\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := run(t, "app.luna", map[string]string{
				"app.luna":    "import bad;\n",
				"bad.luna":    tc.bad,
				"hidden.luna": "",
			})
			equal(t, "files", paths(res), []string{"app.luna"})
			equal(t, "edges", edges(res), []string{"(root)->bad"})
		})
	}
}

// TestErrors covers the one channel that is not a diagnostic: discovery could not start.
func TestErrors(t *testing.T) {
	for _, tc := range []struct{ name, entry, want string }{
		{"missing entry", "nope.luna", "does not exist"},
		{"absolute path", "/app.luna", "not a valid path"},
		{"parent traversal", "../app.luna", "not a valid path"},
		{"empty path", "", "not a valid path"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fsys := fstest.MapFS{"app.luna": &fstest.MapFile{Data: []byte("")}}
			_, err := modules.Discover(fsys, tc.entry)
			if err == nil {
				t.Fatalf("Discover(%q) succeeded, want an error", tc.entry)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}

// TestEmptyTree is the floor: an entry that imports nothing yields exactly itself, and no
// edges. A harness that silently discovered nothing would pass several tests above.
func TestEmptyTree(t *testing.T) {
	res := run(t, "app.luna", map[string]string{"app.luna": ""})
	equal(t, "files", paths(res), []string{"app.luna"})
	if len(res.Edges) != 0 {
		t.Errorf("edges = %q, want none", edges(res))
	}
}
