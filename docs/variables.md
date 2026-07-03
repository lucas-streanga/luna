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

```
let myName = 'Lucas';
```

```
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
  does *not* widen on later assignment. Use `any` to opt out of static typing.

```
var numbers: stream|table = 0..100;
println(@numbers.typeName);        // "stream"
&numbers.map(fn (number: int) => number * 2);
println("$num, ") foreach (numbers as num);   // 0, 2, 4, 6, 8 ...
println(numbers.consumed());       // "true"
numbers = 0..50 as table;
println(@numbers.typeName);        // "table"
numbers = null;                    // compile error IncompatibleTypeError:
                                   // null is not a member of stream|table
```

Because an unannotated `var` fixes its inferred type, changing a value's *kind*
requires either declaring the union up front or opting into `any`:

```
var myFile: File|stream = openFile('myfile.txt', File.modeRead);
println(@myFile.typeName);         // "File"
myFile = myFile.lines();           // OK: stream is a member of File|stream
println(@myFile.typeName);         // "stream"

var f = openFile('myfile.txt', File.modeRead);   // inferred type: File (fixed)
f = f.lines();                     // compile error IncompatibleTypeError:
                                   // stream is not a member of File
```

### 1.1 Rebinding a `let` or `const`

Rebinding a fixed binding is a **compile error** (`ReassignmentError`; compile errors
will carry codes):

```
let numbers = 0..100;
numbers = 0..50;                   // compile error ReassignmentError:
                                   // cannot rebind a variable declared with let
```

### 1.2 Write-once optional `let`

An optional `let` initialized to `null` may receive **exactly one** non-null
assignment, after which it behaves like any other `let`:

```
let numbers?: stream = null;
numbers = 0..100;                  // OK: the single permitted write
numbers = 0..50;                   // compile error ReassignmentError
```

The single write must be non-null; once it is spent, *every* further assignment -
including `numbers = null`, is rejected. Where the double write is statically
evident (straight-line code, as above) it is a compile-time `ReassignmentError`;
where it is branch-dependent it cannot be seen statically, so it is tracked with a
runtime flag and raises `WriteOnceViolationError`:

```
let handle?: stream = null;
if (a) handle = openA();
if (b) handle = openB();           // runtime WriteOnceViolationError if a and b both hold
```

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

`const` is `let` plus deep immutability. On a table it applies the table protocol's
seals, `neverOpen` (no new keys) and `neverThaw` (no overwrites), **recursively**,
so nested tables are sealed too. On a stream it freezes the cursor: a `const` stream
cannot be consumed. Any mutation attempt raises the corresponding table-protocol
error.

```
const config = [db => [host => 'localhost']];
config.db.host = 'remote';         // FreezeViolationError: const is deep
config.timeout = 30;               // OpenViolationError: const seals growth too
```

`const` binds an **independent** value; it never seals a value still in use
elsewhere. Const-binding a value that is currently shared conceptually copies it to
the new binding and seals the copy, leaving the original untouched:

```
var original = [count => 0];
const snapshot = original;         // snapshot is a deep, sealed copy
original.count = 1;                // OK: original is an ordinary var
snapshot.count = 1;                // FreezeViolationError: snapshot is const
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
  never mutate a closure's contents through the name that holds it. (Its only mutable
  state is whatever it captured by reference with `use`, and that is mutated through the
  *captured* binding, not through the function binding.) So `let f = fn ...` and `const f
  = fn ...` are equivalent.

This is not a special case for any of these types; it falls out of the general rule that
`let`-versus-`const` is exactly the presence or absence of interior freezing, which is
vacuous when there is no interior. Both spellings are therefore permitted for such
values, and mean the same thing.

**Convention**: for functions specifically, prefer `const` (a function is normally a
fixed definition, functions spec §1). `let f` is allowed and identical in meaning, but
`const f` reads as the intended "this name is a fixed function." The choice is stylistic,
not semantic.

---

## 4. Scoping

Variables are **lexically (block) scoped**, as is typical in modern languages, and
unlike PHP, where variables are function-scoped.

```
foreach (0..10 as num) {
  let i = num;                     // scoped to the loop body
}
println(i);                        // compile error: i is undefined in this scope
```

---

## 5. Passing semantics

By default all variables are passed **by value**, implemented as copy-on-write:
technically a reference is passed and the payload is copied on first mutation. This
is opaque to the programmer. The sole exception is `stream`, which is always passed
**by reference** because streams are consumable.

### 5.1 The reference operator `&`

`&` passes by reference and may be written on either the caller or the callee side.
A function that mutates its argument in place takes (or is given) a reference:

```
var myTable = [];
var fillTable = fn (&myTable: table) => myTable['element'] = 1;   // & on the callee
fillTable(myTable);
println(myTable);                  // [element => 1]

fillTable = fn (myTable: table) => myTable['element'] = 1;        // & on the caller
myTable = [];
fillTable(&myTable);
println(myTable);                  // [element => 1]
```

**`&` requires a `var`.** Applying it to a `let` or `const` binding is a compile
error. A single reference kind cannot distinguish "mutate through the reference" from
"rebind through the reference," and rebinding is what `let`/`const` forbid, so
references are restricted to `var`. To let a function mutate your value in place,
declare it `var`; a `let`/`const` value can still be passed by value (the callee
mutates only its own copy).

```
let t = [];
fillTable(&t);                     // compile error: cannot take a reference to a let binding
```

Because streams pass by reference, a method that extends the pipeline is visible to
the caller, and consuming the stream in the callee consumes it for everyone:

```
let myStream = 0..10;
&myStream.map(fn (num: int) => num * 2);
println(myStream.consumed());      // "true"
println(num) foreach (myStream as num);   // nothing: the stream is consumed
```

### 5.2 The `copy` operator

`copy` produces an independent **deep** copy. For a table, `copy` is applied
recursively to every element. Passing `copy x` hands the callee its own value, so
in-place mutation cannot reach the original:

```
let myTable = [];
var fillTable = fn (&myTable: table) => myTable['element'] = 1;
fillTable(copy myTable);
println(myTable);                  // []
```

For a stream, `copy` captures the **current** state: the cursor position at the
moment of the copy, not the stream's origin:

```
let myStream = 0..10;
println(myStream x 5);             // 01234
var myNewStream = copy myStream;
println(myNewStream x 5);          // 56789: copy starts where myStream left off
myNewStream = (copy myStream).reset();
println(myNewStream x 5);          // 01234: reset rewinds the copy
```

---

## 6. The `@` (type-of) operator

`@` returns the underlying `type` of a value: its current type, reported through the
`type` virtual table (so `@x.typeName` yields the type's name). It reflects the
value's *current* type, which for an absent key is `undefined` and for a stored null
is `null`:

```
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
| `ReassignmentError` | Rebinding a `let` / `const`; assigning a spent write-once optional | compile (runtime when branch-dependent) |
| `WriteOnceViolationError` | Second write to a write-once optional on a runtime path | runtime |
| `IncompatibleTypeError` | Assigning a value outside the binding's declared type | compile |
| `OpenViolationError` | Adding a key to a `const` (or otherwise sealed) table | see table spec |
| `FreezeViolationError` | Overwriting a value in a `const` (or frozen) table | see table spec |

(Reference-of-`let` and out-of-scope use are compile errors without dedicated names
here.)
