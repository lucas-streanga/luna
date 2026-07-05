# Tables

This document defines the concepts behind Luna's table type: keys and how they are
accessed (§3), value semantics (§4), table-level growth sealing (§5), element
`get` / `set` permissions (§6), and flag propagation (§7). The built-in operations that act on tables
are catalogued separately in **table-api.md**. Behavior beyond the built-in
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
  'name' => 'Lucas',
  'lastName' => 'Streanga',
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
**table-api.md §2.1**):

- `isList()`, true iff the table is a list (incrementing int keys from 0).
- `isContiguousMemory()`, true iff stored contiguously. All lists are
  contiguous; some non-list tables are too. **Any string key ⇒ never contiguous.**

### 2.1 `table` is the primitive; `list` is a constraint on it

**`table` is the one table type.** There is no second table-like primitive, no `hashmap`,
and list-ness is not a distinct kind of value: it is a **shape a table currently has**
(keys exactly `{0, 1, ..., n-1}` and nothing else), queryable in O(1) as **`isList()`**
(§2.2).

**`list` is a type only in the way every constraint is** (constraints spec): the built-in
**invariant table-level constraint** (§8, constraints §7.1, §10) whose predicate is
"`isList()` holds." It mints a typeid inside `table`'s interval (constraints §9.1), so
`list <: table` and a list is usable anywhere a `table` is expected (widening is implicit,
`as` spec), while treating a `table` as a `list` is a checked narrowing (§2.3). It is
special among table-level constraints only in cost: its predicate reads the maintained
O(1) bit (§2.2) instead of scanning keys, so declaring `list` buys full invariant
protection at O(1) per mutation where a general table constraint pays its predicate.

The division of labor is **type = commitment, method = fact**. An *unconstrained* table's
shape is a fact that varies freely as keys come and go, reported by `isList()`, and `@t`
reports `"table"` regardless of the current shape, no key layout changes a value's type.
A *`list`-declared* position is a commitment: the value entered through the constraint,
carries its typeid, `@` reports `"list"` (ordinary constraint reflection, constraints §8),
and a shape-breaking write is a violation, a **compile error** where statically evident
(`xs['k'] = 1` on `xs: list`), a **panic** where index-dependent (constraints §7). So:
declare `table` and nothing about shape can ever be violated; declare `list` and the shape
is protected, through every path, like any invariant constraint (constraints §9.4).

The empty table `[]` satisfies the predicate vacuously, so `let xs: list = []` is legal,
and a literal whose shape visibly satisfies it (`[1, 2, 3]`) enters a `list` declaration
with the check discharged at compile time; `['x' => 1]` into a `list` binding is a compile
error. An **unannotated** binding infers `table` (`var t = [1, 2, 3]` is a table that
happens to be list-shaped): a literal carries no constraint, constraints enter only
through declarations (constraints §7, "on entry"), so protection is always an explicit
choice, never an accident of the initializer.

### 2.2 List-ness is a maintained, O(1) property

A table's list-ness is **maintained**, not recomputed: every table carries it as a cheap
derived bit, so `isList()` is O(1), never an O(n) key scan. The bit is a **fact about the
current shape**, not a type (§2.1): on an unconstrained table it simply flips as the shape
changes, and on a `list`-constrained value it is what makes the constraint's per-mutation
predicate O(1) (constraints §7, §10), the check *reads the bit the write would leave*, and
a write that would clear it **panics** with the value unchanged (invariant semantics,
constraints §7.1), compile error where statically evident (§2.1). Operations set or clear
the bit predictably:

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

- **`tab as list`**: asserts the table is currently a list; a `typeError` (panic) if it is
  not (has a gap or a string key).
- **`tab is list`**: a **boolean test** of whether the table is currently a list. It does not
  narrow `tab` (Luna does no flow-narrowing, as spec §7); to obtain a `list`-typed binding, use
  `tab as list` (or a `@list` match pattern), which produces a narrowed value.

