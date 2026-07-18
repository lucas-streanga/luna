# Stream API

The **stream-only** operation surface. A stream is an `iterable`, so the shared catalogue —
transform, search, segment, aggregate, and the two bridges — lives in
**iterable-functions.md** (R92) and applies to streams directly: transformers are lazy (they
return a new stream that runs as it is consumed), full consumers exhaust the stream, and the
whole-input family (`sort`, `groupBy`, …) deliberately takes a `table` instead — a stream
spells the retention explicitly, `s.collect().sort()` (indexable-functions §4). This
document holds only what is meaningful for streams alone: producing, one-element
inspection, `foreach` consumption, and restarting. Semantics are in the **stream** spec.

---

## 1. Producing

A stream is produced by a **generator function**, a function whose body uses `yield` (stream
spec §1). Calling it returns a lazy, not-yet-started stream. Bare `yield v` produces
implicit keys `0, 1, 2, …`; `yield k => v` produces explicit keys (stream §1.1, R93).

```
const countTo = fn (n: int): stream {
  var i = 0;
  while (i < n) { yield i; i = i + 1; }
};

let s = countTo(1000000);        // lazy: nothing has run yet
```

Retained data enters the stream world through `toStream` (iterable-functions §2.11), O(1)
and lazy. The former per-operation `asStream: bool` flags are retired (R92, R94): output
kind follows the primary operand, and the one explicit bridge is the spelling for the lazy
direction.

---

## 2. Inspecting

These query a stream with at most **one element of lookahead**, never full consumption
(stream spec §3).

```
fn peek(s: stream): any            // the next value, buffered (not consumed); undefined if empty
fn isConsumed(s: stream): bool     // whether the stream has been exhausted (runs nothing)
```

- **`peek`** runs the generator just far enough to produce the first element, buffers it,
  and returns its **value**; the buffered element is still delivered by the next
  consumption. The corresponding key is `keyFirst` (iterable-functions §2.2) — the old
  key-aware-peek question is closed by R93: values through `peek` / `first`, keys through
  `keyFirst` / `keyLast`, pairs through `foreach (k => v)`.
- **`isConsumed`** reports exhaustion. It does not start or advance the stream; it reads
  state.
- **`isEmpty`** is no longer stream-only: it is total over `iterable`
  (iterable-functions §2.1). On a stream it peeks one element — it **starts** the stream (a
  bounded side effect) but **consumes nothing**.

---

## 3. Consuming

Consumption is single-pass and exhausts the stream (stream spec §2).

```
foreach (v in s)      { ... }     // values
foreach (k => v in s) { ... }     // key => value (implicit or explicit keys)
```

`foreach` is the primary consumer. Every other consumer is a catalogue function marked
**consumes** in iterable-functions (`count`, `first`, `last`, `reduce`, the aggregates,
`join`, `random`, `collect`, …). Full consumption never terminates on an infinite stream —
bound it first (`s |> take(n)`); the guard question stays open (§6).

---

## 4. Restarting

```
fn restart(s: stream): stream      // re-run the generator from the start; source-dependent
fn canRestart(s: stream): bool     // whether this stream's source supports restart
```

`restart` re-runs the generator from the beginning, valid only when the source can be
replayed (stream spec §4). On a non-replayable source it raises (or is reported unavailable
by `canRestart`). The general way to traverse again is to re-create the stream from its
source. `canRestart` is predictable by one rule (R105): a stream over an **immutable
retained snapshot** restarts for free — string producers, ranges, `toStream` over tables
and `bytes`; generators are source-dependent.

---

## 5. The shared surface, at a glance

Everything below lives in **iterable-functions.md**; this table is orientation only.

| Group | Functions | On a stream |
|-|-|-|
| Transform / reshape | `map`, `filter`, `mapLeaves`, `each`, `keyCase`, `values`, `keys`, `column`, `flip`, `chunk`, `flatten` | lazy stages |
| Segment | `take`, `skip`, `takeWhile`, `dropWhile` | lazy; `take` short-circuits upstream |
| Combine | `merge` (subsumes the retired `concat`), `diff`, `intersect`, `distinct`, `unique`, `replace`, `prepend`, `append`, `remove`, `combine` | lazy |
| Search | `find`, `keyOf`, `exists`, `some`, `every` | short-circuit |
| Consume | `count`, `first`, `last`, `keyFirst`, `keyLast`, `reduce`, `sum`, `average`, `product`, `min`, `max`, `mode`, `join`, `random` | consumes, partly or fully |
| Bridge | `toStream` (identity on a stream) · `collect` (the one stream→retained bridge) | iterable-functions §2.11 |
| Whole-input | `sort`, `reverse`, `shuffle`, `groupBy`, `partition` | **table-only**: `s.collect().sort()` (indexable-functions §4) |

Retired spellings (`values` as a collector, `concat`, `enumerate`, `asStream`, `empty`) are
tabulated in iterable-functions §3; do not reintroduce.

---

## 6. Open questions

- **Infinite streams and full consumers:** consuming an unbounded stream (`naturals`) never
  terminates; whether the API should mark or guard against it, or leave it to the
  programmer (as `take` before `collect`), stays open. The whole-input family is already
  fenced by the retention rule — it cannot be handed a stream at all.
- **Resource cleanup on abandonment:** how `restart`, short-circuit (`take`), and
  abandonment release an underlying resource (close the file), pending the resource/`io`
  model (stream §8 records the `defer` baseline).
- *(**Parallel transformers: resolved** — concurrency §5, R118: stream pipelines are lazy
  and sequential by default, and concurrency is **opt-in per stage** by spawning inside a
  hook — `xs.map(fn (x) => spawn work(x)) use (io)` — then awaiting the promise stream,
  await §1.1. No transformer is implicitly parallel.)*
