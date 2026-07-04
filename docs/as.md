# The `as` operator

`as` is the **checked narrowing** operator. It narrows a value from a wider type to a
narrower one it *already* satisfies (a union to one of its members, a supertype to a subtype),
checking at runtime that the value really is that narrower type. It is the counterpart to
type **annotations** (which assert a type that must *already* statically hold) and to
**conversion functions** (which transform one value into another).

The defining property: **`as` never transforms a value and never needs `!`.** Its only failure
is a `TypeError`, which is a `panic` (errors §9), so a failing `as` is a programming error that
propagates ambiently, not a declarable `UserError`. `as` is a type-level assertion with a
runtime check, not a computation.

---

## 1. What `as` does

```
let u = someFn();            // u : string | int   (inferred union)
let s = u as string;         // narrow to string; TypeError (panic) if u is currently an int
```

- **Union to member.** `string | int` narrowed to `string`. The value already is a string or
  it is not; `as` checks. On success, the result is typed `string`. On mismatch, it raises a
  `TypeError` (panic).
- **Subtype narrowing.** A supertype narrowed to a subtype it currently is: `error as
  commandError`, `table as list`, `capability as reveal`. Same check, same `TypeError` on
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

`as` narrows *types*; it never converts *values*. Transforming a value into a different value
(with different bits) is a **function**, not `as`, because it runs custom code at runtime:

```
let text = toString(42);        // int -> string: a total function, "42"
let n    = parseInt("42");      // string -> int: a fallible function, int!
```

- **`toString(n): string`** formats an int as text. It is a total function (every int has a
  string form), never fails.
- **`parseInt(s): int!`** parses text into an int. It is **fallible** (`"hello"` is not an
  int), so it returns `int!` (errorable) and its failure is a `UserError` handled with `try`,
  because malformed input is an expected, recoverable condition, not a programming error.

The line between `as` and a conversion function is exact:

| | `as` (narrowing) | conversion function |
|-|-|-|
| Transforms the value? | No, same bits | Yes, new value |
| Runs custom code? | No, a type check | Yes |
| Failure kind | `TypeError` (panic) | `UserError` (`!`), or total (no failure) |
| Needs `!`? | Never | Yes if fallible (`parseInt`), no if total (`toString`) |

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
  satisfies, runtime-checked, `TypeError` (panic) on mismatch. Explicit, visible at the point
  the check happens.
- **Conversion function** (`toString`, `parseInt`): transforms a value into a different value,
  running custom code; fallible ones return `!`.

So a runtime type check is always spelled `as` (never hidden in an annotation), and a value
transformation is always a function call (never hidden in `as`). Each operation is visible in
the source where it happens.

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

---

## 6. Known uses across the specs

- **`"text" as secret`** (secret spec §3): widening a `string` into a `secret`. A total,
  never-failing coercion (any string is a valid secret), so it is `as`, not a function. (Its
  asymmetry: `secret` back to `string` is *not* `as`, it is `reveal`, a capability-gated
  extraction.)
- **`tab as list`** (tables §2.1, §2.3): narrowing a `table` to a `list`, checked. It
  **asserts** the table is *already* a list and raises a `TypeError` (panic) if it is not
  (has a gap or a string key). It does not reshape the table; if it passes, the same value is
  simply re-typed. Contrast `tab.values()`, which **produces** a fresh list by reindexing and
  therefore *always* succeeds regardless of the table's shape:

  ```
  someFn(tab)              // COMPILE ERROR: table is not implicitly a list
  someFn(tab as list)      // legal; asserts tab is currently a list, TypeError panic if not
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

---

## 7. Narrowing produces a new binding; `is` does not flow-narrow

Narrowing in Luna **only** happens by producing a **new binding** of the narrower type, through
`as` (`let s = x as string`, checked, panics on mismatch) or through a `match` arm (`match (x) {
string s => ... }`, which binds a fresh `s`). A binding's type is **fixed at its declaration** and
never changes based on the branch it is in.

In particular, **`is` is a boolean test only**: it reports whether a value currently has a type, and
does **not** narrow the tested binding within the guarded branch.

```
if (x is int) {
  foo(x);          // x is STILL its declared type here (e.g. int | string), not narrowed
  let n = x as int; foo(n);   // to use it as int, produce a narrowed binding
}
match (x) {
  int n => foo(n); // idiomatic: the arm binds a fresh n : int
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

`x as T` is equivalent to "`x` if `x is T`, else raise `TypeError`": `is` is the total
(non-panicking) test, `as` the asserting (panicking) narrowing that yields the narrowed value.

## 8. Open questions

- **`as` on secret payloads:** interaction with `reveal` once `bytes` exists (whether a
  revealed `string | bytes` is narrowed with `as`), pending the `bytes` type.