# The Luna Programming Language

Luna is a data-focused language that closes whole classes of bugs by construction, not by
discipline. It pairs the immediacy of a scripting language with the guarantees of an ahead-of-time
compiled one: run a file directly like a script, or compile it to a self-contained native binary,
from the same source, with the same semantics. Its wager is that most of what makes robust programs
hard, silent coercions, data races, hidden control flow, wraparound, null confusion, uninspectable
values, comes from operators and types quietly doing more than they say. Luna makes them do exactly
what they say: strict equality, explicit conversion, panics instead of silent wrong values,
concurrency with no shared mutable state, and a small surface where keywords are reused rather than
multiplied. It is heavily inspired by Raku, Lua, PHP, and Go.

The intended distribution is a single `luna` binary that is the whole toolchain, runner, compiler,
formatter, and language server, and a static library for embedding. No implementation exists yet;
this repository is the language's design.

## Backend

Luna compiles to **Go source**, which the Go toolchain then compiles to a native binary. Go is the
chosen backend because:

- Its semantics are close enough to Luna's that little is lost in translation.
- It is a well-performing language with a high-quality garbage collector, which Luna uses directly.
- It provides multiple target platforms and backend optimizations for free.
- Its compiler is fast, which is what lets Luna also be a scripting language.

The tradeoff Go imposes, that Luna values holding heap pointers cannot share a machine word with
scalars under Go's precise GC, is met in the value representation (see the internal-representation
specs); it does not leak into the language.

## Using the compiler

```
$ luna myprogram.luna
```

Runs the program. Imports are filesystem-based, so one file may pull in many Luna modules. Build
artifacts are cached under `$HOME/.lunalang/`, and rebuilds are incremental, so after the first run
there is no start-up cost, which is what keeps direct execution scripting-fast.

```
$ luna -c myprogram.luna        # or --compile
```

Compiles the program to a **self-contained** native binary, with no dynamic dependence on libc or
the Go runtime (both statically linked). Same caching and incremental rebuilds.

```
$ luna -d myprogram.luna        # or --debug
```

Builds in debug mode, emitting debug symbols; release mode is the default. Debug builds are
unoptimized at both the Luna and Go levels so a debugger can map native frames back to Luna source
(see the tooling and compiler specs).

```
$ luna -f myprogram.luna        # or --format
```

Runs the built-in formatter over the program and, recursively, its imported modules.
