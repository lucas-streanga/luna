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
Where the automatic check is too strict but you know a function is compatible, **`as`**
asserts it (runtime-checked), the same escape hatch as everywhere. So function-type
compatibility is unions plus `as`, consistent with the language's "no solver, runtime checks
over static cleverness" stance, and there is no variance system to reason about.

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

A function that can throw declares an errorable result with `!` (value-representation
error model):

```
const parseInt = fn (s: string): !int => { ... };   // may throw
const double   = fn (n: int): int => n * 2;          // cannot throw
```

Errorability is part of the function type. It is computed for a function from its body
(a function that throws, or calls a throwing function without handling it, is throwing)
and may be declared for contract stability. A caller must handle a throwing function's
result with `try` or an errorable binding; a non-throwing function's result needs
neither. This is the same errorability the protocol `apply` uses (`: !self`) and the same
that static protocol application inherits (protocols spec §7.5).

---

## 5. Comptime-eligibility

A function may be evaluated at compile time, `const c = comptime someFn();`, only if it
is **comptime-eligible**. Eligibility is a type-level property with a single local rule,
propagated over the call graph:

> A function is comptime-eligible iff it has **no `use` clause** and every function it
> calls is comptime-eligible.

The reasoning: `use` (reference capture) is the only way a function reaches mutable
outside state, so a `use`-free function has no outward effects through capture, and a
`use`-free function that only calls other `use`-free functions has none transitively.
With no outward effects, it is safe to run in the compile-time context, where there is no
runtime world to affect (no live references, no real `io`, which is `nocopy` and so can
only be reached through `use`).

### 5.1 It is transitive, but cheap

"Every function it calls is comptime-eligible" is transitive, but it is computed **once,
for the whole program**, as a single fixpoint over the call graph, the same pass and
cost as errorability propagation:

```
f.comptimeEligible = (f has no `use` clause) AND (every g that f calls is comptimeEligible)
```

This is O(functions + call-edges), linear, and handles recursion by the standard
monotonic fixpoint (assume eligible, remove eligibility for any function with a `use` or
an ineligible callee, iterate to a fixed point). A `comptime f()` site is then an **O(1)**
read of `f`'s eligibility bit; the transitivity was paid once, globally, not per call
site. This is not a parse-time concern, it is a post-resolution analysis pass.

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

### 5.3 Inferred, optionally declared

Comptime-eligibility is **inferred** by default: any `use`-free (transitively) function
may be called in `comptime` without annotation. A function may optionally be **declared**
`comptime` to require the compiler to enforce eligibility at its definition, so that a
later edit adding a `use` clause becomes a compile error at the function rather than
silently breaking distant `comptime` callers. Inference for convenience, declaration for
contract, the same pattern as errorability (§4).

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
reached only through a `use` reference capture (§2). Comptime-eligibility forbids a `use`
clause (§5). Therefore **comptime code can hold no capability at all**: holding one would
require `use`, which would make the function comptime-ineligible. A comptime function can
allocate, compute, throw, and return a value, and nothing else. It categorically cannot
exfiltrate data or execute effects on the build machine, because it has no capability to
do so.

This is an *invariant*, not a feature to maintain per capability: any new outside-reaching
operation (an `http` module, a `system` syscall interface) is automatically comptime-safe
the moment it is defined as a `nocopy` capability, with no comptime-specific code. The
corresponding obligation: **adding an outside-reaching operation that is not a `nocopy`
capability is a soundness bug in the sandbox.** The rule must be total, no ambient
authority anywhere, or comptime could reach it.

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

## 9. Open questions

- **`use` of a reassignable `let`:** when a function reference-captures a `let` binding that is
  later reassigned within its write-once or ordinary rules (variables spec), whether the capture
  sees the **binding** (later values) or the **slot** at capture time. Interacts with the
  reference rules in the variables spec, pending their alignment.
