# claude-agent-plan

Orchestration plan for the pipeline in `proposed-claude-pipeline.md`. Two phases: **Phase 0**
hand-builds the oracle (a naive complete interpreter) and the human-owned semantic core; **Phase
1** is the automated loop that builds the production Go-emitting backend and validates it against
the oracle. This file holds the Phase-0 discipline, tooling/container changes, retry metering, the
mount matrix, the artifact set, the quality tiers, and the alpha scope. **The correctness spine —
oracle, differential, metamorphic, fuzz, and non-determinism detection — lives in
`testing-strategy.md`.** Serial, no subagent parallelism.

> Mostly still the plan of record, not a description of the current tree. **Applied (2026-08-10):**
> §A.1 — the `Containerfile` pins Go, installs `golangci-lint` + `libfaketime`, and `compose.yaml`
> carries named volumes for `GOPATH`/`GOCACHE`. **Not applied:** §A.2's per-gate services and §B's
> mount matrix — `compose.yaml` still mounts the whole repo `rw` and has only the `claude` service.

## Phase 0 — Foundations (human-owned, before the automated loop)

The oracle is the keystone: it replaces "LLM-authored expected outputs" with an independent
implementation. Two artifacts must exist before Phase 1 runs, and neither is built by the loop.

1. **Type checker + semantic analysis — hand-built by the human.** The highest-ambiguity,
   most-novel part (structural subtyping, unions/intersections, refinement constraints,
   capability/effect propagation). The deep narrow bore that proves the spec is implementable; gaps
   become `CHANGES.md` rulings + sweeps, by hand, before anything freezes. If the spec can't survive
   a real refinement-aware structural checker, stop — nothing downstream rescues that.
