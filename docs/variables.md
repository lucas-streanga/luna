# Variables
Variables are declared using the `let` and `var` keywords. The difference is that `let` is for *immutable* variables, whereas `var` is for *mutable* variables. There is no equivalent of const, as `let` covers this usecase and Luna will automatically apply constant access optimizations when appropriate. Here are some examples:

`let myName = 'Lucas';`

```
var myTable = [];
myTable.name = 'Lucas';
myTable['lastName'] = 'Streanga';
```

```
var numbers: stream|table = 0..100;
io.println(@numbers.typeName); // prints "stream"
numbers.map(fn (number: int) => number * 2);
println("$num, ") foreach (numbers as num); // print 0, 2, 4, 6, 8 ...
println(numbers.consumed()); // prints "true"
numbers = 0..50 as table;
println(@numbers.typeName) //prints "table"
numbers = null; // throws IncompatibleTypeError: null cannot be assigned to variable of type "stream|table"
```

```
var myFile = openFile('myfile.txt', File.modeRead);
println(@myFile.typeName); // prints "File"
myFile = myFile.lines();
println(@myFile.typeName); // prints "stream"
```

```
let numbers = 0..100;
numbers = 0..50; // throws ReassigmentError: cannot reassign a variable declared with "let"
```

```
let numbers?: stream = null;
numbers = 0..100; // OK: optional variables declared with "let" may be assigned a value which is not null exactly once. After, they may never be reassigned
numbers = 0..50; // throws ReassigmentError: cannot reassign a variable declared with "let"
```

```
let numbers = 0..5 as table;
println(numbers); // prints "[0, 1, 2, 3, 4, 5]"
```

```
var myTable = [];
println(@myTable['element'].typeName); // prints "undefined"
myTable.element = null;
println(@myTable['element'].typeName); // prints "null"
```

```
foreach (0..10 as num) {
  let i = num; // OK: variables are lexigraphically scoped
}
println(i); // Compile-time error: i is undefined in this scope
```

# Important Notes
- variables are lexigraphically scoped, as is typical in modern programming languages. But, this differs from PHP, where variables are function-scoped.
- Optional variables declared with `let` which are initially assigned `null` may be reassigned exactly once, and never again.
- The `@` operator is the "type of" operator, and will return the underlying `type` of any variable. Please see the "types" section for more info.

# Passing Semantics
By default, all variables use *pass-by-value* semantics when passed to a function, with the sole exception of `stream` variables. Internally, the pass-by-value is performed as copy-on-write. Meaning, technically, a reference is passed, and on first mutation a memory copy occurs. This fact is opaque to the programmer, and they need not worry about it.

Variables may be passed by reference using the *reference operator*, `&`. This operator works both on the caller and callee side. 

```
var myTable = [];
fillTable = fn (&myTable: table) => myTable.set('element', 1);
fillTable(myTable);
println(myTable); // prints [1]
fillTable = fn (myTable: table) => myTable.set('element', 1);
myTable = [];
fillTable(&myTable);
println(myTable); // prints [1]
```

As stated above, streams are pass-by-reference. This is because streams are consumable.

```
let myStream = 0..10;
myStream.map(fn (num: int) => num * 2);
println(myStream.consumed()); // prints "true"
println(num) foreach (myStream as num); // prints nothing, stream is consumed
```

We may explicitly call for a variable to be copied. This is done with the `copy` operator. For all types, this immediately performs a deep copy. Meaning, for tables, copy is recursively called for every element.

```
let myTable = [];
fillTable = fn (&myTable: table) => myTable.set('element', 1);
fillTable(copy myTable);
print(myTable); // prints "[]";
```

Bear in mind, for streams, the *current* state of the stream is copied. Not the beginning state.

```
let myStream = 0..10;
println(myStream x 5); // prints 01234
var myNewStream = copy myStream;
println(myNewStream x 5); // prints 56789
myNewStream = (copy myStream).reset();
println(myNewStream x 5); // prints 01234
```

# Notes: internal representation

How Luna stores variables at runtime: the `lval`, the `typetable`, and where each
piece of per-value, per-type, and per-binding state lives.