Removal leaves gaps rather than reindexing (§2.2), so a table with gaps is genuinely not a
list. To **re-compact** a gapped or keyed table into a fresh contiguous list, call
`values()` (table-api.md), which reindexes the values to `0..m-1` and returns a
`list`. So `values()` is the explicit "make this a list again" operation; nothing reindexes
silently.

**`as list` asserts; `values()` produces.** These are not interchangeable, they answer
different questions:

- **`tab as list`** *asserts* the table is **already** a list. It runs a check and raises a
  `typeError` (panic) if `tab` has a gap or a string key; on success the same value is
  re-typed, unchanged. Use it when a non-list is a bug you want caught.
- **`tab.values()`** *produces* a fresh list by reindexing, so it **always** succeeds
  whatever `tab`'s shape. Use it when you want a list out of any table.

They even differ in result: `[5=>'a', 9=>'b'] as list` **panics** (not contiguous), while
`[5=>'a', 9=>'b'].values()` returns `['a', 'b']` (drops the keys, reindexes the values). And
because narrowing is never implicit, passing a bare `table` where a `list` is required is a
compile error; you choose `as list` (assert) or `values()` (produce) explicitly:

```
someFn(tab)             // COMPILE ERROR: table is not implicitly a list
someFn(tab as list)     // asserts tab is currently a list; typeError panic if not
someFn(tab.values())    // always legal; builds a fresh list from tab's values
```

### 2.4 `list` in signatures

