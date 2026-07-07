# Regex

A regular expression in Luna is its **own type**, `regex`, not a string. It is written
with a dedicated literal, `/.../ `, compiled once, and passed and stored as a first-class
value. Making it a distinct type gives better ergonomics (a function that wants a pattern
declares `regex`, not `string`), compile-time validation of literal patterns, and a hard
guarantee about matching cost. This document specifies the type, its literal and flags, the
engine guarantees, and how a `regex` is built from a runtime string. How strings *consume* a
regex (`matches`, `find`, `findAll`) is in string-api §5; this document defines the regex
side.

---

## 1. `regex` is its own type

`regex` is a distinct type: a compiled matcher, not a string. The benefits follow directly:

- **Compiled once.** A `/.../ ` literal is compiled to a matcher at compile time, not
  recompiled from a string on each use.
- **Type-enforced.** A function taking `regex` cannot be handed an arbitrary string; "this
  is a validated pattern" is guaranteed by the type. A malformed literal is a **compile
  error at the literal**, not a runtime surprise.
- **First-class.** A `regex` is bound, passed, stored, and returned like any value.

```
let year = /\d{4}/;          // year : regex, compiled once
matchYear(text, year);       // passed as a regex, not a string
```

`regex` is not a table and has no protocol applieds; it is a primitive value type like `string`
or `int`, with its own literal.

---

## 2. The `/.../ ` literal

A regex literal is delimited by `/`, and its type is inferred as `regex`:

```
let digits = /\d+/;
let word   = /[A-Za-z_]\w*/;
```

- A literal `/` inside the pattern is escaped as `\/`. Patterns heavy in slashes are
  usually more readable in verbose mode (§4).
- The pattern between the delimiters is the regex source; escapes (`\d`, `\w`, `\/`, and
  the rest) follow the engine's syntax (§5).
- A malformed pattern (`/[/`) fails to compile **at the literal**, a compile error, because
  the literal is compiled at compile time.
- An **empty pattern** is written `/(?:)/`, not `//`: a bare `//` is a line comment (lexer
  §2, §8), so the empty matcher uses an explicit empty group — the spelling JavaScript
  adopted for the same reason. `regex("")` is the runtime equivalent (§6).

Flags follow the closing delimiter (§3).

---

## 3. Flags

Flags are letters after the closing `/`, and they modify how the pattern is compiled or
which engine runs it. They compose (`/.../im`).

| Flag | Meaning |
|-|-|
| `i` | Case-insensitive (full Unicode case folding). |
| `m` | Multiline: `^` and `$` match at line boundaries, not just string start/end. |
| `s` | Dotall: `.` matches a newline as well. |
| `x` | Verbose / extended: whitespace in the pattern is ignored and `#` starts a comment (§4). |
| `b` | Backtracking engine: enables backreferences and lookbehind, at the cost of the linear-time guarantee (§5.2). |

```
/hello/i             // case-insensitive
/^\d+$/m             // multiline anchors
/(\w+)\s+\1/b        // backreference: requires b (§5.2)
```

Global-versus-single matching is **not** a flag: it is a call choice in the string API
(`find` returns the first match, `findAll` returns all, string-api §5), because whether you
want one match or every match is a property of the operation, not of the pattern.

---

## 4. Verbose mode: readable patterns

The `x` flag makes a pattern readable by ignoring whitespace and allowing `#` comments, so a
long pattern can be spelled out across lines instead of packed onto one:

```
let isoDate = /
  \d{4}    # year
  -
  \d{2}    # month
  -
  \d{2}    # day
/x;
```

This is the same regex language, not a second syntax: `x` only changes how whitespace and
`#` are treated during compilation. Literal whitespace in a verbose pattern is matched with
`\ ` (escaped space) or a class like `[ ]`. Verbose mode is the intended way to write long
or slash-heavy patterns legibly.

---

## 5. Engines and the matching guarantee

The `regex` type is backed by an engine that is an **opaque implementation detail**: the
programmer writes `/.../ ` and consumes matches, never sees the engine, and the engine may
be swapped without any language-surface change. Two engines exist, selected by the `b` flag.

### 5.1 The default engine: linear-time, safe

Without `b`, a regex uses a **linear-time automaton engine** (currently Go's `regexp`, which
is an RE2-family engine; the specific library is an implementation note, the *guarantee* is
the spec). It provides two guarantees that together make it safe even for patterns built
from untrusted input:

