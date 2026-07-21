# Types

A compact index of the types Luna has so far, and where each is specified. This is an
overview, not a definition; each type's spec is authoritative.

---

## Primitive and value types

| Type | What it is | Spec |
|-|-|-|
| `int` | 64-bit signed integer, inline; overflow panics | int |
| `double` | 64-bit IEEE 754 float, inline; IEEE semantics (inf/nan, no throw); never a key | double |
| `float` | 32-bit IEEE 754 float (separate primitive, lower precision) | numeric-tower §1.3 (own spec deferred) |
| `string` | Immutable, valid-UTF-8 text | string, string-api |
| `bool` | `true` or `false`, inline; no truthiness; conversions are functions | bool |
| `null` | The explicit "present nothing" value | value-representation, coalescing |
| `regex` | Compiled regular expression (own type, `/.../ ` literal) | regex |
| `command` | Structured, inert program/pipeline (own type, backtick literal) | command |
| `secret` | Sensitive `string`/`bytes` payload, redacts everywhere, `reveal` to extract | secret |
| `bytes` | Packed, mutable, growable byte buffer (own type, not a table) | bytes |
| `byte` | An `int` constrained to `0..255` (the element of `bytes`); a constraint instance | constraints, bytes |
| `duration` | A span of time, nanosecond-backed; dimensional arithmetic | std.time §2 |
| `instant` | A monotonic point in time; `instant - instant` is a `duration` | std.time §3 |
| `type` | A type as a first-class value (inline `typeid`; comparable, const; `@` yields one) | type |

`undefined` is the absence sentinel (a missing key or a void return), distinct from `null` (a present
nothing); it is language-produced (never written), storable but panics on use, and unstorable *in a
table*. See the undefined spec (and value-representation, coalescing).

The wider numeric set — `uint`, the sized-integer constraints, `f16`, and the built-in
`decimal`/`rational`/`complex` — is committed and fully specced (decimal/rational/complex
specs, R161/R162/R164), with delivery deferred past alpha (numeric-tower §6).

---

## Structured types

| Type | What it is | Spec |
|-|-|-|
| `table` | The general keyed/ordered structure; lists are tables | tables |
| `list` | A `table` whose keys are exactly `0..n-1` (a refinement of `table`, `list <: table`) | tables §2.1 |
| `sink` | The send end of a channel; `stream` is the receive end | channels §3 |
| `fn` | A function value (`fn` cannot throw; `fn!` may throw a declarable error) | functions |
| `stream` | A lazy, single-pass sequence (generator producer, `foreach` consumer) | stream |
| `promise` | A single future value from a spawned green thread; `await` collapses it to `T!` | concurrency |
| `stringBuilder` | A table with the applied `stringBuilder` protocol (the builder) | string-builder |

---

## Declaration forms (and their union supertypes)

Some forms declare **distinct member types** and have a **union supertype** over them. The
supertype is the natural "any member," the way `any` is the union of all types. These are
declaration forms, not type-theory "kinds"; Luna has no kind system.

| Form | Declares | Union supertype | Members are | Spec |
|-|-|-|-|-|
| `proto` | A protocol (`const p = proto {...}`, const-only) | `proto` | distinct protocols | protocols |
| `error` | An error type (`e = error {...}`) | `error` | distinct error types (the `panic` subtree, and declarable errors under the root) | errors |
| `capability` | A capability (`c = capability`) | `capability` | distinct capability types | capabilities |

- **`proto`** is the union of all protocols; a specific protocol (`stringBuilder`) is a
  member. A protocol is also usable as a type via `@proto` (the application type).
- **`error`** is the root and union of all error types; specific errors (`commandError`,
  `panic`, user-defined errors) are members, related by single inheritance.
- **`capability`** is the union of all capabilities; a specific capability (`revealSecret`,
  `io`) is a member. Capabilities are zero-data, nocopy, and reached only through `use`. They
  live in its defining module's exports (`revealSecret` from `std.secret`).

A related declaration form is **`constraint`**, which refines a base type by a pure predicate
(`byte = constraint i: int where i >= 0 && i <= 255`). A constrained type is a subtype of
its base (`byte <: int`), widens to the base implicitly, and narrows from it via `as`
(runtime-checked). Constraints are refinement types, checked at runtime, never solved
(constraints spec); `byte` and `list` are instances.

Another declaration form is **`enum`**, a discriminated union (tagged sum): `shape = enum {
circle: ['radius' => int], point, ... }`. A value is exactly one variant at a time, each carrying
at most one payload value (a table for structured data, or any other type). Named enums are
**nominal** (distinct by name); an anonymous enum written inline (`enum {a, b}`) is
**structural** (same variant set, order-independent, is the same type). Constructed with braces
(`{circle ['radius' => 5]}`, target-typed), discriminated with `match` (`{circle ['radius' =>
r]} => ...`); `@` yields the enum type (name if named, structure if anonymous), never erased
(enum spec).

