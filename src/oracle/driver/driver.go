// Package driver wires the oracle's phases into a batch compile (compiler §1, driver.md).
//
// It is a driver, not *the* compiler: §1 makes the compiler "a library of reusable passes,
// not a monolith", because the formatter, LSP and debugger are each a driver calling the
// passes it needs. This one runs the batch pipeline; cmd/luna is a thin main over it.
//
// # What it owns
//
// Fan-out, ordering, and the decision to stop. Every pass is pure and sequential and reports
// without judging (driver.md §3, §4); the driver decides what runs in parallel, merges the
// results in file order, and enforces §3's rule that a phase cannot consume the broken output
// of the previous one.
//
// Merging in file order rather than completion order is what makes diagnostics deterministic,
// which the oracle owes as the reference for differential testing.
//
// # How far it goes
//
// §1.0 discover, §1.1 lex, §1.2 import validation. There is no parser yet, so the pipeline
// stops where the passes stop and Build returns what it found.
package driver

import (
	"errors"
	"fmt"
	"io/fs"
	"runtime"
	"sync"

	"luna/oracle/diagnostic"
	"luna/oracle/lexer"
	"luna/oracle/modules"
	"luna/oracle/source"
	"luna/oracle/token"
)

// unit is one file's accumulating state, the driver's bookkeeping and deliberately not a
// parameter to anything (driver.md §5).
//
// No pass takes a *unit. A pass takes what it needs, so the formatter can lex one file without
// constructing one of these, which is the library-of-passes property §1 asks for. It grows a
// field per phase: tokens now, then the CST, then the typed AST.
type unit struct {
	file   modules.File
	src    *source.File // nil when ingress rejected the bytes
	tokens []token.Token
	diags  diagnostic.List

	// err is a condition that is not about the program: the file vanished between discovery
	// and here. It travels separately from diags because modules §12 deliberately leaves it
	// uncoded, and a diagnostic needs a code that a spec table names.
	err error
}

// Phase names how far a build got. A phase is *reached* when it ran to completion, so
// Reached is the last one that finished rather than the one that failed.
type Phase uint8

const (
	PhaseDiscover Phase = iota // §1.0 — the file set exists
	PhaseLex                   // §1.1 — every file has a token stream
	PhaseValidate              // §1.2 — the graph exists
)

func (p Phase) String() string {
	switch p {
	case PhaseDiscover:
		return "discover"
	case PhaseLex:
		return "lex"
	case PhaseValidate:
		return "validate"
	}
	return "unknown"
}

// Result is what a build produced. It grows a field per phase: the CST and the typed AST land
// here as the parser and the checker arrive.
type Result struct {
	// Diagnostics is everything wrong with the program, ordered by file and within a file by
	// the order its phase found them.
	Diagnostics diagnostic.List

	// Files is the module set that was compiled, in discovery's breadth-first order.
	Files []modules.File

	// Graph is the validated DAG. Meaningful only once Reached is PhaseValidate.
	Graph modules.Graph

	// Reached is the last phase that completed.
	//
	// It is here because no other field answers the question. An empty Graph.Layers looks like
	// "§1.2 never ran", but a graph whose every module sits in a cycle is also empty, so
	// inferring the phase from the graph would be a fact derived from something not
	// authoritative for it, and would read wrong exactly when a cycle is what went wrong.
	Reached Phase
}

