# Double

`double` is Luna's floating-point type: a **64-bit IEEE 754** double-precision float, stored
**inline in the `lval`** (value-representation), like `int`. It follows IEEE semantics
faithfully, including infinities and nan, and it is deliberately **not** interchangeable with
`int` (no implicit conversion, §7).

```
let x: double = 3.14;
let y = 1.0 / 3.0;         // 0.3333333333333333
```

- **64-bit IEEE 754**, roughly 15-17 significant decimal digits, with the usual special values:
  positive and negative infinity, negative zero, and nan.
- **Inline in the `lval`**, copied by value, no allocation.

The name `double` is the 64-bit type; the 32-bit float is a separate primitive named `float`
(§6), so there is no ambiguity about which is the default width.

---

## 1. IEEE arithmetic: defined values, never a throw

Floating-point overflow, underflow, and invalid operations follow **IEEE**, producing defined
special values, and **never throw**:

- **Overflow** produces `inf` or `-inf` (a value too large to represent becomes
  infinity, which is itself a usable value: `1.0 / inf` is `0.0`, `inf > x` for any
  finite `x`).
- **Underflow** produces subnormals and then `0.0` (a graceful loss of precision toward zero,
  not a cliff).
- **Invalid operations** produce `nan`: `0.0 / 0.0`, the square root of a negative, `inf -
  inf`, and similar. nan propagates through further arithmetic (any operation with a nan
  operand yields nan).

This is the **opposite** of `int`, which panics on overflow and division by zero (int spec §2,
§5), and the difference is principled, not inconsistent. Int overflow is always a bug: there is
no meaningful "bigger than 2^63" result, so panicking catches an error. Float overflow has a
**meaningful** result (inf), and invalid operations have a meaningful sentinel (nan);
IEEE was designed precisely so numeric code can run through these cases and inspect results at
the end rather than trapping mid-computation. So `double` yields inf/nan where `int` panics,
because floats have defined out-of-range values and ints do not.

### 1.1 Division by zero produces inf or nan, not a panic

Following from §1, floating-point division by zero does **not** panic (unlike int division,
which does):

```
1.0 / 0.0        // inf
-1.0 / 0.0       // -inf
0.0 / 0.0        // nan
```

To treat a non-finite result as an error, check it (§2.1) or narrow to `finiteDouble` (§5),
which converts the IEEE sentinel into a panic at a boundary you choose.

---

## 2. Equality and ordering are IEEE, and are not well-behaved

`double` comparison follows IEEE, which means it does **not** provide the clean equivalence and
ordering the rest of the language assumes for well-behaved values:

- **`nan != nan`.** nan is equal to nothing, including itself, so `==` on doubles is **not
  reflexive**. A nan never equals a nan.
- **`-0.0 == +0.0`** is true, though they are distinct bit patterns.
- **nan is unordered.** `nan < x`, `nan > x`, and `nan == x` are all false for every `x`. So
  `<` / `>` do not form a total order when nan is present.

These are inherent to IEEE and are the reason `double` cannot be a table key (§3) and must be
used carefully in `match` and sorting.

### 2.1 Checking for special values

```
fn isNan(x: double): bool         // true iff x is nan (the reliable nan test, since x == x fails for nan)
fn isInf(x: double): bool          // true iff x is inf or -inf
fn isFinite(x: double): bool       // true iff x is neither nan nor inf
```

`isNan` is the correct way to test for nan, because `x == nan` and even `x == x` do not work
(the latter is a nan detector only by accident and reads as a bug). Sorting and pattern matching
do not use IEEE `==` or IEEE ordering (both partial); they use the **total order** of §2.2.

### 2.2 The total order (for `sort` and `match`)

IEEE `==` and IEEE ordering are not well-behaved (§2), so anything that needs a well-behaved
relation over doubles, **sorting** and **`match` value-patterns** (match spec), uses a **total
order** instead, defined over every double including the infinities and nan:

```
-inf  <  negative finite  <  0.0  <  positive finite  <  inf  <  nan
```

