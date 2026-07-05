# `undefined`

`undefined` is Luna's **absence sentinel**: the value that means "there is no value here, and using
it is a bug." It is distinct from `null` (an explicit, meaningful "nothing"), and the two never
collapse into each other. The defining rule, from which everything else follows:

> **`undefined` is language-produced, never programmer-written.** The language hands it to you in the
> situations where something is genuinely absent; you never conjure it as a literal. `null` is the
> value you *choose* for "intentionally empty"; `undefined` is the absence the language *reports*.

---

## 1. What `undefined` is

`undefined` marks a **structural absence**, a place where the language has no value to give:

- **A missing table key.** Reading a key that is not present yields `undefined` (coalescing spec):
  `tab['missing']` is `undefined`. Absence is a routine question for a table-as-hashmap, so it does
  not throw; it reports `undefined`.
- **A void return.** A function that returns no meaningful value returns `undefined` (§4).

These are the two productions, and they share one meaning: "nothing is here." `undefined` is
therefore the single, unambiguous answer to "was there anything?", which is what lets the coalescing
operators (`??`, `?.`, `???`) navigate absence unambiguously (coalescing spec).

## 2. `undefined` is storable, but using it panics

`undefined` can be **held by a binding**. Reading a missing key into a variable is not an error:

```
const l = tab['missing'];      // l holds undefined; this is fine, not an error
```

The binding `l` need not declare that it can be `undefined`; any binding may end up holding it,
because absence is something the language can report anywhere a value is produced. So `undefined` is
a real, storable value at the **binding** level.

But **using** an `undefined`, dereferencing it, doing arithmetic on it, calling it, indexing it,
assigning it into a table (§3.1), passing it where a concrete value is required, is an error, because
`undefined` represents a value that is not there, so *acting* on it as though it were a real value is
a bug. The language catches this at the earliest point it can:

- if the value is **statically known** to be undefined (its type *is* `undefined`, a void return or
  an `undefined`-typed binding), the use is a **compile error**;
- if the value **might** be undefined (its type is `T` but it could hold undefined at run time, a
  missing-key read), the use is a **runtime panic** (a `panic`, errors spec) *iff* it is actually
  undefined at that moment.

This is the same static-when-possible discipline used throughout the language: prove it and reject at
compile time, otherwise stop loudly at run time rather than proceeding with a nonsense value.

```
const l = tab['missing'];      // l is undefined (fine, a binding, §3.3)
const n = l + 1;               // PANIC: used a possibly-undefined value that is undefined now
const u = voidFn();            // u : undefined (fine, inert binding)
const m = u + 1;               // COMPILE ERROR: used a statically-known undefined
```

So `undefined` is **storable but not usable**: you may hold it, check it, and coalesce it away, but
the moment you treat it as a real value it is rejected, statically if the compiler can prove it,
dynamically otherwise. Holding it is safe; using it is the bug.

### 2.1 Checking and coalescing, the safe ways to handle it

Because `undefined` panics on use, the safe response is to **resolve it before use**, which the
coalescing operators do (coalescing spec):

```
const v = tab['missing'] ?? fallback;      // ?? catches undefined, gives fallback
const w = obj?.field;                      // ?. short-circuits an undefined receiver to undefined
if (tab['k'] == undefined) { ... }         // an explicit absence check (equality spec)
```

`==` against `undefined` is well-defined: `undefined == undefined` is **true** (so absence checks
work), and `null == undefined` is **false** (the two absences are distinct; equality spec). So you
can test for absence explicitly, and coalesce it away, without ever *using* the undefined value.

## 3. `undefined` is never stored *in a table*, only *in a binding*

There is a sharp line: `undefined` may be held by a **variable or a return**, but it is **never a
table value**. A present key always holds `null` or a real value, never `undefined` (coalescing
spec). This gives the load-bearing equivalence:

> **existence ⟺ not-`undefined`.** A key that exists holds something real (possibly `null`); a key
> that does not exist reads as `undefined`. There is no third case.

This is what makes absence unambiguous: `tab['k'] == undefined` means exactly "no such key," never
"a key storing undefined," because the latter cannot exist. The equivalence holds **by
construction**: there is no path, at compile time or run time, by which `undefined` can enter table
storage.

