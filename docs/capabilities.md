# Capabilities

A **capability** is an unforgeable token of authority: the right to perform an
outside-reaching or guarantee-affecting operation. Reaching the network, spawning a process,
revealing a secret, and calling foreign code are all gated by capabilities. A capability
carries **no data**; its value is nothing and its *type identity* is the authority. Holding
one means "you may do this"; not holding one means, checkably, that you cannot.

Capabilities underpin several guarantees relied on elsewhere: they sandbox comptime (comptime
cannot hold one, functions §5.5), they make effects auditable (a function's `use` clause is
its capability manifest), and they let the *absence* of a capability be a guarantee (a
function without `use (caps.reveal)` cannot reveal a secret, secret spec).

---

## 1. Declaring a capability

A capability is declared with the **`capability` declaration form**, bound to a `const`, the
same way a protocol or error type is declared (`proto {...}`, `error {...}`):

```
const reveal = capability;
const exec   = capability;
const io     = implicit capability;      // opt into silent inference (§6)
```

- **No body, no braces.** Unlike `proto {}` and `error {}`, whose braces delimit a body, a
  capability has **no body and can never have one**: it is zero-data by form. So it is
  written bare, with no `{}`, and `capability { ... }` with any content is a syntax error.
- **Each declaration is a distinct type.** `const reveal = capability` declares `reveal` as a
  **distinct capability type**, just as `myError = error {}` declares a distinct error type.
  `reveal` and `io` are *different types*, which is what lets `use (caps.reveal)` demand the
  reveal authority specifically and nothing else.
- **The `implicit` modifier** marks a capability as silently inferable (§6). Its absence is
  the default: a plain `capability` must be declared wherever it is required.
- **Always `const`.** A capability binding is always `const`, never `var` or `let`. Nothing
  else makes sense: a capability is a fixed, unforgeable token of authority, so rebinding it
  (`var`) or leaving it reassignable serves no purpose and would only invite confusion.
  Declaring a capability with anything but `const` is an error. The runtime provides the single
  instance for stdlib capabilities; a user capability's instance is minted where it is declared
  (§7.2).

Capabilities live in the **`caps` module** (accessed `caps.reveal`, `caps.io`), a light,
dataless module, so that capability names never collide with ordinary function names. `reveal`
the function (secret spec) and `caps.reveal` the capability coexist because they are in
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
`use` scope. Aliasing a capability-using function does not escape its requirement either, the
requirement rides in the function's type, so a renamed alias still requires the capability.

---

## 4. `use` and the `caps` namespace

A capability is acquired through **`use`** (functions §2.2), the referential-capture operator,
the same `use` that captures any nocopy value. There is no separate keyword: capability
acquisition *is* referential capture, so it is spelled `use`, not a distinct `require`.

```
const authenticate = fn (s: secret) use (caps.reveal): !response => {
  let raw = reveal(s);          // permitted: this function holds the reveal capability
  ...
};
```

A `use` clause may name several capabilities (`use (caps.io, caps.exec)`), and capability
*sets* (§7) bundle common groups under one name to keep clauses short.

---

## 5. Propagation and the absence guarantee

Capability requirements **propagate transitively up the call graph**: calling a function that
requires a capability requires the caller to hold it too.

```
const println = fn (s: string) use (caps.io) { ... };        // holds io

const greet = fn () use (caps.io) { println("hi"); };        // must hold io: it calls println

const broken = fn () { println("hi"); };                     // COMPILE ERROR: calls println,
                                                             // which needs io, but declares none
```

