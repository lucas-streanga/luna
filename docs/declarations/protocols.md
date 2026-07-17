# Protocols

Protocols are how behavior and encapsulated state attach to tables in Luna. Luna has no
separate object system: an object is a table with one or more protocols applied. This
document specifies what a protocol is, the member model (the binding ladder plus access
grants), how members are reached with `->`, how application works, how protocols require
one another, and the `@P` type layer. The former **views** document is retired (R95);
everything it specified that survives is here.

Two spaces, two operator families, and they never overlap:

- **Element space** — the table's own keyed data. Reached by `.` and `[]`, open and
  dynamic (any string is a key), fully accessible to whoever holds the table. Protocols
  **never** put anything here.
- **Protocol space** — the members contributed by applied protocols. Reached by `->`,
  **closed and compile-time known** (every protocol's member set is fixed at its
  definition), namespaced per protocol so two protocols can never collide.

Because the spaces are disjoint, a data key `map` and a protocol member `map` and the
built-in free function `map` (iterable-functions) coexist with no precedence rules:
`tab.map` is data, `tab.map(f)` is UFCS, `tab->map` is protocol space.

---

## 1. What a protocol is

A protocol is **data**: a first-class value of type `proto`, defined with a `proto` block,
storable in variables, passable to functions, and inspectable at runtime (§8). Protocols
are applied to tables **at runtime** (§4), so which protocols a table carries is a runtime
property of that table value, not a fact fixed at its declaration. A table does not
inherit or subclass a protocol; it *has applied* one.

A protocol contributes exactly **one kind of thing**: **members**. A member is a named,
typed slot in protocol space, declared with the ordinary binding ladder (`const` / `let` /
`var`) and optional access grants (`get` / `set`). Behavior is not a separate category:
a protocol function is simply a `const` member whose value is a `fn` (§2.4). The former
three-way split (meta functions / meta members / declared element members) is retired:
the old "meta member" is an ungranted per-table member (§2.3), the old "declared element
member" no longer exists (protocols do not touch element space), and `meta` is no longer
a keyword.

---

## 2. The `proto` block: member declarations

A `proto` block contains member declarations and nothing else. Each declaration is:

```
<const | let | var> [get] [set] name[?]: type [= default];
```

```
person = proto {
  const get name: string;                  // required at apply; immutable after (§2.1)
  const get species: string = 'human';     // definition-fixed: a protocol constant (§2.1)
  let get set nickname?: string = null;    // write-once: externally settable exactly once
  var get set visits: int = 0;             // freely readable and writable
  var lastSeen?: string = null;            // ungranted: private to person's functions

  const get greet = fn (p: @person): string => { ... };   // a public protocol function
  const normalize = fn (s: string): string => { ... };    // a private helper
};
```

A protocol is a value, so it binds like any other (usually `const`) and is exported by
exporting its binding. Three orthogonal mechanisms compose in each declaration, each with
its ordinary meaning:

- **The binding keyword** is the member's mutability, exactly as in variables
  (variables §1): `const` never changes, `let` fixes the slot but allows interior mutation
  (and the write-once `?: T = null` pattern, variables §1.2), `var` is free.
- **`?` sets the type** (`nickname?: string` is `string | null`), never the requiredness.
- **The default decides omissibility at apply** (§4.2): a member with no default must be
  supplied by an apply initializer; a member with a default may be omitted. This is the
  same rule as function parameters (functions §3.3.1), by the same means, so there is no
  `required` / `optional` keyword and no marker.

### 2.1 `const` means one binding, ever

The presence of a default splits `const` members into the two things they can be:

- **Default present → definition-fixed.** `const get species: string = 'human'` is bound
  at *proto definition*. Apply cannot rebind a `const`, so no initializer may name it
  (compile error), and its value is **uniform across every table** — it is a protocol
  fact, not a value fact. Definition-fixed members are the protocol's constants and
  functions; they are reachable off the proto value itself (`person->species`, §3.4) and
  excluded from equality, serialization, and the initializer surface (§5).
- **Default absent → bound at apply.** `const get name: string` receives its one binding
  from a required initializer (§4.2) and is immutable for the table's lifetime: a
  per-table constant, the record-field case.

This split is not a special rule; it is `const`'s own semantics. For `let` and `var`
members a default is merely the initial value each table starts with, so an initializer
may override it; a `const`'s default *is* its binding, so nothing may.

