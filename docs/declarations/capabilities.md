# Capabilities

A **capability** is an unforgeable token of authority: the right to perform an
outside-reaching or guarantee-affecting operation. Reaching the network, spawning a process,
revealing a secret, and calling foreign code are all gated by capabilities. A capability
carries **no data**; its value is nothing and its *type identity* is the authority. Holding
one means "you may do this"; not holding one means, checkably, that you cannot.

Capabilities underpin several guarantees relied on elsewhere: they sandbox comptime (comptime
cannot hold one, functions §5.5), they make effects auditable (a function's `use` clause is
its capability manifest), and they let the *absence* of a capability be a guarantee (a
function without `use (reveal)` cannot reveal a secret, secret spec).

---

## 1. Declaring a capability

A capability is declared with the **`capability` declaration form**, bound to a `const`, the
same way a protocol or error type is declared (`proto {...}`, `error {...}`):

```
const reveal = capability;
const exec   = capability;
const io     = capability;
const cfg    = comptime capability;             // opt into comptime use (§8); non-comptime is the default
const webApp = capability { io, net };  // a composed set: grants io and net together (§7.1)
```

- **Bare declares a leaf; braces compose a set.** A bare `capability` (no braces) declares a
  single new authority. **Braces declare a composed capability**, a *set* (§7.1) whose members
  are the listed capabilities: `capability { A, B }` grants the authority of `A` and `B`
  together, and `use`-ing it requires and propagates each member. The braces are legible here
  precisely *because* a capability is zero-data: they can only be reading a **set of member
  capabilities**, never a data body, so there is no confusion with `proto {}` / `error {}`,
  whose braces delimit state. A capability still has **no body and can never carry data**; only
  capabilities may appear inside the braces (`capability { 5 }` or any non-capability content is
  a syntax error), and there is no `union` to consider, the braces are a **set** of members, not
  a choice among them.
- **Each declaration is a distinct type.** `const reveal = capability` declares `reveal` as a
  **distinct capability type**, just as `myError = error {}` declares a distinct error type.
  `reveal` and `io` are *different types*, which is what lets `use (reveal)` demand the
  reveal authority specifically and nothing else.
- ~~The `implicit` modifier~~: **removed** (R33). Every capability is explicit; a
  required-but-undeclared `use` is a compile error, uniformly, no inference tier exists.

