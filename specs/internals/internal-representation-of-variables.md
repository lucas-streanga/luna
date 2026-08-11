# Value Representation

How Luna stores variables at runtime: the `lval`, the `typetable`, and where each
piece of per-value, per-type, and per-binding state lives.

---

## 1. The `lval`

Every variable is an `lval`, logically 16 bytes (the physical Go-hosted layout is 24 bytes,
§1.1).

```
struct lval {
  uint64_t typeAndFlags;   // 8 flag bits + 8 string-inline bits + 48 typeid bits
  void*    dataPtr;        // pointer to payload, the payload itself, or up to 8 inline string bytes
};
```

- **`typeAndFlags`** packs the per-value flags (low 8 bits, §2), an 8-bit
  string-inline control field (§2.2), and the `typeid` (high 48 bits, §3).
- **`dataPtr`** either points to the payload or *is* the payload. `int`, `double`,
  and `bool` are stored inline in the 8 bytes, no allocation, no indirection. A short
  `string` is stored inline too, up to 8 UTF-8 bytes across `dataPtr` with its length
  in the string-inline field (see string-representation, tier-1 inline). Longer
  strings and other managed types (`table`, `stream`) point to their own memory.

Copying a variable copies the `lval` (logically 16 bytes; physically 24 under the Go
hosting, §1.1) and nothing else. The payload is duplicated only when copy-on-write is
triggered by a mutation.

**Referencing an inline scalar.** A reference (`&`, or a `use`-capture of a `var`,
variables §5.1) to a heap-backed value shares the `lval`'s pointer, which already names a
stable object. An inline scalar (`int`, `double`, `bool`) has no separate object, its payload
*is* the `dataPtr` word, so a reference to it is a **pointer to the binding's `lval` slot**.
For a synchronous call this needs no allocation (the caller's frame outlives the call); only an
escaping `use`-capture **boxes** the scalar so binding and closure share one cell. The compiler
emits a pointer to the binding and defers the stack-vs-heap decision to the Go backend's escape
analysis, so no Luna-level escape analysis is required (compiler §1.4.1). A reference **shares**
the slot, it never **moves** it: a scalar is copyable and so is never *taken* (§2.1,
concurrency §2.3), and references cannot cross a spawn boundary anyway (concurrency §2.1), so a
referenced or boxed scalar is always confined to one task.

### 1.1 Logical model vs. physical layout under Go's GC

The struct above is the **logical** model: a 16-byte value whose second word is a
*discriminated union*, either an inline scalar or a pointer, chosen by the `typeid`. This is
the right way to think about the value semantically, and it is how a runtime with its **own**
garbage collector (one that could be taught to read the `typeid`) would physically lay it out.

The reference implementation, however, **hosts values on Go's garbage collector** (compiler
spec: generate Go, reuse Go's GC and goroutines). Go's GC is **precise**: for every word of
every heap object it must decide "is this a pointer to trace?", and it decides this from a
**static, per-type pointer bitmap** fixed at compile time, **without running any of our code and
without ever reading our `typeid`**. That single fact forbids the 16-byte layout, and it is
worth recording precisely so it is not re-attempted:

- The union's second word would have to be **one Go field** that is *sometimes* a scalar
  (`int` bits) and *sometimes* a pointer (`*Table`). A Go field has exactly one static
  pointer-ness. Declared as a traced pointer, the GC follows an integer as an address
  (corruption); declared as an untraced scalar, the GC fails to trace a live pointer
  (use-after-free) and cannot relocate it. There is no third choice.
- The **`typeid` cannot rescue this**, because the GC never consults it. Our tag tells *our*
  code how to read the word; the GC decides tracing from the static layout at a moment of its
  choosing, concurrently, with our code not running. A tag helps whoever reads it, and the GC
  does not read it. (This is exactly why nan-boxing / pointer-tagging works only in a runtime
  that owns its GC and can teach the collector to read the tag.)
- The tag also **cannot be hidden inside a payload word**: the scalar word is fully used by
  64-bit `int`/`double` values (no spare bits), and the pointer word is traced (so stuffing tag
  bits into it produces a non-canonical pointer the GC mis-follows).

Therefore the **physical Go layout is three separate 8-byte words** (24 bytes): the
`typeAndFlags`/`typeid` word, an always-scalar (untraced) word for inline payloads, and an
always-pointer (traced) word for managed payloads. Go may overlap *scalar-with-scalar* and
*pointer-with-pointer* (the `typeid` disambiguates `int` vs `double`, or `*Table` vs `*Stream`,
within a word, safely, since the GC treats each class uniformly), but it may **not** overlap
*scalar-with-pointer*, which is the one union this value fundamentally is. Shrinking the tag
does not help: each of the three words is independently forced, and Go rounds struct size up to
an 8-byte multiple regardless.

The 24-byte physical size does not change any semantics in this document (the flags, the
string-inline field, the `typeid`, copy-on-write, all hold identically); it changes only the
byte count of the hosted representation. Performance is recovered not by shrinking the value but
by **static unboxing** (compiler spec): where a value's type is statically known, the compiler
emits a raw Go primitive (`int64`, `*Table`) and never materializes an `lval` at all, so the
three-word value appears only at genuinely dynamic sites (`any`, heterogeneous table elements,
un-narrowed unions). A move to a self-hosted GC (compiler spec, the native-runtime alternative)
would restore the 16-byte single-word union, at the cost of writing and owning a collector.

**The escape hatches, pressure-tested (R170).** Every known way around the static bitmap was
reviewed and fails for a specific, checkable reason — recorded so none is re-attempted
piecemeal:

| Candidate | Why it fails |
|-|-|
| scalars in an `unsafe.Pointer` word | actively fatal, not merely risky: the runtime's `invalidptr` check throws on a non-pointer value in a traced slot — and a `double` whose bit pattern aliases a live span address **silently retains arbitrary objects**, a leak-by-coincidence that never reproduces |
| pointers in a `uintptr` word | the GC frees the referent under you unless something else holds it; Go's GC being non-moving *today* is explicitly non-contractual |
| NaN-boxing / low-bit pointer tagging | the LuaJIT/JSC classics are the two rows above in costume: they work only where the collector is taught to read the tag — which is the fork (below) |
| Go's own `any` interface value | *is* a 16-byte tagged union, but its payload word is always a pointer (Go ≥ 1.4): every stored `int`/`double` **allocates and boxes** (bar the runtime's 0..255 cache) — an allocation plus a pointer chase per scalar, catastrophic for numeric-heavy dynamic slots; and 48 bits of `typeid` plus flags plus the string-inline field do not fit a Go type word |
| handle/slab indirection (payload word = index into a GC-visible slab) | GC-sound at 16 bytes, but the slab pins everything it holds: freeing slots requires tracking liveness yourself — a memory manager built to avoid a memory manager, plus a double-hop on every read |
| off-heap memory (cgo / mmap) | pointers from off-heap into the Go heap are invisible to the GC — forbidden without handles (row above), with cgo overhead throughout |

