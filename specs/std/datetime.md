# `std.datetime`

```luna
import std.datetime;
```

The calendar module: the **`datetime`** protocol (an immutable table carrying a
timestamp and a timezone), the **`timezone`** type, the **`weekday`** enum, and the
construction, arithmetic, and derivation surface over them. Wall-clock reading lives
here — `std.time` (R132) exiled it, keeping that module date-less — and `duration` is
the shared currency across the seam, exactly as R132's contract fixed.

Three commitments shape everything (R133):

- **Every datetime has a timezone.** There is no zoneless datetime and never will be —
  the lesson C# learned by failing (`DateTimeKind.Unspecified` is the acknowledged
  disaster that forced `DateTimeOffset` and NodaTime into existence). 
- **Immutable by grants.** `datetime` is a protocol whose members carry no `set` —
  interface immutability with zero new machinery (protocols §2.2); every operation
  derives a new value (the factory pattern, protocols §4.5), and COW value semantics
  already forbids aliased mutation underneath (tables §4).
- **No operators.** Comparison, difference, and arithmetic are named functions —
  `datetime` is a protocol table, operators are built-in-only (operators §1), and the
  names are clearer anyway (`difference`, `isBefore`, `addDays`).

## 1. The `datetime` protocol

```luna
export const datetime = proto {
  const get epochSeconds: int;     // seconds since 1970-01-01T00:00:00Z (unix semantics)
  const get nano: int;             // nanosecond-of-second, 0..999999999
  const get zone: @timezone;       // always present; never optional
};
```

- **Representation: two integer members plus the zone — about 24 bytes of protocol
  state, heap-backed like any table's** (the same size PHP lands on; a datetime is a
  protocol table, not an inline scalar). Seconds-since-epoch and nanosecond-of-second
  (the Go model) — full historical and future range with exact nanosecond precision. A single `duration`-since-epoch was rejected:
  int64 nanoseconds spans only 1678–2262, which amputates historical dates. `duration`
  stays the *arithmetic* currency (§5) without being the storage.
- **All three members are `get`-granted and `set`-less**: readable, immutable, and —
  by the one-boundary rule (protocols §5) — exactly the equality, serialization, and
  initializer surface. `==` therefore compares timestamp **and** zone: the same moment
  in Chicago and Tokyo is **not** `==` (strict same-type-same-value, as everywhere);
  the instant-level question is the named function **`sameMoment(a, b): bool`** —
  technically a different question, so it gets a different name.
