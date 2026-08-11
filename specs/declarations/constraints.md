# Constraints

A **constraint** refines a type by attaching a predicate: a constrained type admits exactly
the base-type values that satisfy the predicate. `byte` is `int` restricted to `0..255`; a
port is `int` restricted to `1..65535`; `list` is `table` restricted to contiguous keys from
zero. A constraint is, in one phrase, **a pure predicate function pushed into a type
definition**, and that framing determines everything: it is checked at runtime (you call the
predicate), it composes by execution (you run predicates in order), and it never requires a
solver.

Luna deliberately takes the **runtime-checked** form of refinement types, not the
statically-verified form. A value's membership in a constrained type is checked when the value
*enters* the type (construction, assignment, `as`), and a failure is a `typeError` (panic,
errors §9), exactly like any other narrowing (`as` spec). There is no theorem prover, no
flow-sensitive proof obligation, and type-checking stays decidable. The price is a runtime
check at the boundary (cheap, one predicate call) and no reasoning about predicate implication
(§6).

**The frame-independence principle (R79).** A constraint's predicate must be a **function of
the value alone**: frame state, capability grants, task identity, and every other ambient
fact are illegal inputs. This is not style, it is what the fact model *requires*: a checked
constraint becomes a fact riding the value (§7.1), and facts travel across frames, so a
predicate whose truth varies by frame would mint facts that stop being true in transit,
authority laundered into a type. This is why a `can` operator inside `where` (proposed and
rejected, R79) cannot exist: "the current frame may call f(s)" is the paradigm ambient fact,
and gating secrets by capability is done soundly on the secret's value at the effect site
instead (secret §3.1, §5). The frame-inspecting `can` *expression* is likewise **scrapped
entirely** (R80): no form of `can` exists anywhere in the language, in or out of constraint
position, and grants stay invisible to programs, checkable only by exercising them.

---

## 1. Declaring a constraint

A constraint is declared with the **`constraint` declaration form**, bound to a `const`:

```luna
const byte = constraint i: int where i >= 0 && i <= 255;
const port = constraint i: int where i >= 1 && i <= 65535;
```

(`port`, this file's running example since R9, is exported for real by `std.net` §4
as of R143.)

- **`name: base`** names the base type and binds the value under test to `name`. It is the
  **typed binder** of the pattern grammar (match §2.1), the same form as a parameter (`fn (i: int)`)
  and a `let`, because that is what it is: a value of the base type, given a name.
- **`where <predicate>`** is the predicate: a pure boolean expression over `name`. A value of
  the base type inhabits the constrained type iff the predicate holds.

A constraint body is therefore exactly a match arm's `pattern where guard`, with the pattern
narrowed to one typed binder and the body dropped, which is why the two share both the `:` and the
`where`. The narrowing is deliberate: shape, literal, range, and alternation patterns are **not**
available in a constraint body, so Luna gains no shape types by the back door (tables spec).

The form is **braceless** (R137), and the parse needs no delimiter at either end: `where` is
never part of a type — which is what lets it terminate one (keywords §2) — so the type ends and
the predicate begins unambiguously; the declaration's own `;` ends the predicate; and a further
`where`, a reserved word that can never appear inside an expression, unambiguously starts the
next conjunct (below). The braces the form once carried held no list (every other brace-former
holds one: proto members, enum variants, error fields), enforced nothing (§2's purity belongs
to the declaration form, not to braces), and obscured the arm-identity this section claims —
retired. A constraint now looks like the arm it is.

A `constraint` literal is legal in exactly **one position**: as the initializer of a `const`
declaration — the `proto` / `capability` precedent (R126, R137). The binding's identifier **is
the constraint's name**: what `typeName` reports (`"byte"`, `"json"`, `"path"`) and what a
failing check cites; aliases carry the type, never rename it. One declaration, one `typeid`
(§9.1), one name.

Earlier drafts spelled the binder `int as i` (R27 and before). That made `as` a binder, which it is
nowhere else in the language, and inverted the type-on-the-right order every other binding form
uses; it is retired (R87). `as` is now purely the checked-narrowing operator (as spec) and the
import alias (modules §8).

The form **produces a distinct type**. `byte` is a type usable in annotations (`let x: byte`),
and, because of how types are stored (a constrained type has its own type identity), `int` and
`byte` (int-plus-constraint) are **separate types**, not the same type with a flag. When the
introspection API for types is fleshed out, that distinction is visible there.

