# Protocols

Protocols are how behavior and per-key contracts attach to tables in Luna. Luna has
no separate object system: an object is a table with one or more protocols applied.
This document specifies what a protocol is, how it is applied, how its members are
reached, and how multiple protocols coexist on one table. Views, the values produced
by reaching into a protocol's namespace, are specified in the companion **views**
document; this document defines everything a view depends on and refers to it where
the two meet.

---

## 1. What a protocol is

A protocol is **data**: a first-class value of type `proto`, defined with a `proto`
block, storable in variables, passable to functions, and inspectable at runtime. It
is not a compile-time-only construct the way a trait or interface is in a static
language. This is the defining difference from trait systems: protocols are applied to
tables **at runtime** (§4), so which protocols a table carries is a runtime property
of that table value, not a fact fixed at its declaration.

A protocol contributes three kinds of thing to a table it is applied to:

- **Meta functions**, behavior. Callable through the table but not stored in it (§3).
- **Meta members**, protocol-private state. Real storage, namespaced to the protocol,
  invisible to the table's element space (§3.3).
- **Declared element members**, ordinary keyed data the protocol requires the table to
  carry (§5). These live in element space and are the only contribution that is not
  protocol-private.

Because a protocol is "applied, not implemented," a table does not inherit or subclass
a protocol; it *wears* one, and can shed or gain protocols over its lifetime.

---

## 2. The `proto` block

Most protocols are purely declarative: a set of declared members, meta functions, and
meta members, with **no `apply`**. Applying such a protocol is pure machinery, attach
the protocol and install its declared members, so it never runs user code, never throws,
and is always statically checkable. This is the common case (the large majority of
protocols), and it is the baseline the rest of this document assumes unless an `apply` is
present.

```
myProto = proto {
  // a declared element member, with an optional type and permissions
  status?: string;

  // a meta member: protocol-private state, addressed only inside this protocol
  meta buffer: bytes;

  // meta functions
  reset = meta fn (tab: table): self => { ... };
  emit  = meta fn (tab: table): string => { ... };
};
```

A protocol is a value, so it binds like any other (`myProto = proto { ... }`, usually
`const`). Modules export variables, so a protocol is exported by exporting the
variable it is bound to. There is no separate protocol-declaration namespace.

The block contains:

- **Declared element members** (`status?: string`), naming keys the protocol installs
  or requires in element space, with optional type and permission annotations (§5).
- **Meta members** (`meta buffer: bytes`), protocol-private state (§3.3).
- **Meta functions** (`name = meta fn (...) => ...`), the behavior (§3).

A protocol that needs to run code at attach time, logging, initializing a meta member,
validating the table, installing dynamic members, adds an **`apply`** meta function (§4).
This is the exception, not the rule:

```
  // optional: only when the protocol needs attach-time code
  apply = meta fn (&tab: table): self => { ... };       // non-throwing
  // or, if it installs dynamic members or otherwise throws:
  apply = meta fn (&tab: table): !self => { ... };      // throwing (§5.1, §7.5)
```

Whether a protocol has an `apply`, and if so whether it is throwing (`!self`), is what
determines the errorability of applying it, and it is readable directly from the block
(§7.5).

---

## 3. Meta functions and meta members

The distinction that makes protocols work: **meta functions and meta members belong to
the protocol, not to the table.** They are reachable *through* a table that wears the
protocol, but they are not elements of it.

### 3.1 Meta functions are reached, never stored

A meta function is invoked through the table but is not a key in it. This is what makes
`tab->reset()` work without `reset` being a table element:

- `tab->reset()` dispatches to the protocol's `reset` (§ dispatch below and the views
  document).
- `tab.reset` is an **element** access, the data under key `reset`, which is almost
  always `undefined`. It is never the meta function.
- `tab.reset = fn ...` is an element **assignment**; it writes data under key `reset`.
  It does not, and cannot, redefine the meta function. A meta function is not an
  assignable slot.

So the two operators cleanly partition (this is the core rule the views document
builds on):

- **`.` reads and writes element space only.** Data. A miss is `undefined`.
- **`->` reaches meta space.** Behavior. Never a data slot, never assignable.

