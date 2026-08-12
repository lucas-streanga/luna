# String builder

Luna strings are immutable (string-representation, string §1), so accumulating a string
by repeated pairwise joining in a loop reallocates on every step and is O(n^2) in the
total length. The **string builder** is the mutable accumulator that solves this: you
append into it freely, then materialize an immutable `string` once. It is the single place
in the string story where mutation lives.

A builder is not a new primitive type. It is realized as a **table with the applied
`stringBuilder` protocol** (protocols): an element-empty table whose growable byte buffer
is an ungranted per-table protocol member (protocols §2.2, §2.3). This is the canonical
minimal use of the protocol model, so the builder doubles as its worked example. This
document is the authoritative builder spec; string §4 references it, and
interpolation (string §5) lowers to it.

---

## 1. What a builder is

A builder is a value of type `@stringBuilder` (protocols §6): a table with the
`stringBuilder` protocol applied.

- **Element-empty.** The builder holds no element data; its `.`/`[]` element space is
  empty (`[]`). All of its state is the buffer, which is a **protocol member**
  (protocols §2.3), not an element. So the builder is genuinely an empty table with an
  applied protocol, the object-from-table-plus-protocol pattern in its simplest form.
- **The buffer is private.** It is an ungranted member of `stringBuilder` — no `get`, no
  `set` — so it is namespaced to the protocol, invisible to element introspection
  (`count()` sees nothing) and to `==` (and the protocol declares `identityEquality`
  besides, protocols §5), and writable only by `stringBuilder`'s own functions
  (protocols §2.2).
- **Mutable, by write-back.** Appending follows the universal convention (tables §4,
  protocols §2.4): the receiver is passed by value, the updated builder is returned, and
  the caller writes back with `&` (`&b->append(x)`). Bind with `var` or `let`; `const`
  would deep-freeze the builder and forbid appending (variables §3).
- **An ordinary value table.** Copying a builder (`var b2 = b`) is a copy-on-write copy
  that yields an independent accumulator, the normal table value semantics. A builder is
  not a string, so the "no `&` on strings" rule does not apply to it; it may be passed
  by reference (§4).

---

## 2. Construction

```luna
const builder = fn (seed: string = "", capacityHint: nat = 0): @stringBuilder => {};
```

`builder()` returns a fresh empty builder. `seed` optionally starts the builder with an
initial string (equivalent to constructing empty and appending `seed` once, but done in
one step). `capacityHint` optionally pre-sizes the buffer, in **bytes** (R228 — the
unit `reserve` already speaks), to avoid early reallocations.
Construction is **non-errorable**: application is pure machinery and the operator form
never fails (protocols §4.1), so no `try` is needed.

```luna
var b    = builder();                 // empty builder
var doc  = builder("<<HEADER>>\n");   // seeded with an initial string
var big  = builder("", 4096);         // pre-sized for ~4 KB
```

`builder()` is the idiomatic form — an ordinary factory function (protocols §4.5). It is
equivalent to `var b: @stringBuilder = [] apply stringBuilder`, with the factory
additionally handling the seed and capacity hint. Bind with `var` or `let`, not `const`.

---

## 3. The builder surface

The builder's operations are `stringBuilder`'s protocol functions, reached with `->`
(protocols §3). The protocol:

```luna
const stringBuilder = proto {
  identityEquality;                    // builders compare by identity (protocols §5)

  var buf: bytes = bytes();            // ungranted: private, per-table (Go-backed)

  const get append          = fn (b: @stringBuilder, value: any): self => {};
  const get appendAll       = fn (b: @stringBuilder, items: iterable): self => {};
  const get appendCodepoint = fn (b: @stringBuilder, cp: int): self => {};
  const get appendUtf8Bytes = fn (b: @stringBuilder, raw: bytes): self! => {};
  const get reserve         = fn (b: @stringBuilder, numBytes: nat): self => {};
  const get byteLength      = fn (b: @stringBuilder): int => {};
  const get isEmpty         = fn (b: @stringBuilder): bool => {};
  const get clear           = fn (b: @stringBuilder): self => {};
  const get build           = fn (b: @stringBuilder): string => {};
};
```

