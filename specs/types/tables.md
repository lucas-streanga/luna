# Tables

This document defines the concepts behind Luna's table type: keys and how they are
accessed (§3), value semantics (§4), the removal of sealing (§5), element access and
encapsulation (§6), and propagation (§7). The built-in operations that act on tables
are catalogued separately in **iterable-functions.md** and **indexable-functions.md**
(R91–R92). Behavior beyond the built-in operations is contributed by protocols: a table
is also Luna's object, an empty or data-bearing table with protocols applied. Protocols
and the `->` operator that reaches their member space are introduced in §3.3 and
specified in **protocols** (the views document is retired, R95).

---

## 1. Tables Overview

A table maps **keys** to **values**. Depending on how it is used it behaves as a
contiguous array, a hashmap, or a record/object, with no change in type and
minimal overhead versus a raw array.

```luna
var myTable = [];                 // empty table, always []
myTable = [
  'name' => 'Lucas',
  'lastName' => 'Streanga',
];                                // record / object style
myTable = [0, 1, 2, 3, 4];        // array / list style
```

Empty tables are always written `[]`. Elements are given as `value` (implicit
incrementing integer keys) or `key => value`.

### 1.1 Capacity: at most 2³¹ − 1 entries (R195)

A table holds at most **`INT32_MAX` (2,147,483,647) entries** — a stated **guarantee**, not
an implementation accident. Three precisions:

- **The cap is on entry count, never key magnitude.** Keys remain full `int64` values:
  `[5_000_000_000 => x]` is one entry and entirely legal. (For a *list*, whose keys are
  exactly `0..n-1`, the largest index is consequently also bounded by the cap.)
- **It is one cap, not per key space**: the runtime's string- and int-key indexes point into
  a single shared entries array (table-representation §1), so the mix of key types never
  changes the limit.
- **Insertion past the cap panics with `outOfMemory`**: the structure cannot allocate
  another slot, which is resource-exhaustion-shaped (errors §9's arm); at ~40+ bytes per
  entry the cap sits at ≥85 GB for one table anyway, and anything at that scale is a
  database's or a stream's job.

The guarantee is deliberately contractual in both directions: programs may rely on the limit
being this and no smaller, and the runtime may rely on it being this and no larger — which is
what keeps 4-byte entry indexes legal forever (table-representation §1, R194), halving the
index memory an 8-byte scheme would cost. The cap is industry-normal (JVM and .NET arrays and
V8 all cap near 2³¹).

---

## 2. Lists & contiguous memory

When a table has **exclusively incrementing integer keys beginning at 0**, it is a
**list**. Lists are stored internally as a contiguous block of memory (an array).

- The moment a non-integer key is introduced, the table converts internally to a
  hashmap.
- Removing elements, or adding under new integer keys, may leave the table
  contiguous even when it is no longer a strict list.

Two O(1) predicates report internal shape (both defined in
**indexable-functions.md §1**):

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
  `tab as list` (or a `l: list` typed binding in a match arm, match §2.2), which produces a
  narrowed value. `list` is a type, not a protocol, so it is written bare after the `:`, never
  `@list` (overview/types, type §1.1).

Removal leaves gaps rather than reindexing (§2.2), so a table with gaps is genuinely not a
list. To **re-compact** a gapped or keyed table into a fresh contiguous list, call
`values()` (iterable-functions §2.5), which reindexes the values to `0..m-1` and returns a
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

```luna
_ = someFn(tab);          // COMPILE ERROR: table is not implicitly a list
_ = someFn(tab as list);  // asserts tab is currently a list; typeError panic if not
_ = someFn(tab.values()); // always legal; builds a fresh list from tab's values
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

```luna
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
name; a `[expr]` target is computed), with no dataflow analysis. The const-table
representation leans on exactly this to pick an access path per site (§A.3), and the UFCS
rule leans on it to keep `tab.name(` unambiguous (functions §3.4); the rule is stated
globally here because it governs all element access.

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

### 3.3 Element space (`.` / `[]`) vs. protocol space (`->`)

`.` and `[]` reach **element space**: the table's own keyed data. A third operator,
`->`, reaches **protocol space**: the members contributed by the protocols a table has
applied (protocols §3). The two spaces are disjoint, and the operators never overlap:

- **`tab.name` / `tab['name']`** — element (data). Static or dynamic key. A miss is
  `undefined`. Assignable.
- **`tab.name(...)`** — a UFCS call of a free function (functions §3.4). The table
  operation catalogue (`map`, `filter`, `count`, `pop`, and the rest) is reached this
  way; it is **not** protocol behavior (R91) but ordinary built-in functions
  (**iterable-functions.md**, **indexable-functions.md**).
- **`tab->name`** — a protocol member (protocols §3.1): per-table state or a protocol
  function, resolved at **compile time** against the closed member space of the
  protocols in scope, qualified as `tab->P.name` when more than one declares the name.
  A bare read is `undefined` if the member's protocol is not applied; a hard use
  panics; `?->` is the explicit soft form (protocols §3.2).