### 3.1 Assigning `undefined` into a table never stores it

Because `undefined` can never be a table value, assigning it into a key does **not** store it, and it
does **not** silently delete the key either (deletion is explicit, §3.2). Instead:

- **`tab['k'] = undefined` (the literal)** is a **compile error**, undefined cannot be written
  explicitly at all (§5), so this is caught the same way `var x = undefined` is.
- **`tab['k'] = v` where `v` is *statically* undefined** is a **compile error**. When the compiler
  *knows* the right-hand side is undefined, because its type *is* `undefined` (a void return, or a
  binding of type `undefined`), it rejects the assignment at compile time rather than deferring to a
  runtime panic:

  ```
  someTab['i'] = voidFn();          // COMPILE ERROR: voidFn() returns undefined (§4)
  const u = voidFn();               // legal: u : undefined (an inert binding, §3.3)
  someTab['k'] = u;                 // COMPILE ERROR: u is statically undefined
  ```

- **`tab['k'] = v` where `v` *might* be undefined at run time** is a **runtime panic** if `v` is in
  fact undefined at that moment. When the value's type is `T` but it could hold undefined (for
  example, the result of a missing-key read bound to a variable), the compiler cannot prove it either
  way, so the check is deferred: assigning it panics *iff* it is actually undefined when the
  assignment runs.

  ```
  var v = tab['maybeMissing'];      // v might be undefined
  dest['k'] = v;                    // PANIC iff v is undefined right now
  dest['k'] = v ?? fallback;        // safe: ?? resolves the absence before the write
  ```

The unifying rule: **assigning `undefined` into a table is illegal**, and the language catches it
**statically when it can prove the value is undefined (a compile error) and dynamically when it can
only tell at run time (a panic)**, the same static-when-possible discipline used throughout the
language. Assignment into a table is a **use** of the value, so this is just the general
undefined-on-use rule (§2) applied to the assignment position: a use of a statically-known undefined
is a compile error, a use of a possibly-undefined value is a runtime panic.

Why panic rather than delete-on-assign: silent deletion would be a severe footgun. If
`tab['balance'] = computeBalance()` and `computeBalance()` accidentally produced undefined (a missing
lookup deep inside), a delete-on-assign rule would make `tab['balance']` **silently vanish**, and
later reads could not distinguish "never set" from "deleted by a bad write." Panicking (or erroring
at compile time) instead surfaces the upstream undefined-producing bug **at the assignment site**,
which is exactly where the corruption would have entered the table. Deletion is therefore always
explicit (§3.2), never a side effect of assignment.

### 3.2 Deleting a key is explicit

Removing a key is done with the table's explicit removal operations (`remove`, `unset`, tables
spec), never by assigning `undefined`. This keeps deletion a deliberate, named act, and it keeps
`undefined` out of the assignment-to-delete role that would require making it writable. So: to read
absence, read a missing key (yields `undefined`); to *create* absence, call `remove`; assignment
never deletes.

### 3.3 A binding of type `undefined` is legal but inert

Holding `undefined` is always legal, whether or not the compiler knows statically that the value is
undefined. `const u = voidFn()` binds `u : undefined`; a missing-key read binds a possibly-undefined
value. Binding is never the error, this is the same decision that lets `const l = tab['missing']`
bind undefined (§2). What is controlled is **use**, not holding:

- a statically-`undefined` binding (`u : undefined`) is **inert**, you may hold it, compare it, and
  coalesce it, but any *use* (arithmetic, deref, call, index, assignment into a table, passing where
  a concrete value is required) is a **compile error**, because the compiler knows it is undefined;
- a possibly-undefined binding (type `T`, might hold undefined) is usable *if* it currently holds a
  real value, and **panics** if it is used while actually undefined.

Forbidding the *binding* would be inconsistent: it would penalize the compiler for knowing more (a
statically-undefined binding would be illegal while an equally-undefined-but-less-provable one is
legal). So binding is uniformly legal, and the static knowledge is spent making *uses* compile
errors rather than runtime panics, which is a strict improvement (caught earlier, clearer
diagnostic), not a new prohibition.

## 4. Void functions return `undefined`

