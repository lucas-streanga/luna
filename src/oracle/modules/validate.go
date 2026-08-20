// Import validation: compiler §1.2 (R190, R250, R251).
package modules

import (
	"slices"
	"strings"

	"luna/oracle/diagnostic"
	"luna/oracle/token"
)

// Graph is the validated module DAG.
//
// Layers rather than a flat topological order, because layers are what the consumer wants:
// §2 analyses "modules at the same topological depth in parallel, and the layers proceed in
// dependency order". Concatenating them yields the topological order §1.2 owes, so nothing is
// lost by being specific.
//
// Nodes are module paths, the root's being empty, which is unambiguous only because R251
// makes importing the root an error, so no file answers to two names by the time a Graph
// exists. Adjacency is not repeated here; Result.Edges already holds it.
type Graph struct {
	Layers [][]string
}

// Order flattens the layers into the topological order: every module appears after everything
// it imports.
func (g Graph) Order() []string {
	var out []string
	for _, layer := range g.Layers {
		out = append(out, layer...)
	}
	return out
}

// Validate turns discovery's raw edges into the DAG, and judges everything discovery declined
// to (R250).
//
// It reports, in one pass and without stopping at the first (compiler §3):
//
//   - unresolved imports: an edge naming no file, excluding `std.*`, which reaches no file by
//     construction (R251)
//   - root imports: an edge resolving to the entry's file, which is its own error rather than
//     a cycle, because the fix is to stop importing the entry (R251, modules §3)
//   - cycles: every one of them, each with its full path (R251)
//   - late imports: any KW_IMPORT past a file's PreludeEnd, the prelude rule R250 moved here
//     from §1.3
//
// toks is keyed by File.Path and must hold an entry for every file in res. A missing entry is
// a driver bug, not a program condition, and panics: §1.2 runs after §1.1, so the streams
// exist by construction.
//
// The token streams are needed only for the late-import check. Everything else reads res
// alone, which is why §1.2's DAG half could be computed before §1.1 if an implementation
// wanted to: it is the reporting that waits (R250).
func Validate(res Result, toks map[string][]token.Token) (Graph, diagnostic.List) {
	v := &validator{res: res, file: map[string]File{}, imports: map[string][]string{}}
	for _, f := range res.Files {
		v.file[f.Module] = f
		v.byPath = append(v.byPath, f)
	}

	v.resolve()
	v.preludes(toks)
	v.cycles()
	return Graph{Layers: v.layers()}, v.diags
}

// validator carries what the four checks share. Every traversal runs over res.Files or
// res.Edges in their given order, never over a map, so the diagnostics come out the same way
// on every run, which §2 asks of the whole compiler and differential testing needs.
type validator struct {
	res     Result
	file    map[string]File     // module path -> file
	byPath  []File              // res.Files, in order
	imports map[string][]string // module path -> the in-set modules it imports
	diags   diagnostic.List
}

// resolve turns each edge into an adjacency, reporting the two ways an edge can fail to name
// a module in the set.
func (v *validator) resolve() {
	// The root is found by its empty module path, which modules §3 makes a fact about the
	// program. Discovery also happens to list it first, but position is an artifact of the BFS
	// Reading it from there would keep working until discovery sorted its output or gained a
	// pass, and would then stop reporting root imports rather than fail.
	entry := ""
	for _, f := range v.byPath {
		if f.Module == "" {
			entry = f.Path
			break
		}
	}

	for _, e := range v.res.Edges {
		// `std.*` reaches no file by construction (modules §10, R251): the virtual root has no
		// tree, so excluding it here is what keeps importing the standard library from being a
		// list of unresolved-import errors.
		if reserved(e.To) {
			continue
		}

		target := fileOf(e.To)
		switch {
		case target == entry:
			v.report(diagnostic.RootImport, e, "`%s` is the root module and cannot be imported", e.To)
		case !v.known(e.To):
			v.report(diagnostic.UnresolvedImport, e, "no module `%s` under the source root", e.To)
		default:
			// Deduped: discovery keeps the *raw* edge list (R190), so importing a module twice
			// yields two edges. Adjacency has no use for the second, and leaving it in reports
			// the same cycle once per duplicate: `import b; import b;` in b.luna gave two
			// identical M0003s.
			if !slices.Contains(v.imports[e.From], e.To) {
				v.imports[e.From] = append(v.imports[e.From], e.To)
			}
		}
	}
}

// known reports whether a module path names a file discovery found.
func (v *validator) known(module string) bool {
	_, ok := v.file[module]
	return ok
}

