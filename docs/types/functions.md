# Functions

Luna has no free-standing function declarations. Every function is a **lambda**, a `fn`
value, and a named function is simply a lambda bound to a variable. This document
specifies the function value, how it captures its surroundings, and the three properties
its type carries: its capture surface, its errorability, and its comptime-eligibility.

---

## 1. Functions are lambdas bound to variables

A function is a value of `fn` type, written with `fn`, and named by binding it:

```
const substring = fn (str: string, start: int): string => { ... };
```

Because a function is a value, it follows the ordinary binding and module rules: it is
usually bound with `const`, and it is exported from a module by exporting its variable.
There is no separate function-declaration namespace, and no overloading (unions cover
what overloading would, per the language overview).

A `fn` value is, at runtime, a code pointer plus a captured environment (§2). Its `lval`
is an ordinary 16-byte value: the `typeid` identifies the specific function type (which
encodes the properties in §4 and §5), and `dataPtr` refers to the closure. No
function-specific flag bits exist; everything that distinguishes one function type from
another lives in the type, not in per-value flags (§6).

---

## 2. Capture

A function may refer to bindings from its enclosing scope. How it captures them is the
core of the function model, because capture is where a function's *effects* come from.

### 2.1 Auto-capture is by value

By default, a function captures any binding it uses **by value**: it snapshots the
value at closure-creation time. This is safe and implicit precisely because a value
capture cannot affect the outside world, it is a copy, so mutations inside the function
never escape, and changes outside never leak in. Value capture is the same
copy-on-write value semantics as passing an argument (variables spec): logically a copy,
physically shared until a write.

```
let n = 10;
let f = fn (): int => n * 2;    // captures n by value (snapshot); f() is 20
// reassigning n afterward does not change what f captured
```

Because auto-capture is by value and therefore effect-free, it needs no declaration.

### 2.2 `use` is the referential capture operator

To capture a binding **by reference**, so that the function can mutate it and the
mutation is visible outside, the function must declare it in a `use` clause. `use` is the
**referential capture operator**: it exists only to introduce reference captures, so it
always means "by reference" and needs no `&`.

```
let log = fn (msg: string) use (sink) => { sink->append(msg); };
//                          ^^^^^^^^^^^ sink is captured by reference: appends are visible outside
```

A `use` clause is a function's **declared outward-effect surface**. Everything a function
can reach and mutate outside itself is named there; anything not in `use` is captured by
value and inert. This makes effects visible at a glance: a function with no `use` clause
has no outward effects through capture, and a function with `use (a, b)` reaches exactly
`a` and `b` by reference and nothing else.

This mirrors the argument-passing rule (value by default, reference is explicit), so one
principle governs both boundaries a value crosses, a call and a closure: **crossing by
value is implicit and safe; crossing by reference is declared and effectful.**

#### When `use` is required: value-capture cannot serve

`use` (reference capture) is needed exactly when **value capture cannot serve**, and there
are two such cases, for opposite reasons:

- **A `var` you intend to mutate outward.** You reference-capture it so writes inside the
  function escape. Reference capture here requires a **`var`**: a `let` or `const` binding
  cannot be reference-captured (as with `&`, variables spec §5), because reference-capturing
  a fixed binding would imply mutating through it, which `let`/`const` forbid.
- **A `nocopy` value (§2.3), which cannot be copied at all**, so there is no snapshot to
  value-capture. This includes every **capability**. Such a value **must** be `use`d
  regardless of whether its binding is `let` or `const`, not because it is mutable (a
  capability is `const` and zero-data), but because it is uncopyable.

So the two triggers are distinct: a `var` is `use`d because it is *mutable-through*, a
capability is `use`d because it is *uncopyable*. The `let`/`const`-versus-`var` distinction is
the wrong lens for capabilities: a capability is reference-captured because it is `nocopy`, not
because of its binding mode. An ordinary, copyable value with no outward mutation is
value-captured (a snapshot) and never `use`d; a `let` or `const` holding such a value cannot be
reference-captured. The rule is uniform at the use site, `use (x)` always means "reference
capture x," whichever trigger applies, so nothing about the writing or reading of `use` differs
between the two.

### 2.3 Capturing a `nocopy` value

A `nocopy` type (protocols spec) opts out of value semantics: it cannot be copied. It
therefore **cannot be captured by value**, there is no snapshot to take. A function that
uses a `nocopy` binding must capture it by reference, which means it must name it in
`use`, or it is a compile error.

This is the mechanism that makes capabilities explicit. The standard library defines
`io` as `nocopy`, so any function that performs I/O must `use (io)`:

