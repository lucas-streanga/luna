# `std.filesystem`

The filesystem-structure module (R135): the `path` constraint, the pure path
operations, and the metadata surface — probing, enumerating, creating, and destroying
filesystem entries — under the **`filesystem`** capability (renamed from `system` by
R134; the io boundary stands: contents are `io`'s, structure is this module's).

**The canonical import is two lines**, one per kind of thing, each forced by an
existing ruling:

```luna
import { filesystem, path } from std.filesystem;   // capability (use clauses) + type (annotations)
const fs = import std.filesystem;                   // the function surface, namespaced (R136)
```

Function names like `delete`, `copy`, `join`, `exists` are too valuable to dump bare,
so the *importer-collects* idiom (modules §6) is the documented convention: `fs.delete(p)`,
`fs.join(dir, name)`. The capability cannot ride in the table — capability tokens never
inhabit value slots (capabilities §3.1), and a `use` clause names *bindings* (R19) — so
it arrives bare; and annotations take type *names*, not element accesses, so `path`
arrives bare too (type expressions admit names and type operators, never `table.member`
— pinned against type.md if ever contested). Scripts that prefer bare names use
selective import and aliasing as anywhere (modules §8: `import { join as joinPath } …`).

## 1. `path`

```luna
export const path = constraint p: string where isValidPath(p);
```

Relocated from io §2.0 (R135; `std.io` now imports it — the DAG is clean). Paths are
not bare strings, on the `json` precedent: an entry into a `path` position validates
once, at the boundary, and everything downstream trusts it. **`isValidPath` is this
module's export** (R138, terminating the io → filesystem → platform deferral chain):
its body comptime-branches on `platform.os` (std.platform §4), and for the sole
current target the rule is **nonempty, no NUL byte, at most 4096 bytes**; other
targets add branches when they land. Relative paths resolve against the working directory **at process start**,
always — there is no `chdir` (std.process §3, R134).

## 2. The pure half: path operations

```luna
export const join      = fn (base: path, ...parts: string): path => {};
export const dirname   = fn (p: path): path => {};
export const basename  = fn (p: path): string => {};
export const extension = fn (p: path): string? => {};    // null when there is none
export const normalize = fn (p: path): path => {};       // collapses '.', '..', '//'
```

Path manipulation is string math: no capability, comptime-eligible (`const cfg =
join(configDir, 'app.json')` folds) — the pure-data-half / gated-effect-half shape
`std.time` and `std.datetime` established. `extension` yields `null` for
extension-less names (absence is absence; coalesce where a string is wanted:
`extension(p) ?? ''`). `join` is a variadic (functions §3.3.3).

## 3. The gated half

Every function below is `use (filesystem)`: non-comptime by construction (the R43
theorem — the capability *is* the ineligibility), and every blocking call is a
suspension point (concurrency §6.1).

### 3.1 Probing

```luna
export const exists = fn (p: path) use (filesystem): bool => {};
export const stat   = fn (p: path) use (filesystem): @fileInfo! => {};

export const fileInfo = proto {
  const get size: int;              // bytes
  const get modified: @datetime;    // UTC (std.datetime, R133)
  const get kind: entryKind;
};
export const entryKind = enum { file, directory, symlink, other };
```

- **`exists` is the probe form** (the `canReveal` precedent): it never errors — `true`
  iff `stat` would succeed, so `false` covers absent *and* inaccessible alike. And it
  is **advisory**: check-then-open is a race the filesystem will not referee (TOCTOU);
  the open's or stat's own error is the truth. Probe for flow control, never for
  safety.
- **`stat` follows symlinks** and returns a get-only `@fileInfo` (the datetime
  immutability pattern). Permissions are deliberately absent from `fileInfo` — the
  permission *model* (mode bits, ownership) is deferred whole (§5), so this module
  neither reads nor writes it.

### 3.2 Enumerating

```luna
export const entries = fn (dir: path) use (filesystem): stream! => {};   // immediate children
export const walk    = fn (dir: path) use (filesystem): stream! => {};   // recursive descent
```

Both are stream producers (R102; a million-entry directory costs nothing to start),
yielding **`path` values** with implicit keys — paths only, by ruling: the smallest
surface, composing with `stat` where more is wanted (the runtime may cache entry-type
internally; that is its business). The streams obey io's lazy-effect law unchanged —
**authority is checked at creation of the carrier** (capabilities §3.1's generalized
laundering theorem, R121) — and are **non-restartable** (the file-stream precedent:
re-listing is re-calling). **`walk` does not follow directory symlinks** — cycle
safety is not optional in alpha; the C++-style follow option can come with the symlink
axis (§5).

### 3.3 Creating, destroying, moving

```luna
export const createDir = fn (p: path, recursive: bool = false) use (filesystem): undefined! => {};
export const delete    = fn (p: path, recursive: bool = false) use (filesystem): undefined! => {};
export const rename    = fn (from: path, to: path) use (filesystem): undefined! => {};
export const copy      = fn (from: path, to: path) use (filesystem): undefined! => {};
export const tempDir   = fn (prefix: string = '') use (filesystem): path! => {};
export const tempFile  = fn (prefix: string = '') use (filesystem): file! => {};
```

- **`createDir(p, recursive: true)`** is `mkdir -p`; the bare form errors on a missing
  parent (`fileNotFound`) or an existing entry (`alreadyExists`).
- **`delete`** removes files and *empty* directories (remove(3) semantics; a non-empty
  directory is `directoryNotEmpty`). **`delete(p, recursive: true)` is `rm -rf`** —
  the most destructive call in the standard library, and R108's named argument makes
  it read as loudly as it should at every call site. There is no separately-named
  `deleteAll` to reach for by accident.
- **`copy` is file-only** in alpha; recursive tree copy is deferred (§5) rather than
  half-specified.
- **`tempDir` / `tempFile`** create uniquely-named entries in the platform temp
  location — the primitive tests.md's per-test-resources deferral has been waiting on.
  The unique names need entropy, and that is sound *here*: inside an already-gated
  effectful function, internal nondeterminism is as lawful as `now()`'s (the
  `filesystem` capability already makes callers ineligible for comptime). `tempFile`
  returns the opened `file` (io §2), not a path — handing back a path would be a
  TOCTOU invitation (§3.1).

## 4. Errors: the `ioError` family, extended

Filesystem failures are errno-shaped exactly like io's, so this module **extends the
existing `ioError` family** (io-errors.md is explicitly small and extensible) rather
than minting a parallel tree: `fileNotFound` and `permissionDenied` are **reused**;
**`alreadyExists`** and **`directoryNotEmpty`** are added under the same root. Errors
classify *what failed* (the OS boundary); capabilities classify *who may* — orthogonal
axes, so the error tree does not mirror the module split.

## 5. Deferred, each with its owner

- **Permissions, whole** (chmod, chown, mode bits in `fileInfo`): the permission model
  is platform-shaped and setuid is privilege-escalation-adjacent — it deserves its own
  scrutiny, under this same `filesystem` capability when it lands (a separate
  capability was considered and rejected: R121's split was contents-versus-structure,
  and permissions *are* structure — `delete` is strictly more destructive and lives
  here). Until then, `exec` running `chmod` is the escape hatch that already exists
  for every rare OS operation.
- **Symlinks** (creation, `readLink`, `lstat`, `walk`'s follow option) — one axis,
  deferred together.
- **Watching** — carried from the R121 record.
- **Recursive `copy`** — with a metadata-preservation decision it should not prejudge.
- **`cwd(): path`** — a read-only gated read, if real use demands it (std.process §3
  holds the no-`chdir` rule either way).
