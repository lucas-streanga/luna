# Stream

A `stream` is a **lazy, single-pass sequence**: it produces its elements one at a time, on
demand, and does not retain them. A file read line by line, a transformed sequence, a
generator of values, all are streams. The two defining properties are **lazy** (nothing is
produced until a consumer asks) and **single-pass** (each element is seen once, as it flows
by, and is not kept). Together these are what make a stream memory-efficient: you never hold
the whole sequence at once.

```
let f = openFile('data.log', {read});
foreach (f.lines() as lineNumber => line) {
  // `lines()` is a stream: one line in memory at a time, not the whole file
}
```

A stream is inspired by generators (as in PHP). It is **not** a kind of table (§5): it shares
*iteration* with tables but not random access or cheap length, because its elements are not
retained. The right mental model is "a lazy single-pass sequence with a generator producer
and a `foreach` consumer," not "a table computed lazily."

---

## 1. Producing a stream: generator functions

A stream is produced by a **generator function**, a function whose body uses `yield`. Calling
it returns a stream; the return type is `stream`:

```
const naturals = fn (): stream {
  var n = 0;
  loop {
    yield n;              // values-only: yields a value
    n = n + 1;
  }
};

const entries = fn (t: table): stream {
  foreach (t as k => v) {
    yield k => v;          // key-value: yields a key and a value
  }
};
```

`yield` suspends the function, hands the value (or `key => value` pair) to the consumer, and
resumes when the consumer asks for the next element. A function containing `yield` is a
generator and its result type is `stream`.

### 1.1 Values-only and key-value streams

A stream yields either **values only** (`yield v`) or **key-value pairs** (`yield k => v`),
mirroring the list-versus-table distinction on retained data:

- A **values-only** stream is the stream analogue of a `list`. Collected (§5), it produces a
  `list` (values reindexed from 0).
- A **key-value** stream is the analogue of a keyed `table`. Collected, it produces a
  `table`.

So the values-only/key-value choice on a stream is the same distinction as list/table on
in-memory data, carried into the lazy world.

### 1.2 Lazy-start: producing nothing until consumed

A stream is **lazy-start**: its body does not run until consumption begins. Constructing or
returning a stream runs nothing, so `let s = f.lines()` opens no file and reads no line yet;
the file is opened and the first line read only when something starts consuming `s`. This is
what makes returning a stream free of side effects, and it is why building a pipeline (§7)
consumes nothing until the pipeline itself is consumed.

---

## 2. Single-pass consumption

A stream is consumed by iterating it, normally with `foreach`:

```
foreach (s as v) { ... }         // values-only stream
foreach (s as k => v) { ... }    // key-value stream
```

Consumption is **single-pass**: each element is produced once, seen once, and not retained.
After a stream is consumed, it is **exhausted**, iterating it again yields nothing (an empty
pass), because its elements are gone and its generator has run to completion.

This is a **discipline, not an enforced move** (matching how single-pass already works):
iterating an exhausted stream is not a compile error, it simply produces nothing. To traverse
a sequence more than once, re-create the stream (§4) or materialize it into a table or list
(§5), paying the memory cost deliberately.

---

## 3. Inspecting a stream: emptiness, peek, consumed

Because a stream is lazy, "is it empty?" cannot be answered without running the body far
enough to produce (or fail to produce) the first element, this is inherent to laziness, not a
Luna limitation. Luna answers it with **one-element lookahead**, which starts the stream
minimally without losing data:

- **`peek()`** runs the body just far enough to produce the first element, buffers it, and
  returns it without consuming it. A later `foreach` still sees that element, it was buffered,
  not taken.
- **`isEmpty()`** peeks and reports whether an element exists. It starts the stream (a bounded
  side effect, e.g. opening the file and reading one line) but consumes nothing (the peeked
  element is still delivered).
- **`isConsumed()`** reports whether the stream has been exhausted (fully consumed). It runs
  nothing; it reads the stream's state.

So emptiness is knowable at the cost of *starting* the stream (one element of work, a bounded
side effect) but never at the cost of *losing* an element. Zero-side-effect emptiness is
impossible under laziness; one-element lookahead is the best achievable, and it is
non-destructive. These operations are catalogued in **stream-api**.

---

## 4. Restart is explicit and source-dependent

A stream is single-pass, so it does not restart implicitly. Restarting is an **explicit**
operation whose availability depends on the source:

- **`restart()`** re-runs the generator from the beginning, valid only when the source can be
  re-run (a file can be re-opened, a pure computation re-executed). On a source that cannot be
  replayed (a consumed socket, stdin, a one-shot sensor), `restart()` is unavailable or
  raises. The API is honest per-source about whether restart is possible.
- **Re-creating** the stream (calling `f.lines()` again) is the general way to traverse again,
  it makes a fresh stream from the source.

Streams are **not replayable by default** on purpose. Replay-by-default would either lie for
non-replayable sources (sockets, stdin, random generators) or force every stream to buffer
everything it produces, which destroys the memory efficiency that is the whole point. Making
restart explicit keeps the type honest (single-pass is deliverable by every source) and keeps
the cost of re-traversal visible.

---

## 5. A stream is not a table

A stream shares **iteration** with tables (`foreach` works the same) but is a **weaker,
forward-only interface**, because its elements are not retained:

- **No random access.** `stream[5]` is not available: a stream does not have element 5 in
  memory; reaching it would require producing and discarding 0 through 4. Random indexing is a
  table operation; a stream cannot offer it.
- **No cheap length.** `count()` on a stream must consume it entirely to count, and doing so
  **exhausts** it. A table knows its length in O(1); a stream can only learn its length by
  running out.

So a stream is not "a table you compute lazily"; it is a distinct forward-only sequence that
happens to share the iteration protocol. Treat it as single-pass and sequential.

### 5.1 Bridging streams and retained data

- **Table or list to stream (`asStream`):** a `table`/`list` can be iterated lazily as a
  stream (the `asStream` option on producing operations, and a `table.asStream()` adapter).
  This does **not** reclaim the table's memory (it is already in memory); its purpose is
  **interface uniformity** (feeding in-memory data into stream-consuming pipelines) and
  **downstream laziness** (deferring and short-circuiting per-element work, so
  `bigTable.asStream().map(expensive).take(3)` runs `expensive` only three times).
- **Stream to table or list (materialize):** collecting a stream (`values()` and the collect
  operations, stream-api) consumes it into a retained `table` or `list`, paying the memory
  cost, so the result can be randomly accessed and re-iterated. A values-only stream collects
  to a `list`; a key-value stream to a `table` (§1.1).

So `asStream` goes from retained to lazy (for pipeline uniformity and deferred work), and
materializing goes from lazy to retained (for random access and reuse). The memory tradeoff is
explicit in which direction you cross.

---

## 6. The `stream` type

`stream` is its own type. Like `regex` and `command`, it is an opaque value: you produce it
(a generator function), inspect it (§3), consume it (§2), transform it, and collect it
(stream-api), but its internal generator state is not user-inspectable.

A stream is single-owner and, like a builder, not meant to be shared across concurrent tasks;
under green threads with enforced copying, each task holds its own. Its element type follows
the source (typed for a typed source, `any` for an untyped one, the same rules as table
elements, tables and protocols specs).

---

## 7. The pipeline operator `|>`

`|>` connects a dataflow pipeline. It is **not** general function application, Luna already
has that through UFCS (`x.f().g()`), so a general pipe operator would be redundant. `|>` exists
only where "pipeline" is a genuine domain concept: **streams and commands**. Seeing `|>`
therefore always means "a dataflow pipeline," never merely "call a function."

### 7.1 Closed over kind

`|>` connects two values of the same pipeline-able kind and returns a new value of that kind:

- **`stream |> transformer` produces a `stream`**: a new stream describing the composed
  dataflow.
- **`command |> command` produces a `command`** (command spec §4): a new command describing the
  composed process pipeline.

So `|>` does not convert commands into streams or vice versa; each pipes within its own kind.
Bridging a stream to a command (feeding stream elements to a process's stdin, or reading a
process's stdout as a stream) is an explicit `exec`-level operation, not bare `|>`, because it
requires serialization across the boundary, which is a real operation deserving an explicit
API rather than an operator.

### 7.2 Stream pipelines are pull-driven

A stream pipeline is a **processing** pipeline: the left of `|>` is a stream (the source), and
each right-hand stage is a **stream transformer** (a stream-to-stream operation like `map`,
`filter`, `take`):

```
f.lines() |> filter(isError) |> map(parse) |> take(10)
```

Consumption is **pull-driven** (demand-based), which is what laziness requires: the consumer at
the end pulls an element, which pulls from the previous stage, back to the source, which
produces one element that flows forward through the stages. One element traverses the whole
pipeline per step, on demand. So `take(10)` pulls only ten times, and the file source produces
only about ten lines, the pipeline short-circuits, and memory stays bounded. The pipeline
result is itself a lazy stream (a description of the pull-chain), inert until consumed (§1.2).

### 7.3 Piping transfers and consumes (single-pass through the pipeline)

`a |> t` **transfers** the stream `a` into the resulting pipeline. Because of lazy-start,
building the pipeline consumes nothing; but when the pipeline is consumed, it pulls from `a`,
so **consuming the pipeline consumes `a`**. After piping, `a` is a **stage of the pipeline,
not an independent stream**: consume the pipeline result, not the original.

This is not a special side effect of `|>`; it is the same single-pass rule that governs
`foreach`. Using a piped-from stream independently gives you a stream that will be (or has
been) consumed through the pipeline, exactly as iterating any stream twice gives an exhausted
second pass. The discipline is: **once you pipe a stream, consume the pipeline, not the
original.** As with all single-pass behavior (§2), this is a discipline, not an enforced move,
independent use is not a compile error, it just yields consumed elements.

---

## 8. Open questions

- **Inline streams:** whether an anonymous stream literal (a `stream { ... }` block) exists
  alongside named generator functions, or whether generators are always named functions.
- **Bidirectional generators:** whether `yield` can receive a value back from the consumer (a
  two-way generator, as in Python), or is one-way (produce only); current model is one-way.
- **Parallel consumption:** how a stream interacts with green threads if one is ever shared
  despite the single-owner intent, pending the concurrency model.
- **`asStream` element typing:** whether a `stream` carries its element type as precisely as a
  typed table does, pending how far element typing is carried through transformers.
- **Early termination and cleanup:** how a stream releases a resource (closes the file) when a
  pipeline short-circuits (`take(10)` stops before the file ends) or is abandoned before
  exhaustion.
