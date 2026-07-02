# Tables — Language & Protocol Reference

Tables are Luna's primary general-purpose data structure. A single type serves as
array, hashmap, and object. They are high-performance, flexible, and carry a
built-in protocol of operations described in this document.

---

## 1. Overview

A table maps **keys** to **values**. Depending on how it is used it behaves as a
contiguous array, a hashmap, or a record/object — with no change in type and
minimal overhead versus a raw array.

```
var myTable = [];                 // empty table — always []
myTable = [
  name => 'Lucas',
  lastName => 'Streanga',
];                                // record / object style
myTable = [0, 1, 2, 3, 4];        // array / list style
```

Empty tables are always written `[]`. Elements are given as `value` (implicit
incrementing integer keys) or `key => value`.

---

## 2. Lists & contiguous memory

When a table has **exclusively incrementing integer keys beginning at 0**, it is a
**list**. Lists are stored internally as a contiguous block of memory (an array).

- The moment a non-integer key is introduced, the table converts internally to a
  hashmap.
- Removing elements, or adding under new integer keys, may leave the table
  contiguous even when it is no longer a strict list.

Two O(1) predicates report internal shape:

- `isList()` — true iff the table is a list (incrementing int keys from 0).
- `isContiguousMemory()` — true iff stored contiguously. All lists are
  contiguous; some non-list tables are too. **Any string key ⇒ never contiguous.**

---

## 3. Table keys

Permitted key types:

| Key type | Handling |
|-|-|
| `int` | Native. |
| `string` | Native. |
| `double` | Converted to `string`. |
| `bool` | Converted to `int` (`false` → 0, `true` → 1). |
| `table` | Must be stringified first, typically via `toJson()`. |

---

## 4. Value semantics: copy-on-write, references, and write-back

**Tables are copy-on-write value types.** Assignment shares storage until a write
forces a split; logically, every table is an independent value.

Consequently **the protocol never mutates its receiver in place.** Every operation
returns a *new* table (COW), and the caller decides whether to keep it or write it
back. Write-back uses a caller-side reference:

```
var sorted = myTable.sort();      // myTable unchanged; sorted is a new table
&myTable.sort();                  // write-back: myTable becomes the sorted result
```

No protocol method takes a `&table` parameter. The receiver is always passed by
value; `&` on the *call site* means "assign the return value back to me."

### 4.1 Removers return the shortened table

Because a method's return value *is* the intended new table, operations that would
otherwise return a removed element instead return the **shortened table**. Read the
element first, then remove:

```
var x = myTable.last();           // read the element
&myTable.pop();                   // then shrink in place
```

This keeps `&`-write-back meaning one simple thing everywhere — "assign the return
value back" — with no method whose return value is something other than a table.
`pop`, `shift`, `unset`, `remove`, and `clear` all return tables. To learn how many
elements `remove` deleted, diff `count()` around the call (both O(1)).

### 4.2 `&`-write-back is a flag-respecting structural update

Write-back is **not** a blind rebind. It applies the result to the target while
respecting the target's current structural and permission flags, so open/close and
`noSet` retain their force under references:

```
var b = closedTab.prepend(0);     // OK — b is a new, open table; closedTab untouched
&closedTab.prepend(0);            // throws OpenViolationError — write-back would grow a closed table
```

---

## 5. Table-level seals: growth and mutation

Two orthogonal table-level flags gate structural change. **Growth** governs whether
*new keys* may be added; **mutation** governs whether *existing values* may be
changed. Neither governs reading or removal, and the two are independent — a table
may be sealed on either axis, both, or neither.

### 5.1 Growth: open, closed, neverOpen

| State | Add new key? | Reached via | Reversible? |
|-|-|-|-|
| `open` | yes | `open()` | — |
| `closed` | no → `OpenViolationError` | `close()` | yes, via `open()` |
| `neverOpen` | no → `OpenViolationError` | `neverOpen()` | **no, permanently** |

```
var myTable = [];
myTable.name = 'Lucas';           // open: OK
&myTable.close();
myTable.lastName = 'Streanga';    // OpenViolationError: cannot add to a closed table
&myTable.open();
myTable.lastName = 'Streanga';    // OK
&myTable.neverOpen();
myTable.age = 0;                  // OpenViolationError
&myTable.open();                  // InvalidOpenError: cannot open a table set neverOpen
```

`close()` is the **revocable** seal — you keep the key and may `open()` later.
`neverOpen()` is the **irrevocable** seal — the key is discarded, and `open()` on
such a table raises `InvalidOpenError`. Irrevocability is the whole point: code
holding a `neverOpen` table may permanently rely on its key-set being fixed — cache
the shape, skip existence checks, assume no growth. A reopen operation would destroy
that guarantee, so it does not exist. The only path from `neverOpen` back to
growable is to **derive a fresh, open table** holding the same values (§7.1) —
honest construction of a new value, not unsealing of the old one.

### 5.2 Mutation: frozen, neverThaw

Freezing is the mirror of closing, one axis over: it seals *existing values* rather
than the *key-set*.