```
let greet = fn (name: string) use (io) => { io->println("hi " . name); };
```

A function cannot silently close over `io`, because value capture of a `nocopy` value is
impossible and reference capture must be declared. So "can this function do I/O" is
answerable from its signature: it does I/O only if `io` (or another I/O capability) is in
its `use` clause. Ambient authority is designed out: `nocopy` on a capability forces
every user of it to declare the capability.

---

## 3. The function type

A function's type records its parameters and result, and additionally two type-level
properties that govern how it may be used:

```
fn (params) : result             // base shape
fn (params) : !result            // errorable (§4)
```

Two functions differ in type if they differ in parameters, result, errorability (§4), or
comptime-eligibility (§5). These are not per-value flags; they are part of the type
identity, so the type system checks them at assignment and call (§7). A `fn` value of one
type is not assignable where an incompatible function type is required, e.g. a throwing
function is not accepted where a non-throwing one is demanded.

Comptime-eligibility being type-distinguishing has two consequences worth stating, since a
function that is comptime and one that is not are genuinely different kinds of value:

- **Type comparison respects it.** `@f != @g` when one is comptime-eligible and the other is
  not, they are different function types. (Value equality is separate: `fn` compares by
  **identity**, equality spec, so `f == g` never consults the signature; the comptime
  distinction lives in the *type*, `@f`, not the value.)
- **The substitution rule is one-directional**, and it turns on the difference between
  **foldability** (a function *can* run at compile time) and **runtime-absence** (a function exists
  *only* at compile time). These are two different properties, and only the first coerces:

  - **An eligible plain `fn` may be used where a `comptime fn` is expected.** A comptime-fn context
    requires that the function be **foldable**, and a comptime-*eligible* plain `fn` is exactly that.
    The compiler already computes eligibility (§5.1), so at the boundary it checks: if the passed
    function is eligible, it is **accepted** (an automatic coercion, no runtime cost); if it is not
    eligible (it has a `use` clause or calls ineligible code), it is a **compile error** at that
    call site. This gating is what makes the coercion both safe and ergonomic, you need not annotate
    a function `comptime` merely to hand it to a comptime-fn parameter; the compiler resolves it.
  - **A declared `comptime fn` may *not* be used where a runtime `fn` is expected.** This is not a
    safety policy but an **impossibility**: a declared `comptime fn` has **no runtime existence**
    (it is consumed at compile time, like an attribute), so a runtime call site handed one would
    have nothing to call. The runtime-`fn` slot needs a callable that exists at runtime, and a
    declared `comptime fn` is definitionally absent then.

  So foldability **widens into** comptime-fn contexts (eligible plain fn is usable as a comptime
  fn), while runtime-absence **does not widen out** to runtime contexts (a comptime fn is not a
  runtime fn). This mirrors errorability (§4): a non-throwing function widens into a context expecting
  a throwing one, but not the reverse. Here an eligible function widens into a comptime context, but
  a runtime-absent one does not widen back to runtime.

Note the distinction between *eligible* and *declared* here: a comptime-**eligible** plain `fn`
exists at runtime (it is an ordinary pure function that *may* also be folded at a `comptime` call
site, and so coerces into comptime contexts), whereas a **declared** `comptime fn` exists only at
compile time (and so cannot go where a runtime function is required). Only the declared form is
runtime-absent; eligibility alone never removes a function from runtime.

The `use` clause is **not** part of the externally-visible type in the same way: it
describes how the function captures its *own* defining scope, which is fixed when the
closure is created, not a parameter the caller supplies. What the caller sees of capture
is its *consequence*: whether the function is comptime-eligible (§5), which is derived
partly from the `use` clause.

### 3.1 Bare `fn` is any callable; a signature opts into checking

A bare **`fn`** (no signature) is the type "any function," the top of the function types,
the callable analogue of `any`. It records no parameters or result, so nothing about a
value's signature is statically checked through a bare-`fn` slot:

```
fn f(cb: fn): ...            // cb is any callable; its signature is unchecked here
fn g(cb: fn (int): string)   // cb's parameters and result ARE checked (§3, §3.2)
```

So **writing a signature opts into checking; bare `fn` opts out.** A field or parameter
typed `fn (int): string` is checked at assignment and call; one typed bare `fn` accepts any
function and forfeits signature safety for that slot (the deliberate trade for
callable-flexibility, mitigated by arity panics, §3.3, and by `fn` still distinguishing a
callable from a non-callable). Choose per use: `fn (A): R` where you want the check, bare
`fn` where you want to accept anything callable.

Specific function types are **subtypes of bare `fn`** (`fn (A): R <: fn`), so the narrowing
asymmetry is the usual one:

- **`fn (int): string` into `fn`** , implicit (a specific function *is* a function).
- **`fn` into `fn (int): string`** , not implicit; requires `as` (the erased signature is
  not statically known, so asserting it is a checked narrowing, `as` spec).

### 3.2 Compatibility is per-position assignability, not a variance system

When a signature *is* checked, one function type is compatible with an expected function
type by **ordinary per-position assignability**, reusing union subtyping and `as`, with no
separate variance calculus:

- **Result**: the callee's result must be usable as the expected result, which is ordinary
  union widening (a callee returning `int` fits a slot expecting `int | string`, because
  `int <: int | string`). This gives result covariance for free from the union subtype
  relation (value-representation).
- **Parameters**: each expected argument must be acceptable to the callee's corresponding
  parameter, an ordinary "does this parameter accept this argument type" check (a wider
  parameter, `int | string`, accepts a narrower argument, `int`). This gives parameter
  flexibility from the same assignability rule.

The checker does **only** this per-position assignability; it never infers deeper variance.
Where the automatic check is too strict, or the value is a bare `fn` whose signature is not
statically known, **`as`** asserts a target signature (runtime-checked), the same escape hatch
as everywhere.

**What the `as` check does, and when.** Narrowing a function to a signature does **not** verify
the whole signature at the `as` site, because a function's conformance to `fn (A): R` is a claim
about *all* its inputs and outputs, observable only when it runs. So `as` defers the signature
check to **each call**, the faithful analogue of value `as` ("check when you can"): a value's type
is checkable now, a function's behaviour only on call. Each call through the narrowed value is
checked in two directions, panicking (a `Panic`, errors spec) on a mismatch:

- **Arguments are checked against the callee's *real* parameters** (contravariance). A missing
  required parameter is a deficit `ArityError` (§3.3); a surplus argument is dropped (§3.3); an
  argument whose type the real parameter does not accept is a `TypeError`. This protects the
  function body, which runs assuming its declared parameter types.
- **The result is checked against the *claimed* return type** (covariance). A returned value that
  is not of the narrowed result type is a `TypeError` **on return**. This protects the caller,
  which consumes the result at the claimed type with no further check, and it is the direction that
  would otherwise cause type confusion: narrowing `fn (int): (int | string)` to `fn (int): int` is
  allowed *optimistically*, and a call that actually returns a `string` panics on return rather
  than seating a `string` in an `int`.

The two runtime checks are exactly the two variance directions above, deferred from compile time to
the call: arguments against the callee's parameters, result against the caller's claim. Because
every call is checked this way, **higher-order narrowing is sound with no variance calculus**: a
function passed to a narrowed function is itself checked when *it* is called, recursively at each
boundary, so nested function types need no structural variance reasoning (this is what makes the
"never infers deeper variance" rule above safe).

**Coercion is the softer alternative to asserting a result.** The result check above *asserts* (it
panics on a mismatch). Where a panic is not what you want, you need not narrow with `as` at all: a
returned value can instead be **coerced** through an ordinary function, which *transforms* rather
than asserts (`as` spec §3–§4, conversion spec). A coercion defines its own failure behaviour, the
API decides it: `toString` always succeeds, `parseInt` returns an error, and another helper may
return a default or a `T?` instead of panicking. So if you *truly need* a value of some type, you
can try to coerce it and let the API say what happens when it cannot, rather than asserting with
`as` and panicking on return. This has always been available and is orthogonal to the `as` rules
above: `as` gives an assert-and-panic result, a coercion gives a transform-and-handle one.

Two limits keep this consistent with `as` elsewhere:

- **Kind mismatch is still eager.** `as` between a function and a non-function (a `fn` value
  `as int`, or a non-callable `as fn (...)`) is disjoint and fails immediately, exactly like
  `"h" as int` (`as` spec §5). Only *signature* conformance defers; being a function at all is
  checked now.
- **Statically-disjoint signatures are a compile error.** When the value's signature is statically
  known and no function could inhabit both it and the target (disjoint parameters *and* disjoint
  results, so no call could satisfy either direction), the narrowing is a **compile error**, not a
  deferred panic (`as` spec §5), for locality. Deferral to per-call checks is for a value whose
  signature is not statically visible (a bare `fn`) or a compatible-but-not-statically-provable
  narrowing.

One consequence follows from deferral: an optimistic narrowing that is **never called** never
panics, there is no behaviour to observe, so no confusion to prevent. This is harmless but differs
from eager value `as`, which panics at the assertion regardless of later use. So function-type
compatibility is unions plus `as`, consistent with the language's "no solver, runtime checks over
static cleverness" stance, and there is no variance system to reason about.

