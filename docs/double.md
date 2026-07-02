# Double

`double` is Luna's floating-point type: a **64-bit IEEE 754** double-precision float, stored
**inline in the `lval`** (value-representation), like `int`. It follows IEEE semantics
faithfully, including infinities and NaN, and it is deliberately **not** interchangeable with
`int` (no implicit conversion, §7).

```
let x: double = 3.14;
let y = 1.0 / 3.0;         // 0.3333333333333333
```

- **64-bit IEEE 754**, roughly 15-17 significant decimal digits, with the usual special values:
  positive and negative infinity, negative zero, and NaN.
- **Inline in the `lval`**, copied by value, no allocation.

The name `double` is the 64-bit type; the 32-bit float is a separate primitive named `float`
(§6), so there is no ambiguity about which is the default width.

---

## 1. IEEE arithmetic: defined values, never a throw

Floating-point overflow, underflow, and invalid operations follow **IEEE**, producing defined
special values, and **never throw**:

- **Overflow** produces `+Infinity` or `-Infinity` (a value too large to represent becomes
  infinity, which is itself a usable value: `1.0 / Infinity` is `0.0`, `Infinity > x` for any
  finite `x`).
- **Underflow** produces subnormals and then `0.0` (a graceful loss of precision toward zero,
  not a cliff).
- **Invalid operations** produce `NaN`: `0.0 / 0.0`, the square root of a negative, `Infinity -
  Infinity`, and similar. NaN propagates through further arithmetic (any operation with a NaN
  operand yields NaN).

This is the **opposite** of `int`, which panics on overflow and division by zero (int spec §2,
§5), and the difference is principled, not inconsistent. Int overflow is always a bug: there is
no meaningful "bigger than 2^63" result, so panicking catches an error. Float overflow has a
**meaningful** result (Infinity), and invalid operations have a meaningful sentinel (NaN);
IEEE was designed precisely so numeric code can run through these cases and inspect results at
the end rather than trapping mid-computation. So `double` yields Inf/NaN where `int` panics,
because floats have defined out-of-range values and ints do not.

### 1.1 Division by zero produces Inf or NaN, not a panic

Following from §1, floating-point division by zero does **not** panic (unlike int division,
which does):

```
1.0 / 0.0        // +Infinity
-1.0 / 0.0       // -Infinity
0.0 / 0.0        // NaN
```

To treat a non-finite result as an error, check it (§2.1) or narrow to `finiteDouble` (§5),
which converts the IEEE sentinel into a panic at a boundary you choose.

---

## 2. Equality and ordering are IEEE, and are not well-behaved

`double` comparison follows IEEE, which means it does **not** provide the clean equivalence and
ordering the rest of the language assumes for well-behaved values:

- **`NaN != NaN`.** NaN is equal to nothing, including itself, so `==` on doubles is **not
  reflexive**. A NaN never equals a NaN.
- **`-0.0 == +0.0`** is true, though they are distinct bit patterns.
- **NaN is unordered.** `NaN < x`, `NaN > x`, and `NaN == x` are all false for every `x`. So
  `<` / `>` do not form a total order when NaN is present.

These are inherent to IEEE and are the reason `double` cannot be a table key (§3) and must be
used carefully in `match` and sorting.

### 2.1 Checking for special values

```
fn isNan(x: double): bool         // true iff x is NaN (the reliable NaN test, since x == x fails for NaN)
fn isInf(x: double): bool          // true iff x is +Infinity or -Infinity
fn isFinite(x: double): bool       // true iff x is neither NaN nor Infinity
```

`isNan` is the correct way to test for NaN, because `x == NaN` and even `x == x` do not work
(the latter is a NaN detector only by accident and reads as a bug). Sorting a collection of
doubles that may contain NaN needs a total-order comparison (a library function), since IEEE
ordering is partial.

---

## 3. `double` is never a table key

A `double` **cannot** be a table key. This follows from the key-type rule (keys are `string`
or `int`, tables spec) and is reinforced by §2: a key needs a well-behaved equivalence relation,
and IEEE equality is not one. A NaN key could never be looked up (`NaN != NaN`), distinct
doubles that print alike could collide, and `-0.0`/`+0.0` would key inconsistently with `==`.
Rather than paper over these, `double` is simply excluded as a key.

If you genuinely need to key by a floating-point value, convert it to a `string` first with a
**library round-trip stringification** (the shortest string that parses back to the exact same
double), and use that string as the key. That moves keying onto string identity (well-behaved)
and makes the lossiness explicit and opt-in, rather than hiding it in the table machinery. This
is a library operation, not a language feature, keeping the core key rule simple.

