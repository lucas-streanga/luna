package driver_test

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"luna/oracle/diagnostic"
	"luna/oracle/driver"
	"luna/oracle/modules"
)

func build(t *testing.T, entry string, files map[string]string) diagnostic.List {
	t.Helper()
	fsys := fstest.MapFS{}
	for path, src := range files {
		fsys[path] = &fstest.MapFile{Data: []byte(src)}
	}
	res, err := driver.Build(fsys, entry)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return res.Diagnostics
}

func codes(l diagnostic.List) []string {
	out := make([]string, 0, len(l))
	for _, d := range l {
		out = append(out, fmt.Sprintf("%s@%s:%d", d.Code, d.Primary.Filename, d.Primary.Offset))
	}
	return out
}

func equal(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("\n got %q\nwant %q", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("\n got %q\nwant %q", got, want)
		}
	}
}

func TestCleanBuild(t *testing.T) {
	diags := build(t, "app.luna", map[string]string{
		"app.luna": "import b;\nlet x = 1;\n",
		"b.luna":   "let y = 2;\n",
	})
	if !diags.Empty() {
		t.Errorf("clean tree produced %q", codes(diags))
	}
}

// TestLexicalErrorsStopBeforeValidation is §3's rule: a phase cannot consume the broken
// output of the previous one. app.luna also has a cycle, which must NOT be reported — §1.2
// never ran.
func TestLexicalErrorsStopBeforeValidation(t *testing.T) {
	diags := build(t, "app.luna", map[string]string{
		"app.luna": "import b;\nlet s = \"unterminated;\n",
		"b.luna":   "import app;\n",
	})
	for _, d := range diags {
		if d.Code.Stage() != diagnostic.Lexical {
			t.Errorf("%s reached the caller; §1.1 should have stopped the compile", d.Code)
		}
	}
	if diags.Empty() {
		t.Fatal("expected a lexical diagnostic")
	}
}

// TestIngressIsReported closes R251's loop. Discovery lists a BOM-bearing file precisely so
// this phase sees it, and the driver is the only place L0002 can be raised — lexer.Lex takes
// a *source.File, which for rejected bytes never exists.
func TestIngressIsReported(t *testing.T) {
	diags := build(t, "app.luna", map[string]string{
		"app.luna": "import bad;\n",
		"bad.luna": "\ufeffimport hidden;\n",
	})
	equal(t, codes(diags), []string{"L0002@bad.luna:0"})
}

func TestModuleDiagnosticsReachTheCaller(t *testing.T) {
	diags := build(t, "app.luna", map[string]string{
		"app.luna": "import ghost;\n",
	})
	equal(t, codes(diags), []string{"M0001@app.luna:7"})
}

// TestMissingEntryCarriesItsCode pins the mechanism modules §12 describes: M0005 travels as
// the code on discovery's error, and the driver converts it rather than returning it raw.
func TestMissingEntryCarriesItsCode(t *testing.T) {
	res, err := driver.Build(fstest.MapFS{}, "nope.luna")
	if err != nil {
		t.Fatalf("a missing entry is a diagnostic, not an error: %v", err)
	}
	equal(t, codes(res.Diagnostics), []string{"M0005@nope.luna:0"})
}

// TestUnrunnableBuildsAreErrors is the other half of that split: conditions modules §12
// leaves uncoded must not arrive as diagnostics, since a diagnostic needs a code a spec
// table names.
func TestUnrunnableBuildsAreErrors(t *testing.T) {
	res, err := driver.Build(fstest.MapFS{}, "../escape.luna")
	if err == nil {
		t.Fatalf("a malformed entry path produced diagnostics %q, want an error", codes(res.Diagnostics))
	}
	if res.Diagnostics != nil {
		t.Errorf("both channels used: %q", codes(res.Diagnostics))
	}
}

// TestEveryDiagnosticValidates guards the whole pipeline's output: Validate rejects a code
// with no title, so a phase raising something absent from its spec table fails here.
func TestEveryDiagnosticValidates(t *testing.T) {
	for _, files := range []map[string]string{
		{"app.luna": "import ghost;\n"},
		{"app.luna": "import bad;\n", "bad.luna": "\xffx\n"},
		{"app.luna": "let s = \"oops;\n"},
		{"app.luna": "import b;\n", "b.luna": "import app;\n"},
	} {
		diags := build(t, "app.luna", files)
		if diags.Empty() {
			t.Errorf("%v produced no diagnostics", files)
		}
		for _, d := range diags {
			if err := d.Validate(); err != nil {
				t.Errorf("%s: %v", d.Code, err)
			}
		}
	}
}

