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

## Deferrals — resolved by the protocol redesign (R95–R98)

The deferrals recorded here pending the protocol redesign are **resolved by deletion**
(R98). Under the redesign, protocols never place members in element space; per-key
`get` / `set` grants on element keys no longer exist, and protocol-member grants are
compile-time assertions (protocols §3.1). Consequently:

- **`onNoGet` / `onNoSet`** are retired for good: bulk operations traverse element space,
  which carries no permissions. The enums return to no signature.
- **`TableReadViolationError` / `TableMutationViolationError`** are retired: grant
  violations are compile errors, not runtime events.
- **`canGet` / `canSet`** are retired from indexable-functions: element keys are always
  readable and writable by whoever holds the table (`has` covers presence); protocol
  grants are statically known and need no runtime predicate.
