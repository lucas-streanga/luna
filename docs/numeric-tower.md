# The numeric tower

This spec defines Luna's complete set of **numeric types** and the **widening and conversion rules**
that relate them. It builds on the operator rules (operators spec: operators are built-in only, no
overloading) and the per-type arithmetic edges (int spec, double spec). The arithmetic operators
themselves are in the operators spec; this document is the **type set and the relationships**.

Two organizing rules govern the whole tower:

1. **Widening is implicit within a family, to the widest operand.** Types that form a lossless
   widening chain (the signed integers, the unsigned integers, the floats) widen implicitly, and a
   mixed-width expression takes the widest operand's type.
2. **Crossing families is explicit.** Between signed and unsigned integers, between integers and
   floats, and between any fixed type and the arbitrary-precision types, conversion is always an
   explicit function, never implicit. Different families are different types.

Every numeric type is **built-in** (so every one has operators, operators spec §1). "Built-in" means
the language defines the type and its operators; it does **not** mean "inline in the `lval`." Some
numeric types are inline (the fixed-width integers and floats), others are heap-backed reference
types (the arbitrary-precision ones), exactly as `string`, `bytes`, and `table` are built-in yet
heap-backed. Heap allocation is an implementation detail, not a reason to make a type a library type.

---

## 1. The families

### 1.1 Signed integers

```
i8  <:  i16  <:  i32  <:  int          // int is i64, the primitive
```

`int` is the 64-bit signed primitive (int spec). The narrower widths `i8`, `i16`, `i32` are
**constraints** on `int` (int spec §6.1): a value of the narrower type is an `int` in the narrower
range, so `i8 <: i16 <: i32 <: int` by the constraint-subtype relation (constraints spec). Arithmetic
computes at `int` width and the range is checked on entry (assignment, `as`, store-back), so a value
leaving its range **panics** (int spec §6.1), it never silently wraps. Widening up the chain is
implicit and lossless; narrowing down is an explicit checked `as` that panics out of range.

### 1.2 Unsigned integers

```
u8  <:  u16  <:  u32  <:  u64
```

`u64` is the 64-bit unsigned primitive (int spec §6.2), a **separate primitive** from `int` because
its top range (2^63 to 2^64 - 1) does not fit in signed 64-bit storage. The narrower widths `u8`,
`u16`, `u32` are **constraints** on `u64`, exactly as the signed smalls are constraints on `int`, so
`u8 <: u16 <: u32 <: u64`. Same discipline: compute wide, check on entry, panic out of range, implicit
widening up, checked narrowing down.

### 1.3 Floats

```
f16  <:  float  <:  double
```

`double` is the 64-bit IEEE 754 primitive (double spec). `float` is the 32-bit IEEE 754 primitive,
and `f16` is the 16-bit IEEE 754 primitive (half precision). Each is a **separate primitive**, not a
constraint on a wider one (their values are different-precision sets, not subsets). But the family is
a **lossless widening chain**, `f16 <: float <: double`, because each narrower binary format embeds
**losslessly** in the wider one: binary32 is wider than binary16 in *both* mantissa (23 vs 10 stored
bits) and exponent (8 vs 5 bits), and binary64 is wider than binary32 in both, so every narrower value,
including subnormals, the zeros, the infinities, and NaN, is an exact value of the wider type. No
information is lost widening up the chain, so **widening is implicit** (`f16` to `float`, `float` to
`double`), on the same lossless-therefore-safe justification as integer widening (the mechanism is a
lossless representation change rather than a subset relation). Narrowing down the chain (`double` to
`float`, `float` to `f16`) is **lossy** (it rounds and can overflow to Infinity), so it is an explicit
checked narrowing (`double as float`), parallel to integer narrowing.

So the float family is a widening chain like the integer families, and the widen-to-widest rule (§2)
applies: `float + double` is `double`, `f16 + float` is `float`.

`f16` is **committed but deferred** (§6): it is used chiefly in machine-learning and GPU work
(compact storage of many low-precision values), and it slots into the family with no special
mechanism, so it is added when those domains are targeted. `float` and `double` are the current core
floats.

### 1.4 Arbitrary-precision and exact (built-in, heap-backed)

```
decimal       // arbitrary-precision base-10 (exact decimal fractions)
rational      // exact fraction (a pair of integers, kept reduced)
complex       // a pair of doubles (real, imaginary)
```