// TestOrderFollowsTheFileSet is what the parallel lex could quietly break.
//
// The assertion is the **expected** sequence, not "the same as last time". Self-comparison
// looks like it tests ordering and does not: a merge that collected in completion order but
// happened to be stable would pass, and so would one that was deterministic and simply wrong
// — reversed, or sorted by filename. Naming discovery's order outright is what makes those
// fail.
//
// The repeat is for the other half: ordering must not depend on which goroutine finished
// first, and one run cannot show that.
func TestOrderFollowsTheFileSet(t *testing.T) {
	// app imports b, c, d, so discovery's BFS yields app, b, c, d — and every file carries an
	// unterminated string, so each contributes exactly one diagnostic.
	files := map[string]string{"app.luna": "import b;\nimport c;\nimport d;\nlet s = \"a;\n"}
	for _, n := range []string{"b", "c", "d"} {
		files[n+".luna"] = "let s = \"" + n + ";\n"
	}
	want := []string{
		"L0009@app.luna:38",
		"L0009@b.luna:8",
		"L0009@c.luna:8",
		"L0009@d.luna:8",
	}

	for i := 0; i < 20; i++ {
		equal(t, codes(build(t, "app.luna", files)), want)
	}
}

// errFS fails the second read of a named file, standing in for the tree changing under a
// build — discovery read it, the driver could not.
type errFS struct {
	fs.FS
	fail  string
	reads int
}

func (e *errFS) Open(name string) (fs.File, error) {
	if name == e.fail {
		e.reads++
		if e.reads > 1 {
			return nil, fmt.Errorf("vanished")
		}
	}
	return e.FS.Open(name)
}

// TestFileVanishingMidBuildIsAnError pins the read-twice window as an error rather than a
// diagnostic. It is not a claim about the program, and modules §12 allocates it no code.
func TestFileVanishingMidBuildIsAnError(t *testing.T) {
	base := fstest.MapFS{
		"app.luna": &fstest.MapFile{Data: []byte("import b;\n")},
		"b.luna":   &fstest.MapFile{Data: []byte("let y = 2;\n")},
	}
	_, err := driver.Build(&errFS{FS: base, fail: "b.luna"}, "app.luna")
	if err == nil {
		t.Fatal("a file vanishing mid-build was not reported")
	}
	if !errors.Is(err, err) || err.Error() == "" {
		t.Error("error carries no message")
	}
}

// justCodes drops the spans, for cases where which check fired is the point and where it
// fired is pinned elsewhere.
func justCodes(l diagnostic.List) []string {
	out := make([]string, 0, len(l))
	for _, d := range l {
		out = append(out, string(d.Code))
	}
	return out
}

// TestRealisticProject is the headline end-to-end: a tree using every import form the grid
// admits, nested directories, `std` imports, and keyword path segments (R252), compiling
// without a word of complaint.
//
// Every case below asserts that something goes wrong. This one asserts that nothing does,
// which is the harder property — a driver that reported spuriously would pass all of them.
func TestRealisticProject(t *testing.T) {
	diags := build(t, "main.luna", map[string]string{
		"main.luna": `// the app root
import std.io;
import text.strings;
import { parse, split } from text.parse;
const fs = import { stat } from std.filesystem;
const cfg: table = import config;
export const shared = import util.shared;
import test.fixtures;
import error.codes;

fn main() {
  let x = 1;
}
`,
		"text/strings.luna":  "import util.shared;\nexport const upper = fn (s) => s;\n",
		"text/parse.luna":    "import util.shared;\nimport text.strings;\nexport const parse = fn (s) => s;\n",
		"util/shared.luna":   "export const version = \"1.0\";\n",
		"config.luna":        "export const port = 8080;\n",
		"test/fixtures.luna": "export const sample = \"x\";\n",
		"error/codes.luna":   "export const notFound = 404;\n",
	})
	if !diags.Empty() {
		t.Errorf("a valid project produced %q", codes(diags))
	}
}

