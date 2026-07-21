# `is`

`is` is the **type test** operator. `x is T` asks whether the value `x` currently has type `T`, and
evaluates to a **`bool`**. It is total (it never panics), it never transforms the value, and it never
narrows the tested binding. It is the non-asserting counterpart to `as` (as spec): where `as`
*asserts* a type and panics on mismatch, `is` merely *reports* the answer.

```
const x: int | string = getIt();
x is int        // true or false, a bool
x is string     // the complement (for a two-member union)
```

---

## 1. Always a `bool`, always total

`x is T` **always** produces a `bool`, for any value and any type: a value either has the type or
it does not, and both answers are ordinary booleans. `is` has **no failure of its own** — no
mismatch panic (that is `as`), no error channel — and is safe anywhere a boolean is wanted (a
condition, a `where` guard, a boolean expression).

One precision (R178, the conversion §2 pattern): "never panics" means no `is`-**specific**
failure, not "nothing can panic while the test runs." A **constraint** target runs its predicate
(§2), and a predicate — pure by form, but purity is not totality (constraints §2.1) — can panic
*ambiently* (`i * i > 0` overflows at large `i`: an `overflowError` from inside the predicate).
Such a panic **propagates**, never swallowed into `false` — a suppressed panic would hide the
bug and make the answer a silent wrong value. Every mechanism `is` dispatches to *except* the
predicate run is panic-free and bounded (§2.1).

This is the sharp contrast with `as` (as spec):

| | result | on mismatch |
|-|-|-|
| `x is T` | `bool` | returns `false` (never throws) |
| `x as T` | a value of type `T` | raises `typeError` (panic) |

`x as T` is equivalent to "`x` if `x is T`, else raise `typeError`." So `is` is the *question* and
`as` is the *assertion*: use `is` to ask, `as` to commit.

## 2. `is` tests a value against a type

The left operand is a **value**; the right operand is a **type** (a `type` expression, types spec):
a primitive (`int`, `string`, `bool`), a union (`int | string`), a constraint (`byte`), an enum, a
protocol-application refinement (`@P`), or any other type value. `x is T` reports whether `x` is a
**member of `T`'s value set** — whether `x` would seat in a `T`-declared position ("usable as a
`T`"). For nominal tree types that coincides with `x`'s current type being a *subtype* of `T`
(`e is commandError`); for a **constraint** it deliberately does not — `200 is byte` is `true`
while `int` is not a subtype of `byte` (the relation runs the other way, `byte <: int`) — the
test is the **predicate over the base** (constraints §7); and for **`@P`** it is the applied-set
test, a value property never encoded in the typeid (type §5). One question — membership —
answered by whichever mechanism the type's shape requires.

```
v is int              // primitive
v is int | string     // union: true if v is either
v is byte             // constraint: true if v is an int in 0..255
v is @drawable        // protocol: true if v is a table with the applied drawable protocol
```

Because Luna's type universe is statically closed and each value carries its type
(value-representation spec), the test is cheap, and it is dispatched by the shape of `T`
(value-representation §4.2) — the value's current `typeid` is always **concrete**, never a union:
an **interval check** when `T` is a tree node (a named nominal type, an error type, and also bare
`fn` / `fn!`, whose ladder is a tree, functions §3.1); for a **constraint**, the **predicate over
the base** (constraints §7): the value's `valueBase` must be the constraint's base and the
predicate must hold — a current `typeid` already inside the constraint's interval is the fast
path that skips the re-run, entry-only checking having already paid it; **member decomposition**
when `T` is a union (one test per flat, canonicalized member); the **applied-set membership
test** for a `@P` refinement (protocols §6); and a **pairwise-table lookup** when `T` is a
function **signature**, whose subtyping is a DAG rather than a tree and whose relation is
therefore folded once at link time. Beyond what the type requires it never evaluates the value's
contents, and it never *runs* a function to decide `f is fn (A): R`, it compares the reified
signature (functions §3).

