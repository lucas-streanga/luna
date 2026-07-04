# Operators

This is the comprehensive reference for **every operator in Luna**: the master catalogue (§0) is a
single table listing each one with its **kind** (arithmetic, comparison, logical, and so on), name,
function, and governing spec. The rest of this document specifies the **arithmetic** operators in
detail and the foundational rule that governs *all* operators: **operators are built-in only; there
is no operator overloading.** Equality (`==`) and ordering are specified separately (equality spec);
each operator's full semantics live in the spec named in the catalogue.

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
| `a + b` | arithmetic | add | numeric addition (never concatenation, never merge) | operators §2, int, double |
| `a - b` | arithmetic | subtract | numeric subtraction | operators §2 |
| `a * b` | arithmetic | multiply | numeric multiplication | operators §2 |
| `a / b` | arithmetic | divide | numeric division (integer div-by-zero panics; float yields IEEE) | operators §2, int, double |
| `a % b` | arithmetic | modulo | remainder | operators §2, int |
| `-a` | arithmetic | negate | unary minus, the additive inverse (uniform across the numeric tower); there is no unary `+` (§2.0) | operators §2.0 |
| `a == b` | comparison | equal | semantic equality (strict, typeid-first; IEEE for doubles) | equality |
| `a != b` | comparison | not equal | negation of `==` | equality |
| `a < b`, `a > b`, `a <= b`, `a >= b` | comparison | ordering | relational comparison (numbers; strings lexicographic by the total order). Precise cross-type behavior is under review | equality (total order) |
| `a && b` | logical | and | short-circuiting logical and (both operands `bool`) | bool |
| `a \|\| b` | logical | or | short-circuiting logical or (both operands `bool`) | bool |
| `!a` | logical | not | logical negation (**prefix**; distinct from postfix `!` errorable) | bool |
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
| `T!` | type | errorable | adds the error arm to a type / function (**postfix, type position**; distinct from prefix `!`) | errors, functions |
| `@x` | type | type-of | value position: the value's `type`; type position: wearer-refinement `@P` | type §1.1 |
| `@@x` | type | protocol-reflect | reflect the protocols a table/view wears | reflection, views |
| `declared x` | type | declared-type | the binding's declared (static) type | type §4 |
| `&x` | reference | reference | pass-by-reference / write-back marker (**prefix, value position**; distinct from infix `&` intersection) | variables §5.1 |
| `copy x` | reference | deep copy | an independent deep copy of a value | variables §5.2 |
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
"make the types that need operators **built-in**" (§4). A limited overloading mechanism "just for
numerics" is rejected: it is the first step back toward general overloading (once `+` is
library-definable for one type, nothing principled stops a user's `Vector3`), and it reintroduces
every hidden-cost and hidden-control-flow problem the rule exists to prevent.

---

## 2. The arithmetic operators

The binary arithmetic operators are `+` (add), `-` (subtract), `*` (multiply), `/` (divide), `%`
(remainder), and unary `-` (negate). They apply to the built-in numeric types (§3). Their **result
type is the operand type**: `int + int` is `int`, `double * double` is `double`. There is **no
implicit cross-type arithmetic**, `int + double` does not silently promote (§3.1); operands must be
the same numeric type, obtained by explicit conversion where needed.

### 2.0 Unary minus is the additive inverse; there is no unary plus

Unary `-` (`-x`) is **negation**: it produces the **additive inverse** of its operand. This is a
single, uniform operation defined identically across the **entire numeric tower**, present and
future: `int`, `double` (including the IEEE signed zeros and infinities), and the built-in exact
types `decimal`, `rational`, and `complex` (numeric-tower spec) each have a well-defined additive
inverse (`-(a/b)` is `(-a)/b`; `-(a+bi)` is `-a-bi`). So unary minus is **monomorphic**, the same
operation everywhere, not an overload that means different things on different types, and it does
**not** conflict with the future numeric types: those types *need* it (a `complex` literal like
`-3-4i` relies on negation, since there is no negative-literal syntax). Keeping it upholds the
no-overloading rule (§1) because there is exactly one meaning.

