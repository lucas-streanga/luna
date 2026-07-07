# claude-agent-plan

Implementation plan for the pipeline in `proposed-claude-pipeline.md`. Two phases: **Phase 0**
hand-builds the oracle (a naive complete interpreter) and the human-owned semantic core;
**Phase 1** is the automated loop that builds the production Go-emitting backend and validates
it **differentially against the oracle**. This file holds the Phase-0 discipline, tooling/
container changes, retry metering, the mount matrix, the artifact set, the quality tiers, the
correctness harness (§G), and the non-determinism detector (§F). Serial, no subagent
parallelism.

> None of the podman/agent changes below have been made yet — this is the plan of record, not
> a description of the current tree. The current `compose.yaml` mounts the whole repo `rw` and
> the `Containerfile` has no Go toolchain.

## Phase 0 — Foundations (human-owned, before the automated loop)

The oracle is the keystone: it replaces "LLM-authored expected outputs" with an independent
implementation, which is what makes the whole assurance story work. Two artifacts must exist
before Phase 1 runs, and neither is built by the automated loop.

1. **Type checker + semantic analysis — hand-built by the human.** The highest-ambiguity,
   most-novel part (structural subtyping, unions/intersections, refinement constraints,
   capability/effect propagation). This *is* the deep narrow bore that proves the spec is
   implementable; gaps found here become `CHANGES.md` rulings + sweeps, by hand, before
   anything freezes. If the spec can't survive a real refinement-aware structural checker,
   stop — nothing downstream rescues that.
2. **Oracle / alpha interpreter — naive, slow, obvious, end-to-end.** Lex → parse → (1's
   checker) → tree-walk eval; bignum-then-bound-check for overflow; zero optimization. This is
   **alpha v0** and the **oracle** for all of Phase 1. Frozen and mounted `:ro` to the
   differential gate.
3. **Independence discipline.** A differential test only has power if the two sides fail
   *differently*. Tree-walk vs Go-emission is structurally different code → implementation bugs
   uncorrelated for free. Residual *shared spec-misreading* is covered by the human owning the
   semantic core and adjudicating every oracle-vs-compiler disagreement. **Alpha choice:** Phase
   1 reuses the human front-end and builds only the **backend** (Go emission + runtime +
   incremental cache); the shared front-end is not differential-tested — acceptable for alpha,
   closable later by an independent front-end.

## A. Tooling / container changes (prerequisite)

Nothing runs until the sandbox can build and test Go.

1. **Image (`Containerfile`):** add Go, pinned to **1.26.4** (current stable). Prefer Fedora's
   `golang`; if `dnf` lags, fall back to the official arm64 tarball — but heed the existing
   caution about non-dnf aarch64 binaries under Asahi's 16K pages (a 4K-aligned binary aborts
   at startup). Also set `go 1.26` in `go.mod`: the Containerfile pin is the **toolchain**,
   `go.mod` the **language floor**. Add `libfaketime` for the §F layer-4 clock perturbation.
2. **Services (`compose.yaml`):** reuse the image, override command + mounts.
   - `builder` — `go vet ./... && go build ./...`. Compile gate.
   - `test-runner` — `go test -json -race -shuffle=on -count=1 -timeout=<T> ./...` with
     `GOFLAGS=-mod=vendor`; writes `test-results.json` + text tee. `-count=1` defeats the cache;
     `-race`/`-shuffle=on` are determinism gates; `-timeout` turns a naive-algorithm hang into a
     failure; `-mod=vendor` enforces the vendor-deps rule at the toolchain level.
   - `lint` — `gofmt -l` + `go vet` + `golangci-lint run` (config carries the §F `forbidigo`
     denylist, scoped to `_test.go`). **Hard pre-gate** (see §C): runs before any judgment
     review; failures never reach a reviewer.
   - `oracle` — runs a `.luna` program through the frozen Phase-0 interpreter, emitting its gold
     result (stdout, or error/panic type). Used to generate `e2e/expected/*` and as the
     differential reference. Mounted `:ro`.
   - `diff-runner` — the §G correctness harness: differential (oracle vs compiled, script vs
     compiled) + metamorphic battery + a fuzz batch. Emits `diff-results.json` /
     `fuzz-results.json`.
   - `e2e-runner` — every `e2e/*.luna` (+ a fresh fuzz batch) through the produced backend,
     compared to the oracle per policy (exact success stdout; partial error type + location;
     partial panic type), **and** the §F layer-4 stability check. Emits `e2e-results.json`.
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
| `docs/` (spec)   | ro        | —                   | —          | ro      |
| `pipeline/` state| rw        | rw                  | rw         | rw      |

`spec-reconcile` is the one step that mounts `docs/` + `CHANGES.md` `rw`.

## C. Retry metering + gate ordering

Principle: **count the mutating action, never the check.** Mechanical failures never burn a
judgment budget. **LLM reviews are triage, not a gate** — they filter low-hanging fruit ahead of
the human; the correctness *gate* is the §G harness, never an LLM sign-off.

