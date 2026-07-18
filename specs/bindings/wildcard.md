# The wildcard operator `_`

`_` is a single operator with one meaning that resolves differently by context: **a
deliberate blank, a slot the programmer intentionally leaves unnamed.** What fills the
blank depends on where it appears (a destination, an argument, a pattern, a binding
position), but the intent is always the same: "there is a slot here, and I am choosing not
to name it."

`_` is never a value. You cannot read it, reference it, take `&_`, or store it. It only
ever *marks* an absence; it never *produces* one. This keeps its meaning coherent across
every context below and rules out the uses it deliberately does not have (§7).

---

## 1. The unifying idea

Every use of `_` is one of these blanks:

- a **destination** blank: evaluate something and discard the result (§2);
- an **argument** blank: leave a call argument open, to be filled by a lambda (§3);
- a **pattern** blank: match anything, binding nothing (§4);
- a **binding-position** blank: a parameter or destructuring slot that exists but is not
  named (§5, §6).

Because `_` is always "a blank I am not naming," one symbol serves all four, and the
surrounding syntax (assignment target, call argument, match arm, parameter list,
destructuring pattern) determines how the blank is resolved. There is nothing to memorize
per context beyond "`_` means: intentionally left blank here."

Each occurrence of `_` is **independent and non-binding**. Two `_`s never refer to the
same thing: `f(_, _)` is two separate open arguments, not "the same value twice," and
`[_, _]` ignores two positions independently. `_` never binds a name, so it can never be
referred back to.

---

## 2. Discard: `_ = expr`

`_ = expr` evaluates `expr` and **discards its result**. It is not an assignment (there is
no binding to assign to); it borrows assignment syntax to express "run this for its
effects, drop its value."

```
_ = someFunc();     // call someFunc, explicitly ignore what it returns
```

This is the required companion to strict unused-result checking: an unused return value is
a **compile error**, not a warning, so calling a value-returning function purely for its
effect *needs* an explicit way to say "I am ignoring this on purpose." `_ =` is that way.
Without it, effect-only calls to returning functions would be impossible.

Because `_` is not a binding, none of the binding rules apply: there is no type for `_`,
no `var _` / `let _` / `const _`, and no rebinding. `_ = expr` is a discard statement, not
a declaration.

---

## 3. Partial application: `f(a, _)`

In a call, `_` marks an **open argument**, producing a function of the remaining
parameters. It is **sugar for a lambda** (functions spec §6), never new functionality:

```
add(5, _)                 // sugar for: fn (x) => add(5, x)
f(_, 5, _)                // sugar for: fn (a, c) => f(a, 5, c)
```

The non-blank arguments are fixed (captured by value, per the function capture rules); each
`_` becomes a parameter of the resulting lambda, in left-to-right order. The result is an
ordinary `fn` value, and it inherits the original's errorability and comptime-eligibility
because it is just a closure.

This works precisely because under-supplying arguments is an error, not implicit currying
(functions spec §3.1): `add(5)` is a deficit-arity error, while `add(5, _)` is a deliberate
partial application. The two are syntactically distinct, so there is no ambiguity between
"I forgot an argument" and "I am leaving one open."

---

## 4. Match default: `_ => ...`

In a `match`, `_` is the **match-anything pattern**. It matches any value and binds
nothing, so it is the catch-all / default arm:

```
let label = match (value) {
  0  => 'zero',
  1  => 'one',
  _  => 'many',        // matches anything not matched above
};
```

`_` also carries an **annotation** in a pattern, `_: int`, which tests the value's type and binds
nothing (match §2). That is `_` in *binding* position with a type, exactly as `fn (_: int, x)` is
(§5), not `_` in type position (§7). `_: any` is `_`, because `any` constrains nothing.

As elsewhere, `_` binds nothing, so the default arm cannot refer to the matched value by
`_`. (A default arm that *needs* the value binds it with a name instead.) Because `_`
matches anything, it is typically the last arm; arms after it are unreachable.

---

## 5. Unused parameters: `fn (_, x) => ...`

In a parameter list, `_` declares a **parameter that exists but is not used**. The
parameter still occupies its position (so later parameters line up with the caller's
arguments), but it is not named and cannot be referenced in the body:

```
xs.reduce(fn (_, item) => item);         // want each item, not the running carry
```

This is common with callbacks that receive more than they need. Combined with the arity
rule (surplus arguments are dropped, functions spec §3.1), `fn (_, index)` cleanly says "I
take the second argument, and I am deliberately ignoring the first." Multiple `_`
parameters are each independent unused slots: `fn (_, _, z) => z`.

---

## 6. Destructuring skip: `let [a, _, c] = ...`

In a destructuring binding, `_` **skips a position**: the value at that position is not
bound to any name.

```
let [first, _, third] = triple;      // bind positions 0 and 2, skip position 1
```

The skipped position is still consumed (so following bindings align), but its value is
discarded, the destructuring analogue of the discard in §2 and the unused parameter in §5.
Each `_` skips one position independently.

---

## 7. What `_` is not

`_` is deliberately *not* used for these, to keep its single meaning ("a blank I am not
naming") from sprawling into unrelated concepts:

- **Not type inference.** `let x: _ = expr` is not a thing. Type inference is already the
  default (a bare `let x = expr` infers), so `_` in type position would be a redundant
  second spelling, and it would dilute `_` from "a value/binding blank" into "infer this,"
  a different idea. Inference stays implicit; `_` stays out of type position. This is what
  `_: T` (a match type test, §4; an unused typed parameter, §5) does **not** violate: there
  `_` is the thing being typed, on the **left** of the `:`, which is binding position. `_`
  never appears on the right.
- **Not a default or natural bound.** `_` does not mean "the default value" or "the natural
  end" of a range or slice. Those are *produced* values, and `_` never produces anything;
  it only marks an omission. Conventions like "`length <= 0` means to the end" (string API)
  cover that need without overloading `_`.
- **Not a readable value.** `_` cannot appear where a value is expected (`x = _`,
  `f(_)` as an actual argument value, `&_`). It marks slots; it is never itself a value.

The boundary is: **`_` always marks an intentional blank in destination, argument, pattern,
or binding position; it never produces a value, names a binding, or stands in a type.**
Everything in §2 through §6 fits that; everything in §7 would break it.

---

## 8. Rulings (were open questions)

All three ruled (R37):

- **`_` in nested destructuring: subsumed.** Nested destructuring itself is deferred by
  decision (destructuring §7, R35); when depth lands, `_` composes by the flat rules at
  each level with nothing new to define. Solved by the same ruling.
- **Trailing `_` parameters: legal, kept.** `fn (x, _)` and `fn (x)` behave identically at
  the call (surplus arguments drop either way, functions §3.3), and the trailing `_` is
  kept precisely because it *says so*: it documents "a second argument arrives here and is
  deliberately ignored" at the signature, an arity intent the shorter form leaves implicit.
  Explicitness that costs nothing is Luna's default posture.
- **Whole-value discard is `_ = expr`, only.** No bare-`_` statement form, no postfix
  discard; the one spelling is the assignment (errors §8.1's no-silent-drop form), one way
  to say one thing.
