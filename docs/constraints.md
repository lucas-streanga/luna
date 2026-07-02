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
  a `TypeError` (panic) if it does not hold. `x is byte` is the boolean-plus-narrowing
  counterpart.

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

A constraint is checked **on entry** (when a base value becomes a constrained value) and
**trusted on use**:

- **On entry:** construction, assignment to a constrained slot, and `as` narrowing run the
  predicate. This is the one place an unconstrained value becomes constrained.
- **On use:** reading a value already typed as the constraint runs no check, because entry
  guaranteed the invariant. A `byte` read from a `bytes` buffer is known to be `0..255` because
  every write checked it (bytes spec).

This is the same checked-on-write, trusted-on-read discipline as protocol element types
(protocols §5.4): validate at the boundary, rely on it afterward.

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

## 9. Built-in instances

- **`byte`** = `constraint { int as i where i >= 0 && i <= 255 }`. The element type of `bytes`
  (bytes spec).
- **`list`** = `table` constrained to keys exactly `0..n-1` (tables §2.1). `list` is
  conceptually a constraint on `table`, though its membership is maintained as an O(1) property
  rather than re-checked by a predicate scan (tables §2.2); it is the same refinement idea with
  a specialized, cheaper implementation.

General user-defined constraints (ports, percentages, non-empty, and so on) use the same
`constraint {}` form.

---

## 10. Open questions

- **Predicate expressiveness:** exactly which pure expressions a `where` may contain (calls to
  pure user functions, references to `const` values), and whether any are restricted to keep
  checks cheap.
- **Static elision (optimization, not semantics):** eliding a runtime check where the compiler
  can trivially prove it (a literal `byte`, a value freshly produced as a `byte`), as an
  optimization that never changes meaning, distinct from the rejected static-verification model.
- **Constraint over constrained base:** whether a constraint's base may itself be a constrained
  type (`byte`-based constraint), stacking refinements, and how that interacts with the
  base-match rule (§6).
- **Constraints on other bases:** constraints over `string` (e.g. non-empty, matches-a-regex),
  `float`, and compound types, and any base-specific concerns.
