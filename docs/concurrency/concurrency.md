# Concurrency

Concurrency operates on **tasks**. A task is Luna's unit of concurrent work, implemented as a green thread: lightweight, scheduled by the always-present
runtime (goroutines under the hood, compiler spec §7.3). Concurrency is built on three pieces, a
**`spawn`** that starts a task, a **`promise`** that stands for its future result, and an
**`await`** that collapses a promise to a value, and on one architectural decision that makes it
safe: **a task shares no mutable state.** Everything crossing a spawn boundary is copied, except
`const` (which is frozen, so safe to share by reference), so there are no data races by
construction, which is why there is no function coloring: any function may be spawned.

```
let p = spawn compute(x);       // start compute(x) as a task; p is its promise
let v = await p;                // wait for the result: a value, or an error (§4)
```

---

## 1. No coloring: any function may be spawned

There is **no** `async`/colored-function distinction. Any function, ordinary or not, may be
spawned with `spawn`, exactly as any eligible function may be called at comptime (functions spec
§5). This is possible *because* of the copy discipline (§2): since a task shares no mutable state,
there is no data race to prevent, and therefore no need to mark functions as concurrency-safe.
Coloring exists in other languages to manage shared-state hazards; Luna removes the hazard
structurally, so it removes the coloring.

`spawn f(args)` evaluates `args`, starts `f` as a **task**, and immediately returns a
**`promise`** (§3) for its eventual result. Green threads are light and the runtime is always
present, so spawning is cheap.

---

## 2. Isolation by copying: what crosses a spawn boundary

A spawned task runs on **its own copy** of everything it needs, so it cannot observe or mutate the
spawner's state (or a sibling task's). The base rule:

**Everything that crosses a spawn boundary is deep-copied, except a `const` value, which is shared
by reference because it is deep-frozen (variables spec) and therefore safe to share.**

This covers both channels by which data could enter a task, and both obey the rule:

- **Arguments** to `spawn f(args)` are deep-copied into the task (a `const` argument shared by
  reference).
- **Captures** of a spawned closure need no rule of their own anymore: a closure's environment
  is an implicit **deep-`const` snapshot** taken at creation (functions spec §2.1), so by the
  time a closure reaches `spawn` its environment is already frozen data with no live link to any
  binding. It crosses exactly as `const` does above, shared by reference where already frozen,
  with nothing mutable to copy-protect. Any stream the closure came to own was transferred at
  *capture* time (functions §2.3), not at spawn. The one referential capture in the language,
  `use` of a **capability**, is **shared by reference** (immutable, no slot to race on), which
  is how a task comes to hold `io` (§2.1, capabilities §7). So **every closure is spawnable**;
  the old compile error for spawning a `use`-captured `var` is gone because the capture it
  forbade no longer exists.

So a task's inputs are an isolated copy, and there is **no shared mutable state** between tasks or
between a task and its spawner. That is the whole safety story: no shared mutable state means no
data races on Luna values, with no locks and no coloring.

This same guarantee pays off inside the runtime: because a mutable table is sole-owned by one task
at every instant, its copy-on-write sharing count is only ever touched single-threaded, so
copy-on-write refcounting is **non-atomic** (value-representation §6.1). The copy discipline
localizes all concurrency to the spawn and await boundaries, leaving the runtime interiors
(refcounting, COW splits, table bookkeeping) as ordinary single-threaded code. The contract that
sustains this is that the boundary copy is **transitively deep for mutable data** (const sub-tables
may stay shared, since they carry no count).

### 2.1 The crossing taxonomy

Every kind of value crosses the boundary in exactly one way, and together these leave no path for
sharing mutable state:

- **Ordinary copyable values** (scalars, strings, tables, lists), **copied** (or, if `const`,
  shared by reference). Because `const` is deep-frozen and there is no lazy/memoized const
  evaluation (a read of a shared const is never secretly a write), reference-sharing a `const` is
  race-free.