**Two legal recoveries, beyond static unboxing** (both stock Go, both emitter-level):

- **The traced word goes first.** The physical field order is `{dataPtr, scalar, typeAndFlags}`
  (the §1 struct is the *logical* model; physical order is the emitter's), so the type's
  `ptrdata` is 8: the GC scans **one word in three** and the scan metadata is minimal. 24 bytes
  is also an exact Go size class, so boxed singles waste nothing.
- **Homogeneous storage specialization for tables.** The maybe-pointer union is a *per-slot*
  problem, and it vanishes when a whole array is provably one class: a list whose elements are
  all inline scalars can be stored as **noscan** parallel words (16 bytes per element; 8 for a
  monotyped numeric list, one header serving all) with **zero** GC scanning — where bulk data
  actually lives, this *beats* the 16-byte C layout. List-ness and element type are maintained
  O(1) facts (tables §2.2), Amendment A is the compile-time version of the same idea, and
  genuinely mixed tables fall back to `[]lval` at 24. The honest residue: heterogeneous dynamic
  arrays fit 2.67 `lval`s per cache line instead of 4, paid only on data that is actually mixed.

**The fork, scoped and declined (R170).** The pinned Go toolchain makes patching the GC
*possible*, and a conditional-pointer slot is what the 16-byte union needs — but it is not one
function: it permeates the per-type `gcdata` bitmaps and the `typePointers` scan iterator,
`scanobject`, **the write barrier** (emitted unconditionally on pointer stores today; a tagged
slot needs a *conditional* barrier, which is compiler codegen, not runtime code), stack scanning
and liveness maps, and the span classes. That is a permanent fork of the hottest,
most safety-critical code in Go, rebased every release, and it damages two recorded premises:
"the backend is Go source handed to the Go toolchain" (compiler §0) and the determinism that
R149's cache keying leans on. The verdict: **forking the GC is the self-hosted runtime on an
installment plan** — the moment this project owns the collector's invariants, it owns a
collector. Declined now; reserved, deliberately, beside the string refcount-completeness theorem
(string-representation §11.1, R169) for a future self-hosted backend, which would take the
16-byte union and string refcounting together.

---

## 2. Flags (low 8 bits)

The flag byte holds **only per-value dynamic state**, properties that can differ
between two `lval`s of the same type, and that change over a value's lifetime:

| Flag | Meaning |
|-|-|
| `isNull` | This value is currently null. |
| `isUndefined` | This slot currently holds `undefined` (absent). |

`isNull`, `isUndefined`, and "holds a real value" are mutually exclusive, together
they are a 3-state condition, so they cost far less than the byte allotted. The
remaining bits are reserved for future per-value state.

### 2.1 What deliberately is *not* a flag

Four properties that look flag-like belong elsewhere, because they are not
per-value:

- **Nullability** (declared `T?`) is a property of the **declared type**, identical
  for every value of that type. It lives in `typeinfo` (§4), not the `lval`. Keeping
  it in the flag byte would let it drift out of sync with the type.
- **Mutability** (`var` vs. a fixed binding) is a property of the **binding**, not
  the value. It lives in the symbol/slot. If it rode in the `lval`, copying a value
  by value would wrongly carry mutability across into a fixed slot.
- **Error-ness** is **derivable** from the `typeid`: a value is an error iff its
  current type descends from the error type. Storing it as a flag denormalizes that
  and invites disagreement with the id.
- **Moved-from** (a stream or builder transferred across a spawn boundary, concurrency
  §2.3) is a property of the **shared referent**, not of an individual `lval`. A stream
  can be aliased by several `lval`s at once, a binding and a table slot both referring to
  the one stream (stream §6.1), and a transfer must invalidate **all** of them; a
  per-`lval` flag would mark only the slot handed to `spawn` and leave the aliases live and
  dangling. So taken lives in the referent's heap state (beside the stream's cursor or
  the builder's buffer), where every alias dereferences it, the same reason error-ness lives
  in the `typeid` rather than a flag: one shared fact must not have per-slot copies that can
  disagree. `taken(x)` (concurrency §2.3) reads this referent state; **using** a taken
  value panics. This is distinct from `isUndefined`, which is a genuine per-slot fact (one
  binding is absent) and so *does* belong in the flag byte.

This keeps the flag byte to genuinely dynamic, non-derivable, per-value bits, which
is a small and slow-growing set.

### 2.2 The string-inline field (next 8 bits)

Eight bits above the flag byte are taken from the `typeid` (which drops from 56 to 48
bits, still 281 trillion types, comfortably overkill). They are meaningful only when
the `typeid` is `string`, and they encode the **tier-1 inline** case: a bit marking
the string as inline-in-`lval` rather than pointed-to, plus a 4-bit length (0..8) for
the bytes held in `dataPtr`. A short string therefore lives entirely in the `lval`
with no allocation; `""` is simply an inline string of length 0. The full mechanics
are in string-representation; the only fact the `lval` layout fixes is that these 8
bits exist and cost 8 bits of `typeid` headroom.

---

## 3. The `typeid` (high 48 bits)

The 48 id bits hold a **single** id: the value's **current** (concrete) type. A value
in motion carries only what it currently *is*, `int`, `string`, a specific error
subtype, never a union. (`string` is one `typeid` regardless of how a given string
value is represented; inline-versus-pointed-to is a representation detail carried in
the string-inline field of §2.2, not a separate type.) Unions exist only as
*declared* types, and a declared type is a property of the **location** a value
occupies, not of the value:

- **Slots, parameters, and fields** carry their declared type statically; the
  compiler knows it at every assignment site.
- **Typed containers** (e.g. a table under a protocol) carry their element
  declared-types in their own `typeinfo`.
- **Untyped containers** impose no declared type; a value dropped into an untyped
  cell simply *is* its current type there.

Because every assignment is checked against the location's declared type, always
known at compile time or from the container, the `lval` never carries a declared
union. This keeps the id a single full-width id, and lets the current field point at
the shared, plain entries for `string`, `int`, and so on, instead of minting a
narrowed variant for each declared union a concrete type appears in.

Membership is a subtype check: a current type `ioError` satisfies a declared
`string | error` because `ioError <: error`. The concrete subtype is what the value
stores; the union is what the location declares.

### 3.1 Errors are declared, not dynamic

`try` converts a thrown exception into a value, and the binding must declare that it
may hold an error, explicitly as `string | error`, or with the `!` suffix, the
error-analogue of `?`:

```luna
let myVal: string|error = try someFunc();
let myVal!:  string      = try someFunc();   // identical; ! means "or error"
```

Either way the declared type is written down and **static**. At runtime `myVal` holds
either a `string` or a concrete error subtype as its current type; asking "is it
currently an error?" is the derived check `currentType <: error` (§2.1), not a flag,
and not a stored declared union.

This mirrors nullability exactly. `?` and `!` are both **declared** capacities that
live with the type and location, while the **current** state is read per value:

| Capacity | Declared as | Current-state check | Why |
|-|-|-|-|
| nullable | `?` | `isNull` flag | a null has no content to carry a type |
| errorable | `!` | derived: `currentType <: error` | an error *has* content, a real error-typed payload |

The asymmetry, flag for null, derived check for error, is exactly why §2.1 keeps
error-ness out of the flag byte: an error value's current `typeid` genuinely *is* an
error type, so the information is already there to read.

`try` into a type that cannot hold an error, no `!`, no `| error`, is a **compile
error**, exactly as assigning `null` to a non-`?` type is. The two capacities compose
freely and order-independently: `string?!` and `string!?` both denote
`string | null | error`.

The concrete error subtype is stored the way any current type is, **as the value's
current `typeid`** in the high 48 bits. The location declares the coarse
`string | error`; the value carries the precise subtype (`ioError`, …). Because
every type is statically declared, that subtype id already exists in the `typetable`;
a throw introduces no new type (§4.1).

**The error arm of a `try` result is always the root `error`**, written `error` or
`!`, never a specific subtype. `let myVal: string | ioError = try someFunc()` is a
compile error. `try` is **not total**: it converts every **declarable-error** throw
(anything outside the `panic` subtree, errors §2) into the value, and a **`panic`
unwinds through it** untouched (errors §8.1), so a panic never dynamically lands in
the arm even though the arm's spelling, root `error`, statically includes `panic`,
the declarable category has no type of its own, so the root is the tightest honest
spelling (errors §7). Within the caught category, Luna keeps throw types opaque, a
call is known to throw, but not *what* (precise throw-set inference would have to
chase transitive calls and dies at dynamic dispatch). The compiler cannot prove
`someFunc` throws only `ioError`, so a `string | ioError` result would assert an
exhaustiveness it cannot verify.

No precision is lost. The thrown value's exact subtype is its runtime current
`typeid`, recovered by narrowing, `catch (e: ioError)`, `is ioError`, via the O(1)
subtype test (§4.2). Coarse in the declared type, precise in the value: the same split
as everywhere else.

The restriction is scoped strictly to `try`. Error subtypes are otherwise
**first-class**: declarable ones may be constructed (`ioError('disk full')`; `panic`
types are runtime-minted only, errors §9), and any error value may be stored and named
in ordinary declared types, `let e: ioError = ioError('disk full')`, or a function
that returns `ioError`. Only the arm *introduced by catching a throw* is pinned to the
root `error`, because only there is the thrown type unknown. The two `try` rules
together make that arm **exactly** `| error` (or `!`): it must be *present*, a `try`
into a non-errorable type is a compile error, since the error must be handled, and it
may not be *narrower*, since the thrown subtype cannot be known. Neither less nor
more specific than the root.

There is no `void`. Every function returns a value; a function with no `return`, or one that
specifies no return type, returns **`undefined`** (undefined spec §4), the absence sentinel, because
a void function has no meaningful result and using a result that does not exist is a bug (which
`undefined` reports by panicking on use). A void call needs no `_ =` discard, since there is nothing
to drop, and the compiler knows this statically from the `undefined` return type (undefined spec
§4.1). A throwing function whose success carries nothing therefore has success type `undefined`, and
`try f()` on it collapses the error arm as usual. This keeps a single answer to "what did this
give back?" and two distinct absences: **`undefined` for a structural absence (missing key, void
return), `null` for an explicit, chosen nothing.**

