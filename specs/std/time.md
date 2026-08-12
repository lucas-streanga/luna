# `std.time`

```luna
import std.time;
```

The time module: two built-in types (`duration`, `instant`), one monotonic clock, and
`sleep`. One exported capability, **`time`**, gates the two effects (reading the clock,
waiting); everything else here is pure data. The module is **date-less by design** (§6):
no wall clock, no calendar, no time zone — those are `std.datetime`'s (R133), and
their absence here is what kills the classic monotonic-versus-wall conflation bug
*structurally*: there is no wall type in scope to conflate. This module supersedes the
R120 deferral record it replaces (R132) and is the named prerequisite of the timeout /
`awaitAny` surface (concurrency §8), which sits directly on §5's `sleep`.

---

## 1. Two built-in types: `duration` and `instant`

`duration` (a signed span of time) and `instant` (a point on the monotonic clock) are
**built-in types**, not constraints and not tables. This is forced, and worth stating
because the R120 record leaned the other way ("the constraint idiom, as everywhere"):
the essence of a chrono-style time library is **dimensional safety** — a duration is
not a number, an instant is not a duration, `instant + instant` must not compile — and
Luna's only route to operator semantics is the **built-in-only operator rule**
(operators §1): no generics, no operator overloading. A constraint over `int` erodes on
first use (`byte + byte` widens to `int`, so a constraint-`duration` forgets itself
after one addition) and can forbid nothing dimensional. So the rule becomes the
feature: only built-ins get operators — make them built-ins. The tower already reserves
exactly this path: adding built-in types is a breaking change that lands **before the
1.0 stability commitment**, and `match` exhaustiveness and introspection are required
to tolerate the universe growing (numeric-tower §6). Both types report `{scalar}`
(introspection §4.3, the as-they-land clause) and are predeclared identifiers
(keywords §5).

**Representation: 64-bit signed nanoseconds.** One inline word each (efficient storage,
no allocation), a span of ±292 years — beyond any monotonic-uptime or timeout need.
Arithmetic **overflow panics** (`overflowError`), never wraps, like every Luna integer
(int §2). Durations are **signed**: `earlier - later` is a negative duration, and
negation is total (with the one `overflowError` at the minimum value, the int
precedent).

An `instant` is meaningful only **relative to other instants from the same process**:
the epoch is arbitrary (boot-ish), readings never go backward, and wall-clock
adjustments never touch it. Consequences: instants order and subtract, and nothing
else; and an instant has **no data representation that survives the process**, so
`toJson` on an `instant` raises `typeError` (the `fn` precedent, json §2.1). A
`duration` serializes as its canonical string (§3) — exact, and round-trippable through
`parseDuration` at the reader.

## 2. The dimensional operator table

The complete operator surface of the two types. Anything not in this table is a
**compile error** — that is the point:

| Expression | Result | Notes |
|-|-|-|
| `duration + duration`, `duration - duration` | `duration` | overflow panics |
| `-duration` | `duration` | signed; `overflowError` at minimum |
| `duration * int`, `int * duration` | `duration` | scaling; overflow panics |
| `duration / int` | `duration` | truncates toward zero; `divisionByZero` |
| `duration / duration` | `int` | the dimensionless ratio, truncated; `divisionByZero` |
| `duration % duration` | `duration` | remainder; alignment idioms; `divisionByZero` |
| `instant - instant` | `duration` | elapsed; either order (signed) |
| `instant + duration`, `duration + instant`, `instant - duration` | `instant` | overflow panics |
| `==`, `!=`, `<`, `<=`, `>`, `>=` | `bool` | on `duration`×`duration` and `instant`×`instant`; strict, no coercion |

