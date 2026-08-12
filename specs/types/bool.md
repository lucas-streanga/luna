# Bool

`bool` is Luna's boolean type: a **two-valued** primitive, `true` or `false`, stored **inline in
the `lval`** like the other scalars. It is the simplest type in the language, one bit of
information, two literals, no coercion.

```luna
let ok: bool = true;
let done = false;
```

- **Two values only:** `true` and `false`. There is no third state; a `bool` is never null unless
  its binding is explicitly optional (`bool?`), and even then the null is the binding's, not a
  third boolean value.
- **Inline:** a `bool` carries one bit of information in the `lval`'s scalar word
  (value-representation §1, §1.1), copied by value, never allocated.
- **Literals:** `true` and `false`.

---

## 1. No truthiness: conditions require a `bool`

Anywhere a boolean is expected, a condition in `if` / `while`, a `match` guard, an operand of a
boolean operator, the value **must be a `bool`**. There is **no truthiness**: non-boolean values
are not implicitly treated as true or false.

```luna
if (count > 0) {}        // ok: a bool
if (count) {}             // ERROR: int is not a bool
if (name != "") {}        // ok: write the comparison
if (handle != null) {}    // ok: explicit null check
```

This is deliberate and consistent with the language's no-magic stance: `if (count)` is ambiguous
(is `0` false? a negative? an empty string?), and every language that coerces has to answer those
awkwardly. Luna answers by not coercing, you state the condition you mean (`count > 0`, `name !=
""`, `x != null`), so the boolean is always explicit and there is never a hidden truthiness rule
to remember.

---

## 2. Operators

The boolean operators take and produce `bool`:

- **`&&`** (logical and), **`||`** (logical or), both **short-circuiting**: the right operand is
  evaluated only if needed (`a && b` skips `b` when `a` is false; `a || b` skips `b` when `a` is
  true). Short-circuiting matters when the right operand has effects or could fail, it is not
  evaluated unless reached.
- **`!`** (logical not), **prefix**: `!true` is `false`. (Postfix `!` is the errorable type
  suffix, a different, position-resolved role — operators §0, type §1.1.)

All three require `bool` operands (no truthiness, §1) and yield a `bool`. Comparisons (`==`,
`!=`, `<`, `>`, and so on) yield a `bool`, and are how non-boolean values enter boolean logic (a
comparison, not a coercion).

---

## 3. Conversions are functions, never `as`

A `bool` does **not** implicitly coerce to or from `int` or `string`, and conversions are
**functions**, not `as`. This follows the language's rule that `as` is checked **narrowing** and
**never transforms a value** (`as` spec): turning `true` into `1` or `"true"` produces a
*different value* of a different type, which is a transformation, so it is a function, exactly as
`toString` and `parseInt` are functions rather than `as` forms.

```luna
_ = b.toInt();    // true -> 1, false -> 0   (total; UFCS, same as toInt(b))
_ = b.toString(); // true -> "true", false -> "false"   (total; the ordinary toString)
_ = parseBool(s); // "true" -> true, "false" -> false, else an error: bool!   (fallible)
```

- **`toInt(b): int`** , `true` to `1`, `false` to `0`. Total (always succeeds). Reachable as
  `toInt(b)` or `b.toInt()` (UFCS), the same function either way.
- **`toString(b): string`** , `true` to `"true"`, `false` to `"false"`. Total; this is the same
  `toString` that applies across types, not a bool-specific operator.
- **`parseBool(s): bool!`** , parses a string to a `bool`, fallible (a declarable error on
  anything but the accepted spellings), like `parseInt`. **The accepted spellings are ruled
  (R186): exactly the two words `true` and `false`, case-insensitively** — `"tRue"`,
  `"FALSE"`, `"False"` all parse; one word, any case, because booleans arrive from a casing
  zoo (JSON `true`, Python `True`, SQL and INI `TRUE`) and the value set is two unambiguous
  words. Everything else errors, with two deliberate rejections:
  - **`"1"` / `"0"` are rejected** — those are *ints inside strings*, and accepting them
    would be the truthiness policy this spec refuses (§1, §3) arriving at the text
    boundary. The spelling is the composition: `parseInt(s)` and then the comparison you
    mean (`n != 0`, `n == 1`), each step visible.
  - **Surrounding whitespace is rejected** — `" true"` errors. Parsing is exact; trimming
    is the caller's explicit, visible step (`s.trim()`, string-api §6). This is the `parse*`
    family's general stance: the function parses the text it is given, never a cleaned-up
    version of it.

There is deliberately **no `int` to `bool` conversion**: rather than pick a truthiness policy (is
`0` false and everything else true? only `0` and `1`?), you write the comparison you mean (`n !=
0`), consistent with the no-truthiness rule (§1). So integers enter boolean logic through a
comparison, never a conversion.

Using `as` for any of these would be a mistake, `as` asserts a type without changing the value,
while these change the value, so they are functions. `true as int` is not valid; `true.toInt()`
is.

---

## 4. Bool as a key and in tables

A `bool` is a valid, well-behaved value in every ordinary position, an element, a payload, a
field. Its equality is total and trivial (`true == true`, `false == false`, and they are
distinct), so unlike `double` it has no equality hazard. Whether it is a permitted **table key**
follows the key-type rule (keys are `string` or `int`, tables spec), a `bool` is not one of those,
so it is not a key directly; key by its `toInt()` (0 or 1) or `toString()` if a boolean-keyed
mapping is wanted, though a two-entry table keyed on a bool is usually better expressed as two
named fields or a `match`.

---

## 5. Resolved and deferred

- **Bitwise-style boolean operators: deferred** (R186). Non-short-circuiting forms
  (evaluating both operands always) are rarely needed — most code wants short-circuit, and
  explicit sequencing spells the rest — so they wait for a demonstrated need, alongside the
  integer bitwise design (int §8, operators §0.4).
- *(**`parseBool` accepted spellings: resolved by R186** — exactly the two words,
  case-insensitive; `"1"`/`"0"` and whitespace rejected, each deliberately; §3.)*
