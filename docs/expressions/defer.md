# Defer

`defer` schedules cleanup to run when the **enclosing block** is left, by any path: falling off
the end, `return`, `break`, `continue`, or a **panic** unwinding through it. It is Luna's
deterministic resource-cleanup mechanism: the way a file is closed, a lock released, or a handle
freed exactly once, at a known point, regardless of how control leaves the block. It exists
because garbage collection is non-deterministic (value-representation §6): the GC eventually
frees memory, but it does not promise *when*, so a resource that must be released promptly (a
file descriptor, a socket, a lock) needs an explicit, deterministic release. `defer` is that
release.

```
let f = openFile('data.log', {read});
defer f.close();                        // runs when this block exits, however it exits
foreach (line in f.lines()) {
  if (isDone(line)) { return; }         // f.close() still runs before the function returns
  process(line);
}
// f.close() runs here on normal completion too
```

---

## 1. `defer` is block-scoped

A deferred statement runs when the **nearest enclosing `{}` block** is left, not when the
enclosing function returns. This is a deliberate choice over the function-scoped `defer` of some
languages, and it matters most in loops:

```
foreach (path in paths) {
  let h = open(path);
  defer h.close();          // runs at the end of THIS iteration's block, each time
  use(h);
}                            // each h is closed as its iteration ends, not all at function exit
```

Block scoping means each `h` is released as its iteration completes, so a loop over a thousand
files holds one handle at a time, not a thousand until the function returns. It also makes
`defer` consistent with the language's **block scoping** of variables (variables spec §4): a
resource is cleaned up with the block that owns it, the same scope its binding lives in. A
`defer` written directly in a function body (its outermost block) runs at function return, which
is the function-scoped case as a natural consequence, not a separate rule.

---

## 2. Deferred code runs on every exit path

The point of `defer` is that cleanup is **unconditional** once registered: it runs no matter how
the block is left.

- **Normal completion**, control reaches the end of the block.
- **`return`**, the return value is computed first, then deferred code runs, then the function
  returns (§5).
- **`break` / `continue`**, leaving the block early still runs its defers before control
  transfers.
- **A panic unwinding** (errors §9), a `TypeError`, `OverflowError`, or any other panic
  propagating out of the block runs the block's defers as it unwinds. This is what makes `defer`
  a reliable cleanup even on the failure path: a file opened and `defer`-closed is closed even
  if the code between panics.

So once a `defer` has executed (registered its cleanup, §4), that cleanup **will** run when the
block exits. There is no path, success or failure, that skips it.

This is the division of labor with `try` (errors spec): **`try` handles error *values*; `defer`
handles *cleanup* on all paths.** They are orthogonal and compose in the common pattern of
"acquire, defer release, then do fallible work":

```
let conn = db.open();
defer conn.close();               // released whether the work below succeeds, fails, or panics
let rows = try conn.query(sql);   // handle the error value with try
// conn.close() runs on the way out regardless
```

---

## 3. Multiple defers run last-in, first-out

Several `defer`s in one block run in **reverse order of registration**: the most recently
deferred runs first. This is the correct order for nested resource acquisition, resources
acquired in order are released in the reverse order, so dependencies are torn down before the
things they depend on:

```
let outer = openOuter();
defer outer.close();          // runs second
let inner = outer.child();
defer inner.close();          // runs first
// on block exit: inner.close(), then outer.close()
```

LIFO is what keeps release symmetric with acquisition: the last thing you opened is the first
thing you close.

---

## 4. Registration happens when the `defer` is reached; operands are captured by value

A `defer` **registers** its cleanup at the moment control reaches the `defer` statement, not at
block entry. Two consequences follow:

- **A `defer` that is never reached never runs.** A `defer` inside a branch that is not taken, or
  after an early exit that skips it, registers nothing. Only defers actually executed are
  scheduled. So `defer` follows normal control flow up to the point of registration, and runs
  unconditionally only *after* that point.
- **Operands are captured by value at registration time**, the same value-capture rule as closure
  auto-capture (functions spec §2.1). The deferred call's target and arguments are snapshotted
  when the `defer` runs, not when the block exits, so later reassignment of a binding does not
  change what was deferred:

