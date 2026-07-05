# Concurrency

Luna runs concurrent work as **green threads**: lightweight tasks scheduled by the always-present
runtime (goroutines under the hood, compiler spec §7.3). Concurrency is built on three pieces, a
**`spawn`** that starts a task, a **`promise`** that stands for its future result, and an
**`await`** that collapses a promise to a value, and on one architectural decision that makes it
safe: **a task shares no mutable state.** Everything crossing a spawn boundary is copied, except
`const` (which is frozen, so safe to share by reference), so there are no data races by
construction, which is why there is no function coloring: any function may be spawned.

```
let p = spawn compute(x);       // start compute(x) as a green thread; p is a promise
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

`spawn f(args)` evaluates `args`, starts `f` on a green thread, and immediately returns a
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
- **Captures** of a spawned closure are deep-copied too (a closure's captured environment is the
  sneaky path by which state could be shared, so it is copied on the same rule as arguments,
  functions spec §2). A captured `const` is shared by reference; a captured `var` or `let` is
  copied (the task sees a snapshot, not the spawner's live binding).

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
  the value **moved-from** (§2.3), and any later access through any alias panics. This is the
  ownership-transfer escape for large data (move a stream into a task rather than copy a table).
  Because such values can never be `const`, they can never ride the const-share path into
  *multiple* tasks, so the "stateful value shared through a frozen container" race cannot arise.
- **Promises**, **confined** (§3): a promise cannot cross a spawn boundary in either direction.
- **`&` references**, **forbidden**. Passing `&t` into a task would give the task a mutable
  reference to the spawner's binding, shared mutable state, a race. So a `&` reference may not be
  passed as a spawn argument or captured into a spawned closure; it is a compile error. Mutation
  visible to the spawner is expressed by the task **returning** a value the spawner uses (§2.2),
  never by a shared reference.

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

### 2.3 Transfer is enforced: the moved-from state

A transferred value's move is **not** a convention the spawner must remember; it is **enforced at
runtime**. Because Luna does no static move or use-after-consume analysis (compiler §1.4.1), "the
spawner stopped using it" cannot be a compile-time guarantee, so it is a runtime one, on the same
footing as reading an already-exhausted stream.

**The moved-from state lives on the value, not the binding.** A stream or builder is a by-reference
value that can be **aliased**, most sharply, a stream held in a table is shared by reference, so one
stream is reachable through several bindings and table slots at once (stream §6.1, tables §4).
Marking a single binding would leave the other aliases live and dangling. So transfer marks the
**referent**, the stream's / builder's heap state, beside its cursor / buffer, which **every** alias
dereferences: the direct binding, a table element, a captured copy, all see it. This is the same
principle as constraint and protocol enforcement, which follow the **value**, not the binding
(constraints §9.4, tables §6.4); moved-from is that rule applied to ownership, and it is deliberately
**not** the `undefined` mechanism, which is a per-*binding* `lval` flag that cannot live in a table
(undefined §3, §7, value-representation §2.1).

- **Set synchronously at spawn, on the spawner's thread, before the task runs.** The mark is written
  once, at the spawn point, then only read afterward (write-once-then-read-only), so the spawner's
  panic-checks and the task's ownership never race and **no atomic is needed**, the same discipline
  that lets the eager copy skip atomic refcounting (§2).
- **Using a moved-from value panics** (a `Panic`, errors §9), immediately, on any access through any
  alias. This is **distinct from consumed**: a *consumed* stream yields **empty** on read (a normal
  end-state, no panic), a *moved-from* stream **panics**, because another task now owns it. The
  referent therefore carries a small terminal state, **active / consumed / moved-from**, not one
  reused bit.
- **`taken(x): bool`** is the non-panicking query. `const taken = fn (x: any): bool` reports whether
  `x` has been moved-from, reading the referent's state **without touching the value**. It returns
  `false` for any non-movable value (an `int` is never moved-from) and never panics, the total,
  *asking* counterpart to the panicking *use* (as `has` is to a hard key read, `is` to `as`). It is
  an ordinary **runtime** query, not comptime-foldable, since moved-from is set at spawn, at runtime.
  Because it reads the referent, `taken(s)` and `taken(t['s'])` **agree** after a move: both see the
  one moved stream.

**This dissolves the container question.** A copyable table that *contains* a stream needs no special
"move the whole table" rule: each element crosses by its **own** rule, the copyable parts are
deep-copied, the stream moves (its referent marked). So the task receives a copy of the copyable
parts plus the moved stream, and on the spawner's side the table survives, its copyable parts usable,
while the moved stream element panics on access:

```
var t = ['s' => makeStream(), 'n' => 5];
spawn f(t);        // copyable parts deep-copied; the stream moves (referent marked moved-from)
t['n'];            // 5: the copy is independent and usable
t['s'];            // PANIC: the stream is moved-from, seen through the table alias
taken(t['s']);     // true (a query, not a use): no panic, reports the move
```

The `&`-reference ban (§2.1) and the moved-from state are the two halves of one guarantee: a live
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

**Error collection is opt-in through the type system.** When you want *every* result including
failures (rather than fail-fast), await into a **stream of `T!`**: each element is a value-or-error,
and nothing is cancelled on the first failure. Because the elements are `T!`, the **type system
forces** the consumer to handle them: feeding a `T!` into a stage that expects a plain value is a
compile error (you are passing a maybe-error where a value is required), so collected errors cannot
silently flow downstream as if they were values. You either handle each `T!` (with `try`, or by
being an errorable stage) or the program does not compile. So the choice between fail-fast and
collection is made by *which await you use*, and the collection form is kept honest by the ordinary
errorable-value rules.

### 4.1 A task panic resolves its promise; `await` never hangs on a dead task

A task may also **panic** (a `Panic`, not a `UserError`: an `OverflowError`, a failed `as`, and so
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
  xs |> map(spawn expensiveThing) |> await        // fan out: each element on its own task,
                                                   // then await all results
  ```

  `map(spawn f)` produces a stream of **promises**; `await` over that stream waits for them and
  yields their results.

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
  let total = xs |> map(spawn work) |> await |> reduce(add);
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

