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
- **Clean `@` match arms.** `match (@x) { t where t == int => ..., t where t == string => ... }`
  compares `type` values directly, one `typeid` compare per arm, which is both fast and semantically
  exact. The comparison lives in a **guard** because a bare name in a pattern position is always a
  binding (match §2.1); dispatching on a *value's* type, the far commoner case, is spelled by
  matching the value with typed bindings (`match (x) { n: int => ... }`) and never reaches for `@`
  at all.

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
the value's structural type, not the protocols the value has applied (§6) or any attributes on its
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
  expression**: a protocol-application refinement (§6), "a table guaranteed to have `P` applied." It does not
  reflect a value and does not produce a `type` value (§6, §5).

The two roles share the glyph but never collide, because a parser always knows which position it is
in, without consulting any type information. So `@stringBuilder` in `var x: @stringBuilder` (type
position) is the application refinement, while `@stringBuilder` in `let t = @stringBuilder` (value
position) is reflection on the proto value, and so yields `proto` (a proto's own type). Same text,
two meanings, disambiguated purely by position, like C's `*`.

**Within type position, `@X` is an application refinement only when `X` is a protocol.** Whether `X` is a
protocol is a **static** fact (protocols are declared, the type universe is closed,
value-representation §4.1, and nothing is constructed at runtime), so semantic analysis always knows
it. If `X` in `@X` (type position) resolves to a non-protocol, for example `const p = int | table;
var x: @p = ...`, where `p` is a *type*, not a protocol, it is a **compile error** ("`@` in type
position requires a protocol; `p` is a type"). The **parser stays context-free**: it parses `@X`
into one node regardless of what `X` is; **semantic analysis** then assigns the meaning (application
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

### 3.1 Intersections: `&`

`A & B` is the **type intersection** ("all of"), the dual of `|`, and like the union it is
**structural, canonical, and total**, defined for every pair of types by normalization at
interning time (the universe is closed, value-representation §4.1). The normal form is a
canonical union of **compound atoms**, each: one nominal base atom (a node of the tree), plus
an optional **constraint conjunction** over that base, plus an optional **protocol set** (for
`table`-based atoms). Normalization rules:

- **Distribute over unions**: `(A | B) & C` is `(A & C) | (B & C)`.
- **Meet tree atoms**: identical atoms yield themselves; tree-related atoms yield the
  **lower** (`int & byte` is `byte`); atoms with disjoint bases are **uninhabited** and the
  conjunct is dropped from the surrounding union.
- **Merge refinements over one base**: constraints conjoin (`byte & even`, the type-position
  spelling of constraints §6's `where`-composition, run in canonical order, base-match
  required exactly as there, and with the same **no-implication** rule, conjuncts are
  executed, never reasoned about); protocol sets union (`@P & @Q`, a table with both applied);
  the two mix over `table` (`list & @drawable`, a list-shaped application).
- **A written type that normalizes to uninhabited is a compile error** (`var x: int &
  string`), the same stance as constraints §6's base-mismatch: you wrote an impossible
  thing, and Luna errors rather than warns. `never` is reachable only by writing `never`.
- **`fn` types never intersect.** `fn (int): int & fn (string): string` would be a
  multi-signature callable, which is **overloading**, and Luna rejects overloading by design
  (strings spec, functions spec); the intersection is a **compile error**, not `never`.
- `any & X` is `X`; `never & X` is uninhabited (hence the error above if written).

**Membership decomposes by conjunction**, the exact dual of the union rule
(value-representation §4.2): `x is (A & B)` iff `x is A` **and** `x is B`, each conjunct by
its own test (interval for tree atoms, predicate for constraints, applied-set for protocol
refinements). One consequence is free and worth stating: `(A & B) <: A` holds
**structurally** (conjunct elimination on the normal form, no solving), which does not
breach constraints §6's no-implication rule, that rule forbids reasoning about *predicate*
bodies (`0..100` implying `0..255`), while this is set algebra on the written form.

Why intersections earn their place despite the tree: meets of ordinary atoms always
**collapse** (to the lower type, or to nothing), so `&` is genuinely productive exactly on
the **multi-membership axes**, protocol sets and constraint conjunctions, which is what the
overview's `(@P & @Q) | null` always needed and what §5 now grounds.

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

## 5. Every type-position form is a `type` value, including `@P`

Everything that can appear in **type position** is a `type` value with an interned `typeid`
(§3): scalars, `table`, `list`, `view`, `fn`, `stream`, `promise`, enums, constraints (`byte`),
unions (`int | double`), intersections (§3.1), and **protocol-application refinements** (`@P`,
`@P & @Q`). The last of these follows the pattern unions established (value-representation
§4.2): **identity from interning, membership from a test that is not the interval check.** A
application refinement's `typeinfo` records its **protocol set**, canonicalized sorted and deduped
so `@Q & @P` and `@P & @Q` intern to one id, and that id is what lets `@P` do everything a
public type must: sit in a union's member list (`(@P & @Q) | null`), be bound to an alias
(`export const file = @fileDescriptor`, std.io §2), and compare by `typeid` (`@P == @P` across
modules, §3).

What stays exactly as before is the **membership semantics**, because protocol-applying remains
a **value** property (the applied-protocol set, the `@@` axis, views spec), never encoded in a
value's own `typeid`: `x is @P` and entry into a `@P`-declared position run the O(1)
**applied-set test** (protocols §9, is spec §2), never an interval containment, and `@x` on a
applying table still reports the value's type (`table`, or its constraint), never `@P`, since
`@` reads the typeid and applying is not in it. For `==` on *values*, a refinement's
`valueBase` is `table` (equality §1): applying never perturbs value equality except through
`identityEquality` (equality §4.4). So the axes hold with one fewer exception: `@` is types,
`@@` is protocols-as-data, and `@P` is a first-class type whose membership question happens to
be answered on the `@@` axis, precisely as a union is a first-class type whose membership
question is answered by decomposition rather than intervals.

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
- **Subtype tests.** On a **value**, `x is U`, the single meaning of `is` (is spec §2, §4): the
  value's current `typeid` (always concrete, never a union) against `U`, an interval check when
  `U` is a tree node, member decomposition when `U` is a union (value-representation §4.2).
  Between two **`type` values**, the `isSubtype(t, of)` reflection function (reflection §3.1);
  there is **no** type-to-type operator (is spec §4), and `<:` in these documents is notation
  for the relation, never grammar.
- **Nullability**, whether the type admits null (`T` vs `T?`, value-representation §2).
- **Union members**, for a union type, the set of member types (`(int | double)` yields `int` and
  `double`), which is what makes `declared n` returning a union useful: you can enumerate what it
  admits.
- **Constraint base**, for a constrained type (`byte`), its base type (`int`); the predicate itself
  is comptime-only.

**Comptime-only (the deeper reflection surface, the `comptime fn` tier of the reflection spec):**

- **Field enumeration** (`fields(t)`), for a **protocol-typed** table, its protocol-declared
  element members and their types (what serialization and attribute-driven generation walk,
  attributes spec §4, reflection spec §3.2). There is no record type and no shape types, so a bare
  `table` has no declared fields and `fields(table)` is empty.
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

## 7. Matching a table that has a protocol applied

Protocol-applying is **not** part of a value's type (§1): `@t` on a table gives its structural table
type, not the protocols applied to it, because protocols are applied to **values** (protocols spec),
so two tables of the same `@`-type may differ in the protocols applied to them. Putting protocol-applying in
`@` would break type comparison (two same-shape tables, one applying a protocol, would compare
unequal). So protocol-applying lives on its **own axis**, reached by `@@` (protocol reflection), not
`@`.

Therefore matching "a table with protocol applied `P`" is **not** a match on `@t`. It is a match against
the **application refinement** `@P` (§5, §1.1: in type position, "a table guaranteed to have `P` applied"), and
matching an application-refinement pattern is defined as a **protocol-membership test** (is `P` applied, an `@@` check), not a `typeid` equality:

```
match (x) {
  b: @stringBuilder => b->stringBuilder.append("!"),   // b is guaranteed to have stringBuilder applied
  _: @otherProto    => ...,                            // tested, not bound: nothing to reach through
  _                 => ...,
}
```

The type test `@stringBuilder` does **not** compile to `@x == @stringBuilder` (that is false, `@x` is
a table type, not an application refinement). It compiles to "does `x`'s protocol set include
`stringBuilder`," an `@@`/`is`-protocol test. So protocol-applying stays out of `@` (comparison stays
clean), and `match` reaches it through application-refinement patterns.

**A typed binding on an application refinement binds a table guaranteed to have `P` applied, reached
through `->`.** This is the key point where the two spaces stay separate (views spec §1): the bound
`b` is still a **table**, so `.` on it is still **element** access; the protocol's meta functions are
reached through `->`, exactly as everywhere else. What the arm guarantees is that `b->P` is
**present** (not `undefined`), so `b->stringBuilder.append("!")` is safe without `?.`. The match does
**not** turn `b` into a `view` (a `view` is what `b->stringBuilder` *produces*, and on a view `.` is
a meta call, views spec §3.1); it binds a table whose `->P` is guaranteed. So the arm body
uses `->` to reach the meta, never bare `.`. Matching a protocol is therefore a test plus a
presence guarantee, not a switch of `.` into meta space.

Note the **binder**, not the scrutinee, is what carries the guarantee. `_: @stringBuilder => ...`
tests and discards; inside that arm `x` still has its declared type, because Luna does no
flow-narrowing (as spec §7) and there is no new binding to hold the narrowed one. Reaching `->P`
requires the arm to have bound it. This is the general rule (as §7, match §2.2), not a protocol
special case.

### 7.1 A match pattern's test depends on the pattern's kind

Section 7 is one instance of a general rule: the type in a `match` arm's typed binding or type test
(`n: T`, `_: T`, match §2.2) is **not** always tested by `typeid` equality. The test is `is T`
(is spec §2), and the compiler knows the type's **kind** statically, so it emits the appropriate
one:

- a **concrete type** (`int`, `Shape`), a `typeid` compare (a fast switch),
- a **application refinement** (`@stringBuilder`), a protocol-membership test (`@@`); the *binder*
  is what carries the `->P`-present guarantee, and it is a table, not a view (§7),
- a **constraint** (`byte`), the constraint predicate (constraints spec),
- a **union** (`int | double`), "is the current type one of the members,"
- an **enum variant** (`{circle ...}`), the variant-tag (refinement `typeid`) check (enum spec §8),
- a **wildcard callable** (`fn`, `fn!`), an interval check over the function typeid region; the
  wildcard ladder is a tree (functions §3.1), so `_: fn` matches every non-errorable function and
  `_: fn!` every function at all, which makes an errorability dispatch out of two arms,
- a **function signature** (`fn (int): string`), per-position assignability (functions §3.2) against
  the value's reified signature: contravariant parameters, covariant result, `&` positions by
  identity, arity against defaults. This is the one test whose relation is a **DAG**, not a tree, so
  it is decided by a pairwise table folded at link time rather than by an interval
  (value-representation §4.2). Still O(1) at runtime, one indexed load.

The **syntax is uniform** (every arm is `pattern => result`); the **test differs by the type's
kind**, chosen at compile time. So matching a protocol reads like any other typed binding, while
doing the right thing (a protocol-membership check) underneath. The cost of a typed binding is
therefore exactly the cost of `is` for that kind, and the bind itself is free (match §2.2).

The **function signature** row is where `match` and `as` visibly part, and the divergence is the point.
`as` on a function type is *optimistic*: it may claim a signature that is not a subtype of the real
one, and pays for the claim on every call (functions §3.2). A pattern's test is `is`, which is total,
so it never claims, it **decides**. A signature-bound function therefore needs **no per-call signature
checks at all** (match §2.2), where an `as`-narrowed one carries four. `match` is the fast door as well
as the total one.

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
