# Parallel grep: fan out with `spawn`, fan in with a channel

Search a directory tree for a string, using every core. The shape is the one Luna's
concurrency model actually supports and the one most concurrent programs want: **`spawn`
fans work out, a channel fans results back in**, and the results print as they arrive
rather than after the last worker finishes.

```luna
import std.io;
import std.process;
import { filesystem, path } from std.filesystem;
const fs = import std.filesystem;

const batch = 64;

const scan = fn (paths: table, needle: string, hits: sink) use (io): undefined => {
  foreach (p in paths) {
    let opened = try openFile(p as path);
    if (opened is error) {
      printerr("skipped ${p}: ${(opened as error).message}");
      continue;
    }
    var fd = opened as file;
    defer close(fd);                       // block-scoped: this iteration's handle, this iteration

    var n = 0;
    foreach (line in fd.lines()) {
      n += 1;
      hits.send(['path' => p, 'line' => n, 'text' => line]) if (contains(line as string, needle));
    }
  }
};

const main = fn () use (io, filesystem, argv): int! => {
  let arguments = args();
  die("usage: ${arguments[0]} <needle> <dir>") if (arguments.count() != 3);
  let needle = arguments[1] as string;

  let [tx, rx] = channel(256);

  foreach (paths in fs.walk(arguments[2] as path).chunk(batch)) {
    _ = spawn scan(paths, needle, tx);     // each task gets its own handle on the same channel
  }
  finish(tx);                              // this handle is done; the workers' are not

  var found = 0;
  foreach (['path' => p, 'line' => n, 'text' => text] in rx) {
    println("${p}:${n}: ${text}");
    found += 1;
  }
  return found > 0 ? 0 : 1;                // grep's convention
};
```

What it exercises:

- **Fan-in is what channels are for** (channels §3): `sink` is the one shared handle in the
  stateful class, because it has no readable surface — nothing to race over. Every task
  holds a handle to the same channel, which is MPSC's *many senders* half, and `rx` is
  literally a `stream`, so the consumer loop is an ordinary `foreach` with the whole
  catalogue available.
- **No `WaitGroup`, because finishing is per-handle** (channels §5): there is no
  whole-channel `close` to coordinate, so nobody can close the channel out from under a
  sibling. `main` finishes *its* handle; each task's is finished by its own scope exit; the
  receive stream ends exactly when the last one goes. The counting that Go needs a
  `sync.WaitGroup` for is the runtime's, and it is not a Luna value.
- **`_ = spawn`** (await §2): the promise is acknowledged and dropped. Dropping it does not
  orphan the task — lifetime is **scope**-bounded, not promise-bounded (concurrency §6), so
  `main`'s scope owns all of them — and the no-discard rule still makes the drop visible.
  A promise could not be stored in a table anyway (§3.1: promises are confined).
- **Backpressure, not deadlock** (channels §1): `main` spawns every batch before it receives
  anything, so with a big tree the workers fill the 256-slot buffer and park on `send`.
  That is the buffer doing its job; `main` reaches its receive loop and drains them. A
  capacity of `0` would rendezvous instead, coupling each send to a receive.
- **Batching is the granularity knob, and it is not a cap.** `chunk(64)` divides the task
  count by 64; it does not bound it, because the tree decides how many batches there are.
  Green threads are cheap enough that this is usually the right trade. What *is* bounded is
  descriptors per task — exactly one, because `defer close(fd)` releases at the end of **its
  iteration's block** (defer §1), not at the end of the task — so live handles track live
  tasks. A hard cap on both means a fixed task count over a materialized list
  (`walk(…).collect()`), and the `collect` is the visible O(n) price of asking for one
  (iterable-functions §1.4).
- **Elements of an untyped stream are `any`** (any §2): `contains(line as string, needle)`
  names the type before the string operation, and `p as path` does the same at `openFile`.
  Interpolation needs no such thing (`"${p}:${n}"`) — display is total.
- **Destructuring in the loop binding** (control-flow §1.2): the consumer names the three
  keys it needs from each message, which is ordinary destructuring in a binding position,
  not a `foreach` feature.
- **The worker prints its own skips** (std.io §5): `scan` declares `use (io)`, and the
  capability crosses the spawn boundary shared by reference (concurrency §2.1) — the one
  referential capture in the language, because there is no slot to race on.