---

## 7. What is guaranteed

For **Luna values**, the model makes the two classic concurrency failures **structurally
impossible**, not merely discouraged:

- **No data races on Luna values.** No live mutable value is reachable from two tasks at once
  (§2.1): copyable values are copied, frozen `const` values are shared read-only (with no lazy
  initialization, so a read is never a hidden write), stateful values (streams, builders) transfer
  sole ownership (enforced by the moved-from state, §2.3), capabilities are immutable, promises are
  confined, and `&` references cannot cross. This rests on one runtime obligation: **the copy-on-write
  machinery and any structure reachable from multiple tasks must be thread-safe** (atomic refcounting
  and splits), which the runtime must provide.
- **No deadlocks.** There are no locks (aggregation is return-and-fold, §5, not shared-memory
  locking), so no lock-ordering deadlock exists. Awaiting cannot deadlock either: promises are
  confined so the await graph is a **DAG** (§3.1, no await cycles); every task's death resolves its
  promise so `await` never hangs on a dead task (§4.1); `await` in a `defer` is forbidden so
  teardown cannot hang (§6); and `await` yields the scheduler thread so the scheduler cannot
  starve (§3).
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
  bound them (§8).
- **Cancellation is yield-point-bounded.** A cancelled task observes cancellation only where it
  yields (an `await`, a `yield`, a scheduling point). A task in a long uninterruptible computation
  cancels only when it next reaches such a point, so cancellation is prompt but not instantaneous.

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

- **Channels.** A dedicated channel type is deferred. The **receive** end of a channel is
  stream-shaped (a sequence of values arriving over time, exactly a stream), so streams already
  cover consumption; a **send** end would be a new *sink* type. Moreover, a green thread that
  `yield`s is already a stream producer (stream spec §1), so a concurrently-generated stream may
  subsume most producer/consumer channel needs, leaving true channels for many-to-many or fan-in.
  The full design is pending.
- **The result-collecting await surface.** The exact spelling and laziness of `await` over a stream
  (eager "wait for all, yield a list" versus lazy "yield each as it is awaited"), and how it and
  `awaitAsCompleted` interact with fail-fast cancellation (§4), pending the stream-concurrency
  design.
- **Cancellation semantics in detail.** How cancellation interrupts a task (at what points a task
  observes cancellation, and whether long-running pure computation is interruptible or only
  cancels at await/yield points), and how cancellation unwinding runs `defer` in emitted Go
  (compiler spec §11), pending the runtime model.
- **Task-local resources and capability scoping.** Whether a task may hold capabilities beyond
  those shared from its spawner, and how capability lifetime interacts with scope-bounded task
  lifetime, pending alignment with the capability model.
- **Backpressure and scheduling controls.** Whether spawning over a large collection bounds
  concurrency (a limit on simultaneously-running tasks) or spawns unboundedly, and how backpressure
  from a slow consumer propagates through an opt-in-parallel stream, pending the scheduler design.
