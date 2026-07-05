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
- There is **no `[]`** (an error is not a hashmap) and **no `->`** (an error wears no
  protocols). The three-operator table model (tables §3.3) does not apply; an error has
  only its declared fields.

---

## 2. The hierarchy

Every error descends from a single root, `error`, which has two subtrees:

```
error                     (root: catchable; the target of `!`; "catch everything")
├── Panic                 (sealed: no user inheritance; ambient; undeclarable)
│   ├── OutOfMemory
│   ├── TypeError
│   ├── ArityError
│   ├── OverflowError     (int arithmetic overflow, incl. INT_MIN / -1; int spec)
│   ├── DivisionByZero    (int division or remainder by zero; int spec)
│   └── ... (runtime-defined)
└── UserError             (user-extensible; declarable)
    ├── ApplyError
    ├── ... (user-defined)
```

The single root is deliberate: **`catch error` catches everything**, because everything
is an `error`. There is no separate top type above it (no `Throwable`) that a catch could
miss, so the obvious catch is the complete catch. This is the property the hierarchy
exists to guarantee.

Two policies attach to the two subtrees, and each is an O(1) subtree test
(value-representation §4.2):

- **Declarability** is governed by the `UserError` subtree. A function must be declared
  `fn!` iff it can raise a `UserError` (§7). A `Panic` may arise from any function without
  declaration.
- **Inheritability** is governed by the same split. User error types extend the
  `UserError` subtree; the `Panic` subtree is **sealed**, no user type may inherit from
  `Panic` or any of its descendants (§9). The runtime owns that subtree.

So "must this be declared" and "may a user extend this" both reduce to "which subtree,"
answered by the interval test on the statically-known hierarchy.

### 2.1 The root `error` shape

The root `error` is a real, constructable type with a fixed shape that every error
inherits as its prefix (value-representation §4.2), so these fields are present on every
error and readable on any base-`error` value without narrowing:

```
error {
  message: string;        // human-readable description; "" if none
  stacktrace: table;      // list of frames; sealed (§6.1); [] until first thrown
  cause: error?;          // the underlying error when wrapping (§6.2); null otherwise
  data: table?;           // arbitrary ad-hoc fields for throwaway errors (§5.2); null otherwise
}
```

- **`message`** is the description, empty string when unspecified.
- **`stacktrace`** is a list of frames, **sealed**: user code may read it but never write
  it; only the `throw` operator writes it (§6.1). It is `[]` until the error is first
  thrown, so `stacktrace.isEmpty()` is a sound, unforgeable test for "never thrown."
- **`cause`** links a wrapping error to the error it wraps (§6.2), `null` when there is no
  cause.
- **`data`** carries arbitrary keyed data for throwaway errors that want structure without
  a declared type (§5.2), `null` when unused.

Because these are the prefix of every error, `e.message`, `e.stacktrace`, `e.cause`, and
`e.data` are valid on any `error` value, including the base-`error` arm of a `try` (§8),
with no downcast.

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
root type (`| error`, `catch error`), exactly as `proto` is both keyword and type. An
error definition binds like any value; a module exports an error type by exporting its
variable.

