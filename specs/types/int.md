# Int

`int` is Luna's integer type: a **64-bit signed** integer, stored **inline in the `lval`**
(value-representation), never boxed or heap-allocated. It is a primitive, foundational type.

```luna
let x: int = 0;
let big = 9223372036854775807;      // 2^63 - 1, the maximum int
```

- **64 bits, signed.** The range is -2^63 to 2^63 - 1 (-9223372036854775808 to
  9223372036854775807). There is one integer width; narrower or wider integers, and unsigned
  integers, are handled as constraints or (for full-width unsigned) a deferred separate
  representation (§6).
- **Inline in the `lval`.** An int lives in the value word itself, so it is copied by value and
  costs nothing to allocate.

---

## 1. References to ints

An int can be referenced with `&`, which is genuinely useful (out-parameters, in-place update
through a reference, swapping) and has no reason to be excluded:

```luna
var n = 5;
increment(&n);          // pass a reference; increment mutates n in place
```

As everywhere, `&` requires a **`var`** binding (variables spec): you may reference a mutable
int, not a `const` one. A reference to an int refers to the variable's storage; reading it
yields the current value, writing through it updates the variable.

---

## 2. Overflow panics by default

Arithmetic that exceeds the 64-bit range **panics**; it never silently wraps. Silent
wraparound is a severe and common bug class (a wrapped length becomes a tiny allocation, a
wrapped index goes out of bounds, wrapped money goes negative), and the failure is invisible.
So the default is safe: overflow raises a `panic` (an `overflowError`, errors §2), stopping the
program at the point the wrong value would have been produced.

Because overflow is a **`panic`** (ambient, undeclarable, errors §2), arithmetic does **not**
make a function `!`: `a + b` returns a plain `int`, and functions doing arithmetic keep clean
signatures. The panic is still real and catchable at a `try`/`catch` block (§3); it simply is not a declarable
declarable error, so it does not infect every arithmetic-using signature with `!`. This is the same
treatment as `typeError` and out-of-bounds access.

### 2.1 The performance cost is negligible for Luna

Detecting overflow is cheap: modern CPUs set an overflow flag as part of the arithmetic
instruction, so the check is a single, almost-always-not-taken conditional branch, which the
branch predictor renders nearly free in the common case. The only real cost is some inhibited
optimization (auto-vectorization) in tight numeric loops, which Luna, a garbage-collected,
green-threaded language, is not optimizing for. For ordinary application code the cost is
unmeasurable, and the safety is worth far more than the lost cycles. Where wrapping or
saturation is actually the intended arithmetic, it is available explicitly (§4).

---

## 3. Detecting overflow: the `try`/`catch` block, not a checked-add function

Because overflow is a `panic`, the `try` **expression** does **not** catch it: `try` catches
declarable errors only (everything outside the `panic` subtree), and a panic unwinds
through it (errors §8.1). `let sum! = try
bigA + bigB` is therefore **not** an overflow check, the `try` there can only ever catch a
declarable error from a callee, never the `overflowError`, and code anticipating overflow that way
is wrong. Anticipated overflow is handled where every panic is handled, at a **`try`/`catch`
block** (errors §8.2), which catches everything:

```luna
try {
  let sum = bigA + bigB;         // may panic with overflowError
  process(sum);                   // success path stays inside the block
} catch (e: overflowError) {
  // handle the anticipated overflow: fall back, clamp, or fail the unit of work
};
```

This is the correct shape for the two-category design: overflow is an invariant violation, so
absorbing it must be a deliberate, block-sized act at a real boundary, not something a
one-token `try` (whose job is *expected* errors) can do by accident (errors §8.1). The block
provides checked semantics for **every** operation uniformly, addition, subtraction,
multiplication, and the rest, which is why there is still no dedicated `checkedAdd`: where
overflow is *anticipated as a possibility to detect*, write the block; where an alternative
result is *intended arithmetic*, use the explicit wrapping/saturating operations (§4).

---

## 4. Alternative arithmetic: wrapping and saturating

`try` **detects** overflow but cannot **produce** a different result, it catches the panic, it
does not compute an alternative value. When wrapping or saturation is the *intended* arithmetic
(not an error to detect), explicit operations produce those values:

```luna
const wrappingAdd = fn (a: int, b: int): int => {};         // two's-complement wraparound (no panic)
const wrappingSub = fn (a: int, b: int): int => {};
const wrappingMul = fn (a: int, b: int): int => {};
const saturatingAdd = fn (a: int, b: int): int => {};       // clamp to int min/max on overflow (no panic)
const saturatingSub = fn (a: int, b: int): int => {};
const saturatingMul = fn (a: int, b: int): int => {};
```

