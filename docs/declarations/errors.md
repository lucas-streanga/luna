# Errors

An error in Luna is its own kind of thing. It is **not** a table, not a protocol, and
not an ordinary value with a protocol applied. Errors have a sealed type hierarchy,
single inheritance, and immutable fields, none of which fit the table model, so they are
a distinct category. This document specifies that category: the hierarchy, how errors are
defined and inherited, how they are constructed and thrown, and how they are caught. The
low-level representation (single inheritance as a prefix layout, O(1) subtype tests, the
`typetable`) is in **value-representation**; this document is the language-level
semantics.

---

## 1. Errors are their own kind

An error is a distinct kind, separate from tables and values, because it needs things the
table model does not provide:

- **A sealed, single-inheritance hierarchy** rooted at `error`, with O(1) subtype tests
  (value-representation §4.2). Tables have protocols (a runtime set), not a supertype
  chain.
- **Immutable, declared fields.** An error's fields are set once at construction and read
  thereafter; an error is a snapshot of what went wrong, not a mutable container.
- **Declarability.** Whether a function can raise an error is tracked in its type (`!`,
  §7), which requires errors to be a fixed, statically-known category, not open-ended
  data.

Because errors are their own kind, they are accessed differently from tables:

- **`.field`** reads a declared field. Fields are static (declared in the error type) and
  immutable, so error access is `.name` only.
- There is **no `[]`** (an error is not a hashmap) and **no `->`** (an error has no applied
  protocols). The three-operator table model (tables §3.3) does not apply; an error has
  only its declared fields.

---

## 2. The hierarchy

Every error descends from a single root, `error`, which has two subtrees:

```
error                     (root: catchable; constructable, the throwaway §5.2; the target of `!`; "catch everything")
├── panic                 (sealed: no user inheritance; ambient; undeclarable)
│   ├── outOfMemory
│   ├── typeError
│   ├── ArityError
│   ├── NamedArgumentError (unknown or double-bound named argument; functions §3.3.2)
│   ├── OverflowError     (int arithmetic overflow, incl. INT_MIN / -1; int spec)
│   ├── DivisionByZero    (int division or remainder by zero; int spec)
│   └── ... (runtime-defined)
├── ApplyError
└── ... (user-defined: a definition with no explicit parent extends the root directly, §4)
```

The single root is deliberate: **a bare `catch (e)` catches everything**, because everything
is an `error` and a bare binder inherits the type it is given, here the root (match §2). There is
no separate top type above it (no `Throwable`) that a catch could miss, so the obvious catch is
the complete catch. This is the property the hierarchy exists to guarantee.

The hierarchy has exactly **one distinguished subtree, `panic`**, and everything else,
the root itself and every user-defined type, is the other category by complement. An
error outside the `panic` subtree is a **declarable error**: the term names a
*category*, not a type. There is deliberately no `UserError` node giving the category a
type of its own, because none is needed for disambiguation: any error defined in a
program extends the root (directly or transitively) and is therefore automatically not
a `panic`, and the category test, "is the `typeid` in the `panic` interval," is one
O(1) interval check (value-representation §4.2), used negated. The partition is total by
construction, every error value either is in the `panic` subtree or it is not, so
there is no third category and no orphan. What the category gives up by having no type
is precision at type level: the widest arm and the widest catch are both typed
`error` (a `!` arm, §7; a bare `catch (e)`, §8.2), which statically includes `panic` even in
positions where a panic can never dynamically land. The everyday throwaway error is the base
`error` itself (§5.2).

Two policies attach to the two subtrees, and each is an O(1) subtree test
(value-representation §4.2):

- **Declarability** is governed by the `panic` subtree, by exclusion. A function must be
  declared `fn!` iff it can raise a **declarable** error, any error outside `panic` (§7).
  A `panic` may arise from any function without declaration.
- **Inheritability** is governed by the same split. User error types extend the root (the
  default, §4) or any non-`panic` type; the `panic` subtree is **sealed**, no user type
  may inherit from `panic` or any of its descendants (§9). The runtime owns that subtree.

So "must this be declared" and "may a user extend this" both reduce to "which subtree,"
answered by the interval test on the statically-known hierarchy.