A definition with no explicit parent **implicitly extends `UserError`**, so `myError`
above is a `UserError`. Its body declares **fields**: typed, optionally `?` (which makes
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
  extends `myError`, which extends `UserError`, which extends `error`.
- **Default parent is `UserError`**: omitting `: Parent` extends `UserError` directly
  (§3).
- **The `Panic` subtree is sealed**: `error : Panic` or extending any `Panic` descendant
  is a **compile error**. User errors live under `UserError` only.
- **Fields inherit as a prefix** (value-representation §4.2): a child's fields are laid
  out after its parent's, so `diskError` has `code`, `detail` (from `myError`), and
  `path`. Upcast to `myError` or `error` is a no-op; the `typeid` discriminates for
  narrowing.
- **Subtype tests are O(1)**: `diskError <: myError`, `catch myError`, and `is myError`
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

An error is constructed by naming its type and supplying its fields:

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

When a throwaway error wants **structured** data, that data goes in the `data` table
(§2.1), not into a new type:

```
throw error('bad config', [key => 'timeout', value => -1]);
// error('msg', dataTable) sets message and data
```

A handler reads it off the base error:

```
} catch error e {
  let k = e.data?.key ?? "unknown";
}
```

**There are no anonymous error types.** A `throw [k => v]` does *not* mint an implicit
type: that would create a type at a throw site (which the type model forbids,
value-representation §4) and, worse, an unnameable type no `catch` clause could select,
so it could only ever be caught as base `error` anyway. Ad-hoc keyed data is what tables
are for, so it rides in `data` on the base `error`, which is already catchable and
already the throwaway type.

The rule for when to declare a type versus use base `error` with `data`: **declare an
error type exactly when a handler will catch it by name** (`catch diskError`) and branch
on that type. If handlers will only ever `catch error` and inspect fields, there is
nothing for a distinct type to do, so use base `error` with `message` and `data`.

---

## 6. Throwing

`throw` raises an error value, unwinding until it is caught:

```
throw myError(11);                      // construct, then throw
```

`throw` takes an already-constructed error value. Whether a function must declare that it
throws depends on which subtree the error is in (§7): throwing a `UserError` requires the
enclosing function to be `fn!`; throwing a `Panic` does not. Throwing a **non-error** value
is a compile error; only `error`-typed values are throwable.

### 6.1 `throw` populates the sealed stacktrace

Writing the `stacktrace` field is a privileged effect of the `throw` operator. The field
is **sealed**: user code may read it (freely, anywhere) but never write it, and there is
no user-reachable name that grants the write, `throw` is an operator with a built-in
capability, not a function that can be aliased. This is what makes the empty-trace state
unforgeable.

`throw` writes the trace by checking whether it is empty:

- **Empty (`stacktrace.isEmpty()`, i.e. never thrown)** , `throw` **captures** the current
  call stack into it. This is the origin: the full stack where the error first failed.
- **Non-empty (already thrown)** , `throw` **appends the single current throw site** as
  one frame. This records a propagation hop without re-dumping the whole stack.

So the trace accumulates the path the error travelled: a full stack at the origin, then a
breadcrumb for each place it was re-thrown. Because `throw` never yields an empty trace
(it always at least captures its own site) and no other writer exists, a non-empty trace
reliably means "has been thrown," and `stacktrace.isEmpty()` reliably means "has not."

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
capacities compose freely (`string?!` is `string | null | error`).

A function's type records whether it can raise a declarable error, and it does so **by
declaration, not inference** (functions spec §4, never spec §5):

- A function is **`fn!`** iff its signature **declares** `!`. The `!` is **required, and
  verified**, whenever the body can raise a `UserError`, by a direct `throw` or by an
  **unhandled** call to an `fn!` function (an `fn!` call not lexically enclosed in a `try` /
  `catch` or resolved by an errorable binding). The compiler **checks** that a declared `!` is
  justified and that a missing `!` is safe; it never **adds** `!` for you. A function that can
  throw but omits `!` is a **compile error**, not a silent promotion to `fn!`.
- A function is **`fn`** (not `!`) iff it declares no `!`, which the compiler admits only when
  **no `UserError` can escape it**, every throwing call is handled. This is a real guarantee: a
  caller of a non-`!` function need not handle any declared error, because none can emerge.

The check is the **local, syntactic containment check** of functions spec §4 (per call site,
lexical, not control-flow analysis), not a call-graph propagation: "can `g` throw?" is read off
`g`'s **signature**, so errorability is never computed transitively.

**Panics are ambient and undeclarable.** Any function may raise a `Panic` (an `OutOfMemory`,
a `TypeError`, an `ArityError`) without being `fn!`. So `fn` guarantees "no `UserError`
escapes," not "cannot fail": a non-`!` function may still panic. This is what keeps `!`
meaningful (it tracks the failures you can locally anticipate and handle) without forcing
every allocating or calling function to be `!` for failures nobody can locally prevent
(§9).

This is the same `fn` / `fn!` distinction the function spec defines, and the same `!self`
a protocol's `apply` carries when it can throw (protocols §7.5).

---

## 8. Catching

There are two ways to handle a thrown error, and they split along the two error categories, which is
the central design point of the whole error model:

- The **`try` expression** (§8.1) catches **`UserError` only**, expected failures, collapsing them
  to a value inline. Panics unwind through it.
- The **`try`/`catch` block** (§8.2) catches **everything**, both `UserError` and `Panic`, the
  deliberate boundary where you stop even the exceptional.

The distinction is visible at a glance (a block has a `catch`, an expression does not) and it maps
exactly onto the `UserError`/`Panic` split: routine, inline handling of expected errors versus a
boundary that catches all. This is what keeps the model safe *and* ergonomic: expected errors are
handled where they arise, invariant violations propagate to a real boundary, and the syntax for the
first can never silently absorb the second.

### 8.1 The `try` expression catches `UserError` only

`try expr` runs `expr` and, if it throws a **`UserError`**, yields that error as a value instead of
unwinding:

```
let v!: string = try someFunc();          // v : string | UserError
let w:  string | error = try someFunc();  // identical; ! is sugar for the UserError arm
```

The `try` **expression** catches the **`UserError`** subtree only. A **`Panic`** is **not** caught
by a `try` expression: it **unwinds through** it, propagating up to a `try`/`catch` block (§8.2) or,
if none, to the task or program boundary. This is deliberate and is the heart of the two-category
design:

- A `UserError` is an **expected** outcome, part of a function's normal control-flow contract (it is
  what `!` declares). Handling it inline with `try` is routine, so `try` collapses it to a value.