with two deliberate rules:

- **`-0.0` and `+0.0` are equal** under the total order (merged into one value). They are
  semantically the same number to a human, so they sort together and match together: `match
  (-0.0) { 0.0 => ... }` matches. (This merges the two zeros, a humane deviation from strict
  IEEE `totalOrder`, which distinguishes them.)
- **nan is the single greatest value**, greater than `inf`. **All nans are equal** to each
  other (nan payloads are not ordered), forming one equivalence class at the top. So nan sorts
  to the end of an ascending sort, and `match (x) { nan => ... }` catches any nan.

The total order and `==` differ in **exactly one way**: under the total order nan is a normal,
greatest, self-equal value, while under `==` nan equals nothing (not even itself). They agree on
everything else (both treat the two zeros as equal). So the rule to remember is: **`==` is
IEEE semantic equality (nan unequal to all); the total order makes nan a well-behaved greatest
value.** `sort` and `match` use the total order (so nan is sortable and matchable); explicit
`==` keeps IEEE semantics (so arithmetic code sees the standard nan behavior).

A total-order comparison function is provided for explicit use (sorting with a custom
comparator, placing nan deliberately); `sort` uses it by default for doubles.

---

## 3. `double` is never a table key

A `double` **cannot** be a table key. This follows from the key-type rule (keys are `string`
or `int`, tables spec) and is reinforced by §2: a key needs a well-behaved equivalence relation,
and IEEE equality is not one. A nan key could never be looked up (`nan != nan`), distinct
doubles that print alike could collide, and `-0.0`/`+0.0` would key inconsistently with `==`.
Rather than paper over these, `double` is simply excluded as a key.

