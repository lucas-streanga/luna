// Package modules implements the compiler's two module phases: §1.0 discovery and §1.2
// import validation.
//
// One package because R190 makes them two halves of one job: discovery **finds**, validation
// **judges**. Discovery answers *which files* and raises no diagnostic (R250); every import
// error — unresolvable path, cycle, late import — is validation's.
//
// Discovery exists to break the pipeline's bootstrap circle: lexing cannot start without the
// file set, and the file set is written in imports, which only lexing can read.
//
// # Layout
//
//	modules.go   the types both phases pass
//	discover.go  §1.0 — Discover, the BFS walk, the prelude reader, path resolution
//	validate.go  §1.2 — Validate, the DAG, and every import diagnostic
package modules

import (
	"fmt"

	"luna/oracle/diagnostic"
)

// Error is a reason discovery could not proceed, carrying the code that names it.
//
// It is not a diagnostic.Diagnostic, for the reason source.Error is not: there is no file to
// point a span at — the entry named one that does not exist — so the caller converts, adding
// what it knows. The dependency runs one way, modules to diagnostic, which keeps diagnostic a
// leaf and lets a renderer sit above both.
//
// Only M0005 travels this way. Every other module error is raised by §1.2 as a proper
// diagnostic, discovery itself raising none (R250).
type Error struct {
	Code diagnostic.Code
	Path string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Path)
}

// File is one module discovered on disk.
type File struct {
	Path string // slash-separated, relative to the root: `utils/parse.luna`

	// Module is Path with dots for slashes and no extension: `utils.parse`. The root module's
	// path is empty (modules §3) — the one module not named by its file.
	Module string

	// PreludeEnd is the offset of the first token that is not part of an import, or the file
	// length if there is none, so it lands on the declaration rather than the blank line above
	// it.
	//
	// Free to record (the walk stops there) and needed by §1.2, which is what makes its
	// prelude check a filter over §1.1's tokens rather than a second read (R250).
	PreludeEnd int
}

// Edge is one import, from the module that wrote it to the path it named.
//
// To is the *written* path, not a resolved file, and that is the point: an edge survives when
// nothing on disk answers it. Discovery skips the missing file silently and §1.2 reports it
// from here — the no-diagnostics contract without losing the error (R250).
type Edge struct {
	From, To string // module paths

	// Offset and Len span the path as written, in the file From came from. Free — the prelude
	// reader is holding those tokens when it records the edge — and §1.2 cannot do without
	// them: an unresolved import whose diagnostic points at the file rather than at the import
	// is a diagnostic that makes the reader search.
	//
	// Len is measured rather than derived from len(To), because the two differ wherever trivia
	// sits inside the path: `import a . b;` is the module `a.b` written across five bytes.
	Offset, Len int
}

// Result is discovery's whole output.
//
// Deliberately not a graph: R190 has discovery retain raw edges and §1.2 build the DAG. The
// file *set* unlocks lex and parse; the DAG orders only semantic layering.
type Result struct {
	Files []File // reachable from the entry, breadth-first, entry first
	Edges []Edge // every import followed or attempted, in encounter order
}
