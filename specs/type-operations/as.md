# The `as` operator

`as` is the **checked narrowing** operator. It narrows a value from a wider type to a
narrower one it *already* satisfies (a union to one of its members, a supertype to a subtype),
checking at runtime that the value really is that narrower type. It is the counterpart to
type **annotations** (which assert a type that must *already* statically hold) and to
**conversion functions** (which transform one value into another).

The defining property: **`as` never transforms a value and never needs `!`.** Its only failure
is a `typeError`, which is a `panic` (errors §9), so a failing `as` is a programming error that
propagates ambiently, not a declarable error. `as` is a type-level assertion with a
runtime check, not a computation.

---

## 1. What `as` does

```
let u = someFn();            // u : string | int   (inferred union)
let s = u as string;         // narrow to string; typeError (panic) if u is currently an int
```

- **Union to member.** `string | int` narrowed to `string`. The value already is a string or
  it is not; `as` checks. On success, the result is typed `string`. On mismatch, it raises a
  `typeError` (panic).
- **Subtype narrowing.** A supertype narrowed to a subtype it currently is: `error as
  commandError`, `table as list`, `ioError as fileNotFound`. Same check, same `typeError` on
  mismatch.
- **Safe widening.** The reverse direction (member to union, subtype to supertype, `list` to
  `table`, `commandError` to `error`) always succeeds, because the value already satisfies the
  wider type. Widening is **implicit**: a `list` is usable anywhere a `table` is expected with
  no `as`, a `commandError` is usable as an `error`, and so on. `as` is reserved for the
  narrowing direction, the one that can fail. Writing `list as table` is redundant and
  unnecessary.

In every case `as` leaves the value's bits unchanged; it only changes (and checks) the type
the value is *seen through*. Nothing is parsed, formatted, or recomputed.

---

## 2. `as` supplies the result type

`as T` is an expression of type `T`, so it **supplies the binding's type** and a separate
annotation is redundant:

```
let s = u as string;         // s : string, from the `as`
```

Writing both is allowed only if they **agree**; a disagreement is a compile error, because it
is a contradiction:

```
let s: string = u as string; // redundant but legal (they agree)
let s: int    = u as string; // COMPILE ERROR: annotation says int, `as` says string
```

Idiomatically, use one or the other: an **annotation** when the type already holds (no
narrowing), and **`as`** when narrowing (which then types the binding). You rarely write both.

---

## 3. `as` is not a value conversion

`as` narrows *types*; it never converts *values*. Transforming a value into a **different
value** is a function, not `as`, because it runs custom code at runtime:

```
let text = toString(42);        // int -> string: a total function, "42"
let n    = parseInt("42");      // string -> int: a fallible function, int!
```

- **`toString(n): string`** formats an int as text. It is a total function (every int has a
  string form), never fails.
- **`parseInt(s): int!`** parses text into an int. It is **fallible** (`"hello"` is not an
  int), so it returns `int!` (errorable) and its failure is a declarable error handled with `try`,
  because malformed input is an expected, recoverable condition, not a programming error.

The line between `as` and a conversion function is exact:

| | `as` (narrowing) | conversion function |
|-|-|-|
| Transforms the value? | No: the exact value, preserved | Yes: a new value |
| Runs custom code? | No, a type check | Yes |
| Failure kind | `typeError` (panic) | declarable error (`!`), or total (no failure) |
| Needs `!`? | Never | Yes if fallible (`parseInt`), no if total (`toString`) |

The criterion, stated once (R124): **losslessness**. `as` may move a value between
representations — `int as u64`, `int as decimal` (numeric-tower §3.1, §4) — exactly when
the value, where accepted, is preserved **exactly**, so that the only possible failure is
a membership/range check (`typeError` panic on a negative `int as u64`), never a
precision question. A direction that loses information is a conversion no matter how it
is dressed: `double as float` (rounds) and `decimal as int` (a fractional `decimal` has
no lossless `int` reading) do not exist — they are `toFloat` and the policy verbs
(conversion §2, numeric-tower §4). The retired shorthand "same bits" was wrong on both
sides: `int as u64` legally changes representation while preserving the value, and a
bit-preserving reinterpretation that changed the *value* would be exactly what `as`
forbids.

R161/R162 sharpened the criterion from the other side: **lossless is necessary, not
sufficient** — the preserved value must be the value the source type *presents*, not a
faithful embedding of its representation. `double as decimal` and `double as rational` are
**rejected although mathematically lossless** (every finite double embeds exactly), because
the embedding surfaces the binary representation's hidden digits (`0.1` →
`0.1000000000000000055511…`, the `BigDecimal(0.1)` trap) — lossless in the worst way. The
accepted contrast is `double as complex` (R164): the component is carried bit-for-bit into
the same format on a wider plane, nothing reinterpreted, no digits invented
(numeric-tower §3.1).