| Function | Effect |
|-|-|
| `append(value)` | Append `value.toString()`'s bytes (§3.1). Returns `self`, chainable. |
| `appendAll(items)` | Append each element of an iterable, stringified via `toString`, in order (§3.2). Returns `self`. |
| `appendCodepoint(cp)` | Append one Unicode scalar value as UTF-8. Returns `self`. |
| `appendUtf8Bytes(raw)` | Validate `raw` is self-contained UTF-8 and append it; **throws** otherwise (§3.3). The one errorable builder operation. |
| `reserve(numBytes)` | Ensure capacity for at least `numBytes` total, in bytes, without reallocating (the parameter's former name `bytes` shadowed the predeclared type, R228). Returns `self`. |
| `byteLength()` | Bytes buffered so far. O(1). |
| `isEmpty()` | Whether nothing has been appended. O(1). |
| `clear()` | Reset to empty, keeping the allocated capacity. Returns `self`. |
| `build()` | Materialize: an immutable `string` of the current contents; builder unchanged (§5). |

Note the space split at work: `b->isEmpty()` asks the *protocol* whether the buffer is
empty; `b.isEmpty()` is UFCS to the iterable catalogue's `isEmpty` and asks whether the
*element space* is empty — which for a builder is always true. The two coexist without
ambiguity because `.` and `->` are different spaces (tables §3.3); the old rule
forbidding protocol members from taking catalogue names died with the built-in protocol
(R95).

(`take()`, the consume-and-reset variant of `build()`, is **retired**, R99: see §5.)

### 3.1 `append` coerces via `toString`

`append` takes `any` and appends the bytes of `value.toString()`, the same coercion
interpolation and `join` use (string-api §8, string §5; conversion §3 — there is no
concatenation operator, string §3). So `b->append(42)` appends `"42"`. To
append a raw scalar value by code point rather than its decimal text, use
`appendCodepoint`.

### 3.2 `appendAll` appends any iterable

`appendAll(items)` appends every element of an `iterable` (iterable-functions §1), each
stringified via `toString`, in order. It is the bulk form of `append`:
`appendAll([1, 2, 3])` appends `"123"`. Because a stream is an iterable, values produced
lazily (from a transform, a split, a generator) can be appended without first
materializing them into a collection; the stream is taken (iterable-functions §1.5).

### 3.3 `appendUtf8Bytes` validates, and is the one errorable operation

Every other append accepts inputs that are valid UTF-8 by construction (strings and their
`toString` forms, scalar codepoints), so the builder's buffer is **always** valid UTF-8 and
`build` never needs to check. `appendUtf8Bytes` is the opt-in escape hatch for callers
who already hold UTF-8 as raw bytes and want to append them without a `string` wrapper.

It preserves the always-valid invariant by validating at the call:

- The bytes must be **self-contained, complete valid UTF-8**. If they are valid and
  complete, they are appended. If they are invalid, or end mid-codepoint (a split
  sequence), `appendUtf8Bytes` **throws** at the call site, near the cause, and appends
  nothing.
- It is therefore the **only errorable** builder operation (`: self!`). Every other
  append, and `build`, remain non-errorable, because the buffer they see is always valid.

The cost (an O(slice) UTF-8 scan of the appended bytes) is paid only by callers who opt
into byte input, and only on the bytes they append. The validation is per-call and carries
no state between calls.

Because each call must be self-contained, `appendUtf8Bytes` deliberately does **not**
support streaming that splits codepoints across chunks. Reassembling split byte chunks into
valid text is a **decoder's** job (a separate API, to be defined), which yields complete
validated string pieces that are then appended normally. The builder stays a UTF-8-valid
sink; the decoder owns the pending-partial-codepoint bookkeeping.

---

## 4. Reaching and chaining

Builder operations are protocol functions, reached with `->` and chained on their `self`
returns; mutation is caller-side `&`-write-back (tables §4):

```luna
var b = builder();
&b->append("Hello, ")->append(name)->append("!");   // chain, then write back
let s = b->build();                                  // : string — no mutation, no &
```

To let a helper append into *your* builder, pass it by reference; the reference rules
require a `var` binding for `&`. The helper is an ordinary free function — an extension
in the UFCS sense (protocols §9) — and it is *functions like this*, not protocol
functions, that take `&` parameters (protocols §2.4):

```luna
const addHeader = fn (&b: @stringBuilder, title: string) => {
  &b->append("== ")->append(title)->append(" ==\n");
};

var doc = builder();
addHeader(&doc, "Intro");          // helper appends into doc; &doc requires var
```

Passing a builder **by value** instead copies it (copy-on-write), so a helper that takes
`b: @stringBuilder` (no `&`) appends into its own copy and leaves the caller's builder
untouched, the ordinary table value semantics.

---

## 5. `build`, and why `take` is gone

`build()` materializes the current contents as an immutable `string` and leaves the
builder unchanged and reusable: you may `build()`, append more, and `build()` again.
Calling `build()` to peek at the current contents never destroys the buffer. To consume
deliberately, say both halves:

```luna
let snapshot = b->build();          // b still usable; snapshot is independent
let final    = b->build();          // then, if done with b:
&b->clear();                        // reset explicitly (or just drop b)
```

An earlier surface paired `build()` (snapshot, O(n) copy) with `take()` (consume:
return the string *and* reset the builder, transferring the buffer zero-copy). `take` is
**retired** (R99), for two reasons that arrive at the same place:

- **It is inexpressible under the pure-receiver convention.** A protocol function
  returns one value and takes its receiver by value (protocols §2.4); "return the string
  and reset the receiver" needs a second channel that `&`-write-back (which rebinds the
  *table* return) cannot provide.