- A `Panic` is an **invariant violation**, a bug or exhausted resource, not an expected outcome
  (which is exactly why it is undeclared, §9). It must **not** be silently absorbable by the syntax
  used to handle expected errors: a programmer writing `_ = try cleanup()` to ignore an *expected*
  failure must not thereby swallow an overflow or out-of-memory. So a panic ignores `try` entirely
  and keeps unwinding to a real boundary, where a supervisor can decide to crash, restart, or fail
  the unit of work.

Letting `try` catch panics would defeat the reason `Panic` exists: a panic's job is to propagate a
violated invariant to a level that can act on it (or to crash loudly), which is what makes
"assume-no-overflow" safe in the first place (§9). Absorbing it at an unrelated `try` site would make
the bug invisible, the smuggled-error failure mode, at the panic layer. So the `try` expression
respects the category split: expected errors in, invariant violations through.

Because the expression catches only `UserError`, its error arm is the **base `UserError`**, not the
root `error` (a panic never lands here). Binding the result into a type that cannot hold the error
(no `!`, no `| UserError`/`| error`) is a compile error, exactly as assigning `null` to a non-`?`
type is: the error must be handled. The result is a union (`string | UserError`), so the value cannot
be used as a plain `string` until the error arm is narrowed away (§8.3), which is what makes
swallowing type-impossible: you cannot proceed as if the call succeeded without first dealing with
the error case.