---

## 4. Smaller floats are a different representation, not a constraint

Unlike `int`, whose narrower widths are constraints (int spec §6.1), `double` has no
constraint-expressible smaller float. A 32-bit float is not a sub-range of `double`; it has
**fewer mantissa bits** (less precision), a genuinely different representation with different
rounding. A constraint filters which doubles are valid; it cannot reduce precision. So the
32-bit float is a **separate primitive** (`float`, §6), not a constraint.

Constraints on `double` instead restrict the **value set**, which is useful:

```
const probability   = constraint { double as x where x >= 0.0 && x <= 1.0 };
const finiteDouble   = constraint { double as x where isFinite(x) };       // excludes NaN and Infinity
```

So constraints give ranges and finiteness, not reduced precision.

---

## 5. `finiteDouble`: opt-in strictness

`finiteDouble` (§4) is the antidote to IEEE's silent NaN/Infinity: a `double` guaranteed to be
a real, finite number. Because it is an ordinary constraint (constraints spec), narrowing to it
runs the check and **panics** (a `TypeError`) on a NaN or Infinity:

```
let r = someComputation();          // r : double, possibly NaN or Infinity
let f = r as finiteDouble;          // panics here if r is NaN or Infinity
// f is known finite from here
```

So `as finiteDouble` is the float analogue of catching int overflow: IEEE's default is to run
through NaN/Infinity silently, and `as finiteDouble` is where you assert "this must be a real
number," turning the silent sentinel into a loud, checked failure **at a boundary you choose**.
Between the IEEE default (never throws) and `finiteDouble` (throws on demand), you pick per use
whether non-finite values are tolerated or rejected, with no special float-exception machinery.

---

## 6. `float` is a separate primitive

The 32-bit float is a distinct primitive named **`float`**, not a narrowing of `double`,
because its semantics differ: fewer mantissa bits, different rounding, a different
representation. It is deferred to its own spec. Conversion between `double` and `float` is
explicit and lossy (double to float loses precision), a function, not an implicit widening or an
`as` narrowing (§7). `double` is the default floating-point type; `float` is used where its
compactness or its match to an external format (graphics, some binary protocols) is needed.

---

## 7. Conversions to and from `int` are explicit functions

There is **no implicit conversion** between `int` and `double`, and neither `as`, because the
two are disjoint representations and converting **transforms** the value (`as` never transforms,
`as` spec §3). Conversion is by function:

```
fn toDouble(n: int): double        // int to double; exact for |n| <= 2^53, lossy above (mantissa is 52 bits)
fn toInt(d: double): int!          // double to int; truncates toward zero; fallible
```

- **`int` to `double`** is total but **lossy for large magnitudes**: a `double`'s 52-bit
  mantissa cannot exactly represent every 64-bit int, so ints beyond 2^53 lose low bits. The
  function succeeds but the result may be rounded.
- **`double` to `int`** truncates toward zero and is **fallible** (`int!`): NaN, the infinities,
  and values outside the int range have no int result, so the conversion throws (a `UserError`
  to handle, or a panic, matching how out-of-range narrowing behaves). Truncation of an
  in-range finite double is exact.

Keeping these as named functions (not `as`, not implicit) makes the lossy and fallible nature
visible at every crossing, consistent with the language's conversion-is-a-function rule.

---

## 8. Literals

Decimal floating-point literals are written with a point or exponent (`3.14`, `1.0`, `6.022e23`,
`1e-9`). A literal with neither a point nor an exponent is an `int` (§7 governs crossing between
them). The exact literal grammar (exponent form, digit separators, whether a trailing or
leading point is allowed) is specified with the literal grammar.

---

## 9. Open questions

- **Total-order comparison:** the library function for a total order over doubles (placing NaN
  definitely, distinguishing `-0.0` from `+0.0`) for sorting, since IEEE ordering is partial.
- **`==` and `match` on doubles:** whether `==` is raw IEEE equality (with the NaN and signed-
  zero surprises) or whether a stricter total equality is offered alongside, and how `match`
  handles a NaN scrutinee.
- **`float` semantics:** the full 32-bit `float` spec (§6), and the exact double/float
  conversion functions.
- **Fused and rounding operations:** whether fused multiply-add, explicit rounding modes, and
  rounding functions (floor, ceil, round, trunc) belong in the core or a math library.
- **Decimal type:** whether a base-10 decimal type (for money and exact decimal fractions, where
  binary floating point is inappropriate) is ever provided, likely a library type.
