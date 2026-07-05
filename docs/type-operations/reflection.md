# Reflection

Reflection is how a program asks questions about a **type**. It is a set of built-in functions that
take a `type` (type spec) and return data about it, called by UFCS (`t.typeName()` is `typeName(t)`,
functions spec), so reflection reads like method calls on a type.

Reflection comes in **two tiers**, and the tier is exactly the `fn`-versus-`comptime fn` distinction
(functions spec §5.3):

- **Runtime reflection** (`fn (type): ...`), cheap, `typeid`-level facts (name, kind, nullability,
  subtype, union members), available on **any** `type` value, including one obtained from a value
  whose type is only known at runtime.
- **Comptime reflection** (`comptime fn (type): ...`), deep, structural data (fields, attributes,
  enum variants, constraint predicate), available only when the `type` is **statically known**,
  because these are `comptime fn`s that fold at compile time.

The tier boundary is not a special "reflection context"; it is ordinary comptime-eligibility applied
to `@`. A `comptime fn` requires compile-time-known arguments, and the runtime tier does not.

---

## 1. The two tiers

**Runtime tier (`fn`).** These are ordinary functions returning O(1) reads of the `typetable`
(value-representation §4, emitted as static data, compiler spec §7.2). Because a `typetable` lookup
is cheap and always available, they work on **any** `type` value, including `@x` where `x`'s static
type is `any` (so `@x` is only known at runtime). This is required, not incidental: without runtime
reflection on dynamic values, `any` would be a dead end (you could hold an `any` but never ask what
it is), which is footgunny. So the runtime tier makes `any` inspectable: a generic printer can ask a
value's `typeName` and `kind` at runtime.

**Comptime tier (`comptime fn`).** These walk a type's **structure** (its fields, its variants, its
attributes), which is the expensive, static-only work. They are declared `comptime fn` (functions
spec §5.3), so each call **folds at compile time** and **requires a compile-time-known `type`
argument**. This is the same tiering type spec §6 draws (cheap runtime facts vs. deep comptime
structure), now with the enforcing mechanism named: the comptime-tier functions *are* `comptime fn`s,
and that is what requires the static type.

The tiers cooperate. The cheap runtime `kind` (which category a type is) is the guard that makes the
comptime-tier calls safe: check `kind` (or `match` on it), then call the structural query that fits
that kind. The canonical use, a serializer generator, does exactly this (§4).

---

## 2. What "compile-time-known type" means

A `comptime fn` reflection call folds iff its `type` argument is a **compile-time constant**. The
criterion is about the **type**, not the **value**:

- `@x` is a compile-time constant **iff `x`'s static type is statically known**, which is the common
  case: any binding with a declared or inferred concrete type. `fields(@x)` folds for `var x: int`,
  for `const y: User`, for `let z: Shape`, because in every case the *type* (`int`, `User`, `Shape`)
  is known at compile time.
- What matters is **not** whether `x`'s *value* is `const`. A mutable `var x: int` whose value is
  computed at runtime still has the statically-known type `int`, so `@x` is `int` at compile time,
  and `fields(@x)` folds. Const-ness of the value is irrelevant; static-ness of the type is
  everything.
- `@x` is a **runtime** type value (so a `comptime fn` on it is a **compile error**) only when `x`'s
  static type is `any` (or otherwise erased), so `@x` is genuinely not known until the program runs.
  `fields(@someAny)` is a compile error: "the type is not known at compile time." (You would first
  narrow the `any` with `as`/`is`/`match` to a known type, then reflect.)

So the split is clean: **runtime-tier reflection works on any `@x`; comptime-tier reflection works on
`@x` whenever `x`'s type is statically known**, which is everything except `any`. A named type
(`fields(User)`) is of course always compile-time-known.

---

## 3. The API

All functions take a `type` and are called by UFCS. Runtime-tier functions are `fn`; comptime-tier
functions are `comptime fn`.

### 3.1 Runtime tier

