# Equality

`==` is **strict** equality. It never coerces and never approximates: two values are equal only
when they are the **same underlying type** and, within it, the **same value**. "Underlying" is
the one deliberate transparency: a **constraint** refines the *description* of a value, not the
value (`byte` 65 *is* `int` 65, constraints §3, §9), so constraints are **transparent to `==`**,
while **nominal identity** (an enum variant, an error type) is part of what a value *is* and
never erases. When strictness is not what you want, the escapes are explicit, conversion
functions (conversion spec) to compare across types, and `match` (match spec) for partial or
structural-subset matching. `==` itself is exact.

The evaluation of `a == b` has a uniform shape:

1. **Flags first**: the `null`/`undefined` states compare as their own values (§5).
2. **Same `typeid`**: the fast path, one integer compare, then value/identity comparison
   dispatched by the type's category (§2).
3. **Different `typeid`s: erase constraints and re-compare.** Load each type's **`valueBase`**,
   its precomputed constraint-erasure, from the `typetable` (value-representation §4). If the
   bases **differ**, the result is **`false`**, no value comparison (§1): different underlying
   types are never equal. If the bases **match**, the two values are the same underlying type
   described at different precisions (a `byte` against an `int`, a `list` against a `table`), and
   comparison proceeds by **value under the shared base's category** (§2).

Step 2 is unchanged from a pure typeid-first design and remains the overwhelmingly common case.
Step 3 costs two indexed loads into the hot, read-only, compile-time-fixed `typetable` plus one
compare, and it rarely runs at all: wherever both operands' **static** types are known, the
compiler resolves `valueBase` at compile time and emits either the direct base comparison or a
constant `false`, so the table loads exist only for dynamically-typed operands (`any`, unions).
In statically-typed code the erasure rule costs nothing.

---

## 1. Different base types are never equal

`a == b` is **`false`** whenever the two operands' **`valueBase`s** differ. There is **no
cross-type equality** between underlying types: `1 == 1.0` is **false** (an `int` is not a
`double`, the bases differ), `"1" == 1` is false, `null == undefined` is false (§5). This follows
directly from the no-implicit-coercion stance (the language never coerces `1` to `1.0` to make a
comparison succeed), and the payoffs stand:

- **A fast path.** Same-typeid comparison is one integer compare; a base mismatch is two table
  loads and a compare, and compiles away entirely where static types are known (intro).
- **It strengthens conversions.** To compare across underlying types, convert explicitly
  (`x.toDouble() == y`, conversion spec); the conversion is visible in the source.
- **It strengthens `match`.** Cross-type and partial comparisons are `match`'s job (match spec).

What changed from a pure typeid-first rule, and why: a **constraint typeid is not a different
type of value**, it is the *same* base value carrying a more precise description (constraints §3,
§9.2), so rejecting `someByte == 65` on the typeid alone would deny that `byte` 65 and `int` 65
are the same integer, and it broke real code, `bytes` filtering (`x != 0` with `x` a `byte`,
bytes §6), a `list`-declared value compared against an equal-content plain `table`, any
constrained value against a literal (a literal always bears the plain base type). So `==` **erases constraints**:
`someByte == 65` is `true` when the payloads match, `somePort == someByte` compares the two ints,
`someList == someTable` compares structurally. Erasure applies **only** to constraints:

- **Nominal identity never erases.** Enum variants are distinct kinds (`circle == square` is
  `false` however equal the payloads, §6); distinct error types are unequal; the erasure chain
  follows constraint edges only, and every non-constraint type is its own `valueBase`.
- **Type values are exempt.** `@a == @b` compares two values *of type `type`*, whose payload
  **is** the `typeid`, so `@someByte == @someInt` is **false** (`byte` and `int` are different
  types, constraints §8). Erasure applies to an operand's type *tag*, never to a type-valued
  *payload*; reflection sees constraints exactly as before.
- **Relational parity.** `<`, `<=`, and arithmetic already operate on the widened base
  (constraints §5); `==` was the one comparison that did not, and now agrees.
- **Keying and hashing must agree with `==`.** Any structure keyed by `==` (table keys, tables
  spec; membership and dedupe operations) must hash the **pair (`valueBase`, payload)**, never
  the raw typeid, so `t[someByte]` and `t[65]` are the same slot, as equality demands.