### 3.2 Meta functions take the table explicitly

Every meta function's first parameter is the table it operates on, written
`(tab: table)` or `(&tab: table)` when it mutates (§4.1). Inside the body, `self`
refers to the receiver **wrapped in the current protocol's view** (see views), which
is how a meta function returns a chainable result (`: self`).

### 3.3 Meta members are protocol-private and namespaced

A meta member (`meta buffer: bytes`) is real storage attached to the table, but:

- It is **namespaced to its protocol.** Two protocols may both declare a `meta buffer`
  and they never collide, because each is addressed only from within its own protocol's
  meta functions. There is no syntax to reach another protocol's meta member.
- It is **invisible to element space.** It is not a key; the built-in introspection
  (`count`, `keys`, `has`) never sees it. A table whose only protocol state is meta
  members still reports element-empty.
- It is **writable by its owning protocol's meta functions** and by nothing else. This
  is inherent to meta-ness, not an extra permission: because no outside name can reach
  it, no outside code can read or write it. A meta member therefore does **not** need
  and should **not** carry `noSet`; the builder's buffer, for instance, must be mutable
  by `stringBuilder`'s own `append`, so marking it `noSet` would break it. "Meta" is
  the access rule; `noSet` is a separate per-key element contract (§5) and does not
  apply to meta members.

Meta members are why the string builder can be an element-empty table (`[]`) that still
holds a growable buffer: the buffer is a meta member of `stringBuilder`, not an element.

---

## 4. Application

Applying a protocol attaches it to a table's applied-protocol set and installs the
protocol's declared members. If the protocol has an `apply` meta function (§2), that body
runs as part of application; if it does not, application is pure machinery and runs no
user code.

```
fn apply(&tab: table, protocol: proto): table | !table
```

Application **mutates the table in place**, so it takes the table by reference and, by
the reference rules (variables spec), requires a **`var`** table. Applying a protocol to
a `let` or `const` table is a compile error, because `&` may not be taken of a
`let`/`const` binding.

**Whether application is errorable is read directly off the protocol**, and reads the
same on the protocol side and the caller side:

- A protocol with **no `apply`**, or a **non-throwing `apply`** (`: self` / `: table`),
  applies **non-errorably**: the only possible failure is a declared-member collision,
  which is compile-checked on a statically-known table (§7.4) and a runtime check
  otherwise. This is the common case (§2).
- A protocol with a **throwing `apply`** (`: !self`, one that installs dynamic members or
  otherwise throws, §5.1) applies **errorably**, and the caller must handle it with `try`
  or an errorable binding.

```
var tab = [];
tab->apply(myPlainProto);               // non-errorable: no apply, or non-throwing apply
try tab->apply(myThrowingProto);        // errorable: the protocol's apply is : !self
```

This section describes **dynamic** (runtime) application. There is also a **static**
form, spelled `apply` without call syntax, that produces a value whose *type* records the
protocol and whose collision check happens at compile time (§7). The static form runs the
same application, including the `apply` body if present, and inherits the same
errorability rule above.

### 4.1 What application does, in order

1. **Pre-check the declared surface (option A).** The machinery reads the protocol's
   *declared* names, its meta function names and its declared element members, and
   checks them against the table as it currently is:
   - A declared **meta function** name that would be ambiguous is not a problem, meta
     functions are always protocol-qualified (see views), so two protocols may share a
     meta function name and coexist; there is nothing to reject here.
   - A declared **element member** name (`status`) that already exists in the table's
     element space is a **collision** if it was installed by a *different* protocol,
     because element space is flat and un-namespaced (§5). It is **not** a collision if
     this same protocol installed it on a prior application: re-applying a protocol is
     idempotent for its own declared members, so re-apply does not fail the pre-check on
     members it already owns.
2. **Attach the protocol** to the table's applied-protocol set, and install its meta
   functions and meta members (the latter initialized as the block specifies).
3. **Run the `apply` body**, if present, for any custom setup, including dynamic member
   installation (§5.1).

If any step fails, `apply` returns the error and the table is unchanged (application is
all-or-nothing).

### 4.2 Failure modes

