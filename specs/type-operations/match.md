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
is one of a small set of kinds. A position matches a value, binds a name, tests a type, or
recurses; **a type test and a binding compose in one position** (`n: int`), which is the one place
two jobs are done at once, and everything else, value conditions and relations between bindings,
lives in the guard (§3):

- **Literal** (`10`, `-5`, `"move"`, `true`, `false`, `null`, `undefined`, `nan`, `inf`, `-inf`):
  matches iff the value
  **equals** it, by the **total order** (§7), not IEEE `==`, so it is well-behaved for every type
  (a `nan` literal matches a nan, `0.0` matches both zeros; double spec §2.2). A **numeric**
  literal admits a leading **`-`** (R183): number literals are non-negative and the sign is an
  operator everywhere else (numeric-operators §1.1), but a pattern admits **no operators**, so
  pattern position **folds** `MINUS` + numeric literal into one signed-literal pattern with no
  ambiguity and one token of lookahead — the same fold keywords §4 already blesses for `-inf`.
  The sign composes where a number does: literals (`-5`, `-1.5`), range endpoints (`-10..-1`,
  §5), alternation (`-1 | 1`). Three edges, each inherited rather than new: the most-negative
  `int` literal form is recognized exactly as at the expression boundary (numeric-operators
  §1.1); `-0.0` as a pattern matches both zeros, because the total order merges them (§7) — the
  sign adds no new distinction; and there is **no `-nan`** (a nan carries no meaningful sign at
  the language level). Every one of these is
  a keyword or a literal token (lexer §3, §4), never an identifier, which is exactly what keeps them
  out of the binding rule below. An `undefined` arm **compares against** the absence sentinel; it
  does not conjure one, so it is the same permitted use as `tab['k'] == undefined` (undefined §2.1) and
  no exception to "`undefined` is language-produced" (undefined spec). Since `undefined` and `null`
  are types with a single value each, their literal arms and the type tests `_: undefined` / `_: null`
  coincide.
- **Wildcard** `_` (wildcard spec §4): matches anything, **binds nothing** (discard).
- **Binding** `name`: matches anything, and **binds** the value to `name` **at the type the value
  already has** in that position, the scrutinee's static type at top level, or the source element's
  type inside a shape pattern, exactly as destructuring types its bindings (destructuring §5). A
  bare binder therefore **inherits**; it never widens, never narrows, and never checks. The binding
  is scoped to **that arm only**: it is visible in the arm's guard (§3) and body, and **nowhere
  else**, not in sibling arms, not after the match. This is the only sound scoping, since exactly
  one arm runs (§8) and match is an expression whose information leaves through its result value,
  not through leaked names. To use a bound value outside the match, return it from the arm body
  (the value escapes as the match result; the name does not).
- **Typed binding** `name: T`: the value is tested against the **type expression** `T`, written
  exactly as in any type position, bare `int`, a union `int | string`, a constraint `byte`, an
  application refinement `@stringBuilder` or `@P & @Q`, and iff the value **is** of that type (the
  single meaning of `is`, is spec §2, running whichever check the type's shape dictates, interval,
  union decomposition, applied-set, constraint predicate, or signature table, as the type
  dictates) the arm matches and `name` is bound **at `T`**, narrowed. So a bare binder *inherits* a
  type and a typed binder *tests and supplies* one; the two are different jobs and both are needed
  (§2.2). It composes with a guard: `n: int where n > 10`.
- **Type test** `_: T`: the same test, binding nothing. `_` is the binding-position blank (wildcard
  §5, `fn (_, index)`), and a binding position takes an annotation, so `_: T` is that blank with its
  type, exactly as `fn (_: int, x)` is. `_: any` is `_`.
- **Table / list pattern** (`['k' => sub]`, `[sub, sub]`): matches the value's **shape**
  structurally, recursing into sub-patterns (§4).
- **Enum variant** `{tag}` / `{tag pattern}`: matches iff the scrutinee is that variant, the
  payload sub-pattern (if any) recursing into the carried value (`{circle ['radius' => r]}`,
  enum §4). Grammar row in §2.1; the discrimination idiom of the enum spec.
- **Range** `lo..hi` and **alternation** `a | b` (§5): membership in a range, and "any of these."

Pattern-type position **is type position**, so `@` there means what it means in every type
position, the application refinement, and **`_: @int` is a compile error** by the existing rule
(`@X` on a non-protocol type, protocols spec), never a type test. (`@int` as an *expression* is
`typeof(int)`, the type `type`, a value-position reading that has nothing to do with patterns;
type §1.1 fixes the two roles by position.) Dispatching on a value's type is done by matching **the
value** (`match (x) { n: int => ..., s: string => ... }`); matching a **type value** itself is the
rare reflective case and uses a guard (`t where t == int`, one typeid compare, equality §3).

