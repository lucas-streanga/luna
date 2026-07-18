# Serialization: the `json` constraint and `toJson`

Serialization is the canonical consumer of attributes (attributes §4) and the proving ground
for the comptime pipeline (`comptype`, introspection §4.2). This spec fixes its two public
pieces: the **`json` type**, so serializers never return a bare string, and the **two
serialization entry points**, generated and dynamic.

## 1. `json` is a constraint on `string`

```
const json = constraint { str: string where isValidJson(str) };
```

`json` is an ordinary **invariant constraint** (constraints §7.1) over `string`: a `json`
value *is* a string (widening to `string` is implicit, `json <: string`), carrying the
validated commitment that its content is well-formed JSON. Because `string` is an
**immutable base**, the constraint is **entry-only** (constraints §7): the predicate runs
where a value becomes `json`, construction, assignment into a `json` slot, `as json`, and
never again, there is no interior to mutate and nothing to re-check. This is what makes the
type nearly free: one validation at the boundary, zero cost thereafter, and the compiler
elides even the entry check where validity is provable (constraints §9.5), which includes
the output of the generators below, whose emission is well-formed by construction.

`isValidJson` is a pure, capability-free predicate (constraints §2), so it is
comptime-eligible and the `json` constraint works identically in both phases.

### 1.1 Why not a bare `string`

Returning `string` from a serializer is a footgun in both directions. Downstream, any string
flows where JSON was meant, an error message, a user name, an unescaped fragment, and the
mistake surfaces at the consumer, far from the cause. Upstream, composition loses
information: building a larger document from parts cannot tell rendered-JSON parts from raw
strings, so it must either re-escape everything (corrupting embedded JSON) or escape nothing
(injection). With `json` as a type, both directions are typed: a `json` slot rejects a raw
string at compile time where static, at entry otherwise, and an **embedder can splice a
`json` value verbatim, no re-escaping**, while a `string` in the same position is escaped as
data. The type is the escaping decision.

## 2. The two entry points

```
const toJson = comptime fn (ct: comptype): fn (any): json;   // generated, tags honored
fn toJsonDynamic(v: any): json;                              // structural, tags erased
```

- **`toJson`** is the attribute-aware **generator** of attributes §4: it walks the
  descriptor's `fields`, reads each `jsonTag`, extracts the key/tag pairs into plain data,
  and returns a runtime serializer that const-captures that data (functions §2.1). The
  returned function is `fn (any): json`, no dependent signature, the specialization lives in
  the captured map, and `comptype` confinement guarantees the descriptor itself never reaches
  the closure (introspection §4.2, compiler §6).

  ```
  const writeUser = toJson(comptype User);   // generated once, at compile time
  let doc: json = writeUser(someUser);       // runtime: tags compiled in, output typed
  ```

- **`toJsonDynamic`** is the attribute-blind **structural walk**: `foreach` over the value,
  `@` per element, the closed primitive set (value-representation §4.1) making the case
  analysis exhaustive. It serializes the value **as it stands**, actual keys as names, since
  attributes have no runtime existence (attributes §1). It is a comptime-eligible plain
  function, phase-invariant (functions §5.5): the same value yields the same `json` at
  either phase, which is what permits folding it.

They are **deliberately two names, not one** (attributes §4): the paths differ in observable
output (tags honored versus erased), and a single function over `comptype | any` would be
callable in both phases while phase-divergent, exactly what functions §5.5 forbids. The
contracts, stated once: **`toJson` serializes the declaration; `toJsonDynamic` serializes
the value.**

## 3. Open questions

- **Parsing.** `fromJson(j: json): any` (or into a protocol-typed shape) is the inverse
  pipeline and is deferred; the `json` type is designed to be its natural input.
- **Other formats.** Whether the pattern generalizes as one generator per format
  (`toYaml`, ...) or a format-parameterized generator, pending use.