Capabilities live in the **modules that define them** (`io` exported by `std.io`, `reveal` by `std.secret`), imported and named like any binding (§4), a light,
dataless module, so that capability names never collide with ordinary function names. `reveal`
the function (secret spec) and `reveal` the capability coexist because they are in
different namespaces (the exact namespacing mechanism is the module system's, deferred).

---

## 2. `capability` is the union of all capabilities

`capability` is itself a type: the **union of all declared capability types**, the natural
"any capability," exactly as `any` is the union of all types. A specific capability is a
member:

```
reveal <: capability          // a specific capability is-a capability
io     <: capability
```

So `capability` is a **supertype** of every specific capability, the same relationship
`error` has to each error type. This is a declaration form and union supertype in the same
family as `proto` and `error` (types spec). It is **not** a type-theory "kind"; Luna has no
kind system.

---

## 3. Capabilities are nocopy

Every capability is **nocopy**, but not every nocopy value is a capability:

- **Capability implies nocopy.** A capability that could be copied could be forged or
  laundered, so nocopy is required. This also makes a capability comptime-excluded (comptime
  forbids capturing nocopy values, §8) and reachable only through `use`.
- **Nocopy does not imply capability.** A nocopy value that is not declared with the
  `capability` form is just nocopy data, not an authority. For example `argv` (the program's
  arguments) is nocopy immutable data passed to `main` (§7); it is not a capability, because
  reading your own arguments is not an authority whose absence anyone needs to prove.

So a capability is precisely **a nocopy value that is also a declared authority with a
distinct type identity**. The nocopy-ness gives unforgeability and comptime-exclusion; the
`capability` form gives the checkable identity that makes `use`-absence a guarantee.

Because capabilities are nocopy, a function **cannot** take one as an ordinary by-value
parameter, stash it, and hand it to other code. The only way to hold a capability is to `use`
it, which is what makes it unlaunderable: there is no value-level copy to smuggle out of a
`use` scope. Aliasing a capability-using function does not escape its requirement either,
because the alias itself is impossible: a capability set rides on the function **value**
(fixed where the closure is created, §5.1), and a capability-holding function value is
**confined**, it cannot be bound to a new name, passed, stored, or returned at all
(§3.1); note this is the capability *token*, function values that require capabilities
are first-class and carry a requirement set instead (§3.1, R39). There is no renamed alias to worry about, because the rename is a compile error.

### 3.1 `use` is the sole channel: capabilities never enter a value slot

For the `use`-clause to be a **complete** manifest of a function's authority (§4), and for the
comptime sandbox (§8) to hold, a capability must reach a scope **only** through `use`, and never
through any value-carrying channel. It does not, because it is nocopy (and always `const`, §1):

- **Not a parameter.** `fn (c: reveal) { ... }` is illegal: passing an argument copies it into
  the parameter slot, and a capability cannot be copied. So a capability cannot be smuggled in
  as a function argument (the hole this closes: otherwise a function could reveal without
  declaring `use (reveal)`, voiding the absence guarantee).
- **Not a binding.** `let c = reveal` is illegal (nocopy). **Aliasing** an existing capability to
  a new name is likewise **not permitted**: it would copy a nocopy value, and there is no use for
  the alias anyway, since a `use` clause names the capability's declared type, not an alias.
  (Declaring a capability, `const reveal = capability`, §1, is a different thing and is how
  capabilities come into being; it is not aliasing an existing one.)
- **Not a field, element, or return value.** A capability cannot be stored in a table or list or
  returned, all are value slots a nocopy value cannot enter.
- **Not an `&` reference.** `&reveal` is illegal because `&` requires a **`var`** binding
  (variables spec) and a capability is always `const`. So a capability cannot be taken by
  reference outside `use`; the const-ness already blocks `&`, and no separate "no-reference"
  mechanism is needed (a blanket one would wrongly block `use` itself, which *is* a referential
  capture).

The one open channel is **`use`**, which is a referential capture, not a copy. A capability
acquired through `use` is **not itself a value**: it cannot be assigned, passed, stored, or
returned. It only authorizes the capability's operations and **propagates to callees through
their own `use` clauses** (§5), again by reference, never as an argument. So a capability is
canonical and always-there, reachable only by naming it in `use`, and it never becomes a
manipulable value at any point. This is what makes `use (X)` the complete and only account of a
function's authority.

**Function values carry a requirement set, and are first-class.** An earlier draft made
capability-holding function values second-class (unstorable, unpassable), the strongest
possible confinement, adopted because no dynamic check existed. Repealed (R39): the check
exists, and the confinement was stronger than the invariant needed. The model:

- **A function value carries its requirement set**, fixed at creation (§5.1): the union of
  its `use` clause and its direct callees' declared requirements. This is a static fact
  stamped on the **value**, a small set over the closed, compile-time-enumerable capability
  universe (a bitmask in practice, one word on the function value), it is **not** a captured
  capability instance, the tokens themselves still never enter any value slot (above).
- **Direct calls stay statically checked**, unchanged: calling a function by its declared
  name is verified against the caller's own `use` set at compile time (§5), and this remains
  the overwhelmingly common case.
- **Calls through function-typed values are checked dynamically**: invoking any value in a
  `fn`-typed slot verifies **requirement set ⊆ the executing frame's granted set** (the
  frame's declared `use`, granted transitively from `main`, §7), one bitmask compare, and
  **panics** on shortfall. `let f = println; f("hi")` is now legal, and runs iff the frame
  holds `io`.
- **`spawn` on a function value** performs the same check against the spawner's grant at the
  spawn.

**The laundering theorem survives the repeal, and is worth stating**: every actual exercise
of a capability occurs under a declared `use` on the executing path, because the dynamic
caller must hold the requirement, and holding means having declared it, granted from `main`
down. What travels through capability-free code is the closure as **inert data**, never as
exercisable authority: possession of the value is not possession of the grant. So the audit
(§4, "who can do io" is a search for `use (io)`) is exactly as true as before; what moved is
*when* a violation surfaces, compile time for direct calls, a panic at the call for
value-mediated ones, the honest cost of first-classness, paid only on the dynamic frontier.
Two consequences come free: the **comptime sandbox** (§8) preserves itself, no grants exist
at comptime, so a requirement-carrying value invoked there fails the check vacuously; and
callback-taking APIs (`map`, handlers in tables, the test table, tests spec §4) accept
effectful functions with no special cases, their effects gated by the ambient grant at the
point of invocation.

**Sandboxing is latent, deferred by decision (R80).** Compiler-driven capability
revocation, compiling untrusted source under restricted authority, needs exactly **one
mechanical act: the compiler modifies `main`'s `use` set**, and everything else falls out
for free, static propagation refuses what the root never granted, the R39 dynamic check
binds even smuggled closures, and R79's gates keep restricted code out of gated secrets, no
second enforcement mechanism exists or is needed. The CLI/embedding API is undecided and
deferred; resource limits (CPU, memory, spawn) are the genuinely separate missing piece.

**The same check has a third application (R79): gated secrets.** A `secret` value carries a
**gate set** of capability typeids (secret §3.1), and `reveal` runs requirement-set ⊆
frame-grant against it at the effect site, so per-secret authority (`use (dbCred)` opens the
database password, and only it) costs zero new machinery: one field on the value, the one
check redirected. The laundering theorem covers it verbatim, possession of a gated secret is
possession of inert data, never of exposure.

The asymmetry with errors (`!`) is deliberate and principled (functions §3, §7): **an
error is data**, it rides the return value into the caller, so it must be carried in the
type or it smuggles; **a capability is authority**, it rides nothing, so it is confined
at the value instead of described in the type. Neither can be laundered, by opposite
mechanisms.

---

## 4. `use` and where capabilities come from

A capability is acquired through **`use`** (functions §2.2), the referential-capture operator,
the same `use` that captures any nocopy value. There is no separate keyword: capability
acquisition *is* referential capture, so it is spelled `use`, not a distinct `require`.

There is **no `caps` namespace and no capability registry**: a capability is an ordinary
`const` declaration (§1) that its defining **module `export`s** like any other binding
(modules spec), `std.io` exports `io`, `std.secret` exports `reveal`, and a `use` clause names
the **imported binding**. Modules are unnamed and resolved at compile time, so the name in a
`use` clause is unambiguous the same way every imported name is; auditing "what can exercise
X" is a search for `use (X)` against the one canonical declaration the import resolves to.

```
const authenticate = fn (s: secret) use (reveal): response !=> {
  let raw = reveal(s);          // permitted: this function holds the reveal capability
  ...
};
```

A `use` clause may name several capabilities (`use (io, exec)`), and capability
*sets* (§7) bundle common groups under one name to keep clauses short.

---

## 5. Propagation and the absence guarantee

Capability requirements **propagate transitively up the call graph**: calling a function that
requires a capability requires the caller to hold it too.

```
const println = fn (s: string) use (io) { ... };        // holds io

const greet = fn () use (io) { println("hi"); };        // must hold io: it calls println

const broken = fn () { println("hi"); };                     // COMPILE ERROR: calls println,
                                                             // which needs io, but declares none
```

A function's capability requirement is `(what it names in its own use)` union `(the
requirements of everything it calls)`. `println` contributes `io` directly; `greet` inherits
`io` by calling `println`; `broken` is rejected because it calls something needing `io`
without holding it.

This transitivity is what makes capability-absence a **deep** guarantee rather than a shallow
one. Because a callee cannot secretly exercise an authority the caller does not hold, "no
`use (io)` in this signature" means "no io happens anywhere in this call tree," not just
"this body does no io." So:

```
// Guaranteed not to reveal s: no use(reveal) anywhere in its call tree.
const forward = fn (s: secret, dest: command): command => attachAuth(dest, s);
```

Auditing "what can exercise authority X" is a search for `use (X)`, which finds every
function *able* to, transitively. The honest limit: a capability governs **reaching** an
authority, not what happens to a value **after** a permitted use. Once a function that holds
`use (reveal)` reveals a secret, it has a plain value the type system no longer tracks.
The capability boundary is the last point the type system can help.

### 5.1 The creation-site check

A function value's capability set is a property of the **value**, fixed once, where the
literal is evaluated: it is the literal's own `use` clause unioned with the requirements
of everything the body calls (§5, computed by the same inference as §6). The compile-time
check anchors there: **a function literal is legal exactly where the enclosing scope
holds every capability in that set.** After creation, no further check ever runs,
because none is needed:

- a **direct call** by declared name is checked against the caller's own set (§5), which
  is the same creation-site rule applied one level up;
- an **indirect call**, through any function-typed slot, needs no check at all, because
  a capability-holding value cannot enter a slot (§3.1), so the called value's set is
  empty by construction. This is what keeps §6's one-pass inference exact rather than
  approximate: an indirect call contributes nothing, and that is the truth, not an
  assumption.

So the whole system is checked at two kinds of place only, creation sites and named
calls, both fully static, with no capability information in any function type and no
runtime capability state anywhere.

---

## 6. No inference: every capability is explicit, always

An earlier design had an `implicit` modifier letting a capability be silently inferred into
`use` clauses. **Removed** (R33), as genuinely useless: the entire value of the capability
system is that effects are *visible at the signature*, and a capability that appears without
being written is exactly the invisibility the system exists to prevent, purchased to save
one `use (io)`. The rule is now uniform: a function exercising a capability **declares it**,
a required-but-undeclared use is a **compile error**, and the audit story (§4) has no
carve-outs.

## 7. The root: `main` holds the ambient capabilities

Capabilities must originate somewhere, or propagation (§5) would regress forever. The origin
is **`main`**: the runtime hands `main` exactly the capabilities its `use` clause names, and
every other function obtains a capability only by receiving it transitively from `main`
downward.

```
main = fn () use (io, argv) { ... };
```

Here `io` is a capability the runtime grants; `argv` is nocopy immutable data (the program's
arguments), reached by `use` because it is nocopy, but **not** a capability (§3).

The consequence is powerful: **`main`'s `use` clause is a complete, machine-checked upper
bound on the whole program's authority.** If `main` names only `io`, the program can do
io and nothing else, no network, no process spawning, no ffi, no secret revelation, because
those capabilities were never handed in at the root and cannot be forged (nocopy) or conjured
(only the runtime holds the instances). A reviewer reads the program's entire permission
surface off one line, and the type system guarantees it cannot lie.

### 7.1 Capability sets

Threading many capabilities is eased by **capability sets**: a named bundle of capabilities
usable in a `use` clause as one name, so common groups need not be listed individually. A set
is an ergonomic grouping of exported capabilities; it changes nothing about the guarantees
(using a set still requires and propagates each member). A set is declared with the
**braced form** of the capability declaration (§1):

```
const webApp = capability { io, net, fs };   // grants io, net, and fs together
const query  = fn (sql: string) use (webApp): rows !=> { ... };
// use (webApp) requires and propagates io, net, and fs, exactly as if all three were listed
```

The braces list **members**, other capabilities, never data, so a set is still zero-data and
inert (§1); it only names a group. Membership is a **set**, not a union: holding the set is
holding *all* its members, and there is no "one-of" reading to consider.

A set's **comptime** status is **not derived**; like any capability it is non-comptime unless
declared `comptime` (§1, §8). A `comptime capability { ... }` set additionally requires **every
member to be a `comptime` capability**, so a set that lists a non-comptime member (e.g. `io`) can
never be `comptime`, listing one under `comptime` is a **compile error**, decidable at the
declaration. This is the whole of the anti-smuggling guarantee for sets: a comptime set provably
grants only comptime authority, so it cannot be a path to a non-comptime effect at compile time.

### 7.2 User-declared capabilities

Capabilities are not reserved to the standard library. **Application code may declare its own
capabilities** to draw its own authority boundaries, using the same form and getting the same
guarantees:

```
const dbAccess = capability;

