# String API

The `string` function catalogue — the public surface. The type itself (immutability
and binding, the units doctrine, the no-concat rule, interpolation and the escape
table) is **string** (its own document, R227); the internal layout (inline vs.
descriptor, borrowed slices, UTF-8 validity) is in string-representation and is not
visible here except through performance notes.

The API is implemented in Go and exposed virtually, but to the programmer every entry
below behaves like an ordinary function reachable through UFCS: `str.trim()` and
`trim(str)` are the same call.

---

## 1. Design rules that shape every signature

These follow from the language and from string immutability, and they are assumed by
every entry in the catalogue rather than restated per function.

**Strings are immutable, so every operation is pure** (string §1): no method mutates
its receiver; each returns a new string (or a view, number, bool, or table). Binding
modes are likewise the type's (string §1).

**No overloading; unions instead.** Where another language would overload, Luna takes
a union parameter. `split` accepts `string | int` for its separator; `replace` accepts
`string` targets; a function never exists in two arity/type variants under one name.

**UFCS everywhere.** Every function takes the string as its first parameter, so it
reads equally as `str.f(...)` or `f(str, ...)`. The catalogue is written receiver-first
for that reason.

**Naming scheme.** Predicates are `isX` / `startsWith` / `contains` and return `bool`.
Producers are verbs (`trim`, `replace`, `padStart`). Views are plural nouns
(`bytes`, `codepoints`, `graphemes`, `lines`). Counts are `xLength` / `xCount`; there
is no bare `length` (see §2). Case-insensitive variants take an explicit option rather
than a separate name (see §3).

**Return shapes.** A function that yields many elements returns a **stream** —
**producers produce streams** (R102), the convention shared with generators, ranges, and
`io`; retention is an explicit `collect()` (iterable-functions §2.11), and a
string-derived stream is **restartable** for free (immutable source, stream §4). A
function that yields one string returns `string`. Indices are byte offsets and are
always `int` (see §2).

---

## 2. Length, indexing, and units

The units doctrine — no `length`, no integer indexing, three explicit units, because
silently picking one meaning is the most common source of Unicode bugs — is the
type's (string §2). The counts:

- **`byteLength(str): int`** the number of UTF-8 bytes. O(1).
- **`codepointCount(str): int`** the number of Unicode scalar values. O(n).
- **`graphemeCount(str): int`** the number of grapheme clusters, i.e. what a person
  counts as characters. O(n).

Positions returned by search functions (`indexOf`, etc.) are **byte offsets**, and
functions that take a position (`slice`, `sliceBytes`) take byte offsets too. Byte
offsets are the only unit that is O(1) to act on; codepoint and grapheme positions
would each be O(n) to resolve, so the API does not pretend they are cheap. Every byte
offset returned by the API lands on a codepoint boundary, and slicing validates that
its arguments do, so you cannot split a multi-byte codepoint by accident (it is an
error, see §6).

---

## 3. Common options

One option recurs and is always spelled the same way:

- **`caseInsensitive: bool = false`** on comparison, search, and replace functions.
  Case folding uses full Unicode case folding, not ASCII-only.

(The former second common option, `asStream: bool = false` on element producers, is
retired with the whole flag model, R92/R102: producers now always return streams, §1.)

---

## 4. Construction and conversion

#### toString()
```
fn toString(value: any): string
```
Every value's canonical string form. Defined for all types; for `table` it is the
same rendering used by `println`. The identity function on `string`.

#### interpolation
Interpolation is surface syntax, not a function: `"$name is $age"` and `"${expr}"`
splice values into a double-quoted literal using their `toString`. Its rules, and its
relationship to the string builder, are in string §5. (A positional `format` function is
deliberately omitted for now; it is a larger design in its own right.)

#### repeat()
```
fn repeat(str: string, times: int): string
```
`"ab".repeat(3)` is `"ababab"`. `times <= 0` yields `""`.

