# High-level overview

Luna is a **data-focused**, statically-typed language. "Data-focused" means it emphasizes the
**shape of data** over type-level programming: the type system is expressive (unions, intersections,
refinements, protocols) but exists to describe and constrain data, not to be programmed in for its
own sake. Luna is **structurally typed** by default, so a value fits a type when its shape fits, and
it is **dynamic only where you ask for it**: the `any` type and runtime dispatch are available but
always explicit, never a silent fallback.

Two commitments shape everything else:

- **Small surface area.** Keywords are reused across contexts rather than multiplied, and the set of
  built-in types is kept deliberately small. New needs are met by composing existing pieces before
  adding new ones.
- **Safe by construction.** Luna is garbage collected (via the Go backend's collector) and offers no
  unsafe API, so memory is never the programmer's concern for correctness. Beyond memory: equality is
  strict, numeric conversions are explicit, integer overflow panics rather than wrapping, and
  concurrency shares no mutable state, so data races, silent coercions, and wraparound are closed off
  by the design, not by discipline. The memory model is fully documented even though it is never
  something you must manage.

## A taste

The canonical first program lives at the front of the spec (index); the shapes it shows,
functions as values, `use`-declared effects, `in`-bound iteration, postfix modifiers, are
this document's subject.


## The type set

Luna's types divide into value types, structured types, and declaration forms. This is a summary;
the types spec is the authoritative index, and each type has its own spec.

**Value types** (mostly primitive; several are their own type rather than a table):

| Type | What it is |
|-|-|
| `int` | 64-bit signed integer; overflow panics |
| `double` | 64-bit IEEE 754 float; Inf/NaN, never panics |
| `string` | immutable, valid-UTF-8 text |
| `bool` | `true`/`false`; no truthiness |
| `null` | the explicit "present nothing" |
| `bytes` | packed, mutable byte buffer |
| `byte` | an `int` constrained to `0..255` |
| `regex` | a compiled regular expression |
| `command` | a structured, inert program/pipeline |
| `secret` | a redacting sensitive payload (`reveal` to read) |
| `type` | a type as a first-class, comparable value |

`undefined` is the **absence** sentinel (a missing key), distinct from `null` (a present nothing) and
unstorable. The wider numeric set (`u64`, `float`, `f16`, sized-integer constraints, and the built-in
`decimal`/`rational`/`complex`) is committed but deferred (numeric-tower spec).

**Structured types:**

| Type | What it is |
|-|-|
| `table` | the general keyed, ordered structure; lists are tables |
| `list` | a `table` keyed exactly `0..n-1` (`list <: table`) |
| `fn` | a function value (`fn` cannot throw; `fn!` may) |
| `stream` | a lazy, single-pass sequence |
| `promise` | a future value from a spawned task; `await` collapses it |
| `view` | a table seen through one applied protocol |

**Declaration forms** introduce user-defined types: `proto` (a protocol), `enum` (a discriminated
union), `error` (an error type), `attribute` (compile-time declaration metadata), and `capability`
(a permission to reach an effect). Each is its own spec.

## Composing types: unions and intersections

Types combine with the **union** operator `|` ("one of") and the **intersection** operator `&`
("all of"):

```
var x: stream | fn = fn (): null => null;      // a stream or a function
var y: @proto1 & @proto2 = [] apply proto1, proto2;   // a table with both applied protocols
var z: (@proto1 & @proto2) | null = null;
```

Union and intersection are structural and canonical: `int | double` and `double | int` are the
same type, and `@Q & @P` is `@P & @Q`. Intersection is **general** (type spec §3.1), defined for
every pair by normalization, though the single-inheritance tree makes it collapse everywhere
except the multi-membership axes, protocol sets (`@P & @Q`, a table with both applied, how a table
composes capabilities) and constraint conjunctions (`byte & even`).

## Type inference

Types are inferred where they are obvious, and written where they add clarity or constraint:

```
var t = [];        // table
var d = 0.0;       // double
var n = 0;         // int
var s = '';        // string

const person = [
  'firstName' => 'Lucas',
  'lastName'  => 'Streanga',
  'age'       => 0,
];                 // a table; keys are quoted string literals
```

## Optionality and errorability, as type suffixes

- **`?` (optional)** adds `null` to a type: `var name?: string` is `string | null`.
- **`!` (errorable)** adds the error types a value may carry: a `fn!` may raise a declarable error (errors spec §2), and a
  binding written `x!` may hold an error alongside its value. `?` and `!` compose.

These are not special forms; they are shorthands over the same union machinery (`?` is `| null`),
which is the recurring pattern in Luna: new syntax tends to be sugar over one existing mechanism
rather than a new mechanism.

## `any`

`any` is the top type: it accepts any value, and it is the one place the type is not statically
known. Reaching into an `any` (its fields, its specific type) requires an explicit narrowing (`as`,
`is`, or `match`), so dynamism is available but never silent. Runtime reflection still works on an
`any` (you can ask its type name and kind), so `any` is inspectable rather than a dead end.

---

This overview is a map, not a definition. For any type or feature named here, its own spec is
authoritative; the types spec is the full index.
