# Compiler implementation

This document specifies the high-level structure of the Luna compiler: its phase pipeline, where
work parallelizes, the typed intermediate representation (the **Luna IR**), the optimization
passes that run on it, how compile-time evaluation (comptime) fits, and how the IR is lowered to
**Go source** for the Go toolchain to build. It is an implementation blueprint, not a
user-facing language spec; it refers throughout to the language specs that fix the semantics it
implements.

The two load-bearing choices:

- **Generate Go, not VM bytecode.** Luna's runtime *is* Go: garbage collection is Go's GC
  (value-representation §6), and green threads are goroutines. A custom bytecode VM would discard
  the reason Go was chosen. So the backend emits Go source and hands it to the Go toolchain,
  inheriting Go's register allocation, machine-code generation, GC, and scheduler.
- **Keep a typed IR between analysis and Go.** The compiler does **not** emit Go straight from the
  AST. It lowers the typed AST to a **Luna IR** (a typed, lowered tree, not a linear instruction
  stream), runs Luna-semantic optimization passes there, and emits Go from the optimized IR. This
  is where the language specs' optimizations live (constraint-check elision, const-table
  specialization, protocol-member resolution — R144) and where **comptime** is evaluated, none of which
  Go's compiler can do, because by the time it sees Go source the Luna semantics are gone.

---

## 0. Target

