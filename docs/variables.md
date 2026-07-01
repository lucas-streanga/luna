# Variables
Variables are declared using the `let` and `var` keywords. The difference is that `let` is for *immutable* variables, whereas `var` is for *mutable* variables. There is no equivalent of const, as `let` covers this usecase and Luna will automatically apply constant access optimizations when appropriate. Here are some examples:

`let myName = 'Lucas';`

```
var myTable = [];
myTable.name = 'Lucas';
myTable['lastName'] = 'Streanga';
```

```
var myTable = [];
myTable.name = 'Lucas';
myTable.close();
myTable.lastName = 'Streanga'; // throws OpenViolationError: cannot add new elements to a closed table.
myTable.open();
myTable.lastName = 'Streanga'; // OK
myTable.neverOpen();
myTable.age = 0 // Throws OpenViolationError: cannot add new elements to a closed table.
myTable.open() // Throws InvalidOpenError: cannot open a table which has been set as "never open"
myTable.setGet('lastName'); // OK: no change, 'lastName' was already readable
myTable.noSet('lastName');
myTable['lastName'] = 'new'; // Throws TableMutationViolationError: 'lastName' is not mutable
myTable.noGet('lastName');
println(myTable.lastName); // Throws TableReadViolationError: 'lastName' is not readable
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