```
var h = openA();
defer h.close();      // captures the handle to A (the current value of h)
h = openB();          // reassigns h to B
defer h.close();      // captures the handle to B
// on exit (LIFO): closes B, then closes A, each the handle captured at its defer
```

This matches the language's by-value capture everywhere else: a `defer` is, in effect, a
zero-argument cleanup closure whose free values are captured by value when it is registered. To
defer cleanup of a value computed later, place the `defer` after that value exists.

A `defer` may take a **single call** (`defer f.close();`, `defer unlock(m);`) or a **block**
(`defer { ...; ... }`) for multi-step cleanup; the block form is the same construct with several
statements, and its free values are likewise captured by value at registration.

---

## 5. `defer` does not change the return value

Deferred code runs **after** the return value is computed, and it **cannot alter** that value.
`return expr;` evaluates `expr`, then runs the block's (and enclosing blocks') defers, then hands
back the already-computed value. A `defer` is for cleanup, not for post-processing a result, so
it has no access to and no effect on what the function returns. This deliberately avoids the
named-return-value mutation that some languages allow through defer, which is a common source of
confusion; in Luna a function's result is fixed at `return` and cleanup cannot rewrite it.

If a value genuinely needs post-processing, do it in ordinary code before `return`, not in a
`defer`.

---

## 6. Panics inside deferred code

Deferred cleanup should be **panic-free**: cleanup that can fail is a design smell, and a panic
raised inside a `defer` propagates like any other panic. If a `defer` panics while the block is
exiting normally, that panic begins unwinding from the `defer`. If a `defer` panics while a panic
is *already* unwinding, the cleanup's panic propagates and supersedes, so a cleanup path that can
panic can mask the original failure. For that reason, keep deferred cleanup simple and
non-throwing (close a handle, release a lock); if a cleanup step can fail meaningfully, handle its
error inside the `defer` (with `try`) rather than letting it panic out. The remaining defers in the
block still run: a panic in one deferred statement does not skip the others (each is unwound in
turn).

---

## 7. What `defer` is and is not

- **It is deterministic cleanup**, running at a known point (block exit), unlike GC finalization,
  which runs at an unknown later time. Resources needing prompt release use `defer`; memory alone
  needs nothing (the GC handles it).
- **It is structured**, it runs on the way *out* of an enclosing block, never jumping to an
  arbitrary location. Like the rest of the language's control flow, there is no `goto`
  (control-flow spec §3); `defer` only ever runs enclosing-block cleanup in LIFO order.
- **It is not `finally`-with-`catch`.** `defer` does not catch or handle errors; it runs cleanup
  regardless of whether an error occurred. Error *handling* is `try` and the error-value model
  (errors spec); `defer` is orthogonal cleanup that runs on both the success and failure paths.
- **It is not function-scoped** (§1), so it does not accumulate across a loop body; each block's
  defers run when that block exits.

---

## 8. Open questions

- **`defer` and early stream cleanup.** How a `defer` interacts with a stream pipeline that
  short-circuits (`take(10)` abandoning a file, stream spec §8): the intended pattern is
  `let f = open(...); defer f.close();` at the scope that owns the file, so the file closes on
  block exit whether or not the stream is fully consumed. Confirming this covers the abandoned
  pipeline case, and whether streams also expose their own scoped-cleanup convenience, is pending
  the stream resource model.
- **Deferring in the outermost program scope.** Whether a `defer` at top level (the program's
  entry block) runs at program exit, and how that interacts with abnormal termination, pending the
  program-entry and process-exit model.
- **Cleanup during green-thread cancellation.** How deferred cleanup runs when a green thread is
  cancelled or the program is torn down (whether cancellation unwinds and runs defers like a
  panic), pending the concurrency model.
- **A panicking defer during unwinding.** The precise semantics when a deferred statement panics
  while another panic is already propagating (§6), whether the second panic chains, supersedes,
  or is reported alongside the first, pending the final panic/unwinding model.