So `==` presupposes equal **bases**: the value comparisons in the rest of this document all
assume the operands' `valueBase`s match, and "same type" below means same base, with the payload
comparison run under that base's category.

---

## 2. Three equality categories

Given equal types, how the values compare depends on the type's category. There are three, and
every type belongs to exactly one:

- **Value-equality types**, compared by their **contents**: the scalars (`int`, `double`, `bool`,
  `byte`, `null`), `string`, and `table` (structural, §4). Two distinct values with the same
  contents are equal.
- **Identity-equality types**, compared by **identity** (same underlying object): `stream`, `fn`,
  `promise`, `capability`, `command`, and a **table that has an applied `identityEquality` protocol**
  (§4.4, which is how builders compare). Two of these are equal only if they are the **same** value,
  because comparing their contents is impossible (a `stream` cannot be read without consuming it,
  and may be infinite), undecidable (`fn` extensional equality), or meaningless (a `capability` is a
  zero-data singleton; a builder is a transient accumulator). Identity equality is a pointer compare.
- **The `type` value**, compared by **`typeid`** (§3): a single integer compare on the canonical
  type identity. This is a degenerate value-equality (the "content" of a type is its `typeid`).

Two types stand outside the three because their comparison is *defined specially*: **`secret`**
never equals anything, not even itself (§5), comparing it is disallowed by design; and **`regex`**
compares by **source** (§5), its pattern text, rather than by its compiled form.

The dividing principle: a value's contents are compared **only** when comparing them is possible,
decidable, and meaningful. Where it is not, equality is identity (or, for secrets, disallowed). This
is why streams, functions, promises, and builders compare by identity, there is no coherent content
comparison, so identity is the only sound answer, and attempting a content comparison (consuming a
stream, deciding function equivalence, reading a secret) would be wrong, impossible, or a leak.

---

## 3. Scalars and `type`

- **`int`, `bool`, `byte`**, trivial value equality (a machine-integer compare on the payload).
  Exact, no surprises. `byte` (and every int-based constraint, `port`, `i8`..., constraints §10)
  erases to `int` (§1): `someByte == 65` is a plain integer compare.
- **`type`**, a `typeid` compare (value-representation §4). Because type equality is **canonical**
  (type spec §3), `int | double == double | int` and `number == int | double` are true by a single
  integer compare, no structural walk. This is the cheapest possible comparison.
- **`double`**, **IEEE 754 semantics**, and therefore **not trivial** (double spec §1.1):
  - **`nan != nan`.** A nan equals nothing, including itself, so `==` on doubles is **not
    reflexive**: `let x = 0.0/0.0; x == x` is **false**.
  - **`-0.0 == +0.0`** is **true**, although the two have distinct bit patterns.

  So `double` `==` is IEEE `==`, the least surprising choice for numeric code, and it **cannot** be
  implemented as a bit compare (bit equality would wrongly make `nan == nan` true and `-0.0 == +0.0`
  false). The cases where IEEE `==` is inconvenient, matching a nan, treating the two zeros as one,
  are served by the **total order** used in `match` and sorting (double spec §2.2, match spec §7),
  which is reflexive (`nan` equals `nan`, both zeros merge). The two relations differ **only** on
  nan and signed zero, and each is used where it fits: `==` for semantic equality, the total order
  for matching and ordering.

This nan behavior propagates into `table` equality (§4): a table containing a nan is not equal to
itself under `==`, because element comparison is `==`.

---

## 4. Table equality: strict and structural

Two tables are equal when they have the **exact same structure**: same length, same key-to-value
pairs, same types, same values, and **same order** — and, beyond element space, the same
**applied protocols** with equal **granted state** (§4.5). Equality is strict because `match`
already covers partial and subset matching (match spec), so `==` is reserved for exact
structural identity.

Concretely, `a == b` for tables holds iff:

- **Same length**, differing counts are unequal (a fast reject).
- **Same entries in the same order**, iterating both in order (tables are insertion-ordered,
  tables spec), each pair matches: the **keys equal** and the **values equal**, both by `==`.