- **Capabilities**, **shared by reference**. A capability is `nocopy` (capabilities spec §3) *and*
  immutable, zero-data, so it is const-natured and safe to share, and it cannot be smuggled inside
  a table, because a capability can never enter a value slot (capabilities spec §3.1). So a task
  can hold `io` and do I/O; `nocopy` and const-shareable coincide exactly for capabilities.
- **Streams and string builders**, **transferred**. These are stateful, single-owner,
  by-reference values (a stream's cursor, a builder's buffer are mutable through the reference),
  so they cannot be copied and cannot be `const` (deep-freeze is meaningless for a stateful
  value). They cross by **ownership transfer**: the spawner hands the stream or builder to the
  task, after which the spawner is **enforced-ly** prevented from touching it, the transfer marks
  the value **taken** (§2.3), and any later access through any alias panics. This is the
  ownership-transfer escape for large data (move a stream into a task rather than copy a table).
  Because such values can never be `const`, they can never ride the const-share path into
  *multiple* tasks, so the "stateful value shared through a frozen container" race cannot arise.
- **Sinks** (channels §3), **shared** (R119). A sink is a **write-only** handle whose channel
  interior is runtime synchronization machinery, not a Luna value: sharing it shares no
  readable state — *ownership follows readability*, and the readable half, the receive
  stream, is single-owner above. Copies and crossings all reference the one channel; that
  is fan-in, the feature. The taxonomy row is capabilities', for capabilities' reason.
- **Promises**, **confined** (§3): a promise cannot cross a spawn boundary in either direction.
- **References to mutable bindings**, **forbidden**. A reference that shares a `var`, i.e. a
  `&` argument (`spawn f(&t)`), would give the task a mutable reference to the spawner's
  binding, shared mutable state, a race. So **a `&` argument may not cross a spawn boundary**;
  it is a compile error. Mutation visible to the spawner is expressed by the task **returning**
  a value the spawner uses (§2.2), never by a shared reference. `&` arguments are the *only*
  case left to forbid, because a closure can no longer smuggle one: a closure's environment is
  an implicit deep-`const` snapshot (functions §2.1), there is no `use`-capture of a `var` in
  the language, so **any closure is spawnable** with no environment check at all. Its captured
  environment crosses by the rules above as the frozen data it is, already-`const` parts shared,
  and any stream it came to own at capture (functions §2.3) already transferred then. The one
  referential capture that exists, `use` of a **capability**, *does* cross (that is exactly how
  a task comes to hold `io`, capabilities §7; `spawn f(...)` is a direct-call form, not a value
  slot, capabilities §3.1), because a capability is immutable and zero-data, no slot to race on,
  the same reason capabilities cross by reference above.

The result: no live mutable value is ever reachable from two tasks at once. Copyable values are
copied, frozen values are shared read-only, stateful values transfer sole ownership, capabilities
are immutable, promises are confined, and mutable references cannot cross. Data races on Luna
values are impossible by construction. (Races on *external* state reached through a shared
capability, two tasks writing one file, are outside the value model and are the programmer's
responsibility, managed with the thread-safe resource APIs the standard library provides; §7.)

### 2.2 Results transfer back without a visible copy

When a task completes, its result flows back to whoever awaits it. Because the task is **done**, it
no longer touches the result, so the result **transfers** (moves) to the awaiter with no
semantically-visible copy. This is opaque: the caller cannot tell a move from a copy, so it is
purely an optimization (a task that builds a large table and returns it does not pay to copy it
back). Inputs are copied (task and spawner both live on); outputs move (the task is finished). A
task cannot return a **promise**, however (§3, confinement).

### 2.3 Transfer is enforced: the taken state

A transferred value's move is **not** a convention the spawner must remember; it is **enforced at
runtime**. Because Luna does no static move or use-after-consume analysis (compiler §1.4.1), "the
spawner stopped using it" cannot be a compile-time guarantee, so it is a runtime one, on the same
footing as reading an already-exhausted stream.

