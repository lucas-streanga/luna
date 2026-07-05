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
- **Grammar**: `await` is a word-prefix operator, tier 11 (associativity §1), binding loose
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
where the declarable channel exists. Timeouts and racing remain deferred with
cancellation (§3, §4).

## 2. Never awaited

A promise that is never awaited is legal: the task runs to completion regardless (spawn is
not lazy), its result is dropped when the promise is collected by the GC, and a thrown
*declarable* error in a never-awaited task is **elevated to a panic** at task exit, an error
nobody can ever handle must not vanish silently (the same no-silent-drop instinct as
`_ =`, errors §8.1). Fire-and-forget is therefore spelled honestly: spawn a function whose
type is not errorable, or `_ = spawn f()` acknowledges the promise while the elevation rule
still guards the error.

## 3. Cancellation: deferred, with the shape on record

There is **no cancellation in the alpha**, and the omission is a decision, explicitly:
the Go runtime underneath helps with the *mechanics* (contexts, goroutine parking), but
the *semantics*, what a half-cancelled task's effects and cleanup obligations are, stay
complex regardless of backend, and that is the part that must be designed, not inherited.
Preemptive
cancellation, killing a task at an arbitrary instruction, is the async-exceptions trap: a
task stopped mid-external-effect (a half-written file, a half-sent message) with no unwind
point, the mechanism Java deprecated (`Thread.stop`) and every runtime since has spent its
complexity budget taming. When cancellation arrives it will be **cooperative and
suspension-point-delivered**: a cancelled task keeps running until its next suspension point
(`await`, a blocking io call), where the runtime delivers **`cancelled`**, a runtime-minted
type in the `panic` subtree (errors §9: user code can neither originate nor be forced to
declare it), unwinding through `defer`s so cleanup runs (std.io §4's `defer close(&fd)`
works unchanged). That design slots into machinery that already exists, panic origination
rules, defer unwinding, the io-errors §4 note on `EINTR`, and is recorded here so the alpha
does not accidentally foreclose it.

## 4. Open questions

- **Timeouts and racing** (`awaitAny`, `awaitAll`, await-with-deadline): combinators over
  promises, wanted eventually, deferred with cancellation since a deadline is a
  cancellation.
- **Promise as a value**: whether a promise may be stored in a table or cross a further
  spawn boundary before collection (currently: it is a transferred, single-owner value like
  the rest of its class, concurrency §2.1, which answers both questions as "it moves").
