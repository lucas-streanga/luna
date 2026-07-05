# Enum

An **enum** is a **discriminated union** (a tagged sum type): a set of named **variants**, of
which a value is **exactly one at a time**. Each variant may carry a single **payload** value.
Enums fill the one structural gap the language otherwise has, a *tagged sum*: union types
(`int | string`) are untagged and structural, and tables are products (many fields at once),
but neither expresses "one of several named alternatives, each with its own data." That is
exactly an enum.

```
const Shape = enum {
  circle ['radius' => int],       // variant `circle`, carrying a table typed ['radius' => int]
  square ['side' => int],         // variant `square`, carrying a table
  labeled string,                 // variant `labeled`, carrying a bare string
  point,                          // variant `point`, carrying nothing
};
```

---

## 1. A value is one variant, carrying one payload

An enum value is **exactly one variant at a time**, no exceptions. This one-at-a-time property
is the defining difference from a table: a table holds *all* its fields at once (a product),
while an enum holds *one* variant at once (a sum).

A variant carries **at most one payload value**, of a declared type:

- **No payload**: `point` carries nothing (the C-style "just a name" case).
- **A table payload**: `circle ['radius' => int]` carries a table. When several pieces of data
  belong to a variant, the payload is a **table**, and it is an *ordinary* table, with quoted
  keys, table patterns, and all table machinery (§4). There is no separate "enum field" concept.
- **Any other value**: `labeled string` carries a bare `string`; a payload may be a primitive,
  another enum, a list, anything. It is one value of one type.

So the structure is **a sum of (optionally table) payloads**: the sum is at the variant level
(one tag), and any product is *inside* a variant's table payload. These nest and do not conflict:
the enum discriminates variants (sum), and a table payload holds several values (product). This
is why enums stay simple, they contribute only the tagged sum, and reuse tables for structured
payloads.

---

## 2. Declaration

A named enum is declared with the `enum` form, bound to a `const`, consistent with the other
type-declaration forms (`constraint`, `capability`, `protocol`):

```
const Shape = enum {
  circle ['radius' => int],
  square ['side' => int],
  point,
};
```

- `enum { ... }` lists the **variants**, comma-separated.
- Each variant is a **name**, optionally followed by a **payload type**. The payload type is an
  ordinary type: a table type (`['radius' => int]`, quoted keys, §4), a primitive (`string`),
  or any other type. A variant with no payload type carries nothing.
- The enum has **no per-variant `enum` keyword** and no nesting ceremony; `enum` appears once,
  and each variant is just `name` or `name payloadType`.

Variant **payload keys are quoted** (`['radius' => int]`), exactly as table literals and
patterns are (a quoted key is a literal; an unquoted name would be a variable, tables spec), so
enum payloads follow table rules with no special-casing, because they *are* tables.

### 2.1 Payload types accept the full type language

A payload type (and each field type within a table-payload shape) may be **any type**, not just
primitives. Because a payload is a value of a type, the type slot accepts the whole type
language with no enum-specific contract mechanism:

```
const Event = enum {
  logged  ['entry' => @Loggable],       // field wears a protocol (protocols spec)
  counted ['n' => byte],                 // field is a constrained type (constraints spec)
  wrapped ['inner' => Shape],            // field is another enum
};
```

So a payload or payload-field type may be a constrained type (`byte`), a protocol-wearer type
(`@SomeProto`), another enum, a table, a list, or any other type. Shape contracts on payload
fields are therefore not a separate feature, they are just the field types being drawn from the
full type language.

### 2.2 Recursive enums

A payload type **may reference the enum being declared**, which allows tree- and AST-shaped data
where each node is one of several kinds:

```
const Expr = enum {
  literal ['value' => int],
  add     ['left' => Expr, 'right' => Expr],     // sub-expressions are Exprs
  negate  ['operand' => Expr],
};
```

This works with **no special support**: the enum's name is in scope within its own body (as a
recursive function's name is in its body), and the recursion **terminates in representation**
because a table payload holds **references** to its children (tables are reference values, COW
with `&`, tables spec), not inlined copies, so a recursive value is an ordinary finite heap
structure. Recursion costs nothing extra to allow, so it is allowed; its primary use is the
specialized one of typed heterogeneous trees (parse trees, ASTs), where the static "this child
is an `Expr`" guarantee is wanted. (For nested *homogeneous* data a table suffices, and where the
static child type is not needed, an untyped `table`/`list` payload suffices; the recursive enum
is the tool when a typed heterogeneous tree is specifically wanted.) Mutually recursive enums
across module boundaries are forbidden by the module DAG (modules spec §2), so they must share a
module.

---

## 3. Construction