### 3.3 Arity

Argument count and parameter count need not match, and the mismatch is directional:

- **Surplus arguments are dropped.** Calling a function with *more* arguments than it
  declares is fine; the extras are ignored. This is what lets a callback declare fewer
  parameters than the protocol supplies: `myTab->filter(fn () => true)` works even though
  `filter` calls the callback with a value, the callback simply does not name it.
- **Deficit arguments are an error.** Calling with *fewer* arguments than declared is an
  error, not `undefined`: a missing required parameter is a broken contract, not an absent
  value, and `undefined` is too weak (it would be silently coalesced away). The check is a
  **compile error** where the callee's concrete signature is statically visible, and a
  runtime **`ArityError`** (a `Panic`, errors spec) where it is reached through an erased
  `fn` boundary. As a `Panic`, a deficit `ArityError` does not make the enclosing
  higher-order function `fn!`.

So a callback may declare *fewer* parameters than the caller supplies (ignore the rest),
never *more required* parameters than the caller supplies (it cannot invent them). This
directional rule is what makes partial application (§6) a distinct, explicit operation rather
than something that could be confused with under-calling.

### 3.3.1 Default parameters

A parameter may declare a **default value**, in which case it is **not required**, and
omitting it is not a deficit (§3.3): the default is supplied.

```
fn normalize(str: string, form: enum {nfc, nfd, nfkc, nfkd} = {nfc}): string
normalize(s)            // form defaults to {nfc}; not a deficit
```

Defaults are the case where a function that declares *more* parameters than the caller
supplies is still acceptable: the extra parameters must **all have defaults** (so they are
optional, and calling without them supplies the defaults). A callback typed against `fn (A):
R` is therefore acceptable if it can be called with `A`, its parameters beyond `A` must all
be defaulted. Extra *required* parameters remain a deficit error; only defaulted extras are
tolerated. (This is the precise sense in which "more parameters" can be fine, it is defaults,
not an exception to the deficit rule.)

---

## 4. Errorability

A function that can throw a `UserError` **declares** it with `!` on the result type
(value-representation error model):

```
const parseInt = fn (s: string): !int => { ... };   // declared throwing
const double   = fn (n: int): int => n * 2;          // not throwing
```

**Errorability is declared, never inferred.** A function is throwing **iff** its signature
says `!`; the compiler reads `!`-ness off the **signature** and **never computes it from the
body or propagates it up the call graph**. A function that can throw but omits `!` is a
**compile error**, not a function silently promoted to `fn!` (never spec §5). This mirrors
nullability: `!` is a declared type suffix the reader sees and the compiler *verifies*, exactly
as `?` is, not an effect the compiler *discovers*.

What the compiler verifies is a **local, syntactic containment check** on the body, **not**
control-flow analysis (compiler §1.4.1). In a function **not** declared `!`, every call to a
`!`-declared function, and every direct `throw`, must be **lexically enclosed in a handler** (a
`try` expression, a `try` / `catch` block, or an errorable binding that resolves the error,
errors spec). An unhandled throwing call or a bare `throw` in a non-`!` function is a **compile
error**; the fix is to add `!` to the signature, or a handler at the call site. In a function
declared `!`, unhandled throwing calls and direct `throw`s are exactly what the `!` licenses.

Three properties keep this local and predictable:

- **The check reads callee *signatures*, not callee bodies.** "Can `g` throw?" is answered by
  `g`'s declared signature; `g`'s body was already checked against `g`'s own signature when `g`
  was compiled. So there is **no fixpoint, no propagation, and no order dependence**: each
  function is checked in isolation against the declarations of what it calls. This is why
  errorability is **not** a call-graph fixpoint the way comptime-eligibility and capabilities
  are (§5.1); it is a per-function local check, which sits more cleanly beside the no-whole-
  program-analysis stance (compiler §1.4.1).
- **The check is per-call-site and lexical, never path-sensitive.** A throwing call handled on
  one branch and unhandled on another is judged **per site**: the unhandled one requires `!` (or
  its own handler), regardless of whether another path happens to handle it. The compiler never
  asks "is this handled on *every* path" (that would be the flow analysis Luna does not do,
  compiler §1.4.1); it asks only "is *this* call lexically inside a handler." That is stricter
  and predictable, in the same family as the `use`-clause and `undefined`-on-use checks.
- **Panics are exempt.** `!` governs only the `UserError` channel. Any function may raise a
  `Panic` (overflow, a failed `as`, an `ArityError`) without being `fn!` (errors spec §7, §9),
  so the containment check applies to `UserError`-throwing calls, never to panics.

