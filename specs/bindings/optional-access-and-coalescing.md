# Optional Access & Coalescing

Semantics for `?.`, `??`, `???`, and their compound-assignment forms in Luna — plus how
the same absence model extends to protocol space (`->`, `?->`).

## The model: three states of "no value"

Luna keeps `null` and `undefined` genuinely distinct, and this distinction is what the whole operator set is built to navigate:

- **`undefined`, absent.** `undefined` can *never* be stored as a table value. It appears only in variable and return slots, and it is **language-produced, never programmer-written** (undefined spec): the two productions are a missing key and a void return. It is therefore the one unambiguous sentinel for "there is no entry here." Reading a missing key yields `undefined`, and because absence is a routine question for a table-as-hashmap, absence never throws. Holding an `undefined` is fine; *using* it panics, so it is resolved by the operators below before use.
- **`null`, present-but-null.** `null` *is* a storable value, and with optional types it is common. A key holding `null` exists; someone put it there on purpose. `null` means "explicitly nothing," which is distinct from "no entry."
- **value, a real, present value.**

These three are the whole runtime model. An earlier design had a fourth, runtime *access*
axis — per-key `get` / `set` permissions on element keys, with denied reads raising
`TableReadViolationError`. That axis is **deleted** (R98): element space carries no
permissions (tables §6), and the encapsulation it expressed lives in protocol space as
**compile-time** grants (protocols §2.2). Denial is no longer a runtime state these
operators can meet; a forbidden access does not compile.

> Because `undefined` is unstorable, **existence ⟺ not-`undefined`**. A present key always holds `null` or a real value, never `undefined`. This equivalence is what lets `?.`, `??`, and existence checks compose without ambiguity.

---

## Read side

### Coalescers

Both operators are lazy: the right operand is evaluated only if the fallback triggers.

| `a` is… | `a ?? b` | `a ??? b` |
|-|-|-|
| `undefined` (absent) | `b` | `b` |
| `null` (stored) | `a` → `null` | `b` |
| value `v` | `a` → `v` | `a` → `v` |

- **`??`, absent-only.** Falls back solely on `undefined`. A stored `null` passes through unchanged. This is the safe default: it can never discard a meaningful `null` you intended to keep.
- **`???`, absent-or-null.** Falls back on both empties. The deliberately louder spelling for "treat null as nothing too." Being slightly awkward to type is intentional, the null-swallowing behavior should be opt-in and visible.

`??` cuts between `undefined` and `{null, value}`; `???` cuts between `{undefined, null}` and `value`. That is the entire difference between them — and it is a cut on the **value** axis, which is why both operators survive the permission model's deletion untouched.

### Optional chaining `?.`

`a?.b` guards against a receiver that cannot be indexed. Both `null` and `undefined` fail that test, and a broken chain **always yields `undefined`** (never `null`) so it feeds cleanly into `??`.

| `a` is… | `b` is… | `a?.b` |
|-|-|-|
| `undefined` |, (short-circuits) | `undefined` |
| `null` |, (short-circuits) | `undefined` |
| table | absent key | `undefined` |
| table | stored `null` | `null` |
| table | value `v` | `v` |

`?.` collapses "receiver was `null`" and "receiver was `undefined`" into one `undefined`, because the chain's only question is "could I reach `b`?", the answer is just "no."

### Composition, why they pair

Chain, then coalesce, across every input state:

| `a` | `a.b` | `a?.b` | `a?.b ?? d` | `a?.b ??? d` |
|-|-|-|-|-|
| `undefined` | *(broken)* | `undefined` | `d` | `d` |
| `null` | *(broken)* | `undefined` | `d` | `d` |
| table | absent key | `undefined` | `d` | `d` |
| table | stored `null` | `null` | **`null`** | `d` |
| table | value `v` | `v` | `v` | `v` |

The two coalescer columns diverge in exactly one row, **stored `null`**. `?? ` preserves the explicit `null`; `???` replaces it. Every *broken-chain* row produces `d` in both columns, because `?.` funnels all receiver failures to `undefined`, which both coalescers catch.

