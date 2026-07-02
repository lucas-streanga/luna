# Match

`match` selects a result by testing a value (or a series of conditions) against arms in order
and taking the first that matches. It is an **operator** and an **expression**: it produces a
value, so it can be assigned, returned, and nested. It has two forms, **valued match** (a
scrutinee matched against patterns) and **open-ended match** (no scrutinee, a chain of boolean
guards), under one keyword.

```
let label = match (code) {
  10 => "ten",
  20 => "twenty",
  _  => "other",
};
```

---

## 1. Valued match: patterns over a scrutinee

`match (expr) { ... }` evaluates `expr` once (the scrutinee) and tests each arm against it in
order, top to bottom, taking the **first** that matches. An arm is:

```
PATTERN [where GUARD] => BODY
```

the pattern matched against the scrutinee, an optional `where` guard adding conditions (§3), and
the body producing the arm's value. Every arm is, at bottom, **a predicate on the scrutinee**.

---

## 2. Pattern positions

A **pattern** is matched against the scrutinee (or, when nested, against a sub-value). A pattern
is one of a small set of kinds, and each does exactly **one** job, matching, binding, or
recursing, so patterns stay simple and conditions live in the guard (§3):

- **Literal** (`10`, `"move"`, `true`, `NaN`): matches iff the value **equals** it, by the
  **total order** (§6), not IEEE `==`, so it is well-behaved for every type (a `NaN` literal
  matches a NaN, `0.0` matches both zeros; double spec §2.2).
- **Wildcard** `_` (wildcard spec): matches anything, **binds nothing** (discard).
- **Binding** `name`: matches anything, **binds** the value to `name`. The binding is scoped to
  **that arm only**: it is visible in the arm's guard (§3) and body, and **nowhere else**, not in
  sibling arms, not after the match. This is the only sound scoping, since exactly one arm runs
  (§7) and match is an expression whose information leaves through its result value, not through
  leaked names. To use a bound value outside the match, return it from the arm body (the value
  escapes as the match result; the name does not).
- **Type pattern** `@T` (optionally binding: `@T name`): matches iff the value **is** of type
  `T` (`value is T`), and **narrows** it to `T`. You match a value always; to match on a type you
  use `@` (the type-of operator), so `@string` is the type as a value to match against. `match
  (@x) { @int => ..., @string => ... }` dispatches on `x`'s type. Appending a name binds the
  matched value: **`@int n`** is the concise **typed binding**, "an `int`, bound to `n`" (narrowed
  to `int` in the guard and body), and it composes with a guard: `@int n where n > 10`. So `@T`
  type-checks without binding, and `@T name` type-checks and binds, both building on the same `@`
  rather than a separate `as` or `:` form.
- **Table / list pattern** (`['k' => sub]`, `[sub, sub]`): matches the value's **shape**
  structurally, recursing into sub-patterns (§4).

A position does not carry conditions or types-plus-guards inline; those go in the `where` guard
(§3). This is what keeps a position readable: it either matches a literal, binds a name,
discards, tests a type, or recurses, never all at once.

---

## 3. Guards: one `where` per arm

Conditions on the bound values, value predicates, type refinements, and relationships between
bindings, live in a single **`where` guard** after the pattern, not inside the positions. The
arm matches iff the pattern matches **and** the guard holds:

```
[1, 2, _, x] where x is int && x > 10 => ...,
```

Here the positions are simple (two literals, a discard, a binding `x`), and the `where` guard
does the work: `x is int` is the type refinement (using `is`, which also **narrows** `x` to
`int` in the body), and `x > 10` is the value condition. This is the clean form of what would
otherwise be an unreadable per-position tangle:

```
[1, 2, _, x where int as x && x > 10]      // avoid: binding, type, and guard crammed into one position
[1, 2, _, x] where x is int && x > 10       // prefer: position binds; guard conditions
```

`where` is the **same operator as in constraints** (constraints spec): "restrict a value by a
predicate." A guard is an ordinary boolean expression over the arm's bindings, so it composes
freely:

- **Type refinement:** `where x is int` (narrows `x` to `int` in the body).
- **Value condition:** `where x > 10`.
- **Cross-binding:** `where x > y` (references several bindings from the pattern).
- **Any pure predicate:** `where isPrime(x)`.

One guard per arm, evaluated after the pattern binds, seeing all of the pattern's bindings.

---

## 4. Table and shape patterns

A table or list pattern matches a value's **shape**: which keys or elements it has, and whether
their values match the sub-patterns. This is a **runtime structural test**, not a static shape
type (Luna has no shape types, tables spec); a shape pattern is a predicate that inspects the
value at runtime. It reuses destructuring syntax (destructuring spec), with sub-patterns where
destructuring has bindings.

