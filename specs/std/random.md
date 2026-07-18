# `std.random`

```
import { entropy } from std.random;
const rand = import std.random;       // or collect the function surface (modules §6)
```

The randomness module, built on one idiom: **seeding is the effect, generation is
pure.** One capability-gated entropy read at the top of a program; a pure,
deterministic stream everywhere after. The consequence is a feature no bolted-on RNG
gets: **every randomized run is replayable by logging one int** — the seed.

```
let rng = randomStream();             // gated once, visibly: use (entropy)
shuffle(deck, rng);                   // pure from here on
let roll = nextInt(rng, 1, 6);
```

## 1. The two findings that fixed the shape (R139)

- **The ungated default was unsound.** The catalogue's `random(it, …, randFn?, …)` and
  `shuffle(tab, randFn?)` defaulted their randomness source to nothing specced — and an
  ungated nondeterministic default breaks the R43 theorem (every ineligibility source
  is a capability): a capability-free `shuffle(t)` would be comptime-eligible and fold
  a "random" order into the binary. The parameter is now **required** (§5).
- **A function-shaped PRNG is unimplementable, so the shape was never a choice.** A
  PRNG carries mutable state between calls, and Luna closures capture a **`const`
  snapshot** (functions §2.1) — a stateful counter closure cannot exist. Stateful
  sequence generation has exactly one home in this language: the **generator**, which
  is to say a **`stream`**. `randFn` was mis-typed the day it was written; the PRNG is
  a stream because streams are what stateful functions *are* here — and a stream is
  O(1) state besides.

## 2. The gated half: `entropy`

```
export const entropy = capability;
export const randomSeed   = fn () use (entropy): int;
export const randomBytes  = fn (n: int) use (entropy): bytes;
export const randomStream = fn () use (entropy): stream;    // ≡ prng(randomSeed())
```

- **The capability is `entropy`, not `random`** — deliberately: the catalogue's element
  picker is already named `random`, and the authority is precisely the right to read
  the system entropy source.
- **`randomBytes` is the secure path** — straight from OS entropy, never the PRNG —
  for session ids, tokens, key material (wrap the result `as secret` where it is one).
- **`randomStream` is the everyday spelling**: one call, genuinely seeded, the
  capability visible in the signature. It exists because the convenient alternatives
  do not survive scrutiny (§6).

All three are ineligible for comptime *because* they are gated — the R43 fix landing
exactly where it should: a build cannot depend on entropy it did not declare.

## 3. The pure half: `prng` and the `next*` family

```
export const prng = fn (seed: int): stream;    // infinite, pure, deterministic

export const nextInt    = fn (rng: stream, lo: int, hi: int): int;  // inclusive, unbiased
export const nextDouble = fn (rng: stream): double;                 // [0, 1), 53-bit
export const nextBool   = fn (rng: stream): bool;
```

- **The algorithm is pinned and part of the contract: PCG-64** — Go `math/rand/v2`'s
  *seedable* engine, so the backend is free. (Precision matters here, R140: Go v2's
  auto-seeded *default* is ChaCha8, chosen to harden the inevitable misuse of a rand
  API that predates the secure/statistical split. Luna has no such legacy pressure —
  this module *is* the split, and §3.1's seed theorem bounds what hardening could buy
  anyway — so PCG keeps the role it is actually good at.) Pinning is not an
  implementation detail: if `prng(42)`'s output changed between toolchain releases,
  comptime-folded values and seeded tests would silently change; because the pin is
  contract, any future engine change is a visible, versioned break (the pre-1.0
  window), never silent drift. One algorithm, **no engine zoo** — PHP's menagerie
  (Mt19937, Pcg, Xoshiro, Secure) is compatibility history this module does not
  inherit; the whole zoo compresses to one pinned deterministic engine plus one
  secure source (§2).
- **`next*` consumes from the stream** — visibly, since streams are by-reference.
  `nextInt` maps to the range without modulo bias (rejection sampling, which may
  consume more than one raw value); nobody writes `n % 6` and ships dice that roll
  low. The raw stream yields full-range `int`s for the rare consumer that wants them.
