# Stream Representation

How Luna stores streams and runs generators at runtime. The fifth internals sibling
(value-representation, string-representation, table-representation, function-representation):
it describes the payload a `stream`-typed `lval`'s `ptr` points at, and how a generator body
becomes resumable (R207). The semantics are the stream spec's (lazy start §1.2, single-pass
§2, peek/emptiness §3, restart §4, chains §7, defer-on-exhaustion §1.3); this file is the
hosted realization and the reasoning, including the two rejected implementations.

---

## 1. The stream block

```
streamBlock:
  state     int32                       // resume label; 0 = unstarted
  flags     (peeked, canRestart, taken, exhausted)
  peekSlot  lval                        // the one-element buffer (stream §3)
  resumeFn  func(block) (lval, bool)    // the compiled step function (§2)
  env + hoisted body locals             // captures, loop variables, the implicit-key counter (R93)
```

- **Lazy start is free**: it *is* `state == 0` — constructing a stream allocates the block
  and runs nothing (stream §1.2 realized by representation, not machinery).
- **The peek buffer is exactly one `lval`** plus a `peeked` bit: `peek`/`isEmpty` fill it
  (running the body just far enough, stream-api §2), the next pull drains it first. Nothing
  in the API demands more lookahead, so nothing more exists.
- **`taken` lives here** — value-representation §2.1 already ruled consumption state
  referent-side ("beside the stream's cursor"); this block is that referent. `exhausted` is
  the done sentinel the R207 ordering stores *before* final defers run (stream §1.3).
- A generator fn is an ordinary fn whose descriptor has the `generator` bit
  (function-representation §2, R206): its **native entry constructs this block** instead of
  running the body.

## 2. Generator bodies compile to state machines

`yield` lowers by **state-machine transformation** on the already-desugared IR (compiler
§1.5 — branches and loops only, which keeps the transform tractable): the body becomes a
step function switching on `state`, its locals hoisted into the block; each `yield` stores
the value, sets the resume label, returns. A pull is one indirect call and a switch —
nanoseconds, no synchronization, no stack.

