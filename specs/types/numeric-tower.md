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

The family carries one **range** refinement beside the width rungs: **`nat`** —
`constraint i: int where i >= 0` (constraints §10, R226) — non-negative `int`, range
`0..2^63 - 1`, `nat <: int`. It is deliberately **not** the unsigned family below: a `nat`
stays inside `int`'s arithmetic, literals, and every `int`-taking surface (the §3 crossing
rules never engage), and intermediates compute at `int` width — a delta may go negative;
only a store-back into a `nat` slot checks. `nat` is what a signature says for indexes,
counts, and sizes; `uint` (§1.2) is the boundary type for values that genuinely need the
top half.

### 1.2 Unsigned integers

```
u8  <:  u16  <:  u32  <:  uint
```

`uint` is the 64-bit unsigned primitive (int spec §6.2; named `u64` until R226), a **separate
primitive** from `int` because
its top range (2^63 to 2^64 - 1) does not fit in signed 64-bit storage. The narrower widths `u8`,
`u16`, `u32` are **constraints** on `uint`, exactly as the signed smalls are constraints on `int`, so
`u8 <: u16 <: u32 <: uint` — the exact mirror of the signed chain, each family topped by its
width-unsuffixed 64-bit primitive (R226). Same discipline: compute wide, check on entry, panic out of range, implicit
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
including subnormals, the zeros, the infinities, and nan, is an exact value of the wider type. No
information is lost widening up the chain, so **widening is implicit** (`f16` to `float`, `float` to
`double`), on the same lossless-therefore-safe justification as integer widening (the mechanism is a
lossless representation change rather than a subset relation). Narrowing down the chain (`double` to
`float`, `float` to `f16`) is **lossy** (it rounds and can overflow to inf), so it is not `as` at
all but an explicit **conversion function**: `toFloat(d: double): float`, total — IEEE
round-to-nearest-even, overflow to `float` inf, `nan` to `nan` — and `toF16` likewise when `f16`
lands. `as` re-types a value losslessly and never computes a new one (as spec §3, R124); a rounding
step is a computation, so it gets a name. Integer narrowing, by contrast, stays `as` (§4): where it
accepts, the value is exact.

So the float family is a widening chain like the integer families, and the widen-to-widest rule (§2)
applies: `float + double` is `double`, `f16 + float` is `float`.

`f16` is **committed but deferred** (§6): it is used chiefly in machine-learning and GPU work
(compact storage of many low-precision values), and it slots into the family with no special
mechanism, so it is added when those domains are targeted. `float` and `double` are the current core
floats.

### 1.4 Arbitrary-precision and exact (built-in, heap-backed)

```text
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
- **`complex`**, a real/imaginary pair of `double`s, with the standard complex arithmetic —
  the tower's one *inexact* heap-backed member (IEEE per component) and its one **unordered**
  member: `<`/`<=`/`>`/`>=` are compile errors, since no total order can coexist with the
  arithmetic (complex spec §2, R164).

There is **no arbitrary-precision integer** (`bigint`): Luna does not provide one, because its
motivation is diffuse, `decimal` covers the concrete need for exactness (currency), and 64-bit `int`
and `uint` cover ordinary integers. A **fixed** wide integer (`int128`) is a *possible future library
type* (method syntax), not committed and not built-in, since it is rarely needed and `decimal` and the
64-bit integers cover the common cases.

The backend mapping is recorded (compiler §7.5, R163/R164): the shared internal bignum is
`big.Int`, `rational` is backed by `big.Rat` nearly wholesale, `decimal` is a thin
`{unscaled, scale}` struct over `big.Int` (`big.Float` is arbitrary-precision *binary* — a
semantic mismatch, never used), and `complex` is Go's native `complex128`, boxed. None of the
machinery is written from scratch.

---

## 2. Widening within a family

In an expression combining two values of the **same family** but different widths, the result takes
the **widest** operand's type, and the narrower operand widens implicitly (losslessly):

```luna
_ = someI8 + someI32;       // i32   (the i8 widens to i32)
_ = someU16 + someUint;     // uint  (the u16 widens to uint)
_ = someFloat + someDouble; // double (the float widens to double, losslessly)
```

This is safe because within-family widening loses no information (a subset relation for integers, a
lossless representation change for floats). It is the constraint-subtype relation (constraints spec)
doing the work for integers, and the lossless float embedding (§1.3) for floats. Narrowing (the
reverse) is never implicit: integers narrow with an explicit checked `as` that panics out of range
(lossless where accepted), and floats narrow with an explicit conversion, `toFloat` (§1.3) — a
rounded result is a computed new value, which is never `as` (as spec §3, R124).

---

## 3. Crossing families is explicit

Between families, there is **no implicit conversion**; you convert with an explicit function
(conversion spec):

```luna
_ = someInt + someDouble;            // ERROR: different families, no implicit crossing
_ = someInt.toDouble() + someDouble; // OK: explicit widening to double, then double arithmetic

