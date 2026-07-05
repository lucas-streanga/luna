# Range

A **range** is written `lo..hi` and denotes a contiguous sequence of integers. A range is **not
a type** and **not a first-class value**: there is no `Range` object with range methods. `..` is
a **syntactic construct** whose meaning depends on where it appears:

- **In value position** (assigned, passed, iterated), `lo..hi` produces a **`stream`** of
  integers (stream spec), a lazy sequence, so it costs almost no memory and can be iterated,
  piped, and transformed like any stream.
- **In a `match` pattern**, `lo..hi` is consumed as a **membership test** (endpoints only), not a
  stream (match spec §5).

The unifying rule: **`..` becomes a stream where a sequence is wanted (value position), and is
read as endpoints where a bound is wanted (match).** Slicing is a separate construct with its
own syntax (`list[a:b]`, tables and bytes specs), not `..`, so ranges never need half-open
slice semantics (§2.1).

---

## 1. Inclusive by default

`lo..hi` is **inclusive** of both ends: `1..10` is `1, 2, ..., 10` (ten values, 10 included).
This is the human-intuitive reading ("1 to 10" includes 10) and is what iteration and `match`
membership want:

```
foreach (v in 1..10) { ... }        // v takes 1 through 10 inclusive
match (code) { 200..299 => "ok" }   // matches 200 through 299 inclusive
```

Inclusivity is **consistent in every position** `..` appears (value and match), because the one
position that wants half-open semantics, slicing, uses a different syntax (`:`), so `..` is
never pulled in two directions. `..` always means inclusive.

### 1.1 `..<`: exclusive of the top

`lo..<hi` is inclusive of `lo` and **exclusive** of `hi`: `0..<10` is `0, 1, ..., 9`. The `<`
marks the excluded top ("up to less than `hi`"). This is the natural form for the
**index-iteration** idiom, where the valid indices of an `n`-element collection are `0..<n`:

```
foreach (i in 0..<len) { ... }      // i takes 0 through len-1: each valid index
```

Only the top-exclusive form exists. A bottom-exclusive form (`<..`) is not provided: for
integers "just after `lo`" is simply `lo + 1`, so excluding the low end is `lo+1..hi`, needing
no syntax. So there are two forms, inclusive `..` and top-exclusive `..<`, and no others.

---

## 2. Value position: a range is a stream

In value position, `lo..hi` produces a **`stream`** of integers, evaluated lazily. It inherits
the entire stream toolkit (stream spec) with no range-specific machinery:

```
let r = 1..1000000;                 // a stream: no million-int allocation, just a lazy sequence
foreach (v in r) { ... }            // consume it
let evens = 1..100 |> filter(isEven) |> take(5);   // pipe and transform like any stream
```

- **Low memory.** `1..1000000` does not allocate a million integers; it is a lazy stream that
  yields them on demand. This is the point of ranges being streams.
- **Foreach with enumeration keys.** A range stream is **values-only** (stream spec §1.1), so
  `foreach (k => v in lo..hi)` gives `k` the sequential enumeration index (0, 1, 2, ...) and `v`
  the value. For `10..20`, `k` and `v` differ (`k=0, v=10`; `k=1, v=11`; ...), so you get both
  the zero-based position and the actual value. This "where am I" index falls out of values-only
  stream semantics, not a range special case.
- **Restartable.** Unlike many streams, a range is trivially restartable (re-run from `lo`), so
  `restart()` works cleanly on a range stream (stream spec §4).
- **Single-pass otherwise.** Being a stream, a range is single-pass per traversal (stream spec
  §2); re-iterate by re-writing the literal (cheap) or `restart()`.

### 2.1 Slicing is not a range

Slicing a list or buffer (`list[1:3]`, `bytes[2:]`) uses a **separate `:` syntax** with
**half-open** semantics (tables and bytes specs), not `..`. This is deliberate: slicing wants
half-open, exclusive-end indexing (so `[0:len]` is the whole thing and `[a:b] + [b:c]` composes
without overlap, the Python and Rust convention), while ranges want inclusive iteration. Giving
them different syntax lets each have its natural convention without one forcing the other. So
`..` is always inclusive (ranges), `:` is always half-open (slices), and they never conflict.

---

## 3. `by`: an arbitrary step expression; the sign is the direction

`lo..hi by step` takes **any `int`-valued expression**, evaluated **once** at range creation
(`0..n by stride * 2` is fine), and the **step's sign selects the direction**:

```
foreach (v in 0..100 by 10) { ... }     // 0, 10, 20, ..., 100
1..<10 by 2                              // 1, 3, 5, 7, 9 (composes with ..<)
10..0 by -2                              // 10, 8, 6, 4, 2, 0: EXPLICIT descending
0..10 by -1                              // empty (0 >= 10 is false at the first check)
```

A step of **`0` panics** (a misuse, the infinite-loop guard; checked once, at creation).
Everything above is a corollary of the desugar (§4a), not a rule table. Non-constant
*progressions* (geometric, arbitrary next-from-current) remain **not** range syntax; they are
ordinary generator functions (stream spec §1), the step is an arbitrary *value*, never a
per-iteration computation.

