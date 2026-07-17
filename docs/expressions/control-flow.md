# Control flow

Luna has **two** loop constructs, `foreach` (iterate something) and `while` (loop on a
condition), and a conditional, `if`. There is no C-style `for`: it is redundant, because ranges
are lazy streams (range spec), so a counted loop is `foreach` over a range. Every control-flow
construct has both a **block** form and a **postfix** form, so one-liners read naturally.

```
foreach (v in xs) { use(v); }        // block
use(v) foreach (v in xs);             // postfix (one-liner)

while (more()) { step(); }            // block
step() while (more());                // postfix

if (ready) { go(); }                  // block
go() if (ready);                       // postfix
```

Loops (`foreach`, `while`) are **statements**, not expressions: they do not produce a value. (The
expression that selects a value by case is `match`, which is an operator, not a loop; match spec.)

**The postfix form is an exact desugar** (R46): `expr foreach (h);` **is**
`foreach (h) { expr; }`, `expr if (c);` is `if (c) { expr; }`, and everything follows from
that identity rather than from new rules, the head's names bind **within the body
expression only** (shadowing outer names there, leaking nowhere), the source is evaluated
once, and the no-discard rule applies to the body per iteration (a non-`undefined` body
needs `_ =`, exactly as the block would). The body **textually precedes** the head that
binds its names (`println("${line}") foreach (line in fd.lines());`), and that is
presentation, not a resolution problem: the parser needs no binding knowledge (identifiers
parse as identifiers), and name resolution walks the **AST**, head first, body inside the
head's scope, the comprehension-order every such construct uses. **One postfix modifier per
statement**: `expr if (c) foreach (h)` is a **compile error**, chained modifiers pose the
which-nests-which trap, and the block forms exist for exactly that.

---

## 1. `foreach`: iterate a stream, range, or collection

`foreach` binds each element in turn and runs its body. The binding uses **`in`** (not `as`,
which is checked narrowing and would be a different meaning, as spec; `in` reads as ordinary
iteration and conflicts with nothing):

```
foreach (v in xs)        { use(v); }         // value only
foreach (k => v in xs)   { use(k, v); }       // key and value
```

- **`foreach (v in xs)`** binds each element's **value** to `v`.
- **`foreach (k => v in xs)`** binds each element's **key** to `k` and **value** to `v`, reusing
  `=>` (key-to-value, as everywhere). For a list or an implicit-keyed stream (a range, a
  bare-`yield` generator; R93), `k` is the sequential index (0, 1, 2, ...) — which *is* its
  key set; for a keyed table or a `yield k => v` stream, `k` is the actual key.

The source `xs` may be anything iterable, and `foreach` is simply **the consumer** for the
iteration mechanisms already specified:

- **A stream** (stream spec) is consumed element by element. Because streams are **single-pass**,
  a `foreach` over a stream **consumes** it; iterating the same stream again requires restarting
  it (stream spec) or rebuilding it.
- **A range** (`0..<n`, range spec) is a stream in value position, so `foreach (i in 0..<n)` is
  the counted loop that a C-style `for` would express. `by`, descending bounds, and the
  enumeration key all come from the range and stream specs, `foreach` adds nothing range-specific.
- **A collection** (a table or list) is iterated over its entries; a list yields values in order
  (with the index as `k` in the keyed form), a table yields its key-value entries.

So `foreach` introduces no new iteration semantics; it is the statement that drives a stream,
range, or collection to completion (or to a `break`, §3). What can be iterated, and with what
laziness and single-pass rules, is defined by those specs.

### 1.1 The loop binding is scoped to the body

The `foreach` binding (`v`, or `k` and `v`) is scoped to the loop **body** and is fresh each
iteration; it does not leak after the loop (variables spec, loop scoping). The binding follows the
ordinary binding rules for what it holds (a value binding is not rebindable within the iteration
in a way that affects the next; each iteration gets its own).

### 1.2 Destructuring in the binding

The value (or key) binding position may **destructure** (destructuring spec), so iterating a
collection of structured elements binds their parts directly:

```
foreach ([x, y] in points)        { plot(x, y); }        // each element is a [x, y] list
foreach (k => [lo, hi] in ranges) { span(k, lo, hi); }    // value destructured
```

This is ordinary destructuring in the binding position, not a `foreach`-specific feature, so its
rules (exact-length lists, partial keyed, `...rest`) are the destructuring spec's.

