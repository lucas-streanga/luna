## 9. Protocol API

Each entry gives the signature, its time complexity, and a description. `X?` marks
an optional parameter.

### 9.1 Introspection

#### empty()
```
fn empty(tab: table): bool
```
**O(1).** True if the table has no elements (`[]`).

#### count()
```
fn count(tab: table): int
```
**O(1).** Number of elements.

#### isList()
```
fn isList(tab: table): bool
```
**O(1).** True iff the table is a list — incrementing `int` keys from 0.

#### isContiguousMemory()
```
fn isContiguousMemory(tab: table): bool
```
**O(1).** True iff stored contiguously. Any string key ⇒ false.

#### has()
```
fn has(tab: table, key: any): bool
```
**O(1).** Key presence. Distinct from `exists`, which defaults to an O(n) value
scan.

#### canGet() · canSet()
```
fn canGet(tab: table, key: any): bool
fn canSet(tab: table, key: any): bool
```
**O(1).** Whether `key` may currently be read / written. `canGet` is false for a
missing key or a protocol `noGet` key. `canSet` is false for a missing key, a
protocol `noSet` key, or any key while the table is frozen.

### 9.2 Table-level seals

Each returns the modified table (COW); use `&tab.method(...)` to apply in place.
Per-key get/set permissions are not set here — they are declared by protocols (§6).

#### open() · close() · neverOpen()
```
fn open(tab: table): table
fn close(tab: table): table
fn neverOpen(tab: table): table
```
**O(1).** Set the growth axis: open, closed (revocable), or permanently
non-growable. `open()` on a `neverOpen` table raises `InvalidOpenError`.

#### thaw() · freeze() · neverThaw()
```
fn thaw(tab: table): table
fn freeze(tab: table): table
fn neverThaw(tab: table): table
```
**O(1).** Set the mutation axis: unfrozen, frozen (revocable), or permanently
frozen. `thaw()` on a `neverThaw` table raises `InvalidThawError`.

### 9.3 Endpoints

#### first() · last()
```
fn first(tab: table): any
fn last(tab: table): any
```
**O(1).** First / last value. `undefined` if empty.

#### keyFirst() · keyLast()
```
fn keyFirst(tab: table): any
fn keyLast(tab: table): any
```
**O(1).** First / last key. `undefined` if empty.

### 9.4 Search & predicates

#### find()
```
fn find(tab: table, value: any = null, key: any = null, compareFn?: fn,
        mode: enum {values, keys, both, either} = {values},
        onNoGet: enum {throw, skip} = {throw}): any
```
**O(n).** The first value satisfying the match; `undefined` if none. With no
operands, returns the first value. Uses `==` unless `compareFn` (`fn(a, b): bool`)
is set.

#### keyOf()
```
fn keyOf(tab: table, value: any, compareFn?: fn,
         all: bool = false, onNoGet: enum {throw, skip} = {throw}): any
```
**O(n).** The first key whose value matches `value`; `undefined` if none. With
`all = true`, returns a table of every matching key. The key-returning complement of
`find`.

#### exists()
```
fn exists(tab: table, value: any = null, key: any = null, compareFn?: fn,
          mode: enum {values, keys, both, either} = {values},
          onNoGet: enum {throw, skip} = {throw}): bool
```
**O(1) / O(n).** True if at least one value / key / either / both matches. O(1) when
matching keys only; O(n) otherwise. Use for equality tests; use `some` for predicate
tests.

#### some() · every()
```
fn some(tab: table, predicateFn?: fn mode: enum {values, keys, both} = {values}, onNoGet: enum {throw, skip} = {throw}): bool
fn every(tab: table, predicateFn?: fn mode: enum {values, keys, both} = {values}, onNoGet: enum {throw, skip} = {throw}): bool
```
**O(n).** Whether any / all elements satisfy `predicateFn` (`fn(x): bool`). With
`predicateFn` omitted, tests truthiness. Both short-circuit.

### 9.5 Transform

#### map()
```
fn map(tab: table, transformFn?: fn mode: enum {values, keys, both} = {values},
       onNoGet: enum {throw, skip} = {throw}, asStream = false): table | stream
```
**O(n).** Transforms each value via `transformFn` (`fn(value): any`). In `{keys}`
mode keys are passed; in `{both}`, `fn(value, key): [key, value]`. In `{keys}` /
`{both}`, a returned key that is `null` / `undefined` is skipped.

#### filter()
```
fn filter(tab: table, predicateFn?: fn mode: enum {values, keys, both} = {values},
          onNoGet: enum {throw, skip} = {throw}, asStream = false): table | stream
```
**O(n).** Keeps elements where `predicateFn` (`fn(value): bool`) returns true.
`{keys}` passes keys; `{both}` passes `fn(value, key): bool`. Keys are preserved.

#### mapLeaves()
```
fn mapLeaves(tab: table, transformFn?: fn mode: enum {values, keys, both} = {values}, onNoGet: enum {throw, skip} = {throw}): table
```
**O(n).** Recursively descends to every leaf and applies `transformFn`.