```
fn typeName(t: type): string
fn kind(t: type): TypeKind
fn isNullable(t: type): bool
fn isSubtype(t: type, of: type): bool
fn unionMembers(t: type): list
fn constraintBase(t: type): type?
```

- **`typeName(t)`**, the type's display name: `"int"`, `"Shape"`, `"int | double"`. The universal
  query, used for output, logging, and generic debugging on any value (`typeName(@x)`).
- **`kind(t)`**, which category the type is, returned as a **`TypeKind`** enum (§3.3) so it is
  matchable. This is the dispatch primitive: `match (kind(t)) { ... }` branches on the type's
  category, and it is what guards the comptime-tier structural queries (call `fields` only when
  `kind` is a record-like table, `variants` only when it is an enum).
- **`isNullable(t)`**, whether the type admits null (`T?` vs `T`), a flag read (value-representation
  §2).
- **`isSubtype(t, of)`**, whether `t <: of`, the interval check (value-representation §4.2) as a
  function over two `type` **values**. The `is` and `<:` operators cover the common case on values
  and annotations; this function form is for when both operands are `type` values in hand.
- **`unionMembers(t)`**, for a union type, the list of its member types (`unionMembers(int |
  double)` is `[int, double]`); for a non-union, the single-element list `[t]`. This is runtime-tier
  because union membership is a flat `typetable` fact, not a structural walk. It makes `declared`
  (type spec §4) returning a union useful: enumerate what the binding admits.
- **`constraintBase(t)`**, for a constraint type (`byte`), its base type (`int`); `null` for a
  non-constraint. A flat `typetable` read. The predicate itself is comptime-tier (§3.2).

### 3.2 Comptime tier

```
comptype v                                   // operator: the comptime type of v (comptime-only)
comptime fn fields(t: type): list
comptime fn variants(t: type): list
comptime fn attributes(t: type): table
comptime fn constraintPredicate(t: type): fn
```

- **`comptype v`**, an **operator**, the bridge from a **value** to its **declaration
  descriptor**, a value of the built-in type **`comptype`**. Like `error`, the word is both
  the operator and the type's name, disambiguated by position (errors spec §3): in expression
  position it is a prefix word operator in the `copy` / `try` family; in type position it is
  the type. A `comptype` value is a **nominal record**, read with ordinary `.` like an error's
  fields (errors §2.1), **not** a table and not a subtype of one (so it can never leak into
  runtime through a `table`-typed slot):

  ```
  (comptype v).type          // type: exactly @v, the ordinary, runtime-identical type
  (comptype v).fields        // list: the declared members, in the fields() row shape below
  (comptype v).attributes    // table: the declaration's attributes (attributes spec)
  ```

  `fields` and `attributes` are drawn from the value's **originating declaration** (a table
  literal's annotated field list, or a protocol's declaration for a `@P` value). This is what
  makes attributes reachable for **anonymous** shapes: the *type* of a bare table literal has
  no fields and no attributes (no shape types; same-shape literals intern to one `typeid`,
  attributes spec §1, value-representation §4), but the *declaration* has both, and `comptype`
  reads the declaration. Because `comptype` is a type, a generator can **demand** it in its
  signature, `const toJson = comptime fn (ct: comptype): string`, called as
  `toJson(comptype User)`, which is what gives the serialization pipeline a typed
  intermediate.

  **`comptype` is comptime-confined.** No `comptype` value exists at runtime: the operator is
  a **compile error** outside a comptime context, and, stronger, the confinement rides the
  *type*, a value of type `comptype` may not flow into anything that survives lowering, a
  runtime `const` splice, a stored table element, an `any`-typed slot that outlives comptime
  evaluation, each a compile error at the offending point. The rule-on-the-type is what makes
  "no runtime existence" airtight where a rule on the operator alone would leak through
  `any`. Equality on `comptype` values is **structural over its three fields**, so it is
  attribute-**inclusive** by construction, the right meaning for asking whether two
  declarations share one shape, while `type` and value equality stay exactly as at runtime
  (below).

  **How the value finds its declaration: provenance.** A comptime type holds strictly more
  information than the runtime type, they are separate things joined by one bridge, and the
  extra information rides the **value during comptime evaluation** as a provenance tag: a value
  originating from a declaration carries that declaration; **copies preserve** the tag (so a
  value passed through `comptime fn` parameters still knows its declaration, which is what
  makes generator pipelines work); **computed** values (`merge(a, b)`, a freshly built table)
  carry a fresh, attribute-free provenance; and **lowering to runtime erases** the tag
  entirely (the same erasure discipline as any attribute access, attributes spec §1).

  **Equality is untouched, in both phases, deliberately.** Provenance is invisible to `==`:
  values compare at comptime exactly as they would at runtime, and `type` values compare by
  `typeid` at comptime exactly as at runtime, no phase-dependent equality anywhere. The
  honest consequence, stated rather than hidden: `comptype` is **not a function of the
  value alone**, two `==`-equal values may yield different descriptors, because it reads the
  value's provenance, not its content. That is precisely why it cannot exist at runtime, and
  it is confined to this one introspective operator and its confined type.

