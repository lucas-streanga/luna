# Numeric operators

This specifies the **arithmetic operators** in detail: `+`, `-`, `*`, `/`, `%`, and unary `-`. It
covers what they compute, their result type, and their behavior on violation (overflow, division by
zero, IEEE sentinels). Two things are deliberately *not* re-specified here and are referenced
instead:

- **The type set** the operators apply to (the signed integers, unsigned integers, floats, and the
  exact/arbitrary-precision types, with their widening and cross-family rules) is defined once in the
  **numeric-tower** spec. This document assumes that tower and does not redefine it.
- **The rule that operators are built-in only, with no overloading** is the general operator
  principle in **operators §1**. It is why arithmetic applies only to built-in numerics and never
  dispatches to user code; this document relies on it rather than restating it.

The full catalogue of *all* operators (arithmetic and otherwise) is in **operators §0**.

---

## 1. The arithmetic operators

The binary arithmetic operators are `+` (add), `-` (subtract), `*` (multiply), `/` (divide), `%`
(remainder), and unary `-` (negate). They apply to the built-in numeric types (numeric-tower spec).
Their **result type is the operand type**: `int + int` is `int`, `double * double` is `double`.

There is **no implicit cross-type arithmetic**: `int + double` does not silently promote. Operands
must be the same numeric type, obtained by explicit conversion where needed
(`someInt.toDouble() + someDouble`). Widening *within* a family is implicit (a `byte` is usable
wherever an `int` is), and *crossing* families is an explicit conversion; both rules, and the family
structure they rest on, are the numeric-tower spec's (numeric-tower §2, §3). This document only notes
that arithmetic operands share one type and that the result carries that type.

### 1.1 Unary minus is the additive inverse; there is no unary plus

Unary `-` (`-x`) is **negation**: it produces the **additive inverse** of its operand. This is a
single, uniform operation defined identically across the **entire numeric tower**, present and
future: `int`, `double` (including the IEEE signed zeros and infinities), and the built-in exact
types `decimal`, `rational`, and `complex` (numeric-tower spec) each have a well-defined additive
inverse (`-(a/b)` is `(-a)/b`; `-(a+bi)` is `-a-bi`). So unary minus is **monomorphic**, the same
operation everywhere, not an overload that means different things on different types, and it does
**not** conflict with the future numeric types: those types *need* it (a negative complex value
is written `-complex(3.0, 4.0)`, negation over the comptime-folding constructor, since there is
no negative-literal syntax — and no complex literal at all, complex §4, R164). Keeping it upholds the
no-overloading rule (operators §1) because there is exactly one meaning.

There is **no unary plus** (`+x`): it would be a pure no-op (`+x` equals `x`), so it is removed as
carrying no meaning.

Lexically, **`-5` is unary minus applied to the literal `5`**, not a negative literal: number
literals are non-negative and the sign is the operator, so `-5`, `-x`, `-(a+b)`, and `-someExpr`
all parse as one construct (unary minus on an operand). The one corner is `int`'s asymmetric range
(the most-negative value `-2^63` is representable while `+2^63` is not): the literal
`9223372036854775808` overflows `int` on its own, so `-9223372036854775808` cannot be lexed as
negate-applied-to-a-literal in the naive way. The compiler special-cases this at the boundary,
recognizing the most-negative literal form and otherwise raising an overflow error, so the general
rule (sign is an operator, no negative literals) holds everywhere else. The exact form is a lexer
detail deferred to the literal grammar, and does not affect the operator.

---

## 2. Behavior on violation: panic for integers, IEEE for floats

The **behavior on violation** is uniform across the integer types and follows the language's
panic-on-violation stance:

- **Integer overflow panics** (int spec §2): arithmetic exceeding the type's range raises a `panic`
  (an `overflowError`), never silently wraps. Wrapping and saturating variants are explicit, named
  functions (`wrappingAdd`, `saturatingAdd`, int spec §4), never the operator default.
- **Integer division and remainder by zero panic** (int spec §5): integers have no infinity or nan to
  yield, so `5 / 0` and `5 % 0` panic.
- **Floating-point follows IEEE** (double spec): overflow yields `+/-inf`, invalid operations
  (`0.0 / 0.0`) yield `nan`, and these propagate as values rather than panicking. Float arithmetic
  does not panic; it produces IEEE sentinels.

So the integer types are **safe by panic** (no silent wrong value; a violation stops at its point),
and the float types are **safe by IEEE** (no silent wrong value; a violation becomes an infinity or
nan sentinel that is itself well-defined). Each type's spec is authoritative for its own edges; this
document only names the shared shape.

Because operator arithmetic can panic (integer overflow, divide-by-zero) but the panic is a `panic`
(ambient, undeclarable, errors spec §2), arithmetic does **not** make a function errorable: `a + b`
returns a plain `int`, not `int!`, and the panic is still catchable with `try` (int spec §3). So
operator-bearing code keeps clean signatures.

---

## 3. Open questions

The type-set questions (whether signed smalls ship, `decimal`'s representation, mixed-width
ergonomics, the library-vs-built-in line) are the **numeric-tower** spec's, not this document's. The
arithmetic-operator semantics above are settled; what remains open lives with the tower (numeric-tower
§7) and with the individual type specs (int, double).
