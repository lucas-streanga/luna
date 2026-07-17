# Spread

`...x` in **value-contributing positions** spreads a table or stream into individual
elements. It is the same token as destructuring's rest element, disambiguated by position
exactly like the rest of the `&`/`!`/`@` family: in a **pattern**, `...rest` collects
(destructuring §1.2); in a **literal, argument list, command literal, or interpolation**,
`...x` contributes. (A third position, `...name` in a *parameter* list, declares a variadic;
that form is used across the std surface and is not yet specified, §7.)

One distinction decides everything below. A **table literal** is the only spread context with
a **key slot**: keys survive it, under the fold of §1. **Sequence contexts**, call arguments,
command argv, and interpolation, have no key slot: they require a `list` (§4). Nothing
reindexes silently (tables §2.3).

## 1. Spread in a table literal: the fold

`...expr` inside a table literal inserts all of `expr`'s entries at that point, and spreads
may sit among literal elements:

```
let combined  = [...first, ...second];
let withExtra = [head, ...middle, tail];
```

Spread folds **left to right**, maintaining a result table and a running **next integer
index**. For each contributed entry, in its source's own iteration order (keys are `int` or
`string` and nothing else, tables §3.1):

- **An integer-keyed value reindexes:** it is appended at the running next index, which then
  increments. Integer keys from the source are *not* preserved; they are renumbered onto the
  running counter. So integer keys never collide and never overwrite, they always append.
- **A string-keyed value merges:** it is written at its own key, overwriting any existing
  value there. String keys keep their identity, so a later spread's `'x'` overwrites an
  earlier `'x'`.

Literal elements between spreads follow the same rule: a bare element (`head`) appends at the
next integer index; a `'k' => v` element merges by key. There is no "list mode" versus "map
mode" that changes later processing; every element is folded by the same rule, and the
result's shape is emergent.

Integer-reindex-and-append is what makes spread **concatenation**; string-merge-and-overwrite
is what makes it **merge**. One construct expresses concat, merge, and insert-in-the-middle
(`[first, ...rest, last]`); this is why `+` on tables stays a type error (operators §0.4),
structural combination is spread, arithmetic is arithmetic.

Entries are contributed by **value** (copy-on-write, tables §4): the new table shares
nothing observable with the sources.

### 1.1 Worked example

```
let first  = [10, 20];            // entries 0=>10, 1=>20
let second = [30];                // entries 0=>30
let mixed  = ['x'=>99, 5=>50];    // a string key and a non-contiguous integer key
let third  = [40, 50];

[...first, ...second, ...mixed, ...third]
```

Folding left to right (running next index in brackets):

1. `...first` [0]: append 10, 20. Result `[0=>10, 1=>20]`. Next index 2.
2. `...second` [2]: append 30. Result `[0=>10, 1=>20, 2=>30]`. Next index 3.
3. `...mixed` [3]: `'x'=>99` merges at `'x'`; integer key 5 **reindexes** to 3. Result
   `[0=>10, 1=>20, 2=>30, 'x'=>99, 3=>50]`. Next index 4.
4. `...third` [4]: append 40, 50 at indices 4, 5. Next index 6.

Final: `[0=>10, 1=>20, 2=>30, 'x'=>99, 3=>50, 4=>40, 5=>50]`.

Two things to note. `third` does **not** overwrite earlier integer keys; it *continues
appending* at the running index, because integer keys always reindex to the next slot and
never collide. And the interleaved string key `'x'` does not consume an integer index, so the
integer keys stay contiguous (`0..5`) in insertion order with `'x'` sitting between them.

### 1.2 List-ness of the result is emergent

A table is a **list** iff its keys are exactly `0..n-1` and nothing else (tables §2). Spread
preserves or breaks this:

- If **every** contributed entry is integer-keyed, the result stays a list (integer keys
  reindexed contiguously from zero).
- As soon as **any** entry is string-keyed, the result is a general **table**, not a list,
  even though its integer keys remain contiguous. The §1.1 result has `'x'`, so `isList()` is
  false: a valid table with mixed keys, just not a list.

List-ness is emergent from the fold, not a mode that alters it: later integer-keyed spreads
still append normally whether or not a string key already broke list-ness. Because list-ness
is a maintained O(1) property of a table (tables §2.2), the fold can check "is this element a
list?" cheaply per element, though it does not need to: it applies the same
integer-reindex / string-merge rule regardless.

