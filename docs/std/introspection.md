# `std.introspection`

Introspection is how a program asks questions about **declarations** — types, protocols,
constraints, enums, and function values (§4.6). It is a standard module of pure,
capability-free query functions over `type` and `proto` values, called by UFCS
(`t.typeName()` is `typeName(t)`, functions spec). Almost every program needs only the
built-in **operators** (§2) — `@`, `@@`, `declared`, `comptype`, `is`, `as` — which are
the hot path and stay language-level. The named queries here exist for the legitimately
rare needs: serializer generators, tooling, generic walkers. The module import is the
visible marker of that rarity.

**Why `introspection`, not `reflection`** (R127). Names carry the contract (conversion
§2's discipline). "Reflection" is the industry's name for the surface this module
refuses to be: runtime type mutation, accessibility overrides, method definition — the
PHP/Ruby capability that lets library code break language invariants. Luna's surface is
structurally read-only, and *introspection* is the read-only word. The name is the first
line of defense: the module's own name declines the feature requests.

---

## 1. The principles

Five pillars, each load-bearing. Everything in this module, and every future addition to
it, must satisfy all five.

1. **Introspection, never reconstruction.** Every export answers a question about what
   was declared; none builds, registers, or resurrects. Rebuilding a value is ordinary
   code — `apply` plus initializers (protocols §4.2, json §3, R125) — never an
   introspection feature.
2. **No name→value resolution, ever.** A string cannot summon a type, a proto, a
   function, or a capability. There is no registry and no `forName` anywhere in the
   language (R19 removed the last registry), and introspection preserves that: every
   query takes a **value you already hold**. The consequence is already visible
   elsewhere: `fromJson` cannot resurrect protocols from their serialized names
   (json §3, R125).
3. **Declarations, never values.** Introspection reads what was *declared* — members,
   grants, types, attributes, requirements — never any value's private state.
   Encapsulation stops introspection at the declaration boundary (R126): deep
   introspection of private state is exactly what encapsulation exists to prevent.
4. **Results are inert snapshots.** Every result is an ordinary value-semantic table or
   list with **no live link** back to the type universe (types are immutable and the
   universe is closed, value-representation §4.1). Mutating a result mutates a local
   copy and changes no type, ever. No freezing mechanism is needed; value semantics
   already make results safe, because there is nothing behind them to modify.
5. **Nothing grantable, nothing bypassed.** A query *about* capabilities returns inert
   descriptors — names, `@`-types, the `gatesOf` precedent (secret §3.3) — never
   capability values: introspection may **name** an effect, never **mint** the
   authority. And no query opens a side door: grants hold, constraints check, secrets
   stay sealed, and the closed protocol-member space stays closed — in particular,
   **dynamic member access by runtime name does not exist and never will** (§7).

A corollary worth stating as a fact rather than a choice: **the module is
capability-free, and that is a theorem.** Every export is a pure read of static
declaration data; a surface that can neither mutate nor resolve names has no authority
to protect, so there is nothing for a capability to gate.

---

## 2. Operators are language, functions are library

The split is exact:

| Built-in operator | Role | Spec |
|-|-|-|
| `@x` | the type of a value, a `type` | type spec |
| `@@x` | a value's applied protocols, a `list` of `proto` (total; `[]` off tables, R126) | protocols §8 |
| `declared x` | a binding's declared type, a `type` | type §4 |
| `comptype x` | a value's declaration descriptor (comptime-only, §4.2) | §4.2, keywords §3 |
| `x is T` | value-against-type test | is spec |
| `x as T` | checked narrowing | as spec |

The operators **produce** the values this module's functions **take**: `@` is how you
get a type, the functions are how you query it (`typeName(@x)`, `kindOf(@x)`); `@@` is
the protocol axis, `protoName` queries what it yields; `declared` gives the declared
type, so `unionMembers(declared x)` enumerates what a binding admits. `is` covers the
common subtype question (a value against a type); `isSubtype` (§4.1) is the type-to-type
form for when both operands are `type` values in hand, the rare reflective need — there
is deliberately no subtype operator (is spec §4).

Everything **named** is an import:

```
import { typeName, kindOf, members } from introspection;
```

**The import is an audit signal.** A file that does introspection says so on its first
lines — the same greppable-declaration property that `use` clauses give effects
(capabilities spec). Rarity made visible: reviewing a codebase for introspection use is
one grep of the imports.

---

## 3. The two tiers

Introspection comes in **two tiers**, and the tier is exactly the `fn`-versus-`comptime
fn` distinction (functions spec §5.3):

- **Runtime tier (`fn`)**: cheap, `typeid`-level facts (name, kind, nullability,
  subtype, union members) — O(1) reads of the `typetable` (value-representation §4,
  emitted as static data, compiler spec §7.2). Because a `typetable` lookup is cheap and
  always available, these work on **any** `type` value, including `@x` where `x`'s
  static type is `any` (so `@x` is only known at runtime). This is required, not
  incidental: without runtime introspection on dynamic values, `any` would be a dead end
  (you could hold an `any` but never ask what it is). The runtime tier makes `any`
  inspectable: a generic printer can ask a value's `typeName` and `kindOf` at runtime.
- **Comptime tier (`comptime fn`)**: deep, structural data (fields, attributes, enum
  variants, constraint predicate). These walk a declaration's **structure**, the
  expensive static-only work, so each call **folds at compile time** and **requires a
  compile-time-known `type` argument**. A `comptime fn` imported from a std module folds
  exactly as a builtin would: modules resolve at compile time (modules spec, R19), so
  the tier mechanism is untouched by the move into `std.introspection` (R127).

The tier boundary is not a special "introspection context"; it is ordinary
comptime-eligibility applied to `@`. What "compile-time-known type" means is about the
**type**, not the **value**:

- `@x` is a compile-time constant **iff `x`'s static type is statically known** — any
  binding with a declared or inferred concrete type. `fields(@x)` folds for
  `var x: int`, for `const y: byte`, because in every case the *type* is known at
  compile time. Const-ness of the value is irrelevant; static-ness of the type is
  everything.
- `@x` is a **runtime** type value (so a `comptime fn` on it is a **compile error**)
  only when `x`'s static type is `any` (or otherwise erased). `fields(@someAny)` is a
  compile error: "the type is not known at compile time" — first narrow the `any` with
  `as`/`is`/`match`, then introspect.

The tiers cooperate: the cheap runtime `kindOf` is the guard that makes the comptime-tier
calls safe — check `kindOf` (or `match` on it), then call the structural query that fits.
The canonical use, a serializer generator, does exactly this (§5).

---

## 4. The API

Runtime-tier exports are `fn`; comptime-tier exports are `comptime fn`. All results obey
§1's pillars: inert value-semantic snapshots of declarations.

### 4.1 Runtime tier

```
export const typeName = fn (t: type): string;
export const kindOf = fn (t: type): kind;
export const isNullable = fn (t: type): bool;
export const isSubtype = fn (t: type, of: type): bool;
export const unionMembers = fn (t: type): list;
export const baseOf = fn (t: type): type?;
```

- **`typeName(t)`**, the type's display name: `"int"`, `"byte"`, `"int | double"`. The
  universal query, used for output, logging, and generic debugging on any value
  (`typeName(@x)`). **Aliases never appear in the output** (R131): an alias is pure
  sugar (R21) — `iterable` and `table | stream` are *one* typeid — and one typeid has
  one name, so `typeName(iterable)` is `"table | stream"`. This is forced, not chosen:
  recording `"iterable"` would require the typeinfo to know which alias named it, and
  with two aliases for one type there is no sound answer. The alias name lives in
  source; the canonical structural spelling lives in output.
- **`kindOf(t)`**, which category the type is, returned as the **`kind`** enum (§4.3) so
  it is matchable. This is the dispatch primitive: `match (kindOf(t)) { {table} => ... }`
  branches on the type's category, and it is what guards the comptime-tier structural
  queries. The function is `kindOf` and the enum is `kind` — one name each (a module
  exports one binding per name), with `kindOf` joining the `*Of` query family
  (`gatesOf`; `baseOf` pending, §7) and lowercase `kind` matching the builtin
  convention (`TypeKind` was a PascalCase island — the R122 argument; R128).
- **`isNullable(t)`**, whether the type admits null (`T?` vs `T`), a flag read
  (value-representation §2).
- **`isSubtype(t, of)`**, whether `t <: of`, the subtype test of value-representation
  §4.2 as a function over two `type` **values**, dispatched by the shape of `of` exactly
  as `is` is (an interval check for a tree node, member decomposition for a union, the
  conjunction for an intersection, a pairwise-table lookup for a function signature,
  functions §3.2). The **only** type-to-type form (§2).
- **`unionMembers(t)`**, for a union type, the list of its member types
  (`unionMembers(int | double)` is `[int, double]`); for a non-union, the
  single-element list `[t]`. Runtime-tier because union membership is a flat `typetable`
  fact, not a structural walk. **The sugar decomposes** (R131): `?` is `| null` and `!`
  adds the error arm — sugar over the one union mechanism, never a new one — so
  `unionMembers(int!)` is `[int, error]` and `unionMembers(int?)` is `[int, null]`.
  No special case exists, because no special type exists.
- **`baseOf(t)`** (R131, subsuming the narrower `constraintBase` — one query, one
  question): the **immediate supertype the type refines**, or `null` where it refines
  nothing. The domain is every refining declaration form: a constraint's base
  (`baseOf(byte)` is `int`, `baseOf(list)` is `table`, `baseOf(json)` is `string`), an
  error type's parent (`baseOf(fileNotFound)` is `ioError`; `baseOf(error)` is
  `null` — the root refines nothing), an enum variant's enum (`baseOf(@Shape.circle)`
  is `Shape` — resolving enum.md's standing deferral, which asked for exactly this
  general form), and a refinement's or mixed intersection's base (`baseOf(@person)` is
  `table`). Atoms, unions, and `any` answer `null`: they refine nothing. A flat
  `typetable` read; the predicate itself stays comptime-tier (§4.2).

