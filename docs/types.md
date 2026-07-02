# Types

A compact index of the types Luna has so far, and where each is specified. This is an
overview, not a definition; each type's spec is authoritative.

---

## Primitive and value types

| Type | What it is | Spec |
|-|-|-|
| `int` | Integer number | value-representation |
| `string` | Immutable, valid-UTF-8 text | string-representation, string-api |
| `bool` | Boolean | value-representation |
| `null` | The explicit "present nothing" value | value-representation, coalescing |
| `regex` | Compiled regular expression (own type, `/.../ ` literal) | regex |
| `command` | Structured, inert program/pipeline (own type, backtick literal) | command |
| `secret` | Sensitive `string`/`bytes` payload, redacts everywhere, `reveal` to extract | secret |
| `bytes` | Raw byte sequence | (deferred, not yet specified) |

`undefined` is the absence sentinel (a missing key), distinct from `null` (a present
nothing); it is unstorable and is covered in value-representation and coalescing.

---

## Structured types

| Type | What it is | Spec |
|-|-|-|
| `table` | The general keyed/ordered structure; lists are tables | tables |
| `list` | A `table` whose keys are exactly `0..n-1` (a refinement of `table`, `list <: table`) | tables §2.1 |
| `view` | A table seen through one applied protocol (single surface type) | views |
| `fn` | A function value (`fn` cannot throw; `fn!` may throw a `UserError`) | functions |
| `stream` | A lazy, single-pass sequence (generator producer, `foreach` consumer) | stream |
| `stringBuilder` | A table wearing the `stringBuilder` protocol (the builder) | string-builder |

---

## Declaration forms (and their union supertypes)

Some forms declare **distinct member types** and have a **union supertype** over them. The
supertype is the natural "any member," the way `any` is the union of all types. These are
declaration forms, not type-theory "kinds"; Luna has no kind system.

| Form | Declares | Union supertype | Members are | Spec |
|-|-|-|-|-|
| `proto` | A protocol (`p = proto {...}`) | `proto` | distinct protocols | protocols |
| `error` | An error type (`e = error {...}`) | `error` | distinct error types (under `Panic` / `UserError`) | errors |
| `capability` | A capability (`c = capability`) | `capability` | distinct capability types | capabilities |

- **`proto`** is the union of all protocols; a specific protocol (`stringBuilder`) is a
  member. A protocol is also usable as a type via `@proto` (the wearer type).
- **`error`** is the root and union of all error types; specific errors (`commandError`,
  `UserError`, `Panic`) are members, related by single inheritance.
- **`capability`** is the union of all capabilities; a specific capability (`reveal`, `io`) is
  a member. Capabilities are zero-data, nocopy, and reached only through `use`. They live in
  the `caps` module (`caps.reveal`), and a capability may be marked `implicit` to opt into
  silent inference (capabilities spec).

So `reveal <: capability`, `commandError <: error`, and `stringBuilder <: proto`, each a
specific type inside its form's union supertype, the same is-a relationship in all three.

---

## The top type

| Type | What it is | Spec |
|-|-|-|
| `any` | The union of all types; every value is an `any` | value-representation |

`any` is to all types what each union supertype above is to its form's members: the natural
"any of them." A specific type is always a member of `any`.

---

## Type-adjacent operators

Not types, but how types are reached and tested:

- **`@x`** , the current type of a value (`@someError` is its specific error type, `@proto`
  is a protocol's wearer type). value-representation, views.
- **`@@x`** , protocol reflection over a table or view. views.
- **`x as T`** , checked **narrowing** (union to member, supertype to subtype), runtime-checked
  with a `TypeError` (panic) on mismatch; never transforms a value and never needs `!`. Value
  *conversion* (parsing, formatting) is a function (`parseInt`, `toString`), not `as`. See the
  `as` spec.
- **`x is T`** , subtype test / narrowing (e.g. `e is commandError`), the boolean, non-panicking
  counterpart to `as`. errors, value-representation.

---

## Deferred types

Referenced by existing specs but not yet defined: `bytes` (raw bytes, needed by the string
builder, `secret`, and stream decoding). Its spec will slot in here when written.