#### chr() / parse helpers
```
fn chr(codepoint: int): string             // one-codepoint string from a scalar value
fn parseInt(str: string): int!             // parse; error if not a valid integer
fn parseDouble(str: string): double!       // parse; error if not a valid double
```
`chr` is the inverse of taking a single `codepoints()` element. The parse helpers
return an errorable type (postfix `!`, the value-representation error model) and are
named for their **target** (`parse*` acquires from bare text; `to*` is reserved for
total conversions — the three-prefix contract, conversion §2, R106); `str.parseInt()`
must be consumed with `try` or an errorable binding.

---

## 5. Inspection and search

#### isEmpty()
```
fn isEmpty(str: string): bool
```
True iff `byteLength == 0`. O(1). (`str == ""` also works and is equally O(1).)

#### contains() · startsWith() · endsWith()
```
fn contains(str: string, needle: string, caseInsensitive: bool = false): bool
fn startsWith(str: string, prefix: string, caseInsensitive: bool = false): bool
fn endsWith(str: string, suffix: string, caseInsensitive: bool = false): bool
```
O(n).

#### indexOf() · lastIndexOf()
```
fn indexOf(str: string, needle: string, from: int = 0, caseInsensitive: bool = false): int
fn lastIndexOf(str: string, needle: string, caseInsensitive: bool = false): int
```
The **byte offset** of the first / last match, or `-1` if absent. `from` is a byte
offset. O(n).

#### count()
```
fn count(str: string, needle: string, caseInsensitive: bool = false): int
```
Number of non-overlapping occurrences of `needle`. O(n).

#### matches() · find() · findAll() (regex)
```
fn matches(str: string, pattern: regex): bool
fn find(str: string, pattern: regex): table | null
fn findAll(str: string, pattern: regex): stream
```
`regex` is its own type (a compiled pattern, kept separate so a pattern is compiled
once and reused, rather than recompiled from a string on every call). `matches` tests
for any match; `find` returns the first match as a table of capture groups (or `null`
if none); `findAll` yields one such table per match. A `replace` variant that takes a
`regex` target rides on the union already in §7. The `regex` type, its `/.../ ` literal,
flags, engine guarantees (linear-time by default, opt-in backtracking under `/b`), and
runtime construction are specified in **regex** (its own document); this section only
fixes how strings consume one.

**The match-table shape is ruled** (R217), and it is one table with **both key spaces** —
PHP's `preg_match` shape, proven by two decades of the lineage Luna's tables descend
from: **int keys** are the positional groups (`0` the whole match, `1..n` the groups in
order), and a **named** group (`(?<name>…)`, regex §5.4) appears under **both** its
number and its name:

```
let m = text.find(/(?<year>\d{4})-(?<month>\d{2})/);
// m: [0 => "2026-07", 1 => "2026", 2 => "07", 'year' => "2026", 'month' => "07"]
let ['year' => y] = m;          // keyed destructuring composes for free
```

The duplication is deliberate (positional iteration and named access coexist; one table,
no accessor functions). **Positions are not in the table** — offsets are a different
question and a position-returning variant is deferred until need.

#### regexEscape()
```
fn regexEscape(str: string): string
```
Escapes all regex metacharacters in a string so it can be matched as literal text.
**This is the one function, ruled** (R217): a builtin free function, one name, one
signature (functions §3.4), reached as `regexEscape(s)` or `s.regexEscape()` under UFCS —
pure and comptime-eligible, which is what lets it run inside a regex literal's
interpolation while preserving compile-time compilation (regex §7). (An earlier draft
spelled it `regex.escape` in "the regex module" with this entry as a "delegating alias" —
the module-qualification fossil, R188's class: `regex` is a built-in *type*, no module
exists, and there was never anything to delegate.)

---

## 6. Slicing and extraction