A variant is constructed with **braces around the variant name** and its payload, `{variant
payload}`. The braces mark an enum-variant construction, distinguishing `{nfc}` (the variant
`nfc`) from `nfc` (a variable named `nfc`):

```
let a = {circle ['radius' => 5]};       // variant circle, table payload
let b = {labeled "hello"};               // variant labeled, string payload
let c: Shape = {point};                  // variant point, no payload
```

The payload is written as an ordinary value: a table value (`['radius' => 5]`), a primitive
(`"hello"`), and so on. A no-payload variant is just `{point}`.

### 3.1 Payload shape is closed and checked, statically when possible

A variant's declared payload shape is a **closed contract**: a construction must supply
**exactly** the declared keys, each present and correctly typed, with **no extras**. This
borrows `match`'s per-field rules (present-required keys, type-checked values, match §4) but
**flips one**: where a `match` pattern is *partial* (unmentioned keys ignored), a payload
declaration is *closed*, because a declaration **defines** a complete shape while a pattern only
**probes** one. So:

```
{circle ['radius' => 5]}                        // ok: exactly the declared field, typed int
{circle ['radius' => 5, 'colour' => 'red']}     // error: 'colour' is not a declared field
{circle []}                                      // error: 'radius' is missing
{circle ['radius' => "big"]}                     // error: 'radius' must be int
```

The check runs **at compile time when the payload is statically known** (the common case, a
literal whose keys and value types are visible), so a typo'd or missing field is an error **at
the construction site**, which is most of the safety enums provide. It runs **at runtime when the
payload is dynamic** (a table built at runtime, whose exact keys are not statically known),
checked on construction with a panic on mismatch.

This differs from `match`, which is **always** runtime, not by choice but because a match
scrutinee is always a runtime value with nothing to check early. A construction, by contrast,
usually has the payload literal in hand, so checking it early is both cheap and worthwhile;
deferring it to runtime would discard information that is present and weaken the closed-variant
guarantee. So the rule is static-when-possible, runtime-when-necessary (as elsewhere in the
language), rather than runtime-only. Matching a payload remains fully partial (match §4); only
declaration and construction are closed.

### 3.2 Construction is target-typed

`{variant ...}` is **target-typed**: the enum it belongs to is inferred from the expected type,
the annotated binding, the parameter type, or the return type:

```
let d: Shape = {point};                  // target: Shape
someFn({circle ['radius' => 5]});        // target: someFn's parameter type
fn f(): Shape => {point};                // target: the return type
```

This is how the string API writes an enum default, `= {nfc}` with the parameter typed `enum
{nfc, nfd, nfkc, nfkd}` (§6). Target-typing covers the common case, construction almost always
happens at a typed site.

### 3.3 Qualifying an ambiguous variant

When there is **no target type** and the variant name is **ambiguous across enums**, the enum
is named **inside the braces**: `{Enum.variant}`. For example:

```
const Direction = enum { north, south, east, west };
const Hand      = enum { north, south };

let x = {north};              // ambiguous: Direction.north or Hand.north?
let y = {Hand.north};         // qualified: unambiguously Hand's north
```

The braces remain the sole construction form, so qualification does not introduce a second way
to construct; `{north}` and `{Hand.north}` are the same construction with an optionally-qualified
variant name. The `.` in `{Enum.variant}` is **variant selection on the enum type**, a different
operation from field access (`value.field`), but it is fenced **inside the construction braces**,
so the two uses of `.` never collide: outside braces `.` is field access, and inside a
construction (or pattern) a `Name.variant` is variant selection. This is the same fencing that
distinguishes `{north}` (a variant) from `north` (a variable). Qualification is needed only in
this no-target, name-collision case; at any typed site, target-typing (§3.2) resolves the variant
without it.

---

## 4. Matching and destructuring

An enum is discriminated with **`match`** (match spec), which is its natural and primary
consumer. A variant pattern is `{variant payloadPattern}`, and when the payload is a table, the
payload pattern is **exactly the match table pattern** (match §4):

```
match (s) {
  {circle ['radius' => r]} => area(r),     // circle tag; payload table matched, r bound
  {square ['side' => n]}   => n * n,
  {labeled name}           => name,          // string payload bound to name
  {point}                  => 0,             // no payload
}
```

- `{circle ['radius' => r]}` matches the `circle` variant and destructures its table payload with
  the ordinary table pattern `['radius' => r]`. No enum-specific destructuring exists; the
  payload is a table, so it uses table patterns.
- `{labeled name}` binds a non-table payload (here a string) to `name`.
- `{point}` matches a no-payload variant.

