package modules_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"luna/internal/spec"
	"luna/oracle/diagnostic"

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
		"app.luna":      "import std.io;\nimport std;\nimport stdlib;\nimport standard;\n",
		"std/io.luna":   "import should.not.be.read;\n",
		"stdlib.luna":   "",
		"standard.luna": "",
	})
	equal(t, "edges", edges(res),
		[]string{"(root)->std.io", "(root)->std", "(root)->stdlib", "(root)->standard"})

	// `stdlib` is the case that matters: it *does* begin with the reserved name, so it is what
	// separates "reserved is `std` exactly, or a path beneath it" from "reserved is anything
	// starting with std". `standard` cannot make that distinction — it begins `sta` — and is
	// kept only as an ordinary neighbour. Mutation testing found the original test asserting
	// the distinction with `standard` alone, which tested nothing.
	equal(t, "files", paths(res), []string{"app.luna", "stdlib.luna", "standard.luna"})
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

// TestIngressRejectedFileIsListed pins the fix for a hole this suite originally asserted.
//
// A file the lexer's ingress refuses cannot have its prelude read, so its dependencies go
// undiscovered — that part is unavoidable. What matters is that the file itself is still
// listed: §1.1 lexes the discovered set and owns the lexical codes, so a file dropped here is
// a BOM nobody reports, surfacing at §1.2 as a bogus unresolved import instead.
func TestIngressRejectedFileIsListed(t *testing.T) {
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
			// bad.luna is present; hidden.luna is not, its import having been unreadable.
			equal(t, "files", paths(res), []string{"app.luna", "bad.luna"})
			equal(t, "edges", edges(res), []string{"(root)->bad"})

			if got := res.Files[1].PreludeEnd; got != 0 {
				t.Errorf("PreludeEnd = %d, want 0: no prelude was read", got)
			}
		})
	}
}

// TestIngressRejectedEntryIsListed is the same rule at the root, where no edge exists to fall
// back on: an unreadable entry must still reach §1.1 to be reported.
func TestIngressRejectedEntryIsListed(t *testing.T) {
	res := run(t, "app.luna", map[string]string{
		"app.luna": "\ufeffimport dep;\n",
		"dep.luna": "",
	})
	equal(t, "files", paths(res), []string{"app.luna"})
	if len(res.Edges) != 0 {
		t.Errorf("edges = %q, want none", edges(res))
	}
}

