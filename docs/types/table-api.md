# Table API (retired)

The built-in table protocol is **retired** (R91–R93). Tables no longer carry a method
surface: every operation formerly catalogued here is a **built-in free function**, reached
by call or UFCS (`tab.map(f)` ≡ `map(tab, f)`, functions §3.4), and the catalogue is split
by what an operation actually needs:

- **iterable-functions.md** — functions over `iterable` (`table | stream`): everything whose
  semantics need only ordered traversal of `key => value` pairs.
- **indexable-functions.md** — functions over `table` only: keyed access, seal state,
  positional mutation, and the whole-input (sort) family.

Bare `tab->name()` is dead with the protocol; `->` reaches only *named* user protocols.
`tab.name` is element access, `tab.name(...)` is UFCS — each spelling has one meaning.
Concepts (copy-on-write, `&`-write-back, sealing, per-key access) remain authoritative in
**tables.md**.

---

## Deferrals, pending the protocol redesign

Recorded here so the redesign finds them in one place:

- **`onNoGet` / `onNoSet`.** The bulk-operation permission enums (`enum {throw, skip}`)
  were dropped from **every** signature. How bulk reads and writes interact with per-key
  `get` / `set` grants (tables §6) is re-specified with the protocol redesign, not before.
- **`TableReadViolationError` / `TableMutationViolationError`.** The per-key grant
  violations formerly summarized here are parked (indexable-functions §5 carries the stub
  rows) pending the same redesign.
- **`canGet` / `canSet`.** Signatures are settled (indexable-functions §1); the grant
  semantics they report are pending the same redesign.
