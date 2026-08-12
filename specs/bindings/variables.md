# Variables

Variables are declared with `var`, `let`, or `const`. The three form a ladder from
fully mutable to fully immutable:

| Keyword | Rebind the name? | Mutate the value in place? |
|-|-|-|
| `var` | yes | yes |
| `let` | **no** | yes |
| `const` | **no** | **no** (deep) |

- **`var`**, rebindable and interior-mutable.
- **`let`**, the *binding* is fixed: the name may never be rebound. The *value* can
  still change through its own operations (assigning a table key, consuming a
  stream). This is binding-immutability, not value-immutability.
- **`const`**, no rebind *and* no mutation, applied deeply (§3).

Luna will automatically apply constant-access optimizations to `let` bindings when
appropriate.

```luna
let myName = 'Lucas';
```

```luna
var myTable = [];
myTable.name = 'Lucas';            // interior mutation, fine under var and let
myTable['lastName'] = 'Streanga';
```

---

## 1. Types and reassignment

A variable's declared type is fixed at declaration and every assignment is checked
against it.

- **Annotated**, the annotation is the declared type: `var n: stream|table = 0..100`.
- **Unannotated**, the type is **inferred from the initializer and then fixed**; it
  does *not* widen on later assignment. Use `any` to opt out of static typing. Note on
  constraints: a **literal carries no constraint** (`var t = [1, 2, 3]` infers `table`;
  constraints enter only through declarations, constraints §7), while a value that
  already carries one keeps it (`var b = someByte()` infers `byte`, `var xs =
  sortedNames()` infers `list` when that signature declares `fn (): list`): a
  constraint is a validated commitment the producer made, and inference preserves
  commitments, never manufactures them.

```luna
var numbers: stream|table = 0..100;
println(@numbers.typeName);        // "stream"
&numbers.map(fn (number: int) => number * 2);
println("$num, ") foreach (num in numbers);   // 0, 2, 4, 6, 8 ...
println(numbers.isConsumed());     // "true"
numbers = (0..50).collect();       // materialize the range stream into a list (a table):
                                   // collect is the one stream->retained bridge
                                   // (iterable-functions §2.11); a function, never `as`
println(@numbers.typeName);        // "list"
numbers = null;                    // compile error incompatibleTypeError:
                                   // null is not a member of stream|table
```

Because an unannotated `var` fixes its inferred type, changing a value's *kind*
requires either declaring the union up front or opting into `any`:

```luna
var myFile: File|stream = openFile('myfile.txt', File.modeRead);
println(@myFile.typeName);         // "File"
myFile = myFile.lines();           // OK: stream is a member of File|stream
println(@myFile.typeName);         // "stream"

var f = openFile('myfile.txt', File.modeRead);   // inferred type: File (fixed)
f = f.lines();                     // compile error incompatibleTypeError:
                                   // stream is not a member of File
```

### 1.1 Rebinding a `let` or `const`

Rebinding a fixed binding is a **compile error** (`reassignmentError`; compile errors
will carry codes):

```luna
let numbers = 0..100;
numbers = 0..50;                   // compile error reassignmentError:
                                   // cannot rebind a variable declared with let
```

### 1.2 Write-once optional `let`

An optional `let` initialized to `null` may receive **exactly one** non-null
assignment, after which it behaves like any other `let`:

```luna
let numbers?: stream = null;
numbers = 0..100;                  // OK: the single permitted write
numbers = 0..50;                   // compile error reassignmentError
```

The single write must be non-null; once it is spent, *every* further assignment -
including `numbers = null`, is rejected. Where the double write is statically
evident (straight-line code, as above) it is a compile-time `reassignmentError`;
where it is branch-dependent it cannot be seen statically, so it is tracked with a
runtime flag and raises `writeOnceViolationError`:

```luna
let handle?: stream = null;
handle = openA() if (a);
handle = openB() if (b);           // runtime writeOnceViolationError if a and b both hold
```

