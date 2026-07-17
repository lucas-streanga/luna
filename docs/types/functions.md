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

A function may refer to bindings from its enclosing scope. Capture is deliberately
inert: a closure's environment can never be a channel for effects. Everything a
function can *do* to the outside world arrives through exactly two doors, both visible
in source: a **capability**, named in `use` (§2.2), or a **`&` reference parameter**,
passed at call time (variables §5.1). Capture itself carries neither.

### 2.1 Capture is an implicit deep-`const` binding

A function captures every copyable binding it references as an implicit **deep-`const`
bind of the current value** (variables §3). This is not an analogy but the same
operation: at closure-creation time the closure takes an independent snapshot, exactly
as `const snapshot = original` does, and holds it deeply immutable. Everything follows
from the `const` rules:

- **Snapshot at creation.** The closure binds the value as it is when the closure is
  created; later rebinding or mutation of the outer binding is invisible inside, and
  there is no way to observe "later values" of anything (which is also why capturing a
  `let` or a `var` needs no distinct rules; see §9).
- **Read-only inside, deeply.** Mutating a captured binding, rebinding it or writing
  its interior (`t.k = v` on a captured table), is a **compile error**, the same error
  as writing through any `const` (variables §3). A closure has no writable environment.
- **Copy-on-write cost model.** As with every `const` bind and every by-value pass,
  the snapshot is logically a copy and physically shared until a write (variables §5).
  Since the closure can never write its captures, the only detaching writes are the
  outer binding's own, so a closure that captures a large table costs nothing until
  the *original* is mutated.
- **`const` captures share.** A binding that is already `const` is captured by
  reference with no copy at all, it is deep-frozen either way (the same rule as the
  spawn boundary, concurrency §2.1).
- **Scalars are trivial.** For values with no mutable interior, `const`-capture and
  plain snapshot coincide (variables §3.1).

```
let n = 10;
let f = fn (): int => n * 2;    // captures n: implicit const snapshot; f() is 20
// reassigning n afterward does not change what f captured

var t = ['count' => 0];
let g = fn (): int => t.count;  // captures t: deep-const snapshot of the current table
t.count = 1;                     // detaches the original; g still sees 0
let h = fn () => t.count = 5;   // COMPILE ERROR: cannot mutate through a capture
                                 // (a capture is a const binding, variables §3)
```

Because capture is a `const` bind, one law now covers every boundary a value crosses,
a call, a closure, a spawn: **crossing is by value (copy-on-write) or by declared
reference; capture is the by-value case, made immutable.**

**Consequences, stated as decisions.** A closure cannot accumulate: there are no
stateful closures in Luna. A counter, `once`, `memoize`, a debouncer, anything that
must retain private mutable state across calls, is a **non-goal** of the closure
model; such state lives in a value the caller owns and passes explicitly. The
accumulating idiom is therefore fold-shaped or reference-shaped:

```
var sum = 0;
each(xs, fn (x: int) => sum = sum + x);         // COMPILE ERROR: sum is a const capture

let total = fold(xs, 0, fn (acc: int, x: int): int => acc + x);   // return the state

var out = [];
&out.push(x * 2) foreach (x in xs);   // or mutate through a reference, in statement position
```

The `&` form is the point, not a workaround: the mutable state appears **in the
signature**, so a function's outward writes are readable off its parameter list, the
recurring rule that facts live in types, never in an invisible environment.

### 2.2 `use` names capabilities, and nothing else

`use` has exactly one meaning: it is how a function comes to hold a **capability**.
A capability is `nocopy` (capabilities §3), so it has no snapshot to `const`-capture;
`use` is the referential capture that reaches the canonical instance (capabilities
§3.1, §4). There is no other referential capture: `use` of an ordinary binding, the
old "reference-capture a `var` to mutate outward", **does not exist**. To let a
function mutate a caller's value, the caller passes `&var` at call time (variables
§5.1); mutation is an argument, never an ambient capture.

```
const greet = fn (name: string) use (io) => { println("hi $name"); };
```

A function cannot silently close over `io`: value capture of a `nocopy` value is
impossible and `use` must be declared. So a function's `use` clause is precisely its
**authority manifest**, it can do exactly what it `use`s, and its parameter list is
precisely its **mutation manifest**, it can write exactly what it is `&`-given. The
environment contributes nothing to either.

### 2.3 Single-owner values: pass a stream, or the capture moves it

A `stream` (or string builder) is stateful and single-owner: it cannot be copied and
cannot be `const` (concurrency §2.1), so it has no `const` snapshot to capture. Two
cases, matching whether the compiler can see the stream:

- **A directly captured stream binding is a compile error.** The stream is statically
  visible, so the fix is stated in the error: **pass it as a parameter**. Streams
  always pass by reference (variables §5), so the closure operates on the caller's
  stream, visibly, at each call.
