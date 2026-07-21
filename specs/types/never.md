# `never`

`never` is the **bottom type**: the type with **no values**. Nothing is ever a `never`, and a
function whose return type is `never` does not hand a value back to its caller. `never` is the dual of
`any` (the top type, which accepts every value): `never` accepts none. It sits at the bottom of the
type lattice, so **`never <: T` for every type `T`**, and it is the **identity for union**: `T |
never` is just `T`. That algebra is what makes `never` compose cleanly (§3).

`never` names the situation "calling this does not give you a value." There are two ways that
happens, and they are distinguished by the ordinary errorability suffix `!`:

- **`fn (): never`**, control **never returns at all** (the function exits, halts, or diverges).
- **`fn (): never!`**, control returns **only as an error**, never as a value (the function always
  throws).

These are not two keywords; they are `never` (the empty value type) with and without `!` (the error
channel). The word is honest in both: with `never` you get **nothing** back; with `never!` you never
get a **value** back (the `!` tells you an error comes instead).

`never` is valid **only as a function return type**. Because it has no values, no binding could ever
hold one, so writing it in value position (a variable, field, or parameter) is a compile error (§5);
it is meaningful only as "what the caller receives," which is a claim about control, not storage.

---

## 1. `fn (): never`, control does not return

A function returning `never` (non-errorable) **does not return to its caller**: it
panics, halts the thread, or loops forever. The statement after such a call is
unreachable, and the `never`-returning branch contributes nothing to any value (§3). Its
inhabitants come in exactly two kinds (R215): **divergers** (an event loop that never
terminates) and **always-panickers** — and the latter exist *because* `die` exists:
users construct no panic values themselves (errors §9), but `die(msg)` raises the `died`
panic on their behalf (errors §5.2, R215), so `die` and its wrappers are the failure
inhabitants of `fn (): never` — panic-side, unchecked, no `!`. This section's original
text called `die` "the primitive case," and R215 confirms it (an intermediate ruling,
R214, briefly made `die` a declarable thrower — superseded). What is *not* an
inhabitant, and never will be: a process-exit function (the exit code is `main`'s return
value and teardown is structured — no call unwinds the process past pending `defer`s
from mid-frame; std.process §3, R134); an uncaught `die` *reaches* program exit by
unwinding, which is the difference.

```
const fatal = fn (msg: string): never => { die("fatal: $msg"); };   // an always-panicker
const loop  = fn (): never => { while (true) { tick(); } };          // a diverger
```

### 1.1 `never` (the exit case) is not checked at compile time

The claim "control never returns" is a **control-flow** property, not a type property, and Luna does
**not** analyze it. Proving that every path through a function diverges is control-flow analysis (in
the general case, the halting problem), and Luna deliberately does no such analysis (it has no
reachability solver, consistent with its static-when-simple stance). So a `fn (): never` annotation is
an **unverified claim by the programmer**: the compiler **trusts** it (it treats the call's
continuation as dead, and lets the branch contribute `never` to unions) but does **not prove** it.

### 1.2 A `never` function that returns **panics**

Because the exit claim is unchecked at compile time, Luna checks it at **runtime**. If control ever
reaches the end of a `fn (): never`, that is, the function **returns** when it promised never to, the
runtime **panics** (a `panic`, errors spec): "a `never` function returned." This is the same stance
the language takes everywhere it cannot prove a property statically: it does not allow a silent wrong
result (the caller would proceed with no value where the compiler assumed none, which would be
corruption), and it does not do expensive static analysis to rule it out; it **checks at runtime and
panics on violation**, exactly as integer overflow and constraint violations do.

The cost is a trap on the function's return path, which a **correct** `never` function never executes,
so correct code pays nothing; only a function that lied about diverging hits the trap, and it gets a
loud, catchable panic instead of undefined behavior. So `never` (the exit case) is an **unverified
compile-time claim backed by a runtime guarantee**: assert it freely, and if you are wrong, you get a
panic, not a corrupt program.

---

## 2. `fn (): never!`, always throws

A function typed `never!` has an **empty value arm** and a **live error arm**: it can error (that is
what `!` means, functions spec §4), and because there is no value it could return instead (the value
type is `never`), it **must always error**. So `never!` is precisely "**always throws**."

This falls straight out of the type algebra. An errorable function returns a value-or-error union; if
its value type is `never`, the union is `never | Error`, which collapses to just `Error` (`never` is
the identity for `|`, §3). So `never!` is not a special construct: it is `never` (the value type) under
the ordinary `!` suffix, and its meaning ("only ever an error, never a value") is exactly what that
composition says.

