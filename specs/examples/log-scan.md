# Log scan: first ten errors

Find the first ten `ERROR` lines in a large log without reading past them. The point is
the **chain**: catalogue calls compose lazy stages (the dataflow pipeline, stream §7 —
the `|>` operator is retired, R146), and `take(10)` bounds the whole thing, the
file source produces only about ten matching lines' worth of input (stream §7.2).

```luna
import std.io;
import std.process;

const isError = fn (line: any): bool => contains(line as string, 'ERROR');

const main = fn () use (io, argv): int! => {
  var fd = openFile(args()[0] as path);
  defer close(fd);

  foreach (line in fd.lines().filter(isError).take(10)) {
    println("$line");
  }

  return 0;
};
```

What it exercises:

- **Pull-driven laziness** (stream §7.2): `take(10)` pulls ten times; `filter` pulls
  until ten matches; the source reads only that far. Memory is bounded regardless of log
  size.
- **Chaining takes the stream** (stream §7.3): after `fd.lines().filter(…)`, the chain is
  the stream; the loop consumes the pipeline.
- **Predicates are `fn (any): bool`** (R34, no truthiness): `isError` narrows with
  `as string` before the string operation, elements of an untyped stream are `any`, and
  type-specific operations name their type first (any spec §2).
- **Interpolation is a universal operation** (any spec §1): `"$line"` renders without
  narrowing, display is total.
- **Word-prefix binding** (associativity §3): had this been `spawn`ed or `try`d, the
  prefix would wrap the whole pipeline expression.
