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
  specialization, protocol devirtualization) and where **comptime** is evaluated, none of which
  Go's compiler can do, because by the time it sees Go source the Luna semantics are gone.

---

## 1. The phase pipeline

```
source
  -> lex                 (tokens)
  -> module resolution   (DAG)
  -> parse               (untyped AST)          [parallel per module]
  -> semantic analysis   (typed AST)            [parallel by DAG layer]
  -> lower to Luna IR    (typed IR)
  -> optimize IR         (comptime, elision, specialization, devirtualization)
  -> emit Go             (Go source per module) [parallel per module]
  -> assemble + build    (Go program -> Go toolchain -> binary)
```

Each phase is described below. The parallelism structure (§2) and the error model (§3) cut across
all of them.

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

A pure **context-free** pass: tokens to an **untyped AST**, per module, with no type knowledge
and no name resolution. Parsing is **fully parallel** across modules (§2), each module's tokens
are independent once resolution has run. Parse errors are accumulated per module and aborted at
the phase boundary (§3). The result is one untyped AST per module.

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

### 1.5 Lower to Luna IR

The typed AST is lowered to the **Luna IR** (§4): a typed, explicit, lowered tree. Lowering makes
implicit semantics explicit, coalescing operators become branches, `match` becomes decision
logic, ranges-as-streams become stream constructors (range spec), string interpolation becomes
builder calls (string-builder spec), UFCS calls are resolved to their targets, `.`/`[]`/`->`
become concrete access operations. After lowering, the IR contains no syntactic sugar and no
unresolved dispatch: every operation is a concrete, typed node.

### 1.6 Optimize IR

The Luna-semantic optimization passes (§5) run on the IR: **comptime evaluation** (§6),
**constraint-check elision**, **const-table specialization**, **protocol-dispatch
devirtualization**, and ordinary constant folding and dead-code elimination where Luna semantics
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
compile-time ones (§9): there are no user-visible Go compile errors to map back, only runtime
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
  (`->`) are distinct node kinds (tables spec §3.2, §3.3), and protocol-method calls carry their
  resolved protocol and, where statically known, their concrete target (enabling
  devirtualization, §5).
- **Explicit constraint boundaries.** Every point where a value *enters* a constrained type
  (construction, assignment to a constrained slot, `as`, constraints spec §7) is a marked node
  carrying the predicate, so the optimizer can elide the check where it is provably satisfied
  (§5) or emit it otherwise.
- **Comptime-eligibility marks.** Subtrees that are comptime-eligible (functions spec §5) are
  marked so the comptime evaluator (§6) knows what it may evaluate.
- **Explicit control and cleanup.** `match` is lowered to decision logic (match spec), coalescing
  to branches (coalescing spec), loops to their consuming form over streams/ranges (control-flow
  spec), and `defer` registrations to explicit scoped-cleanup nodes (defer spec, §7.3).
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
- **Constraint-check elision** (constraints spec §10). At a constraint boundary (§4), if the
  compiler can trivially prove the value already satisfies the predicate, a literal in range, a
  value freshly produced as that constrained type, drop the runtime predicate call. This never
  changes meaning (it removes a check that would always pass); it is distinct from the rejected
  static-verification model, which would *replace* runtime checking with proof obligations.
- **Const-table specialization** (tables spec Amendment A). A `const` table gets frozen-storage
  benefits; a *compile-time-shaped* `const` table (built via comptime) additionally gets the
  contiguous-struct layout with a perfect hash, and static `.` accesses on it lower to direct
  offset loads rather than hash probes. This pass chooses the representation and rewrites the
  access nodes.
- **Protocol-dispatch devirtualization** (protocols, views specs). Where the protocol set a table
  wears is statically known at a `->` site, resolve the meta-function to a direct call instead of
  a runtime view lookup. Where it is not known, emit the dynamic dispatch.
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
- **The capability sandbox is enforced statically and dynamically.** Comptime forbids `use`
  (functions spec §5.5): a comptime-eligible subtree provably reaches no capability (checked in
  analysis), so it can touch no outside world. The evaluator additionally runs under the liveness
  guards, deterministic execution budget (fuel, not wall-clock), stack-depth limit, and allocation
  ceiling (functions spec §5.5), each aborting with a compile error rather than hanging the build.
