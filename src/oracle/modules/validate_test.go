package modules_test

import (
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

	"luna/oracle/diagnostic"
	"luna/oracle/lexer"
	"luna/oracle/modules"
	"luna/oracle/source"
	"luna/oracle/token"
)

// The harness runs the real §1.0 → §1.1 → §1.2 chain over an in-memory tree, because that is
// the only way the three agree about what a `Result` looks like. Hand-built `Result`s would
// let a test assert against a shape discovery never produces.

// validate discovers, lexes every discovered file, and validates — the driver's job, in
// miniature (driver.md §1).
func validate(t *testing.T, entry string, files map[string]string) (modules.Graph, diagnostic.List) {
	t.Helper()
	fsys := fstest.MapFS{}
	for path, src := range files {
		fsys[path] = &fstest.MapFile{Data: []byte(src)}
	}
	res, err := modules.Discover(fsys, entry)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}

	toks := map[string][]token.Token{}
	for _, f := range res.Files {
		src, err := source.New(f.Path, files[f.Path])
		if err != nil {
			// Ingress rejected it, so §1.1 would report and abort before §1.2 ran. An empty
			// stream keeps the harness honest about that rather than skipping the file.
			toks[f.Path] = nil
			continue
		}
		stream, _ := lexer.Lex(src)
		toks[f.Path] = stream
	}
	return modules.Validate(res, toks)
}

// codes renders the diagnostics as "CODE@file:offset", which is what tests pin —
// testing-strategy §2 pins the code and the primary span, never the prose.
func codes(l diagnostic.List) []string {
	out := make([]string, 0, len(l))
	for _, d := range l {
		out = append(out, fmt.Sprintf("%s@%s:%d", d.Code, d.Primary.Filename, d.Primary.Offset))
	}
	return out
}

// descriptions is for the two checks whose message carries information no span can: the cycle
// path, and which module went unresolved.
func descriptions(l diagnostic.List) []string {
	out := make([]string, 0, len(l))
	for _, d := range l {
		out = append(out, d.Description)
	}
	return out
}

func layers(g modules.Graph) []string {
	out := make([]string, 0, len(g.Layers))
	for _, layer := range g.Layers {
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

// TestCleanGraphHasNoDiagnostics is the floor. Every case below asserts that some check
// fires; this one asserts they stay quiet, which a validator that reported everything would
// fail and a validator that reported nothing would pass — hence the layer assertion too.
func TestCleanGraphHasNoDiagnostics(t *testing.T) {
	g, diags := validate(t, "app.luna", map[string]string{
		"app.luna": "import b;\nimport c;\n",
		"b.luna":   "import d;\n",
		"c.luna":   "import d;\n",
		"d.luna":   "",
	})
	if !diags.Empty() {
		t.Fatalf("unexpected diagnostics: %q", codes(diags))
	}
	equal(t, "layers", layers(g), []string{"d", "b,c", "(root)"})
}

func TestLayers(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"entry alone", map[string]string{"app.luna": ""}, []string{"(root)"}},
		{"a chain is one module per layer", map[string]string{
			"app.luna": "import b;\n", "b.luna": "import c;\n", "c.luna": "",
		}, []string{"c", "b", "(root)"}},
		{"independent modules share a layer", map[string]string{
			"app.luna": "import b;\nimport c;\n", "b.luna": "", "c.luna": "",
		}, []string{"b,c", "(root)"}},
		// The case a decrementing counter gets wrong: e depends on both a layer-0 and a
		// layer-1 module, so it must land in layer 2, not be double-credited into layer 1.
		{"a module spanning two layers waits for the deeper one", map[string]string{
			"app.luna": "import e;\n",
			"e.luna":   "import a;\nimport d;\n",
			"d.luna":   "import a;\n",
			"a.luna":   "",
		}, []string{"a", "d", "e", "(root)"}},
		{"std is not a node", map[string]string{
			"app.luna": "import std.io;\nimport b;\n", "b.luna": "",
		}, []string{"b", "(root)"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, diags := validate(t, "app.luna", tc.files)
			if !diags.Empty() {
				t.Fatalf("unexpected diagnostics: %q", codes(diags))
			}
			equal(t, "layers", layers(g), tc.want)

			// Order is the layers flattened, and every module must appear after its imports.
			if got, want := len(g.Order()), len(tc.files); got != want {
				t.Errorf("Order() has %d modules, want %d", got, want)
			}
		})
	}
}

