# String Representation

How Luna stores strings at runtime. This is the sibling of the value-representation
spec: it describes the `string` payload that an `lval` of type `string` points at.
The user-facing string API is specified separately; the view operations in §9 are
included here only because they fall out of the representation.

---

## 1. Goals and invariants

- **Immutable.** A string never changes after creation. This is what lets copies
  share storage with no copy-on-write machinery: there is nothing to protect against.
- **Always valid UTF-8.** Validity is established once, at ingress (§8), and every
  internal operation preserves it, so no operation re-validates. Arbitrary byte data
  is `bytes`' job (bytes spec), never a string's.
- **Common operations are cheap.** Copy, pass, compare-common-cases, slice, and
  short-string creation are all O(1) or near it.
- **No interning.** A string's identity is its contents; there is no canonical form
  and no global string table. This keeps the model a plain immutable value and keeps a
  string's birth cheap. (Content-based fast paths, and interning as an internal
  optimization, can be added later without changing this surface; see §11.)
- **Go GC owns lifetime.** There is no manual refcounting and no manual free. Every
  non-static allocation is Go-managed and reclaimed normally. (Refcounting is rejected
  by review, not omitted by oversight — §11.1, R169.)

---

## 2. Representation

A string is one of two things: **inline in the `lval`** (tier 1), or a pointer to a
16-byte **descriptor** behind `dataPtr` (everything else). The single most important
consequence is a branch: every operation that reads a string's bytes must first ask
"inline, or pointed-to?" before it touches `dataPtr`. That check sits *above* the
descriptor's own tag.

### Tier 1: inline in the `lval`

A string of up to **8 UTF-8 bytes** lives entirely in the `lval`, with **zero
allocation**. The bytes occupy `dataPtr`; the string-inline field of the `lval`
(value-representation §2.2) marks the string inline and holds its length (0..8). `""`
is just an inline string of length 0, so it needs no allocation, no descriptor, and
no null-pointer special case. This tier carries the bulk of real string traffic:
short ASCII identifiers and keys (`id`, `name`, `type`), single ASCII characters from
`codepoints()`, small tags, and one or two multi-byte codepoints.

### Tiers 2+: the descriptor

Anything longer than 8 bytes is a descriptor, a tagged union over two `kind`s. The
tag plus a flag occupy spare bits (the high byte of the length word; 56 bits of
length is 72 PB, far more than any real string), so the descriptor stays 16 bytes.

| kind | Layout | Owns bytes? | Lifetime |
|-|-|-|-|
| `owned` | `{length, ptr}` where `ptr` is a Go `[]byte` this string owns | yes | Go GC |
| `borrowed` | `{length, ptr}` where `ptr` points *into* another string's buffer | no | Go GC via the parent (§7) |

A single flag distinguishes them: **`isBorrowed`** marks a non-owning slice, which
matters only to `copy` (§7). Neither kind participates in any global table; a string's
identity is its contents, nothing more.

Descriptor-SSO (inlining up to 15 bytes *in the descriptor* to skip the buffer
allocation) is deliberately **not** included. Tier-1 inline already removes the
allocation for the smallest strings, which is the steep part of the curve;
descriptor-SSO would only turn two allocations into one for the 9..15-byte band, and
would add a fourth representation to branch on. It is left as a possible later
optimization behind the same inline-versus-pointed-to branch (§11).

---

## 3. Memory management

Go's garbage collector owns every non-static allocation. Because strings are
immutable, a value copy is just a copy of the `lval` (logically 16 bytes;
value-representation §1.1) — its `dataPtr` now refers to the same descriptor; no
refcount, no COW, no duplication. The descriptor
and any owned buffer stay alive as long as some `lval` reaches them, and are reclaimed
when none does.

- **Inline (tier 1)** strings own nothing: the bytes are in the `lval`, and copying
  the `lval` copies the string.
- **`owned`** buffers are Go `[]byte`; GC reclaims them.
- **`borrowed`** slices hold an interior pointer into a parent buffer. Go's GC keeps
  the *entire* parent backing array alive as long as the interior pointer is
  reachable, so a slice never dangles. It also means a small slice pins its whole
  parent until the slice dies (§7).

This deletes the two hardest problems from the original draft: there is no
finalizer-driven free, and no shared-pointer free-ordering question, because nothing
is manually freed.

---

## 4. No interning

Luna does not intern strings. There is no canonical form, no global string table, and
no `intern()` in the language. A string's identity is its contents and nothing more,
which keeps it a plain immutable value and keeps every string's birth cheap: no hash,
no map lookup, no lock, ever.

This drops the most complex subsystem the original draft carried, a runtime map with
weak references, a cleanup, and concurrent-access sharding, in exchange for an
equality whose cost is easy to predict (§6). The narrow case interning served, long
strings compared very frequently and often equal, is rare in general-purpose code and
is better addressed later by a content-based fast path than by programmer-facing
canonicalization (§11).

