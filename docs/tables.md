# Tables

This document defines the concepts behind Luna's table type: keys and how they are
accessed (§3), value semantics (§4), the growth and mutation seals (§5), element
permissions (§6), and flag propagation (§7). The built-in operations that act on tables
are catalogued separately in **table-protocol-api.md**. Behavior beyond the built-in
operations is contributed by protocols: a table is also Luna's object, an empty or
data-bearing table with protocols applied. Protocols and the `->` meta operator are
introduced in §3.3 and specified in **protocols** and **views**.

---

## 1. Tables Overview

A table maps **keys** to **values**. Depending on how it is used it behaves as a
contiguous array, a hashmap, or a record/object, with no change in type and
minimal overhead versus a raw array.

```
var myTable = [];                 // empty table, always []
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

Two O(1) predicates report internal shape (both defined in
**table-protocol-api.md §1.1**):

- `isList()`, true iff the table is a list (incrementing int keys from 0).
- `isContiguousMemory()`, true iff stored contiguously. All lists are
  contiguous; some non-list tables are too. **Any string key ⇒ never contiguous.**

### 2.1 `list` is a type: a refinement of `table`

`list` is a **type**, usable in annotations and signatures, not merely an internal shape.
It is a **refinement (subtype) of `table`**: a `list` is exactly a table whose keys are
`{0, 1, ..., n-1}` and nothing else. So:

```
list <: table          // every list is a table; not every table is a list
```

This is the same is-a relationship `IOError` has to `error` or `reveal` to `capability`. A
`list` is usable **anywhere a `table` is expected** with no ceremony (widening is implicit,
`as` spec); the reverse, treating a `table` as a `list`, is a narrowing (§2.3).

The empty table `[]` is the empty list (zero keys vacuously satisfies "keys are `0..n-1`"),
so `let xs: list = []` is statically a list. A table literal whose shape is visibly a list
(`[1, 2, 3]`) is statically `list`; one with a string key (`['x' => 1]`) is not, and
assigning it to a `list` binding is a compile error.

### 2.2 List-ness is a maintained, O(1) property

A table's list-ness is **maintained**, not recomputed: the table carries it as a cheap
derived property, so `isList()` is O(1), never an O(n) key scan. Operations preserve or
break it predictably:

- **Append at the next index** (setting key `n` on an `n`-element list): stays a list.
- **Update an existing in-range integer key** (value change): stays a list.
- **Set an integer key beyond the next index** (creating a gap), or **set/merge any string
  key**: becomes a general table (no longer a list).
- **Remove a key** (§4.1): becomes a general table if the removal leaves a gap. Removing the
  last element keeps it a list; removing a middle key leaves a gap, so the table is no
  longer a list. **Removal does not silently reindex.**

Because list-ness is maintained, spread (spread spec) and the const-table representation
(Amendment A) can rely on it cheaply.

### 2.3 Narrowing a `table` to a `list`, and re-compacting

A value typed `table` is narrowed to `list` by the checked operators (`as` / `is` specs):

- **`tab as list`**: asserts the table is currently a list; a `TypeError` (panic) if it is
  not (has a gap or a string key).
- **`tab is list`**: the boolean test that also narrows `tab` to `list` in the taken branch.

Removal leaves gaps rather than reindexing (§2.2), so a table with gaps is genuinely not a
list. To **re-compact** a gapped or keyed table into a fresh contiguous list, call
`values()` (table-protocol-api), which reindexes the values to `0..m-1` and returns a
`list`. So `values()` is the explicit "make this a list again" operation; nothing reindexes
silently.

**`as list` asserts; `values()` produces.** These are not interchangeable, they answer
different questions:

- **`tab as list`** *asserts* the table is **already** a list. It runs a check and raises a
  `TypeError` (panic) if `tab` has a gap or a string key; on success the same value is
  re-typed, unchanged. Use it when a non-list is a bug you want caught.
- **`tab.values()`** *produces* a fresh list by reindexing, so it **always** succeeds
  whatever `tab`'s shape. Use it when you want a list out of any table.

They even differ in result: `{5=>'a', 9=>'b'} as list` **panics** (not contiguous), while
`{5=>'a', 9=>'b'}.values()` returns `['a', 'b']` (drops the keys, reindexes the values). And
because narrowing is never implicit, passing a bare `table` where a `list` is required is a
compile error; you choose `as list` (assert) or `values()` (produce) explicitly:

```
someFn(tab)             // COMPILE ERROR: table is not implicitly a list
someFn(tab as list)     // asserts tab is currently a list; TypeError panic if not
someFn(tab.values())    // always legal; builds a fresh list from tab's values
```

### 2.4 `list` in signatures

Operations that are guaranteed to produce a contiguous-from-zero result are typed to return
`list`, not `table`, so callers get the list guarantee statically (and can index `0..n-1`,
pass to list-requiring functions, and destructure positionally without a runtime check). A
`const` list additionally takes the tightest representation (a pure contiguous array with no
stored keys, since the keys are implicitly `0..n-1`; Amendment A).

---

## 3. Keys and access

### 3.1 Key types

Permitted key types:

| Key type | Handling |
|-|-|
| `int` | Native. |
| `string` | Native. |
| `double` | Converted to `string`. |
| `bool` | Converted to `int` (`false` → 0, `true` → 1). |
| `table` | Must be stringified first, typically via `toJson()`. |

### 3.2 Static keys (`.`) vs. dynamic keys (`[]`)

Element access comes in two forms, and the distinction is **global**, it holds
everywhere a table is accessed, not only inside protocol `apply` bodies:

- **`tab.name` is a static key.** The key is the literal identifier `name`, known at
  compile time. There is no `.dynamicName`; `tab.someVar` does **not** mean "the key
  held in `someVar`", it means the literal key `someVar`. `.` is only ever a
  compile-time-known name.
- **`tab[expr]` is a dynamic key.** The key is the runtime value of `expr`. This is the
  form for integer keys (`tab[0]`), non-identifier string keys (`tab['has space']`),
  and any computed key (`tab[k]`, `tab['row' . i]`).

`tab.name` is exactly `tab['name']` when `name` is an identifier; `.` is sugar for the
static-string case. Keys that are not identifier-shaped (integers, strings with spaces
or leading digits, anything computed) must use `[]`. Both forms read and write the same
element space: `tab.name` and `tab['name']` are the same slot.

This split is deliberately syntactic, and it is what lets the compiler tell a static
write from a dynamic one by inspecting a single AST node (a `.member` target is a known
name; a `[expr]` target is computed), with no dataflow analysis. The protocol system
relies on exactly this to distinguish declared from dynamic member installation during
`apply` (see **protocols**, declared vs. dynamic members); the rule is stated globally
here because it governs all element access, and `apply` is just one place it matters.

A `.` or `[]` read of a missing key yields `undefined` (§6.1), which coalesces with
`??` / `?.` like any other absence.

### 3.3 Element space (`.` / `[]`) vs. meta space (`->`)

`.` and `[]` reach **element space**: the table's own keyed data. A third operator,
`->`, reaches **meta space**: the behavior contributed by the protocols a table wears.
The two spaces are disjoint, and the operators never overlap:

- **`tab.name` / `tab['name']`** , element (data). Static or dynamic key. A miss is
  `undefined`. Assignable.
- **`tab->name(...)`** , a call to the built-in protocol's meta function (the built-in
  protocol is nameless, so it is reached by bare `->`). The table operation catalogue
  (`map`, `filter`, `count`, `pop`, and the rest) lives here, in
  **table-protocol-api.md**.
- **`tab->protoName`** , a view of a named applied protocol, or `undefined` if the
  table does not wear it. Meta functions are reached through the view with `.`.

Because element space and meta space use different operators, a data key can share a
name with a meta function without ambiguity: `tab.map` is the element under key `map`
(data, likely `undefined`), while `tab->map()` is the built-in `map` operation. Element
space is flat and un-namespaced (any string is a key); meta space is namespaced by
protocol. Meta functions are never assignable, and `->` never appears on the left of an
assignment. The full dispatch rules, views, chaining, and the `@@` protocol-reflection
operator are specified in **views**; **protocols** specifies how a table comes to wear a
protocol in the first place.

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
value; `&` on the *call site* means "assign the return value back to me." The full
list of protocol methods is in **table-protocol-api.md**.

### 4.1 Removers return the shortened table

Because a method's return value *is* the intended new table, operations that would
otherwise return a removed element instead return the **shortened table**. Read the
element first, then remove:

```
var x = myTable.last();           // read the element
&myTable.pop();                   // then shrink in place
```

This keeps `&`-write-back meaning one simple thing everywhere, "assign the return
value back", with no method whose return value is something other than a table.
`pop`, `shift`, `unset`, `remove`, and `clear` all return tables. To learn how many
elements `remove` deleted, diff `count()` around the call (both O(1)).

### 4.2 `&`-write-back is a flag-respecting structural update

Write-back is **not** a blind rebind. It applies the result to the target while
respecting the target's current structural and permission flags, so open/close and
`noSet` retain their force under references:

```
var b = closedTab.prepend(0);     // OK, b is a new, open table; closedTab untouched
&closedTab.prepend(0);            // throws OpenViolationError, write-back would grow a closed table
```

---

## 5. Table-level seals: growth and mutation

Two orthogonal table-level flags gate structural change. **Growth** governs whether
*new keys* may be added; **mutation** governs whether *existing values* may be
changed. Neither governs reading or removal, and the two are independent, a table
may be sealed on either axis, both, or neither.

### 5.1 Growth: open, closed, neverOpen

| State | Add new key? | Reached via | Reversible? |
|-|-|-|-|
| `open` | yes | `open()` | n/a |
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

`close()` is the **revocable** seal, you keep the key and may `open()` later.
`neverOpen()` is the **irrevocable** seal, the key is discarded, and `open()` on
such a table raises `InvalidOpenError`. Irrevocability is the whole point: code
holding a `neverOpen` table may permanently rely on its key-set being fixed, cache
the shape, skip existence checks, assume no growth. A reopen operation would destroy
that guarantee, so it does not exist. The only path from `neverOpen` back to
growable is to **derive a fresh, open table** holding the same values (§7.1),
honest construction of a new value, not unsealing of the old one.

### 5.2 Mutation: frozen, neverThaw

Freezing is the mirror of closing, one axis over: it seals *existing values* rather
than the *key-set*.

| State | Change existing value? | Reached via | Reversible? |
|-|-|-|-|
| unfrozen | yes | `thaw()` | n/a |
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

`freeze()` / `neverThaw()` parallel `close()` / `neverOpen()` exactly, a revocable
and an irrevocable seal, with `thaw()` as the counterpart to `open()`. `neverThaw`
carries the same reliance guarantee: a permanently frozen table's values may be
cached, hashed, and shared without defensive copying, because they can never change.

### 5.3 The two axes are orthogonal

Growth seals the key-set; freezing seals the values. Neither implies the other, and
all four combinations are useful:

| Growth | Mutation | Behaves as |
|-|-|-|
| open | unfrozen | fully mutable (default) |
| closed / neverOpen | unfrozen | fixed-shape record, set keys, mutable fields |
| open | frozen / neverThaw | append-only log, immutable history, new entries allowed |
| closed / neverOpen | frozen / neverThaw | fully immutable snapshot |

This is why `neverOpen` is **not** immutable: it renounces *growth* only. Existing
values remain writable, and elements may still be read and removed. Immutability of
*values* is `freeze`'s job; a fully immutable table is sealed on both axes.

### 5.4 What each seal governs

**Growth-attempting operations** answer to open-state, they introduce a key:

- Direct assignment to a new key: `tab.newKey = x`, `tab[k] = x`
- `prepend`, `append`, `insert`, `pad`, `fill` (for new keys), and `splice` when the
  replacement introduces keys beyond those it removed
- The write-side coalescer `??=` when firing on an absent key

**Mutation operations** answer to freeze-state, they overwrite an existing value:

- Direct assignment to an existing key: `tab[existingKey] = x`
- `fill` / `replace` / `splice` where they overwrite existing keys
- The write-side coalescer `???=` when firing on a present `null`

In their **pure** form all of these are legal, they build a new table, born open
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
private or read-only. Heterogeneous per-key permissions, "this field is private,
that one is read-only", describe a *type contract*, which is exactly what a protocol
expresses. A table therefore acquires `noGet` / `noSet` keys only by conforming to a
protocol that declares them. To seal an *entire* table's values instead, use the
table-level `freeze` (§5.2).

`noSet` and `freeze` both forbid writes but at different scopes: `noSet` is one
protocol-declared key; `freeze` is every key at once. On a key that is both, the
table-level `freeze` check runs first, so the error is `FreezeViolationError`.

### 6.1 Absence, `null`, and denial are three different things

- **Absent key** → reading yields `undefined`. Absence never throws; it is routine
  control flow for a table-as-hashmap. Because `undefined` is unstorable, *present*
  keys never hold it, existence ⟺ not-`undefined`.
- **Present, `null`** → yields the stored `null`. A real, deliberate value.
- **Present, `noGet`** → raises `TableReadViolationError`. A *permission* failure,
  not a value.

These stay distinct throughout the protocol and through the access operators
(`?.`, `??`, `???`, see the separate *Optional Access & Coalescing* reference).
`has()` answers existence in O(1); `canGet()` / `canSet()` answer "may I read /
write this right now?" in O(1), `canSet` accounts for `freeze` as well as `noSet`.
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

- `throw`, raise `TableReadViolationError` / `TableMutationViolationError`.
- `skip`, silently omit the offending element from the operation.

Placement follows a rule rather than per-method choice: **any method that reads
values takes `onNoGet`; any method that overwrites existing keys takes `onNoSet`.**
Keys-only methods (`keys`, `has`, `canGet`, `keyFirst`, `keyLast`) take neither.
`skip` suppresses only the *per-key permission* refusal, never a table-level
`FreezeViolationError` or `OpenViolationError`, which are separate axes. Because
permissions are fixed by protocol, `skip` never alters them; it merely omits the
offending element from this one operation.

The per-method `onNoGet` / `onNoSet` parameters are listed with each entry in
**table-protocol-api.md §1**.

---

## 7. Flag & permission propagation

Tables carry two kinds of flags with **different** propagation rules, split along
the *whose-invariant-is-it* seam.

### 7.1 Structural flags do NOT propagate through transformers

`open` / `close` / `neverOpen` **and** `frozen` / `neverThaw` are properties a
table asserts about **itself**. A table *derived* by a transformer (`map`, `filter`,
`merge`, `prepend`, …) is a new self and re-decides: **derived tables are born `open`
and unfrozen.**

- **Identity copy** (`var b = a;`), same logical value; keeps `a`'s seals on both
  axes, including `neverOpen` and `neverThaw`. Under COW, the first growth or
  mutation attempt on `b` throws before any storage split.
- **Derived / transformed table**, a *different* value with different contents; born
  `open` and unfrozen regardless of the source's state.

This is what makes deriving a fresh table the honest escape from either irrevocable
seal (§5.1 and §5.2): `sealedTab.map(x => x)` yields an open, unfrozen table with the
same values, without unsealing anything.

### 7.2 Permission flags DO propagate, by key identity

`noGet` / `noSet` are invariants a **protocol** asserts about any conforming table.
If a transformer dropped them, they would be properties of "a table nobody has
`.map()`-ed yet", worthless as type invariants. So they must survive transforms.
The precise rule:

> **`noGet` / `noSet` propagate to any output key that is identically an input
> key. Freshly-created keys are born default (get + set).**

- **Key-preserving** operations (`filter`, `map` in `{values}` mode, `reverse`,
  `sort`, `slice` with `preserveKeys`, `replace`, `fill`) carry each surviving
  key's permissions forward.
- **Key-creating** operations (`values`, `keys`, `map` in `{keys}` / `{both}` mode,
  `flip`, `groupBy`, `flatten`, anything reindexing to `0..n`) mint keys that did
  not exist in the input; those keys are born default. `flip` is the sharp case, a
  `noGet` *value* becoming a *key* cannot stay `noGet`; the concept does not
  transfer.

Protocol membership is naturally shed where it cannot apply: a list, for instance,
never carries protocols beyond the built-in one.

Under `&`-write-back the **target's** current permissions govern the write. Because
key-preserving transforms carry permissions along, source and target normally agree;
they diverge only when the target conforms to a protocol the source did not, in which
case the target's contract wins.

---

## Amendment A: representation of `const` tables (implementation note)

This is an **implementation concern, not a semantic one**. The representation described
here is chosen by the compiler and is **unobservable**: a `const` table has exactly the
same semantics as any other table (same `.` / `[]` reads, same `foreach`, same `@` and
`@@`, same protocol behavior, same absence rules). Only its performance and memory layout
differ. The programmer writes and uses a `const` table identically to any table and cannot
tell which representation was chosen.

### A.1 What `const` licenses

A `const` table is deep-frozen (`neverOpen` + `neverThaw`, §5, variables spec): its key
set can never change, and its contents never mutate. Two facts follow, and they are the
basis of the representation:

- **The structure is frozen**, so none of the machinery that supports mutation is needed:
  no hashmap growth or rehashing, no tombstones, no load-factor slack, no copy-on-write
  bookkeeping.
- **When the table is built at compile time** (the common case, a `comptime`-constructed
  lookup table, functions spec §5), its **shape is statically known**: the exact key set,
  and each value's type, are known to the compiler.

### A.2 The representation

A `const` table is represented as **contiguous frozen storage plus a compile-time perfect
hash** over its keys, never as a live hashmap:

- **Contiguous storage:** the values are packed in a contiguous block, exactly sized (no
  empty slots), cache-friendly for access and iteration. Heterogeneous value types are
  fine; the block is a struct of mixed fields at fixed offsets.
- **Perfect hash index:** a collision-free compile-time hash from key to (offset, type),
  which also fixes an iteration order. Because the key set is fixed, the hash is *perfect*:
  one probe, no chaining, no collisions, which a runtime hashmap (whose keys change) cannot
  achieve.
- **Keys retained as runtime data:** the keys are kept as real runtime values in the
  index, not compiled away. They must be, because iteration and dynamic access need them
  (§A.4).

### A.3 Access paths

The `.` / `[]` split (§3.2) tells the compiler, per access site, which path to take:

- **`constTab.name` (static key):** the key is a compile-time literal, so the compiler
  shortcuts directly to the field's offset, `load [base + offset]`, skipping the hash
  entirely. This is the "virtual table" fast path: struct-field performance from table
  syntax, available only for static `.` access.
- **`constTab[expr]` (dynamic read):** the key is a runtime value, so it resolves through
  one perfect-hash probe to an offset, then loads. Reading a `const` lookup table with a
  runtime key (the primary reason such tables exist) is fully supported and fast, one
  collision-free probe into contiguous storage.
- **Writes** (`constTab.x = v`, `constTab[x] = v`): already impossible on a frozen table
  (§5); no representation concern.

Reading via `[]` is therefore **not** disallowed on a `const` table; it is a core use case.
Only mutation is forbidden, and that is the seal, not a representation rule. As a bonus,
where a dynamic-looking read uses a compile-time-literal key the compiler knows is absent
from the fixed key set (`constTab[42]` with `42` not a key), it can report this at compile
time rather than deferring to the runtime absence (`undefined`) result.

### A.4 Why the keys and hash are always present

The direct-offset path alone would let the compiler erase keys to bare field offsets, but
it cannot, because a `const` table is still used **as data**, not only as named fields:

- **`foreach (constTab as k => v)`** must yield real (key, value) pairs.
- **`keys()`, `values()`, `count()`, serialization (`toJson`), reflection**, and any
  protocol meta that walks entries, all need the keys as runtime values.

So the perfect-hash-plus-retained-keys structure is the **baseline** representation, and
the direct-offset shortcut for static `.` keys is an optimization layered **on top** of it,
not a replacement for it. The keys can never be fully compiled away.

### A.5 Scope

- **Every `const` table** gets the frozen benefits (exactly-sized, perfectly-hashed, no
  mutation machinery, no COW, shared by pointer).
- **Compile-time-shaped `const` tables** (the `comptime`-constructed case) additionally get
  the contiguous-struct layout with direct-offset static access, because only there is the
  shape statically known.
- A table frozen at *runtime* (built mutably, then sealed) gets the frozen benefits but not
  the compile-time struct layout, since the compiler did not know its shape.

The whole representation is opaque: chosen by the compiler, licensed by the `const` seal
and the static-key rule (§3.2), and invisible to the programmer, who sees only ordinary
table semantics running faster.

---

*See also:* **table-protocol-api.md** for the complete operation catalogue and the
error summary, **protocols** and **views** for how tables wear protocols and how `->`
reaches their behavior, and the *Optional Access & Coalescing* reference for `?.`, `??`,
and `???`.