- **Matching is O(n) in the input**, for **any** pattern. There is no catastrophic
  backtracking, because the engine does not backtrack. A malicious pattern cannot cause a
  ReDoS-style blowup.
- **Compilation is bounded** by a pattern-size limit: a pattern whose compiled form would be
  too large is rejected at compile time (for a literal) or throws (for a runtime-compiled
  pattern, §6), rather than exhausting memory.

Because both axes are bounded, a `regex` is **safe to build from untrusted input**: neither
the match nor the compilation of an attacker-chosen pattern can be made to blow up. The one
thing this engine does **not** support is backtracking-only features (backreferences,
arbitrary lookbehind), which are not expressible in a linear-time automaton.

**Unicode.** The default engine supports Unicode **general categories** and **scripts** in
property escapes: `\p{L}`, `\p{Lu}`, `\p{N}`, `\p{Han}`, `\p{Greek}`, the `\pL` shorthand,
and the negated `\P{...}`. This covers essentially all real use. More exotic property
classes (script extensions, emoji and grapheme-break properties, derived binary properties)
are **not** guaranteed; they depend on the engine and the bundled Unicode tables
(string-representation §8).

**Regex matches codepoints, not graphemes.** `.`, character classes, and `\p{...}` operate
on Unicode **scalar values** (codepoints), not on grapheme clusters. This differs from the
rest of the string API, where the user-facing "character" is a grapheme (string-api §2,
§9). So regex `.` matches one codepoint, and a single user-perceived character that spans
several codepoints (a combining sequence, a ZWJ emoji cluster) is *multiple* units to a
regex. This is inherent to how regex engines work and is worth keeping in mind when a
pattern is meant to match "one character."

### 5.2 The `b` engine: backtracking, under contract

The `b` flag opts into a **backtracking engine** that adds the features the automaton cannot
express: **backreferences** (`\1`) and **lookbehind**. Backtracking engines have an
exponential worst case, so `b` gives up the O(n) guarantee, and a `/b` pattern therefore
runs under a **deterministic step budget** (§5.3) so that even a pathological match aborts
with an error rather than hanging.

The `b` engine is bound by a strict **semantic-compatibility contract**:

> The `b` engine must behave **identically** to the default engine on every pattern the
> default engine accepts. `b` may only **add** behavior (backreferences, lookbehind); it may
> **never change** the meaning, the match, or the captures of a pattern that already works
> without `b`.

This is deliberate: `b` looks like a small addition to a pattern (one letter), so it must
*be* a small addition. Adding `b` to enable a backreference must not silently alter how the
rest of the pattern matches. A `/re/` and `/re/b` that share the pattern `re` must produce
the same result on the same input whenever `re` is expressible without `b`.

The contract constrains engine choice: an off-the-shelf backtracking engine (PCRE, Perl)
whose semantics diverge from the default's (in alternation order, empty-match handling,
capture semantics, and the like) does **not** satisfy it as-is. Meeting the contract may
require a purpose-built backtracking engine that matches the default engine's semantics and
adds only the backtracking features. That cost is accepted; silent semantic divergence
between `/re/` and `/re/b` is not, because it would make `b` a footgun.

### 5.3 The step budget on `b`

A `/b` pattern's match runs under a **deterministic step budget**: a ceiling on backtracking
work, counted in steps, not wall-clock time (so it is machine-independent and reproducible,
the same reasoning as the comptime budget, functions §5.5). A match that exceeds the budget
**throws** rather than hanging the process. The budget is a backstop, not a correctness
mechanism: it bounds the exponential worst case so that opting into `b` cannot deadlock a
program, only fail loudly. The default engine needs no such budget, because it is linear by
construction.

---

## 6. Building a regex from a runtime string

A `/.../ ` literal is compiled at compile time. To compile a pattern known only at runtime
(from configuration, or from user input), use the constructor:

```
fn regex(pattern: string, flags: string = ""): regex!
```

- `regex(userPattern)` compiles `userPattern` into a `regex` at runtime.
- It is **errorable** (`regex!`): a malformed pattern, or one that exceeds the compile-size
  limit (§5.1), throws, so the caller handles a bad pattern with `try` or an errorable
  binding.