Discarding the result entirely still requires the explicit no-discard form `_ = try someFunc()`
(variables spec: a return value may not be silently dropped). That discard is visible and greppable,
and it discards only the `UserError` (a panic would still unwind through), so it is an honest,
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
} catch e {
  // e is the caught error, typed root `error`; may be UserError or Panic
}
```

The `try`/`catch` **block** is the **catch-all**: a bare `catch e` catches the **root `error`**, so
it catches **both** `UserError` and `Panic`. This is the intended boundary form, and it honors the
universal intuition that a `try`/`catch` block catches everything. The two catch forms therefore
split cleanly by the two error categories, and the split is visible at a glance (one has a `catch`
block, one does not):

- **`try expr`** (expression, no block), catches **`UserError` only**; panics unwind through. The
  ergonomic, inline handling of expected failures (§8.1).
- **`try { } catch { }`** (block), catches **everything** (UserError and Panic). The deliberate
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

- **Non-`!` `main`** handles every `UserError` internally (the supervisor pattern above), and its
  signature promises so. Then the program provably exits with a `UserError` **only via a panic**,
  never via an unhandled declared error, the whole declarable-error surface was closed inside the
  program.
- **`fn!` `main`** declares `!` (`fn (...): !int`) and may let a `UserError` reach the top. There is
  no caller to catch it, so **the runtime is `main`'s top handler**: an escaped `UserError`
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
treat them differently distinguishes by type in the `catch` (§8.3): catch `UserError` and `panic`
separately, or catch the root and `match` on which subtree it is. So the block is a genuine
catch-all safety net without conflating the categories, it can see both and still tell them apart.

### 8.3 Narrowing a catch

A `catch` may name a type to catch only that subtree, letting the rest propagate:

```
try {
  ...
} catch diskError e {     // catches diskError and its subtypes
  ...
} catch UserError e {     // catches any other UserError
  ...
} catch panic p {         // catches any Panic (OOM, TypeError, ArityError, ...)
  ...
}
```

- **`catch error e`** (or bare `catch e`), everything.
- **`catch UserError e`**, the declarable subtree only; panics propagate.
- **`catch panic p`**, the `Panic` subtree only; user errors propagate.
- **`catch SomeType e`**, that type and its descendants.

Each `catch` selects a subtree by the O(1) subtype test; an error is caught by the first
clause whose type is an ancestor of it. A narrow catch that does not match lets the error
continue unwinding. Narrowing after a `try` expression uses the same subtype machinery
(`is SomeType`, or a following `catch`), recovering the concrete subtype the base-`UserError`
arm erased (a `try` expression's error arm is `UserError`, since panics are not caught there, §8.1).

---

## 9. Panic

`Panic` is the sealed subtree for runtime failures that are ubiquitous and not locally
preventable:

- Membership includes `OutOfMemory`, `TypeError` (a runtime type violation), `ArityError`
  (calling a function with fewer arguments than it declares, when not statically caught,
  functions spec), division by zero, failed runtime invariants, and similar.
- Panics are **catchable**: they are `error` values under the root, so the `try`/`catch` **block**
  (`catch error` or `catch panic`, §8.2) captures them. The `try` **expression** does **not**: a
  panic unwinds through it (§8.1). Catching a panic is a deliberate, block-level, boundary act, never
  an incidental effect of an inline `try`.
- Panics are **undeclarable**: a function that can only panic is still `fn`, not `fn!`
  (§7). This is why `!` stays meaningful, ambient failures do not infect every signature.
- The `Panic` subtree is **sealed against user inheritance**: `error : Panic` (or
  extending any `Panic` descendant) is a compile error (§4). The runtime owns this
  subtree; user error types extend `UserError`.

The design goal is the two-axis separation: **catchability and declarability are
independent.** A panic is catchable (it is an `error`) yet undeclarable (it is not a
`UserError`). This lets the `try`/`catch` block be the honest catch-all while `!` tracks only the
failures a caller can meaningfully be required to handle. And the two *catch* forms respect the
split: the `try` **expression** catches only the declarable `UserError` (so panics cannot be
absorbed by inline expected-error handling), while the `try`/`catch` **block** catches everything (so
a boundary can stop even the exceptional).

The practical consequence for higher-order code: a function like a callback runner that
can raise an `ArityError` on a bad callback does **not** thereby become `fn!`, because
`ArityError` is a `Panic`. It stays `fn`, and a caller who wants to guard the panic wraps
the call in a `try`/`catch` **block** and catches `panic` (not a `try` expression, which would let
the panic through), rather than being forced to handle a declared error that was never really the
contract.

---

## 10. Open questions

- **Constructing the subtree roots:** base `error` is constructable and throwable (§2.1,
  §5.2). Whether `UserError` and `Panic` themselves can be constructed directly, or only
  their concrete subtypes, is unresolved.
- **The built-in `Panic` set:** the exact enumeration of runtime panic subtypes
  (`OutOfMemory`, `TypeError`, `ArityError`, and the rest) and where it is defined (here,
  or the runtime spec).
- **Stack frame shape:** what a single `stacktrace` frame contains (function, file,
  line, and how a re-throw breadcrumb is distinguished from an origin frame), pending the
  runtime spec.
