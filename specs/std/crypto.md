# `std.crypto`

**Status: deferred, in full, deliberately (R140).** Cryptographic constructions —
CSPRNGs, hashing, ciphers, MACs, KDFs, constant-time comparison — are really tight
stuff, and poor decisions in a crypto surface last a very long time: an ecosystem
inherits its crypto API's mistakes for decades (the cautionary precedents are
everywhere — PHP's `mcrypt` era, early OpenSSL bindings). This record exists so the
absence is a decision on the ledger, not an oversight: the module will be designed as
its own dedicated effort, with the care the domain demands, not invented in passing.

What exists **today**, and is enough for the non-crypto program:

- **Secure random material**: `randomBytes(n) use (entropy): bytes` (std.random §2) —
  OS entropy, never the statistical PRNG (whose slot is structurally non-secure,
  std.random §3.1). Session ids, tokens, nonces: covered.
- **Containment**: the `secret` type (secret spec) — capability-gated reveal,
  concealed display and serialization, gate-constraints for typed authority
  (`dbSecret`). Key *handling* discipline exists before key *generation* does.
- **The escape hatch**: `exec` running real tools, for the rare alpha program that
  must hash or encrypt today.

Constraints already fixed by existing rulings, which any future design inherits:
effects will be capability-gated (and pure constructions comptime-eligible only where
determinism is sound); key material is `secret`-shaped, never bare `bytes` in
signatures that keep it; the statistical/secure split is permanent — nothing in
`std.crypto` will ever be seedable-for-replay, and nothing in `std.random` will ever
claim security (the R140 seed theorem).