---

## 4. The `typetable` and `typeinfo`

The `typetable` is a global array of `typeinfo` structs. A `typeinfo` says how to
interpret the `dataPtr` of any `lval` bearing its id, and carries the type-level
properties:

- **Nullability.** Whether the declared type admits null. (The *current* null state
  is the `lval`'s `isNull` flag; the *capacity* to be null is here.)
- **What is *not* here: attributes.** Attributes have **no runtime existence** and are
  **not** part of a `typeinfo` or a `typeid` (attributes spec §1, §5): they are a
  compile-time sidecar on the *declaration*, consumed by comptime and then gone. Two
  declarations that differ only in attributes intern to the **same** id, an attributed
  `int` *is* `int`, and `@a == @b` is never perturbed by tags. (Consequence for
  reflection: a bare `type` value identifies attributes only where typeid and declaration
  are 1:1, named declarations; anonymous shapes are reached through the `comptype`
  operator, which reads comptime provenance, introspection spec §4.2.)
- **`valueBase`.** One precomputed `typeid`: the type's **constraint-erasure**. For a
  constraint type (constraints §9.1) it is the base's `valueBase`, so chains collapse
  transitively to the plain root (`byte` → `int`, `port` → `int`, `list` → `table`);
  for **every other** type, plain types, unions, enum variants, error types, function
  types, it is the type's **own id**, nominal identity never erases. Computed once at
  interning (the universe is closed, §4.1), immutable, one word per entry. This is what
  makes constraint-transparent `==` cheap (equality spec §1): same-typeid comparisons
  never touch it, and a typeid mismatch resolves with two indexed loads and one compare,
  or at compile time outright when the operands' static types are known. Hashing for
  `==`-keyed structures hashes (`valueBase`, payload) for the same reason (equality §1).

### 4.1 The type universe is static; interning bounds the table

**The type universe of a program is finite and fixed at compile time, and there is no
runtime type creation.** Every type any value can ever bear is written in the source or
derivable from what is written, all before execution: primitives, named declared types
(enums, constraints, protocols, error types, capabilities, each a compile-time `const X
= <form>`), and anonymous structural types (anonymous enums, function types, unions),
which are enumerable by scanning the program. Luna has **no** mechanism that manufactures
a type at runtime: no `eval`, no runtime type definition, no reflection that *creates*
types, and no generics (parametric types are out of scope), so nothing generates types
combinatorially. The set of distinct types a program uses is therefore a finite set known
after compilation.

This is the property that makes the `typeid` cheap, and it has two consequences worth
stating plainly:

- **A `typeid` is an index, not stored structure.** The 48-bit id is a small integer
  pointing into the `typetable`; the type's actual structure (a function type's
  parameters, an enum's variants, an error's ancestry) lives **once** in its `typeinfo`,
  not in any value. A million values of one type all carry the same integer id pointing at
  the same one table entry. Copying a value copies an integer; comparing types compares
  integers (or does an O(1) `typeinfo` lookup, §4.2). So a rich type system with many
  *kinds* of type (enums, constraints, protocols, capabilities, unions, refinements) costs
  nothing per value: many kinds is compile-time richness, not runtime count, and any single
  program instantiates only a small, finite, statically-known set of actual types. The
  48-bit id space (over 2^48 distinct types) dwarfs any real program's needs by many orders
  of magnitude.

