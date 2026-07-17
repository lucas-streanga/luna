# Iterable Functions

This is the operation catalogue for **iterables** — values traversable in order. `iterable`
is a built-in type, the union `table | stream`, written bare in signatures exactly as `list`
is (tables.md §2). Everything in this catalogue is a **built-in free function**: in scope
everywhere, no import, no module, no protocol. A call is written directly or through UFCS
(functions §3.4) — `map(x, f)` and `x.map(f)` are the same call, resolved statically.

There is no table method surface. `tab.name` is element access, `tab.name(...)` is UFCS to a
free function, `tab->P` is a named user protocol — each spelling has exactly one meaning.
Table-only operations (keyed access, seal state, mutation, and the whole-input family) live
in **indexable-functions.md**. Stream-only operations (`peek`, `isConsumed`, `restart`,
`canRestart`, and producing via `yield`) live in **stream-api.md**. Concepts remain
authoritative in **tables.md** and **stream.md**.

---

## 1. The governing rules

### 1.1 Eligibility: traversal only

A function belongs here iff its semantics need only **ordered traversal of `key => value`
pairs** — it is lazy, short-circuiting, or bounded-memory. Anything that needs keyed (random)
access, table state, or the entire input at once is table-only (indexable-functions.md).

### 1.2 Every element has a key

Every iterable yields `key => value` pairs. A keyed table's pairs are its elements; a list's
keys are `0..n-1`; a stream whose generator yields bare values has **implicit keys**
`0, 1, 2…`. There is no "values-only" kind of stream: a stream is to a table exactly what a
list is to a keyed table, and the implicit keys are the same integers a list would carry.
The symmetry makes every key-facing function (`keys`, `keyOf`, `flip`, `keyFirst`, mode
`{keys}`) total over `iterable`.