### 1.3 Every declaration must be initialized

A variable declaration **must** provide an initializer. There is no uninitialized
declaration form: `var x: int;` is a **syntax error**. You write `var x: int = 0`, or, for
a value that is not yet known, an **optional** initialized to `null`:

```text
var x: int;             // SYNTAX ERROR: a declaration must be initialized
var x: int = 0;         // OK
let x?: int = null;     // OK: "not set yet" is an explicit null, filled once later (§1.2)
```

The first line is a **syntax** error, not a semantic one, which is why this block is not
fenced as Luna: the initializer is part of the declaration's production (grammar §0.1), so
there is nothing to reject later — an uninitialized declaration does not parse. That is the
strongest form the rule can take, and it is why the block above cannot be a compiling
example of its own subject.

This is not merely stylistic; it is what lets Luna's compiler do **no control-flow analysis**
(compiler spec). "Is this variable assigned before it is used?" is the classic question that
requires definite-assignment analysis, tracking, along every control-flow path, whether an
assignment has happened yet. By forbidding the uninitialized state entirely, Luna makes that
question vacuous: there is never a point where a binding lacks a value, so there is nothing to
analyze. The two legitimate "no value yet" needs are served without an uninitialized state:

- **"Empty for now, will be set once"**, an optional `let x?: T = null` (§1.2). The not-yet-set
  value is a *checkable* `null` (the type is `T | null`, so the type system forces you to handle the
  null before using it as `T`), not a value that panics on use.
- **"Absent"**, handled by `undefined` (undefined spec), which the language produces (a missing
  key, a void return) but which you never write as an initializer.

So "not yet set" is always an explicit, checkable `null` via an optional, never an uninitialized
binding. The fact carried is in the **type** (`T | null`), not in a control-flow path, which is the
recurring rule that keeps the compiler analysis-free: facts live in types or in runtime state, never
in "what is true on this path."

---

## 2. `let` and interior mutation

Since `let` fixes only the name, interior mutation is allowed, but note how this
meets the table protocol, whose method operations are *pure* and write back through a
reference:

- **Direct** interior mutation is fine: `letTab.name = 'x'`, `letTab[k] = v`, or
  consuming a `let` stream.
- **Write-back** mutation is not: `&letTab.sort()` would rebind `letTab` to the
  method's result, and rebinding a `let` is forbidden. (It is doubly forbidden -
  `&` may not be applied to a `let` at all; see §5.)

So on a `let` table you mutate directly; to apply a pure transform and keep the
result, the binding must be a `var`.

---

## 3. `const`

`const` is `let` plus **deep immutability**, a compile-time property of the binding: through a
`const` binding, neither the value nor anything reachable from it may be changed, **recursively**, so
nested tables are immutable too, no key may be added and no value overwritten. On a stream it fixes
the cursor: a `const` stream cannot be consumed. Any mutation attempt through a `const` binding is a
compile error where the target is statically known, and otherwise panics (`typeError`)
at runtime.

This immutability is `const`'s own **compile-time** guarantee, a property of the *binding*, **not a
revocable runtime seal** on the value: both former seal axes — the `freeze` / `thaw` mutation seal
and the `open` / `close` / `neverOpen` growth seal — are removed outright (tables §5, R109). The
runtime arm of the sentence above is therefore not a `const` flag being tested: where a mutation
reaches a `const` value through a dynamic path the compiler could not rule out statically, the value
is already in its **permanently-immutable representation** — frozen storage with no mutation
machinery at all (tables Amendment A) — so the write panics (`typeError`) and
the value still never changes. Because a `const` table is known immutable at compile time, the
compiler can specialize it (perfect-hashing, inlining; compiler spec), and, as concurrency relies on,
it can be shared by reference across tasks without copying, since it can never change.

```luna
const config = ['db' => ['host' => 'localhost']];
config.db.host = 'remote';         // error: const is deeply immutable
config.timeout = 30;               // error: const admits no new keys either
```