- **Recursively**, value comparison is `==`, so nested tables compare structurally, scalars by
  their scalar rules, and identity-typed values (a stream in a table) by identity.

Two consequences of "by `==`" throughout:

- **Order is significant.** `['a' => 1, 'b' => 2] != ['b' => 2, 'a' => 1]`: the entries are the
  same, but the order differs, and order is observable (via `foreach`, tables spec), so it is part
  of a table's identity and therefore part of equality. Tables are ordered values, not unordered
  maps.
- **Element base-strictness.** `['a' => 1] != ['a' => 1.0]`: the values `1` and `1.0` have
  different bases, so unequal (§1), so the tables are unequal. Table equality inherits element
  `==` exactly, including constraint erasure (a table of `byte`s equals a table of the same
  `int`s; a `list` equals an equal-content, equal-order `table`, their bases match, §1) and IEEE
  nan (a table with a nan element is not equal to itself, §3).

### 4.1 Termination: tables cannot form cycles

Structural equality recurses into nested tables, so it would be a concern if a table could contain
itself (a cycle would make the recursion non-terminating). It cannot. Tables are **copy-on-write
value types** (value-representation §6): storing a table into another table stores a **value**
(a copy, COW-shared until mutation), not a back-reference, so `t['self'] = t` stores a *snapshot* of
`t`, not a link to `t`. References (`&`) are a binding-level construct and cannot be stored in a
table. So a table's structure is always a **finite, acyclic** tree of values, and structural `==`
always **terminates**. No cycle detection is needed; value semantics preclude cycles by
construction. (A stream held in a table does not reopen this: a stream is identity-compared, never
recursed into, §2.)

### 4.2 Copy-on-write makes equality fast

Copy-on-write gives table equality a powerful fast path. Two table values copied from one another
**share storage** until one is mutated (value-representation §6.1), so they are **pointer-equal**
while unmutated. Because shared storage means identical contents:

- **Pointer-equal tables are equal**, `==` short-circuits to **`true`** on a storage-pointer match,
  in O(1), with no structural walk. Comparing a table to a copy of itself (a common case, and the
  `t['self'] = t` snapshot of §4.1) is therefore instant.
- **Pointer-unequal tables fall through** to the structural comparison above, they may still be
  equal by content, having been built independently.

This is the general rule for reference types (§4.3): a shared-storage match is a sound fast path to
`true`, never to `false`.

### 4.3 Pointer equality is a fast path to `true`, never to `false`

For every reference-backed type (`string`, `table`, and the identity types), the runtime may
short-circuit `==` to **`true`** when the two operands share the same storage pointer, because the
same object trivially has the same value. The converse never holds: **distinct pointers imply
nothing**, two separate allocations may hold equal contents, so a pointer mismatch must fall
through to the type's real comparison (content for `string`/`table`, and for identity types the
pointer mismatch is itself the answer: not identical, so not equal). Pointer equality is thus a
sound positive optimization, never a decision procedure for inequality, except for identity types,
where identity *is* the definition.

### 4.4 A protocol may declare its applications identity-equal (`identityEquality`)

A table's equality can be switched from structural (the default) to **identity** by a protocol it
has applied. A protocol declares this with **`identityEquality`** inside its `proto` block:

```
const stringBuilder = proto {
  identityEquality;              // tables with this applied protocol compare by identity, not structure
  var buf: bytes = bytes();      // ungranted: private, per-table (protocols §2.2)
  const get append = fn (b: @stringBuilder, s: string): self => { ... };
};
```

A table that has applied **any** protocol declaring `identityEquality` compares by **identity** (§2), not
by structure. This is what makes **builders** compare by identity: a builder is a table applying
`stringBuilder`, and `stringBuilder` declares `identityEquality`, so builders are identity-equal.
There is **no special builder type**; the behavior is entirely a consequence of the protocol
declaration, which is exactly why a **user-defined** builder-like protocol gets the same treatment,
declare `identityEquality`, and its applications are identity-equal too. Builtin and user builders are
first-class equals; `stringBuilder` is just the builtin protocol that happens to declare it.

Why this is a bounded, declarative switch and **not** a user-defined `==`:

- **The choice is only identity-vs-structural**, never arbitrary code. Both are sound equivalence
  relations (reflexive, symmetric, transitive), so equality can never be corrupted into a
  non-equivalence by a user, and `==` stays **total and non-throwing**. Arbitrary user `==` would
  also be an unbounded performance and correctness footgun; `identityEquality` avoids all of it.
- **Why builders need it.** A builder's real state is its private buffer — an **ungranted**
  protocol member (protocols §2.2), which is never part of the equality surface (§4.5). So
  structural equality would compare builders only on their (usually empty) element data and call
  all empty builders equal, which is wrong. Identity is the correct relation for a transient
  accumulator; to compare *contents*, build and compare the results
  (`b1->build() == b2->build()`, comparing strings), exactly as one compares the materialized
  results of two streams rather than the streams.

**Conflict rule: any identity wins.** If a table has several protocol applieds and *any* of them declares
`identityEquality`, the table is identity-equal. Identity is the conservative choice (if any applied
protocol considers its applications not structurally comparable, trust it). This is statically visible
whenever the protocol set is known (static apply, `@P` types), so it is an inspectable property, not
a hidden one, and `identityEquality` is a rare, explicit, loud declaration, so a table does not
become identity-equal by accident: some protocol it has said applied so, in its definition.

### 4.5 Protocol state: the granted surface compares, private state does not

A table's applied protocols are part of its identity (protocols §5). For two structurally-equal
element spaces to make two equal tables, the protocol axis must also agree:

- **The applied-protocol sets must match**, as sets (application order is not load-bearing,
  protocols §8). A `@person` is never equal to a bare table with the same elements: one carries
  a contract and state the other lacks.
- **Each protocol's `get`-granted per-table members compare by `==`.** The granted surface is
  the value's public state, and it is exactly the equality surface (R96, protocols §5): two
  `@person`s with different `name`s are unequal, whatever their element spaces hold.
- **Ungranted members never compare.** Private state is incidental *by declaration* — a cache
  or buffer must not make two logically equal values unequal. Where hidden state *should*
  distinguish values, the protocol declares `identityEquality` (§4.4); that rule is this one's
  other half, not an exception to it.
- **Definition-fixed members are vacuous** (uniform across every application, protocols §2.1)
  and are skipped. A fn-typed `get` member compares by identity, as `fn` always does (§2).

One boundary, stated once (protocols §5): the `get` surface is the access surface, the equality
surface, and the serialization surface.

---

## 5. Strings, `null`, `undefined`, capabilities, secrets, commands, regex

- **`string`**, value equality by contents. The fast path (§4.3) applies: **equal storage pointers
  imply equal strings**, so `==` short-circuits to `true` on a pointer match (a `const` string
  compared to itself, or two references to one string). On a pointer mismatch, `==` compares
  **length then bytes** (a `memcmp`). Strings are **not interned**, so distinct allocations of the
  same text are pointer-unequal and require the byte comparison; the pointer path is an
  optimization, not the semantics. A consequence at the FFI boundary: a foreign/C string is a
  distinct allocation from an equivalent native string, so they are pointer-unequal and compared by
  bytes (equal if the bytes match); callers crossing the FFI boundary must not assume pointer
  identity.
- **`null`**, `null == null` is **true** (one value, equal to itself). `null` is a value-equality
  value with a single inhabitant.
- **`undefined`**, `undefined == undefined` is **true** (absence equals absence, reflexively, so
  that every value equals itself, the sole exception being IEEE nan, §3). `null == undefined` is
  **`false`** (different types, §1): present-nothing is not absent.
- **`capability`**, identity equality (§2): same capability is equal, a pointer compare. Capabilities
  are immutable, zero-data singletons reached only through `use` (capabilities spec), so identity is
  the only meaningful relation, "is this the same capability."
- **`secret`**, **never equal, not even to itself**: `s == s` is **`false`** for any secret `s`, and
  `s == anything` is `false`. A secret is deliberately opaque (secret spec), so whether two secrets
  are equal is *unknowable by design*, and any comparison would be a probing oracle (including a
  timing side channel). So `==` on a secret always yields `false`. This is the **one deliberate
  exception to reflexivity** (every other value equals itself except IEEE nan): a secret is not
  introspectable, so "is this secret equal to itself" has no answer the language will give. To
  compare secrets, `reveal` them (an explicit, auditable act) and compare the revealed values.
