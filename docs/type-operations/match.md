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
does the work: `x is int` is a **boolean type test** and `x > 10` is a value condition. A `where`
guard is a pure boolean expression; it conditions whether the arm matches, but it does **not**
narrow a binding's type (Luna does no flow-narrowing, as spec §7). To bind `x` **already narrowed**
to `int`, put the type in the **position** with the `@T name` type pattern, which binds a fresh
`x : int`:

```
[1, 2, _, @int x] where x > 10 => ...,      // @int x binds x AS int; guard is a pure value condition
[1, 2, _, x] where x is int && x > 10 => ..., // also valid: x is the union type; `is int` is a boolean guard
```

The first form is idiomatic when you want `x` typed as `int` in the body: the **type pattern**
narrows (by binding a new name of that type), which is the only way narrowing ever happens (as spec
§7). In the second form `x` keeps its matched type in the body and `is int` merely gates the arm;
if you then need it as `int`, bind it narrowed with `@int x` in the position instead.

`where` is the **same operator as in constraints** (constraints spec): "restrict a value by a
predicate." A guard is an ordinary boolean expression over the arm's bindings, so it composes
freely:

- **Type test:** `where x is int` (a boolean test that gates the arm; to bind `x` narrowed to
  `int`, use the `@int x` type pattern in the position, as spec §7).
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

### 4.3 Match patterns vs destructuring: same syntax, one deliberate difference

Match shape patterns and destructuring (destructuring spec) share syntax and mostly share
behavior, because a match pattern *is* a destructuring pattern with literals and type tests
added. They agree on:

- **Keyed is partial** in both (unmentioned keys ignored).
- **Positional is exact** in both (`_` skips, `...rest` captures, `..._` discards).
- **`_` means discard** in both (match nothing, bind nothing).

They differ in **exactly one place**, and it is deliberate: **an absent named key.**

| | Absent named key `['x' => x]` where `'x'` is missing |
|-|-|
| **Destructuring** (`let [...] = tab`) | binds `x = undefined`, and **succeeds** |
| **Match pattern** (`match { [...] => }`) | the pattern **does not match**; falls to the next arm |

The reason is their different purposes: **destructuring extracts what is there** (a missing key
yields the absence value, `undefined`), while a **match pattern tests that the shape fits** (a
missing key means the shape does not fit, so the arm does not apply). Both are correct for their
context.

The one hazard to be aware of: because the syntax is shared, a reader who knows destructuring
may expect `['x' => x]` in a match to bind `undefined` for a missing `'x'`, when it actually
skips the arm. So the shared syntax has **context-dependent absent-key behavior**, extract-with-
undefined in a binding, require-presence in a match. This is noted in both specs.

---

## 5. Range and alternation patterns

Two further pattern forms cover "in a range" and "one of these," reusing existing syntax rather
than adding a new construct:

- **Range pattern** `lo..hi`: matches iff the scrutinee is within the **inclusive** range
  (range spec), a membership test (endpoints), not a stream. `200..299` matches 200 through 299
  inclusive; `lo..<hi` excludes the top. Natural for classifying numbers:

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

### 9.2 `match!`: strict match, panic on fall-through

`match!` is the **strict** form: identical to `match` in every way except that a fall-through
(no arm matches) **panics** (a `Panic`) instead of yielding `undefined`.

```
let v = match! (state) {       // v : the union of arm bodies, with NO | undefined
  "idle"    => 0,
  "running" => 1,
  "done"    => 2,
};                             // panics if state is none of these
```

- **Result type has no `undefined`.** Because fall-through panics rather than producing a value,
  `match!`'s result type is exactly the union of its arm bodies (§10), with no `| undefined`
  arm. So `match!` also gives the cleaner result type when you believe the match is total.
- **No new compiler analysis.** `match!` needs **no** coverage proving; it is the same "is there
  a `_`" machinery (§9.1) with the fall-through case emitting a panic instead of `undefined`.
  This keeps it consistent with the no-coverage-analysis rule, the strictness is a **runtime**
  guarantee (a loud failure on an unhandled case), not a compile-time exhaustiveness proof.
- **The `!` is the ordinary fail-loud `!`.** As elsewhere, `!` marks the form that fails loudly
  (functions §4, errorability). `match!` reads as "match, and fail loudly if nothing matches,"
  consistent with the rest of the language. (The panic is a `Panic`, so `match!` does not by
  itself make the enclosing function `fn!`; a fall-through is a programming error, like any
  panic.)

**When to use which.** Plain `match` and strict `match!` fit different situations, and neither
dominates, so users do not simply always reach for one:

- **`match!` fits a closed, known case set:** a state enum, an error hierarchy, a fixed set of
  message tags, where a missing case is a **bug** and should fail loudly (and, on refactor, where
  a newly added case *should* surface as a panic at the unhandled site rather than silently
  returning `undefined`).
- **Plain `match` fits an open or partial case set:** matching a few interesting values out of an
  unbounded space (a handful of ints), tag dispatch where "anything else, ignore" is the honest
  intent, or quick lookups where a miss is a normal absent value to coalesce. Here `match!` would
  panic on cases you never meant to handle, so it is the wrong choice, which is why users do not
  default to strict everywhere.

So `match` is lenient (miss yields `undefined`, softly handled) and `match!` is strict (miss
panics, loudly caught), chosen per use by whether an unhandled case is normal or a bug.

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
(functions spec §2): as **implicit deep-`const` snapshots**, read-only, taken when the match
is evaluated. `use` on a match means what `use` means everywhere, **capabilities only**
(capabilities §3.1), for the case where a guard or arm calls a capability-requiring function
by name:

```
let v = match use (io) {
  _ where confirm(io) => ...,     // a guard that reaches io must hold it, like any body
};
```

So match introduces no new capture rule; it captures exactly as a lambda body would: `const`
snapshots for data, `use` for authority, and nothing else. A stream is never captured by a
match either, scrutinize it as the subject or pass it through a binding pattern, per
functions §2.3.

---

## 12. Open questions

Match itself is settled. Range patterns (§5) now follow the **range spec**: `..` is inclusive,
`..<` excludes the top, and a range pattern is a membership test over the inclusive (or
top-exclusive) bounds. No open match-specific questions remain.
