# Tests

Testing is a language feature, not a library convention: a declaration form (`test`), a
runner in the suite binary (`luna -t` / `luna --test`), and semantics assembled entirely
from ratified machinery, tasks for isolation and parallelism, `await` for collection,
throws for failure, capabilities for effects.

## 1. The `test` declaration

```
test 'validate the lexer works' {
  let toks = lex("let x = 1;");
  throw error('wrong count') if (toks.count() != 5);
}

test 'reads the fixture file' use (io) {
  var fd = try openFile('fixtures/config.json' as path);
  defer close(&fd);
  _ = try fromJson(readAll(fd) as json);
}
```

- **The name is a string literal**, and it is the test's identity: two tests with the same
  name in one program are a **compile error** (duplicate key, caught at compile time); the
  empty string is an error. String names are the point, `'openFile panics on a closed fd'`
  reads as the specification line it is.
- **Zero parameters.** The runner has nothing meaningful to pass; fixtures are ordinary
  bindings and helpers. (A parameter list implies a caller contract nobody holds; revisit
  only if fixture injection earns it, §6.)
- **The return type is `undefined!`, implicitly, always.** `undefined` because a test's
  result is its completion (nothing to return, nothing to `_ =`); errorable because
  **throwing is how a test fails**, and every originating `throw` requires errorability
  (errors §4), so the `test` form declares the `!` for you. No annotation is written or
  permitted.
- **`use` clauses work exactly as on functions.** A test exercising a capability declares
  it (`use (io)`), and the **runner is the granting entry point**, precisely as `main` is
  (capabilities §7): tests have no other origination story and need none.

## 2. Failure is a throw; the runner catches everything

A test **passes** by returning and **fails** by throwing, either kind: a declarable error
(`throw error('...')`, a `try`-less propagated `!` call) or a **panic** (a failed `as`, an
index misuse, an assertion that dereferenced wrongly). The runner sits at a `try`/catch
boundary per test, and `await` already delivers both a task's error and its panic at the
collection point (await spec §1), so "collect all throws" is the machinery's default
behavior, not new mechanism.

## 3. Isolation and parallelism are the concurrency model, verbatim

The runner **spawns each test as a task** and awaits the promises:

```
// the runner, morally:
foreach (t in <tests>) { promises->push(spawn t()); }
foreach (name => p in promises) { results[name] = try await p; }
```

Everything the vision asked for falls out of rulings already made: **parallel** because
tasks are; **self-isolated** because tasks share only `const` values, whose freeze covers
meta space and applied sets (variables §3, R34), so two tests **provably cannot race on
Luna state**; deep-copy-in is vacuous (zero arguments). What tests *can* race on is
external resources through shared capabilities, two tests writing one file, which is the
concurrency spec's stated boundary (§2.1), managed by convention (per-test temp paths,
std work, §6). Output interleaving on `stdout` is already line-atomic (std.io §2.1).

## 4. The test table: every test is callable, requirements checked at the call

Tests are not callable by ordinary syntax (a string is not an identifier), so the runtime
exposes a **`const` table** for programmatic access, `std.test`'s `tests`, keyed by name,
and **every test gets a callable `run`** (function values are first-class, carrying their
requirement set on the value, capabilities §3.1, R39):

```
tests['reads the fixture file']
// ['name' => 'reads the fixture file', 'requirements' => [io], 'run' => fn (): undefined!]

_ = try tests['reads the fixture file'].run();   // legal anywhere; the call checks the
                                                 // requirement set against YOUR frame's
                                                 // grant, and panics without `use (io)`
```

No laundering is possible: invoking a test through the table checks its requirements
against the **caller's** granted set at the call (panic on shortfall), so an io-exercising
test runs only under a frame that declared io, exactly as if called by the runner, whose
grant covers everything. The `requirements` field exists so callers can check before
invoking rather than catching the panic.

## 5. The runner

`luna -t` (`luna --test`) compiles the program with its tests, runs them per §3, and
reports per test: name, pass/fail, and the collected throw (type, message, stack) on
failure. Exit status is nonzero iff any test failed. Ordering of the report is
declaration order; execution order is the scheduler's. Filtering (`luna -t 'lexer*'`),
and whether test declarations are stripped from non-test builds (they should be, dead
weight otherwise), are runner details for the build spec.

## 6. Open questions

- **Discovery and file conventions**: whether tests live anywhere in the build set
  (current assumption: yes, any module) or `*_test` files get special treatment; build
  spec territory.
- **Fixtures and parameters**: zero-arg is the ruling; if shared expensive setup demands
  injection, revisit, likely as ordinary `const` comptime values first.
- **Per-test temp resources**: a `std.test` helper for isolated temp paths, pending
  `std.system`.
- **Assertion helpers**: `throw error(...) if (cond)` is workable today; whether a
  dedicated `assert` (with expression capture in the message) earns keyword or std status,
  pending use.
- **comptime tests**: whether a test may be comptime-eligible and run at build time as a
  static check, attractive, deferred.