---

## 1. The `lval`

Every variable is an `lval` — always exactly 16 bytes.

```
struct lval {
  uint64_t typeAndFlags;   // 8 flag bits + 56 typeid bits
  void*    dataPtr;        // pointer to payload, or the payload itself (inline)
};
```

- **`typeAndFlags`** packs a small set of per-value flags (low 8 bits) with the
  `typeid` (high 56 bits).
- **`dataPtr`** either points to the payload or, for small scalars, *is* the
  payload. `int`, `double`, and `bool` are stored inline in the 8 bytes — no
  allocation, no indirection. Larger or managed types (`string`, `table`, `stream`)
  point to their own memory.

Copying a variable copies the 16-byte `lval` and nothing else. The payload is
duplicated only when copy-on-write is triggered by a mutation.

---

## 2. Flags (low 8 bits)

The flag byte holds **only per-value dynamic state** — properties that can differ
between two `lval`s of the same type, and that change over a value's lifetime:

| Flag | Meaning |
|-|-|
| `isNull` | This value is currently null. |
| `isUndefined` | This slot currently holds `undefined` (absent). |

`isNull`, `isUndefined`, and "holds a real value" are mutually exclusive — together
they are a 3-state condition, so they cost far less than the byte allotted. The
remaining bits are reserved for future per-value state.

### 2.1 What deliberately is *not* a flag

Three properties that look flag-like belong elsewhere, because they are not
per-value:

- **Nullability** (declared `T?`) is a property of the **declared type**, identical
  for every value of that type. It lives in `typeinfo` (§4), not the `lval`. Keeping
  it in the flag byte would let it drift out of sync with the type.
- **Mutability** (`var` vs. a fixed binding) is a property of the **binding**, not
  the value. It lives in the symbol/slot. If it rode in the `lval`, copying a value
  by value would wrongly carry mutability across into a fixed slot.
- **Error-ness** is **derivable** from the `typeid`: a value is an error iff its
  current type descends from the error type. Storing it as a flag denormalizes that
  and invites disagreement with the id.

This keeps the flag byte to genuinely dynamic, non-derivable, per-value bits, which
is a small and slow-growing set.

---

## 3. The `typeid` (high 56 bits)

Every type has a unique `typeid`, including complex types: `string | int` has its
own id, distinct from `string` and from `int`. The id indexes the global
`typetable`.

> The internal layout of these 56 bits — whether they hold a single id or are split
> to carry a value's declared and current type together — is decided separately (see
> the variable-typing discussion). This document describes everything that is
> independent of that choice.

---

## 4. The `typetable` and `typeinfo`

The `typetable` is a global array of `typeinfo` structs. A `typeinfo` says how to
interpret the `dataPtr` of any `lval` bearing its id, and carries the type-level
properties:

- **Nullability.** Whether the declared type admits null. (The *current* null state
  is the `lval`'s `isNull` flag; the *capacity* to be null is here.)
- **Attributes.** Attributes are tied to the type. Each distinct combination of base
  type and attributes is its own `typeinfo`, and therefore its own id.

### 4.1 Runtime growth

The `typetable` is **not static** — entries are appended at runtime. The main driver
is checked errors: Luna knows *that* a call may throw, but not the exact error
subtype at compile time. When a call throws through a site whose value has type `T`,
a new entry describing `T | E` (the union with the thrown error) is appended.

New-type creation must **intern**: the same structural type maps to the same id
every time, or the table would grow without bound. `T | E` resolves to one id, not a
fresh id per throw.

---

## 5. The `type` type and `@`

The typeof operator `@` returns the `typeinfo` associated with a value. The
underlying implementation is a struct, but it is exposed through a virtual table so
that, to the Luna programmer, a `type` behaves like an ordinary `table`.

---

## 6. Memory management

`lval`s are collected by the Go garbage collector. Types that own internal
allocations — `string`, `table`, `stream` — free those allocations when collected.
Because ordinary copies only duplicate the 16-byte `lval`, and payloads are shared
until COW, a value's managed memory has a single owner responsible for release at
collection time.
