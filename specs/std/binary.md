# `std.binary`

```luna
import std.binary;                          // or: import { endian, readI32, ... } from std.binary;
```

The binary-encoding module: endian-explicit fixed-width integer reads over `bytes`, and the
`endian` type they share. **Pure throughout, no capability** — these are memory reads over a
value — and therefore **comptime-eligible** (functions §5.5): parsing embedded binary data at
build time falls out for free, the std.math property.

The placement is ruled (R187). The `bytes` *type* stays built-in with its universal catalogue
(indexing, `slice`, `toList` — bytes spec); this module holds the **domain** functions, because
endianness is an *encoding* concern, not a property of the buffer, and six rare functions are a
domain library, not core surface — the std.math precedent (pure functions over built-in types,
importing pulls only what is used, R141). The module name is the backend's own: Go's
`encoding/binary` is precisely this surface, the same mirror that placed std.exec against
`os/exec` (R172).

## 1. The `endian` type

```luna
export const endian = enum { little, big };
```

A named, exported enum (R187) — deliberately its **own small type** rather than an anonymous
inline enum, so other libraries and user functions can declare `endian`-typed parameters and
reuse the vocabulary. Call sites are unchanged either way: a fenced literal resolves against
the parameter's expected type (enum §3.3), so callers write `{little}` / `{big}`.

**No function in this module defaults its endianness.** A default would bake a silent
portability bias — network formats are big-endian, most file formats little-endian — so the
byte order is named at every call, the cost-visible stance (and Go's choice too). The rule
turned out to carry a second leg (R193): because every read names its order, **the host's
byte order is unobservable**, which is one of the channels that make comptime folding
host-independent (compiler §6.3) — `readI32(b, off, {little})` folds to the same value on
any machine.

## 2. The read family

```luna
export const readI16 = fn (b: bytes, offset: int, endianness: endian): i16 => {};
export const readU16 = fn (b: bytes, offset: int, endianness: endian): u16 => {};
export const readI32 = fn (b: bytes, offset: int, endianness: endian): i32 => {};
export const readU32 = fn (b: bytes, offset: int, endianness: endian): u32 => {};
export const readI64 = fn (b: bytes, offset: int, endianness: endian): int => {};
export const readU64 = fn (b: bytes, offset: int, endianness: endian): uint => {};
```

- **The width lives in the name, not a parameter.** A size enum
  (`size: enum { b16, b32, b64 }`) was considered and rejected (R187): it couples the return
  type to an argument *value* the type system cannot see, forcing a union return
  (`i16 | i32 | int`) and a narrowing at every call site. Different return types are
  different contracts, and different contracts get different names — the policy-verb and
  R157 two-names discipline. One name, one signature (functions §3.4).
- **Reads are exact-width and position-explicit**: `readI32(b, off, {little})` reads the four
  octets `b[off..off+3]` (inclusive) as a little-endian two's-complement integer. Signed reads
  sign-extend into the return type; unsigned reads zero-extend. `readU64` is the one member
  whose values need `uint` (its top range does not fit `int`; the function keeps the wire
  width in its name per this section's rule — it reads 64 bits — while the type it returns
  is `uint`, R226).
- **Out of range panics.** `offset < 0`, or `offset + width > byteLength(b)`, is the
  indexing-misuse class (`b[i]` out of bounds, bytes §3) and panics identically. The caller's
  guard is the O(1) `byteLength` check; a truncated-input concern is handled *before* the
  read, visibly.
- **Unsigned is half the family on purpose**: lengths, magic numbers, and checksums read
  unsigned at least as often as signed — a signed-only surface was the reviewed sketch's gap.

### 2.1 Scheduling: the small typeids are pulled forward (R187)

Four return types are small-width constraints (`i16`, `u16`, `i32`, `u32`) and one is the
`uint` primitive (`u64` until R226) — all previously scheduled post-alpha with the extended tower. Shipping this
family in alpha **pulls those five typeids forward**, and this is a scheduling choice, not new
design: numeric-tower §6's own words make the smalls "new typeids under existing rules" (the
constraint-subtype mechanism `byte` already exercises), and `uint` is the one new primitive.
The alternative — alpha signatures returning widened `int` and tightening later — was
rejected: changed signatures are a compatibility wart, and `readU64` must not ship broken.
Recorded in numeric-tower §6.

## 3. Deferred, deliberately

- **The write family** (`writeI16(b, offset, v, endianness)` and siblings): the symmetric
  half, real and expected, awaiting its own pass — writes touch the mutation surface
  (in-place vs append, bytes §3) and deserve their own ruling.
- **`readI8`**: `b[i]` already yields the unsigned octet; a signed-8 *reinterpretation*
  (255 → −1) cannot be `as` (it changes the value, as spec §3) and is rare — waits for need.
- **Varints, floats-from-bytes, checksums**: the domain module is the natural home when a
  need arrives; none is committed.
- **A native-endian read or write: never** (R193, a permanent fence, not a deferral). An
  "int to host-order bytes" function would make the host's byte order observable and reopen
  the comptime-portability channel this module's explicit-`endian` rule closes (compiler
  §6.3). Code that wants "the target's natural order" states which one that is.