Implicit keys behave precisely like list keys: per-element functions **preserve** them (so
they go sparse after `filter`, as a list's do), and `values` reindexes from 0. A stream whose
implicit keys were never disturbed collects to a `list` (§2.11).

### 1.3 Output kind follows the primary operand

A function typed `iterable → iterable` returns the **same kind as its first operand**: table
in, table out; stream in, stream out (for `combine`, the constructor-style exception, the
first operand decides). There is no `asStream` flag anywhere; the two bridges are explicit
and priced honestly:

- **`toStream(tab)` is O(1) and lazy** — adapting retained data costs nothing, so the
  table→stream direction is a cheap incantation. A stream out of table inputs is spelled by
  bridging the primary: `a.toStream().merge(b)`.
- **`collect(s)` retains O(n)** — the stream→table direction is the expensive one, and
  `collect` is its single, visible spelling (§2.11).

### 1.4 Memory collection is explicit

No function here retains the whole input at once. Fully **consuming** a stream is allowed
(`count`, `last`, the aggregates, `reduce`, `collect` — each with O(1) or bounded working
state), but any operation that must hold **every element before producing its first output**
— `sort`, `reverse`, `shuffle`, `groupBy`, `partition` — takes a `table` and lives in
indexable-functions §4. On a stream the retention is spelled out: `s.collect().sort()`.

This is also the infinite-stream guard: a stream may be infinite, and the functions that
never terminate on one are exactly the full consumers (marked **consumes** below); the
whole-input family cannot even be handed one. Guard with `take` (stream-api §9).

### 1.5 Streams are taken

Passing a stream to any function here — as the primary or as an operand — **takes** it, the
same transfer discipline as `|>` (stream spec §7): the variable is consumed and later use
panics (a compile error where detectable). Lazy results consume their sources as they are
themselves consumed.

### 1.6 Reading the signatures

- **Receiver first.** Signatures begin `fn name(it: iterable, …)`; `combine` is the one
  constructor-style exception, noted at its entry.
- **Parameter order:** `it`, required operands, function hooks (`transformFn` /
  `predicateFn` / `compareFn` / `keyFn`), `mode`, then result-shaping flags
  (`preserveKeys`, `all`, `recursive`, `depth`).
- **Keyword-only tail.** Where a function takes a `...` variadic, the variadic is terminal
  and any options following it are keyword-only, written after a `*,` marker.
- **Notation.** `name?` marks an optional parameter (default `null` unless shown). Hook
  parameters are of type `fn`. Every enum parameter is written out in full, so each synopsis
  stands alone.
- **Deferred.** The former `onNoGet` / `onNoSet` permission enums are absent from every
  signature pending the protocol redesign (table-api.md records the deferral).

### 1.7 The canonical `mode` enum

The base enum has three members with a uniform meaning — `values` puts values in play,
`keys` puts keys in play, `both` puts both in play:

```
mode: enum {values, keys, both} = {values}
```

Two families extend or restrict the base set, each for a specific reason:

| Family | Members | `both` means |
|-|-|-|
| **Callback-transform**: `map`, `filter`, `each`, `every`, `some`, `mapLeaves` (and `groupBy`, `partition`, indexable-functions §4) | `{values, keys, both}` | callback receives both operands: `fn(value, key)` |
| **Matcher**: `find`, `exists`, `remove` | `{values, keys, both, either}` | value **and** key must match |

- **`either`** belongs to the matcher family only: "match if value **or** key matches." It
  is coherent only for membership tests, so transforms exclude it.
- In the callback family, `both` always passes `(value, key)` in that order. The callback's
  *input* shape is constant across the family; only its *return* interpretation varies
  (`map` returns a `[key, value]` pair; predicate transforms return `bool`).
- `sort`'s extended set (`keyThenValue` / `valueThenKey`) is its own, indexable-functions §4.
- `keyOf` is intentionally mode-less (fixed value → key); `find` in `{keys}` mode already
  covers key → value. `diff` and `intersect` take the base set; `distinct` / `unique`
  compare values only and take no `mode`.

---

## 2. The catalogue

Each entry gives the signature, its time complexity (per kind where they differ), and a
description. **Consumes** marks full consumption of a stream input (never terminates on an
infinite stream). Where behavior differs between kinds it is stated; otherwise it is
identical.

### 2.1 Introspect

#### isEmpty()
```
fn isEmpty(it: iterable): bool
```
**O(1).** True if there are no elements. On a stream, peeks one element: the stream is
started (a bounded side effect) but nothing is consumed. Replaces the retired table `empty`
— one name, and it joins the `isList` / `isConsumed` predicate family.

#### count()
```
fn count(it: iterable): int
```
**O(1) table / O(n) stream, consumes.** Number of elements. On a stream, exhausts it; to
count and keep the data, `collect` first.

### 2.2 Endpoints

#### first() · last()
```
fn first(it: iterable): any
fn last(it: iterable): any
```
**first O(1) · last O(1) table / O(n) stream (consumes).** First / last value; `undefined`
if empty. On a stream, `first` consumes exactly one element (`peek`, stream-api §2, buffers
without consuming); `last` runs to the end with O(1) working memory.

#### keyFirst() · keyLast()
```
fn keyFirst(it: iterable): any
fn keyLast(it: iterable): any
```
**keyFirst O(1) · keyLast O(1) table / O(n) stream (consumes).** First / last key;
`undefined` if empty. On an implicit-key stream (§1.2), `keyFirst` is `0` and `keyLast` is
`count − 1` — the latter at full-consumption cost, the same class as `count`.

### 2.3 Search & predicates

#### find()
```
fn find(it: iterable, value: any = null, key: any = null, compareFn?,
        mode: enum {values, keys, both, either} = {values}): any
```
**O(n), short-circuits.** The first value satisfying the match; `undefined` if none. With no
operands, returns the first value. Uses `==` unless `compareFn` (`fn(a, b): bool`) is set.

#### keyOf()
```
fn keyOf(it: iterable, value: any, compareFn?, all: bool = false): any
```
**O(n), short-circuits (consumes with `all`).** The first key whose value matches `value`;
`undefined` if none. With `all = true`, a table of every matching key. The key-returning
complement of `find`.

#### exists()
```
fn exists(it: iterable, value: any = null, key: any = null, compareFn?,
          mode: enum {values, keys, both, either} = {values}): bool
```
**O(1) / O(n), short-circuits.** True if at least one value / key / either / both matches.
O(1) when matching keys only on a table; O(n) otherwise. Use for equality tests; use `some`
for predicate tests.

#### some() · every()
```
fn some(it: iterable, predicateFn?, mode: enum {values, keys, both} = {values}): bool
fn every(it: iterable, predicateFn?, mode: enum {values, keys, both} = {values}): bool
```
**O(n), short-circuit.** Whether any / all elements satisfy `predicateFn` (`fn(x): bool`).
With `predicateFn` omitted, tests truthiness.

### 2.4 Transform

Transformers are lazy on streams: they return a new stream that runs only as it is consumed
(stream spec §7), and are equivalently written as chained UFCS calls or a `|>` pipeline.

#### map()
```
fn map(it: iterable, transformFn?, mode: enum {values, keys, both} = {values}): iterable
```
**O(n), lazy on streams.** Transforms each value via `transformFn` (`fn(value): any`). In
`{keys}` mode keys are passed; in `{both}`, `fn(value, key): [key, value]`. In `{keys}` /
`{both}`, a returned key that is `null` / `undefined` is skipped.

#### filter()
```
fn filter(it: iterable, predicateFn?, mode: enum {values, keys, both} = {values}): iterable
```
**O(n), lazy on streams.** Keeps elements where `predicateFn` (`fn(value): bool`) returns
true. `{keys}` passes keys; `{both}` passes `fn(value, key): bool`. Keys are preserved —
implicit stream keys go sparse exactly as a list's would; follow with `values` to reindex.

#### mapLeaves()
```
fn mapLeaves(it: iterable, transformFn?, mode: enum {values, keys, both} = {values}): iterable
```
**O(n), lazy on streams.** Recursively descends to every leaf and applies `transformFn`.
Table-valued elements are retained data, so descent stays lazy per element.

#### each()
```
fn each(it: iterable, callbackFn: fn, mode: enum {values, keys, both} = {values}): iterable
```
**O(n), lazy on streams.** Side-effecting iteration. Returns its input, so it chains; on a
stream it is a lazy tap, running `callbackFn` as elements flow through. Returning `false`
from `callbackFn` stops early — on a stream, the result ends (downstream sees end-of-stream).

#### reduce()
```
fn reduce(it: iterable, reductionFn?, initial: any = null): any
```
**O(n), consumes.** Folds via `reductionFn` (`fn(carry, item): any`), starting from
`initial`. O(1) working memory beyond the accumulator — and therefore the deliberate escape
hatch for building retained data from a stream by hand.

#### keyCase()
```
fn keyCase(it: iterable, uppercase: bool): iterable
```
**O(n), lazy on streams.** Upper- or lower-cases every string key.

### 2.5 Reshape

#### values() · keys()
```
fn values(it: iterable): iterable
fn keys(it: iterable): iterable
```
**O(n), lazy on streams.** Values-only / keys-as-values, reindexed sequentially from 0.
`values` is the reindexer that composes after any key-disturbing stage
(`s |> filter(p) |> values`); `keys` reads no values. Note `values` is a kind-preserving
**transform** everywhere — the retired stream collector of the same name is `collect` (§2.11).

#### column()
```
fn column(it: iterable, column: string|int, newKeyColumn?: string|int): iterable
```
**O(n), lazy on streams.** From an iterable of rows (tables), extracts the value at key
`column` from each row. With `newKeyColumn` set, keys the result by that column's value;
otherwise reindexes from 0.

#### flip()
```
fn flip(it: iterable): iterable
```
**O(n), lazy on streams.** Swaps each key with its value. On a table, colliding keys
overwrite in order; on a stream, duplicates flow through and resolve only at `collect`
(last wins — the same final table).

#### combine()
```
fn combine(keys: iterable, values: iterable): iterable
```
**O(n), lazy on streams.** Constructor-style (no receiver): pairs the values of `keys` with
the values of `values` into a new iterable. Output kind follows the first operand.

#### chunk()
```
fn chunk(it: iterable, length: int, preserveKeys: bool = true): iterable
```
**O(n), lazy on streams (O(length) working memory).** Splits into successive sub-tables of
`length` elements; on a stream, each completed chunk is yielded as one element.
`preserveKeys = false` reindexes each chunk as a list.

#### flatten()
```
fn flatten(it: iterable, depth: int = -1, preserveKeys: bool = false): iterable
```
**O(n), lazy on streams.** Flattens nested tables. `depth = -1` flattens fully. Reindexes by
default, since keys collide across levels (combiners reindex, R90).

### 2.6 Segment

The segmenters are the stream API's positional cuts, now total over `iterable` — on tables
they are the traversal-order counterpart of the key-addressed `slice`
(indexable-functions §3). Keys are preserved; follow with `values` to reindex.

#### take() · skip()
```
fn take(it: iterable, n: int): iterable
fn skip(it: iterable, n: int): iterable
```
**O(n), lazy on streams.** At most the first `n` elements / everything after the first `n`.
`take` short-circuits: it stops pulling upstream once satisfied, so
`s |> map(expensive) |> take(3)` runs `expensive` three times.

#### takeWhile() · dropWhile()
```
fn takeWhile(it: iterable, predicateFn: fn): iterable
fn dropWhile(it: iterable, predicateFn: fn): iterable
```
**O(n), lazy on streams.** Yield until / discard until `predicateFn` (`fn(value): bool`) is
first false. `takeWhile` short-circuits like `take`.

### 2.7 Combine & compare

#### merge()
```
fn merge(it: iterable, ...its: iterable,
         *, recursive: bool = false, preserveKeys: bool = false): iterable
```
**O(n), lazy on streams.** Appends `its` onto `it` in order: integer keys reindex from 0 and
append, string keys overwrite by key. This is exactly the spread fold (spread §1), so
`a.merge(b)` is `[...a, ...b]`, and merging lists concatenates them. `preserveKeys = true`
is the other operation: `its`' integer keys are kept, so they collide with `it`'s and
overwrite. When `recursive`, keys shared between tables have their (table) values merged
recursively. **`merge` subsumes the retired stream `concat`**: on streams it is lazy
concatenation, duplicate string keys flowing through in order and resolving at `collect`
(last wins), the same table `merge` would build.

#### diff() · intersect()
```
fn diff(it: iterable, ...tabs: table,
        *, compareFn?, mode: enum {values, keys, both} = {both}): iterable
fn intersect(it: iterable, ...tabs: table,
             *, compareFn?, mode: enum {values, keys, both} = {values}): iterable
```
**O(n²), lazy on streams.** Keeps elements of `it` **not present in all** / **present in
all** of `tabs`. Keys preserved. Uses `==` unless `compareFn` (`fn(a, b): bool`) is set. The
operands are `table`, not `iterable`: they are probed for membership, which is keyed access.
Under `mode = {both}`, equal iff key **and** value equal.

#### distinct() · unique()
```
fn distinct(it: iterable, compareFn?): iterable
fn unique(it: iterable, compareFn?): iterable
```
**O(n), lazy on streams (working memory grows with distinct values seen).** Both drop
duplicate values, first occurrence winning, using `==` unless `compareFn` is set. They
differ only in keying: `distinct` reindexes the result from 0, while `unique` preserves the
original keys of the surviving elements.

#### replace()
```
fn replace(it: iterable, ...replacements: table, *, recursive: bool = false): iterable
```
**O(n), lazy on streams.** Replaces values in `it` by matching key against `replacements`.
When `recursive` and a matched value is a table, descends and replaces within it.

### 2.8 Grow & shrink

#### prepend() · append()
```
fn prepend(it: iterable, value: any): iterable
fn append(it: iterable, value: any): iterable
```
**prepend O(n) table / O(1) stream · append O(1), lazy on streams.** Add `value` at the
front / back. On a table, `prepend` reindexes a list and `append` is O(1) amortized; on a
stream both are lazy stages. On a table these are grow operations: legal in pure form, and
answering to open-state only under `&`-write-back onto a sealed target (tables §5).

#### remove()
```
fn remove(it: iterable, value: any = null, key: any = null, compareFn?,
          mode: enum {values, keys, both, either} = {values}, all: bool = false): iterable
```
**O(n), lazy on streams.** Removes the first matching element, or every match with
`all = true` — the matcher-family complement of `filter` (equality match rather than
predicate). To count removals, diff `count()` around the call.

### 2.9 Pick

#### random()
```
fn random(it: iterable, num: int = 1, randFn?, preserveKeys: bool = true): table
```
**O(n), consumes.** Picks `num` random elements; `preserveKeys = false` reindexes from 0.
Returns a `table` (the picks are retained), and on a stream uses reservoir sampling: one
pass, O(num) working memory — the one member of the order family that never needs the whole
input at once, which is why its siblings (`sort`, `shuffle`, `reverse`) are table-only
(indexable-functions §4) and it is not.

### 2.10 Aggregate

All aggregates **consume** a stream and return a scalar with bounded working state. The
numeric aggregates ignore values that are not `int` or `double`.

#### sum() · average() · product()
```
fn sum(it: iterable): int | double
fn average(it: iterable): int | double
fn product(it: iterable): int | double
```
**O(n), consumes.** Sum / mean / product of the numeric values.

#### min() · max()
```
fn min(it: iterable, compareFn?): any
fn max(it: iterable, compareFn?): any
```
**O(n), consumes.** Least / greatest value by standard comparison, or by `compareFn`
(`fn(a, b): int`). `undefined` if empty.

#### mode()
```
fn mode(it: iterable, compareFn?): any
```
**O(n), consumes (working memory grows with distinct values).** The most frequent value;
ties resolve to the first occurrence.

#### join()
```
fn join(it: iterable, glue: string = '', finalGlue?: string = null): string
```
**O(n), consumes.** Concatenates values into a string separated by `glue`. `finalGlue` sets
a distinct separator before the last element (`"a, b and c"`); on a stream this costs one
element of lookahead. Non-strings are coerced.

### 2.11 Bridges

The two bridges are total over `iterable` and are the identity on their own kind, so either
can be applied unconditionally to normalize.

#### toStream()
```
fn toStream(it: iterable): stream
```
**O(1), lazy.** On a stream, the identity. On a table, adapts retained data into the stream
interface (stream spec §5.1) without copying — the memory is already spent; this exists for
pipeline uniformity and downstream laziness. The free direction (§1.3).

#### collect()
```
fn collect(it: iterable): table
```
**O(1) table / O(n) stream, consumes.** On a table, the identity. On a stream, consumes it
into retained data — **the only stream→retained bridge**, and the visible O(n) cost (§1.3).
A stream whose implicit keys (§1.2) were never disturbed collects to a `list`; explicit or
disturbed keys collect to a keyed table. Replaces the retired stream `values` collector.

---

## 3. Retired spellings

For the corpus sweep and for readers of older drafts; do not reintroduce:

| Retired | Current |
|-|-|
| `tab->map()` and the built-in table protocol | UFCS to built-in free functions: `tab.map(f)` ≡ `map(tab, f)` |
| `empty(tab)` | `isEmpty(it)` |
| `concat(a, b)` | `merge(a, b)` |
| `enumerate(s)` | nothing — every stream is keyed, implicitly `0, 1, 2…` (§1.2) |
| `values(s)` as a stream→list collector | `collect(s)`; `values` is the reindexing transform (§2.5) |
| `asStream: bool` parameters | output kind follows the primary operand (§1.3); bridge with `toStream()` |
| `asStream(tab)` as the bridge's name | `toStream(it)` — `as` is narrowing (as spec); conversions are `to*` (conversion spec) |
| `onNoGet` / `onNoSet` parameters | deferred pending the protocol redesign (table-api.md) |