One consequence worth naming: because definition-fixed members are never initializable, a
fn-typed `const` member cannot be overridden per table. Luna therefore has **no virtual
dispatch**: `tab->greet()` runs the one `greet` the protocol defined, for every table.

There is **no mutable protocol-level state**. A `var` member is per-table; a `const` with
a default is uniform and immutable; nothing in between exists. Shared mutable state is a
concurrency question and lives where the concurrency model puts it (a task that owns it),
never implicitly in a protocol.

### 2.2 Grants: `get` and `set`, external reach only

Grants control **external** access — access from anywhere other than the protocol's own
functions, which always have full access to their own protocol's members:

- **No grant** — private. Unreachable from outside; the old meta-member privacy, now as
  the default. (Cross-protocol member access does not exist; another protocol is
  "outside.")
- **`get`** — externally readable: `tab->visits`, and for definition-fixed members
  `person->species` (§3.4).
- **`set`** — externally mutable: slot assignment (`tab->visits = 4`) and interior writes
  (`tab->prefs.theme = 'dark'`), to the extent the binding keyword permits any mutation
  at all.

Rules:

- **Canonical order is `get set`** (writing `set get` is a compile error; one spelling).
- **Grants are orthogonal.** `set` does not imply `get`; a write-only member
  (`var set sink: …`) is legal and rare.
- **A grant that can never be exercised is a definition error.** `const … set` grants
  writing to a deep-frozen member; `let set m: int = 5` grants writing to a fixed slot
  with no interior. Both are dead grants and the protocol is ill-formed. Statically
  decidable from keyword plus type.
- **Required ⇒ granted, by construction.** A member with no default and no grant could
  never be bound (no initializer may reach it, §4.2/§5) — the protocol is ill-formed.
  This is a theorem of the model, not a rule to remember.

Invariants on members are the type system's job, not a setter's: a member's declared type
may carry a refinement constraint (constraints spec), checked on every write like any
constraint. An invariant that relates *two* members is expressed by not granting `set`
and exposing a protocol function that maintains it.

### 2.3 Per-table members are the state

`let` and `var` members (and no-default `const` members) are **per-table**: each table
with the protocol applied carries its own slots, invisible to element space. The built-in
introspection and the iterable catalogue (`count`, `keys`, `has`, iteration) never see
them; a table whose only state is protocol members still reports element-empty. This is
what makes the string builder an element-empty table that holds a growable buffer (§8).

### 2.4 Functions are `const` members of type `fn`

A protocol function is a definition-fixed member whose value is a function:

- **`get` makes it public** (`tab->greet()`, `person->greet`); ungranted makes it a
  private helper, callable only from the protocol's own functions. One visibility knob
  for the whole block.
- **Receiver first, by value.** A protocol function takes its receiver as the first
  parameter, typed with the protocol's own type (`fn (p: @person)`), and follows the
  same convention as the entire built-in catalogue (iterable-functions §1.6): the
  receiver is passed by value, mutation is expressed by returning the updated receiver,
  and the *caller* writes back with `&` (`&b->append("x")`, tables §4). No protocol
  function takes a `&` parameter.
- **`self`**, in a return-type position inside a proto block, is the receiver's type,
  `@CurrentProto` — the idiomatic return for chainable functions. In a body, `self` is
  the receiver value. There are no views (R95): `self` is a plain table, statically
  typed.
- **`identityEquality`** (optional declaration in the block) marks tables with this
  protocol as comparing by **identity** rather than structure (equality spec §4.4) —
  retained unchanged, and newly load-bearing for builders whose private state should
  never enter `==` (§5).

---

## 3. Protocol space and `->`

`->` is the single operator for protocol space, and its member space is **closed**: every
protocol's members are fixed at proto definition, so every `->` access is resolved at
compile time against the protocols in scope. `->` is to protocol space what `.` is to
element space — with the difference that this space is statically known, so misspellings
and grant violations are compile errors, not runtime discoveries.

### 3.1 Resolution: bare when unique, qualified when not

- **`tab->name`** resolves `name` against the members of the protocols **in scope** at
  the access site. If exactly one in-scope protocol declares `name`, that is the member.
  If more than one does, the access is a **compile error demanding qualification**.
