# Stream

A `stream` is a **lazy, single-pass sequence**: it produces its elements one at a time, on
demand, and does not retain them. A file read line by line, a transformed sequence, a
generator of values, all are streams. The two defining properties are **lazy** (nothing is
produced until a consumer asks) and **single-pass** (each element is seen once, as it flows
by, and is not kept). Together these are what make a stream memory-efficient: you never hold
the whole sequence at once.

```luna
let f = openFile('data.log', .{read});
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

A stream is produced by a **generator function**, a function whose body uses `yield` — or
inline, by a `gen` block (§1.4). Calling a generator returns a stream; the return type is
`stream`:

```luna
const naturals = fn (): stream => {
  var n = 0;
  while (true) {
    yield n;              // bare yield: implicit keys 0, 1, 2, ... (§1.1)
    n = n + 1;
  }
};

const entries = fn (t: table): stream => {
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

Generator-ness is deliberately **not readable off the type** (R209): a generator
`fn (): stream` and an ordinary function that *returns* streams built elsewhere (a stored
stream, or an invoked nested generator — `return (fn () => { yield 0; })();`, where the
invocation is what constructs the stream) are **observably identical to callers** — both
hand back an unstarted, lazy stream. Generator-ness is a private property of the literal
(a descriptor bit, function-representation §2), which is exactly why classification is the
lexical scan above and never the return type. Forgetting the invocation is a **compile
error**, not a silent bug: a `fn` value does not fit a `stream`-typed position.

**In a generator body, `return` must be bare** (R209). `return;` ends the stream — the
early end, without `break` shenanigans through enclosing loops — taking the ordinary
exhaustion path (§1.3: mark done, run defers). `return expr;` is a **compile error**: the
generator's caller already received the stream at construction, so a returned value has
**no recipient**. This deliberately refuses PHP's `getReturn()` (a second, out-of-band
result channel you must know to poll), and it keeps the lexical rule airtight — a body
mixing `yield` with a valued `return` is *rejected*, never ambiguously classified. The fix
the diagnostic teaches is structural: put the choose-what-to-return decision in an outer,
ordinary function, and the stream construction in the nested generator.

**`yield` may not appear inside a `try` block** (R210). A try's protection covers its
dynamic extent, and a yield inside it would make that extent **span suspensions** — the one
enclosing construct whose hosted realization is frame-shaped rather than pc-shaped
(stream-representation §2.1 records the machinery this would have demanded). The rule's
edges, precisely:

- **Catch blocks are unrestricted** — a catch body is post-recovery ordinary code, and
  `catch (e) => { yield fallback; }` is legal. (A yielding catch inside an *outer* try makes
  that try transitively contain a yield, and the same rule rejects it — self-consistent.)
- **Try-expressions cannot contain a yield structurally** (expressions do not contain
  statements), so `let v = try parse(x); yield v;` — the workhorse per-element recovery —
  is untouched and idiomatic.
- **Defer bodies are the parallel ban** (§1.3, R207): the two places a yield can never run
  are the two places it is rejected at parse, in the same lexical walk that classifies the
  generator (one try-depth counter; near-zero cost).

