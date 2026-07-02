# Int

`int` is Luna's integer type: a **64-bit signed** integer, stored **inline in the `lval`**
(value-representation), never boxed or heap-allocated. It is a primitive, foundational type.

```
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

```
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
So the default is safe: overflow raises a `Panic` (an `OverflowError`, errors §2), stopping the
program at the point the wrong value would have been produced.

Because overflow is a **`Panic`** (ambient, undeclarable, errors §2), arithmetic does **not**
make a function `!`: `a + b` returns a plain `int`, and functions doing arithmetic keep clean
signatures. The panic is still real and catchable (§3); it simply is not a declarable
`UserError`, so it does not infect every arithmetic-using signature with `!`. This is the same
treatment as `TypeError` and out-of-bounds access.

### 2.1 The performance cost is negligible for Luna

Detecting overflow is cheap: modern CPUs set an overflow flag as part of the arithmetic
instruction, so the check is a single, almost-always-not-taken conditional branch, which the
branch predictor renders nearly free in the common case. The only real cost is some inhibited
optimization (auto-vectorization) in tight numeric loops, which Luna, a garbage-collected,
green-threaded language, is not optimizing for. For ordinary application code the cost is
unmeasurable, and the safety is worth far more than the lost cycles. Where wrapping or
saturation is actually the intended arithmetic, it is available explicitly (§4).

---

## 3. Detecting overflow: `try`, not a checked-add function

Because overflow is a `Panic` and `try` is **total** (it converts every throw, from either
subtree, into a value, errors §8.1), detecting overflow needs no dedicated `checkedAdd`
function. `try` on the arithmetic expression catches the overflow:

```
let sum! = try bigA + bigB;      // sum : int | error; the error arm is the OverflowError if it overflowed
if (sum is error) {
  // handle the overflow
} else {
  // sum is a valid int here
}
```

`try bigA + bigB` yields `int | error` (`int!`): the sum on success, or the caught
`OverflowError` on overflow. So "add these and tell me if it overflowed" is the ordinary
`try`-expression pattern, not a special arithmetic API. This is why there is no `checkedAdd`:
`try` already provides checked semantics for **every** operation uniformly, addition,
subtraction, multiplication, and the rest.

---

## 4. Alternative arithmetic: wrapping and saturating

`try` **detects** overflow but cannot **produce** a different result, it catches the panic, it
does not compute an alternative value. When wrapping or saturation is the *intended* arithmetic
(not an error to detect), explicit operations produce those values:

```
fn wrappingAdd(a: int, b: int): int         // two's-complement wraparound (no panic)
fn wrappingSub(a: int, b: int): int
fn wrappingMul(a: int, b: int): int
fn saturatingAdd(a: int, b: int): int       // clamp to int min/max on overflow (no panic)
fn saturatingSub(a: int, b: int): int
fn saturatingMul(a: int, b: int): int
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

```
7 / 2       // 3   (integer division truncates toward zero)
-7 / 2      // -3  (truncation, not floor)
7 % 2       // 1   (remainder; sign follows the dividend)
-7 % 2      // -1
```

- **Division truncates toward zero.** `7 / 2` is `3`, `-7 / 2` is `-3` (not `-4`). This is the
  hardware-native, staple convention. A floor-dividing pair (`floorDiv`, `floorMod`) is
  available for the alternative (floor semantics, remainder following the divisor).
- **Remainder sign follows the dividend**, pairing with truncated division: `-7 % 2` is `-1`.
- **Division by zero panics** (a `Panic`, errors §2): `5 / 0` and `5 % 0` stop the program;
  ints have no NaN or infinity to yield.
- **`INT_MIN / -1` panics**: the mathematical result 2^63 does not fit in a signed 64-bit int,
  so this edge case is an **overflow** and panics like any other (§2).

The through-line matches overflow: arithmetic never silently produces a wrong or undefined
value; the undefined and out-of-range cases panic.

---

## 6. The integer family: three mechanisms

Luna has one integer *primitive* (`int`, signed 64-bit); every other integer type is built by
one of three mechanisms, chosen by whether the type fits in 64 bits and whether it needs new
arithmetic.

### 6.1 Small widths are constraints on `int`

The narrower signed and unsigned widths all **fit within signed 64-bit**, so they are
**constraints** (constraints spec), shipped as stdlib declarations, not new primitives:

```
const u8  = constraint { int as i where i >= 0 && i <= 255 };            // == byte
const i16 = constraint { int as i where i >= -32768 && i <= 32767 };
const u32 = constraint { int as i where i >= 0 && i <= 4294967295 };
// i8, u16, i32, ... likewise
```

These are **range-checked ints** (Model A): arithmetic happens at `int` width, and the width
constraint is a **bound enforced on entry** (assignment, `as`, store-back), not a change to how
`+` computes. So `someU8 + someU8` computes in `int` (no wrap at the int level), and storing the
result back into a `u8` re-checks the range, a value that left `0..255` **panics** (the ordinary
constraint-on-entry check, constraints §7), it does not silently wrap.

This is deliberate and consistent with `int`'s overflow rule (§2): a `u8` leaving its range
panics, just as an `int` leaving 64 bits panics. It is **not** C's wrapping `uint8_t`, which is
a bug factory; wrapping is available explicitly (`wrappingAdd`, §4) when it is the intended
math. So the small widths cost no new machinery (they are constraint declarations) and inherit
the safe, panic-on-out-of-range behavior.

### 6.2 `u64` is a separate primitive

`u64` (the full 0 to 2^64 - 1 range) **cannot** be a constraint on `int`, because a constraint
filters values but cannot add representable ones: the values 2^63 to 2^64 - 1 do not fit in
signed 64-bit storage at all (the top bit is the sign). So `u64` is a **separate primitive**
with its own representation (64 bits read as unsigned) and its own arithmetic (unsigned add,
subtract, multiply, divide, and comparison differ from signed at the instruction level). It is
inline in the `lval` like `int`, so it is representationally cheap; the cost is a parallel
arithmetic surface. `u64` is provided as a primitive because there is genuinely no other way to
reach its range. (Its own spec is deferred.)

### 6.3 `int128` and larger are library types

Integers **wider than 64 bits** (`int128`, and arbitrary-precision) need something neither
constraints nor `bytes` can provide: **multi-word arithmetic**. A constraint refines which
values are valid but does not implement arithmetic; `bytes` can *store* a 128-bit value (16
octets) but has no meaningful `+`. Wide arithmetic is *code*, a 128-bit add is two 64-bit adds
with carry propagation, a multiply is several 64-bit multiplies combined, so a wide integer is
a **library type** carrying a hand-written multi-word arithmetic implementation, not a primitive
and not a constraint.

So `int128` (and a general arbitrary-precision `bigint`) are **library types**. Their
operations are **functions** (or operators over a library type), not the primitive `+`, and
that friction is acceptable and even appropriate: a user reaching for wide integers should know
they are paying for multi-word arithmetic and should be explicit about it. The library layer is
where the carry-propagation lives; the language provides `int` and `u64` as the native words
those libraries are built from. (These library types are specified separately, not here.)

### 6.4 Summary

| Type | Mechanism | Why |
|-|-|-|
| `int` (i64) | primitive | the native word; inline; panic on overflow |
| `u8`, `i8`, `u16`, `i16`, `u32`, `i32` | constraint on `int` | fit in signed 64-bit; range-checked, panic out of range |
| `u64` | separate primitive | needs the full 0..2^64 - 1 range signed-64 cannot hold |
| `int128`, `bigint` | library type | need multi-word arithmetic, which is code, not a predicate or a byte layout |

---

## 7. Literals

Decimal literals are written directly (`0`, `42`, `-7`, `9223372036854775807`). Alternative
integer notations, hexadecimal `0x...` and binary `0b...` (with digit separators like
`0b0100_0001`), are lexer features covered when the literal grammar is specified; they are
integer literals, not a bytes-specific or int-specific runtime feature. A single byte value is
just an int literal used in a `byte` context (bytes spec).

---

## 8. Open questions

- **`int` and `float`:** the numeric tower, whether `float` exists as a sibling primitive, how
  mixed arithmetic and conversions work, and whether any implicit int-to-float widening occurs
  (leaning no implicit widening, consistent with explicit conversion elsewhere). Pending a
  `float` spec.
- **Bit operations:** bitwise and, or, xor, not, and shifts, their behavior on the signed
  representation (arithmetic vs logical shift), and shift-amount edge cases.
- **`u64` primitive surface:** the concrete operations and conversions for the `u64` primitive
  (§6.2), and how it interconverts with `int` (a narrowing both ways, since neither range
  contains the other), pending its own spec.
- **`bigint` / `int128` library:** the concrete library type(s) for wide integers (§6.3):
  fixed `int128` versus arbitrary-precision `bigint`, the operator surface, and literals large
  enough to need them.
- **Digit separators and literal grammar:** the exact literal syntax (`_` separators, `0x`,
  `0b`, leading-zero rules), with the literal grammar.