**The taken state lives on the value, not the binding.** A stream or builder is a by-reference
value that can be **aliased**, most sharply, a stream held in a table is shared by reference, so one
stream is reachable through several bindings and table slots at once (stream §6.1, tables §4).
Marking a single binding would leave the other aliases live and dangling. So transfer marks the
**referent**, the stream's / builder's heap state, beside its cursor / buffer, which **every** alias
dereferences: the direct binding, a table element, a captured copy, all see it. This is the same
principle as constraint enforcement, which follows the **value**, not the binding
(constraints §9.4, tables §6.2); taken is that rule applied to ownership, and it is deliberately
**not** the `undefined` mechanism, which is a per-*binding* `lval` flag that cannot live in a table
(undefined §3, §7, value-representation §2.1).

- **Set synchronously at spawn, on the spawner's thread, before the task runs.** The mark is written
  once, at the spawn point, then only read afterward (write-once-then-read-only), so the spawner's
  panic-checks and the task's ownership never race and **no atomic is needed**, the same discipline
  that lets the eager copy skip atomic refcounting (§2).
- **Using a taken value panics** (a `panic`, errors §9), immediately, on any access through any
  alias. This is **distinct from consumed**: a *consumed* stream yields **empty** on read (a normal
  end-state, no panic), a *taken* stream **panics**, because another task now owns it. The
  referent therefore carries a small terminal state, **active / consumed / taken**, not one
  reused bit.
- **`taken(x): bool`** is the non-panicking query. `const taken = fn (x: any): bool` reports whether
  `x` has been taken, reading the referent's state **without touching the value**. It returns
  `false` for any non-movable value (an `int` is never taken) and never panics, the total,
  *asking* counterpart to the panicking *use* (as `has` is to a hard key read, `is` to `as`). It is
  an ordinary **runtime** query, not comptime-foldable, since taken is set at spawn, at runtime.
  Because it reads the referent, `taken(s)` and `taken(t['s'])` **agree** after a move: both see the
  one moved stream.

**This dissolves the container question.** A copyable table that *contains* a stream needs no special
"move the whole table" rule: each element crosses by its **own** rule, the copyable parts are
deep-copied, the stream moves (its referent marked). So the task receives a copy of the copyable
parts plus the moved stream, and on the spawner's side the table survives, its copyable parts usable,
while the moved stream element panics on access:

```
var t = ['s' => makeStream(), 'n' => 5];
spawn f(t);        // copyable parts deep-copied; the stream moves (referent marked **taken**)
t['n'];            // 5: the copy is independent and usable
t['s'];            // PANIC: the stream is taken, seen through the table alias
taken(t['s']);     // true (a query, not a use): no panic, reports the move
```

The `&`-reference ban (§2.1) and the taken state are the two halves of one guarantee: a live
mutable value is never reachable from two tasks, `&` references cannot cross at all, and a
stream / builder that *does* cross leaves an **enforced-dead** handle behind, on the referent, so
every alias of it is dead too.

---

## 3. `promise`: a single future value

`spawn` returns a **`promise`**, a built-in type standing for **one** eventual value. A promise
resolves exactly once, to a value or to an error (§4). It is *not* a stream: a stream is a lazy
*sequence*, a promise is a *single* future value. A collection of tasks is a stream of promises or
results (§5); a single task's result is a promise.

**`await p`** waits for the promise to resolve and yields its result. `await` **does not block the
underlying thread**: a task waiting on `await` yields its OS thread back to the scheduler, so other
tasks run and awaiting cannot deadlock the scheduler (compiler spec §7.3). Awaiting is cheap.

`await` collapses the *time* dimension: `spawn f(x)` is conceptually a future `T!` (functions and
errors specs: the task may produce a value or fail), and `await` turns the `promise` into that
**`T!`**, the value, or an error. So promises ride entirely on the existing error model (§4); they
add no new error mechanism.