Because a value is one variant at a time, `match` over the variants is the way to both
discriminate (which variant) and extract (its payload) in one step. `{...}` (enum) is distinct
from `[...]` (table) in a pattern: `['type' => "move"]` matches a **table**, `{circle [...]}`
matches an **enum variant**. A pattern may qualify the variant (`{Hand.north} => ...`) the same
way construction does (§3.3), though in `match` the scrutinee's type is known, so qualification
is almost never needed.

### 4.1 Exhaustiveness over variants

A `match` over an enum is subject to the ordinary exhaustiveness rule (match §9.1): with a `_`
it is exhaustive; without one, its result type carries `| undefined`. To require every variant
be handled, `match!` (match §9.2) panics on any unhandled variant. So a closed enum handled by
`match!` is the checked, exhaustive dispatch, and `match` with a `_` is the lenient form.
(Variant coverage is not proved by the compiler; `_` or `match!` is how totality is stated, per
match §9.1.)

---

## 5. Identity: named is nominal, anonymous is structural

An **anonymous** enum is an enum type written inline, with no name, most commonly in a
signature:

```
fn normalize(str: string, form: enum {nfc, nfd, nfkc, nfkd} = {nfc}): string
```

Anonymous enums exist for **one-off closed sets**, where a named top-level declaration would be
ceremony. They matter for safety: without a cheap inline closed set, the temptation is to use a
bare `string` for a fixed set of options, which is unchecked and non-exhaustive. An anon enum
makes the closed, checked option as cheap as the magic string.

Named and anonymous enums differ in **type identity**:

- **Named enums are nominal.** `const Direction = enum {north, south}` and `const Hand = enum
  {north, south}` are **different types** despite identical variants, because they are named.
  Naming is a deliberate act of declaring a distinct type. (This matches the language's other
  named declarations, and is why domain types stay distinct.)
- **Anonymous enums are structural.** Two anonymous enums are the **same type** iff they have the
  **same variant set**, same tags and same payload types, **ignoring order** (variants are a
  set). So `enum {a, b}` and `enum {b, a}` are the same type, and a value of one is usable where
  the other is expected. Structural identity is what makes anonymous enums usable at all (a value
  can pass between functions that both write the same inline enum).

Identity rules:

- **Exact structural match, no subtyping.** `enum {a, b}` and `enum {a, b, c}` are **different**
  types, not subtype-related. An anonymous enum matches another only on the exact same variant
  set. (This keeps identity simple and avoids enum-subtyping and its exhaustiveness
  complications.)
- **A named enum is not structurally equal to an anonymous one** with the same variants. Naming
  opts into nominal identity, so a named `Foo = enum {a, b}` is distinct from an anonymous `enum
  {a, b}`. In practice this rarely bites, because construction is target-typed (§3.2): you write
  `{a}` and it becomes whatever type the site expects, named or anonymous, so you seldom hold an
  anonymous-typed value that must match a named type.

This mirrors the language's grain: structural where shapes are compared (tables, protocols,
anonymous enums), nominal where a name declares a distinct thing (constraints, capabilities,
named enums).

---

## 6. Reflection: `@` gives the variant, `match` binds its payload

`@x` gives the value's **current variant type**, the most-specific `typeid` it carries, exactly as
`@` does everywhere (`@someByte` is `byte`, not `int`; `@someError` is `IOError`, not `error`). It
is **not** widened to the enum type. For a **named** enum, `@x` is the variant, `Shape.circle`; for
an **anonymous** enum, `@x` is the **structural** variant (and two structurally-equal anonymous
enums still share `@`, so structural identity is preserved). This is the cheapest possible read:
the `lval` already carries the variant `typeid` (§8), so `@x` just returns it, with **no widening
step**.

Two questions, and the `is`/`match` split answers them the same way it does everywhere, identity
vs. extraction:

- **Which variant is this? (type-level, no payload.)** `@x == Shape.circle`, or the operator form
  `x is Shape.circle`, tests the specific variant; `x is Shape` tests "*any* variant of `Shape`"
  (the subtype/interval check, §8). Neither binds the payload; they only identify.
- **Which variant, *and* its payload? (value-level, binds.)** `match` discriminates the variant and
  **extracts its payload together** (`match { {circle ['radius' => r]} => ... }`), so `match` is how
  you *use* a variant, `@`/`is` is how you *test* one. Same relationship as `is` to `as`/`match`
  throughout the language.

**One idiom shifts** from the old "`@` gives the enum type" model: "is this a `Shape` at all" is now
`x is Shape` (true for every variant), **not** `@x == Shape` (which is now *false*, `@x` is the
variant). "Is it a circle" is `x is Shape.circle` / `@x == Shape.circle` / `match`. This is the same
shift constraints already made (`@x == int` is false for a `byte`; you write `x is int`), so enums
now behave under `@` exactly like every other refinement `typeid`.

