# Bytes

`bytes` is a **packed, mutable, growable buffer of bytes**. It is what `string` is not:
`string` is immutable and enforces valid UTF-8; `bytes` enforces no text validity and can be
modified and grown in place. Bytes are for binary work, file and network I/O, hashing, FFI
buffers, and parsing binary formats, where the data is a sequence of octets, not text.

The two properties that distinguish `bytes` from `string`:

- **No validity enforcement.** Any sequence of octets is a valid `bytes`. There is no UTF-8
  (or any) invariant on the contents.
- **Mutable and randomly modifiable.** Unlike `string` (immutable, no in-place edit), a
  `bytes` can be modified at any offset and appended to, because the offset of every byte is
  always known: the buffer is packed and contiguous, so random access and in-place update are
  trivial and O(1).

---

## 1. Representation: a packed buffer, not a table

`bytes` is its **own type** with a **packed representation**: a contiguous block of octets,
one byte per byte. It is deliberately **not** a `table`. A table stores boxed `lval`s (16
bytes each), so a table of a million bytes would cost 16MB; the entire point of `bytes` is a
compact buffer, so it stores raw octets, like `string`'s packed representation, not a table's
boxed elements.

So `bytes` is **table-like in interface** (integer-indexed, appendable, directly
`foreach`-iterable — R104) but **not a table in storage**, and deliberately **not an
`iterable`** (that alias stays exactly `table | stream`; the catalogue is reached through
`toStream`, §6). You get `b[i]` and `b[]` syntax over a tight byte buffer. To obtain an
actual table/list of the bytes (paying the boxing cost), use `toList()` (§4).

---

## 2. Elements are `byte` (int constrained to 0..255)

An element of `bytes` is a **`byte`**: an `int` constrained to `0..255` (constraints spec,
`byte = constraint i: int where i >= 0 && i <= 255`). There is no separate scalar "byte"
value distinct from int; a byte *is* an int in range, so all integer arithmetic and comparison
apply, and a `byte` widens to `int` implicitly.

- **Reading** `b[i]` yields a `byte` (usable as an `int`, widening is free).
- **Writing** `b[i] = x` requires `x` to satisfy the `byte` constraint. A write of an out-of-
  range value (`b[i] = 300`) fails the constraint and raises a `typeError` (panic), the ordinary
  constraint-on-entry check (constraints §7). So the one invariant `bytes` enforces is
  per-octet range (a byte is `0..255`), checked on write, not text validity.

Because elements are `byte` (an `int` refinement), all integer arithmetic and comparison
apply to elements directly; no distinct byte type is needed (§6).

---

## 3. Access, modify, append, and growth

Indexing follows the global `.`/`[]` rule (tables §3), but `bytes` has no static string keys,
so access is by integer offset with `[]`:

```luna
let b: bytes = [];        // empty buffer
b[] = 65;                 // append: b is now [65], length 1
b[0] = 66;                // in-bounds modify: b is now [66]
let x = b[0];             // read: x = 66 (a byte)
```

- **In-bounds modify** (`b[i] = x` for `i < len`): updates the octet at `i` in place. O(1).
- **Append** (`b[] = x`, or `b[len] = x`): grows the buffer by one octet. O(1) amortized.
- **Out-of-range write is an error.** `b[i] = x` for `i > len` is rejected (a `typeError`
  panic), **not** a silent gap-fill. `bytes` is a packed buffer with no representation for a
  hole, so writing past the end cannot leave a gap, and silently zero-filling an arbitrary index
  (`b[1000000] = 1` allocating a megabyte) is a footgun. To grow deliberately, use `fill` (§3.1).

Note this differs from **table** gap semantics (setting a far key creates a sparse gap, tables
§2.2): a table can be sparse; a packed `bytes` cannot, so it fills-or-rejects rather than
gapping. `bytes` rejects (except append-at-end); explicit growth is `fill`.

### 3.1 `fill`: deliberate growth

