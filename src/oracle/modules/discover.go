// Discovery: compiler §1.0, the pipeline's stage 0 (R190, R250).
package modules

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"luna/oracle/lexer"
	"luna/oracle/source"
	"luna/oracle/token"
)

const (
	ext = ".luna"
	// std is the standard library's virtual root (modules §10) — resolved by the compiler,
	// never a directory, and a name a project may not take.
	std = "std"
)

// Discover reads the import graph from entry and returns every file it reaches.
//
// fsys is rooted at the **module root**, so a file's path within it is its module path with
// dots for slashes: `utils/parse.luna` is `utils.parse`, and entry is the root module with
// the empty path (modules §3). That is why the parameter is an fs.FS and not a directory
// name — the mapping needs no path arithmetic, callers pass os.DirFS(dir), and tests pass
// fstest.MapFS. §3's one rule covers applications and libraries alike: the root is the
// directory of the entry file.
//
// It raises no diagnostic (R250). A path resolving to no file is skipped and its Edge kept
// for §1.2; cycles are terminated by the visited set and diagnosed there too.
//
// The error return is not a diagnostic channel — only "discovery could not proceed": an
// unreadable entry, or an I/O failure on a file that exists.
//
// A file ingress rejects (invalid UTF-8, a leading BOM) cannot be lexed, so its own imports
// go undiscovered. Sound for the same reason the early stop is: §1.1 reports the ingress
// error and the compile aborts at the phase boundary.
func Discover(fsys fs.FS, entry string) (Result, error) {
	if !fs.ValidPath(entry) {
		return Result{}, fmt.Errorf("modules: %q is not a valid path within the source root", entry)
	}

	var res Result
	// Keyed by file, not module, so a file reached by two spellings is read once. This is also
	// what terminates cycles — discovery's whole involvement with them (R190).
	seen := map[string]bool{entry: true}

	for queue := []string{entry}; len(queue) > 0; {
		file := queue[0]
		queue = queue[1:]

		src, err := fs.ReadFile(fsys, file)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			// The entry is the exception: no edge names it, so skipping would lose it, and a
			// missing entry is a reason discovery cannot start rather than a claim about the
			// program.
			if file == entry {
				return Result{}, fmt.Errorf("modules: entry %q does not exist", entry)
			}
			continue
		case err != nil:
			return Result{}, fmt.Errorf("modules: reading %s: %w", file, err)
		}

		f, err := source.New(file, string(src))
		if err != nil {
			continue // ingress rejected the bytes; §1.1 reports it
		}

		end, imports := readPrelude(f)
		module := moduleOf(file, file == entry)
		res.Files = append(res.Files, File{Path: file, Module: module, PreludeEnd: end})

		for _, imported := range imports {
			res.Edges = append(res.Edges, Edge{From: module, To: imported})

			// R250 left std resolution unruled and there is no tree to resolve against, so the
			// edge is recorded and nothing followed — committing to neither answer.
			if reserved(imported) {
				continue
			}
			if target := fileOf(imported); !seen[target] {
				seen[target] = true
				queue = append(queue, target)
			}
		}
	}
	return res, nil
}

// reserved reports whether a module path lies under std. A module named `standard` does not.
func reserved(module string) bool {
	return module == std || strings.HasPrefix(module, std+".")
}

// fileOf maps a module path to its file: `utils.parse` to `utils/parse.luna` (modules §3).
func fileOf(module string) string {
	return strings.ReplaceAll(module, ".", "/") + ext
}

// moduleOf is the inverse. The entry is the root module, whose path is empty (modules §3).
func moduleOf(file string, isEntry bool) string {
	if isEntry {
		return ""
	}
	return strings.ReplaceAll(strings.TrimSuffix(file, ext), "/", ".")
}

// readPrelude reads a file's imports, returning where the prelude ended and the paths named.
//
// This is the imports-only mode R190 requires be the real lexer and never a second scanner:
// `// import x` arrives as LINE_COMMENT and `"import x"` as DQ_TEXT, so neither reaches here
// as a KW_IMPORT, and the mode stack is what keeps that true through interpolation.
//
// A malformed import is simply not an import: the prelude ends there and the parser raises
// the syntax error. That is how the no-diagnostics contract survives without discovery having
// to tell "not an import" from "a broken one".
func readPrelude(f *source.File) (end int, imports []string) {
	p := &preludeReader{f: f, s: lexer.New(f)}
	p.advance()

	for {
		if !p.ok {
			return len(f.Text()), imports // ran out of tokens: the file is all prelude
		}
		start := p.tok.Offset
		imported, ok := p.item()
		if !ok {
			return start, imports
		}
		imports = append(imports, imported)
	}
}

// preludeReader walks significant tokens.
//
// No pushback, and none is needed: an item that fails to parse ends the prelude, so the
// reader stops rather than reconsidering. The only state a failure needs is where the item
// began, and the caller holds that.
type preludeReader struct {
	f   *source.File
	s   *lexer.Scanner
	tok token.Token
	ok  bool
}

// advance moves to the next significant token. Stopping early is just not calling it again —
// what Scanner exists for.
func (p *preludeReader) advance() {
	for {
		p.tok, p.ok = p.s.Next()
		if !p.ok || !p.tok.IsTrivia() {
			return
		}
	}
}

func (p *preludeReader) at(k token.Kind) bool { return p.ok && p.tok.Kind == k }

func (p *preludeReader) text() string { return p.f.Slice(p.tok.Offset, p.tok.Len) }

// atWord matches an identifier by spelling. `from` and `as` are contextual rather than
// reserved (R223), which is what keeps `import { parse as from } from m;` legal.
func (p *preludeReader) atWord(w string) bool { return p.at(token.Ident) && p.text() == w }

func (p *preludeReader) accept(k token.Kind) bool {
	if !p.at(k) {
		return false
	}
	p.advance()
	return true
}

// item parses one prelude item and returns the path it imports. §5's grid (R136), all four
// cells being prelude members (R250):
//
//	import p;              import { a, b } from p;
//	const n = import p;    const n = import { a, b } from p;
//
// `export` may precede either assigned form, re-exporting the collected table.
func (p *preludeReader) item() (string, bool) {
	// Why the stopping condition is a parse decision and not a token test: `const n = 5;` and
	// `const n = import p;` diverge only at the token after the `=`.
	p.accept(token.KwExport)
	if p.accept(token.KwConst) {
		if !p.accept(token.Ident) || !p.accept(token.Assign) {
			return "", false
		}
	}
	if !p.accept(token.KwImport) {
		return "", false
	}
	return p.spec()
}

// spec parses what follows `import`: a bare path, or a braced name list and a from-clause.
func (p *preludeReader) spec() (string, bool) {
	if p.accept(token.LBrace) {
		// Names are skipped wholesale — discovery wants the path, and the list's contents are
		// §1.3's business.
		for !p.at(token.RBrace) {
			if !p.ok {
				return "", false
			}
			p.advance()
		}
		p.advance()
		if !p.atWord("from") {
			return "", false
		}
		p.advance()
	}

	imported, ok := p.path()
	if !ok || !p.accept(token.Semicolon) {
		return "", false
	}
	return imported, true
}

// path parses a dotted module path: IDENT ('.' IDENT)*.
func (p *preludeReader) path() (string, bool) {
	if !p.at(token.Ident) {
		return "", false
	}
	var b strings.Builder
	b.WriteString(p.text())
	p.advance()

	for p.accept(token.Dot) {
		if !p.at(token.Ident) {
			return "", false
		}
		b.WriteString(".")
		b.WriteString(p.text())
		p.advance()
	}
	return b.String(), true
}