### 2.1 The root `error` shape

The root `error` is a real, constructable type (it is the throwaway, §5.2) with a fixed
shape that every error inherits as its prefix (value-representation §4.2), so these
fields are present on every error and readable on any `error`-typed value without
narrowing:

```
error {
  message: string;        // human-readable description; "" if none
  stacktrace: secret;     // the trace: a secret-wrapped table of frames (R111); written only by throw (§6.1)
  cause: error?;          // the underlying error when wrapping (§6.2); null otherwise
  data: table?;           // arbitrary ad-hoc fields for throwaway errors (§5.2); null otherwise
}
```

- **`message`** is the description, empty string when unspecified.
- **`stacktrace`** is a **secret-wrapped table** of frames (R111, secret §6): only the
  `throw` operator writes it (§6.1), and reading its *contents* is a reveal —
  `revealTable(e.stacktrace)`, gated (by the default `[@reveal]` set; whether traces
  deserve a dedicated capability is open, §10). User-facing display therefore **redacts
  traces by construction** (secret §4) — no internal paths in user-visible output — while
  the runtime's crash reporter reveals at its boundary, the secret §5.1 pattern. The one
  bit that is *not* secret is whether the error was thrown:
  **`fn wasThrown(e: error): bool`** — the unforgeable test the old
  `stacktrace.isEmpty()` idiom provided, now a dedicated predicate disclosing no
  contents and needing no capability.
- **`cause`** links a wrapping error to the error it wraps (§6.2), `null` when there is no
  cause.
- **`data`** carries arbitrary keyed data for throwaway errors that want structure without
  a declared type (§5.2), `null` when unused.

Because these are the prefix of every error, `e.message`, `e.stacktrace`, `e.cause`, and
`e.data` are valid on any `error` value, including the error arm of a `try` (§8) and a
block-caught `error` (§8.2), with no downcast.

### 2.2 Equality: the identity surface, and `toTable`

Errors are **value-equality** types (equality §5, R110): `a == b` iff the two are the
**same error type** (nominal, no erasure — the enum-variant rule, equality §1) and their
**identity surfaces** deep-match by `==`. The identity surface is what the author put
there — `message`, the declared fields, `data`, and `cause` (compared recursively, each
level by its own surface) — and **never `stacktrace`**: the trace is runtime-attached
provenance, written by `throw` (§6.1), recording *where* the error happened rather than
*what* it is. Two errors of the same type with the same fields are equal however far
apart they were thrown, which is exactly what tests (`r == expectedError`) and
deduplication need. This is the surface principle's third instance (protocols' granted
members, R96; tables' element space): **equality compares what the author declared;
what the runtime attaches is not identity.**

```
fn toTable(e: error): table     // total: the identity surface, reified
```

**`toTable`** converts an error to a table of its identity surface — `message`, the
declared fields, `data`, and `cause` (as an error value) — excluding `stacktrace`
(holdable as `e.stacktrace`, its contents revealed only under its gate, R111). It is total (`to*` contract, conversion §2),
it is *the* definition of the surface (`a == b` ⇔ same typeid ∧
`toTable(a) == toTable(b)`), and it is the **shape-matching bridge**: error structure is
matched through ordinary table patterns, `match (e.toTable()) { ['code' => 11] => … }`,
while the *type* axis stays with the type system (typed binders `p: parseError`, `is`
for subtree membership, `@` for reflection).

One consequence to know: a declared field of type `secret` makes its error **never
equal**, including to itself — secret contagion, the same rule as a table holding a
secret and the same family as nan (equality §5). Don't put secrets in fields you intend
to compare; match on the others. (The `stacktrace` being a secret, R111, adds **no**
contagion: it sits outside the identity surface — the R110 exclusion and the R111
wrapping interlock exactly.)

---

## 3. Defining an error

An error type is defined with the `error` block and bound to a variable, the same way a
protocol is defined with `proto`:

```
myError = error {
  code?: int;
  detail?: string;
};
```

`error` is both the definition keyword (`myError = error { ... }`) and the name of the
root type (`| error`, `catch (_: error)`), exactly as `proto` is both keyword and type. An
error definition binds like any value; a module exports an error type by exporting its
variable.