A final declaration form is **`attribute`**, a static, compile-time-only data tag applied to a
declaration: `jsonTag = attribute ['tag' => string = '']`, applied with `#[jsonTag('user_name')]`
on a binding or a table-literal field. Unlike the forms above, an attribute is **not a type** and
does **not** appear in the type system: it has no runtime presence, is absent from what `@`
returns (so attributes never affect `@a == @b`), is transparent to assignability, and is readable
**only by comptime** (which consumes attributes to generate code, such as serializers). It is a
declaration-metadata sidecar, not a member of any type universe (attributes spec).

So `revealSecret <: capability`, `commandError <: error`, and `stringBuilder <: proto`, each a
specific type inside its form's union supertype, the same is-a relationship in all three.

---

## The top and bottom types

| Type | What it is | Spec |
|-|-|-|
| `any` | The top type: the union of all types; every value is an `any` | any |
| `never` | The bottom type: no values; `never <: T` for all `T`; identity for `\|` | never |

`any` is to all types what each union supertype above is to its form's members: the natural
"any of them." A specific type is always a member of `any`. `never` is its dual: the empty type,
the result of a function that does not return a value, `fn (): never` (exits or diverges,
runtime-guarded) or `fn (): never!` (always throws, checked as ordinary errorability). Because
`T | never` is `T`, non-returning branches drop out of union types cleanly (never spec).

---

## Type-adjacent operators

Not types, but how types are reached and tested:

- **`@x`** , two operators sharing a glyph, **resolved by grammatical position**, never by what `x`
  is (type spec §1.1). This is the same positional rule as `&`, `!`, `error`, and `comptype`.
  - **In type position** (after `:`, after `is` / `as`, after a pattern's `:`), `@P` is the
    **application refinement**: "a table guaranteed to have protocol `P` applied." A protocol is a
    shape-predicate, not a value-set, so it is not itself a type and `@` derives one. Everything
    that *is already* a type is written **bare, no `@`**: `int`, `string`, an enum name, a
    constraint name (`byte`), a function type (`fn (int): string`), and bare `fn` (any callable).
    Applying `@` in type position to something already a type (`@int`, `@someEnum`,
    `@(fn (int): string)`) is a **compile error**: there is no meta-type, and `@` has nothing to
    derive. This is the rule that makes `someEnum` (a type, bare) and `@someProto` (a protocol,
    needs `@`) consistent rather than arbitrary, and it is why `_: @int` is rejected in a pattern
    (match §2), pattern-type position being type position.
  - **In value position** (an expression: `let t = @x`, `f(@x)`, `@a == @b`), `@` is **introspection**,
    "the type associated with this operand," always yielding a comparable `type` — with one
    statically-steered arm: on a **proto** binding it yields the type the proto *induces* (the
    application refinement, which is what makes `export const file = @fileDescriptor` an alias
    of the refinement; type §1.1, §5, R175). It applies to **any** value,
    including one that happens to be a type: `@someError` is its specific error type, `@f` is a
    function value's full type (`fn (int): string`, since function types are not erased, functions
    spec §3), and `@int` is `type`, since `int` is a `type`-valued binding. There is no error case
    here, only in type position.

  value-representation, protocols.
- **`@@x`** , protocol introspection over any value (`[]` off tables, R126; distinct from `@`; not
  used for enum variants or function signatures). protocols §8.
- **`x as T`** , checked **narrowing** (union to member, supertype to subtype), runtime-checked
  with a `typeError` (panic) on mismatch; never transforms a value and never needs `!`. Value
  *conversion* (parsing, formatting) is a function (`parseInt`, `toString`), not `as`. See the
  `as` spec.
- **`x is T`** , the **total boolean type test** (e.g. `e is commandError`): always a `bool`, never
  panics, never transforms, and does **not** narrow the tested binding. The non-asserting counterpart
  to `as` (which panics and yields the narrowed value). See the `is` spec.
- **`match`** , expression-operator selecting by pattern (value / `@type` / guard) over a
  scrutinee, or a guard chain with no scrutinee; value patterns use the total order (double
  §2.2), non-exhaustive matches yield `| undefined`. The strict form **`match!`** panics on
  fall-through instead (no `| undefined`), for closed case sets. match.
- **`lo..hi`** , a **range**: inclusive integer sequence, a `stream` in value position, a
  membership test in `match`; `lo..<hi` excludes the top. Not a type. range.
- **`list[a:b]`** , a **slice**: half-open, returns a new `list` (or `bytes`); `[a:]`, `[:b]`,
  `[:]` open-ended forms. Distinct from `..` (slices half-open, ranges inclusive). tables §2.5,
  bytes §4.
- **`moduleof name`** , a unary compile-time prefix operator giving the module a binding is
  defined in, as a `table` (path, etc.); operand is one identifier, not a member access or
  expression. modules §7.1.

---

## Deferred types

The core type inventory is specified, and the committed remainder is **delivery, not
design**: the extended numeric tower (`uint`, the sized smalls, `float`, `f16`, and
`decimal`/`rational`/`complex` — all specced, R161/R162/R164) lands post-alpha as new
typeids under existing rules (numeric-tower §6), except `uint` and the 16/32-bit
constraints, pulled into alpha by std.binary's read family (R187). What stays genuinely
later is API surface, not types: a stream decoder waits on need (typed multi-byte reads
landed — std.binary, R187).