| Counter          | Max | Increments on                                             | Exhaustion |
|------------------|-----|----------------------------------------------------------|------------|
| `reconcile`      | 2   | each `spec-reconcile` run *and* its "edits unsafe" abort  | hard-fail to human (step 1) |
| `test_builder`   | 2   | each `test-builder` run                                  | HARD STOP for human |
| `review_fix`     | 2   | each fix round across sec/perf/code (shared)             | carry findings into `human-review-final` |

`spec-review`, the builder/test/lint/diff gates, and review re-confirms are unmetered — checks.

**Gate ordering inside implement→review:**

```
implement slice
  → builder gate (compile)                 [unmetered]
  → test-runner gate (behaviour)           [unmetered]
  → DIFFERENTIAL gate (oracle vs compiled, script vs compiled)  [unmetered, §G]
  → metamorphic / fuzz gate                 [unmetered, §G]
  → LINT gate (gofmt/vet/golangci-lint)     [unmetered, HARD] ── fail ──▶ back to implement
  → sec / perf / code TRIAGE reviews        [review_fix budget]
        each must emit a counterexample/failing test where possible (runnable vs oracle)
        any fix ──▶ re-run lint ──▶ re-run sec+perf on touched files ──▶ code re-confirm
```

Re-review scope after a fix is `git diff <last-green-SHA>..HEAD` (§D). The correctness gates
(differential/metamorphic) sit *before* the reviews so the reviews are pure triage over
already-verified-correct code.

## D. Artifact set + state schema

Everything lives in a tracked `pipeline/` dir. `git` is history; `state.json` is the cursor.
Each step reads listed artifacts → writes one → commits. Loop-back overwrites in place (git
tracks versions); the counters, not the tree, record that we've looped.

### `pipeline/state.json`

```
{
  "target":    "<backend slice picked in step 1>",
  "step":      "<current step name>",
  "counters":  { "reconcile": 0, "test_builder": 0, "review_fix": 0 },
  "last_green": {                    // base SHAs for diff-scoped re-review
      "build": "<sha>", "test": "<sha>", "lint": "<sha>", "diff": "<sha>",
      "sec": "<sha>", "perf": "<sha>", "code": "<sha>", "e2e": "<sha>"
  },
  "gates":     { "build": "pass|fail", "test": "...", "diff": "...", "lint": "...", ... },
  "human":     { "spec_review": "pass|fail|pending", "review_2": "...", "final": "..." }
}
```

### Immutable inputs (`:ro`)

- `oracle/` — frozen Phase-0 interpreter (the reference).
- `tests/` — unit tests, trace to spec sections.
- `e2e/*.luna` + `e2e/expected/*` — programs (hand/fuzzer) with **oracle-generated** expected
  output.

### Mutable, regenerated on loop-back

- `pipeline/gaps.md`, `implementation-plan.md`, `slices.md`, `pipeline/reviews/{sec,perf,code}.md`,
  `fuzz/` corpus (grows).

### Machine gate outputs (regenerated every run)

- `test-results.json`, `diff-results.json`, `fuzz-results.json`, `e2e-results.json`, lint/build logs.

### Spec-governance artifacts (real repo files)

- `CHANGES.md` ruling append + swept `docs/` files — written by `spec-reconcile`.

### Human markers

- `pipeline/gates/pass-*.md` / `fail-*.md` for the three hard stops.

### The `last_green` seam

SHAs in `last_green` are the boundary between "run everything" and "run the diff." A gate that
never passed has no SHA → full scope; once it passes, its SHA advances → subsequent re-reviews
are diff-scoped. Works only because execution is one linear history.

## E. Quality tiers (feeds `quality.md`)

Tier 1 runs as the §C lint gate **before** any reviewer token is spent; the metered LLM reviews
spend attention only on judgment.

### Tier 1 — lint-enforceable (hard gate, unmetered)

- `gofmt` formatting; `go vet`; `golangci-lint` (idiomatic-Go, the mechanizable half of
  "idiomatic Go, pending `style.md`").
- `forbidigo` determinism denylist (§F layer 2), scoped to `_test.go`: bans `time.Now`/`Since`,
  `math/rand`·`crypto/rand`, `os.Getenv`/`Args`/`Hostname`/`Getpid`, `runtime.NumGoroutine`,
  `net.Dial`/`Listen`.
- Doc-comment presence on exported identifiers (godoc is a **separate category** from the
  why-not-what inline rule — required, not a violation).

### Tier 2 — judgment (LLM triage, metered)