### 3.1 A promise is confined to its creating scope

A promise **cannot cross a spawn boundary in either direction**: it may not be passed into another
spawned task, captured into a spawned closure, or returned out of a task (§2.1, §2.2). It is
confined to the scope that created it. This single rule secures two properties at once:

- **The await graph is a DAG.** Because a promise never reaches a second task, no two tasks can
  await each other, so there are no await cycles, and therefore no deadlock from circular awaiting
  (§6, the guarantee). Awaits only ever flow from a scope to the tasks it spawned.
- **A promise never outlives its task's scope.** A promise cannot escape to a parent or sibling
  scope, so it can never refer to a task whose scope has already exited and cleaned up (§6). The
  promise and the task it names live and die together within one scope.

So a promise is a within-scope handle, not a value that circulates. Attempting to pass, capture,
or return a promise across a spawn boundary is a compile error.

The precise circulation rule (R117): a promise flows only through **bindings** and
**streams** — single-pass and scope-local, the §5 composition (`map(fn (x) => spawn f(x))`
yields a stream of promises, each created and consumed in flight). It may **not** enter
**retained storage**: a table element would let a copyable, boundary-crossing container
carry it, so storing a promise in a table is an error, and `collect` on a stream of
promises is likewise an error (it would retain them) — await the stream, then collect the
*results*. A compile error where statically evident, a panic otherwise, the house split.

---

## 4. Errors: fail-fast by default, collection by opt-in

A spawned task can fail (produce an error, errors spec). The default is **fail-fast**, matching
command semantics (command spec) and the language's error model:

- **`await p`** on a failed task yields an **error** (the promise resolved to an error `lval`), and
  the caller handles it exactly as any errorable value, with `try` (errors spec). So a single
  awaited task's failure is an ordinary `T!` the caller must handle.
- **When awaiting many tasks (§5), the first error fails the whole `await` and cancels the
  siblings.** Structured concurrency (§6) makes this natural: the failure fails the enclosing
  scope, and the scope cancels its still-running tasks. The error propagates at the `await` point,
  so **downstream stages never receive an error**, control has already left via the failure.
  Consuming code after the `await` therefore need not handle per-element errors; it only runs on
  the all-succeeded path.

**Error collection is per-promise, not per-stream** (R116, reconciling with await §1.1): a
stream has no error channel, so the stream-collecting `await` surfaces a failed task as a
**panic at the pull** — the fail-fast posture in stream form. When *every* result matters,
failures included, await the promises **individually**, where the declarable channel
exists: `try await p` yields the task's `T!` as a handleable value, and a loop of
individual awaits collects successes and failures without cancelling anything. The choice
between fail-fast and collection is made by *how you await* — the stream form is
fail-fast, the individual form collects — and the collection form is kept honest by the
ordinary errorable-value rules (`try`, or an errorable binding, per element). (An earlier
paragraph here promised an "await into a stream of `T!`" collection form; retired — it
contradicted the no-error-channel rule await §1.1 states, and the individual form already
covers the need.)

### 4.1 A task panic resolves its promise; `await` never hangs on a dead task

A task may also **panic** (a `panic`, not a declarable error: an `OverflowError`, a failed `as`, and so
on, errors spec §9). A panicking task does **not** die silently. Its panic **resolves its promise
as a failure**, and `await` **propagates** that panic to the awaiter, rather than leaving the
awaiter blocked forever on a promise that will never resolve. So:

- A task panic becomes the task's failure and travels the **same fail-fast path** as any other
  failure (§4): it propagates at the `await` point and cancels the siblings.
- `await` is therefore a site where a panic may surface, and the awaiter may catch it (panics are
  catchable and need no declaration, errors spec §9) or let it propagate. No `!` annotation is
  needed for this, exactly as elsewhere.