#### each()
```
fn each(tab: table, callbackFn: fn, mode: enum {values, keys, both} = {values}, onNoGet: enum {throw, skip} = {throw}): table
```
**O(n).** Side-effecting iteration. Returns the table, so it chains. Returning
`false` from `callbackFn` stops early.

#### reduce()
```
fn reduce(tab: table, reductionFn?: fn initial: any = null, onNoGet: enum {throw, skip} = {throw}): any
```
**O(n).** Folds the table via `reductionFn` (`fn(carry, item): any`), starting from
`initial`. Returns the accumulated value — commonly a scalar.

#### keyCase()
```
fn keyCase(tab: table, uppercase: bool, asStream = false): table | stream
```
**O(n).** Upper- or lower-cases every key.

### 9.6 Reshape

#### values() · keys()
```
fn values(tab: table, onNoGet: enum {throw, skip} = {throw}, asStream = false): table | stream
fn keys(tab: table, asStream = false): table | stream
```
**O(n).** Values-only / keys-only, reindexed sequentially from 0. `keys` reads no
values and so takes no `onNoGet`.

#### column()
```
fn column(tab: table, column: string|int, newKeyColumn?: string|int,
          onNoGet: enum {throw, skip} = {throw}, asStream = false): table | stream
```
**O(n).** From a table of rows, extracts the value at key `column` from each row.
With `newKeyColumn` set, indexes the result by that column's value; otherwise
reindexes from 0.

#### flip()
```
fn flip(tab: table, onNoGet: enum {throw, skip} = {throw}, asStream = false): table | stream
```
**O(n).** Swaps keys and values; keys may be overwritten. Former values become keys
and are born with default permissions.

#### combine()
```
fn combine(keys: table|stream, values: table|stream, asStream = false): table | stream
```
**O(n).** Constructor-style (no `tab`): pairs the values of `keys` with the values
of `values` into a new table.

#### chunk()
```
fn chunk(tab: table, length: int, preserveKeys: bool = true,
         asStream = false): table | stream
```
**O(n).** Splits into successive sub-tables of `length` elements.
`preserveKeys = false` reindexes each chunk as a list.

#### groupBy()
```
fn groupBy(tab: table, keyFn?: fn mode: enum {values, keys, both} = {values},
           preserveKeys: bool = true,
           onNoGet: enum {throw, skip} = {throw}): table
```
**O(n).** Groups elements into `groupKey => table-of-members`. `keyFn`
(`fn(value): any`) computes each element's group; with it omitted, groups by value.

#### partition()
```
fn partition(tab: table, predicateFn: fn,
             mode: enum {values, keys, both} = {values},
             preserveKeys: bool = true,
             onNoGet: enum {throw, skip} = {throw}): table
```
**O(n).** Splits into `[passed, failed]` by `predicateFn` (`fn(value): bool`).

#### flatten()
```
fn flatten(tab: table, depth: int = -1, preserveKeys: bool = false,
           onNoGet: enum {throw, skip} = {throw}, asStream = false): table | stream
```
**O(n).** Flattens nested tables. `depth = -1` flattens fully. Reindexes by default,
since keys collide across levels.

### 9.7 Combine & compare

#### merge()
```
fn merge(tab: table, ...tabs: table|stream,
         *, recursive: bool = false, preserveKeys: bool = true,
         asStream = false): table | stream
```
**O(n).** Appends `tabs` onto `tab` in order. `preserveKeys = false` reindexes from
0. When `recursive`, keys shared between tables have their (table) values merged
recursively.

#### diff()
```
fn diff(tab: table, ...tabs: table,
        *, compareFn?: fn, mode: enum {values, keys, both} = {both},
        onNoGet: enum {throw, skip} = {throw}, asStream = false): table | stream
```
**O(n²).** Keeps elements of `tab` **not present in all** of `tabs`. Keys preserved.
Uses `==` unless `compareFn` (`fn(a, b): bool`) is set.

#### intersect()
```
fn intersect(tab: table, ...tabs: table,
             *, compareFn?: fn, mode: enum {values, keys, both} = {values},
             onNoGet: enum {throw, skip} = {throw}, asStream = false): table | stream
```
**O(n²).** Keeps elements of `tab` present in **all** of `tabs`.

#### distinct() · unique()
```
fn distinct(tab: table, compareFn?: fn, onNoGet: enum {throw, skip} = {throw}, asStream = false): table | stream
fn unique(tab: table, compareFn?: fn, onNoGet: enum {throw, skip} = {throw}, asStream = false): table | stream
```
**O(n).** Both drop duplicate values, first occurrence winning, using `==` unless
`compareFn` is set. They differ only in keying: `distinct` reindexes the result from
0, while `unique` preserves the original keys of the surviving elements.

#### replace()
```
fn replace(tab: table, ...replacements: table,
           *, recursive: bool = false, onNoSet: enum {throw, skip} = {throw}): table
```
**O(n).** Replaces values in `tab` by matching key against `replacements`. When
`recursive` and a matched value is a table, descends and replaces within it.

