# `std.system`

**Status: deferred, boundary fixed (R121).** `std.io` deliberately excludes filesystem
*metadata* — `exists`, `stat`, deletion, directory listing and creation (io §9) — and this
record names their future home so the boundary is a decision, not an omission: metadata
operations belong to the **`system` capability** (capabilities §9), separately gated from
`io`, because "may read and write file contents" and "may enumerate, inspect, and delete
the filesystem" are different authorities a program should be able to hold separately.

What is fixed now: the boundary itself (io stays contents-only), the gating capability's
identity (`system`), and the composition constraints other rulings already impose —
operations that reach the OS are non-comptime by construction (functions §5.5), blocking
calls are suspension points (concurrency §6.1), and results are ordinary values (a
directory listing is a table or a stream per R102's producer rule).

Deferred: the entire surface — names, signatures, the error family (which errnos map where,
extending io-errors' four-fates partition), recursive operations, watching. Pending real
use.
