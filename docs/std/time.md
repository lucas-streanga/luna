# `std.time`

**Status: deferred, in full (R120).** This file exists so that the time surface's absence
is a *decision on the ledger*, not an oversight: the concurrency scrutiny against BEAM
found that nothing in the corpus today can read a clock, wait a duration, or build a
timer — a retry loop with backoff is currently unwritable. The API, the names, the
capability, and the representations are all deferred; this file records what the module
must eventually cover and the few constraints already fixed by other rulings.

---

## 1. What the module must cover

- **`sleep(duration)`** — park the task for a duration. Two things are ruled *now*, by
  composition with R115/R119: a sleeping task is at a **suspension point** (cancellation
  delivers there, refused-on-entry, concurrency §6.1), and `sleep` is the **timer half of
  the deferred timeout / select surface** (concurrency §8): "await with deadline" is a
  race against `sleep`, so the timeout family lands with — not before — this module.
- **Reading the clock** — capability-gated: the clock is already named among the
  authorities that reach outside the program (functions §5.5), so clock reads are
  non-comptime by construction (a build must not depend on when it runs). The
  capability's name (`time`?) and grain are deferred.
- **Monotonic versus wall-clock** — must be distinct types or distinct readings; conflating
  them is the classic bug class (elapsed-time arithmetic across a wall-clock adjustment).
  Shape deferred.
- **Durations** — representation deferred (a bare `int` of nanoseconds versus a distinct
  type; a distinct type would be the constraint idiom, as everywhere).

## 2. Open, deliberately

Everything above the two ruled constraints: whether `sleep` itself is capability-gated
(delaying is not reading, but an ungated unbounded park is comptime-hostile and
scheduling-visible); literal or constructor syntax for durations; timer streams (a
`tick(interval)` producer would be an ordinary stream source under R102's
producers-produce-streams rule). This module is the named prerequisite of the top
post-alpha priority (concurrency §8) and should be designed together with it.