---

## 2. `while`: loop on a condition

`while` evaluates a **`bool`** condition (no truthiness, bool spec) and runs its body while the
condition holds:

```
while (queue.isEmpty() == false) { handle(queue.pop()); }
```

The condition must be a `bool` (a comparison or boolean expression), never a coerced value (bool
spec §1). `while` is the construct for iteration that is not over a definite sequence, when the
stopping point is an arbitrary runtime condition rather than "the elements ran out." Anything
counted or sequence-driven is `foreach` over a range or stream (§1); `while` is for the genuinely
condition-driven case.

There is no `do`/`while` (test-at-bottom) form in the core; a loop that must run its body once
before testing is written by testing at the appropriate point, or deferred until a need is shown
(§5).

---

## 3. `break` and `continue`

Two statements alter loop flow, in both `foreach` and `while`:

- **`continue`** skips to the **next iteration** (in `foreach`, the next element; in `while`, the
  next condition test).
- **`break`** exits the **enclosing loop** immediately.

Both act on the **innermost** enclosing loop only. **Multi-level break is not provided yet**: to
exit several nested loops, extract the inner loops into a function and `return`, which is clean and
needs no new construct. A **labeled** `break` / `continue` (naming an enclosing loop to target) may
be added later if the need is shown; it would stay **structured** (it can only exit enclosing
loops, never jump arbitrarily). Luna will **never** have `goto`: control always flows
forward-and-out of enclosing structures, never to an arbitrary point. Numeric multi-level break
(`break 2`) is deliberately **not** adopted, it is count-based and silently retargets when an
intervening loop is added; a label (when introduced) says what it means and survives refactoring.

---

## 4. Postfix form

Every control-flow construct has a **postfix** form: a single statement followed by a trailing
control clause, which desugars to the block form wrapping that one statement.

```
use(v)   foreach (v in xs);      // desugars to: foreach (v in xs) { use(v); }
step()   while (more());          // desugars to: while (more()) { step(); }
warn()   if (bad);                // desugars to: if (bad) { warn(); }
```

- The postfix body is a **single statement**, not a block. (A braced block belongs to the block
  form; postfix exists precisely for the one-statement case, so `{ ... } foreach (...)` is not a
  postfix form.)
- Postfix is available for **all** control flow (`if`, `while`, `foreach`) uniformly, not a
  loop-only feature, so the one-statement-plus-trailing-clause reading is consistent everywhere.
- Because it desugars to the block form, postfix carries identical semantics (the same scoping,
  the same `break`/`continue` meaning within a postfix loop, though a one-statement body rarely
  needs them).

Postfix is sugar, not a distinct construct: anything a postfix form does, its block form does
identically. It exists for readability of the common single-statement case.

---

## 5. Resolved and deferred

- **No `do`/`while`.** A test-at-bottom loop is not provided; a run-once-then-test loop is the
  rare case, written as `while (true) { body; if (done) { break; } }`. It does not warrant a
  construct.
- **No loop-as-expression.** Loops are statements and never yield a value. Value-producing
  iteration is a **stream** (`map` / `filter` / `collect`, stream spec) or a **lambda**: a
  comprehension is a stream pipeline collected into a list, and a value-producing block is a
  function. So the division of labor is clear, statements iterate for effect, streams and lambdas
  iterate for a value, with no ambiguity about whether a loop is a statement or an expression.
- **No `foreach` iteration protocol.** `foreach` needs no user-extensible iteration hook, because
  every iterable the language has is already covered: **tables and lists are iterable**,
  **streams (including ranges) are iterable**, and **`bytes` is foreach-iterable directly**
  (R104): the governing rule is that `foreach` consumes anything with an **unambiguous
  element sequence**, which `bytes` passes (every element is a byte) and a bare `string`
  fails (bytes, codepoints, or graphemes? — strings choose their unit through explicit
  producers, string-api §9). A user type that wants to be iterated either *is* a
  table or **exposes a stream** (`thing.entries()`), which `foreach` then consumes. There is no
  further kind of collection needing a protocol, so iteration extensibility is already complete.
- **Labeled `break` / `continue` (deferred).** Multi-level break is not provided yet (§3); to
  exit several nested loops, extract them into a function and `return`. A labeled form may be
  added later, and would stay **structured** (exiting only enclosing loops, never `goto`). The
  exact label syntax is left until the need is demonstrated.
