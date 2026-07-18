# Tooling

Luna's developer tools, the **formatter**, the **language server (LSP)**, and the **debugger**,
are provided **by the Luna compiler itself**, not as separate binaries. One distribution ships the
compiler and all tools, so the tools always share the compiler's exact frontend and can never drift
out of sync with the language it implements.

This document fixes the **architecture** and the **frontend properties** the tools require, and
sketches each tool. The detailed rules of each tool (the formatting style, the full LSP feature
set, the debugger interface) are specified later; what matters now is that the compiler frontend is
built so those tools are *possible*, because the properties they depend on are foundational and
painful to retrofit.

The guiding freedom: the tools may reuse the compiler's **internal** representations aggressively
(the lossless syntax tree, the typed AST, the interface extraction, the Luna IR, the `typeinfo`),
because none of these is a public, stability-committed artifact (the IR is explicitly private,
compiler spec §10). Internal reuse costs nothing in external commitment.

---

## 1. The compiler is a library of reusable passes

"Tools in the compiler" requires the compiler to be structured as a **library**, a set of callable
passes (lex, parse, analyze, extract interface, lower, emit), not a `main`-centric monolith. Each
tool is a *driver* that calls the passes it needs:

- the **batch compiler** runs the full pipeline to a binary,
- the **formatter** calls only the lossless parse (§2),
- the **LSP** calls parse and analyze in error-tolerant mode (§3, §4) plus interface extraction,
- the **debugger** calls the full pipeline in debug mode plus `typeinfo`/IR export (§5).

So the frontend must have **clean pass boundaries and reusable components** from the start. This is
an **early design decision** (it shapes how the whole compiler is written); it cannot be retrofitted
onto a monolith cheaply.

---

## 2. The parser produces a lossless CST

The parser produces a **lossless concrete syntax tree (CST)**: a full-fidelity tree from which the
**exact source bytes are reconstructable**, retaining **all trivia**, comments, whitespace, and
original token spelling. This is distinct from the AST the compiler analyzes (which discards
trivia); the CST is the shared root, the compiler ignores trivia, and the tools use it.

- The **formatter** requires this: a formatter that loses comments is useless, so it must work from
  a tree that preserves them. It reads the CST, applies the formatting rules (specified later), and
  writes back source with comments intact.
- The **LSP** benefits too (exact spans for every token, including trivia, for precise
  highlighting, selection, and refactoring).

This is a **now, not later** decision: a lossy parser cannot be made lossless without being
rewritten, so the parser is built lossless from the start, even though the formatting *rules* come
later. Deferring the rules is fine; deferring the lossless parse is a trap.

---

## 3. The frontend is error-tolerant; only the batch driver aborts

The frontend is built to **recover from errors and produce a best-effort partial result**, a
partial CST and a partial typed AST with as much resolved as possible, rather than stopping at the
first error. Error tolerance is a property of the **passes**; whether to **abort** is a property of
the **driver**:

- The **batch compiler driver** accumulates errors within a phase and **aborts at the phase
  boundary** (compiler spec §3): a binary is not produced from broken input.
- The **LSP driver** never aborts. The user's code is almost always mid-edit and partially
  invalid, and the LSP must still deliver types, completions, and diagnostics for the **valid
  parts**. So it takes the passes' partial results and keeps going past the errors.

So the same passes serve both: error recovery makes the partial result *available*, and each driver
chooses whether to consume it (LSP) or discard it and abort (batch). Building the passes
error-tolerant from the start also improves the batch compiler's diagnostics (it can report more
independent errors before aborting). This is a **now** decision: error recovery is deeply woven into
parse and analysis and is hard to add to a frontend built to abort.

---

## 4. The language server (LSP)

The LSP is the largest tool. It reuses the frontend and the interface-extraction primitive, with
tool-specific requirements layered on:

- **Reuses interface extraction** (incremental-compilation spec §1): the same public-interface
  artifact the cache fingerprints and the tooling reads powers cross-module "go to definition,"
  "find references," and "what does this module export." Cache and LSP agree on interfaces by
  construction, they read the same extraction.
- **Reuses the typed AST and symbol/type tables** (post-analysis): mapping a source position to a
  symbol to its type is hover, completion, and signature help.
- **Requires error tolerance** (§3): it runs the error-tolerant passes so a half-typed file still
  yields useful results.