Multiple `where` clauses are allowed and run in order as a **conjunction** (all must hold):

```luna
const byte = constraint i: int where i >= 0 where i <= 255;   // same as i >= 0 && i <= 255
```

The multi-clause form is sugar for `&&`; order affects only which failure is reported first. This
is the **one** place a constraint body diverges from a match arm, which takes a single guard
(match §3): a constraint *reports which clause failed*, so the clauses are worth keeping apart,
while a failing arm has nothing to report, it simply falls through.

### 1.1 Inline `where` in signatures: considered and rejected (R137)

Anonymous constraints in parameter position — `fn (n: int where n > 0)` — were considered
and **rejected**, three times over:

- **It breaks R79 structurally.** The inline predicate sits in a scope where sibling
  parameters are visible, so `fn (lo: int, hi: int where lo <= hi)` — a predicate that is
  not a function of the value alone — becomes *representable* and must be forbidden by
  rule. The named form makes it unrepresentable by construction, and safety by
  construction is the house rule; the frame-independence principle stays a property of
  the grammar, not a lint.
- **It forces a typeid identity crisis.** Constraints mint typeids (§9.1). Per-occurrence
  minting makes textually identical signatures *different types*, against the structural
  rule that same-shaped signatures are one type (functions §3.2, R130); interning instead
  demands alpha-equivalence over predicate ASTs (`n > 0` ≟ `0 < x`) — permanent machinery
  bought for a convenience.
- **It optimizes away the feature.** The name is the API: `typeName` says `"port"`, the
  panic names the check that failed, the third caller reuses the declaration instead of
  forking a slightly-different copy. An anonymous constraint's diagnostic is a source
  location.

What rejection costs is one reusable line — cheaper still now that the form is braceless.

---

## 2. Pure by construction

A constraint predicate **must be pure**: no side effects, no capability use (no `io`, no
`revealSecret`), deterministic on its input. This is not a rule the author must remember; it is
enforced by the form. A `constraint` declaration's predicate slot admits only a pure boolean expression over the
bound value, so an impure constraint is **unrepresentable**, the same "safe by construction"
move used elsewhere (a `capability` cannot have a body, a regex literal is compiled once).

Purity is essential to soundness. A constraint is checked when a value *enters* the type, and
for that check to mean something durable, the predicate must depend only on the value, not on
mutable external state that could change afterward. An impure constraint could pass at entry
and be false later, making the type incoherent. Pure predicates cannot go stale: if `x`
satisfied `byte` when it became a `byte`, it still does, because `x` and the predicate are
both fixed. This is also why a constraint is **its own declaration form** rather than an
arbitrary function in a type position: the form is what makes purity enforceable.

### 2.1 Purity is not totality: predicate panics and divergence (R178)

The form enforces purity; it does not — cannot — enforce **totality**, and the two gaps are
ruled honestly rather than papered over:

- **A predicate can panic ambiently.** `constraint i: int where i * i > 0` panics with
  `overflowError` at large `i` — inside the predicate, before any membership answer exists.
  Such a panic **propagates**, from every site that runs the predicate: entry (assignment,
  `as`, a `match` arm) and the `is` test alike. It is **never swallowed** into "check
  failed" / `false`: a suppressed panic would hide the bug and turn the answer into a silent
  wrong value. Consequence stated plainly: an entry check can therefore surface a panic
  that is *not* `typeError` (the `overflowError` above arrives first), and `is`'s
  "never panics" contract carries the matching precision (is §1) — no *check-specific*
  failure exists, but the predicate's own panics are ambient, as everywhere (errors §7).
- **A predicate can diverge.** Since predicates admit pure function calls (§11), unbounded
  recursion is expressible, and Luna has no termination checker. A diverging predicate makes
  its constraint unusable at every site equally — the bug is the constraint's, and no
  checking site adds or escapes the exposure.

Both are authoring bugs in the constraint, not holes in the checking model: the model's
guarantee is *where* checks run and that their verdicts are durable (purity), never that an
arbitrary pure computation is cheap, terminating, or panic-free.

---

## 3. A constraint carries its base type

A constraint names its base inside the form (`i: int`), so the constraint **is** a complete
refined type on its own. There is no need to restate the base when using it:

```luna
const byte = constraint i: int where i >= 0 && i <= 255;
let x: byte = 65 as byte;        // byte is the type directly; no `int where byte` restatement
```

Writing `int where byte` would repeat `int` redundantly, since `byte` already is "int plus
predicate." The constraint declaration binds one name that is the refined type.

---

## 4. One name: a type and a predicate

The name a constraint binds serves in **two syntactic positions**, unambiguously:

- **As a type** (annotations, `@`, `is`, `as`): `let x: byte`, `x is byte`, `x as byte`. Here
  `byte` denotes the refined type.
- **As a predicate** (inside another constraint's `where`): the enclosing constraint runs
  `byte`'s predicate as one of its conjuncts (§6).

These positions are syntactically distinct (type position versus `where` position), so one
name suffices; no second name for "the predicate of byte" is needed. This mirrors `list` being
both a type and a narrowing test (`is list`).

---

## 5. Widening and narrowing

A constrained type is a **subtype of its base** (`byte <: int`, `list <: table`), so the two
directions follow the general rule (`as` spec):

- **Widen to base: implicit and free.** `byte` is usable anywhere `int` is expected with no
  `as`, because dropping a constraint only enlarges the admitted set and the value already
  fits. `byte -> int` never checks and never fails.
- **Narrow to constrained: explicit `as`, runtime-checked.** `int -> byte` adds the
  constraint, which can fail, so it is written `x as byte`, which runs the predicate and raises
  a `typeError` (panic) if it does not hold. `x is byte` is the **boolean test** counterpart: it
  reports whether `x` satisfies the constraint but does not narrow `x`; to obtain a `byte` binding,
  use `as` (as spec §7).

So constraints slot exactly into the widen-implicit / narrow-via-`as` model that governs every
other subtype relationship in Luna.

---

## 6. Composition by conjunction, never by solving

Constraints compose by **running predicates**, not by reasoning about them. A named constraint
used in another's `where` conjoins its predicate:

```luna
const asciiByte = constraint i: int where byte where i <= 127;   // byte AND i <= 127
```

`asciiByte` holds iff `byte`'s predicate holds **and** `i <= 127`. Composition is conjunction,
evaluated in order. Two rules govern it:

- **Base types must match.** A named constraint used inside another's `where` must have a
  compatible base type; using an `int`-based constraint inside a `float`-based constraint is a
  **compile error** (base mismatch). This is the one composition check, and it is cheap (compare
  declared bases, no solving).
- **No implication reasoning.** Constraints are **never** automatically subtypes of one another
  by predicate logic, even when one clearly implies another. `int where 0..100` is not
  automatically usable where `int where 0..255` is wanted; converting between them re-runs the
  target predicate via `as`, a runtime check that happens to always pass. Proving implication
  would require the solver Luna refuses to build, so a redundant clause is simply re-executed,
  not optimized away. `constraint i: int where byte where i <= 255` is sound but
  redundant (the second clause is already implied by `byte`); a *tightening* clause (`i <= 127`)
  is what makes composition useful.

So composition is "execute the conjoined predicates in order," with base-type agreement checked
at declaration and no predicate-implication reasoning ever. The **type-position spelling of the
same conjunction is `&`** (type spec §3.1): `byte & even` in an annotation interns the anonymous
conjunction with identical semantics, base-match, canonical order, no implication, while a
`constraint` declaration like `asciiByte` gives the conjunction a name and its own `typeid`.

### 6.1 Constrained bases: stacking (R155)

The base position itself admits a constrained type — the third spelling of the same
conjunction, and the best one when a refinement genuinely *builds on* another:

```luna
const asciiByte = constraint i: byte where i <= 127;    // ≡ constraint i: int where byte where i <= 127
```

- **Base-first execution is a typing necessity, not a convention**: the binder is
  `i: byte`, so the new predicate's body may *assume base membership* — call
  byte-taking functions, rely on the range — which is only honest if the base chain
  ran first. Entry from an unconstrained value runs root-to-leaf.
- **Delta checking falls out of the fact model** (§7): a value *already typed* `byte`
  entering `asciiByte` skips `byte`'s predicate — the fact rides the value
  (checked-on-write, trusted-on-read) — so `b as asciiByte` on a `byte` runs **one**
  predicate, not two.
- **Representation was already ready**: constraint typeids nest in their base's
  subtype interval (§9.1), so `asciiByte <: byte <: int` is interval-in-interval;
  widening is implicit down the whole chain (§5); `baseOf(asciiByte)` answers `byte`,
  the *immediate* parent (introspection §4.1, R131).
- The base-match rule above reads through the chain: an `int`-based constraint name is
  usable in a `byte`-based constraint's `where` (`byte <: int`, widening is free);
  a `float`-based one still is not.

---

## 7. When the check runs

A constraint is checked on **every operation that could make the value violate it**, and
**trusted everywhere else**. Concretely:

- **On entry:** construction, assignment to a constrained slot, and `as` narrowing run the
  predicate. This is where an unconstrained value becomes constrained.
- **On mutation (mutable-base constraints only):** a constrained value with a mutable interior,
  a table constrained as `list` or by a user table-level constraint (tables §8), or a `bytes`
  buffer, runs the check on **every mutation that could break it** — and that class is
  defined by what a predicate can *observe*, not by a list of write forms (R123). For a
  table that is: every element-key write; every write to a `get`-granted protocol member,
  through any path (external `set` or the protocol's own functions — a predicate's reach
  is the granted surface, protocols §5, so ungranted writes are unobservable and exempt);
  and every change to the applied-protocol set in **either direction** (dynamic `apply`
  and `unapply`, protocols §4). For `bytes` it is every element write. The set-change
  clause bites even in the monotone direction: a predicate can observe protocol facts
  negatively (`where !(t is @tagged)`), so an `apply` can be a breaking write too —
  monotonicity protects `@P` promises (protocols §6.3), never arbitrary predicates. The
  check reads the value's
  **own constraint typeid** (§9.4), so it fires regardless of whether the mutating site is
  statically typed as the constraint or sees only a wider **container path** (references
  themselves never widen, variables §5.1). A violating mutation **panics** (`typeError`,
  errors §9) and leaves the value unchanged, and is a **compile error** where the violation
  is statically evident (`xs['k'] = 1` on `xs: list`, tables §2.1); it never silently
  widens the binding's type (which would need the flow-narrowing Luna refuses, `as` spec
  §7).

  **Immutable-base** constraints (`byte` and the sized ints over `int`, a `string` constraint)
  have no interior to mutate, so **entry is their only check**.