- **It is deterministic.** The budget is a step counter, not wall-clock time, so comptime results
  are reproducible and cacheable across machines (functions spec §5.5). Determinism here is what
  keeps builds reproducible.
- **Its main product is `const` data.** Comptime-built `const` tables become the
  compile-time-shaped tables that get the perfect-hash struct layout (§5, tables Amendment A), so
  comptime and const-table specialization compose: comptime builds the table, specialization lays
  it out.

---

## 7. The Go backend

Emission maps the optimized IR onto Go source. The mapping is direct because the language was
designed against a Go runtime.

### 7.1 The `lval`

The 16-byte `lval` (value-representation §1) maps to a Go struct: a `uint64` for the packed flags,
string-inline field, and `typeid`, plus a data word for the payload-or-pointer. The **precise-GC
constraint is the sharp implementation point**: Go's garbage collector is precise, so a single
field that is *sometimes* a pointer (managed payload) and *sometimes* inline bytes (an inline
scalar or short string) cannot be type-punned freely, Go must know statically whether a word holds
a pointer. Resolving this, whether via a representation that keeps the pointer word always a valid
pointer or nil, a split representation, or a carefully-managed unsafe encoding, is the central
backend design task (§9). The IR's per-node representation classification (§4) is what tells
emission which case each value is.

### 7.2 The `typetable`

The `typetable` (value-representation §4) is emitted as **static Go data**: a global array of
`typeinfo` values built at compile time, holding nullability, attributes, error ancestry with the
preorder `enter`/`size` interval numbering (value-representation §4.2), and protocol metadata.
Because the type universe is closed at compile time (value-representation §4.1), this table is
fully known during emission and never grows at runtime, subtype tests compile to the two integer
comparisons of the interval check.

### 7.3 Control, errors, and cleanup

- **Green threads to goroutines.** Luna's green threads map to goroutines; the enforced-copying
  discipline (value-representation, functions specs) is realized by the IR inserting copies at task
  boundaries, so tasks share no mutable state. (The full concurrency model is pending; this fixes
  only the mapping.)
- **Error model.** Luna panics (errors §9) map to Go panic/recover: a panic propagates ambiently
  and `try` recovers it at the boundary, converting it to a value. Errorable results (`!`,
  value-representation §3.1) need **no special calling convention**: an errorable value is just an
  `lval` with its **error bit** set and its **`typeid`** giving the concrete error subtype. Errors
  are lvals like every other value, so an errorable return is returned as an ordinary `lval`, and
  `try` is a check of the error bit (with the subtype read from the `typeid`). There is no tagged
  tuple, interface, or side channel; the representation is the same 16-byte `lval` used
  everywhere.
- **`defer` to scoped cleanup.** Luna's `defer` is **block-scoped** (defer spec §1), whereas Go's
  `defer` is function-scoped, so Luna `defer` cannot map one-to-one onto Go `defer`. It lowers to
  explicit scoped cleanup: the block's deferred statements run at every exit edge of that block
  (normal, `return`, `break`/`continue`, and panic unwinding), in LIFO order, which emission
  realizes with per-block cleanup code (or a scoped closure carrying a Go `defer`). Operands
  captured by value at registration (defer spec §4) are snapshotted into the cleanup at the
  registration point.

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

## 9. Incremental and cached compilation

The compiler caches **per-module compiled binary artifacts** in a home-directory cache, so an
unchanged module is not recompiled and only **linking** remains. The design turns on the cache key,
which must be correct (never serve a stale artifact) and cheap (avoid `open()` on the hot path).

### 9.1 The cache key is transitive over the module interface

A module's artifact depends on its own source **and** on the **public interface of everything it
imports**: if an imported module changes a function's result type, a dependent's artifact is stale
even though the dependent's own file is untouched. So the key is:

```
key(M) = own_key(M) + { interface_hash(I) : I in imports(M) }
```

where `interface_hash(I)` fingerprints `I`'s **public interface**, the surface a dependent compiles
against. Keying only on a module's own file would silently miscompile when a dependency's interface
changes (wrong output that looks correct), so the transitive key is mandatory, not an optimization.
Because the module graph is a DAG (modules spec §2), interface hashes propagate cleanly along
topological order.