// Build runs the batch pipeline over a source tree.
//
// fsys is rooted at the module root and entry names the root module's file within it, the
// convention modules §3 sets and modules.Discover documents.
//
// The two returns are two different kinds of news, and the split is the same one discovery
// already makes. The **list** is what the program got wrong, ordered by file and within a
// file by the order its phase produced them. The **error** is that the compile could not be
// run at all (a malformed entry path, or a file that vanished mid-build), which modules §12
// deliberately allocates no code for, since it is not a claim about the program.
//
// A non-nil error means the list is not a verdict. It is the whole result for now: with no
// parser there is nothing further to hand back, and a caller wanting the graph calls the
// passes directly.
func Build(fsys fs.FS, entry string) (Result, error) {
	// §1.0, discovery, which raises no diagnostics of its own (R250). M0005 is the one code
	// that reaches here, carried on the error; everything else it can fail with is uncoded.
	found, err := modules.Discover(fsys, entry)
	if err != nil {
		var me *modules.Error
		if errors.As(err, &me) {
			return Result{Diagnostics: diagnostic.List{diagnostic.New(me.Code,
				diagnostic.Span{Filename: me.Path}, "no such entry file: %s", me.Path)}}, nil
		}
		return Result{}, err
	}
	res := Result{Files: found.Files, Reached: PhaseDiscover}

	// §1.1, lex, all files in parallel (§2: no symbol knowledge exists, so nothing orders it).
	units := lexAll(fsys, found.Files)
	for _, u := range units {
		if u.err != nil {
			return Result{}, u.err
		}
		res.Diagnostics = append(res.Diagnostics, u.diags...)
	}
	if !res.Diagnostics.Empty() {
		// §3: a phase cannot meaningfully consume the broken output of the previous one. This
		// is also what makes R251's ingress hole sound: a file whose imports went unread
		// cannot reach a phase that would miss them.
		return res, nil
	}
	res.Reached = PhaseLex

	// §1.2, import validation.
	toks := make(map[string][]token.Token, len(units))
	for _, u := range units {
		toks[u.file.Path] = u.tokens
	}
	graph, vdiags := modules.Validate(found, toks)
	res.Graph, res.Diagnostics, res.Reached = graph, vdiags, PhaseValidate
	return res, nil
}

// lexAll runs §1.1 across the file set, returning one unit per file **in the order given**.
//
// Results land in a preallocated slice indexed by position, so the merge above is by file
// rather than by whichever goroutine finished first. Determinism is structural here, not
// something a test has to notice.
//
// Bounded to one worker per CPU. Unbounded would also work, Go scheduling blocked goroutines
// fine, but the bound is the driver making the fan-out decision §3 says is its to make,
// rather than leaving it to the size of the file set.
func lexAll(fsys fs.FS, files []modules.File) []*unit {
	units := make([]*unit, len(files))

	sem := make(chan struct{}, max(1, runtime.NumCPU()))
	var wg sync.WaitGroup
	for i, f := range files {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			units[i] = lexOne(fsys, f)
		}()
	}
	wg.Wait()
	return units
}

// lexOne ingests and lexes a single file.
//
// Ingress happens here, and it has to: lexer.Lex takes a *source.File, which for rejected
// bytes never exists, so oracle/lexer structurally cannot raise L0001 or L0002. compiler §1.1
// is the *phase* that reports them and the driver is what implements that half of it, which
// is also why discovery listing an ingress-rejected file (R251) is what puts it in front of
// this function at all.
func lexOne(fsys fs.FS, f modules.File) *unit {
	u := &unit{file: f}

	src, err := fs.ReadFile(fsys, f.Path)
	if err != nil {
		// Discovery read this file moments ago, so failing now means the tree changed under
		// the build. Not a claim about the program, and modules §12 leaves it uncoded.
		u.err = fmt.Errorf("driver: reading %s: %w", f.Path, err)
		return u
	}

	u.src, err = source.New(f.Path, string(src))
	if err != nil {
		u.diags.Add(fromSourceError(err, f.Path))
		return u
	}
	u.tokens, u.diags = lexer.Lex(u.src)
	return u
}

// fromSourceError converts an ingress failure into a diagnostic.
//
// source.Error carries the code and the offset but no filename, package source having no *File
// to name after refusing to build one. So the caller supplies it, which is the conversion
// its doc describes and nothing had performed until now.
func fromSourceError(err error, path string) *diagnostic.Diagnostic {
	var se *source.Error
	if !errors.As(err, &se) {
		// source.New returns nothing else. Asserting rather than inventing a code keeps a
		// future third failure mode from arriving mislabelled as a UTF-8 problem.
		panic(fmt.Sprintf("driver: unexpected ingress error %T: %v", err, err))
	}
	span := diagnostic.Span{Filename: path, Offset: se.Offset}
	return diagnostic.New(se.Code, span, "%s", describeIngress(se.Code))
}

// describeIngress is a placeholder for the rendering layer that does not exist yet. A
// description is per-instance and volatile (§11, §12), so these will be replaced wholesale
// once diagnostics have a renderer of their own.
func describeIngress(code diagnostic.Code) string {
	switch code {
	case diagnostic.ByteOrderMark:
		return "a source file may not begin with a byte-order mark"
	case diagnostic.InvalidUTF8:
		return "source is not valid UTF-8"
	}
	return code.Title()
}
