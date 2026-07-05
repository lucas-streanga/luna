# `std.io`

The first standard module, and deliberately a stress test: everything here is expressed with
the language's own tools, the capability model, protocols, streams, constraints, enums,
`defer`, and nothing io-specific is added to the language.

```
import std.io;

const main = fn () use (io) {
  println("hello");
};
```

## 1. The capability

`std.io` **exports** one capability:

```
export const io = capability;
```

There is no `caps` namespace (capabilities §4): `io` is an ordinary exported `const`, named in
`use` clauses as the imported binding. Modules are unnamed and resolved at compile time, so
the binding is unambiguous, and "what can do io" is a search for `use (io)` against this one
declaration. Every effectful function below carries `use (io)` and propagates it (capabilities
§5); `main` holds it by declaring it.

## 2. The `file` type

A `file` is a **table wearing the `fileDescriptor` protocol**, following the string-builder
precedent exactly (protocols §10, equality §4.4, concurrency §2.1):

- **`identityEquality`**: two files are equal only if they are the same handle; comparing
  contents is not what `==` on handles means.
- **Transferred crossing class**: a file is stateful and single-owner, like a stream or a
  builder. It **moves** across a spawn boundary, the source is marked *moved-from*, and any
  later access through any alias panics (concurrency §2.3). Ownership, not locking, is what
  makes operations on an opened file race-free: the type system guarantees one task holds it
  at every instant.
- **Meta state** (protocol-private): the OS handle, the `mode` / `format` / `sourceEncoding`
  it was opened with, the cursor, and any buffer.

**`file` is the exported name of the refinement**, a type alias (type spec, aliases are pure
sugar: a name, not a new type):

```
export const file = @fileDescriptor;
```

Free functions take `file` and are reached by UFCS (`fd.lines()` is `lines(fd)`); everywhere
this module says `file`, it means `@fileDescriptor`.

### 2.0 `path`

Paths are not bare strings. Like `json` (std.json §1), **`path`** is an invariant
constraint over `string`:

```
export const path = constraint { string as p where isValidPath(p) };
```

`isValidPath` is **platform-dependent** (separators, forbidden characters, length limits) and
its exact definition is deferred to `std.platform`'s maturation; the constraint exists now so
the boundary is typed from day one: an entry into a `path` position validates once,
immutable-base so entry is the only check (constraints §7), and a raw string reaching
`openFile` is caught at the signature instead of at the OS.

### 2.1 The standard handles

```
export const stdin:  file;
export const stdout: file;
export const stderr: file;
```

The three standard handles are **`const file`** values, and `const` is what lets them break
the single-owner rule safely: a `const` value is shared **by reference** across tasks with no
copy (concurrency §2.1), its Luna-side value never mutates, writing through a handle is an
**external** effect, the capability system's domain, not a value mutation, so `const`'s
guarantee is intact. The runtime **locks** the sinks behind them; that lock is the only lock
in this module, opened files need none (§2). The promise the lock buys is **line-level
atomicity**: one `println` call is written indivisibly, two tasks' lines never interleave
mid-line.

## 3. Modes, formats, encodings

Option parameters are **inline anonymous enums**, and call sites name variants with the
**fenced literal**, resolved by the parameter's expected type (enum spec §3.3):

```
openFile('data.bin', {read}, {binary});
```

Anonymous enums intern structurally (value-representation §4.1), so the same inline enum in
two signatures is one type. The axes: **mode** `enum {read, write, append}`, **format**
`enum {text, binary}`, **encoding** `enum {utf8, utf16le, utf16be, latin1}` (set open,
§9). `format` is the word, never `type`, which names a built-in.

## 4. Opening, closing, flushing

```
export const openFile = fn (
  fileName: path,
  mode:           enum {read, write, append} = {read},
  format:         enum {text, binary}        = {text},
  sourceEncoding: enum {utf8, utf16le, utf16be, latin1} = {utf8},
) use (io): file!;
```

Opening is the canonical **expected failure**, so the return is `file!` (errors §2, §7). The
error family, extensible pending experience (§9):

```
fileError        = error { path: path };
fileNotFound     = error : fileError {};
permissionDenied = error : fileError {};
```

```
export const close = fn (&fd: file) use (io): undefined;
export const flush = fn (&fd: file) use (io): undefined;
```

`close` releases the handle and marks the file's terminal state; any later operation on a
closed file **panics** (misuse, an invariant violation, the same category as wrong-mode use,
§8). The idiom is `defer`:

```
let fd = try openFile('log.txt', {append});
defer close(&fd);
```

`flush` forces any buffered writes to the external sink; `close` implies it. A file never
closed explicitly is released by the runtime eventually (the Go collector), a **backstop, not
a contract**: flush timing and error reporting are only defined through explicit `flush` /
`close`. The standard handles are never closed and need no `defer`.

## 5. Printing