Operations that are guaranteed to produce a contiguous-from-zero result are typed to return
`list`, not `table`, so callers get the list guarantee statically (and can index `0..n-1`,
pass to list-requiring functions, and destructure positionally without a runtime check). A
returned `list` is a constraint-carrying value like any other (constraints §9.2), so the
guarantee is **maintained** by ordinary invariant enforcement, and an unannotated binding
initialized from such a call infers `list` (inference keeps constraints as-is, variables
§1: the producer's commitment stays). A
`const` list additionally takes the tightest representation (a pure contiguous array with no
stored keys, since the keys are implicitly `0..n-1`; Amendment A).

### 2.5 Slicing a list: the `:` syntax

A list is sliced with the **`:` syntax**, which is **half-open** (the end index is excluded),
the Python and Rust convention. A slice returns a new **`list`** (reindexed from 0):

```
let mid  = xs[1:3];      // elements at indices 1 and 2 (end excluded): a 2-element list
let tail = xs[2:];       // from index 2 to the end
let head = xs[:3];       // from the start to index 2 (indices 0, 1, 2)
let copy = xs[:];        // a full shallow copy
```

- **Half-open**: `xs[a:b]` is indices `a` through `b - 1`, so `xs[0:len]` is the whole list and
  adjacent slices compose without overlap (`xs[0:k]` and `xs[k:n]` partition it). Reading a
  single index `xs[i]` returns the element; slicing with `:` returns a `list`.
- **Open-ended forms**: `xs[a:]` (to the end), `xs[:b]` (from the start), `xs[:]` (full copy).
- **Slicing is deliberately `:`, not the inclusive `..` range syntax** (range spec §2.1): slices
  are half-open (natural for indexing), ranges are inclusive (natural for iteration), so each
  keeps its own convention and `..` and `:` never conflict.

Slicing applies to lists (contiguous integer keys); slicing a non-list table is not defined
(there is no positional order to slice), so slice a table's `values()` if a positional slice of
its values is wanted.

---

## 3. Keys and access

### 3.1 Key types

A key is **`int` or `string`, and nothing else.** There is **no implicit key coercion**:
a `double`, `bool`, `table`, or any other type is **not** a valid key and is **not** silently
converted into one. This matches the language's no-implicit-coercion stance everywhere else
(no truthiness, no implicit numeric coercion, conversions are explicit functions): a key of
another type is a compile error (statically) or a `typeError` (dynamically), not a quiet
normalization.

| Key type | Allowed? |
|-|-|
| `int` | Yes, native. |
| `string` | Yes, native. |
| any other type (`double`, `bool`, `table`, ...) | No, convert explicitly first |

To key by a value of another type, convert it **explicitly** to an `int` or `string` first,
using the ordinary conversion functions (conversion spec): `tab[d.toString()]` for a `double`,
`tab[b.toInt()]` for a `bool`, `tab[t.toJson()]` for a table used as a composite key. The
conversion is visible in the source, so what a key actually is is never hidden behind an
implicit rule.

### 3.2 Static keys (`.`) vs. dynamic keys (`[]`)

Element access comes in two forms, and the distinction is **global**, it holds
everywhere a table is accessed, not only inside protocol `apply` bodies:

- **`tab.name` is a static key.** The key is the literal identifier `name`, known at
  compile time. There is no `.dynamicName`; `tab.someVar` does **not** mean "the key
  held in `someVar`", it means the literal key `someVar`. `.` is only ever a
  compile-time-known name.
- **`tab[expr]` is a dynamic key.** The key is the runtime value of `expr`. This is the
  form for integer keys (`tab[0]`), non-identifier string keys (`tab['has space']`),
  and any computed key (`tab[k]`, `tab["row$i"]`).

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

**A table never stores `undefined`**, so assigning `undefined` into a key never stores it and never
deletes the key. Assigning `undefined` is a **use** of the value and is illegal (undefined spec §3):
the literal `tab['k'] = undefined` and any assignment of a *statically* undefined value (a void call,
an `undefined`-typed binding) are **compile errors**, and assigning a value that turns out to be
undefined at run time (for example a missing-key read passed along) is a **runtime panic**. So
`existence ⟺ not-undefined` holds by construction: no assignment can put `undefined` into storage.
Deletion is always **explicit**, `remove` / `unset` (§4.1), never a side effect of assigning
`undefined`, silent delete-on-assign would let a stray undefined make a key vanish untraceably, so
the language rejects the assignment instead. To move a possibly-undefined value into a table, resolve
the absence first (`tab['k'] = maybe ?? fallback`).

### 3.3 Element space (`.` / `[]`) vs. meta space (`->`)

`.` and `[]` reach **element space**: the table's own keyed data. A third operator,
`->`, reaches **meta space**: the behavior contributed by the protocols a table has applied.
The two spaces are disjoint, and the operators never overlap:

- **`tab.name` / `tab['name']`** , element (data). Static or dynamic key. A miss is
  `undefined`. Assignable.
- **`tab->name(...)`** , a call to the built-in protocol's meta function (the built-in
  protocol is nameless, so it is reached by bare `->`). The table operation catalogue
  (`map`, `filter`, `count`, `pop`, and the rest) lives here, in
  **table-api.md**.
- **`tab->protoName`** , a view of a named applied protocol, or `undefined` if the
  table does not have it applied. Meta functions are reached through the view with `.`.

Because element space and meta space use different operators, a data key can share a
name with a meta function without ambiguity: `tab.map` is the element under key `map`
(data, likely `undefined`), while `tab->map()` is the built-in `map` operation. Element
space is flat and un-namespaced (any string is a key); meta space is namespaced by
protocol. Meta functions are never assignable, and `->` never appears on the left of an
assignment. The full dispatch rules, views, chaining, and the `@@` protocol-reflection
operator are specified in **views**; **protocols** specifies how a protocol comes to be applied to a
protocol in the first place.

---

## 4. Value semantics: copy-on-write, references, and write-back

**Tables are copy-on-write value types.** Assignment shares storage until a write
forces a split; logically, every table is an independent value.

**One exception: a `stream` held as a table value is shared by reference, not copied.**
A stream is a reference value that cannot be copied at all (a single-pass stream cannot
be forked or replayed without buffering everything; stream spec §4, §6.1). So when a
table is copied, a stream *element* is **shared**, not duplicated: if `a` holds a stream
and `var b = a;`, then `a` and `b` refer to the **same** stream, and consuming it through
either consumes it for both. This is the ordinary single-pass stream discipline (stream
spec §2), not a table-specific rule; it just means a table is not fully independent of its
copies with respect to a contained stream. To get independent sequences, materialize the
stream into retained data first (stream spec §5.1). Every non-stream value follows the
normal copy-on-write independence.

