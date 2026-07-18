# Log scan: first ten errors

Find the first ten `ERROR` lines in a large log without reading past them. The point is
the **pipeline**: `|>` composes lazy stages, and `take(10)` bounds the whole thing, the
file source produces only about ten matching lines' worth of input (pipeline §5, stream
§7).

```
import std.io;
import std.process;

const isError = fn (line: any): bool => contains(line as string, 'ERROR');

const main = fn () use (io, argv): int! => {
  var fd = openFile(args()[0] as path);
  defer close(fd);

  foreach (line in fd.lines() |> filter(isError) |> take(10)) {
    println("$line");
  }

  return 0;
};
```

What it exercises:

- **Pull-driven laziness** (stream §7.2): `take(10)` pulls ten times; `filter` pulls
  until ten matches; the source reads only that far. Memory is bounded regardless of log
  size.
- **Piping moves the stream** (pipeline §5.1): after `fd.lines() |> ...`, the pipeline is
  the stream; the loop consumes the pipeline.
- **Predicates are `fn (any): bool`** (R34, no truthiness): `isError` narrows with
  `as string` before the string operation, elements of an untyped stream are `any`, and
  type-specific operations name their type first (any spec §2).
- **Interpolation is a universal operation** (any spec §1): `"$line"` renders without
  narrowing, display is total.
- **Word-prefix binding** (associativity §3): had this been `spawn`ed or `try`d, the
  prefix would wrap the whole pipeline expression.
