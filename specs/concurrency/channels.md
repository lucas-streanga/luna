# Channels

A **channel** is the task-communication primitive (R119): a conduit with two ends, created
together, through which values flow from tasks to a task. It completes the concurrency
story — `spawn` starts work, `promise`/`await` collects single results, and channels carry
**many values over time between running tasks**, which is what makes the owner-task pattern
(protocols §2.1's answer to shared mutable state) buildable. Channel operations need **no
capability**: like `spawn` and `await`, a channel is internal communication, not a reach
outside the program; its send and receive points join spawn and await as the
**synchronization points** of the model (concurrency §7).

```
let [tx, rx] = channel();       // a sink (send end) and a stream (receive end)
spawn worker(rx);               // the stream transfers into the consumer task
tx.send(job);                   // values flow in; copies of tx can flow to many producers
```

---

## 1. Creation

```
fn channel(capacity: int = 0): list      // [sink, stream]
```

`channel(capacity)` returns a two-element list — the **sink** (send end) and the
**stream** (receive end) — idiomatically destructured: `let [tx, rx] = channel();`.

- **`capacity: 0`** (the default) is a **rendezvous** channel: a send parks until a
  receive meets it, and vice versa. Every handoff is a synchronization.
- **`capacity: n`** buffers up to `n` in-flight values: sends park only when the buffer
  is full (backpressure), receives park only when it is empty.

Guidance (R120): rendezvous couples producer and consumer maximally — the strongest
backpressure, and the easiest shape to tie into a wait-cycle (§6). A small `capacity` is
the **decoupling knob**: prefer it wherever two tasks send to each other.

Both ends are ordinary values with extraordinary crossing rules (§2, §3).

---

## 2. The receive end is a stream

The receive end is **literally a `stream`** — not stream-like, the type itself — so the
entire catalogue applies with no new surface: `foreach (msg in rx)` consumes, parking on
empty (the scheduler runs other tasks meanwhile); `rx.filter(p).take(n)` composes;
single-pass, single-owner, transfer-with-taken across spawn boundaries — every existing
stream rule, unchanged. Two properties follow from rulings already made:

- **`canRestart(rx)` is `false`**, by R105's own rule: the source is not an immutable
  snapshot — received values are gone.
- **Exhaustion is the closed signal.** When every sink is finished (§5), the stream ends;
  receive-from-closed is not an error, it is the ordinary end of a stream, which every
  consumer loop already handles.

---

## 3. The send end: `sink`, a shared write-only handle

`sink` is a new built-in type (predeclared like `stream`), and it is the one deliberate
exception to single-ownership in the stateful class — governed by the principle the
exception makes precise:

> **Ownership follows readability.** A stream is taken because reading is stateful
> consumption — two readers would race over a cursor. A sink has **no readable surface**:
> no peek, no count, no query — sending is fire-and-forget ("once sent, it's gone") — so
> there is nothing to take, and sharing it shares no readable state.

The no-readable-surface fact also fixes equality: a sink compares by **identity** ("the
same channel's send end"), never by contents — contents are not a coherent question for a
write end (equality §2, §6, R181).

- **Sinks are shared, everywhere**: a copy, a table element in a copied table, a spawn
  crossing — all yield handles to the **same** channel (the crossing-taxonomy row is
  capabilities', for capabilities' reason: what is shared is not observable mutable data,
  concurrency §2.1). This is **fan-in**, the feature: many producer tasks, each holding a
  copy of `tx`, feed one consumer.
- **The channel interior is runtime machinery**, not a Luna value — it joins the scheduler
  on the far side of the value model, and its internal synchronization is the runtime's
  business. The residual ways sink-sharers can observe each other are all
  **synchronization-class, never data-class**: message interleaving (inherent to fan-in),
  backpressure timing (scheduler-visible only), and finished-ness (§5). §7's "no data
  races on Luna values" survives verbatim.
- **MPSC now; MPMC deferred** (§7): many senders, one receive stream. Fan-out is
  `spawn`/`await`'s job, or one channel per consumer.

---

## 4. Sending

```
fn send(tx: sink, v: any): undefined
```

- **The sent value crosses by the spawn taxonomy** (concurrency §2.1), unchanged:
  copyables are **eagerly deep-copied** in (the non-atomic-refcount argument, concurrency
  §2, forces eagerness here exactly as at spawn), `const` shares, streams and builders
  **transfer** (taken behind them), promises are **forbidden** (confined, R117). "Gone,
  poof" is the semantic content: an in-flight value is owned by the runtime alone — no
  peeking, no retraction, no shared view.
- **A parked send is a suspension point** (R115): a send on a full buffer (or an unmet
  rendezvous) parks the task, and cancellation delivers there, refused-on-entry, like any
  suspension point. Likewise a parked receive.
- **Send after the handle is finished, or to a departed receiver, panics** —
  `closedChannelError` (errors §2): a coordination bug, loud. The departed-receiver case
  is rare under structured lifetime: a consumer that exits its scope triggers
  cancellation of sibling producers, which are parked on their sends (a suspension point)
  and receive `cancelled` there rather than panicking; the panic remains for a send that
  races the shutdown window.

---

## 5. Finishing, liveness, and the owner-task pattern

There is **no whole-channel `close`**. Completion is **per-handle**:

```
fn finish(tx: sink): undefined
```

`finish(tx)` relinquishes **this handle**: further sends through it panic
(`closedChannelError`), other handles are untouched. The receive stream **ends when every
sink is finished or dropped** — scope exit finishes a sink automatically (structured
lifetime, concurrency §6), so the common case needs no explicit call, and the runtime
counts handles internally (atomics inside runtime machinery, outside the value model). A
second `finish` on the same handle panics (misuse, loud).

Per-handle finishing is why multi-producer completion needs **no coordination**: nobody
can close the channel out from under a sibling; each producer finishes its own handle and
the stream ends exactly when the last one does. (The name is `finish`, not `close`,
because `close` is taken with a different shape — `fn close(fd: file) use (io)`, std.io —
whose `use (io)` requirement a union signature cannot carry per-arm, a sink's finishing
needing no capability; one name, one signature, functions §3.4. The io spelling's former
`&fd` drift was resolved by R121: no `&`, the stream convention.)

**The owner-task pattern** — the referent of protocols §2.1's "a task that owns it," and
R96's mutable-static replacement, now expressible:

```
let [tx, rx] = channel();
spawn fn () => {
  var count = 0;                              // the "mutable static", owned, race-free
  foreach (req in rx) {
    count += 1;
    req.reply.send(count);                    // reply sinks travel inside requests
  }
}();

// a client (holding a copy of tx):
let [rtx, rrx] = channel();
tx.send(['reply' => rtx]);
let n = rrx.first();                          // parks until the owner replies
```

Every access to `count` is a message; every message is a visible synchronization; the
counter can never race because only its owner touches it. This is `sync` with the
synchronization made explicit — which is why a `sync` variable mechanism was **considered
again and rejected again** (R119): concurrency §5 already names the disease (reintroduced
shared state, false safety on compound operations, hidden cost), and the owner task is
the cure with the cost visible.

One leak shape to know: a sink sent through **its own** channel (a self-referential
request) keeps the handle count above zero forever — the stream never ends, the owner
loop never exits, the scope hangs. Classed with logic-bug hangs (concurrency §7); the
same shape exists in every channel system.

---

## 6. Guarantees and the honest cost

- **No data races, unchanged**: sent values cross by the taxonomy, sinks share nothing
  readable, receive streams keep single ownership. The guarantee list of concurrency §7
  survives with channels added to its synchronization points.
- **The honest cost — one deadlock shape now exists** (concurrency §7, amended):
  **channel-wait cycles**. Two tasks each parked sending to the other's full channel wait
  forever. No lock deadlock (still no locks), no await deadlock (promises still
  confined) — but a wait-cycle through channels is the classic shape every channel system
  admits, and Luna admits it too, classed with logic-bug hangs: contained by
  scope-bounding, attributable, and **cancellable at the parks** when the scope fails
  elsewhere (a parked send is a suspension point, §4) — but not structurally prevented.
  **In-language recovery exists** (R142): `receiveTimeout(rx, d)` bounds any wait —
  "wait, but not forever" — so the admitted shape is now also *escapable*, not merely
  contained (concurrency §5.1).

---

## 7. Resolved and deferred — nothing open (R153)

The channel design itself is complete; every remaining item is a resolution record or
a deferral with a fixed direction:

- *(**Select / racing over channels: mostly dissolved, remainder deferred — R142**
  (concurrency §5.1): fan-in is one MPSC channel with N senders, so "whichever is ready"
  is the merged channel's next element; residual heterogeneous races get a merge task or
  `awaitAny` over wrappers; a dedicated `select` waits for a case that survives the
  merge idiom.)*
- **MPMC / work-stealing: deferred by decision.** Multiple consumers on one channel
  (each element to exactly one) — one consumer per stream is the model's grain
  (ownership follows readability, §2.1), and fan-out already has `spawn`/`await`.
  Revisited only if a real workload outgrows the owner-task pattern.
- **Channel-of-channels patterns: deferred to practice.** Nothing forbids them
  (§5's reply sinks already work); idioms are documented as they prove out, never
  pre-specified.
- **The stdlib patterns layer (R120): deferred as library work, fully unblocked.**
  `call(tx, req)` — mint a reply channel, send, receive with a deadline — the
  `GenServer.call` shape (R142: `receiveTimeout` is the deadline); the supervisor loop;
  the registry task (name → sink). Library, not language; pending write-up only.
