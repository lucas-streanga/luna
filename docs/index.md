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

## A taste

```luna
import std.io;
import std.json;

const exitCode = ['success' => 0, 'usage' => 64];

const main = fn () use (io, argv): int! => {
  let arguments = args();                        // argv is a capability
  die("usage: ${arguments[0]} <config.json>") if (arguments.count() != 2);

  var fd = openFile(arguments[1] as path);      // file!: error propogates out of main
  defer close(&fd);

  let config = fromJson(readAll(fd) as json);    // table!: bad JSON propagates too

  match (config['server']) {
    ['host' => h: string, 'port' => p: int] => println("serving on $h:$p"),
    ['port' => p: int]                      => println("serving on localhost:$p"),
    undefined                               => die('config has no server section'),
    _                                       => die('unrecognized server shape'),
  };

  return exitCode.success;
};
```

Note the shapes on display: functions are values bound to names; effects (`io`) and program
inputs (`argv`) are reached only through explicit `use` clauses; an errorable `main` lets
`openFile` and `fromJson` propagate bare, no ceremony on the happy path; `as` narrows at
boundaries (`path`, `json`); `defer` pairs the close with the open; and `match` dispatches
on the data's **shape**, structural patterns naming the keys they require (absent key, arm
fails; extra keys, nobody's business, match §4) with typed binders inside (`h: string` tests the
key's value *and* binds it narrowed, match §2.2), a literal `undefined` arm for the missing
section, and `_` because this match chooses to be exhaustive. First matching arm wins, top to
bottom.

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

## Spec index

Every spec in this repository, grouped by area. Each spec is authoritative for its own topic; this
table is a map.

### Orientation

| Spec | File | What it is |
|-|-|-|
| High-level overview | `high-level-overview.md` | A map of the language's shape, commitments, and full type set. |
| Types | `types.md` | The compact index of every type and where each is specified. |

### Value & primitive types

| Spec | File | What it is |
|-|-|-|
| Int | `int.md` | The 64-bit signed integer primitive (inline, overflow panics). |
| Double | `double.md` | The 64-bit IEEE 754 float primitive (inf/nan, never panics). |
| Bool | `bool.md` | The two-valued `true`/`false` primitive, with no truthiness. |
| String API | `strings.md` | The programmer-facing surface of the immutable UTF-8 `string` type. |
| Bytes | `bytes.md` | The packed, mutable, growable byte buffer type. |
| Regex | `regex.md` | The compiled regular-expression type and its `/.../` literal. |
| Command | `command.md` | The structured, inert program/pipeline type built from a backtick literal. |
| Secret | `secret.md` | The self-redacting sensitive-payload type, read only via `reveal`. |
| JSON | `json.md` | The `json` constraint on `string`, and the `toJson` / `toJsonDynamic` serialization surface. |
| Type | `type.md` | The `type` type: a type as a first-class, comparable value. |
| Undefined | `undefined.md` | The language-produced absence sentinel (missing key / void return). |
| Numeric tower | `numeric-tower.md` | The complete numeric type set and the widening/conversion rules relating them. |

### Structured & composite types