- **A stream reachable inside a captured table moves.** Whether a table holds a
  stream is a runtime fact, so no compile-time rule can apply; capture therefore
  applies the same **crossing taxonomy as a spawn boundary** (concurrency §2.1):
  copyable parts are snapshotted as usual, and any stream referent is
  **transferred**, marked **taken** (concurrency §2.3), so any later access
  through the outer binding or any alias panics. The closure now solely owns the
  stream, exactly as a spawned task would.

```
let s = 0..10;
let f = fn () => s.take(3);          // COMPILE ERROR: cannot capture a stream; pass it:
let g = fn (src: stream) => src.take(3);

var t = ['s' => (0..10)];
let h = fn (): int => t['s'].count(); // capture moves the stream into h's snapshot
t['s'];                               // PANIC: taken (concurrency §2.3)
```

One rule, two boundaries: a closure environment and a task are the same kind of place,
and a value enters both the same way.

---

## 3. The function type

A function's type records its parameters and result, and additionally two type-level
properties that govern how it may be used:

```
fn (params) : result             // base shape
fn (params) : result!            // errorable (§4)
```

**The literal's header order is fixed** (R45): `fn (params) [use (...)] [: returnType] =>
body`, the `use` clause **before** the return type, then the mandatory `=>`, then a body
that is an expression or a block. Parsing is LL(1) by construction: `fn` commits the
production, and each junction is decided by its next token (`use`, `:`, `=>`). One
disambiguation rule, pinned because the fenced variant literal also uses braces (enum
§3.3): **after `=>`, a `{` always opens a block**; an expression body that is a variant
literal takes parentheses, `fn (): mode => ({read})`. The same rule governs match arms
(match spec).

Two functions differ in type if they differ in parameters, result, or errorability (§4),
and in **nothing else**: the function typeid is signature plus errorability, checked at
assignment and call (§7). Comptime-eligibility left the typeid (R43, below), joining
capabilities on the value. A `fn` value of one
type is not assignable where an incompatible function type is required, e.g. a throwing
function is not accepted where a non-throwing one is demanded.

**Capabilities are deliberately not on this list.** A function's capability set is a
property of the **value**, not the type: two values of written type `fn (int): int` may
hold different authorities, because the set is fixed where the closure is *created*
(capabilities §5.1), which is exactly the per-value situation §7 says the type must not
encode. The type system keeps capability *tokens* out of every value slot (capabilities §3.1), and
function values carry their **requirement set on the value** (capabilities §3.1, R39):
first-class, storable, passable, with direct calls checked statically and calls through
`fn`-typed slots checked dynamically against the executing frame's grant, panic on
shortfall. No capability information appears in the *type*, which is exactly why the check
rides the value. The
division of labor between §4 and this rule is principled, not accidental: **an error
rides a return value**, it is data flowing to the caller, so it must be in the type or it
smuggles (§4, §7); **a capability is authority**, it flows nowhere, so it confines
instead (capabilities §3.1).

**Comptime-eligibility is a value fact, not a type fact** (R43), and a *derived* one: a
function value is eligible iff its **requirement set is empty** (capabilities §3.1, every
ineligibility source is a capability, §5.5, the standing audit assumption recorded in
capabilities §10). Consequences:

- **Type comparison ignores it.** `@f == @g` for same-signature functions regardless of
  eligibility; the fact lives where the requirement set lives, on the value, readable at
  any boundary that needs it. (Value equality is separate as before: `fn` compares by
  **identity**, equality spec.)
- **The substitution rule is one-directional**, and it turns on the difference between
  **foldability** (a function *can* run at compile time) and **runtime-absence** (a function exists
  *only* at compile time). These are two different properties, and only the first coerces:

  - **An eligible plain `fn` may be used where a `comptime fn` is expected.** A comptime-fn context
    requires that the function be **foldable**, and a comptime-*eligible* plain `fn` is exactly that.
    The boundary check reads the value's requirement set (empty = foldable, one word,
    capabilities §3.1): if the passed function is eligible, it is **accepted** (no runtime
    cost); if it is not
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

### 3.1 Bare `fn` is any non-errorable callable; `fn!` admits errorable ones

A bare **`fn`** (no signature) is the type "any **non-errorable** function." It erases
parameters and result, so nothing about a value's *signature* is statically checked
through a bare-`fn` slot, but it does **not** erase errorability: a throwing function
(`fn (...): R!`) does not fit a bare-`fn` slot. The errorable wildcard is **`fn!`**,
"any function, which may error":

```
fn f(cb: fn): ...            // cb is any non-errorable callable; signature unchecked
fn e(cb: fn!): ...           // cb is any callable, errorable or not; calls need try/handling
fn g(cb: fn (int): string)   // cb's parameters and result ARE checked (§3, §3.2)
```

The reasoning is the no-laundering rule (§3, §4): an error rides a return value, so a
slot that erases errorability would let a throwing callback flow to a caller that never
handles the error, the exact smuggling the `!` marker exists to prevent. Signature
erasure forfeits only *signature* safety (arity and argument/result types, recoverable
per call, §3.3, §3.2); erasing errorability would forfeit a **caller obligation**, which
no per-call check can restore. So the wildcard tier splits in two, and a call through
`fn!` carries the same obligations as any errorable call (§4): handle or propagate.

**Writing a signature opts into checking; bare `fn`/`fn!` opts out** of signature
checking only. Choose per use: `fn (A): R` where you want the check, `fn` where you want
to accept anything callable that cannot fail, `fn!` where you accept fallibility and are
prepared to handle it.

The subtype ladder is the usual asymmetry, with errorability widening one way only:

- **`fn (A): R <: fn`** , implicit (a specific non-throwing function *is* one).
- **`fn (A): R! <: fn!`** , implicit, likewise.
- **`fn <: fn!`** , implicit: a function that cannot fail is acceptable wherever
  fallibility is already handled (the same direction as `R <: R!`, i.e. `R <: R | error`,
  §3.2).
- **`fn (A): R!` into `fn`** , **never**, this is the laundering the split forbids.
- **`fn` into `fn (A): R`** , not implicit; requires `as` (the erased signature is not
  statically known, so asserting it is a checked narrowing, `as` spec).

Capability-holding values need no wildcard and get none, but not because they are excluded: the
wildcard tier is a two-way split on **errorability alone** because that is all the typeid carries
besides the signature (§3). A capability-holding function value is **first-class** and enters `fn`,
`fn!`, and signed slots like any other value (R39, capabilities §3.1); what rides with it is a
**requirement set on the value**, and a call through an `fn`-typed slot checks that set against the
executing frame's grant, panicking on shortfall. So authority is confined at the **call**, not
encoded in the type, and no capability tier is needed. (An earlier draft made capability-holding
function values second-class, unstorable and unpassable; R39 repealed that, and the sentence
asserting it survived here until R88.)

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
- **`&` parameters are invariant**: a reference position matches only the **exact** same
  referenced type, `&list` never fits `&table`, in either direction (variables §5.1). A
  reference is a write channel, so the widening that is safe for by-value parameters is the
  classic variance unsoundness for references; exactness is checked here like any other
  per-position rule, with no variance calculus.

The checker does **only** this per-position assignability; it never infers deeper variance.
Where the automatic check is too strict, or the value is a bare `fn` whose signature is not
statically known, **`as`** asserts a target signature (runtime-checked), the same escape hatch
as everywhere.

**What the `as` check does, and when.** Everything about a function value is decidable **at the `as`
site**: the function typeid is *signature plus errorability*, and nothing else (§3), so the real
parameters, the real result, and whether the function throws are all in hand the moment the `as`
runs. Function types are not erased. Deferral is therefore **not forced by ignorance**, and the
rationale earlier drafts gave for it, that a function's conformance is "observable only when it
runs," was false.

`as` defers because **`as` is licensed to assert a claim that is not a subtype relation.** Narrowing
`fn (int): (int | string)` to `fn (int): int` is allowed *optimistically*: no comparison of
signatures can validate it, because it is not true of the type, only (perhaps) of the returns that
will actually happen. Such a claim can be tested only against the calls that occur, so that is where
it is paid for. The license has exactly one boundary, and it is the rule that keeps the model sound:

> **`as` may be optimistic about *values*, never about *obligations*.**

An argument type and a result type are **beliefs** about the calls that will occur; the caller may
know more than the type system does, and being wrong is caught at the call. **Errorability** is a
caller **obligation** (§4): you cannot believe a function will not throw and be checked afterwards,
because the check would arrive *after* the throw. A **`&` parameter** is likewise an obligation, a
write channel (variables §5.1): the calls that occur bound what a callee *reads*, never what it
*writes back*, so no belief about them can make an unsound reference position safe. And
callable-ness is neither belief nor obligation but a precondition.

**Eager**, at the `as`, or at compile time where the source's signature is statically known:

- **Kind.** A `fn` value `as` a non-function type, or a non-callable value `as fn (...)`, is
  disjoint and fails immediately, exactly like `"h" as int` (`as` spec §5). Being a function at all
  is checked now.
- **Errorability.** `fn (A): R!` into `fn`, or into any non-errorable signature, is the laundering
  the split forbids (§3.1). Where the source's signature is statically known, a **compile error**.
  Where it is reached through `fn!` or `any`, both of which admit non-errorable values, the
  narrowing is *not* disjoint and may legitimately succeed, so the typeid decides **at the `as`**
  and a mismatch is a `typeError` there, never at a call.
- **Reference positions.** A `&` parameter is **invariant** (above), so a claimed `&` type differing
  from the real one at all admits no call and is rejected here.
- **Hopelessness.** A narrowing that **no call could satisfy** is rejected rather than deferred,
  because it is a bug whether or not it is ever exercised. A narrowing is hopeless when **any** of:
  a parameter position's claimed and real types are **disjoint** (`fn (int | string): R as
  fn (double): R`, every call fails on the argument); the **result** position's claimed and real
  types are disjoint (`fn (): int as fn (): string`, every call fails on the return); a `&` position
  differs; or the real function has a **required** parameter beyond the claimed arity (§3.3.1), so
  every call deficits. Any one of these dooms every call, so the condition is a **disjunction**. (An
  earlier draft required disjoint parameters *and* disjoint results, which let both examples above
  through, a zero-parameter function has no disjoint parameters at all. It also justified itself as
  "no function could inhabit both," separately wrong: a function value has exactly one signature
  typeid, so *any* two distinct signatures are value-set-disjoint, including the `fn (int): int` /
  `fn (int): int | string` pair the optimistic rule exists to permit. The criterion is "no **call**
  could satisfy," never "no value could inhabit.")

**Deferred**, to each call through the narrowed value, checked in the two variance directions and
panicking (a `panic`, errors spec) on a mismatch:

- **Arguments are checked against the callee's *real* parameters** (contravariance). A missing
  required parameter is a deficit `ArityError` (§3.3); a surplus argument is dropped against the
  **real** arity, not the claimed one (§3.3); an argument whose type the real parameter does not
  accept is a `typeError`. This protects the function body, which runs assuming its declared
  parameter types.
- **The result is checked against the *claimed* return type** (covariance). A returned value that
  is not of the narrowed result type is a `typeError` **on return**. This protects the caller,
  which consumes the result at the claimed type with no further check, and it is the direction that
  would otherwise cause type confusion: narrowing `fn (int): (int | string)` to `fn (int): int` is
  allowed *optimistically*, and a call that actually returns a `string` panics on return rather
  than seating a `string` in an `int`.

Because every call is checked this way, **higher-order narrowing is sound with no variance
calculus**: a function passed to a narrowed function is itself checked when *it* is called,
recursively at each boundary, so nested function types need no structural variance reasoning (this
is what makes the "never infers deeper variance" rule above safe). Errorability is protected at every
depth by those same two checks: an errorable function in **argument** position is rejected against
the real parameter, and in **result** position against the claim, so the no-laundering rule holds
however deep the nesting goes.

**One further check is deferred by necessity, not by license.** A function's **capability requirement
set rides the value, not the type** (§3, capabilities §3.1), while the *grant* rides the executing
frame, which the `as` site cannot see. So a call through an `fn`-typed slot checks the two against
each other and **panics on shortfall, before the callee's body begins** (capabilities §5.1), so a
shortfall refuses an unauthorized call at the boundary rather than halting an effect in progress.
That, and not signature conformance, is the one thing about a function that is genuinely observable
only when it runs, the "check when you can" principle applied to the one fact an `as` genuinely cannot
know. Where the callee *is* statically known, the check is a compile-time one (capabilities §5) and
nothing survives to runtime.

**Coercion is the softer alternative to asserting a result.** The result check above *asserts* (it
panics on a mismatch). Where a panic is not what you want, you need not narrow with `as` at all: a
returned value can instead be **coerced** through an ordinary function, which *transforms* rather
than asserts (`as` spec §3–§4, conversion spec). A coercion defines its own failure behaviour, the
API decides it: `toString` always succeeds, `parseInt` returns an error, and another helper may
return a default or a `T?` instead of panicking. So if you *truly need* a value of some type, you
can try to coerce it and let the API say what happens when it cannot, rather than asserting with
`as` and panicking on return. This has always been available and is orthogonal to the `as` rules
above: `as` gives an assert-and-panic result, a coercion gives a transform-and-handle one.

### 3.2.1 Reading a narrowing: widening a parameter narrows the function type

The single fact that makes function `as` legible, and that inverts most people's first guess:
**`fn (int | string): R` is a *subtype* of `fn (int): R`.** A slot expecting `fn (int): R` only ever
passes ints, and a function accepting `int | string` accepts those, so the wider-parameter function
is usable wherever the narrower-parameter one is. Parameter contravariance. It follows that the
forms which *look* like narrowings are free widenings, and the forms that look harmless are the real
narrowings, which are exactly the ones that defer:

| Written | Relation | What it is | Checked |
|-|-|-|-|
| `fn (int \| string): R as fn (int): R` | source **<:** target | **widening** (parameter contravariance) | never; free and implicit, the `as` is a no-op |
| `fn (int \| string): R as fn` | source **<:** target | **widening** (signature erasure, §3.1) | never; free and implicit |
| `fn (int): R as fn (int \| string): R` | target <: source | **narrowing**, optimistic | per call, on the **argument** |
| `fn (int): (int \| string) as fn (int): int` | target <: source | **narrowing**, optimistic | per call, on the **return** |
| `fn (int \| string): R as fn (double): R` | incomparable | **hopeless**: disjoint parameter | eager |
| `fn (): int as fn (): string` | incomparable | **hopeless**: disjoint result | eager |
| `fn (int, string): R as fn (int): R` | incomparable, `string` required | **hopeless**: deficit | eager |
| `fn (&x: table): R as fn (&x: list): R` | incomparable | **hopeless**: `&` invariance | eager |
| `fn (int): R! as fn (int): R` | — | **laundering** | eager |
| `fn (int): R! as fn` | — | **laundering** (§3.1) | eager |

"Eager" means: a **compile error** where the source's signature is statically known, and a
`typeError` **at the `as`** where it is reached through `fn`, `fn!`, or `any`, since the typeid knows
the real signature either way. The last six rows are not narrowings at all; each is a claim no call
could satisfy, and locality demands they fail where they are written.

The deficit row **flips** when the real `string` parameter is *defaulted* (§3.3.1): the source is then
callable with `(int)` alone, so it *is* a subtype of `fn (int): R`, and the row joins the top of the
table as a free widening. This is the precise sense in which a function may declare "more parameters"
than its slot, it is defaults, never an exception to the deficit rule.

One consequence follows from the deferral, and it is **bounded**. An *optimistic* narrowing that is
**never called** never panics: there is no behaviour to observe, so no confusion to prevent. This is
harmless, and it differs from eager value `as`, which panics at the assertion regardless of later
use. A *hopeless* narrowing gets no such grace, because it is a bug independent of whether it is
exercised, so it is rejected where it is written. So function-type compatibility is unions plus `as`,
consistent with the language's "no solver, runtime checks over static cleverness" stance, and there
is no variance system to reason about.

**`match` decides what `as` defers.** A pattern's typed binder tests with `is`, which is total and
never claims (match §2.2), so `match (f) { g: fn (int): string => ... }` matches only when the real
signature genuinely **is** assignable to the claim. Every per-call check above then folds away, and a
signature-bound function is called with no argument check, no return check, no `ArityError`, and no
laundering check (match §2.3). The optimism, and the per-call price that buys it, belongs to `as`
alone. What survives both is the **capability** check: the requirement set rides the value, not the
type (§3), so no narrowing of any kind can discharge it.

### 3.3 Arity

Argument count and parameter count need not match, and the mismatch is directional:

- **Surplus arguments are dropped.** Calling a function with *more* arguments than it
  declares is fine; the extras are ignored. This is what lets a callback declare fewer
  parameters than the caller supplies: `myTab.filter(fn () => true)` works even though
  `filter` calls the callback with a value, the callback simply does not name it.
- **Deficit arguments are an error.** Calling with *fewer* arguments than declared is an
  error, not `undefined`: a missing required parameter is a broken contract, not an absent
  value, and `undefined` is too weak (it would be silently coalesced away). The check is a
  **compile error** where the callee's concrete signature is statically visible, and a
  runtime **`ArityError`** (a `panic`, errors spec) where it is reached through an erased
  `fn` boundary. As a `panic`, a deficit `ArityError` does not make the enclosing
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

A parameter's `?` marks only its **type** (`p?: T` is `T | null`, the same rule everywhere,
R96); it is never an optionality marker. One piece of sugar (R108): in a parameter list,
`p?: T` with **no written default** reads as `p?: T = null` — a nullable parameter's natural
default is its null, and requiring the `= null` to be spelled would be noise. A non-`?`
parameter without a default remains required.

### 3.3.2 Named arguments

Any argument may be passed **by name** — `f(x, mode: {both})`. Named binding is always
optional and exists for ergonomics: skipping defaulted parameters and making option-heavy
calls readable (`a.merge(b, preserveKeys: true)`). The rules (R108):

- **Positional first.** Positional arguments precede named ones; a positional argument
  after a named one is a compile error.
- **Names bind scalar parameters.** A named argument binds the parameter of that name. The
  variadic is not nameable (it has no single slot, §3.3.3); a destructured parameter
  (destructuring §4) has no name and is not nameable; the receiver *is* nameable in call
  form (`join(it: parts, glue: ", ")`) — no special case.
- **Double-binding is an error.** A parameter bound both positionally and by name: a
  compile error where the signature is statically visible, a **`NamedArgumentError`** (a
  `panic`, errors §2) through an erased `fn`.
- **An unknown name is an error**, on the same static/dynamic split — never silently
  dropped. This is a deliberate asymmetry with §3.3's surplus-positional rule: an extra
  positional argument means "the callee ignores what it does not name" (the callback
  idiom); a wrong *name* always means the caller was aiming at something, and dropping it
  would hide the typo forever.
- **The deficit check runs after named binding** (§3.3): a required parameter filled by
  name is not a deficit.
- **Named arguments work through any function value**, including a type-erased `fn`: a
  `fn` value carries its parameter names as runtime metadata, and named binding resolves
  against them at the call. Two consequences, both accepted: **parameter names are
  contract** (renaming a parameter is a breaking change for name-using callers; std
  parameter names are stable API), and function values pay a small, fixed metadata cost.
- **Names are never manufactured from data.** Spread into an argument list is a sequence
  context requiring a `list` (spread §4); a string-keyed table does **not** spread into
  named arguments. A named argument is visible at the call site, always.
- **Resolution is untouched.** Names play no part in *function* resolution (UFCS is name
  lookup, §3.4; there is no overloading); they only bind arguments once the callee is
  known.

Grammar: `name: value` at the top level of an argument is decidable at one token (`IDENT`
followed by `:` in argument position is a named argument; nothing else begins that way
there). The apply operator's initializer list (protocols §4.2) reads identically on
purpose — same surface, different binding target (protocol members, not parameters).

### 3.3.3 Variadic parameters

A signature's final positional parameter may be a **variadic**, `...name: T` — the pattern
rest element in parameter position. The R35 unification holds (a parameter list is a
pattern): the variadic is the trailing rest of the **positional** parameter sublist, and
what follows it sits outside positional space entirely.

```
fn merge(it: iterable, ...its: iterable,
         recursive: bool = false, preserveKeys: bool = false): iterable

merge(a, b, c)                        // its = [b, c]
merge(a, b, c, recursive: true)       // post-variadic options: named-only, necessarily
```

- **Collected as a `list`, always.** The variadic's arguments are collected into a `list`
  bound to `name`; zero arguments yield `[]`. It declares no default (its default *is*
  `[]`), and each argument is checked against `T` (`...rest` bare is `...rest: any`).
- **Last positional, by law.** After the variadic, positional resolution is undecidable,
  so parameters declared after it are **named-only by construction** — and they must all
  be **defaulted** (R108): a *required* named-only parameter would make named arguments
  mandatory at every call, contradicting their always-optional charter (§3.3.2). The
  former `*,` keyword-only marker is retired as redundant — "after the variadic" already
  says named-only.
- **Spread composes as the identity.** `f(...xs)` explodes a list into arguments and the
  variadic re-collects them, so the callee's `rest` equals `xs` (COW makes the round trip
  nearly free); a list-like stream may fill a variadic (R89, spread §2, §4); mixed forms
  fill in written order (spread §4).

---

### 3.4 UFCS: `x.f(args)` is `f(x, args)`, resolved statically by shape

Uniform function-call syntax lets any free function be called method-style: the receiver
becomes the **first argument** (the convention every std surface already follows, strings
spec, `s.trim()` is `trim(s)`). The resolution rule is **positional and static**, no runtime
probing, no shape types needed:

- **`x.name(` , with a call, is UFCS.** `name` resolves as a **free function** by ordinary
  lexical and import scope (modules spec), exactly as `name(x, ...)` would; there is one
  function per name (no overloading), so resolution is name lookup, nothing more. This holds
  for every receiver type, tables included.
- **`x.name` , without a call, is what `.` otherwise means for the receiver**: element access
  on a table (tables §3.2), the member surface of other types. It never falls back to a
  function value.
- **Calling a *stored* function element is spelled explicitly**: `t['handler'](x)` or
  `(t.handler)(x)`. `t.handler(x)` is UFCS and resolves `handler` in scope; if the intent was
  the element, the parenthesized or indexed form says so. The split is deliberate: a bare
  `table` declares no members statically (no shape types), so "does this table hold a
  function under this key" is a runtime fact, and a resolution rule may not depend on one.
  Shape decides, at the parser, every time.
- **`->` is untouched**: protocol members are reached only by `->` — bare or qualified,
  `b->append(...)` / `b->stringBuilder.append(...)` (protocols §3.1) — never through UFCS,
  and UFCS resolves free functions only; it never searches protocol space. The two
  channels stay disjoint by operator.

One consequence to hold the std surface to: **UFCS plus no-overloading means one signature
per name, receiver first**. Where earlier drafts implied per-receiver variants (`toInt` on
`string` returning `int!` and on `bool` returning `int`; `join`'s receiver position
wandering), those are either **one function with union parameters** (the documented pattern:
"no overloading; unions instead", strings spec) or **distinct names**. The std surface now
satisfies this in full: the catalogues (R91–R92), `join` (R102), and the conversion family
(R106 — `toInt` on `string` became `parseInt`, the *distinct-names* arm; `toInt` survives
exactly once, total, `bool → int`). The rule stays fixed: one name, one signature, first
parameter is the receiver.

## 4. Errorability

A function that can throw a declarable error (errors §2) **declares** it with `!` on the result type
(value-representation error model):

```
const parseInt = fn (s: string): int !=> { ... };   // declared throwing
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
`!`-declared function, and every originating direct `throw` (every user `throw` except a
`panic`-typed re-throw, errors §6), must be **lexically enclosed in a handler** (a
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
- **Panics are exempt.** `!` governs only the declarable channel. Any function may raise a
  `panic` (overflow, a failed `as`, an `ArityError`) without being `fn!` (errors spec §7, §9),
  so the containment check applies to declarable-error-throwing calls, never to panics.

A caller must handle a throwing function's result with `try` or an errorable binding; a
non-throwing function's result needs neither. This is the same errorability an errorable
protocol function declares (`: self!`, string-builder §3.3) and the same the dynamic
`apply()` function carries (`table!`, protocols §4.4).

**`main` is no exception.** If `main` can throw and does not handle it, `main` **must be declared
`fn!`**, exactly as any other function that lets an error escape. A `fn!` `main` is the deliberate,
signature-visible statement "**`main` may error and end the program, and that is fine**": there is
no caller above `main`, so an escaped declarable error reaches the **runtime**, which terminates the
program with the error reported (errors §8). A `main` that is *not* `fn!` is therefore statically
guaranteed to have handled every declarable error internally. `main` gets no inferred `!` and no special
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

The reasoning: `use` (the sole referential capture, §2.2) is the only way a function
reaches the outside world at all, ordinary captures are inert `const` snapshots (§2.1),
so what matters is *which* capabilities it reaches. A **non-comptime**
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

### 5.2 It lives on the value, and the callee is statically known anyway

Eligibility is a fact about the **value**, not the type (§3, R43): a function value is eligible iff
its requirement set is empty, and that set rides the value as a bitmask beside it (capabilities §3.1),
never in the typeid, which carries signature and errorability and nothing else. Two consequences. `@f
== @g` for same-signature functions of differing eligibility, and no `as` can assert it, which is
exactly what keeps eligibility **un-launderable**: were the bit in the type, `f as comptime fn (int):
int` would be an optimistic claim, and `as` may be optimistic about values but never about obligations
(§3.2). Because eligibility is derived from a value fact that `as` cannot reach, there is nothing to
launder and no check to defer.

It needs no place in the type, because comptime evaluation requires the callee to be **statically
known** in any case, so the compiler always holds the function itself and reads its requirement set
directly. A comptime call must therefore be to a `const` binding (a fixed function), not a `let`
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
It applies to both error subtrees uniformly, a comptime `panic` (division by zero, an
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
I/O, filesystem, network, syscalls, the clock, is a `nocopy` capability (capabilities spec)
reached only through a `use` reference capture (§2.2). Comptime-eligibility is computed from
a function's **declared** requirement set only — call-site delegation (capabilities §5.2) is
a runtime-frontier concept, invisible to eligibility and unable to cross a comptime boundary
(capabilities §8, R114) — and it forbids using any **non-comptime** capability (§5).
Therefore **comptime code can hold no non-comptime
capability**: reaching one would require *declaring* a `use` of it (making the function
comptime-ineligible) or *delegating* it in (a compile error onto a comptime call, and reset
to ∅ at the boundary regardless). A comptime function may hold `comptime` capabilities, zero-data tags
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

**Phase invariance.** A comptime-*eligible* plain function is callable in **both** phases, and
comptime evaluation **substitutes its result into the program** (§5.4, compiler spec §6), so its
result must not depend on *which* phase ran it: same arguments, same result, at comptime and at
runtime. Eligibility already guarantees most of this (no capabilities, no outside world, §5.5);
the one further source of phase-divergence is comptime-only information, and it is closed by
construction: the only such information is a value's comptime provenance, readable only through
the `comptype` operator (reflection §3.2), and a function that touches `comptype` is not callable
at runtime at all (its argument type cannot be inhabited there), so it is exempt from the rule
vacuously rather than able to violate it. Every function that *can* run in both phases therefore
computes from values alone, and folding is behavior-preserving.

### 5.6 The `unsafe` capability naming convention

Most capabilities do effects *within* Luna's guarantees: `io`, `http`, and safe syscalls
(`system`) reach the outside world, but their implementations respect Luna's memory model,
type and seal model, and error model. A small class of capabilities is different: reaching
them **suspends** those guarantees, because the callee is untrusted native code that can
corrupt memory, mutate frozen data, or ignore the error model. These are marked by an
**`unsafe` camelCase prefix** on the capability name (`unsafeExec`, `unsafeReveal`); hyphens are not legal in identifiers, `-` is subtraction (associativity §1), so the prefix is a naming convention with zero lexer impact.

The test for the prefix: **a capability takes the `unsafe` prefix iff, after the call, Luna's guarantees
may no longer hold.** Ordinary capabilities are effects inside the guarantees; `unsafe`
capabilities are doors out of them.

- **`unsafeFfi`** , the foreign-function interface. Native code can do anything, so the
  guarantees end at the boundary. Named `unsafeFfi` (not `ffi`) so both callers and
  callees see the danger at every `use (unsafeFfi)` site.
- **`system` vs `unsafeSystem`** , syscalls split by the same test. Safe syscalls that
  respect the process (reading the clock, `getpid`, `stat`) are `system`, an ordinary
  capability. Syscalls that can corrupt the process or hand back memory outside Luna's
  model (`mmap`, `ptrace`, and the like) are `unsafeSystem`.

`unsafe` capabilities are still ordinary capabilities in every mechanical respect: they
are `nocopy`, reached only through `use`, and therefore **comptime-safe by the same
invariant** (comptime forbids using any non-comptime capability, §5.5, so it can reach
neither `io` nor `unsafeFfi`). The
prefix adds no new mechanism; it is a warning carried in the name. Luna has no separate
`unsafe` construct (no `unsafe` blocks, no `unsafe fn`), because it has no
guarantee-suspending *language* operation, no raw pointers, no manual memory, no bitcasts
or reinterpret casts, no unsynchronized shared state (green threads enforce copying, so
concurrency never suspends the guarantees and is never `unsafe`). The only way to leave
the guarantees is through a capability, so the capability system carries the entire
"unsafe" axis, and the `unsafe` prefix names the subset that does so.

Two properties every `unsafe` capability carries, beyond an ordinary capability, because
its callee is untrusted:

- **Non-escaping handles.** Anything an `unsafe` capability hands back (a foreign function
  value, a raw handle) is itself `nocopy` and non-escaping, so the capability cannot be
  laundered by acquiring the handle in a `use`-scope and passing it to un-`use`d code that
  invokes it without the capability. Danger stays contained to declared sites.
- **Invariant-preserving marshalling.** An `unsafe` boundary must not expose data whose
  immutability the compiler relies on as mutable to the outside, most concretely, it must
  not hand native code a mutable pointer into frozen or `const` table storage, or the
  const-table representation (tables Amendment A) would be unsound. It copies or refuses
  such data rather than exposing it mutably.

The full design of `unsafeFfi` and the syscall capabilities is deferred to their own
specs; this note fixes only the convention and its comptime-safety, which follow from the
capability model already in this section.

---

## 6. Partial application

Luna has no dedicated partial-application primitive, because a **lambda already is one**.
Binding some arguments and leaving others open is written as an ordinary closure:

```
const add5 = fn (x: int): int => add(5, x);   // "add, with the first argument fixed to 5"
```

This goes through the normal capture rules (§2): `5` is captured as a const snapshot, the result
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

So `fn (int): int` and `fn (int): int!` are distinct types with distinct typeids, and the
comptime-eligible and comptime-ineligible variants likewise. A function value's `lval` is
unchanged by any of this: `typeid` (the specific function type) plus `dataPtr` (the
closure). The caller reads errorability and eligibility from the callee's type in O(1),
and the type system checks them at assignment and call, which it could not do if they
were hidden in per-value flags.

The same discipline, run in reverse, is why **capabilities live on the value and not in
the type** (§3): a capability set is genuinely per-value, two closures of one written
type may hold different authorities, so encoding it in the typeid would be false, and
encoding it per-value in the type system's view would need per-value types. The rule
that resolves both directions at once: **an error is data, it rides the return value to
the caller, so the type must carry it or it smuggles; a capability is authority, it
rides nothing, so it is confined at the value instead of described in the type**
(capabilities §3.1). Errorability in the typeid, authority in the value, and neither can
be laundered: the first because `fn` never erases `!` (§3.1), the second because a
capability-holding value never enters a slot.

---

## 8. Resolved notes

- **`use` and the type:** a function's capture surface stays out of the externally-visible
  type, now for a stronger reason than before. Ordinary captures are inert `const`
  snapshots (§2.1), so there is nothing about them a caller could need; and capability
  captures never need to be *read* through a type because the requirement set rides the
  **value** and is checked at the call (capabilities §3.1, R39), so an annotation would
  duplicate information the runtime already enforces. Surfacing captures in the type was
  considered and rejected as needless: the value-carried check makes the annotation's job
  vacant. Only the comptime-consequence (§5) leaks to callers, which is already handled.
- **`let` vs `const` for a function binding:** they **coincide**. A function has no
  interior-mutable state reachable through its binding, so the interior-freezing that
  distinguishes `const` from `let` has nothing to act on (variables spec §3.1). Both are
  permitted and mean the same thing; **`const` is the convention** for a fixed function
  definition (§1). This is not function-specific: `let` and `const` coincide for every value
  without a mutable interior (scalars, immutable strings, functions).

## 9. Resolved: capturing a `let` (and a `var`)

Formerly this section reconciled "binding versus slot" for `let` captures. Under §2.1 the
question no longer exists for **any** binding mode: every copyable binding, `var`, `let`,
or `const`, is captured the same way, as an implicit deep-`const` snapshot of the current
value. There is no reference capture of an ordinary binding, so no capture can "see later
values" of anything: a captured `var`'s later reassignments, a write-once `let`'s single
`null`-to-value transition (variables §1.2), are all invisible to an already-created
closure, by the same rule and with no per-mode reasoning. The only referential capture in
the language is `use` of a `nocopy` capability (§2.2), which is immutable, so "later
values" is moot there too.
