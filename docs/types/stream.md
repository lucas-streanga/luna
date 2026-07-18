# Stream

A `stream` is a **lazy, single-pass sequence**: it produces its elements one at a time, on
demand, and does not retain them. A file read line by line, a transformed sequence, a
generator of values, all are streams. The two defining properties are **lazy** (nothing is
produced until a consumer asks) and **single-pass** (each element is seen once, as it flows
by, and is not kept). Together these are what make a stream memory-efficient: you never hold
the whole sequence at once.

```
let f = openFile('data.log', {read});
foreach (lineNumber => line in f.lines()) {
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
  while (true) {
    yield n;              // bare yield: implicit keys 0, 1, 2, ... (§1.1)
    n = n + 1;
  }
};

const entries = fn (t: table): stream {
  foreach (k => v in t) {
    yield k => v;          // explicit keys: yields a key and a value
  }
};
```

`yield` suspends the function, hands the value (or `key => value` pair) to the consumer, and
resumes when the consumer asks for the next element. Generator classification is
**per function literal and purely lexical**: a literal is a generator iff its **own body,
excluding nested function literals**, contains `yield`. A nested `fn` with its own `yield`
is its own generator returning its own stream (the PHP rule), and contributes nothing to
the enclosing function's classification. One parse-tree walk decides it, no flow analysis.
A function containing `yield` is a
generator and its result type is `stream`.

### 1.1 Implicit and explicit keys

Every stream yields `key => value` pairs (iterable-functions §1.2, R93). A generator that
yields bare values (`yield v`) produces **implicit keys** `0, 1, 2, …` — exactly the keys a
`list` carries; `yield k => v` yields **explicit keys**, the keyed-table analogue. There is
no "values-only" kind of stream (the earlier dichotomy is erased): a stream is to a table
exactly what a list is to a keyed table.

