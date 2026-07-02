# Capabilities

A **capability** is an unforgeable token of authority: the right to perform an
outside-reaching or guarantee-affecting operation. Reaching the network, spawning a process,
revealing a secret, and calling foreign code are all gated by capabilities. A capability
carries **no data**; its value is nothing and its *type identity* is the authority. Holding
one means "you may do this"; not holding one means, checkably, that you cannot.

Capabilities are the mechanism behind several guarantees already relied on elsewhere: they
sandbox comptime (comptime cannot hold a capability, functions §5.5), they make effects
auditable (a function's `use` clause is its capability manifest), and they let the *absence*
of a capability be a guarantee (a function without `use (reveal)` cannot reveal a secret,
secret spec).

---

## 1. Declaring a capability

A capability is declared with the **`capability` declaration form**, bound to a `const`, the
same way a protocol or error type is declared (`proto {...}`, `error {...}`):

```
const reveal = capability;
const io     = capability;
const exec   = capability;
```

- **No body, no braces.** Unlike `proto {}` and `error {}`, whose braces delimit a body
  (meta functions, fields), a capability has **no body and can never have one**: it is
  zero-data by form. So it is written bare, `const reveal = capability`, with no `{}`. The
  absence of braces says "there is nothing to configure here," and `capability { ... }` with
  any content is a syntax error.
- **Each declaration is a distinct type.** `const reveal = capability` declares `reveal` as
  a **distinct capability type**, just as `myError = error {}` declares a distinct error
  type. `reveal` and `io` are *different types*, which is what lets `use (reveal)` demand the
  reveal authority specifically and nothing else.
- **`const`, and runtime-minted.** The binding is `const` (a capability is not rebound), and
  the runtime provides its single instance. User code does not construct capability values;
  it receives them by `use` (§3).

---

## 2. `capability` is the union of all capabilities

`capability` is itself a type: the **union of all declared capability types**, the natural
"any capability," exactly as `any` is the natural union of all types. A specific capability
is a member of it:

```
reveal <: capability          // a specific capability is-a capability
io     <: capability
```

So `capability` is a **supertype** of every specific capability, the same relationship
`error` has to each error type. A function that accepts any capability can be typed over
`capability`; a function that needs a specific authority names that capability (`reveal`).

This is a declaration form and supertype in the same family as `proto` and `error` (types
spec): a dedicated form that declares distinct member types, with a union supertype over
them. It is **not** a "kind" in the type-theory sense; Luna has no kind system.

---

## 3. `use`: holding a capability

A capability is **nocopy** and is reached only through **`use`** (functions §2.2), the
referential-capture operator. It cannot be passed by value:

```
const authenticate = fn (s: secret) use (reveal): !response => {
  let raw = reveal(s);          // permitted: this function holds the reveal capability
  ...
};
```

- A function that exercises an authority declares the capability in its **`use` clause**, so
  the authority is visible in the signature. `use (reveal)` means "this function may reveal
  secrets."
- Because capabilities are nocopy, a function **cannot** take a capability as an ordinary
  by-value parameter, store it in a table, and hand it to other code. The only way to hold a
  capability is to `use` it. This is what makes a capability **unforgeable and
  unlaunderable**: there is no value-level copy to smuggle out of a `use` scope.
- Capability requirements ride in the function **type**. Aliasing a capability-using
  function (`const r = reveal`) does not escape the requirement: `r` has `reveal`'s type,
  including its `use (reveal)`, so calling `r` still needs the capability. The requirement
  cannot be laundered by renaming.

---

## 4. The absence of a capability is a guarantee

Because a function can only exercise an authority it holds via `use`, the **absence** of a
capability in a signature is a checkable guarantee that the authority is not exercised:

```
// Guaranteed not to reveal s: no use(reveal) in the signature, so reveal is unreachable here.
const forward = fn (s: secret, dest: command): command => attachAuth(dest, s);
```

`forward` holds a `secret` but cannot read it, because it does not hold the `reveal`
capability, and a capability cannot be obtained except by declaring `use`. So "this function
does not reveal secrets" is read off its type (no `use (reveal)`), not trusted. The same
applies to every capability: a function without `use (io)` performs no I/O, without
`use (exec)` runs no process, and so on.

This composes: a subsystem whose entry points declare no `use (io)` performs no I/O anywhere
within it, because the capability would have to be threaded in through those entry points.
Auditing "what can exercise authority X" is a search for `use (X)`, which finds every
function *able* to, a stronger statement than finding call sites.

The honest limit: a capability governs **reaching** an authority, not what happens to a value
**after** a permitted use. A function that legitimately holds `use (reveal)` and reveals a
secret then has a plain value the type system no longer tracks. The capability boundary is
the last point at which the type system can help; past a permitted use, the result is
ordinary data.

---

## 5. Comptime and capabilities

Comptime code **cannot hold any capability**, because comptime forbids `use` (functions
§5.5) and a capability is reachable only through `use`. So comptime is capability-free by
construction: it can compute, but cannot reach the network, spawn a process, reveal a
secret, or call foreign code. This is the whole basis of comptime's security sandbox, and it
extends automatically to every capability, including ones not yet defined: any new authority
declared as a `capability` is comptime-unreachable the moment it exists, with no
comptime-specific rule.

---

## 6. The `unsafe-` convention

Most capabilities are effects **within** Luna's guarantees (`io`, `exec`, `system`): their
implementations respect the memory, type, and error models. A capability whose use
**suspends** those guarantees, because it reaches untrusted native code, is marked with an
**`unsafe-` prefix** (functions §5.6): `unsafe-ffi`, `unsafe-system`. The prefix is a naming
convention that flags danger; mechanically these are ordinary `capability` declarations,
nocopy and `use`-gated like any other, and therefore comptime-safe by the same invariant
(§5). The prefix warns; it adds no separate mechanism.

---

## 7. Known capabilities

The capabilities referenced across the specs so far:

| Capability | Authority | Spec |
|-|-|-|
| `io` | Input/output (files, streams, console) | (io / stdlib, deferred) |
| `exec` | Spawn and run a structured `command` | exec |
| `reveal` | Reveal a `secret`'s payload | secret |
| `system` | Safe syscalls (clock, `getpid`, `stat`, ...) | (system, deferred) |
| `unsafe-ffi` | Call foreign (native) code | (ffi, deferred) |
| `unsafe-system` | Dangerous syscalls, shell-string execution | (unsafe-system, deferred) |

This set will grow; each new authority is a `capability` declaration, gated by `use`,
comptime-unreachable, and auditable by the absence of its `use` clause.

---

## 8. Open questions

- **Capability granularity:** whether a capability can be scoped to a specific resource (this
  file, this secret) rather than a whole class of authority, which trends toward
  information-flow typing and is likely out of scope; the current model is one capability per
  class of authority.
- **Declaring capabilities in user code:** whether application code may declare its own
  capabilities (`const myCap = capability`) for its own authority boundaries, or whether the
  form is reserved to the standard library, pending the module system.
- **Capability sets in types:** whether a function type can be generic over "some set of
  capabilities" for higher-order code that forwards authority, versus naming each capability
  explicitly, pending experience with capability-passing patterns.