`const` binds an **independent** value; it never freezes a value still in use
elsewhere. Const-binding a value that is currently shared conceptually copies it to
the new binding and makes *that copy* deeply immutable, leaving the original untouched:

```luna
var original = ['count' => 0];
const snapshot = original;         // snapshot is a deep, immutable copy
original.count = 1;                // OK: original is an ordinary var
snapshot.count = 1;                // error: snapshot is const (deeply immutable)
println(original.count);           // 1
println(snapshot.count);           // 0
```

### 3.1 `let` and `const` coincide when there is no mutable interior

The only thing separating `let` from `const` is **interior mutation** (§2): both fix
the name, and `const` additionally freezes the value's contents. So `let` and `const`
differ **only for values that have a mutable interior to protect**, tables, `bytes`
buffers, string builders, and the like. For a value with **no** interior-mutable
contents reachable through the binding, there is nothing for `const` to freeze that
`let` leaves open, so the two are **equivalent**:

- **Scalars and immutable values**, `int`, `double`, `bool`, an immutable `string`, have
  no interior to mutate, so `let x = 5` and `const x = 5` mean the same thing.
- **Functions**, a `fn` value has no interior state reachable through its binding: you
  never mutate a closure's contents through the name that holds it. (It has no mutable
  contents at all: a closure's environment is an implicit deep-`const` snapshot,
  functions spec §2.1, and its only referential capture is a `use`d capability, which
  is itself immutable.) So `let f = fn ...` and `const f = fn ...` are equivalent.

This is not a special case for any of these types; it falls out of the general rule that
`let`-versus-`const` is exactly the presence or absence of interior freezing, which is
vacuous when there is no interior. Both spellings are therefore permitted for such
values, and mean the same thing.

**Convention**: for functions specifically, prefer `const` (a function is normally a
fixed definition, functions spec §1). `let f` is allowed and identical in meaning, but
`const f` reads as the intended "this name is a fixed function." The choice is stylistic,
not semantic.

---

