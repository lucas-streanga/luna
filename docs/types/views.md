# Views

A **view** is a value that pairs a table with one of the protocols applied to it, and
redirects member access to that protocol's meta functions. Views are how behavior is
reached in Luna: `tab->stringBuilder` is a view, and `.append(...)` on it calls
`stringBuilder`'s `append`. This document specifies the `view` type, the two access
operators that produce and consume views, how views chain, how meta functions return
them, and the `@@` operator that reflects a view's protocol. It is the companion to the
**protocols** document, which defines protocols, meta functions, meta members, and
application; this document defines how those are *reached*.

---

## 1. Element access and meta access

Luna has two member-access operators, and they partition cleanly. This partition is the
foundation of the whole model; everything else follows from it.

- **`.` and `[]` access element space.** `tab.name` reads the data under the static key
  `name`; `tab[expr]` reads the data under the dynamic (runtime-computed) key `expr`.
  Both reach the same element space; a miss yields `undefined`. Neither ever reaches a
  meta function, under any circumstance, and neither falls back to protocol space.
  `tab.name = v` and `tab[k] = v` write element data.
- **`->` reaches meta space.** `tab->name(...)` calls a meta function; `tab->protoName`
  produces a view. `->` never reads or writes data, and never appears on the left of an
  assignment (meta functions are not assignable).

The `.`-static / `[]`-dynamic distinction is a **global** property of element access,
not something special to any one context. It holds in ordinary code exactly as it holds
inside a protocol's `apply` body: `.name` is always a compile-time-known key and
`[expr]` is always a runtime key, everywhere. Protocol application leans on this split to
tell declared members (`.name`) from dynamically installed ones (`[expr]`) (see
**protocols**), but the split is defined once, globally, in **tables** (Keys and
access); `apply` is only one place it is used, not where it comes from.

Because the element operators (`.`, `[]`) and the meta operator (`->`) denote disjoint
spaces, name collisions between data and behavior cannot arise:

- A table with a data key `map` and the built-in meta `map` is not ambiguous: `tab.map`
  (or `tab['map']`) is the element, `tab->map` is the meta. Different syntax, different
  space.
- No name is ever resolved against a shared pool of "elements and metas together," so
  there is no precedence rule to remember and no dynamic re-resolution when protocols
  are applied.

`.` and `[]` are data; `->` is behavior. Hold those two and the rest is mechanical.

---

## 2. What `->` produces

`->` has two forms, distinguished by what follows it:

- **`tab->name(...)`** , a **bare** name immediately called: a call to the **built-in**
  protocol's meta function `name`. The built-in protocol has no name (protocols
  document), so bare `->` reaches it. `tab->map()`, `tab->count()`, `tab->pop()`.
- **`tab->protoName`** , a named protocol: if `protoName` is applied to `tab`, produces
  a **`view`** value pairing `tab` with `protoName`; if `protoName` is a real protocol
  that is **not** currently applied to `tab`, produces **`undefined`** (§3.2). Calls on
  a produced view use `.`: `tab->stringBuilder.append(...)`.

`protoName` must resolve to a `proto` value in scope. A name that resolves but is not
applied yields `undefined` (a runtime capability check); a name that resolves to nothing
at all (a typo, an unimported protocol) is a **compile error**, not `undefined`. Protocol
references are resolved identifiers, not open-ended string keys, so the compiler knows
every protocol in scope and catches misspellings before they can masquerade as absence.

The bare-vs-named distinction is syntactic: a bare `->name` reaching a built-in is
followed by a call; a `->protoName` yielding a view is followed by `.`. The built-in is
reached directly because it is nameless; every named protocol is reached through a view.

---

## 3. The `view` type

A `view` is a first-class value. It can be bound, passed to functions, returned, and
stored, exactly like any other value.

```
let sb = tab->stringBuilder;    // sb : view
sb.append("x");                 // meta call through the view
someFn(sb);                     // views pass like any value
```