### 4.2 Comptime tier, and the `comptype` bridge

```
comptype v                                   // OPERATOR, not an export: the declaration descriptor
export const fields = comptime fn (t: type): list;
export const variants = comptime fn (t: type): list;
export const attributes = comptime fn (t: type): table;
export const constraintPredicate = comptime fn (t: type): fn;
```

- **`comptype v`** is an **operator and a keyword, not an export** (keywords §3): the
  bridge from a **value** to its **declaration descriptor**, a value of the built-in
  type **`comptype`**. Like `error`, the word is both the operator and the type's name,
  disambiguated by position (errors spec §3). A `comptype` value is a **nominal
  record**, read with ordinary `.` like an error's fields (errors §2.1), **not** a table
  and not a subtype of one (so it can never leak into runtime through a `table`-typed
  slot):

  ```
  (comptype v).type          // type: exactly @v, the ordinary, runtime-identical type
  (comptype v).fields        // list: the declared members, in the fields() row shape below
  (comptype v).attributes    // table: the declaration's attributes (attributes spec)
  ```

  `fields` and `attributes` are drawn from the value's **originating declaration**. This
  is what makes attributes reachable for **anonymous** shapes: the *type* of a bare
  table literal has no fields and no attributes (no shape types; same-shape literals
  intern to one `typeid`, attributes spec §1, value-representation §4), but the
  *declaration* has both, and `comptype` reads the declaration. Because `comptype` is a
  type, a generator can **demand** it in its signature,
  `const toJson = comptime fn (ct: comptype): fn`, called as `toJson(comptype User)` —
  the serialization pipeline's typed intermediate (json §2).

  **`comptype` is comptime-confined.** No `comptype` value exists at runtime: the
  operator is a **compile error** outside a comptime context, and, stronger, the
  confinement rides the *type* — a value of type `comptype` may not flow into anything
  that survives lowering (a runtime `const` splice, a stored table element, an
  `any`-typed slot that outlives comptime evaluation), each a compile error at the
  offending point. The rule-on-the-type is what makes "no runtime existence" airtight
  where a rule on the operator alone would leak through `any`. Equality on `comptype`
  values is **structural over its three fields**, so it is attribute-**inclusive** by
  construction — the right meaning for asking whether two declarations share one
  shape — while `type` and value equality stay exactly as at runtime.

  **How the value finds its declaration: provenance.** A comptime type holds strictly
  more information than the runtime type; they are separate things joined by this one
  bridge, and the extra information rides the **value during comptime evaluation** as a
  provenance tag: a value originating from a declaration carries that declaration;
  **copies preserve** the tag (a value passed through `comptime fn` parameters still
  knows its declaration, which is what makes generator pipelines work); **computed**
  values (`merge(a, b)`, a freshly built table) carry a fresh, attribute-free
  provenance; and **lowering to runtime erases** the tag entirely (the same erasure
  discipline as any attribute access, attributes spec §1).

  **Equality is untouched, in both phases, deliberately.** Provenance is invisible to
  `==`: values compare at comptime exactly as at runtime, and `type` values compare by
  `typeid` in both phases — no phase-dependent equality anywhere. The honest
  consequence, stated rather than hidden: `comptype` is **not a function of the value
  alone** — two `==`-equal values may yield different descriptors, because it reads the
  value's provenance, not its content. That is precisely why it cannot exist at runtime,
  and it is confined to this one introspective operator and its confined type.