- **Wrapping** gives two's-complement wraparound, the intended math for hashing, checksums, and
  ring-buffer arithmetic, where overflow is expected and defined. It never panics.
- **Saturating** clamps to the representable range (min on underflow, max on overflow), the
  intended behavior for signal, audio, and graphics-style arithmetic. It never panics.

These are needed precisely because they yield **values** `try` cannot: `try` can tell you an
add overflowed, but only `wrappingAdd` gives you the wrapped result and only `saturatingAdd`
gives you the clamped one. The default operators (`+`, `-`, `*`) panic; the alternative
semantics are always opt-in and named, so wrapping and saturation are never silent.

---

## 5. Division, remainder, and edge cases

```luna
_ = 7 / 2;  // 3   (integer division truncates toward zero)
_ = -7 / 2; // -3  (truncation, not floor)
_ = 7 % 2;  // 1   (remainder; sign follows the dividend)
_ = -7 % 2; // -1
```

- **Division truncates toward zero.** `7 / 2` is `3`, `-7 / 2` is `-3` (not `-4`). This is the
  hardware-native, staple convention. A floor-dividing pair (`floorDiv`, `floorMod`) is
  available for the alternative (floor semantics, remainder following the divisor).
- **Remainder sign follows the dividend**, pairing with truncated division: `-7 % 2` is `-1`.
- **Division by zero panics** (**`divisionByZero`**, the tower-wide panic — the same name
  `rational`'s `r / 0` and `decimal`'s `div(a, 0, …)` raise; errors §2): `5 / 0` and `5 % 0`
  stop the program; ints have no nan or infinity to yield.
- **`INT_MIN / -1` panics**, and so does unary negation of `int.min` (`-x` where `x == int.min`), the same missing +2^63: the mathematical result does not fit in a signed 64-bit int,
  so this edge case is an **overflow** and panics like any other (§2).

The through-line matches overflow: arithmetic never silently produces a wrong or undefined
value; the undefined and out-of-range cases panic.

---

## 6. The integer family: three mechanisms

Luna has one integer *primitive* (`int`, signed 64-bit); every other integer type is built by
one of three mechanisms, chosen by whether the type fits in 64 bits and whether it needs new
arithmetic.

### 6.1 Small signed widths are constraints on `int`

The narrower **signed** widths fit within signed 64-bit, so they are **constraints**
(constraints spec), shipped as stdlib declarations, not new primitives:

```luna
const i8  = constraint i: int where i >= -128 && i <= 127;
const i16 = constraint i: int where i >= -32768 && i <= 32767;
const i32 = constraint i: int where i >= -2147483648 && i <= 2147483647;
```

(The unsigned smalls `u8`/`u16`/`u32` are the same mechanism on the **other** base:
constraints on `uint`, the unsigned primitive — §6.2, §6.4, numeric-tower §1.2. An earlier
version of this block showed `u8`/`u32` as constraints on `int` with a `u8 == byte` note,
contradicting §6.4's `u8`-is-not-`byte` row; stale since the family split, fixed R226.)

The family also has one **range** (not width) refinement: **`nat`**,
`constraint i: int where i >= 0` (constraints §10, R226) — non-negative `int`,
`nat <: int`, the type for indexes, counts, and sizes.

These are **range-checked ints** (Model A): arithmetic happens at `int` width, and the
constraint is a **bound enforced on entry** (assignment, `as`, store-back), not a change to how
`+` computes. So `someI16 + someI16` computes in `int` (no wrap at the int level), and storing the
result back into an `i16` re-checks the range, a value that left `-32768..32767` **panics** (the ordinary
constraint-on-entry check, constraints §7), it does not silently wrap.

This is deliberate and consistent with `int`'s overflow rule (§2): a `u8` leaving its range
panics, just as an `int` leaving 64 bits panics. It is **not** C's wrapping `uint8_t`, which is
a bug factory; wrapping is available explicitly (`wrappingAdd`, §4) when it is the intended
math. So the small widths cost no new machinery (they are constraint declarations) and inherit
the safe, panic-on-out-of-range behavior.

### 6.2 `uint` is a separate primitive

`uint` (the full 0 to 2^64 - 1 range; named `u64` until R226) **cannot** be a constraint on
`int`, because a constraint
filters values but cannot add representable ones: the values 2^63 to 2^64 - 1 do not fit in
signed 64-bit storage at all (the top bit is the sign). So `uint` is a **separate primitive**
with its own representation (64 bits read as unsigned) and its own arithmetic (unsigned add,
subtract, multiply, divide, and comparison differ from signed at the instruction level). It is
inline in the `lval` like `int`, so it is representationally cheap; the cost is a parallel
arithmetic surface. `uint` is provided as a primitive because there is genuinely no other way to
reach its range — it is `int`'s full-width unsigned twin, and the name says so (R226). (Its
own spec is deferred.)