- **Interning happens at compile time and the table does not grow at runtime.** Any type
  placed in the `typetable` is **interned**: the same structural type resolves to the same
  id every time (so `fn (int): string` written in fifty places, or `enum {a, b}` and `enum
  {b, a}`, each intern to one id). This deduplication is a compile-time pass; at runtime the
  table is fixed. No runtime operation appends to it, because no runtime operation creates a
  type. In particular, **throwing is not a source of new types**: `try` surfaces an error
  into a *declared* result type (§3.1), `T | error`, static and written at the binding,
  while the value's current type is a concrete error subtype, itself a statically-defined
  class. Neither is synthesized on throw. So a value's current type is always one that
  already exists in the table.

The implementation must therefore **never** perform runtime type interning (a runtime
hash of type-structure to id, with per-new-type allocation and lookup): there is nothing
to intern at runtime, because the type universe is closed at compile time. Runtime type
interning would only be needed in a language with runtime type creation or generics, which
this is not. The one bounding obligation is compile-time: intern structurally so duplicates
collapse to one id, which is cheap and bounded because programs write shallow, finitely
many types (no generics means no generative, deeply-nested type families to compare).

---

### 4.2 Inheritance and subtype tests

Errors are their own type, **not** tables, and support inheritance. Because
inheritance is a property of the *type*, it lives entirely in the `typetable`, never
in the value: each error `typeinfo` records its supertype. A value carries only its
concrete error `typeid`; every `<: error`, `catch (e: ioError)`, or `is` test is a lookup
over `typeinfo` ancestry.

