# testing-strategy

The correctness spine for the Luna compiler pipeline, extracted from `claude-agent-plan.md` so
the plan stays orchestration-only. Referenced by `proposed-claude-pipeline.md` (step gates) and
`claude-agent-plan.md` (§A services, §C ordering). Context: Phase 0 hand-builds a naive
interpreter that is both **alpha v0** and the **oracle**; Phase 1's automated loop builds the
Go-emitting backend and is validated **against the oracle**, never against LLM-authored expected
outputs.

## 1. The oracle (keystone)

The Phase-0 tree-walk interpreter, mounted `:ro`. For any program it yields the gold
result (stdout, or error/panic type). Slow by design — a test instrument, not the product.

**"Frozen" means frozen to the agent loop, not feature-complete** (R234). The `:ro` mount is what
stops an agent editing its way past a failing differential; it is not a claim that the oracle stops
changing. A reference that cannot track rulings drifts from the spec on the first one after it is
written, and then every divergence is ambiguous — compiler bug or stale oracle? So the oracle has a
**human edit path outside the loop**, and language changes land in it like anywhere else.

**It is permanent, and it ships exactly once** (R234). The oracle is *alpha v0* — the first
shippable Luna, slow but complete — and stops shipping once a stable Luna-written implementation
exists. It never stops being maintained: after self-hosting it is the only implementation the
production compiler did not produce, so retiring it would end differential testing (§3). The
production compiler **may never import it** (compiler §6.1), gated on the build graph.

**Independence discipline** (what gives it power): a differential test only bites if the two
sides fail *differently*. Tree-walk vs Go-emission is structurally different code → implementation
bugs uncorrelated for free. Residual *shared spec-misreading* is covered by the human owning the
semantic core (Phase 0.1) and adjudicating every oracle-vs-compiler disagreement. Alpha reuses the
human front-end (lex/parse/check/desugar → LIR) and builds only the backend, so the front-end is
not differential-tested — acceptable for alpha, closable later by an independent front-end.

## 2. e2e comparison policy

`e2e/*.luna` programs (hand-written or fuzzer-seeded) with **oracle-generated** expected output —
the oracle defines "correct." Enforced by `e2e-runner`:

- *Success stdout* — **exact**. Maps and streams have ruled iteration order (no sets), so a program
  with no undeclared effects and no concurrent multi-writer output is deterministic.
- *Diagnostics / compile errors* — **partial**: match the ruled error **type** (`typeError`,
  `ioError`, …) + source location, never the prose.
- *Panics* — **partial**: match the panic type, not the address/stack dump.

**Alpha note:** diagnostics are highly volatile in alpha — message wording will churn constantly,
so tests pin type + location only, and diagnostic *quality* is deferred (not a gate).

Non-determinism is excluded by construction (§6), not author discipline.

## 3. Differential testing

Two independent runs must agree:
- **oracle vs compiled binary** (tree-walk vs emitted-Go semantics), and
- **script-mode vs compiled binary** — Luna's own "same source runs or compiles, identical
  semantics" claim, a differential oracle the language hands you free.

**The second pair degenerates after self-hosting** (R234), and this is why the first is
load-bearing forever: once one Luna-written compiler drives both paths, script-vs-compiled tests
the run/cache path rather than two readings of the semantics. Oracle-vs-compiled is then the only
pair with two independent implementations behind it.

**Failure routing** (this is effectively a fourth human touchpoint, unmetered): a divergence is
**human-adjudicated** → *compiler bug* (loop back to `implement`, pipeline step 11) or *spec gap*
(back to `spec-reconcile`, pipeline step 3). It is **not** a blind auto-loop — a spec-gap verdict
must not churn `implement`.

## 4. Metamorphic testing

Input transforms with a predicted effect on output — no gold value needed:

- **Preserving** (output must not change): rename identifiers; reorder independent declarations;
  wrap an expression in an identity function; extract/inline a `const`; add dead code; reformat.
- **Invariant-mandated** (straight from Luna's governing ideas; each invariant is a relation
  generator): nudge an operand past `maxInt` → **must panic** (never wrap); `==` across coerced
  types → strict (type error / false); strip a used capability from `use (...)` → **must fail to
  compile**; shared mutable state across tasks → **must not typecheck**.