- **Unix semantics; leap seconds do not exist** (everyone's answer). The calendar is
  **proleptic Gregorian**: dates before 1582 are computed as if Gregorian rules always
  applied — what every mainstream library means and rarely says. Other calendars may
  come later; none are promised.

## 2. `timezone`

```luna
export const timezone = proto {
  const get id: string;            // "America/Chicago", "UTC", "+05:30"
  const get isFixed: bool;         // true for fixed offsets, false for rule-carrying zones
};

export const zone = fn (id: string): @timezone! => {};          // IANA lookup; unknown id errors
export const offset = fn (d: duration): @timezone => {};        // fixed offset; out-of-range panics
export const localZone = fn () use (time): @timezone => {};     // the machine's zone: an effect

export const utc = offset(seconds(0));                          // the zero offset; `id` is "UTC"
```

- **`utc` is the zero offset, not a distinguished zone.** UTC *is* +00:00 — no rules, no
  DST, always zero — so it needs no origin of its own and is defined as `offset(seconds(0))`:
  `isFixed` is true, and it is pure and comptime-eligible like any other `offset`, which is
  what lets it serve as the default argument it is used as throughout this module. The zero
  offset **renders its `id` as `"UTC"`** rather than `"+00:00"`, which is why `"UTC"` appears
  in the `id` examples above; the two name the same value. (`seconds` is `std.time`'s, reached
  across the same seam as `duration` and the `time` capability — R132's contract, above.)
- **Zones and offsets are not the same thing, on purpose.** `zone('America/Chicago')`
  carries the full IANA ruleset and performs DST automatically; `offset(hours(-6))` is
  a frozen number with no rules. One `timezone` type, two origins, distinguished by
  `isFixed`. The trap to know: ISO 8601 text carries *offsets*, never zones, so parsed
  datetimes (§3) hold fixed-offset zones — calendar arithmetic on them is exact but
  DST-blind. `withZone` (§6) is the upgrade path.
- **The tzdb is bundled with the runtime** (the Go backend's `time/tzdata` embeds the
  IANA database, so this is nearly free). Consequence: `zone` is **pure and
  comptime-eligible** — `const chicago = zone('America/Chicago')` folds at build time,
  and a typo'd id is a *compile-time* error on the const path. Deterministic,
  portable, same-source-runs-or-compiles; staleness is accepted and refreshed with
  toolchain releases.
- **A `timezone` enum was considered and rejected** (R133): ~600 ids with renames and
  additions in nearly every tzdb release would make every update a breaking enum
  change, and zone ids arrive as runtime data (config, user records), which an enum
  cannot absorb without the string path existing anyway. Comptime folding already
  delivers the enum's one benefit — static checking — without its breakage.
- **`localZone` is an effect** — it reads the environment (`TZ`, `/etc/localtime`) —
  and sits under the **`time`** capability (std.time §4): one authority for the
  temporal environment (clock, sleep, local zone).

## 3. Construction

```luna
export const create = fn (year: int, month: int, day: int,
                          hour: int = 0, minute: int = 0, second: int = 0,
                          nano: int = 0, zone: @timezone = utc): @datetime! => {};
export const now = fn (zone: @timezone = utc) use (time): @datetime => {};
export const fromUnixSeconds = fn (n: int, zone: @timezone = utc): @datetime! => {};
export const parseDatetime = fn (s: string): @datetime! => {};
```

- **The default timezone is UTC, and the language decided it** (R133): reading the
  local zone is a capability-gated effect, so a *pure* constructor structurally cannot
  default to it. "Local" is always an explicit, visible opt-in:
  `create(2025, 6, 1, zone: localZone())` — the non-portability rides in the `use`
  clause where the audit can see it.
- **`create` is errorable** — datetimes are data-shaped and the failure surface is
  wide (Feb 30, month 13, nano out of range, a wall time that falls in a DST gap
  under a rule-carrying zone). Named arguments (R108) carry the optional fields.
- **`now(zone)`** reads the wall clock (the reading R132 exiled here), under the same
  `time` capability. `now(zone: localZone())` is the "what time is it here" spelling.
- **`parseDatetime`** accepts **ISO 8601 / RFC 3339** — the offset it carries becomes
  a fixed-offset `timezone` (§2's trap noted). Adopting (stealing) **PHP's format
  specification** for custom parsing and formatting is planned and **deferred** (§9):
  it is good, and it is a sub-spec of its own.

## 4. Components

Component access is the protocol's function surface, zone-resolved from the stored
timestamp (the state is two integer members; components are derived, not stored):

```luna
_ = dt->year();    _ = dt->month();   _ = dt->day();             // month is int 1..12
_ = dt->hour();    _ = dt->minute();  _ = dt->second();  _ = dt->nanoOfSecond();
_ = dt->weekday();                                               // the weekday enum value
_ = dt->dayOfYear();
```

```luna
export const weekday = enum { monday, tuesday, wednesday, thursday,
                              friday, saturday, sunday };
```

**Weeks are ISO: Monday is first.** Ruled once, globally, and it keeps this module out
of locale space entirely. **Months are `int` 1–12**, not an enum: month values are
arithmetic operands (`create(y, m + 1, 1)`) far more often than they are matched by
name; the `weekday` enum exists because the derivation family (§6) dispatches on day
*names*.

## 5. Arithmetic: two families, never one

Adding time to a datetime is **two different operations** — the split R132 committed
to when the duration ladder stopped at `hours`:

- **Absolute** — `add(dt, d: duration): @datetime`, `subtract(dt, d: duration)`:
  moves the *instant* by exact physics. Across a DST transition the wall-clock
  reading shifts; the moment is exactly `d` away. Total (overflow panics).
- **Calendar** — `addDays(dt, n: int): @datetime`, `addWeeks`, `addMonths`,
  `addYears`: moves the *calendar fields*, zone-aware. `addDays(dt, 1)` across
  spring-forward yields the same wall time tomorrow — 23 elapsed hours. Total, with
  three ruled policies:

  - **Month-end clamps**: Jan 31 + 1 month = Feb 28 (29 in leap years). The C#
    answer, ruled explicitly.
  - **DST gap: lenient shift-forward.** A result landing in a nonexistent wall time
    (2:30 AM on spring-forward day) shifts forward by the gap. No panic, no error —
    the time that "should" exist maps to the moment the clock actually showed next.
  - **DST overlap: earlier wins.** A result landing in a repeated wall time (1:30 AM
    on fall-back day) resolves to the **earlier** offset — the first occurrence in
    real time, the java.time / NodaTime convention. Overridable where it matters:
    `addDays(dt, 1, onAmbiguous: .{later})`, the policy visible by name (R106's
    discipline, R108's named arguments).

`difference(a, b): duration` is the exact elapsed time between two datetimes' instants
(signed, either order) — the operation that would have been `-`, clearer as a name.

## 6. Derivation and comparison

```luna
export const next     = fn (dt: @datetime, day: weekday): @datetime => {};   // strictly after dt
export const previous = fn (dt: @datetime, day: weekday): @datetime => {};   // strictly before dt
export const startOfDay   = fn (dt: @datetime): @datetime => {};
export const startOfWeek  = fn (dt: @datetime): @datetime => {};             // ISO: Monday
export const startOfMonth = fn (dt: @datetime): @datetime => {};
export const startOfYear  = fn (dt: @datetime): @datetime => {};
export const endOfDay     = fn (dt: @datetime): @datetime => {};             // last nanosecond
export const endOfMonth   = fn (dt: @datetime): @datetime => {};
export const withZone  = fn (dt: @datetime, z: @timezone): @datetime => {};  // same instant, re-zoned
export const isBefore  = fn (a: @datetime, b: @datetime): bool => {};        // instant order
export const isAfter   = fn (a: @datetime, b: @datetime): bool => {};
export const sameMoment = fn (a: @datetime, b: @datetime): bool => {};       // instant ==, zone-blind
```

"Monday next week" is `dt.startOfWeek().addWeeks(1)` — or `dt.next(.{monday})` when
"the coming Monday" is what is meant; the enum makes both typed where PHP's
`strtotime("monday next week")` is stringly. `isBefore`/`isAfter` compare instants
(the ordering `<` would have provided; operators are builtin-only); `withZone` is the
same moment re-expressed — `sameMoment(dt, withZone(dt, z))` is always `true`, and
`dt == withZone(dt, z)` is `false` unless `z` is `dt`'s own zone (§1's strict `==`).

## 7. Text

`toString(dt)` renders ISO 8601 with the offset (`"2025-06-01T14:30:00-05:00"`) —
exact, and `parseDatetime` round-trips it (into a fixed-offset zone, §2). This is the
serialization idiom too: **a datetime serializes like any protocol table** (R125 —
nothing here is a special case, exactly as any proto table needing a wire form uses a
named renderer), so the JSON convention is `format` / `toString` into the string field
you mean to emit:

```luna
let doc = toJson(['created' => toString(order->createdAt())]);
```

## 8. Capability summary

One capability, `time` (std.time §4), covers this module's two effects: `now` (wall
reading) and `localZone` (environment reading). Everything else — construction,
arithmetic, derivation, parsing, `zone` itself (bundled tzdb, §2) — is pure and
comptime-eligible. `const deadline = create(2026, 1, 1);` folds at build time.

## 9. Deferred, each with its reason

- **PHP's format specification** for custom parsing/formatting — planned adoption,
  deliberately separate (a mini-language is a sub-spec); ISO 8601 suffices for alpha.
- **Recurring datetimes / sequences** ("every second Tuesday") — a separate library,
  naturally stream-shaped (R102: producers produce streams); PHP's ecosystem does
  this externally too. Will be included; not now.
- **Floating (zoneless) civil values** — the one legitimate pressure is recurring
  local events ("9 AM alarm"); if it lands it will be a distinct `date` / `timeOfDay`
  component pair, **never** a zoneless `datetime`. The no-zoneless rule is absolute.
- **Locale-aware output** (month names, ordering, translations) — a future
  `std.locale`'s problem; this module refuses it wholesale (§4's ISO week rule is
  what keeps it out).
- **Non-Gregorian calendars** — possible future additions; nothing promised.

## 10. Open

Nothing blocking. The deferrals above are scoped and owned; the tzdb refresh cadence
(toolchain releases, §2) is an implementation policy, not a design question.
