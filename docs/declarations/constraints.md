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
*enters* the type (construction, assignment, `as`), and a failure is a `TypeError` (panic,
errors §9), exactly like any other narrowing (`as` spec). There is no theorem prover, no
flow-sensitive proof obligation, and type-checking stays decidable. The price is a runtime
check at the boundary (cheap, one predicate call) and no reasoning about predicate implication
(§6).

---

## 1. Declaring a constraint

A constraint is declared with the **`constraint` declaration form**, bound to a `const`:

```
const byte = constraint { int as i where i >= 0 && i <= 255 };
const port = constraint { int as i where i >= 1 && i <= 65535 };
```

- **`base as name`** names the base type and binds the value under test to `name`.
- **`where <predicate>`** is the predicate: a pure boolean expression over `name`. A value of
  the base type inhabits the constrained type iff the predicate holds.

The form **produces a distinct type**. `byte` is a type usable in annotations (`let x: byte`),
and, because of how types are stored (a constrained type has its own type identity), `int` and
`byte` (int-plus-constraint) are **separate types**, not the same type with a flag. When the
reflection API for types is fleshed out, that distinction is visible there.

Multiple `where` clauses are allowed and run in order as a **conjunction** (all must hold):

```
const byte = constraint { int as i where i >= 0 where i <= 255 };   // same as i >= 0 && i <= 255
```

The multi-clause form is sugar for `&&`; order affects only which failure is reported first.

---

## 2. Pure by construction

A constraint predicate **must be pure**: no side effects, no capability use (no `io`, no
`reveal`), deterministic on its input. This is not a rule the author must remember; it is
enforced by the form. A `constraint {}` body admits only a pure boolean expression over the
bound value, so an impure constraint is **unrepresentable**, the same "safe by construction"
move used elsewhere (a `capability` cannot have a body, a regex literal is compiled once).

Purity is essential to soundness. A constraint is checked when a value *enters* the type, and
for that check to mean something durable, the predicate must depend only on the value, not on
mutable external state that could change afterward. An impure constraint could pass at entry
and be false later, making the type incoherent. Pure predicates cannot go stale: if `x`
satisfied `byte` when it became a `byte`, it still does, because `x` and the predicate are
both fixed. This is also why a constraint is **its own declaration form** rather than an
arbitrary function in a type position: the form is what makes purity enforceable.

---

## 3. A constraint carries its base type

A constraint names its base inside the form (`int as i`), so the constraint **is** a complete
refined type on its own. There is no need to restate the base when using it:

```
const byte = constraint { int as i where i >= 0 && i <= 255 };
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
  a `TypeError` (panic) if it does not hold. `x is byte` is the **boolean test** counterpart: it
  reports whether `x` satisfies the constraint but does not narrow `x`; to obtain a `byte` binding,
  use `as` (as spec §7).

So constraints slot exactly into the widen-implicit / narrow-via-`as` model that governs every
other subtype relationship in Luna.

---

## 6. Composition by conjunction, never by solving

Constraints compose by **running predicates**, not by reasoning about them. A named constraint
used in another's `where` conjoins its predicate:

```
const asciiByte = constraint { int as i where byte where i <= 127 };   // byte AND i <= 127
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
  not optimized away. `constraint { int as i where byte where i <= 255 }` is sound but
  redundant (the second clause is already implied by `byte`); a *tightening* clause (`i <= 127`)
  is what makes composition useful.

So composition is "execute the conjoined predicates in order," with base-type agreement checked
at declaration and no predicate-implication reasoning ever.

---

## 7. When the check runs

A constraint is checked on **every operation that could make the value violate it**, and
**trusted everywhere else**. Concretely:

- **On entry:** construction, assignment to a constrained slot, and `as` narrowing run the
  predicate. This is where an unconstrained value becomes constrained.