#### The primitive is interface extraction, not a checksum

The compiler extracts each module's **public interface** as a structured artifact, and
`interface_hash` is a deterministic fingerprint of that artifact. This is deliberately **not** a
whole-file checksum (comments stripped or not), and the difference is the whole point of the
scheme.

A checksum answers "did the file change?" The interface hash answers the narrower question "did the
public *interface* change?", and only the second enables the **recompile-versus-relink** split that
makes compilation actually incremental:

- If an imported module's **interface hash is unchanged** (only bodies changed), its dependents'
  keys are unchanged, so they **do not recompile**; they **relink** against the module's new
  artifact. Their emitted code is still valid because the interface they compiled against did not
  move.
- If the **interface hash changed**, dependents **recompile** (re-analyze and re-emit against the
  new interface).

A checksum cannot make this distinction: any body-only edit changes the checksum, so every
dependent recompiles. Editing one function body in a widely-imported module would then recompile
its entire dependency cone, the "change one file, rebuild the world" cascade, even though every
dependent's generated code is identical. The interface hash cuts the cascade at every module whose
interface did not change, so a body-only edit recompiles just that module and relinks the rest.
Body-only edits are the most common kind of change, so this is where the savings concentrate. A
checksum is *correct* (it over-invalidates, which is safe) but pessimal; the interface hash is what
makes incremental builds incremental.

#### What the public interface includes

The interface is what a dependent could compile differently against, reached transitively:

- **Every exported binding and its full type**, functions (all of the signature: parameters,
  result, errorability, and comptime-eligibility, since all four are type identity, functions spec
  §3), consts and their types, enums, constraints, protocols, capabilities.
- **The full structural content of each exported type**, not just its name: an enum's complete
  variant set and payload types (dependents' `match` exhaustiveness and destructuring depend on
  them, match spec §9), a constraint's **predicate** (dependents' `as` checks and check-elision
  depend on it, constraints spec §10), a protocol's full meta-function surface and element
  contract. Two definitions that share a name but differ in content must hash differently.
- **Transitively-reachable types, even private ones exposed through a public signature.** If an
  exported `fn f(): Internal` returns a non-exported `Internal`, then `Internal`'s structure is
  observable through `f` (a dependent can `match` and destructure the result), so `Internal`'s
  structure is in the interface even though it is not itself exported. Missing this silently
  miscompiles dependents when a "private" type that leaks through a public API changes.
- **The bodies of comptime-eligible exported functions.** Comptime (functions spec §5) can execute
  an imported function at compile time, so a dependent's compile-time result depends on that
  function's **body**, not just its signature. For a comptime-eligible exported function the body
  is therefore part of the observable interface and is included. (An ordinary exported function's
  body is not, §below.)

#### What the interface excludes (the payoff)

Everything a dependent cannot observe is excluded, which is exactly what a whole-file checksum
cannot exclude and what makes the interface hash worth having:

- **Ordinary (non-comptime) function bodies.** Changing an exported function's implementation
  without changing its type does not change its interface, so dependents relink rather than
  recompile.
- **Private definitions not reachable from any exported type**, a helper used only internally is
  not in the interface; changing it recompiles the module itself (via `own_key`, §9.2) but does not
  cascade to dependents.
- **Comments, formatting, and private names**, unobservable, excluded.

So the interface is a true **interface fingerprint**: it changes only when a dependent could
compile differently, and implementation churn (the common case) does not cascade.

#### One primitive, several consumers

Interface extraction is a single primitive with more than one consumer, so it is not cost
attributable to the cache alone:

- **The incremental cache** uses `interface_hash` for change detection and the recompile-versus-
  relink split (above).
- **The in-compiler tooling** (formatter, linter, LSP, all to be specified later) must compute a
  module's public interface regardless, navigation, hover types, find-references, and "what does
  this module export" all need it. So the tooling builds interface extraction anyway, which makes
  the cache's hash a near-free fingerprint of an artifact that already exists, and guarantees the
  cache and the tooling agree on what a module's interface is (they read the same extraction).
- **Future binary distribution** (§10) can reuse the same fingerprint as a compatibility check: an
  opaque compiled artifact carries its interface hash, and a consumer verifies it matches what they
  linked against.