### 1.3 Equivalent to `merge`, and to concatenation

Spread is the literal-syntax form of `merge`; `merge` is the function form. They agree
because `merge` folds the same way: integer keys append, string keys overwrite
(iterable-functions §2.7, `preserveKeys = false`, the combiner default it shares with
`flatten`).

```
[...a, ...b]        // the same fold as a.merge(b)
[...list1, ...list2]   // for list operands: concatenation, of their combined length
```

*(`merge(preserveKeys: true)` is the other operation, layering `b`'s entries onto `a`'s at
their own integer keys so they collide and overwrite. That is not spread, and it is not the
default; a combiner whose integer keys collide across sources is the same hazard `flatten`
names when it reindexes across levels.)*

## 2. Spreading a stream consumes it, entirely and eagerly

A stream is a lazy sequence, so `[...s]` **materializes** it: spread pulls the stream **to
exhaustion, eagerly, at the point of the literal**, and contributes each element in order. A
stream is lazy-start (stream §1.2), so the generator's body does not run at all until the
spread pulls it. A stream's `k => v` elements enter the fold as keyed entries (§1): implicit
integer keys (R93) reindex-append exactly as a list's keys do; explicit keys follow the
ordinary fold rule.

Precisely, and this is the part that is easy to get wrong: **spread is the *fold* of the
collection, not the collection.** `collect(s)` preserves a stream's keys (iterable-functions
§2.11); the fold of §1 reindexes integer keys. So `[...s]` is `[...collect(s)]`, never bare
`collect(s)`: for a stream carrying **explicit integer keys** the fold reindexes what
`collect` would keep, and that is the whole difference. For implicit keys (R93) and string
keys the two coincide.

This is foreach-class consumption (stream §2): the stream is exhausted afterward, not taken,
iterating it again yields nothing. Two consequences, both the programmer's deliberate choice:
an **unbounded stream spreads into unbounded memory** (bound it first, `[...s |> take(n)]`),
and spreading is the opposite of the pipeline's laziness, use it exactly when you *want* the
whole sequence in hand now.

In a **sequence context** (§4), a stream must be **list-like** — implicit, undisturbed
integer keys (R93), precisely the stream analogue of a `list`; an explicitly-keyed stream
needs `values()` first, exactly as a keyed table does.

## 3. One level, always; depth is the flatten API's job

Spread contributes a table's **entries**, and an entry that is itself a table arrives as
**one value**. There is no deep or recursive spread (`[......t]` is not a thing), because
depth is a *policy*, and policies get functions, not operators:

```
fn flatten(it: iterable, depth: int = -1, preserveKeys: bool = false): iterable
```

`flatten` (iterable-functions §2.5) is the explicit form, and its policy knobs are the
argument: how deep (`depth = -1` is fully) and whether keys survive. Neither fits in a
token. (The former `onNoGet` and `asStream` knobs are gone with their models, R92/R98:
output kind follows the input, and element space carries no permissions.) It composes with
spread when both are wanted (`[...flatten(t, 2)]`), and it reindexes by default for the
same reason spread's integer keys reindex: keys collide across levels.

## 4. Spread into call arguments

`f(...args)` spreads a **list** into **positional arguments**, the PHP lineage. The
requirement is a `list`, and spread introduces **no new rule** to enforce it: `f(...xs)`
demands exactly what `someFn(xs)` demands of a `list` parameter, because narrowing is never
implicit (tables §2.3).

- **Static `list`:** accepted.
- **Static `table`:** a **compile error**, the `someFn(tab)` error of tables §2.3. Adding
  `...` must not launder a table into a list; that would make the spread token the one
  implicit `table → list` narrowing in the language.
- **Dynamic** (`any`, or a table reached through an erased boundary): the `as list` check runs
  at the call, and a `typeError` panic if it is not a list.

The two ways to satisfy it are the two already ruled, and they say which they mean:
`f(...(t as list))` **asserts** that a non-list here is a bug; `f(...t.values())` **produces**
a list from any table, dropping the keys. A string key names something no argument position
can receive, and an integer key that is not its own position would have to be renumbered,
which is the silent reindex tables §2.3 forbids. Named-argument spreading therefore does not
arise: an argument list has no key slot.