- **`fields(t)`**, for an **error type** — the one nominal declaration form with
  declared fields (errors §1: `.field` reads a declared field) — its declared fields as
  a **list of tables**, each `['name' => string, 'type' => type, 'attributes' =>
  table]`, in **declaration order** (R129). Everything else answers **empty**: a bare
  `table` has no declared fields (no shape types), enums enumerate through `variants`,
  constraints through `baseOf` / `constraintPredicate`, refinements decompose
  (§4.5), and anonymous shapes go through the `comptype` operator, the general form.
  *(The pre-R95 domain claim — "a `@P` type's declared element members" — is deleted,
  R129: protocols declare no element members, and member structure lives on the
  protocol axis, §4.4.)*
- **`variants(t)`**, for an enum type, its variants, as a **list of tables**, each
  `['name' => string, 'payloadType' => type?]` (payload `null` for a payload-less
  variant). Drives exhaustive code generation over an enum.
- **`attributes(t)`**, the attributes attached to the declaration a **named** type came
  from, as a **table** keyed by attribute (attributes spec §4). Well-defined only where
  `typeid` and declaration are **1:1** — **errors, enums, constraints** (the scope
  scrubbed by R129: the inherited list said "protocols", but protos are not types;
  proto-member attributes ride the `jsonTag` deferral, json §4, surfacing in `members`
  rows when they land, §4.4). On an anonymous structural type it is a **compile error**
  whose message directs to the `comptype` operator (attributes spec §6).