Consequently **the protocol never mutates its receiver in place.** Every operation
returns a *new* table (COW), and the caller decides whether to keep it or write it
back. Write-back uses a caller-side reference:

```
var sorted = myTable.sort();      // myTable unchanged; sorted is a new table
&myTable.sort();                  // write-back: myTable becomes the sorted result
```

No protocol method takes a `&table` parameter. The receiver is always passed by
value; `&` on the *call site* means "assign the return value back to me." The full
list of protocol methods is in **table-api.md**.

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

#### 4.1.1 Deletion is dynamic-only: there is no static `.key` delete form

Reads and writes come in two forms, static (`tab.key`, compiler-checked against the table's
shape) and dynamic (`tab['key']`, a runtime key). Deletion has **only** the dynamic form:
`remove('key')` / `unset('key')` take a **runtime key**, and there is deliberately no
`delete tab.key` static form. This asymmetry is intentional, and it is forced by two facts that
together leave no coherent static-delete to build:

- **An unconditional static delete is redundant with editing the source.** If both the table's
  shape and the key are known at compile time, then so is the result (the table without that key),
  so `delete t.key1` on a statically-known `t` is equivalent to just writing the smaller literal.
  The operation exists only to *not be written*:

  ```
  let t = ['key1' => 'value1'];
  delete t.key1;                 // pointless: identical to `let t = [];`
  ```

- **A conditional static delete is impossible without control-flow analysis Luna does not do.**
  The moment a static delete is conditional, the table's resulting shape becomes runtime-dependent,
  and any later static `.key` access can no longer be checked, the compiler would have to track
  "is this key still present on this path?", which is exactly the control-flow analysis Luna
  refuses (the same reason a `never` exit is not statically verified, never spec):

  ```
  delete t.key1 if (condition);  // would require CFA: is t.key1 present afterward? unknowable
  t.key1;                        // could no longer be compiler-checked
  ```

  A conditional static delete injects runtime-dependence into static key space, whose entire
  purpose is a compile-time-known shape, so the two cannot coexist without the analysis Luna does
  not perform.

So static delete is either **pointless** (unconditional, redundant with the source) or
**unsupportable** (conditional, would need control-flow analysis), with no version that is both
meaningful and implementable. Deletion is therefore **dynamic-only**, and `remove` / `unset` taking
a runtime key is the correct and complete design: deletion carries information only when the key is
dynamic (a variable, a computed string, a loop key), which is precisely the case those operations
serve, and a conditional dynamic delete (`remove(k) if cond`) is fine, because dynamic key space is
already runtime-shaped and has no static shape-knowledge to destroy.

(A table's growth and mutation seals, §5, govern whether keys may be added or changed at all; a
`neverOpen` table whose shape callers rely on being fixed, for instance, is not a place where
removal should silently change the cached shape. The interaction of removal with the seals and with
statically-declared shape is governed there.)

### 4.2 `&`-write-back is a flag-respecting structural update

Write-back is **not** a blind rebind. It applies the result to the target while
respecting the target's current structural and access flags, so open/close and a
key's protocol `set` grant retain their force under references:

```
var b = closedTab.prepend(0);     // OK, b is a new, open table; closedTab untouched
&closedTab.prepend(0);            // throws OpenViolationError, write-back would grow a closed table
```

---

## 5. Table-level growth sealing

A table-level flag governs **growth**: whether *new keys* may be added. (An earlier design paired
this with a *mutation* seal over existing values; that axis is removed, §5.2.) Growth sealing does
not govern reading or removal.

