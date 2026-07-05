# The pipeline operator `|>`

`|>` connects a **dataflow pipeline**: it joins a producer to the stages that process what it
produces, so data flows from one end to the other. It is defined for exactly two kinds,
**streams** and **commands**, the two places where "pipeline" is a genuine domain concept.
Seeing `|>` therefore always means "a dataflow pipeline," never merely "call a function."

This document gives the operator's unified semantics. The per-kind detail lives with each
kind: command pipelines in the **command** spec, stream pipelines in the **stream** spec.

---

## 1. `|>` is not general function application

Luna already composes functions through **UFCS**: `x.f().g()` applies `f` then `g`. So a
general pipe operator meaning `x |> f |> g` == `g(f(x))` would be **redundant**, a second
spelling of what `.`-chaining already does, and redundant syntax splits idiom for no gain.

`|>` is therefore *not* general function application. It is reserved for dataflow, a notion
UFCS cannot express: connecting a producer's output to a consumer's input so elements or bytes
**flow stage to stage**, lazily and (for commands) concurrently. That connection is not "call
`g` on the result of `f`"; it is "wire these stages together and run them as a pipeline." Only
streams and commands have that notion, so only they get `|>`.

Restricting the operator **sharpens** its meaning: `|>` unambiguously marks a pipeline, and
ordinary function application stays with UFCS. The two never blur.

---

## 2. Why `|>` and not `=>`

The pipeline operator is `|>`, not `=>`. `=>` is already load-bearing elsewhere (lambda
bodies, `match` arms, keyed table-literal entries `k => v`), so using it for pipelines would
collide with those forms. `|>` is distinct and reads as flow (the bar-and-arrow suggesting a
channel), so it carries the pipeline meaning without ambiguity.

---

## 3. Closed over kind

`|>` connects two values of the **same** pipeline-able kind and returns a new value of **that
same kind**:

```
stream |> transformer   ->  stream        // a new stream describing the composed dataflow
command |> command      ->  command        // a new command describing the composed process pipeline
```

- A **stream** pipeline produces a `stream` (stream spec §7): the source stream wired to
  stream-to-stream transformers.
- A **command** pipeline produces a `command` (command spec §4): the structured process
  pipeline, still inert until run.

So `|>` never converts one kind into the other. A stream pipe yields a stream; a command pipe
yields a command. This closed-over-kind rule is what keeps the operator simple: there is no
cross-kind coercion hidden in `|>`, and the result type is always the kind you piped.

### 3.1 Crossing kinds is an explicit bridge, not `|>`

Feeding a stream's elements into a command's stdin, or reading a command's stdout as a stream,
is meaningful but is **not** bare `|>`. Crossing the stream/command boundary requires
serialization (Luna values to bytes, or bytes to values), which is a real operation deserving
an explicit `exec`-level API, not an operator that silently does it. So `|>` stays within a
kind, and the stream/command bridge is a named operation (exec spec). This keeps `|>` from
having to define what it means to pour lazy Luna values into an OS process.

---

## 4. The pipeline is inert until consumed or run

In both kinds, building a pipeline with `|>` **does nothing on its own**:

- A **stream** pipeline is **lazy-start** (stream spec §1.2): constructing it runs no stage and
  touches no source. It is a description of a pull-chain, inert until something consumes it.
- A **command** pipeline is **inert** (command spec): it is a structured value describing
  processes to run, and nothing executes until it is handed to `exec` (which requires the
  `exec` capability).

So `let p = a |> b |> c` is side-effect-free at the point of construction in both kinds. Work
happens only at consumption (a stream's `foreach`) or execution (a command's `run`).

---

## 5. Stream pipelines: pull-driven, source-then-transformers

For streams, the left of `|>` is a **source stream** and each right-hand stage is a **stream
transformer** (a stream-to-stream operation such as `map`, `filter`, `take`; stream-api):

```
f.lines() |> filter(isError) |> map(parse) |> take(10)
```

Consumption is **pull-driven** (demand-based), which laziness requires: the consumer at the end
pulls one element, which pulls from the previous stage, back to the source, which produces one
element that flows forward through the stages. One element traverses the whole pipeline per
step. So `take(10)` pulls only ten times, the source produces only about ten lines, the
pipeline **short-circuits**, and memory stays bounded. Detail in stream spec §7.2.

### 5.1 Piping moves streams, and only streams, because `|>` has no move rule of its own