Implicit keys are real keys, behaving precisely as a list's: every key-facing function
(`keys`, `keyOf`, `keyFirst`, `flip`, mode `{keys}`) sees them; per-element transforms
preserve them (so they go sparse after `filter`, as a list's would); `values` reindexes
them. A stream whose implicit keys were never disturbed collects to a `list`; explicit or
disturbed keys collect to a keyed `table` (§5.1, iterable-functions §2.11).

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
foreach (v in s) { ... }         // values; keys ignored
foreach (k => v in s) { ... }    // key => value (implicit or explicit keys, §1.1)
```

Streams are **consumable**: consumption is **single-pass**, each element is produced once, seen once, and not retained.
After a stream is consumed, it is **exhausted**, iterating it again yields nothing (an empty
pass), because its elements are gone and its generator has run to completion.

This is a **discipline, not an enforced move** for plain re-iteration: iterating an
exhausted stream is not a compile error, it simply produces nothing (deterministically; the
one aliasing case that was worse than exhaustion, a piped-from stream, is an **enforced**
move instead, §7.3, and whether re-iteration should follow is flagged in §8). To traverse
a sequence more than once, re-create the stream (§4) or materialize it into a table or list
(§5), paying the memory cost deliberately.

### 2.1 Destructuring consumes a prefix

A positional pattern may take its source from a stream (destructuring §1.4, R103): it
**pulls exactly as many elements as it binds**, and no more.

```
let [a, b] = s.split(' ');       // consumes two pieces; later pieces stay in the stream
let [head, ...rest] = s;         // consumes one; rest IS the stream, advanced
```

- **Exhaustion binds `undefined`.** A stream that runs out mid-pattern binds the
  remaining targets to `undefined` (coalesce with `??`) — the stream's ordinary absence,
  the same answer `peek` gives on empty. There is deliberately no length check: a
  stream's length is unknowable without consumption (and may be infinite), so the pattern
  is a *take-request*, never the shape assertion it is on a table (destructuring §1.1's
  exact-length rule needs an O(1) length to check against).
- **The rest element is the remaining stream.** `...rest` binds the stream itself,
  advanced past the consumed prefix — lazy head/tail decomposition, no buffering. (On a
  table, `...rest` collects a `list`; on a stream, collecting would defeat the point.)
- **Destructuring takes the stream** (§7.3's discipline): after `let [a, b] = s`, `s` is
  taken, and `rest`, if bound, is the one live handle. Without a rest binding, the
  unconsumed tail is dropped with the stream — or recoverable via `restart` where the
  source allows it (§4).

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
non-destructive. `peek` and `isConsumed` are catalogued in **stream-api** (§2); `isEmpty`
is total over `iterable` and lives in **iterable-functions** (§2.1).

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

One rule makes `canRestart` predictable across the corpus (R105): **a stream whose source
is an immutable retained snapshot is restartable.** String producers (`split`,
`graphemes`, …, string-api), ranges (re-run from `lo`), and `toStream` over a table or a
`bytes` (the COW capture *is* a snapshot) all restart for free; generator functions remain
source-dependent, since a generator over a socket cannot promise replay.

The rule's cost is a **keep-alive**: a restartable stream pins its snapshot (the string,
the captured table) for as long as the stream lives. This is ordinary value lifetime, not
a leak — identical in kind to a closure's deep-`const` capture (functions §2.1) and to a
string slice borrowing its parent's buffer (string-api §6) — and the escape is the same
as ever: `collect` the small result and drop the stream.

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

- **Table or list to stream (`toStream`):** a `table`/`list` can be iterated lazily as a
  stream — `tab.toStream()` (iterable-functions §2.11), O(1) and lazy. This does **not**
  reclaim the table's memory (it is already in memory); its purpose is **interface
  uniformity** (feeding in-memory data into stream-consuming pipelines) and **downstream
  laziness** (deferring and short-circuiting per-element work, so
  `bigTable.toStream().map(expensive).take(3)` runs `expensive` only three times). (The
  former per-operation `asStream: bool` flags are retired, R92: output kind follows the
  primary operand, and this one explicit bridge — renamed from `asStream` in R94 — is the
  spelling for the lazy direction.)
- **Stream to table or list (`collect`):** `collect(s)` (iterable-functions §2.11) consumes
  a stream into retained data, paying the memory cost, so the result can be randomly
  accessed and re-iterated — the single stream→retained bridge. A stream with undisturbed
  implicit keys collects to a `list`; explicit or disturbed keys collect to a keyed
  `table` (§1.1).

So `toStream` goes from retained to lazy (for pipeline uniformity and deferred work), and
`collect` goes from lazy to retained (for random access and reuse). The memory tradeoff is
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

### 6.1 A stream is always by reference, including inside a table

A stream is a **reference value**: it is passed by reference (variables spec §5), never copied
by assignment, because copying a single-pass stream is impossible without either forking it
(which single-pass forbids) or buffering everything (which defeats the point, §4). This
by-reference nature holds **even when a stream is a value inside a table**, and it is the one
exception to a table being an independent value:

- Tables are copy-on-write **value** types: copying a table yields a logically independent
  table (tables spec §4). But a **stream held as a table value is shared by reference**, not
  deep-copied, when the table is copied, because a stream cannot be copied at all.
- So if a table `a` holds a stream and you write `var b = a;`, both `a` and `b` refer to the
  **same** stream. Consuming that stream through `a` (or `b`) consumes it for **both**, exactly
  as consuming any shared stream reference consumes it everywhere (§2, §7.3).

This is the ordinary single-pass discipline (§2), not a new rule: a stream is a shared,
single-pass reference wherever it lives, so aliasing one through a containing table is the same
"don't consume a stream twice" situation as aliasing one directly. A table holding a stream is
therefore **not** fully independent of its copies with respect to that stream element; the
stream is shared, and first consumption wins. To give two tables genuinely independent
sequences, materialize the stream into retained data first (§5.1), or produce a fresh stream
per table (§4).

---

## 7. The pipeline operator `|>`

`|>` connects a dataflow pipeline. It is **not** general function application, Luna already
has that through UFCS (`x.f().g()`), so a general pipe operator would be redundant. `|>` exists
only where "pipeline" is a genuine domain concept: **streams and commands**. Seeing `|>`
therefore always means "a dataflow pipeline," never merely "call a function." This section
covers stream pipelines; the operator's unified semantics across both kinds are in the
**pipeline** spec.

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

Unlike general single-pass exhaustion (§2), piping is an **enforced move** (pipeline spec
§5.1): after `a |> t`, `a` is **taken** and any later use **panics**, a compile error
where statically evident, the same enforcement as a stream crossing `spawn` (concurrency
§2.3), a promise after `await`, a file after `close`. The upgrade from the earlier
"discipline" wording is deliberate: a piped-from handle and its pipeline shared a live
cursor, so interleaved pulls made elements silently vanish from one consumer into the other,
which is worse than an exhausted second pass and exactly what single ownership exists to
prevent. **Once you pipe a stream, the pipeline is the stream.**

The same transfer discipline extends beyond `|>`: passing a stream to **any** function of
the iterable catalogue — as the primary or as an operand (`merge(a, s)`,
`combine(ks, vs)`) — **takes** it (iterable-functions §1.5), and a lazy result consumes its
sources as it is itself consumed.

---

## 8. Open questions

- **Inline streams:** whether an anonymous stream literal (a `stream { ... }` block) exists
  alongside named generator functions, or whether generators are always named functions.
- **Bidirectional generators:** whether `yield` can receive a value back from the consumer (a
  two-way generator, as in Python), or is one-way (produce only); current model is one-way.
- **Single-pass enforcement generally:** §2's exhausted-second-pass behavior (`foreach`
  twice) remains a discipline while `spawn`, `await`, `close`, and now `|>` all enforce
  moves; whether plain re-iteration should also upgrade to taken-value enforcement is a
  consistency review to run once real code exists (the pipe case was upgraded because it
  aliased a *live* cursor; a fully exhausted stream is deterministic, so the case is
  weaker there).
- *(**Parallel consumption: resolved** — a stream is never shared across tasks: it crosses
  a spawn boundary by **ownership transfer**, leaving every spawner-side alias
  enforced-dead (the taken state, concurrency §2.1, §2.3). Two tasks can never hold live
  handles to one stream, so the question does not arise.)*
- **Element typing through `toStream`:** whether a `stream` carries its element type as
  precisely as a typed table does, pending how far element typing is carried through
  transformers.
- **Early termination and cleanup:** the general mechanism for releasing a resource on any exit
  path is **`defer`** (defer spec): `let f = open(...); defer f.close();` closes the file when
  the owning block exits, whether the pipeline is fully consumed, short-circuits (`take(10)`
  stops early), or is abandoned. What remains open is whether a stream *also* offers its own
  scoped-cleanup convenience (an automatic close when a stream tied to a resource is dropped or
  fully consumed), on top of `defer`, pending the stream resource model.