- **Incrementality**: the LSP reuses the module-level incremental cache (incremental-compilation
  spec). **Finer-than-module incrementality** (re-analyzing a single function body without
  re-analyzing its module) is a **later** optimization for large files, not a correctness
  requirement: re-analyzing a whole module per edit is correct and fast enough for typical modules,
  and finer granularity slots in on top of the error-tolerant, library-structured frontend without
  redesign. So the LSP ships module-granular first and gets finer later.

The full LSP feature set is specified later; this fixes only what it reuses and what the frontend
must provide for it.

---

## 5. The debugger

The debugger is the hardest tool, because a Luna program is compiled to **Go**, then to native
code, so there is an **extra layer**: the debugging stack is Luna source over generated Go over
native machine code, a **two-hop** mapping where most languages have one. Exporting the Luna IR /
`typeinfo` is *necessary but not sufficient*: it solves the *value-semantics* half, but not the
*position* half.

- **Value reconstruction (solved by exported `typeinfo`/IR).** At a breakpoint, the debugger reads
  the live `lval`s (their Go structs) from the frame and interprets them with **Luna** semantics
  using exported `typeinfo`: the `typeid` gives the type, redaction is applied (a `secret` shows
  redacted, secret spec), the payload is decoded. So debug binaries **export the `typeinfo` and IR**
  needed to interpret raw values as Luna values.
- **Position mapping (the two-hop problem).** A breakpoint on Luna line 40 must reach a native
  address through both hops: **Luna to Go** via the emitted `//line` directives (compiler spec
  §1.7), and **Go to native** via Go's own DWARF debug info (inherited from the Go toolchain). The
  `//line` directives are therefore load-bearing for debugging, not only for panic stack traces:
  they are the Luna-to-Go bridge the debugger chains with Go's DWARF to place breakpoints and
  unwind Luna frames.
- **Debug builds are unoptimized, in both Luna and Go.** Optimizations destroy the mapping: static
  unboxing (compiler spec §7.1.1) turns a Luna `int` into a raw Go primitive with no `lval` to
  read, constraint elision removes checks, comptime folding removes computation, so an optimized
  frame does not match the Luna source or the exported IR. Therefore a **debug build disables
  representation-destroying optimizations at both levels**: Luna-level (no static unboxing, no
  elision, no folding, so every Luna value is a real, findable `lval`) and Go-level (emit Go built
  without optimization/inlining, so Go's DWARF is faithful). Debug builds trade speed for a frame
  that exactly matches the Luna source and the exported `typeinfo`.

Because of the two-hop mapping and the unoptimized-build requirement, the debugger is the most
expensive tool and reasonably the **last** built (formatter and LSP first). Its detailed design (the
wire protocol, breakpoint and stepping semantics, value inspection UI) is specified later.

---

## 6. What is decided now vs. later

**Now (foundational, hard to retrofit), shapes how the frontend is written:**

- The compiler is a **library of reusable passes** (§1).
- The parser produces a **lossless CST** with trivia (§2).
- The frontend is **error-tolerant**, producing partial results; only the batch driver aborts (§3).
- Debug builds are **unoptimized at both Luna and Go levels**, and `//line` discipline is
  maintained (§5).

**Later (deferrable, does not require frontend rework):**

- The **formatting rules** (the actual style the formatter applies), §2.
- The **full LSP feature set**, and **finer-than-module incrementality** (§4).
- The **debugger's detailed design** (protocol, stepping, inspection UI), §5.

The split is deliberate: the *rules and feature sets* of the tools are deferrable, but the
*frontend properties they depend on*, lossless, error-tolerant, library-structured, unoptimized
debug builds, are decided now, because retrofitting them means rewriting the frontend.

---

## 7. Open questions

- **Formatting rules.** The formatter's style (indentation, wrapping, alignment, comment handling,
  idempotence guarantees) is unspecified; to be a separate design.
- **LSP feature set and protocol.** Which language-server features are supported, and the exact
  server behavior, pending the LSP design.
- **Finer-grained incrementality.** The mechanism for intra-module incremental re-analysis (below
  module granularity), pending measurement of where module-level incrementality is too coarse.
- **Debugger protocol and UI.** The debug wire protocol, breakpoint/stepping model, and value
  inspection surface, pending the debugger design.
- **Trivia in interface extraction.** Whether doc comments (a subset of trivia) are carried into the
  extracted interface for tooling (hover documentation), distinct from ordinary comments excluded
  from the interface hash (incremental-compilation spec §1.3).