- **On mutation (mutable-base constraints only):** a constrained value with a mutable interior,
  a table constrained as `list` or by a user table-level constraint (tables §8), or a `bytes`
  buffer, re-checks the invariant on **every mutation that could break it** (every key write for
  a table, every element write for `bytes`). The check reads the value's **own constraint
  typeid** (§9.4), so it fires regardless of whether the mutating binding is statically typed as
  the constraint or as its **widened base**: a mutation reached through a `&t: table` reference
  to a `list` still checks `list`-ness, because the value is a `list`. A violating mutation
  **panics** (`TypeError`, errors §9) and leaves the value unchanged; it never silently widens
  the binding's type (which would need the flow-narrowing Luna refuses, `as` spec §7).
  **Immutable-base** constraints (`byte` and the sized ints over `int`, a `string` constraint)
  have no interior to mutate, so **entry is their only check**.
- **On read:** reading a value already typed as the constraint runs **no check**, because entry
  and mutation together maintain the invariant. A `byte` read from a `bytes` buffer is known to
  be `0..255` because every write checked it (bytes spec).

This is the same checked-on-write, trusted-on-read discipline as protocol element types
(protocols §5.4), with "write" spanning **both entry and interior mutation**: validate wherever
the value could change, rely on it everywhere else. The per-mutation cost of a table-level
constraint is the price of that guarantee (tables §8); `list` keeps it O(1) by maintaining
membership as a bit rather than rescanning (tables §2.2), and the compiler elides checks it can
prove redundant (§9.5, §11).

---

## 8. Reflection

Because a constrained type has its own type identity, it reflects through the ordinary type
operators, with no constraint-specific machinery:

- **`@x`** on a constrained value yields its constraint type (`@b` is `byte` for a byte value),
  the most specific type it has, as `@` does everywhere.
- **`x is byte` / `x as byte`** run the predicate (test-and-narrow / assert-and-narrow), the
  same `is`/`as` as any subtype.

`int` and `byte` being separate types (§1) means reflection distinguishes them; the detailed
type-reflection API is deferred.

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
within `table`'s. So "is this constrained value usable as its base", the widening, `is`, `<:`, and
`as` questions of §5, is the **O(1) interval check on the id alone**, with no flag to decode and no
identity to reconstruct. The subtype relation §5 relies on is the interval, not a per-value bit.

### 9.2 The value carries the constrained typeid

A constrained value's `lval` carries its constraint typeid in the ordinary type tag
(value-representation §1), exactly as any value carries its type. There is **no separate
"constrained?" bit** and no base-plus-annotation encoding.

- **`@` is one uint compare.** `@someByte` is `byte`; `@a == @b` compares the constrained typeids
  directly (type spec §3), a single integer comparison, never a decode.
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

- **Consequence (deliberate):** a function taking `&t: table` may **panic on a write that would be
  legal for a bare table**, because the value it was handed is a constrained table and the write
  would break the constraint. This is the honest cost of *collapse-carries-constraints* (§9.2); the
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

- **`byte`** = `constraint { int as i where i >= 0 && i <= 255 }`. The element type of `bytes`
  (bytes spec). An **immutable-base** constraint: checked on entry only (§7).
- **`list`** = `table` constrained to keys exactly `0..n-1` (tables §2.1). `list` is a
  **table-level** constraint (tables §8): it refines `table` by structure, not by a stored value.
  Its membership is maintained as an **O(1) bit** (tables §2.2) rather than re-checked by a
  predicate scan, so it is the cheap, specialized instance of the general table-level constraint,
  checked on entry *and* on every key mutation (§7) at O(1) rather than O(n).

General user-defined constraints (ports, percentages, non-empty, sorted, and so on) use the same
`constraint {}` form; those over `table` are table-level constraints (tables §8).

---

## 11. Open questions

- **Predicate expressiveness:** exactly which pure expressions a `where` may contain (calls to
  pure user functions, references to `const` values), and whether any are restricted to keep
  checks cheap.
- **Static elision (optimization, not semantics):** the precise set of mutation sites at which the
  compiler elides a runtime check because it can trivially prove the invariant preserved (§9.5),
  distinct from the rejected static-verification model. The mechanism is fixed (§9.5); the exact
  provable cases are open.
- **Constraint over constrained base:** whether a constraint's base may itself be a constrained
  type (`byte`-based constraint), stacking refinements, and how that interacts with the
  base-match rule (§6).
- **Constraints on other bases:** constraints over `string` (e.g. non-empty, matches-a-regex) and
  `float`, and any base-specific concerns. Constraints over `table` are specified as table-level
  constraints (tables §8); `list` and user structural refinements are their instances.
