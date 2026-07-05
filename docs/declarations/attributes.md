# Attributes

An **attribute** is a static, named data tag attached to a **declaration**, carrying a small table
payload, and readable **only at compile time**, by comptime (functions spec §5). Attributes are
declaration metadata: they annotate *what a thing is for* (a field's JSON name, a route path, a
column name) without affecting the value, its type, or anything at runtime. Their purpose is to
drive **comptime code generation**, most centrally serialization: a comptime generator reads a
type's field attributes and emits a specialized serializer, so the tags are compiled in and
nothing is reflected at runtime.

```
const jsonTag = attribute ['tag' => string = ''];   // an attribute declaration (a `const`)

const User = [
  #[jsonTag('user_name')] 'name' => string,
  #[jsonTag('user_age')]  'age'  => int,
];

#[jsonTag('id')]
const currentId: string = '';
```

---

## 1. Attributes are compile-time-only

The defining property, from which everything else follows: **an attribute has no runtime
existence.** It is not stored on the value, not stored in the type the runtime carries, and not
readable at runtime. It exists only during compilation, where **comptime** can read it (§4).

This has three consequences that resolve every representation question at once:

- **No storage cost.** An attribute needs no room in the `lval` (value-representation §1), no
  pointer in a table header, no per-value metadata. Since it never exists at runtime, there is
  nothing to store. This is why attributes work uniformly on tables *and* on primitive bindings
  (`#[jsonTag('id')] const currentId: string` is fine: the attribute is consumed at compile time,
  and the `string` value carries nothing extra).
- **Attributes are absent from the type `@` returns.** `@value` yields the **structural type**,
  attribute-free (value-representation §3), so two values that differ *only* in attributes have the
  **same** `@`. Type identity is never perturbed by attributes:

  ```
  const someFn = fn (t: table, t2: table) => {
    const same = @t == @t2;      // compares structural types; attributes never affect this
  };
  ```

  If attributes were part of `@`, `@t == @t2` could be surprisingly false for two same-shape
  tables carrying different tags. They are not part of `@`, so type comparison means type
  comparison, and attributes never leak into it.
- **No effect on the type universe.** Attributes do not create types or `typeid`s
  (value-representation §4.1), so they add nothing to the type count and nothing to reflect on at
  runtime.

Reading an attribute outside a comptime context is a **compile error**, not a runtime empty
result. "Compile-time-only" is a guarantee, not a convention: the compiler rejects any runtime
attribute access, consistent with the language's error-or-nothing stance (no warnings).

**The deliberate cost.** Because attributes have no runtime presence, a value whose type is known
only at runtime cannot be reflected for its attributes. Dynamic, runtime attribute reflection is
**not** supported. This is an accepted trade: the use case is small and the cost of supporting it
(per-value metadata, or a split type descriptor, and the loss of a pure `@`) is large. Comptime
generation (§4) serves the actual need (attribute-driven serialization) without it.

---

## 2. Declaration

An attribute is declared with the `attribute` form, bound to a `const`, consistent with the other
declaration forms (`enum`, `constraint`, `capability`, `proto`, all `const X = <form>`):

```
const jsonTag = attribute ['tag' => string = ''];
const route   = attribute ['path' => string, 'method' => string = 'GET'];
const column  = attribute ['name' => string];
```

- **`attribute [ ... ]`** declares the attribute's **payload shape**, an ordinary table shape
  (tables spec) with **quoted keys** and optional **defaults** (`'tag' => string = ''`), exactly as
  a table type or a function parameter list is written. `[]` is the payload (a table), not `{}`.
  A payload field with a default may be omitted at application; one without must be supplied.
- An attribute **must** be a `const` (it is a fixed, statically-known definition, like every other
  declaration form).
- Attributes are **nominal**: `jsonTag` and another attribute with an identical payload shape are
  distinct attributes. Applying `#[jsonTag(...)]` requires `jsonTag` to be in scope (imported or
  local), like any other name (modules spec).
- The payload may be **empty** (`attribute []`) for a pure marker attribute that carries no data.

---

## 3. Application

An attribute is applied at a **declaration site** with the `#[ ... ]` prefix, immediately before
the declaration it annotates:

```
#[jsonTag('user_name')]
const name: string = '';

#[route('/users', 'POST')]
const createUser = fn (body: table): table => ...;    // functions are data, so a function
                                                       // binding is attributed like any binding
```

The payload is constructed positionally or by key:

- **Positional**: `#[jsonTag('user_name')]` supplies payload fields in declaration order.
- **Keyed**: `#[route('path' => '/users', 'method' => 'POST')]` supplies them by name (mirroring
  table construction), useful when defaults make positional ambiguous.

Fields with defaults may be omitted; `#[jsonTag()]` (or `#[jsonTag]` for an all-defaulted or empty
attribute) applies the defaults.

### 3.1 Where attributes may be applied

Attributes attach to **declaration sites**, and there are two:

- **Variable declarations** (`const`, `let`, `var`), including function bindings (functions are
  data, so an attributed function binding needs no special case):

  ```
  #[column('created_at')] const createdAt: int = 0;
  ```

