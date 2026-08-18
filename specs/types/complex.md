# Complex

`complex` is the **complex-plane numeric type**: a real/imaginary pair of `double`s
(`a + bi`, where `i² = -1`), with the standard complex arithmetic. It is the numeric
tower's scientific member — the working type of signal processing, electrical
engineering, control theory, and fractal work — and its committed row has said "a pair
of doubles, IEEE per component" all along (numeric-tower §1.4, §5). **Specced here
(R164), delivered post-alpha** with the extended tower (numeric-tower §6).

The one-sentence model: **`double`'s semantics on a plane.** Unlike its shelf-mates
`decimal` and `rational`, `complex` is *not* an exact type: its components are IEEE
doubles, and every IEEE behavior — inf, nan, rounding — carries over per component.
What it adds is not precision but *closure*: with `i` in the number system, negative
numbers have square roots and every polynomial has its full set of roots.

---

## 1. Representation and model

A complex is **two `double`s** — the real part and the imaginary part. Heap-backed
(numeric-tower §5): sixteen bytes exceed the `lval` payload word, so it is a
reference type like `rational`, one allocation, immutable like every scalar.
`kindOf` answers `.{scalar}` (introspection §4.3); `@x` reports `complex`.

**The component type is always `double`, permanently** (R164, closing numeric-tower
§7's open). No `complex` over `float` (a storage optimization for large arrays, an
audience Luna does not serve), none over the exact types (Gaussian rationals have no
practical audience), and no parameterization mechanism to express either. One
component type also keeps the backend story trivial: `complex` is Go's native
`complex128`, boxed (compiler §7.5).

`complex` is its own family (numeric-tower §3): no implicit widening, no
mixed-family arithmetic — `z + 1.0` is a compile error; construct or convert the
operand (`someDouble as complex`, §4). The tower's uniform stance, as everywhere.

## 2. The operator table: four operators, no ordering

| Expression | Result | Notes |
|-|-|-|
| `a + b`, `a - b`, `a * b`, `a / b` | `complex` | the standard formulas over the components; **IEEE per component** — inexact, no panic, inf/nan where IEEE says so |
| `-a` | `complex` | negates both components (numeric-operators §1.1) |
| `==`, `!=` | `bool` | strict, same-type; componentwise IEEE — a nan component poisons `==` exactly as in `double` (**not reflexive** when a component is nan; equality §3) |

- **`/` is an operator here**, unlike `decimal`'s (decimal §2), and the contrast is
  principled: decimal banished `/` because rounding would hide a *policy decision
  inside an exact type*; `complex` is already inexact — its components are doubles —
  so division hides nothing that `double`'s own `/` does not. **Division by complex
  zero yields IEEE inf/nan components, never a panic** — the float family's rule
  (numeric-tower §4), deliberately *not* `rational`'s `divisionByZero`: the format
  defines the sentinels, so the language uses them.
- **`%` is omitted** — remainder has no meaning on the plane (compile error, as for
  every operator a type does not define).
- **There is no ordering: `<`, `<=`, `>`, `>=` are compile errors on `complex`.**
  This is a theorem, not a taste call: no total order can coexist with the
  arithmetic (were `i > 0`, then `i·i = -1 > 0`; were `i < 0`, the same
  contradiction — both squares of nonzero values would have to be positive).
  `complex` is the tower's one unordered member (operators §2's ordering row);
  sorting complex values means choosing an explicit key — `magnitude(z)`, `real(z)`
  — and writing it.

## 3. Accessors

```luna
const real = fn (z: complex): double => {};         // total, exact: the real component
const imag = fn (z: complex): double => {};         // total, exact: the imaginary component
const conj = fn (z: complex): complex => {};        // total, exact: a - bi (flips the imaginary sign)
const magnitude = fn (z: complex): double => {};    // total: |z| = √(a² + b²), computed hypot-style
```

- `real` and `imag` are the exits from the type: **there is no `complex as double`**
  (§4) — reading a component is an accessor, not a narrowing.