// TestErrors covers the one channel that is not a diagnostic: discovery could not start.
// TestErrors covers the one channel that is not a diagnostic: discovery could not start.
//
// A malformed path carries no code — it is a caller bug, belonging to the `I` stage per
// modules §12's uncoded list — so those cases pin the prose, having nothing better. The
// missing entry does carry one, and pins that instead (testing-strategy §2).
func TestErrors(t *testing.T) {
	t.Run("missing entry carries M0005", func(t *testing.T) {
		fsys := fstest.MapFS{"app.luna": &fstest.MapFile{Data: []byte("")}}
		_, err := modules.Discover(fsys, "nope.luna")

		var me *modules.Error
		if !errors.As(err, &me) {
			t.Fatalf("error is %T (%v), want *modules.Error", err, err)
		}
		if me.Code != diagnostic.MissingEntry {
			t.Errorf("code = %s, want %s", me.Code, diagnostic.MissingEntry)
		}
		if me.Path != "nope.luna" {
			t.Errorf("path = %q, want nope.luna", me.Path)
		}
	})

	for _, tc := range []struct{ name, entry string }{
		{"absolute path", "/app.luna"},
		{"parent traversal", "../app.luna"},
		{"empty path", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fsys := fstest.MapFS{"app.luna": &fstest.MapFile{Data: []byte("")}}
			_, err := modules.Discover(fsys, tc.entry)
			if err == nil {
				t.Fatalf("Discover(%q) succeeded, want an error", tc.entry)
			}
			if !strings.Contains(err.Error(), "not a valid path") {
				t.Errorf("error = %q, want it to mention an invalid path", err)
			}
			// A caller bug, so it must not masquerade as a module diagnostic.
			var me *modules.Error
			if errors.As(err, &me) {
				t.Errorf("carries code %s; a malformed path has none", me.Code)
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

// TestFormSpace sweeps the grammar the prelude admits, rather than the forms I happened to
// think of. §5's grid crossed with the two modifiers §6 allows on an assigned import —
// `export`, and a type annotation — is ten spellings of one edge, and every one must produce
// it.
//
// This exists because the annotation was missed: `const d = import p;` was tested and
// `const d: table = import p;` was not, though modules §6 makes both legal. A dropped edge
// there is silent, the form being valid, so no parser error backstops it. Enumerating the
// space is what stops the next modifier from going the same way.
func TestFormSpace(t *testing.T) {
	specs := []struct{ name, text string }{
		{"bare", "dep"},
		{"selective", "{ a } from dep"},
	}
	bindings := []struct{ name, prefix string }{
		{"statement", ""},
		{"assigned", "const d = "},
		{"annotated", "const d: table = "},
		{"exported", "export const d = "},
		{"exported annotated", "export const d: table = "},
	}

	for _, b := range bindings {
		for _, s := range specs {
			t.Run(b.name+"/"+s.name, func(t *testing.T) {
				src := b.prefix + "import " + s.text + ";\n"
				res := run(t, "app.luna", map[string]string{
					"app.luna": src,
					"dep.luna": "",
				})
				if got := edges(res); len(got) != 1 || got[0] != "(root)->dep" {
					t.Fatalf("%q produced %q, want one edge to dep", src, got)
				}
				// The edge is worth nothing if the file it names went undiscovered.
				equal(t, "files", paths(res), []string{"app.luna", "dep.luna"})
				// And the prelude must not have ended at this line.
				if res.Files[0].PreludeEnd != len(src) {
					t.Errorf("%q ended the prelude at %d, want %d",
						src, res.Files[0].PreludeEnd, len(src))
				}
			})
		}
	}
}

// TestSpecExamplesAreDiscoverable pins discovery to the spec's own worked examples. Every
// import modules.md writes in a `luna` block must yield an edge — if the spec shows a form,
// discovery reads it, and neither can drift without the other noticing.
//
// The corpus is read rather than copied for the reason §0's token table is: a form added to
// the spec should fail here until the code catches up, which a hand-copied list cannot do.
func TestSpecExamplesAreDiscoverable(t *testing.T) {
	blocks, err := spec.LunaBlocks()
	if err != nil {
		t.Fatalf("reading the spec corpus: %v", err)
	}

	checked := 0
	for _, b := range blocks {
		if !strings.HasSuffix(b.Path, "modules.md") {
			continue
		}
		for _, line := range strings.Split(b.Source, "\n") {
			line = strings.TrimSpace(stripComment(line))
			if !strings.HasPrefix(line, "import ") && !strings.Contains(line, "= import ") {
				continue
			}
			if !strings.HasSuffix(line, ";") {
				line += ";" // several examples elide it, being fragments
			}
			checked++

			res := run(t, "app.luna", map[string]string{"app.luna": line + "\n"})
			if len(res.Edges) != 1 {
				t.Errorf("%s: %q produced %d edges, want 1", b.Path, line, len(res.Edges))
			}
		}
	}
	if checked == 0 {
		t.Fatal("no import examples found in modules.md; this test checked nothing")
	}
	t.Logf("%d import examples from the spec", checked)
}

// stripComment drops a trailing `//` comment, which the spec's examples use to annotate the
// form. Crude on purpose: no example puts `//` inside a string.
func stripComment(line string) string {
	if i := strings.Index(line, "//"); i >= 0 {
		return line[:i]
	}
	return line
}

// keywordLexemes is every keyword's spelling, read from lexer §0's first column.
//
// Enumerated from the spec rather than sampled, because sampling is what let the annotated
// import through: a keyword added to §3 should arrive here on its own and fail if the segment
// rule cannot take it.
func keywordLexemes(t *testing.T) []string {
	t.Helper()
	inv, err := spec.Load()
	if err != nil {
		t.Fatalf("reading lexer §0: %v", err)
	}
	var out []string
	for _, r := range inv.Rows {
		if r.Category != "keyword" {
			continue
		}
		out = append(out, strings.Trim(strings.TrimSpace(r.Token), "`"))
	}
	if len(out) == 0 {
		t.Fatal("no keywords found in §0; the table changed shape")
	}
	return out
}

// TestEveryKeywordIsAPathSegment sweeps §3's keywords through path position. A path segment
// is not a name (modules §5), so every one of them must resolve.
func TestEveryKeywordIsAPathSegment(t *testing.T) {
	for _, kw := range keywordLexemes(t) {
		t.Run(kw, func(t *testing.T) {
			// `yield from` is one token whose lexeme carries a space, so it names a directory
			// no ordinary tree has. It still has to parse as a segment rather than end the
			// prelude, which is what this asserts.
			file := strings.ReplaceAll(kw, ".", "/") + ".luna"
			res := run(t, "app.luna", map[string]string{
				"app.luna": "import " + kw + ";\n",
				file:       "",
			})
			if got := edges(res); len(got) != 1 || got[0] != "(root)->"+kw {
				t.Fatalf("`import %s;` produced %q", kw, got)
			}
			equal(t, "files", paths(res), []string{"app.luna", file})
		})
	}
}

// TestKeywordSegmentPositions puts a keyword everywhere a segment can go, since accepting one
// leading a path says nothing about one in the middle or after a from-clause.
func TestKeywordSegmentPositions(t *testing.T) {
	for _, tc := range []struct{ name, src, want, file string }{
		{"leading", "import test.helpers;", "test.helpers", "test/helpers.luna"},
		{"trailing", "import helpers.test;", "helpers.test", "helpers/test.luna"},
		{"interior", "import a.if.b;", "a.if.b", "a/if/b.luna"},
		{"every segment", "import if.else.while;", "if.else.while", "if/else/while.luna"},
		{"after from", "import { x } from error.codes;", "error.codes", "error/codes.luna"},
		{"assigned", "const e = import error;", "error", "error.luna"},
		{"annotated", "const e: table = import test;", "test", "test.luna"},
		{"wildcard", "import _.x;", "_.x", "_/x.luna"},
		{"compound with a bang", "import match!.x;", "match!.x", "match!/x.luna"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := run(t, "app.luna", map[string]string{
				"app.luna": tc.src + "\n",
				tc.file:    "",
			})
			equal(t, "edges", edges(res), []string{"(root)->" + tc.want})
			equal(t, "files", paths(res), []string{"app.luna", tc.file})
		})
	}
}

// TestKeywordsAreNotNames is the boundary. A path segment is not a name, so relaxing path
// position must not relax the two places a name is actually bound — otherwise this stops
// being "keywords may appear in paths" and becomes "keywords are identifiers".
func TestKeywordsAreNotNames(t *testing.T) {
	t.Run("a binding name may not be a keyword", func(t *testing.T) {
		// `const test = import dep;` is not an import: the prelude ends at the `const`, and
		// §1.3 reports the syntax error.
		res := run(t, "app.luna", map[string]string{
			"app.luna": "const test = import dep;\n",
			"dep.luna": "",
		})
		if len(res.Edges) != 0 {
			t.Errorf("a keyword binding name was accepted: %q", edges(res))
		}
		if got := res.Files[0].PreludeEnd; got != 0 {
			t.Errorf("PreludeEnd = %d, want 0: the prelude ends at the const", got)
		}
	})

	t.Run("a braced name list is the parser's to judge", func(t *testing.T) {
		// Discovery skips the names wholesale, so `{ test }` is followed rather than rejected.
		// Over-approximation is the safe direction — the file set grows, and §1.3 reports the
		// illegal binding — but it is deliberate, not an oversight.
		res := run(t, "app.luna", map[string]string{
			"app.luna": "import { test } from dep;\n",
			"dep.luna": "",
		})
		equal(t, "edges", edges(res), []string{"(root)->dep"})
	})
}

// TestKeywordPathsReachTheRightFile closes the loop: a keyword path must resolve through
// fileOf to the directory it names, and its module identity must round-trip.
func TestKeywordPathsReachTheRightFile(t *testing.T) {
	res := run(t, "app.luna", map[string]string{
		"app.luna":            "import test.helpers;\nimport error;\n",
		"test/helpers.luna":   "import if.deep;\n",
		"error.luna":          "",
		"if/deep.luna":        "",
		"unrelated/test.luna": "",
	})
	equal(t, "files", paths(res), []string{
		"app.luna", "test/helpers.luna", "error.luna", "if/deep.luna",
	})
	equal(t, "edges", edges(res), []string{
		"(root)->test.helpers", "(root)->error", "test.helpers->if.deep",
	})
	if got := res.Files[1].Module; got != "test.helpers" {
		t.Errorf("module = %q, want test.helpers", got)
	}
}

// TestEdgeSpansThePath pins the span discovery records for each import, which §1.2 turns
// into a caret. Len is measured from the tokens rather than derived from the module path,
// because the two disagree wherever trivia sits inside the path.
func TestEdgeSpansThePath(t *testing.T) {
	for _, tc := range []struct {
		name, src        string
		wantOff, wantLen int
	}{
		{"plain", "import a.b;\n", 7, 3},
		{"spaced", "import a . b;\n", 7, 5},
		{"comment inside", "import a./*c*/b;\n", 7, 8},
		{"after a from-clause", "import { x } from a.b;\n", 18, 3},
		{"assigned", "const d = import a.b;\n", 17, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := run(t, "app.luna", map[string]string{
				"app.luna": tc.src, "a/b.luna": "",
			})
			if len(res.Edges) != 1 {
				t.Fatalf("got %d edges, want 1", len(res.Edges))
			}
			e := res.Edges[0]
			if e.Offset != tc.wantOff || e.Len != tc.wantLen {
				t.Errorf("span = %d..%d, want %d..%d (source %q)",
					e.Offset, e.Offset+e.Len, tc.wantOff, tc.wantOff+tc.wantLen, tc.src)
			}
		})
	}
}