Since the hierarchy is statically known, subtype tests over the **nominal tree** are
made **O(1)** by numbering the tree once at load: give each type a preorder index
`enter` and its subtree `size`, stored in its `typeinfo`. Then, for tree nodes,
`a <: b` holds iff `b.enter <= a.enter < b.enter + b.size`, two integer comparisons,
no pointer chasing.

**The interval test is scoped to the tree, and unions are deliberately not in it.**
The intervals of one preorder numbering are laminar (any two nest or are disjoint), so
they can encode only single-parent relationships, and union membership is exactly not
one: `int <: int | double` and `int <: int | string` both hold while the two unions are
incomparable, so no interval assignment can satisfy all three facts, this is the
multiple-supertype DAG the parenthetical below warns about, arising for every type with
two union supertypes. Unions therefore decide by **member decomposition** over the flat,
canonicalized member list already stored in the union's `typeinfo` (type spec §3, §5,
"union members"): `T <: (A | B | ...)` iff `T <:` some member, each an interval check;
a union on the left distributes, `(A | B) <: U` iff both members are. Canonicalization
flattens union-of-union, so members are always non-union and the decomposition never
recurses. **Intersections** (type spec §3.1) decompose dually, by **conjunction**: `x` is a
member iff it is a member of every compound atom's parts, tree atom by interval, constraint
conjunct by predicate, protocol set by the applied-set test, and **protocol-application refinements**
intern typeids exactly as unions do (identity in the table, membership never by interval;
their `typeinfo` records the canonical protocol set, their `valueBase` is `table`, type spec
§5). Honest cost: **O(1) for tree edges, O(members) for unions and intersections**, over
statically tiny lists, with pairs between statically-known types foldable at compile time
(§4.1).