A definition with no explicit parent **implicitly extends the root `error`**, so
`myError` above is a declarable error by construction (it is not under `panic`, §2). Its
body declares **fields**: typed, optionally `?` (which makes
the field nullable and omissible at construction, defaulting to `null`). Fields are
immutable after construction (§5).

An error type carries no meta functions, no protocols, and no dynamic members; it is a
declared shape plus a position in the hierarchy. Behavior that operates on an error is an
ordinary function (reachable by UFCS, `message(err)`), not a method on the error.

---

## 4. Inheritance

An error extends another with `error : Parent`:

```
diskError = error : myError {
  path?: string;
};
```

- **Single inheritance** (value-representation §4.2): exactly one parent. `diskError`
  extends `myError`, which extends `error`.
- **Default parent is the root `error`**: omitting `: Parent` extends `error` directly
  (§3).
- **The `panic` subtree is sealed**: `error : panic` or extending any `panic` descendant
  is a **compile error**. Everything a user defines is therefore outside `panic`, i.e.
  declarable (§2), automatically.
- **Fields inherit as a prefix** (value-representation §4.2): a child's fields are laid
  out after its parent's, so `diskError` has `code`, `detail` (from `myError`), and
  `path`. Upcast to `myError` or `error` is a no-op; the `typeid` discriminates for
  narrowing.
- **Subtype tests are O(1)**: `diskError <: myError`, `catch (e: myError)`, and `is myError`
  are two integer comparisons (the preorder-interval test), not pointer chasing.

The hierarchy is fixed at compile time. Throwing an error never creates a new type
(value-representation §4): every error type, including every subtype, exists in the
`typetable` before the program runs.

An error whose fields are all optional may be constructed with no arguments:

```
myError = error : someOtherError {
  newField?: string;
};

throw myError();        // all fields optional (own and inherited), so no arguments needed
```

---

## 5. Construction

A **declarable** error is constructed by naming its type and supplying its fields.
**`panic` types are not constructable in user code** (§9): the runtime mints every
`panic` value at the failing operation, so `OverflowError(...)` or `typeError(...)` in
source is a compile error. Construction below therefore always means a declarable
error:

```
let e = myError(code: 11);              // named fields
let f = myError(11, "disk full");       // positional, in declared order
let g = diskError(code: 5, path: "/x"); // inherited and own fields together
```

Fields marked `?` may be omitted and default to `null`. Construction is first-class
(value-representation §3.1): an error value may be built, bound, stored, and passed like
any value, `let e: myError = myError(11)`, entirely apart from throwing. Constructing an
error does not throw it; the two are separate steps (§6).

An error is **immutable after construction**: `e.code` reads, there is no `e.code = x`.

### 5.1 There is no `this`; construct by fields, compute in factories

An error definition has no constructor method and no `this`. Setting fields *is*
construction: `myError(code: 11)` assigns `code`, with no code to write. A hand-written
initializer that only copies arguments into fields, such as a `fn (code) { this.code =
code }`, is exactly what field construction already does, so it is unnecessary, and
`this` (an implicit receiver) does not exist in Luna: receivers are always explicit or
absent (functions and protocols spec), and errors carry no methods to need one.

When construction needs **logic**, validation, or derived fields, that logic goes in an
ordinary **factory function** that returns the error, not in the error type:

```
const makeDiskError = fn (path: string): diskError => {
  return diskError(code: 5, detail: "disk full", path: path);
};
```

This keeps errors as pure typed carriers (data plus a hierarchy position) and puts all
behavior where behavior lives in Luna, in functions.

### 5.2 Throwaway errors: base `error`, `message`, and `data`

You do not need to declare an error type to fail. The base `error` is itself the
throwaway error, constructed directly:

```
throw error('disk full');       // base error with a message
throw error;                    // base error, empty message (sugar for error(''))
```

`error('msg')` is `error(message: 'msg')`. Bare `throw error` (no parens) is the
empty-message form. This is the common case, failing with a message, and it needs no type
declaration, so the boilerplate of minting a type per failure is avoided.