A caller must handle a throwing function's result with `try` or an errorable binding; a
non-throwing function's result needs neither. This is the same errorability the protocol `apply`
uses (`: !self`) and the same that static protocol application inherits (protocols spec §7.5).

**`main` is no exception.** If `main` can throw and does not handle it, `main` **must be declared
`fn!`**, exactly as any other function that lets an error escape. A `fn!` `main` is the deliberate,
signature-visible statement "**`main` may error and end the program, and that is fine**": there is
no caller above `main`, so an escaped `UserError` reaches the **runtime**, which terminates the
program with the error reported (errors §8). A `main` that is *not* `fn!` is therefore statically
guaranteed to have handled every `UserError` internally. `main` gets no inferred `!` and no special
pass, the compiler will not add `!` for it any more than for a leaf function, and (like every
function) it cannot declare panics, which remain ambient (errors §7). So whether the program can end
on a declared error is readable off one line: `main`'s signature.

---

## 5. Comptime-eligibility

A function may be evaluated at compile time, `const c = comptime someFn();`, only if it
is **comptime-eligible**. Eligibility is a type-level property with a single local rule,
propagated over the call graph:

> A function is comptime-eligible iff **every capability it uses is declared `comptime`**
> and every function it calls is comptime-eligible.

The reasoning: `use` (reference capture) is the only way a function reaches mutable
outside state, so what matters is *which* capabilities it reaches. A **non-comptime**
capability authorizes outside effects (real `io`, the filesystem, the network), so using
one makes the function ineligible. A **`comptime`** capability is zero-data and, by its
declaration rule, can compose only other `comptime` capabilities (capabilities §1, §7.1), so
it is provably not a path to any outside effect, holding one at compile time is harmless. A
function that uses only `comptime` capabilities, and calls only comptime-eligible functions,
therefore has no outward effect and is safe to run at compile time, where there is no runtime
world to affect. `io` is **not** declared `comptime`, so any function that reaches it is
ineligible, and the sandbox holds by construction.

### 5.1 It is transitive, but cheap

"Every function it calls is comptime-eligible" is transitive, but it is computed **once,
for the whole program**, as a single fixpoint over the call graph, the same pass and
cost as **capability propagation** (capabilities spec §5). (Errorability is **not** such a
fixpoint, §4: it is a per-function local check against callee *signatures*, not a propagated
computation.)

```
f.comptimeEligible = (every capability f uses is `comptime`) AND (every g that f calls is comptimeEligible)
```

This is O(functions + call-edges), linear, and handles recursion by the standard
monotonic fixpoint (assume eligible, remove eligibility for any function that uses a
**non-comptime** capability or has an ineligible callee, iterate to a fixed point). A
`comptime f()` site is then an **O(1)** read of `f`'s eligibility bit; the transitivity was
paid once, globally, not per call site. This is not a parse-time concern, it is a
post-resolution analysis pass.

### 5.2 It lives in the function type, so indirect calls stay checkable

The eligibility bit is part of the `fn` type. This is what lets a `comptime` call be
checked even when the exact callee is not statically obvious: the type carries the
guarantee. But comptime evaluation additionally requires the callee to be **statically
known**, so a comptime call must be to a `const` binding (a fixed function), not a `let`
that could be reassigned:

```
const pure = fn (n: int): int => n * 2;
const c = comptime pure(21);        // OK: const, comptime-eligible, statically known

let maybe = pure;
comptime maybe(21);                 // error: let binding is not a statically-known callee
```

`const` fixes *which* function is called; comptime-eligibility (in the type) guarantees
*that* function is safe to run at compile time. Both are required: a `const` binding of an
ineligible function is still not comptime-callable, and an eligible function reached
through a reassignable `let` is not either.

### 5.3 Inferred, optionally declared; and `comptime fn` for always-comptime functions

Comptime-eligibility is **inferred** by default: any `use`-free (transitively) function
may be called in `comptime` without annotation. A function may also be **declared**
`comptime`, and there are two strengths of declaration:

- **`comptime` as an eligibility contract on an ordinary function.** Annotating a normal
  function `comptime` requires the compiler to enforce eligibility at its definition, so
  that a later edit adding a `use` of a **non-comptime** capability becomes a compile error at
  the function rather than silently breaking distant `comptime` callers. The function is still an
  ordinary runtime function; the annotation only *guarantees* it stays comptime-eligible.