- **It is redundant under copy-on-write.** `build()` does not eagerly copy: the produced
  string *shares* the buffer's storage, and a copy happens only if the builder is
  appended to again afterward (the ordinary COW split, tables §4). So the zero-copy path
  `take` existed for is the default: build and drop the builder, and no copy ever
  happens.

---

## 6. Performance

- **Append is amortized O(1).** The buffer grows geometrically, so `n` appends totalling
  `m` bytes cost O(m), turning an O(n^2) concatenation loop into O(n). This is the reason
  the builder exists.
- **`byteLength()` and `isEmpty()` are O(1).**
- **`build()` is O(1) at the call** (the string shares the buffer, COW); the O(n) copy is
  deferred to the next append, and never happens if there isn't one (§5).
- **`reserve(n)` / `builder("", n)`** pre-size the buffer so a known-size build performs
  no reallocation at all.
- The produced string is an ordinary immutable string: inline in the `lval` if it fits in
  8 bytes, otherwise an owned buffer (string-representation §2).

---

## 7. Concurrency

A builder is **single-owner and not synchronized.** It is deliberately mutable, and mutable
shared state is exactly what the immutable-string design avoids elsewhere, so a builder is
not meant to be shared across concurrent tasks. Under Luna's green threads with enforced
copying, a builder is not shared across threads in the first place; each task builds its
own and the immutable results are combined (with `join` or interpolation). This keeps the
common path lock-free and contention-free. A concurrently-appended sink, if ever needed, is
a different type with its own contract, not a lock bolted onto the builder.

---

## 8. Interpolation lowers to a builder

String interpolation (string §5) is surface syntax that **lowers to builder calls**.
A double-quoted literal with interpolations, `"$greeting, $name!"`, becomes a builder that
appends the literal runs and the `toString` of each interpolated expression in order, then
materializes once:

```luna
_ = "$greeting, $name!";
// lowers to roughly:
//   var _b = builder();
//   &_b->append(greeting)->append(", ")->append(name)->append("!");
//   _b->build()
```

So interpolation and manual builder use share one mechanism, and interpolation carries no
hidden O(n^2) cost: an interpolated literal is one builder pass, not a chain of
reallocating pairwise joins. The temporary builder is dropped immediately after
`build()`, so the produced string keeps sole ownership of the bytes and the COW copy
never happens (§5) — the zero-copy path, with no `take` needed.

---

## 9. Relationship to the string API

- There is **no mutable string and no `&str`** (string-api §1); the builder is the mutable
  accumulator, and it is a distinct value (`@stringBuilder`), not a string.
- `build()` produces an ordinary immutable `string`, consumed through the string API like
  any other string.
- For joining a *known* collection of pieces, `join` (string-api §8) is the direct tool;
  the builder is for *incremental* accumulation where the pieces arrive one at a time.

---

## 10. Open questions

- **The decoder API:** streaming decode of split byte chunks (and transcoding from
  encodings other than UTF-8) is a separate decoder API, deferred. It produces validated
  string pieces that feed a builder; the builder itself only accepts self-contained UTF-8
  (§3.3).
- **`bytes` type:** `appendUtf8Bytes` takes a `bytes` value, and `buf`'s declaration
  presumes one; the precise `bytes` type (a raw byte sequence, distinct from `string`) is
  pending its own design.