```
const rejectAll = fn (msg: string): never! => { throw error(msg); };   // always throws, never a value
```

(This is the **declarable** always-fails shape — a user-written thrower whose callers
must handle or propagate. It is deliberately *not* `die`: `die` lives on the **panic**
side, `fn (msg): never` with no `!`, raising the `died` panic — errors §5.2, R215 — so it
imposes no `fn!` contagion on its callers. The two one-line failure forms, one per
channel, are paired at errors §5.2.)

### 2.1 `never!` (the throw case) **is** checked

Unlike the exit case, `never!` is a **type-level** fact, and Luna already tracks it. Errorability is
a declared part of the function type, locally verified — never propagated or inferred (functions
spec §4): the compiler
knows a `never!` is errorable and enforces it the same way it enforces every `fn!`. A caller must
handle the error (with `try`, or by being `fn!` itself), exactly as for any errorable call. So the
**error is checked**, at compile time, through the existing errorability machinery. What is asserted
(that the value arm is genuinely empty, that the function does not somehow return a value) is the
programmer's claim, but it lives *inside* the checked errorability frame, and there is no value channel
for a caller to misuse: a `never!` call yields control only through the handled error path.

---

## 3. The lattice role: why `never` composes

`never` is the bottom of the type lattice, and its two algebraic properties are what make it useful
beyond annotating no-return functions:

- **`never <: T` for all `T`.** A `never` value is usable where any type is expected (vacuously, since
  there are no `never` values), so a `never`-returning branch type-checks in any context. This is what
  lets `if (c) { x } else { die("no x") }` have the type of `x`: the `die` branch is `never` (its
  panic is ambient, on no channel at all — R215), and `T | never` is `T`. (An earlier draft's
  example called `exit()`, a function R134 abolished — the sweep's missed site, R214.)
- **`T | never == T`.** `never` is the identity for union, so a branch that never produces a value
  contributes nothing to the result type. A function that "returns an `int` or exits" has value type
  `int | never == int`, which is correct: the exit branch adds no values. This is also why `never!`
  collapses to just its error type (§2).

So `never` is the neutral element that makes union types compose: unreachable and non-returning
branches drop out, leaving exactly the types that can actually be produced.

---

## 4. The asymmetry, stated plainly

The two forms are checked at **opposite ends**, and the reason is precise:

- **`never!` is a type fact** (the value/error split), which Luna's type system already tracks, so it
  is **checked at compile time** (it is ordinary errorability).
- **`never` is a control fact** (whether control returns), which Luna's type system does **not**
  analyze (that would be control-flow analysis, which Luna does not do), so it is **asserted, not
  checked, at compile time, and guarded at runtime**: a `never` function that returns panics (§1.2).

This is not an inconsistency; it is a direct consequence of what the type system sees. It tracks
errorability (so `never!` is checked) and does not track reachability (so `never` is runtime-guarded).
The language's uniform rule underneath both: **never a silent wrong value**, checked statically where
the type system can see it, and by a runtime panic where it cannot.

---

## 5. Resolved

- **`never` is valid only as a function return type.** In any **value position**, a variable
  annotation, a field type, a parameter type, `never` is a **compile error**, because `never` has no
  values: no binding could ever be given an inhabitant, so such a type describes a slot that can never
  be filled. `never` is meaningful only as a **return type**, where it describes what the caller
  receives (nothing, or only an error), which is a statement about control, not about storage. The
  compiler still uses `never` **internally** as the union identity (`T | never == T`, §3), so a
  diverging branch contributes `never` to a union that then collapses; that is the type algebra, not
  the programmer writing `never` in value position. The rule governs where you may **write** it:
  return position only.

- **`never!` is never inferred, because errorability is never inferred.** Luna's errors are **checked
  but explicit** (functions spec §4, errors spec): the `!` on a function is a **declared** part of its
  signature, a contract the reader can see, that the compiler **verifies** but never **discovers**. A
  function that can throw must say so; the compiler checks that the `!` matches reality but does not
  add it for you. So a `fn` whose every path throws is **not** silently promoted to `never!` (or even
  to `fn!`); it is either already declared errorable or it is a compile error. This is not a
  `never`-specific rule, it is the universal "errorability is explicit" applied to `never`. It mirrors
  the exit case (§1.1): both forms of `never` are always **written** by the programmer, never inferred,
  one because proving divergence is control-flow analysis Luna does not do, the other because
  errorability is a deliberate, visible contract.