```luna
fn fill(b: bytes, length: int, value: byte = 0): bytes    // grow to `length`, new octets set to `value`
```

`fill` grows the buffer to `length`, setting any newly added octets to `value` (zero by
default). This is the **explicit** way to extend a buffer, replacing the rejected
"write-past-end silently zero-fills" footgun with a deliberate call. After `b.fill(10)`, indices
`0..9` exist and are writable in bounds.

---

## 4. Slicing and conversion

```luna
fn slice(b: bytes, start: int, end: int): bytes          // a sub-buffer b[start:end], end excluded
fn toList(b: bytes): list                                 // a list of int (0..255); COPIES and expands
fn cString(b: bytes): bytes                               // ensure a trailing NUL (0x00); for FFI
```

- **Slicing** `b[start:end]` (or `slice`) returns a **`bytes`** sub-buffer — **half-open**,
  end excluded, the corpus-wide slice convention (`:` slices half-open, `..` ranges
  inclusive, and they never mix; tables §2.5, range §2.1). Reading a single index `b[i]`
  returns a `byte` (an int); slicing returns a `bytes`. The single-vs-slice distinction is
  clear from the syntax, so it avoids the inconsistency some languages have.
- **`toList()`** produces a `list` of `int` (each `0..255`). This **copies** the packed buffer
  into boxed `lval`s (the table representation), so it is O(n) and expands the memory footprint;
  it is the explicit "I want the table/list machinery over these bytes" operation, not a free
  view. (The name is the R106 `to*` contract — a total, copying, value-to-value conversion is
  `to*`, never `as*`, which would falsely suggest a free re-typing. Not `values()` — that
  means something else on tables — and not `collect` — `bytes` is deliberately not
  `iterable`, R104. This is *the* `toList`, one name, one signature, functions §3.4.)
- **`cString()`** returns bytes with a trailing NUL appended if not already present, for passing
  to C FFI that expects NUL-terminated data. Total (always succeeds).

---

## 5. Text conversion is fallible, and lives in the string API

Converting `bytes` to `string` is **fallible**, because arbitrary octets are not necessarily
valid UTF-8. So it is a **function returning `string!`**, not a total `toString` and not an
`as` narrowing (`as` never runs conversions, `as` spec §3):

```luna
fn fromBytes(b: bytes): string!        // string.fromBytes: validates UTF-8, throws if invalid
```

`string.fromBytes(b)` validates the octets as UTF-8 and returns a `string`, or throws a
declarable error (handled with `try`) if they are not valid UTF-8. The name and the `!` make the
fallibility visible, unlike a `b.toString()` that would falsely imply a total operation.

The reverse direction, a `string`'s raw octets, is the string spec's `toBytes()`
conversion (string-api §9, R102 — the packed half of its producer/conversion split). So
`str.toBytes()` gives a packed `bytes`, and `string.fromBytes(b)` validates octets back
into a `string`; the two are inverses across the fallible UTF-8 boundary.

---

## 6. Why elements are `byte`, not a distinct scalar type

Every mainstream language with a byte buffer (Go `[]byte`, Rust `Vec<u8>`, Python `bytes`, JS
`Uint8Array`) makes an element an **integer 0..255**, not a distinct byte scalar, because a
separate byte type would need its own arithmetic, literals, and conversions for no benefit.
Luna follows this: a `byte` is an `int` constrained to `0..255` (§2), so it reuses all integer
operations and widens to `int` freely. `foreach (x in b)` iterates ordinary ints (R104),
and comparing an element against the `int` literal `0` works because `==`/`!=` erase
constraints (`byte` and `0` share the base `int`, equality spec §1). Whole-buffer equality
is **content equality** — length then bytes, a `memcmp`, with the shared-storage fast path
(equality §5, R181) — the element rule bulked. Bulk transforms are
**not** direct — `bytes` is not an `iterable` (R104): a transform's output cannot promise
to stay `0..255`, so a packed result kind would be unsound in general. The spelling is the
explicit bridge, `b.toStream().filter(fn (x) => x != 0)`, yielding ints; repack with
`toBytes` (string-api §9, R107), whose iterable arm is a bulk append — each element
checked against the `byte` constraint, panicking on a non-byte exactly as `b[] = x`
would (§2). No new scalar type.