- **`comptime fn` as an always-comptime declaration.** A binding declared
  `const f = comptime fn (...) => ...` is a function that is **always** evaluated at
  compile time. Every call to it folds, and it **may only be called with
  compile-time-known arguments** (or from within other comptime code). Such a function
  has **no runtime existence**: like an attribute (attributes spec), it is consumed during
  compilation and is never emitted. This is the form the reflection API's structural
  queries use (reflection spec): `const fields = comptime fn (t: type): table` is always
  comptime, so calling it *requires* a compile-time-known `type` argument and *never*
  appears at runtime.

The difference: a plain (or `comptime`-annotated) function *may* be folded at a `comptime`
call site but otherwise runs at runtime; a **`comptime fn`** is folded at *every* call,
requires compile-time-known arguments *at every call*, and has no runtime form. `comptime
fn` is to comptime what an attribute is to data: a compile-time-only construct. Inference
for convenience, `comptime` annotation for contract, `comptime fn` for a function that is
compile-time-only by construction.

A `comptime fn` may of course still be called from an ordinary runtime function, as long as
its arguments are compile-time-known there. Because the criterion is *compile-time-known
arguments*, not *const values*, a call like `fields(@x)` folds whenever `x`'s **type** is
statically known (the common case, any typed binding), even when `x` itself is a mutable
`var` whose value is computed at runtime, what must be static is the *type* passed in, not
the value `x` holds (reflection spec §2).

Conversely, a comptime-*eligible* plain function may be passed **into** a context that expects a
`comptime fn` (the compiler checks eligibility and coerces, or errors if ineligible), while a
declared `comptime fn` may **not** be passed where a runtime `fn` is expected (it has no runtime
existence). This one-directional substitution rule is stated with the function type (§3).

### 5.4 A comptime error is a compile error

Comptime-eligibility (no outward effects) and errorability (can throw) are **independent**
axes. A comptime-eligible function may be throwing (`fn!`): a pure function that parses a
string reaches no outside state (eligible) yet can fail (throwing). So `comptime` places
no constraint on errorability; a throwing eligible function is comptime-callable.

If a comptime call throws, the throw happens during compilation, so it becomes a
**compile error** carrying the thrown error value:

```
const c = comptime parseThing("bad input");   // if parseThing throws, compilation fails
```

This is a nice property: a failure that would have surfaced at runtime is surfaced at
compile time instead, and the error's `stacktrace` (errors spec) is a compile-time trace.
It applies to both error subtrees uniformly, a comptime `Panic` (division by zero, an
`ArityError`) is equally a compile error, so comptime evaluation catches those failures
before the program runs.

A comptime throw is **unconditional**: it is not catchable at compile time. There is no
`try comptime f()` that handles a compile-time throw, because that would pull error
control flow into the const-evaluation phase and raise "what is the type of a maybe-failed
const." The rule is simply: `comptime f()` requires `f` to *succeed* at compile time; any
throw, from either subtree, fails compilation. You already know at compile time that it
errors, so the program does not compile.

### 5.5 Comptime safety

Running code at compile time is, in general, a supply-chain hazard: a dependency's
build-time code could read secrets, touch the filesystem, or reach the network on the
build machine. Luna closes this **by construction**, and then adds liveness guards on top.
The two are different guarantees and worth keeping separate: the capability sandbox
provides *confidentiality and integrity* (comptime cannot reach the outside world), and
the budgets below provide *availability* (comptime cannot hang or crash the build).

**The capability sandbox (security).** Every operation that reaches outside the program,
I/O, filesystem, network, syscalls, the clock, is a `nocopy` capability (protocols spec)
reached only through a `use` reference capture (§2). Comptime-eligibility forbids using any
**non-comptime** capability (§5). Therefore **comptime code can hold no non-comptime
capability**: reaching one would require a `use` of it, which would make the function
comptime-ineligible. A comptime function may hold `comptime` capabilities, zero-data tags
that authorize only comptime-safe operations and provably compose no non-comptime capability
(capabilities §1, §7.1), and may allocate, compute, throw, and return a value, and **nothing
that reaches outside**. It categorically cannot exfiltrate data or execute effects on the build
machine, because every outside-reaching operation is gated by a non-comptime capability it
cannot hold.

This is an *invariant*, not a feature to maintain per capability: any new outside-reaching
operation (an `http` module, a `system` syscall interface) is automatically comptime-safe
the moment it is defined as a `nocopy` capability, because **non-comptime is the default**, a
capability is comptime-unreachable unless someone deliberately declares it `comptime`, and that
opt-in is checked to compose only other `comptime` capabilities (capabilities §1). So the
failsafe direction holds: forgetting the modifier leaves an authority *out* of comptime, never
smuggled in. The corresponding obligations: **adding an outside-reaching operation that is not a
`nocopy` capability is a soundness bug in the sandbox** (no ambient authority anywhere), and
**declaring an outside-reaching capability `comptime` is likewise a bug**, `comptime` is only for
authority whose operations are themselves comptime-safe.