Internally a view carries two things: a reference to the underlying **table** and a
**runtime protocol tag** identifying which applied protocol it views. Consistent with
Luna's dynamic protocol model (protocols are data, applied at runtime), the tag is a
runtime value, not a static type parameter. **`view` is a single surface type with no
generics.** There is no `view<stringBuilder>` to write; a view's protocol is data it
carries, reflected with `@@` (§7), not a type argument.

### 3.1 `.` on a view is a meta call

`.` is uniformly "access a member of the receiver," and what a member *is* depends on
the receiver's kind:

- on a **table**, members are elements (data);
- on a **view**, members are the viewed protocol's meta functions.

So `sb.append(x)` resolves `append` against `sb`'s tagged protocol and calls it. This is
not a second meaning for `.`; it is the same "member of the receiver" rule, applied to a
value whose members happen to be meta functions. A view is not a table, so `.` on it
doing something table-`.` does not is expected, not a contradiction.

Member resolution on a view is dynamic: `sb.append` looks up `append` in the view's
tagged protocol at call time. If the protocol has no such meta function, it is a runtime
error. This matches the rest of the dynamic protocol model, where protocol membership is
a runtime property; views do not introduce static generics to change that.

### 3.2 An unapplied protocol yields `undefined`

`tab->protoName` produces `undefined` when `protoName` is a real, in-scope protocol that
not currently applied to the table. A view of an unapplied protocol is a *missing
capability*, and Luna's absence model handles it exactly like any other missing thing:
`undefined`, navigable with `??` and `?.`. This makes protocol presence a constant-time,
composable check rather than a separate predicate:

```
if (tab is @stringBuilder) { ... }                      // the presence test is `is @P` (is spec §2);
                                                        // a view is not a bool, and there is no truthiness
let s = tab->stringBuilder?.build() ?? "";              // build if capable, else default
let sb = tab->stringBuilder ?? { try tab->apply(stringBuilder); tab->stringBuilder };
```

This mirrors element access precisely: `tab.missingKey` is `undefined`, and
`tab->unappliedProtocol` is `undefined`. Two absences, one rule.

What follows the view then obeys ordinary absence navigation:

- **`tab->stringBuilder.append(x)`** , hard access. If the protocol is applied, this
  calls `append`. If it is not, `tab->stringBuilder` is `undefined` and `.append` on
  `undefined` **throws**, because you asserted a capability the table does not have. This
  is a real error worth surfacing loudly, the same as `tab.missingKey.foo` throwing.
- **`tab->stringBuilder?.append(x)`** , safe access. If unapplied, the `?.`
  short-circuits to `undefined` and nothing is called.

So the presence check is not gone, it is *relocated to the point of use*: careless use
(hard `.`) fails loudly, deliberate checking (`?.`, `??`, truthiness) composes. Requiring
`protoName` to resolve to an in-scope `proto` (§2) keeps a typo from silently becoming
`undefined`; only a genuinely-unapplied-but-real protocol yields it.

Note that **`self` is never `undefined`**: inside a meta function you are running a
protocol the table provably has applied, so the current-protocol view always exists. The
`undefined` case arises only for outward reach (`tab->someProto`), never for `self` or
`view(receiver)`.

---

## 4. Chaining

Because a view is a value and `.` on a view is a meta call, chains are ordinary:

```
tab->map().filter().reduce()
```

reads as: `tab->map()` enters the built-in protocol and calls `map`, whose result is a
table **still in the built-in view**, so `.filter()` is a built-in meta call, and so on.
The chain works because **each meta call's table result is itself viewed**, so `.`
keeps meaning "meta call" for the length of the chain.

### 4.1 The propagation rule

The context that makes chaining work is carried by the **return type** of each meta
function, not by any parser-level notion of "inside a chain." A meta function declares
what it returns:

- **`: self`** returns the receiver, wrapped in the current protocol's view. Fluent
  mutation (`append` returning the builder to chain more appends).
