# The `type` type

`type` is its own **primitive** type, an inline value that *is* a `typeid` (value-representation
§3). It is not a table. A `type` value names a type: `int`, `Shape`, `int | double`, `secret`. The
`@` operator produces one (`@x` is "the type of `x`"), it may be bound to a name
(`const number: type = int | double`), compared, and matched.

Making `type` a primitive rather than a table is deliberate and buys three things:

- **Trivial comparison.** A `type` is a `typeid`, a machine integer, so `@a == @b` is a single
  integer compare (value-representation §4.2), not structural table equality. Type comparison is the
  cheapest possible operation.
- **Always const.** The type universe is closed at compile time (value-representation §4.1), so a
  `type` value is inherently fixed and immutable. A table (mutable, copy-on-write) would misrepresent
  that; a primitive is const by nature.
- **Clean `@` match arms.** `match (@x) { int => ..., string => ... }` compares `type` values
  directly, a `typeid` switch, which is both fast and semantically exact.

---

## 1. `@`: the type of a value

`@value` returns the value's **current type** as a `type` value. It is uniform: `@` always means
"the type of this value," and always yields a comparable `type`.

```
@5            // int
@5.0          // double
@"hi"         // string
@someShape    // Shape
```

`@` reflects a **value**. It is protocol-blind and attribute-blind: the type it returns identifies
the value's structural type, not the protocols the value wears (§6) or any attributes on its
declaration (attributes spec), both of which are separate axes kept out of `@` so that type
comparison is never perturbed by them (`@a == @b` compares types, nothing else).

### 1.1 `@` has two roles, resolved by grammatical position

`@` occupies two roles, and **which one applies is fixed by grammatical position**, exactly as
`*` in C means multiplication in an expression and pointer in a declaration. This is a **static
parsing guarantee, not a runtime decision**: the position is known at parse time, so there is never
any runtime ambiguity about what `@` means.

- **In value position** (an expression: `let t = @x`, `f(@x)`, `@a == @b`), `@` is **reflection**:
  "the type of this value," always yielding a comparable `type`. This is the uniform meaning above.
- **In type position** (after `:`, after `as`, in a match pattern head), `@P` is a **type
  expression**: a protocol-wearer refinement (§6), "a table guaranteed to wear `P`." It does not
  reflect a value and does not produce a `type` value (§6, §5).

