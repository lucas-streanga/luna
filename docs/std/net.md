# `std.net`

**Status: deferred, in full — the largest missing std family (R121).** Nothing in the
corpus can open a socket, resolve a name, or speak TLS, and until this record there was
not even a boundary note saying so. The absence is now a decision on the ledger, with the
composition constraints other rulings already fix:

- **Capability-gated**: a `net` capability (capabilities §9's family), separate from `io`
  and `system` — network reach is its own authority, and `use (net)` will be its complete
  audit.
- **Connections are handles in the transferred / taken class**, like files (io §2,
  concurrency §2.1): stateful, single-owner, referent-marked, `identityEquality`,
  no-`&` operations (the R121 stream convention).
- **Reads are streams** (R102's producers-produce-streams; creation-authorized lazy
  effects per R121's laundering-theorem precision), non-restartable (a socket is the
  canonical non-replayable source, stream §4).
- **Blocking operations are suspension points** (concurrency §6.1): a parked read is
  interruptible by cancellation, refused-on-entry — which is what makes a task blocked on
  a dead peer cancellable at all.
- **Credentials are secrets** (secret spec), revealed at the connection boundary — the
  exec pattern (exec §1).

**The gating dependency, stated plainly**: `std.net` should not ship before the timeout /
`awaitAny` surface and `std.time` (concurrency §8, the named top post-alpha priority,
std/time.md) — a network API without deadlines is the BEAM lesson in reverse, and the
R120 scrutiny is the reason this module waits for that one.

Deferred: everything else — the surface (dial/listen/accept), DNS, TLS, datagrams, the
error family. Pending the timeout surface, then real use.
