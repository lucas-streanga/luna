# String builder

Luna strings are immutable (string-representation, string-api), so accumulating a string
with the concatenation operator in a loop reallocates on every step and is O(n^2) in the
total length. The **string builder** is the mutable accumulator that solves this: you
append into it freely, then materialize an immutable `string` once. It is the single place
in the string story where mutation lives.

A builder is not a new primitive type. It is realized as a **table with the applied
`stringBuilder` protocol** (protocols, views): an element-empty table whose growable byte
buffer is a protocol-private meta member. This is the canonical minimal use of the
protocol model, so the builder doubles as its worked example. This document is the
authoritative builder spec; string-api §12 references it, and interpolation (string-api
§13) lowers to it.

---

## 1. What a builder is

A builder is a value of type `@stringBuilder` (protocols §7.1): a table with the
`stringBuilder` protocol applied.

- **Element-empty.** The builder holds no element data; its `.`/`[]` element space is
  empty (`[]`). All of its state is the buffer, which is a **meta member** (protocols
  §3.3), not an element. So the builder is genuinely an empty table with a applied protocol,
  the object-from-table-plus-protocol pattern in its simplest form.
- **The buffer is protocol-private.** It is a meta member of `stringBuilder`, namespaced
  to the protocol, invisible to element introspection (`count()` sees nothing), and
  writable only by `stringBuilder`'s own meta functions (protocols §3.3). It is not
  `noSet`, because the builder's own `append` must mutate it.
- **Mutable.** A builder is mutated in place by appending. It is bound with `var` or
  `let` (both permit the interior mutation that appending performs); `const` would
  deep-freeze it and forbid appending (variables).
- **An ordinary value table.** A builder is not `nocopy`: copying one (`var b2 = b`) is a
  copy-on-write copy that yields an independent accumulator, the normal table value
  semantics. A builder is not a string, so the "no `&` on strings" rule does not apply to
  it; it may be passed by reference (§4).

---

## 2. Construction

```
fn builder(seed: string = "", capacityHint: int = 0): @stringBuilder
```

`builder()` returns a fresh empty builder. `seed` optionally starts the builder with an
initial string (equivalent to constructing empty and appending `seed` once, but done in
one step). `capacityHint` optionally pre-sizes the buffer to avoid early reallocations.
Construction is **non-errorable**: it applies a non-throwing protocol to a fresh table
(protocols §7.5), so no `try` is needed.

```
var b    = builder();                 // empty builder
var doc  = builder("<<HEADER>>\n");   // seeded with an initial string
var big  = builder("", 4096);         // pre-sized for ~4 KB
```

`builder()` is the idiomatic form. It is equivalent to constructing an empty table and
statically applying the protocol, `var b: @stringBuilder = [] apply stringBuilder`, with
the factory additionally handling the capacity hint. Bind with `var` or `let`, not
`const`.

---

## 3. The builder surface

The builder's operations are the meta functions of `stringBuilder`, reached through `->`
(views). The protocol:

```
stringBuilder = proto {
  meta buf: <byte buffer>;        // protocol-private growable buffer (Go-backed)

  append          = meta fn (&b: table, value: any): self => { ... };
  appendAll       = meta fn (&b: table, items: stream | table): self => { ... };
  appendCodepoint = meta fn (&b: table, cp: int): self => { ... };
  appendUtf8Bytes = meta fn (&b: table, bytes: bytes): self !=> { ... };
  reserve         = meta fn (&b: table, bytes: int): self => { ... };
  byteLength      = meta fn (b: table): int => { ... };
  isEmpty         = meta fn (b: table): bool => { ... };
  clear           = meta fn (&b: table): self => { ... };
  build           = meta fn (b: table): string => { ... };
  take            = meta fn (&b: table): string => { ... };
};
```