A function's capability requirement is `(what it names in its own use)` union `(the
requirements of everything it calls)`. `println` contributes `io` directly; `greet` inherits
`io` by calling `println`; `broken` is rejected because it calls something needing `io`
without holding it.

This transitivity is what makes capability-absence a **deep** guarantee rather than a shallow
one. Because a callee cannot secretly exercise an authority the caller does not hold, "no
`use (caps.io)` in this signature" means "no io happens anywhere in this call tree," not just
"this body does no io." So:

```
// Guaranteed not to reveal s: no use(caps.reveal) anywhere in its call tree.
const forward = fn (s: secret, dest: command): command => attachAuth(dest, s);
```

Auditing "what can exercise authority X" is a search for `use (caps.X)`, which finds every
function *able* to, transitively. The honest limit: a capability governs **reaching** an
authority, not what happens to a value **after** a permitted use. Once a function that holds
`use (caps.reveal)` reveals a secret, it has a plain value the type system no longer tracks.
The capability boundary is the last point the type system can help.

---

## 6. Inference: explicit by default, `implicit` to opt in

Capability requirements are **inferred**, by the same cheap transitive pass as
comptime-eligibility (functions §5.1): one propagation over the call graph, each function's
requirement computed from its direct `use` clause and its callees' already-computed
requirements, O(N + E), not a re-walk of bodies per call site. So inference costs no more than
the analysis already run for comptime.

What differs per capability is **whether a required-but-undeclared use is an error or is
filled silently.** This is the `explicit` / `implicit` distinction:

- **Explicit (the default, plain `capability`).** If inference finds a function requires the
  capability but its signature does not declare it, that is a **compile error**. The
  capability must appear in the source signature wherever required. So its presence is always
  visible, and the absence guarantee (§5) is readable off the source, not merely computed.
  `reveal`, `exec`, and the `unsafe-` capabilities are explicit.
- **`implicit` (opt in, `implicit capability`).** A required-but-undeclared use is **fine**;
  inference fills it silently. The requirement is still tracked (and tooling can show it), but
  it need not clutter every signature. `io` is `implicit`, because threading `use (caps.io)`
  through every function that logs would be noise for a low-stakes, ubiquitous effect.

Inference runs either way; `implicit` only changes whether a *missing* declaration is an error
or is quietly supplied.

**Explicit is the default on purpose, and the direction is failsafe.** A newly declared
capability, if no one thinks about its tier, is explicit, so forgetting fails toward
*over-disclosure* (an annotation you did not strictly need), which is harmless. If `implicit`
were the default, forgetting would fail toward *under-disclosure* (a security-critical
authority silently inferred and invisible in signatures), which is exactly the auditability
hole. So hiding a capability from signatures must be a deliberate `implicit` opt-in, never the
default, consistent with Luna's stance of opting *into* the sharp thing (`unsafe-`,
backtracking regex, shell strings) rather than defaulting to it.

Tooling shows the full inferred capability set of any function regardless of tier, so even
`implicit` capabilities are auditable on demand; the explicit default makes the critical ones
auditable *always*, in source.

---

## 7. The root: `main` holds the ambient capabilities

Capabilities must originate somewhere, or propagation (§5) would regress forever. The origin
is **`main`**: the runtime hands `main` exactly the capabilities its `use` clause names, and
every other function obtains a capability only by receiving it transitively from `main`
downward.

```
main = fn () use (caps.io, argv) { ... };
```

Here `io` is a capability the runtime grants; `argv` is nocopy immutable data (the program's
arguments), reached by `use` because it is nocopy, but **not** a capability (§3).

The consequence is powerful: **`main`'s `use` clause is a complete, machine-checked upper
bound on the whole program's authority.** If `main` names only `caps.io`, the program can do
io and nothing else, no network, no process spawning, no ffi, no secret revelation, because
those capabilities were never handed in at the root and cannot be forged (nocopy) or conjured
(only the runtime holds the instances). A reviewer reads the program's entire permission
surface off one line, and the type system guarantees it cannot lie.

### 7.1 Capability sets

Threading many capabilities is eased by **capability sets**: a named bundle of capabilities
usable in a `use` clause as one name, so common groups need not be listed individually. A set
is an ergonomic grouping over the `caps` module; it changes nothing about the guarantees
(using a set still requires and propagates each member). The exact declaration form for a set
is deferred with the module system.

### 7.2 User-declared capabilities

Capabilities are not reserved to the standard library. **Application code may declare its own
capabilities** to draw its own authority boundaries, using the same form and getting the same
guarantees:

```
const dbAccess = capability;

const query = fn (sql: string) use (dbAccess): !rows => { ... };   // only holders may query
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
(§1), a user capability cannot impersonate or breach a standard one: `use (caps.reveal)` still
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

## 8. Comptime, and the `unsafe-` convention

**Comptime** code cannot hold any capability, because comptime forbids `use` (functions §5.5)
and a capability is reachable only through `use`. So comptime is capability-free by
construction: it can compute but cannot reach the network, spawn a process, reveal a secret,
or call foreign code. This extends automatically to every capability, including ones not yet
defined; any new `capability` is comptime-unreachable the moment it exists.

The **`unsafe-` convention** (functions §5.6): a capability whose use *suspends* Luna's
guarantees (by reaching untrusted native code) is marked with an `unsafe-` prefix
(`unsafe-ffi`, `unsafe-system`). The prefix is a naming convention that flags danger;
mechanically these are ordinary `capability` declarations, nocopy, `use`-gated, and explicit
(never `implicit`), so they are comptime-safe and always source-visible. The prefix warns; it
adds no separate mechanism.

---

## 9. Known capabilities

| Capability | Tier | Authority | Spec |
|-|-|-|-|
| `io` | implicit | Input/output (files, streams, console) | (io / stdlib, deferred) |
| `exec` | explicit | Spawn and run a structured `command` | exec |
| `reveal` | explicit | Reveal a `secret`'s payload | secret |
| `system` | explicit | Safe syscalls (clock, `getpid`, `stat`, ...) | (system, deferred) |
| `unsafe-ffi` | explicit | Call foreign (native) code | (ffi, deferred) |
| `unsafe-system` | explicit | Dangerous syscalls, shell-string execution | (unsafe-system, deferred) |

`argv` is **not** here: it is nocopy immutable data passed to `main`, not a capability (§3,
§7). The set will grow; each new authority is a `capability` (explicit unless deliberately
`implicit`), `use`-gated, comptime-unreachable, and auditable by the absence of its `use`
clause.

---

## 10. Open questions

- **`argv` and other program inputs:** confirming `argv` (and environment, working directory)
  are nocopy data rather than capabilities, versus treating some as authorities to thread; the
  current call is data for `argv`.
- **Capability set declaration:** the exact form for declaring a capability set (§7.1), pending
  the module system.
- **Reusing `implicit` elsewhere:** the `implicit` modifier (silent-inference opt-in) may
  generalize to other declarations beyond capabilities; its general meaning is left open.
- **Capability-set polymorphism:** whether a higher-order function that forwards authority can
  be generic over "some set of capabilities" rather than naming each, pending experience with
  capability-passing patterns.
