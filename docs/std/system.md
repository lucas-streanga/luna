# `std.system` — retired name, split (R134)

The module this record deferred (R121) ships as **two**, and the name ships as
neither:

- **`std.filesystem`** — the metadata surface (`exists`, `stat`, delete, directory
  creation and listing) under the **`filesystem`** capability. The capability is
  *renamed* from `system`, superseding R121's naming half by R121's own argument: the
  record justified the split from `io` because contents and structure are "different
  authorities" — and the authority is the *filesystem*, which is what a `use
  (filesystem)` clause should say to an audit. Every effect module is 1:1 with its
  authority (`io`/`io`, `time`/`time`); this one now is too. The surface **landed**
  (R135, std/filesystem.md) with R121's constraints carried unchanged: the boundary
  stands (io stays contents-only, io §9), operations are non-comptime by construction,
  blocking calls are suspension points, listings are stream producers (R102).
- **`std.process`** (landed, R134) — everything process-shaped the old grab-bag
  implied: `args()` under `argv`, `envVars()` under `env` (relocated from exec §6),
  the no-`chdir` and no-`exit()` refusals.

The `system` capability name is dead with the module name; the "safe syscalls" framing
died earlier still (the clock went to `time`, R132). Nothing else lived here.