Because element space and protocol space use different operators, a data key can share a
name with a protocol member without ambiguity: `tab.map` is the element under key `map`
(data, likely `undefined`), `tab.map(f)` is the built-in transform, and `tab->map` could
only be an in-scope protocol's member named `map`. Element space is flat, un-namespaced
(any string is a key), and open; protocol space is namespaced by protocol and closed at
compile time. `->` appears on the left of `=` exactly where the member grants it: writing
a `set`-granted member (`tab->visits = 4`) is legal and compile-checked (protocols §3.3);
every other `->` write is a compile error. The dispatch rules end to end, and the `@@`
protocol-reflection operator, are specified in **protocols** (§3.5, §8).

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

Consequently **no table operation mutates its receiver in place.** Every built-in
function (the catalogues) and every protocol function (protocols §2.4) returns a *new*
table (COW), and the caller decides whether to keep it or write it back. Write-back uses
a caller-side reference:

```luna
var sorted = myTable.sort();      // myTable unchanged; sorted is a new table
&myTable.sort();                  // write-back: myTable becomes the sorted result
&b->append("x");                  // the same convention through protocol space
```

No built-in or protocol function takes a `&table` receiver. The receiver is always
passed by value; `&` on the *call site* means "assign the return value back to me." The
catalogues are **iterable-functions.md** and **indexable-functions.md**.

### 4.1 Removers return the shortened table

Because a function's return value *is* the intended new table, operations that would
otherwise return a removed element instead return the **shortened table**. Read the
element first, then remove:

```luna
var x = myTable.last();           // read the element
&myTable.pop();                   // then shrink in place
```

This keeps `&`-write-back meaning one simple thing everywhere, "assign the return
value back", with no function whose return value is something other than a table.
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

(A fixed key-set that callers rely on is a **declared** invariant — a table-level
constraint, §8, or a protocol — never a runtime seal; sealing is removed, §5. A declared
shape invariant governs removal exactly as it governs addition: the constraint's
predicate runs on every mutation.)

### 4.2 `&`-write-back is ordinary assignment

Write-back assigns the function's result to the target binding, and is governed by
exactly what any assignment is governed by: the binding's mutability (variables §1) and
its declared type and constraints — `&myList.merge(...)`'s result re-enters the `list`
constraint like any assigned value (§8, constraints §7). There are no per-value flags
left for it to respect (the mutation seal is removed in §5.2, the growth seal in §5.1,
R109), so write-back is not a special operation, just assignment spelled at the call
site.

---

## 5. Sealing is removed

Earlier designs gave tables two runtime seal axes — a **mutation** seal (`freeze` /
`thaw`) and a **growth** seal (`open` / `close` / `neverOpen`). **Both are removed**, by
the same argument arriving at each in turn (§5.2 first, then §5.1, R109). A table
carries no runtime seal state of any kind: a table you hold is yours to grow, change,
and shrink (§6), and every shape guarantee worth having is a *declared* contract, never
a toggled flag.

### 5.1 Growth sealing (`open` / `close` / `neverOpen`) is removed

The growth seal let a table be closed to new keys at runtime — a three-state flag
(`open` / `closed` / `neverOpen`), a revocability distinction, and two panic types
(`OpenViolationError` on a sealed add, `InvalidOpenError` on reopening a `neverOpen`
table). **It is removed** (R109), by §5.2's own argument transferred to the growth axis:
tables are copy-on-write value types (§4), so a callee receives a copy and tasks never
share a mutable table — nobody can add a key to *your* table but you, through your own
binding or an `&`-write-back you explicitly granted. A growth seal therefore protected a
table only from its own holder: runtime state spent encoding *discipline*, in a language
whose charter is safety by construction. Every role it played has a declared owner:

- **A fixed shape as contract** is a **protocol** (protocols §2): a closed,
  compile-checked member space, where a violation does not compile.
- **An element-space invariant** — including a fixed key-set — is a **table-level
  constraint** (§8): value-carried, checked on mutation, compiler-elided where provable;
  `list` is the built-in instance. This is the "compile-time contract" the old §5 design
  note said the seal probably wanted to be.
- **The optimization claim** (`neverOpen` as "cache the shape, skip existence checks")
  belongs to `const` tables (Amendment A) for compile-known data, and to the deferred
  `toImmutable()` (§5.2.1) for runtime-built data.
- **Accidental key creation** (`tab['tpyo'] = v` growing instead of failing) is the one
  use with no zero-setup replacement: declare the constraint or apply the protocol. That
  is deliberate — an invariant you care about is declared, not toggled.

What the deletion buys: element-space operations have **no runtime errors at all**
(indexable-functions §5), `??=` / `???=` are **total** (coalescing spec), and
`&`-write-back is ordinary assignment (§4.2).

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
- **Member access** is a protocol-space concern: protocol members carry compile-time
  `get` / `set` grants (protocols §2.2). Element keys carry no per-key access at all
  (§6, R98).

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