This split is why `as` keeps its clean "never `!`" property: everything that can fail with a
*handleable* error is a function returning `!`, so `as` is left with only the panic-on-mismatch
case, which never forces `!`. It also means **parsing is user-extensible**: a user-defined
`parseMyFormat(s: string): myType!` is exactly as first-class as `parseInt`, because both are
ordinary fallible functions. `as` could never offer that, since it is a fixed type check;
functions can.

---

## 4. Annotations assert, `as` narrows, conversions transform

The three are distinct and never silently substitute for one another:

- **Annotation** (`x: string = ...`): asserts a type the value must **already** have,
  compile-checked. `myStr: string = 5` is a compile error, `5` is not a string. An annotation
  **never** implies an `as`: `let s: string = someUnion()` is a compile error (narrow it
  explicitly), not a hidden runtime-checked narrowing.
- **`as`** (`x as string`): narrows a wider type to a narrower one the value currently
  satisfies, runtime-checked, `typeError` (panic) on mismatch. Explicit, visible at the point
  the check happens.
- **Conversion function** (`toString`, `parseInt`): transforms a value into a different value,
  running custom code; fallible ones return `!`.

So a runtime type check is always spelled `as` (never hidden in an annotation), and a value
transformation is always a function call (never hidden in `as`). Each operation is visible in
the source where it happens.

One position reuses the annotation's `:` and does check at runtime: a **pattern's typed binding**,
`match (x) { s: string => ... }` (match §2.2). This is not an exception to the rule above, because
nothing is hidden: a pattern's entire job is to be a predicate on the scrutinee, so the arm's
selection **is** the check, and it is written where it happens. The invariant the rule protects
holds in both positions, **an annotation never lies**: no value is ever seated in an `x: T` that is
not a `T`, whether the mismatch is caught at compile time (a declaration, `let s: string = someUnion()`)
or by falling through to the next arm (a pattern). What a pattern never does is *panic*: its test is
`is`, not `as` (is spec §1). Which reading `:` takes is fixed by position, as it is for `@`, `&`,
`!`, `error`, and `comptype` (type §1.1, associativity §2).

---

## 5. Incompatible types are a compile error

When two types can **never** be the same value, no `as` applies, the mismatch is caught at
compile time:

```
const someFn = fn (): int => ...;
let s: string = someFn();        // COMPILE ERROR: int is not string, and cannot be
let s = someFn() as string;      // COMPILE ERROR: int and string are disjoint; `as` cannot narrow
```

`as` narrows *within* a type relationship (a union to a member, a supertype to a subtype);
it cannot bridge disjoint types (`int` and `string` share no values). Bridging those is a
*conversion* (§3), `toString` / `parseInt`, not a narrowing. So `int as string` is a compile
error (use `toString`), and `string as int` is a compile error (use `parseInt`).

### 5.1 Function narrowing: obligations eagerly, values per call

Narrowing to a **function type** splits, and the split is not the one an earlier draft drew.
Everything about a function value is decidable **at the `as`**: the function typeid is *signature
plus errorability*, and nothing else (functions §3), so the real parameters, the real result, and
whether the function throws are all in hand the moment the `as` runs. Function types are not erased.
Deferral is therefore **not forced by ignorance**, and the rationale this section used to give, that
a function's conformance is "observable only when it runs," was false.

`as` defers because **`as` is licensed to assert a claim that is not a subtype relation.** Narrowing
`fn (int): (int | string)` to `fn (int): int` is allowed *optimistically*: no comparison of signatures
can validate it, because it is not a fact about the type, only (perhaps) about the returns that will
actually happen. Such a claim can be tested only against the calls that occur, so that is where it is
paid for. The license has exactly one boundary, and it is what keeps the model sound:

> **`as` may be optimistic about *values*, never about *obligations*.**

An argument type and a result type are **beliefs** about the calls that will occur; the caller may
know more than the type system does, and being wrong is caught at the call. **Errorability** is a
caller **obligation** (errors spec): you cannot believe a function will not throw and be checked
afterwards, because the check would arrive *after* the throw. A **`&` parameter** is an obligation
too, a write channel (variables §5.1): the calls that occur bound what a callee reads, never what it
writes back.

So **kind, errorability, reference positions, and hopelessness are eager** (a compile error where the
source's signature is statically known, a `typeError` at the `as` otherwise), while **argument and
result conformance are deferred** to each call, checked contravariantly against the callee's real
parameters and covariantly against the caller's claim. A narrowing no call could ever satisfy is
*hopeless* and is rejected where it is written, not at some later call that may never come.

One check is deferred by genuine necessity rather than by license. A function's **capability
requirement set rides the value, not the type** (functions §3, capabilities §3.1), while the *grant*
rides the executing frame, which the `as` site does not know. A call through an `fn`-typed slot
checks the two against each other and panics on shortfall. That, and not signature conformance, is
the one thing about a function that is observable only when it runs, and it is the one place "check
when you can" genuinely cannot hoist a check.

The full rules, the narrowing table, and the definition of hopelessness live in functions §3.2
and §3.2.1.

---

## 6. Known uses across the specs

- **`"text" as secret`** (secret spec §3): a **lossless entry** into another type — the
  `int as decimal` class (§3, numeric-tower §3.1): the payload is preserved exactly, only
  now sealed, and the move is explicit though it cannot fail, because the crossing should
  be *seen* — "where are secrets created" is answerable by searching `as secret`, secret
  §3's own argument. (The gated constructor `secret(...)` is a separate, R79 form — secret
  §3.2; and the asymmetry: `secret` back to `string` is *not* `as`, it is `reveal`, a
  capability-gated extraction.)
