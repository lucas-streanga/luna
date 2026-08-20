package modules_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"luna/oracle/lexer"
	"luna/oracle/modules"
	"luna/oracle/source"
	"luna/oracle/token"
)

// FuzzDiscover runs arbitrary bytes through §1.0 and §1.2.
//
// The lexer earned its fuzzer by consuming arbitrary input; discovery consumes exactly the
// same input and then runs a hand-written parser over the result, which is the part no other
// technique reaches. The risk is not a wrong answer, which the tables cover, but a panic on
// a shape nobody thought to write down.
//
// Properties beyond "did not panic" are asserted below, because a fuzzer that only checks for
// crashes finds only crashes. Each is something §1.2 or the driver relies on and would
// otherwise discover the hard way.
func FuzzDiscover(f *testing.F) {
	for _, seed := range []string{
		"", "\n", "import", "import ;", "import a;", "import a",
		"import a.b.c;", "import a . b ;", "import a./*c*/b;", "import a.;", "import .a;",
		"import { x } from a;", "import { x, y, } from a;", "import { x as y } from a;",
		"import { x as from } from a;", "import { } from a;", "import { x } a;", "import {",
		"const d = import a;", "const d: table = import a;", "const d: = import a;",
		"export const d = import a;", "export import a;", "let d = import a;",
		"const d = 5;\nimport a;", "import test.error.if;", "import _;", "import match!;",
		"// import a;\nimport b;", "/* import a; */ import b;", "\"import a;\"",
		"import a;\nimport a;\nimport a;", "import std.io;", "import std;",
		"\ufeffimport a;", "\xffimport a;", "import \"a\";", "import a b;",
		"import a;;", "importa;", "IMPORT a;", "import\na\n;", strings.Repeat("import a;\n", 64),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, src string) {
		const entry = "app.luna"
		fsys := fstest.MapFS{entry: &fstest.MapFile{Data: []byte(src)}}

		res, err := modules.Discover(fsys, entry)
		if err != nil {
			t.Fatalf("a readable entry failed discovery: %v", err) // only I/O can fail here
		}

		// The entry is always present and always first: §1.2 finds the root by its empty
		// module path, and the driver reports against Files in order.
		if len(res.Files) != 1 || res.Files[0].Path != entry {
			t.Fatalf("files = %v, want exactly the entry", res.Files)
		}
		if res.Files[0].Module != "" {
			t.Errorf("entry module = %q, want the empty root path", res.Files[0].Module)
		}
		if end := res.Files[0].PreludeEnd; end < 0 || end > len(src) {
			t.Errorf("PreludeEnd = %d, outside [0, %d]", end, len(src))
		}

		for _, e := range res.Edges {
			// §1.2 anchors a diagnostic on this span; a slice outside the file would panic
			// there rather than here.
			if e.Offset < 0 || e.Len < 0 || e.Offset+e.Len > len(src) {
				t.Errorf("edge %q spans %d..%d, outside [0, %d]",
					e.To, e.Offset, e.Offset+e.Len, len(src))
			}
			if e.To == "" {
				t.Errorf("edge from %q names no module", e.From)
			}
			if e.From != "" {
				t.Errorf("edge from %q, but only the root was read", e.From)
			}
		}

		// §1.2 over the same input. It panics by contract on a missing token stream, so
		// building one for every file is part of the property being checked.
		toks := map[string][]token.Token{}
		for _, file := range res.Files {
			f, err := source.New(file.Path, src)
			if err != nil {
				toks[file.Path] = nil // ingress rejected it; §1.1 would report and abort
				continue
			}
			stream, _ := lexer.Lex(f)
			toks[file.Path] = stream
		}

		_, diags := modules.Validate(res, toks)
		for _, d := range diags {
			// A code with no title is one no spec table names, which Validate refuses:
			// same guard the end-to-end tests apply, here over inputs nobody chose.
			if err := d.Validate(); err != nil {
				t.Errorf("%v", err)
			}
			if d.Primary.Offset < 0 || d.Primary.Offset > len(src) {
				t.Errorf("%s anchored at %d, outside [0, %d]", d.Code, d.Primary.Offset, len(src))
			}
		}
	})
}
