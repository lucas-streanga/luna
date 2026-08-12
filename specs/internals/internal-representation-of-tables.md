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

### 1.1 The three storage modes, and the two header flags (R196)

Two flags — **`isList`** and **`isContiguous`** — select among a one-way ladder of three
modes:

1. **List** (`isList`, which implies `isContiguous`): keys are exactly `0..n-1` (the ruled
   O(1) semantic property, tables §2.2 — this flag *is* its implementation), the maps are
   **unallocated** (nil), and `t[i]` is `entries[i]` directly: list access is bounds-checked
   array access, **zero hashing**. This is what "lists are stored contiguously" means
   operationally.
2. **Contiguous-with-holes** (`isContiguous` only), governed by one invariant:
   > for every live entry, **key ≡ entries index**; holes are tombstones; iteration order ≡
   > index order.
   Deleting from a list lands here (`isList` clears, identity addressing survives), which is
   the filter-by-delete workload staying fast. Preserving operations: delete (mark the hole),
   overwrite a live key, append at key > max — including sparse appends that create
   intermediate holes (legal; they spend hole budget). **Breaking operations flip to mode 3
   immediately, regardless of hole count**: re-inserting a previously deleted key (insertion
   order demands it iterate *last*; refilling its old slot would iterate it in the middle —
   the order-violation trap), any string-key insert, any out-of-order int insert. The flip —
   also triggered lazily by the hole-ratio threshold (§6) — is one O(n) compact-and-populate
   pass. This is PHP's **packed array** design (`IS_UNDEF` holes, hash on violation), the
   zend precedent a second time.
3. **Mapped**: the general §1 shape (the tiny-table linear knob lives inside this mode).

**The flags live on the table's heap header, not in the `lval`** — and the corpus had
already ruled this in disguise: value-representation §2.1's principle for `taken` ("one
shared fact must not have per-slot copies that can disagree") applies verbatim, since
several lvals alias one COW-shared table and a mutation through any alias changes the mode
for all. (The header's final shape is §1.4's; the mode "bits" themselves turned out to be
**derivable** and are not stored at all, §1.5.) `is list` stays O(1): two loads off one
cache line.

**Tombstones are in-line and cost nothing; no tombstone list exists** (R196):

- The dead-slot marker is the entry value's own **`isUndefined` flag** — unambiguous
  *because* `undefined` is unstorable in a table (the semantic rule doing representation
  work), so no dedicated bit or key sentinel is needed.
- The closure that falls out: in contiguous mode, reading a deleted key returns the
  tombstone itself — `t[2]` on holed key 2 reads `entries[2]`, an undefined-flagged `lval`,
  and an absent key is *supposed* to read `undefined`. **The tombstone is its own correct
  return value**; the read path has no special case.
- A tombstone free-list would serve nothing **by semantics, not by optimization**: the only
  thing free-lists buy is slot *reuse*, and insertion order makes reuse illegal — a new
  entry must iterate last, so it can only append. Which slots are dead is never needed;
  *how many* (the threshold input) is the header's hole counter.
- The honest iteration cost: one well-predicted branch per slot, and dead slots pollute
  cache lines until compaction — bounded by the threshold knob (§6).

### 1.2 Iteration is contiguous; sorting pays the honest price (R197)

