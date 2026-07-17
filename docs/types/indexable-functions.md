# Indexable Functions

This is the operation catalogue for functions that take a **`table` only** — the complement
of **iterable-functions.md**, and like it a set of **built-in free functions**: in scope
everywhere, no import, no module, no protocol, called directly or through UFCS
(`tab.sort()` ≡ `sort(tab)`, functions §3.4). Concepts remain authoritative in **tables.md**.

An operation is indexable for one of two reasons, and the catalogue is organized by them:

1. **It needs the table itself** — keyed (random) access, the growth seal, storage shape, or
   positional mutation (§1–§3). A stream has none of these to offer.
2. **It must hold the entire input at once before producing its first output** — the sort
   family (§4). On a stream that would silently buffer an unbounded (possibly infinite)
   input, hiding exactly the cost `collect` exists to make visible
   (iterable-functions §1.4). So the stream spelling is explicit: `s.collect().sort()`.

Every function returns a new table (COW); mutation is caller-side write-back, `&tab.pop()`
(tables §4: `&` at the call site means "assign the return value back to me"; no function
takes a `&table` parameter). Signatures follow the conventions of iterable-functions §1.6,
including the deferral of the former `onNoGet` / `onNoSet` enums (table-api.md).

---

## 1. Keyed introspection

#### has()
```
fn has(tab: table, key: any): bool
```
**O(1).** Key presence. Distinct from `exists` (iterable-functions §2.3), which defaults to
an O(n) value scan.

#### canGet() · canSet()
```
fn canGet(tab: table, key: any): bool
fn canSet(tab: table, key: any): bool
```
**O(1).** Whether `key` may currently be read / written. False for a missing key or a key
the applying protocol does not grant `get` / `set` (tables §6). Grant semantics are
**pending the protocol redesign** (table-api.md); the signatures are settled.

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

---

## 2. Table-level growth seal

Each returns the modified table (COW); use `&tab.method(...)` to apply in place. Per-key
`get` / `set` access is not set here — it is declared by protocols (tables.md §6).

#### open() · close() · neverOpen()
```
fn open(tab: table): table
fn close(tab: table): table
fn neverOpen(tab: table): table
```
**O(1).** Set the growth axis: open, closed (revocable), or permanently non-growable.
`open()` on a `neverOpen` table raises `InvalidOpenError`.

(The mutation-seal functions `freeze` / `thaw` / `neverThaw` are **removed**, tables spec
§5.2: tables are value types, so value-sealing protected nothing. Immutability is `const` or
a future deferred `toImmutable()`.)

---

## 3. Keyed & positional mutation

Grow operations are legal in pure form (they build a new, open table) and answer to
open-state only under `&`-write-back onto a sealed target. Removal is always legal, even on
`closed` / `neverOpen` tables, since it introduces no key; removers return the shortened
table (tables §4.1) — read the element first (`last()`, `first()`), then shrink.

#### slice()
```
fn slice(tab: table, offset: any, length: int = 0, preserveKeys: bool = true): table
```
**O(n).** A subsection starting at key `offset` for `length` elements. `preserveKeys =
false` reindexes from 0. The keyed `offset` is what makes this indexable; the
traversal-order cuts are `take` / `skip` (iterable-functions §2.6).

#### splice()
```
fn splice(tab: table, offset: any, length: int = 0, replacement: table = []): table
```
**O(n).** Removes a section and substitutes `replacement`. Under `&`-write-back, new keys
answer to open-state.

#### insert()
```
fn insert(tab: table, key: any, value: any): table
```
**O(n).** Inserts `value` at `key`, shifting subsequent list elements. Sugar over `splice`.

#### pad()
```
fn pad(tab: table, size: int, value: any): table
```
**O(n).** Grows the table to `size` elements with `value`. Negative `size` pads the front.

#### fill()
```
fn fill(tab: table, keys: iterable, value: any): table
```
**O(n).** Sets `value` for each key in `keys` — only the *values* of `keys` are used, so
`keys` may be a stream such as `0..10` (and is taken, iterable-functions §1.5). The primary
is written by key, which is what makes `fill` indexable. Under `&`-write-back, new keys
answer to open-state.

#### pop() · shift()
```
fn pop(tab: table): table
fn shift(tab: table): table
```
**pop O(1) · shift O(1) / O(n).** Remove the last / first element; read it via `last()` /
`first()` beforehand. `shift` is O(1) unless a list must be internally converted. (On a
stream, the same shapes are traversal: `skip(1)` for the front; the back requires
consumption.)

#### unset()
```
fn unset(tab: table, key: any): table
```
**O(1) / O(n).** Removes `key`. O(1) for hashmap storage; O(n) if a list must reindex.

#### clear()
```
fn clear(tab: table): table
```
**O(1).** Empties the table.

---

## 4. The whole-input family

Each of these needs **every element before it can produce its first output** — a sort's
first element may be the input's last, a group is not complete until the input ends. Handing
them a stream would be a silent `collect` (iterable-functions §1.4), so they take a `table`
and a stream spells the retention: `s.collect().sort()`. The one order-family member that
escapes this rule is `random` (reservoir sampling, bounded memory), which is iterable
(iterable-functions §2.9). `reduce` remains the deliberate escape hatch for building
retained data from a stream by hand.

#### sort()
```
fn sort(tab: table,
        mode: enum {values, keys, keyThenValue, valueThenKey} = {values},
        order: enum {ascending, descending} = {ascending},
        compareFn?, combineFn?): table
```
**O(n·log n).** Quicksort. `mode` selects the sort operand; `keyThenValue` / `valueThenKey`
stand in place of `both` (sorting needs a primary key and a tiebreak; an unordered pair is
undefined for it) and combine key and value via `combineFn`, or `+` if unset. `compareFn`
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

#### groupBy()
```
fn groupBy(tab: table, keyFn?, mode: enum {values, keys, both} = {values},
           preserveKeys: bool = true): table
```
**O(n).** Groups elements into `groupKey => table-of-members`. `keyFn` (`fn(value): any`)
computes each element's group; with it omitted, groups by value. `mode` follows the
callback-transform family (iterable-functions §1.7).

#### partition()
```
fn partition(tab: table, predicateFn: fn, mode: enum {values, keys, both} = {values},
             preserveKeys: bool = true): table
```
**O(n).** Splits into `[passed, failed]` by `predicateFn` (`fn(value): bool`).

---

## 5. Error summary

| Error | Raised when |
|-|-|
| `OpenViolationError` | Adding a new key to a `closed` or `neverOpen` table. |
| `InvalidOpenError` | Calling `open()` on a `neverOpen` table. |
| `TableReadViolationError` · `TableMutationViolationError` | Per-key grant violations — **pending the protocol redesign** (table-api.md). |

Absence is **not** an error: reading a missing key yields `undefined`. See the *Optional
Access & Coalescing* reference for how `?.`, `??`, and `???` navigate absence, `null`, and
denial.