- **On read:** reading a value already typed as the constraint runs **no check**, because entry
  and mutation together maintain the invariant. A `byte` read from a `bytes` buffer is known to
  be `0..255` because every write checked it (bytes spec).

This is the same checked-on-write, trusted-on-read discipline as protocol member types
(protocols §3.3), with "write" spanning **both entry and interior mutation**: validate wherever
the value could change, rely on it everywhere else. The per-mutation cost of a table-level
constraint is the price of that guarantee (tables §8); `list` keeps it O(1) by maintaining
membership as a bit rather than rescanning (tables §2.2), and the compiler elides checks it can
prove redundant (§9.5, §11).

### 7.1 One outcome: a breaking write panics

Every constraint names a **commitment**: the value entered through a declaration that
promised the property, the constraint typeid on the value is the record of that promise
(§9.2), and a breaking write, through any path, is a **wrong write**, the program asked for
a value the promise forbids. The check **panics** with the value unchanged (a compile error
where statically evident). The panic is forced, not a style choice: with references
invariant (variables §5.1), the widened-`&` route is a compile error, but a mutable
constrained value (a `pair` or a `list`, tables §8) can still sit in an **untyped container
slot** and be mutated through it, and that write site cannot see the demands on the value;
the value-carried check (§9.4) is what protects them, and panicking is its only sound
response.

The complementary behavior needs no enforcement at all: a table **not** under any
constraint has made no promise, its shape (list-ness, tables §2.1) is a freely-varying
**fact** reported by `isList()` and the maintained bit (tables §2.2), and no write to it
can violate anything. So "a list is a list until it's not" describes *unconstrained*
tables, where list-ness is a fact, and "no silent conversion, ever" describes
*`list`-declared* positions, where it is a promise. (A drifting "shape class" of
constraint, where breaking writes retag the value instead of panicking, was considered and
rejected: it required a second enforcement mechanism keyed to the write path rather than
the value, plus special inference rules, and the fact/promise split above delivers the same
two behaviors with one mechanism. History in the changelog.)