- **`fields(t)`**, for a **protocol-typed** table (a `@P` type, protocols §5.4), its **declared
  element members**, as a **list of tables**, each of shape `['name' => string, 'type' => type,
  'attributes' => table]`. Fields come from the protocol's declaration, the only place a table's
  per-key types are declared (Luna has **no shape types** and no record type, so a **bare `table`**
  has no declared fields and `fields(table)` is the **empty list**). This is the primary
  serialization primitive: walk the fields, read each field's type and attributes, emit code.
  `comptime` because it walks structure and reads attributes (which are compile-time-only,
  attributes spec).
- **`variants(t)`**, for an enum type, its variants, as a **list of tables**, each of shape
  `['name' => string, 'payloadType' => type?]` (payload `null` for a payload-less variant). Drives
  exhaustive code generation over an enum.
- **`attributes(t)`**, the attributes attached to the declaration a **named** type came from, as
  a **table** keyed by attribute (attributes spec §4). Comptime-only, exactly as attributes are.
  Well-defined only where `typeid` and declaration are **1:1**, named declarations (protocols,
  errors, enums, constraints); on an anonymous structural type it is a **compile error** whose
  message directs to the `comptype` operator, since same-shape anonymous declarations share one
  `typeid` and the type value alone cannot say which declaration is meant (attributes spec §6).
  `fields(t)` has the same named-only scope for the same reason; the `comptype` operator is the
  general form of both.
- **`constraintPredicate(t)`**, for a constraint type, its predicate as a callable. The reflection
  call is `comptime` (getting the predicate needs a static type), but the **returned predicate is a
  comptime-*eligible* plain `fn`**, not a declared `comptime fn`, so it **exists at runtime** and may
  be run at **both** comptime (to fold a validation) and runtime (to validate a runtime value). A
  constraint's predicate is pure and `use`-free (constraints spec), which is why it is eligible and
  runnable anywhere. You cannot introspect the predicate's **body** (reflection returns the function,
  not its source), which is fine, you never need the body; you run it. This is the general shape of a
  constraint: a predicate function the runtime auto-runs, now handed to you to run yourself.

All comptime-tier returns are **ordinary tables and lists** (not a dedicated reflection type), so
they are inspected, iterated, destructured, and matched with the normal table and list machinery.
Reflection produces data; the data is tables, because tables are the general structure (tables
spec).

Reflection result tables have **ordinary value semantics**, and mutating one is **harmless**: a
result *describes* a fixed compile-time fact, and there is no live link from the description back to
the type (types are immutable and the type universe is closed, value-representation §4.1). So
mutating a reflection result mutates a local value copy and changes **no** type; it does not, and
cannot, dynamically modify a type at runtime. No freezing mechanism is needed, value semantics
already make the results safe, because there is nothing behind them to modify.