The operator's ownership behavior is **not the operator's**: operands behave per their
value class, exactly as function arguments do. A **stream** is a transferred, single-owner
resource (concurrency §2.1), so piping it moves it, below. A **command** is an **immutable
value** (command spec §4), so piping it shares it like any immutable value, the original
stays valid and reusable in other pipelines, no move, no copy anyone can observe. One rule,
the one every call site already follows; nothing pipe-specific to memorize.

For the stream operand: `a |> t` **moves** the stream `a` into the resulting pipeline: `a` is **taken**,
and any later use of `a` **panics** (a compile error where statically evident), the same
enforcement every other single-owner resource already has, a stream crossing `spawn`
(concurrency §2.3), a promise after `await` (await spec §1), a file after `close` (std.io
§4). Building the pipeline still consumes nothing (§4, lazy); the move is about **ownership
of the cursor**, not about elements: after the pipe there is exactly one handle to the
source, the pipeline. The discipline the earlier draft stated ("consume the pipeline, not
the original") is now enforced rather than requested, because the soft version was not
merely a stale-read hazard: the piped-from handle and the pipeline shared a live cursor, so
interleaved pulls made elements silently vanish from one consumer into the other, the
aliased-mutable-cursor bug single ownership exists to prevent.

### 5.2 Stages are effect-free by construction, and panic on failure

A transformer argument (`map(f)`, `filter(p)`) is an ordinary function value, so the
capability rules already decide two properties, worth surfacing:

- **Stages are `use`-free.** A capability-holding function value is second-class and cannot
  be passed as an argument at all (capabilities §3.1), so `f` in `map(f)` is necessarily
  capability-free: **a pipeline performs no effects beyond its source and its consumer**,
  by construction, not convention. Effects live at the endpoints (the `file` behind
  `lines()`, the `exec` that runs a command pipeline), which is exactly where `use` clauses
  already sit.
- **A failing stage panics at the pull.** Stages are plain `fn`, not `fn!`: a stream has no
  error channel (std.io §8, the mid-stream ruling), so a stage that cannot do its job
  panics, surfacing at the consumption site, boundary-caught like every mid-stream failure.
  Expected failure belongs before the pipeline (validate, then pipe), not inside it.

### 5.3 Precedence

`|>` is **left-associative** and binds looser than every value operator but tighter than
the word prefixes (associativity §1, its own tier): `a |> f(x) |> g` is `(a |> f(x)) |> g`,
and `try src() |> stage` wraps the whole pipeline, the reading a dataflow expression wants.

## 6. Command pipelines: structured process flow

For commands, both sides of `|>` are **commands**, and the pipeline wires each command's
standard output to the next command's standard input, the shell-pipeline notion, but built as a
**structured value** with no shell (command spec §4). The result is a `command` describing the
whole pipeline, run as a unit by `exec` with pipefail-style semantics (exec spec §4).

Because a command pipeline is structured (not a shell string), the stage boundaries are
explicit and introspectable (`stages`, `stageCount`, `isPipeline`; command spec §5), and
interpolated values remain single arguments, so a command pipeline is injection-safe by
construction just as a single command is.

---

## 7. Summary

- `|>` connects a **dataflow pipeline**; it is **not** general function application (that is
  UFCS). It exists only for **streams** and **commands**.
- It is `|>`, not `=>`, which is taken by lambdas, `match` arms, and keyed table entries.
- It is **closed over kind**: `stream |> ... -> stream`, `command |> ... -> command`. Crossing
  kinds is an explicit `exec` bridge, never bare `|>`.
- The pipeline is **inert until consumed or run**: stream pipelines are lazy-start, command
  pipelines are inert until `exec`.
- **Stream** pipelines are **pull-driven** (source then transformers, short-circuiting), and
  piping **transfers and consumes** the source under the single-pass discipline.
- **Command** pipelines wire stdout to stdin as a **structured**, shell-free, injection-safe
  value.

---

## 8. Open questions

- **Precedence and associativity:** where `|>` sits relative to other operators, and its
  associativity (left-associative is assumed, `a |> b |> c` == `(a |> b) |> c`).
- **Stream-to-command and command-to-stream bridges:** the exact `exec`-level API for crossing
  kinds (§3.1), pending the resource/`io` model.
- **User-defined pipeline kinds:** whether any type other than `stream` and `command` could
  participate in `|>` (currently no; the operator is closed to these two domain kinds).