2. **Oracle / alpha interpreter — naive, slow, obvious, end-to-end.** Lex → parse → (1's checker) →
   desugar to LIR → tree-walk eval; bignum-then-bound-check for overflow; zero optimization. This is
   **alpha v0** and the **oracle** (`testing-strategy.md` §1). Frozen, mounted `:ro`.
3. **Independence discipline.** Detailed in `testing-strategy.md` §1. **Alpha choice:** Phase 1
   reuses the human front-end (lex/parse/check/**desugar → LIR**) and builds only the **backend**
   (Go emission from LIR + runtime + incremental cache); the shared front-end is not
   differential-tested — acceptable for alpha, closable later by an independent front-end.

## A. Tooling / container changes (prerequisite)

Nothing runs until the sandbox can build and test Go.

1. **Image (`Containerfile`) — applied.** Go pinned to **1.26.5** (current stable). Fedora 42 lags
   at 1.25.10, so this is the official tarball, checksum-verified and arch-switched
   (`amd64`/`arm64`) — the one deliberate exception to the image's dnf-only rule. The Asahi caution
   is about third-party binaries that hardcode a 4K page size, which upstream Go is not; a `-race`
   build at image-build time proves the mapping (and that the race detector links) instead of
   assuming it. **The pin is dual-purpose, which is what makes it a design decision rather than a
   packaging one: it is both the toolchain that builds the compiler and the language floor of the
   Go the compiler *emits*** — the emitted program is one Go module with a single static `go.mod`
   (compiler §1.8), and that `go.mod` carries the same floor. Bump deliberately, never by drift.
   **R233** rules the rest: this same toolchain ships *inside* the `luna` binary (minus `pprof`,
   `crypto/internal/boring`, and the race `.syso` blobs, excluded on licensing grounds), so no
   user installs Go — but the sandbox keeps all three, which is why the `-race` gate still works.
   This repo's own `go.mod` gets `go 1.26` too: the Containerfile pin is the **toolchain**, `go.mod`
   the **language floor**. `GOTOOLCHAIN=local` is set so a floor/toolchain mismatch breaks the build
   loudly rather than silently fetching a toolchain from `proxy.golang.org` on every fresh
   container. Also installed: `libfaketime` for the `testing-strategy.md` §6-L4 clock perturbation,
   and `golangci-lint` for the §E Tier-1 gate (Fedora ships **v2** — `.golangci.yml` needs
   `version: "2"` and a `formatters:` block separate from `linters:`).
2. **Services (`compose.yaml`):** reuse the image, override command + mounts.
   - `builder` — `go vet ./... && go build ./...`. Compile gate.
   - `test-runner` — `go test -json -race -shuffle=on -count=1 -timeout=<T> ./...` with
     `GOFLAGS=-mod=vendor`; writes `test-results.json` + text tee (flags rationale in
     `testing-strategy.md` §7; `-mod=vendor` also enforces the vendor-deps rule).
   - `lint` — `gofmt -l` + `go vet` + `golangci-lint run` (config carries the `forbidigo` denylist,
     `testing-strategy.md` §7). **Hard pre-gate** (see §C): runs before any judgment review.
   - `oracle` — runs a `.luna` program through the frozen Phase-0 interpreter, emitting its gold
     result. Generates `e2e/expected/*` and is the differential reference. Mounted `:ro`.
   - `diff-runner` — the correctness harness (`testing-strategy.md` §3–5): differential + metamorphic
     + fuzz batch. Emits `diff-results.json` / `fuzz-results.json`.
   - `e2e-runner` — every `e2e/*.luna` (+ fresh fuzz batch) through the produced backend, compared to
     the oracle per policy (`testing-strategy.md` §2), **and** the §6-L4 stability check. Emits
     `e2e-results.json`.
3. **Invocation:** `podman compose run --rm <service>`. Machine gate outputs are artifacts, not
   scrollback — every loop-back decision keys off an exit code + a JSON file.

## B. Mount / capability matrix (kernel-enforced)

`:ro` on `tests/`, `e2e/`, and the frozen `oracle/` is what makes immutability real (a later,
last-match-wins volume line after the repo mount), not agent goodwill.

| Mount            | implement | build / test / lint | diff / e2e | reviews |
|------------------|-----------|---------------------|------------|---------|
| repo code        | rw        | ro                  | ro         | ro      |
| `tests/`         | **ro**    | ro                  | ro         | ro      |
| `e2e/`           | **ro**    | ro                  | ro         | ro      |
| `oracle/` (Ph 0) | **ro**    | —                   | ro         | ro      |
| `fuzz/` corpus   | ro        | —                   | rw         | ro      |
| `specs/` (spec)  | ro        | —                   | —          | ro      |
| `pipeline/` state| rw        | rw                  | rw         | rw      |

`spec-reconcile` is the one step that mounts `specs/` + `CHANGES.md` `rw`.

The Go caches (`GOPATH`, `GOCACHE`; named volumes, §A.1) are deliberately outside this matrix and
`rw` everywhere. They are derived data, not inputs — nothing's correctness keys off them, and a
`:ro` cache would simply turn every gate into a cold rebuild. Immutability discipline is about the
`tests/`/`e2e/`/`oracle/` **inputs** an agent must not edit its way past; it is not about caches.

## C. Retry metering + gate ordering

Principle: **count the mutating action, never the check.** Mechanical failures never burn a
judgment budget. **LLM reviews are triage, not a gate** — they filter low-hanging fruit ahead of the
human; the correctness *gate* is the `testing-strategy.md` harness, never an LLM sign-off.

| Counter          | Max | Increments on                                             | Exhaustion |
|------------------|-----|----------------------------------------------------------|------------|
| `reconcile`      | 2   | each `spec-reconcile` run *and* its "edits unsafe" abort  | hard-fail to human (step 1) |
| `test_builder`   | 2   | each `test-builder` run                                  | HARD STOP for human |
| `review_fix`     | 2   | each fix round across sec/perf/code (shared)             | carry findings into `human-review-final` |

`spec-review`, the builder/test/lint/diff gates, and review re-confirms are unmetered — checks. The
differential gate's *failure* is human-adjudicated, not auto-looped (`testing-strategy.md` §3).

**Gate ordering inside implement→review:**

```
implement slice
  → builder gate (compile)                 [unmetered]
  → test-runner gate (behaviour)           [unmetered]
  → DIFFERENTIAL gate (oracle vs compiled, script vs compiled)  [unmetered; failure → adjudicate]
  → metamorphic / fuzz gate                 [unmetered]
  → LINT gate (gofmt/vet/golangci-lint)     [unmetered, HARD] ── fail ──▶ back to implement
  → sec / perf / code TRIAGE reviews        [review_fix budget]
        each must emit a counterexample/failing test where possible (runnable vs oracle)
        any fix ──▶ re-run lint ──▶ re-run sec+perf on touched files ──▶ code re-confirm
```

Re-review scope after a fix is `git diff <last-green-SHA>..HEAD` (§D). The correctness gates sit
*before* the reviews so the reviews are pure triage over already-verified-correct code.

## D. Artifact set + state schema

Everything lives in a tracked `pipeline/` dir. `git` is history; `state.json` is the cursor. Each
step reads listed artifacts → writes one → commits. Loop-back overwrites in place (git tracks
versions); the counters, not the tree, record that we've looped.

### `pipeline/state.json`

```
{
  "target":    "<spec subsystem picked in step 1; decomposed into slices at step 6>",
  "step":      "<current step name>",
  "counters":  { "reconcile": 0, "test_builder": 0, "review_fix": 0 },
  "last_green": {                    // last-known-good SHAs; sec/perf/code also scope re-review
      "build": "<sha>", "test": "<sha>", "lint": "<sha>", "diff": "<sha>",
      "sec": "<sha>", "perf": "<sha>", "code": "<sha>", "e2e": "<sha>"
  },
  "gates":     { "build": "pass|fail", "test": "...", "diff": "...", "lint": "...", ... },
  "human":     { "spec_review": "pass|fail|pending", "review_2": "...", "final": "..." }
}
```

### Immutable inputs (`:ro`)

- `oracle/` — frozen Phase-0 interpreter. `tests/` — unit tests (trace to spec sections).
  `e2e/*.luna` + `e2e/expected/*` — programs with **oracle-generated** expected output.

### Mutable, regenerated on loop-back

- `pipeline/gaps.md`, `implementation-plan.md`, `slices.md`, `pipeline/reviews/{sec,perf,code}.md`,
  `fuzz/` corpus (grows).

### Machine gate outputs (regenerated every run)

- `test-results.json`, `diff-results.json`, `fuzz-results.json`, `e2e-results.json`, lint/build logs.

### Spec-governance artifacts (real repo files)

- `CHANGES.md` ruling append + swept `specs/` files — written by `spec-reconcile`.

### Human markers

- `pipeline/gates/pass-*.md` / `fail-*.md` for the three hard stops (step 4 `spec-review-human`,
  step 10 `human-review-2`, step 17 `human-review-final`).

### The `last_green` seam

A gate that never passed has no SHA → full scope; once it passes, its SHA advances → subsequent
`sec`/`perf`/`code` re-reviews are diff-scoped (`git diff <sha>..HEAD`). Works only because
execution is one linear history.

## E. Quality tiers (feeds `quality.md`)

Tier 1 runs as the §C lint gate **before** any reviewer token is spent; the metered LLM reviews
spend attention only on judgment.

### Tier 1 — lint-enforceable (hard gate, unmetered)

- `gofmt`; `go vet`; `golangci-lint` (idiomatic-Go, the mechanizable half of "idiomatic Go, pending
  `style.md`").
- `forbidigo` determinism denylist (`testing-strategy.md` §7), scoped to `_test.go`.
- Doc-comment presence on exported identifiers (godoc is a **separate category** from the
  why-not-what inline rule — required, not a violation).

### Tier 2 — judgment (LLM triage, metered)

- **Naming** — name things what they are; don't reuse names.
- **Comments** — inline comments explain *why*, never *what*/*how*. Exported doc comments exempt.
- **Imperative shell, pure-ish core** — isolate effects (IO, exec, `reveal`) at the edges; the core
  threads state but does no IO. "Effects at the edges," not "the core is pure."
- **DRY-with-intent** — duplication is fine when copies are expected to diverge; DRY only where the
  coupling is warranted. Intent, not mechanical.
- **Function complexity — rubric, not line count.** Judge nesting depth, independent concerns,
  tangled vs flat control flow. A flat 300-line dispatch `switch` passes; a short deeply-nested
  stateful function fails. `gocyclo`/`nestif` are **fed to the reviewer as a signal**, not gated.

## F. Alpha scope & architecture

- **IR / desugaring — already in the spec, purposely underspecified.** The spec has both an LIR and
  desugaring for exactly this reason. Phase 1's backend consumes the **desugared LIR** from the
  human front-end (not the surface AST), so the emitter targets a small core — matching Luna's
  small-surface ethos and giving the differential oracle a clean checkpoint.
- **Standard library — deliberately tiny for alpha.** `io`, `stringBuilder`, and perhaps 1–2 others.
  Library design is a separate problem; with the language tools in hand an arbitrary std can be
  designed later. Alpha ships the minimum the e2e programs need.
- **Incremental cache — file/module-granular for alpha (decided).** Content-hash per module, layered
  on Go's own build cache. Query-based/demand-driven (Salsa/rustc-style) is the post-alpha path;
  retrofitting incrementality is expensive, so the granularity is fixed now.
- **Diagnostics — volatile in alpha.** Error-message wording will churn constantly; tests pin error
  *type* + location only (`testing-strategy.md` §2), and diagnostic quality is deferred.

## G. Open TBDs

- **`quality.md` / `style.md`** — Tier 2 rubrics and the Go style guide `golangci-lint` points at.
- **`-timeout=<T>`** — the test-runner timeout that turns naive-algorithm hangs into failures without
  flaking on a slow-but-correct build.
- **Benchmarks (deferred)** — overflow-enforcement tax (checked vs raw Go arithmetic), incremental
  build latency, binary size. Baseline once the backend exists; gate on regression, not absolutes.
- Testing TBDs (fuzz generator, EMI, translation validation, mutation testing) live in
  `testing-strategy.md` §9.