- **`: view`** returns a table wrapped in the current protocol's view. A meta function
  that produces a *new* table but wants the caller to keep chaining returns `: view`;
  the built-in `map`/`filter` do this, which is why `tab->map().filter()` chains.
- **`: table`** (or `string`, `int`, any non-view type) returns a **plain** value.
  Element `.` (data) applies to a returned `table`; the chain has left meta space. This
  is how a chain ends and hands you a normal value.

So whether `.` after a call means meta or data is decided by that call's static return
type: a `view`/`self` return keeps you in meta space, any other return drops you to the
value. There is no ambient chain state; the types carry it.

### 4.2 Dropping the view

You leave a view by landing its result in a non-view type. Binding is the common way:

```
let t: table = tab->map();      // coerce the view to its underlying table
t.key;                          // t is a plain table; `.` is data again
```

A `view` coerces to its underlying `table` (dropping the protocol tag) where a `table`
is expected. So a chain that ends at a binding of table type yields plain data, while a
chain continued with `.` stays viewed. The programmer controls "still in meta space" by
the type they bind to.

---

## 5. Producing a view from inside a protocol

Meta functions return views using `self` and the `view` constructor.

```
fn view(tab: table, protocol: proto = self): view | undefined
```

- **`self`** , inside a meta function, is the receiver wrapped in the current protocol's
  view. It is exactly `view(receiver)`, and is the idiomatic return for fluent methods.
- **`view(tab)`** wraps `tab` in the **current** protocol's view (the protocol whose
  meta function is running). This is what a meta like `map` returns for a freshly built
  table: `return view(newTable);`.
- **`view(tab, someProto)`** wraps `tab` in an explicitly named protocol's view, for the
  rare case a meta function hands back a view of a protocol other than its own.

The `protocol` parameter defaults to the current protocol (`self`), so inside a meta
function `view(t)` means "t, viewed in my protocol." Since protocols are first-class
values, `someProto` is passed directly as a `proto` value; no reflection operator is
needed to name it.

`view(...)` follows the same absence rule as `->` (§3.2): if `protocol` is not applied
to `tab`, it yields **`undefined`**, so `view(tab, someProto)` and `tab->someProto`
behave identically. Its return type is therefore `view | undefined`. In practice the
common internal call, `view(t)` or `self` with the default current protocol, is **never**
`undefined`, because a meta function only runs on a table that provably has its applied
protocol; the `undefined` case arises only when wrapping in an *explicit foreign*
protocol that may not be applied to the table. As with `->`, a subsequent hard `.` on an `undefined`
result throws, so a mistakenly-constructed view fails at once rather than being handed
around to fail distantly. The operator and the constructor thus stay consistent, and a
bad view cannot be silently passed along.

`view(self)` is not a distinct form; it is just `self`. There is one way to say "the
receiver, viewed," and it is `self`.

Note the distinction between the two absences a view can encounter:

- **Protocol not applied to the table** , `tab->P` / `view(tab, P)` is `undefined`. A
  *capability* absence, coalescable with `??` / `?.`.
- **Protocol does not define the meta function** , `someView.M()` where `M` is not a
  meta function of the view's protocol is an *error*, not `undefined` (§3.1). Whether a
  protocol defines `M` is a fixed fact about the protocol, not a per-table capability, so
  reaching for a non-member meta is a mistake, not an absence.

---

## 6. Assignment and views

Views are read/call values, not assignment targets for their metas:

- **`->` never appears on the left of `=`.** `tab->append = fn ...` is meaningless; a
  meta function is not a slot. This is a compile error.