- `magnitude` is computed without intermediate overflow (the `hypot` discipline,
  math §2's inclusion criterion for that function).
- **`magnitude` is deliberately not `abs`**: math's `abs` is `fn (x: number): number`
  with **kind follows the operand** (math §2, the R92 catalogue precedent) — a
  contract `complex` cannot honor, since the absolute value of a complex is a
  *double*. One name, one signature (functions §3.4): the operand-preserving
  operation keeps `abs`, the plane-to-line projection gets its own name.
- The **argument/phase angle** (`arg`) and polar construction are deferred with the
  transcendentals (§6).

## 4. Boundaries: how values become `complex`

- **`complex(re: double, im: double): complex`** — the constructor, and **the
  literal story** (the R161 story verbatim): it is pure, so
  `const w = complex(3.0, -4.0);` **folds at compile time** — literal ergonomics,
  zero runtime cost, zero new grammar. A dedicated literal form (an `i` suffix,
  `3.0-4.0i`) is deferred until it earns grammar — and it would need real grammar:
  the naive `-3-4i` cannot typecheck under the family rules (`3` is `int`, `4i`
  would be `complex`, and cross-family arithmetic is explicit), so a literal form
  means either untyped-constant machinery (Go's answer, rejected as a mechanism) or
  a single fused lexical form. The constructor makes the question idle.
- **`parseComplex(s: string): complex!`** — the text boundary (the R106 `parse*`
  contract): accepts the canonical form (§5) plus a bare real (`"3.0"`) or bare
  imaginary (`"4.0i"`) part, components per `parseDouble`. Errors on text with no
  complex reading.
- **`double as complex` — lossless, legal `as`** (R124's criterion; the
  numeric-tower §3.1 entry pattern). The component is carried **bit-for-bit**: none
  of the trap that rejected `double as decimal` and `double as rational`
  (R161/R162) exists here, because the value is not *reinterpreted* into a
  different radix or reduced form — it is the same double, now on the plane.
  Explicit because it allocates (the lossless-but-not-cheap rule, numeric-tower
  §3.1). An `int` enters via `double` (`n.toDouble() as complex`), inheriting
  `toDouble`'s exact-to-2^53 caveat visibly.
- **`complex as double` does not exist**: a complex with a nonzero imaginary part
  has no double reading, and "take the real part" is a projection, not a narrowing
  — spelled `real(z)`, always (§3).
- **No exact-type interop, ever** (resolving rational §6's parked item): `complex`
  does not cross to or from `decimal`/`rational` directly. Its components are
  doubles, and `double`'s crossings to the exact types are already ruled
  (string-mediated, visible; decimal §5, rational §4) — a direct
  `complex`-to-exact path would just smuggle those crossings out of sight, twice.

## 5. Text and serialization

- **`toString` is total and canonical**: both components always rendered, real part
  first, each by `double`'s own rendering, the sign fold between them —
  `"3+4i"`, `"3-4i"`, `"-1.5+0.5i"`, `"0+0i"`, `"nan+infi"`. The `i` is always
  present; there is no shortened pure-real or pure-imaginary output form (those are
  *accepted* by `parseComplex`, not produced). `parseComplex(toString(z)) == z` —
  the round-trip law, with `double`'s one asterisk: a nan component round-trips
  bit-faithfully but `==` cannot confirm it (nan contagion, equality §3).
- **`toJson` serializes a complex as its canonical string** (the fourth application
  of the R132 precedent, after `duration`, `decimal`, and `rational`): a JSON
  number cannot hold a pair. Readers `parseComplex` at the boundary.

## 6. Deliberately absent

- **Ordering** — impossible, not deferred (§2).
- **`%`** — no meaning on the plane (§2).
- **Transcendentals and polar form** (`arg`, `sqrt`, `exp`, `log`, trig over
  `complex`; construction from magnitude and angle) — *deferred*, not rejected:
  unlike decimal §7's exactness argument, an inexact type can hold their inexact
  results honestly, but the surface waits for the scientific audience that needs it
  (a future std.math extension or std.cmath).
- **A `float`-component or exact-component `complex`** — rejected permanently (§1).
- **Exact-type interop** — rejected (§4); crossings go through `double`.
