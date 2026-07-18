# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repository is

This is the **specification** of the Luna programming language, not an implementation. No compiler
exists yet; the deliverable is the design itself. In practice that means the work here is almost
always **prose editing under a consistency discipline**, not coding. The three artifacts:

- `specs/` — the spec, one Markdown file per topic, grouped into subdirectories (`types/`,
  `expressions/`, `declarations/`, `concurrency/`, `build/`, `std/`, `internals/`, `type-operations/`,
  `bindings/`, `overview/`, `examples/`, `retired/`). `specs/index.md` is the authoritative map of every spec file
  and what it owns. `specs/overview/high-level-overview.md` and `specs/overview/types.md` are the orientation
  layer. Each spec is authoritative for its own topic and cross-references others by section number.
- `CHANGES.md` — the design-decision log: a numbered sequence of rulings (`R1`, `R2`, … currently past
  `R83`) that each resolve a contradiction or open question and record every file swept to apply it.
  This is the most important file to understand before changing any spec.
- `tooling/` — syntax highlighting only (tree-sitter grammar, VSCode extension, Zed extension, a Shiki
  grammar). Independent of the spec's correctness.

Note: the spec directory was renamed `docs/` → `specs/` (R145); rulings before R145 cite `docs/`
paths and are frozen history. `user-docs/` (README) is planned and does not exist yet.

## Working on the spec (the main task)

The spec is a single tightly cross-linked corpus. A change is rarely local. The established workflow,
visible throughout `CHANGES.md`, is:

1. **A ruling resolves one question.** When two specs disagree, or an open question is decided, the
   resolution is a ruling with a stated rationale — including the rejected alternative and why. Rulings
   supersede earlier rulings (e.g. R11 supersedes R10 supersedes R9); the log preserves the whole chain
   rather than rewriting history.
2. **Sweep every affected site.** A ruling is not applied until every contradicting passage across the
   corpus is fixed. Rulings routinely list "33 sites across 11 files." Before editing, grep the corpus
   for the term, spelling, or claim you are changing — partial application is the primary defect this
   process guards against.
3. **Record it in `CHANGES.md`.** Add a new ruling entry (prose, matching the existing dense style)
   naming the rationale and the files swept. Keep the "Not changed / still open" section at the tail
   honest.

Key conventions that recur and must stay consistent:

- **Cross-references are by section**, e.g. "capabilities §3.1", "tables §2.2". When you move or
  renumber content, the references pointing at it must move too.
- **Spelling and surface syntax are load-bearing and have been ruled.** Examples of settled forms:
  errorable type is postfix `T!` (not prefix `!T`); logical-not is prefix `!a`; capabilities are
  ordinary exported `const`s named in `use (io)` (the old `caps.*` / `use (&io)` spellings are dead);
  `export` not `pub`; `die` not `fail`; error/panic type names are lowercase (`panic`, `typeError`,
  `ioError`); identifier prefixes are camelCase (`unsafeFfi`, not `unsafe-ffi`). Do not reintroduce a
  retired spelling — check `CHANGES.md` if unsure whether a form is current.
- **Match the house prose style**: terse, decision-dense, rationale-forward, first-matching-arm
  precision. The overview and index are the models for tone.

## Tooling commands

Grammar generation runs in a pinned one-shot container (no local node/npm needed); the two halves are
separate on purpose:

```bash
tooling/generate-grammar.sh    # regenerate tree-sitter-luna/src via podman (does NOT touch git)
tooling/publish-grammar.sh     # commit + push the grammar, then repoint zed-luna at the new rev
```

`tooling/generate-grammar.sh` deletes and regenerates `tooling/tree-sitter-luna/src/` (the checked-in
`parser.c` etc. are generated artifacts). `tooling/publish-grammar.sh` is the only thing that writes
`tooling/zed-luna/extension.toml` — do not hand-edit that file. There is no build/test/lint step for
the language itself, because there is no implementation.

Package the spec for distribution (respects every nested `.gitignore`):

```bash
./make-archive.sh              # -> luna-spec.zip
```

The root `Containerfile` / `compose.yaml` are a Claude Code sandbox for Fedora Asahi (aarch64, 16K
pages) and are unrelated to the spec or the grammar tooling.

## The language's governing ideas (context for spec edits)

These commitments decide most spec questions; when a design choice is ambiguous, the one that upholds
these wins:

- **Safe by construction, not by discipline.** Strict `==` (same type and value, no coercion), explicit
  numeric conversion, integer overflow panics (never wraps), no shared mutable state across tasks, no
  unsafe API. Whole bug classes are meant to be closed by the design.
- **Small surface area.** Keywords and built-in types are deliberately few and reused across contexts;
  new syntax is preferably sugar over one existing mechanism (`?` is `| null`, `!` adds the error union)
  rather than a new mechanism.
- **Same source runs or compiles.** A `.luna` file executes directly like a script (via cached,
  incremental builds under `$HOME/.lunalang/`) or compiles to a self-contained native binary, with
  identical semantics. The backend is **Go source**, handed to the Go toolchain.
- **Data-focused, structurally typed.** The type system (unions `|`, intersections `&`, refinement
  constraints, protocols) exists to describe the shape of data; `any` and dynamism are available but
  always explicitly narrowed (`as` / `is` / `match`), never a silent fallback.
- **Effects are gated by capabilities.** Reaching outside the program (io, exec, reveal) requires an
  unforgeable capability named in a function's `use` clause; this is the audit the whole system protects.