- **`.` on a view is a call, not an assignable slot.** `sb.append = x` does not redefine
  `append`; there is no writable member space on a view. (To store data, write to the
  underlying table's element space with `.` on the table, not on a view.)
- Assignment always targets **element space via `.` on a table**. That is the only
  left-hand form that writes.

---

## 7. `@@`: reflecting a value's protocols

`@@` is the reflection operator for protocol space, the sibling of `@` (which reflects
type). Where `@x` answers "what type is this value," `@@x` answers "what protocol(s)
does this value carry."

```
@sb;        // the type: `view`
@@sb;       // the protocol the view is over: a single `proto` value
@@tab;      // the user protocols applied to a table: an application-ordered list of `proto`
```

- On a **view**, `@@` returns the single `proto` the view is tagged with.
- On a **table**, `@@` returns the applied **user** protocols as a list (a table) of
  `proto` values, in **application order** (§7.1). The built-in protocol is never
  included (§7.2).

`@@` reads the runtime protocol tag a view carries (§3) and, for a table, its
applied-protocol set. Because protocols are first-class data, the result is ordinary
`proto` data: each element can be compared (`p == stringBuilder`), tested for membership
(`stringBuilder in @@tab`), iterated, or passed back to `apply`.

`@@` is an **operator, not a function or a meta member**, for three reasons: a view's
`.` is meta dispatch, so `sb.whichProto` could not name it (it is not a meta function of
the viewed protocol); reading the protocol tag is intrinsic view/table machinery, not
library surface; and an operator has no namespace or import to manage. It is deliberately
in `@`'s visual family: `@` reflects the type layer, `@@` reflects the protocol layer on
top of it, and a reader who knows one guesses the other.

### 7.1 `@@tab` is application-ordered, and order is free

The runtime stores a table's applied protocols as an append-ordered list: each
successful `apply` appends. `@@tab` hands that order back. Tracking it costs nothing,
the storage is naturally ordered, and it would take extra work to discard the order (by
hashing into an unordered set). For the small number of protocols a table has applied,
membership (`p in @@tab`) is a short linear scan, so no hash set is wanted anyway.

The order is **deterministic but not semantically load-bearing**: protocols are an
unordered set of capabilities (meta functions never clash, so application order does not
affect dispatch), and `@@tab` reports application order only because a deterministic
order is friendlier to iterate and debug than an arbitrary one. Do not attach meaning to
the order beyond "the sequence in which these were applied."

### 7.2 The built-in protocol is never in `@@tab`

`@@tab` lists user protocols only. The built-in protocol (protocols document) is always
applied to every table, and is deliberately **not** a reifiable `proto` value, so it
never appears. This is by design, not omission, and it is what keeps every element of
`@@tab` useful:

- The built-in has **no name**, so it could not be compared against anything you can
  write; an element you cannot name breaks `@@v == something`.
- The built-in cannot be **re-applied** (it is already on every table; applying it is
  meaningless), so it would be the one element of `@@tab` you must skip when iterating to
  `apply` protocols elsewhere.
- The built-in is **information-free** as a value: it is true of every table, so its
  presence in a "what capabilities does this table have" answer conveys nothing.

A value that is nameless, non-re-applicable, and information-free cannot sit usefully in
a list whose elements are meant to be compared, applied, and iterated. So the built-in
is the always-present substrate, reached only by bare `->`, and `@@tab == []` cleanly
means "a plain table with no applied user protocols." Discriminating "is this a built-in
view" is not a reflection-list question; if it is ever needed, it is a dedicated
predicate, not membership in `@@tab`.

### 7.3 `protoName`: names when you actually need them

`@@` yields protocol *values*, which is what nearly every consumer wants (`apply`,
`view`, `==`, membership all take values, not names). For the peripheral cases that
genuinely need a name string (reflection tooling, serialization, debugging), a separate
`protoName(p: proto): string` returns a protocol's name, and callers can build a
name-keyed table themselves. Name lookup is deliberately not the shape of `@@tab`,
because keying by name would be both less useful (names are not what you dispatch on)
and lossy (two distinct protocols bound to same-named variables in different modules
would collide as keys, whereas distinct `proto` values are distinct list elements).

---

## 8. Dispatch, end to end

Putting the operators together, for any table `tab`, view `v`, and names:

| Expression | Meaning |
|-|-|
| `tab.name` | element read, static key (data); `undefined` if absent |
| `tab[expr]` | element read, dynamic key (data); `undefined` if absent |
| `tab.name = x` | element write, static key (data) |
| `tab[expr] = x` | element write, dynamic key (data) |
| `tab->name(...)` | built-in protocol meta call |
| `tab->protoName` | a `view` of the named protocol, or `undefined` if not applied |
| `tab->protoName.name(...)` | meta call on that view; throws if the protocol is not applied |
| `tab->protoName?.name(...)` | meta call, or `undefined` if the protocol is not applied |
| `v.name(...)` | meta call through the view `v` |
| `v.name = x` | error (a view has no assignable members) |
| `tab->name = x` | error (meta functions are not assignable) |
| `@tab` / `@v` | the type (`table` / `view`) |
| `@@v` | the view's protocol (a single `proto`) |
| `@@tab` | the table's user protocols (application-ordered list of `proto`; built-in excluded) |

Resolution rules:

- **`.` and `[]`** resolve against element space on a table (`.` a static key, `[]` a
  dynamic one; the distinction is global, §1). On a **view**, `.` resolves against the
  tagged protocol's meta functions instead. `.` is always member-of-the-receiver; the
  receiver's kind decides what a member is.
- **`->`** reaches meta space: a bare name calls the built-in protocol, a protocol name
  yields a view of that named protocol (or `undefined` if the protocol, though a real
  in-scope `proto`, is not applied to the table). A name that resolves to no `proto` in
  scope is a compile error, not `undefined`. Never data, never assignable.
- **Meta resolution on a view is dynamic**, resolved at the call against the view's
  runtime protocol tag; an absent meta function is a runtime error, consistent with
  Luna's runtime protocol model.

---

## 9. Worked examples

### Built-in chain (data pipeline)
```
let total = tab->map().filter().reduce();
```
Each of `map`, `filter` returns `: view` (built-in), so `.` keeps calling built-in
metas; `reduce` returns a scalar, ending the chain.

### Named protocol, bound view (string builder)
```
var b = [];
try b->apply(stringBuilder);
let sb = b->stringBuilder;          // sb : view, tagged stringBuilder
sb.append("Hello, ").append(name);  // append returns : self, so chaining continues
let greeting = sb.build();          // build returns : string, chain ends
```

### Leaving a view
```
let t: table = tab->map();          // view coerced to table on binding
let v = t.value;                    // t is plain; `.` is data
```

### Reflecting protocols
```
if (@@sb == stringBuilder) { ... }  // compare the view's protocol against a proto value
if (stringBuilder in @@b) { ... }   // membership test by value
let applied = @@b;                  // the table's user protocols, application-ordered
foreach (p in @@b) { other->apply(p); }   // re-apply this table's protocols elsewhere
```
The built-in never appears in `@@b`, so the `foreach` never has to skip an
un-re-appliable element.

---

## 10. Open questions

- **`@@` on a value with no user protocols:** for a bare table (only the built-in) or a
  non-protocol value (an `int`), `@@` on a table returns `[]` (empty list, no user
  protocols); `@@` on a non-table, non-view value is likely an error or `[]`, to be
  confirmed once the type surface of `@@` is settled.
- **View coercion direction:** a `view` coerces to `table` (drop the tag) where a
  `table` is expected. A `table` must **not** implicitly coerce to a `view` (that would
  blur the element-vs-meta boundary the operator split keeps clean); this should be
  stated as prohibited. Confirm.
- **View equality and identity:** whether two views of the same table and protocol are
  `==`, and whether a view is `==` to its underlying table (it should not be; they are
  different kinds).
- **`isBuiltin`-style predicate:** whether discriminating "is this a built-in view" is
  ever needed; if so it is a dedicated predicate, not `@@` membership (§7.2). Deferred
  until a real use appears.