- **Fields in a table literal**, annotating a field of that literal's **type**:

  ```
  const User = [
    #[jsonTag('user_name')] 'name' => string,
    #[jsonTag('user_age')]  'age'  => int,
  ];
  ```

  The attribute is metadata on that field of the **declaration**, held in the compiler's record
  of the declaration site, which comptime can read when it processes it (§4). It is **not**
  carried by the runtime table value, and it is **not** recorded in the interned type either
  (same-shape literals share one `typeid` whatever their attributes, §1, §5): "attributes stick
  to the table literal" means the *compiler's view of that declaration* records them for
  comptime, not that the runtime table, or the type, holds them.

**Multiple attributes** on one declaration stack, each in its own `#[ ... ]`:

```
#[jsonTag('created_at')]
#[column('created_at')]
const createdAt: int = 0;
```

### 3.2 Attributes are never dynamic

An attribute is fixed at the declaration and can never be added, removed, or computed later:

- There is **no** mechanism to add an attribute conditionally at compile time (no
  `#[if ...] attr`), so attributes are unconditionally present as written.
- A table key **added after the literal** (`tab['late'] = x`) has **no** attributes and cannot be
  given any: only fields written in the literal carry attributes, because only those are
  declaration sites.
- Because a table's attributes belong to the literal's *type* (fixed by the literal's syntax), a
  literal written **inside a loop** poses no analysis problem: the literal has one static type with
  fixed attributes, and each iteration produces a value of that one type. Attributes are a property
  of the written literal, not of any runtime execution, so no control-flow analysis is ever needed
  to determine them.

---

## 4. Comptime reads attributes; generation is the use

Attributes exist to be consumed by **comptime** (functions spec §5). At compile time, comptime
reflection (reflection spec: `comptime fn` queries over a statically-known `type`) can, for a
**statically-known** type:

- **Enumerate the type's fields** (`fields(t)`, reflection spec §3.2), and
- **Read each field's attributes** (each field's `attributes` table, and `attributes(t)` for a
  binding, reflection spec §3.2).

From these, a comptime function **generates specialized code**. The canonical example is JSON
serialization: a comptime generator walks a type's fields, reads each `jsonTag`, and emits a
serializer that writes the tagged names directly.

```
// Sketch: a comptime generator reads field attributes and emits a specialized serializer.
const toJson = comptime fn (T: type): fn (v: T): string => {
  // at compile time: for each field of T, read its jsonTag and build the writer
  // returns a runtime function specialized to T's tags
  ...
};

const writeUser = toJson(User);     // generated at compile time from User's jsonTag attributes
writeUser(someUser);                // runtime: no reflection; the tags are compiled in
```

Two properties make this the right model:

- **The serializer is specialized and static.** The JSON shape is baked into the emitted code at
  compile time, so runtime serialization does no attribute lookup and no reflection, it just writes
  the compiled-in structure. This is faster than runtime reflection and needs no runtime attribute
  storage.
- **It requires a statically-known type.** Generation reads attributes off a type the compiler
  knows, which is exactly the compile-time-only constraint (§1). Serializing a value whose type is
  unknown until runtime is the unsupported case, deliberately (§1).

So attributes are not read by ordinary runtime code; they are read by comptime, which turns them
into code. Ordinary runtime code then runs that generated code.

---

## 5. Relationship to the type system

Attributes sit entirely beside the type system, not inside it:

- **Not part of assignability.** Attributes are transparent to `<:` (functions spec §3.2): a
  value's attributes never restrict where it may flow. An attributed table is assignable exactly
  where the same-shape unattributed table is, and a signature never mentions attributes, a
  parameter typed `table` accepts any attributed table. You do not, and cannot, annotate a
  parameter or field type with an attribute requirement.
- **Not part of type identity (`@`).** As in §1, attributes are absent from `@`, so they never
  affect `@a == @b`, `is` tests, or reflection *of the type*. Two same-shape values are the same
  type whatever their attributes.
- **Consumed only by comptime.** The single place attributes are visible is comptime introspection
  (§4). Everywhere else, the type system and the runtime, they are invisible.

So an attribute is a compile-time sidecar on a declaration: it rides along in the source and in the
compiler's view of the declaration, feeds comptime generation, and then is gone. It changes neither
the value, nor its type, nor its assignability, nor anything at runtime.

---

## 6. Open questions

- ~~**Comptime introspection surface, and the keying problem**~~, **resolved** (reflection spec
  §3.2): attributes are read through `comptime fn` queries and nowhere else. For **named**
  declarations (typeid and declaration 1:1), `fields(t)` and `attributes(t)` on the `type` work
  directly; for **anonymous** shapes, where same-shape declarations share one `typeid` (§1), the
  bridge is the **`comptype` operator** (`comptype v`), which reads the **declaration descriptor** off the value's
  comptime **provenance** (declaration-originated values carry their declaration through copies
  and parameters; computed values carry fresh attribute-free provenance; runtime lowering erases
  it). Equality semantics are identical in both phases, provenance is invisible to `==`, for
  values and for `type` values alike; the comptime/runtime difference lives only in the
  provenance sidecar and this one API.
- **Attribute targets and validation.** Whether an attribute declaration may restrict what it can
  be applied to (only fields, only functions, only bindings), and whether misapplication or
  duplicate application of the same attribute on one declaration is an error, pending use.
- **Attributes on other declaration forms.** Whether enum variants, protocol members, or constraint
  declarations may carry attributes, beyond variable and table-field declarations, pending a
  concrete need.
- **Payload richness.** Whether an attribute payload may hold more than a flat table of scalars
  (nested tables, enum values, lists) as generation use cases grow, pending those cases.