```
match (msg) {
  ['type' => "move", 'x' => x, 'y' => y] => moveTo(x, y),   // tag matched, x and y bound
  ['type' => "stop"]                     => stop(),
  _                                      => ignore(),
}
```

The rules follow destructuring exactly, so shape matching and destructuring stay consistent:

- **Keyed (table) patterns are partial.** A keyed pattern lists the keys it cares about; **extra
  keys are ignored**. `['type' => "move"]` matches any table whose `'type'` is `"move"`,
  whatever else it holds. This is destructuring's "unmentioned keys ignored" rule and is what
  makes tag-dispatch on messages natural.
- **List (positional) patterns are exact.** A positional pattern matches by integer position and
  is **exact-length** by default (destructuring §1.1): `[1, 2, _, 4]` matches a four-element list
  whose positions are `1`, `2`, anything, `4`. `_` discards a position; `...rest` captures the
  tail; `..._` discards it. So lists keep destructuring's exact-length discipline.
- **A named key is present-required.** In a match, mentioning a key means the value **must have
  it**: `['x' => x]` matches only if `'x'` is present (and binds it), and does **not** match if
  `'x'` is absent. This differs from plain destructuring, where an absent key binds `undefined`,
  because a match asks "does this shape fit," and a missing key means it does not. (Destructuring
  extracts what is there; a match tests that it is there.)
- **`_` in a value position discards** without requiring a bind: `['x' => _]` matches iff `'x'`
  is present, ignoring its value; `[1, _, 3]` matches position 1 as anything.
- **Patterns nest.** A sub-pattern may itself be a table or list pattern: `['user' => ['name' =>
  n]]` matches a table whose `'user'` is a table with a `'name'`, binding `n`. Recursion is
  unbounded; the cost is runtime structural inspection, which the programmer should weigh for
  hot paths.

So a shape pattern is destructuring where positions may be **literals** (matched) as well as
**names** (bound), `@T` (type-tested), or nested patterns, with keyed-partial and list-exact
semantics inherited unchanged.

### 4.1 A nested pattern that fails just falls to the next arm

A nested sub-pattern that does not match makes the **whole arm** not match, and control falls to
the next arm, exactly as any pattern failure does. There is no partial match and no special
nested-exhaustiveness rule: `['user' => ['name' => n]]` simply does not match a value whose
`'user'` lacks a `'name'`, and the next arm is tried. So nesting adds depth to a pattern without
adding any new fall-through behavior; the whole-match `_` rule (§9.1) is the only exhaustiveness
rule, at any nesting depth.

### 4.2 As-patterns are intentionally omitted

Binding a sub-value **and** matching its shape in one pattern (an "as-pattern," e.g. binding
`u` to a whole user table while also matching its `name`) is **not** provided. It is uncommon,
and the need is met with no new syntax: bind the whole value and access its parts in the body.

```
match (msg) {
  ['user' => u] => useWhole(u, u['name']),    // bind the whole user; read parts in the body
}
```

This is a deliberate omission, not a gap: adding an as-pattern would overload `as` (a checked-
narrowing operator, not a binder) for a rare case the body already handles. It is additive if
demand ever appears.

---

## 5. Range and alternation patterns

Two further pattern forms cover "in a range" and "one of these," reusing existing syntax rather
than adding a new construct:

- **Range pattern** `lo..hi`: matches iff the scrutinee is within the range (reusing the range
  syntax that slicing uses, bytes spec). Natural for classifying numbers:

  ```
  match (status) {
    200..299 => "ok",
    400..499 => "client error",
    500..599 => "server error",
    _        => "other",
  }
  ```

