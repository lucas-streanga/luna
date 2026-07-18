# `any`

`any` is the runtime-representable **top type**: every value is an `any`, and an `any`-typed
position accepts everything (except `comptype`, which lives outside the runtime universe,
introspection §4.2). This spec fixes what you can *do* with an `any`-typed value, the F22
question, and the answer is surgical rather than total: **universal operations work,
type-specific operations require narrowing first.**

## 1. The universal operations, always available on `any`

These are the operations defined for *every* value, so no narrowing is needed and none is
asked for:

- **Store, pass, return, bind**: `any` flows anywhere `any` is declared.
- **`==` / `!=`**: total over all values (typeid-first, equality spec).
- **`is T`**: the type test, `any`'s natural companion (is spec).
- **`as T`**: the checked narrowing, the door out of `any`.
- **`match` with typed bindings**: the structured door out (`match (v) { n: int => ... }`).
- **`@v`**: the type of the value, always answerable.
- **`@@v`**: the value's applied protocols, always answerable — `[]` for any non-table,
  the same "no" that `v is @P` gives (protocols §8, R126).
- **`toString(v)` and interpolation** (`"$v"`): rendering is total (conversion §3).
- **`copy v`**, crossing `spawn` (deep copy applies per the actual value's class).

## 2. Type-specific operations are compile errors on `any`

Everything whose meaning depends on the operand's type is **disallowed on `any`
statically**, narrow first:

- **Arithmetic and ordering** (`+`, `<`, ...): a compile error on an `any` operand, never a
  runtime dispatch. `(v as int) + 1`, or match.
- **Indexing and member access** (`v[k]`, `v.name`): requires a table (or other indexable);
  `(v as table)['k']`.
- **UFCS calls to typed functions**: `v.trim()` resolves `trim(v)`, whose parameter is
  `string`; `any` does not fit `string`, same per-position assignability as any call
  (functions §3.2, §3.4). Only functions genuinely typed `fn (any)` accept it, which is
  exactly right: those are the functions that promised to handle anything.
- **`&v`**: references are invariant (variables §5.1); a `&any` is a different thing from a
  `&table` and neither converts.

## 3. Why not disallow `any` operations entirely, and where that would break

Total disallowal was considered (make even `==` and interpolation require narrowing) and
rejected, because the breakage lands on the language's own flagship paths:

- **Heterogeneous table iteration**: `foreach (v in t)` binds `v: any` for any untyped
  table, the most common loop in PHP-lineage code. Under total disallowal, `println("$v")`
  and `v == sentinel` inside that loop would each demand a narrowing that adds nothing, the
  operations are total anyway.
- **`toJsonDynamic` and every structural walk** (std.json §2): walking with `@`, `==`, and
  `match` *is* the universal-operations set; removing it removes the dynamic tier.
- **Errors and logging**: `error.data` payloads and log arguments are `any` by nature;
  rendering them must not require a match per call site.

Meanwhile the *type-specific* disallowal (§2) breaks nothing that was sound: code doing
arithmetic on an `any` was either narrowing anyway or hiding a bug, and the narrowing idiom
(`as`, or a `match` arm binding a typed name) is the language's own no-flow-typing answer,
one explicit step, checked once. So the rule in one line: **`any` supports exactly what is
total; everything else names its type first.**

## 4. Note on `any` in signatures

`fn (v: any)` is an honest signature, "handles every value," and such functions are the
legitimate consumers of unnarrowed `any` (they typically begin with a `match`). It is not a
loophole: inside, `v` obeys §1–§2 like any other `any`.
