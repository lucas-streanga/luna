# High-level Overview of Luna

Luna is a data-focused language. This means that de-emphasizes type programming (although Luna does have an expressive type system) and instead emphasizes the shape of data. Luna is structurally-typed mostly, and it is also statically-typed, although the type system allows both union and intersection (called "complex") types. In this way, Luna can be highly-dynamic, but only explicitly.

One major design goal of Luna is keeping the language surface area small. Keywords are often reused in different contexts, and Luna purposefully does not feature a large amount of built-in types. 

Luna is garbage collected, and does not offer an unsafe API. This means that the programmer need never worry about memory allocation. Although, the documentation will thoroughly explain the memory layout and characteristics of Luna. 

# Syntax overview

```
main = fn (table argv) use (&io): int => {
  die ('No files given') if argv.empty();
  files = argv.map(fn (filename: string) => openFile(filename, File.modeRead));
  foreach (files as file) {
    io.println("File size: ${file.byteSize}");
    foreach (file.lines() as lineNumber => line) {
      io.println("$lineNumber: $line");
    }
  }
  file.close() foreach (files as file);
  return ExitCodes.exitSuccess;
}
```

# Types

Luna features the following types:

| Type | Keyword | Can be user-defined? |
|-------|-|-|
| Function | fn | Yes |
| Stream | stream | Yes |
| Table | table | Yes |
| Protocol | proto | Yes |
| Enum | enum | Yes |
| Attribute | attribute | Yes |
| Error | error | Yes |
| Test | test | Yes |
| Type | type | Yes |
| Int | int | No |
| Double | double | No |
| String | string | No |
| Boolean | bool | No |
| Null | null | No |
| Undefined | undefined | No |
| Never | never | No |
| Any | any | No |

## Complex types
*Complex types* are types which are composed of combinations of the types listed above. Any variable may be a complex type. We can use the *Type Union Operator* `|` and the *Type Intersection Operator* `&` to define complex types. For example:

`let streamOrFn: stream|fn = fn () => null;`

`let proto1AndProto2: proto1&proto2 = [];`

`let proto1AndProto2OrNull: (proto1&proto2)|null = [];`

## Type deduction
Luna will try to automatically deduce types. Here's some examples:
`let myTable = [];`

`let myDouble = 0.0;`

`let myInt = 0;`

`let myString = '';`

```
let myPerson = [
  firstName => 'Lucas',
  lastName => 'Streanga',
  age: int => 0,
]; // myPerson is of type table
```

## Optional types
Variables postfixed with `?` are optional. Optional types are equivilant to writing `|null` at the top level of the type declaration.

`let name?: string = null; // name is of type string|null`

## Error types
Variables postfixed with `!` can bind to any `error` type. This may be combined with optional types.

`let myResult!?: table = null; // myResult is of type table|null|error1|error2...`

## The any type
The `any` type will bind to any type, with no restrictions

`let myAny: any = [];`

## The never type
The `never` type may not be assigned to a variable. It may only be used as a function return value, to indicate the function will never return, e.g. it throws an error or exits.

```
myNever = fn (): never => throw Error('We will never return from this function.');
```