This funnel is the reason `?.` yields `undefined` and not `null` on a broken chain: if it returned `null`, the safe `??` would leak it through and only `???` would catch it, inverting the safety story.

---

## Protocol space: one more absence, same rules

Protocol space (`->`, protocols §3) contributes exactly one absence of its own: the
**unapplied protocol**. A bare read of a member whose protocol is not applied yields
`undefined` (protocols §3.2) and coalesces like any absence (`tab->nickname ?? 'anon'`);
a hard use — a write or a call — panics; and **`?->`** is the explicit soft form for
calls, `tab?->greet() ?? fallback`, short-circuiting argument evaluation exactly as `?.`
short-circuits the rest of a chain. Two absences, one rule.

Two facts keep this composable with everything above:

- **Members of an applied protocol are never absent.** Application binds every member
  (protocols §4), so per-member absence does not exist: absence in protocol space is
  all-or-nothing per protocol. `??=` therefore has no role on protocol members; there is
  no absent case for it to fill.
- **A member's *value* can still be `null`** (`let get set nickname?: string = null`,
  protocols §2.2), and that is the ordinary value axis: `tab->nickname ??? 'anon'`
  treats the stored null as nothing, exactly as it would on an element.

---

## Write side

Compound assignment inherits the coalescers' laziness and adds one distinction the read
side doesn't have: **firing on an absent key is an *add*; firing on a present `null` is
an *overwrite*.** Both are unconditionally legal — element space carries no permissions
and no seal (tables §5, §6; R98, R109) — so **both compound forms are total: they cannot
fail.**

- **`??=` asks only "does the key exist?"** — an existence check, never a value read.
  When it fires it is *always* an add.
- **`???=` asks "is the value null?"**, which reads the value to classify. It fires as an
  add (absent) or an overwrite (null).

### `a.k ??= b`, fires only when `k` is absent

| `k` state | fires? | reads value? | operation | outcome |
|-|-|-|-|-|
| absent | yes | no | add | assigns `b` |
| present = `null` | no | no |, | no-op, `null` kept |
| present = value | no | no |, | no-op |

`??=` decides by existence alone, and is **total** (R109): no state of the key or the
table can make it fail.

### `a.k ???= b`, fires when `k` is absent *or* null

| `k` state | reads value? | fires? | operation | outcome |
|-|-|-|-|-|
| absent | no (existence) | yes | add | assigns `b` |
| present = `null` | yes | yes | overwrite | assigns `b` |
| present = value | yes | no |, | no-op |

Like `??=`, `???=` is **total** (R109): reads cannot be denied (R98), overwrites are
unconditional (tables §6), and adds meet no seal (tables §5).

---

## Rulings

**Laziness.** For all four operators, the right operand is evaluated only when the
fallback/assignment fires. A non-firing `??=` / `???=` performs no write; nothing
happened. (The old form of this rule disclaimed open-, freeze-, and `noSet`-checks on
the non-firing path; all three mechanisms are gone — tables §5, R98, R109 — and the
*firing* paths are total now anyway.)

**No runtime "forbidden."** The coalescers distinguish *absent* from *empty*; there is no
third runtime state for them to meet. What used to be denial (`TableReadViolationError`
passing through `??`) is a **compile error** now (protocol grants, protocols §3.1), so
the question "does a permission throw pass through a coalescer?" no longer exists. Two
states, two behaviors:

| "no value for you" | `??` | `???` |
|-|-|-|
| absent (`undefined`) | coalesces | coalesces |
| present `null` | passes through | coalesces |

**Associativity & mixing.** Both coalescers share one precedence, left-associative, sitting just above the ternary. `a ?? b ?? c` → `(a ?? b) ?? c` is fine. When `??` and `???` appear at the same level, the parser **requires parentheses** rather than silently applying left-associativity: write `(a ?? b) ??? c`, never `a ?? b ??? c`.

**Chaining yields `undefined`.** A broken `?.` chain always resolves to `undefined`, never `null`, so it composes correctly with the safe `??`.