- **`constraintPredicate(t)`**, for a constraint type, its predicate as a callable. The
  call is `comptime` (getting the predicate needs a static type), but the **returned
  predicate is a comptime-*eligible* plain `fn`**, not a declared `comptime fn`, so it
  exists at runtime and may be run at **both** phases. A constraint's predicate is pure
  and `use`-free (constraints spec §2), which is why it is eligible and runnable
  anywhere. You cannot introspect the predicate's **body** (the function is returned,
  not its source) — you never need the body; you run it. Pillar 1 in action: the
  predicate is handed to you to *run*, not machinery to *rebuild*.

### 4.3 The `kind` enum

```
export const kind = enum {
  scalar,          // int, double, bool, string, null, undefined, never — and the
                   // committed tower primitives (u64, float, f16, decimal, rational,
                   // complex) as they land (numeric-tower §6)
  bytes, table, stream, sink, promise, command, regex, secret,
  fnType, errorType, enumType, protocol, constraintType, capabilityType,
  refinement,      // @P and @P & @Q: interned protocol-set typeids (type §5, R25)
  intersection,    // the mixed constraint-and-protocol normal form (list & @drawable; R131)
  union, type, any
};
```

Twenty variants, **closed — no ellipsis** (R128, R131; re-derived from keywords §5's
predeclared list): the type universe is closed (value-representation §4.1), and a
closed universe deserves a closed enum, because exhaustive `match` over kinds is the
enum's entire purpose. Two spelling rules:

- **Where the natural name is a reserved keyword, the variant appends `Type`**:
  `fnType`, `errorType`, `enumType`, `constraintType`, `capabilityType` — keywords §1
  reserves the bare words, and nothing lexes them as identifiers, fenced or not. Every
  other variant keeps its bare name, including `type` and `protocol`, which are **not**
  reserved (`type` is a predeclared identifier, keywords §5; the keyword is `proto`,
  not `protocol`). The old `typeType` dodge was over-caution.
- **Predeclared names are safe as variants** because variant references are **fenced**
  (`{table}`, `{kind.table}` — enum §3.3, R20): inside the fence, names resolve against
  the expected enum, never lexical scope. The precedent is the catalogue's mode enum,
  whose variant `values` lives beside the live `values` function without collision.

Notes pinned by the derivation (R128):

- **`string` is a scalar**, not its own kind: the enum exists to dispatch structural
  queries, and a string walks like an atom. `undefined`, `null`, and `never` fold in on
  the same argument — with one eyebrow raised at `never` (it has no values), harmless
  since there is nothing to walk either way.
- `list`, `byte`, `json`, and every user constraint report `{constraintType}` (R10,
  R124); `baseOf` (§4.1) gives the base. This is also the reconciliation with
  tables §2.1's `typeName` behavior: a value that *entered* `list` carries the
  constraint typeid (constraints §9.2), so `typeName` says `"list"` while `kindOf`
  says `{constraintType}` — a constraint name, not a kind.
- `iterable` and `number` are predeclared union aliases and report `{union}`; so does
  every `T!`, the error union.
- **`{intersection}` names exactly the mixed normal form** (R131, resolving R128's
  pin). `&` normalizes at interning (type §3.1, R25): atoms collapse, pure constraint
  meets conjoin into a constraint (`{constraintType}`), pure protocol meets union into
  a refinement (`{refinement}`) — so the *only* form that survives interning as an
  intersection is the mixed one, a single interned type carrying both a constraint and
  a protocol set (`list & @drawable`). It is genuinely both, so its kind privileges
  neither: dispatch code queries both halves — `baseOf`/`constraintPredicate` for the
  constraint half, `protocolsOf` (§4.5) for the protocol half. A further `{complex}`
  variant for "union and intersection both involved" was **rejected** (R131): `&`
  distributes over `|` at normalization, so the outermost form is always a union whose
  members are themselves intersection-free-or-mixed — `kindOf` reports the outermost
  constructor and `unionMembers` recurses, which answers the question `{complex}` would
  double-encode; and the name collides with the committed numeric type `complex`
  (numeric-tower §5), which will report `{scalar}` — one word naming a kind and a
  scalar type would be actively confusing.
- `comptype` is outside the runtime type universe (§4.2; any spec §1), so `kindOf` can
  never receive it.

### 4.4 The protocol axis

```
export const protoName = fn (p: proto): string;
export const declaresIdentityEquality = fn (p: proto): bool;
export const members = fn (p: proto): list;
export const requirements = fn (p: proto): list;
export const binding = enum { constBinding, letBinding, varBinding };
```

