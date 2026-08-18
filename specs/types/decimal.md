# Decimal

`decimal` is the **exact radix-10 numeric type**: the type for currency and every
domain where `int` overflows the fraction and `double` corrupts it. `0.1 + 0.2 ==
parseDecimal("0.3")` is `true`, exactly, always. It is a committed member of the
numeric tower (its own family, numeric-tower §3, §5) — **specced here (R161), delivered
post-alpha** with the rest of the extended tower (numeric-tower §6).

The one-sentence model: **string-like exactness without string storage.**

---

## 1. Representation and model

A decimal is `unscaledInteger × 10⁻ˢᶜᵃˡᵉ` — an **arbitrary-precision integer** plus a
scale. `19.99` is `1999 × 10⁻²`. Heap-backed (numeric-tower §5), one allocation,
**grows and never overflows**: addition, subtraction, and multiplication are *always
exact* — the digits simply grow. Every decimal digit a program was given is preserved;
none is invented. `kindOf` answers `.{scalar}` (introspection §4.3); values are
immutable like every scalar; `@x` reports `decimal`.

`decimal` is its **own family** (numeric-tower §3): no implicit widening in or out, no
mixed-family arithmetic — `d + 1` is a compile error; construct the operand
(`1 as decimal`, §5). This is the tower's uniform stance, not a decimal special case.

## 2. The exact operators — and the two that are deliberately missing

| Expression | Result | Notes |
|-|-|-|
| `a + b`, `a - b`, `a * b` | `decimal` | **exact, always** — digits grow, nothing rounds, nothing overflows |
| `-a` | `decimal` | exact |
| `==`, `!=`, `<`, `<=`, `>`, `>=` | `bool` | strict, same-type (`parseDecimal("1") == 1` is `false`, as everywhere); equality is **normalized** (§4) |

**`/` and `%` do not exist for `decimal` — compile error, and the error names `div`
(§3).** This is the R106 policy-verb discipline applied to the one operation where
exact decimal arithmetic is mathematically impossible: `1 / 3` has no finite decimal
expansion, so decimal division *is* a rounding decision, and a rounding decision
hidden inside an operator is exactly what this language refuses (the same argument
that made `toInt(d)` into `trunc`/`round`/`floor`/`ceil`). Nobody divides money
without saying to how many places and which way ties go — so the language makes that
sentence the signature.

The desire for *exact* division has a home, and it is not this type: **`rational`**,
the exact-fractions family (numeric-tower §3, §5, committed and deferred). The family
split does the work — `decimal` is exact radix-10 *arithmetic*; `rational` is exact
*division*.

## 3. `div`, and the `roundingMode` enum

```luna
const div = fn (a: decimal, b: decimal, scale: int, rounding: roundingMode = .{halfEven}): decimal => {};

export const roundingMode = enum { halfEven, halfUp, trunc, floor, ceiling };
```

- **`scale` is required**: the result has exactly `scale` fractional digits, rounded
  per `rounding`. A division that happens to terminate within `scale` rounds exactly
  (no special case). Division by zero is `divisionByZero`, the tower-wide panic.
- **`.{halfEven}` (banker's rounding) is the default** — the statistically unbiased
  choice and the finance-industry standard; **`.{halfUp}`** is what humans expect of
  money at the till; `.{trunc}`, `.{floor}`, `.{ceiling}` complete the set with the
  R106 verbs' own vocabulary. The enum is closed; exotic modes (half-down,
  half-toward-zero) wait for a real customer.
- **There is no ambient context** — Python's thread-local precision/rounding context
  was considered and **rejected**: it makes arithmetic results depend on frame state,
  the paradigm violation of the function-of-the-value-alone principle (constraints
  §0, R79's family), and it would poison comptime determinism. Every rounding in a
  Luna program is written at the site it happens.
- `%`'s analogue, when needed, is derivable (`a - b * div(a, b, 0, .{trunc})`); a
  named `rem` waits for demand.

## 4. Equality is normalized value equality

`parseDecimal("1.10") == parseDecimal("1.1")` is **`true`**: scale is
*representation*, value is *value*, and `==` compares values (equality §1's strict
same-type-same-value, with "value" meaning the number). This deliberately kills
Java's most infamous decimal footgun (`BigDecimal.equals` distinguishing `1.10` from
`1.1` while `compareTo` does not — two equalities, endless bugs). Consequences:

- Ordering, hashing (table keys), and `==` all agree, one notion of sameness.
- **Display scale is formatting, not identity**: "always show two places" is a
  rendering concern (`toString` renders normalized, §6; a `format` function with a
  places parameter is the eventual home, deferred with std.locale-adjacent work).
  The *number* `1.1` and the *display* `"1.10"` are different kinds of thing.

## 5. Boundaries: how values become `decimal`

- **`parseDecimal(s: string): decimal!`** — the primary boundary (the R106 `parse*`
  contract), because **text is how exact decimals actually arrive**: prices in JSON
  strings, amounts in CSV, user input. Errors on text with no decimal reading.
- **The literal story is comptime, and it dissolves the literal question**:
  `const price = parseDecimal("19.99");` — `parseDecimal` is pure, so this **folds at
  compile time**: literal ergonomics, zero runtime cost, zero new grammar. A
  dedicated literal form (`19.99d`) is deferred until it earns grammar.
- **`int as decimal`** — lossless, legal `as` (R124's criterion, unchanged).
- **`double as decimal`: considered and rejected** (R161, resolving R124's hedge to
  *no*). It would be lossless in the worst way: `0.1 as decimal` would faithfully
  embed the double's true value, `0.1000000000000000055511151231257827…` — Java's
  `new BigDecimal(0.1)` trap, the most famous decimal footgun in industry. The
  deliberate spelling, when a double must cross, is the composition
  **`parseDecimal(toString(d))`** — `toString` renders the shortest round-trip
  representation, so the composition yields the decimal a human reads off the double
  (the `BigDecimal.valueOf` behavior), and the two-step spelling makes the lossy
  moment visible.
- **`decimal as int` does not exist** (R124): a fractional decimal has no lossless
  `int` reading — the policy verbs `trunc`/`round`/`floor`/`ceil` **widen to
  `double | decimal`** with this type's landing, exactly as R124 promised
  (conversion §2, §5).

## 6. Text and serialization

- **`toString` is total and normalized**: the exact value, minimal digits, no
  trailing zeros (`"1.1"`, never `"1.10"`), no exponent games for ordinary
  magnitudes. `parseDecimal(toString(d)) == d`, always — the round-trip law.
- **`toJson` serializes a decimal as its canonical string** (the R132 `duration`
  precedent): a JSON *number* would round-trip through IEEE doubles and destroy the
  exactness that is this type's entire point. Readers `parseDecimal` at the boundary,
  which is where money validation belonged anyway.

## 7. Deliberately absent

- **Transcendental and irrational functions** (`sqrt`, `exp`, `ln`, trig) — an exact
  type cannot hold their inexact results honestly; that arithmetic belongs to
  `double` (std.math), with explicit, visible boundaries if a program crosses.
- **A precision/rounding context** — rejected (§3), permanently.
- **Scale-preserving display** — formatting's job (§4), not the value's.
- **`rational` interop** — *delivered with `rational`* (R162): the
  `toRational`/`exactDecimal!`/`toDecimal(scale)` trio, rational §3. The seam (exact
  division lives there) is fixed by the family split (§2).