#### fill()
```
fn fill(tab: table, keys: table|stream, value: any,
        onNoSet: enum {throw, skip} = {throw}, asStream = false): table | stream
```
**O(n).** Sets `value` for each key in `keys` — only the *values* of `keys` are
used, so `keys` may be a stream such as `0..10`. Under `&`-write-back, overwriting
existing keys answers to freeze-state and any protocol `noSet` (via `onNoSet`); new
keys answer to open-state.

### 9.8 Order

#### sort()
```
fn sort(tab: table,
        mode: enum {values, keys, keyThenValue, valueThenKey} = {values},
        order: enum {ascending, descending} = {ascending},
        compareFn?: fn, combineFn?): table
```
**O(n·log n).** Quicksort. `mode` selects the sort operand; `keyThenValue` /
`valueThenKey` combine key and value via `combineFn`, or `+` if unset. `compareFn`
(`fn(a, b): int` — negative, zero, or positive) overrides the default comparison.

#### reverse()
```
fn reverse(tab: table): table
```
**O(n).** Reverses element order.

#### shuffle()
```
fn shuffle(tab: table, randFn?): table
```
**O(n).** Randomizes order.

#### random()
```
fn random(tab: table, num: int = 1, randFn?: fn preserveKeys: bool = true): table
```
**O(n).** Picks `num` random elements. `preserveKeys = false` reindexes from 0.

### 9.9 Segment

#### slice()
```
fn slice(tab: table, offset: any, length: int = 0,
         preserveKeys: bool = true, asStream = false): table | stream
```
**O(n).** A subsection starting at key `offset` for `length` elements.
`preserveKeys = false` reindexes from 0.

#### splice()
```
fn splice(tab: table, offset: any, length: int = 0, replacement: table = [],
          onNoSet: enum {throw, skip} = {throw}, asStream = false): table | stream
```
**O(n).** Removes a section and substitutes `replacement`. Under `&`-write-back, new
keys answer to open-state and overwritten values to freeze-state.

### 9.10 Grow

Each grow operation is legal in pure form (it builds a new, open table) and answers
to open-state only under `&`-write-back onto a sealed target.

#### prepend() · append()
```
fn prepend(tab: table, value: any, asStream = false): table | stream
fn append(tab: table, value: any, asStream = false): table | stream
```
**prepend O(n) · append O(1).** Add `value` at the front / back. `prepend` reindexes
a list; `append` is O(1) amortized.

#### insert()
```
fn insert(tab: table, key: any, value: any): table
```
**O(n).** Inserts `value` at `key`, shifting subsequent list elements. Sugar over
`splice`.

#### pad()
```
fn pad(tab: table, size: int, value: any): table
```
**O(n).** Grows the table to `size` elements with `value`. Negative `size` pads the
front.

### 9.11 Shrink

Removal is always legal, even on `closed` / `neverOpen` tables, since it introduces
no key. Each returns the shortened table (§4.1).

#### pop() · shift()
```
fn pop(tab: table): table
fn shift(tab: table): table
```
**pop O(1) · shift O(1) / O(n).** Remove the last / first element; read it via
`last()` / `first()` beforehand. `shift` is O(1) unless a list must be internally
converted.

#### unset()
```
fn unset(tab: table, key: any): table
```
**O(1) / O(n).** Removes `key`. O(1) for hashmap storage; O(n) if a list must
reindex.

#### remove()
```
fn remove(tab: table, value: any = null, key: any = null, compareFn?: fn,
          mode: enum {values, keys, both, either} = {values},
          all: bool = false, onNoGet: enum {throw, skip} = {throw}): table
```
**O(n).** Removes the first matching element, or every match with `all = true`. To
count removals, diff `count()` around the call.

#### clear()
```
fn clear(tab: table): table
```
**O(1).** Empties the table.

### 9.12 Aggregate

The numeric aggregates ignore values that are not `int` or `double`.

#### sum() · average() · product()
```
fn sum(tab: table, onNoGet: enum {throw, skip} = {throw}): int | double
fn average(tab: table, onNoGet: enum {throw, skip} = {throw}): int | double
fn product(tab: table, onNoGet: enum {throw, skip} = {throw}): int | double
```
**O(n).** Sum / mean / product of the numeric values.

#### min() · max()
```
fn min(tab: table, compareFn?: fn, onNoGet: enum {throw, skip} = {throw}): any
fn max(tab: table, compareFn?: fn, onNoGet: enum {throw, skip} = {throw}): any
```
**O(n).** Least / greatest value by standard comparison, or by `compareFn`
(`fn(a, b): int`). `undefined` if empty.

#### mode()
```
fn mode(tab: table, compareFn?: fn, onNoGet: enum {throw, skip} = {throw}): any
```
**O(n).** The most frequent value; ties resolve to the first occurrence.

#### join()
```
fn join(tab: table, glue: string = '', finalGlue?: string = null,
        onNoGet: enum {throw, skip} = {throw}): string
```
**O(n).** Concatenates values into a string separated by `glue`. `finalGlue` sets a
distinct separator before the last element (e.g. `"a, b and c"`). Non-strings are
coerced.