_ = someInt + someUint;              // ERROR: signed and unsigned are different families
_ = someInt.toUint() + someUint;     // OK: explicit (and checked: a negative int cannot become uint)
```

The families that require explicit crossing are: **signed integers, unsigned integers, floats, and
each arbitrary-precision type**. Crossing any two of these is a conversion function, never an operator
widening. This is the direct consequence of no-implicit-coercion (the language never silently changes
a value's numeric family), and it is what keeps `1 == 1.0` false (equality spec §1) and arithmetic
predictable: an operator never silently reinterprets one family as another.

### 3.1 Fixed to arbitrary-precision is explicit, though lossless

Widening a fixed type to an arbitrary-precision one (`int` to `decimal`, `int` to the
numerator of a `rational`) is **lossless** (every `int` is exactly a `decimal`), so by the
within-family logic it *could* be implicit. It is nonetheless **explicit**, because the conversion has
a **representation cost** (it allocates a heap value), and Luna makes cost visible: an implicit `int`
to `decimal` widening would hide an allocation inside an innocuous-looking mixed expression. So the
rule for implicit widening is not merely "lossless" but "**lossless and cheap**"; fixed-to-arbitrary
is lossless but not cheap (it allocates), so it is explicit: **`n as decimal`** — legal `as`
because the move is lossless (as spec §3, R124), explicit because the cost should be seen. This is a
deliberate, principled exception: losslessness alone does not earn *implicit* widening when the
representation cost should be seen.

**`double as decimal` is rejected** (R161, resolving R124's hedge): it would be
lossless in the worst way, faithfully embedding the double's true dyadic value
(`0.1` → `0.1000000000000000055511151231257827…` — Java's `new BigDecimal(0.1)`
trap). The deliberate crossing is the composition `parseDecimal(toString(d))`, which
yields the decimal a human reads off the double and makes the lossy moment visible
(decimal §5). **`double as rational` is rejected identically** (R162), and the trap
is sharper there — the embedding would be *mathematically exact* (every finite
double is a dyadic rational), which is precisely the problem; the crossing is
`parseRational(toString(d))` (rational §4). **`double as complex` is legal** (R164),
and the asymmetry is the point: the component is carried bit-for-bit, not
reinterpreted into another radix or reduced form — the same double, now on the
plane — so R124's lossless criterion is met with no trap to reject. Explicit
because it allocates, like every entry on this list (complex §4).

So the arbitrary-precision types are their **own** families for widening purposes: you enter them by
an explicit, lossless `as`, and once in them, their arithmetic is exact and their operators are the
built-in operators (operators spec).

---

## 4. Narrowing and violations

Narrowing, moving to a narrower or lossier type, is always **explicit**, and the spelling follows
one criterion (as spec §3, R124): **`as` where lossless, a function where not.** A move is `as`
exactly when the value, where accepted, is preserved exactly, so range is its only question; a move
that computes a new value gets a name.

- **Integer narrowing** (`int as i8`, `uint as u8`, and cross-family `int as uint`): an explicit `as`
  that **panics** if the value is out of the target range (the constraint-on-entry check, constraints
  spec; int spec §6). Lossless where accepted, so it is `as`.
- **Float narrowing** (`toFloat(d: double): float`): a **conversion function**, total — IEEE
  round-to-nearest-even, `float` inf if the magnitude overflows binary32, `nan` to `nan` (double
  spec). It rounds, so there is no `double as float` (§1.3).
- **Arbitrary to fixed**: the **policy verbs**. A fractional `decimal` has no lossless `int`
  reading, and choosing a nearby one is a rounding *decision* — exactly the double→int question —
  so when `decimal` lands, `trunc` / `round` / `floor` / `ceil` widen to `double | decimal`
  (conversion §2), and there is no `decimal as int`. (The reverse entry, `int as decimal`, is
  lossless and is `as`, §3.1.)

The through-line is the language's uniform stance: **no silent wrong value.** Within-family widening
is lossless and cheap (so implicit); every other move is explicit — `as` panicking where the value
does not fit, or a named function where a new value is computed (IEEE sentinels where the format
defines them). Overflow of the fixed integers panics (int spec §2); the
arbitrary-precision `decimal` does not overflow (it grows); the floats produce IEEE infinities and nan
rather than panicking (double spec).

---

## 5. The complete tower

| Type | Family | Representation | Widens implicitly to | Overflow |
|-|-|-|-|-|
| `i8`, `i16`, `i32` | signed int | inline (constraint on `int`) | wider signed, up to `int` | panic |
| `int` (i64) | signed int | inline primitive | (widest signed fixed) | panic |
| `u8`, `u16`, `u32` | unsigned int | inline (constraint on `uint`) | wider unsigned, up to `uint` | panic |
| `uint` (u64) | unsigned int | inline primitive | (widest unsigned fixed) | panic |
| `f16` | float | inline primitive (binary16) | `float`, `double` (lossless) | IEEE inf |
| `float` | float | inline primitive (binary32) | `double` (lossless) | IEEE inf |
| `double` | float | inline primitive (binary64) | (widest float) | IEEE inf |
| `byte` | signed int | inline (`int` in `0..255`) | `int` | panic |
| `nat` | signed int | inline (`int` in `0..2^63 - 1`) | `int` | panic |
| `decimal` | arbitrary decimal | heap-backed built-in | (none; explicit entry) | grows, no overflow |
| `rational` | exact fraction | heap-backed built-in | (none; explicit entry) | grows / panic on /0 |
| `complex` | complex | heap-backed built-in | (none; explicit entry) | IEEE per component |

All are **built-in** (all have operators). Crossing any family boundary (a horizontal move between
family groups, or fixed to arbitrary-precision) is **explicit** — `as` where the move is lossless
(`int as uint`, `int as decimal`), a conversion function where it is not (`toDouble`, `toFloat`, the
policy verbs; §4, as spec §3). Widening within a family group (vertical, to a wider width) is
**implicit and lossless**. There is no arbitrary-precision
integer; a fixed `int128` is a possible future library type only (§1.4).

---

## 6. Status: committed, partly deferred past alpha

The **alpha-delivered core** is `int`, `double`, and the `byte` constraint — plus, by the
R187 carve-out below, `uint` and the 16/32-bit constraints — and, by R226, the `nat`
constraint beside `byte` (a one-line constraint typeid; the string-API adoption that
motivated it is a deferred follow-up) (R216: an earlier sentence here
said "the current *specced* primitives," stale twice — the exact types are specced in
full, and specced ≠ delivered). The rest of this tower,
the remaining small widths, `float` and `f16`, and the
arbitrary-precision and exact built-ins `decimal`/`rational`/`complex` (all three
**specced** — decimal.md R161, rational.md R162, complex.md R164 — delivery
unchanged), is **committed** but
**deferred past the alpha surface**: int and double cover the majority of code, and
the remaining types slot into mechanisms fixed now (the built-in-only operator rule of operators §1,
the constraint-subtype widening, explicit cross-family conversion), so adding them later is **new
typeids under existing rules**, not new machinery.

One carve-out (R187): **`uint` (named `u64` at that ruling; renamed R226) and the
small-width constraints `i16`/`u16`/`i32`/`u32` are
pulled into the alpha surface**, because std.binary's read family returns them and shipping
widened-then-tightened signatures was rejected. This is scheduling, not design — the smalls
are exactly this section's "new typeids under existing rules," and `uint` is the one new
primitive (binary §2.1). `i8`/`u8`, `float`/`f16`, and the exact types keep the post-alpha
schedule.

Adding a built-in numeric type is a **breaking change** (it expands the closed type universe and
claims operator behavior), so these all land **before the 1.0 stability commitment**, even though they
are not in alpha. The requirement this places on alpha is that `match` exhaustiveness and reflection
**tolerate the type universe growing**, so that adding numeric typeids later does not silently break
exhaustiveness or type-set reflection.

---

## 7. Resolved and deferred (R216)

Nothing here is open: three questions are resolved, and the two remainders are
**deferred**, each waiting on a trigger, not a decision.

- *(**`decimal` rounding and context: resolved by R161** — the `decimal` spec exists
  (decimal.md). `+`/`-`/`*` are exact always; **`/` and `%` are omitted from the
  operator table** (compile error naming `div`), and division is the policy-carrying
  function `div(a, b, scale, rounding: roundingMode = {halfEven})` — the R106
  discipline applied to the one operation where exactness is impossible. **There is no
  context**, rejected permanently: ambient precision/rounding state is frame-dependent
  arithmetic, the R79-family violation, and a comptime poison.)*
- *(**`rational` normalization and overflow: resolved by R162** — the `rational` spec
  exists (rational.md). The wide option won, and cheaply: the pair is two
  **arbitrary-precision integers** on the same internal bignum `decimal`'s unscaled
  value already needs (R161) — this table's own committed row said "grows" all along —
  held in **canonical form as an invariant** (reduced, denominator positive), which
  makes equality structural and `1/2`-vs-`2/4` unrepresentable. There is still no
  user-facing `bigint`; the bignum stays internal, with two public faces.)*
- **Literals for the wider types — deferral reaffirmed, not spent** (R216, reaffirmed R238).
  This rode "the literal grammar", and the literal grammar has now been ruled (lexer §4)
  **without** adding a suffix or a context-driven form, so the standing answer stands
  unchanged: **the comptime-folded constructor is the literal story** (R161's own ruling,
  applied three times since — `parseDecimal("19.99")`, `parseRational("1/3")`,
  `complex(3.0, -4.0)` all fold at build time). A dedicated suffix would buy spelling,
  not capability; it waits until it earns grammar, and may still be considered later.
  One consequence is now load-bearing elsewhere: because no wider-type literal exists,
  **every integer literal is an `int`**, which is exactly what lets a too-large literal be
  diagnosed in parsing with no type information (int §7, R238).
- **Bit operations — deferred** (R216). Bitwise `and`/`or`/`xor`/`not` and shifts on the
  integer types (arithmetic vs logical shift, shift-amount edges) wait for the bitwise
  spec as a whole (int §8, operators §0.4): the natural tokens `&` and `|` are spoken
  for, so the surface needs its own design pass, and nothing in alpha demands it.
- *(**`complex` over what component type: resolved by R164** — always `double`,
  permanently (the `complex` spec exists, complex.md). A `float`-component complex is an
  array-storage optimization for an audience Luna does not serve, exact-component complex
  (Gaussian rationals) has no practical audience, and there is no parameterization
  mechanism to express either. One component type also makes the backend Go's native
  `complex128`, boxed — the cheapest tower member to deliver.)*
