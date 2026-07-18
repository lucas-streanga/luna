# `std.math`

```
const math = import std.math;        // or selectively: import { sqrt, pi } from std.math
```

Scalar mathematics: constants, the elementary functions, and the statistics the
builtin catalogue lacks. **Everything here is pure, capability-free, and
comptime-eligible** — `const halfPi = pi / 2.0;` and `sqrt(2.0)` fold at build time —
and everything follows IEEE sentinel semantics (double spec): `sqrt(-1.0)` is `nan`,
`ln(0.0)` is `-inf`, nothing errors, nothing panics. What this module deliberately
does **not** duplicate: the catalogue owns collection aggregates (`sum`, `average`,
`product`, `min`, `max`, `mode` — iterable-functions §2.10) and double.md owns the
special-value probes (`isNan`, `isInf`, `isFinite`) and the int-conversion **policy
verbs** (`trunc`/`round`/`floor`/`ceil`, conversion §2).

## 1. Constants

```
export const pi = 3.141592653589793;
export const e  = 2.718281828459045;
```

## 2. Scalar functions

```
export const abs   = fn (x: number): number;             // kind follows the operand
export const sign  = fn (x: number): int;                // -1, 0, 1
export const clamp = fn (x: number, lo: number, hi: number): number;
export const lerp  = fn (a: double, b: double, t: double): double;

export const sqrt  = fn (d: double): double;
export const hypot = fn (x: double, y: double): double;
export const pow   = fn (base: double, exp: double): double;
export const exp   = fn (d: double): double;
export const ln    = fn (d: double): double;
export const log2  = fn (d: double): double;
export const log10 = fn (d: double): double;
```

- **`abs`, `sign`, `clamp` take `number`** (`int | double`, the predeclared union) with
  **kind following the operand** — the R92 catalogue precedent: `abs` of an `int` is an
  `int` (and `abs(minInt)` panics with `overflowError`, the int rule); of a `double`, a
  `double`.
- **`lerp(a, b, t)`** is `a + t·(b − a)` with the endpoint contract guaranteed:
  `lerp(a, b, 0.0) == a` and `lerp(a, b, 1.0) == b` **exactly** (the C++20 `std::lerp`
  discipline — the naive spellings lose the endpoints to rounding). `t` is unclamped:
  values outside `[0, 1]` extrapolate, which is the graphics convention; compose with
  `clamp` when clamping is meant.
- **`hypot` exists because the naive spelling is wrong**: `sqrt(x*x + y*y)` overflows
  for magnitudes `sqrt` would handle fine — which is the inclusion criterion for this
  module generally: a function earns a row when the obvious composition is a trap.
- **`ln`, not `log`**: bare "log" is ambiguous between mathematics (ln) and
  engineering (log₁₀), so no export bears it. Three precision-optimized names; a
  general base is `ln(x) / ln(base)`, a one-liner not worth a fourth.

## 3. Trigonometry

```
export const sin  = fn (r: double): double;      // radians, throughout
export const cos  = fn (r: double): double;
export const tan  = fn (r: double): double;
export const asin = fn (d: double): double;
export const acos = fn (d: double): double;
export const atan = fn (d: double): double;
export const atan2 = fn (y: double, x: double): double;

export const toRadians = fn (degrees: double): double;   // the to* contract (conversion §2)
export const toDegrees = fn (radians: double): double;
```

Radians only; degrees are a display unit, converted at the boundary with the `to*`
pair (total conversions, R106's contract exactly). Out-of-domain inputs produce `nan`
(`asin(2.0)`), the IEEE answer.

## 4. Statistics: what the catalogue lacks, split by the retention rule

```
export const variance = fn (it: iterable, sample: bool = false): double;
export const stddev   = fn (it: iterable, sample: bool = false): double;
export const median     = fn (xs: table): double;
export const percentile = fn (xs: table, p: double): double;    // p in [0, 1]
```

The R92 retention rule slots these exactly: **`variance` and `stddev` take
`iterable`** — single-pass (Welford), bounded working state, legal on a stream —
while **`median` and `percentile` take `table` only**, because they need a sort, the
whole-input family (on a stream: `s.collect().median()`… spelled `median(collect(s))`
or by UFCS). `sample: false` is population variance (÷ n); `sample: true` is Bessel's
correction (÷ n−1). `mean` is not here — it is the catalogue's `average`. Empty input
yields `nan` (the IEEE answer to an undefined statistic), never an error.

## 5. Deliberately absent

- **A double-returning `floor`/`ceil`/`round` family** — the policy verbs own rounding
  (conversion §2, `fn (d: double): int!`), and a parallel float-rounding family would
  put two meanings behind one name. If float-valued flooring is ever earned, it needs
  new names, not these.
- **Hyperbolics, `gcd`/`lcm`, combinatorics, integer `pow`** — deferred until a real
  program earns them; each is buildable today and none is average-program-blocking.
- **Distributions and sampling** — `std.random`'s deferral (random §7), not this
  module's.