### 3.3 The `TypeKind` enum

`kind` returns a matchable enum so reflection code can dispatch on a type's category:

```
const TypeKind = enum {
  scalar, string, table, list, view, fn, stream, promise, capability,
  enumType, protocol, constraint, union, typeType, ...
};
```

Variants are named to avoid the keywords they describe (`enumType`, not `enum`; `typeType`, not
`type`). `kind` lets a generic reflection routine branch safely: check the kind, then call only the
structural query that fits it.

### 3.4 Reflecting a protocol

A **protocol** is a first-class `proto` value (protocols spec), so reflecting it is well-defined,
unlike reflecting a wearer refinement (§3.5). Protocol reflection queries take a `proto`, not a
`type`:

```
fn protoName(p: proto): string
comptime fn elementMembers(p: proto): list
comptime fn metaMembers(p: proto): list
comptime fn metaFunctions(p: proto): list
fn declaresIdentityEquality(p: proto): bool
fn hasApply(p: proto): bool
```

- **`protoName(p)`**, the protocol's name (runtime-tier; the same name `protoName` already provides,
  views spec §7.3).
- **`elementMembers(p)`**, the element members the protocol declares (protocols spec §5), each
  `['name' => string, 'type' => type]`. These are the public, element-space members, the same space
  `fields` reports on a table.
- **`metaMembers(p)`**, the protocol's declared meta members (its protocol-private state, protocols
  spec §3.3), each `['name' => string, 'type' => type]`. This reflects the **declaration** of the
  private state, never any table's **values**.
- **`metaFunctions(p)`**, the protocol's meta functions (its behavior), each `['name' => string,
  'signature' => type]` where the signature is the `fn` type.
- **`declaresIdentityEquality(p)`**, whether the protocol declares `identityEquality` (equality spec
  §4.4), a cheap fact.
- **`hasApply(p)`**, whether the protocol has an `apply` meta function (protocols spec §4), a cheap
  fact bearing on errorability of application.

Two boundaries make protocol reflection safe for encapsulation:

- **It is keyed on the protocol, not the wearing table.** Meta members are the same for every table
  wearing a protocol (they are the protocol's declaration), so they are reflected off the `proto`,
  reached from a value via `@@t` (views spec §7: the protocols a table wears), then reflected. You do
  not reflect meta members off a table.
- **It exposes declarations, never values.** `metaMembers(p)` tells you a protocol declares
  `meta buffer: bytes`; it does **not** expose any table's actual buffer contents. Meta *values* stay
  private, reachable only inside the protocol's own meta functions (protocols spec §3.3). Reflection
  sees the protocol's public *shape*, not any wearer's private *state*. This is deliberate: deep
  introspection of private state is exactly what encapsulation exists to prevent, so reflection stops
  at the declaration boundary.

### 3.5 Wearer refinements (`@P`) and views are reflected by decomposition, not directly

A wearer refinement `@P` and a `view` value are **not** in the domain of type reflection, because
`@P` is **not a `type` value** (type spec §5): it has no `typeid`, so `typeName`/`kind`/`fields` do
not take it. This is not a gap; it is a consequence of the type model, protocol-wearing lives on the
`@@` axis, not the `@` axis, so type reflection structurally cannot apply to it.

Reflecting "a table wearing `P`" therefore **decomposes** along the two axes it actually is:

- **The table** is reflected by ordinary type reflection: `@t` gives the table type and `kind(@t)`
  is `table`. A bare `table` type has **no declared fields** (no shape types), so `fields(@t)` is
  the **empty list**; a table's per-key types are declared by a protocol, so they surface through
  the protocol reflection below, not through `@t`.
- **The protocols** are reflected via `@@t` (the protocols the value wears, views spec §7), each a
  `proto` reflected by §3.4.

And to **dispatch** on whether a value wears a protocol, the tool is **`match`** (type spec §7):
matching a wearer-refinement pattern (`match (x) { @stringBuilder => ... }`) is the protocol-
membership test. So "reflect a wearer" is answered by existing machinery, `@` for the table, `@@`
for the protocols, `match` to branch on wearing, with no unified "wearer reflection" that would
recreate the category error of treating `@P` as a `type`.

---

## 4. The canonical use: comptime code generation

Reflection exists mainly to drive **comptime code generation**, most centrally serialization. The
two tiers compose: the runtime-tier `kind` dispatches, and the comptime-tier `fields`/`attributes`
do the structural work, all at compile time, emitting specialized code that carries no reflection at
runtime.

```
// Sketch: generate a JSON serializer for a statically-known type T.
const toJson = comptime fn (t: type): fn => {
  match (kind(t)) {
    table => {
      // for each field, read its jsonTag attribute (or fall back to the field name),
      // and emit code that writes  "tag": <serialize field>
      foreach (f in fields(t)) {
        let tag = f['attributes']['jsonTag'] ?? f['name'];
        // ... emit writer for f['type'] under key `tag`
      }
    }
    enumType => {
      foreach (v in variants(t)) { /* emit a case per variant */ }
    }
    // ... other kinds
  }
};