---

## 5. No static set

There is no permanently-resident table of static strings, not for ASCII, not for the
empty string. Everything that would have gone in one is handled by tier-1 inline
instead:

- **The empty string `""`** is an inline string of length 0. No allocation, no
  descriptor, and no null pointer to guard against, because an inline string
  dereferences nothing.
- **Single ASCII characters** (from `codepoints()`, `bytes()`, and the like) are
  inline strings. Producing one allocates nothing and compares in O(1) (§6), which is
  exactly what a static ASCII set was for.
- **Small non-English strings** (a few 2..4-byte codepoints) are inline when they fit
  in 8 bytes, and `owned` otherwise.

A per-codepoint static table would also have been the wrong tool regardless: there are
~150,000 assigned scalar values, mostly cold, and "one character" as a user means a
*grapheme cluster*, which is unbounded (combining marks, ZWJ emoji). Inline strings
cover the hot small cases without pinning a large table or pretending codepoints are
characters.

---

## 6. Equality

```luna
fn equal(a: string, b: string): bool {
  if a.byteLength != b.byteLength      return false   // O(1)
  if a.isInline && b.isInline          return a.word == b.word   // O(1), one masked compare
  return memcmp(a.bytes, b.bytes, a.byteLength) == 0   // content compare
}
```

- The length check rejects most inequal pairs in O(1).
- If both are inline, equality is a single masked word compare of the inline bytes.
  This is the tiny-string fast path that the static ASCII set used to provide.
- The content compare is cheap whenever an inline string is involved, because equal
  lengths then force the other operand to be at most 8 bytes as well. Only two long
  strings hit a full O(n) `memcmp`, and even then unequal strings usually differ in
  the first few bytes, so the walk rarely runs to completion in practice.

An optional cached hash (stored in the owned buffer's header, not in the 16-byte
descriptor) can add an O(1) reject before the `memcmp` for long strings. Deferred
(§11).

---

## 7. Slicing

The rule is one invariant, by representation family:

- **Slice of a buffer-backed string** (`owned` or `borrowed`) produces a
  `borrowed` slice: a descriptor with the parent's `ptr` advanced by the offset and
  the new length. O(1), non-copying. Go's interior-pointer GC keeps the parent buffer
  alive (§3).
- **Slice of an inline string** stays inline (a copy of at most 8 bytes). There is no
  buffer to borrow, and a sub-span of an inline string always fits inline, so this
  case copies, trivially.

In short: slicing something inline stays inline by copy; slicing something
buffer-backed borrows.

Because a `borrowed` slice pins its whole parent, a small slice of a huge string keeps
the huge buffer alive. The escape hatch is the **`copy`** operator (from the variables
spec): `copy sliced` allocates a fresh `owned` buffer holding just the slice's bytes and
drops the interior pointer, so the parent can be collected. On any non-borrowed string
`copy` is effectively a no-op: inline strings already carry their own bytes, and an
immutable buffer shared by pointer is indistinguishable from a duplicate, so there is
nothing to deep-copy.

---

## 8. UTF-8 validity

Validity is a boundary property. It is checked once, with an O(n) validation pass, at
every point where untrusted bytes enter: file and network reads, FFI, and the
`bytes`-to-`string` decodes (fallible, living in the string API; bytes §5).
Construction from already-valid strings (slice,
concatenation, view element) needs no re-validation, because slicing respects codepoint
boundaries and concatenation joins two valid sequences.

The Unicode data needed for validation and for grapheme segmentation (§9) is bundled
into the compiler, not imported. This pins the runtime to a specific Unicode version,
which is the accepted cost of making correct Unicode handling available by default.

---

## 9. Views and indexing

Strings are **not** integer-indexable. UTF-8 has no O(1) random access by codepoint or
grapheme, and a single "character index" is the most common source of Unicode bugs
across languages, so Luna does not offer one. Nor is there a bare `length`: it is
`byteLength`, because "length" silently means three different things.

- **`byteLength`** returns the byte count. O(1), stored in the descriptor.
- **`bytes()`** returns the raw UTF-8 bytes as borrowed views. For I/O, hashing, and
  low-level work; the copying bridge to the `bytes` type is `toBytes()` (string-api §9).
- **`codepoints()`** returns Unicode scalar values, one per element. The safe technical
  unit: fixed meaning, no Unicode-version dependence. ASCII elements are inline strings
  (zero allocation); non-ASCII elements are borrowed slices into the parent (O(1), no
  per-element allocation).
- **`graphemes()`**, also spelled **`characters()`**, returns grapheme clusters, what a
  person points at as "a character," and is the correct unit for user-facing counting,
  truncation, and cursor movement. Elements are borrowed slices. This is the one view
  that depends on the bundled grapheme-break tables (§8). `characters()` never means
  codepoints; if it meant codepoints, `"café"` with a combining accent would give a
  surprising count.