- **Abandonment is just garbage.** A dropped stream is an unreferenced block; the collector
  reclaims it. The generator-leak problem is not solved — it never exists (§3's contrast).
- **Restart is two stores** (stream §4, R105's "replay"): `state := 0`, clear hoisted
  locals, keep the immutable const-snapshot env — gated by the existing `canRestart` bit
  (ranges and string-derived streams yes; file streams no, R121). Under the rejected
  goroutine design this operation was *impossible* (kill-and-respawn, and goroutines cannot
  be killed).
- **Chains are uniform**: `s.map(f)` is a small fixed streamBlock whose `resumeFn` pulls
  its upstream pointer (transferred in, source marked taken — stream §7.3); pull-driven
  composition all the way down, every stage the same shape.
- **Defer-on-exhaustion costs nothing** (stream §1.3, R207): the body's top-level defers
  compile into the final resume states via the ordinary R148 machinery, with the one
  `exhausted := true` store hoisted before them (the mark-first ordering the semantics
  require). The abandonment half of that ruling is likewise representation-honest: no
  finalizer exists to run an abandoned stream's defers, which is exactly why the contract
  says they never run.

### 2.1 The lowering, generically (R208)

What can be pinned before the IR exists — starting with the constraint that dictates the
shape. The textbook emission (a switch whose `goto`s jump to resume labels inside loop
bodies, Duff-style) is **illegal in Go source**: `goto` may not jump into a block. The
general shape is therefore the **flattened dispatch loop** —

```go
for {
    switch b.state {
    case 3: /* basic block; ends with b.state = 5; continue */
    ...
    }
}
```

— every basic block a case, every jump a state assignment: fully general over any CFG and
legal Go (the same forced move regenerator makes for JavaScript). The honest cost — Go's
loop optimizations die inside the dispatch loop — is recovered by **structured islands**:
only the *yield spine* (constructs a `yield` is nested within) is flattened; every maximal
yield-free subtree emits as ordinary structured Go inside its case, so yield-free inner
loops stay real Go loops.

The algorithm, whose inputs are small *because* lowering already desugared everything
(compiler §1.5 — branches, loops, calls, `yield`, `return`, defer-reach, try-regions):

1. build the CFG;
2. cut suspension edges at yields (store the value, set the resume state, return);
3. **liveness → the hoist set**: only locals *live across a suspension* move into the
   block; the rest stay Go locals inside their islands, register-allocated by Go
   (hoist-everything is the correct, fatter v1 — a knob);
4. partition into resume regions; emit the pc-loop with islands;
5. append the R207 exhaustion tail (mark done → drain defers → report done) — mechanical.

Two interactions with existing rulings, both discovered by this analysis:

- **R148's defer machinery relocates for generator frames** (compiler §7.3 carries the
  carve-out): registration-on-reach makes the pending-defer list runtime state, and R148
  parks it per-*task* — which assumes frames that do not outlive their activation. A
  generator frame suspends, so **its defer list lives in the stream block**, surviving
  across pulls and stream handoffs; the exhaustion states drain the block's list.
- **`try` spanning a `yield`: ruled out at parse** (stream §1, R210) — the transform's
  hardest corner, deleted rather than built. The machinery a spanning try would have
  demanded is recorded as the road not taken: a Luna `try` enclosing a yield spans multiple
  `resumeFn` invocations while Go's `recover` is per-call-frame, so protection would have
  re-established from state via a *handler-range table* (state → active handlers, a recover
  per resume routing panics to the correct catch state — C#'s iterator exception design),
  multiplied by its worst neighbors (nested trys, per-state defer-drain depths, rethrow).
  The restriction costs no expressiveness — **a spanning catch abandons the rest of its try
  body anyway**, so it only ever offered grouping sugar over the per-element try and the
  consumer-side supervisor, both ruled idioms — and it keeps every resume frame's
  protection ordinary R148 machinery. Catch bodies may still yield (post-recovery code is
  pc-shaped); try-expressions are structurally immune; defer bodies were already banned
  (R207).

And the parity note that closes the loop with R192: **the evaluator needs none of this** — both
tree-walkers, evaluator and oracle alike (compiler §6.1, two artifacts since R234), interpret IR
with an explicit stack, so a suspended generator is a saved interpreter frame. The state-machine
transform is *emitter-only*, sitting exactly on the divergence surface the oracle patrols:
generator semantics are differentially testable by construction, and comptime generator folding
(`const xs = collect(naturals())`, legal at requirement-mask zero) runs in the evaluator with zero
transform machinery.

### 2.2 The fused lowering (R225)

The state machine is the **general** lowering; where a stream cannot be observed *as a
value*, the emitter may skip it entirely — the **fused lowering**: producer and consumer
compile into one loop, no streamBlock, no `resumeFn` dispatch.
`foreach (x in (1..n).filter(p).take(k)) { body }` emits the counting loop a programmer
would have written — a `continue` for the filter, a counter break for the take, the body
inlined.

- **The license is the as-if rule**, elision's doctrine extended from checks to
  representation (constraints §9.5: never changes meaning, only removes what is provably
  redundant). The semantics fix what is observable — producer and consumer steps
  alternating per pull (stream §7.2), defers at the final pull (R207), panics propagating
  from the pull site — and a fused loop preserves that order **by construction**, because
  pull-driven single-pass semantics *is* loop semantics. The language, not analysis, is
  what makes fusion invisible: stream §7.3's enforced move outlaws aliasing a chain
  mid-consumption, and R222 closed the exhausted-handle path. Fusion is a fast path,
  never load-bearing; the general lowering is always correct.
- **Identification is a syntax-directed catalogue, not analysis** — the compiler's
  standing refusal of flow-sensitivity (compiler §1.4.1), in the §9.5 shape: mechanism
  fixed here, catalogue pending implementation. **Tier 1** fuses a chain **wholly visible
  at its consumption site**: the `foreach` head (or a spread / `collect` argument) is one
  expression whose source is a range, a `toStream`, a `gen` block, or a generator call,
  and whose stages are catalogue transformers with literal arguments. Nothing is bound,
  so nothing escapes, so nothing can observe — decidable from the parse tree alone. A
  second tier (a stream bound once, consumed in the same block) is deferred with the rest
  of the catalogue.
- **What blocks fusion is exactly observation**: binding and reusing any stage, `peek` /
  `isEmpty` / `restart`, crossing `spawn`, entering a table. Blocked fusion means the
  general lowering, nothing more.
- **Stage shapes**: `map(f)` a binding, `filter(p)` a `continue`, `take(k)` a counter
  break; a fused `gen` block or generator body inlines with each `yield` becoming the
  consumer body's entry — the §2.1 machinery unneeded exactly when no suspension survives
  fusion. Effect stages fuse unchanged: creation-site authorization (R121, capabilities
  §5.1) ran where the chain was written, and fusion moves no code across that boundary.
- Distinct from **comptime generator folding** (the parity note above): folding
  *evaluates* a capability-free chain at compile time in the evaluator (the compiler's own,
  not the test oracle — compiler §6.1, R234); fusion *emits better code* for a runtime chain.
  Different tiers, both meaning-preserving.

## 3. The rejected implementations, with full grounds

**Goroutine-per-generator** (yield over an unbuffered channel) — rejected four ways:

1. **The leak is unfixable politely.** `take(3)` on an infinite generator abandons a
   goroutine **blocked on send forever**; Go cannot kill it. The escapes are
   finalizer-driven cleanup (the machinery this corpus deletes wherever it appears) or a
   done-channel select at every yield (cost and complexity up). Abandonment is *normal*
   stream usage; a design that leaks a stack per abandonment is disqualified.
2. **Cost**: kilobytes of stack per stream and ~100ns of channel synchronization per
   element, against tens of bytes and a switch dispatch — 10–20× on the language's
   bulk-data idiom.
3. **Confinement fragility**: the runtime's soundness rests on one-task-one-thread-of-access
   (non-atomic COW counts §6.1, boxed `&`-cells); a hidden goroutine inside a task holds
   only by the lockstep-alternation argument, which any future buffering silently breaks.
   Load-bearing invariants must not rest on an implementation accident.
4. **The pinning escape does not exist**: Go has no co-scheduling primitive
   (`LockOSThread` pins the calling goroutine to an OS thread; it cannot marry two
   goroutines to one thread), so "run this goroutine on my thread" is not available even in
   principle.

**Range-over-func** (Go 1.23 push iterators) — considered, and rejected as *the*
representation: it compiles generator bodies with no transform at all (`yield` becomes a
callback call), but it is **push**, and Luna's stream surface is **pull** — `peek`,
`isEmpty`, and every two-source operation (`zip`, `merge`) require pulling from multiple
streams alternately, which push cannot express without reintroducing goroutines. Fine at a
`foreach` boundary as an emission detail; insufficient as the stream's identity. The
rejection sharpened to a theorem (R208): push offers early *stop*, never *suspend-resume*
— delivering element k+1 after stopping at k means replaying from scratch and skipping,
**resume-by-replay, O(n²)** over the stream's length, and legal only for restartable
sources in the first place.

## 4. Costs, honestly

Per stream: one block allocation (a few dozen bytes plus hoisted locals). Per element: an
indirect call, a switch, and the value move — with the peek path adding one flag test. Per
chain stage: the same again. The state-machine transform is the one genuinely complex piece
of the emitter this file buys, and it is bought once, on lowered IR, for every stream in
the language.