`@@` (protocol reflection) is **not** used for enum variants: an enum is not a protocol, and the
variant is not a protocol member, so overloading `@@` would conflate two different concepts.
Recovering the **enum** type from a variant `typeid` (e.g. `Shape` from `Shape.circle`) is a
reflection query deferred for now (§9).
`@@` stays protocol-only; the variant is a runtime tag for `match`.

---

## 7. Binding rules

An enum **value** follows the ordinary binding rules (variables spec), with one clarification
about what "mutate" means for a sum:

- **`const c = {circle ['radius' => 5]};`** , frozen: cannot rebind, cannot switch variant,
  cannot mutate the payload.
- **`let c = ...;`** , the **variant is fixed** (switching variant is not allowed), but the
  variant's **payload is interior-mutable** if the payload is itself mutable (for example a
  mutable table payload's contents may change), consistent with `let` on a table.
- **`var c = ...;`** , may **switch variant** (`c = {square ['side' => 3]}`) as well as mutate,
  because switching variant replaces the value with a different-variant value, which needs
  rebind power.

So switching which variant a binding holds requires `var` (it is a value replacement); `let`
fixes the variant while allowing payload mutation; `const` freezes everything.

---

## 8. Representation (implementation)

An enum needs no new runtime construct. The **`lval` is already a tagged union** (a `typeid`
discriminant plus payload words, value-representation §1), which is exactly an enum's shape (a
tag plus a payload), so an enum value **is** an ordinary `lval`. This matters because the host
runtime (Go) has no discriminated-union type; the language does not need one, because the value
representation it already defines is the tagged union, and enums are one use of it.

- **The variant is a refinement `typeid`.** Each `(enum, variant)` pair is its own `typeid`, a
  **subtype of the enum type**: `Shape.circle <: Shape`, `Shape.square <: Shape`, and so on. So
  "which variant is this?" is a **subtype test** on the `typeid`, the same interval check
  (value-representation §4.2) that answers "is this error a `commandError`?" A variant is to its
  enum exactly what an error subtype is to `error`: a refinement `typeid`, assignment-compatible
  with the base, distinguished by the concrete id. `@value` yields the **variant** `typeid`
  directly (§6), a plain type-tag read with no widening; the *enum* is tested by the subtype check
  `x is Shape`. The type universe stays finite (variants are
  written in source, bounded, value-representation §4.1), so one `typeid` per variant is cheap
  (ids are indices).
- **The payload rides in the `lval` payload word.** A no-payload variant carries nothing; a
  scalar payload sits inline in the scalar word; a table payload is behind the pointer word, an
  ordinary heap `lval`. So constructing `{circle ['radius' => 5]}` sets the variant `typeid` and
  points the payload at the `['radius' => 5]` table.
- **`match` compiles to a `switch` on the variant tag.** Each arm is a case on the variant
  `typeid`; the arm's payload pattern (`['radius' => r]`) lowers to ordinary table-field access
  on the payload. Exhaustiveness is checked by the compiler (§4.1, match spec §9), not by the
  host `switch`; `match!` emits a `default` that panics, and a non-exhaustive `match` yields the
  `| undefined` result.
- **Recursive enums need nothing extra.** A recursive payload (`node ['left' => Tree, 'right'
  => Tree]`) is a table holding `lval`s that are themselves enum values, ordinary heap pointers,
  traced by the garbage collector like any other managed payload. So a tree of enums is a tree
  of heap `lval`s, and recursion falls out of "payloads are tables, tables hold `lval`s,
  `lval`s can be enums" (§2.2).

So the whole enum feature lowers onto machinery that already exists: the `lval` tagged union for
the value, refinement `typeid`s (the error-subtype mechanism) for the variant, and a host
`switch` for `match`. The host language's lack of sum types is irrelevant, because the value
representation is the sum type, built once, and enums are an instance of it.

---

## 9. Open questions

- **Parameterized enums.** An enum parameterized by a type (an `Option` over any element type
  rather than a fixed payload type) reads as generics, which the language does not have, so this
  is **out of scope** unless parametric types are ever adopted.
- **Enum-recovery reflection.** Now that `@x` yields the *variant* `typeid` (§6), recovering the
  **enum** type from a variant (e.g. `Shape` from `Shape.circle`) needs a reflection query. The
  likely form is a general `baseOf(t: type): type` returning the immediate refinement parent, which
  would serve enum variants (`Shape.circle` → `Shape`), constraints (`byte` → `int`), and error
  subtypes (`IOError` → `error`) uniformly, since all three are interval-refinement `typeid`s; a
  narrower enum-specific `enumOf` is the alternative. **Deferred**: the need is real but the
  choice of general-vs-specific (and the reflection surface) is not yet settled (reflection spec).