func TestUnresolvedImport(t *testing.T) {
	_, diags := validate(t, "app.luna", map[string]string{
		"app.luna": "import ghost;\nimport b;\n",
		"b.luna":   "import also.missing;\n",
	})
	equal(t, "codes", codes(diags), []string{"M0001@app.luna:7", "M0001@b.luna:7"})
	equal(t, "descriptions", descriptions(diags), []string{
		"no module `ghost` under the source root",
		"no module `also.missing` under the source root",
	})
}

// TestStdIsNeverUnresolved is why resolve() has a reserved() call: `std.*` reaches no file by
// construction (R251), so without the exclusion every stdlib import would be an error.
func TestStdIsNeverUnresolved(t *testing.T) {
	_, diags := validate(t, "app.luna", map[string]string{
		"app.luna": "import std;\nimport std.io;\nimport std.a.b;\n",
	})
	if !diags.Empty() {
		t.Errorf("std imports reported: %q / %q", codes(diags), descriptions(diags))
	}
}

// TestRootImport pins R251: importing the entry is its own error, not a cycle.
func TestRootImport(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"self-import", map[string]string{"app.luna": "import app;\n"}, []string{"M0002@app.luna:7"}},
		{"through another module", map[string]string{
			"app.luna": "import b;\n", "b.luna": "import app;\n",
		}, []string{"M0002@b.luna:7"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, diags := validate(t, "app.luna", tc.files)
			equal(t, "codes", codes(diags), tc.want)
			// Not M0003. A cycle report would send the reader hunting for a dependency to
			// invert when the fix is to stop importing the entry.
			for _, d := range diags {
				if d.Code == diagnostic.ImportCycle {
					t.Errorf("reported as a cycle: %q", d.Description)
				}
			}
		})
	}
}

func TestCycles(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"two modules", map[string]string{
			"app.luna": "import b;\n", "b.luna": "import c;\n", "c.luna": "import b;\n",
		}, []string{"import cycle: b -> c -> b"}},
		{"self-import below the root", map[string]string{
			"app.luna": "import b;\n", "b.luna": "import b;\n",
		}, []string{"import cycle: b -> b"}},
		{"three modules", map[string]string{
			"app.luna": "import b;\n", "b.luna": "import c;\n",
			"c.luna": "import d;\n", "d.luna": "import b;\n",
		}, []string{"import cycle: b -> c -> d -> b"}},
		// A duplicated import is two edges to one module, and the adjacency must dedupe them
		// or the same cycle reports once per duplicate. Mutation testing found this untested.
		{"a duplicated import reports one cycle", map[string]string{
			"app.luna": "import b;\n", "b.luna": "import b;\nimport b;\n",
		}, []string{"import cycle: b -> b"}},
		{"a duplicated import in a longer loop", map[string]string{
			"app.luna": "import b;\n",
			"b.luna":   "import c;\nimport c;\n", "c.luna": "import b;\n",
		}, []string{"import cycle: b -> c -> b"}},
		// Every cycle, not the first (R251): two disjoint loops both get reported.
		{"two disjoint cycles", map[string]string{
			"app.luna": "import b;\nimport d;\n",
			"b.luna":   "import c;\n", "c.luna": "import b;\n",
			"d.luna": "import e;\n", "e.luna": "import d;\n",
		}, []string{"import cycle: b -> c -> b", "import cycle: d -> e -> d"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, diags := validate(t, "app.luna", tc.files)
			equal(t, "descriptions", descriptions(diags), tc.want)
			for _, d := range diags {
				if d.Code != diagnostic.ImportCycle {
					t.Errorf("unexpected %s: %s", d.Code, d.Description)
				}
			}
		})
	}
}