The load-bearing guarantee is that **a task's death always resolves its promise** (as a value on
success, as an error or panic on failure), so `await` on any task terminates. There is no path
where a task vanishes and its awaiter hangs. Combined with promise confinement (§3.1, no await
cycles), this is what makes awaiting deadlock-free: awaits form a DAG, and every node in the DAG
eventually resolves.

---

## 5. Many tasks are a stream

A single task is a `promise`; a **collection** of tasks is a **`stream`** (stream spec), and this
is where tasks compose and iterate. Streams are the reused mechanism for "many results over time,"
so spawning over a collection yields a stream of promises (or results), and all stream operations
apply.

- **Streams are not automatically parallel.** A stream pipeline runs **lazily and sequentially** by
  default (pull-based, one element at a time, deterministic); `|>` stays deterministic data flow
  (pipeline spec). Concurrency is **opt-in per stage** by spawning inside it:

  ```
  let results = await (xs |> map(fn (x) => spawn expensive(x)));
  // fan out: each element on its own task; await over the promise stream collects in order
  ```

  The map hook returns a **promise** per element, so the pipeline yields a stream of
  promises; `await` over that stream (a word-prefix operator, not a pipeline stage —
  await §1.1) pulls and collects them. Where `expensive` is effectful, the hook's
  requirement set rides it and the caller authorizes at the call site
  (`xs.map(fn (x) => spawn expensive(x)) use (io)`, capabilities §5.2, R112).

- **`await` over a stream is ordered by default.** The results come back in **input order** (the
  order of `xs`), which is deterministic and matches stream ordering, at the cost of waiting for
  the slowest task before a later-but-faster one is delivered. This is the less surprising default.

- **`awaitAsCompleted`** is the opt-in variant that yields results in **completion order** (fastest
  first), which is faster to drain but non-deterministic in order. Its longer name is deliberate:
  completion-order is the rarer, more footgun-prone choice, so it is spelled out loudly.

- **Aggregation is return-and-fold, never shared state.** Because tasks share no mutable state
  (§2), they cannot write into a shared accumulator. The shared-nothing way to aggregate is: each
  task **returns** its piece, and the awaiter **folds** the returned results, in one place,
  sequentially, race-free:

  ```
  let total = (await (xs |> map(fn (x) => spawn work(x)))).reduce(add);
  ```

  There is deliberately no shared-mutable-with-locking mechanism (no automatic locking): it would
  reintroduce the shared mutable state this model eliminates, give false safety (compound
  read-then-write operations would still race), and hide a large cost behind ordinary access.
  Aggregation is return-and-fold; genuine inter-task *communication* is the job of channels (§7),
  not shared memory.

---

## 6. Structured lifetime: tasks are scope-bounded

A spawned task's lifetime is **bounded by the scope that spawned it.** Tasks are not
fire-and-forget: a task cannot outlive its scope, orphaned and running after the code that started
it has returned.

- **Scope exit waits for or cancels its tasks.** When a scope exits, its outstanding tasks are
  resolved: awaited results are collected, and any task still running that is no longer needed (for
  example after a fail-fast cancellation, §4) is **cancelled**. So there are no stray background
  tasks and no ambiguity about program exit, the top scope waits for its tasks, so a program does
  not exit while structured work is unfinished.
- **Cancellation is guaranteed, and composes with `defer`.** A cancelled task unwinds and runs its
  **`defer`** cleanup (defer spec), so a task that opened a resource and `defer`-closed it releases
  it even when cancelled. This is why scope-bounding and `defer` fit together: the task's cleanup
  runs on cancellation exactly as it runs on normal completion or panic.
- **Cancellation and completion are atomic per task.** A task is either cancelled or allowed to
  complete, never both and never partway: the runtime makes the cancel-versus-complete decision
  atomic, so a task cannot be transferring its result (§2.2) *and* being cancelled at the same
  instant. Without this, a torn half-transferred result on a cancelled task would be possible; with
  it, each task has a single, clean terminal state.