These are **built-in** types with full operator support (`+`, `-`, `*`, `/`, operators spec), and
they are **heap-backed reference types** where their value is unbounded (`decimal`) or composite
(`rational`, `complex`), stored through the `lval` pointer word like `string` and `table`. They are
built-in, not library, because the built-in-vs-library line is *who defines the type and its
operators* (the language), not *how it is stored*; heap allocation is an implementation detail (§0).
Making them built-in is what gives them operators under the no-overloading rule (operators spec §1),
which is why they are built-in rather than library: they are meant to be ergonomic and exact.

- **`decimal`**, **arbitrary-precision** base-10. Exact for decimal fractions (`0.1 + 0.2` is exactly
  `0.3`, unlike `double`), which is why it is the correct type for **currency** and any exact-decimal
  domain, its defining motivation. Arbitrary precision (not a fixed width) so that scale is never a
  surprise, which is why it is heap-backed rather than an inline primitive.
- **`rational`**, an exact fraction held as a reduced numerator/denominator pair of integers. Exact
  rational arithmetic (`1/3 + 1/3` is exactly `2/3`).
- **`complex`**, a real/imaginary pair of `double`s, with the standard complex arithmetic.

There is **no arbitrary-precision integer** (`bigint`): Luna does not provide one, because its
motivation is diffuse, `decimal` covers the concrete need for exactness (currency), and 64-bit `int`
and `u64` cover ordinary integers. A **fixed** wide integer (`int128`) is a *possible future library
type* (method syntax), not committed and not built-in, since it is rarely needed and `decimal` and the
64-bit integers cover the common cases.

The backend can implement `decimal` by wrapping Go's arbitrary-precision facilities (`math/big`'s
`big.Float` / `big.Rat`, or a dedicated decimal library), so the arbitrary-precision machinery is not
written from scratch.

---

## 2. Widening within a family

In an expression combining two values of the **same family** but different widths, the result takes
the **widest** operand's type, and the narrower operand widens implicitly (losslessly):

```
someI8 + someI32       // i32   (the i8 widens to i32)
someU16 + someU64      // u64   (the u16 widens to u64)
someFloat + someDouble // double (the float widens to double, losslessly)
```

This is safe because within-family widening loses no information (a subset relation for integers, a
lossless representation change for floats). It is the constraint-subtype relation (constraints spec)
doing the work for integers, and the lossless float embedding (§1.3) for floats. Narrowing (the
reverse) is never implicit: it is an explicit checked `as` that panics (integers out of range) or
rounds explicitly (`double as float`).

---

## 3. Crossing families is explicit

Between families, there is **no implicit conversion**; you convert with an explicit function
(conversion spec):

```
someInt + someDouble          // ERROR: different families, no implicit crossing
someInt.toDouble() + someDouble   // OK: explicit widening to double, then double arithmetic

someInt + someU64             // ERROR: signed and unsigned are different families
someInt.toU64() + someU64     // OK: explicit (and checked: a negative int cannot become u64)
```