- **Reapply / element collision:** a declared element member already present, or the
  `apply` body detecting a prior application, throws (typically `ApplyError`, see §5.2).
- **Incompatible table:** the table does not satisfy something the protocol requires.
- **Dynamic-install collision:** an `install` of a computed key that already exists,
  unless the body chose a replacing variant (§5.1).

### 4.3 Re-application is permitted

The machinery does **not** reject applying a protocol a table already wears. Re-apply is
naturally idempotent for declared element members (§4.1 tolerates re-installing a
protocol's own members) and for meta functions (which are defined by the `proto` value,
not installed on the table, so re-apply reaches the same functions and changes nothing).
A protocol whose `apply` body has custom setup may guard against re-running it, returning
early or throwing `ApplyError`, but that is the protocol author's choice, not a
machinery-enforced rule. Applying the built-in protocol is meaningless (every table
already wears it) and is not an operation.

Note that a protocol's meta functions can **never** be redefined by application, because
application does not *define* meta functions; it makes the ones already declared in the
`proto` block reachable. Redefinition could only be attempted at proto-definition time by
declaring the same meta function name twice in one block, which is ill-formed: **a
duplicate meta-function name within a `proto` block is a definition error.**

### 4.4 Reaching an unapplied protocol yields `undefined`

Reaching a protocol a table does not wear, `tab->someProto` or `view(tab, someProto)`,
yields **`undefined`**, not an error (views document, §3.2 and §5). This makes "does
this table wear protocol P" a constant-time, coalescable check
(`tab->someProto ?? ...`, `tab->someProto?.method()`), consistent with how a missing
element key yields `undefined`. A hard `.` on the resulting `undefined` throws, so
careless use of an absent capability still fails at once. The protocol name must resolve
to a `proto` in scope; a name that resolves to nothing is a compile error, so a typo
cannot masquerade as an unapplied protocol.

---

## 5. Element members: declared vs. dynamic

Everything in §3 (meta functions, meta members) is protocol-namespaced and cannot
collide across protocols. Element members are the exception: they live in the table's
flat, un-namespaced element space, so they are the one place two protocols can clash on
a name. Luna handles the two ways an element member can be created differently, and the
distinction is **purely syntactic**, which keeps it cheap to enforce.

### 5.1 Declared (`.name`) vs. dynamic (`[expr]`) installation

Inside an `apply` body:

- **A `.member` write installs a declared member.** `tab.status = 'active'` writes the
  key `status`, whose name is statically known (a `.`-member is always a literal
  identifier; Luna has no `.dynamicName`). The machinery pre-checked `status` in step 1
  of §4.1, so this write is collision-safe by construction.
- **A `[expr]` write installs a dynamic member.** `tab['new' . suffix] = value` writes a
  computed key, whose name is not known until runtime. The machinery cannot pre-check
  it, so a dynamic write during application carries **install semantics**: it fails if
  the key already exists, unless an explicit replacing form is used. The author must
  acknowledge the collision case; an unguarded dynamic clobber is not possible.

The compiler distinguishes the two for free: a `.member` assignment target is a
statically-known name (pre-checkable), a `[expr]` target is computed (runtime-checked).
The check is one AST-node-kind test on the assignment target, gated by the lexical fact
of being inside an `apply` body. No dataflow analysis is required.

```
apply = meta fn (&tab: table): !self => {   // : !self, because it installs a dynamic member (§7.5)
  tab.status = 'active';                 // declared: pre-checked, safe
  if (has(tab, computedKey)) {
    throw ApplyError('key exists');      // author acknowledges the dynamic collision
  }
  tab[computedKey] = 'value';            // dynamic: install semantics, fails if present
};
```

### 5.2 Collision handling is declaration-driven, not convention-driven

For **declared** members, collision safety comes from the machinery (step 1 of §4.1),
not from the author remembering to guard. A protocol that declares `status?: string`
cannot silently clobber an existing `status`; application fails first. The `apply`
body's job is custom logic, not collision defense, for declared members.

For **dynamic** members, safety comes from the install semantics of `[expr]` writes:
the write itself fails on an existing key, so a forgetful author still cannot clobber;
the worst case is a failed `apply`, not a silent overwrite.