The throwaway is **declarable** and **`try`-catchable** like any user error, because both
policies key on "outside the `panic` subtree" (§2), and the root is outside its own
`panic` child. So `throw error('msg')` requires the enclosing function to be `fn!` (§7)
and is caught by a `try` expression (§8.1), exactly as a declared error type would be.

When a throwaway error wants **structured** data, that data goes in the `data` table
(§2.1), not into a new type:

```
throw error('bad config', [key => 'timeout', value => -1]);
// error('msg', dataTable) sets message and data
```

A handler reads it off the base error:

```
} catch (e) {
  let k = e.data?.key ?? "unknown";
}
```

**There are no anonymous error types.** A `throw [k => v]` does *not* mint an implicit
type: that would create a type at a throw site (which the type model forbids,
value-representation §4) and, worse, an unnameable type no `catch` clause could select,
so it could only ever be caught as base `error` anyway. Ad-hoc keyed data is what
tables are for, so it rides in `data` on the base `error`, which is already catchable
and already the throwaway type.

The rule for when to declare a type versus use base `error` with `data`: **declare an
error type exactly when a handler will catch it by name** (`catch (e: diskError)`) and branch
on that type. If handlers will only ever catch broadly and inspect fields, there is
nothing for a distinct type to do, so use base `error` with `message` and `data`.

---

## 6. Throwing

`throw` raises an error value, unwinding until it is caught:

```
throw myError(11);                      // construct, then throw
```

`throw` takes an already-constructed error value. Because user code cannot construct a
`panic` (§5, §9), everything user code can *originate* is declarable, so **every
originating `throw` requires the enclosing function to be `fn!`** (§7), with no
per-throw category test. The one `throw` that does *not* require `fn!` is the
**re-throw of a `panic`-typed expression** (possible only inside a `catch` that
received one, the sole way user code ever holds a `panic`-typed value): that is
propagation of an ambient failure already in flight, and panics are undeclarable (§9).
A `throw` whose expression is root-`error`-typed is treated as origination (it requires
`fn!`), even if the value is dynamically a panic; the narrowed `catch (p: panic) { throw
p; }` is the `!`-free relay spelling (§8.3). Throwing a **non-error** value is a
compile error; only `error`-typed values are throwable.

### 6.1 `throw` populates the secret stacktrace

Writing the `stacktrace` field is a privileged effect of the `throw` operator. The field
is a **secret** (R111, §2.1): user code may hold it, but neither writes it nor reads its
contents without the gate, and there is no user-reachable name that grants the write —
`throw` is an operator with a built-in capability, not a function that can be aliased.
This is what makes the trace state unforgeable.

`throw` writes the trace by checking whether the error has been thrown:

- **Never thrown (`!wasThrown(e)`)** , `throw` **captures** the current call stack into
  it. This is the origin: the full stack where the error first failed.
- **Already thrown** , `throw` **appends the single current throw site** as one frame.
  This records a propagation hop without re-dumping the whole stack.

So the trace accumulates the path the error travelled: a full stack at the origin, then a
breadcrumb for each place it was re-thrown. Because `throw` always at least captures its
own site and no other writer exists, thrown-ness is reliable, and `wasThrown(e)` reports
it (§2.1) without revealing contents.

### 6.2 Re-throwing and wrapping

There is **no dedicated re-throw syntax**. Because a caught error is an ordinary value
(via `try`, §8), propagating it conditionally is ordinary control flow:

```
let r = try someFunc();
throw r if (r is error);        // re-raise; appends this site to r's existing trace
// ... otherwise use r as the success value
```

Re-throwing a caught error with `throw` appends the current site to its existing trace
(§6.1), preserving the origin, so no special construct is needed to keep the original
failure location.

**Wrapping** raises a new, higher-level error that carries the original as its `cause`:

```
let r = try lowLevel();
throw someError(message: 'init failed', cause: r) if (r is error);
```

The wrapping error is freshly constructed, so its own `stacktrace` is empty and `throw`
captures a fresh origin for it; the wrapped error sits in `cause` with its accumulated
trace untouched (it is stored, not thrown). So the two mechanisms compose cleanly:
**`stacktrace` records one error's journey; `cause` chains distinct errors.** A handler
can walk `e.cause` to inspect the underlying failure.

---

## 7. Declarability: `!` and `fn!`

