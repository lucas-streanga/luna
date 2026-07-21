# `await`

`await` completes the `spawn` story: `spawn` starts a task and hands back its `promise`;
`await` collects it.

```
let p = spawn compute(data);     // data deep-copied in (isolation, concurrency §2)
let result = await p;            // park until done; result MOVED out
```

## 1. Semantics

- **`await p` parks the current task** until `p`'s task completes, then yields the
  task's result. Parking is a scheduler operation, not an OS block: the carrier thread runs
  other tasks meanwhile (the Go runtime underneath).
- **No function coloring.** Any function may `await`; there is no `async` marker, no split
  between sync and async call worlds, and no viral signature rewriting. Awaiting is just a
  call that may park, exactly like a blocking read on `stdin` (std.io). This is the green-
  thread dividend and it is deliberate.
- **The result is MOVED out, never copied.** This is the symmetric half of spawn's isolation
  rule: arguments are **deep-copied in** because, at spawn, two live owners would otherwise
  share state; the result **moves out** because, at await, the task has finished, its frame
  is dead, no alias to the result can exist, so transfer is free and safe. One copy at
  entry, zero at exit.
- **A promise is consumed by `await`.** Collecting the value moves it, so the promise joins
  the taken-value discipline that streams, builders, and files already follow (concurrency
  §2.3): a second `await p`, or any use of `p` after the first, **panics** (the value is taken). One
  task, one result, one collection.
- **Errors surface at the `await`.** If the task's function is errorable and it threw, the
  error is delivered at the collection point: `await p` on a task running `fn (...): T!` has
  type `T!` and sits behind `try` like any errorable call (errors §8). If the task
  **panicked**, the panic propagates from the `await` (the fail-fast stance, concurrency
  §4): the collector inherits the failure it was waiting on.
- **Grammar**: `await` is a word-prefix operator, tier 12 (associativity §1), binding loose
  like its siblings: `await p ?? fallback` is `await (p ?? fallback)`; write
  `(await p) ?? fallback` for the other reading, or lean on `try` when the arm is an error.

### 1.1 Collecting many: `await` over a stream

The result-collecting surface is not a combinator zoo; it is **`await` applied to a
stream of promises**, yielding a **lazy stream of results**:

```
let results = await promises;        // promises: a stream of promise values
foreach (r in results) { ... }       // each pull awaits the next task
let all = [...await promises];       // force everything: spread materializes (spread §2)
```

Each pull awaits one promise (taking it, §1) and yields its value; laziness composes with
pipelines and `take`, and eager collection is one spread away, streams can always be
forced to a table. One consequence of the stream setting, stated: a stream has no error
channel (std.io §8), so a task that **failed** surfaces as a **panic at the pull**; when
per-task error handling matters, await the promises **individually** (`try await p`),
where the declarable channel exists. Timeouts and racing **landed** (R142): `awaitAny`
and the timeout family, concurrency §5.1.

## 2. Never awaited

A promise that is never awaited is legal: the task runs to completion regardless (spawn is
not lazy), its result is dropped when the promise is collected by the GC, and a thrown
*declarable* error in a never-awaited task is **elevated to a panic** at task exit, an error
nobody can ever handle must not vanish silently (the same no-silent-drop instinct as
`_ =`, errors §8.1). Fire-and-forget is therefore spelled honestly: spawn a function whose
type is not errorable, or `_ = spawn f()` acknowledges the promise while the elevation rule
still guards the error.

## 3. Cancellation: specified and runtime-initiated; the user surface deferred

Cancellation is **specified alpha semantics** (concurrency §6.1, R115): cooperative and
**suspension-point-delivered, refused-on-entry** — a cancelled task keeps running until its
next suspension point (`await`, a blocking io call, a scheduling point), where the runtime
delivers **`cancelled`**, a runtime-minted type in the `panic` subtree (errors §2, §9:
user code can neither originate nor be forced to declare it), *instead of* performing the
operation; catchable but re-delivered (observe, never stop); unwinding through `defer`s,
which run **uncancelled** so cleanup and compensation complete (std.io §4's
`defer close(fd)` works unchanged). Preemptive cancellation — killing a task at an
arbitrary instruction, the async-exceptions trap Java deprecated with `Thread.stop` — does
not exist and is **extremely unlikely** ever to (R120: foreclosed by the Go backend, which
cannot kill a goroutine, and disfavored by the model regardless; only a backend change
could reopen it); the suspension-point design slots into machinery that already
exists (panic origination rules, defer unwinding, the io-errors §4 note on `EINTR`).

The user-facing surface **landed as the timeout family** (R142, concurrency §5.1) —
and there is **still no `cancel(p)` primitive**, deliberately: `timeout` owns a scope
and lets scope exit cancel the loser, so the only cancellers remain the runtime's own —
fail-fast sibling-cancel and scope exit (concurrency §4, §6) — with the race adding no
third kind, only a new reason for the second to fire. (The earlier form of this section
deferred cancellation entirely, which contradicted concurrency §4/§6/§7's reliance on
it; R115 specified the semantics and narrowed the deferral to the surface; R142 closed
the surface.)

## 4. Open questions

- *(**Timeouts and racing: landed, R142** — `awaitAny` (the primitive), `timeout`,
  `awaitTimeout`, `receiveTimeout`; concurrency §5.1. `awaitAll` needs no combinator:
  awaiting many is §1's stream form.)*
- *(**Promise as a value: resolved by R117** — confinement wins, concurrency §3.1: a
  promise is a scope-local handle that flows only through bindings and **streams**
  (single-pass, scope-local — the §5 composition, load-bearing for §1.1); it may not
  enter **retained storage** (a table element), be captured into a spawned closure,
  passed into or returned from a task — a compile error where statically evident, a
  panic otherwise. `collect` on a stream of promises would retain them and is likewise
  an error; await the stream, then collect the *results*. The "it moves" reading this
  bullet once floated is retired: confinement is what the await-DAG deadlock proof
  cites.)*
