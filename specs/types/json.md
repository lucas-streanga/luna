# JSON: the `json` type and `toJson`

Serialization output is not a bare `string`. Luna defines **`json`**, a constraint on
`string`, and the serialization functions return it. This file specifies the type, the two
serialization functions, and why the design is cheap.

---

## 1. The `json` constraint

```
const json = constraint str: string where isValidJson(str);
```

An ordinary **invariant constraint** (constraints §1, §7.1) over `string`: a value inhabits
`json` iff its text is valid JSON. Everything about it follows from the general constraint
rules; nothing here is a special case:

- **`json <: string`.** A `json` value is usable anywhere a `string` is expected, with no
  ceremony (widening is implicit, constraints §5). Returning `json` instead of `string`
  therefore costs consumers nothing: printing, concatenating, hashing, storing all work
  unchanged.
- **Value-carried** (constraints §9.2): the `json` typeid travels with the value through
  widening, so a `json` handed to `string`-typed code is still known-`json` when it comes
  back, `@s` reports `json`, and `s is json` is a typeid check, not a re-parse.
- **Equality erases** (equality §1): `someJson == "{}"` compares string contents, `json` and
  a string literal share the base `string`, so the constraint never makes equal texts
  unequal.
- **Entry-only checking.** `string` is an immutable base, so `json` is checked **on entry
  only** (constraints §7): construction into a `json`-typed position, assignment to a
  `json`-typed slot, and `x as json`. There is no mutation surface, an immutable string can
  never *become* invalid, so there is no per-mutation cost, no re-validation on read, ever.

### 1.1 Why not a bare `string`: the footgun

A serializer that returns `string` throws away the one fact it just established: that the
text is valid JSON. Downstream code must then either re-validate (paying the parse again,
possibly many times) or trust blindly (the footgun: any string, hand-built, concatenated,
truncated, flows into the same slots). Returning `json` keeps the fact **in the type**,
which is the recurring rule (variables §1.3): a `json`-typed parameter *demands* validity
and gets it checked at entry; a `json`-typed value *carries* validity and is trusted on
every read. The boundary idiom falls out of constraint semantics with no new machinery:

```
const body = networkInput as json;    // validate external text once, at the boundary
sendDownstream(body);                  // everything downstream demands and trusts `json`
```

### 1.2 The cost, precisely

"Very cheap" means **once per value, then free**, not zero:

- The entry check runs `isValidJson`, a full O(n) parse of the text. That is the honest
  price of the fact.
- It runs **at most once per value**: strings are immutable, so a value that entered `json`
  never re-checks, on read, on widening, on copy (copies carry the typeid, constraints
  §9.2).
- For serializer output (§2) the check is an O(n) pass appended to an O(n) operation, a
  constant factor, and the compiler may **elide** it where validity is provable (constraints
  §9.5); where it cannot prove it, the check runs, correctness over cleverness.
- `s is json` and `@s == json` after entry are O(1) typeid operations, never re-parses.

### 1.3 Predicate dependency

`isValidJson(str)` is a **function call inside a `where` clause**. This spec therefore
depends on constraints §11 ("predicate expressiveness") resolving to admit calls to **pure,
comptime-eligible functions** in predicates. `isValidJson` is exactly such a function, total
over `string`, no capabilities, deterministic, so it is the motivating instance for that
resolution rather than an exception to it.

---

## 2. `toJson`: the attribute-aware generator

```
const toJson = comptime fn (ct: comptype): fn (any): json;
```

`toJson` is the canonical comptime generator (attributes §4): it takes a **`comptype`**
descriptor (introspection §4.2), reads the declaration's fields and their `jsonTag` attributes,
extracts the key/tag pairs into **plain data**, and returns a runtime serializer that
`const`-captures that data (functions §2.1) and emits the tagged JSON directly:

```
const writeUser = toJson(comptype User);   // generated once, at compile time
let body = writeUser(someUser);            // body: json; tags compiled in, no reflection
```

The mechanics, guarantees, and enforcement are specified in attributes §4 and are not
repeated here; the load-bearing points: the return is `fn (any): json`, **not** a dependent
`fn (v: T)` (the specialization lives in the captured data, not the parameter's type);
capturing `ct` itself is a **compile error** (comptype confinement, introspection §4.2), so
what reaches runtime is always plain data; and the generated function's result enters
`json` under §1.2's cost rules.

---

## 3. `toJsonDynamic`: the structural walk

```
const toJsonDynamic = fn (v: any): json;
```

A plain, **comptime-eligible** function that serializes a value's structure **as it
stands**: it walks the value with `foreach` and `@`. This is trivially implementable with
no new machinery because the primitive set is **closed** (value-representation §4.1), a
runtime walk meets only known cases, and new *tables* introduce no new kinds of value.

Its contract differs from `toJson`'s **observably**, which is why the two are separate
functions and are never unified behind one name (attributes §4): `toJsonDynamic` is
**attribute-blind**, attributes are erased at runtime (attributes §1), so it emits actual
key names, never `jsonTag` renames. It is **phase-invariant** (functions §5.5), the same
value serializes to the same text whether the call folds at comptime or runs at runtime,
which a unified `toJson(ct: comptype | any)` could not satisfy.

Choose by contract: `toJson` is *"the declaration's serialization, tags honored"*;
`toJsonDynamic` is *"this value's structure, as it stands."*

---

## 4. Summary

| Surface | Phase | Attributes | Returns |
|-|-|-|-|
| `json` | both (a type) | n/a | , (constraint on `string`, entry-only check) |
| `x as json` | both | n/a | `json` , the boundary validation idiom |
| `toJson(comptype T)` | comptime (generator) | **honored** | `fn (any): json` |
| `toJsonDynamic(v)` | both (eligible, phase-invariant) | erased | `json` |

---

## 5. Open questions

- **The JSON grammar `isValidJson` accepts:** which standard (RFC 8259), and whether
  extensions (trailing commas, comments) are rejected, expected: strict RFC 8259, pending
  confirmation.
- **Number fidelity:** how `int` and `double` values round-trip through JSON numbers
  (precision limits, `nan`/infinity handling, which JSON has no representation for),
  pending the numeric-tower treatment.
- **Parsing (`fromJson`):** deliberately not specified here; this file covers the `json`
  type and serialization only. A parser's output typing (tables of `any`? demanded shapes?)
  is a separate design.