- **`prng` with a fixed seed is comptime-eligible**: reproducible test data and
  property-based inputs can fold at build time — deterministic by construction, so
  there is nothing to gate.

### 3.1 `prng` is never for secrets — and no engine could make it so

PCG-64 is a *statistical* generator with **zero cryptographic strength**, by design
and by its author's statement — and the prediction question is settled, not open:
practical full-state recovery from a handful of outputs is published (Bouillaguet,
Martinez & Sauvage, 2020). An attacker who sees `prng` outputs predicts all future
ones. But the deeper fact is **the seed theorem** (R140): `prng(seed: int)` has a
64-bit seed, so *no* algorithm in this slot could be secure — an attacker
brute-forces 2⁶⁴ seeds offline regardless of the engine; swap in a cipher and the
seed search, not the cipher, is the attack. And this module's headline feature —
every run replayable from one logged int — is precisely the property a security RNG
must never have. The slot is structurally non-secure, which is why security lives
elsewhere by construction: **`randomBytes`** (§2, OS entropy, gated) for secure
material today, and **`std.crypto`** (deferred, its own record) for cryptographic
constructions when they are designed with the care they demand.

## 4. Restart is replay; reseed is a new stream

Two operations that must not share a spelling:

- **`restart(rng)` replays.** R105's rule — restartable iff the stream can re-run from
  an **immutable snapshot of its source** — and a PRNG's source *is its seed*, an
  immutable int. `prng` is not just restartable; it is the **exemplar** the rule was
  made for (a range restarts from `lo`; a PRNG restarts from its seed). This holds for
  `randomStream()` too: its snapshot is the seed drawn at creation, so restarting
  replays *that run's* sequence — restart the rng, replay the exact randomized
  execution. A debugging gift.
- **Reseeding is rebinding.** Fresh randomness is `rng = randomStream();` — an
  ordinary new stream. No in-place reseed operation exists: a stream never mutates its
  own source, and "same sequence again" and "different sequence now" stay different
  spellings.

## 5. The catalogue respec (swept with this ruling)

`random` and `shuffle` now **require** their randomness source, typed as the stream it
must be:

```
fn random(it: iterable, rng: stream, num: int = 1, preserveKeys: bool = true): table
fn shuffle(tab: table, rng: stream): table
```

Optional-with-a-default was the unsoundness (§1): a pure catalogue function cannot
conjure entropy, and its `use` set cannot be conditional on an argument's absence.
Shuffling without a stated randomness source was always an incoherent ask; the
signature now says so, and the call is one argument away: `shuffle(deck, rng)`.

## 6. The seed default: considered and rejected

`prng(seed: int = …)` was considered for convenience and rejected — the record, since
the question will recur:

- **An entropy-read default is structurally impossible**: it would make `prng` require
  `use (entropy)` only when the argument is omitted, and a function's capability set
  is fixed, never conditional on argument presence (the same argument as §5); default
  expressions are comptime-known besides (functions §3.3.1).
- **A constant default is the C `rand()` footgun, verbatim**: `prng()` would yield the
  identical sequence in every program on every run — the classic "why does my shuffle
  deal the same hands" bug, shipped as the convenient spelling. Safe by construction
  forbids documented footguns.
- **`randomStream()` is the synthesis** (§2): one call, genuinely random, capability
  visible. The convenience without the trap.

## 7. Not here

Distributions (normal, exponential) and sampling utilities beyond the catalogue's
`random` — library territory, post-alpha, buildable on `prng`/`nextDouble` without
new machinery. **Everything cryptographic belongs to `std.crypto`** (deferred in
full, its own record, R140): `randomBytes` (§2) covers secure *material* today;
constructions — CSPRNGs, hashing, ciphers, KDFs — are deliberately not invented
here, because crypto surfaces are really tight stuff and poor decisions last a long
time.
