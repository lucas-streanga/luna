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
`foreach` boundary as an emission detail; insufficient as the stream's identity.

## 4. Costs, honestly

Per stream: one block allocation (a few dozen bytes plus hoisted locals). Per element: an
indirect call, a switch, and the value move — with the peek path adding one flag test. Per
chain stage: the same again. The state-machine transform is the one genuinely complex piece
of the emitter this file buys, and it is bought once, on lowered IR, for every stream in
the language.
