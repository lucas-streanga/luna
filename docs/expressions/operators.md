# Operators

This is the comprehensive reference for **every operator in Luna**: the master catalogue (§0) is a
single table listing each one with its **kind** (arithmetic, comparison, logical, and so on), name,
function, and governing spec. This document also fixes the foundational rule that governs *all*
operators (§1): **operators are built-in only; there is no operator overloading.** The **arithmetic**
operators are specified in detail in the **numeric-operators** spec, and the **numeric type set** they
operate on is the **numeric-tower** spec; equality (`==`) and ordering are the **equality** spec. Each
operator's full semantics live in the spec named in its catalogue row.

---

## 0. The master catalogue

This is **every operator in Luna**, in one table. The **Kind** column groups them by role
(arithmetic, comparison, logical, access, coalescing, type, reference, assignment, control flow,
concurrency, bitwise). Several tokens have **more than one meaning, disambiguated by grammatical
position** (prefix vs. infix vs. postfix, or value-position vs. type-position), the same way C's `*`
is both multiplication and pointer-deref; this is a static parsing fact, not overloading (§1).
Positionally-overloaded tokens (`&`, `!`, `@`) get **one row per meaning**, marked by position.

| Operator | Kind | Name | Function | Spec |
|-|-|-|-|-|
| `a + b` | arithmetic | add | numeric addition (never concatenation, never merge) | numeric-operators, int, double |
| `a - b` | arithmetic | subtract | numeric subtraction | numeric-operators |
| `a * b` | arithmetic | multiply | numeric multiplication | numeric-operators |
| `a / b` | arithmetic | divide | numeric division (integer div-by-zero panics; float yields IEEE) | numeric-operators, int, double |
| `a % b` | arithmetic | modulo | remainder | numeric-operators, int |
| `-a` | arithmetic | negate | unary minus, the additive inverse (uniform across the numeric tower); there is no unary `+` (numeric-operators §1.1) | numeric-operators §1.1 |
| `a == b` | comparison | equal | semantic equality (strict, typeid-first; IEEE for doubles) | equality |
| `a != b` | comparison | not equal | negation of `==` | equality |
| `a < b`, `a > b`, `a <= b`, `a >= b` | comparison | ordering | relational comparison; operands must share a type (widening within a numeric family is implicit, crossing families needs an explicit conversion, as with arithmetic). Numbers order numerically; strings lexicographically by the total order | equality (total order) |
| `a && b` | logical | and | short-circuiting logical and (both operands `bool`) | bool |
| `a \|\| b` | logical | or | short-circuiting logical or (both operands `bool`) | bool |
| `!a` | logical | not | logical negation (**prefix, expression position**; distinct from postfix, type-position `T!` errorable) | bool |
| `x.key` | access | static key | compile-checked element access on a table (**infix, right operand an identifier**) | tables §3.2 |
| `x[k]` | access | dynamic key | runtime element access (miss yields `undefined`) | tables §3.2 |
| `x->name` | access | meta | protocol / meta-space navigation and method call | tables §3.3, views |
| `x?.y` | access | optional access | guarded access: a `null`/`undefined` receiver short-circuits to `undefined` | coalescing |
| `a ?? b` | coalescing | absent-coalesce | `b` if `a` is `undefined`, else `a` (preserves a stored `null`) | coalescing |
| `a ??? b` | coalescing | null-coalesce | `b` if `a` is `undefined` **or** `null`, else `a` | coalescing |
| `a ??= b` | coalescing | absent-assign | assign `b` to `a` only if the key is absent | coalescing |
| `a ???= b` | coalescing | null-assign | assign `b` to `a` if absent or `null` | coalescing |
| `x is T` | type | type test | total **boolean** test: does value `x` have type `T`? never panics, never narrows | is |
| `x as T` | type | checked narrow | narrow `x` to `T`, or panic (`TypeError`); yields a value of `T`, never transforms | as |
| `A \| B` | type | union | type union ("one of"; **type position**) | types |
| `@P & @Q` | type | intersection | protocol-wearer intersection ("all of"; **infix, type position**; distinct from prefix `&` reference) | types, protocols |
| `T?` | type | optional | `T \| null` shorthand (**postfix, type position**) | variables, coalescing |
| `T!` | type | errorable | adds the error arm (`| error`, errors §7) to a type / function (**postfix, type position**; distinct from prefix `!`) | errors, functions |
| `@x` | type | type-of | value position: the value's `type`; type position: wearer-refinement `@P` | type §1.1 |
| `@@x` | type | protocol-reflect | reflect the protocols a table/view wears | reflection, views |
| `declared x` | type | declared-type | the binding's declared (static) type | type §4 |
| `&x` | reference | reference | pass-by-reference / write-back marker (**prefix, value position**; distinct from infix `&` intersection) | variables §5.1 |
| `copy x` | reference | deep copy | an independent deep copy of a value | variables §5.2 |
| `A & B` | type | intersection | the type meet, canonical and total (**infix, type position**; distinct from prefix `&` reference) | type §3.1 |
| `await p` | concurrency | await | park until the task completes; move its result out; consume the promise (word prefix) | await |
| `a += b` (and `-=` `*=` `/=` `%=` `??=`) | assignment | compound assign | `a = a op b`, target evaluated once; `??=` assigns only when `a` is null | associativity §1 |
| `comptype x` | reflection | comptime type | the declaration descriptor of `x`, a `comptype` value (**comptime-only**; like `error`, the word is also the type's name, position-disambiguated) | reflection §3.2 |
| `...x` | reference | spread / rest | spread a table/stream into elements, or collect rest | spread, destructuring |
| `a = b` | assignment | assign | assign `b` to `a`; **the expression evaluates to the assigned value** (§0.3) | variables, operators §0.3 |
| `a .. b` | control flow | range | lazy range stream from `a` to `b` | range |
| `.. by n` | control flow | step | range step | range |
| `x in xs` | control flow | iterate-binding | binds iteration elements in `foreach` | control-flow |
| `pattern where guard` | control flow | guard | boolean guard on a match arm / comprehension | match |
| `match` / `match!` | control flow | match | pattern selection (total `match`, panicking `match!`) | match |
| `defer stmt` | control flow | defer | run `stmt` on scope exit | defer |
| `a \|> b` | control flow | pipeline | pipe left into right (command pipelines and stream stages) | pipeline, command §4 |
| `tab apply proto` | control flow | apply | apply a protocol to a table | protocols |
| `spawn f()` | concurrency | spawn | start a green thread, yielding a `promise` | concurrency |
| `await p` | concurrency | await | resolve a `promise` to `T!` | concurrency |

**Two things deliberately absent from the table**, each because the language provides the capability
another way rather than as an operator:

- **No string-concatenation operator.** Joining strings is **interpolation** (`"$a$b"` is `a`
  followed by `b`; `"$name is $age"`, string-api §13), with `join` and the string builder for
  accumulation. A concat operator would want the `.` token, which is already table key access, and
  that overload is undecidable on a union like `table | string`; interpolation removes the need, so
  `.` keeps its single meaning (§0.1).
- **No bitwise operators yet.** Bitwise and / or / xor / not and shifts are **deferred** (int §8).
  The natural tokens `&` and `|` are already taken (reference / intersection, and union), so the
  bitwise surface will need different tokens or a method form; this is an open decision (§0.4), not
  silently assigned.



### 0.1 `.` has one meaning: table key access

The `.` token means **only** static key access on a table (`tab.key`, tables §3.2). It is **not**
string concatenation, Luna has no concat operator (joining is interpolation, `"$a$b"`; string-api).

This is deliberate. A concat operator would naturally want `.` (the PHP/Perl lineage), but `.` is
already key access, and the two cannot be disambiguated by the left operand's type on a **union**: for
`x: table | string`, `x.foo` would be key-access-or-concat with no way to decide statically (the type
is genuinely both), and deciding by the runtime type would give `.` a runtime-dependent meaning, which
is unacceptable. So concat does not get `.` (or any operator); interpolation covers it, and `.` keeps
its single, unambiguous meaning.

### 0.2 `&` reference vs. intersection is decided by position and fixity

The `&` token has two roles, and they are distinguished **at parse time** by two independent facts,
so the overload is decidable and never ambiguous:

- **`&x`**, prefix, in **expression/value position**, is a **reference** (by-ref argument,
  write-back; variables §5.1).
- **`@P & @Q`**, infix, in **type position**, is protocol-wearer **intersection** (types, protocols).

Value position vs. type position is the same grammatical distinction that decides `@` (type-of vs.
wearer-refinement) and `!` (logical-not vs. errorable), Luna always knows syntactically whether it is
parsing a type (after `:`, `as`, `is`, in a declared type) or an expression. On top of that, fixity
differs (prefix `&x` vs. infix `A & B`), so the two are distinguished twice over. Because types and
expressions are syntactically separated in Luna, there is no context where both could parse, so the
overload is genuinely parse-time decidable. Keeping `&` for intersection preserves the set-theory
pairing with `|` (union), which is the conventional and expected notation.

### 0.3 Assignment produces the assigned value

`a = b` is an **expression**: it performs the assignment and **evaluates to the assigned value**.
This enables capture-and-use in one step and assignment chaining:

```
foreach (line in (buffer = file.lines())) { ... }   // capture the stream, then iterate it
a = b = c;                                            // chains: c assigned to b, then to a
```

This is safe in Luna specifically because there is **no truthiness** (bool spec): the classic
assignment-in-condition footgun `if (x = 5)` is a **compile error** (a condition must be a `bool`,
and `5` is not), so assignment-as-expression cannot be mistaken for an equality test in a condition.
The only residual case, `if (x = true)`, is not a footgun: it assigns `true` and the expression *is*
`true`, so it does exactly what it says (surprising perhaps, but well-defined). A condition is a
`bool` or it does not compile.

### 0.4 What some tokens deliberately do NOT do

- **`+` on tables is a type error, not `merge`.** Combining tables is done with **spread**
  (`[...tab1, ...tab2]`, spread spec), which already expresses merge (and concatenation, and
  insertion) precisely, so no `+`-as-merge and no dedicated merge operator is needed. Keeping `+`
  numeric-only upholds the no-overloading rule (§1): `+` has exactly one meaning. Pervasive numeric
  ops get operators; combining structures is spread.
- **There is no string-concatenation operator.** Joining is interpolation (`"$a$b"`); `.` is table
  key access only (§0.1).
- **Bitwise `&` / `|` have no token yet.** `&` is reference (prefix) and intersection (type
  position); `|` is union (type position). Bit operations are deferred (int §8); when they land they
  need tokens that do not collide, resolved with the bitwise spec, not assumed here.

---

## 1. Operators are built-in only; there is no operator overloading

**An operator's behavior is fixed by the language and applies only to built-in types. No user code
ever runs as a consequence of an operator.** `a + b` is machine arithmetic on built-in numerics, and
nothing else: it never dispatches to a user-defined function, never allocates behind your back, never
takes a lock, never runs a method body. This is the same principle the whole language applies to its
operators: `.` and `[]` are element access, `->` is a meta call, `==` is the strict equality of the
equality spec, and none of them hides user-defined behavior. Operators are **primitive, cheap,
transparent, and single-meaning.**

There is deliberately **no operator overloading**. A library type does not get `+`; a table does not
get `+`; a protocol cannot make `+` mean something. The reasons are exactly the properties operators
are meant to preserve:

- **No hidden runtime cost.** Overloading turns `a + b` into a call that might allocate, block, or be
  expensive. In Luna, a simple operator is a simple operation, its cost is visible in its syntax.
- **No hidden control flow.** An overloaded operator can throw or run arbitrary logic; a primitive one
  cannot surprise you. `a + b` on ints either yields an int or panics on overflow (a `Panic`), with no
  third possibility.
- **One meaning per operator.** `+` is numeric addition, everywhere, for every type it applies to. It
  is never string concatenation, never list append, never a user's notion of "combine." Readers never
  have to ask "what does `+` mean *here*."
- **Consistency with the rest of the language.** Luna already forbids arbitrary user `==` (equality
  spec allows only the bounded `identityEquality` switch, never user comparison code), forbids implicit
  coercion, and forbids truthiness. Operators-are-primitive is the same stance applied uniformly.

So the rule is a hard line: **operator syntax applies if and only if the operand is a built-in type.**
Everything else uses ordinary function or method calls.

### 1.1 Library types use methods, and that is the intended design

Because operators are built-in only, a **library** type expresses arithmetic with methods:
`a.add(b)`, `a.mul(b)`. This is not a workaround; it is the design, and it is acceptable precisely
where it lands. The types that would be library (a fixed `int128`, or non-numeric library types) are
used **deliberately and sparingly**, not woven casually through expressions, so method syntax reads
fine (`total = total.add(item)` is clear, and it makes the operation explicit).
The types used **pervasively and casually** in expressions are built-in primitives, so they get
operators. The set of types that need operator ergonomics and the set that would be library **do not
overlap**, which is why "no overloading" costs almost nothing.

The resolution to "a numeric library type wants `+`" is therefore never "add overloading." It is
"make the types that need operators **built-in**" (numeric-tower spec). A limited overloading
mechanism "just for numerics" is rejected: it is the first step back toward general overloading (once
`+` is library-definable for one type, nothing principled stops a user's `Vector3`), and it
reintroduces every hidden-cost and hidden-control-flow problem the rule exists to prevent.

---

## 2. Resolved and deferred

- **Comparison operators are cross-type-strict, like arithmetic.** `<`, `>`, `<=`, `>=` require
  their operands to **share a type**. Widening *within* a numeric family is implicit (`byte < int`
  works, since `byte <: int`), and *crossing* families (signed / unsigned / float) needs an
  **explicit conversion** (`someInt.toDouble() < someDouble`), exactly the discipline arithmetic uses
  (numeric-operators §1) and the same no-implicit-coercion stance as `==` (equality §1). There is no
  cross-type ordering to fall back on, so a mixed comparison does not compile; you convert first. This
  makes `<` and `==` parallel: `==` is `false` across types, `<` does not compile across types, both
  because the language never coerces to force a comparison through.
- **Bitwise operators are deferred.** Bitwise and / or / xor / not and shifts are not yet specified
  (int §8). The natural tokens `&` and `|` are already taken (reference / intersection, and union),
  so the bitwise surface will need different tokens or a method form (§0.4). This is deliberately left
  to the bitwise spec, not assumed here.

The **arithmetic operators** (`+`, `-`, `*`, `/`, `%`, unary `-`) are specified in the
**numeric-operators** spec; the **numeric type set** they operate on is the **numeric-tower** spec.
Open questions about the numeric types (signed smalls, `decimal` representation, mixed-width
ergonomics, library-vs-built-in numerics) live with those specs, not here.