Because the tooling requires interface extraction independently, using a checksum for the cache
would be a false economy: it would forgo the recompile-versus-relink savings *and* maintain a
second, weaker notion of "what changed" beside the interface the tooling already computes.

### 9.2 `stat()`-first, hash-on-change

`own_key(M)` uses the filesystem cheaply on the common path and hashes only when something changed:

- **Fast path (no `open()`):** if `(mtime, size)` **matches** the cached entry, the module is
  unchanged, reuse its artifact. This is a `stat()` only, no file read, for the overwhelming
  common case of "nothing changed."
- **Change path (hash to confirm):** if `(mtime, size)` **differs** from the cached entry, the
  module is *possibly* changed, so **read and content-hash** it. This costs an `open()`, but only
  when the module was going to be recompiled anyway, so the hash is nearly free. Hashing on this
  path closes the two silent-miscompile holes that pure `stat()` leaves:
  - a **size-preserving edit** (`<` to `>`, `+` to `-`) that `size` cannot see, and
  - a **preserved or coarse `mtime`** (`cp -p`, `rsync --times`, `tar`, `git checkout`, coarse-
    granularity filesystems) that makes a changed file look unchanged or an unchanged file look
    changed.

  If the content hash matches the cached entry despite the differing `(mtime, size)`, the artifact
  is reused (and the entry's `(mtime, size)` refreshed), so a `touch` or a no-op checkout does not
  force a recompile. If the hash differs, the module is recompiled.

So `stat()` decides "unchanged" on the hot path with no file read, and a content hash decides on the
cold path (where a read is already implied). This pays neither the silent-miscompile risk of
pure-`stat()` nor the always-`open()` cost of always-hashing.

One residual blind spot is accepted and documented, not engineered against: a size-preserving edit
whose `mtime` is restored to the *exact* value the cache recorded (which needs a timestamp-pinning
tool, not normal editing or a `git` checkout, both of which move `mtime` forward) matches
`(mtime, size)` on the fast path and is not detected. This is effectively unreachable through
ordinary editor-and-git workflows; when a workflow does normalize or restore timestamps in place,
the remedy is a manual full rebuild (`--clean`), which the cache cannot infer on its own.

### 9.3 The cache is namespaced by compiler version

Cache entries are stored under a **compiler-version-stamped** namespace, because a new compiler may
emit different Go or different artifacts for identical source. Namespacing by version prevents an
upgraded compiler from serving artifacts built by the old one. Comptime results are deterministic
(§6, §8), so they cache cleanly under the same keying, an unchanged comptime-eligible module
reuses its computed constants.

### 9.4 Eviction and sharing

**The cache is private and per-user, not shared across users or projects.** Cross-project artifact
sharing is deliberately not done: an artifact is a compiled binary specialized to *its* build
context (comptime results, constraint-elision decisions, const-table specialization), so the same
module source built in a different context may need a different artifact; a shared cache is also a
code-injection trust boundary; and the cross-project hit rate is low. (Path relativity is *not* the
reason, the cache key is content-and-interface derived, §9.1, so it is already path-independent.)
Cross-project reuse, if ever wanted, belongs at the package layer (a package manager caching
packages), not at the compiler's module-artifact layer.

#### Two locks: builds share, eviction excludes

Liveness (an artifact must not be deleted while a build needs it) is protected by a **build lock**,
not by a timestamp heuristic. Reading a recent `mtime` as "probably in use" is too frail for a
delete decision, so liveness is a fact, not a guess:

- **The build lock is a shared (reader) lock.** Many concurrent builds hold it at once; it does not
  serialize builds. While any build holds it, the evictor must not delete artifacts those builds
  are using.
- **The eviction lock is exclusive (writer).** The evictor takes it to evict. So builds are
  readers and the evictor is the writer of a readers-writer lock: builds run in parallel, and
  eviction excludes (and is excluded by) active builds.

This separates concerns cleanly: `mtime` is used **only** as the last-used / age signal for LRU and
the age threshold (below), never as a liveness signal. The build lock alone decides what is in use.

#### Age-based eviction, with a size-cap backstop

Each artifact's **`mtime` is its last-used time**, bumped on every cache hit (an `utimes` call, no
file read), and read via the `stat()` the cache already performs (§9.2). Eviction is:

- **By age (default):** an artifact unused for more than **30 days** is evictable. Thirty days is
  chosen so a program run even occasionally, a script invoked every week or two, still finds its
  cache warm; a shorter window (a day, a week) would cold-cache occasional and script-style users,
  who are exactly the ones a cross-run cache should serve. Anything untouched for a month is
  genuinely stale (abandoned project, deleted branch, replaced dependency).
- **By size (backstop):** if the cache still exceeds a size cap after age eviction, evict
  least-recently-used entries (oldest `mtime`) until under the cap, so high-volume users cannot
  grow the cache without bound within the 30-day window.

Both the age threshold and the size cap are configurable; the defaults are 30 days and a sensible
size limit.

#### The background evictor: throttled, crash-safe, parallel

Eviction runs in a **detached background worker** so it never blocks a build, and its scheduling is
governed by the eviction lock:

- **Throttled to at most once per day, across all programs.** A build spawns an evictor only if the
  last eviction ran more than **1 day** ago (read from the eviction lock's timestamp via `stat()`).
  So the overwhelming majority of builds spawn nothing. This 1-day **throttle** (how often eviction
  runs, machine-wide) is a distinct number from the 30-day **artifact age** (how long a specific
  program's artifacts survive unused): the sweep runs daily; artifacts live for a month.
- **At most one evictor at a time.** The exclusive eviction lock is the mutex: a second would-be
  evictor that cannot take the lock simply does not run.
- **Crash-safe via steal-on-stale.** The eviction lock records the evictor's **PID and start
  time**. A would-be evictor that finds a lock whose PID is dead, or whose start time is older than
  any sane eviction run, **steals** it (deletes and re-acquires). So a crashed or killed evictor
  never disables eviction permanently, which a naive create-on-start / delete-on-finish lock would.
- **Parallel deletion within the lock holder.** Once it holds the exclusive lock, the evictor
  deletes provably-stale entries (old `mtime`, not held by any build) concurrently. Holding the
  lock plus the age threshold makes each deletion safe.

The single unifying signal is `mtime`: bumped on every use, it serves as last-used time (LRU),
age (the 30-day threshold), and nothing else, liveness is the build lock's job, so no delete
decision ever rests on a frail timestamp guess.

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
  build compiles (and caches, §9) like any other module. This keeps the IR entirely internal.
- **Opaque compiled binaries / dynamic module loading (later).** A future mechanism may distribute
  **compiled binary artifacts** (opaque, compiler-version-stamped, §9.3) or load compiled modules
  dynamically. These distribute *compiled output*, not IR, so the IR stays private and free to
  change; the versioned binary is the compatibility unit, not the IR. Such an artifact can carry
  its **interface hash** (§9.1) as a compatibility fingerprint, so a consumer verifies the binary's
  interface matches what they compiled against, the same extracted interface the cache and tooling
  already use.

The rule to hold: the IR is never a distribution or interchange format. Source is the portable
form today; opaque versioned binaries are the portable form later. Neither exposes the IR.

---

## 11. Open questions

- **The `lval` under Go's precise GC (§7.1).** The central backend design task: how to represent a
  word that is sometimes a managed pointer and sometimes inline payload, given Go's precise,
  pointer-aware GC. The chosen encoding affects every value operation and needs its own design.
- **Block-scoped `defer` lowering (§7.3).** The concrete emission for block-scoped, panic-composing
  LIFO cleanup on top of Go's function-scoped `defer`, including the interaction with recover on
  the panic path.
- **Green-thread mapping and copying (§7.3).** How enforced-copying at task boundaries is realized
  in emitted Go, and how cancellation unwinds and runs deferred cleanup, pending the concurrency
  model.
- **Canonical interface serialization (§9.1).** The precise, deterministic serialization of a
  module's extracted public interface that the hash fingerprints, in particular the canonical form
  and ordering (so semantically-equal interfaces serialize identically), and the exact inclusion
  boundary already fixed in §9.1 (full exported types, transitively-reachable private types exposed
  through public signatures, and comptime-eligible bodies; excluding ordinary bodies,
  unexposed-private definitions, comments, and formatting). This is shared with the tooling that
  extracts the same interface.
