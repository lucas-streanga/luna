# Optional Access & Coalescing

Semantics for `?.`, `??`, `???`, and their compound-assignment forms in Luna.

## The model: three states of "no value," plus a fourth axis

Luna keeps `null` and `undefined` genuinely distinct, and this distinction is what the whole operator set is built to navigate:

- **`undefined`, absent.** `undefined` can *never* be stored as a table value. It appears only in variable and return slots, and it is **language-produced, never programmer-written** (undefined spec): the two productions are a missing key and a void return. It is therefore the one unambiguous sentinel for "there is no entry here." Reading a missing key yields `undefined`, and because absence is a routine question for a table-as-hashmap, absence never throws. Holding an `undefined` is fine; *using* it panics, so it is resolved by the operators below before use.
- **`null`, present-but-null.** `null` *is* a storable value, and with optional types it is common. A key holding `null` exists; someone put it there on purpose. `null` means "explicitly nothing," which is distinct from "no entry."
- **value, a real, present value.**

These three are the *value* axis. There is a second, independent **access** axis: a key the wearing protocol does not grant `get` cannot be read, so reading it raises `TableReadViolationError`, it does **not** collapse into `undefined`. Absence and denial are different failures and stay different everywhere below.

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

`??` cuts between `undefined` and `{null, value}`; `???` cuts between `{undefined, null}` and `value`. That is the entire difference between them.

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

**`?.` does not suppress permission errors.** It guards the receiver's *existence*, not access rights. `tab?.noGetKey`, where the key exists but is `noGet`, still raises `TableReadViolationError`. `?.` swallows null/undefined receivers only, never `TableReadViolationError` / `TableMutationViolationError`.

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

## Write side

Compound assignment inherits the coalescers' laziness and adds one distinction the read side doesn't have: **firing on an absent key is an *add* (governed by open/close); firing on a present `null` is an *overwrite* (governed by the key's `set` grant).**

One principle drives both forms:

- **`??=` asks only "does the key exist?"**, an existence check, never a value read. It needs neither `get` nor `set` to *decide*, and when it fires it is *always* an add. Its only possible failure is `OpenViolationError`.
- **`???=` asks "is the value null?"**, which requires *reading* the value, so it needs `get` to decide. It can fire as an add (absent → open-state) or an overwrite (null → the key's `set` grant).

### `a.k ??= b`, fires only when `k` is absent

| `k` state | fires? | reads value? | operation | outcome |
|-|-|-|-|-|
| absent, table open | yes | no | add | assigns `b` |
| absent, closed / neverOpen | yes (attempts) | no | add | **`OpenViolationError`** |
| present = `null` | no | no |, | no-op, `null` kept |
| present = value | no | no |, | no-op |
| present, no `get` grant | no | no |, | no-op, **no throw** |
| present, no `set` grant | no | no |, | no-op |

`??=` decides by existence alone: a key without `get` never throws (it doesn't read) and a key without `set` never throws (it doesn't write). The operator can raise **only** `OpenViolationError`, and only when growing a `closed` / `neverOpen` table.

### `a.k ???= b`, fires when `k` is absent *or* null

| `k` state | reads value? | fires? | operation | outcome |
|-|-|-|-|-|
| absent, table open | no (existence) | yes | add | assigns `b` |
| absent, closed / neverOpen | no | yes (attempts) | add | **`OpenViolationError`** |
| present = `null`, has `set` | yes | yes | overwrite | assigns `b` |
| present = `null`, no `set` grant | yes | yes (attempts) | overwrite | **`TableMutationViolationError`** |
| present = `null`, no `get` grant | can't |, |, | **`TableReadViolationError`** |
| present = value, has `set` | yes | no |, | no-op |
| present = value, no `get` grant | can't |, |, | **`TableReadViolationError`** |
| present = value, no `set` grant | yes | no |, | no-op |

Two subtleties:

- **A missing `get` grant throws even in the value case.** `???=` must read to classify null-vs-value; a present key without `get` raises `TableReadViolationError` even where it ultimately wouldn't have written, because it cannot know that without reading.
- **A missing `set` grant bites only on `null`.** A no-`set` key holding a real value is a clean no-op, reading is allowed, and no write is attempted.

### Sealed tables and the caller's responsibility

Adding to a `closed` or `neverOpen` table throws `OpenViolationError`; it is the caller's job to `open()` first. The irrevocable seal (`neverOpen`) can never be lifted, so the corresponding add throws *permanently*, the point of the seal, not a gap. Overwriting a present value is governed by the key's protocol `set` grant (a missing grant raises `TableMutationViolationError`); the growth and access concerns are independent, an `??=` add succeeds regardless of `set` grants, and a `???=` overwrite succeeds on a `closed`-but-settable key.

---

## Rulings

**Laziness.** For all four operators, the right operand is evaluated only when the fallback/assignment fires. A non-firing `??=` / `???=` performs no write, and therefore triggers **no** open-check, **no** `freeze` check, and **no** `noSet` check, nothing happened.

**Permission throws *through* coalescers.** `??` / `???` distinguish *absent* from *empty*; they do **not** distinguish *forbidden*. `tab.missing ?? d` is exception-free (absence → `undefined` → caught). But `tab.noGetKey ?? d` **raises** `TableReadViolationError`, a forbidden read is a program error, not an absent value, and must not be silently swallowed into `d`.

**Three states, three behaviors, no overlap:**

| "no value for you" | `??` | `???` | throws? |
|-|-|-|-|
| absent (`undefined`) | coalesces | coalesces | no |
| present `null` | passes through | coalesces | no |
| present, `noGet` |, |, | **yes** (`TableReadViolationError`) |

**Associativity & mixing.** Both coalescers share one precedence, left-associative, sitting just above the ternary. `a ?? b ?? c` → `(a ?? b) ?? c` is fine. When `??` and `???` appear at the same level, the parser **requires parentheses** rather than silently applying left-associativity: write `(a ?? b) ??? c`, never `a ?? b ??? c`.

**Chaining yields `undefined`.** A broken `?.` chain always resolves to `undefined`, never `null`, so it composes correctly with the safe `??`.