### 6.3 Integers wider than 64 bits

Integers **wider than 64 bits** need **multi-word arithmetic**, which neither constraints nor `bytes`
provide: a constraint refines which values are valid but does not implement arithmetic; `bytes` can
*store* a 128-bit value but has no meaningful `+`. Wide arithmetic is *code* (a 128-bit add is two
64-bit adds with carry propagation).

Luna does **not** provide an arbitrary-precision integer (`bigint`). The concrete need for exactness
beyond 64-bit integers, currency and exact decimals, is met by the built-in **`decimal`** type
(numeric-tower spec §1.4), which has a sharp motivation; a general arbitrary-precision integer does
not, and 64-bit `int` and `uint` cover ordinary integer work. A **fixed** wide integer (`int128`) is a
*possible future library type* (method syntax, since operators are built-in only, operators spec §1),
not committed and not built-in. The language provides `int` and `uint` as the native words such a
library type would be built from.

### 6.4 Summary

| Type | Mechanism | Why |
|-|-|-|
| `int` (i64) | primitive | the native word; inline; panic on overflow |
| `i8`, `i16`, `i32` | constraint on `int` | fit in signed 64-bit; range-checked, panic out of range |
| `nat` | constraint on `int` | non-negative `int` (`i >= 0`), the range refinement for indexes, counts, sizes (constraints §10, R226) |
| `u8`, `u16`, `u32` | constraint on `uint` | the unsigned tower's smalls, `u8 <: u16 <: u32 <: uint` (numeric-tower §2); range-checked, panic out of range. **`u8` is not `byte`**: same value range, different bases (`byte` is `int`-based, the `bytes` element and IO workhorse; `u8` is `uint`-based, the unsigned tower's smallest), so crossing them is a family crossing and needs an explicit conversion, like every signed/unsigned crossing |
| `uint` | separate primitive | needs the full 0..2^64 - 1 range signed-64 cannot hold (`u64` until R226) |
| `int128` | library (if ever) | fixed 128-bit; rarely needed; `decimal` covers exactness, `int`/`uint` cover ordinary work |

There is no arbitrary-precision integer (`bigint`); the built-in **`decimal`** covers exact,
unbounded numeric needs like currency (numeric-tower spec §1.4).

---

## 7. Literals

Decimal literals are written directly (`0`, `42`, `-7`, `9223372036854775807`). Alternative
integer notations — hexadecimal `0x…`, binary `0b…`, and octal `0o…` (R238), with digit
separators like `0b0100_0001` — are lexer features, **now fully specified in lexer §4**; they
are integer literals, not a bytes-specific or int-specific runtime feature. A single byte value
is just an int literal used in a `byte` context (bytes spec).

The rules that bite in practice: prefixes are **lowercase only** (`0X` is a lex error), a
**leading zero is a lex error** (`0755` is neither decimal 755 nor octal — write `0o755`), and
`_` must sit strictly **between digits**. **Every integer literal is an `int`** — there are no
wider-type literal forms (numeric-tower §9, R216/R238) — so a literal too large for i64 is
diagnosed in **parsing**, needing no type information to do it. Assigning an in-range literal
to a narrower target (`let b: byte = 300;`) is a separate check, and belongs to analysis.

---

## 8. Open questions

- **`int` and `float`:** the overall numeric tower, the type set, families, and widening and
  conversion rules, is specified in the numeric-tower spec (`float <: double` lossless implicit
  widening; no implicit int-to-float crossing, explicit conversion). The `float` type's own IEEE
  detail is pending a `float` spec.
- **Bit operations:** bitwise and, or, xor, not, and shifts, their behavior on the signed
  representation (arithmetic vs logical shift), and shift-amount edge cases.
- **`uint` primitive surface:** the concrete operations and conversions for the `uint` primitive
  (§6.2), and how it interconverts with `int` (a narrowing both ways, since neither range
  contains the other), pending its own spec.
- **Wide integers:** whether a fixed `int128` library type is provided (§6.3); there is no
  arbitrary-precision integer (`bigint`), with `decimal` covering exact unbounded needs
  (numeric-tower spec). Pending the wide-integer decision if one is ever wanted.
- *(**Digit separators and literal grammar: resolved by R238.** The exact literal syntax is
  ruled in lexer §4 and summarized in §7 — `0x`/`0b`/`0o` lowercase-only, leading zeros a lex
  error with an explicit error production, `_` strictly between digits, no leading or trailing
  point, plain digits in exponents. Octal was **added**; the leading-zero ban is what makes
  adding it safe, since `0755` can no longer mean two different things to two readers.)*