The compiler targets **`linux-x86-64` only, for now**. This is a sequencing decision, not a
design one: the Go backend is inherently multi-platform, but pinning one target lets
platform-relative surfaces (`std.platform`, the `path` predicate, the errno grounding of
`std.io`'s error hierarchy, io-errors spec) be specified concretely against one ABI instead of
abstractly against all of them. Widening the target set revisits exactly those named
surfaces and nothing else.

## 0.1 The `luna` binary: flags and composition

One binary, the complete suite. Known flags:

| Flag | Long | Function |
|-|-|-|
| *(none)* | | `luna someProgram`: **run**, via the binary cache (below) |
| `-c` | `--compile` | AOT compile to a binary artifact |
| `-o` | `--output` | output path for `-c`; an error without `-c` |
| `-t` | `--test` | run the program's tests (tests spec §5); accepts a name filter |
| `-f` | `--format` | format every file discovered by module resolution (formatter spec pending) |
| `-l` | `--lsp` | run the language server on stdio |
| `-d` | `--debugger` | run the program under the debugger |

**Run mode and the binary cache.** `luna someProgram` compiles (if needed) and runs. Builds
are **content-addressed**: the cache key is a hash over the resolved module set's sources
plus the compiler version **and the compile target** (R149 — platform facts are target
facts that fold at comptime, R138, so per-target artifacts differ by design; the build-cache
spec §3 carries the same dimension), so unchanged programs skip straight to execution, and the
determinism that comptime already guarantees (§ the comptime evaluator; functions §5.5) is
what makes the cache sound, same inputs, same binary, always.

**`-c` and the output name.** With `-o`, the artifact goes exactly there. Without it, the
output name is the program name (extension dropped). If that path already exists, it is an
**error, never a silent overwrite**, with one refinement: a Luna-built binary embeds a build
marker, and an existing file *bearing the marker* is fair to replace (otherwise every
recompile would trip on its own previous output); a foreign file is never clobbered.

**Composition.** Flags compose where the composition means something, and the canonical
pipeline order is fixed regardless of flag order on the command line:

```
format  ->  build  ->  test  ->  (on success) artifact / run / debug
```

- **`-t`, `-f`, `-c` compose freely**: `luna -f -t -c prog` formats, builds, tests, and
  emits the artifact only if the tests pass (tests spec §5, `luna -t -c`).
- **`-d` composes with `-c`** (a debug session compiles first anyway; `-c` additionally
  keeps the artifact, built with debug info) and excludes `-l`. Debugging a single test
  (`-d -t 'name'`) is attractive and deferred with the debugger spec.
- **`-l` composes with nothing.** The language server is a long-running JSON-RPC process
  that **owns stdio** and never terminates on its own; it is launched by an editor, not by
  people, and combining it with any batch-mode flag is incoherent, an error. (The server
  itself is a large deferred spec; the phase pipeline's error-tolerant analysis, §1.4, is
  its foundation.)

Both `-l` and `-d` details, and the formatter, are pending their own specs; this table is
the stable surface they will slot into.

## 1. The phase pipeline

```
source
  -> lex                 (tokens)
  -> module resolution   (DAG)
  -> parse               (lossless CST -> AST)     [parallel per module]
  -> semantic analysis   (typed AST)               [parallel by DAG layer]
  -> lower to Luna IR    (typed IR)
  -> optimize IR         (comptime, elision, specialization, member resolution)
  -> emit Go             (Go source per module)    [parallel per module]
  -> assemble + build    (Go program -> Go toolchain -> binary)
```

Each phase is described below. The parallelism structure (§2) and the error model (§3) cut across
all of them.

The compiler is structured as a **library of reusable passes**, not a monolith, because the
developer tools (formatter, LSP, debugger) are provided by the compiler itself and each is a driver
that calls the passes it needs (tooling spec §1). This, the **lossless, error-tolerant** frontend
(§1.3, §3), and the **unoptimized debug build** mode (§7.1.1, tooling spec §5) are foundational
choices made from the start because they shape the whole frontend and cannot be retrofitted cheaply.

### 1.1 Lex

Tokenizes each source file. Invalid tokens are **collected**, not aborted on individually (§3):
the lexer scans the whole file, accumulates every lexical error, and the compile aborts at the
phase boundary if any occurred. Output is a token stream per file.

### 1.2 Module resolution

Builds the module dependency graph from imports (modules spec). The graph must be a **DAG**;
a cycle, or an unresolved import, aborts the compile (modules spec §2). Resolution fixes each
module's identity by its path relative to the root (empty path for the root, modules spec §3) and
produces a topological order. **This phase is the gate for all downstream parallelism**: once the
DAG exists, per-module work can proceed concurrently within the ordering the DAG imposes (§2).

### 1.3 Parse

A pure **context-free** pass: tokens to a syntax tree, per module, with no type knowledge and no
name resolution. Parsing is **fully parallel** across modules (§2), each module's tokens are
independent once resolution has run. Parse errors are accumulated per module; the batch driver
aborts at the phase boundary (§3), but the parser itself is **error-tolerant** and produces a
best-effort partial tree, which the tooling drivers consume (tooling spec §3).

The parser produces a **lossless concrete syntax tree (CST)**: full-fidelity, retaining all trivia
(comments, whitespace, original spelling) so the exact source is reconstructable. The compiler
ignores trivia and works from the AST view of this tree; the **tools** (formatter, LSP) require the
trivia, so the lossless CST is the shared root (tooling spec §2). Producing a lossless,
error-tolerant tree is a frontend property fixed from the start because it cannot be retrofitted
cheaply.

Parsing is *only* syntax. It does not consult imports, resolve names, or assign types; those are
semantic analysis (§1.4). Keeping parse free of semantics is what lets it run before any
cross-module information is available, and what keeps the grammar context-free.

### 1.4 Semantic analysis

Turns the untyped AST into a **typed AST** by **name resolution** followed by **type checking**.
This is the one phase with a cross-module ordering constraint: a module needs its imports' **public
signatures** (the types of the names it imports) before it can resolve and check its own uses of
them. So analysis proceeds **in DAG order, parallel within each topological layer** (§2).

Two sub-steps, per module:

- **Name resolution.** Bind every identifier to its declaration: locals to their block scope
  (variables spec §4), imported names to the exporting module's bindings (modules spec), builtins
  to the ambient set, `std` names to the reserved virtual root (modules spec §10). Collision and
  unresolved-name errors are raised here.
- **Type checking.** Assign each expression its type (a `typeid`, value-representation §3), check
  every assignment against its location's declared type (variables spec §1), check `as`/`is`
  narrowings (as spec), constraint entry points (constraints spec §7), protocol application and
  member contracts (protocols spec), function compatibility (functions spec §3), match
  exhaustiveness (match spec §9), enum payload shapes (enum spec §3.1), capability `use` clauses
  and comptime-eligibility (functions spec §2, §5). No warnings are ever produced; every
  diagnostic is an error (§3).

To widen parallelism, analysis may run a lighter **signature-extraction** sub-pass first (resolve
and type only the *exported* surface of each module), so dependents can begin on signatures before
their dependencies' *bodies* finish checking. Signature extraction is itself DAG-ordered but
cheaper, so it unblocks more modules sooner.

The output is a **typed AST**: every node annotated with its resolved type, every name bound to a
declaration, every dispatch site classified (element vs meta, static vs dynamic key, tables spec
§3.2).

#### 1.4.1 No control-flow analysis (a hard implementation guarantee)

Luna's compiler performs **no control-flow analysis**. No pass tracks facts along execution paths
within a function, and no pass tracks facts across function boundaries beyond reading declared
signatures. This is a firm implementation guarantee, not an aspiration, and it is what keeps the
compiler free of whole-program and borrow-checker-class analysis. Concretely, none of the following
is performed:

- **No definite-assignment analysis.** "Is this variable assigned before use?" is never asked,
  because an uninitialized binding cannot exist: every declaration must be initialized (variables
  spec §1.3), and "not set yet" is an explicit optional `null`, not an uninitialized slot. There is
  no unassigned state to track.
- **No flow-narrowing.** A binding's type is fixed at its declaration and never changes based on the
  branch it is in. `is` is a boolean test that does **not** narrow the tested binding (as spec §7);
  narrowing happens only by producing a **new binding** via `as` or a `match` arm. So a narrowed type
  is always a property of a fresh binding at its declaration, never a path-dependent fact about an
  existing one.
- **No definite-return analysis.** "Does every path return a value?" is not checked; a function that
  falls off the end returns `undefined` (undefined spec), and using that result where a real value is
  required fails at the use site (a compile error if statically `undefined`, a panic otherwise), so
  the missing return surfaces without path analysis.
- **No reachability or divergence analysis.** A `fn (): never` that actually returns is not caught
  statically (that would be the halting problem in general); it panics at runtime (never spec).
- **No move, linearity, or use-after-consume analysis.** Single-pass stream consumption, builder
  transfer, and promise use are **runtime** properties: an exhausted stream, a **taken** stream
  or builder (transferred across a spawn boundary, concurrency §2.3), or a spent promise fails at
  runtime, not through static move-tracking. Ownership transfer marks the value's **referent**
  taken (value-representation §2.1) and any later access through any alias panics, a runtime
  referent check, precisely because proving "the spawner stopped using it on this path" would be the
  move-tracking this guarantee forgoes. (Optional, purely additive lints over obvious straight-line
  cases may be added later; they are never required for the core compile and never gate it.)

The positive principle underneath: **every static fact Luna checks lives in a type**, errorability
(`!`), capabilities (`use`), nullability (`?`), union and constraint membership, established by
structural rules, by **local per-function checks** (errorability and nullability, both declared
type suffixes verified against the body, not inferred, functions spec §4), or by **call-graph
fixpoints** (comptime-eligibility and capabilities) that read declared signatures, never by
path-sensitive analysis. Every fact that *would* require path-sensitivity is instead either
**dissolved by construction** (mandatory initialization removes the unassigned state; new-binding
narrowing removes the path-dependent type) or made a **runtime property** checked at the point of
violation (divergence, stream consumption, the branch-dependent write-once flag, variables spec
§1.2). Facts live in **types** or in **runtime state**, never in "what is true on this path," which
is precisely the thing control-flow analysis computes and Luna does not.

The call-graph fixpoints for **comptime-eligibility and capabilities** are **not** an
exception to this. They are monotone least-fixpoint computations over the static call graph that read
each callee's declared signature; they do not track intra-function paths or path-sensitive state, so
they are ordinary type-level propagation, not flow analysis. **Errorability is not among them:** it is
**declared, never inferred**, and verified by a **local, syntactic containment check** per call site
(functions spec §4, errors spec §7), which reads callee signatures but performs no propagation,
simpler than a fixpoint and equally free of path-sensitivity.

### 1.5 Lower to Luna IR

The typed AST is lowered to the **Luna IR** (§4): a typed, explicit, lowered tree. Lowering makes
implicit semantics explicit, coalescing operators become branches, `match` becomes decision
logic, ranges-as-streams become stream constructors (range spec), string interpolation becomes
builder calls (string-builder spec), UFCS calls are resolved to their targets, `.`/`[]`/`->`
become concrete access operations. After lowering, the IR contains no syntactic sugar and no
unresolved dispatch: every operation is a concrete, typed node.

### 1.6 Optimize IR

The Luna-semantic optimization passes (§5) run on the IR: **comptime evaluation** (§6),
**constraint-check elision**, **const-table specialization**, **protocol-member
resolution** (R144), and ordinary constant folding and dead-code elimination where Luna semantics
make them safe. These are the optimizations Go cannot perform because they depend on Luna-level
invariants (a constraint predicate, a frozen table's shape, a statically-known protocol set) that
are erased by the time Go source exists.

### 1.7 Emit Go

The optimized IR is lowered to **Go source**, per module, **fully parallel** (§2): once every
module is analyzed and optimized, each module's emission is independent. Emission maps the IR onto
Go constructs (§7): the `lval` onto a Go struct, the `typetable` onto emitted Go data, green
threads onto goroutines, Luna's error model onto Go panic/recover and returned lvals, `defer` onto
scoped cleanup (defer spec).

**Valid IR implies valid Go, so a Go compile failure is always a compiler bug.** By the time
emission runs, the IR has passed semantic analysis (§1.4) and is well-typed by construction. So
every user error, lexical, syntactic, semantic, has already been reported *before* emission, and
the emitted Go is guaranteed to compile. This fixes a clean error-domain split:

- **All user errors live before emission** (lex, parse, analyze). Users never see a Go compile
  error.
- **A Go compile failure is an internal compiler error (ICE)**, never surfaced as if the user did
  something wrong. If valid IR produces Go that Go rejects, the emitter is wrong, and it is
  reported as a compiler bug.

A consequence is that user-facing **source mapping is only needed for runtime positions**, not
compile-time ones: there are no user-visible Go compile errors to map back, only runtime
panics, which arise in valid emitted Go and are mapped to Luna source with Go `//line` directives
so stack traces report Luna file and line.

### 1.8 Assemble and build

The emitted Go modules are assembled into a Go program: the **deterministic module-init order**
(modules spec §2) is emitted as an explicit ordered init sequence (not left to Go's package-init
order, §7.4), the **builtin runtime** is included, and **only the opted-in `std` modules** that
the program actually references are linked (tree-shaken, modules spec §10). The Go toolchain then
compiles and links the program to a binary. Linking of the native code is the Go toolchain's job;
the compiler's job is to emit a correct, self-contained Go program with the right init order and
the right std subset.

---

## 2. Parallelism model

The module DAG (modules spec §2) is what makes the compiler parallel, and the parallel structure
is **not** uniform across phases. It is:

- **Parse: unordered parallel.** Every module parses independently and simultaneously right after
  resolution. No cross-module dependency exists at the syntax level.
- **Semantic analysis: DAG-layered parallel.** A module analyzes only after its imports'
  signatures are available, so modules at the same topological depth analyze in parallel, and the
  layers proceed in dependency order. Signature extraction (§1.4) widens this by unblocking
  dependents on signatures before bodies complete.
- **Emit Go: unordered parallel.** Once all modules are analyzed and the IR optimized, emission is
  independent per module again.

So the shape is **free / layered / free**: parse and codegen are embarrassingly parallel, and the
one ordering constraint lives in analysis, where cross-module types must flow along the DAG. This
is the concrete payoff of the no-cycles rule (modules spec §2): a cyclic module graph would defeat
the topological layering and force whole-program analysis; the acyclic graph gives clean parallel
layers and deterministic ordering.

---

## 3. Error model: accumulate within a phase, abort at the boundary

Each phase **accumulates** all the errors it finds and aborts at the **phase boundary**, rather
than stopping at the first error. Lexing reports every bad token, parsing every syntax error,
analysis every type error, before aborting. This is a deliberate developer-experience choice:
one-error-at-a-time compilation is painful, so each phase surfaces as many independent errors as
it safely can.

- **No warnings, ever.** Every diagnostic is an error (a firm language-project rule): there is no
  warning severity to ignore. A condition is either an error that stops the build or it is not
  reported.
- **Abort at boundaries, not mid-phase.** A phase runs to completion collecting diagnostics; if it
  produced any, the compile stops before the next phase (a phase cannot meaningfully consume the
  broken output of the previous one). Within a phase, error recovery is best-effort to find more
  independent errors (e.g. resynchronizing the parser at statement boundaries).
- **Compile errors carry codes** (variables spec §7 and elsewhere), so diagnostics are stable and
  referenceable.
- **Tolerance is a pass property; aborting is a driver policy.** The passes recover from errors and
  produce a best-effort partial result (a partial CST, a partial typed AST); the **batch driver**
  chooses to discard that partial result and abort at the boundary, but the **tooling drivers** (the
  LSP especially) consume it and never abort (tooling spec §3). So the same error-tolerant frontend
  serves both: a binary is never built from broken input, yet the LSP still gives types and
  completions for the valid parts of half-edited code.

---

## 4. The Luna IR

The Luna IR is a **typed, lowered tree**, not a VM bytecode and not a linear SSA instruction
stream. It is the AST after lowering (§1.5), with all sugar removed and every node carrying full
type and representation information. It exists to (a) hold Luna-semantic optimizations (§5), (b) be
the thing comptime evaluates (§6), and (c) be a clean lowering target for Go emission (§7).

What every IR node carries and makes explicit:

- **A resolved type**, the node's `typeid` (value-representation §3), so representation decisions
  (§7.1) and type-directed lowering are local.
- **A representation classification**, whether a value is an inline scalar (`int`, `double`,
  `bool`), an inline short string, or a pointer-to-managed payload (`table`, `stream`, long
  `string`, error), following the `lval` layout (value-representation §1). This lets emission
  choose the Go representation without re-deriving it.
- **Explicit dispatch sites.** Element access (`.` static-key, `[]` dynamic-key) and meta dispatch
  (`->`) are distinct node kinds (tables spec §3.2, §3.3), and protocol-member accesses carry
  their resolved protocol and, where statically known, the applied-set fact (enabling static
  member resolution, §5, R144).
- **Explicit constraint boundaries.** Every point where a value *enters* a constrained type
  (construction, assignment to a constrained slot, `as`, constraints spec §7) is a marked node
  carrying the predicate, so the optimizer can elide the check where it is provably satisfied
  (§5) or emit it otherwise.
- **Comptime-eligibility marks.** Subtrees that are comptime-eligible (functions spec §5) are
  marked so the comptime evaluator (§6) knows what it may evaluate.
- **Explicit control and cleanup.** `match` is lowered to decision logic (match spec), coalescing
  to branches (coalescing spec), loops to their consuming form over streams/ranges (control-flow
  spec), and `defer` registrations to explicit scoped-cleanup nodes (defer spec; the lowering is §7.3 below, R148).
- **Capability threading.** `use` clauses and the resulting reference captures (functions spec §2)
  are explicit, so emission can thread captured references and so the comptime sandbox can verify
  the absence of capability use (§6).

The IR is **not** a place where new *types* appear: the type universe is fixed at compile time
(value-representation §4.1), so the IR references the already-interned `typetable` entries and
never mints a type. Lowering and optimization only rewrite *expressions and operations*, never the
type set.

---

## 5. Optimization passes on the IR

These run on the Luna IR (§1.6). Each depends on a Luna-level invariant that Go's compiler cannot
see, which is the reason the IR must exist.

- **Comptime evaluation** (§6). Evaluate comptime-eligible subtrees and replace them with their
  computed constants (folded values, fully-built `const` tables). This is both an optimization and
  a language feature (functions spec §5).
- **Constraint-check elision** (constraints spec §11). At a constraint boundary (§4), if the
  compiler can trivially prove the value already satisfies the predicate, a literal in range, a
  value freshly produced as that constrained type, drop the runtime predicate call. This never
  changes meaning (it removes a check that would always pass); it is distinct from the rejected
  static-verification model, which would *replace* runtime checking with proof obligations.
- **Const-table specialization** (tables spec Amendment A). A `const` table gets frozen-storage
  benefits; a *compile-time-shaped* `const` table (built via comptime) additionally gets the
  contiguous-struct layout with a perfect hash, and static `.` accesses on it lower to direct
  offset loads rather than hash probes. This pass chooses the representation and rewrites the
  access nodes.
- **Protocol-member resolution** (protocols spec). Where a table's applied set is statically
  known at a `->` site (a `@P`-typed binding, protocols §6.2), resolve the member to a direct
  access instead of a runtime applied-set check. Where it is not known, emit the dynamic
  check (protocols §3.2). (Pre-R95 this pass spoke of views and meta-function lookups; the
  member model has no virtual dispatch to devirtualize — protocols §2.1 — so the pass is
  resolution, not devirtualization.)
- **Ordinary folding and dead-code elimination**, applied where Luna semantics make them sound
  (pure expressions, unreachable arms). Go repeats much of this on the emitted code, but doing the
  Luna-level cases here is what enables the passes above (e.g. folding a constant key so a const
  table access can specialize).

Representation selection (inline vs pointer, §7.1) is driven off the type annotations the IR
already carries and is applied during or just before emission.

---

## 6. Comptime evaluation

Comptime (functions spec §5) runs Luna code **at compile time**, over the Luna IR, inside the
capability sandbox. The compiler includes an **IR evaluator** (an interpreter over the typed IR)
that executes comptime-eligible subtrees and substitutes their results back into the IR as
constants.

- **It evaluates the IR, not generated Go.** Because comptime runs *during* compilation, it is far
  cleaner to interpret the IR than to generate-and-run Go mid-compile. This is an independent
  reason the IR must exist: comptime needs something evaluable before any Go is emitted.
- **The capability sandbox is enforced statically and dynamically.** Comptime forbids using
  any **non-comptime** capability (functions spec §5.5): a comptime-eligible subtree provably
  reaches no outside authority (checked in analysis, and vacuously true besides, since no
  non-comptime capability instance exists before runtime, capabilities spec §8), so it can
  touch no outside world. The evaluator additionally runs under the liveness
  guards, deterministic execution budget (fuel, not wall-clock), stack-depth limit, and allocation
  ceiling (functions spec §5.5), each aborting with a compile error rather than hanging the build.
- **It is deterministic.** The budget is a step counter, not wall-clock time, so comptime results
  are reproducible and cacheable across machines (functions spec §5.5). Determinism across runs is
  complemented by **phase invariance** across phases (functions spec §5.5): a comptime-eligible
  function yields the same result at comptime as it would at runtime, which is what makes
  substituting the folded result behavior-preserving.
- **Comptime-produced values splice as constants, functions included.** The evaluator substitutes
  a comptime result into the IR as a constant. A **`fn` value** is spliceable exactly when its
  captured environment is **confinement-free plain data**: the code pointer references a literal
  already in the program, and the environment is a `const` snapshot (functions spec §2.1) of
  ordinary values. A `comptype` value, or any environment containing one, cannot be spliced,
  lowering erases comptime provenance and confined values may not survive it (introspection spec
  §4.2), so a generator that tried to capture its descriptor fails to compile at the capture,
  and what reaches the runtime program is always plain data (attributes spec §4). Determinism here is what
  keeps builds reproducible.
- **Its main product is `const` data.** Comptime-built `const` tables become the
  compile-time-shaped tables that get the perfect-hash struct layout (§5, tables Amendment A), so
  comptime and const-table specialization compose: comptime builds the table, specialization lays
  it out.

---

## 7. The Go backend

Emission maps the optimized IR onto Go source. The mapping is direct because the language was
designed against a Go runtime.

### 7.1 The `lval`: a three-word Go struct, with static unboxing

The `lval` (value-representation §1) maps to a Go struct. Its logical form is a 16-byte
discriminated union (a `typeid`/flags word plus a second word that is *either* an inline scalar
*or* a pointer), but **Go's precise GC forbids the single punned word**, so the physical layout is
**three separate 8-byte words** (24 bytes):

```go
type lval struct {
    tagAndType uint64          // flags + string-inline field + typeid (scalar, never traced)
    scalar     uint64          // inline payloads: int, double bits, bool, short string (never traced)
    ptr        unsafe.Pointer  // managed payloads: *table, *string, *stream (always traced); nil for scalars
}
```

The pointer and scalar payloads must be **separate fields** because Go's GC decides "trace this
word?" from the static field type at scan time and **never reads our `typeid`** to defer that
decision (value-representation §1.1 records the full rationale). So `ptr` is always a real pointer
or nil (safe to trace) and `scalar` is always non-pointer bits (safe to skip); the `typeid` then
disambiguates *within* each word (`int` vs `double` in `scalar`, `*table` vs `*stream` in `ptr`),
which is a same-GC-class overlap Go permits. This is a settled layout, not an open task: the
single-word 16-byte form is available only under a self-hosted GC (§11, the native alternative),
not on Go's collector.

The 24-byte value appears only where a value is **genuinely dynamic**; where its type is
statically known, emission does **not** materialize an `lval` at all (§7.1.1).

### 7.1.1 Static unboxing

The primary value-performance mechanism is **static unboxing**: wherever the IR's per-node type
(§4) is a concrete type rather than `any`, emission produces the **raw Go representation** and no
`lval`, an `int` becomes a Go `int64`, a `double` a `float64`, a `bool` a `bool`, a table an
`*Table`. Arithmetic, comparisons, and calls on statically-typed values are then ordinary Go
operations on Go primitives, with no tag checks and no three-word value.

The `lval` is materialized only at **dynamic sites**: `any`-typed values, heterogeneous table
elements (element type `any`), and union values before narrowing (`as`/`is`). Boxing (scalar into
an `lval`) happens at the boundary where a typed value flows into an `any` slot; unboxing (`lval`
back to a Go primitive) happens where an `any` is narrowed to a concrete type. Because the type
system is precise (most hot code is monomorphic, not `any`), the three-word `lval` is off the hot
path, and its byte count is not the lever that matters. This is why the physical size is accepted
rather than optimized: the win is avoiding the `lval` on typed paths, not shrinking it.

**Debug builds disable static unboxing** (and the other representation-destroying optimizations,
constraint elision, comptime folding), so that **every Luna value is a real, findable `lval`** in
the frame and matches the exported `typeinfo` the debugger reads (tooling spec §5). The emitted Go
is likewise built unoptimized in debug mode, so Go's DWARF faithfully maps Go to native. Debug
builds trade this performance for a frame that exactly matches the Luna source; release builds unbox
freely.

### 7.2 The `typetable`

The `typetable` (value-representation §4) is emitted as **static Go data**: a global array of
`typeinfo` values built at compile time, holding nullability, attributes, error ancestry with the
preorder `enter`/`size` interval numbering (value-representation §4.2), and protocol metadata.
Because the type universe is closed at compile time (value-representation §4.1), this table is
fully known during emission and never grows at runtime: subtype tests **over the nominal tree**
compile to the two integer comparisons of the interval check, unions and intersections decompose
over their statically tiny member lists, and function **signatures** index a pairwise assignability
table folded at link time, their relation being a DAG the interval numbering cannot encode
(value-representation §4.2).

### 7.3 Control, errors, and cleanup

- **Green threads to goroutines.** Luna's green threads map to goroutines; the enforced-copying
  discipline (value-representation, functions specs) is realized by the IR inserting copies at task
  boundaries, so tasks share no mutable state. (The concurrency model is **complete** —
  R115–R119, R142: cancellation, channels, the timeout family — and the runtime obligations it
  places here are the per-task state the defer lowering already enumerates: the cancellation
  flag, the promise, the defer list, the shield bit, §7.3.)
- **Error model.** Luna panics (errors §9) map to Go panic/recover: a panic propagates ambiently
  and `try` recovers it at the boundary, converting it to a value. Errorable results (`!`,
  value-representation §3.1) need **no special calling convention**: an errorable value is just an
  `lval` with its **error bit** set and its **`typeid`** giving the concrete error subtype. Errors
  are lvals like every other value, so an errorable return is returned as an ordinary `lval`, and
  `try` is a check of the error bit (with the subtype read from the `typeid`). There is no tagged
  tuple, interface, or side channel; the representation is the same 16-byte `lval` used
  everywhere.
- **`defer` to scoped cleanup — implementable, two lowerings, ruled (R148).** Luna's `defer` is
  **block-scoped** (defer spec §1), whereas Go's is function-scoped, so it cannot map one-to-one
  onto Go `defer` *at function granularity* — but the differences end there: registration-on-reach
  with by-value operand capture (defer spec §4) is exactly Go's argument-evaluation rule, LIFO
  matches, and panic-in-defer supersession with remaining-defers-run (defer spec §6) is Go's own
  behavior. Two sound lowerings:

  1. **Blocks become functions**: each defer-carrying block wraps in an immediately-invoked Go
     func holding real Go `defer`s — block exit is function exit, and Go's machinery delivers
     every §3/§4/§6 property natively; `return`/`break`/`continue` crossing the wrapper use the
     classic control-signal return the emitter switches on. Simple to verify; costs a closure
     per defer-carrying block.
  2. **The zero-cost hybrid (the target)**: on normal exit edges, *no runtime machinery* — the
     pending-defer set at each edge is statically known modulo conditionals, so emission inlines
     guarded cleanup calls (a registered-flag per conditional defer, the destructor-flag
     lowering). Panic paths use a **per-task defer list with block-depth markers**, drained at
     recovery sites — and the constraint that decides *where* is `catch (p: panic)` (errors §8):
     panics are catchable mid-stack, so defers between the throw and an intervening catch must
     run **before** the catch body. Every catch lowers to a Go `recover` boundary anyway (above),
     so its handler **drains the list down to the catch's depth marker** first; the
     goroutine-entry trampoline — which already exists, because a task panic must resolve its
     promise (concurrency §4.1) — drains the remainder. Same trampoline, one added drain.

  **Goroutine composition is free by construction**: the defer list is per-task state beside the
  cancellation flag and promise; scope-bounding means defers never outlive their task and
  const-snapshot capture means they never reference another task's frame. Cancellation is
  delivered as a panic-class unwind at suspension points (concurrency §6.1, R115), so defers run
  on cancellation through the *same* machinery — plus the one addition R115 demands, the
  **shield flag**: a per-task in-defer bit suppressing `cancelled` delivery at suspension points
  inside defer bodies (cleanup is io and must not be re-cancelled mid-close), checked exactly
  where delivery already checks. A task leaked on an uncancellable loop (concurrency §5.1's
  bounded-waiting contract) runs its defers when it eventually reaches a suspension point and
  unwinds — consistent, not special.

### 7.4 Module initialization

Module init order is **deterministic** (modules spec §2) and is **not** left to Go's package-init
ordering (which does not match Luna's DAG-derived order). The compiler emits an explicit,
topologically-ordered init sequence that runs each module's top-level initialization exactly once,
in dependency order, so a module's imports are initialized before it.

### 7.5 Builtins, std, and capabilities

- **Builtins** are ambient (modules spec §10) and compiled into the runtime the emitted program
  links against.
- **`std`** modules live under the reserved virtual root (modules spec §10); only those the program
  references are included in the emitted program (tree-shaken), so unused std costs nothing.
- **Capabilities** are zero-data and reached only through `use` (capabilities spec); at runtime
  they carry no payload, so they erase to nothing (or a zero-size token). Their entire force is
  compile-time: the type system checks that only `use`-ing functions reach them, and the comptime
  sandbox that comptime reaches none. The backend emits no runtime capability representation beyond
  what threading a reference requires.

---

## 8. Determinism

Two determinism guarantees run through the pipeline and must be preserved by emission:

- **Deterministic module initialization** (§7.4, modules spec §2), the same program initializes
  its modules in the same order every run.
- **Deterministic comptime** (§6, functions spec §5.5), comptime uses a step budget, not
  wall-clock time, so compile-time evaluation produces identical results and identical
  pass/fail outcomes on any machine, which is what makes builds reproducible and comptime results
  cacheable.

Parallelism (§2) must not perturb either: parallel analysis and emission may interleave freely,
but the *emitted* init order and comptime results are fixed by the DAG and the step budget, not by
scheduling.

---

## 9. Incremental compilation and caching

The compiler caches **per-module compiled binary artifacts** so an unchanged module is not
recompiled and only linking remains, and it keys the cache on each module's **public-interface
hash** (not a whole-file checksum) so that a body-only change to a widely-imported module
recompiles just that module and relinks its dependents, rather than recompiling the whole
dependency cone. The full design, the interface-hash cache key, `stat()`-first change detection,
version namespacing, and the eviction and sharing model, is specified separately in the
**incremental-compilation spec**. The assemble-and-build phase (§1.8) consults that cache before
emitting and building.

---

## 10. Distribution: no exposed IR

The Luna IR (§4) is an **internal, unstable implementation detail and is never a public
artifact.** It is deliberately *not* exposed as a compilation or distribution target, because
doing so would make the IR a permanent compatibility surface: distributed IR artifacts would have
to keep working across compiler versions, which would ossify the IR and prevent the optimizer and
lowering from evolving. That is the same trap that freezes bytecode formats and their VMs, and
the language avoids it by keeping the IR private.

So distribution has exactly two supported forms:

- **Source distribution (now).** A library is distributed as Luna **source**, which the consuming
  build compiles (and caches, incremental-compilation spec) like any other module. This keeps the IR entirely internal.
- **Opaque compiled binaries / dynamic module loading (later).** A future mechanism may distribute
  **compiled binary artifacts** (opaque, compiler-version-stamped, incremental-compilation spec §3) or load compiled modules
  dynamically. These distribute *compiled output*, not IR, so the IR stays private and free to
  change; the versioned binary is the compatibility unit, not the IR. Such an artifact can carry
  its **interface hash** (incremental-compilation spec §1) as a compatibility fingerprint, so a consumer verifies the binary's
  interface matches what they compiled against, the same extracted interface the cache and tooling
  already use.

The rule to hold: the IR is never a distribution or interchange format. Source is the portable
form today; opaque versioned binaries are the portable form later. Neither exposes the IR.

---

## 11. Resolved and open

**Resolved: the `lval` under Go's precise GC (§7.1).** The value is a **three-word (24-byte) Go
struct** with the scalar payload and the pointer payload in **separate fields** (one never traced,
one always traced), because Go's precise GC resolves tracing from static field layout and never
reads the `typeid` (value-representation §1.1). The single-word 16-byte union is not achievable on
Go's collector; it is available only under the **native-runtime alternative** below. Value
performance comes from **static unboxing** (§7.1.1), not from shrinking the struct.

**The native-runtime alternative (not chosen).** Generating native code (via LLVM or C) instead of
Go, with a self-owned garbage collector (for example MMTk, or a bespoke collector), would allow the
16-byte single-word tagged value (the GC could be taught to read the `typeid`) and remove the
precise-GC constraint. It is not chosen because it discards the reasons Go was targeted, its GC and
its goroutine scheduler (green threads), turning both into work the project would own. The Go-hosted
path with a 24-byte `lval` and static unboxing is the committed design; this alternative is recorded
only as the escape hatch if the value representation ever becomes a decisive bottleneck.

Open:

- **Block-scoped `defer` lowering (§7.3).** The concrete emission for block-scoped, panic-composing
  LIFO cleanup on top of Go's function-scoped `defer`, including the interaction with recover on
  the panic path.
- **Green-thread mapping and copying (§7.3).** How enforced-copying at task boundaries is realized
  in emitted Go, and how cancellation unwinds and runs deferred cleanup, pending the concurrency
  model.
- **Static-unboxing boundaries (§7.1.1).** The precise rules for where boxing and unboxing occur
  (the `any` boundary, table-element access, union narrowing), and how to minimize box/unbox churn
  across those boundaries, pending implementation measurement.

(Open questions specific to the build cache, canonical interface serialization and eviction tuning,
are in the incremental-compilation spec.)