**Liveness guards (availability).** Capability-absence stops exfiltration but not
denial-of-service: a malicious or buggy comptime can still loop forever, recurse without
bound, or allocate without bound, hanging or crashing the build. Four guards bound this,
each aborting with a clear **compile error**, never a hang or a crash:

- **`--no-comptime`** (opt-out flag): forbids all comptime execution, for builds whose
  policy is "no compile-time code runs, period." Comptime is on by default; this is the
  policy switch to turn it off wholesale.
- **Stack depth limit:** a maximum comptime call-stack depth, catching runaway recursion.
  A simple frame counter, generous default, overridable.
- **Execution budget:** a **deterministic** ceiling on comptime work, a step / fuel
  counter decremented per operation, not wall-clock time. Wall-clock is deliberately
  avoided: a compiler-phase limit must be machine-independent, reproducible, and
  cache-compatible, all of which wall-clock time breaks (the same source would pass on a
  fast machine and fail on a slow or loaded one, making builds flaky and non-reproducible
  and undermining cached comptime results). The precise definition of a "step" is deferred;
  the default is calibrated so legitimate comptime is far under it while a runaway trips in
  a fraction of a second of real work. Overridable.
- **Allocation ceiling:** a cap on **peak live memory** during comptime evaluation
  (not cumulative allocation, so allocate-and-discard loops are not falsely tripped),
  **default 8 MB**, overridable. Legitimate comptime (constants, small tables) lives in
  kilobytes to low single-digit MB, so 8 MB is generous headroom while catching runaway
  allocation before it pressures the build host. The default errs low deliberately: a
  too-low ceiling is a one-line opt-up with a clear error, while a too-high one risks
  OOM-pressuring other processes on the build machine before it fires.

The split to remember: **the capability sandbox is the security control** (no reachable
outside world), and **the four guards are liveness controls** (no hang, no crash). Neither
substitutes for the other; together they make compile-time execution safe.

### 5.6 The `unsafe-` capability convention

Most capabilities do effects *within* Luna's guarantees: `io`, `http`, and safe syscalls
(`system`) reach the outside world, but their implementations respect Luna's memory model,
type and seal model, and error model. A small class of capabilities is different: reaching
them **suspends** those guarantees, because the callee is untrusted native code that can
corrupt memory, mutate frozen data, or ignore the error model. These are marked by an
**`unsafe-` prefix** on the capability name.

The test for the prefix: **a capability is `unsafe-` iff, after the call, Luna's guarantees
may no longer hold.** Ordinary capabilities are effects inside the guarantees; `unsafe-`
capabilities are doors out of them.

- **`unsafe-ffi`** , the foreign-function interface. Native code can do anything, so the
  guarantees end at the boundary. Named `unsafe-ffi` (not `ffi`) so both callers and
  callees see the danger at every `use (unsafe-ffi)` site.
- **`system` vs `unsafe-system`** , syscalls split by the same test. Safe syscalls that
  respect the process (reading the clock, `getpid`, `stat`) are `system`, an ordinary
  capability. Syscalls that can corrupt the process or hand back memory outside Luna's
  model (`mmap`, `ptrace`, and the like) are `unsafe-system`.

`unsafe-` capabilities are still ordinary capabilities in every mechanical respect: they
are `nocopy`, reached only through `use`, and therefore **comptime-safe by the same
invariant** (comptime forbids `use`, so it can reach neither `io` nor `unsafe-ffi`). The
prefix adds no new mechanism; it is a warning carried in the name. Luna has no separate
`unsafe` construct (no `unsafe` blocks, no `unsafe fn`), because it has no
guarantee-suspending *language* operation, no raw pointers, no manual memory, no bitcasts
or reinterpret casts, no unsynchronized shared state (green threads enforce copying, so
concurrency never suspends the guarantees and is never `unsafe-`). The only way to leave
the guarantees is through a capability, so the capability system carries the entire
"unsafe" axis, and the `unsafe-` prefix names the subset that does so.

Two properties every `unsafe-` capability carries, beyond an ordinary capability, because
its callee is untrusted:

- **Non-escaping handles.** Anything an `unsafe-` capability hands back (a foreign function
  value, a raw handle) is itself `nocopy` and non-escaping, so the capability cannot be
  laundered by acquiring the handle in a `use`-scope and passing it to un-`use`d code that
  invokes it without the capability. Danger stays contained to declared sites.