### 5.3 `noSet` on element members

`noSet` (element-permissions, tables document) is a per-key **write** contract on an
element member: it governs whether that key, once it exists, may be overwritten. It is
unrelated to protocol collision, which is a *definition*-time question handled by §5.1.
A protocol may declare an element member `noSet` to make it read-only after
installation. This is distinct from meta-member privacy (§3.3), which needs no `noSet`.

### 5.4 Declared element types are enforced, and give typed access

When a protocol declares an element member's **type** (`status?: string`, `name: string`,
`age: int`), that type is an **enforced invariant**, not an advisory hint. A write of a
non-conforming value to that key on a conforming table is rejected (a `TypeError`, errors
§9). So a table known to wear the protocol **provably** has each declared element member at
its declared type: a `@person` genuinely has a `string` `name`.

This enforcement is what makes element access **typed** on a value whose type names the
protocol. Because a `@person`'s `name` is guaranteed to be a `string`, the compiler types
`p.name` (and `['name' => n] = p`) as `string`, reading the type off the protocol's
declaration, in O(1), with no shape inference (Luna has no shape types) and no runtime
check. The guarantee comes from the type `@person` plus this enforcement, not from
inspecting the value.

The typing follows the **declared type of the value, never the runtime value's actual
protocols**, because protocol application is dynamic (§4) and statically invisible:

- A value typed **`@person`** (or an intersection `@person & @stringBuilder`, §7.2) has
  typed element access to each named protocol's declared members: `.name` is `string`.
- A value typed bare **`table`** has element access typed **`any`**, even if it happens to
  wear `person` at runtime, because the declared type does not say so. Recover typed access
  by narrowing (`as @person` / `is @person`), one O(1) runtime protocol-tag check.

So a function that wants to hand typed data to its caller **declares the richer return type**
(`fn (): person`, not `fn (): table`); declaring `table` erases the protocol at the boundary
and callers get `any`. The declaration is the knob that propagates typed access; enforcement
is what keeps it sound.

---

## 6. Union: applying many protocols

A table may wear any number of protocols; you `apply` as many as you like, and their
surfaces union.

```
var w = [];
try w->apply(stringBuilder);
try w->apply(jsonWriter);        // w now wears both
```

### 6.1 Meta functions never clash, because they are always qualified

Meta functions are reached only through their protocol (`w->stringBuilder.append(...)`,
`w->jsonWriter.writeField(...)`), so two protocols defining the same meta function name
(`build`, say) do **not** conflict: `w->stringBuilder.build` and `w->jsonWriter.build`
are different qualified names. There is no shared unqualified pool for them to collide
in, so there is nothing to disambiguate and nothing to reject at apply time. This is why
union is unconditional for behavior: **any two protocols compose, because their meta
functions are namespaced by protocol.** The mechanics of qualified reach are in the
views document; the consequence for protocols is that meta-function names are free to
overlap across protocols.

### 6.2 Meta members never clash, because they are protocol-private

Two protocols each declaring `meta buffer` coexist silently (§3.3): each buffer is
reached only from its own protocol's meta functions, so they are never in the same
namespace. Union imposes no meta-member constraint.

### 6.3 Element members can clash, and that is the only apply-time name check

Because element space is flat, two protocols both declaring (or installing) the key
`status` is a real collision. This is the sole naming conflict `apply` must reject:
applying a protocol whose declared element member already exists fails (§4.2, §5.2). If
you need two protocols that both want a `status`, one of them must install it under a
different (possibly dynamic) key, or the tables must be kept separate.

### 6.4 Built-in name avoidance

A protocol may **not** define a meta function whose name is one of the built-in
protocol's (§8): `map`, `filter`, `count`, `pop`, and so on. This is checked at
`proto`-definition time (the protocol is ill-formed if it names a built-in), against
the fixed, known set of built-in names. There is no override, a protocol cannot replace
a built-in meta function, which keeps dispatch from being history-dependent and keeps
built-in calls unambiguous. The practical effect is a naming discipline: a builder
exposes `byteLength`, not a shadowing `count`.

---

## 7. Static application and protocol types

