// Discovery: compiler §1.0, the pipeline's stage 0 (R190, R250, R251).
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
// A file ingress rejects (invalid UTF-8, a leading BOM) is still listed, with an empty prelude
// and no edges. Being in the file set is what puts it in front of §1.1, which owns the lexical
// codes and raises the error. Its own imports stay undiscovered, and that is sound because the
// compile aborts at §1.1's boundary before anything could miss them.
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

		module := moduleOf(file, entry)

		f, err := source.New(file, string(src))
		if err != nil {
			// Ingress rejected the bytes, so the prelude cannot be read — but the file is still
			// part of the program. Listing it is what puts it in §1.1's input, and §1.1 is what
			// reports the ingress error; dropping it here would leave nobody to.
			res.Files = append(res.Files, File{Path: file, Module: module})
			continue
		}

		end, imports := readPrelude(f)
		res.Files = append(res.Files, File{Path: file, Module: module, PreludeEnd: end})

		for _, imported := range imports {
			res.Edges = append(res.Edges, Edge{
				From: module, To: imported.path,
				Offset: imported.offset, Len: imported.len,
			})

			// R250 left std resolution unruled and there is no tree to resolve against, so the
			// edge is recorded and nothing followed — committing to neither answer.
			if reserved(imported.path) {
				continue
			}
			if target := fileOf(imported.path); !seen[target] {
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

// moduleOf inverts fileOf for the paths fileOf produces, and the entry, which is the root
// module and has the empty path (modules §3).
//
// Not a total inverse: a directory whose name contains a dot would round-trip to the wrong
// module. No import path can address such a directory — dots become slashes — so the only file
// that could sit in one is the entry, which returns the empty path and never round-trips.
func moduleOf(file, entry string) string {
	if file == entry {
		return ""
	}
	return strings.ReplaceAll(strings.TrimSuffix(file, ext), "/", ".")
}

// importRef is one import the prelude named, and the span of the path as written. The span
// covers the path and not the `import` keyword: the path is the part a diagnostic is about.
type importRef struct {
	path        string
	offset, len int
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
func readPrelude(f *source.File) (end int, imports []importRef) {
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

// atSegment reports whether the current token can be a module-path segment.
//
// Keywords qualify, and that is the point: a path segment is not a name. modules §5 makes
// `import std.filesystem;` bind nothing called `filesystem`, so `test` and `error` in a path
// collide with nothing — while `test/` and `error/` are among the most ordinary directory
// names there are, and §3 maps paths straight onto directories.
//
// The line this must not cross is the braced name list, whose entries *do* bind. Nothing here
// reaches those: spec skips them wholesale, and the `const NAME` of an assigned import still
// demands token.Ident, so a keyword cannot become a binding by this route.
//
// Only the parser's view changes; the lexer still emits KW_TEST and is untouched. Luna
// already resolves one token-kind-versus-role question by position — R223's unreserved
// `from` — and this is the same move in the other direction.
func (p *preludeReader) atSegment() bool {
	if !p.ok {
		return false
	}
	switch p.tok.Kind.Category() {
	case token.Identifier, token.Keyword:
		return true
	}
	return false
}

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
func (p *preludeReader) item() (importRef, bool) {
	// Why the stopping condition is a parse decision and not a token test: `const n = 5;` and
	// `const n = import p;` diverge only at the token after the `=`.
	p.accept(token.KwExport)
	if p.accept(token.KwConst) {
		if !p.accept(token.Ident) {
			return importRef{}, false
		}
		if p.at(token.Colon) && !p.skipAnnotation() {
			return importRef{}, false
		}
		if !p.accept(token.Assign) {
			return importRef{}, false
		}
	}
	if !p.accept(token.KwImport) {
		return importRef{}, false
	}
	return p.spec()
}

// skipAnnotation consumes `: T`, leaving the reader on the `=`.
//
// The type is skipped rather than parsed: §1.3 owns type syntax, and discovery needs only to
// reach the `=` to learn whether an import follows. modules §6 makes the annotation legal —
// an assigned import's type "is `table`, annotatable and inferable" — and dropping the form
// loses its edge with nothing to catch it, the annotated spelling being as valid as the bare
// one.
//
// Bracket depth is tracked so an `=` inside the type cannot be mistaken for the declaration's
// own, and a `;` at depth zero ends the search: that is a declaration with no initializer,
// which is not an import however it is annotated.
func (p *preludeReader) skipAnnotation() bool {
	p.advance() // the `:`

	depth := 0
	for p.ok {
		switch p.tok.Kind {
		case token.LParen, token.LBracket, token.LBrace:
			depth++
		case token.RParen, token.RBracket, token.RBrace:
			depth--
		case token.Assign:
			if depth == 0 {
				return true // left for the caller to accept
			}
		case token.Semicolon:
			if depth == 0 {
				return false
			}
		}
		p.advance()
	}
	return false
}

// spec parses what follows `import`: a bare path, or a braced name list and a from-clause.
func (p *preludeReader) spec() (importRef, bool) {
	if p.accept(token.LBrace) {
		// Names are skipped wholesale — discovery wants the path, and the list's contents are
		// §1.3's business.
		for !p.at(token.RBrace) {
			if !p.ok {
				return importRef{}, false
			}
			p.advance()
		}
		p.advance()
		if !p.atWord("from") {
			return importRef{}, false
		}
		p.advance()
	}

	at := p.tok.Offset
	imported, end, ok := p.path()
	if !ok || !p.accept(token.Semicolon) {
		return importRef{}, false
	}
	return importRef{path: imported, offset: at, len: end - at}, true
}

// path parses a dotted module path, returning where its last token ends. A segment is an
// identifier or a keyword — see atSegment.
//
// The end is tracked rather than inferred from the assembled string, because the two disagree
// wherever trivia sits between the segments: `a . b` is three characters of module path and
// five of source.
func (p *preludeReader) path() (text string, end int, ok bool) {
	if !p.atSegment() {
		return "", 0, false
	}
	var b strings.Builder
	b.WriteString(p.text())
	end = p.tok.Offset + p.tok.Len
	p.advance()

	for p.accept(token.Dot) {
		if !p.atSegment() {
			return "", 0, false
		}
		b.WriteString(".")
		b.WriteString(p.text())
		end = p.tok.Offset + p.tok.Len
		p.advance()
	}
	return b.String(), end, true
}