#### slice()
```
fn slice(str: string, offset: int, length: int = 0): string
```
A substring beginning at byte `offset` for `length` bytes (`length <= 0` means "to the
end"). O(1): the result borrows the parent's buffer (string-representation §7), so a
small slice pins its parent until `copy`d. Both `offset` and `offset + length` must
land on codepoint boundaries, or it raises `stringBoundaryError` (§10).

#### before() · after() · between()
```
fn before(str: string, sep: string): string     // up to the first sep, or all of str
fn after(str: string, sep: string): string       // after the first sep, or ""
fn between(str: string, open: string, close: string): string
```
Convenience extractors over `indexOf` + `slice`. All borrow.

#### trim() · trimStart() · trimEnd()
```
fn trim(str: string, cutset: string = whitespace): string
fn trimStart(str: string, cutset: string = whitespace): string
fn trimEnd(str: string, cutset: string = whitespace): string
```
Remove leading/trailing characters in `cutset` (default: Unicode whitespace). The
result borrows.

#### padStart() · padEnd()
```
fn padStart(str: string, width: int, fill: string = " "): string
fn padEnd(str: string, width: int, fill: string = " "): string
```
Pad to `width` **grapheme** clusters (the visually meaningful unit for alignment) with
`fill`. No-op if already at least `width` wide.

---

## 7. Transformation

#### toUpper() · toLower() · toTitle() · fold()
```
fn toUpper(str: string): string
fn toLower(str: string): string
fn toTitle(str: string): string
fn fold(str: string): string        // case-folded form, for case-insensitive keys
```
Full Unicode case mapping, not ASCII-only. `fold` produces the canonical
case-insensitive form (what `caseInsensitive: true` compares under), useful as a map
key.

#### replace() · replaceFirst()
```
fn replace(str: string, target: string, with: string, caseInsensitive: bool = false): string
fn replaceFirst(str: string, target: string, with: string, caseInsensitive: bool = false): string
```
Replace all / the first occurrence. Returns a new string (immutability).

#### reverse()
```
fn reverse(str: string): string
```
Reverses by **grapheme cluster**, so combining marks and emoji sequences stay intact.
O(n).

#### normalize()
```
fn normalize(str: string, form: enum {nfc, nfd, nfkc, nfkd} = {nfc}): string
```
Unicode normalization. Needed so that visually identical strings built differently
(precomposed vs. combining) compare and hash equal after normalizing.

---

## 8. Splitting and joining

#### split()
```
fn split(str: string, sep: string | int, limit: int = 0): stream
```
Split on a `string` separator, or into fixed-width chunks of `int` bytes (the union
replaces an overload). `limit > 0` caps the number of pieces, the last holding the
remainder. `""` separator is an error; use `graphemes()` for per-character. Pieces
borrow.

#### lines()
```
fn lines(str: string): stream
```
Split on line boundaries (`\n`, `\r\n`, and Unicode line breaks), terminators removed.
Pieces borrow.

#### join()
```
fn join(it: iterable, glue: string = '', finalGlue?: string = null): string
```
`join` is the catalogue function (iterable-functions §2.10), listed here for
discoverability: it concatenates the elements of any iterable (each coerced via
`toString`) separated by `glue`, with `finalGlue` as a distinct last separator
(`"a, b and c"`). Receiver-first like the whole catalogue — `parts.join(", ")` — which
settles the receiver-position wander functions §3.4 flagged. For a known collection it
is the direct tool; for incremental accumulation, the builder (string §4; a pairwise
concatenation loop is O(n^2), string-representation §10).

---

## 9. UTF-8 views

Each returns a **stream** of borrowed slices (or inline strings for ASCII), one element
per unit — `collect()` retains a list (§1). None allocate per element for ASCII input.

#### bytes() · toBytes()
```
fn bytes(str: string): stream              // the producer: yields each byte (int 0..255)
fn toBytes(src: string | iterable): bytes  // the conversion: the packed bytes value
```
Two operations, split along the producer/conversion seam (R102). `bytes()` is the
**iteration view**, a stream of `byte` (int `0..255`). `toBytes()` is the **conversion**
(the `to*` family, R94) to a packed **`bytes`** value (bytes spec), the natural
representation for I/O, hashing, and low-level work, rather than a boxed list of
integers. Its union arm (R107) repacks an **iterable of byte-valued ints** — a
transformed byte stream, a list of octets — as a bulk append, each element carrying the
ordinary `byte` constraint check: a non-byte value **panics** (`typeError`, bytes §2),
exactly as the `b[] = x` writes it abbreviates would, so the signature stays `!`-free
(panics are signature-exempt, functions §4); a stream argument is taken
(iterable-functions §1.5). On a `string`, `toBytes` is the inverse of
`string.fromBytes(b): string!` (bytes spec §5), which validates bytes back into a
string.

#### codepoints()
```
fn codepoints(str: string): stream   // elements are string
```
One element per Unicode scalar value. The safe technical unit; no Unicode-version
dependence.

#### graphemes() / characters()
```
fn graphemes(str: string): stream    // elements are string
fn characters(str: string): stream   // alias of graphemes
```
One element per grapheme cluster, what a person points at as a character. This is the
correct default for user-facing iteration; `characters` is the friendly alias and
never means codepoints.

#### isValidUtf8()
```
fn isValidUtf8(str: string): bool
```
Always `true` for a live `string` (validity is an invariant, string-representation §8).
Present for the boundary case of bytes about to become a string; the validating
conversion itself is `string.fromBytes(b): string!` (bytes spec §5).

---

## 10. C-string interop

Luna strings are counted, never null-terminated, and Luna **never** appends a
terminator on its own. A NUL is an ordinary byte: it may appear anywhere in a string
and is counted in `byteLength` like any other. C interop is therefore explicit, and
the two functions below exist precisely because Luna does not do what C assumes.

#### cString()
```
fn cString(str: string): string
```
Returns a string with a single NUL byte appended. It is defined to be exactly
`"$str\0"` (one interpolation, string §5), nothing more: no scanning,
no de-duplication of existing NULs, just an appended terminator so the result is safe
to hand to C. Because the terminator is a real, counted byte, the result's
`byteLength` is `str.byteLength + 1`.

#### cStringLength()
```
fn cStringLength(str: string): int
```
The number of bytes up to and including the **first** NUL, which is the length C will
actually see. This is an O(n) scan, not a field read, because a Luna string's
`byteLength` counts every byte (including any terminator and any interior NULs),
whereas C stops at the first NUL. The two disagree whenever the string has an interior
NUL:

```
let s = "ab\0cd";
s.byteLength;              // 5, Luna counts every byte
s.cStringLength();         // 3, C stops at the first NUL ("ab" + terminator)
"hi".cString().byteLength; // 3, the appended NUL is counted
"hi".cString().cStringLength(); // 3, scan reaches the appended NUL
```

So `cStringLength` is the tool for "how many bytes will C read," while `byteLength`
remains "how many bytes this string actually has." Neither is derivable from the other
in the presence of interior NULs, which is the whole reason `cStringLength` exists.

---

## 11. Open questions

- **Builder capacity semantics:** whether `capacityHint` is bytes or something coarser.
  (The other half of this question — whether `build()` transfers or copies — is resolved,
  R99: under COW, `build()` shares the buffer and the copy is deferred to the next
  append; string-builder §5.)
- **`slice` unit:** confirm `slice` is byte-offset (fast, boundary-checked) and that a
  separate `graphemeSlice` is not needed, versus offering both.
- **Error vs. sentinel:** `indexOf` returns `-1`, `parseInt` returns `int!`. Confirm
  this split (position-absent is ordinary and uses a sentinel; parse-failure is
  exceptional and uses the error channel) rather than unifying on one.
- **`cString()` return type:** it returns a `string` (bytes plus an appended NUL);
  confirm the FFI actually consumes a `string` here rather than a distinct pointer or
  `bytes` handle once the FFI is designed.
- **Interpolation and `regex`:** whether regex literals interpolate is tracked in the
  **regex** spec (its open questions), now that `regex` is specified there.
