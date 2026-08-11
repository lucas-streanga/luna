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
function that yields one string returns `string`. Positions are byte offsets typed
**`nat`** (the non-negative `int` constraint, constraints §10, R228), and a search
**miss is `null`**, never a sentinel (§5): the three failure channels, each spelled as
what it is — a **miss** is an answer (`null`, typed `T?`), a **failure** is the error
channel (`parseInt: int!`), a **misuse** panics (R228).

---

## 2. Length, indexing, and units

The units doctrine — no `length`, no integer indexing, three explicit units, because
silently picking one meaning is the most common source of Unicode bugs — is the
type's (string §2). The counts:

- **`byteLength(str): nat`** the number of UTF-8 bytes. O(1).
- **`codepointCount(str): nat`** the number of Unicode scalar values. O(n).
- **`graphemeCount(str): nat`** the number of grapheme clusters, i.e. what a person
  counts as characters. O(n).

Positions returned by search functions (`indexOf`, etc.) are **byte offsets**, and
functions that take a position (`slice`) take byte offsets too. Byte
offsets are the only unit that is O(1) to act on; codepoint and grapheme positions
would each be O(n) to resolve, so the API does not pretend they are cheap. Every byte
offset returned by the API lands on a codepoint boundary, and slicing validates that
its arguments do, so you cannot split a multi-byte codepoint by accident
(`stringBoundaryError`, §6; the probe form is `isCodepointBoundary`, §6). (A
`sliceBytes` this paragraph once named beside `slice` was a fossil — no such catalogue
entry ever existed; deleted, R228.)

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
```luna
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
```luna
fn repeat(str: string, times: nat): string
```
`"ab".repeat(3)` is `"ababab"`; `times = 0` yields `""` (a negative count, formerly a
silent `""`, is unrepresentable: `nat`, R228).

#### chr() / parse helpers
```luna
fn chr(codepoint: nat): string             // one-codepoint string from a scalar value
fn parseInt(str: string): int!             // parse; error if not a valid integer
fn parseDouble(str: string): double!       // parse; error if not a valid double
```
`chr` is the inverse of taking a single `codepoints()` element. A `codepoint` that is
not a Unicode scalar value (a surrogate `0xD800..0xDFFF`, or above `0x10FFFF`)
**panics** (`typeError`), a compile error where statically evident — the runtime mirror
of `\u{…}`'s lex-time rejection (string §5.1, R228). The parse helpers
return an errorable type (postfix `!`, the value-representation error model) and are
named for their **target** (`parse*` acquires from bare text; `to*` is reserved for
total conversions — the three-prefix contract, conversion §2, R106); `str.parseInt()`
must be consumed with `try` or an errorable binding.

---

## 5. Inspection and search

#### isEmpty()
```luna
fn isEmpty(str: string): bool
```
True iff `byteLength == 0`. O(1). (`str == ""` also works and is equally O(1).)

#### contains() · startsWith() · endsWith()
```luna
fn contains(str: string, needle: string, caseInsensitive: bool = false): bool
fn startsWith(str: string, prefix: string, caseInsensitive: bool = false): bool
fn endsWith(str: string, suffix: string, caseInsensitive: bool = false): bool
```
O(n).

#### indexOf() · lastIndexOf()
```luna
fn indexOf(str: string, needle: string, from: nat = 0, caseInsensitive: bool = false): nat?
fn lastIndexOf(str: string, needle: string, caseInsensitive: bool = false): nat?
```
The **byte offset** of the first / last match, or **`null`** if absent (R228: a miss is
an answer, not a sentinel — `nat?` structurally rules out the old `-1`, and `??` /
`is null` are the consuming idioms). `from` is a byte offset. O(n).

#### count()
```luna
fn count(str: string, needle: string, caseInsensitive: bool = false): nat
```
Number of non-overlapping occurrences of `needle`. O(n). An empty `needle` panics
(`emptyNeedle` — the rule below).

**The empty needle, ruled** (R229): `""` is a *determinate* needle for single-match
operations and a *banned* one for every-match operations. The empty string occurs at
every position, so an operation that must enumerate **all** matches — `split` (§8),
`count` (above), `replace` (§7) — would have to advance past each zero-width match by
some silently chosen unit (JS's code-unit choice severs surrogate pairs), the exact
silent pick string §2 refuses; these **panic** (`emptyNeedle`, errors §2), the
diagnostic naming the three explicit spellings of "every position": `graphemes()`,
`codepoints()`, `bytes()` (§9). Single-match operations need no unit and are
**defined**: `indexOf(s, "")` is `0` and `lastIndexOf(s, "")` is `byteLength(s)` (both
ends are boundaries), `contains` / `startsWith` / `endsWith` with `""` are `true`,
`replaceFirst(s, "", x)` inserts `x` at offset 0 (§7), and `before` / `after` follow
`indexOf` (§6). `split`'s **int arm joins the ban**: a chunk width `<= 0` is a
zero-byte step — the same emptiness — and panics `emptyNeedle` (§8).

#### matches() · find() · findAll() (regex)
```luna
fn matches(str: string, pattern: regex): bool
fn find(str: string, pattern: regex): table?
fn findAll(str: string, pattern: regex): stream
```
`regex` is its own type (a compiled pattern, kept separate so a pattern is compiled
once and reused, rather than recompiled from a string on every call). `matches` tests
for any match; `find` returns the first match as a table of capture groups (or `null`
if none); `findAll` yields one such table per match. A `replace` variant that takes a
`regex` target rides on the union already in §7. The `regex` type, its `~"…"` literal,
flags, engine guarantees (linear-time by default, opt-in backtracking under `/b`), and
runtime construction are specified in **regex** (its own document); this section only
fixes how strings consume one.

**The match-table shape is ruled** (R217), and it is one table with **both key spaces** —
PHP's `preg_match` shape, proven by two decades of the lineage Luna's tables descend
from: **int keys** are the positional groups (`0` the whole match, `1..n` the groups in
order), and a **named** group (`(?<name>…)`, regex §5.4) appears under **both** its
number and its name:

```luna
let m = text.find(/(?<year>\d{4})-(?<month>\d{2})/);
// m: [0 => "2026-07", 1 => "2026", 2 => "07", 'year' => "2026", 'month' => "07"]
let ['year' => y] = m;          // keyed destructuring composes for free
```

The duplication is deliberate (positional iteration and named access coexist; one table,
no accessor functions). **Positions are not in the table** — offsets are a different
question and a position-returning variant is deferred until need.

#### regexEscape()
```luna
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
```luna
fn slice(str: string, offset: nat, length: nat? = null): string
```
A substring beginning at byte `offset` for `length` bytes; **`null` means "to the
end"**, the default (the `finalGlue` pattern, §8 — absence spelled as absence; the
former `length <= 0` sentinel is retired, R228: a negative *computed* length silently
returned the whole tail, the silent-wrong-value shape). O(1): the result borrows the
parent's buffer (string-representation §7), so a
small slice pins its parent until `copy`d. Both `offset` and `offset + length` must
land on codepoint boundaries, or it panics (`stringBoundaryError`, errors §2, R228).

