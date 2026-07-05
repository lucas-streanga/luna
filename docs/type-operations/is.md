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

`x is T` **always** produces a `bool`, for any value and any type. It cannot fail: a value either has
the type or it does not, and both answers are ordinary booleans. So `is` never raises, never panics,
and is safe to use anywhere a boolean is wanted (a condition, a `where` guard, a boolean expression).

This is the sharp contrast with `as` (as spec):

| | result | on mismatch |
|-|-|-|
| `x is T` | `bool` | returns `false` (never throws) |
| `x as T` | a value of type `T` | raises `TypeError` (panic) |

`x as T` is equivalent to "`x` if `x is T`, else raise `TypeError`." So `is` is the *question* and
`as` is the *assertion*: use `is` to ask, `as` to commit.

## 2. `is` tests a value against a type

The left operand is a **value**; the right operand is a **type** (a `type` expression, types spec):
a primitive (`int`, `string`, `bool`), a union (`int | string`), a constraint (`byte`), an enum, a
protocol-wearer refinement (`@P`), or any other type value. `x is T` reports whether `x`'s current
type is a subtype of `T` (whether `x` is usable as a `T`).

```
v is int              // primitive
v is int | string     // union: true if v is either
v is byte             // constraint: true if v is an int in 0..255
v is @drawable        // protocol: true if v is a table wearing the drawable protocol
```

Because Luna's type universe is statically closed and each value carries its type
(value-representation spec), the test is a cheap runtime check on the value's current `typeid`,
which is always **concrete**, a current type is never a union, so the check is the two-tier rule
of value-representation §4.2: an **interval check** when `T` is a tree node (a named type, a
constraint, an error), **member decomposition** when `T` is a union (one interval check per flat,
canonicalized member), and the worn-set membership test for a `@P` refinement (protocols §9). It
does not evaluate the value's contents beyond what the type requires (a constraint runs its
predicate, constraints §7).

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
match (x) { int n => foo(n) }                     // idiomatic
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
function (reflection spec §3.1), not by a written operator.

So the whole surface of the subtype relation is: **`is`** (does this value have this type, at
runtime), **`as`** (narrow this value to this type, or panic), **declarations** (state the
relationships), and **`isSubtype`** (compare two types, in comptime reflection). `is` is the common
one.

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
  `is` / `as` / declarations and, for comptime, the `isSubtype` reflection function.