| Spec | File | What it is |
|-|-|-|
| Tables | `tables.md` | The core concepts of the table type: keys, value semantics, sealing, permissions. |
| Table API | `table-api.md` | The operation catalogue for tables (the built-in protocol's methods). |
| Views | `views.md` | The `view` type that pairs a table with one applied protocol and redirects access to it. |
| Functions | `functions.md` | The `fn` value: capture, errorability, and comptime-eligibility. |
| Stream | `stream.md` | The lazy, single-pass sequence type and its two defining properties. |
| Stream API | `stream-api.md` | The operation surface of `stream` (inspect, transform, collect, consume). |
| String builder | `stringBuilder.md` | The mutable string accumulator that avoids O(n²) concatenation. |
| Enum | `enum.md` | The discriminated union (tagged sum) declaration form. |

### Declaration forms & the type system

| Spec | File | What it is |
|-|-|-|
| Protocols | `protocols.md` | How behavior and per-key contracts attach to tables (Luna's object model). |
| Constraints | `constraints.md` | Refinement types: a base type narrowed by a pure runtime-checked predicate. |
| Errors | `errors.md` | The sealed error type hierarchy and the error model. |
| Capabilities | `capabilities.md` | Unforgeable authority tokens gating outside-reaching effects. |
| Attributes | `attributes.md` | Static, comptime-only declaration metadata tags. |
| Never | `never.md` | The bottom type: no values, the identity for `|`, the non-returning result type. |

### Type operators & narrowing

| Spec | File | What it is |
|-|-|-|
| `as` | `as.md` | The checked narrowing operator (runtime-checked, panics on mismatch). |
| `is` | `is.md` | The total boolean type-test operator (never panics, never narrows). |
| Match | `match.md` | The pattern/guard selection expression, in valued and open-ended forms. |
| Equality | `equality.md` | Strict `==`: same type and same value, no coercion. |
| Conversion | `conversion.md` | Value conversion via functions, and how it differs from `as`. |
| Reflection | `reflection.md` | Built-in functions that ask questions about a `type`. |

### Values, bindings & access

| Spec | File | What it is |
|-|-|-|
| std.io errors | `io-errors.md` | The `ioError` hierarchy, errno-grounded for linux-x86-64. |
| Keywords | `keywords.md` | The reserved-word inventory, contextual keywords, predeclared names, and flagged definition gaps. |
| Any | `any.md` | The top type: universal operations vs narrow-first, the F22 rule. |
| Examples | `examples/` | Worked programs: the one-billion-row challenge, log scanning, serialization, testing. |
| Tests | `tests.md` | The `test` declaration, the runner, isolation via tasks, the capability-shaped test table. |
| Await | `await.md` | Collecting a task: parking, move-out results, consumed promises, cancellation deferred. |
| Associativity | `associativity.md` | Precedence and associativity: the expression and type grammars, word-prefix binding, resolved drift, parser-blocking questions. |
| std.io | `io.md` | The first standard module: the `io` capability, files, printing, streams. |
| std.json | `json.md` | The `json` type, `toJson` / `toJsonDynamic`, `fromJson`. |
| std.csv / std.yaml / std.xml | `csv.md`, `yaml.md`, `xml.md` | Per-format constraint + reader modules. |
| Variables | `variables.md` | The `var`/`let`/`const` binding ladder, passing semantics, and scoping. |
| Destructuring | `destructuring.md` | Binding several variables from a table by position or key. |
| Spread | `spread.md` | `...x`: the table-literal fold, streams, call arguments, interpolation; one level, always. |
| Optional access & coalescing | `optional-access-and-coalescing.md` | Semantics of `?.`, `??`, `???` and their compound-assignment forms. |
| Wildcard | `wildcard.md` | The `_` operator: a deliberate unnamed blank, resolved by context. |
| Range | `range.md` | The `lo..hi` syntactic construct (a stream, a slice bound, a match test). |

### Operators & control flow

| Spec | File | What it is |
|-|-|-|
| Operators | `operators.md` | The master catalogue of every operator and the rules governing all of them. |
| Numeric operators | `numeric-operators.md` | The arithmetic operators in detail (`+ - * / %`, unary `-`) and their edges. |
| Control flow | `control-flow.md` | The `foreach`, `while`, and `if` constructs. |
| Pipeline | `pipeline.md` | The `|>` dataflow operator over streams and commands. |
| Defer | `defer.md` | Deterministic block-exit cleanup on any exit path, including panic. |

### Concurrency & effects

| Spec | File | What it is |
|-|-|-|
| Concurrency | `concurrency.md` | Green threads: `spawn`, `promise`, `await`, and shared-nothing isolation. |
| Exec | `exec.md` | Running a `command` as a capability-gated, error-governed effect. |

### Modules, tooling & implementation

| Spec | File | What it is |
|-|-|-|
| Lexical structure | `lexical-structure.md` | Source encoding, identifiers, and comments — the rules that apply before tokenizing. |
| Lexer | `lexer.md` | The token inventory: lexer modes, per-lexeme RE2 patterns, and maximal-munch ordering. |
| Modules | `modules.md` | The file-is-a-module system: static, DAG-shaped imports bringing names into scope. |
| Compiler | `compiler.md` | The compiler's phase pipeline, IR, optimization passes, and Go emission. |
| Incremental compilation & build cache | `incremental-compilation-build-cache.md` | The per-module artifact cache and its correctness/reuse rules. |
| Tooling | `tooling.md` | The compiler-provided formatter, language server, and debugger. |

### Internal representation

| Spec | File | What it is |
|-|-|-|
| Value representation | `internal-representation-of-variables.md` | Runtime storage: the `lval`, the `typetable`, and where per-value/type/binding state lives. |
| String representation | `internal-representation-of-strings.md` | Runtime storage of the `string` payload an `lval` points at. |