The families that require explicit crossing are: **signed integers, unsigned integers, floats, and
each arbitrary-precision type**. Crossing any two of these is a conversion function, never an operator
widening. This is the direct consequence of no-implicit-coercion (the language never silently changes
a value's numeric family), and it is what keeps `1 == 1.0` false (equality spec §1) and arithmetic
predictable: an operator never silently reinterprets one family as another.

### 3.1 Fixed to arbitrary-precision is explicit, though lossless

Widening a fixed type to an arbitrary-precision one (`int`/`double` to `decimal`, `int` to the
numerator of a `rational`) is **lossless** (every `int` is exactly a `decimal`), so by the
within-family logic it *could* be implicit. It is nonetheless **explicit**, because the conversion has
a **representation cost** (it allocates a heap value), and Luna makes cost visible: an implicit `int`
to `decimal` widening would hide an allocation inside an innocuous-looking mixed expression. So the
rule for implicit widening is not merely "lossless" but "**lossless and cheap**"; fixed-to-arbitrary
is lossless but not cheap (it allocates), so it is an explicit conversion (`n.toDecimal()`). This is a
deliberate, principled exception: losslessness alone does not earn implicit widening when the
representation cost should be seen.

So the arbitrary-precision types are their **own** families for widening purposes: you enter them by
explicit conversion, and once in them, their arithmetic is exact and their operators are the built-in
operators (operators spec).

---

## 4. Narrowing and violations

Narrowing, moving to a narrower or lossier type, is always **explicit** and **checked**, consistent
across the tower:

- **Integer narrowing** (`int as i8`, `u64 as u8`, and cross-family `int as u64`): an explicit `as`
  that **panics** if the value is out of the target range (the constraint-on-entry check, constraints
  spec; int spec §6).
- **Float narrowing** (`double as float`): explicit, rounding to the nearer `float`, and yielding
  `float` Infinity if the magnitude overflows binary32 (an IEEE result, not a panic, double spec).
- **Arbitrary to fixed** (`decimal as int`): explicit, panicking if the value does not fit or is not
  exact (a fractional `decimal` narrowed to `int`, unless a rounding conversion is chosen).

The through-line is the language's uniform stance: **no silent wrong value.** Within-family widening
is lossless (so implicit); everything narrowing or crossing is explicit and either panics or produces
a well-defined IEEE sentinel. Overflow of the fixed integers panics (int spec §2); the
arbitrary-precision `decimal` does not overflow (it grows); the floats produce IEEE infinities and NaN
rather than panicking (double spec).

---

## 5. The complete tower

| Type | Family | Representation | Widens implicitly to | Overflow |
|-|-|-|-|-|
| `i8`, `i16`, `i32` | signed int | inline (constraint on `int`) | wider signed, up to `int` | panic |
| `int` (i64) | signed int | inline primitive | (widest signed fixed) | panic |
| `u8`, `u16`, `u32` | unsigned int | inline (constraint on `u64`) | wider unsigned, up to `u64` | panic |
| `u64` | unsigned int | inline primitive | (widest unsigned fixed) | panic |
| `f16` | float | inline primitive (binary16) | `float`, `double` (lossless) | IEEE Infinity |
| `float` | float | inline primitive (binary32) | `double` (lossless) | IEEE Infinity |
| `double` | float | inline primitive (binary64) | (widest float) | IEEE Infinity |
| `byte` | signed int | inline (`int` in `0..255`) | `int` | panic |
| `decimal` | arbitrary decimal | heap-backed built-in | (none; explicit entry) | grows, no overflow |
| `rational` | exact fraction | heap-backed built-in | (none; explicit entry) | grows / panic on /0 |
| `complex` | complex | heap-backed built-in | (none; explicit entry) | IEEE per component |

All are **built-in** (all have operators). Crossing any family boundary (a horizontal move between
family groups, or fixed to arbitrary-precision) is an **explicit conversion**. Widening within a
family group (vertical, to a wider width) is **implicit and lossless**. There is no arbitrary-precision
integer; a fixed `int128` is a possible future library type only (§1.4).

---

## 6. Status: committed, partly deferred past alpha

The current specced primitives are `int`, `double`, and the `byte` constraint. The rest of this tower,
`u64` and the signed and unsigned small-width constraints, `float` and `f16`, and the
arbitrary-precision and exact built-ins `decimal`/`rational`/`complex`, is **committed** but
**deferred past the alpha surface** (operators spec §4): int and double cover the majority of code, and
the remaining types slot into mechanisms fixed now (the built-in-only operator rule, the
constraint-subtype widening, explicit cross-family conversion), so adding them later is **new typeids
under existing rules**, not new machinery.

Adding a built-in numeric type is a **breaking change** (it expands the closed type universe and
claims operator behavior), so these all land **before the 1.0 stability commitment**, even though they
are not in alpha. The requirement this places on alpha is that `match` exhaustiveness and reflection
**tolerate the type universe growing**, so that adding numeric typeids later does not silently break
exhaustiveness or type-set reflection.

---

## 7. Open questions

- **`decimal` rounding and context.** How `decimal` handles operations that are not exact under a
  target scale (division producing a repeating decimal), whether it carries a precision/rounding
  context, pending the `decimal` spec.
- **`rational` normalization and overflow.** Whether `rational`'s numerator and denominator are
  fixed integers (so reduction can overflow and panic) or a wider representation (so a rational never
  overflows), pending the `rational` spec. Since there is no `bigint`, the wide option would need its
  own arbitrary-precision integer internal to `rational`, a decision for that spec.
- **Literals for the wider types.** Whether there are literal forms for `decimal` and the
  unsigned/float types (suffixes, or context-driven), with the literal grammar.
- **Bit operations.** Bitwise `and`/`or`/`xor`/`not` and shifts on the integer types (arithmetic vs
  logical shift, shift-amount edges), pending the bitwise spec (int spec §8).
- **`complex` over what component type.** Whether `complex` is always over `double`, or can be over
  `float` or exact types, pending the `complex` spec.