A position does not carry conditions inline; those go in the `where` guard (§3). This is what
keeps a position readable: it matches a literal, binds a name, tests a type (binding or not),
discards, or recurses.

### 2.1 The pattern grammar

```
pattern := "_"                        // wildcard: match, bind nothing
         | "_" ":" type               // type test, bind nothing
         | IDENT                      // binding; inherits the value's type
         | IDENT ":" type             // type test; binds at `type`
         | literal                    // ("-")? (INT | DOUBLE | "inf") | STRING | true false null undefined nan  (signed fold, R183)
         | range                      // §5
         | "[" ... "]"                // table / list shape, sub-patterns recurse (§4)
         | "{" tag pattern? "}"       // enum variant (enum §4)
         | pattern ("|" pattern)+     // alternation (§5)
         | "(" pattern ")"

arm     := pattern ("where" expr)? "=>" body
```

Three properties, and each is the reason for the shape:

- **A pattern's kind is decided by syntax alone**, with one token of lookahead: at an `IDENT` or a
  `_`, peek for `:`. The type universe is **never consulted** to decide what a position *means*. So
  a bare `list`, `table`, `path`, or `count` is a binding wherever it appears, and no declaration
  or import elsewhere in the program can silently turn a binding arm into a type test. This is the
  whole reason a type is written after `:` rather than bare: builtin type names are ordinary
  shadowable identifiers (keywords §5, modules §7), so a bare type pattern would have made every
  arm's meaning depend on what happened to be in scope.
- **A type appears in a pattern only after `:`.** Consequently `|` is the **union** operator when
  the parser is inside a type and the **alternation** separator everywhere else, and the two can
  never meet. `n: int | string` is a union; `1 | 2 | 3` is an alternation. To use a typed pattern as
  an alternative, parenthesize it: `(_: string) | 5`. This is rarely wanted, because alternatives
  must bind the same names (§5) and a union already says what an alternation of types would say.
- **The type after `:` ends** at the first token that cannot extend a type expression at bracket
  depth 0: `where`, `=>`, `,`, `]`, `}`, `)`. None of these can continue a type, so no lookahead is
  needed and `where` is unambiguously the guard. Note `{` is **not** in that set and does not need
  to be: `{` extends a type directly after `enum`, and the one position where a type would otherwise
  butt against an opening brace, a `catch` clause head, is parenthesized (errors §8.3), so the `)`
  closes the type first.

Two positions take a **restricted** pattern, because they have nothing to fall through to:

- a `catch` clause takes a **parenthesized binder pattern**, `catch (_)`, `catch (_: T)`,
  `catch (name)`, `catch (name: T)`, and nothing else (errors §8.3);
- a `constraint` body takes a **typed binder** (`name: T`) and nothing else (constraints §1). It is
  a declaration of a base type, not a probe, so the shape and literal forms are not available there
  and Luna gains no shape types by the back door (§4).

A constraint body is therefore an arm's `pattern where guard` with the pattern narrowed to one
typed binder and the body dropped, which is why the two share the `:` and the `where`. They differ
in exactly one place: a constraint admits **several** `where` clauses, conjoined (constraints §1),
because a constraint *reports which clause failed*; an arm takes **one**, because a failing arm has
nothing to report, it simply falls through, and `&&` already says the rest.

### 2.2 A typed binding is `is` for the test, `as` for the type

`name: T` is exactly two things, and neither is new machinery:

1. the test `value is T` (is §2), **total**, never panics, never transforms; and
2. on success, a **fresh binding** of static type `T` (as §7), scoped to the arm.

The compiler emits **one** check: the narrowing cannot fail once the test has passed, so the cost
of a typed binding is exactly the cost of `is` for that kind of type (type §7.1), and the bind is
free.

The test is **`is`, not `as`**. `as` panics on mismatch (as §1); a pattern must fall through to the
next arm. `as` is how you commit to a type you believe; a pattern is how you ask.

Two consequences follow from `as`'s existing rules rather than from any new one:

- **A disjoint type is a compile error, not a dead arm.** `match (x: int | string) { b: bool => ... }`
  is rejected where `x as bool` would be (as §5): `bool` shares no values with `int | string`, so
  the arm could never run. A dead arm is a bug, and it is caught.
- **A supertype is irrefutable, and the check is elided.** `n: table` on a `@person` scrutinee is a
  free widening (as §5), no runtime check, and it **discards** the protocol, exactly as
  `p as table` does. `n: any` is legal, lossy, and pointless, which is precisely why the bare binder
  exists: write `n` to keep what the value already has.