const writeUser = toJson(User);   // generated at compile time from User's fields and attributes
writeUser(someUser);              // runtime: no reflection; the shape is compiled in
```

The generated serializer is specialized and static: the JSON shape is baked into the emitted code,
so runtime serialization does no reflection and no attribute lookup. This is why the structural tier
is comptime-only, it turns type structure into code at compile time, and needs the type statically
to do so, exactly the constraint `comptime fn` enforces.

---

## 5. Relationship to the operators

Reflection functions complement, and never replace, the reflection **operators**:

- **`@x`** (type spec) produces the `type` value that reflection functions take. `@` is how you get
  a type; the functions are how you query it. `typeName(@x)`, `kind(@x)`, `fields(@x)`.
- **`@@x`** (views spec) reflects a value's **protocols**, a separate axis from type reflection. The
  functions here are about the type layer (`@`), not the protocol layer (`@@`).
- **`declared x`** (type spec §4) gives a binding's declared type, itself a `type`, so reflection
  functions apply to it too (`unionMembers(declared x)` enumerates what the binding admits).
- **`is` / `<:`** are the operator forms of `isSubtype`, preferred on values and in annotations; the
  function form is for two `type` values in hand.

So the operators produce and narrow types; the functions interrogate them. Together they are the
type-reflection surface, split by cost into the runtime tier (any value, cheap) and the comptime
tier (static type, structural).

---

## 6. Resolved and open

**Resolved this pass:**

- **`constraintPredicate` returns a runnable predicate** (§3.2): a comptime-eligible plain `fn`,
  runnable at both comptime and runtime; the body is not introspectable (never needed), only
  callable.
- **`fields` reports element (table) fields only** (§3.2); meta members are reflected off the
  **protocol** via `metaMembers`/`metaFunctions` (§3.4), keyed on the `proto`, exposing declarations
  never values (encapsulation).
- **`@P` / views are reflected by decomposition, not directly** (§3.5): the table via `@`-reflection,
  the protocols via `@@`, dispatch via `match`, because `@P` is not a `type` value.
- **Reflection results are value-semantic and safely mutable** (§3.2): mutation is harmless (no live
  link to a type), so no freezing is needed.

**Open:**

- **Field ordering and worn-member inclusion.** Whether `fields(t)` reports fields in declaration
  order (the natural choice, given tables are ordered) and whether it includes element members
  installed by *worn protocols* (which are element-space, so arguably yes) or only the type's own
  declared fields, pending use with the protocol model.
- **`elementMembers` vs `fields` overlap.** Whether a table type's fields and a worn protocol's
  declared element members are reported through one unified query or the two separate ones here,
  pending the same protocol-model use.