Everything in §1 through §6 is the runtime, value-space model: protocols are `proto`
values, applied at runtime, reflected with `@@`. This section adds the static,
type-space layer: a way to write "a table that wears protocol P" as a *type*, and a way
to construct such a value with the guarantee checked at compile time. The runtime model
is unchanged; this is additional static information that some values carry.

### 7.1 A protocol as a type: `@proto`

A protocol is a `proto` value, but in **type position** the form `@P` denotes a type: the type of
tables that wear `P`.

```
var l: @stringBuilder = ...;    // l is a table statically known to wear stringBuilder
```

This is **not** a special case of the reflection operator; it is the type-position role of `@`,
disambiguated purely by grammatical position, the same way `*` in C is multiplication in an
expression and a pointer in a declaration (type spec §1.1). In **value** position, `@x` reflects a
value and yields a `type` (so `@stringBuilder` in value position is `proto`, the proto's own type).
In **type** position, `@P` is a **wearer refinement**: "a table guaranteed to wear `P`." Same glyph,
two roles, fixed by position at parse time, so there is no runtime ambiguity and the parser needs no
type information. Within type position, `@X` is a wearer refinement only when `X` resolves to a
protocol; `@X` on a non-protocol type is a compile error, decided in semantic analysis (protocol-ness
is static, the type universe is closed).

A wearer refinement is deliberately **not a `type` value** (type spec §5): it has no `typeid`,
because protocol-wearing is a *value* property (the applied-protocol set, reflected by `@@`, views
document), not a type property. So `@P` cannot be produced by `@` on a value, cannot be compared by
`typeid` equality, and cannot be bound to a `type` binding; it is a static guarantee about `->P`, not
a first-class type. This is why it composes with `@@` without collision: `@@` crosses from a value to
the `proto` values it carries (reflection on the protocol axis), while `@P` in type position is a
static guarantee *about* that axis, "`l->P` is present." The two `@`-family operators stay coherent:
`@` is types (value-position reflection, `typeid`s), `@@` is protocols-as-data, and `@P` in type
position is a refinement guaranteeing membership on the `@@` axis.

### 7.2 Composing protocol types: `@P & @Q`

Protocol types compose. `@P & @Q` is "a table that wears both P and Q"; `@P | @Q` is
"wears one or the other" (which narrows like any union, views document and the value
model).

`@P & @Q` is a **well-formed type only if P and Q have disjoint declared element
members.** This is the sole constraint, and it comes from element space being the one
flat, un-namespaced space (§5, §6.3):

- **Meta functions never constrain it.** They are protocol-qualified (reached as
  `l->P.member` / `l->Q.member`), so two protocols with a meta function of the same name
  compose freely; `@P & @Q` imposes nothing on meta-function names.
- **Meta members never constrain it.** They are protocol-private (§3.3), so same-named
  meta members coexist silently.
- **Declared element members are the only thing that can collide**, because they land in
  the shared element space. If P and Q both declare `status`, no table can soundly wear
  both, so `@P & @Q` is an **ill-formed type**: a compile error naming the shared member.

So `@P & @Q` well-formedness reduces to one compile-time check, disjoint declared element
members, and this is the static mirror of the single apply-time collision in the runtime
model (§6.3).

### 7.3 Static application

Static application is the construction operator that produces a value of a protocol
type. It is spelled as `apply` **without call syntax**, distinguishing it from the
runtime `apply` (which is a call, §4):

```
var l: @stringBuilder            = [] apply stringBuilder;
var m: @stringBuilder & @tagged  = [] apply stringBuilder, tagged;
```

Dropping the parens is not cosmetic: `apply(...)` is a runtime call returning `!table`
(it may fail, handled with `try`), while `apply` as an operator is a compile-time-checked
construction whose result type is the precise protocol type. Static `apply` is syntax,
not a callable value.

Static application still applies the protocol at runtime; the static protocol type is
*additional* information the value carries, not a replacement for runtime application. A
value of type `@P` is an ordinary table that also carries the static guarantee that it
wears P.

"Static" describes what is **checked** (the collision check and the result type), not
what is **executed**. Static apply runs the *same* runtime application as dynamic apply,
including the protocol's `apply` body if it has one. A protocol that logs, initializes a
meta member, or otherwise runs attach-time code does so identically under `[] apply P`
and under `p->apply(P)`; the two forms never diverge in what runs. If static apply
skipped the `apply` body, the two forms would produce differently-initialized tables,
which they must not. So static apply is dynamic apply plus a compile-time collision check,
minus the errorability that check removes, plus a sharpened result type, never minus the
`apply` body.

### 7.4 Compile-time collision checking

Static application onto a **statically-known table shape** is collision-checked at
compile time. A table *literal* has statically-known keys, so the check applies even when
the literal is non-empty, not only for the fresh `[]` case:

```
var l: @stringBuilder = ['collision' => "value"] apply stringBuilder;
// compile error if stringBuilder declares an element member `collision`
```

The compiler sees the literal's keys (`collision`) and the protocol's declared element
members, and rejects the application if they intersect, exactly the §7.2 check, now
between a concrete table shape and a protocol rather than between two protocols. A fresh
`[]` is the special case with no keys, so it can never collide on declared members.

Only application onto a table whose shape is **not** statically known (a runtime `var`
table, a table built at runtime) defers this check to runtime. There, a declared-member
collision is a runtime possibility, because whether the runtime table already holds the
key is runtime information.

### 7.5 Errorability, and `!self`-gated dynamic installation

Whether constructing a protocol-typed value can fail depends on two things, and they are
independent of the type's well-formedness:

1. **Is the table shape statically known?** A literal or known shape is compile-checked
   (§7.4). An unknown runtime shape can collide on declared members at runtime.
2. **Does any constituent protocol's `apply` throw?** A protocol with **no `apply`**, or
   a **non-throwing `apply`** (`: self` / `: table`), cannot throw at application: an
   apply-less protocol runs no code, and a non-throwing body (logging, non-dynamic
   initialization) runs but is declared not to throw. A protocol that installs a
   **dynamic** element member (an `[expr]` write during `apply`, §5.1) must declare its
   `apply` as `!self` (a throwing apply). So the throwing case is exactly "the protocol
   has a `: !self` apply", readable straight from the `proto` block.

From these, the errorability tiers:

- **Non-errorable:** static application onto a statically-known table shape, where every
  protocol has no `apply` or a non-throwing one. All collisions are caught at compile
  time and no body can throw, so construction cannot fail. `var l: @P = [] apply P;` needs
  no `try` and no `!`. This is the common case, since most protocols have no `apply` (§2).
  Note that a **non-throwing `apply` body still runs** (§7.3); it simply cannot make
  construction errorable.
- **Errorable:** static application where any constituent protocol's `apply` is `!self`,
  or where the table shape is not statically known. Construction may fail, so the binding
  must handle it.

The independence is the key rule: **`@P & @Q` well-formedness (disjoint declared members)
and the errorability of constructing one are separate.** `@P & @Q` can be a perfectly
well-formed type whose construction is errorable because Q's `apply` is `!self`. The type
does not carry the errorability; the `apply` does.

The precise residual picture:

- **Fresh or literal table, non-throwing protocols:** compile-time sound, non-errorable.
- **Existing or runtime-shaped table:** runtime-fallible on declared-member collision,
  even for non-throwing protocols.
- **Any protocol with `!self` apply (dynamic installation):** adds a runtime failure
  source, declared in the type so callers see it.

Declared (`.member`) installation is always statically collision-checkable and needs no
`!self`; only dynamic (`[expr]`) installation does. This keeps the common case, a
protocol with a fixed declared schema, entirely non-throwing, and confines errorability
to protocols that genuinely compute their members.

### 7.6 Handling construction errors

When construction is errorable (§7.5), the binding must handle the error, using the same
`try` and `!` forms as any other errorable value (value-representation error model). Two
forms:

```
try {
  var l: @stringBuilder & @someThrowingProto = [] apply stringBuilder, someThrowingProto;
} catch error {
  // handle the apply failure
}

var l!: @stringBuilder & @someThrowingProto = try [] apply stringBuilder, someThrowingProto;
// the ! binding converts a thrown apply error into an errorable value
```

The precise semantics of `try`, `catch`, and the `!` binding, statement versus
expression forms, what `catch` binds, the auto-conversion rule, are specified in the
error/exception document, not here. This section fixes only *when* construction is
errorable (so *whether* these forms are required); *how* they behave is the error
document's concern.

---

## 8. The built-in protocol

Every table always wears one protocol implicitly: the **built-in protocol**, whose meta
functions are the table operation catalogue (`map`, `filter`, `reduce`, `count`, `pop`,
`sort`, and the rest, specified in the table-protocol-api document). It is not special
machinery; it is simply the protocol that is always applied and that has **no name**.

Its namelessness is why the built-in is reached by bare `->`:

- `tab->map()` calls the built-in `map` (no protocol name after `->`).
- `tab->stringBuilder.append()` calls a *named* protocol's meta.

Because the built-in is always present and unnameable, and because no other protocol may
take a built-in name (§6.4), built-in meta calls are never ambiguous with any user
protocol. The one collision the built-in still participates in is element-vs-meta: a
table may have a data key named `map`. That is resolved by the operator split, `tab.map`
is the element (data), `tab->map` is the built-in meta, so there is no ambiguity to
resolve (views document, dispatch).

---

## 9. Dispatch summary

The full dispatch rules live in the views document because they concern how `->` and
`.` produce and consume views. In brief, as it pertains to protocols:

- **`tab.name`** , element access. Data only. Miss is `undefined`. Never a meta.
- **`tab->name(...)`** , built-in meta call (nameless protocol).
- **`tab->protoName`** , a view of the named protocol (a `view` value; see views), on
  which `.member(...)` calls that protocol's meta functions.
- Meta functions are never assignable; `->` never appears on the left of `=`.

---

## 10. Worked example: the string builder

The builder is the canonical minimal protocol: element-empty, one meta member, a few
meta functions.

```
stringBuilder = proto {
  meta buf: bytes;                        // protocol-private growable buffer

  append = meta fn (&b: table, value: any): self => {
    // append value.toString()'s bytes into buf; returns the receiver, viewed
    ...
    return self;
  };

  byteLength = meta fn (b: table): int => { ... };   // not `count`, per §6.4

  clear = meta fn (&b: table): self => { ... };

  build = meta fn (b: table): string => {
    // snapshot: copy buf into a new immutable string; b is unchanged and reusable
    ...
  };

  apply = meta fn (&b: table): self => {
    // no declared element members to install, and no dynamic installation,
    // so apply is non-throwing (: self, not : !self); buf is a meta member, auto-present
    return self;
  };
};

// dynamic application onto a var table:
var b = [];
b apply stringBuilder;                     // static form: b now has type @stringBuilder,
                                           // non-errorable (fresh table, non-throwing apply)
let sb = b->stringBuilder;                 // a view of stringBuilder (views document)
sb.append("Hello, ").append(name);         // chained meta calls on the view
let greeting = sb.build();                 // an immutable string; sb still usable

// or construct with a static protocol type directly:
var c: @stringBuilder = [] apply stringBuilder;   // non-errorable, checked at compile time
```

`b` remains element-empty throughout; `buf` is meta state, never an element. `build` is
a snapshot (it does not consume the builder), so `sb` may be appended to and built again.
Because `stringBuilder`'s `apply` installs no dynamic members, it is non-throwing, so
static construction on a fresh table needs no `try`.

---

## 11. Open questions

- **`apply` reach syntax:** whether `apply` is spelled `tab->apply(proto)` (a built-in
  meta) or a bare `apply(&tab, proto)`; the former is consistent with all other meta
  reach, and is assumed above.
- **Applied-check rule:** the precise definition of "protocol is applied to this table"
  that decides whether `tab->someProto` is a view or `undefined` (§4.4), once table
  typing under protocols is fully specified. The mechanism (a membership test against the
  table's applied-protocol list) is settled; the exact typing rules are not.
- **Cross-protocol meta member access:** confirmed *impossible* by design (§3.3); noted
  in case a reflective escape hatch is ever wanted (it should not be).
- **Removal:** whether a protocol can be un-applied (`unapply`?), and what that does to
  meta members and installed element members. Deferred.
