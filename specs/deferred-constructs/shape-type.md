# Deferred: shape types

**Status: deferred core-language construct (R212).** This is not a spec of something Luna
has; it is the full design analysis of something Luna deliberately does not have yet,
recorded so it is never re-derived — and so that if it is ever revived, the design lands in
the one form the corpus permits. The `deferred-constructs/` folder exists for exactly this
class: deferred *language* constructs, as distinct from deferred std libraries (recorded in
their modules) and deferred decisions (recorded where they arose).

## 1. The question

A general **shape type**: a structural record contract on plain tables —
`['radius' => int]` as a type, sealing a table to exact keys with typed values. Luna
deliberately has no such thing (match §4, tables spec); the one scoped exception is the enum
variant payload shape (enum §2.3, R211), justified there because two consumers *read* the
contract rather than run it.

## 2. The sharpened case for

- **The real payoff is not sealing — it is static field access.** Sealing is expressible
  today: a table constraint plus constraints §7's mutation-class machinery (breaking writes
  panic) is exactly "a record that cannot grow." What nothing gives is `t.radius` yielding
  `int` statically, no narrowing — a contract *read* by accessors and binders, not merely
  run. This is enum §2.3's two-readers lesson generalized, and it is the only payoff that
  would justify the construct.
- The re-validation complaint (match probing the same table's shape N times along a call
  chain) is answered by the constraint model's value-carried typeid (constraints §9.2), so
  it argues for *a constraint-shaped thing*, not for new checking machinery.

## 3. The sharpened case against — and why it currently wins

- **The corpus already has a records trajectory, and it is protos.** Every time the spec
  needed a typed record it reached for a proto: `@fileInfo` (R135), `@commandResult`
  (R172), `datetime` (R133) — const-get members *are* a sealed, statically-typed, read-only
  record. And the flagship lightweight-shape use case — `fromJson` into a known shape —
  already has a planned answer in json §4's open (the generated read side, parsing into a
  protocol-typed table). Shape types would open a **second record mechanism** beside a
  trajectory in motion: the language would stop having one answer to "how do I type a
  record" — the abuse vector named at proposal time, made concrete.
- **Small surface**: a new type former, with every question it drags (identity, membership
  axis, list-ness interaction), while the first mechanism is unproven in use.

## 4. The impossibility result: "static-only checking" cannot exist

The proposal's desire — checked "in the type itself," never at runtime like constraints —
is provably unavailable: it works only for values whose shape is statically known
(literals). The flagship use case is *dynamic* data (`fromJson` output), where there is
nothing static to check — **entry must validate at runtime, or shapes cannot be applied at
boundaries**, which is where they are wanted most. Any Luna shape type is therefore forced
onto the **constraint checking model**: static where provable (the enum §3.1
static-when-possible rule), runtime at entry otherwise, mutation-class checks thereafter
(value-carried, aliasing-safe — constraints §7, §9.2). The distinction from constraints is
never *when* it checks — only *what the compiler can read*.

## 5. The forced design, if ever revived

The analysis leaves exactly one corpus-shaped form:

```
const circle = shape ['radius' => int];
```

- **A const-only declaration form** (the R137 constraint discipline), braceless, reusing
  the R211 payload spelling (brackets, `=>` rows — braces belong to enums and blocks).
- **Sugar over a table constraint plus retained structure**: the generated predicate checks
  through machinery that already exists (§7 entry + mutation class, §9.2 value-carried);
  the retained key→type table is what the compiler *reads* — static field access, binder
  types, field-level diagnostics. No new checker; one new reader.
- **Exact and closed, no width subtyping** — inherited from enum §2.3/§3.1.
  `['radius' => int]` is not a subtype of a two-field shape, ever. Width subtyping is the
  swamp (row polymorphism, field variance, another membership axis); the enum exception
  stayed simple *because* it is closed, and a general form survives only by keeping that.
- **Inline anonymous shapes: rejected permanently** (`let t: ['radius' => int]` in a
  signature) — the ad-hoc-shapes-instead-of-declared-records abuse vector, plus the
  identity/interning swamp per utterance. Shapes, if they exist, are named declarations.

## 6. The revival condition

Revisit **only** if post-alpha experience shows protos too heavy for plain data records —
the apply ceremony, the nominal-brand identity (R126), or the serialization mode split
(R125) proving a real tax on the record-heavy workloads json §4's generated read side is
meant to serve. Absent that evidence, the deferral stands: protos are the record answer,
match is the probe, constraints are the value refinement, and the variant payload shape
remains the lone, justified exception.
