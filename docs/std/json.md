# `std.json`

```
import std.json;
```

The JSON module: the `json` type, the writers, the reader. Pure throughout, no capability;
everything here is comptime-eligible (functions §5.5). The per-format module layout is
deliberate: importing a JSON parser never pulls an XML one.

## 1. The `json` type

```
export const json = constraint { string as str where isValidJson(str) };
```

`json` is an ordinary **invariant constraint** (constraints §7.1) over `string`: a `json`
value *is* a string (`json <: string`, widening implicit), carrying the validated commitment
that its content is well-formed JSON. `string` is an **immutable base**, so the constraint is
**entry-only** (constraints §7): the predicate runs where a value becomes `json`,
construction, assignment into a `json` slot, `as json`, and never again. One validation at
the boundary, zero cost thereafter, elided where validity is provable (constraints §9.5),
which includes the generator's by-construction-valid output (§2). `isValidJson` is pure and
capability-free; its exact definition is deferred (§4) but its meaning is fixed:
well-formedness per the JSON grammar, nothing more.

### 1.1 Why not a bare `string`

Returning `string` from a serializer is a footgun in both directions. Downstream, any string
flows where JSON was meant and the mistake surfaces at the consumer, far from the cause.
Upstream, composition loses information: building a document from parts cannot tell
rendered-JSON parts from raw strings, so it must either re-escape everything (corrupting
embedded JSON) or escape nothing (injection). With `json` as a type, both directions are
typed: a `json` slot rejects a raw string, and an **embedder splices a `json` value
verbatim, no re-escaping**, while a `string` in the same position is escaped as data. **The
type is the escaping decision.** Every format module follows this pattern.

## 2. Writing

```
export const toJson = comptime fn (ct: comptype): fn (any): json;   // generated, tags honored
export fn toJsonDynamic(v: any): json;                              // structural, tags erased
```

- **`toJson`** is the attribute-aware **generator** (attributes §4): it walks the
  descriptor's `fields`, reads each `jsonTag`, extracts the key/tag pairs into plain data,
  and returns a runtime serializer that const-captures that data (functions §2.1). The
  return is `fn (any): json`, no dependent signature; `comptype` confinement guarantees the
  descriptor never reaches the closure (reflection §3.2, compiler §6).

  ```
  const writeUser = toJson(comptype User);   // generated once, at compile time
  let doc: json = writeUser(someUser);       // runtime: tags compiled in, output typed
  ```

- **`toJsonDynamic`** is the attribute-blind **structural walk**: `foreach` over the value,
  `@` per element, the closed primitive set (value-representation §4.1) making the case
  analysis exhaustive. It serializes the value **as it stands**, actual keys as names,
  attributes having no runtime existence (attributes §1). It is phase-invariant (functions
  §5.5).

They are **deliberately two names** (attributes §4): the paths differ in observable output
(tags honored versus erased), and one function over `comptype | any` would be callable in
both phases while phase-divergent, which functions §5.5 forbids. **`toJson` serializes the
declaration; `toJsonDynamic` serializes the value.**

## 3. Reading

```
export const fromJson = fn (j: json): table!;
```

- **The parameter is the constraint, and the constraint is the validation.** Callers reach
  `fromJson` through an entry into `json`, typically `raw as json`, narrowing and validating
  in one step; inside, the parser trusts well-formedness, and the `!` arm carries only
  *semantic* failures the predicate does not cover (§4).
- **Pure and comptime-eligible**: `const cfg = fromJson('{"port": 8080}' as json)` is a
  build-time constant.
- **Any source.** File-to-table is a composition with `std.io`, not an export here:
  `try fromJson(readAll(fd) as json)`, the `as json` narrowing `readAll`'s
  `string | bytes` union and running the format entry in one visible step at the seam.

## 4. Open questions

- **`isValidJson` edges**: exact grammar profile (trailing bytes, duplicate keys, number
  precision).
- **Document shape**: what a scalar-rooted document (`"5"` is valid JSON) yields when
  `fromJson` promises `table!`, a one-element wrapper or a declarable error.
- **`fromJson` into declared shapes**: parsing into a protocol-typed table (the inverse of
  the `toJson` generator), pending the reflection pipeline's read side.
