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
All variables in Luna are stored internally by the `lval` struct, which is always 16 bytes. 

```
struct lval {
  uint64_t typeAndFlags;
  void * dataPtr;
}
```

The `typeAndFlags` member contains information about both the type of variable and various flags. The lower 16 bits are reserved for flags, such as:

- isNull
- isNullable
- isVar
- isError

The upper 48 bits is the `typeid`. All types, including complex types, have their own unique `typeid`. This means the type `string|int` has a unique id, different from `int` and `string`. The types are stored in the global typetable, where is an array in which all elements are `typeinfo` structs. The `typeinfo` struct determines how to interpret the `dataPtr` in an `lval`. Nullable types are not seperate `typeinfo` structs; they contain their nullability as a flag, and their current null state as a flag. Attributes, too, are tied to the type and are stored in the `typeinfo` struct of a variable, if said variable has attributes. This means each unique combination of types and attributes has their own `typeinfo` struct. 

The typetable is not static. New types can be created at runtime, particularly because functions can throw. When that occurs, a new type is appended to the typetable describing the union of the previous type and whatever specific error was thrown. This has to be done because, while Luna has checked errors, the exact error subtype is not known at compile time. 

Some simple types like `int`, `double` and `bool` will not have their own `dataPtr` pointing to a place in memory. Instead, the value will be stored in-line inside the 8 bytes of the `dataPtr`.

Copying variables around then merely necessitates copying the 16 byte `lval` struct. When COW activates, then the data pointed to by the `dataPtr` is copied. 

For the typeof operator `@`, this merely returns the associated `typeinfo`. This is provided as a virtual table, as the underlying implementation is a struct. But, to the Luna programmer, the `type` type functions as a table.

Variables are automatically collected by the Go garbage collector. Individual types may manage their own internal memory: for example, `stream`, `table` and `string`. When they are about to be collected, they will need to free their internal allocations.