**Function signatures are the second DAG, and they take the parenthetical's other escape.** The
wildcard tier stays in the interval test: function typeids occupy **one contiguous region**,
non-errorable signatures numbered before errorable ones, so `fn!` is the whole region and `fn` its
non-errorable prefix (functions §3.1's ladder is a tree: `fn (A): R <: fn <: fn!`, with
`fn (A): R! <: fn!` alone). `is fn` and `is fn!` are therefore ordinary interval checks. The
**leaves** are the problem. Signature subtyping is structural and **contravariant in the parameters**,
so `fn (int | string): (int | string)` is a subtype of both `fn (int): (int | string)` (narrow the
parameter) and `fn (int | string): any` (widen the result), and those two are incomparable: two
supertypes, no single parent, exactly the shape laminar intervals cannot encode, for exactly the
reason a union cannot be. So an interval check must **never** decide a signature target; among the
leaves the sibling relation carries no subtype information at all.

The resemblance to errors is superficial and worth refusing outright. Errors carry subtype rules too,
but they are **single inheritance by construction** (errors §2), a nominal tree declared node by node,
which is precisely why the interval numbering serves them to the leaves. Nothing about a function type
is declared: signatures are interned structurally, so there is no tree to number.

Function signatures therefore take the other option this section's parenthetical names, a **pairwise
table**. It is affordable because the type universe is closed at compile time (§4.1) and function types
are interned: a program's distinct function typeids form a finite set **F**, enumerable at link time,
and `S <: T` over them is decided once, structurally, by functions §3.2's per-position assignability,
errorability (an errorable `S` demands an errorable `T`), arity against defaults (functions §3.3.1),
each parameter **contravariantly**, each `&` position by **identity**, the result **covariantly**,
recursing into nested function positions and bottoming out on the tiers above. Store the `F × F` bits,
or memoize lazily on `(S, T)`. `F` counts *distinct written signatures*, not call sites, so it is in the
hundreds; a thousand of them is 10^6 bits, 125 KB. Runtime `x is fn (A): R` is then a single indexed
load.