The two roles share the glyph but never collide, because a parser always knows which position it is
in, without consulting any type information. So `@stringBuilder` in `var x: @stringBuilder` (type
position) is the wearer refinement, while `@stringBuilder` in `let t = @stringBuilder` (value
position) is reflection on the proto value, and so yields `proto` (a proto's own type). Same text,
two meanings, disambiguated purely by position, like C's `*`.

**Within type position, `@X` is a wearer refinement only when `X` is a protocol.** Whether `X` is a
protocol is a **static** fact (protocols are declared, the type universe is closed,
value-representation §4.1, and nothing is constructed at runtime), so semantic analysis always knows
it. If `X` in `@X` (type position) resolves to a non-protocol, for example `const p = int | table;
var x: @p = ...`, where `p` is a *type*, not a protocol, it is a **compile error** ("`@` in type
position requires a protocol; `p` is a type"). The **parser stays context-free**: it parses `@X`
into one node regardless of what `X` is; **semantic analysis** then assigns the meaning (wearer
refinement, or error) once `X`'s kind is resolved. The parser never needs type information, and
because every binding's kind (protocol, type, value) is statically fixed, the analyzer always has
what it needs to decide. This is the concrete payoff of the closed type universe: an `@`
disambiguation that asks "is this operand a protocol?" is always decidable, which it would not be
if protocols or types could be constructed at runtime.

---

## 2. Declaring a type

A `type` value may be bound like any other value, so a type can be given a name:

```
const number: type = int | double;
var  x: number = 5;          // number is usable as an annotation
const same = @x == number;   // and as a value to compare against
```

Because binding a type to a name is ordinary, **type aliases fall out for free**: `const StringOrInt:
type = string | int` names a type. This is the same act the declaration forms already perform, an
`enum`, `constraint`, `protocol`, or `capability` declaration binds a type to a name (types spec);
`const name: type = <type expression>` generalizes it to any type expression, including unions and
the primitive types themselves.

A `type` binding is `const` (a type is immutable); binding a type to `var` is not meaningful and is
not allowed.

---

## 3. Structural equality of types

Type equality is **structural**, and it is decided at compile time by **canonicalization**. The
compiler reduces every type expression to a canonical form (union members sorted and de-duplicated,
aliases expanded) and assigns each canonical form exactly **one** `typeid`. So:

```
number == (int | double)      // true: same canonical type, same typeid
(int | double) == (double | int)   // true: unions are order-independent
```

`number`, `int | double`, and `double | int` are the **same** type (one typeid), so `number` is a
name for that type, not a distinct type. Comparison is therefore one integer compare, and a type
alias is pure sugar (it introduces a name, not a new type). This matches the language's other
structural rules (anonymous enums, function-type compatibility). A feature that needed a *distinct*
type from `int | double` (nominal) would use a declaration form that mints a new typeid (an `enum`,
a `constraint`), not an alias.

---

## 4. `declared`: a binding's declared type

A **value** has exactly one type (its current type, what `@` returns). What people sometimes call a
value's "supertype", the wider type it was *declared* as, is not a property of the value at all: it
is a property of the **binding**. Consider:

```
const number: type = int | double;
var n: number = 5;
```

- `@n` is **`int`**, the value's current type. The value `5` is an `int`, full stop, it has one type.
- The value came to rest in a binding *declared* as `number`. That declared type, `int | double`,
  is the binding's, not the value's: move the same `5` into a `var m: int`, and its declared type is
  now `int`, same value, different binding, different declared type.

So the current/declared pair lives at the **binding** level, and the declared type is a **static**
fact (the compiler knows every binding's declared type). The companion operator **`declared`**
exposes it:

```
@n           // int          (current type, from the value)
declared n   // int | double (declared type, from the binding, resolved at compile time)
```

Because `declared` reads the binding's declared type, it is a **compile-time lookup**, not a
per-value field: the value stores **one** typeid, and `declared` costs nothing at runtime and bloats
no value. Consequently `declared` is defined on a **binding**, not on a free-floating `type` value: a
bare `type` value has no binding and therefore no declared type. `declared` answers "what was this
binding declared as," which only a binding can answer.

**Inferred declarations are not a special case.** `declared` reports the binding's type whether it
was written or **inferred**: `var k = 3` infers `k: int`, so `declared k == int == @k` (they coincide
when inference picks the exact type); `var u = cond() ? 5 : 5.0` infers `u: int | double`, so
`declared u == int | double`. There is no distinction between written and inferred declared types;
`declared` returns whatever the binding's type is.

`@` and `declared` are companions: `@` gives what a value **is**; `declared` gives what its binding
**accepts**. For a binding whose declared type is exact (`var k: int = 3`), the two coincide
(`@k == declared k == int`); for a widening declaration (`var n: number = 5`), they differ, which is
exactly the useful case.

The name `declared` is chosen over `super` deliberately: `super` carries inheritance connotations
from other languages that Luna does not share, whereas `declared` says exactly what the operator
returns, the binding's declared type. Like `@`, `copy`, and `spawn`, `declared` is a keyword
operator, written prefix on a binding.

---

## 5. Two categories of type: `type` values vs. protocol-wearer refinements

Not everything that can appear in **type position** is a `type` **value**. There are two
categories, and keeping them distinct is what keeps `@` and comparison coherent:

- **Types that are `type` values.** Scalars (`int`, `double`, `bool`, `type` itself), `table`,
  `list`, `view`, `fn`, `stream`, `promise`, enums, **constraints** (`byte`), and **unions**
  (`int | double`), all have a `typeid`. They *are* `type` values: `@x` returns them, `==` compares
  them (a `typeid` compare, §3), and they can be bound (`const t: type = byte`). A constraint and a
  union are ordinary `type` values because canonicalization gives each a single `typeid` (§3): a
  `byte` value's type is `byte` (`@someByte == byte`), and `number == int | double`.
- **Protocol-wearer refinements.** `@P` and `@P & @Q` (§6) are the **one** kind of type-position
  construct that is **not** a `type` value. They have **no `typeid`**, because protocol-wearing is a
  **value** property (the applied-protocol set, reflected by `@@`, views spec), not a type property.
  So `@P` cannot be produced by `@` on a value, cannot be compared by `typeid` equality, and cannot
  be bound to a `type` binding. It is a **type-position refinement** that compiles to a
  protocol-membership test (§6), not a first-class type.

This split is the reason the earlier axes hold: `@` reflects the type (category 1, `typeid`s);
`@@` reflects protocols (the value property); and `@P` in type position is a **static guarantee
about `@@`**, not a member of category 1. Constraints and unions look like they might be the odd
ones out, but they are ordinary category-1 `type` values (they have `typeid`s); the genuine odd one
out is the protocol-wearer refinement, precisely because protocol-wearing lives on the `@@` axis,
never in the type.

---

## 6. What a `type` value carries

A `type` is a `typeid` indexing the `typetable` (value-representation §4), so "what a `type`
exposes" is "what reflection over a type reveals." This is **tiered**: cheap, `typeid`-level facts
are available at **runtime**; deep structural reflection is **comptime-only**, mirroring the
attribute decision (attributes spec) and preserving the discipline that runtime values carry no
per-value reflection cost.

**Runtime (cheap, O(1) `typetable` lookups):**

- **Identity / comparison** (`==`), the core operation, a `typeid` compare.
- **Name**, the type's display name (`int`, `Shape`, `int | double`), for output and debugging.
- **Kind**, which declaration form or built-in category the type is (scalar, table, list, enum,
  protocol, constraint, union, ...), so reflection can branch on type category.
- **Subtype tests** (`t is u`, `t <: u`), the interval check (value-representation §4.2).
- **Nullability**, whether the type admits null (`T` vs `T?`, value-representation §2).
- **Union members**, for a union type, the set of member types (`(int | double)` yields `int` and
  `double`), which is what makes `declared n` returning a union useful: you can enumerate what it
  admits.
- **Constraint base**, for a constrained type (`byte`), its base type (`int`); the predicate itself
  is comptime-only.

**Comptime-only (the deeper reflection surface, the `comptime fn` tier of the reflection spec):**

- **Field enumeration** (`fields(t)`), for a table or record type, its fields and their types (what
  serialization and attribute-driven generation walk, attributes spec §4, reflection spec §3.2).
- **Attributes** (`attributes(t)`), readable only at comptime (attributes spec).
- **Enum variants** (`variants(t)`), for an enum type, its variant set and payload types
  (compile-time exhaustiveness uses this, match spec §9).
- **Constraint predicate** (`constraintPredicate(t)`) and other full structural detail.

The dividing line is cost: the runtime tier is a handful of integer and string lookups on the
`typetable`; the comptime tier is structural walking that, if permitted at runtime on arbitrary
values, would reintroduce the per-value reflection cost the value model avoids. So the deep
structural surface is a **comptime** capability (the `comptime fn` reflection tier), and a runtime
`type` is the cheap, comparable identity plus its immediate typetable facts (the `fn` reflection
tier).

---

## 7. Matching a table that wears a protocol

Protocol-wearing is **not** part of a value's type (§1): `@t` on a table gives its structural table
type, not the protocols applied to it, because protocols are applied to **values** (protocols spec),
so two tables of the same `@`-type may differ in the protocols they wear. Putting protocol-wearing in
`@` would break type comparison (two same-shape tables, one wearing a protocol, would compare
unequal). So protocol-wearing lives on its **own axis**, reached by `@@` (protocol reflection), not
`@`.

Therefore matching "a table wearing protocol `P`" is **not** a match on `@t`. It is a match against
the **wearer refinement** `@P` (§5, §1.1: in type position, "a table guaranteed to wear `P`"), and
matching a wearer-refinement pattern is defined as a **protocol-membership test** (does the value
wear `P`, an `@@` check), not a `typeid` equality:

```
match (x) {
  @stringBuilder => x->stringBuilder.append("!"),   // x is guaranteed to wear stringBuilder here
  @otherProto    => ...,
  _              => ...,
}
```

The arm `@stringBuilder` does **not** compile to `@x == @stringBuilder` (that is false, `@x` is a
table type, not a wearer refinement). It compiles to "does `x`'s protocol set include
`stringBuilder`," an `@@`/`is`-protocol test. So protocol-wearing stays out of `@` (comparison stays
clean), and `match` reaches it through wearer-refinement patterns.

**Matching a wearer refinement narrows `x` to a table guaranteed to wear `P`, reached through
`->`.** This is the key point where the two spaces stay separate (views spec §1): the narrowed `x`
is still a **table**, so `.` on it is still **element** access; the protocol's meta functions are
reached through `->`, exactly as everywhere else. What the match guarantees is that `x->P` is
**present** (not `undefined`), so `x->stringBuilder.append("!")` is safe without `?.`. The match does
**not** turn `x` into a `view` (a `view` is what `x->stringBuilder` *produces*, and on a view `.` is
a meta call, views spec §3.1); it narrows `x` to a table whose `->P` is guaranteed. So the arm body
uses `->` to reach the meta, never bare `.`. Matching a protocol is therefore a test plus a
presence guarantee, not a switch of `.` into meta space.

### 7.1 A match pattern's test depends on the pattern's kind

Section 7 is one instance of a general rule: a type pattern in a `match` arm is **not** always a
`typeid` equality. The compiler knows each pattern's **kind** statically and emits the appropriate
test:

- a **concrete type** (`int`, `Shape`), a `typeid` compare (a fast switch),
- a **wearer refinement** (`@stringBuilder`), a protocol-membership test (`@@`), narrowing to a
  table with `->P` guaranteed present (not to a view),
- a **constraint** (`byte`), the constraint predicate (constraints spec),
- a **union** (`int | double`), "is the current type one of the members,"
- an **enum variant** (`{circle ...}`), the variant-tag (refinement `typeid`) check (enum spec §8).

The **syntax is uniform** (every arm is `pattern => result`); the **test differs by the pattern's
type-kind**, chosen at compile time. So `match` on a protocol reads like any other type-pattern arm,
while doing the right thing (a protocol-membership check) underneath.

---

## 8. Representation (implementation)

A `type` value is a `typeid` carried in the scalar word of an `lval` (value-representation §1): an
inline primitive, no pointer, no allocation. Comparison is integer equality on the `typeid`. The
runtime facts of §5 are reads of the statically-emitted `typetable` (compiler spec §7.2) indexed by
that `typeid`. Because the type universe is closed (value-representation §4.1), every `type` value is
one of a fixed, compile-time-known set, so a `type` never needs construction at runtime; it is only
ever selected (by `@`) or named (by a `const` binding). The comptime tier of §6 is served by the
compiler's `typeinfo`/IR, not by any runtime structure.

---

## 9. Open questions

- **`declared` on nested and destructured bindings.** How `declared` behaves for a binding reached
  through destructuring or for a field of a declared table (whether it reports the enclosing
  declaration's element type or something finer), pending use. The written-vs-inferred case is
  resolved (§4: `declared` reports the binding's type either way); only nested and destructured
  forms remain open. (The comptime reflection surface itself, `fields`, `variants`, `attributes`,
  `constraintPredicate`, is specified in the reflection spec.)