- **A `defer` may not `await`.** Deferred cleanup must be prompt and non-blocking, so **`await`
  inside a `defer` is a compile error**. Cleanup that could block on another task would let
  scope-teardown hang (awaiting something that may itself be cancelled), so cleanup is restricted to
  synchronous, terminating work (close a handle, release a lock). This keeps teardown finite: a
  scope always finishes cancelling and cleaning up.
- **Concurrent cleanup is safe through safe APIs, not special treatment.** When a scope cancels
  several tasks, their `defer` cleanups may run concurrently. Cleanup is ordinary code, so it is not
  magically race-free; its safety comes from the **resources it touches being thread-safe**. The
  standard library's resource APIs (files and the like) are thread-safe, so `defer f.close()` is
  safe because `close` is safe, not because `defer` is. Concurrent cleanup that mutates shared
  external state is the same external-state concern as any concurrent access (§2.1, §7), handled the
  same way.
- **A dropped promise does not leak a task.** Because lifetime is scope-bounded rather than
  promise-bounded, forgetting to `await` a promise does not orphan a running task; the task is
  still bounded by (and cleaned up with) its scope.

Structured lifetime is what makes the model safe to reason about: every task has a well-defined
end, cleanup always runs (promptly, and non-blocking), and the program never exits with silent
unfinished work.

### 6.1 Cancellation semantics (R115)

Cancellation is **specified alpha semantics**, and it is **runtime-initiated only**: the two
cancellers are fail-fast (§4, the first error cancels the siblings) and scope exit (§6).
There is **no user-facing cancel primitive** — timeouts, `awaitAny`, and await-with-deadline
are future *surface* over these semantics and stay deferred (await §3, §4). The rules:

- **Cooperative, suspension-point-delivered, refused-on-entry.** A cancelled task keeps
  running until its next **suspension point** — an `await`, a blocking io call, a channel
  send or receive (channels §4, R119), a
  scheduling point — where the runtime delivers **`cancelled`**, a runtime-minted type in
  the `panic` subtree (errors §2, §9: user code can neither originate nor be forced to
  declare it). Delivery is **before the operation**: a pending cancellation *refuses* the
  suspension point's operation rather than performing it and cancelling afterward — the
  same before-the-effect principle as the dynamic capability check panicking before the
  callee's body begins (capabilities §5.1). A task already **parked** (on an `await`, on a
  blocked read) is interrupted and receives `cancelled` at the park — which is what makes a
  task blocked on a dead socket cancellable at all.
- **Observable, never stoppable.** `cancelled` is **catchable** like any panic (errors
  §8.2 — uniformity, no special case), but the task remains **cancel-pending**:
  `cancelled` is **re-delivered at the next suspension point**, and the pending flag is
  runtime state user code cannot clear. Catching is for observation and state adjustment
  on the way out; a task cannot un-cancel itself. This adds **no new attack surface**:
  catch-resistance (catch and spin, or a catch-loop around io — each attempt refused on
  entry) is exactly as strong as never reaching a suspension point at all, which is the
  already-conceded compute-hang carve-out (§7).