Arity and defaults follow the ordinary call rules (functions §3.3): the spread elements fill
parameters in written order, defaults cover the tail, **surplus elements are dropped** (a
spread of ten into a two-parameter function is legal, exactly as a written ten-argument call
is), and a **deficit is an `ArityError`** (a `panic`, errors spec), a compile error where the
callee's signature and the spread's length are both statically visible, checked at the call
otherwise. A list-like stream (R93) spreads the same way, materialized (§2) before the arity check.

Two consequences worth stating:

- **Mixing is positional composition:** `f(x, ...rest, y)` fills in written order. The
  trailing-only restriction on destructuring's rest (destructuring §1.2) is a *pattern* rule
  and does not apply here; a value spread is just elements in written order.
- **Spread contributes by value** (§1), so `...` can never fill a `&` reference parameter:
  `&` marks a whole named argument (variables §5.1, operators §0.2), never a spread element.
- **A string-keyed table never spreads into named arguments** (R108): spread into an
  argument list is a sequence context and requires a `list`, full stop. Named arguments
  (functions §3.3.2) are visible at the call site, never manufactured from data — the
  PHP 8.1 associative-spread behavior is deliberately rejected.

Note that `f(...t)` is deliberately stricter than `[...t]`, which reindexes `[5=>50]` to
`[50]` without complaint. A literal is *building* a table, where integer keys carry no
positional contract; a parameter list has one. `[...t]` is not the workaround for a keyed
table, `t.values()` is, because it says what it does.

## 5. Spread in interpolation and command literals

`"${...xs}"` interpolates **each element in sequence**, every element rendered by
**`toString`** exactly as scalar interpolation renders (`"${x}"`, conversion §3), so spread
interpolation is **total**: it is precisely the loop of `"${x}"`s it abbreviates, and display
never fails. No separator is inserted; a separator is `join`'s job (`"${join(', ', xs)}"`,
strings §8). Interpolation is a sequence context, so `xs` is a `list` (or a list-like
stream) under §4's rule. *(A stricter rendering rule, panic unless every element is already a
`string`, was considered and rejected: it would make this the one non-total rendering form in
the language, diverging from the scalar interpolation it claims to abbreviate; strictness at a
boundary is spelled `as`, upstream.)*

The braced form is the **only** form. There is no bare `"$...xs"`: the lexer's `$name`
interpolation is `\$[A-Za-z_][A-Za-z0-9_]*` (`INTERP_IDENT`, lexer §6), which `$.` cannot
match, and the spread splice is defined only inside `${ }` (`INTERP_OPEN` + `SPREAD`). It has
to be this way, since bare `$name` is `DQ_STRING`-only while spread must also work in command
literals.

In a **command literal**, `${...flags}` spreads a list into **that many arguments** (command
§3): one element, one argv entry, each rendered by the same `toString` rule, and never
re-tokenized, an element with a space is still exactly one argument, which is the whole
injection-safety point of structured commands.

## 6. Comptime spread

When the spread operands are comptime-known, the whole literal folds at compile time:

```
const combined = [...tab1, ...tab2];   // requires tab1, tab2 comptime-known
```

A `const` table literal whose spread operands are themselves `const` (or otherwise
comptime-known) is comptime-evaluable (functions §5), because the fold is a pure computation.
It folds into a `const` table at compile time, which then takes the const-table representation
(tables, Amendment A), and a `const` result that is a list takes the tighter list
representation. Nothing new is needed; comptime spread is ordinary const-table construction
whose parts happen to be spreads.

## 7. Open questions

- *(**Resolved by R108.** `...name: T` in a parameter list is specified in functions
  §3.3.3 — the pattern rest element in parameter position, so the R35 unification holds
  after all: the variadic is the trailing rest of the *positional* sublist, with only
  defaulted, named-only parameters after it. The `*,` marker is retired, `name?` is
  defined in functions §3.3.1, and named arguments landed in §3.3.2. The token's three
  positions are all specified.)*
- **Spread of `bytes` / `string`**: whether `[...someBytes]` yields a table of `byte`
  elements (plausible, cheap) or is an error pending a use case; deferred.