| State | Change existing value? | Reached via | Reversible? |
|-|-|-|-|
| unfrozen | yes | `thaw()` | — |
| `frozen` | no → `FreezeViolationError` | `freeze()` | yes, via `thaw()` |
| `neverThaw` | no → `FreezeViolationError` | `neverThaw()` | **no, permanently** |

```
var config = [host => 'localhost', port => 8080];
&config.freeze();
config.port = 9090;               // FreezeViolationError: table is frozen
&config.thaw();
config.port = 9090;               // OK
&config.neverThaw();
config.port = 80;                 // FreezeViolationError
&config.thaw();                   // InvalidThawError: cannot thaw a table set neverThaw
```

`freeze()` / `neverThaw()` parallel `close()` / `neverOpen()` exactly — a revocable
and an irrevocable seal, with `thaw()` as the counterpart to `open()`. `neverThaw`
carries the same reliance guarantee: a permanently frozen table's values may be
cached, hashed, and shared without defensive copying, because they can never change.

### 5.3 The two axes are orthogonal

Growth seals the key-set; freezing seals the values. Neither implies the other, and
all four combinations are useful:

| Growth | Mutation | Behaves as |
|-|-|-|
| open | unfrozen | fully mutable (default) |
| closed / neverOpen | unfrozen | fixed-shape record — set keys, mutable fields |
| open | frozen / neverThaw | append-only log — immutable history, new entries allowed |
| closed / neverOpen | frozen / neverThaw | fully immutable snapshot |

This is why `neverOpen` is **not** immutable: it renounces *growth* only. Existing
values remain writable, and elements may still be read and removed. Immutability of
*values* is `freeze`'s job; a fully immutable table is sealed on both axes.

### 5.4 What each seal governs

**Growth-attempting operations** answer to open-state — they introduce a key:

- Direct assignment to a new key: `tab.newKey = x`, `tab[k] = x`
- `prepend`, `append`, `insert`, `pad`, `fill` (for new keys), and `splice` when the
  replacement introduces keys beyond those it removed
- The write-side coalescer `??=` when firing on an absent key

**Mutation operations** answer to freeze-state — they overwrite an existing value:

- Direct assignment to an existing key: `tab[existingKey] = x`
- `fill` / `replace` / `splice` where they overwrite existing keys
- The write-side coalescer `???=` when firing on a present `null`

In their **pure** form all of these are legal — they build a new table, born open
and unfrozen (§7.1). They throw only under `&`-write-back onto a target sealed on the
relevant axis. It is the caller's responsibility to `open()` or `thaw()` first.

---

## 6. Element permissions: get / set

Independently of the table-level seals, **each individual key** may carry read and
write permissions: `get` (readable), `set` (writable), both, or neither.

| Endpoint state | Read | Write |
|-|-|-|
| get + set (default) | yes | yes |
| `noSet` | yes | `TableMutationViolationError` |
| `noGet` | `TableReadViolationError` | yes |
| neither | error | error |

**Permissions are declared by protocols, not set ad-hoc.** A bare table literal is
full get + set on every key; there are no runtime methods to make an individual key
private or read-only. Heterogeneous per-key permissions — "this field is private,
that one is read-only" — describe a *type contract*, which is exactly what a protocol
expresses. A table therefore acquires `noGet` / `noSet` keys only by conforming to a
protocol that declares them. To seal an *entire* table's values instead, use the
table-level `freeze` (§5.2).

`noSet` and `freeze` both forbid writes but at different scopes: `noSet` is one
protocol-declared key; `freeze` is every key at once. On a key that is both, the
table-level `freeze` check runs first, so the error is `FreezeViolationError`.

### 6.1 Absence, `null`, and denial are three different things

- **Absent key** → reading yields `undefined`. Absence never throws; it is routine
  control flow for a table-as-hashmap. Because `undefined` is unstorable, *present*
  keys never hold it — existence ⟺ not-`undefined`.
- **Present, `null`** → yields the stored `null`. A real, deliberate value.
- **Present, `noGet`** → raises `TableReadViolationError`. A *permission* failure,
  not a value.

These stay distinct throughout the protocol and through the access operators
(`?.`, `??`, `???` — see the separate *Optional Access & Coalescing* reference).
`has()` answers existence in O(1); `canGet()` / `canSet()` answer "may I read /
write this right now?" in O(1) — `canSet` accounts for `freeze` as well as `noSet`.
On a **missing** key both return `false`, and `has()` disambiguates absence from
denial.

### 6.2 Bulk operations and permissions: `onNoGet` / `onNoSet`

A method that **reads element values** may encounter a protocol-declared `noGet`
key; one that **writes to existing keys** may encounter a `noSet` key. Behavior is
controlled by a two-member enum, defaulting to `throw`:

```
onNoGet: enum {throw, skip} = {throw}    // on value-reading operations
onNoSet: enum {throw, skip} = {throw}    // on operations that overwrite existing keys
```

- `throw` — raise `TableReadViolationError` / `TableMutationViolationError`.
- `skip` — silently omit the offending element from the operation.