There is **no unary plus** (`+x`): it would be a pure no-op (`+x` equals `x`), so it is removed as
carrying no meaning.

Lexically, **`-5` is unary minus applied to the literal `5`**, not a negative literal: number
literals are non-negative and the sign is the operator, so `-5`, `-x`, `-(a+b)`, and `-someExpr`
all parse as one construct (unary minus on an operand). The one corner is `int`'s asymmetric range
(the most-negative value `-2^63` is representable while `+2^63` is not): the literal `9223372036854775808`
overflows `int` on its own, so `-9223372036854775808` cannot be lexed as negate-applied-to-a-literal
in the naive way. The compiler special-cases this at the boundary, recognizing the most-negative
literal form and otherwise raising an overflow error, so the general rule (sign is an operator, no
negative literals) holds everywhere else. The exact form is a lexer detail deferred to the literal
grammar, and does not affect the operator.

The **behavior on violation** is uniform across the integer types and follows the language's
panic-on-violation stance:

- **Integer overflow panics** (int spec §2): arithmetic exceeding the type's range raises a `Panic`
  (an `OverflowError`), never silently wraps. Wrapping and saturating variants are explicit, named
  functions (`wrappingAdd`, `saturatingAdd`, int spec §4), never the operator default.
- **Integer division and remainder by zero panic** (int spec §5): integers have no infinity or NaN to
  yield, so `5 / 0` and `5 % 0` panic.
- **Floating-point follows IEEE** (double spec): overflow yields `+/-Infinity`, invalid operations
  (`0.0 / 0.0`) yield `NaN`, and these propagate as values rather than panicking. Float arithmetic
  does not panic; it produces IEEE sentinels.

So the integer types are **safe by panic** (no silent wrong value; a violation stops at its point),
and the float types are **safe by IEEE** (no silent wrong value; a violation becomes an infinity or
NaN sentinel that is itself well-defined). Each type's spec is authoritative for its own edges; this
document only names the shared shape.

Because operator arithmetic can panic (integer overflow, divide-by-zero) but the panic is a `Panic`
(ambient, undeclarable, errors spec §2), arithmetic does **not** make a function errorable: `a + b`
returns a plain `int`, not `int!`, and the panic is still catchable with `try` (int spec §3). So
operator-bearing code keeps clean signatures.

---

## 3. The built-in numeric types and how they relate

The built-in numeric types fall into two **precision/signedness families**, and the relationships
follow one rule: **widening is implicit within a family; crossing families is explicit.**

The current built-in numerics are `int` (64-bit signed), `double` (64-bit float), and the constrained
`byte` (`int` in `0..255`). The numeric tower is being expanded (§4).

### 3.1 Within a family: implicit widening via constraints

The small integer types are **constraints** on a wider primitive (constraints spec), the exact pattern
`byte` already uses (`byte` is `int` constrained to `0..255`). So the sized integers form a widening
chain by the constraint-subtype relation: a value of the narrower type **is** a value of the wider one
in range, so `byte <: int` and (when added, §4) `u8 <: u16 <: u32 <: u64`. Widening is therefore
**implicit and safe** (a `byte` is usable wherever an `int` is), and **narrowing is explicit and
checked** (`int as byte` narrows, panicking if out of range, the `as` of the as spec). This reuses the
constraint machinery entirely: a sized integer is a constrained wider integer to the programmer (clean
semantics, panics out of range), and the compiler may represent it as a machine integer of the exact
width where the type is statically known (static unboxing, compiler spec §7.1.1). Clean semantics,
efficient representation.

### 3.2 Across families: explicit conversion only

Crossing between the signed integers, the unsigned integers, and the floats is **never implicit**.
`int` and `double` are different types (`1 == 1.0` is `false`, equality spec §1), and (when added)
`int` and `u64` are different (signedness differs; `-1` is not a `u64`), and `double` and `float`
are different (precision differs). So `int + double` is **not** valid arithmetic; you convert first:
`someInt.toDouble() + someDouble`. Conversions are explicit functions (conversion spec), visible in
the source, so a change of numeric family is never hidden. This is the direct consequence of
no-implicit-coercion, and it is what "natural and ergonomic" means under strict typing: widening
within a family just works, and crossing families is a visible, deliberate conversion.