> Design note: growth sealing is currently a **runtime** property set by methods (`close()`,
> `neverOpen()`). Whether a fixed key-set is better expressed as a **protocol/type contract** (a
> fixed-shape table type, where adding a key is a *compile* error rather than a runtime
> `OpenViolationError`), consistent with how per-key access is protocol-declared (§6), is an open
> direction under review. The runtime behavior below is the current model.

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

### 5.2 Mutation sealing (`freeze` / `thaw`) is removed

An earlier design had a second seal axis, `freeze` / `thaw` / `neverThaw`, mirroring growth but over
*existing values*. **It is removed.** Tables are value types (§4): a table is copied at every
boundary that could otherwise alias it (sole-ownership, concurrency spec), so a function you pass a
table to receives a copy and cannot mutate yours, and tasks never share a mutable table. Freezing to
*protect* a table from mutation-through-sharing therefore protects nothing, the sharing it defends
against cannot happen. And a *revocable* freeze (`freeze` then `thaw`) provides no permanent
guarantee either, so it cannot even serve as an optimization signal. With neither a protection nor an
optimization role, revocable mutation-sealing has no use case and is cut.

Immutability *as a value's contract* is expressed differently, two ways, neither of which is a
revocable runtime flag:

- **Compile-time immutability** is `const` (variables spec): a `const` table is deeply immutable,
  known at compile time, which is what enables const-table specialization (perfect-hashing, inlining;
  compiler spec).
- **Per-key access** (whether a key may be read or written at all) is declared by **protocols**
  (§6), not by a table-level runtime flag.

#### 5.2.1 Deferred: runtime immutability for interning and caching

There is one immutability role that value semantics does **not** already cover, and it is worth
recording even though it is **deferred past the current design**: promoting a **runtime-built**
table into an immutable-optimization regime.

A table built at runtime (a config loaded at startup, a lookup table computed from input, a
memoization cache filled once then read-only) is not `const` (its contents are not known at compile
time), so the compiler must treat it as mutable and cannot apply the optimizations that permanent
immutability would allow: a stable cached hash, interning so equal values share one representation,
eliding the copy-on-write refcount for a value that will never be written, and caching the shape to
skip existence checks. A future `toImmutable()`-style operation would let such a table opt into that
regime.

The design constraints for that future feature, so it is added correctly, are:

- It is an **optimization**, not a correctness feature. Value semantics already provides the
  correctness (no aliasing, no races); this only makes runtime-stable tables faster to hash, compare,
  and store. That is exactly why it can be **deferred**: it is purely additive and changes no
  semantics.
- It produces a **runtime-immutable value whose immutability travels in its type** (a table typed as
  immutable), not a `const` (which is a *binding*, compile-time). Immutability is a property of the
  *value*, established at runtime, like `secret` is a value property; a write to such a value is a
  compile error because its type says immutable.
- It **derives a fresh value** (a copy-on-write-lazy copy), leaving the original mutable, rather than
  sealing in place. Deriving-a-new-binding is also what keeps it compatible with the no-control-flow
  guarantee (compiler spec §1.4.1): the immutable value's type comes from the operation's return
  type, so the original binding's type never changes mid-function (which in-place sealing would
  require, and which would need flow analysis).

The primary motivation is **interning and caching of runtime-stable data**; using tables as
*composite keys* is a related possible motivation, but it is covered more simply by serializing a
table to a `string` key (`toJson` and equivalents), so it does not by itself justify the feature.
Until a concrete performance need arises, runtime immutability is left unspecified.

---

## 6. Element permissions: get / set

Access to a key, whether it may be **read** (`get`) or **written** (`set`), is governed by
**protocols**, and the default is **no access**: a protocol-declared key exposes reading only if the
protocol grants `get`, and writing only if it grants `set`. This inverts the usual default: a typed
table's fields are **private unless exposed**, rather than public unless hidden.

| Protocol grants | Read | Write |
|-|-|-|
| `get` + `set` | yes | yes |
| `get` only | yes | `TableMutationViolationError` |
| `set` only | `TableReadViolationError` | yes |
| neither (default) | `TableReadViolationError` | `TableMutationViolationError` |