If you genuinely need to key by a floating-point value, convert it to a `string` first with
**`toString`** — ruled as the shortest round-trip rendering, the string that parses back to
the exact same double (the property the exact-type crossings lean on, decimal §5) — and use
that string as the key. That moves keying onto string identity (well-behaved)
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
const probability   = constraint x: double where x >= 0.0 && x <= 1.0;
const finiteDouble   = constraint x: double where isFinite(x);       // excludes nan and inf
```

So constraints give ranges and finiteness, not reduced precision.

---

## 5. `finiteDouble`: opt-in strictness

`finiteDouble` (§4) is the antidote to IEEE's silent nan/inf: a `double` guaranteed to be
a real, finite number. Because it is an ordinary constraint (constraints spec), narrowing to it
runs the check and **panics** (a `typeError`) on a nan or inf:

```
let r = someComputation();          // r : double, possibly nan or inf
let f = r as finiteDouble;          // panics here if r is nan or inf
// f is known finite from here
```

So `as finiteDouble` is the float analogue of catching int overflow: IEEE's default is to run
through nan/inf silently, and `as finiteDouble` is where you assert "this must be a real
number," turning the silent sentinel into a loud, checked failure **at a boundary you choose**.
Between the IEEE default (never throws) and `finiteDouble` (throws on demand), you pick per use
whether non-finite values are tolerated or rejected, with no special float-exception machinery.

---

## 6. `float` is a separate primitive

The 32-bit float is a distinct primitive named **`float`**, not a narrowing of `double`,
because its semantics differ: fewer mantissa bits, different rounding, a different
representation. It is deferred to its own spec; the **family rules are ruled at the tower**
(numeric-tower §1.3, R124) and the two directions split: **`float` → `double` widens
implicitly** — every binary32 value, subnormals, zeros, infinities, and nan included, embeds
losslessly in binary64 — while **`double` → `float` is the conversion function `toFloat`**,
total (IEEE round-to-nearest-even, overflow to `float` inf, nan to nan): a rounded result is
a computed new value, never `as` and never implicit. `double` is the default floating-point
type; `float` is used where its compactness or its match to an external format (graphics,
some binary protocols) is needed.

---

## 7. Conversions to and from `int` are explicit functions

There is **no implicit conversion** between `int` and `double`, and neither `as`, because the
two are disjoint representations and converting **transforms** the value (`as` never transforms,
`as` spec §3). Conversion is by function:

```
fn toDouble(n: int): double        // int to double; exact for |n| <= 2^53, lossy above (mantissa is 52 bits)
fn trunc(d: double): int!          // toward zero          — the policy verbs, each fallible
fn round(d: double): int!          // nearest; ties away from zero
fn floor(d: double): int!          // toward -inf
fn ceil(d: double): int!           // toward +inf
```

- **`int` to `double`** is total but **lossy for large magnitudes**: a `double`'s 52-bit
  mantissa cannot exactly represent every 64-bit int, so ints beyond 2^53 lose low bits. The
  function succeeds but the result may be rounded.
- **`double` to `int` is a policy, not a conversion** (conversion §2, R106): the narrowing
  hides a rounding decision, so it is spelled by **policy verbs** — `trunc` (toward zero),
  `round` (nearest, ties away from zero), `floor`, `ceil` — each **fallible** (`int!`):
  nan, the infinities, and values outside the int range have no int result, so the verb
  throws (a declarable error to handle, or a panic, matching how out-of-range narrowing
  behaves). Each verb is exact on an in-range finite double. (The retired generic
  `toInt(d)` silently truncated; the choice is now visible in the source, and `toInt`
  names only the total `bool → int` conversion, bool spec.)

Keeping these as named functions (not `as`, not implicit) makes the lossy and fallible nature
visible at every crossing, consistent with the language's conversion-is-a-function rule.

---

## 8. Literals

Decimal floating-point literals are written with a point or exponent (`3.14`, `1.0`, `6.022e23`,
`1e-9`). A literal with neither a point nor an exponent is an `int` (§7 governs crossing between
them). The exact grammar is **now ruled** in lexer §4 (R238), and three of its answers are
double-side facts worth stating here:

- **Neither a leading nor a trailing point.** `.5` is written `0.5`, `5.` is written `5.0`. The
  trailing ban is load-bearing rather than stylistic — requiring a digit on both sides is what
  lets `1..5` lex as `INT RANGE INT` and `1.toDouble()` as `INT DOT IDENT` with no lookahead.
- **Exponents take an optional sign and plain digits**, at least one; `_` separators are legal
  in the significand (`3.141_592`) but never inside an exponent, and there is no hex-float
  form (`0x1p3` is not Luna).
- **A literal that overflows to infinity is a compile error.** `1e400` does not silently become
  `inf`, because `inf` is a keyword and the explicit spelling exists (§2); a finite literal
  turning infinite is a wrong value, not a rounding. Ordinary rounding, underflow included, is
  normal IEEE behaviour and is not diagnosed — `1.1` is inexact too.

---

## 9. Resolved and deferred (R189)

- *(**A total-equality operator: resolved as no** — `===` / `!==` do not exist (R27's F11,
  associativity §4), and no second equality operator ever will: the total order's explicit
  form is the **comparator function** (§2.2), and `match`/`sort` use it implicitly. The
  `match` nan-scrutinee subtleties are settled — nan matches nan, the zeros merge (§2.2,
  match §7).)*
- **`float` semantics: the spec deferred, the rules ruled.** The family structure and both
  conversion directions are the tower's (numeric-tower §1.3, R124: implicit lossless
  widening up, `toFloat` down — §6); what waits is `float`'s own spec, with the type's
  delivery (numeric-tower §6).
- *(**Rounding functions: resolved** — `trunc`/`round`/`floor`/`ceil` are the core policy
  verbs (§7, conversion §2, R106), not a library question.)* **Fused multiply-add and
  explicit rounding modes stay deferred**, a std.math extension when the numeric audience
  arrives (math §5 deliberately omits them today).
- *(**Decimal type: resolved by R161** — and the bullet's lean was wrong twice: `decimal`
  exists, and it is **built-in**, not a library type (operators need the built-in line,
  numeric-tower §1.4), specced in full (decimal.md) with delivery post-alpha. The money
  motivation this bullet named is its §0 sentence.)*