- **`tab->P.name`** qualifies: the member `name` of protocol `P`. `P` must resolve to a
  `proto` in scope. This is pure syntax — it does not produce a view or any intermediate
  value (R95).
- **An unknown name is a compile error.** The member space is closed, so a name no
  in-scope protocol declares cannot be an absence; it is a typo, caught at compile time.
  `undefined` (§3.2) therefore never masks a misspelling.

Grant checks land at the same moment: reading an ungranted member, or writing a
non-`set` member, is a **compile error**, whatever the binding's type — the grant is a
property of the member, and the member space is closed. What was a runtime permission
model (the retired `TableReadViolationError` / `TableMutationViolationError`, R98) is now
a static assertion.

### 3.2 Unapplied protocols: `undefined` on read, panic on hard use, `?->` to ask

Whether the resolved protocol is **applied** to this table is the one runtime question
`->` can face (application is dynamic, §4). The rule mirrors element-space absence — two
absences, one rule:

- **A bare read yields `undefined`.** `tab->name` where `name`'s protocol is not applied
  is a missing capability, navigable like any absence: `tab->nickname ?? 'anon'`.
- **A hard use panics.** A write (`tab->visits = 4`) or a call (`tab->greet()`) on an
  unapplied protocol asserts a capability the table does not have; it panics. A silent
  no-op call is the worst failure mode (the program continues, wrong), so it does not
  exist.
- **`?->` is the explicit soft form.** `tab?->greet()` calls if the protocol is applied
  and yields `undefined` otherwise, **short-circuiting**: on the unapplied path the
  arguments are not evaluated, as with `?.`. It composes with coalescing:
  `tab?->visits ?? 0`.

Through a `@P`-typed binding (§6) the compiler *proves* application, so none of this
arises: access is unconditional and check-free. The `undefined`/panic/`?->` triad exists
only for dynamic probing through bare-`table` bindings. The presence test itself is
`tab is @P` (is spec).

### 3.3 Assignment

`tab->m = v` and interior writes (`tab->m.k = v`) are legal exactly when the member's
binding keyword permits the mutation and the member grants `set` (§2.2), checked at
compile time; the write is then type- and constraint-checked like any write. Assignment
through `->` reaches protocol space only — it can never create, shadow, or clobber an
element key, and `.`/`[]` writes can never touch a protocol member. The widening /
`&`-alias hole of the old model (a bare-`table` binding laundering a write into a
protocol-declared key) is closed **by construction**: there is no spelling for it.

### 3.4 `->` on a proto value, and functions as values

Definition-fixed `get` members are protocol facts, so they are reachable off the proto
value itself, no table required:

```
person->species              // 'human'
person->greet                // the fn value: fn (p: @person): string
people.map(person->greet)    // receiver-first, so it IS a transformFn already
```

`P->name` is the one way to take a protocol function as a **value**. It yields the bare
`fn` exactly as declared — receiver-first, nothing captured — which composes directly
with the iterable catalogue and with any consumer of functions. There are **no bound
methods**: `tab->greet` *without* a call is a **compile error** (message pointing at both
spellings: `tab->greet()` to call, `person->greet` for the value). A bound value would
either capture the receiver as a deep-`const` snapshot (functions §2.1) — a stale-looking
live binding — or alias it, which value semantics forbid; and a bound *mutator* is
incoherent under caller-side `&`-write-back (the returned table would have nowhere to
go). To bake a receiver in deliberately, write the closure explicitly and get snapshot
semantics by the ordinary capture rule:

```
let f = fn (): string => p->greet();     // p captured deep-const, per functions §2.1
```

A **per-table** member of type `fn` is not ambiguous with any of this: `var get handler:
fn` is data, `tab->handler` reads it, `tab->handler()` calls the stored value. The
declaration decides, statically — the closed space needs no syntactic rule of the kind
element space required for UFCS (functions §3.4).

### 3.5 Dispatch summary

| Expression | Meaning |
|-|-|
| `tab.name` / `tab[expr]` | element read (data); miss is `undefined` |
| `tab.name(...)` | UFCS call of a free function (functions §3.4) |
| `tab->name` | protocol member read; `undefined` if the protocol is unapplied |
| `tab->name = x` | protocol member write (needs `set` + a mutable keyword); panics if unapplied |
| `tab->name(...)` | protocol function call (or call of a fn-typed member); panics if unapplied |
| `tab?->name(...)` | as above, but `undefined` (args unevaluated) if unapplied |
| `tab->P.name` | the same, qualified to protocol `P` |
| `P->name` | definition-fixed member off the proto value (constant or function) |
| `@@tab` | the applied protocols, as data (§7) |