// TestEveryModuleCodeEndToEnd walks each M code from source text to the caller. The unit
// tests in oracle/modules prove the checks fire; this proves the wiring delivers them.
func TestEveryModuleCodeEndToEnd(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"M0001 unresolved", map[string]string{
			"app.luna": "import ghost;\n",
		}, "M0001"},
		{"M0002 root import", map[string]string{
			"app.luna": "import b;\n", "b.luna": "import app;\n",
		}, "M0002"},
		{"M0003 cycle", map[string]string{
			"app.luna": "import b;\n", "b.luna": "import c;\n", "c.luna": "import b;\n",
		}, "M0003"},
		{"M0004 late import", map[string]string{
			"app.luna": "import b;\nlet x = 1;\nimport b;\n", "b.luna": "",
		}, "M0004"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			equal(t, justCodes(build(t, "app.luna", tc.files)), []string{tc.want})
		})
	}

	t.Run("M0005 missing entry", func(t *testing.T) {
		res, err := driver.Build(fstest.MapFS{}, "nope.luna")
		if err != nil {
			t.Fatal(err)
		}
		equal(t, justCodes(res.Diagnostics), []string{"M0005"})
	})
}

// TestEveryFileIsLexed proves reachability rather than asserting it. Each module in a deep,
// branching tree carries its own lexical error, so a file the driver failed to lex would go
// silently unreported — which is exactly how a dropped file looks.
func TestEveryFileIsLexed(t *testing.T) {
	files := map[string]string{
		"app.luna":          "import a.one;\nimport a.two;\nimport b.deep.three;\nlet s = \"app;\n",
		"a/one.luna":        "import b.deep.three;\nlet s = \"one;\n",
		"a/two.luna":        "let s = \"two;\n",
		"b/deep/three.luna": "let s = \"three;\n",
	}
	got := justCodes(build(t, "app.luna", files))
	if len(got) != len(files) {
		t.Fatalf("got %d diagnostics for %d files: %q", len(got), len(files), got)
	}
	for _, c := range got {
		if c != "L0009" {
			t.Errorf("unexpected %s", c)
		}
	}
}

// TestManyFilesSaturateThePool pushes past the worker bound. With more files than CPUs the
// semaphore actually queues, which is the path a single-digit tree never exercises — and
// -race is watching while it does.
func TestManyFilesSaturateThePool(t *testing.T) {
	const n = 200

	var entry string
	files := map[string]string{}
	for i := range n {
		name := fmt.Sprintf("m%03d", i)
		entry += "import " + name + ";\n"
		files[name+".luna"] = "let s = \"" + name + ";\n" // one lexical error each
	}
	files["app.luna"] = entry

	got := build(t, "app.luna", files)
	if len(got) != n {
		t.Fatalf("got %d diagnostics, want %d", len(got), n)
	}
	// Discovery is breadth-first from app, which imports them in written order, so the
	// diagnostics must arrive in that order too.
	for i, d := range got {
		want := fmt.Sprintf("m%03d.luna", i)
		if d.Primary.Filename != want {
			t.Fatalf("diagnostic %d is from %s, want %s", i, d.Primary.Filename, want)
		}
	}
}

// TestMultipleDiagnosticsInOneFile checks the within-a-file half of the ordering rule: the
// lexer collects rather than stopping (§1.1), and the driver must not reorder what it hands
// back.
func TestMultipleDiagnosticsInOneFile(t *testing.T) {
	diags := build(t, "app.luna", map[string]string{
		"app.luna": "let a = ^;\nlet b = ~;\nlet c = #;\n",
	})
	if len(diags) < 3 {
		t.Fatalf("got %q, want one per bad byte", codes(diags))
	}
	for i := 1; i < len(diags); i++ {
		if diags[i].Primary.Offset < diags[i-1].Primary.Offset {
			t.Errorf("offsets went backwards: %q", codes(diags))
		}
	}
}

// TestErrorsAcrossPhasesDoNotMix is §3 seen from the outside. A tree with both a lexical
// error and a module error yields only the lexical one, because §1.2 never runs.
func TestErrorsAcrossPhasesDoNotMix(t *testing.T) {
	diags := build(t, "app.luna", map[string]string{
		"app.luna": "import ghost;\nlet s = \"unterminated;\n",
	})
	equal(t, justCodes(diags), []string{"L0009"})
}