**EMI (Equivalence Modulo Inputs) — noted, to research (Lucas).** A stronger, *input-directed*
version of "add dead code": profile which statements a program does **not** execute for a given
input, mutate/delete only that dead code, and require identical output for that input. Found
hundreds of GCC/LLVM bugs ([PLDI'14](https://www.vuminhle.com/pdf/pldi14-emi.pdf)). Adopt into the
preserving battery later.

Distinct from **mutation testing** (§9), which mutates *code* to grade the suite, not input.

## 5. Fuzzing

- **Grammar-based generator** emits random *valid* Luna programs (seed the grammar from
  `tooling/tree-sitter-luna`); plus `go test -fuzz` for byte-level mutation of seeds.
- Every program feeds §3 differential (graded by the oracle) and the §4 metamorphic battery — many
  tests, zero hand-authored outputs. This is what makes fuzzing find *wrong answers*, not just
  crashes.
- **Csmith's lesson, applied:** a generator must avoid producing programs whose behaviour the spec
  leaves unspecified, or you can't tell a wrong-code bug from legitimate divergence
  ([Csmith/Regehr](https://blog.regehr.org/archives/804)). For Luna this is cheap because it is
  safe-by-construction — and **§6 *is* that constraint**: the generator is restricted to Luna's
  deterministic subset.
- **Infra vs content (resolves the earlier conflation):** the generator + `diff-runner` are
  **build-once infrastructure** (still a TBD, §9), *not* rebuilt per target. `test-builder`
  (pipeline step 7) authors only **per-target content** — the metamorphic relations and seed
  programs — on top of that infra.

## 6. Non-determinism detection (four layers)

e2e comparison is exact, so any non-determinism is a latent flake. Enforced by construction — no
single layer complete, only the first *sound*. Reframe: Luna gates every outside effect through a
fine-grained capability named in `use`, so the source of nearly all non-determinism is
syntactically visible in a program's `use` set.

- **L1 Capability audit (mechanical, hard, *sound*).** Allowlist only the stdout capability; any
  clock/random cap, `exec`, `reveal`, fs, net → fail. Sound because an undeclared outside effect is
  a language violation; fine-grained caps distinguish "writes stdout" from "reads the clock."
- **L2 Identifier denylist (mechanical, hard, a net).** `.luna`: grep/AST for time/random/env/
  address/fs/net families. Go `_test.go`: the §7 `forbidigo` list. Provisional — shrinks as the std
  capability taxonomy lands and L1 tightens.
- **L3 Concurrency-observable-output smell (judgment).** ≥2 concurrent tasks writing observable
  output is non-deterministic; flag unless join-then-print. Rare; forced empirically by L4's
  GOMAXPROCS perturbation.
- **L4 Empirical stability (mechanical, hard, backstop).** Run N times (3–5), require identical
  stdout, then perturb: `libfaketime` at two far-apart wall times; two `TZ`; `GOMAXPROCS=1` vs `8`;
  two cwds. Any diff fails.

## 7. Go test harness (distinct from programs under test)

- `go test -race -shuffle=on -count=1 -timeout=<T>`: `-race` (data races), `-shuffle=on` (test-order
  dependence), `-count=1` (defeat cache), `-timeout` (naive-algorithm hang → failure).
- `forbidigo` determinism denylist, scoped to `_test.go`: bans `time.Now`/`Since`,
  `math/rand`·`crypto/rand`, `os.Getenv`/`Args`/`Hostname`/`Getpid`, `runtime.NumGoroutine`,
  `net.Dial`/`Listen`.

## 8. Gate map (where each runs)

| Check | Pipeline step |
|---|---|
| L1 + L2 + L4 + oracle-gold generation | 5 `e2e-define` (before lock) |
| **test-file lint** (`gofmt` + `forbidigo`, while tests still mutable) | 8 |
| coverage / metamorphic-relation completeness / L3 judgment | 9 `test-review` |
| differential (oracle vs compiled, script vs compiled) + metamorphic/fuzz | 11 `implement` |
| Go-harness determinism (`-race -shuffle`) | 11 `test-runner` gate |
| full e2e + fresh fuzz batch + L4 re-run | 16 `e2e-runner` |

Note the L2/`forbidigo` fix: the test-file lint runs at **step 8**, while `test-builder` output is
still mutable — not at the step-12 impl lint gate, by which point tests are frozen `:ro` and a
determinism smell would have no legal remedy.

## 9. Deferred / future

- **Fuzz generator (TBD)** — grammar-based `.luna` generator design (seed from tree-sitter); which
  metamorphic transforms ship first.
- **EMI** — §4; noted, Lucas to research, adopt later.
- **Translation validation (deferred)** — SMT-proving a pass preserves semantics; the principled
  answer for the desugar→LIR→emit passes, but alpha-overkill.
- **Mutation testing (future)** — mutate the backend to confirm the differential+metamorphic+fuzz
  suite actually catches the change; a suite-quality meter, distinct from metamorphic (§4).

Sources: [EMI, PLDI'14](https://www.vuminhle.com/pdf/pldi14-emi.pdf) ·
[Csmith / Regehr](https://blog.regehr.org/archives/804) ·
[Survey of compiler fuzzing](https://arxiv.org/pdf/2306.06884)
