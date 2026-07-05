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

Attributes exist to be consumed by **comptime** (functions spec §5). The consumer is a
**generator**: a `comptime fn` that takes a **`comptype`** descriptor (reflection spec §3.2),
walks its `fields` and their `attributes`, and returns a **plain runtime function**. The
canonical example is JSON serialization, and its shape is fully expressible with no dependent
types:

```
// The generator: comptime in, plain runtime function out.
const toJson = comptime fn (ct: comptype): fn (any): json => {   // json: a string constraint, json spec §1
  // Extract what generation needs into PLAIN data: names and tags are strings.
  var cols = [];
  foreach (f in ct.fields) {
    cols->push(['key' => f.name,
                'out' => f.attributes.jsonTag?.tag ?? f.name]);
  }
  // Return the specialized serializer. It const-captures `cols` (functions §2.1),
  // ordinary strings in an ordinary table, and walks the value at runtime.
  return fn (v: any): json => {
    ...   // for each col: emit "\"${col.out}\":${serialize(v[col.key])}"
  };
};

const writeUser = toJson(comptype User);   // generated at compile time from User's jsonTags
writeUser(someUser);                        // runtime: no reflection; the tags are compiled in
```

Three properties make this the right model, and each is enforced, not hoped for:

- **The return type is `fn (any): json`, not a dependent `fn (v: T)`.** The specialization
  lives in the **captured data** (`cols`), not in the parameter's type: the emitted function
  walks any table using the compiled-in key/tag map. Luna has no type-parameterized
  signatures, and this pattern shows none are needed, the descriptor's information is
  extracted into plain values and rides an ordinary `const`-snapshot capture.
- **Confinement forces the extraction.** The generator may not capture `ct` itself into the
  returned function: a `comptype` value cannot survive lowering (reflection §3.2), and a
  closure returned from comptime is spliced into the runtime program, so `use`-less capture of
  `ct` is a **compile error** at exactly the right place. The type system, not discipline,
  guarantees that what reaches runtime is plain data.
- **The result is spliceable.** A comptime-produced `fn` value lowers to runtime exactly when
  its captured environment is confinement-free plain data (compiler spec §6), which the
  previous point guarantees. The JSON shape is baked in; runtime serialization does no
  attribute lookup and no reflection.

**The dynamic counterpart is a separate function, deliberately.** A structural,
attribute-blind serializer is trivially writable today, `toJsonDynamic = fn (v: any): json` (json spec §3),
walking the value with `foreach` and `@`: the primitive set is closed
(value-representation §4.1), so a runtime walk meets only known cases, all the tools are
there. It is **not** unified with the generator behind one name (no
`toJson(ct: comptype | any)`), for a reason stronger than taste: the two paths differ in
**observable output**, the generator honors `jsonTag`, the dynamic walk cannot (attributes are
erased, §1), and a comptime-eligible function callable in both phases must be
**phase-invariant** (functions §5.5), or folding it at comptime would change program behavior.
A `comptype`-taking function is exempt from that rule vacuously, it cannot be called at
runtime at all; a unified union function would be subject to it and would violate it. Two
names, two contracts: `toJson` is "the declaration's serialization, tags honored";
`toJsonDynamic` is "this value's structure, as it stands."

So attributes are not read by ordinary runtime code; they are read by comptime, which turns
them into plain data captured by generated code. Ordinary runtime code then runs that code.

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

## 6. Resolved, ruled, and open

### 6.1 Resolved

- **Comptime introspection surface, and the keying problem**, resolved (reflection spec
  §3.2): attributes are read through `comptime fn` queries and nowhere else. For **named**
  declarations (typeid and declaration 1:1), `fields(t)` and `attributes(t)` on the `type` work
  directly; for **anonymous** shapes, where same-shape declarations share one `typeid` (§1), the
  bridge is the **`comptype` operator** (`comptype v`), which reads the **declaration descriptor** off the value's
  comptime **provenance** (declaration-originated values carry their declaration through copies
  and parameters; computed values carry fresh attribute-free provenance; runtime lowering erases
  it). Equality semantics are identical in both phases, provenance is invisible to `==`, for
  values and for `type` values alike; the comptime/runtime difference lives only in the
  provenance sidecar and this one API.
### 6.2 Ruled (R42)

- **Attributes cannot declare targets.** An attribute declaration carries no restriction on
  what it may be applied to; any attribute attaches to any declaration site (§3.1), and
  "misapplication" therefore cannot exist, the consumer (a comptime generator) simply reads
  or ignores what it finds.
- **Payloads may be arbitrarily nested plain data.** Tables in tables, lists, enum values;
  a payload is an ordinary comptime-known value, and nothing about the machinery (§1, §4)
  cared about flatness in the first place.

### 6.3 Open

- **Duplicate application** of the same attribute on one declaration (error, last-wins, or
  a list), pending use.
- **Attributes on other declaration forms** (enum variants, protocol members, constraint
  declarations), deferred by decision pending a concrete need.