const query = fn (sql: string) use (dbAccess): rows !=> { ... };   // only holders may query
```

Now "which parts of the app can touch the database" is a checkable property: a function
without `use (dbAccess)` provably does not query (§5), enforced by the type system and
auditable by searching for `use (dbAccess)`. The same pattern gates a library's dangerous
operations behind a capability its callers must opt into, or hands a `pluginCap` only to
trusted plugins.

This is safe, and the reason is the crux of the whole model: **declaring a capability grants
no authority.** A capability is a zero-data, inert boundary token; it *does* nothing on its
own. Real authority (io, process spawning, secret revelation) comes from the runtime-held
instances handed to `main` (§7), which a user declaration never touches. So a user capability
can only *gate* functions, not *empower* them. And because every capability is a distinct type
(§1), a user capability cannot impersonate or breach a standard one: `use (reveal)` still
demands the real reveal type, which only the runtime hands out. Declaring `dbAccess` gives you
no more access to anything than you already had; it only lets you *require* it.

**Rooting and scope.** A user capability is rooted where it is declared: whoever declares it
holds its instance and threads it to the functions that should be inside the boundary, exactly
as the runtime roots the stdlib capabilities at `main`. A capability declared inside `main` (or
any scope) is reachable within that binding's scope and no further, so it gates the region its
author controls and cannot leak outside it. Stdlib capabilities are simply the ones the
**runtime** roots (because they gate runtime-provided powers); user capabilities are rooted by
user code and gate user-defined boundaries. Same mechanism, different root.

---

## 8. Comptime, and the `unsafe` convention

**Comptime** code cannot hold any **non-comptime** capability, because comptime-eligibility
forbids using one (functions §5.5) and a capability is reachable only through `use`. So comptime
is free of outside authority by construction: it can compute, and hold `comptime` capabilities
(zero-data tags that authorize only comptime-safe operations, §1, §7.1), but it cannot reach the
network, spawn a process, reveal a secret, or call foreign code, all of which are gated by
**non-comptime** capabilities. This extends automatically to every such capability, including
ones not yet defined: **non-comptime is the default**, so any new `capability` is
comptime-unreachable the moment it exists unless it is deliberately declared `comptime`, and that
opt-in is checked to compose only other `comptime` capabilities, so it can never be a path to an
outside effect. The failsafe direction is preserved, forgetting the modifier leaves an authority
*out* of comptime, never smuggled in.

With capability sets living on values (§5.1), the sandbox also holds **by existence**, not
only by rule: capability instances are minted by the runtime at `main` (§7), so at comptime
**no non-comptime capability instance exists yet**. There is nothing for a comptime-created
closure's creation-site check to bind against, so every function value reachable at comptime
is capability-free by construction, including through any function-typed parameter of a
comptime function, since a slot could not hold a capability-holding value even at runtime
(§3.1) and runtime values do not exist yet besides. Higher-order comptime code therefore
needs no capability reasoning at all: the eligibility rule above and this existence argument
reach the same verdict independently, a belt over a brace.

The **`unsafe` convention** (functions §5.6): a capability whose use *suspends* Luna's
guarantees (by reaching untrusted native code) is marked with an `unsafe` prefix
(`unsafeFfi`, `unsafeSystem`). The prefix is a naming convention that flags danger;
mechanically these are ordinary `capability` declarations, nocopy, `use`-gated, and explicit
(never `implicit`), so they are comptime-safe and always source-visible. The prefix warns; it
adds no separate mechanism.

---

## 9. Known capabilities

| Capability | Authority | Spec |
|-|-|-|
| `io` | Input/output (files, streams, console) | std.io |
| `exec` | Spawn and run a structured `command` | exec |
| `reveal` | Reveal a `secret`'s payload | secret |
| `env` | Enumerate environment variables (`envVars()`, values are `secret`, reading them needs `reveal`) | exec §6 |
| `argv` | Read the program's arguments (`args() use (argv): list`) | (module home flagged with std.system) |
| `system` | Safe syscalls (clock, `getpid`, `stat`, ...) | (system, deferred) |
| `unsafeFfi` | Call foreign (native) code | (ffi, deferred) |
| `unsafeSystem` | Dangerous syscalls, shell-string execution | (unsafeSystem, deferred) |

(The tier column is gone with the `implicit` modifier, R33: every capability is explicit.)
`argv` **is** a capability (ruled, R43, reversing the earlier call): program arguments are
program *input*, and there is no reason an arbitrary function, or an arbitrary dependency,
should read them unbidden; the surface mirrors `env`'s, one gated function returning the
data, and `main` reads arguments by declaring `use (argv)` like any other authority. The set will grow; each new authority is a `capability` (explicit unless deliberately
`implicit`), `use`-gated, comptime-unreachable, and auditable by the absence of its `use`
clause.

---

## 10. Rulings (were open questions)

All five ruled or resolved (R43):

- **`argv` is a capability** (reversing the earlier data call): program input is authority
  like any other, arbitrary code and arbitrary dependencies have no business reading it
  unbidden. Surface mirrors `env`'s: `args() use (argv): list` (§9); `main` declares
  `use (argv)`. `env` likewise sits in the table (§9, exec §6).
- **Capability set declaration: already defined** (§7.1 as it stands); nothing further
  pending.
- **`implicit`: gone** (R33); the generalization question died with the modifier.
- **Capability-set polymorphism: resolved**, and re-grounded, the old resolution cited the
  repealed second-class rule; the standing one is R39's: no polymorphism annotation can be
  needed because the requirement set never appears in a *type*, it rides the **value** and
  is checked at the call against the ambient grant, so a higher-order function is
  transparently "polymorphic" over authorities it never sees.
- **Comptime-eligibility has left the typeid** (functions §3, R43): eligibility is a
  *derived value fact*, requirement set empty, over the same one-word set R39 installed, so
  the function typeid is signature plus errorability and nothing else. The **standing audit
  assumption** this rests on, recorded rather than hidden: every ineligibility source is a
  capability (functions §5.5); candidates to keep honest as std grows, allocation limits,
  ungated nondeterminism, the `unsafe` conventions.