| Meta function | Effect |
|-|-|
| `append(value)` | Append `value.toString()`'s bytes (§3.1). Returns `self`, chainable. |
| `appendAll(items)` | Append each element of a stream or table, stringified via `toString`, in order (§3.2). Returns `self`. |
| `appendCodepoint(cp)` | Append one Unicode scalar value as UTF-8. Returns `self`. |
| `appendUtf8Bytes(bytes)` | Validate `bytes` is self-contained UTF-8 and append it; **throws** otherwise (§3.3). The one errorable builder operation. |
| `reserve(bytes)` | Ensure capacity for at least `bytes` total without reallocating. Returns `self`. |
| `byteLength()` | Bytes buffered so far. O(1). |
| `isEmpty()` | Whether nothing has been appended. O(1). |
| `clear()` | Reset to empty, keeping the allocated capacity. Returns `self`. |
| `build()` | Snapshot: a new immutable `string` of the current contents; builder unchanged (§5). |
| `take()` | Consume: the built `string`, and reset the builder to empty (§5). |

### 3.1 `append` coerces via `toString`

`append` takes `any` and appends the bytes of `value.toString()`, the same coercion the
`.` concatenation operator uses (string-api §11). So `b->stringBuilder.append(42)` appends
`"42"`. To append a raw scalar value by code point rather than its decimal text, use
`appendCodepoint`.

### 3.2 `appendAll` appends a stream or table

`appendAll(items)` appends every element of a `stream` or `table`, each stringified via
`toString`, in order. It is the bulk form of `append`: `appendAll([1, 2, 3])` appends
`"123"`. Because a stream is accepted, values produced lazily (from a transform, a split,
a generator) can be appended without first materializing them into a collection.

### 3.3 `appendUtf8Bytes` validates, and is the one errorable operation

Every other append accepts inputs that are valid UTF-8 by construction (strings and their
`toString` forms, scalar codepoints), so the builder's buffer is **always** valid UTF-8 and
`build`/`take` never need to check. `appendUtf8Bytes` is the opt-in escape hatch for callers
who already hold UTF-8 as raw bytes and want to append them without a `string` wrapper.

It preserves the always-valid invariant by validating at the call:

- The `bytes` must be **self-contained, complete valid UTF-8**. If they are valid and
  complete, they are appended. If they are invalid, or end mid-codepoint (a split
  sequence), `appendUtf8Bytes` **throws** at the call site, near the cause, and appends
  nothing.
- It is therefore the **only errorable** builder operation (`: self!`). Every other append,
  and `build`/`take`, remain non-errorable, because the buffer they see is always valid.

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

Builder operations are meta functions, so they are reached with `->`, either inline or
through a bound view (views §2):

```
var b = builder();
b->stringBuilder.append("Hello, ").append(name).append("!");   // inline
let s = b->stringBuilder.build();

let sb = b->stringBuilder;         // bind the view once
sb.append("x").append("y");        // then call with `.`
```

The mutating meta functions (`append`, `appendCodepoint`, `reserve`, `clear`, `take`)
return **`self`**, the receiver viewed in `stringBuilder` (views §4.1), so they chain:
`sb.append(x).append(y).append(z)`. `build` returns a `string`, ending the chain.

**Mutation and binding.** Appending mutates the builder in place, which is interior
mutation, permitted on a `var` or `let` builder (variables). To let a helper append into
*your* builder, pass it by reference; the reference rules require a `var` binding for `&`:

```
const addHeader = fn (&b: @stringBuilder, title: string) => {
  b->stringBuilder.append("== ").append(title).append(" ==\n");
};

var doc = builder();
addHeader(&doc, "Intro");          // helper appends into doc; &doc requires var
```

Passing a builder **by value** instead copies it (copy-on-write), so a helper that takes
`b: @stringBuilder` (no `&`) appends into its own copy and leaves the caller's builder
untouched, the ordinary table value semantics.

---

## 5. `build` (snapshot) vs `take` (consume)

Because the produced string is immutable, materializing has two honest forms, and the
builder offers both:

- **`build()` is a snapshot.** It copies the current buffer into a new immutable `string`
  and leaves the builder **unchanged and reusable**. `build()` is pure with respect to the
  builder; you may `build()`, append more, and `build()` again. This is the default and the
  safe one: calling `build()` to peek at the current contents never destroys the buffer.
- **`take()` consumes.** It returns the built `string` and resets the builder to empty,
  which lets the runtime hand the buffer's bytes to the new string with no copy (the
  zero-copy path). Use `take()` when you are done with the builder and want to avoid the
  snapshot copy.

```
let snapshot = b->stringBuilder.build();   // b still usable, snapshot is independent
let final    = b->stringBuilder.take();    // b now empty; final owns the bytes
```

Whether `build()` copies and `take()` transfers is a representation detail
(string-representation §11); semantically, `build()` leaves the builder usable and `take()`
empties it, and both return an ordinary independent immutable string. `clear()` resets
without producing a string, for reuse when you do not need the current contents.

---

## 6. Performance

- **Append is amortized O(1).** The buffer grows geometrically, so `n` appends totalling
  `m` bytes cost O(m), turning an O(n^2) concatenation loop into O(n). This is the reason
  the builder exists.
- **`byteLength()` and `isEmpty()` are O(1).**
- **`build()` is O(n)** in the buffered length (one copy). **`take()` is O(1)** (transfer
  the buffer, reset).
- **`reserve(n)` / `builder(n)`** pre-size the buffer so a known-size build performs no
  reallocation at all.
- The produced string is an ordinary immutable string: inline in the `lval` if it fits in
  8 bytes, otherwise an owned buffer (string-representation §2).

---

## 7. Concurrency

A builder is **single-owner and not synchronized.** It is deliberately mutable, and mutable
shared state is exactly what the immutable-string design avoids elsewhere, so a builder is
not meant to be shared across concurrent tasks. Under Luna's green threads with enforced
copying, a builder is not shared across threads in the first place; each task builds its
own and the immutable results are combined (with `join` or concatenation). This keeps the
common path lock-free and contention-free. A concurrently-appended sink, if ever needed, is
a different type with its own contract, not a lock bolted onto the builder.

---

## 8. Interpolation lowers to a builder

String interpolation (string-api §13) is surface syntax that **lowers to builder calls**.
A double-quoted literal with interpolations, `"$greeting, $name!"`, becomes a builder that
appends the literal runs and the `toString` of each interpolated expression in order, then
materializes once:

```
"$greeting, $name!"
// lowers to roughly:
//   var _b = builder();
//   _b->stringBuilder.append(greeting).append(", ").append(name).append("!");
//   _b->stringBuilder.take()
```

So interpolation and manual builder use share one mechanism, and interpolation carries no
hidden O(n^2) cost: an interpolated literal is one builder pass, not a chain of
reallocating `.` concatenations. Because the temporary builder is not reused, interpolation
lowers to `take()` (consume) rather than `build()` (snapshot).

---

## 9. Relationship to the string API

- There is **no mutable string and no `&str`** (string-api §1); the builder is the mutable
  accumulator, and it is a distinct value (`@stringBuilder`), not a string.
- `build()` / `take()` produce an ordinary immutable `string`, consumed through the string
  API like any other string.
- For joining a *known* collection of pieces, `join` (string-api §8) is the direct tool;
  the builder is for *incremental* accumulation where the pieces arrive one at a time.

---

## 10. Open questions

- **The decoder API:** streaming decode of split byte chunks (and transcoding from
  encodings other than UTF-8) is a separate decoder API, deferred. It produces validated
  string pieces that feed a builder; the builder itself only accepts self-contained UTF-8
  (§3.3).
- **`bytes` type:** `appendUtf8Bytes` takes a `bytes` value; the precise `bytes` type
  (a raw byte sequence, distinct from `string`) is pending its own design.