### 6.1 Bare tables are fully accessible; protocols are the encapsulation boundary

The default-no rule applies to **protocol-declared keys**, not to bare tables. A **bare table
literal** is fully readable and writable on every key by whoever holds it, it is your own data, and
there is nothing to encapsulate from yourself:

```
var t = ['name' => 'Lucas'];
t.name;                 // OK: bare table, full access
t.age = 0;              // OK
```

A **protocol** is the unit of encapsulation. When a protocol declares a table's shape, its declared
keys default to **no access**, and the protocol opts specific keys into `get` and/or `set` as part
of its contract. This is how "this field is private, that one is read-only, this one is read-write"
is expressed: as a *type contract* (a protocol), checkable at compile time, rather than an ad-hoc
runtime flag on an individual key. There are no runtime methods to change a key's access; access is a
property of the protocol a table has applied.

This matches the language's deny-by-default stance elsewhere (capabilities are granted, not revoked;
`secret` conceals by default): a typed table exposes only what its protocol deliberately grants.

### 6.2 Absence, `null`, and denial are three different things

- **Absent key** → reading yields `undefined`. Absence never throws; it is routine
  control flow for a table-as-hashmap. Because `undefined` is unstorable, *present*
  keys never hold it, existence ⟺ not-`undefined`.
- **Present, `null`** → yields the stored `null`. A real, deliberate value.
- **Present, no `get`** → raises `TableReadViolationError`. A *permission* failure,
  not a value.

These stay distinct throughout the protocol and through the access operators
(`?.`, `??`, `???`, see the separate *Optional Access & Coalescing* reference).
`has()` answers existence in O(1); `canGet()` / `canSet()` answer "may I read /
write this right now?" in O(1). On a **missing** key both return `false`, and
`has()` disambiguates absence from denial.

### 6.3 Bulk operations and permissions: `onNoGet` / `onNoSet`

A method that **reads element values** may encounter a key the applying protocol does not grant `get`;
one that **writes to existing keys** may encounter a key it does not grant `set`. Behavior is
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
Because access is fixed by protocol, `skip` never alters it; it merely omits the
offending element from this one operation.

The per-method `onNoGet` / `onNoSet` parameters are listed with each entry in
**table-api.md §2**.

### 6.4 Protocol contracts are value-carried, and enforced on write

Both kinds of contract a protocol places on a key, its **access grant** (`get` / `set`, §6) and
the **declared type** of the value (protocols §5.4), are properties of the **value's applied-protocol
set** (the `@@` axis, views spec), not of the binding through which the value is reached. They are
enforced the same way constraints are (constraints §9.4): **checked on write, keyed on the value,
trusted on read.**

The consequence that matters is that **widening cannot launder them.** Widening a protocol-applying
table to bare `table`, or passing `&t` to a `fn (&t: table)`, relaxes the *static* view to `any`
element access (§6.1, protocols §5.4.1) but does **not** strip the protocol from the *value*. So a
write through that bare-`table` binding is still checked against the value's actual contracts:

- a write to a key the applied protocol does not grant `set` still raises `TableMutationViolationError`;
- a write of a value that violates the key's declared type still raises `typeError` (protocols
  §5.4.2).

This is the exact parallel of constraint collapse (constraints §9.2): a `list` widened to `table`
is still checked as a `list` on mutation, because the value carries `list`; a `@person` widened to
`table` is still checked as a `person` on mutation, because the value carries `person`. In both, the
check reads the value, so the widened static view is a loss of *precision on reads*, never a loss of
*enforcement on writes*.

The **bare-table-literal** rule of §6.1 is unchanged and consistent with this: a table applying **no**
protocol has no contracts to enforce, so it is fully accessible to its holder. §6.1 is about a value
that has nothing applied; §6.4 is about a value that has a protocol applied but is *seen through* a bare-`table`
binding, a different thing. To obtain a genuinely contract-free table from a protocol-applying one,
derive a fresh one (`copy`, variables §5.2, or a transformer, §7.1), which is born applying only the
built-in protocol.