#### isCodepointBoundary()
```luna
fn isCodepointBoundary(str: string, offset: int): bool
```
The **probe form** to `slice`'s assertion — the hard/soft pairing the language uses
everywhere (`canReveal`/`reveal`, `peek`/`foreach`; R228). True iff
`0 <= offset <= byteLength` and `offset` does not land inside a multi-byte codepoint
(`byteLength` itself is a boundary: the end). Deliberately takes `int`, not `nat`, and
**never panics**: a guard must be askable with exactly the arithmetic-produced offset
it guards, a negative one answering `false`.

#### before() · after() · between()
```luna
fn before(str: string, sep: string): string     // up to the first sep, or all of str
fn after(str: string, sep: string): string       // after the first sep, or ""
fn between(str: string, open: string, close: string): string
```
Convenience extractors over `indexOf` + `slice`. All borrow. An empty `sep` follows
`indexOf`'s rule (§5): found at offset 0, so `before(s, "")` is `""` and
`after(s, "")` is `s` — determinate, not banned (only the every-match operations
refuse `""`, §5).

#### trim() · trimStart() · trimEnd()
```luna
fn trim(str: string, cutset: string = whitespace): string
fn trimStart(str: string, cutset: string = whitespace): string
fn trimEnd(str: string, cutset: string = whitespace): string
```
Remove leading/trailing characters in `cutset` (default: Unicode whitespace). The
result borrows.

#### padStart() · padEnd()
```luna
fn padStart(str: string, width: nat, fill: string = " "): string
fn padEnd(str: string, width: nat, fill: string = " "): string
```
Pad to `width` **grapheme** clusters (the visually meaningful unit for alignment) with
`fill`. No-op if already at least `width` wide.

---

## 7. Transformation

#### toUpper() · toLower() · toTitle() · fold()
```luna
fn toUpper(str: string): string
fn toLower(str: string): string
fn toTitle(str: string): string
fn fold(str: string): string        // case-folded form, for case-insensitive keys
```
Full Unicode case mapping, not ASCII-only. `fold` produces the canonical
case-insensitive form (what `caseInsensitive: true` compares under), useful as a map
key.

#### replace() · replaceFirst()
```luna
fn replace(str: string, target: string, with: string, caseInsensitive: bool = false): string
fn replaceFirst(str: string, target: string, with: string, caseInsensitive: bool = false): string
```
Replace all / the first occurrence. Returns a new string (immutability). An empty
`target` in `replace` panics (`emptyNeedle`, §5 — every-match); `replaceFirst` is
determinate on `""` and inserts `with` at offset 0 (§5).

#### reverse()
```luna
fn reverse(str: string): string
```
Reverses by **grapheme cluster**, so combining marks and emoji sequences stay intact.
O(n).

#### normalize()
```luna
fn normalize(str: string, form: enum {nfc, nfd, nfkc, nfkd} = {nfc}): string
```
Unicode normalization. Needed so that visually identical strings built differently
(precomposed vs. combining) compare and hash equal after normalizing.