Each view returns a **stream** — producers produce streams (R102, string-api §1) — and every
string-derived stream is restartable for free, because its source is immutable (stream §4,
R105).

---

## 10. Performance summary

- **Copy / pass:** O(1). Copy the `lval` (value-representation §1.1); inline bytes come
  along, a descriptor is shared. No refcount (immutable), no COW.
- **Creation (tiny, <= 8 bytes):** inline, **zero allocation**.
- **Creation (generic, > 8 bytes):** allocate + copy + validate-at-ingress. No map,
  no lookup, no lock.
- **`==`:** O(1) on differing lengths, O(1) when both inline (masked word compare),
  O(n) `memcmp` only for two long strings, and typically far less than O(n) in
  practice since unequal strings differ early.
- **Slice:** O(1) borrow (buffer-backed parents) or O(<=8) copy (inline parents).
- **`copy` (detach):** O(n) over the slice's bytes; drops the parent pin.
- **Joining:** O(n + m) per pairwise join; a result <= 8 bytes is inline. There is no
  concatenation operator (string §3, R27) — joining is interpolation, `join`, and the
  builder — and immutability makes a repeated pairwise-join loop O(n^2), so the builder
  (stringBuilder spec) is the accumulation tool.
- **`byteLength`:** O(1). **`codepoints()` / `graphemes()` counting:** O(n).
- **Memory:** inline removes allocation for the common tiny case entirely; borrowed
  slices avoid copies at the cost of pinning parents until `copy`.

---

## 11. Resolved and deferred

- **Concatenation builder type: resolved** — the builder exists in full (stringBuilder
  spec): a table wearing the `stringBuilder` protocol, single-owner, `build()` handing
  its buffer to an owned string with the COW copy elided.
- **Interning, later and invisibly** (§4): if a comparison-heavy workload ever warrants
  it, the answer is a content-based fast path (the cached hash below) or invisible
  interning of a specific internal population (e.g. an interpreter's own identifiers),
  never a programmer-facing `intern()`. Deferred entirely for now.
- **Descriptor-SSO** (§2): whether to add a representation that inlines 9..15 bytes in
  the descriptor to skip the buffer allocation, versus leaving that band as `owned`. A
  measurement-driven optimization, behind the existing inline-versus-pointed-to branch;
  not needed for correctness.
- **Cached hash** (§6): whether to store a hash in the owned buffer header for a faster
  long-string `==` reject.

### 11.1 The allocator review (R169): three optimizations rejected, one theorem kept

Immutability invites owning the allocator, and the package was reviewed deliberately
(R169): a dedicated string heap, deliberate close packing, and naive reference counting.
All three are **rejected on the Go backend**. One fact from the review is a theorem and
stays recorded.

- **A separate string heap solves a problem Go has already solved.** The premise —
  strings are expensive to GC — mostly dissolves on this backend: a pointer-free
  `[]byte` buffer lives in a **noscan span**, so the collector never scans string
  bytes; it marks the 16-byte descriptor and sweeps. The marginal GC cost of strings
  is near the floor with no new machinery. An actual private heap under Go means
  arenas (experimental, stalled) or manual memory via `unsafe` — reintroducing
  exactly the two problems §3 records deleting (finalizer-driven frees, free-ordering),
  and the first unsafe memory anywhere in the system. (The JVM analogy points the same
  way: no modern JVM has a separate string heap — the interned pool moved into the
  ordinary heap in Java 7 — and the live feature, G1 string *deduplication*, is
  GC-time invisible interning: this file's §4 stance, not a heap.)
- **Deliberate close packing** rides on the same allocator ownership and degrades §7:
  a borrowed slice today pins one parent buffer, with `copy` as the escape hatch;
  packed into an arena it pins the whole arena, promoting `copy` from optimization to
  memory-correctness obligation.
- **The refcount-completeness theorem — true, kept, and rejected.** The string
  reference graph is **acyclic by construction**: an inline string references
  nothing, an owned buffer references nothing, and a borrowed slice references
  exactly one buffer that existed before it. Immutability means no edge is ever
  added after birth, so no cycle can ever form — naive reference counting is
  therefore **complete** for strings: no cycle collector, no tracing, no backup GC,
  ever. That is a real theorem, and it stays recorded here for a hypothetical
  self-hosted backend. It is rejected today because it buys nothing and costs on the
  hottest path: under Go the GC already owns lifetime (and noscan spans already make
  strings cheap to collect, above), so a refcount would be pure addition — an
  inc/dec on **every `lval` copy and drop**, the most frequent operation in the
  runtime — and because immutable strings are exactly the values safe to share by
  reference across tasks, the counts would have to be **atomic**: cross-core
  contention on every shared hot string, purchased for a collector we do not need.
  "No refcount" (§1, §3) stays load-bearing.

Not on the rejected list, because it was never a proposal: tier-1 inline (§2) — the
review's fourth candidate was already this spec's ruled representation, flag bits and
all (value-representation §2.2).
