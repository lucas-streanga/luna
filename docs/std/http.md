# `std.http`

**Status: deferred, by decision (R143).** HTTP is a whole other beast than sockets —
methods, headers, bodies, redirects, connection pooling, a client *and* a server story —
and it builds naturally on `std.net` without being it. This record exists so the
absence is a decision on the ledger, and so the ordering is fixed:

- **The chain is crypto → tls → http** (net §6, crypto's record R140): the secure half
  of HTTP is gated two modules deep, and no link ships before the one under it. An
  http module without https would be a footgun shipped as a convenience; it waits.
- **What exists today**: `std.net`'s TCP surface (dial, the accept stream, io's byte
  functions on connections) is sufficient to hand-speak HTTP/1.1 for the program that
  truly must, and `exec` running `curl` is the honest escape hatch for the rest.
- **Constraints any future design inherits**: capability story rides net's
  `egress`/`ingress` (no new authority — HTTP is a protocol, not a new reach);
  timeouts stay the R142 family (zero deadline parameters, net's own principle);
  bodies are streams (R102); credentials and tokens are `secret`-shaped.