- **`tab as list`** (tables §2.1, §2.3): narrowing a `table` to a `list`, checked. It
  **asserts** the table is *already* a list and raises a `typeError` (panic) if it is not
  (has a gap or a string key). It does not reshape the table; if it passes, the same value is
  simply re-typed. Contrast `tab.values()`, which **produces** a fresh list by reindexing and
  therefore *always* succeeds regardless of the table's shape:

  ```
  someFn(tab)              // COMPILE ERROR: table is not implicitly a list
  someFn(tab as list)      // legal; asserts tab is currently a list, typeError panic if not
  someFn(tab.values())     // always legal; builds a fresh list from tab's values (reindexes)
  ```

  So `as list` and `values()` are not interchangeable: `[5=>'a', 9=>'b'] as list` panics
  (not contiguous), while `[5=>'a', 9=>'b'].values()` returns `['a', 'b']`. Use `as list`
  when a non-list is a bug you want caught (assert current type); use `values()` when you
  want a list out of any table (transform). This is the general split (§3): `as` asserts a
  type, a function transforms a value.
- **`e as commandError`** (errors, exec): narrowing a caught base `error` to a specific error
  type, checked. The `is` form (`e is commandError`) is the **boolean test** counterpart: it
  reports whether `e` is a `commandError` but does **not** narrow `e`; to obtain a `commandError`
  binding you use `as` (or `match`), which produces a new narrowed value.
- **The numeric tower's `as` moves** (numeric-tower §2–§4): integer narrowing and crossing
  (`int as i8`, `u64 as u8`, `int as u64`), the arbitrary-precision entry
  (`int as decimal`), and the plane entry (`double as complex`, R164) are `as` because
  each is lossless where it accepts and preserves the value as read — range is the
  only question, a panic. The lossy directions are functions, not `as`: `double as float`
  and `decimal as int` do not exist (`toFloat`; `trunc`/`round`/`floor`/`ceil`), and
  `double as decimal` / `double as rational` are rejected though lossless (§3's
  sharpening, R161/R162 — the crossing is `parseDecimal(toString(d))`, lossy moment
  visible). And int-to-double was a function before the rule had its name: `toDouble` is
  lossy above 2^53, which is why R106 could never have spelled it `as`. §3's criterion,
  applied.

---

## 7. Narrowing produces a new binding; `is` does not flow-narrow

Narrowing in Luna **only** happens by producing a **new binding** of the narrower type, through
`as` (`let s = x as string`, checked, panics on mismatch) or through a `match` arm's typed binding
(`match (x) { s: string => ... }`, which binds a fresh `s`). A binding's type is **fixed at its
declaration** and never changes based on the branch it is in. A match arm with a *bare* binder
narrows nothing: `match (x) { s => ... }` binds `s` at `x`'s own type (match §2). The binder is what
narrows, so a type test that discards its binding (`_: string`) narrows nothing either, and has
nothing to narrow.

In particular, **`is` is a boolean test only**: it reports whether a value currently has a type, and
does **not** narrow the tested binding within the guarded branch.

```
if (x is int) {
  foo(x);          // x is STILL its declared type here (e.g. int | string), not narrowed
  let n = x as int; foo(n);   // to use it as int, produce a narrowed binding
}
match (x) {
  n: int => foo(n); // idiomatic: the arm binds a fresh n of type int
}
```

This is a deliberate design choice tied to the compiler guarantee: Luna does **no control-flow
analysis** (compiler spec). Flow-narrowing, changing `x`'s type inside a branch because of a
preceding `is` test, would require the compiler to track a binding's type *along control-flow paths*,
which is exactly the intraprocedural flow-typing Luna refuses to do. By confining narrowing to
new-binding forms (`as`, `match`), the narrowed type is always a property of a *fresh binding at its
declaration*, never a path-dependent fact about an existing one, so no flow analysis is needed. The
tools (`match`, `as`) cover every case; `if (x is int) { let n = x as int; ... }` is the explicit
form, and `match` is the ergonomic one.

`x as T` is equivalent to "`x` if `x is T`, else raise `typeError`": `is` is the total
(non-panicking) test, `as` the asserting (panicking) narrowing that yields the narrowed value.

## 8. Open questions

*(none — the one this section carried is resolved:)*

- *(**`as` on secret payloads: resolved by R113.** `reveal` returns the union
  `string | bytes | table`, and the result is ordinary union typing, narrowed with
  `as string` (checked) or `match` (total) — secret §5. No special rule was needed;
  the open predated the union form.)*