- **Naming** — name things what they are; don't reuse names.
- **Comments** — inline comments explain *why*, never *what*/*how*. Exported doc comments exempt.
- **Imperative shell, pure-ish core** — isolate effects (IO, exec, `reveal`) at the edges; the
  core threads state but does no IO. "Effects at the edges," not "the core is pure."
- **DRY-with-intent** — duplication is fine when copies are expected to diverge; DRY only where
  the coupling is warranted. Intent, not mechanical.
- **Function complexity — rubric, not line count.** Judge nesting depth, independent concerns,
  tangled vs flat control flow. A flat 300-line dispatch `switch` passes; a short deeply-nested
  stateful function fails. `gocyclo`/`nestif` are **fed to the reviewer as a signal**, not gated
  (they false-positive on the big switch). Encoding the reasoning keeps judgment reproducible.

## F. Non-determinism detection (four layers)

e2e comparison is exact, so any non-determinism in an e2e program is a latent flake. Enforced by
construction, in four layers — no single layer complete, only the first *sound*.

Reframe: Luna gates every outside effect through a fine-grained capability named in `use`, so the
source of nearly all non-determinism is syntactically visible in a program's `use` set.

- **L1 Capability audit (mechanical, hard, *sound*).** Allowlist only the stdout capability; any
  clock/random cap, `exec`, `reveal`, fs, net → fail. Sound because an undeclared outside effect
  is a language violation; fine-grained caps distinguish "writes stdout" from "reads the clock."
- **L2 Identifier denylist (mechanical, hard, a net).** `.luna`: grep/AST for time/random/env/
  address/fs/net families. Go `_test.go`: the §E `forbidigo` list. Provisional — shrinks as the
  std capability taxonomy lands and L1 tightens.
- **L3 Concurrency-observable-output smell (judgment).** ≥2 concurrent tasks writing observable
  output is non-deterministic; flag unless join-then-print. Rare; forced empirically by L4's
  GOMAXPROCS perturbation.
- **L4 Empirical stability (mechanical, hard, backstop).** Run N times (3–5), require identical
  stdout, then perturb: `libfaketime` at two far-apart wall times; two `TZ`; `GOMAXPROCS=1` vs
  `8`; two cwds. Any diff fails. Runs at `e2e-define` (before lock) and again at `e2e-runner`.

Go test-*harness* determinism is covered by `go test -race -shuffle=on -count=N` + `forbidigo`;
the judgment remainder is a `test-review` smell. Where: L1+L2+L4 hard at `e2e-define` (L4 again
at `e2e-runner`); L3 judgment at `e2e-define`.

## G. Correctness harness (oracle / differential / metamorphic / fuzz)

The spine that replaces LLM-authored expected outputs with independent verification. The Phase-0
oracle is the keystone; the other three are what it enables.

- **Oracle.** The frozen tree-walk interpreter (`:ro`). For any program it yields the gold result
  (stdout, or error/panic type). Slow by design — a test instrument, not the product.
- **Differential.** Two independent runs must agree: (a) oracle vs compiled binary (tree-walk vs
  emitted-Go semantics); (b) script-mode vs compiled binary — Luna's own "same source runs or
  compiles, identical semantics" claim, a differential oracle the language hands you free. Any
  divergence fails and is human-adjudicated (compiler bug or spec gap).
- **Metamorphic** (input transforms with a predicted effect on output — no gold value needed):
  - *Preserving* (output must not change): rename identifiers; reorder independent declarations;
    wrap an expression in an identity function; extract/inline a `const`; add dead code; reformat.
  - *Invariant-mandated* (from Luna's governing ideas): nudge an operand past `maxInt` → **must
    panic** (never wrap); `==` across coerced types → strict (type error / false); strip a used
    capability from `use (...)` → **must fail to compile**; shared mutable state across tasks →
    **must not typecheck**. Each invariant is a relation generator; the fuzzer manufactures cases.
- **Fuzz.** A grammar-based generator emits random *valid* Luna programs (seed the grammar from
  `tooling/tree-sitter-luna`), plus `go test -fuzz` for byte-level mutation of seeds. Every
  program feeds differential (graded by the oracle) and the metamorphic battery — many tests, zero
  hand-authored outputs. This is what makes fuzzing find *wrong answers*, not just crashes.

Gates at step 11 (per-slice differential + metamorphic/fuzz) and step 16 (full e2e + fresh fuzz
batch). Not mutation testing — that mutates *code* to grade the suite (§H, future).

## H. Open TBDs

- **`quality.md` / `style.md`** — Tier 2 rubrics and the Go style guide `golangci-lint` points at.
- **Fuzz generator** — grammar-based `.luna` generator design (seeded from tree-sitter); the
  metamorphic-transform set to ship first.
- **`-timeout=<T>`** — the test-runner timeout that turns naive-algorithm hangs into failures
  without flaking on a slow-but-correct build.
- **Layer-2 denylist scope** — exact `.luna` identifier families, firmed up once the std
  capability taxonomy is designed (L1 then subsumes most of it).
- **Benchmarks (deferred)** — overflow-enforcement tax (checked vs raw Go arithmetic), incremental
  build latency, binary size. Baseline once the backend exists; gate on regression, not absolutes.
- **Mutation testing (future)** — mutate the backend to confirm the differential+metamorphic+fuzz
  suite actually catches the change; a suite-quality meter, distinct from metamorphic (§G).