As with constraints, the runtime cost is confined to **writes the compiler cannot prove safe**: a
write is statically discharged where the site knows the protocol set, the key, and the value type
(protocols §5.4.2), so the common typed-binding write pays nothing.

---

## 7. Flag & permission propagation

Tables carry two kinds of flags with **different** propagation rules, split along
the *whose-invariant-is-it* seam.

### 7.1 Structural flags do NOT propagate through transformers

`open` / `close` / `neverOpen` is a property a table asserts about **itself** (the mutation-seal axis
is removed, §5.2). A table *derived* by a transformer (`map`, `filter`, `merge`, `prepend`, …) is a
new self and re-decides: **derived tables are born `open`.**

- **Identity copy** (`var b = a;`), same logical value; keeps `a`'s growth seal, including
  `neverOpen`. Under COW, the first growth attempt on `b` throws before any storage split.
- **Derived / transformed table**, a *different* value with different contents; born `open`
  regardless of the source's state.

This is what makes deriving a fresh table the honest escape from the irrevocable growth seal (§5.1):
`sealedTab.map(x => x)` yields an open table with the same values, without unsealing anything.

### 7.2 Protocol access DOES propagate, by key identity

The `get` / `set` access a **protocol** grants a key is an invariant that protocol asserts about any
conforming table. If a transformer dropped it, it would be a property of "a table nobody has
`.map()`-ed yet", worthless as a type invariant. So it must survive transforms. The precise rule:

> **A key's protocol-granted access propagates to any output key that is identically an input key.
> Freshly-created keys that no protocol governs are bare (full access to their holder).**

- **Key-preserving** operations (`filter`, `map` in `{values}` mode, `reverse`,
  `sort`, `slice` with `preserveKeys`, `replace`, `fill`) carry each surviving
  key's protocol access forward.
- **Key-creating** operations (`values`, `keys`, `map` in `{keys}` / `{both}` mode,
  `flip`, `groupBy`, `flatten`, anything reindexing to `0..n`) mint keys that did
  not exist in the input; those keys are bare unless the result conforms to a protocol
  that declares them. `flip` is the sharp case, a restricted *value* becoming a *key*
  cannot carry its access; the concept does not transfer.

Protocol membership is naturally shed where it cannot apply: a list, for instance,
never carries protocols beyond the built-in one.

Under `&`-write-back the **target's** current permissions govern the write. Because
key-preserving transforms carry permissions along, source and target normally agree;
they diverge only when the target conforms to a protocol the source did not, in which
case the target's contract wins.

---

## 8. Table-level constraints

A constraint (constraints spec) whose **base type is `table`** refines a table by its
**structure**, the key set, the counts, the relationships between entries, rather than by any
single stored value. `list` (§2.1) is the built-in instance; this section specifies the general
form, of which `list` is the cheap special case.

A **value constraint** (`byte`, `port`) restricts what a single value may be, and when a table's
element is typed by one, it restricts that one element. A **table-level constraint** is different
in kind: it does **not** act on a value at a key, it acts on **the table as a whole**. So the two
occupy different slots, a value constraint types an element; a table-level constraint types the
table, and never conflict.

### 8.1 Declaring one

A table-level constraint is an ordinary `constraint {}` (constraints spec §1) with base `table`
and a pure predicate over the whole table:

```
const pair    = constraint { table as t where t->count() == 2 };
const sorted  = constraint { table as t where isSorted(t->values()) };
const tagged  = constraint { table as t where t.kind is string };
```

The predicate is **any pure boolean expression over the table** (constraints §2, purity is enforced
by the form): it may read counts, inspect keys, compare entries, or `match` the table's shape, as
the author wants. `list` is exactly `constraint { table as t where <keys are 0..n-1> }`, with a
maintained-bit implementation instead of a scan.