`:` in a **pattern** tests; `:` in a **declaration** asserts, compile-checked, and never checks at
runtime (as §4). Nothing is hidden by the reuse, because in a pattern the arm's selection **is** the
check: no value is ever seated in an `x: T` that is not a `T`, whether the mismatch is caught at
compile time (a declaration) or by falling to the next arm (a pattern). An annotation never lies in
either position. Position decides which reading applies, as it already does for `@`, `&`, `!`,
`error`, and `comptype` (type §1.1, associativity §2).

### 2.3 Deciding a function's signature discharges its per-call checks

The `is`-not-`as` rule of §2.2 has one consequence sharp enough to name, and it is where `match` and
`as` visibly part. `as` on a **function type** is *optimistic*: it may assert a signature that is not
a subtype of the real one, and it pays for the claim on **every call**, checking arguments against the
callee's real parameters and the result against the caller's claim (functions §3.2). A pattern's test
is `is`, which is total, so it never claims, it **decides**.

When `g: fn (int): string` matches, `real <: claim` holds, and every per-call **signature** check folds
away: the argument is statically within the claimed parameter, which the real parameter accepts by
contravariance; the returned value lies within the real result, which lies within the claim by
covariance; the real function has no required parameter beyond the claimed arity; errorability agrees;
`&` positions are identical. So a signature-bound function is called with **no argument check, no
return check, no `arityError`, and no laundering check**, where an `as`-narrowed one carries all four.
`match` is the fast door as well as the total one, and this is the same `is`/`as` relationship that
holds everywhere else in the language.

Two boundaries keep the claim honest.

- **A bare `g: fn` decides nothing about the signature**, because there is none in the type to decide
  (functions §3.1). Calls through it keep all four checks, exactly as an `as fn (A): R` would. Only a
  **signature** pattern discharges them. What the binder buys is precisely what the type carries.
- **No pattern discharges the capability check.** A function's requirement set rides the **value**, not
  the typeid (functions §3, capabilities §3.1), so `is` never inspected it and no narrowing can reach
  it. A call through any `fn`-typed slot still verifies *requirement set ⊆ the executing frame's
  granted set*, one bitmask compare, panicking on shortfall. Narrowing a signature is a claim about
  **data**; authority is not data, and it is discharged where it is exercised, never where it is typed.

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
to `int`, put the type in the **position** with a typed binding (§2.2), which binds a fresh
`x` of type `int`:

```
[1, 2, _, x: int] where x > 10 => ...,        // x: int binds x AS int; the guard is a pure value condition
[1, 2, _, x] where x is int && x > 10 => ..., // also valid: x is the union type; `is int` is a boolean guard
```

The first form is idiomatic when you want `x` typed as `int` in the body: the **typed binding**
narrows (by binding a new name of that type), which is the only way narrowing ever happens (as spec
§7). In the second form `x` keeps its inherited type in the body and `is int` merely gates the arm;
if you then need it as `int`, bind it narrowed with `x: int` in the position instead. The two
spellings of the test never collide, because a **position** carries `:` and a **guard** carries
`is`: `x: int where x is int` is visibly the same test done twice.

`where` is the **same operator as in constraints** (constraints spec): "restrict a value by a
predicate." A guard is an ordinary boolean expression over the arm's bindings, so it composes
freely:

- **Type test:** `where x is int` (a boolean test that gates the arm; to bind `x` narrowed to
  `int`, use the `x: int` typed binding in the position, as spec §7).
- **Value condition:** `where x > 10`.
- **Cross-binding:** `where x > y` (references several bindings from the pattern).
- **Any pure predicate:** `where isPrime(x)`.
- **Comparison against a named constant:** `where c == exitCode.success`. A bare name in a
  *position* is always a fresh binding (§2.1), never a comparison against a constant of that name,
  so value dispatch over named constants is spelled in the guard, where it is visible.

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
**names** (bound), **typed bindings** (`'x' => x: int`) and **type tests** (`'x' => _: int`),
or nested patterns, with keyed-partial and list-exact semantics inherited unchanged. A bare name
means the same thing in both, a binding that inherits the source element's type (§2,
destructuring §5), which is what makes a match pattern a strict superset of a destructuring
pattern rather than a homograph of one (§4.3). (The nesting flows back: destructuring
statements reuse this nested grammar restricted to binding positions — no literals, no
typed binders, since a failed test needs a next arm — with statement-side failure
semantics per mode, destructuring §3.1, R147.)

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
narrowing operator, **not a binder** anywhere in the language, since constraints stopped spelling
their binder `int as i`, constraints §1) for a rare case the body already handles. It is additive
if demand ever appears.