Errorability is declared with `!`, the error-analogue of `?` (value-representation §3.1).
`!` on a result type means "or error": `string!` is `string | error`, and the two
capacities compose freely (`string?!` is `string | null | error`). The arm is the root
`error`, because the declarable category has no type of its own (§2): what *lands* in the
arm is always a declarable error, a `panic` never becomes a value through `!` or `try`
(it unwinds, §8.1), but the type spelling the arm is the root, which statically includes
`panic`. The imprecision is accepted and one-directional: `try` can never seat a panic in
the arm, though a program may *manually* store a block-caught panic (§8.2) into an
`error`-typed location, and an `is panic` test on a `try` arm is legal but always false.

A function's type records whether it can raise a declarable error, and it does so **by
declaration, not inference** (functions spec §4, never spec §5):

- A function is **`fn!`** iff its signature **declares** `!`. The `!` is **required, and
  verified**, whenever the body can raise a **declarable** error (§2), by a direct `throw` (every
  originating `throw` qualifies, §6; only a `panic`-typed re-throw does not) or by an
  **unhandled** call to an `fn!` function (an `fn!` call not lexically enclosed in a `try` /
  `catch` or resolved by an errorable binding). The compiler **checks** that a declared `!` is
  justified and that a missing `!` is safe; it never **adds** `!` for you. A function that can
  throw but omits `!` is a **compile error**, not a silent promotion to `fn!`.
- A function is **`fn`** (not `!`) iff it declares no `!`, which the compiler admits only when
  **no declarable error can escape it**, every throwing call is handled. This is a real guarantee: a
  caller of a non-`!` function need not handle any declared error, because none can emerge.

The check is the **local, syntactic containment check** of functions spec §4 (per call site,
lexical, not control-flow analysis), not a call-graph propagation: "can `g` throw?" is read off
`g`'s **signature**, so errorability is never computed transitively.

**Panics are ambient and undeclarable.** Any function may raise a `panic` (an `outOfMemory`,
a `typeError`, an `ArityError`) without being `fn!`. So `fn` guarantees "no declarable
error escapes," not "cannot fail": a non-`!` function may still panic. This is what keeps `!`
meaningful (it tracks the failures you can locally anticipate and handle) without forcing
every allocating or calling function to be `!` for failures nobody can locally prevent
(§9).

This is the same `fn` / `fn!` distinction the function spec defines, and the same `self!`
a protocol's `apply` carries when it can throw (protocols §7.5).

---

## 8. Catching

There are two ways to handle a thrown error, and they split along the two error categories, which is
the central design point of the whole error model:

- The **`try` expression** (§8.1) catches **declarable errors only** (everything outside the
  `panic` subtree, §2), expected failures, collapsing them to a value inline. Panics unwind
  through it.
- The **`try`/`catch` block** (§8.2) catches **everything**, declarable errors and `panic`, the
  deliberate boundary where you stop even the exceptional.

The distinction is visible at a glance (a block has a `catch`, an expression does not) and it maps
exactly onto the declarable/`panic` split: routine, inline handling of expected errors versus a
boundary that catches all. This is what keeps the model safe *and* ergonomic: expected errors are
handled where they arise, invariant violations propagate to a real boundary, and the syntax for the
first can never silently absorb the second.

### 8.1 The `try` expression catches declarable errors only

`try expr` runs `expr` and, if it throws a **declarable error** (any error outside the `panic`
subtree, §2), yields that error as a value instead of unwinding:

```
let v!: string = try someFunc();          // v : string | error
let w:  string | error = try someFunc();  // identical; ! is sugar for the error arm
```

The `try` **expression** catches everything **outside the `panic` subtree**, one negated O(1)
interval test (§2). A **`panic`** is **not** caught
by a `try` expression: it **unwinds through** it, propagating up to a `try`/`catch` block (§8.2) or,
if none, to the task or program boundary. This is deliberate and is the heart of the two-category
design:

- A declarable error is an **expected** outcome, part of a function's normal control-flow contract
  (it is what `!` declares). Handling it inline with `try` is routine, so `try` collapses it to a value.
