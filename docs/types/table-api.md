# Table API

This is the operation catalogue for Luna tables: the built-in protocol every table
exposes, with signatures, time complexity, and a short description for each method.

The **concepts** these operations rely on are defined in **tables.md**, and that
document is authoritative for behavior: copy-on-write value semantics and
`&`-write-back (tables.md §4), table-level growth sealing (tables.md §5), element
`get` / `set` permissions declared by protocols (tables.md §6), and flag and access
propagation through transformers (tables.md §7). This document is the reference; read
tables.md to understand *why* tables behave as they do, and read this to look up *what*
a given operation does.

---

## 1. The built-in protocol

Every table exposes a built-in protocol of the operations below. Internally it is
represented **virtually**, in a memory-efficient manner; to the programmer it
behaves like any other protocol. All protocol members are readable and callable but
cannot be reassigned, you may call `tab.map(...)`, but not overwrite `tab.map`.

### 1.1 Reading the signatures

- **Receiver first.** Signatures begin `fn name(tab: table, …)`. The one
  constructor-style exception, `combine`, is noted at its entry.
- **Parameter order:** `tab`, required operands, function hooks
  (`transformFn` / `predicateFn` / `compareFn` / `keyFn`), `mode`, result-shaping
  flags (`preserveKeys`, `all`, `recursive`, `depth`), permission enums
  (`onNoGet` / `onNoSet`), then `asStream` last.
- **Keyword-only tail.** Where a method takes a `...tabs` variadic, the variadic is
  terminal and any options following it are keyword-only, written after a `*,`
  marker.
- **`asStream`** is present only on methods that can emit their first output element
  before consuming all input. Methods that must see everything first, or that return
  a scalar, return `table` or a scalar and never a `stream`.
- **Notation.** `name?` marks an optional parameter (default `null` unless shown).
  Hook parameters are of type `fn`. Every enum parameter is written out in full, 
  its complete member set followed by its default, so each synopsis stands alone.

### 1.2 The canonical `mode` enum

The base enum has three members with a uniform meaning, `values` puts values in
play, `keys` puts keys in play, `both` puts both in play:

```
mode: enum {values, keys, both} = {values}
```

Only two families extend the base set, each for a specific reason:

| Family | Members | `both` means |
|-|-|-|
| **Callback-transform**: `map`, `filter`, `each`, `every`, `some`, `partition`, `mapLeaves`, `groupBy` | `{values, keys, both}` | callback receives both operands: `fn(value, key)` |
| **Matcher**: `find`, `exists`, `remove` | `{values, keys, both, either}` | value **and** key must match |
| **Set-operation**: `diff`, `intersect`, `distinct`, `unique` | `{values, keys, both}` | equal iff key **and** value equal |
| **Sort**: `sort` | `{values, keys, keyThenValue, valueThenKey}` | *disallowed* |

- **`either`** belongs to the matcher family only: "match if value **or** key
  matches." It is coherent only for membership tests, so transforms and set-ops
  exclude it.
- **`keyThenValue` / `valueThenKey`** belong to `sort` only and stand in place of
  `both`. Sorting needs a primary key and a tiebreak; an unordered pair is undefined
  for it, so `both` is not offered.
- In the callback family, `both` always passes `(value, key)` in that order. The
  callback's *input* shape is constant across the family; only its *return*
  interpretation varies (`map` returns a `[key, value]` pair; predicate transforms
  return `bool`; `groupBy` returns the group key).
- `keyOf` is intentionally mode-less (fixed value → key); `find` in `{keys}` mode
  already covers key → value.

---

## 2. Protocol API

Each entry gives the signature, its time complexity, and a description. `X?` marks
an optional parameter.

### 2.1 Introspection

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
**O(1).** True iff the table is a list, incrementing `int` keys from 0.

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
missing key or a key the applying protocol does not grant `get`. `canSet` is false for
a missing key or a key the applying protocol does not grant `set`.

### 2.2 Table-level growth seal

Each returns the modified table (COW); use `&tab.method(...)` to apply in place.
Per-key `get` / `set` access is not set here, it is declared by protocols (tables.md §6).

#### open() · close() · neverOpen()
```
fn open(tab: table): table
fn close(tab: table): table
fn neverOpen(tab: table): table
```
**O(1).** Set the growth axis: open, closed (revocable), or permanently
non-growable. `open()` on a `neverOpen` table raises `InvalidOpenError`.

(The mutation-seal methods `freeze` / `thaw` / `neverThaw` are **removed**, tables spec §5.2:
tables are value types, so value-sealing protected nothing. Immutability is `const` or a future
deferred `toImmutable()`.)

### 2.3 Endpoints

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

### 2.4 Search & predicates

#### find()
```
fn find(tab: table, value: any = null, key: any = null, compareFn?,
        mode: enum {values, keys, both, either} = {values},
        onNoGet: enum {throw, skip} = {throw}): any
```
**O(n).** The first value satisfying the match; `undefined` if none. With no
operands, returns the first value. Uses `==` unless `compareFn` (`fn(a, b): bool`)
is set.

#### keyOf()
```
fn keyOf(tab: table, value: any, compareFn?,
         all: bool = false, onNoGet: enum {throw, skip} = {throw}): any
```
**O(n).** The first key whose value matches `value`; `undefined` if none. With
`all = true`, returns a table of every matching key. The key-returning complement of
`find`.

#### exists()
```
fn exists(tab: table, value: any = null, key: any = null, compareFn?,
          mode: enum {values, keys, both, either} = {values},
          onNoGet: enum {throw, skip} = {throw}): bool
```
**O(1) / O(n).** True if at least one value / key / either / both matches. O(1) when
matching keys only; O(n) otherwise. Use for equality tests; use `some` for predicate
tests.