A **protocol** is a first-class `proto` value (protocols §1), so introspecting it is
well-defined. Protocol queries take a `proto`, not a `type` — reached from a value via
`@@t`, then queried — and all four are **runtime tier**: protos from `@@t` are runtime
values (a generic walker must work where R125's dynamic writer works), and everything
here is flat typetable declaration data.

- **`protoName(p)`**, the protocol's name: the identifier of the `const` declaration
  that created it (protocols §1, R126) — load-bearing since R125, where it keys the
  `"@@"` serialization sections and their collision refusal.
- **`declaresIdentityEquality(p)`**, whether the protocol declares `identityEquality`
  (equality spec §4.4), a cheap fact.
- **`members(p)`** (R129), the proto's member declarations, in **declaration order**
  (top to bottom in the `proto` block — what makes generated output deterministic), one
  row per member:

  ```
  ['name' => string, 'binding' => binding, 'get' => bool, 'set' => bool,
   'type' => type, 'required' => bool, 'definitionFixed' => bool,
   'read' => fn?, 'attributes' => table]
  ```

  `binding` is the declaration keyword as the **`binding` enum** — variants dodge the
  reserved ladder words (keywords §1) with the R128 suffix discipline; a semantic
  vocabulary (`{mutable, fixed, frozen}`) was rejected as a second name-set for a
  ladder users know by keyword. `required` means no default (and therefore granted, by
  §2.2's theorem); `definitionFixed` means `const` with a default (protocols §2.1).
  `attributes` is reserved against the `jsonTag` deferral (json §4) and empty until
  attributes reach `proto` blocks. **Every member appears, ungranted ones included**:
  a declaration is source structure the author published by writing it, and knowing
  that `stringBuilder` has a private `var buf: bytes` discloses nothing about any
  table (pillar 3's exact line).
- **`read` — the granted-value accessor** (R129). On a `get`-granted row, `read` is a
  function `fn (t: table): any` that reads **that member** from a table; on an
  ungranted row it is `null`. This is the `constraintPredicate` pattern (§4.2): the
  declaration hands you its public reader *to run*. Its semantics mirror `->` exactly —
  on a table without the protocol applied it yields `undefined` (protocols §3.2); on a
  definition-fixed member it yields the uniform value regardless of the table. What it
  deliberately is **not**: dynamic member access by name never exists (pillar 5 — the
  accessor came from the *declaration you hold*, not from a string), it reaches only
  what `get` already made public through `->`, and **there are no setter accessors** —
  introspection is read-only, and generic *writing* is not a walker's need, because
  rebuilding is `apply` plus initializers (R125). This closes the R127 asymmetry: a
  user-space generic walker (pretty-printer, differ) now has exactly the power of
  R125's builtin dynamic writer — the granted surface, nothing more.
- **`requirements(p)`** (R129), the protocols this proto requires (protocols §7), as a
  list of **proto values** — held via `p`'s declaration, never summoned by name
  (pillar 2) — **direct requirements only**, in declaration order: the declaration's
  own `apply` lines, never the transitive closure, which is a fold the caller can
  write.

**Ungranted member values stay sealed** (R129, the ruling's one refusal): the grant
system is the encapsulation mechanism, and a read-side bypass would make `get`
advisory — privacy by discipline rather than construction. The corpus already treats
disclosure itself as the danger (serialization excludes ungranted members for exactly
that reason, protocols §5; secrets gate reads, not writes), and because grants can
never change retroactively, an ungranted member is *permanently* private by
declaration — introspection reading it would be the one place in the language where
that declaration lies. The mutation half of the old worry is structurally dead three
times over (immutable typetable declarations, inert results, no spelling for a grant
change); the disclosure half is a live danger and stays closed.

### 4.5 Application refinements (`@P`): type values whose structure lives on the `@@` axis

An application refinement **is a first-class `type` value** with an interned `typeid`:
its `typeinfo` records the canonicalized protocol set, so `@Q & @P` and `@P & @Q`
intern to one id, it sits in unions, aliases (`export const file = @fileDescriptor`),
and compares by `typeid` (type spec §5, R25; protocols §6). `kindOf(@person)` answers
`{refinement}` (§4.3), and `typeName` answers for it like any type. *(This corrects the
pre-R25 fossil this section carried — "`@P` is not a `type` value, it has no
`typeid`" — which contradicted the very section it cited; the R127 audit's third fossil
layer, fixed by R128.)*

What the old section got right survives, restated on the corrected foundation:

- **Membership is never the interval check.** `x is @P` and entry into a `@P`-declared
  position run the O(1) applied-set test on the **value** (protocols §3.2, §6; is spec
  §2), and `@x` never *reports* a refinement: application is a value property, never
  encoded in the value's own `typeid` (type §5). A refinement is a type whose
  membership question happens to be answered on the `@@` axis — precisely as a union
  is a type whose membership is answered by decomposition rather than intervals.
- **Structural queries do not apply to it.** `fields(@person)` has nothing to walk: a
  refinement declares nothing, its protocols do. Member structure is reached by
  **decomposition** — the protocol set is the refinement's whole substance, each proto
  queried by §4.4. From a value, the set comes via `@@t`; from the type itself, via
  **`protocolsOf`** (R131):

  ```
  export const protocolsOf = fn (t: type): list;
  ```

  The type's protocol component as a list of `proto` values (held through the type in
  hand, never summoned — pillar 2, the `requirements` precedent): a refinement yields
  its set, a mixed intersection (§4.3) yields its protocol half, and **every other
  type yields `[]`** — total, mirroring `@@`'s own totality on values (R126: asking is
  never an error, and "no protocol component" is an answer, not a fault).
- **Dispatch is `match`** (type spec §7): `match (x) { b: @stringBuilder => ... }` is
  the protocol-membership test. No unified "application reflection" exists — `@` for
  the table, `@@` for the protocols, `match` to branch.

### 4.6 The function axis

```
export const capabilitiesOf = fn (f: fn): list;
export const params = fn (f: fn): list;
export const paramTypes = fn (t: type): list?;
export const returnType = fn (t: type): type?;
```

The split that governs this section is already ratified in functions §3.2: **the
signature lives in the type; names and capabilities ride the value.** Function types
are not erased — `@f` reports the full signature, and a value's typeid is always
concrete (R18) — so **erasure is a declared-position phenomenon, never a value
phenomenon**: a binding declared `fn` erases what the *slot* knows, not what the value
*is*. Errorability is in the type even at the wildcard tier: `fn` / `fn!` split on it
alone, and an errorable function never fits a bare-`fn` slot (functions §3.2).

- **`capabilitiesOf(f)`** (R130, closing R88's open): the function's **declared**
  capability requirement set, as a list of **capability types** (`@io`-style `type`
  values) — `gatesOf`'s exact twin (secret §3.3). Inert by construction (pillar 5): a
  type can never appear in a `use` clause, so nothing grantable leaks — the query
  names effects, never mints authority. Declared set only: call-site delegation is
  invisible to it (R112), and frame grants are not part of the value.
- **`params(f)`** (R130, the home R108 promised its metadata): the parameter rows, in
  declaration order (R129), each `['name' => string, 'type' => type, 'optional' =>
  bool, 'variadic' => bool]`. Names are **value metadata, forced**: typing is
  structural and assignability per-position (functions §3.2), so `fn (x: int)` and
  `fn (y: int)` are one type — yet named arguments bind through erased values at
  runtime (R108), so the names demonstrably ride the value, exactly like the
  capability set.
- **`paramTypes(t)` / `returnType(t)`**: signature decomposition over fn **types**,
  for when no value is in hand (walking a proto's fn-typed member type out of
  `members` rows, §4.4). Runtime tier — signatures index a table (R88). On the
  wildcards `fn` / `fn!` both answer **`null`**: the existential honestly has no
  signature to report.

**A rejected equation, recorded** (R130): bare `fn` is *not* `fn (...any): any`. The
property that equation seems to buy already holds operationally — a call through an
erased `fn` is statically accepted and dynamically checked (`arityError`, `typeError`,
`namedArgumentError`; functions §3.2–§3.3, R108) — but the equation is unsound twice
over. Contravariance kills it: `fn (int): string <: fn (...any): any` would require
every possible argument list to fit `(int)`, so *no* concrete function would subtype
the wildcard, inverting its purpose. And the spelling is taken by a real citizen:
`fn (...args: any): any` is a declarable signature (R108 — the `println` shape), the
*most-accepting* signature, while the wildcard means the opposite quantifier — *some*
fixed signature, unknown here — an existential, soundly formalized as the interval
over the function typeid region (type §7.1). One spelling for both would conflate ∃
with the top signature. **`fn` calls like `(...any): any`; it is not typed as it.**

---

## 5. The canonical use: comptime code generation

Introspection exists mainly to drive **comptime code generation**, most centrally
serialization: the runtime-tier `kindOf` dispatches, the comptime-tier structural queries
do the walking, all at compile time, emitting specialized code that carries no
introspection at runtime. The ratified pipeline is json §2's generator: a `comptime fn`
demanding a **`comptype`** descriptor (§4.2), walking `(ct).fields` and each field's
`jsonTag`, and returning a runtime serializer that const-captures the extracted plain
data — the descriptor itself never survives lowering (§4.2 confinement; compiler §6).

The comptime side's worked form *is* that ratified generator (json §2,
examples/serialization.md) — this spec does not duplicate it. (The pre-R95 sketch this
section once carried took `t: type` and walked `fields(User)`; it is deleted, R131 —
"User" as a named shape type has no referent in a language with no shape types, and the
ratified pipeline demands `comptype`.) What belongs here is the **runtime-tier
companion**, a generic walker written entirely on this module's surface:

```
import { protoName, members } from introspection;

// Describe any value's granted protocol surface — pretty-printers, differs, loggers.
export const describe = fn (v: any): string => {
  var b = [] apply stringBuilder;
  foreach (p in @@v) {                       // total: [] off tables (R126)
    &b->append(protoName(p))->append(" { ");
    foreach (m in members(p)) {              // declaration rows (R129)
      if (m.read != null) {                  // granted members only, by construction
        &b->append(m.name)->append(": ")->append(m.read(v))->append(" ");
      }
    }
    &b->append("} ");
  }
  return b->build();
};
```

Every boundary this file rules is visible in ten lines: the walker sees declarations
and the granted surface, and *could not* see more if it tried — ungranted rows carry no
reader (`m.read` is `null`), no name is ever resolved to a value, and nothing here can
write.

---

## 6. The distributed pattern: kind-specific queries live with their kind

`std.introspection` owns the **type and declaration** level. Value-kind-specific probes
deliberately live in their kind's own spec, because each is inseparable from its kind's
semantics: `gatesOf(s)` and `canReveal(s)` in secret (§3.3, §5 — returning inert
`@`-descriptors, pillar 5's precedent), `wasThrown(e)` and `toTable(e)` in errors
(§2.1, §2.2), stream state probes (`isConsumed`, `peek`) in stream-api. Future
kind-specific queries follow the same home rule; this module takes only what is about
declarations as such.

---

## 7. The re-derivation, closed (R127–R131)

The R127 audit found the surface sound in architecture (§3's tiers, §4.2's confinement)
and stale in its rows. Every slice is now ruled, each against §1's pillars:

- **R128** — the `kind` enum: twenty closed variants (§4.3), the `kindOf`/`kind` name
  split, the `*Type` suffix rule, `refinement` on §4.5's fossil correction.
- **R129** — `fields` scoped to error types and `attributes` to errors/enums/
  constraints (§4.2); the protocol member surface (§4.4): `members`/`requirements`/the
  `binding` enum, runtime tier, declaration order, the `read` accessor for granted
  values only, no setters, ungranted values sealed.
- **R130** — the function axis (§4.6): `capabilitiesOf` (inert capability types, R88's
  open closed), `params` (R108's names as value metadata), `paramTypes`/`returnType`
  (null on the wildcards), the `fn ≡ fn (...any): any` equation rejected.
- **R131** — `baseOf` (subsuming `constraintBase`; resolving enum.md's recovery
  deferral); `{intersection}` for the mixed normal form and `{complex}` rejected
  (§4.3); `protocolsOf` (§4.5); the sugar pins (`unionMembers` decomposes `!` and `?`;
  `typeName` never shows alias names — forced by pure-sugar aliasing, R21); the §5
  worked example rewritten on this module's own surface.

**Nothing is open in this spec.** Two external riders are tracked where they live:
`jsonTag` attributes reaching `proto` blocks (json §4, attributes §6.3 — the `members`
row's `attributes` slot is reserved for it), and the deeper `decimal`/`rational`
numeric questions that ride the deferred tower (numeric-tower §7).