**The freeze covers meta space.** A `const` value's deep freeze includes its **protocol
state** (meta members like a builder's buffer) and its **applied set**: a mutating meta call
(one taking `&self`) on a const-rooted value is a **compile error** where statically evident
and a **panic** through any dynamic path, and statement `apply` on a `const` binding is an
error (protocols §10). This is forced, not stylistic: `const` values cross task boundaries
**shared by reference** (concurrency §2.1), justified entirely by immutability, so a mutable
meta side door would be a shared-state data race, the exact race the isolation model exists
to prevent. (The std handles remain coherent: their buffers live runtime-side behind the
runtime's lock, not in Luna meta space, std.io §2.1.)

## 4. Scoping

Variables are **lexically (block) scoped**, as is typical in modern languages, and
unlike PHP, where variables are function-scoped.

```luna
foreach (num in 0..10) {
  let i = num;                     // scoped to the loop body
}
println(i);                        // compile error: i is undefined in this scope
```

**Module level admits `const` and nothing else** (R262). A top-level `let` or `var` is a
compile error; the ladder of §1 exists inside functions and blocks, and collapses at module
scope to its top rung.

The reason is the language's first commitment. Module-level bindings are **referenced** by the
functions that use them, not captured (functions §2.1 — capture would break recursion), so a
mutable one is reachable for writing from any function in the module, and therefore from any
**task** that calls one. That is shared mutable state across tasks, which concurrency §2 closes
by construction and which the closure model already refuses one level down: functions §2.1
names a counter, `once` and `memoize` **non-goals**, and a module-level `var` is that counter
with a wider scope. `let` is no safer than `var` here — it forbids rebinding, not interior
mutation, so `let cache = []; … cache[k] = v` is the same shared state by another spelling.

Both are therefore excluded rather than merely discouraged. The alternative is not absent, it
is elsewhere: state that must change lives in a **frame** (a local, passed explicitly) or under
an **owning task** (the owner-task pattern, channels §5, whose worked example is exactly a
`var count = 0;` reached by message). And the one thing a module-level `var` could otherwise
have been used for — configuration read at startup — is unavailable regardless, since module
initialization runs under the empty capability grant (R257) and cannot reach a file; such
values arrive through `main`.

The rule is enforced by semantic analysis, not by the grammar, which admits all three keywords
uniformly (grammar §9) so that the diagnostic can say which rung was used and why the top one
is the only one available.

### 4.1 An unused local binding is a compile error (R159)

A local `let`, `var`, or `const` that is **never read** after its declaration is a
**compile error**. The discard is explicit — `_`, or do not bind. This is not only the
Go stance adopted on its merits (dead bindings are bugs or noise, and the language
already refuses silent nonsense); it is **forced by the backend contract**: compiler
§1.7 rules that a Go compile failure is always a compiler bug, and Go rejects unused
locals — so Luna either errors at its own level, with its own diagnostic, or the
emitter must litter silencers to launder dead code through. Erroring aligns the
semantics and keeps emission honest.

Two boundaries, both Go's line and both principled: **unused *parameters* are not an
error** — signature conformance is legitimate (a callback must match its slot,
functions §3.2; name a parameter `_` when its unuse is the point) — and module-level
unexported-unused bindings are not an error for alpha (dead-code elimination's
territory, compiler §5). Unused **imports** are the sibling error, ruled at their home
(modules §5, R159).

---

## 5. Passing semantics

By default all variables are passed **by value**, implemented as copy-on-write:
technically a reference is passed and the payload is copied on first mutation. This
is opaque to the programmer. The sole exception is `stream`, which is always passed
**by reference** because streams are consumable.

### 5.1 The reference operator `&`

`&` passes by reference and may be written on either the caller or the callee side.
A function that mutates its argument in place takes (or is given) a reference:

```luna
var myTable = [];
var fillTable = fn (&myTable: table) => myTable['element'] = 1;   // & on the callee
fillTable(myTable);
println(myTable);                  // ['element' => 1]

fillTable = fn (myTable: table) => myTable['element'] = 1;        // & on the caller
myTable = [];
fillTable(&myTable);
println(myTable);                  // ['element' => 1]
```

**`&` requires a `var`.** Applying it to a `let` or `const` binding is a compile
error. A single reference kind cannot distinguish "mutate through the reference" from
"rebind through the reference," and rebinding is what `let`/`const` forbid, so
references are restricted to `var`. To let a function mutate your value in place,
declare it `var`; a `let`/`const` value can still be passed by value (the callee
mutates only its own copy).

**`&` is invariant.** A reference's type is the binding's declared type, **exactly**: a
`&` argument fits a `&` parameter only when the two types are the same, never by
widening. `&xs` on a `list`-declared binding is a `&list` and does not fit `&t: table`;
`&b` on a `byte`-declared binding does not fit `&x: int`. Both are compile errors at the
call. The reason is the classic reference-variance unsoundness: a reference is a
**write** channel, and a callee holding the widened reference could write any value the
wider type admits (a string key, `300`) straight into a binding whose declaration
forbids it, a violation the callee cannot see and the caller never authorized. Reading
widely is safe and needs no reference, pass by value; writing must be exact. (The
value-carried check, constraints §9.4, would catch such a write at runtime anyway;
invariance turns it into a compile error at the call, the earlier and better
diagnostic.)

```luna
let t = [];
fillTable(&t);                     // compile error: cannot take a reference to a let binding
```

Because streams pass by reference, a method that extends the pipeline is visible to
the caller, and consuming the stream in the callee consumes it for everyone:

```luna
let myStream = 0..10;
&myStream.map(fn (num: int) => num * 2);
println(myStream.isConsumed());      // "true"
println(num) foreach (num in myStream);   // nothing: the stream is consumed
```

**References to inline scalars.** The examples above take references to heap-backed values
(`table`, `stream`), whose `lval` already holds a shared pointer, so a reference just shares
that pointer. A scalar (`int`, `double`, `bool`) is stored **inline** in its `lval` (there is no
separate object to point at), so a reference to it is a **pointer to the binding's `lval` slot**
itself. This needs no special handling and **never allocates**: a `&` reference exists only as
an argument, calls are **synchronous**, so the caller's frame outlives the call, the callee
writes through the pointer, and the caller resumes with the new value, exactly as for `&table`.
A reference cannot outlive the frame by any other route either, because a closure cannot hold
one: closures capture `const` snapshots only (functions spec §2.1), so the old
"escaping `use`-capture of a scalar" case, and the boxing it required, **no longer exists**;
no Luna-level escape analysis and no Go-side box is ever needed for a scalar reference
(compiler §1.4.1). The reference **shares** the value; it never **moves** it. A scalar is
copyable, so it is never *taken* (concurrency §2.3, value-representation §2.1): `&` exists
to mutate a value and give it back, the opposite of a transfer, so `fillInt(&x)` leaves `x`
holding the new value, never a taken slot.

**References cannot cross a spawn boundary.** A `&` argument may not cross into a spawned task,
a compile error (concurrency §2.1), because it would share a mutable slot between the spawner
and the task. It is the only case that needs forbidding: a closure cannot carry a reference to a
binding at all (its environment is a deep-`const` snapshot, functions spec §2.1), so there is no
capture-shaped path for a mutable slot to cross, and **any closure is spawnable**. A task
communicates a result by **returning** it (concurrency §2.2), never through a shared reference.
(The one referential capture in the language, `use` of a capability, *does* cross, since a
capability is immutable and there is no mutable slot to race on.)

### 5.2 The `copy` operator

`copy` produces an independent **deep** copy. For a table, `copy` is applied
recursively to every element. Passing `copy x` hands the callee its own value, so
in-place mutation cannot reach the original:

```luna
let myTable = [];
var fillTable = fn (&myTable: table) => myTable['element'] = 1;
fillTable(copy myTable);
println(myTable);                  // []
```

For a stream, `copy` captures the **current** state: the cursor position at the
moment of the copy, not the stream's origin:

```luna
let myStream = 0..10;
foreach (v in myStream.take(5)) { print(v); }     // 01234; consumes 5 elements of myStream
var myNewStream = copy myStream;                   // copy captures the current cursor (now at 5)
foreach (v in myNewStream.take(5)) { print(v); }   // 56789: copy starts where myStream left off
myNewStream = (copy myStream).reset();
foreach (v in myNewStream.take(5)) { print(v); }   // 01234: reset rewinds the copy
```

(`take(n)` yields a stream of the next `n` elements; consuming it advances the source
cursor by `n`, stream-api. There is no dedicated repetition operator.)

---

## 6. The `@` (type-of) operator

`@` returns the underlying `type` of a value: its current type, reported through the
`type` virtual table (so `@x.typeName` yields the type's name). It reflects the
value's *current* type, which for an absent key is `undefined` and for a stored null
is `null`:

```luna
var myTable = [];
println(@myTable['element'].typeName);   // "undefined": no such key
myTable.element = null;
println(@myTable['element'].typeName);   // "null": key exists, holds null
```

See the types spec for the full `type` surface.

---

## 7. Error summary

| Error | When | Detected |
|-|-|-|
| `reassignmentError` | Rebinding a `let` / `const`; assigning a spent write-once optional | compile (runtime when branch-dependent) |
| `writeOnceViolationError` | Second write to a write-once optional on a runtime path | runtime |
| `incompatibleTypeError` | Assigning a value outside the binding's declared type | compile |
| (const immutability) | Adding a key to, or overwriting a value in, a `const` (deeply immutable) table | compile where the target is static; on a dynamic path, a table-protocol violation at runtime (tables §5) |

(Reference-of-`let` and out-of-scope use are compile errors without dedicated names
here.)