- `flags` supplies the same flags as a literal (`"i"`, `"im"`, `"b"`, and so on) as a
  string, since there is no literal delimiter to attach them to.

```
let r = try regex(config.pattern, "i");    // compile a user pattern, handle failure
```

Because the default engine is safe on both axes (§5.1), compiling and matching an untrusted
runtime pattern is safe: the worst a malicious pattern can do is fail to compile (caught by
`regex!`) or match in linear time. A runtime pattern that requests `b` opts into the
backtracking engine and its step budget like any other `b` pattern.

---

## 7. Interpolation into a literal

A `/.../ ` literal may interpolate `${expr}`, but only under one condition that preserves the
literal's defining guarantee (it always compiles at compile time):

> `${expr}` in a regex literal is allowed **iff `expr` is comptime-evaluable to a string**
> (functions §5), so the full pattern is known at compile time and the whole literal
> compiles at compile time.

If `expr` is not comptime-known, the literal is a **compile error** directing the author to
`regex()` (§6), the runtime path. This keeps the two paths cleanly exhaustive: `/.../ ` is
always compile-time-compiled (never errorable at runtime), and `regex()` is the runtime path
(errorable). Interpolation never blurs that line, because it is permitted only up to the
point where the literal would still compile at compile time.

**Interpolation composes pattern source.** A `${expr}` splices the comptime string in **as
regex source**, so patterns can be assembled from comptime pieces:

```
const DIGITS = /\d+/.source;           // comptime-known pattern fragment
let ipPart   = /${DIGITS}\.${DIGITS}/; // composed at compile time, compiled once
```

Two properties follow, and both are why the comptime restriction is worth it:

- **Regex injection is impossible through a literal.** User input is by definition not
  comptime-known, so it can never be interpolated into a `/.../ ` literal. Any runtime or
  untrusted pattern must go through `regex()`, the explicit, errorable, visible path. So
  "regex injection" can only happen deliberately and visibly, never by accident through
  interpolation.
- **All literal compilation stays off the runtime.** Even an interpolated literal is
  compiled once, at compile time; interpolation creates no runtime-compilation exception.

Because interpolation splices *source*, a comptime fragment containing regex metacharacters
changes the pattern's meaning (that is composition, the intended use). To splice a comptime
string as **literal text to match** (escaping metacharacters) rather than as source, use
`regex.escape` inside the interpolation:

```
fn escape(s: string): string        // in the regex module; escapes all regex metacharacters
```

`regex.escape("a.b")` returns `"a\.b"`, so `/${regex.escape(name)}/` matches the literal text
of `name` rather than treating its metacharacters as pattern syntax. `regex.escape` is a pure
function, so it stays comptime-evaluable and preserves the compile-time-compilation guarantee
inside a literal. It is also the tool for escaping a runtime string before `regex()`, e.g.
`regex("${regex.escape(userText)}\\d+")`. No new syntax is needed: bare `${expr}` composes
(raw source), `${regex.escape(expr)}` escapes (literal text). The string API exposes a
delegating convenience, `string.regexEscape` (string-api §5), for reaching this from
string-land.

---

## 8. Consuming a regex

Strings consume a `regex` through the string API (string-api §5), not through methods on the
regex:

- **`matches(str, pattern)`** , whether the pattern matches anywhere in the string.
- **`find(str, pattern)`** , the first match, as a table of capture groups, or `null`.
- **`findAll(str, pattern)`** , all matches, as a stream or table of such tables.
- A `replace` variant taking a `regex` target rides on the `replace` union (string-api §7).

The regex is the second argument (the string is the receiver), so these read as
`text->find(pattern)` under UFCS. The shape of a match result (capture groups, named
captures, positions) is specified with those functions in the string API.

---

## 9. Open questions

- **Named captures:** the syntax for named capture groups (`(?<name>...)`) and how a match
  result exposes them (as keyed fields in the result table), pending the match-result shape
  in the string API.
- **The `b` engine choice:** whether an existing backtracking engine can satisfy the
  semantic-compatibility contract (§5.2) or a purpose-built one is required; deferred, but
  constrained by the contract.
- **Step budget default and override:** the default value of the `b` step budget (§5.3) and
  whether it is tunable per match or only globally.
- **Alternate delimiters:** whether a slash-heavy pattern may use an alternate delimiter, or
  whether `\/` plus verbose mode (§4) is sufficient (current assumption: sufficient).