The layout's quiet crown jewel: **in-order iteration is a linear memory walk.** Entries are
stored in-line (~48 bytes each in mapped mode — 24 in the keyless modes, §1.3 — one to two
entries per cache line, the prefetcher's ideal case), and for the hot element types the walk
dereferences **nothing at all** — `int`, `double`, `bool` are inline scalars, 
and a string of ≤ 8 bytes isstring-representation's **tier-1 inline**, 
living entirely inside the value `lval`. A tableof ints or short strings iterates as one 
sequential memory stream. 
Contrast the node-per-element world (Go's own map, C++ node maps): a guaranteed cache miss per element.

The flip side, accepted with its accounting: **sorting moves whole entries, not pointers** —
and the trade is safer than it first looks, three ways:

1. **Sort was never cheap in this representation.** Sorting changes the *order*, and order
   is the load-bearing axis: every entry's index changes, so the key→index maps rebuild
   wholesale regardless of how bytes move. Entry movement is a constant factor riding an
   operation that is already an O(n) index rebuild.
2. **The common case dodges even that**: sorting a *list* renumbers `0..n-1` by definition
   and a list's maps are nil (§1.1) — list sort is a pure entries permutation, no map
   rebuild at all. The expensive shape (sort-by-value keeping string keys) is the rare one.
   And under COW value semantics a sort that *produces* its result is ordinary table
   construction in sorted order — no in-place movement exists to complain about.
3. **The implementation escape is standard and free until needed**: sort an 8-byte index
   permutation, apply it to entries in one pass — each entry moves exactly once instead of
   O(log n) times. Sort-internals only; no representation change.

The rejected alternative gets its full indictment: `[]*entry` (pointer-per-entry,
double-boxing) would voluntarily import the `unordered_map` disease §2 exists to warn
against — an allocation per entry, GC scan pressure on every pointer, a cache miss per
access, **paid on every iteration forever to subsidize the rare sort** — and it would kill
the homogeneous noscan-storage path (value-representation §1.1's recovery) the moment
entries became pointers.

### 1.3 Keys: 24 bytes where stored — and not stored at all in the keyless modes (R198)

Keys are only ever `int | string`, which invites shrinking the key half of an entry below a
full `lval`. The general case cannot shrink, for two reasons **stronger than the obvious
one** (inline strings are not what closes the door):

- **16 bytes is the forbidden union.** A `{meta, word}` key needs `word` to be sometimes an
  int scalar and sometimes a string pointer — exactly the maybe-pointer word Go's precise GC
  forbids (value-representation §1.1's theorem, striking a second time).
- **8 bytes died by our own ruling.** A tagged single word needs at least one bit stolen
  from int keys — and R195 deliberately pinned "the cap is on entry count, **never key
  magnitude**": full-range `int64` keys are legal, so there is not one bit to take.

One genuine shrink exists and is **deferred** (§6): a 16-byte key whose payload word holds
the int value, the inline string bytes, *or* a scalar **index** into a side array of
heap-string descriptors — an index, not a pointer, so the GC objection vanishes. Costs an
extra hop on heap-string key compares and a side-array lifecycle glued to compaction; saves
8 bytes per stored key. Measurement-driven, not free.

**The real optimization is that the common key costs zero bytes** (R198): in the list and
contiguous modes, key ≡ entries index is the mode invariant (§1.1) — the key column is
*derivable data*. So entries split into a **values column** and a **keys column**, and the
keys column is **nil in the keyless modes** — the third instance of the allocate-on-flip
pattern (the two maps, and now the keys). Consequences:

- A list entry is **24 bytes, not 48**: iteration density doubles for the most common table
  shape in existence, and `foreach (k => v)` yields the index as `k` — the shape the
  implicit-key machinery already has (R93).
- The **homogeneous-noscan composition completes**: a list of ints is *one* pointer-free
  block — no key column exists to spoil the classification
  (value-representation §1.1's recovery, now reachable for every scalar list).
- The flip to mapped mode **materializes the keys column** in the same O(n)
  compact-and-populate pass that builds the maps (§1.1); mapped-mode iteration reads two
  parallel sequential streams (prefetch-friendly), and sort permutes both.

(Where §1 and §1.2 say "entries," read the pair of parallel columns — one column, in the
keyless modes.)

**Interleaved entries: considered and rejected** (R202). The alternative — two entry
layouts, a compact values-only array for lists and a key-beside-value array for mapped
tables — was reviewed against the columns and loses. First, the premise correction that
decides most of it: the keys column is **not an indirection** — `keys[i]` is direct
parallel indexing, and the only pointer-follow anywhere (a heap >8-byte string key reaching
its descriptor) is **layout-invariant**, since an interleaved entry stores the same key
representation and follows the same pointer. Interleaving changes exactly one thing: stream
count for keyed iteration. The accounting:

| Access | Parallel columns | Interleaved |
|-|-|-|
| list iteration (any form) | perfect — values only | same |
| mapped `v`-only iteration | **pure values stream** | keys dragged through cache: **2× bandwidth** |
| mapped `k => v` iteration | two streams (prefetchers eat this) | one stream — the sole win, marginal |
| small-state key scan (§1.5) | **keys packed: 8 keys ≈ 3 lines** | 48-byte stride: ≈ 6 lines, **2× worse** |
| random lookup | map → `values[i]`, key untouched | same, key adjacent but unneeded |

One marginal win against two 2× losses — and iteration, the stated first priority, is where
the columns are strongest. The structural cost seals it: two entry layouts **fork every
value-touching operation by mode** (the equality walk, the COW split copy, the
serialization walk, spread's fold — all mode-blind today over the uniform `[]lval` values
column), and mixing key representations into the value block spoils the noscan
classification for value-homogeneous mapped tables. The same
two-representations-of-one-thing smell §1.5's derive-don't-cache ruling guards against.

### 1.4 The header: one cache line; and the empty singleton (R199)

The full header, with the overhead accounted honestly:

```
flags:   int64            // 1 immortal bit | 31-bit sharing count | 32-bit hole count — no mode bits (§1.5)
entries: []entry          // 24B slice header (the values column)
keys:    *keysBlock       // 8B; nil in the keyless modes (§1.3)
intIdx:  map[int64]int32  // 8B; nil until needed (§1.5)
strIdx:  map[string]int32 // 8B; nil until needed
```

**56 bytes → Go size class 64 → the whole header is one cache line.** The single flag word
is legal because two of §1.1's header fields are **derivable** — live count is
`len(entries) − holeCount`, next-append is `len(entries)` (contiguous mode only ever
appends; sparse appends extend `len` through the holes) — and the packed counts fit
*because* R195 caps entries at 2³¹. List-mode waste is 24 bytes of nils plus the flag word;
per entry, a list is 24 bytes against PHP 7.4's 32-byte packed buckets, and the pre-7.4
~100-byte-per-element disaster is avoided *by construction* (it was the double-boxing §1.2
indicts). Below this header there is no meaningful floor: the remaining bytes are
load-bearing.

**The empty table is a singleton** (R199): `[]` points at one global, immortal empty block
(`isList` set — keys `0..n-1` holds vacuously — everything else nil), so empty tables cost
**zero allocations** until first write, and `[] == []` hits the shared-storage fast path
instantly. PHP's immutable empty-array singleton is the precedent. The load-bearing
subtlety: the singleton is shared by **every task**, so it may not carry a maintained
sharing count — §3's counts are non-atomic *because* mutable tables are task-confined — and
it therefore joins the class value-representation §6.1 already defines for `const` tables:
**count-free, flagged always-shared**, every write path treating it as shared and
COW-splitting unconditionally, no count write ever (the immortal-refcount trick). A
mutation on `var t = [];` splits off a fresh private table exactly as any COW write from
shared storage does.

### 1.5 Small tables and the dispatch: nil-ness is the mode (R200)

The ≤8-entry optimization is not a fourth mode — it is **the maps' allocation gate
decoupling from the mode flip**. An order-violating insert forces the keys *column* to
materialize (identity addressing is dead, §1.1), but not the *maps*: those exist to make
lookup sub-linear, which at ≤8 entries they do not. The ladder refines to:

```
keyless (list / contiguous)
   → order violation:  keys column materializes; maps STAY NIL; lookup = linear scan
   → len > 8:          maps materialize (the compact-and-populate pass)
```

A small string-keyed record — the dominant table shape in PHP-lineage code — lives its
whole life in the middle state. Why the scan genuinely wins there: **a ≤8-entry table is
one Swiss Table group** — swiss tables resolve *within* a group by scanning; the hash
exists only to pick the group, and with one group there is nothing to pick, so scan-only is
the degenerate swiss table with the pointless half removed. And the scan is word-shaped:
record keys are short, an inline (≤8-byte) string key **is one 64-bit word** (§1.3), so
lookup in an 8-entry record is eight masked word-compares against contiguous memory — SIMD
shape, zero dereferences.

**The fingerprint word: considered and ruled against** (R201). One 64-bit word of per-slot key
fingerprints — the swiss *control word* without the table — would SWAR-filter the scan to
candidate slots. Rejected because its economics do not transfer from the context it comes
from: swiss groups filter comparisons that are *expensive* (dereference + memcmp on
distant lines), while our small-state scan compares single words on lines the prefetcher
already pulled — a three-op filter in front of ~1-cycle compares saves approximately
nothing, and it adds a parallel structure maintained on every insert and delete: real
complexity and a real bug surface purchased for a hypothetical win. A textbook
mis-optimization shape. Revisit **only** if measurement ever shows small tables dominated
by heap (>8-byte) string keys, where each scan compare is a dereference plus memcmp and
the filter's skip is genuine — and if revived, the header's 64-byte size class holds one
spare word of allocation slack (§1.4 is 56 bytes), so the cost would be cycles only, zero
marginal memory. Realistically: not expected to be implemented.

The dispatch, with every discriminant on the one header cache line:

```
t[k] where k: int
  if t.keys == nil:                 // keyless modes: identity addressing
      return k in [0, len) ? entries[k] : undefined    // tombstone IS undefined (§1.1)
  if len(t.entries) <= 8:           // small state: scan, maps never consulted
      scan keys column for k  →  entries[i], or undefined
  if t.intIdx != nil:               // mapped: hash lookup
      return entries[intIdx[k]]
  return undefined                  // len > 8 and no int map ⇒ no int keys exist
```

The `len` gate resolves a real ambiguity: in a large int-only table, `strIdx == nil` means
"no string keys exist — answer `undefined`," while in the small state it means nothing (the
scan answers); the gate keeps the two readings from ever colliding. It is also
hysteresis-proof: a table that shrinks back under 8 with maps allocated takes the scan
path, which is *always correct* — maps are maintained but simply not consulted.

**Zero mode bits — the modes are derived, never stored** (R200): every rung is a function
of nil-ness and two counts — **list** = `keys == nil ∧ holes == 0`; **contiguous** =
`keys == nil ∧ holes > 0`; **small** = `keys ≠ nil ∧ len ≤ 8`; **mapped** =
`keys ≠ nil ∧ len > 8`. Ruled *derive, don't cache*: mode bits caching a derivable fact
would be the disagreeing-copies smell (value-representation §2.1) in miniature — a flip
that updates a pointer but not the bits is a bug class that cannot exist when the pointer
*is* the mode. This is what empties the flag word down to §1.4's counts.

The transitions:

- **Order violation** (keyless → small): allocate the keys column, fill `0..len-1` with
  identity keys, apply the violating write. Maps untouched — that is the decoupling.
- **Insert #9** (small → mapped): allocate and populate only the map(s) for key spaces
  that exist; the other stays nil, meaning "no such keys" from then on.
- **Compaction** (hole threshold, §1.1): compact both columns, rebuild whichever maps are
  non-nil.
- **Hysteresis**: once allocated, maps stay (no shrink-back thrash; Go maps release
  nothing anyway).

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

## 3. COW and sharing: the sharing count, precisely (R200)

The header's **sharing count** (§1.4's 31-bit field) is the **copy-on-write
discriminator**: its only job is to answer, at each mutation, *"is this storage shared
right now?"* Tables are value types — `var b = a;` must behave as a full copy — so
assignment copies only the `lval` (both now point at one block) and the count records the
block's reference population:

- **Alias created** (assign, pass, store into another table, bind): count++.
- **Alias dropped** (scope exit, rebind, entry overwritten or removed): count−−.
- **Mutation**: count == 1 → sole owner, **write in place**; count > 1 → **split** (fresh
  block, entries copied, writer's `lval` repointed; new count 1, old count decremented),
  then write. The split is the "copy" in copy-on-write, deferred to the first moment it is
  observable.

This is what makes value semantics affordable — copies are O(1) until somebody writes —
and shared storage is equality's pointer fast path to `true` (equality §4.2).

**It is not garbage collection.** Lifetime and freeing are Go's GC's, entirely (the same
division the string spec draws); the count never decides when memory dies, only whether a
write must split. That division gives the count its most important property, the
**asymmetric failure directions**: an **overcount is safe** — a spurious split, correctness
intact — while an **undercount is unsound** — an in-place write visible through another
alias, value semantics silently broken. The emitter's increment/decrement discipline is a
soundness obligation in one direction only, and "when in doubt, count high" is always
legal. The degenerate alternative value-representation §6.1 names — a sticky *shared* flag
that never clears — is the extreme of that safety (transiently-shared tables copy forever
after); the count's precision is what lets a table *regain* sole ownership as aliases
genuinely drop. Saturation of the 31-bit field degrades to exactly the sticky behavior —
safe.

**The arithmetic is plain and non-atomic** (value-representation §6.1): a mutable table is
owned by exactly one task at every instant — spawn deep-copies mutable arguments and
captures, results transfer by handoff — so a given count is only ever touched from one
thread. No atomics, no locks, no contended lines, on the hottest bookkeeping in the
runtime. The contract protecting this is §6.1's transitively-deep spawn copy: one nested
mutable table aliased across a task boundary would put one count under two threads.

**Two count-free citizens**, each avoiding cross-task count writes by not maintaining a
count at all: **const tables** (deeply immutable, never split — nothing to count, which is
why cross-task const sharing is free) and **the empty singleton** (§1.4, R199 — shared by
every task, flagged immortal, every write treating it as shared and splitting
unconditionally). Where a count cannot be safely maintained, the answer is a class whose
behavior does not need one.

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

- **dynamic `applyDynamic()` / `unapply()`** (protocols §4.4, §4.6, R123): the initializer table's
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
- **The 16-byte key scheme** (§1.3): whether the side-array-index key representation earns
  its extra hop and lifecycle, once real workloads exist.

(The fingerprint word is **not** on this shelf: ruled against, §1.5, R201 — a knob implies
an expected experiment; that one has a rejection with a narrow revival condition instead.)