Placement follows a rule rather than per-method choice: **any method that reads
values takes `onNoGet`; any method that overwrites existing keys takes `onNoSet`.**
Keys-only methods (`keys`, `has`, `canGet`, `keyFirst`, `keyLast`) take neither.
`skip` suppresses only the *per-key permission* refusal — never a table-level
`FreezeViolationError` or `OpenViolationError`, which are separate axes. Because
permissions are fixed by protocol, `skip` never alters them; it merely omits the
offending element from this one operation.

---

## 7. Flag & permission propagation

Tables carry two kinds of flags with **different** propagation rules, split along
the *whose-invariant-is-it* seam.

### 7.1 Structural flags do NOT propagate through transformers

`open` / `close` / `neverOpen` **and** `frozen` / `neverThaw` are properties a
table asserts about **itself**. A table *derived* by a transformer (`map`, `filter`,
`merge`, `prepend`, …) is a new self and re-decides: **derived tables are born `open`
and unfrozen.**

- **Identity copy** (`var b = a;`) — same logical value; keeps `a`'s seals on both
  axes, including `neverOpen` and `neverThaw`. Under COW, the first growth or
  mutation attempt on `b` throws before any storage split.
- **Derived / transformed table** — a *different* value with different contents; born
  `open` and unfrozen regardless of the source's state.

This is what makes deriving a fresh table the honest escape from either irrevocable
seal (§5.1–5.2): `sealedTab.map(x => x)` yields an open, unfrozen table with the same
values, without unsealing anything.

### 7.2 Permission flags DO propagate — by key identity

`noGet` / `noSet` are invariants a **protocol** asserts about any conforming table.
If a transformer dropped them, they would be properties of "a table nobody has
`.map()`-ed yet" — worthless as type invariants. So they must survive transforms.
The precise rule:

> **`noGet` / `noSet` propagate to any output key that is identically an input
> key. Freshly-created keys are born default (get + set).**

- **Key-preserving** operations (`filter`, `map` in `{values}` mode, `reverse`,
  `sort`, `slice` with `preserveKeys`, `replace`, `fill`) carry each surviving
  key's permissions forward.
- **Key-creating** operations (`values`, `keys`, `map` in `{keys}` / `{both}` mode,
  `flip`, `groupBy`, `flatten`, anything reindexing to `0..n`) mint keys that did
  not exist in the input; those keys are born default. `flip` is the sharp case — a
  `noGet` *value* becoming a *key* cannot stay `noGet`; the concept does not
  transfer.

Protocol membership is naturally shed where it cannot apply: a list, for instance,
never carries protocols beyond the built-in one.

Under `&`-write-back the **target's** current permissions govern the write. Because
key-preserving transforms carry permissions along, source and target normally agree;
they diverge only when the target conforms to a protocol the source did not, in which
case the target's contract wins.

---

## 8. The built-in protocol

Every table exposes a built-in protocol of the operations below. Internally it is
represented **virtually**, in a memory-efficient manner; to the programmer it
behaves like any other protocol. All protocol members are readable and callable but
cannot be reassigned — you may call `tab.map(...)`, but not overwrite `tab.map`.

### 8.1 Reading the signatures

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
  Hook parameters are of type `fn`. Every enum parameter is written out in full —
  its complete member set followed by its default — so each synopsis stands alone.

### 8.2 The canonical `mode` enum

The base enum has three members with a uniform meaning — `values` puts values in
play, `keys` puts keys in play, `both` puts both in play:

```
mode: enum {values, keys, both} = {values}
```

Only two families extend the base set, each for a specific reason:

| Family | Members | `both` means |
|-|-|-|
| **Callback-transform** — `map`, `filter`, `each`, `every`, `some`, `partition`, `mapLeaves`, `groupBy` | `{values, keys, both}` | callback receives both operands: `fn(value, key)` |
| **Matcher** — `find`, `exists`, `remove` | `{values, keys, both, either}` | value **and** key must match |
| **Set-operation** — `diff`, `intersect`, `distinct`, `unique` | `{values, keys, both}` | equal iff key **and** value equal |
| **Sort** — `sort` | `{values, keys, keyThenValue, valueThenKey}` | *disallowed* |

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

## 10. Error summary

| Error | Raised when |
|-|-|
| `OpenViolationError` | Adding a new key to a `closed` or `neverOpen` table. |
| `InvalidOpenError` | Calling `open()` on a `neverOpen` table. |
| `FreezeViolationError` | Writing an existing key on a `frozen` or `neverThaw` table. |
| `InvalidThawError` | Calling `thaw()` on a `neverThaw` table. |
| `TableReadViolationError` | Reading a protocol `noGet` key, including bulk reads under `onNoGet = throw`. |
| `TableMutationViolationError` | Writing a protocol `noSet` key, including bulk writes under `onNoSet = throw`. |

Absence is **not** an error: reading a missing key yields `undefined`. See the
*Optional Access & Coalescing* reference for how `?.`, `??`, and `???` navigate
absence, `null`, and denial.