---

## 4. Bounds never determine direction: `lo > hi` is empty

`a..b` with `b < a` is the **empty range** (R28): the deciding counterexample is `0..n-1` at
`n == 0`, the most common loop header in the language, which under bounds-determined
descending would silently iterate `0, -1`. **Descending is explicit**: write a negative step
(`10..0 by -2`, §3), where writing the sign *is* the explicitness, no loop header can descend
by accident. A single-element range `5..5` yields just `5`; `5..<5` is empty.

**Bounds-implied descending (`10..0` desugaring to `by -1`) was considered twice and
rejected (R28, R48)**, and the desugar's simplicity was never the issue, the consequences
were: (1) the `0..n-1` bug returns verbatim, data-dependent (fires only on the empty
collection, the case tests miss) and silent (`t[-1]` reads `undefined` and travels, no
panic); (2) direction becomes runtime-opaque for every range over non-literal bounds, every
reader's model of every loop conditional on data; (3) explicit `by` would then need a
bounds-vs-sign conflict rule that the current design makes unnecessary (`0..10 by -1` is
harmlessly empty, the sign steers, the bounds only bound); (4) the purchase is six
characters on the *rare, deliberate* case, and the frequency asymmetry is the whole
argument, ascending-to-possibly-empty must be safe by default, descending can afford to say
so. The tempting middle, implicit descent **only for literal bounds**, is rejected on
phase-invariance grounds (functions §5.5): extracting `10` into a `const` must not change a
loop's behavior. If `by -1` ever proves heavy in practice, the future ergonomic is a
`downTo` sugar over the explicit form (`10 downTo 0` ≡ `10..0 by -1`), never a change to the
bare one.

---

## 4a. The desugaring, explicit

A range is **sugar for an immediately-invoked generator**, no magic, and every rule above is
a line of it:

```
lo..hi by s      ≡      (fn (): stream => {
                          let step = s;                    // arbitrary expr, evaluated ONCE
                          throw typeError('zero step') if (step == 0);   // panics: misuse
                          var i = lo;
                          if (step > 0) { while (i <= hi) { yield i; i += step; } }
                          else          { while (i >= hi) { yield i; i += step; } }
                        })()
```

- Bare `lo..hi` is `by 1`; `..<` replaces `<=`/`>=` with `<`/`>`; open-top `lo..` (§5) drops
  the condition (`while (true)`).
- **The desugar proves the rules**: `0..-1` runs zero iterations (`0 <= -1` is false), the
  empty-range ruling; `0..10 by 3` yields `0, 3, 6, 9`, settling the alignment question, the
  loop stops at the last value within the bound because the condition says so; `10..0 by -2`
  descends and includes `0`; `restart()` is re-invocation (§2). A range is a stream because
  the desugar returns one; there is nothing else to specify.

---

## 5. Infinite ranges

An **open-top** range `lo..` (no upper bound) is an **infinite stream** from `lo` upward. It is
valid only in value position (a stream needs a starting point), and is bounded lazily by a
downstream `take` or `filter`:

```
let firstTenPrimes = 1.. |> filter(isPrime) |> take(10);    // no known upper bound; take bounds it
```

The use is "I need a sequence but do not know (or want to compute) the upper bound," relying on
laziness plus `take` to make it finite. Because a range is a stream and streams may be infinite
(stream spec), this costs nothing extra.

Open-*bottom* (`..hi`) and fully-open (`..`) forms are **not** value-position ranges (a stream
has no start to generate from); those "from the start" / "to the end" needs belong to **slicing**
(`list[:hi]`, `list[:]`), which is the `:` syntax (§2.1), not `..`.

---

## 6. Element type: integers only

Ranges are over **integers** only for now. Ranges over other types, notably characters
(`'a'..'z'`), are deferred: Luna has no `char` type, and defining character ranges over UTF-8
(codepoints? graphemes?) is genuinely involved (string spec). So `..` is int-only, and other
element types wait for their own treatment.

---

## 7. Summary

- `lo..hi` is **inclusive** (`1..10` is 1 through 10); `lo..<hi` excludes the top (`0..<n` is
  the valid indices of `n` elements). No bottom-exclusive form.
- **Value position → a `stream`** of ints (lazy, low-memory, foreach-able with enumeration keys,
  pipeable, restartable). **Match position → a membership test** (endpoints, no stream).
- **Slicing is separate** (`:`, half-open, tables and bytes specs), so `..` is always inclusive.
- `by step` gives a constant positive step; **descending** when `lo > hi`; **`lo..`** is an
  infinite stream bounded by `take`.
- **Integers only**; character and other ranges deferred.

---

## 8. Open questions

- **Range in more positions:** whether `..` ranges are useful anywhere beyond value position and
  match (they are not used in slicing, §2.1), pending experience.
- **Character and other ranges:** ranges over codepoints or a future `char`/`float` type, and
  what "contiguous" means for each.
- **Reified range for a bound API:** whether any operation ever needs a range's bounds as data
  (not a stream), which would want a lightweight bounds pair rather than consuming the stream;
  currently no such need (match reads endpoints syntactically).