- **Invariant-preserving marshalling.** An `unsafe-` boundary must not expose data whose
  immutability the compiler relies on as mutable to the outside, most concretely, it must
  not hand native code a mutable pointer into frozen or `const` table storage, or the
  const-table representation (tables Amendment A) would be unsound. It copies or refuses
  such data rather than exposing it mutably.

The full design of `unsafe-ffi` and the syscall capabilities is deferred to their own
specs; this note fixes only the convention and its comptime-safety, which follow from the
capability model already in this section.

---

## 6. Partial application

Luna has no dedicated partial-application primitive, because a **lambda already is one**.
Binding some arguments and leaving others open is written as an ordinary closure:

```
const add5 = fn (x: int): int => add(5, x);   // "add, with the first argument fixed to 5"
```

This goes through the normal capture rules (§2): `5` is auto-captured by value, the result
is a plain `fn (int): int`, and it inherits errorability and comptime-eligibility correctly
because it is just a closure. Nothing new is needed.

The `_` **wildcard operator** provides sugar for exactly this lambda: `add(5, _)` desugars
to `fn (x) => add(5, x)`, with `_` marking the open argument position. It is **never new
functionality**, only a shorter spelling of the closure above, so it carries the same
semantics (by-value binding of the fixed arguments, derived result type, preserved
errorability and eligibility). Multiple placeholders open multiple parameters:
`f(_, 5, _)` is `fn (a, c) => f(a, 5, c)`.

This is possible, and unambiguous, only because calling a function with **fewer** arguments
than it declares is an error, not implicit currying (§3.3). Under-supplying is a mistake;
partial application is a deliberate, explicitly-marked operation (`_`), so the two never
collide. The full semantics of `_` across all its contexts are in the **wildcard operator**
spec; here it is simply the partial-application sugar.

---

## 7. Why these live in the type, not the `lval`

Errorability (§4) and comptime-eligibility (§5) are properties of the function **type**,
recorded in its `typeinfo` and encoded into its `typeid`, not in the function value's
`lval` flag byte. Two reasons, both from the value-representation discipline:

- **They are not per-value.** Every value of a given function type has identical
  errorability and eligibility; there is no throwing and non-throwing *instance* of the
  same function type. The `lval` flag byte is reserved for per-value dynamic state
  (`isNull`, `isUndefined`), which can differ between two values of one type. These
  cannot.
- **They are derivable from the type.** Storing them as flags would denormalize the type
  and let a flag disagree with the `typeid`, the same reason error-ness is derived from
  the typeid rather than flagged (value-representation §2.1).

So `fn (int): int` and `fn (int): !int` are distinct types with distinct typeids, and the
comptime-eligible and comptime-ineligible variants likewise. A function value's `lval` is
unchanged by any of this: `typeid` (the specific function type) plus `dataPtr` (the
closure). The caller reads errorability and eligibility from the callee's type in O(1),
and the type system checks them at assignment and call, which it could not do if they
were hidden in per-value flags.

---

## 8. Resolved notes

- **`use` and the type:** a function's capture surface stays a **compiler-internal** concern
  and is not surfaced in the externally-visible type. Only its comptime-consequence (§5) leaks
  to callers, which is already handled; surfacing capability-typed captures in the type was
  considered and rejected as needless complexity.
- **`let` vs `const` for a function binding:** they **coincide**. A function has no
  interior-mutable state reachable through its binding, so the interior-freezing that
  distinguishes `const` from `let` has nothing to act on (variables spec §3.1). Both are
  permitted and mean the same thing; **`const` is the convention** for a fixed function
  definition (§1). This is not function-specific: `let` and `const` coincide for every value
  without a mutable interior (scalars, immutable strings, functions).

## 9. Resolved: capturing a `let`

A `let` binding is only ever **value-captured** (a snapshot at capture time), so there is no
binding-versus-slot ambiguity for it. This follows from two facts:

- **`use` cannot reference-capture a `let`** (§2.2): reference capture requires a `var` (to
  mutate outward) or a `nocopy` value (which a `let`-bound ordinary value is not). So a `let`
  holding an ordinary value is captured by value only.
- **A `let` is not repeatedly reassignable** (variables spec §1.1, §1.2): an ordinary `let`
  cannot be rebound at all, and a write-once optional `let` makes at most one `null`-to-value
  transition before freezing. So there is no stream of later values for a capture to "see."

Together: a value-captured `let` snapshots its value at capture time and is unaffected by the
single write-once transition; and there is no way to reference-capture a `let` to begin with. So
"binding or slot" simply does not arise for a `let`. The only reference-captured bindings are
`var`s (which see the live value through the reference, ordinary reference semantics) and
`nocopy` capabilities (which are immutable, so "later values" is moot).
