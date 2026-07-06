# Spread

`...x` in **value-contributing positions** spreads a table or stream into individual
elements. It is the same token as destructuring's rest element, disambiguated by position
exactly like the rest of the `&`/`!`/`@` family: in a **pattern**, `...rest` collects
(destructuring §1.2); in a **literal, argument list, command literal, or interpolation**,
`...x` contributes. This file is the spec the corpus already cited (tables §2.2, operators
§0.1, command §4).

## 1. Spread in table literals: inline the entries

`[...a, ...b]` contributes `a`'s entries as if written inline at that point: the
**list-part values arrive as positional values** (reindexed in literal order, which is what
makes spread *concatenation*), and the **keyed entries arrive keyed** (later writer wins,
the ordinary literal rule, which is what makes spread *merge*). One construct expresses
concat, merge, and insert-in-the-middle (`[first, ...rest, last]`); this is why `+` on
tables stays a type error (operators §0.1), structural combination is spread, arithmetic is
arithmetic.

Entries are contributed by **value** (copy-on-write, variables §5.2): the new table shares
nothing observable with the sources.

## 2. Spreading a stream consumes it, entirely and eagerly

A stream is a lazy sequence, so `[...s]` is the **materialization**: spread pulls the stream
**to exhaustion, eagerly, at the point of the literal**, and contributes each element in
order (a keyed generator's `k => v` elements arrive keyed, §1). This is foreach-class
consumption (stream §2): the stream is exhausted afterward, not taken, iterating it again
yields nothing. Two consequences, both the programmer's deliberate choice: an **unbounded
stream spreads into unbounded memory** (bound it first, `[...s |> take(n)]`), and spreading
is the opposite of the pipeline's laziness, use it exactly when you *want* the whole
sequence in hand now.

## 3. One level, always; depth is the flatten API's job

Spread contributes a table's **entries**, and an entry that is itself a table arrives as
**one value**. There is no deep or recursive spread (`[......t]` is not a thing), because
depth is a *policy* (how deep? preserve keys?) and policies get functions, not operators:
**`flatten(tab, depth, preserveKeys)`** (table-api) is the explicit form, composing with
spread when both are wanted (`[...flatten(t, 2)]`).

## 4. Spread into call arguments

`f(...args)` spreads a table into **positional arguments**, the PHP lineage:

- `args` must be **list-shaped** (keys `0..n-1`): checked statically where the type says
  (`list`), a **panic** at the call otherwise, positional parameters have no meaning for
  string keys (named-argument spreading does not exist because named arguments do not).
- Arity and defaults follow the ordinary call rules (functions §3.3): the spread elements
  fill parameters in order, defaults cover the tail, and a length that cannot be known
  statically is **arity-checked at runtime**, panic on mismatch, the same error the written
  call would get.
- Mixing is positional-literal composition: `f(x, ...rest, y)` fills in written order.

## 5. Spread in interpolation and command literals

`"$...xs"` (and the braced `"${...expr}"`) interpolates **each element in sequence**, every
element rendered by **`toString`** exactly as scalar interpolation renders (`"$x"`,
conversion §3), so spread interpolation is **total**: it is precisely the loop of `"$x"`s it
abbreviates, and display never fails. No separator is inserted; a separator is `join`'s job
(`"${join(', ', xs)}"`, strings §8). *(A stricter rule, panic unless every element is
already a `string`, was considered and rejected: it would make this the one non-total
rendering form in the language, diverging from the scalar interpolation it claims to
abbreviate; strictness at a boundary is spelled `as`, upstream.)*

In a **command literal**, `${...flags}` spreads a list into **that many arguments** (command
§4): one element, one argv entry, each rendered by the same `toString` rule, and never
re-tokenized, an element with a space is still exactly one argument, which is the whole
injection-safety point of structured commands.

## 6. Open questions

- **Spread of `bytes` / `string`**: whether `[...someBytes]` yields a table of `byte`
  elements (plausible, cheap) or is an error pending a use case; deferred.