- **Alternation** `a | b | c`: matches iff **any** alternative matches. `|` reads as "or" and
  composes with every pattern kind, literals, type patterns, and ranges:

  ```
  match (x) {
    1 | 2 | 3          => "small",
    "a" | "b" | "c"    => "letter",
    @int | @double     => "number",
    1..9 | 90..99      => "edge",
    _                  => "other",
  }
  ```

  Alternation is a **pattern**, not a set value: it means "any of these sub-patterns match," so
  no set-literal type or comma-separated value list is involved (a comma would collide with the
  list-pattern separator). If alternatives **bind**, they must bind the **same names** so the
  body's scope is well-defined: `@int n | @double n` is valid (both bind `n`), while `@int n |
  @string s` (inconsistent bindings) is an error. A name bound in several alternatives has the
  **union** of its per-alternative types: in `@int n | @double n`, `n` is `int | double` in the
  guard and body.

---

## 6. Open-ended match: a guard chain

`match { ... }` with **no scrutinee** is a chain of **boolean guards**, evaluated in order, the
first true one winning. It is a `cond` expression (first-true-guard-wins), for when the arms are
independent conditions rather than comparisons against one value:

```
let tier = match {
  l <= 0   => 0,
  l <= 10  => 1,
  l <= 100 => 2,
  _        => 3,
};
```

Each arm is a boolean expression, not a pattern (there is no scrutinee to match against). This
is the ergonomic replacement for an if / else-if expression chain. The distinction is exactly
the presence of a scrutinee: **`match (x) { ... }` has patterns; `match { ... }` has guards.**

---

## 7. Equality for literals is the total order

A literal pattern matches by the **total order** (double spec §2.2), not IEEE `==`. For every
type except `double` these coincide. For `double`, the total order makes literal patterns
well-behaved where `==` is not:

- **`match (x) { NaN => ... }` matches a NaN scrutinee** (the total order makes NaN equal to
  NaN, unlike `==`, where the arm would be dead code).
- **`match (z) { 0.0 => ... }` matches both `-0.0` and `+0.0`** (the total order merges the
  zeros).

So pattern matching is reflexive and total: matching a value against its own literal always
works. Explicit `==` inside a guard still has IEEE semantics when you want them (`where x ==
someNan` never holds, correctly). So you get both: total-order matching by literal, IEEE `==` by
explicit guard.

---

## 8. No fall-through: first match wins

Arms **never** fall through. Exactly **one** arm body runs, the first whose pattern matches (and
whose guard holds); no later arm is considered, and there is no implicit continuation to the
next arm. A scrutinee that could match several arms runs only the first. (C-style fall-through
is a footgun and is not present.)

---

## 9. Non-exhaustive match yields `undefined`

A match **without a `_` arm** (§9.1) **yields `undefined`** on fall-through, rather than
panicking, when no arm matches:

```
let v = match (code) {
  10 => "ten",
  20 => "twenty",
};
// v : "ten" | "twenty" as string, plus undefined; undefined if code is neither 10 nor 20
```

The partiality is surfaced **in the type**: a non-exhaustive match has result type `(union of
arm bodies) | undefined`, so the caller sees it may not match and handles it, usually by
coalescing (`match (...) { ... } ?? fallback`). Yielding `undefined` rather than panicking makes
match convenient for quick lookups and chaining: a miss is an ordinary absent value to coalesce
or propagate (coalescing spec), not an error to catch.

### 9.1 Exhaustiveness is exactly "has a `_`"

The rule is deliberately simple: **a match is exhaustive iff it has a `_` arm.** With a `_`, the
result type has no `undefined`; without a `_`, it has `| undefined`. That is the whole rule.

The compiler does **no** coverage analysis, not over bools, not over closed unions, not over
integer values or ranges, not over guards. It does not try to prove that `true`/`false` arms
cover a bool, or that a set of type arms covers a union. The single question is "is there a `_`
arm."

This is chosen for predictability and cost. A reader knows exactly when a match can yield
`undefined`, when it lacks a `_`, with no need to reason about whether the compiler *happened* to
prove coverage. And claiming exhaustiveness is one explicit line: add `_` (which may itself
panic or return a fallback if you believe it truly unreachable). Stating totality with `_` in the
source is clearer than having the compiler infer it silently, and it is robust to refactors that
widen the scrutinee. So: **`_` present means exhaustive; `_` absent means `| undefined`.**

---

## 10. Result type: the union of the arms

The result type of a match is the **union of its arm-body types** (plus `undefined` if
non-exhaustive, §8):

```
match (x) { 1 => "a", 2 => "b", _ => "c" }     // string
match (x) { 1 => 10, 2 => "b", _ => true }      // int | string | bool
```

If the union is not statically decidable, the result type is **`any`** (the honest fallback for
an undecidable union). The programmer can always narrow a match result with `as` when they know
what it must be (`as` spec), so an `any` result is recoverable, not a dead end: conservative by
default, precise on request.

---

## 11. Capture is like a lambda

A match expression captures the free variables it references the same way a function does
(functions spec §2): **by value automatically**, and **by reference through `use`** for nocopy
values or deliberate referential capture:

```
let v = match use (someRef) {
  _ where check(someRef) => ...,
};
```

So match introduces no new capture rule; it captures exactly as a lambda body would, `use` for
references, with the nocopy handling that implies.

---

## 12. Open questions

Match itself is essentially settled; its one genuine dependency is on a construct it does not
own:

- **Range semantics come from the range spec, which is not yet written.** Whether `1..10` is
  inclusive or half-open, whether a second form (`..=`) exists for the other, and whether ranges
  apply beyond numbers (to strings, for example) are properties of the `..` syntax itself, used
  already in slicing (bytes spec) and here in range patterns (§5), but never defined. Range
  patterns inherit whatever the range spec decides; this is a language-wide gap, not a match
  question, and is the next spec to write.
