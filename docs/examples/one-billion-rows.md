# The One Billion Row Challenge

Read a file of `station;temperature` lines (a billion of them), and print per-station
min/mean/max, sorted by station. The Luna version is a straight stream fold: constant
memory in the input (the file never materializes; `lines()` is lazy), one table of
aggregates.

```
import std.io;

const main = fn () use (io, argv): int! => {
  let arguments = args();
  die('usage: brc <measurements.txt>') if (arguments.count() != 1);

  var fd = openFile(arguments[0] as path);      // file!: in this fn!, failure propagates
  defer close(&fd);

  var agg = [];                                  // station => ['min','max','sum','count']

  foreach (line in fd.lines()) {
    let [station, rawTemp] = split(line, ';');   // stream destructuring: pulls the two fields (R103)
    let temp    = rawTemp.parseDouble();         // double!: malformed input propagates

    agg[station] ??= ['min' => temp, 'max' => temp, 'sum' => 0.0, 'count' => 0];
    if (temp < agg[station].min) { agg[station].min = temp; }
    if (temp > agg[station].max) { agg[station].max = temp; }
    agg[station].sum   += temp;
    agg[station].count += 1;
  }

  foreach (station in sort(keys(agg))) {
    let s = agg[station];
    println("${station}=${s.min}/${s.sum / s.count.toDouble()}/${s.max}");
  }

  return 0;
};
```

What the example exercises, with the rulings it leans on:

- **Lazy line streaming** (std.io §6): a billion rows never exist in memory at once; the
  file is the cursor and `lines()` is a view over it.
- **`??=`** (associativity §1): first-sight initialization of the aggregate row, exactly
  the absent-assign it was added for.
- **Stream destructuring** (destructuring §1.4): `split` produces a stream (producers
  produce streams, strings §1, R102), and the pattern pulls exactly the two fields it
  binds; a malformed extra field is simply left unconsumed, and a missing one binds
  `undefined`, which **panics at the `parseDouble` use** rather than passing silently
  (undefined spec: holding is fine, using panics).
- **Errorable `main`** (errors §5): `openFile` and `parseDouble` propagate bare, a missing
  file or a malformed row exits with the error; wrap either in `try` to recover instead.
- **`defer close(&fd)`** (std.io §4), `&` on a `var` binding (variables §5.1).
- **Element-path writes and compound assignment**: `agg[station].sum += temp` evaluates
  the target once (associativity §1) and writes through the path.
- **`x.name` vs `x.name()`** (functions §3.4): `s.count` is element access (no call),
  `s.count.toDouble()` is UFCS on its value.

**On parallelizing.** The classic 1BRC trick, split the file into N byte ranges and
aggregate in parallel tasks, meets an honest Luna constraint: `seek` is binary-mode and
`lines` is text-mode (std.io §6, §7), so byte-range chunking means `chunks()` plus manual
line-boundary reassembly per task, each task opening **its own** `file` (files are
single-owner, transferred; concurrency §2). The merge step is then one table-fold over
`await`ed per-task aggregates (await §1.1). Left as the exercise it genuinely is; the
sequential version above is the specification of *what* the parallel one must compute.
