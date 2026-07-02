# Stream API

The operation surface of `stream` (semantics in the **stream** spec). Operations fall into
four groups: **inspecting** (peek at or query a stream without consuming it), **transforming**
(lazy stream-to-stream stages), **collecting** (consume a stream into retained data), and
**bridging** (between streams and tables). Transformers are lazy and single-pass; collectors
consume.

Throughout, a stream is either **values-only** (yields values, the `list` analogue) or
**key-value** (yields `key => value` pairs, the `table` analogue), stream spec §1.1.

---

## 1. Producing

A stream is produced by a **generator function**, a function whose body uses `yield` (stream
spec §1). Calling it returns a lazy, not-yet-started stream.

```
const countTo = fn (n: int): stream {
  var i = 0;
  while (i < n) { yield i; i = i + 1; }
};

let s = countTo(1000000);        // lazy: nothing has run yet
```

Producing operations elsewhere return streams on request via an `asStream` option (for example
`values(tab, asStream: true)`, `split(str, sep, asStream: true)`, string and table APIs).

---

## 2. Inspecting

These query a stream with at most **one element of lookahead**, never full consumption (stream
spec §3).

```
fn peek(s: stream): any | undefined       // the next element, buffered (not consumed); undefined if empty
fn isEmpty(s: stream): bool                // whether the stream has no elements (peeks one)
fn isConsumed(s: stream): bool             // whether the stream has been exhausted (runs nothing)
```

- **`peek`** runs the generator just far enough to produce the first element, buffers it, and
  returns it. The buffered element is still delivered by the next consumption. On a key-value
  stream, `peek` returns the next value (a key-aware peek form is an open question). Returns
  `undefined` if the stream is empty.
- **`isEmpty`** peeks and reports presence. It **starts** the stream (a bounded side effect,
  e.g. opening the file and reading one line) but **consumes nothing**.
- **`isConsumed`** reports exhaustion. It does not start or advance the stream; it reads state.

---

## 3. Consuming

Consumption is single-pass and exhausts the stream (stream spec §2).

```
foreach (s as v)      { ... }     // values-only
foreach (s as k => v) { ... }     // key-value
```

`foreach` is the primary consumer. In addition:

```
fn count(s: stream): int          // number of elements; CONSUMES the stream (exhausts it)
fn first(s: stream): any | undefined   // the first element, consuming only it; undefined if empty
```

- **`count`** must run the stream to the end to count, so it **exhausts** it. To count without
  losing the data, collect first (§4) and count the result.
- **`first`** consumes and returns just the first element (unlike `peek`, which buffers and does
  not consume). The rest of the stream remains for further consumption.

---

## 4. Transforming (lazy stream to stream)

Transformers return a **new stream** and run nothing until the result is consumed. They are the
stages of a `|>` pipeline (stream spec §7).

```
fn map(s: stream, f: fn): stream           // yield f(v) for each element
fn filter(s: stream, pred: fn): stream     // yield only elements where pred(v) holds
fn take(s: stream, n: int): stream         // yield at most the first n, then stop (short-circuits upstream)
fn skip(s: stream, n: int): stream         // discard the first n, yield the rest
fn takeWhile(s: stream, pred: fn): stream  // yield until pred(v) is first false, then stop
fn dropWhile(s: stream, pred: fn): stream  // discard until pred(v) is first false, yield the rest
fn concat(a: stream, b: stream): stream    // yield all of a, then all of b
fn enumerate(s: stream): stream            // values-only to key-value: yield index => value
```

Because transformers are lazy and pull-driven, `take` and `takeWhile` **short-circuit**: they
stop pulling upstream once satisfied, so upstream stages (and the source) do only the work the
downstream demand requires. `s |> map(expensive) |> take(3)` runs `expensive` three times, not
once per source element.

Transformers are equivalently written as a `|>` pipeline or as chained calls:

```
f.lines() |> filter(isError) |> map(parse) |> take(10)
take(map(filter(f.lines(), isError), parse), 10)      // same pipeline, call form
```

---

## 5. Collecting (consume into retained data)

Collectors **consume** the stream and return retained data, paying the memory cost so the
result can be randomly accessed and re-iterated (stream spec §5.1).

```
fn values(s: stream): list                 // consume a values-only stream into a list
fn collect(s: stream): table               // consume a key-value stream into a table
fn reduce(s: stream, f: fn, seed: any): any // fold the stream into a single value
fn join(s: stream, sep: string = ""): string // consume into a joined string (elements stringified)
```

- **`values`** collects a values-only stream into a `list` (reindexed from 0). **`collect`**
  collects a key-value stream into a `table` (keyed). These are the stream-to-retained bridges
  (stream spec §5.1); the values-only/key-value distinction determines list versus table.
- **`reduce`** folds without retaining the whole stream (constant memory), for sums, maxima,
  and the like.
- **`join`** is the common "stream of strings to one string" collector; it consumes.

---

## 6. Restarting

```
fn restart(s: stream): stream              // re-run the generator from the start; source-dependent
fn canRestart(s: stream): bool             // whether this stream's source supports restart
```

`restart` re-runs the generator from the beginning, valid only when the source can be replayed
(stream spec §4). On a non-replayable source it raises (or is reported unavailable by
`canRestart`). The general way to traverse again is to re-create the stream from its source.

---

## 7. Bridging from tables

```
fn asStream(tab: table): stream            // iterate an in-memory table/list lazily as a stream
```

`asStream` adapts retained data into the stream interface (stream spec §5.1). It does **not**
reclaim the table's memory (already spent); it exists for **pipeline uniformity** (feed
in-memory data into stream-consuming code) and **downstream laziness** (defer and short-circuit
per-element work). A `list` streams values-only; a keyed `table` streams key-value.

---

## 8. Summary

| Group | Operations | Consumes? |
|-|-|-|
| Inspect | `peek`, `isEmpty`, `isConsumed` | No (peek buffers one) |
| Consume | `foreach`, `count`, `first` | Yes (`count` fully; `first` one) |
| Transform | `map`, `filter`, `take`, `skip`, `takeWhile`, `dropWhile`, `concat`, `enumerate` | No (lazy; consumed when the result is) |
| Collect | `values`, `collect`, `reduce`, `join` | Yes |
| Restart | `restart`, `canRestart` | Re-runs the source |
| Bridge | `asStream` (table to stream) | n/a |

---

## 9. Open questions

- **Key-aware `peek` / `first`:** whether `peek` and `first` on a key-value stream return the
  `key => value` pair or just the value.
- **Infinite streams and collectors:** collecting or counting an infinite stream (`naturals`)
  never terminates; whether the API should mark or guard against collecting an unbounded stream,
  or leave it to the programmer (as `take` before collect).
- **Resource cleanup on abandonment:** how `restart`, short-circuit (`take`), and abandonment
  release an underlying resource (close the file), pending the resource/`io` model.
- **Parallel / concurrent transformers:** whether any transformer runs stages concurrently
  (across green threads), pending the concurrency model.
