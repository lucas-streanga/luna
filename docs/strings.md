# String API

Some context on Luna:
- Luna has a form of uniform function call syntax. So, `string.func()` is equivilant to `func(string)`.
  - PS: UFCS and Tables: calling `table.func()` first searches members, and then outside functions.
  - PS: no overloading. It doesn't need to exist, because we have union types. The only "overloading" is the table semantics above
- Luna technically doesn't have free-form functions, only lambdas. So a named function is defined as:
```
const substring = fn (str: string): str => { ... // Implementation};
```

The above is due to the lack of namespaces, instead prefering modules. Only VARIABLES are exported from modules. So, all definitions must be a variable, typically `const` but not required.
Internally, the API will not be implemented in Luna, but rather in Go, and the functions will be made available in Luna virtually. 

## What the public string API should cover
- All obvious string functions
- A consistent naming scheme and signatures
- utf8 functionality, e.g. graphemes, codepoints, etc
- C-string support, e.g. `string.cString()` and `string.cStringLength()`
  - Note: the null-terminator is always included in length because of how strings work in Luna, which is why `cStringLength()` has to exist.
- Remember, string are immutable. No references allowed => compile time error and runtime error for complex types (remember, callers can use reference semantics)