---

## 4. Application

Applying a protocol attaches it to the table's applied-protocol set and binds its
per-table members — from initializers where given, defaults otherwise. Application is
**pure machinery**: it runs no user code, ever. There is no `apply` member, custom
attach-time logic does not exist, and everything the old `apply` bodies did is either
unnecessary (validation — the type and constraint system covers it), expressed as data
(initialization, §4.2), or an ordinary factory function (§4.5).

Application is **atomic**: the table has the protocol, with every member bound, or the
application failed and the table is unchanged. No partially-applied state is observable.

### 4.1 Two forms: the operator and the function

- **The `apply` operator** (expression position) is the static form:

  ```
  let p: @person = [] apply person(name: "Lucas");
  let q = existing apply tagged;                       // onto any table expression
  ```

  The protocol is named statically, the initializer list is checked at compile time
  (§4.2), and the result type is the refinement `@P` (§6). **The operator form is never
  errorable.** Under this model a table's element data is irrelevant to application
  (protocol space is separate), so there is nothing runtime-shaped left to fail: no
  collisions (namespaced), no present-member type checks (no members are "present"
  before apply), no shape dependence. The old statically-known-shape precondition is
  gone — the operator applies to *any* table expression, checked entirely at compile
  time.

- **The `apply` function** is the dynamic form: a built-in free function (R91), used
  when the protocol or the initializers are runtime data:

  ```
  fn apply(tab: table, protocol: proto, initializers?: table = null): table!
  ```

  `&tab.apply(p)` mutates in place by ordinary write-back (and so requires a `var`
  binding, variables spec). Because the protocol and initializer keys are runtime
  values, this form is **errorable** — the single residual failure of the whole model
  (§4.4).

### 4.2 Initializers are data

An initializer list supplies values for the protocol's **granted per-table members**
(§5): required members (no default) must appear; defaulted `let` / `var` members may
appear to override their defaults; definition-fixed members may not appear (§2.1);
ungranted members may not appear (they are the protocol's own business and always have
defaults, §2.2). Each value is checked against the member's declared type and constraint.

```
[] apply person(name: "Lucas")                    // required member supplied
[] apply person(name: "Lucas", visits: 1)         // defaulted member overridden
[] apply person(species: 'elf')                   // compile error: definition-fixed
[] apply person()                                 // compile error: `name` missing
```

The `name: value` list is the **apply operator's own grammar** — like the proto block's
declarations, it is not an expression form. It deliberately reads identically to function
**named arguments** (functions §3.3.2, R108): same surface, different binding target —
initializers bind protocol *members*, named arguments bind *parameters*.
No code runs: initializers are typed data, installed by machinery. There is deliberately
no constructor concept — no ordering semantics, no partially initialized value, no
overloading; anything smarter than typed installation is a factory function (§4.5).

In the dynamic form the same list is an ordinary table: `tab.apply(person,
['name' => "Lucas"])`, with the same rules checked at runtime.

### 4.3 Idempotency and re-application

Applying a protocol a table already has is a **no-op**: nothing is re-bound, member
state is untouched, the applied set is unchanged. Application is thereby **monotone**
(the applied set only grows), which is what makes unchecked statement-form application
sound alongside `@P` promises (§6.3).

Re-application **with initializers** is an **error** — silently ignoring the supplied
data would be data loss, and re-binding would break idempotency. A compile error where
the operator form can see it (the target is already `@P`-typed); an `ApplyError` from
the dynamic form otherwise.

### 4.4 Failure modes

The operator form has none (§4.1). The dynamic form has exactly one error, `ApplyError`,
covering: a required member missing from the initializer table, an initializer key naming
no granted per-table member (unknown, definition-fixed, or ungranted), an initializer
value failing the member's type or constraint, and re-application with initializers
(§4.3). All other historical failure modes — declared-member collisions, present-member
type mismatches, incompatible shapes, throwing `apply` bodies — are structurally gone.

### 4.5 Factories are functions

