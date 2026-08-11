# Rational

`rational` is the **exact-fractions type**: the member of the numeric tower where
**division is exact**. `1/3` is not an approximation and not an error — it is the
value `1/3`. It is the committed counterpart to `decimal` (numeric-tower §3, §5) —
**specced here (R162), delivered post-alpha** with the extended tower (§6) — and the
two types split one job cleanly, by ruling (decimal §2, R161): **`decimal` is exact
radix-10 arithmetic; `rational` is exact division.**

---

## 1. Representation and model

A rational is **two arbitrary-precision integers** — numerator and denominator — held
in **canonical form as an invariant**: `gcd(n, d) = 1`, denominator positive, sign on
the numerator, re-reduced after every operation. Heap-backed, grows, never overflows
(the committed row, numeric-tower §5).

Precision on the model (R162): a rational is *not* two decimals — it is **two of the
thing inside a decimal**. The shared component is the runtime's internal
arbitrary-precision integer (the bignum decimal's unscaled value uses; there is no
user-facing `bigint`, numeric-tower §7): a decimal is one bignum *plus a scale*, a
rational is two bignums, *scale-free*. Two-decimals could not be the semantics even
in principle: the scales are redundant degrees of freedom that canonicalization
immediately clears (`(a×10⁻ˢ)/(b×10⁻ᵗ)` reduces to an integer pair), and canonical
form is load-bearing — **one representation per value** is what makes equality
trivially structural and the `1/2`-versus-`2/4` footgun unrepresentable.

`rational` is its own family (numeric-tower §3): no implicit widening, no
mixed-family arithmetic — `r + 1` is a compile error; construct the operand
(`1 as rational`). `kindOf` answers `{scalar}` (introspection §4.3); `@x` reports
`rational`.

## 2. The operator table: all four, exact — the mirror of decimal's

| Expression | Result | Notes |
|-|-|-|
| `a + b`, `a - b`, `a * b`, **`a / b`** | `rational` | **exact, always** — including division; results re-reduce to canonical form |
| `-a` | `rational` | exact |
| `==`, `!=`, `<`, `<=`, `>`, `>=` | `bool` | strict, same-type; equality is structural *because* canonical form makes structural and value equality coincide (§1); ordering by exact cross-multiplication |

- **`/` is an operator here and division is total** except `a / 0`, which panics
  (`divisionByZero`, the tower-wide rule; the committed row's "panic on /0"). The
  tables are deliberate mirror images: decimal omits `/` because its division is a
  rounding decision (decimal §2); rational owns `/` because its division is exact.
  R161's family split is thereby a theorem, not a pointer.
- **`%` is omitted**, with the shortest rationale in the tower: **exact division
  leaves no remainder.**

## 3. Crossing to and from `decimal`

The two exact types interconvert with no silent loss in either direction:

```luna
fn toRational(d: decimal): rational                    // total and exact, always
fn exactDecimal(r: rational): decimal!                 // exact, or an error
fn toDecimal(r: rational, scale: int,
             rounding: roundingMode = {halfEven}): decimal   // total: every rational rounds
```

- **`toRational` is total and exact** — a decimal already *is* rational-shaped
  (`n × 10⁻ˢ` is `n/10ˢ`: one bignum over a power of ten); the conversion writes the
  scale as a denominator and reduces. `to*`'s total contract (conversion §2) holds.
- **`exactDecimal` is the demand form**: exact or error. `n/d` has a finite decimal
  expansion **iff the reduced denominator's prime factors are only 2 and 5** — `3/8`
  converts exactly, `1/3` errors. It sits outside the `to*`/`parse*`/`from*` prefix
  families deliberately, beside the policy verbs (conversion §2's "decisions, not
  conversions" shelf).
- **`toDecimal` with a required `scale` is total** — every rational rounds to any
  scale — so the `to*` contract survives untouched; it is decimal's own `div`
  philosophy (decimal §3) arriving from the other side, same `roundingMode` enum,
  same `{halfEven}` default.
- **The policy verbs widen once more** (completing R124's promise as extended by
  R161): `trunc` / `round` / `floor` / `ceil` take `double | decimal | rational`,
  each `int!` (conversion §2, §5).

## 4. Boundaries

- **`parseRational(s: string): rational!`** — the primary boundary (the R106 `parse*`
  contract): accepts `"2/3"` (reduced on entry), integer text (`"7"` → `7/1`,
  displayed `"7"`), and decimal text (`"1.5"` → `3/2`). Errors on text with no
  rational reading, `"1/0"` included (a value that cannot exist is data-shaped
  failure here, not a panic — no division was attempted).
- **The literal story is comptime** (the R161 story verbatim):
  `const third = parseRational("1/3");` folds at compile time — literal ergonomics,
  zero grammar.
- **`int as rational`** — lossless, legal `as` (R124's criterion; the entry
  numeric-tower §3.1 always gestured at).
- **`double as rational`: considered and rejected** (R162, the R161 trap symmetric —
  and sharper here, because the embedding would be *mathematically exact*: every
  finite double **is** a rational, a dyadic one, which is precisely why `0.1 as
  rational` would faithfully carry the binary noise as
  `3602879701896397/36028797018963968`). The deliberate crossing is
  **`parseRational(toString(d))`** — the shortest-round-trip text a human reads off
  the double, with the lossy moment visible.

## 5. Text and serialization

- **`toString` is total and canonical**: reduced form, sign on the numerator,
  integral values without the `/1` (`"3"`, not `"3/1"`; `"-2/3"`).
  `parseRational(toString(r)) == r`, always — the round-trip law.
- **`toJson` serializes the canonical string** (the R132/R161 precedent, third time):
  a JSON number cannot hold `1/3` at all; the string round-trips via `parseRational`
  at the reader.

## 6. Deliberately absent

- **`%`** (§2 — no remainder exists), **transcendentals** (decimal §7's argument
  verbatim: an exact type cannot hold inexact results honestly; that arithmetic is
  `double`'s), **a reciprocal function** (`1 as rational / r` is three tokens),
  **numerator/denominator accessors** for alpha (their honest return type is the
  nonexistent `bigint`; `toString` exposes the pair as text, and a real need can
  revisit — recorded, not forgotten).
- **`complex` interop** — *resolved by R164, as never*: `complex` does not cross to or
  from the exact types directly. Its components are doubles, and `double`'s crossings
  here are already ruled (string-mediated, §4) — a direct path would smuggle that
  lossy moment out of sight, twice (complex §4).