### 4.3 Match patterns vs destructuring: same syntax, one deliberate difference

Match shape patterns and destructuring (destructuring spec) share syntax and mostly share
behavior, because a match pattern *is* a destructuring pattern with literals and type tests
added. They agree on:

- **Keyed is partial** in both (unmentioned keys ignored).
- **Positional is exact** in both (`_` skips, `...rest` captures, `..._` discards).
- **`_` means discard** in both (match nothing, bind nothing).
- **A bare name binds** in both, inheriting the source element's type (destructuring §5). This is
  the property that makes the superset claim literally true, and it is why a type in a pattern is
  written after `:` rather than bare: were a bare `x` a type test in a match, `['x' => x]` would
  mean opposite things in `let` and in `match` (§2.1).

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
  inclusive; `lo..<hi` excludes the top; endpoints admit the signed-literal fold
  (`-10..-1`, §2, R183). Natural for classifying numbers:

  ```
  match (status) {
    200..299 => "ok",
    400..499 => "client error",
    500..599 => "server error",
    _        => "other",
  }
  ```

- **Alternation** `a | b | c`: matches iff **any** alternative matches. `|` reads as "or" and
  composes with every *untyped* pattern kind, literals, ranges, wildcards, and shapes:

  ```
  match (x) {
    1 | 2 | 3          => "small",
    "a" | "b" | "c"    => "letter",
    _: int | double    => "number",
    1..9 | 90..99      => "edge",
    _                  => "other",
  }
  ```

  Alternation is a **pattern**, not a set value: it means "any of these sub-patterns match," so
  no set-literal type or comma-separated value list is involved (a comma would collide with the
  list-pattern separator). If alternatives **bind**, they must bind the **same names** so the
  body's scope is well-defined: `1 | 2` is valid (neither binds), while `['a' => x] | ['b' => y]`
  (inconsistent bindings) is an error.

  Note the `"number"` arm: `_: int | double` is **one** type test against the union `int | double`,
  not an alternation of two type tests. Inside a type, which is to say after a `:`, `|` is the union
  operator; outside one it is the alternation separator (§2.1). The two readings can never meet, so
  no parenthesization rule is needed to keep them apart, and "an `int` or a `double`, bound"
  is simply `n: int | double`, with `n` typed `int | double` in the guard and body. To use a typed
  pattern as an *alternative* rather than a union member, parenthesize it, `(_: string) | 5`; this
  is rare, and it is the only place the two `|`s come near each other.

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

- **`match (x) { nan => ... }` matches a nan scrutinee** (the total order makes nan equal to
  nan, unlike `==`, where the arm would be dead code).
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
(no arm matches) **panics** (a `panic`) instead of yielding `undefined`.

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
  consistent with the rest of the language. (The panic is a `panic`, so `match!` does not by
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
non-exhaustive, §9):

```
match (x) { 1 => "a", 2 => "b", _ => "c" }     // string
match (x) { 1 => 10, 2 => "b", _ => true }      // int | string | bool
```

If the union is not statically decidable, the result type is **`any`** (the honest fallback for
an undecidable union). The programmer can always narrow a match result with `as` when they know
what it must be (`as` spec), so an `any` result is recoverable, not a dead end: conservative by
default, precise on request.

---

## 11. A match is inline: no capture, no `use`, the enclosing frame (R184)

A match expression is evaluated **immediately, inline, in the enclosing frame**. It is not a
deferred body, so it **captures nothing**: guards and arm bodies read the bindings around
them **live**, exactly as an `if` branch or a `foreach` body does — a mutation between guard
evaluations (through a call that writes back, variables §5.1) is visible to later guards,
because nothing was snapshotted. And it takes no `use` clause: **authority is the enclosing
frame's grant** — an arm that calls a capability-requiring function is covered by the
enclosing function's `use`, precisely as the same call in an `if` branch would be
(capabilities §5). `match use (...)` is **retired** (R184): no non-callable construct owns a
grant frame, and keywords §3's `use` inventory — a `fn`/`test` header, and call-site
delegation (R112) — is complete without it.

(The earlier draft gave match a lambda's capture, deep-`const` snapshots plus a `use` clause
— a fossil of treating the arms as a deferred closure. The distinction that matters survives
unchanged: a **`fn` literal written inside an arm body** is an ordinary closure and captures
by snapshot as every closure does, functions §2 — it is the *function* that captures, never
the match. Streams need no special rule either: scrutinizing a stream consumes it by the
ordinary single-pass rules, stream §2, no capture involved.)

---