Luna has **no `void`**: every function returns something. A function that returns no meaningful
value, one with no `return`, or a body that completes without producing a value, returns
**`undefined`**, not `null`.

```
const log = fn (msg: string) use (io) => { io->println(msg); };   // returns undefined
```

`undefined` is the right choice (over `null`) for a void return because the meanings line up
exactly:

- A void function **has no result**, that is a genuine absence, which is what `undefined` means
  (whereas `null` is a *real, chosen* value, so a void function "returning `null`" would falsely
  imply it meaningfully produced null).
- **Using a void function's result is a bug**, there is no result to use, and `undefined` panics on
  use, so `const l = log("hi"); use(l)` panics, which is the *correct* outcome (you tried to use a
  result that does not exist), not a surprise.

So a void return is "absent result, using it is a bug," which is precisely `undefined`.

### 4.1 A void call needs no discard

A return value may not be silently dropped: `someFn()` in statement position normally requires the
explicit no-discard form `_ = someFn()` (variables spec). But a **void** call is **exempt**, because
there is nothing meaningful to discard:

```
log("hi");        // fine: no _ = needed; the return is undefined (no value to drop)
```

The compiler knows a function's return type at compile time, so it knows statically when the return
is `undefined` (void) and **waives the no-discard requirement** for exactly those calls. This is
provable, not heuristic: the return type either is or is not `undefined`, and the discard exemption
follows. So void calls read cleanly (`log("hi");`, not `_ = log("hi");`), while any call that returns
a real value still must have its result used or explicitly discarded. Discarding is required only
where there is something to discard.

## 5. `undefined` cannot be written explicitly

You **cannot** write `undefined` as a literal or initializer:

```
var x = undefined;        // COMPILE ERROR
```

This is the enforcement of the guiding rule (§0): `undefined` is language-produced, never
programmer-written. The reason is to keep `undefined`'s meaning honest and to protect the
`null`/optional story:

- **Meaning stays honest.** Because `undefined` only ever arises from a real absence (a missing key,
  a void return), it always means "the language found nothing here." If a programmer could write it,
  it would come to mean "the programmer chose nothing," which is `null`'s job, and the two absences
  would blur.
- **No footgun uninitialized-value pattern.** If `undefined` were writable, it would offer a second,
  worse way to express "not set yet": `var x = undefined` would give a value that **panics** when
  read before assignment, instead of the checkable `null` an optional provides. That competes with,
  and is worse than, the tool built for exactly this: an **optional** (`var x?: T = null`), where the
  not-yet-set value is a *checkable* `null`, not a *panicking* `undefined`. So "not yet set" is
  always `null` via optional, and `undefined` is never a programmer's initializer.

So the division of labor is clean: **`null` is the value you choose for "intentionally, meaningfully
empty"; `undefined` is the absence the language reports for "nothing was here," which you may hold
and must check, but never write and never use.**

---

## 6. Representation

`undefined` is a flag state of the `lval` (`isUndefined`, value-representation spec), not a heap
value: a binding holding `undefined` carries the flag, and operations that would use the value check
the flag and panic. Because `undefined` is never a table value (§3), a table's storage never needs to
represent it; only bindings and return slots do. The absence is a single-bit fact, so producing,
storing, and checking `undefined` are all cheap.

---

## 7. Open questions

- **`undefined` in `any` and generic positions.** Whether a binding typed `any` that holds
  `undefined` behaves uniformly with a concretely-typed one (holding is fine, using panics), and how
  reflection reports the type of an `undefined` binding, pending use.
- **Undefined from a partial or interrupted computation.** The productions remain **missing-key and
  void-return only**. A *taken* stream or builder, one transferred across a spawn boundary
  (concurrency §2.3), is deliberately **not** `undefined` but a **separate taken state on the
  referent** (value-representation §2.1). The two are kept distinct on purpose: `undefined` is a
  per-*binding* `lval` flag that can never live in a table (§3), whereas taken must be shared
  across every alias of one stream, **including a table element**, so it lives on the value, not the
  slot, and using it **panics** rather than coalescing. Whether any *further* operation produces
  `undefined` (a reserved-but-unset slot in a future builder API) remains pending, but ownership
  transfer does not extend `undefined`'s productions, it has its own state.