---

## 8. Reflection

Because a constrained type has its own type identity, it reflects through the ordinary type
operators, with no constraint-specific machinery:

- **`@x`** on a constrained value yields its constraint type (`@b` is `byte` for a byte value),
  the most specific type it has, as `@` does everywhere.
- **`x is byte` / `x as byte`** run the predicate (test-and-narrow / assert-and-narrow), the
  same `is`/`as` as any subtype.

`int` and `byte` being separate types (§1) means introspection distinguishes them; the
detailed query API lives in `std.introspection` (R127), where `baseOf` (R131) and
`constraintPredicate` are specified (introspection §4.1, §4.2).

---

## 9. Representation and value-carried enforcement

A constraint is **not a flag on a base value**; it **mints its own typeid**, and that typeid is
what a constrained value carries. Everything about how constraints compare, travel, widen, and are
enforced follows from this one representational fact, and it is what keeps type comparison a single
integer compare even in the presence of constraints.

### 9.1 A constraint mints a typeid, placed in its base's subtype interval

Each declared constraint (`byte`, `port`, `list`) is assigned **exactly one typeid** at compile
time. The type universe is closed (value-representation §4.1) and canonicalization mints one id per
distinct type (type spec §3), so `byte` and `int` are **different typeids** (§1), never one id plus
a "constrained?" bit. Each constraint's id is placed **inside its base type's subtype interval**
(value-representation §4.2): `byte`, `i8..i32`, and `port` lie within `int`'s interval; `list` lies
within `table`'s. So "is this constrained value usable as its base", the widening, `is`, and
`as` questions of §5 (and `isSubtype` in reflection), is the **O(1) interval check on the id
alone**, with no flag to decode and no
identity to reconstruct. The subtype relation §5 relies on is the interval, not a per-value bit.

### 9.2 The value carries the constrained typeid

A constrained value's `lval` carries its constraint typeid in the ordinary type tag
(value-representation §1), exactly as any value carries its type. There is **no separate
"constrained?" bit** and no base-plus-annotation encoding.

- **`@` is one uint compare.** `@someByte` is `byte`; `@a == @b` compares the constrained typeids
  directly (type spec §3), a single integer comparison, never a decode. (That is *type*-value
  equality; **value** equality on the constrained values themselves erases the constraint,
  `someByte == 65` is true, via the precomputed `valueBase` in the typetable, equality spec §1,
  value-representation §4.)
- **The typeid travels with the value through every channel**, because it *is* the value's type
  and a value has only one: a `&` reference points at the same `lval` (same typeid), a passed
  argument carries its typeid, and table storage keeps it. Widening the **binding's declared
  type** to the base (**collapse**) never rewrites the **value's** typeid: pass a `byte` to `int`
  or a `list` to `table` and the value is still a `byte` / a `list` at runtime, only the static
  demand relaxes. So `@` stays `byte` / `list` while `declared` becomes `int` / `table` (type
  spec §4), the ordinary current-vs-declared split, now carrying the constraint.

### 9.3 The predicate lives in the typetable, off the value

The constraint's predicate (§2) hangs off its typeid in the **comptime `typetable`** (type spec
§6), **not** in the `lval`. A distinct typeid per constraint therefore costs **one index**; the
predicate it points to is shared, static, and never stored per value. So value size is unchanged by
constraints, and a program has exactly as many constraint typeids as it has **declared**
constraints (a source-bounded count, canonicalized once), the same way it has one typeid per
declared union or enum.

### 9.4 Enforcement reads the value's typeid, not the mutating site's static type

The runtime check (§7) keys off the **value's typeid**, found in its `lval`, **not** off the static
type of the binding performing the mutation. This is what makes constraints survive the collapse of
§9.2: a value widened to its base still carries its constraint typeid, so a mutation reached through
a **base-typed** binding is still checked against the value's **actual** constraint.

- **Consequence (deliberate):** a write through a **wider-typed container path** (an untyped
  table slot holding an invariant-constrained table, `outer['p']['x'] = v` where `outer['p']`
  is a `pair`) may **panic where the same write to a bare table would be legal**, because the
  value carries an **invariant-class** constraint (§7.1) the write would break. (The widened
  **`&`** route is closed statically, references are invariant, variables §5.1; and a
  **shape** constraint like `list` never panics through a wider path, it retags, §7.1.) This
  is the honest cost of *collapse-carries-constraints* (§9.2); the
  alternative, constraints evaporating on widening, is the unsound one. To hand a callee a genuinely
  unconstrained table, `copy` it (variables §5.2) or rebuild a bare table (`values()`, tables spec),
  both explicit.