- **Defers run uncancelled, unconditionally.** Nothing is delivered inside a `defer` (§6):
  cleanup runs to completion, on cancellation exactly as on success or panic.
  Consequently **defers are the compensation context**: io-bearing remediation of a
  half-done external effect must live in a defer, because a *catch block's* io is a
  suspension point and gets re-delivered. The idiom: bracket a critical external sequence
  with a defer that completes-or-compensates, the body maintaining progress state
  (`var sent = false; …; sent = true;`) the defer consults — which is also how a defer
  learns *why* it is unwinding, with no new mechanism. A defer that blocks forever hangs
  its scope — accepted (§7's residual), because cancelling cleanup is worse semantics
  than trusting it.
- **A cancelled task's death is clean by construction.** Its partial work lived in its own
  copies, invisible to every other task (§2); its transferred handles are already dead
  behind it (taken, §2.3); its promise resolves to `cancelled`, so awaiters never hang
  (§4.1). The classic async-exceptions blast radius is a *shared-state* blast radius, and
  Luna deleted the shared state — what remains is the external world: an
  **uncompensatable external effect** (a message a third party has already received) is
  the irreducible residue, §7's existing carve-out, unchanged.

---

## 7. What is guaranteed

For **Luna values**, the model makes the two classic concurrency failures **structurally
impossible**, not merely discouraged:

- **No data races on Luna values.** No live mutable value is reachable from two tasks at once
  (§2.1): copyable values are copied, frozen `const` values are shared read-only (with no lazy
  initialization, so a read is never a hidden write), stateful values (streams, builders) transfer
  sole ownership (enforced by the taken state, §2.3), capabilities are immutable, promises are
  confined, and `&` references cannot cross. This rests on the **copy discipline, not on locking**:
  because every task's mutable data is sole-owned, copy-on-write refcounts are touched
  single-threaded and are **non-atomic** (§2, value-representation §6.1). The only cross-task
  sharing is **frozen `const`s**, which are immutable and carry **no refcount** (§2), so they are
  safe to read concurrently with no synchronization. The one runtime obligation is therefore that
  the **spawn and await boundaries act as synchronization points** (where the eager copy and the
  taken mark, §2.3, are established); the runtime interiors need no atomic refcounting or
  locks.
- **No lock or await deadlocks.** There are no locks (aggregation is return-and-fold, §5, not
  shared-memory
  locking), so no lock-ordering deadlock exists. Awaiting cannot deadlock either: promises are
  confined so the await graph is a **DAG** (§3.1, no await cycles); every task's death resolves its
  promise so `await` never hangs on a dead task (§4.1); `await` in a `defer` is forbidden so
  teardown cannot hang (§6); and `await` yields the scheduler thread so the scheduler cannot
  starve (§3). **Channel-wait cycles are the one deadlock shape that exists** (R119,
  channels §6): two tasks each parked sending to the other's full channel wait forever —
  classed with logic-bug hangs (below), contained by scope-bounding, and cancellable at the
  parks when the scope fails elsewhere, but not structurally prevented.
- **No orphaned tasks.** Structured, scope-bounded lifetime (§6) guarantees every task ends and its
  cleanup runs; the program never exits with silent unfinished work.

The honest **carve-outs**, things outside the value model that the language does not and cannot
structurally prevent:

- **Races on external state** reached through a shared capability (two tasks writing one file) are
  possible; they are the programmer's responsibility, mediated by the thread-safe resource APIs the
  standard library provides. The guarantee is race-freedom on *Luna values*, not on the external
  world a capability touches.
- **Logic-bug hangs** are not prevented: a task with a non-terminating loop, or unbounded recursive
  spawning, hangs or exhausts resources. These are ordinary bugs (an infinite loop is a bug whether
  or not it is in a task), and the language provides no per-task time or memory limit to forcibly
  bound them (§8). The same family includes a `defer` that blocks forever during teardown
  (§6.1): cleanup is never cancelled, so a misbehaving defer hangs its scope.
- **Cancellation is yield-point-bounded** (§6.1, R115). A cancelled task observes cancellation only
  where it suspends (an `await`, a blocking io call, a channel operation, a scheduling point),
  refused-on-entry. A task
  in a long uninterruptible computation cancels only when it next reaches such a point, so
  cancellation is prompt but not instantaneous — and a task that never suspends never cancels,
  which is the same carve-out as the logic-bug hang. **Unconditional kill** (BEAM's
  `Process.exit(:kill)`) does not exist and is **extremely unlikely** ever to (R120): the Go
  backend cannot kill a goroutine, and the model disfavors it regardless; only a backend
  change could reopen the question.

---

## 8. Resolved and open

**Resolved: no per-task time or memory limits.** The language provides no built-in way to bound a
task's running time or memory. A **time** limit is redundant: it is cancellation on a timer, and
cancellation already exists (§4, §6), so "time-limit a task" is a library pattern that races the
task against a timer and cancels the loser, not a language primitive. A **memory** limit is
impractical on the runtime: the Go garbage collector is global and does not partition the heap per
green thread, so a per-task memory ceiling would require a custom allocator, a large cost for a
rare need. (Comptime's separate memory ceiling, functions spec §5.5, is unrelated: comptime runs
in the compiler's own evaluator, which the compiler controls; runtime tasks allocate through Go's
GC, which it does not.) A task can therefore hang or over-allocate through a logic bug (§7); that is
an accepted cost, mitigated by writing yield points and correct termination, not by a forced limit.

Open:

- **Timeouts and `awaitAny` / select: deferred, and named the top post-alpha priority**
  (R120). The scrutiny against BEAM made the stakes precise: Elixir's ubiquitous call
  timeouts are its *universal hang-recovery mechanism*, and until this surface lands Luna
  cannot express "wait, but not forever" — so the one admitted deadlock shape,
  channel-wait cycles (§7, channels §6), has no in-language recovery. This is **safety,
  not convenience**. The surface rides R115's semantics (a deadline is a cancellation; a
  select is a race), and its timer half is `std.time`'s `sleep` (std/time.md, deferred
  with it) — the two should be designed together.