---

## 7. Byte literals: `b"..."`

A `bytes` value can be written literally with the **`b` prefix** on a quote, `b"..."` or
`b'...'` (the quote style does not matter, since a bytes literal does not interpolate):

```luna
let sig    = b"\x89PNG\r\n";      // the PNG signature: raw octets via \xNN plus text
let method = b"GET ";              // ASCII text as bytes
let empty  = b"";                  // the empty bytes
```

- **The value is the UTF-8 octets of the enclosed text**, as a **compile-time constant**
  `bytes`. `b"GET "` is the four octets `71 69 84 32`, known at parse time.
- **A bytes literal may not span lines** (R244, lexer §4): a raw newline ends it and raises
  `L0009`. Write `\n` — or `\x0a`, the two being the same octet here.
- **Escapes** are the shared string escapes **plus `\xNN`**, a raw hex octet — the
  authoritative table is string §5.1 (R150). **Exactly two hex digits**, checked by the
  lexer: `\x`, `\x8`, and `\xZZ` are `L0016` (R248). In a *string* the same `\x` is `L0005`
  instead, the escape being absent from that row rather than misspelled here. `\xNN` is **bytes-only by ruling**: a raw
  byte in a *string* could break the UTF-8 validity guaranteed at ingress, but bytes have
  no validity to break — so `b"..."` covers both "text as bytes" and "arbitrary raw
  octets" in one form (no `\$` — no interpolation — and no `\u{}`; a codepoint's octets
  are spelled as the bytes they are). A separate hex-string literal is not needed.
- **No interpolation.** Unlike a `string` literal, `b"..."` does **not** interpolate. It is a
  fixed, compile-time sequence of octets, not a runtime-built value. Interpolation is a
  string-building operation (string §5); building bytes from runtime values is a runtime
  operation (append to a `bytes`, or build a `string` and take its `bytes()`), not a literal.
  Because it never interpolates, the quote style (`"` or `'`) carries no difference.

For a `bytes` built from a list of byte values, an ordinary list literal narrowed with `as`
works without special syntax: `[0x89, 0x50, 0x4e, 0x47] as bytes` (once `0x` int literals
exist). The `b"..."` form is the convenience for text-and-octet constants.

Single-byte and integer literal notations (`0x41`, `0b0100_0001`) are **int** literals, not
bytes-specific; a single byte is an int literal used in a byte context (int spec, deferred).

---

## 8. Mutability

`bytes` is **mutable and growable**; there is no separate immutable bytes type. The common
reason to reach for `bytes` (FFI buffers, building up binary output) wants a writable buffer,
so mutability is the default and only form. The rare need for immutable octets is served by the
general freeze machinery (a frozen `bytes`, via the same mechanism that freezes tables) rather
than a distinct type.

Like other single-owner mutable values (the string builder), a `bytes` is not meant to be
shared across concurrent tasks; under green threads with enforced copying, each task holds its
own.

---

## 9. Open questions

- *(**Typed multi-byte reads: resolved by R187** — the family exists in **std.binary**
  (`readI16`/`readU16`/`readI32`/`readU32`/`readI64`/`readU64`, each
  `(b: bytes, offset: int, endianness: endian)`, precise return types, bounds-panic per
  this spec's own indexing class). It is a std module, not bytes surface, because
  endianness is an *encoding* concern, not a property of the buffer — the std.math
  pure-domain precedent. The packed representation supports it exactly as this bullet
  predicted.)*
- **Views over `bytes`:** whether a non-copying view (a sub-range that shares storage, like a
  Rust slice) exists alongside the copying `slice`, pending the ownership model.
- **Growth strategy:** the amortized-growth policy for append (capacity doubling), an
  implementation note affecting the O(1)-amortized claim.