- A `panic` is an **invariant violation**, a bug or exhausted resource, not an expected outcome
  (which is exactly why it is undeclared, §9). It must **not** be silently absorbable by the syntax
  used to handle expected errors: a programmer writing `_ = try cleanup()` to ignore an *expected*
  failure must not thereby swallow an overflow or out-of-memory. So a panic ignores `try` entirely
  and keeps unwinding to a real boundary, where a supervisor can decide to crash, restart, or fail
  the unit of work.

Letting `try` catch panics would defeat the reason `panic` exists: a panic's job is to propagate a
violated invariant to a level that can act on it (or to crash loudly), which is what makes
"assume-no-overflow" safe in the first place (§9). Absorbing it at an unrelated `try` site would make
the bug invisible, the smuggled-error failure mode, at the panic layer. So the `try` expression
respects the category split: expected errors in, invariant violations through.

The expression's error arm is spelled with the **root `error`**, the widest error type, because the
declarable category it actually catches has no type of its own (§2); what lands there is always
declarable, a panic never does (it unwinds), the static `panic` inclusion is the accepted
imprecision of §7. Binding the result into a type that cannot hold the error (no `!`, no `| error`)
is a compile error, exactly as assigning `null` to a non-`?` type is: the error must be handled.
The result is a union (`string | error`), so the value cannot
be used as a plain `string` until the error arm is narrowed away (§8.3), which is what makes
swallowing type-impossible: you cannot proceed as if the call succeeded without first dealing with
the error case.

Discarding the result entirely still requires the explicit no-discard form `_ = try someFunc()`
(variables spec: a return value may not be silently dropped). That discard is visible and greppable,
and it discards only a declarable error (a panic would still unwind through), so it is an honest,
deliberate, bounded act, never a silent or accidental swallow.