#### some() · every()
```
fn some(tab: table, predicateFn?, mode: enum {values, keys, both} = {values}, onNoGet: enum {throw, skip} = {throw}): bool
fn every(tab: table, predicateFn?, mode: enum {values, keys, both} = {values}, onNoGet: enum {throw, skip} = {throw}): bool
```
**O(n).** Whether any / all elements satisfy `predicateFn` (`fn(x): bool`). With
`predicateFn` omitted, tests truthiness. Both short-circuit.

### 2.5 Transform

#### map()
```
fn map(tab: table, transformFn?, mode: enum {values, keys, both} = {values},
       onNoGet: enum {throw, skip} = {throw}, asStream = false): table | stream
```
**O(n).** Transforms each value via `transformFn` (`fn(value): any`). In `{keys}`
mode keys are passed; in `{both}`, `fn(value, key): [key, value]`. In `{keys}` /
`{both}`, a returned key that is `null` / `undefined` is skipped.

#### filter()
```
fn filter(tab: table, predicateFn?, mode: enum {values, keys, both} = {values},
          onNoGet: enum {throw, skip} = {throw}, asStream = false): table | stream
```
**O(n).** Keeps elements where `predicateFn` (`fn(value): bool`) returns true.
`{keys}` passes keys; `{both}` passes `fn(value, key): bool`. Keys are preserved.

#### mapLeaves()
```
fn mapLeaves(tab: table, transformFn?, mode: enum {values, keys, both} = {values}, onNoGet: enum {throw, skip} = {throw}): table
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
fn reduce(tab: table, reductionFn?, initial: any = null, onNoGet: enum {throw, skip} = {throw}): any
```
**O(n).** Folds the table via `reductionFn` (`fn(carry, item): any`), starting from
`initial`. Returns the accumulated value, commonly a scalar.

#### keyCase()
```
fn keyCase(tab: table, uppercase: bool, asStream = false): table | stream
```
**O(n).** Upper- or lower-cases every key.

### 2.6 Reshape

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
fn groupBy(tab: table, keyFn?, mode: enum {values, keys, both} = {values},
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

### 2.7 Combine & compare

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
        *, compareFn?, mode: enum {values, keys, both} = {both},
        onNoGet: enum {throw, skip} = {throw}, asStream = false): table | stream
```
**O(n²).** Keeps elements of `tab` **not present in all** of `tabs`. Keys preserved.
Uses `==` unless `compareFn` (`fn(a, b): bool`) is set.

#### intersect()
```
fn intersect(tab: table, ...tabs: table,
             *, compareFn?, mode: enum {values, keys, both} = {values},
             onNoGet: enum {throw, skip} = {throw}, asStream = false): table | stream
```
**O(n²).** Keeps elements of `tab` present in **all** of `tabs`.

#### distinct() · unique()
```
fn distinct(tab: table, compareFn?, onNoGet: enum {throw, skip} = {throw}, asStream = false): table | stream
fn unique(tab: table, compareFn?, onNoGet: enum {throw, skip} = {throw}, asStream = false): table | stream
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
**O(n).** Sets `value` for each key in `keys`, only the *values* of `keys` are
used, so `keys` may be a stream such as `0..10`. Under `&`-write-back, overwriting
existing keys answers to the key's protocol `set` grant (via `onNoSet`); new
keys answer to open-state.

### 2.8 Order

#### sort()
```
fn sort(tab: table,
        mode: enum {values, keys, keyThenValue, valueThenKey} = {values},
        order: enum {ascending, descending} = {ascending},
        compareFn?, combineFn?): table
```
**O(n·log n).** Quicksort. `mode` selects the sort operand; `keyThenValue` /
`valueThenKey` combine key and value via `combineFn`, or `+` if unset. `compareFn`
(`fn(a, b): int`, negative, zero, or positive) overrides the default comparison.

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
fn random(tab: table, num: int = 1, randFn?, preserveKeys: bool = true): table
```
**O(n).** Picks `num` random elements. `preserveKeys = false` reindexes from 0.

### 2.9 Segment

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
keys answer to open-state and overwritten values to the key's protocol `set` grant.

### 2.10 Grow

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

### 2.11 Shrink

Removal is always legal, even on `closed` / `neverOpen` tables, since it introduces
no key. Each returns the shortened table (removers return the shortened table,
tables.md §4.1).

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
fn remove(tab: table, value: any = null, key: any = null, compareFn?,
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

### 2.12 Aggregate

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
fn min(tab: table, compareFn?, onNoGet: enum {throw, skip} = {throw}): any
fn max(tab: table, compareFn?, onNoGet: enum {throw, skip} = {throw}): any
```
**O(n).** Least / greatest value by standard comparison, or by `compareFn`
(`fn(a, b): int`). `undefined` if empty.

#### mode()
```
fn mode(tab: table, compareFn?, onNoGet: enum {throw, skip} = {throw}): any
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

---

## 3. Error summary

| Error | Raised when |
|-|-|
| `OpenViolationError` | Adding a new key to a `closed` or `neverOpen` table. |
| `InvalidOpenError` | Calling `open()` on a `neverOpen` table. |
| `TableReadViolationError` | Reading a key the applying protocol does not grant `get`, including bulk reads under `onNoGet = throw`. |
| `TableMutationViolationError` | Writing a key the applying protocol does not grant `set`, including bulk writes under `onNoSet = throw`. |

Absence is **not** an error: reading a missing key yields `undefined`. See the
*Optional Access & Coalescing* reference for how `?.`, `??`, and `???` navigate
absence, `null`, and denial.
