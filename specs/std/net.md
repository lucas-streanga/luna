# `std.net`

```luna
import { egress, ingress, port } from std.net;
const net = import std.net;
```

The socket module (R143): TCP and UDP primitives under two capabilities, closing the
alpha standard library. Two principles are the module's shape:

- **Zero timeout parameters, anywhere.** Every net operation is a blocking call and
  therefore a suspension point (R121), so every wait is bounded by the existing R142
  family — `timeout(fn () => dial(host, p), seconds(5))`, `receiveTimeout(datagrams(s),
  d)` — and this module threads no deadline through any signature. (Go threads
  `SetDeadline` through its entire net API because its timeouts do not compose; Luna's
  do. This is the R142 payoff, collected.)
- **Alpha net is plaintext, stated loudly.** TLS requires `std.crypto` (deferred
  deliberately, R140), so the dependency chain is fixed and honest: **crypto → tls →
  http** (§6). Nothing here encrypts; credentials sent over these sockets travel as the
  bytes you wrote.

## 1. Two capabilities: `egress` and `ingress`

```luna
export const egress  = capability;    // may originate connections and datagrams
export const ingress = capability;    // may bind ports and accept
```

"May phone home" and "may open listening ports" are different authorities (the R134
separate-authorities argument, again): a CLI tool declaring `use (egress)` visibly
*cannot* bind a port, and a server's `use (ingress)` is its complete inbound audit —
the distinction every firewall draws, drawn where Luna draws everything, in the `use`
clause. Splitting later would be a breaking change; splitting now costs two good names.
(R121's provisional single `net` capability is superseded, R143.) As everywhere: the
capabilities gate **establishment**; moving bytes on an established connection is
**io's** authority — the R134 symmetry (`filesystem` : structure :: `egress`/`ingress`
: connections :: `io` : contents).

## 2. TCP

```luna
export const dial   = fn (host: string, p: port) use (egress): connection!;
export const listen = fn (p: port) use (ingress): listener!;
export const connections = fn (l: listener): stream!;    // the accept stream
```

- **`dial`** connects; `host` is a name or an IP literal, and resolution happens inside
  (a name that does not resolve is `dnsError`, §4). Bound like any wait:
  `timeout(fn () => dial(host, p), seconds(5))`.
- **A `connection` wears `fileDescriptor`** (its proto requires it, protocols §7), so
  **io's entire byte surface works with no new functions**: `chunks(conn)` to read (a
  non-restartable stream — a socket is the canonical non-replayable source),
  `write(conn, data)`, `close(conn)`, `defer close(conn)` — all under `use (io)`, all
  per R121's referent-stateful, no-`&` convention. `connection`'s own members are the
  socket facts (`conn->peerHost()`, `conn->peerPort()`), `identityEquality`,
  single-owner, transferred crossing class.
- **Accept is a stream** — the three-line server:

  ```
  let l = listen(8080 as port);
  foreach (conn in connections(l)) {
    spawn handle(conn);            // handlers scope-bounded here (concurrency §6)
  }
  ```

  `connections(l)` is an ordinary producer stream (R102): lazy, creation-authorized
  (the listener carried the `ingress` grant, R121's laundering-theorem rule),
  non-restartable, bounded like any wait by `receiveTimeout`. Structured lifetime
  scopes every handler; a failing accept surfaces on the stream's pull (io §8's
  convention).

## 3. UDP

```luna
export const udpBind   = fn (p: port) use (ingress): udp!;
export const send      = fn (sock: udp, host: string, p: port, data: bytes) use (egress): undefined!;
export const datagrams = fn (sock: udp): stream!;
```

Connectionless, so the datagram — not a byte stream — is the unit: `datagrams(sock)`
yields rows `['from' => host, 'port' => p, 'data' => bytes]`, one per packet, with the
usual stream discipline. A `udp` socket is the same handle class as `connection`
(identityEquality, single-owner, `close(sock)`, `defer`). Note the capability shape:
one socket can need both authorities — `udpBind` is `ingress`, `send` is `egress` — a
request/reply UDP program declares `use (ingress, egress)` and each word means itself.

## 4. `port`, and the error family

```luna
export const port = constraint p: int where p >= 1 && p <= 65535;
```

The constraint that has been the corpus's running example since R9 is now real, and
this module owns it.

Errors **extend the `ioError` family** (the R135 rule: errors classify *what failed*
at the OS boundary, orthogonal to module and capability boundaries): five arms join
io-errors' declaration block — `connectionRefused` (ECONNREFUSED), `connectionReset`
(ECONNRESET, EPIPE on a dead peer), `hostUnreachable` (EHOSTUNREACH, ENETUNREACH),
`addressInUse` (EADDRINUSE), and `dnsError` (resolution failure — not an errno; the
resolver's own failure class). All declarable: network failure is expected, recoverable,
data-shaped — the `timeoutError` argument, and the same handling (`try dial(…)`).

## 5. What composes for free

- **Backpressure**: streams are pull-based — a slow consumer slows the socket; no
  buffering-policy surface exists because none is needed.
- **Cancellation**: every park (dial, read, accept, receive) is a suspension point —
  a task blocked on a dead peer is cancellable, refused-on-entry (concurrency §6.1),
  and the R142 contract holds verbatim: timeout bounds waiting, never execution.
- **Credentials are secrets** (secret spec), revealed at the connection boundary —
  the exec pattern, unchanged.

## 6. Deferred, and the chain that orders it

- **TLS** — gated on `std.crypto` (R140), which is deferred deliberately; the chain
  **crypto → tls → http** is the recorded order, and no link ships before the one
  under it. Alpha net is plaintext (the header's loud statement).
- **`std.http`** — deferred by decision (R143): a whole other beast, naturally atop
  net, its secure half gated two modules deep by the chain above.
- **Socket options** (nodelay, keepalive, buffer sizes), **unix domain sockets**,
  **half-close/shutdown**, **explicit IPv6 surface** (dial and listen handle both
  address families transparently; explicit control is deferred) — each waits for
  real use.