A **void** call is the one exception to the no-discard rule, and it connects to how absence works
across the language. Luna has no `void`: a function that returns no meaningful value returns
**`undefined`** (undefined spec), the absence sentinel, not `null`. Because there is genuinely
nothing to discard, a void call needs no `_ =` (`log("hi");` is fine), and the compiler proves this
statically from the `undefined` return type. This is the same `undefined` that a missing key
produces: storable but panics on use, so *using* a void function's result (`const l = log("hi");
use(l)`) panics, which is correct, there was no result to use. The through-line with the error model:
absence (`undefined`) and failure (`error`) are separate channels, just as `null` (a chosen value)
and `undefined` (a reported absence) are separate, and none of them is allowed to masquerade as
another.

### 8.2 The `try` / `catch` block catches everything

```
try {
  ...
} catch (e) {
  // e is the caught error, typed root `error`; may be a declarable error or a panic
}
```

The `try`/`catch` **block** is the **catch-all**: a bare `catch (e)` catches the **root `error`**, so
it catches **both categories**, declarable errors and `panic`. This is the intended boundary form, and it honors the
universal intuition that a `try`/`catch` block catches everything. The two catch forms therefore
split cleanly by the two error categories, and the split is visible at a glance (one has a `catch`
block, one does not):

- **`try expr`** (expression, no block), catches **declarable errors only**; panics unwind through.
  The ergonomic, inline handling of expected failures (§8.1).
- **`try { } catch { }`** (block), catches **everything** (declarable and `panic`). The deliberate
  **boundary**: you opened a `catch`, so you are declaring "I stop errors here," and that includes
  the exceptional ones.

You reach for the block form precisely when you want to catch more, so its catching panics is *why*
you used it, not a surprise. This is the supervisor pattern: a `main`, a request loop, or a task
boundary wraps its work in `try`/`catch` and catches everything, then decides (log, crash, restart,
fail the one unit) based on what it got. A single mechanism catches all errors at the boundary, so
there is no need for two separate constructs to "catch everything in `main`"; the block form already
does.

**`main` is an ordinary function for errorability, with no special exemption or requirement**
(functions §4, never §5). It is special only as the execution entry (modules §3), not in how `!`
applies to it, so it has exactly the two choices any function has:

- **Non-`!` `main`** handles every declarable error internally (the supervisor pattern above), and
  its signature promises so. Then the program provably exits with a declarable error **never**,
  never via an unhandled declared error, the whole declarable-error surface was closed inside the
  program.
- **`fn!` `main`** declares `!` (`fn (...): int!`) and may let a declarable error reach the top.
  There is no caller to catch it, so **the runtime is `main`'s top handler**: an escaped declarable
  error
  terminates the program (nonzero exit, the error and its `stacktrace` / `cause` reported, §6),
  mechanically like an unhandled panic reaching the top but arriving through the declared channel and
  visible in `main`'s signature. `main` must still **declare** the `!` if it can throw, the compiler
  will not add it (functions §4); a throwing `main` that omits `!` is a compile error like any other.

And like every function, **`main` cannot declare panics** (§7): panics are ambient and undeclarable,
so a panic reaching `main`'s boundary is stopped only by an explicit `try` / `catch` there (the
supervisor pattern), never by anything in `main`'s signature. So the runtime terminates on an
unhandled panic out of `main` exactly as it does on an escaped declared error out of a `fn!` `main`,
the two channels stay distinct all the way to the top.

Because the block catches both categories and they share the `error` root, a boundary that wants to
treat them differently distinguishes in the `catch` (§8.3): the `panic` subtree has a type to name
(`catch (p: panic)`), and the declarable category, which has none (§2), is selected by **ordering**, a
leading `catch (p: panic)` clause (re-throwing, or handling) leaves the following bare `catch` with
exactly the declarable errors. So the block is a genuine catch-all safety net without conflating
the categories, it can see both and still tell them apart.

### 8.3 Narrowing a catch

A `catch` clause head is a **parenthesized binder pattern** (match §2.1): `(_)`, `(_: T)`,
`(name)`, or `(name: T)`, and nothing else. Naming a type narrows the clause to that subtree,
letting the rest propagate:

```
try {
  ...
} catch (e: diskError) {    // catches diskError and its subtypes; e is a diskError
  ...
} catch (p: panic) {        // catches any panic (OOM, typeError, ArityError, ...)
  ...
} catch (e) {               // everything else: the remaining declarable errors
  ...
}
```

- **`catch (e)`**, everything; `e` inherits the root `error` (match §2). `catch (e: error)` says the
  same thing out loud, and `catch (_)` says it while discarding the value.
- **`catch (p: panic)`**, the `panic` subtree only; declarable errors propagate.
- **`catch (e: someType)`**, that type and its descendants. Write **`catch (_: someType)`** when the
  clause selects a subtree but does not want the value; the `_` is not noise, it is the visible
  statement that the error is being caught **and** thrown away.
- **`catch (e: ioError | typeError)`**, several subtrees in one clause. `|` after a `:` is the union
  operator (match §2.1), so `e` is typed `ioError | typeError`.

A clause head is the same typed binder used in a match arm and a constraint, and it means the same
thing: `e: someType` tests (`is`) and, on success, binds `e` **at** `someType` (match §2.2). A bare
`catch (e)` tests nothing, which is why it catches everything. There is no `catch (someType)` form,
because a bare name in a binder position is always a binding, never a type (match §2.1); the type
goes after the `:` here exactly as it does everywhere else.

**The parentheses are required, for two reasons.** They make `catch` the same shape as every other
clause head in the language, `if (c)`, `while (c)`, `foreach (k => v in xs)`, `match (x)`, where
`catch` was the sole exception. And they resolve a real ambiguity that the typed binder introduces:
`error` is both the declaration keyword and the root type (§3), so an unparenthesized
`catch (_: error) { ... }` puts a `{` directly after a type in a position where `error { ... }` is
also the error-declaration form. The `)` terminates the type unambiguously, which is also why a
pattern's type needs no `{` in its terminator set (match §2.1); the same holds for an inline
`enum {a, b}` type in a clause head.

There is deliberately no one-word catch for "declarable errors only" (the category has no
type, §2). Where a boundary wants to handle expected errors while letting panics keep
unwinding, the spelling is **ordering plus re-throw**: a leading `catch (p: panic) { throw p; }`
forwards the `panic` subtree, and the following bare `catch (e)` then receives exactly the
declarable errors. This is the price of the flat hierarchy, paid at the rare boundary that
splits the categories block-side; the common inline form for expected errors is the `try`
expression, which makes the split for free (§8.1).

Each `catch` selects a subtree by the O(1) subtype test; an error is caught by the first
clause whose type is an ancestor of it. A narrow catch that does not match lets the error
continue unwinding. Narrowing after a `try` expression uses the same subtype machinery through the
ordinary narrowing forms, `e as someType` or a `match` arm's typed binder (`e: someType`), with
`e is someType` as the boolean test that gates them; `is` never narrows (is spec §3), the binder or
the `as` does. Either recovers the concrete subtype the base-`error` arm erased (a `try` expression's
arm is spelled root `error`, though only declarable errors land there, §8.1).

---

## 9. panic

`panic` is the sealed subtree for runtime failures that are ubiquitous and not locally
preventable:

- Membership includes `outOfMemory`, `typeError` (a runtime type violation), `ArityError`
  (calling a function with fewer arguments than it declares, when not statically caught,
  functions spec), division by zero, failed runtime invariants, and similar.
- Panics are **catchable**: they are `error` values under the root, so the `try`/`catch` **block**
  (a bare `catch (e)`, or `catch (p: panic)`, §8.2) captures them. The `try` **expression** does **not**: a
  panic unwinds through it (§8.1). Catching a panic is a deliberate, block-level, boundary act, never
  an incidental effect of an inline `try`.
- Panics are **undeclarable**: a function that can only panic is still `fn`, not `fn!`
  (§7). This is why `!` stays meaningful, ambient failures do not infect every signature.
- The `panic` subtree is **sealed against user inheritance**: `error : panic` (or
  extending any `panic` descendant) is a compile error (§4). The runtime owns this
  subtree; user error types extend the root `error` (or each other), and are therefore
  declarable by construction.
- The `panic` subtree is **sealed against user origination** too: no `panic` type is
  constructable in user code (§5), and the runtime raises panics only at its own failing
  operations. So `panic` means exactly **"a language or runtime error"**: if a panic is
  unwinding, the runtime put it there, a program cannot fake one. The only `panic`-typed
  act available to user code is **re-throwing** one it caught (`catch (p: panic) { throw
  p; }`, §6, §8.3), which relays, and appends a breadcrumb to (§6.1), a runtime-minted
  value, never forges a new one. Together the two seals make the category meaning
  unspoofable in both directions: nothing user-defined is a `panic`, and nothing
  user-made becomes one.

The design goal is the two-axis separation: **catchability and declarability are
independent.** A panic is catchable (it is an `error`) yet undeclarable (it is in the
`panic` subtree). This lets the `try`/`catch` block be the honest catch-all while `!` tracks only
the failures a caller can meaningfully be required to handle. And the two *catch* forms respect the
split: the `try` **expression** catches only declarable errors (so panics cannot be
absorbed by inline expected-error handling), while the `try`/`catch` **block** catches everything (so
a boundary can stop even the exceptional).

The practical consequence for higher-order code: a function like a callback runner that
can raise an `ArityError` on a bad callback does **not** thereby become `fn!`, because
`ArityError` is a `panic`. It stays `fn`, and a caller who wants to guard the panic wraps
the call in a `try`/`catch` **block** and catches `panic` (not a `try` expression, which would let
the panic through), rather than being forced to handle a declared error that was never really the
contract.

---

## 10. Open questions

- ~~**Constructing the roots**~~, **resolved**: the root `error` **is** constructable and
  throwable, it is the throwaway (§5.2), there is no `UserError` node (§2), and the
  `panic` subtree is **runtime-raised only**, not constructable and not originable from
  user code, re-throw of a caught panic being the sole user-side `panic` act (§5, §6,
  §9).
- **The built-in `panic` set:** the exact enumeration of runtime panic subtypes
  (`outOfMemory`, `typeError`, `ArityError`, and the rest) and where it is defined (here,
  or the runtime spec).
- **Stack frame shape:** what a single `stacktrace` frame contains (function, file,
  line, and how a re-throw breadcrumb is distinguished from an origin frame), pending the
  runtime spec.
- **The trace's gate:** the stacktrace secret is gated by the default `[@reveal]` set
  (R111); whether traces deserve a **dedicated capability** (finer audit: who may see
  internal structure) is open.
