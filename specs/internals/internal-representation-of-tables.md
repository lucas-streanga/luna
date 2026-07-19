# Table Representation

How Luna stores tables at runtime. This is the third internals sibling (value-representation,
string-representation): it describes the payload a `table`-typed `lval`'s `dataPtr` points at.
Table *semantics* — insertion order, key types, list-ness, COW value semantics — are the tables
spec's; this file is how the hosted runtime realizes them (R194). Like the string file, nothing
here changes an observable behavior; it records the representation and the reasoning so it is
not re-derived.

---

## 1. The shape: an ordered entries array, plus key→index maps

A table is **one data block with two faces**:

```
entries:  []entry            // insertion-ordered; the table's real contents
intIdx:   map[int64]int32    // key → index into entries
strIdx:   map[string]int32   // key → index into entries
```

- **The entries array is the table.** Every entry (key + value `lval`) lives there, in
  insertion order — iteration, `foreach`, spread, equality's ordered walk, and list-ness
  tracking (tables §2.2, an O(1) maintained property) all read `entries` and nothing else.
- **The maps hold only small integers.** Values never enter a Go map; the maps are pure
  key→index lookup. Two maps rather than one interface-keyed map, because Luna keys are
  exactly `int | string` (tables §3.1) and Go's runtime has **specialized fast paths for
  precisely `int64` and `string` keys** (no hasher indirection, no interface boxing). The
  `int32` index width is **contractual, not incidental** (R195): tables §1.1 guarantees at
  most 2³¹ − 1 entries — one cap, since both maps index the one entries array — so 4-byte
  indexes are legal forever, half the memory of an 8-byte scheme.
- **Deletion tombstones; compaction is a policy knob.** Removing a key marks its entry dead
  and drops the index; the entries array compacts lazily (on a dead-ratio threshold, or when
  an operation re-lists anyway). This is the **zend_array design** — PHP's array has been
  exactly this shape (hashtable over an ordered entry vector with lazy compaction) for
  decades, and it is the ancestor of Luna's table semantics: the layout is validated by the
  same workloads the semantics came from.
- **Go maps never shrink — defused.** A Go map releases no memory on delete; with values in
  our entries array and only `int32` indexes in the map, the retained residue is index-sized,
  not payload-sized.
- **The map is never iterated.** Iteration order is `entries`' — always — so Go's randomized
  map order **cannot** leak into any observable result. This satisfies compiler §6.3's
  no-bare-map-iteration discipline *structurally*: the map is incapable of contributing
  order, rather than disciplined out of it.
- **Tiny tables may skip the maps entirely.** Most tables are small; below a threshold N a
  linear scan of `entries` beats a hash lookup and allocates two fewer structures. A
  measurement-driven hybrid (§6), not a semantic switch.

## 2. Why Go's map is trusted with the lookup half

Recorded so the choice is not re-litigated. The cautionary tale is C++'s `unordered_map`,
which is slow **by standard-committee contract**: its API guarantees (element pointer
stability, the bucket interface) *force* node-based chained hashing — one allocation per
element, a pointer chase per probe — in every conforming implementation. Go made the opposite
API bet: no interior pointers into a map exist (`&m[k]` does not compile), so the runtime was
always free to move entries — open addressing from the start, and as of **Go 1.24 a Swiss
Table** implementation (control-word group probing, abseil lineage) over **extendible
hashing**, keeping growth incremental at fine granularity. Hashing is AES-based with a
per-process random seed (fast, hash-DoS-resistant); pointer-free key/value groups are noscan
to the GC. The pinned toolchain (compiler §0) inherits map improvements for free. The lesson
is the language's own: a restrictive API is what buys the fast implementation.

## 3. COW and sharing

The `{entries, intIdx, strIdx}` block sits behind the table's single heap header, which
carries the **non-atomic sharing bookkeeping** of value-representation §6.1 (a mutable table
is sole-owned by one task at every instant, so no atomics). Copies share the block until a
write splits it (COW); shared storage is equality's pointer fast path to `true`
(equality §4.2).

## 4. Protocol state: structs, never hashmaps

The protocol axis needs no hash lookup at all, structurally (R194): `->` is **compile-time
resolved** (operators §0), member sets are fixed per proto (protos are const-declared brands
in a closed universe, R126), and **no dynamic protocol index exists** — there is no
`t->[expr]`. So each applied protocol's per-table state is a **struct**: a fixed field block,
with fields **unboxed** to their declared member types (a `duration` member is an `int64`
field, not an `lval`) — protocol state gets C-struct density, which is exactly how `datetime`
was sized at ~24 bytes (datetime §1, R133).

The three dynamic surfaces that *look* like they need name-lookup are all served by **one
shared static descriptor per proto** (name → offset → typeid; a handful of members, linear
scan or better), never a per-value structure:

- **dynamic `apply()` / `unapply()`** (protocols §4.4, §4.6, R123): the initializer table's
  runtime name-strings resolve through the descriptor to struct slots;
- **`@@` serialization** (`includeProtocols`, json §2.1, R125): the name → granted-member
  enumeration walks the descriptor and reads fields by offset;
- **introspection's granted `read`** (introspection §4.4, R129): member descriptors carry the
  offset.

One refinement keeps the claim honest: the per-**member** layout is static, but the
**applied set** is dynamic (`apply`/`unapply` are runtime operations), so a table's protocol
axis is a small vector of `(protoId, *stateStruct)` pairs — the applied-set membership test
is O(#applied), which is O(1) in practice because sets are tiny, and `unapply` (R123) drops
one block from the vector.

## 5. Const tables: one representation, two paths

Amendment A's "perfect-hash struct layout" (compiler §5), sharpened (R194): "struct plus
perfect hash" versus "perfect-hash map" is a **false dichotomy**, because a perfect-hash map
that stores **offsets** *is* the struct viewed from the dynamic path. The representation is:

- **one data block**, insertion-ordered (iteration order is spec, so the PH *permutation*
  maps into the block and never reorders it), slots **unboxed** where the compile-known shape
  pins their types;
- **one key array** — required regardless: a minimal perfect hash yields a slot for *any*
  input, so `t['absent']` can only answer `undefined` by comparing against the stored key and
  rejecting. Membership needs the keys; the hash alone cannot answer it;
- **one small index**: the PH parameters plus a slot→offset array of small integers.

No data is duplicated anywhere. The two access paths read the same representation:

- **Static path** (compile-known key, compile-known shape): devirtualized to a direct field
  read at a constant offset — no lookup of any kind.
- **Dynamic path** (runtime key, or the const table has flowed into a plain `table`-typed
  position and the shape is statically lost): PH → key-verify → offset → **box on read**
  (per-slot typeid metadata makes constructing the `lval` from an unboxed slot cheap, since
  scalars are inline). The boxing cost is accepted: on a const table the dynamic path is the
  rare path by definition.

## 6. Open knobs (measurement-driven, not semantic)

- **The compaction policy**: the dead-entry ratio that triggers entries compaction, and which
  operations compact opportunistically.
- **The tiny-table threshold**: the entry count below which the index maps are skipped for
  linear scan.