The compile errors are the safety: `instant + instant` (points do not add),
`duration * duration` (time squared does not exist), `int + duration` and every other
bare-number mixing (a number is not a span — construct one, §3), `instant` with any
numeric type, `duration` with `double` (scale by `int`; fractional scaling is a policy
question deferred with `std.datetime`'s needs). Equality and ordering are strict and
same-type, as everywhere (equality §1).

## 3. Constructors, extractors, text

```luna
export const nanoseconds  = fn (n: int): duration => {};
export const microseconds = fn (n: int): duration => {};
export const milliseconds = fn (n: int): duration => {};
export const seconds      = fn (n: int): duration => {};
export const minutes      = fn (n: int): duration => {};
export const hours        = fn (n: int): duration => {};

export const wholeNanoseconds  = fn (d: duration): int => {};
export const wholeMicroseconds = fn (d: duration): int => {};
export const wholeMilliseconds = fn (d: duration): int => {};
export const wholeSeconds      = fn (d: duration): int => {};
export const wholeMinutes      = fn (d: duration): int => {};
export const wholeHours        = fn (d: duration): int => {};

export const parseDuration = fn (s: string): duration! => {};
```

- **Constructors are total** and pure (comptime-eligible: `const retryDelay =
  seconds(30)` folds); an argument whose span exceeds the representation panics with
  `overflowError` (constraint-entry style, so no `!`, conversion §2's exemption).
  The unit ladder **stops at `hours`**: a "day" is a calendar claim (leap seconds,
  DST), and calendars live in `std.datetime`; writing `hours(24)` states exactly what
  is meant.
- **Extraction names its loss.** `duration → int` per unit truncates toward zero, so
  the family says so in the word: `wholeSeconds(milliseconds(1500))` is `1`
  (`wholeNanoseconds` is exact by representation). There is deliberately no
  `toSeconds`: one name has one signature (functions §3.4), the constructors own the
  unit names, and a bare `to*` would hide the truncation the `whole*` spelling
  states (the R106 discipline).
- **Text round-trips.** `toString` renders a duration in Go-style compound units,
  exact (`"2h45m0.5s"`, `"-150ms"`, `"0s"`); `parseDuration` (the `parse*` contract,
  conversion §2) accepts everything `toString` emits plus simple unit forms
  (`"250ms"`, `"1h30m"`), and errors on text with no duration reading. `toString` on
  an `instant` renders an opaque diagnostic form (epoch-relative nanos), fine for
  logs, meaningless across processes (§1).

## 4. The clock

```luna
export const time = capability;
export const now = fn () use (time): instant => {};
```

**One clock: monotonic.** High resolution is a quality of implementation (nanosecond
representation; the finest the platform gives), **not a second clock** — a second
clock would mean either a second instant type or cross-clock subtraction that
compiles and lies. Readings never decrease and are immune to wall-clock adjustment.
Elapsed time is subtraction:

```luna
let started = now();
work();
let elapsed = now() - started;        // duration
```

**The capability is a theorem, not a taste** (R43, capabilities §10): comptime
eligibility is "requirement set is empty," every ineligibility source is a capability,
and a clock read must be comptime-ineligible — a build must not depend on when it
runs — so the clock *must* carry one. A corollary the gate buys for free: functions
that take `now: fn (): instant` as a parameter need no capability, so virtual time in
tests is the path of least resistance, not a mocking framework.

## 5. `sleep`

```luna
export const sleep = fn (d: duration) use (time): undefined => {};
```

- **A suspension point, always.** Cancellation delivers here (cooperative,
  refused-on-entry, concurrency §6.1, R115); a zero or negative duration returns
  immediately but **still suspends**, which makes `sleep(seconds(0))` the portable
  yield point.
- **Sleeps at least `d`**, measured on the monotonic clock; scheduling may add,
  never subtract. `EINTR` is absorbed by the runtime and re-sleeps the remainder
  (the R121 stance; interruption is not cancellation).
- **Gated by `time`**: an ungated unbounded park would be comptime-hostile and
  scheduling-visible (the R120 record's own worry, now closed by the same R43
  argument as the clock).
- **The timer half of the timeout surface — which landed** (R142): "await with
  deadline" is a race against `sleep`, and `awaitAny` / `timeout` / `awaitTimeout` /
  `receiveTimeout` (concurrency §5.1) sit directly on this export, exactly as
  designed.

## 6. Deliberately absent

- **Wall-clock time, dates, calendars, time zones** — `std.datetime` (landed, R133).
  The seam held as designed: `duration` is the shared currency; `datetime` stores
  seconds+nanos and differences into ordinary durations; `now`-for-datetimes and
  `localZone` live there under this module's `time` capability; nothing date-shaped
  enters this module. Nothing was lost by the exile — a program that wants "what time
  is it" wants a calendar answer, which was never this module's question.
- **`day` and larger units** — calendar claims (§3).
- **A second (high-resolution/raw) clock** — a quality knob, not a type (§4).
- **`tick(interval)` / timer streams** — an ordinary stream producer under R102,
  trivially buildable once timeouts land; deferred to the stdlib patterns layer
  (R120) so its shape can follow the timeout surface's, not precede it.
- **Deadline objects, stopwatches** — idioms over `now`/`sleep`, patterns layer.

## 7. Open

Nothing in this module. The two consumers it unblocks are tracked at their homes: the
**timeout / `awaitAny` / select surface** (concurrency §8 — the named top priority,
now unblocked), and `std.net` behind it (net.md). `std.datetime` **landed** (R133) on
§6's seam, exactly as contracted.