### 3.3 `float` is its own primitive, not a constraint

`float` (32-bit) is a **distinct primitive**, not a constraint on `double`. Unlike a sized integer,
which is a *subset* of a wider integer's values (so a constraint), `float` is a *different-precision
representation*, its values are not a subset of `double`'s but a distinct set with its own rounding.
So `float` has its own typeid and its own machine representation (a Go `float32`), and moving between
`float` and `double` is an explicit conversion (§3.2), not a widening. This is the float analogue of
`int` versus `u64`: same operator surface, different primitive, no implicit crossing.

---

## 4. The numeric tower: committed, partly deferred

Luna's numeric type set will grow beyond the current `int`/`double`/`byte`. The following are
**committed** and will be added, but are **deferred past the alpha surface** (they are not needed to
validate the language, and int/double cover the large majority of code):

- **`u64`**, a built-in unsigned 64-bit primitive, with the unsigned smalls (`u8`/`u16`/`u32`) as
  **constraints** on it (the `byte`-on-`int` pattern), and signed smalls (`i8`/`i16`/`i32`) as
  constraints on `int` by the same mechanism.
- **`float`** (32-bit) and **`f16`** (16-bit), the smaller float primitives, forming the widening
  chain `f16 <: float <: double` (numeric-tower spec §1.3). `f16` is used mainly for machine-learning
  and GPU storage.
- **`rational`, `decimal`, `complex`**, added as **built-in** types (not library), so they get
  operator syntax (`+`, `-`, `*`, `/`) under the built-in-only rule and are ergonomic and exact.
  `decimal` is **arbitrary-precision** base-10 (for currency and exactness), and the arbitrary or
  composite ones (`decimal`, `rational`, `complex`) are **heap-backed** built-in reference types, like
  `string` and `table`, since "built-in" means language-defined, not inline (numeric-tower spec §1.4).

There is **no arbitrary-precision integer** (`bigint`); `decimal` covers exact unbounded needs. The
only possibly-library numeric is a **fixed `int128`**, if ever wanted, which would use method syntax.
All the pervasive and exact numerics are built-in and use operators.

**Why these are pre-1.0 but post-alpha.** Adding a built-in numeric type is a **breaking change**: it
expands the closed type universe (new typeids, so exhaustive `match` and reflection over the type set
change) and claims operator behavior. So it must land **before the 1.0 stability commitment**. But it
is **safely deferred past alpha**, because the *mechanism* it slots into is fixed now: the
built-in-only operator rule (§1), the constraint-subtype widening (§3.1), and explicit cross-family
conversion (§3.2). With those locked, adding `rational`/`decimal`/`complex`/`u64`/`float` later is
**new typeids under existing rules**, not new machinery, the clean, mechanical kind of breaking
change, not a retrofit. The one requirement this places on the alpha surface: `match` exhaustiveness
and reflection must be designed to **tolerate the type universe growing**, so that adding numeric
typeids later does not silently break exhaustiveness or type-set reflection.

---

## 5. Open questions

- **Signed small integers.** Whether `i8`/`i16`/`i32` are provided alongside the unsigned smalls (as
  constraints on `int`), or only `byte` and the unsigned family, pending the numeric-tower spec.
- **`decimal` representation.** Whether `decimal` is a fixed-precision primitive (e.g. 128-bit, so an
  unboxed primitive with operators) or arbitrary-precision (heap), which decides
  whether it is fully built-in or a hybrid, pending its own spec.
- **Mixed-width integer arithmetic ergonomics.** Whether any convenience is warranted for combining a
  narrow and a wide integer of the same family (currently: the narrow widens implicitly, §3.1, so
  `byte + int` is `int`), or whether that is already the whole story, pending numeric-tower use.
- **Library-numeric operator ergonomics.** Confirmation that method syntax (`a.add(b)`) is the final
  answer for any library numeric (a fixed `int128`, if provided), with no operator sugar, consistent
  with §1. The pervasive and exact numerics are all built-in and use operators.