// TestCycleModulesAreAbsentFromLayers documents what a Graph looks like when the compile is
// about to abort: a module in a cycle never becomes ready, so it is simply missing.
func TestCycleModulesAreAbsentFromLayers(t *testing.T) {
	g, diags := validate(t, "app.luna", map[string]string{
		"app.luna": "import b;\nimport ok;\n",
		"b.luna":   "import c;\n", "c.luna": "import b;\n",
		"ok.luna": "",
	})
	if diags.Empty() {
		t.Fatal("expected a cycle diagnostic")
	}
	equal(t, "layers", layers(g), []string{"ok"})
}

// TestLateImport is the prelude rule R250 moved from §1.3 to here.
func TestLateImport(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      []string
	}{
		{"after a declaration", "import a;\nlet x = 1;\nimport b;\n", []string{"M0004@app.luna:21"}},
		{"inside a function", "import a;\nfn f() {\n  import b;\n}\n", []string{"M0004@app.luna:21"}},
		{"two late imports", "let x = 1;\nimport a;\nimport b;\n",
			[]string{"M0004@app.luna:11", "M0004@app.luna:21"}},
		{"a const that is not an import ends the prelude", "const n = 5;\nimport a;\n",
			[]string{"M0004@app.luna:13"}},
		{"no late import in a clean prelude", "import a;\nconst d = import a;\nlet x = 1;\n", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, diags := validate(t, "app.luna", map[string]string{
				"app.luna": tc.src,
				"a.luna":   "", "b.luna": "",
			})
			var late []string
			for _, d := range diags {
				if d.Code == diagnostic.ImportOutsidePrelude {
					late = append(late, fmt.Sprintf("%s@%s:%d", d.Code, d.Primary.Filename, d.Primary.Offset))
				}
			}
			equal(t, "late imports", late, tc.want)
		})
	}
}

// TestEveryDiagnosticValidates is the pin that makes the codes real. Validate rejects a code
// with no title, so this fails for any diagnostic raised with a code missing from modules §12.
func TestEveryDiagnosticValidates(t *testing.T) {
	_, diags := validate(t, "app.luna", map[string]string{
		"app.luna": "import ghost;\nimport app;\nimport b;\nlet x = 1;\nimport c;\n",
		"b.luna":   "import c;\n", "c.luna": "import b;\n",
	})
	if len(diags) < 4 {
		t.Fatalf("wanted every check to fire, got %q", codes(diags))
	}
	seen := map[diagnostic.Code]bool{}
	for _, d := range diags {
		if err := d.Validate(); err != nil {
			t.Errorf("%s: %v", d.Code, err)
		}
		if d.Code.Stage() != diagnostic.Modules {
			t.Errorf("%s is not an M code", d.Code)
		}
		seen[d.Code] = true
	}
	for _, c := range []diagnostic.Code{
		diagnostic.UnresolvedImport, diagnostic.RootImport,
		diagnostic.ImportCycle, diagnostic.ImportOutsidePrelude,
	} {
		if !seen[c] {
			t.Errorf("%s never fired; this test no longer covers it", c)
		}
	}
}

// TestMissingTokenStreamPanics pins the driver contract: §1.2 runs after §1.1, so a file
// without a stream is a driver bug. Reading nil would silently mean "no late imports here".
func TestMissingTokenStreamPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("Validate accepted a Result with no token stream for its entry")
		}
	}()
	res, err := modules.Discover(fstest.MapFS{
		"app.luna": &fstest.MapFile{Data: []byte("")},
	}, "app.luna")
	if err != nil {
		t.Fatal(err)
	}
	modules.Validate(res, nil)
}
