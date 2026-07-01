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
println(numbers.exhausted()); // prints "true"
numbers = 0..50 as table;
println(@numbers.typeName) //prints "table"
numbers = null; // throws IncompatibleTypesError: null cannot be assigned to variable of type "stream|table"
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
  let i = num; // OK: variables are lexigraphy scoped
}
println(i); // Compile-time error: i is undefined in this scope
```
