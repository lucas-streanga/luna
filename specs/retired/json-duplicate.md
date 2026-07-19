# Retired: the duplicate json spec (was `types/json.md`)

**Retired R180.** The corpus briefly carried two diverged `json.md` files: this one (the
type-side spec, stale since the R145 rename — its `toJsonDynamic` signature never gained
the R125 writer flags) and `std/json.md` (the module spec, R125-current, where most
"json §" cites resolve). R180 collapsed them: **`specs/std/json.md` is the one json
spec.**

Where this file's unique content went (everything else was subsumed by std's richer
text):

| Was (here) | Is (std/json.md) |
|-|-|
| §1's value-carried and equality-erases bullets | §1's constraint-rules bullets |
| §1.1's boundary-idiom code block | §1.1's tail |
| §1.2 "The cost, precisely" | §1.2, carried whole |
| §1.3 "Predicate dependency" (constraints §11) | §1.3, carried whole |
| §2/§3 (generator, dynamic walk) | §2 (already richer: flags, R125/R157) |
| §5's RFC 8259 expectation and number-fidelity open | §4's opens |

Nothing here is authoritative; the full pre-merge text is in git history.