### 2.1 Soundness: the dispatch is static, and every cell but one is bounded (R178)

The mechanism is chosen at **compile time**, never by runtime inspection of `T`. The right
operand is a type expression (associativity §1, tier 6), type position resolves statically (the
closed universe; every binding's kind — protocol, type, value — is statically fixed, type
§1.1), so the compiler *emits* the mechanism for `T`'s shape, and no shape can fall through:
the kind set is closed, the dispatch total. A corollary worth stating: `x is t` where `t` is a
**`var` holding a `type` value** is not writable — a `var`'s kind is *value*, which cannot
appear in type position — so dynamic type-membership questions belong to introspection
(`isSubtype`, `@@`; the operators-are-language, functions-are-library split, introspection
§0). The value's side is total too: a runtime typeid is always concrete, never a union, and
every typeid has a `typeinfo`.

**Termination**: decomposition cannot recurse (canonicalization pre-flattens union-of-union at
intern time, value-representation §4.2), intervals and pairwise loads are constant-time, and
the applied-set test runs no user code. The one unbounded cell is the **constraint
predicate**: purity is enforced by form, totality is not (constraints §2.1), and a predicate
admitting pure function calls (constraints §11) can diverge. This is not an `is`-specific
exposure — the identical predicate runs at every constraint *entry* (assignment, `as`,
`match`), so a diverging predicate makes its constraint unusable everywhere; the bug is the
constraint's, and `is` adds no new surface.

## 3. `is` does not narrow

`is` is a **test only**. It does **not** change the type of the tested binding, not even inside a
branch guarded by it:

```
if (x is int) {
  foo(x);              // x still has its declared type here (e.g. int | string), NOT narrowed to int
}
```

To obtain a value of the tested type, produce a **new binding**, with `as` or a `match` arm (as spec
§7):

```
if (x is int) { const n = x as int; foo(n); }   // explicit
match (x) { n: int => foo(n) }                    // idiomatic
```

This is a deliberate consequence of the no-control-flow-analysis guarantee (compiler spec §1.4.1):
flow-narrowing (changing `x`'s type inside a branch because of a preceding `is`) would require
tracking a binding's type along control-flow paths, which Luna does not do. So `is` answers a
question and stops there; narrowing is always a separate, explicit new-binding step.

## 4. There is no separate subtype operator

`is` is the only surface form of the type-membership question, and it is deliberately the *only* one.
There is **no** type-to-type subtype operator (no `<:` in source). A raw subtype test between two *types*
(`int` subtype of `number`?) would, in a language without generics (Luna has none), always have both
operands statically known, so its answer is a **compile-time constant**, not something worth an
operator. Where subtyping matters it is compiler machinery, surfaced to the programmer through `is`
(test a value), `as` (narrow a value), and type declarations (unions, constraints, protocols). The
rare metaprogramming need to compare two types directly is served by the `isSubtype` reflection
function (introspection spec §4.1), not by a written operator.

So the whole surface of the subtype relation is: **`is`** (does this value have this type, at
runtime), **`as`** (narrow this value to this type, or panic), **declarations** (state the
relationships), and **`isSubtype`** (compare two `type` values, a runtime reflection function,
introspection §4.1). `is` is the common one.

---

## 5. Summary

- `x is T` is a **total boolean** type test: always a `bool`, never panics, never transforms, never
  narrows.
- It is the **question** form; `as` is the **assertion** form (panics on mismatch and yields the
  narrowed value).
- The left operand is a **value**, the right a **type** (primitive, union, constraint, enum,
  protocol refinement).
- It does **not** narrow the tested binding; narrowing is a separate new-binding step (`as` /
  `match`), per the no-CFA guarantee.
- There is **no `<:` operator**; the type-to-type subtype relation is compiler machinery, surfaced through
  `is` / `as` / declarations and, for two `type` values in hand, the `isSubtype` reflection function
  (introspection §4.1, the runtime tier).