Honest cost, restated: **O(1) for tree edges and for function signatures** (the latter after a link-time
`O(F² · arity)` fold), **O(members) for unions and intersections**.

Under **single inheritance** a subtype's fields are laid out as a **prefix** of its
supertype's, so a pointer to an `ioError` is already a valid pointer to `error`:
upcast to the declared type is a no-op, and the `typeid` discriminates for downcast.
(Both this layout and the interval numbering above assume a single-inheritance tree;
a multiple-inheritance DAG would require ancestor sets or a pairwise table instead.)

## 5. The `type` type and `@`

The typeof operator `@` returns the **type of a value** as a `type` value. A `type` is its own
**primitive**: an inline value that *is* a `typeid` (§3), carried in the scalar word, not a table.
So `@a == @b` is a single integer compare on the `typeid`, and a `type` is inherently const (the
type universe is closed, §4.1). The runtime facts a `type` exposes (name, kind, subtype tests,
nullability, union members) are reads of the statically-emitted `typetable` (§4) indexed by that
`typeid`; deeper structural introspection (fields, attributes, enum variants) is comptime-only. The
full model, declaration (`const number: type = int | double`), the `declared` companion (a binding's
declared type), structural type equality, and protocol matching via `@P` refinements, is in the type
spec.

---

## 6. Memory management

`lval`s are collected by the Go garbage collector. Types that own internal
allocations, `string`, `table`, `stream`, free those allocations when collected.
An error is likewise a reference value: `dataPtr` points to a heap-allocated error
instance (its own layout, not a table), traced by the GC like any other, with any
managed-type fields it holds released through their own collection. Because ordinary
copies only duplicate the `lval`, and payloads are shared until COW, a
value's managed memory has a single owner responsible for release at collection time.

### 6.1 Copy-on-write sharing bookkeeping is non-atomic

Copy-on-write needs to know, at each mutation, whether a table's storage is currently
**shared** (so a write must split it) or **sole-owned** (so a write may proceed in place).
This is tracked with the runtime's own per-table sharing bookkeeping (a reference count or
shared flag maintained on alias, copy, and drop), **separate from** Go's GC, which is a
tracing collector and exposes no such count.

**This bookkeeping is non-atomic**, ordinary integer increments and decrements, no locks, no
atomic operations, because the concurrency copy discipline guarantees a mutable table is never
shared across tasks (concurrency spec §2):

- A spawned task receives a **deep copy** of its mutable table arguments and captures, so each
  task's mutable tables (and their sharing counts) are its own.
- A **`const`** table is shared by reference, but it is deeply immutable and never split, so it
  carries **no** sharing count to contend on (COW never triggers on a `const` table). Const
  sharing therefore does not reintroduce a shared mutable count.
- A task's result **transfers** to its awaiter as a clean single-owner handoff (concurrency
  spec §2.2, §6): ownership passes from one task to another, never held live by both.

So every mutable table is **sole-owned by exactly one task at every instant**, and its sharing
count is only ever touched by that one task, single-threaded, so plain non-atomic arithmetic is
correct. The only synchronization in the whole table / COW / sharing-count machinery lives at
the spawn-copy-in and result-move-out **boundaries** (handled by the concurrency layer), and it
is on the **ownership handoff**, not on the count. The runtime interiors, refcounting, COW
splits, list-ness tracking, table representation switching, are ordinary single-threaded code.

**The contract that preserves this**: the spawn-boundary copy must be **transitively deep for
mutable data**, a shallow copy that left a nested *mutable* sub-table aliased across the
boundary would create a cross-task shared count and reintroduce the need for atomics. Const
sub-tables may stay shared (they carry no count); mutable sub-tables must be copied. Given that,
no sharing count is ever reachable from two tasks, and COW stays lock-free and atomic-free.