**Nothing is lost, because a spanning catch never had power — only grouping.** After any
catch runs, the rest of its try body is abandoned (that is what catch means), so
"recover and *continue* yielding" was never expressible with a spanning try in any design —
per-element recovery always required per-element trys. What the restriction forbids is one
spelling of "shared recovery for a prefix-run that then ends," and both replacement
spellings are ruled idioms: the per-element try with the R209 bare return
(`e: error => { yield fallback; return; }`), and the **consumer-side supervisor** — a panic
in a resume propagates out of the pull, and `try` around consumption is already the
designated boundary for mid-stream failure (errors §8.2; io §6's own rule).

### 1.1 Implicit and explicit keys

Every stream yields `key => value` pairs (iterable-functions §1.2, R93). A generator that
yields bare values (`yield v`) produces **implicit keys** `0, 1, 2, …` — exactly the keys a
`list` carries; `yield k => v` yields **explicit keys**, the keyed-table analogue. There is
no "values-only" kind of stream (the earlier dichotomy is erased): a stream is to a table
exactly what a list is to a keyed table.

The implicit key is a **running next-integer-index**, the same counter the table-literal
fold maintains (spread §1), defined over the **whole yield sequence**, not over bare yields
alone (R223): a bare `yield v` emits at the counter and increments it; an integer key that
flows through **explicitly or by delegation** (`yield 5 => v`; `yield from t`, §1.5) is
emitted verbatim and advances the counter past it (`counter = max(counter, k + 1)`); string
keys never touch it. So a bare yield after delegation continues where the delegated keys
left off instead of colliding with them (the PHP wart this rule exists to refuse). The one
divergence from the fold is deliberate: the fold *renumbers* contributed integer keys
because a table's keys are an index and must be unique, while a stream's keys are **flowing
data** — duplicates are representable and pass through — and uniqueness is enforced only at
materialization, by `collect` applying the table's own write rules (§5.1).

A key is **`int` or `string`, and nothing else** — the table key rule, verbatim (tables
spec; R224): a `yield k => v` with any other key type is a `typeError` panic, a compile
error where statically evident. This is what keeps the stream↔table parallel exact (§5.1);
element **values**, like a table's, are per-element dynamic (§6).

Implicit keys are real keys, behaving precisely as a list's: every key-facing function
(`keys`, `keyOf`, `keyFirst`, `flip`, mode `.{keys}`) sees them; per-element transforms
preserve them (so they go sparse after `filter`, as a list's would); `values` reindexes
them. A stream whose implicit keys were never disturbed collects to a `list`; explicit or
disturbed keys collect to a keyed `table` (§5.1, iterable-functions §2.11).

### 1.2 Lazy-start: producing nothing until consumed

A stream is **lazy-start**: its body does not run until consumption begins. Constructing or
returning a stream runs nothing, so `let s = f.lines()` opens no file and reads no line yet;
the file is opened and the first line read only when something starts consuming `s`. This is
what makes returning a stream free of side effects, and it is why building a pipeline (§7)
consumes nothing until the pipeline itself is consumed.

### 1.3 `defer` in a generator body runs on exhaustion (R207)

A generator body's **top-level defers run when the stream exhausts**, and "exhaustion" means
**every body-exit path**: falling off the end, an early bare `return` (§1's R209 rule), and
a body panic alike.
The defers run **during the final pull**, synchronously with consumption — after the
consumer's `foreach` exits, a `defer close()` in the body has already run, deterministically
— with one precisely ruled ordering: **the stream is marked exhausted first, then the defers
run** (LIFO, defer §3), then any unwind continues. The ordering is forced, not chosen: it is
observable in exactly one corner — a *panicking* defer whose pull site catches with `try` —
and mark-first is the only order that leaves that stream coherent (exhausted, defers run
exactly once, a re-pull reporting done rather than resuming a finished frame). A panicked
body likewise marks done before unwinding: a broken generator is never resumable.

Two riders complete the rule:

- **`yield` inside a `defer` body is a compile error.** A defer runs at exhaustion, when
  yielding is definitionally over; lexically a defer block is not a function literal, so its
  `yield` would claim the enclosing generator (§1) while being unable to ever legally run.
  Rejected at parse.
- **An abandoned stream never runs its defers — a stated contract, not an oversight.** A
  stream dropped before exhaustion (`take(3)` on an infinite generator, an early `break`
  and never returning) is reclaimed by the collector, and **no finalizer runs deferred
  code** (the backstop-is-not-a-contract rule, io §4's own discipline). The idiom follows:
  **resources belong to the consumer's `defer`, not the generator's** — which the
  creation-authorization model already enforces structurally (the `fd` is the caller's,
  `lines(fd)` borrows it, `defer close(fd)` sits with the owner, io §6, R121). A
  generator's own defer is for generator-local cleanup, correct where exhaustion is
  guaranteed or forfeiture is acceptable.

Non-top-level defers (inside a block within the body) are unaffected: they run at their
block's exit during ordinary body execution between yields, per defer §1.

### 1.4 Inline generators: `gen` blocks (R221)

An inline stream is spelled with a **`gen` block** — a keyword-introduced literal whose
value is the unstarted stream:

```luna
let countdown = gen {
  var n = 3;
  while (n > 0) { yield n; n = n - 1; }
};

let errLines = gen use (io) {
  foreach (line in lines(fd)) {
    if (line.contains('ERROR')) { yield line; }
  }
};
```

- **Pure sugar.** `gen { body }` is `(fn () => { body })()` — an immediately invoked
  anonymous generator; `gen use (io) { body }` carries the clause onto the literal. The
  invocation *constructs* the stream and runs nothing (lazy-start, §1.2); captures are the
  closure's const-snapshot at construction (functions §2.1); the `use` clause is checked at
  the creation site against the enclosing frame (capabilities §5.1), making the result an
  ordinary effect carrier (R121). Every generator rule applies to the body unchanged: keys
  (§1.1), bare `return` only (R209), no `yield` in `try` (R210), defers on exhaustion
  (R207).
- **A `gen` block is a generator by form.** The keyword is the lexical marker, so §1's
  yield-scan is unnecessary here; a yield-free `gen {}` is the canonical **empty stream**,
  not an error.
- **Parse: one token.** `gen` is a keyword and only ever the literal former (keywords §1);
  `use (` after a `gen` head is the declaration clause, joining `fn` and `test` in R112's
  decided-at-one-token list. The former deliberately does **not** reuse the type's name:
  `stream {}` collides in return-annotation position with the generator's own canonical
  spelling (`fn (): stream => { ... }`), and the house already separates literal former from
  type name — backticks construct a `command`, slashes a `regex`, `gen` a `stream` (R221;
  promoted-`stream` is the recorded road not taken).
- **No parameters.** A `gen` block is invoked at construction; a parameterized producer is
  an ordinary named generator function (§1). Being an expression, it chains:
  `gen { ... }.take(10)` is a pipeline head like any other (§7).

### 1.5 Delegation: `yield from` (R223)

`yield from src;` delegates to any iterable — the whole of it, lazily, one element per
pull:

```luna
const walk = fn (node: table): stream => {
  yield node['value'];
  foreach (c in node['children']) { yield from walk(c); }
};
```

- **Pure sugar**: `yield from src;` is exactly `foreach (k => v in src) { yield k => v; }`.
  Everything inherits: keys pass through verbatim and advance the implicit counter (§1.1),
  a `table` source iterates by the ordinary `foreach` rules, laziness holds (the outer
  generator suspends per inner element), and the desugar contains `yield`, so `yield from`
  is banned inside `try` and `defer` exactly as `yield` is (R210, R207).
- **A stream operand is taken.** Delegating to a stream transfers it (§7.3,
  iterable-functions §1.5 — the syntax obeys the catalogue's rule): the delegating
  generator is now the one live handle, and prior aliases panic (`useAfterTaken`, §2).
- **Lexing**: `yield from` is one compound token; `from` stays unreserved, exactly as
  contextual as import's `from` (modules spec). The one casualty is bare-yielding a
  binding literally named `from` — parenthesize: `yield (from);`.
- **`yield` is a statement**, never an expression: with bidirectionality axed (§8, R224),
  a yield has no value, so `let x = yield v` is unrepresentable rather than forbidden.

---

## 2. Single-pass consumption

A stream is consumed by iterating it, normally with `foreach`:

```luna
foreach (v in s) {}         // values; keys ignored
foreach (k => v in s) {}    // key => value (implicit or explicit keys, §1.1)
```

Streams are **consumable**: consumption is **single-pass**, each element is produced once, seen once, and not retained.
After a stream is consumed, it is **exhausted**: its elements are gone and its generator
has run to completion.

**Re-consuming an exhausted stream panics** (`useAfterConsumed`, errors §2, R222) — the §8
review ran, and single-pass is now **enforced everywhere**, not a discipline (the silent
empty second pass is gone; it hid exactly the double-consumption bugs single-pass exists
to surface). Consumption means `foreach`, destructuring (§2.1), spread, and every
catalogue call; the probes (§3) stay total and never panic — `isConsumed` is the guard,
the probe form to consumption's assertion form, the same hard/soft pairing as
`canReveal`/`reveal`. The panic is about **handle reuse, never emptiness**: a first pass
over an empty stream is zero iterations, ordinary, and marks it exhausted; only the
*second* consumption panics. `useAfterConsumed` and `useAfterTaken` (§7.3, concurrency
§2.3) are siblings under `useAfter` (errors §2) — one family for using a spent handle,
the use-after-free of a language without free, distinguished by *why* the handle is dead:
ended by consumption, or moved. To traverse a sequence more than once, re-create the
stream (§4) or materialize it into a table or list (§5), paying the memory cost
deliberately.

### 2.1 Destructuring consumes a prefix

A positional pattern may take its source from a stream (destructuring §1.4, R103): it
**pulls exactly as many elements as it binds**, and no more.

```luna
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
- **Destructuring takes the stream** (§7.3's move): after `let [a, b] = s`, `s` is
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
- **`isConsumed()`** reports whether the stream can no longer produce — exhausted, **or
  taken** (§7.3): it is the guard for R222's re-consumption panic, so it answers wherever
  that panic could fire. It runs nothing; it reads the referent's terminal state — a
  query, not a use (concurrency §2.3), total even on a taken handle. `taken(x)`
  (concurrency §2.3) distinguishes the reason.

So emptiness is knowable at the cost of *starting* the stream (one element of work, a bounded
side effect) but never at the cost of *losing* an element. Zero-side-effect emptiness is
impossible under laziness; one-element lookahead is the best achievable, and it is
non-destructive. On a **taken** handle (§7.3) the state probes still answer — `isConsumed`
and `taken` are queries, not uses (concurrency §2.3) — while `peek` and `isEmpty` panic
(`useAfterTaken`): they must run a body another owner holds. `peek` and `isConsumed` are catalogued in **stream-api** (§2); `isEmpty`
is total over `iterable` and lives in **iterable-functions** (§2.1).

---

## 4. Restart is explicit and source-dependent

A stream is single-pass, so it does not restart implicitly. Restarting is an **explicit**
operation whose availability depends on the source:

- **`restart()`** re-runs the generator from the beginning, valid only when the source can be
  re-run (an immutable retained snapshot — a range, a string producer, a pure computation
  re-executed; the R105 rule). On a source that cannot be replayed (a consumed socket, stdin,
  a one-shot sensor, **a file stream's live cursor** — io §6, R121), `restart()` is
  unavailable or raises; a file re-traversal is explicit, `seek(fd, 0)` and a fresh view.
  The API is honest per-source about whether restart is possible.
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

This is the ordinary single-pass rule (§2, enforced since R222), not a new one: a stream is a shared,
single-pass reference wherever it lives, so aliasing one through a containing table is the same
"don't consume a stream twice" situation as aliasing one directly. A table holding a stream is
therefore **not** fully independent of its copies with respect to that stream element; the
stream is shared, and first consumption wins. To give two tables genuinely independent
sequences, materialize the stream into retained data first (§5.1), or produce a fresh stream
per table (§4).

---

## 7. Chains are pipelines: pull-driven composition

A chain of catalogue calls over a stream **is** a dataflow pipeline, and needs no
operator to say so (R146: the `|>` operator is **retired**, retired/pipeline.md — after
R91 made every transformer a lazy, kind-following, stream-taking free function, `s |>
map(f)` and `s.map(f)` were the same operation, and the operator had become the
redundant second spelling its own spec forbade). This section states the pipeline
semantics chains have always had.

### 7.1 One kind per chain; commands and bridges

A stream chain produces a `stream` at every stage. **Command** pipelines are the other
dataflow domain, composed by the `pipe` function (command §4) — a `command` at every
stage. Neither converts into the other: bridging a stream to a command (feeding
elements to a process's stdin, reading its stdout as a stream) requires serialization
across the boundary, a real operation deserving an explicit `exec`-level API (exec
spec), never an implicit coercion.

### 7.2 Stream chains are pull-driven

A stream chain is a **processing** pipeline: the source, then transformers
(stream-to-stream operations like `map`, `filter`, `take`):

```luna
_ = f.lines().filter(isError).map(parse).take(10);
```

Consumption is **pull-driven** (demand-based), which is what laziness requires: the consumer at
the end pulls an element, which pulls from the previous stage, back to the source, which
produces one element that flows forward through the stages. One element traverses the whole
chain per step, on demand. So `take(10)` pulls only ten times, and the file source produces
only about ten lines, the chain short-circuits, and memory stays bounded. Each stage's
result is itself a lazy stream (a description of the pull-chain), inert until consumed
(§1.2). (A chain wholly visible at its consumption site may compile to a single fused
loop — the fused lowering, stream-representation §2.2, R225 — with semantics unchanged by
construction.)

### 7.3 Chaining transfers and consumes (single-pass through the chain)

`a.map(f)` **transfers** the stream `a` into the resulting chain. Because of lazy-start,
building the chain consumes nothing; but when the chain is consumed, it pulls from `a`,
so **consuming the chain consumes `a`**. After the call, `a` is a **stage of the chain,
not an independent stream**: consume the result, not the original.

Like §2's exhaustion enforcement, this is an **enforced move**: after
`a.map(f)`, `a` is **taken** and any later use **panics** (`useAfterTaken`, errors §2,
R222), a compile error
where statically evident, the same enforcement as a stream crossing `spawn` (concurrency
§2.3), a promise after `await`, a file after `close`. The upgrade from the earlier
"discipline" wording is deliberate: an aliased source handle and its chain shared a live
cursor, so interleaved pulls made elements silently vanish from one consumer into the other,
which is worse than an exhausted second pass and exactly what single ownership exists to
prevent. **Once you chain a stream, the chain is the stream.**

This is one rule, not a chain-specific one: passing a stream to **any** function of
the iterable catalogue — as the primary or as an operand (`merge(a, s)`,
`combine(ks, vs)`) — **takes** it (iterable-functions §1.5), and a lazy result consumes its
sources as it is itself consumed.

---

## 8. Resolved (R221, R222, R224)

- **Inline streams: ruled, the `gen` block** (R221, §1.4) — a keyword-introduced literal,
  pure sugar over the immediately-invoked anonymous generator, with `use` composing on
  the head. The former is deliberately not the type's name (`stream {}` collides with
  `fn (): stream => { ... }` in annotation position; the command/regex precedent separates
  former from type).
- **Bidirectional generators: axed** (R224). `yield` is one-way; two-way communication is
  a channel pair (`channel()`, channels §1), and a value-returning yield would be a
  second, implicit channel mechanism — one that also composes wrong (a sent-back value
  cannot thread through `map`/`filter`, the wart PHP and Python both carry). Corollary,
  now fixed: **`yield` is a statement**, never an expression (§1.5).
- **Single-pass enforcement: upgraded** (R222, §2). Re-consuming an exhausted stream
  panics `useAfterConsumed`, sibling of `useAfterTaken` under `useAfter` (errors §2); the
  probes stay total (§3). The consistency review this bullet awaited ran: the empty
  second pass hid exactly the bugs single-pass exists to surface.
- *(**Parallel consumption: resolved** — a stream is never shared across tasks: it crosses
  a spawn boundary by **ownership transfer**, leaving every spawner-side alias
  enforced-dead (the taken state, concurrency §2.1, §2.3). Two tasks can never hold live
  handles to one stream, so the question does not arise.)*
- **Element typing: the table rules, nothing more** (R224, §1.1, §6). Elements and keys
  are per-element dynamic exactly as a table's members are; keys are `int | string` and
  nothing else (§1.1); there are no generics to carry more, by doctrine (secret §3.3),
  and transformers track nothing — narrowing is the consumer's `is` / `as` / `match`.
- **Cleanup: R207 is the whole story** (R224). Defers-on-exhaustion during the final
  pull, abandoned streams run nothing (a stated contract), resources belong to the
  consumer's `defer` (io §6). No additional scoped-cleanup convenience rides the stream;
  none earned its weight over the existing rule.
