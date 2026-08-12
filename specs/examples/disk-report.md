# Disk report: the twenty biggest files

`du | sort -n | tail`, as one program. It walks a tree, stats every entry, keeps the
files, and prints the largest — and handles the two things the shell pipeline silently
gets wrong: an entry that vanishes or is unreadable mid-walk, and an empty result.

```luna
import std.io;
import std.process;
import { filesystem, path, fileInfo, entryKind } from std.filesystem;
const fs = import std.filesystem;

const kib = 1024;
const mib = 1024 * 1024;
const gib = 1024 * 1024 * 1024;

const human = fn (size: int): string => match {
  size >= gib => "${size / gib}G",
  size >= mib => "${size / mib}M",
  size >= kib => "${size / kib}K",
  _           => "${size}B",
};

const main = fn () use (io, filesystem, argv): int! => {
  let arguments = args();
  die("usage: ${arguments[0]} <dir>") if (arguments.count() != 2);

  var sizes = [];                                    // path => bytes
  var skipped = 0;

  foreach (p in fs.walk(arguments[1] as path)) {
    let probed = try fs.stat(p as path);
    if (probed is error) {
      skipped += 1;                                  // vanished or unreadable: keep walking
      continue;
    }
    let info = probed as @fileInfo;
    sizes[p] = info->size if (info->kind == {entryKind.file});
  }

  die('no readable files') if (sizes.isEmpty());

  foreach (p => size in sizes.sort(order: {descending}).take(20)) {
    println("${human(size).padStart(6)}  ${p}");
  }
  println("${human(sizes.sum() as int)} across ${sizes.count()} files");
  printerr("${skipped} entries skipped") if (skipped > 0);

  return 0;
};
```

What it exercises:

- **The two-line filesystem import** (std.filesystem): the capability and the *types*
  (`path`, `fileInfo`, `entryKind`) arrive bare because `use` clauses and annotations name
  bindings, never element accesses; the function surface is namespaced behind `fs.` so
  `walk`, `stat`, and `delete` do not land in the global namespace.
- **`walk` is a stream** (std.filesystem §3.2): a million-entry tree costs nothing to
  start, and the loop holds one path at a time. Only `sizes` grows, and it holds one `int`
  per file rather than a `@fileInfo` per entry.
- **`try` at the one call that fails per-element** (errors §8.1): `stat` is errorable
  because a race with `rm` is ordinary, not exceptional. Recovering to a value and counting
  the skips is what makes the report complete instead of aborted; a bare `fs.stat(p)` would
  propagate out of `main` and lose the whole scan to one unreadable file.
- **`is` tests, `as` narrows** (is §3, as §1): `probed is error` gates, `probed as @fileInfo`
  produces the narrowed value. The `as` cannot fail here — the `is` just ruled out the only
  other arm — which is exactly the shape `as`'s panic-on-mismatch is for.
- **`->` is protocol space** (protocols §3): `info->size` reads a granted `@fileInfo` member,
  resolved at compile time, while `sizes[p]` is element space and `human(size).padStart(6)`
  is UFCS to a free function. Three spellings, three meanings, no overlap.
- **A qualified variant literal** (enum §3.3): `{entryKind.file}` names its enum because a
  comparison operand supplies no target type to infer from (§3.2 lists annotations,
  parameters, and return types); at a typed site — `sort(order: {descending})` — the bare
  form is enough.
- **`sum` returns `int | double`** (iterable-functions §2.10), so `human`, which takes an
  `int`, gets an explicit `as int`. The union is the honest type of an aggregate over an
  untyped table, and the narrowing is one visible word rather than a silent coercion.
- **Postfix modifiers on assignment** (control-flow §5): `sizes[p] = … if (…)`
  desugars to the block form with `sizes` still in the outer scope. The same modifier on a
  *declaration* would be a compile error, since the binding would be trapped in the sugar
  block (§5.1).
