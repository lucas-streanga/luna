# `std.io` errors

The declarable error hierarchy behind `openFile`'s `file!` (std.io §4), grounded in the
Linux syscall `errno` surface, the compiler's sole target for now is `linux-x86-64`
(compiler §0), so the grounding is concrete rather than portable-abstract. The **types** are
the stable, portable surface; the `errno` numbers are the platform detail behind them.

## 1. The mapping principle

Errno values do not map 1:1 onto error types. The rule is the one errors §5.2 already fixed:
**declare a type exactly when a handler will catch it by name.** So errnos group by *what the
caller can do about it*, and every type carries the raw number for diagnostics, never for
dispatch:

```luna
export const ioError = error { path: path?; errno: int?; };   // the family root
```

`path` is the path involved (`null` where none applies); `errno` is the raw value for logs
and bug reports. Handlers branch by **type** (`catch (e: fileNotFound)`); an `errno`-switch in
user code is a design smell the hierarchy exists to prevent.

## 2. The hierarchy

```luna
export const fileNotFound     = error : ioError {};   // ENOENT, ENXIO, ENODEV: nothing is there
export const notADirectory    = error : ioError {};   // ENOTDIR: a path component is the wrong kind
export const isADirectory     = error : ioError {};   // EISDIR: opened a directory for writing
export const permissionDenied = error : ioError {};   // EACCES, EPERM, ETXTBSY: not allowed
export const readOnlyTarget   = error : permissionDenied {};   // EROFS: a write on a read-only fs
export const alreadyExists    = error : ioError {};   // EEXIST: exclusive-create found a file
export const directoryNotEmpty = error : ioError {};  // ENOTEMPTY: non-recursive delete of a full dir
                                                      //   (std.filesystem §3.3, R135)
export const invalidPath      = error : ioError {};   // ENAMETOOLONG, ELOOP: filesystem-relative
                                                      //   invalidity the static `path` constraint
                                                      //   (std.filesystem §1, R135) cannot see
export const tooManyOpenFiles = error : ioError {};   // EMFILE, ENFILE: descriptor exhaustion
export const outOfSpace       = error : ioError {};   // ENOSPC, EDQUOT, EFBIG: the device or quota
export const connectionRefused = error : ioError {};  // ECONNREFUSED (std.net §4, R143)
export const connectionReset  = error : ioError {};   // ECONNRESET, EPIPE on a dead peer
export const hostUnreachable  = error : ioError {};   // EHOSTUNREACH, ENETUNREACH
export const addressInUse     = error : ioError {};   // EADDRINUSE
export const dnsError         = error : ioError {};   // name resolution failed (no errno; the resolver's own class)
```

Grouping notes: `fileNotFound` folds the device-absent errnos (`ENXIO`, `ENODEV`) because a
handler treats "the thing is not there" one way regardless of which kernel path noticed;
`readOnlyTarget` is a child of `permissionDenied` so a broad permissions handler catches it
while a precise one can distinguish it; `invalidPath` is the *runtime* complement of the
`path` constraint, `ELOOP` and `ENAMETOOLONG` depend on the live filesystem (symlink chains,
per-fs limits), which no string predicate can know.

## 3. The four fates of an errno

The hierarchy above is only one of four destinations, and the split is the language's error
categories (errors §2, std.io §8) digesting POSIX:

| Fate | Errnos | Why |
|-|-|-|
| **Declarable** (§2, at `openFile`) | `ENOENT ENXIO ENODEV ENOTDIR EISDIR EACCES EPERM ETXTBSY EROFS EEXIST ENAMETOOLONG ELOOP EMFILE ENFILE ENOSPC EDQUOT EFBIG` | expected, environmental, anticipatable at the one boundary where failure is routine |
| **panic, existing subtree** | `ENOMEM` | already `outOfMemory` in the `panic` tree (errors §2); io adds nothing |
| **panic, environmental mid-operation** | `EIO`, and write-side `ENOSPC`/`EDQUOT`/`EPIPE` surfacing at `write`/`flush`/`close` | the mid-stream ruling (std.io §8): no error channel in a stream, rare, boundary-caught; the write-side revisit stays flagged (std.io §9) |
| **Absorbed by the runtime** | `EINTR`, `EAGAIN`/`EWOULDBLOCK` | the Go runtime restarts interrupted syscalls and the green-thread scheduler parks a task on would-block; user code never sees either |
| **Impossible by construction** | `EFAULT`, `EBADF`, mode/flag `EINVAL` | `EFAULT` cannot arise in a memory-safe language; `EBADF` is use-after-close, caught at the Luna layer as a misuse panic before any syscall (std.io §4); flag combinations are unrepresentable through the typed API, so a residual `EINVAL` is a runtime bug and panics as one |

(Five fates counting "impossible"; the table keeps them together because an implementer
needs the whole partition in one view: every errno the target can return from `open`,
`read`, `write`, `close`, or `lseek` has exactly one row.)

## 4. Open questions

- **Write-side declarability**: whether `outOfSpace` at `flush`/`close` should become a
  declarable arm (`flush: undefined!`) instead of the mid-operation panic, the standing
  revisit from std.io §9; this hierarchy is ready either way, the types exist.
- **Portability**: when targets grow beyond `linux-x86-64` (compiler §0), the type surface
  is the contract; whether `errno: int?` stays raw, becomes platform-tagged, or is dropped
  from non-POSIX targets is deferred with `std.platform`.
- *(**`interrupted` as policy: resolved by R115/R121.** The cancellation story arrived,
  and `EINTR` absorption is unaffected — they are separate mechanisms: the runtime
  restarts interrupted syscalls transparently, and *separately* checks cancel-pending at
  the park, delivering `cancelled` refused-on-entry (concurrency §6.1). User code sees
  neither an `EINTR` nor a torn operation.)*