Validation beyond constraints, derived members, convenience shapes — anything a
constructor would do elsewhere — is an ordinary function returning `@P`:

```
export const newPerson = fn (fullName: string): @person! => {
  // validate, derive, then:
  return [] apply person(name: normalize(fullName));
};
```

Same pattern as capabilities-are-consts and extensions-are-UFCS (§9): no mechanism,
just a function. Errorability, if any, is the function's own and is declared in its type.

---

## 5. The granted surface is the contract

One boundary, stated once, consumed four times. For a protocol's **per-table** members:

- **Access:** `get` / `set` decide external reads and writes (§2.2).
- **Equality:** `==` on tables compares element space, the applied-protocol sets, and the
  **`get`-granted per-table members** of each applied protocol. Ungranted members are
  incidental state by declaration and never enter `==` (a builder's buffer does not make
  two logically equal values unequal; if hidden state *should* distinguish values, the
  protocol declares `identityEquality`, §2.4). Definition-fixed members are uniform and
  therefore vacuous.
- **Serialization:** `toJson` emits the `get`-granted per-table members (alongside element
  space; the exact nesting shape is the json spec's concern). Ungranted members do not
  serialize — emitting them would disclose private state and produce output that cannot
  round-trip.
- **Initializers:** the granted per-table members are exactly what an apply initializer
  may bind (§4.2), which is what makes deserialization possible at all: rebuilding a
  value is `apply` plus initializers, so the serializable surface and the initializable
  surface must be the same surface.

**Function values do not serialize.** A `fn` has no data representation, deserializing
one is impossible, and emitting one is a security hazard. `toJson` on a value whose
serialization surface contains a `fn` (a fn-typed `get` member, or a fn stored in element
space) raises `typeError`; `toJson(v, skipFunctions: true)` omits fn-valued slots
instead. Fn values compare by identity in `==` as functions always do.

---

## 6. Protocol types: `@P`

In **type position**, `@P` denotes "a table with `P` applied" — the type-position role of
`@`, fixed by grammatical position (type spec §1.1); `@X` where `X` is not a protocol is
a compile error. A `@P` is an ordinary table carrying a static guarantee about the `@@`
axis (§7); membership (`x is @P`) is the O(1) applied-set test. Refinements are
first-class `type` values, sit in unions, alias (`export const file = @fileDescriptor`),
and compare by `typeid` — all unchanged from the previous model.

### 6.1 Composition is unconditional

`@P & @Q` is "has both applied"; `@P | @Q` is "has one or the other" (narrows like any
union). **`@P & @Q` is always well-formed.** The old disjoint-declared-members constraint
is gone with its cause: protocol members are namespaced, element space is protocol-free,
and there is no name two protocols can fight over. Any two protocols compose.

### 6.2 Typed access, and what the compiler discharges

Through a `@P`-typed binding, every `->` access to `P`'s members is fully static: the
member, its type, its grants, and the fact of application are all compile-time facts, so
reads and conforming writes are check-free and violations are compile errors. Through a
bare-`table` binding, member resolution and grant checking are still static (§3.1); only
the applied-set question is dynamic (§3.2). Element access (`.`/`[]`) is untouched by all
of this — it is the same open, dynamic space on every table.

Declaring the richer type is the knob that propagates typed access across boundaries: a
function that returns a person declares `fn (): @person`, not `fn (): table`.

### 6.3 Facts and promises

The two application forms split exactly as before (no flow typing): the **operator in
expression position** makes a *promise* — the result's static type is `@P`, checked at
compile time. The **dynamic function** records a *fact* — the value's applied set grows
at runtime, no binding's static type changes, and later access through a non-`@P` binding
is resolved dynamically (§3.2). No fact silently upgrades into a static assumption. The
soundness of the unchecked dynamic form rests on monotonicity (§4.3): application only
adds, so no `@P` promise can be broken by someone else's later apply. Removal, if ever
added, must repay this debt (§10).

---

## 7. Requirements: protocols applying protocols

A protocol may require others, spelled with the same keyword doing the same thing:

```
employee = proto {
  apply person;                       // employee requires (and applies) person
  const get badge: int;
  ...
};
```

- **Semantics: auto-application.** Applying `employee` applies `person` first
  (transitively, and idempotently — already-applied requirements are no-ops, §4.3). This
  is sound *because* application is pure machinery: auto-applying runs no code and can
  surprise no one.
- **The requirement graph is a DAG.** Cyclic requirements are a definition error, for the
  same reasons module imports form a DAG (modules §2): a cycle means the protocols are
  not independent and should be one protocol, and acyclicity keeps every question about
  the closure decidable bottom-up.
- **Subtyping falls out:** a `@employee` provably has `person` applied, so
  `@employee <: @person` — a `@employee` value is usable wherever `@person` is expected.
  Composition without inheritance.
- **One restriction keeps initializers simple:** a protocol may be auto-applied only if
  its granted per-table members are all defaulted. If a required protocol has *required*
  members (`person`'s `name`), the requiring protocol's application cannot invent them,
  so the target must have the requirement applied **already** — `[] apply person(name:
  "Lucas") apply employee(badge: 7)` — and applying `employee` to a table without
  `person` is a compile error in the operator form (an `ApplyError` dynamically) naming
  the missing requirement. Initializer lists thereby always belong to exactly one
  protocol; there is no cross-protocol initializer grammar.
- Member-name overlap between requirer and required is permitted like any overlap
  (namespaced); bare `->` access disambiguates by scope or qualifies (§3.1).

---

## 8. Reflection: `@@`

`@@` is the reflection operator for the protocol axis, the sibling of `@` (type): `@@tab`
yields the table's applied protocols as an application-ordered list of `proto` values.
(Its former view-related half is retired with views.)

```
if (stringBuilder in @@b) { ... }         // membership, by value
foreach (p in @@b) { &other.apply(p); }   // protocols are data; re-apply elsewhere
protoName(p)                              // the name string, for tooling (free function)
```

The order is application order — deterministic, cheap, and **not** semantically
load-bearing (protocols are an unordered capability set; nothing dispatches on order).
`@@tab == []` means a plain table. `@@` on a non-table value is settled by the type
surface of `@@` (open question, §10). The `@`-family stays coherent: `@` reflects types,
`@@` reflects protocols-as-data, `@P` in type position is a static promise *about* the
`@@` axis (§6).

---

## 9. Extensions are functions

There is no extension mechanism, because UFCS already is one (functions §3.4): a free
function whose first parameter is `@P`-typed is an extension of `P` —

```
export const initials = fn (p: @person): string => { ... };
p.initials();                             // UFCS; import-scoped like any function
```

— with exactly the right properties: it cannot reach ungranted members (grants hold, §2.2),
it is scoped by imports rather than attached to the protocol, and the proto's own `->`
surface stays closed and complete. The call syntax marks the difference honestly: `->` is
the contract, `.` is an extension.

---

## 10. Worked example, and open questions

```
stringBuilder = proto {
  identityEquality;                       // builders compare by identity (§5)
  var buf: bytes = bytes();               // per-table, ungranted: private state

  const get append = fn (b: @stringBuilder, value: any): self => { ... };
  const get byteLength = fn (b: @stringBuilder): int => { ... };
  const get build = fn (b: @stringBuilder): string => { ... };   // snapshot; b reusable
};

var b: @stringBuilder = [] apply stringBuilder;   // operator form: typed, non-errorable
&b->append("Hello, ")->append(name);              // chain; & writes the result back
let greeting = b->build();                        // : string — a plain value
```

`b` is element-empty throughout; `buf` is protocol state, invisible to `count` / `keys` /
iteration and to `==` (ungranted — and the protocol is `identityEquality` besides).

Open questions:

- **Removal** (`unapply`): still deferred, with the standing condition (§6.3): removal
  would make `@P` a breakable promise and must get invariant-constraint treatment
  (constraints §7.1) — it cannot be added as a free mutation.
- *(The initializer-grammar question is **closed by R108**: named arguments landed with
  the same `name: value` surface; the initializer list stays its own grammar, binding
  members rather than parameters — §4.2.)*
- **Serialization nesting:** *what* serializes is fixed (§5); the JSON shape protocol
  members take (nested under the protocol name, flattened, tagged) is the json spec's
  decision.
- **`?->` token:** semantics are fixed (§3.2); lexer/associativity placement is the
  build-spec sweep's concern.
- **Bound functions:** rejected (§3.4); revisit only if a concrete need survives the
  explicit-closure idiom.
- **`@@` on non-table values:** error or `[]`, pending the `@@` type surface.