### 8.2 It tests the whole table, and that is the cost

Because the predicate ranges over the table, checking a table-level constraint means **running it
over the table** — potentially O(n). This lands on the constraint model's timing (constraints §7):
the predicate runs **on entry** (a bare table becomes constrained by construction, assignment to a
constrained slot, or `as`) and **on every key mutation** that could break it, since any key write
can change the structure the predicate ranges over. So a table-level constraint pays its check on
each mutation, which is the deliberate price of the guarantee, exactly the "checked on every key
mutation, no exceptions" rule (constraints §7), here at whole-table granularity.

`list` avoids the O(n) cost only because its particular predicate (contiguous keys from zero) is
maintainable as an **O(1) derived bit** (§2.2): each mutation updates the bit rather than rescanning.
A general table-level constraint has no such shortcut unless its predicate happens to be similarly
maintainable, so its per-mutation check is as expensive as its predicate. This cost is a **ceiling**:
the compiler elides the check at any mutation site it can prove invariant-preserving (constraints
§9.5), and an author who needs a cheaper check writes a cheaper (or maintainable) predicate.

### 8.3 Enforcement is value-carried, like every constraint

A table-level constraint mints its own typeid, placed in `table`'s subtype interval, and the
constrained table **carries that typeid** (constraints §9). Two consequences that matter for tables
specifically:

- **The constraint travels with the value.** Widening a `pair`-constrained table to plain `table`
  (collapse, constraints §9.2) does not strip the constraint from the value; it only relaxes the
  static demand. A widened **`&`** route no longer exists (references are invariant, variables
  §5.1), but the value can still be reached through a **wider container path**, a `pair` stored
  in an untyped slot and mutated as `outer['p']['x'] = v`, and there the write still re-runs the
  `pair` predicate off the value's own typeid and **panics if it would break it** (constraints
  §9.4), even though the mutating site sees no `pair`. Handing code a genuinely unconstrained
  table is explicit, `copy` it (variables §5.2) or rebuild with `values()`.
- **A violating mutation panics and leaves the table unchanged** (`typeError`, errors §9); it never
  silently downgrades the binding's type (Luna does no flow-narrowing, `as` spec §7).

### 8.4 The one-key degenerate case is allowed

Nothing requires a table-level constraint's predicate to actually inspect the whole table. A
predicate may look at a **single key** (`where t.kind is string`) and ignore the rest. This is
**permitted**: the language does not police how much of the table a predicate reads. It is, however,
**poor form and marginally more expensive** than the alternative, because a single-key contract is
usually better expressed as a **value constraint on that element** (which is checked only when that
one key is written), whereas a table-level constraint fires its check on **every** key mutation, even
mutations to keys the predicate ignores. So a one-key table-level constraint works and is sound; it
just pays the whole-table trigger for a value-constraint job. Prefer a value constraint on the
element when the contract is really about one key, and reserve table-level constraints for genuine
whole-table structure.

---

## Amendment A: representation of `const` tables (implementation note)

This is an **implementation concern, not a semantic one**. The representation described
here is chosen by the compiler and is **unobservable**: a `const` table has exactly the
same semantics as any other table (same `.` / `[]` reads, same `foreach`, same `@` and
`@@`, same protocol behavior, same absence rules). Only its performance and memory layout
differ. The programmer writes and uses a `const` table identically to any table and cannot
tell which representation was chosen.

### A.1 What `const` licenses

A `const` table is **deeply immutable** (variables spec §3): its key set can never change
(as if permanently non-growable), and its contents never mutate. This is `const`'s own
compile-time guarantee, not a runtime seal. Two facts follow, and they are the
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

- **`foreach (k => v in constTab)`** must yield real (key, value) pairs.
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

*See also:* **table-api.md** for the complete operation catalogue and the
error summary, **protocols** and **views** for how protocols apply to tables and how `->`
reaches their behavior, and the *Optional Access & Coalescing* reference for `?.`, `??`,
and `???`.