// report adds a diagnostic anchored on the import path that caused it.
func (v *validator) report(code diagnostic.Code, e Edge, format string, args ...any) {
	from, ok := v.file[e.From]
	if !ok {
		// Every edge comes from a file discovery read, so its module is in the set. A miss is a
		// caller assembling a Result by hand, not a program condition.
		panic(diagnostic.Bugf("modules: edge from unknown module %s", e.From))
	}
	span := diagnostic.Span{Filename: from.Path, Offset: e.Offset, Length: e.Len}
	v.diags.Add(diagnostic.New(code, span, format, args...))
}

// preludes enforces the prelude rule, which R250 moved here from §1.3.
//
// Any KW_IMPORT past a file's prelude end is invalid, and the check needs no structure because
// both violations modules §4 names are errors, a late top-level import and one inside a
// function, a block or a conditional, so it never has to tell them apart.
func (v *validator) preludes(toks map[string][]token.Token) {
	for _, f := range v.byPath {
		stream, ok := toks[f.Path]
		if !ok {
			// §1.2 runs after §1.1, so every file has a stream. A missing one is a driver bug,
			// and silently reading nil would turn it into "this file has no late imports".
			panic(diagnostic.Bugf("modules: no token stream for %s", f.Path))
		}
		for _, t := range stream {
			if t.Kind == token.KwImport && t.Offset >= f.PreludeEnd {
				span := diagnostic.Span{Filename: f.Path, Offset: t.Offset, Length: t.Len}
				v.diags.Add(diagnostic.New(diagnostic.ImportOutsidePrelude, span,
					"imports must precede every other top-level declaration"))
			}
		}
	}
}

// cycles reports every cycle, each with its full path (R251).
//
// Three-colour DFS rather than Kahn's: the path is required, and it falls off the stack here
// where Kahn's would only report which modules take part. One diagnostic per back edge, which
// is what "every cycle" can mean without being exponential in a dense graph.
func (v *validator) cycles() {
	const (
		white = iota // unvisited
		grey         // on the current stack
		black        // finished
	)
	colour := map[string]int{}
	var stack []string

	var walk func(module string)
	walk = func(module string) {
		colour[module] = grey
		stack = append(stack, module)

		for _, next := range v.imports[module] {
			switch colour[next] {
			case white:
				walk(next)
			case grey:
				v.reportCycle(stack, next)
			}
		}

		stack = stack[:len(stack)-1]
		colour[module] = black
	}

	for _, f := range v.byPath {
		if colour[f.Module] == white {
			walk(f.Module)
		}
	}
}

// reportCycle names the loop from the module the back edge reached, round to the top of the
// stack and back.
func (v *validator) reportCycle(stack []string, back string) {
	start := 0
	for i, m := range stack {
		if m == back {
			start = i
			break
		}
	}
	loop := append(append([]string{}, stack[start:]...), back)

	f := v.file[back]
	span := diagnostic.Span{Filename: f.Path, Offset: 0, Length: 0}
	v.diags.Add(diagnostic.New(diagnostic.ImportCycle, span,
		"import cycle: %s", strings.Join(named(loop), " -> ")))
}

// named renders a cycle path, the root's empty module path included.
func named(path []string) []string {
	out := make([]string, 0, len(path))
	for _, m := range path {
		if m == "" {
			m = "(root)"
		}
		out = append(out, m)
	}
	return out
}

// layers peels the graph from the leaves up: layer 0 imports nothing in the set, layer n
// imports only modules in layers below it. §2's "modules at the same topological depth
// analyze in parallel".
//
// Modules caught in a cycle never lose their last dependency and so are simply absent. That
// is sound because a cycle aborts the compile at this phase boundary (§3), so no consumer
// sees a Graph with a module missing.
func (v *validator) layers() [][]string {
	var out [][]string
	placed := map[string]bool{}

	for len(placed) < len(v.byPath) {
		var layer []string
		for _, f := range v.byPath {
			if !placed[f.Module] && v.ready(f.Module, placed) {
				layer = append(layer, f.Module)
			}
		}
		if len(layer) == 0 {
			break // everything left is in a cycle
		}
		for _, m := range layer {
			placed[m] = true
		}
		out = append(out, layer)
	}
	return out
}

// ready reports whether every module this one imports is already placed.
//
// Recomputed per round rather than counted down. A counter has to know which of a module's
// imports it has already credited, and getting that wrong double-decrements a shared
// dependency into a negative count the loop then never places. This is O(V·E) and obviously
// right, which is the trade an oracle takes (compiler §6.1).
func (v *validator) ready(module string, placed map[string]bool) bool {
	for _, imp := range v.imports[module] {
		if !placed[imp] {
			return false
		}
	}
	return true
}