## 6. Element access: no permissions; encapsulation lives in protocol space

Element space carries **no per-key permissions**. A table is fully readable and writable
on every key by whoever holds it — it is your own data, and there is nothing to
encapsulate from yourself:

```luna
var t = ['name' => 'Lucas'];
t.name;                 // OK: full access, always
t.age = 0;              // OK
```

An earlier model attached `get` / `set` grants to *protocol-declared element keys*, with
runtime `TableReadViolationError` / `TableMutationViolationError`, bulk-operation
`onNoGet` / `onNoSet` policies, and `canGet()` / `canSet()` predicates. **All of it is
deleted** (R98): protocols no longer declare element keys at all (R95), so there is
nothing in element space left to permission. What that model expressed lives in
**protocol space**, better: a protocol member is private by default and opts into `get`
and/or `set` as part of the protocol's contract, checked **at compile time** (protocols
§2.2, §3.1). "This field is private, that one read-only, this one read-write" is said in
the proto block, and violating it is a compile error, not a runtime event.

This keeps the language's deny-by-default stance where it belongs (capabilities are
granted; `secret` conceals; protocol members are ungranted until granted) while keeping
plain data plain: a table you build is yours.

### 6.1 Absence and `null` are two different things

- **Absent key** → reading yields `undefined`. Absence never throws; it is routine
  control flow for a table-as-hashmap. Because `undefined` is unstorable (§3.2),
  *present* keys never hold it: existence ⟺ not-`undefined`, by construction.
- **Present, `null`** → yields the stored `null`. A real, deliberate value.

These stay distinct through the access operators (`?.`, `??`, `???`, see the *Optional
Access & Coalescing* reference). `has()` answers existence in O(1). (The former third
case — present but permission-denied — is gone with the permission model, R98.)

### 6.2 Value protection is constraints, and it is value-carried

What per-key permissions could not soundly do — protect a *value's* integrity through
widening and aliasing — constraints and protocol space do:

- A **constrained table** (`list`, or any table-level constraint, §8) is checked on
  every mutation *off the value's own typeid*, whatever binding the write comes through
  (constraints §9.4). Widening never launders a violating write.
- A **protocol's state** is unreachable from element space entirely: no `.` / `[]`
  write can touch a protocol member, and every `->` write carries the member's grant
  and declared type statically (protocols §3.3). The old widen-to-`table`-and-write
  hole has no spelling.

To hand code a genuinely unconstrained, protocol-free table, derive a fresh one —
`copy` (variables §5.2) or a transformer (§7) — which is born bare.

---

## 7. Propagation: derived tables are born bare

The applied-protocol set is a property a table asserts about **itself**. A table
*derived* by a transformer (`map`, `filter`, `merge`, `prepend`, …) is a new self:

- **Identity copy** (`var b = a;`) — the same logical value: keeps `a`'s
  applied-protocol set with its per-table member state. Under COW, the first divergent
  write splits storage.
- **Derived / transformed table** — a *different* value with different contents: born
  **bare** (no applied protocols), whatever the source carried.

Both follow from one fact: a transformer reads the source's *element space* and builds a
new table from it. It never reads protocol space, so it could not reproduce a protocol's
per-table member state, and carrying a protocol without its state would be incoherent.
(Two older axes propagated here and are gone outright: per-key access grants, deleted
with the permission model, R98; and the growth seal, deleted with sealing, §5.1, R109.)

This is what makes deriving a fresh table the honest escape from a protocol's contract:
`p.map(x => x)` yields a bare table with the same element values, without un-applying
anything. Re-attaching behavior is explicit — `derived apply person(...)` (protocols §4)
— and requires re-supplying what the protocol requires, which is exactly the honesty the
bare-birth rule enforces.

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

A table-level constraint is an ordinary `constraint` declaration (constraints spec §1) with base `table`
and a pure predicate over the whole table:

```luna
const pair    = constraint t: table where count(t) == 2;
const sorted  = constraint t: table where isSorted(values(t));
const tagged  = constraint t: table where t.kind is string;
```

The predicate is **any pure boolean expression over the table** (constraints §2, purity is enforced
by the form): it may read counts, inspect keys, compare entries, or `match` the table's shape, as
the author wants. `list` is exactly `constraint t: table where <keys are 0..n-1>`, with a
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
- **Writes** (`constTab.x = v`, `constTab[x] = v`): already impossible on a deeply
  frozen `const` table (variables §3); no representation concern.

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
  operation that walks entries, all need the keys as runtime values.

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

*See also:* **iterable-functions.md** and **indexable-functions.md** for the complete
operation catalogue and the error summary, **protocols** for how protocols apply to
tables and how `->` reaches their members, and the *Optional Access & Coalescing*
reference for `?.`, `??`, and `???`.