- *(**Channels: designed, R119** — `channels.md`. `let [tx, rx] = channel(capacity)`; the
  receive end is literally a `stream`; the `sink` is a shared write-only handle
  (*ownership follows readability*, §2.1); per-handle `finish`, no whole-channel close;
  MPSC, with select and MPMC deferred there. The owner-task pattern this unblocks is
  channels §5.)*
- **The result-collecting await surface: mostly settled.** Laziness is ruled (await §1.1:
  lazy, each pull awaits one promise) and the error interaction is ruled (R116: the stream
  form is fail-fast, panicking at the pull; collection is individual `try await`). What
  remains open is only `awaitAsCompleted`'s interaction with fail-fast cancellation.
- *(**Cancellation semantics: resolved by R115** — §6.1: cooperative,
  suspension-point-delivered, refused-on-entry, observable-never-stoppable, defers
  uncancelled. What remains is implementation only: how the unwinding runs `defer` in
  emitted Go, compiler spec §11.)*
- *(**Task-local capability scoping: resolved by R112/R118** — the task-root entry below;
  `spawn f() use (X)` is the scoping mechanism.)*
- **Backpressure and scheduling controls.** Whether spawning over a large collection bounds
  concurrency (a limit on simultaneously-running tasks) or spawns unboundedly, and how backpressure
  from a slow consumer propagates through an opt-in-parallel stream, pending the scheduler design.

---

## Deferred by decision, and one ruling (R42)

- **Capability scoping at spawn: the task root (R118, superseding this entry's R42-era
  "none").** A task's **root grant** is the spawned function's declared requirement set
  **plus any spawn-site delegation** — `spawn f() use (io)` extends `io` into the task
  (capabilities §5.2, R112) — checked against the spawner's own grant at the spawn
  (capabilities §3.1). Inside the task, frames follow the ordinary rules from that root:
  declared `use` statically, call-site delegation on the dynamic frontier. Still one rule,
  the call rule — now including the call rule's delegation clause.
- **Channels: deferred**, not necessary; tasks communicate by arguments in (deep-copied)
  and results out (moved at `await`), and the collecting surface is `await` over a stream
  (await §1.1).
- **Backpressure and scheduler tuning: deferred**; the Go scheduler underneath is the
  alpha's answer.
