# `std.json`

```
import std.json;
```

The JSON module: the `json` type, the writers, the reader. Pure throughout, no capability;
everything here is comptime-eligible (functions §5.5). The per-format module layout is
deliberate: importing a JSON parser never pulls an XML one.

## 1. The `json` type

```
export const json = constraint { str: string where isValidJson(str) };
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
export const toJson = comptime fn (ct: comptype, skipFunctions: bool = false,
                                   revealSecrets: fn? = null): fn (any): json;   // generated, tags honored
export fn toJsonDynamic(v: any, skipFunctions: bool = false,
                        revealSecrets: fn? = null): json;                        // structural, tags erased
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

### 2.1 What serializes; functions do not

Both writers emit the value's **serialization surface** (protocols §5): its element space
plus, for each applied protocol, that protocol's `get`-granted **per-table** members.
Definition-fixed members are protocol facts, not value facts, and never serialize;
ungranted members are private and never serialize — emitting them would disclose state
that cannot round-trip, since only granted members are apply-initializable (protocols
§4.2). The JSON *shape* protocol members take in the output is open (§4).

**Function values do not serialize** (R96): a `fn` has no data representation, cannot be
deserialized, and emitting one would be a disclosure hazard. A `fn` anywhere in the
surface — a fn-valued element, or a fn-typed `get` member such as `stringify`'s renderer
(conversion §3) — raises `typeError`. `skipFunctions: true` omits fn-valued slots
instead: per call on `toJsonDynamic`, baked into the generated writer on `toJson`.

**Secrets render as `'<secret>'`** — every payload kind alike (secret §4, R113).
Serialization is a display path, and concealment-on-display is the secret's *designed*
behavior, so the placeholder is not an error (contrast fn values, a structural
impossibility) and needs no skip flag. Lossy by design: deserializing yields the literal
string.

**Revealed serialization is explicit, twice over** (R113): both writers take
`revealSecrets: fn (s: secret): string | bytes | table = null`. When supplied, each
secret in the surface is passed to the revealer — a closure created where its gates are
declared, invoked under the call's frame grant — so the idiomatic call **delegates at
the site** (capabilities §5.2):

```
toJson(cfg, revealSecrets: fn (s: secret): string | bytes | table =>
    canReveal(s) ? reveal(s) : '<secret>') use (dbCred, revealStackTrace)
```

A revealed `table` is serialized recursively (secrets inside it meet the same revealer);
revealed `bytes` route through `string.fromBytes` (bytes §5) — the one place the
revealer path can error beyond fn values; and a revealer may decline by returning
`'<secret>'` (the `canReveal` idiom keeps mixed-gate structures panic-free, secret §5).

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
  `fromJson(readAll(fd) as json)` (propagating; `try` to recover), the `as json` narrowing `readAll`'s
  `string | bytes` union and running the format entry in one visible step at the seam.

## 4. Open questions

- **`isValidJson` edges**: exact grammar profile (trailing bytes, duplicate keys, number
  precision).
- **Document shape**: what a scalar-rooted document (`"5"` is valid JSON) yields when
  `fromJson` promises `table!`, a one-element wrapper or a declarable error.
- **`fromJson` into declared shapes**: parsing into a protocol-typed table (the inverse of
  the `toJson` generator), pending the reflection pipeline's read side — reconstruction is
  `apply` plus initializers over the granted surface (protocols §4.2, §5).
- **Protocol-member nesting**: the JSON shape serialized protocol members take (nested
  under the protocol name, flattened into the top object, or tagged), and the reverse on
  the read side.