```
export const println  = fn (line: string, fd: file = stdout,
                            lineEnding: string = platform.lineEnding) use (io): undefined;
export const print    = fn (text: string, fd: file = stdout) use (io): undefined;
export const printerr = fn (line: string,
                            lineEnding: string = platform.lineEnding) use (io): undefined;
```

All three return **`undefined`**: printing is a statement, and a non-`undefined` return would
force `_ =` on every print in every program (errors §8.1, the no-discard rule). `printerr` is
`println` aimed at `stderr` (a line ending on stderr is almost always wanted). Values that
are not strings are the caller's explicit choice: interpolation (`println("$x")`) or
`toString(x)` (conversion §3), never a hidden coercion. `lineEnding` is a `string`, so any
delimiter is expressible; the default reads `platform.lineEnding` (§10).

## 6. Reading

```
export const lines = fn (fd: file, lineEnding: string = platform.lineEnding,
                         includeLineEnding: bool = false) use (io): stream;
export const chunks   = fn (fd: file, size: int) use (io): stream;
export const readAll  = fn (fd: file) use (io): string | bytes;
export const readLine = fn (fd: file = stdin) use (io): string?;
```

- **`lines`** is **text-mode only** (a `Panic` on a binary file, the exact symmetry of `seek`
  being binary-only, §7): a stream of `string`, decoded per `sourceEncoding`, delimiter
  stripped unless `includeLineEnding`.
- **`chunks`** is the binary reader: a stream of `bytes`, `size` bytes per element (short
  final chunk).
- **`readAll`** yields the remaining content from the cursor, `string` in text format,
  `bytes` in binary, an honest union the caller matches on.
- **`readLine`** is the one-shot interactive read, `null` at end of input.

Reading a non-readable file (mode `write`/`append`) **panics**, misuse. An I/O failure
**during** consumption, a disk error mid-stream, also **panics**: streams have no error
channel, environmental mid-read failure is rare, and the supervisor `try`/`catch` block is
the boundary that catches it (errors §8.2). Expected failure lives at `openFile`; everything
after it is either correct or exceptional.

## 7. Writing and seeking

```
export const append     = fn (&fd: file, data: string | bytes) use (io): undefined;
export const appendLine = fn (&fd: file, line: string,
                              lineEnding: string = platform.lineEnding) use (io): undefined;
export const write      = fn (&fd: file, data: string | bytes) use (io): undefined;
export const seek       = fn (&fd: file, pos: int) use (io): undefined;
```

`append` writes at the **end** regardless of cursor; `write` writes **at the cursor** and is
**binary-mode only**, as is `seek`, byte offsets are only meaningful where no decoding sits
between bytes and content. Writing to a `{read}` file panics (misuse). **The file is the
cursor**: `lines()` and `chunks()` are views over it, consuming them advances it, `seek`
moves it under any live stream, and the next stream element reads from wherever the cursor
now is, defined, single-owner, and on the owner's head.

## 7.1 Parsing files: a composition, not an export

`std.io` hosts **no format knowledge**. The parsers are pure functions in the per-format
modules (`fromJson(j: json): table!` in std.json §3; siblings in std.csv, std.yaml,
std.xml), and file-to-table is one expression:

```
let t = try fromJson(readAll(fd) as json);
```

The `as json` narrows `readAll`'s `string | bytes` union and runs the format validation in
one visible step at the seam (constraints §7). Malformed content surfaces as the parser's
declarable error; a binary-mode `fd` fails at the `as` (a `string`-based constraint cannot
hold `bytes`), the misuse caught at the boundary it crosses.

## 8. The failure model, summarized

One table, because the categories were decided elsewhere and io just inherits them:

| Situation | Category | Why |
|-|-|-|
| open fails (missing, permissions) | declarable, `file!` | expected, environmental, anticipatable (errors §7) |
| wrong-mode use, use-after-close, `lines` on binary, `seek`/`write` on text | `Panic` | misuse, an invariant violation (errors §9) |
| I/O failure mid-stream or mid-write | `Panic` | environmental but exceptional; no stream error channel; boundary-caught (errors §8.2) |

## 9. Open questions

- **The error taxonomy**: the exact `fileError` children (disk full on write? interrupted?)
  pending real use; writes may yet warrant a declarable arm instead of the panic above.
- **The encoding set**, and what an encoding error during decode does (currently: the
  mid-stream panic rule).
- **Buffering policy**: sizes, when the runtime flushes implicitly, whether `stdout` is
  line-buffered on a tty.
- **Boundary with `std.system`**: `exists`, `stat`, deletion, directories are metadata and
  belong to the `system` capability (capabilities §9), not here.

## 10. `platform` (stub)

`std.platform` exports **`platform`**, a `const` table of compile-time-known facts about the
target: `platform.lineEnding` (`"\n"` or `"\r\n"`), `platform.os`, `platform.pathSeparator`.
Being `const` and comptime-known, its members are legal in default-parameter expressions
(functions §3.3.1) and fold at compile time. Its own spec is deferred; this module needs only
`lineEnding`.
