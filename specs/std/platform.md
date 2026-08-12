# `std.platform`

```luna
import std.platform;      // dumps exactly one name: the table
```

The smallest module in the standard library, deliberately: **one export**, a `const`
table of compile-time-known facts about the **target**.

```luna
export const platform = [
  'os'            => 'linux',      // Go's GOOS vocabulary: 'linux', 'darwin', 'windows', ...
  'arch'          => 'x86-64',     // 'x86-64', 'arm64', ...
  'lineEnding'    => "\n",         // "\r\n" on windows
  'pathSeparator' => '/',
];
```

## 1. Target facts, not host facts — why comptime access is the point

Every value above describes the **compile target** (in script mode, the host *is* the
target), set as an input to the build — never the build machine's ambient environment.
That distinction is what dissolves the portability worry (R138): a comptime function
branching on `platform.lineEnding` is not baking in where it was built, it is
**conditional compilation done right** — the same source compiled *for* linux folds
`"\n"`, *for* windows folds `"\r\n"`, each binary correct for its target, and builds
stay reproducible under the only definition that survives cross-compilation: same
source + same target = same binary. The flip side is decisive: *without*
comptime-visible platform facts, portable libraries are unwritable — nothing could
select a path separator at compile time. Being `const` and comptime-known is also what
makes members legal in default-parameter expressions (functions §3.3.1) — which
`std.io` exercises today: `println`'s default reads `platform.lineEnding`.

**No capability, and that is the R43 theorem run in reverse** (R121, confirmed):
capabilities exist to mark what must not fold; platform facts are deterministic
per-build inputs, so they are eligible, and there is nothing to gate.

## 2. `os` and `arch` are strings, not enums

The Go vocabulary (`GOOS`/`GOARCH`), by ruling (R138): an exhaustive `match` over an
`os` **enum** would break on every toolchain release that adds a target — the
enum-growth hazard the timezone enum was rejected for (R133), arriving here through
slow growth rather than volatility. Strings grow silently, comparisons stay simple
(`platform.os == 'linux'`), and the canonical vocabulary is documented rather than
enforced. Match with a wildcard arm, as one should against an open set.

## 3. Why a module and not an ambient global

The table could have been predeclared — it was considered (R138). One import line
preserves what ambience would destroy: **the audit trail**. Platform-dependence is
exactly the cross-cutting concern a portability review wants greppable — "is this
library platform-aware?" is one grep of the imports — the same argument that put
introspection behind an import (R127). And the cost is nothing: under the import grid
(R136), `import std.platform;` dumps exactly one name, and the table is its own
namespace — pollution-free by construction.

## 4. The debtors, settled

- **`isValidPath`** — the deferral chain (io → filesystem → platform) terminates
  (R138): it is **`std.filesystem`'s export** (it is a path thing), its body
  comptime-branches on `platform.os`, and for the sole current target
  (`linux-x86-64`, compiler §0) the rule is minimal and real: **nonempty, no NUL
  byte, at most 4096 bytes**. Other targets add branches when they land.
- **errno portability** (io-errors §4) — stays where it is; it is io-errors' own
  question and nothing here forces it.

## 5. Deferred

Endianness, word size, CPU feature flags — FFI-era facts, added when FFI needs them.
Everything runtime-shaped (hostname, environment, filesystem) was never this module's:
it is `std.process` and `std.filesystem`'s, by construction. The fact set grows only
when a *target* fact earns its row.
