# Destructuring

Destructuring binds several variables from a table in one statement, by position or by key.
It is the inverse of a table literal: where a literal *builds* a table from parts,
destructuring *takes it apart* into bindings. The syntax mirrors table-literal syntax, and
the two modes follow the same positional-versus-keyed distinction as the rest of the
language.

The design is PHP-flavored, with one deliberate departure: **silent data loss is not the
default.** Where PHP silently drops trailing values, Luna requires you to say so (with a rest
element), so a shape mismatch is caught rather than hidden.

---

## 1. Positional destructuring

Square brackets with bare targets bind by implicit integer keys `0, 1, 2, ...`:

```
[a, b] = pair;           // a = pair[0], b = pair[1]
[x, y, z] = triple;      // x = triple[0], y = triple[1], z = triple[2]
```

The implicit keys are exactly the list keys, so positional destructuring reads a list (or the
integer-keyed prefix of a table) into ordered bindings.

### 1.1 Exact length by default

Positional destructuring expects the source to have **exactly** as many integer-keyed values
as there are targets. A mismatch is an error, not a silent drop:

- **Too few values** (`[a, b, c] = pair` where `pair` has two): an error. There is no value to
  bind `c`. (A `TypeError` panic at runtime, or a compile error where the source shape is
  statically known.)
- **Too many values** (`[a, b] = triple` where `triple` has three): also an error by default.
  The extra value is data you would be discarding, and discarding it silently hides a shape
  mismatch (the classic PHP footgun). So bare positional destructuring asserts the exact
  shape.

To accept a longer source, say so explicitly with a rest element (§1.2). This is stricter than
PHP on purpose: `[a, b]` means "exactly two," and dropping a tail is an act you opt into, not a
default.

### 1.2 The rest element: `...rest` and `..._`

A trailing rest element consumes the remaining values, and you choose whether to keep or drop
them:

```
[a, b, ...rest] = source;    // a, b bound; rest = a list of everything after
[a, b, ..._]    = source;    // a, b bound; the tail is explicitly discarded
```

- **`...rest`** captures the remaining integer-keyed values into a new **`list`** (reindexed
  from zero). `rest` is empty (`[]`) if there is nothing after.
- **`..._`** is the wildcard rest (wildcard spec): it explicitly discards the tail. This is the
  opt-in form of PHP's silent trailing-drop, made visible.

So the three positional shapes are: `[a, b]` (exactly two), `[a, b, ...rest]` (two plus a
captured tail), and `[a, b, ..._]` (two plus a discarded tail). Silent loss never happens; the
`...` says what becomes of the rest.

### 1.3 Skipping single positions

`_` skips one position without binding it (wildcard spec §6):

```
[a, _, c] = triple;      // bind positions 0 and 2, skip position 1
```

Each `_` skips exactly one position and is independent; `[_, _, c]` skips the first two. This
is distinct from `..._`, which discards *all* remaining positions.

---

## 2. Keyed destructuring

Bracketed `key => target` pairs bind by explicit key, in the same syntax as a keyed table
literal:

```
['name' => n, 'age' => a] = person;      // n = person['name'], a = person['age']
```

Keyed destructuring **names what it wants**, so unmentioned keys are simply **ignored**, no
error, no rest element needed. Naming `'name'` and `'age'` is already saying "just these," so a
source with additional keys is fine and expected. This is the natural, unsurprising semantics
for keyed access, and it is why keyed destructuring needs no exact-length rule (unlike
positional, §1.1): you asked for specific keys, and other keys are irrelevant.

### 2.1 Absent keys bind `undefined`

A named key that is not present in the source binds **`undefined`** (the absence sentinel,
coalescing spec), the same result as `source['missingKey']`:

```
['name' => n, 'nickname' => nick] = person;   // nick = undefined if person has no 'nickname'
```

This is consistent with ordinary table access: reading an absent key yields `undefined`, and
keyed destructuring is ordinary keyed access spread across bindings. Handle a possibly-absent
binding with the coalescing operators (`nick ?? "none"`).

### 2.2 Skipping keys

`_` as a keyed target acknowledges a key without binding it:

```
['name' => _, 'age' => a] = person;      // require/read 'name' but discard it; bind 'age'
```

This reads the key (so its side of the pattern is present) but drops the value, the keyed
analogue of the positional `_`.

---

## 3. Positional and keyed are distinct modes

A destructuring pattern is positional (bare targets, integer keys) or keyed (`key =>` pairs);
the mode follows the same distinction as table literals and the `.`/`[]` split. The two are
not mixed in one pattern in the common case; a source with both integer and string keys is
read positionally for its integer part and by key for its named part using the appropriate
pattern. (Whether a single pattern may mix `[a, b, 'k' => c]` is left to the grammar, open
questions.)

---

## 4. Binding forms

Destructuring binds with the same declaration forms as ordinary bindings (variables spec):

```
let [a, b] = pair;           // let bindings
var [a, b] = pair;           // var bindings (rebindable)
const [a, b] = pair;         // const bindings (requires a comptime-known source)
[a, b] = pair;               // assignment to existing bindings (no keyword)
```

Each target follows the chosen form's rules: `const` destructuring requires the source to be
comptime-known (so each bound value is a constant), `let`/`var` bind at runtime.

---

## 5. Types of destructured bindings

A destructured binding takes the **type of the source element it binds**; destructuring
introduces no typing rules of its own, it inherits whatever the source's element access
already yields. Because Luna has **no shape types and no generics**, that inherited type is
often `any`, but not always. The gradient, from most to least precise:

- **Comptime-known source (`const` table):** exact types. The compiler knows the source's
  contents (const-table representation, tables), so each binding's type (and value) is known
  precisely. `const [a, b] = [1, "x"]` gives `a: int`, `b: string`.
- **Protocol-typed source (`@person`, or an intersection `@person & @stringBuilder`):**
  typed by the protocol's **declared, enforced** element members (protocols §5.4). `['name'
  => n, 'age' => a] = p` where `p: @person` gives `n: string`, `a: int`, read off the
  protocol declaration, soundly, because a `@person` provably has those element types.
- **Bare `table` (or `any`) source:** each binding is **`any`**. With no shape types and no
  static knowledge of the value's runtime protocols, the compiler cannot type the elements.
  `[a, b] = someTable` gives `a: any`, `b: any`. Recover types by narrowing a binding with
  `as` (the checked narrowing operator) where you know better.

The rule throughout is that typing follows the **declared type of the source, never the
runtime value's actual protocols** (which are dynamic and statically invisible, protocols
§5.4). A function that returns `table` yields `any` element bindings even if it always
returns a `@person` at runtime; a function that returns `person` yields typed bindings. So
declaring a richer source type is how typed destructuring is obtained; bare `table`
destructures to `any`, by design, not omission.

The `...rest` binding is always a **`list`** (§1.2) regardless of element typing; its
*elements* carry whatever element type the source had (typed for a `@proto` or const source,
`any` for a bare table).

---

## 6. Summary of rules

- **Positional** (`[a, b]`): binds integer keys `0..n-1`; **exact length by default** (too few
  or too many is an error); `...rest` captures the tail as a `list`, `..._` discards it; `_`
  skips one position.
- **Keyed** (`['k' => v]`): binds named keys; **unmentioned keys ignored**; absent named key
  binds `undefined`; `_` as a target discards a named value.
- **Silent data loss never happens by default:** dropping a positional tail requires `..._`;
  keyed ignores only the keys you did not name.

---

## 7. Open questions

- **Mixed patterns:** whether a single pattern may combine positional and keyed elements
  (`[a, b, 'k' => c]`), pending the destructuring grammar.
- **Nested destructuring:** binding through nested structure (`[[a, b], c] = ...`,
  `['user' => ['name' => n]] = ...`) and how `_` and `...` compose inside nesting.
- **Rest position:** whether a rest element may appear anywhere but the end (`[a, ...mid, z]`),
  or only trailing (current assumption: trailing only).
- **Destructuring in other positions:** whether the same patterns bind function parameters
  (`fn ([a, b]) => ...`) and loop variables (`foreach ([k, v] in pairs)`), pending those
  grammars.
