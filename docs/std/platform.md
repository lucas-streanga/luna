# `std.platform`

**Status: deferred, and the most load-bearing stub in the corpus (R121).** `std.io` reads
`platform.lineEnding` in `println`'s *default parameter* today, and the `path` constraint's
`isValidPath` (filesystem §1 since R135) and io-errors' `errno` portability question (io-errors §4) both
defer here — so this record exists to make the module's absence a decision on the ledger.

What is fixed now (io §10): the module exports **`platform`**, a **`const` table of
compile-time-known target facts** — `platform.lineEnding` (`"\n"` / `"\r\n"`),
`platform.os`, `platform.pathSeparator` — and `const` + comptime-known is what makes its
members legal in default-parameter expressions (functions §3.3.1) and free at runtime
(const-table representation, tables Amendment A). No capability: these are compile-time
facts about the *target*, not a runtime reach outside the program.

Deferred: the full fact set; how `isValidPath`'s platform dependence is expressed; whether
`errno: int?` stays raw, becomes platform-tagged, or is dropped on non-POSIX targets
(io-errors §4); everything beyond the three members io needs today. The current compile
target is `linux-x86-64` alone (compiler §0), which is why deferral is cheap.
