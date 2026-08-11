# driver

How the oracle's phases are wired together. An implementation plan, not spec: compiler §1–§3
own the pipeline and the error model, and this only records how we intend to build against
them. Sits beside `lexer-testing-plan.md`.

---

## 1. Drivers are plural; the batch compiler is one of them

Compiler §1: the compiler is a **library of reusable passes, not a monolith**, because the
formatter, LSP and debugger are each *a driver that calls the passes it needs*. The formatter
wants lex + parse on one file and no import graph at all.

So orchestration lives in a package — `oracle/driver` — and `cmd/luna` is a thin main over it.
Not in `main`, or every other tool would have to import a command or duplicate it. Same split
as `cmd/highlight` → `internal/highlight`.

## 2. A pass is per-unit unless the job is inherently global

Falls out of §2's parallelism table. Lex, parse, lower, emit are per file; discovery, import
validation and assembly are graph-shaped and take the whole set.

The test is the LSP: on a keystroke it lexes **one** file. `lexer.Lex(f *source.File)` serves
that; a `LexAll(files)` with goroutines inside would not, and the second lexer someone then
writes is the miscompilation seed R190 exists to prevent.

## 3. The driver owns fan-out, and merges in file order

Passes are pure and sequential; the driver decides what runs in parallel, per §2's
free/layered/free shape.

The reason is **determinism**, not tidiness. The oracle is the conformance oracle for
differential testing, so diagnostics must arrive in the same order every run. Merging results
in file order makes that structural instead of something tests have to catch.

## 4. Passes report; the driver decides

Every pass returns `(output, diagnostic.List)` and judges nothing about the compile — already
the contract in `lexer.go`: *"the compile aborts at the phase boundary, which is the driver's
decision and not this package's."* §3's rule that a phase cannot consume the broken output of
the previous one is the driver's to enforce.

## 5. `Unit` is the driver's bookkeeping, and passes never see it

The driver holds a per-file `Unit` accumulating what each phase produced — tokens, then CST,
then typed AST. Chosen over threading six parallel maps by hand, and over a demand-driven
query system, which is the wrong shape for an oracle whose stated virtue is being simple and
slow (§6.1: *an oracle you optimize is an oracle you doubt*).

**No pass takes a `*Unit`.** A pass takes what it needs — `Lex(*source.File)`, and
`modules.Validate(Result, map[string][]token.Token)` — so the driver builds that map from its
Units and `modules` never learns Units exist. The moment a pass takes a `*Unit`, the formatter
has to build one to format a file and §1's library-of-passes property is gone.

A `Unit` field that is nil when a pass needs it is a **driver bug**: panic, as `Next` does for
its span invariants, not a diagnostic.

---

## Decided, not yet built

From the §1.2 design pass, agreed and unrecorded elsewhere:

- **Importing the root module is its own error**, not a cycle report. The entry is module `""`
  (modules §3) but is also reachable by filename, and two identities for one file is something
  §7's provenance rules cannot carry. Needs a ruling.
- **§1.2 excludes `std.*` edges** before reporting unresolved imports — they have no file by
  design (modules §10).
- **All cycles are reported**, not the first, per §3's collect-everything model. The full path
  is required, so three-colour DFS rather than Kahn's.
