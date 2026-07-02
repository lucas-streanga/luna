# Spread

Spread (`...`) expands one table's entries into a table literal, so tables can be built by
combining others. It is the building counterpart to destructuring (which takes tables apart)
and, for keyed data, is equivalent to `merge`. The semantics follow PHP: entries are spread in
order, and later entries can overwrite earlier ones, with one precise rule separating integer
keys (which reindex) from string keys (which overwrite).

---

## 1. Spread in a table literal

`...expr` inside a table literal inserts all of `expr`'s entries at that point:

```
let combined = [...first, ...second];
let withExtra = [head, ...middle, tail];      // spreads may sit among literal elements
```

Spread is processed **left to right**, folding each spread element (and each literal element)
into the result in source order. The result's contents and order are exactly what that fold
produces (§2).

---

## 2. The fold: integer keys reindex, string keys merge

Spread folds left to right, maintaining a result table and a running **next integer index**.
For each spread element, in that element's own iteration order:

- **An integer-keyed value reindexes:** it is appended at the running next index, which then
  increments. Integer keys from the source are *not* preserved; they are renumbered onto the
  running counter. So integer keys never collide and never overwrite, they always append.
- **A string-keyed value merges:** it is written at its own key, overwriting any existing value
  at that key. String keys keep their identity, so a later spread's `'x'` overwrites an earlier
  `'x'`.

Literal elements between spreads follow the same rule: a bare element (`head`) appends at the
next integer index; a `'k' => v` element merges by key.

So the two behaviors are: **integer keys reindex-and-append (never overwrite); string keys
merge-and-overwrite.** This is the whole rule, and it makes spread order-simple: there is no
"list mode" versus "map mode" that changes later processing; each element is folded by the same
rule, and the result's shape is emergent.

### 2.1 Worked example

```
first  = [10, 20]              // {0=>10, 1=>20}
second = [30]                  // {0=>30}
mixed  = {'x'=>99, 5=>50}      // a string key and a non-contiguous integer key
third  = [40, 50]              // {0=>40, 1=>50}

[...first, ...second, ...mixed, ...third]
```

Folding left to right (next index in brackets):

1. `...first` [0]: append 10, 20 to `{0=>10, 1=>20}`. Next index 2.
2. `...second` [2]: append 30 to `{0=>10, 1=>20, 2=>30}`. Next index 3.
3. `...mixed` [3]: string `'x'=>99` merges; integer key 5 **reindexes** to index 3 (`3=>50`).
   Result `{0=>10, 1=>20, 2=>30, 'x'=>99, 3=>50}`. Next index 4.
4. `...third` [4]: append 40, 50 at indices 4, 5. Next index 6.

Final: `{0=>10, 1=>20, 2=>30, 'x'=>99, 3=>50, 4=>40, 5=>50}`.

Two things to note. `third` does **not** overwrite earlier integer keys; it *continues
appending* at the running index (4, 5), because integer keys always reindex to the next slot
and never collide. And the interleaved string key `'x'` does not consume an integer index, so
the integer keys stay contiguous (`0..5`) in insertion order with `'x'` sitting between them.

---

## 3. List-ness of the result

A table is a **list** iff its keys are exactly `0..n-1` and nothing else (tables). Spread
preserves or breaks this:

- If **every** spread and literal element contributes only integer keys, the result stays a
  list (integer keys reindexed contiguously from zero).
- As soon as **any** element contributes a string key, the result is a general **table**, not a
  list, even though its integer keys remain contiguous. In the worked example, the result has
  `'x'`, so `isList()` is false, it is a valid table with mixed keys, just not a list.

List-ness is emergent from the fold, not a mode that alters it: later integer-keyed spreads
still append normally whether or not a string key already broke list-ness. Because list-ness is
a maintained O(1) property of a table (tables), the fold can check "is this element a list?"
cheaply per element, though it does not need to: it applies the same integer-reindex /
string-merge rule regardless.

---

## 4. Equivalence to `merge`

For keyed data, spread is `merge`:

```
let m = [...a, ...b];        // equivalent to a.merge(b) for string keys
```

`a.merge(b)` layers `b`'s entries over `a`'s: string keys in `b` overwrite `a`'s, and (matching
the spread rule) integer keys append rather than collide. So `[...a, ...b]` and `a.merge(b)`
describe the same fold. Spread is the literal-syntax form; `merge` is the function form; they
agree.

For purely list operands, spread is **concatenation**: `[...list1, ...list2]` appends
`list2`'s values after `list1`'s (all integer keys reindex), producing a list of their combined
length, the list-append operation you would expect.

---

## 5. Comptime spread

When the spread operands are comptime-known, the whole literal folds at compile time:

```
const combined = [...tab1, ...tab2];      // requires tab1, tab2 comptime-known; folds at compile time
```

A `const` table literal whose spread operands are themselves `const` (or otherwise
comptime-known) is comptime-evaluable (functions §5), because the fold is a pure computation.
It folds into a `const` table at compile time, which then takes the const-table representation
(tables, Amendment A), and a `const` result that is a list takes the tighter list
representation. Nothing new is needed; comptime spread is ordinary const-table construction
whose parts happen to be spreads.

---

## 6. Summary of rules

- **Left-to-right fold.** Spread and literal elements are processed in source order.
- **Integer keys reindex-and-append** at a running next index; they never collide or overwrite.
- **String keys merge-and-overwrite** by key; a later value at a key replaces an earlier one.
- **Result is a list** iff only integer keys were contributed; the first string key makes it a
  general table (integer keys still append normally after).
- **Equivalent to `merge`** for keyed data, and to **concatenation** for lists.
- **Comptime-foldable** when the operands are comptime-known, producing a `const` table.

---

## 7. Open questions

- **Spread of non-table values:** whether `...` may spread a `stream` (spreading a lazy
  sequence into a literal), pending the stream spec; currently spread is over tables.
- **Nested spread depth:** spread inserts a table's *entries* (one level); whether any
  deep-flattening form exists is out of scope (use an explicit flatten).
- **Spread in call arguments:** whether `...` spreads a list into function arguments
  (`f(...args)`) as well as into table literals, related to the command spec's argument spread
  (`${...flags}`), pending the call grammar.