- **`command`**, identity equality (§2): a command equals itself (reflexive) but no distinct command,
  a pointer compare. Commands are structured, inert, and often carry secrets (command spec), so
  structural comparison would both be a rarely-wanted operation and drag in secret comparison (a
  leak); identity sidesteps both. To compare what two commands *would run*, inspect their parts
  explicitly.
- **`regex`**, **source equality**: two regexes are equal iff built from the same **pattern source
  and flags**. This is *source* equality, not behavioral equivalence, `/(a)/` and `/(?:a)/` match the
  same strings but have different source, so they are **unequal** (regex equivalence, "do these match
  the same inputs," is undecidable in general, like function equivalence). So `regex` `==` compares
  the pattern text and flags, cheap and well-defined, and does not claim to decide behavioral
  sameness.

---

## 6. Summary: how each type compares

`a == b` is `false` first if the operands' **`valueBase`s** differ (same-typeid fast path, else
erasure, §1, intro). Otherwise, comparison is by the shared base type below. "Reflexive" means
`x == x` is true (it is, for every type except the two noted).

| Type | `==` compares by | Notes |
|-|-|-|
| `int`, `bool`, `byte` | value (integer compare) | trivial, exact; int-based constraints erase to `int` (§1): `someByte == 65` is true when payloads match |
| `double` | **IEEE 754** | `nan != nan` (**not reflexive**); `-0.0 == +0.0`; no bit-compare; total order handles matching/sorting (§3) |
| `null` | value | `null == null` true; single inhabitant |
| `undefined` | value | `undefined == undefined` **true** (needed for absence checks); `null == undefined` false |
| `string` | contents (length then bytes) | pointer fast-path to `true`; not interned; FFI strings compared by bytes (§5) |
| `type` | canonical `typeid` | one integer compare; `int\|double == double\|int` (§3) |
| `table` (default) | **structural**: same length, key/value pairs, bases, values, **order**; plus applied-protocol sets and their `get`-granted per-table members (§4.5) | recursive; COW pointer fast-path to `true`; terminates (acyclic value type, §4); `list` erases to `table` (§1) |
| `enum` value | same **variant** (nominal, no erasure, §1), then payload structurally | `circle != square` regardless of payloads; same variant compares its payload table by `==` |
| `table` applying `identityEquality` | **identity** | how builders compare; any such protocol makes the whole table identity-equal (§4.4) |
| `stream` | identity | contents uncomparable (single-pass, maybe infinite) |
| `stringBuilder` / builders | identity | via the `identityEquality` protocol declaration (§4.4); compare `->build()` results for content |
| `fn` | identity | signatures compare via `@f == @g` (a separate, type-level question) |
| `promise` | identity | a single future handle |
| `capability` | identity | zero-data singleton |
| `command` | identity | reflexive; avoids comparing embedded secrets (§5) |
| `regex` | **source** (pattern + flags) | source equality, not behavioral equivalence (undecidable) (§5) |
| `secret` | **never equal** | `s == s` is **false** (**not reflexive**); opaque by design; `reveal` to compare (§5) |

The two non-reflexive rows are deliberate: **`double`** (IEEE nan) and **`secret`** (opacity). Every
other type is reflexive. Strictness throughout: no coercion, exact structure, with conversion
functions and `match` as the deliberate escapes for cross-type and partial comparison.

---

## 7. Open questions

- **`error` value equality.** Errors are field-carrying value types (errors §5) but no row above
  covers them; structural comparison is complicated by `stacktrace` (two otherwise-identical
  errors thrown in different places differ) and `cause` chains. Undecided between structural-
  minus-trace, identity, and structural-in-full.
- *(The former `view`-equality question is **mooted by R95**: the `view` type no longer
  exists.)*

Cross-type numeric comparison is **not** an open question: `1 == 1.0` is `false` by design (§1), and
the intent is expressed by an explicit conversion (`someInt.toDouble() == someDouble`, conversion
spec). The explicit conversion is the answer, not an ergonomic gap.