// TestStdImportsNeedNoTree pins modules §10 end-to-end: `std` is virtual, so importing it
// resolves to nothing on disk and must still compile clean.
func TestStdImportsNeedNoTree(t *testing.T) {
	diags := build(t, "app.luna", map[string]string{
		"app.luna": "import std;\nimport std.io;\nimport std.a.b.c;\n",
	})
	if !diags.Empty() {
		t.Errorf("std imports produced %q", codes(diags))
	}
}

// TestEmptyAndTrivialFiles covers the degenerate shapes a real tree contains.
func TestEmptyAndTrivialFiles(t *testing.T) {
	for _, tc := range []struct{ name, src string }{
		{"empty", ""},
		{"whitespace only", "\n\n   \n"},
		{"comment only", "// nothing here\n"},
		{"shebang only", "#!/usr/bin/env luna\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diags := build(t, "app.luna", map[string]string{"app.luna": tc.src})
			if !diags.Empty() {
				t.Errorf("%q produced %q", tc.src, codes(diags))
			}
		})
	}
}

// TestGraphSurvivesTheBuild is what keeping the graph bought: the layer structure §1.4 will
// consume is now observable end to end, where before it was computed and dropped.
func TestGraphSurvivesTheBuild(t *testing.T) {
	res, err := driver.Build(mapFS(map[string]string{
		"app.luna": "import b;\nimport c;\n",
		"b.luna":   "import d;\n",
		"c.luna":   "import d;\n",
		"d.luna":   "",
	}), "app.luna")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Diagnostics.Empty() {
		t.Fatalf("unexpected diagnostics: %q", codes(res.Diagnostics))
	}
	if res.Reached != driver.PhaseValidate {
		t.Fatalf("reached %s, want validate", res.Reached)
	}

	// d has no imports, b and c depend only on d, the root on both — three layers, deepest
	// first, which is the order §1.4 analyses in.
	equal(t, layerNames(res.Graph.Layers), []string{"d", "b,c", "(root)"})
	equal(t, filePaths(res.Files), []string{"app.luna", "b.luna", "c.luna", "d.luna"})
}

// TestReachedNamesThePhase pins the field that exists because no other answers the question.
// The cycle case is the reason: its graph is empty, yet §1.2 ran.
func TestReachedNamesThePhase(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  driver.Phase
	}{
		{"clean build validates", map[string]string{"app.luna": ""}, driver.PhaseValidate},
		{"a lexical error stops after lex", map[string]string{
			"app.luna": "let s = \"oops;\n",
		}, driver.PhaseDiscover},
		// Every module is in the cycle, so the graph is empty — but validation ran, and that
		// is the distinction Reached exists to carry.
		{"a cycle still reaches validate", map[string]string{
			"app.luna": "import b;\n", "b.luna": "import c;\n", "c.luna": "import b;\n",
		}, driver.PhaseValidate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := driver.Build(mapFS(tc.files), "app.luna")
			if err != nil {
				t.Fatal(err)
			}
			if res.Reached != tc.want {
				t.Errorf("reached %s, want %s", res.Reached, tc.want)
			}
		})
	}
}

// TestFileSetIsObservable closes the gap the old API left: which files a build compiled was
// only inferable from diagnostics, so a silently dropped file looked like a clean one.
func TestFileSetIsObservable(t *testing.T) {
	res, err := driver.Build(mapFS(map[string]string{
		"app.luna":       "import std.io;\nimport a.b;\nimport ghost;\n",
		"a/b.luna":       "",
		"unreached.luna": "",
	}), "app.luna")
	if err != nil {
		t.Fatal(err)
	}
	// std is virtual and ghost does not exist, so neither is a file; unreached.luna is never
	// imported, so discovery never sees it.
	equal(t, filePaths(res.Files), []string{"app.luna", "a/b.luna"})
}

func mapFS(files map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for path, src := range files {
		fsys[path] = &fstest.MapFile{Data: []byte(src)}
	}
	return fsys
}

func filePaths(files []modules.File) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.Path)
	}
	return out
}

// layerNames renders the topological layers, the root's empty module path shown as (root).
func layerNames(layers [][]string) []string {
	out := make([]string, 0, len(layers))
	for _, layer := range layers {
		named := make([]string, 0, len(layer))
		for _, m := range layer {
			if m == "" {
				m = "(root)"
			}
			named = append(named, m)
		}
		out = append(out, strings.Join(named, ","))
	}
	return out
}