---

## 8. Splitting and joining

#### split()
```luna
fn split(str: string, sep: string | int, limit: nat = 0): stream
```
Split on a `string` separator, or into fixed-width chunks of `int` bytes (the union
replaces an overload). `limit > 0` caps the number of pieces, the last holding the
remainder. An empty `sep`, or an `int` chunk width `<= 0` (a zero-byte step), panics
(`emptyNeedle`, §5 — the every-match ban; the diagnostic points at `graphemes()` /
`codepoints()` / `bytes()` for per-unit splitting). Pieces
borrow.

#### lines()
```luna
fn lines(str: string): stream
```
Split on line boundaries (`\n`, `\r\n`, and Unicode line breaks), terminators removed.
Pieces borrow.

#### join()
```luna
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
```luna
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
```luna
fn codepoints(str: string): stream   // elements are string
```
One element per Unicode scalar value. The safe technical unit; no Unicode-version
dependence.

#### graphemes() / characters()
```luna
fn graphemes(str: string): stream    // elements are string
fn characters(str: string): stream   // alias of graphemes
```
One element per grapheme cluster, what a person points at as a character. This is the
correct default for user-facing iteration; `characters` is the friendly alias and
never means codepoints.

#### isValidUtf8()
```luna
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
```luna
fn cString(str: string): string
```
Returns a string with a single NUL byte appended. It is defined to be exactly
`"$str\u{0}"` (one interpolation, string §5; the `\0` spelling this entry once used is
an R150 lex error — the sweep miss fixed by R228), nothing more: no scanning,
no de-duplication of existing NULs, just an appended terminator so the result is safe
to hand to C. Because the terminator is a real, counted byte, the result's
`byteLength` is `str.byteLength + 1`.

#### cStringLength()
```luna
fn cStringLength(str: string): nat
```
The number of bytes up to and including the **first** NUL, which is the length C will
actually see. This is an O(n) scan, not a field read, because a Luna string's
`byteLength` counts every byte (including any terminator and any interior NULs),
whereas C stops at the first NUL. The two disagree whenever the string has an interior
NUL:

```luna
let s = "ab\u{0}cd";
s.byteLength;              // 5, Luna counts every byte
s.cStringLength();         // 3, C stops at the first NUL ("ab" + terminator)
"hi".cString().byteLength; // 3, the appended NUL is counted
"hi".cString().cStringLength(); // 3, scan reaches the appended NUL
```

So `cStringLength` is the tool for "how many bytes will C read," while `byteLength`
remains "how many bytes this string actually has." Neither is derivable from the other
in the presence of interior NULs, which is the whole reason `cStringLength` exists.

---

## 11. Resolved (R228, R229)

- **Builder capacity: bytes.** `capacityHint` pre-sizes in bytes — the unit `reserve`
  already speaks (string-builder §2); both are `nat` now, and `reserve`'s parameter,
  formerly named `bytes`, is **`numBytes`** (shadowing a predeclared type name in a std
  signature is legal, keywords §5, and terrible; the surface will not model it). (The
  other half of this question — whether `build()` transfers or copies — was resolved by
  R99: under COW, `build()` shares the buffer and the copy is deferred to the next
  append; string-builder §5.)
- **`slice` unit: confirmed** — byte-offset, boundary-validated (`stringBoundaryError`,
  a leaf in errors §2's panic tree since R228; the §6 cite previously pointed at §10,
  C-string interop, and the type was defined nowhere), no `graphemeSlice`; the probe
  form `isCodepointBoundary` completes the hard/soft pair (§6).
- **Error vs. sentinel: resolved — the three channels** (§1). A miss is an answer:
  `null`, typed `T?` (`indexOf: nat?`; the type-system ground: `undefined` is
  unspellable in a return type, so undefined-returners degrade to `any` — `keyOf`,
  `peek` — while null keeps the signature precise). A failure is the error channel
  (`parseInt: int!`). A misuse panics. (`keyOf` and iterable `find` under the same
  convention: a recorded follow-up for iterable-functions.)
- **`cString()` returns a `string`, and no distinct C-string type exists** —
  unnecessary: NUL is one valid codepoint, so UTF-8 validity holds; the interior-NUL
  honesty lives in `cStringLength` (§10); the deferred FFI consumes a `string`.
- **Interpolation and `regex`: resolved** in the regex spec (§7): literals interpolate
  `${expr}` under the comptime-only condition; compilation stays compile-time.
- **The `""` separator: ban kept, named, and principled** (R229). The challenge ran
  and sharpened the ground: the disease is every-match *enumeration*, so the ban is
  the trio — `split` (both arms: `""`, and chunk width `<= 0`), `count`, `replace` —
  panicking `emptyNeedle` (§5, errors §2), because enumerating zero-width matches
  forces a silently chosen unit, string §2's refused move. Single-match operations
  are determinate on `""` and **defined** instead (§5): a boundary answer needs no
  unit. The diagnostic names `graphemes()` / `codepoints()` / `bytes()`.