### 9.5 Compile-time knowledge fixes the catalog and enables elision; membership stays runtime

Two facts do **different** jobs, and both survive:

- **The catalog is closed and known.** Constraints cannot be added at runtime, so the predicate (or
  interval test) for any typeid is baked into the emitted code, there is no runtime registry of
  "what does `byte` mean." This is what §1's "distinct type identity" buys at the representation
  level.
- **Membership under mutation is a runtime fact.** Whether *this value* still satisfies its
  constraint after an arbitrary mutation, possibly reached through a widened alias, is the
  flow-sensitive question Luna refuses to compute statically (compiler §1.4.1), so it is checked at
  runtime, on every mutation (§7).

The per-mutation runtime cost is therefore a **ceiling** the compiler knocks down by **elision**
(§11): where a single mutation site is trivially invariant-preserving (appending at the end of a
`list`, writing a literal `byte`, a freshly-produced constrained value), the check is elided, a
local, single-site judgment that needs no control-flow analysis. Elision never changes meaning; it
only removes a check the compiler can prove redundant.

---

## 10. Built-in instances

- **`byte`** = `constraint i: int where i >= 0 && i <= 255`. The element type of `bytes`
  (bytes spec). An **immutable-base** constraint: checked on entry only (§7).
- **`nat`** = `constraint i: int where i >= 0` (R226). The **non-negativity** refinement of
  `int` — the type a signature gives indexes, counts, and sizes — range `0..2^63 - 1`. An
  **immutable-base** constraint: checked on entry only (§7). Deliberately the signed
  family's range refinement, not the unsigned family (numeric-tower §1.1): `nat <: int`,
  so a `nat` flows into every `int`-taking surface with no family crossing, and
  intermediates compute at `int` width — a delta may go negative; only a store-back into a
  `nat` slot checks.
- **`list`** = `table` constrained to keys exactly `0..n-1` (tables §2.1). An ordinary
  **invariant table-level constraint** (§7.1, tables §8), special only in **cost**: every
  table maintains its shape as an O(1) bit (tables §2.2), so `list`'s per-mutation predicate
  reads the bit instead of scanning keys, full invariant protection at O(1) where a general
  table constraint pays its predicate, which is also what licenses the compiler's
  contiguous-representation optimizations for `list`-declared values (tables §2.4,
  Amendment A). On unconstrained tables the same bit is just the queryable fact `isList()`,
  no promise, nothing to enforce.

General user-defined constraints (ports, percentages, non-empty, sorted, and so on) use the same
`constraint` form; those over `table` are table-level constraints (tables §8).

---

## 11. Ruled and deferred (R155)

- *(**Predicate expressiveness: ruled full.** A `where` may contain any expression whose
  calls are all **capability-free** — the statically decidable meaning of pure (R43:
  every ineligibility source is a capability), with R79's function-of-the-value-alone
  requirement discharged by existing rules: capability-freedom bars ambient effects,
  and const-snapshot capture (functions §2.1) freezes any referenced environment, so
  nothing a legal predicate reads can go stale. **No cost carve-out**: the
  runtime-checked stance (§7) already accepts per-entry cost, and an expensive
  predicate is the user's code costing what it costs.)*
- **Static elision: deferred by decision** — compiler optimization. The mechanism is
  fixed (§9.5); the catalogue of trivially provable cases is pending implementation.
- *(**Constraint over constrained base: ruled yes** — §6.1: the base position admits a
  constrained type, desugaring to base-chain conjunction; base-first execution as a
  typing necessity; delta checking from the fact model; nested intervals and `baseOf`
  already in place.)*
- *(**Constraints on other bases: confirmed by shipping practice** — the open never
  closed while the corpus filled with its answers: `json` over `string` (R20), `path`
  over `string` (R21), `probability` and `finiteDouble` over `double` (double §5).
  Any base type works; §7 already splits the machinery by base mutability —
  immutable bases are entry-only, mutable interiors check per mutation, tables are
  §8's instance.)*
