# Luna — Lexer Specification

Token inventory for the Luna lexer, cross-referenced from every spec in the repository
(keywords.md is the keyword authority; lexical-structure.md the encoding, identifier,
and comment authority; operators.md and associativity.md the operator catalogue; literal
forms from int.md §7, double.md §8, string.md §5, bytes.md §7, regex.md §2–3,
command.md §2–3; punctuation confirmed against every code sample in the corpus). Regexes target **Go's `regexp` package (RE2)**.

Conventions used throughout:

- Input must be valid UTF-8; the lexer rejects anything else before tokenizing
  (lexical-structure §1). Every pattern below then operates on ASCII bytes only.
- Patterns are written unanchored. In the scanner, anchor with `\A` (Go supports it) and
  match at the current offset.
- The lexer emits **one token per lexeme**. Tokens with positional dual roles (`!` not vs.
  errorable, `&` reference vs. intersection, `@` type-of vs. refinement, `?` in `a?.b` vs.
  `T?`) are a **parser** concern; the grammar separates expression position from type
  position (operators §0.2, associativity §2), so the lexer never disambiguates them.
- Longest match wins. Where patterns overlap, the attempt order in §8 is normative.
- Newlines are not significant; statements are `;`-terminated (every spec example).

## 1. Lexer modes

Three literal forms admit `${expr}` interpolation whose body is a full Luna expression —
including nested strings, braces, and (in principle) nested literals of the same kind — so
those literals are **not regular languages** and cannot be tokenized by any single RE2
pattern (see flags F1–F3). The lexer therefore runs a small mode stack:

| Mode | Entered by | Left by |
|-|-|-|
| `DEFAULT` | start of file | — |
| `DQ_STRING` | `"` (`DQ_OPEN`, §6) | unescaped `"` (`DQ_CLOSE`) |
| `REGEX_BODY` | `~"` (`REGEX_OPEN`, R237) | unescaped `"`, then flags (`REGEX_CLOSE`) |
| `COMMAND` | `` ` `` (`CMD_OPEN`) | unescaped `` ` `` (`CMD_CLOSE`) |
| `INTERP_EXPR` | `${` inside any of the three modes above | the `}` that returns brace depth to zero (depth counted by the lexer) |

`INTERP_EXPR` lexes with the full `DEFAULT` rule set. Single-quoted strings and `b"..."`
bytes literals do **not** interpolate (string §5, bytes §7) and are handled by a single
regex each.

## 2. Whitespace and comments — the trivia tokens

| Token | Name | Go regex (RE2) |
|-|-|-|
| spaces, tabs, newlines | `WHITESPACE` | `[ \t\r\n]+` — one token per maximal run |
| `#!…` (first line only) | `SHEBANG` | `\A#![^\n]*` |
| `// …` | `LINE_COMMENT` | `//[^\n]*` |
| `/* … */` | `BLOCK_COMMENT` | `(?s)/\*.*?\*/` |

**These four are emitted, not skipped (R236)**, and collectively they are the **trivia**
tokens. They carry no meaning — whitespace is insignificant and comments are inert
(lexical-structure §1, §3) — so every consumer but one drops them; the lexer emits them anyway,
because the formatter (`luna -f`, compiler §0.1) cannot reproduce what the lexer discarded, and
it is the only component that needs them. The parser filters trivia from its view, which is one
predicate over the stream and cheaper than maintaining a second lexer mode that suppresses them.
Fidelity rides on the **span**, not on token kinds: a trivia token carries a byte range into the
retained source (§9), so one `WHITESPACE` run reports its exact bytes — tabs versus spaces, `\n`
versus `\r\n` — with no need for per-character token kinds to distinguish them.

Two consequences are worth stating. **Token spans now tile the source exactly** — monotonic, no
gaps, summing to the file length — which promotes a partial invariant into a total one for
anything checking positions. And **trivia attachment is deliberately not decided here**: the
stream is flat, with no binding of a comment to a neighbouring token as leading or trailing
matter, because whether a trailing `// …` belongs to the line above or the line below is a
formatting policy and the formatter spec is pending. A flat stream lets that spec decide; an
attached one would freeze the answer before it is asked.

Both comment forms are specified in lexical-structure §3. Block comments are C-style and do
**not** nest, by ruling, so the non-nesting pattern above is the complete rule (F4). There
is no doc-comment syntax; documentation rides on attributes.

A **shebang** — `#!` as the first two bytes of the file — spans through the next newline as
one `SHEBANG` trivia token (emitted, then dropped by every consumer but the formatter, R236),
so a `.luna` file can be made executable and run directly (R85). It is recognized
**only** at byte offset 0 (the `\A` anchor) and **only** as `#!`; a bare `#` is never a
comment (the `#`-for-comments spelling was weighed and rejected, R85), and `#` otherwise
appears solely in `#[` (attributes §3, §5).

## 3. Keywords

Reserved words per keywords.md §1–§4: 47 word-shaped keywords plus two compound tokens,
`match!` and `yield from` (R223). Recommended implementation: lex an `IDENT` (§7) and
promote via a lookup table, with the one-token peeks of §8 for the compounds, rather
than running 49 patterns; the per-word regex is given for completeness. The word-shaped
keywords all follow `\bword\b`; the two compounds carry their own patterns below.

| Token | Name | Go regex (RE2) |
|-|-|-|
| `var` | `KW_VAR` | `\bvar\b` |
| `let` | `KW_LET` | `\blet\b` |
| `const` | `KW_CONST` | `\bconst\b` |
| `fn` | `KW_FN` | `\bfn\b` |
| `gen` | `KW_GEN` | `\bgen\b` |
| `constraint` | `KW_CONSTRAINT` | `\bconstraint\b` |
| `proto` | `KW_PROTO` | `\bproto\b` |
| `enum` | `KW_ENUM` | `\benum\b` |
| `error` | `KW_ERROR` | `\berror\b` |
| `capability` | `KW_CAPABILITY` | `\bcapability\b` |
| `attribute` | `KW_ATTRIBUTE` | `\battribute\b` |
| `test` | `KW_TEST` | `\btest\b` |
| `export` | `KW_EXPORT` | `\bexport\b` |
| `import` | `KW_IMPORT` | `\bimport\b` |
| `if` | `KW_IF` | `\bif\b` |
| `else` | `KW_ELSE` | `\belse\b` |
| `foreach` | `KW_FOREACH` | `\bforeach\b` |
| `in` | `KW_IN` | `\bin\b` |
| `while` | `KW_WHILE` | `\bwhile\b` |
| `break` | `KW_BREAK` | `\bbreak\b` |
| `continue` | `KW_CONTINUE` | `\bcontinue\b` |
| `return` | `KW_RETURN` | `\breturn\b` |
| `yield from` | `KW_YIELD_FROM` | `\byield[ \t\r\n]+from\b` |
| `yield` | `KW_YIELD` | `\byield\b` |
| `match!` | `KW_MATCH_BANG` | `\bmatch!` |
| `match` | `KW_MATCH` | `\bmatch\b` |
| `where` | `KW_WHERE` | `\bwhere\b` |
| `defer` | `KW_DEFER` | `\bdefer\b` |
| `by` | `KW_BY` | `\bby\b` |
| `try` | `KW_TRY` | `\btry\b` |
| `catch` | `KW_CATCH` | `\bcatch\b` |
| `throw` | `KW_THROW` | `\bthrow\b` |
| `copy` | `KW_COPY` | `\bcopy\b` |
| `spawn` | `KW_SPAWN` | `\bspawn\b` |
| `await` | `KW_AWAIT` | `\bawait\b` |
| `comptime` | `KW_COMPTIME` | `\bcomptime\b` |
| `comptype` | `KW_COMPTYPE` | `\bcomptype\b` |
| `is` | `KW_IS` | `\bis\b` |
| `as` | `KW_AS` | `\bas\b` |
| `apply` | `KW_APPLY` | `\bapply\b` |
| `declared` | `KW_DECLARED` | `\bdeclared\b` |
| `use` | `KW_USE` | `\buse\b` |
| `true` | `KW_TRUE` | `\btrue\b` |
| `false` | `KW_FALSE` | `\bfalse\b` |
| `null` | `KW_NULL` | `\bnull\b` |
| `undefined` | `KW_UNDEFINED` | `\bundefined\b` |
| `nan` | `KW_NAN` | `\bnan\b` |
| `inf` | `KW_INF` | `\binf\b` |
| `self` | `KW_SELF` | `\bself\b` |

Notes. `in`, `by`, and `self` are contextual per keywords.md (foreach heads, range steps,
protocol bodies) but reserved everywhere, so the lexer treats them uniformly. `gen` is the
inline-generator former (R221), an ordinary full keyword. `from` is **not** reserved
(R223, superseding G1's first resolution): it lexes as `IDENT`, and its two consumers —
the import-source clause (`import { x } from m`, modules §4) and delegation — are handled
by the parser and by the compound token respectively. `match!` is listed as its own form
in operators §0 and is tokenized as one unit here, confirmed as the ruling (G7).
`yield from` is the same shape (R223): one compound token, the two words separated by
whitespace only (the regex is normative — a comment between them defeats the fold), which
is why bare-yielding a binding named `from` parenthesizes (`yield (from);`, stream §1.5).
The fold **consumes** the intervening run: `KW_YIELD_FROM`'s own span covers both words and
the whitespace between them, so no `WHITESPACE` trivia token (§2, R236) is emitted inside it —
which is the same fact the "a comment defeats the fold" clause states from the other side.
`panic`, `_`, the proto grant modifiers `get` / `set` (keywords §4 — recognized
positionally by the parser in proto member heads, R232), and every builtin type name
(`int`, `string`, `table`, …) are **not** keywords (keywords.md §4–§5); they lex as `IDENT`.

`nan` and `inf` are **keywords, not predeclared identifiers**, and this is load-bearing rather
than cosmetic: a bare identifier in a pattern is always a fresh binding (match §2.1), so if `nan`
lexed as `IDENT` the arm `match (x) { nan => ... }` (double §2.2, match §7) would bind a fresh
`nan` instead of matching one. They join `true`, `false`, `null`, and `undefined` as the **value
keywords**: word-shaped values that must never be shadowable or bindable. Like those four, they
are lowercase; the reserved set has no capitalized member.

## 4. Literals

| Token | Name | Go regex (RE2) |
|-|-|-|
| `42`, `1_000` | `INT_DEC` | `0\x7c[1-9](?:_?[0-9])*` |
| `0x1F`, `0xdead_beef` | `INT_HEX` | `0x[0-9a-fA-F](?:_?[0-9a-fA-F])*` |
| `0b0100_0001` | `INT_BIN` | `0b[01](?:_?[01])*` |
| `0o755`, `0o1_7` | `INT_OCT` | `0o[0-7](?:_?[0-7])*` |
| `3.14`, `2.5e10` | `DOUBLE` (point form) | `(?:0\x7c[1-9](?:_?[0-9])*)\.[0-9](?:_?[0-9])*(?:[eE][+-]?[0-9]+)?` |
| `6.022e23`, `1e-9` | `DOUBLE` (exponent form) | `(?:0\x7c[1-9](?:_?[0-9])*)[eE][+-]?[0-9]+` |
| `0755`, `0_1` | *(error production)* | `0[0-9_]+` — a leading zero is a **lex error** (R238), never silent decimal and never C-style octal |
| `'literal'` | `STRING_SQ` | `(?s)'[^'\\]*(?:\\.[^'\\]*)*'` |
| `"text $x ${e}"` | `STRING_DQ` | `(?s)"[^"\\]*(?:\\.[^"\\]*)*"` — **only valid when the literal contains no `${`**; otherwise mode-lex (F1) |
| `b"GET \x89"` | `BYTES` (dq) | `(?s)b"[^"\\]*(?:\\.[^"\\]*)*"` |
| `b'GET '` | `BYTES` (sq) | `(?s)b'[^'\\]*(?:\\.[^'\\]*)*'` |
| `~"\d+"im` | `REGEX` | `(?s)~"[^"\\]*(?:\\.[^"\\]*)*"[imsxb]*` — self-identifying since R237; interpolation-limited (F2) |
| `` `grep ${x} f` `` | `COMMAND` | ``(?s)`[^`]*` `` — **only valid when the literal contains no `${`**; otherwise mode-lex (F3). `\` is an ordinary byte, by ruling (command §2.2) |

All span patterns use the alternation-free "unrolled loop" form
(`[^X\\]*(?:\\.[^X\\]*)*`) rather than `(escape or normal)*`, for two reasons: it keeps
regex-alternation bars out of markdown table cells (where cell-escaping mangles them),
and it is the ReDoS-safe spelling should the lexer ever be ported off RE2 (F5). The two
forms accept exactly the same strings.

Notes. A literal with neither point nor exponent is an `int` (double §8); the `DOUBLE`
pattern requires a digit on **both** sides of the point, which is what makes `1..5` lex as
`INT RANGE INT` and `1.toDouble()` as `INT DOT IDENT` with no special cases. Booleans,
`null`, `undefined`, `nan`, and `inf` are keywords (§3), not literal tokens: a literal token is
recognized by a regex over an open-ended set of lexemes, while these six are a fixed, finite set
of words. They **denote** literal values all the same; the distinction is lexical, not semantic
(keywords §4). Minus is never part of a numeric token: `-7` is `MINUS INT_DEC` and `-inf` is
`MINUS KW_INF` (unary minus, tier 2; `0..-1` parses as `0..(-1)`, associativity §4). There is no
unary `+`, so positive infinity is written `inf`, never `+inf`. Regex flags are exactly `i m s x b` (regex §3). The `x`-flag verbose
form spans lines and contains `#` comments; the `REGEX` span pattern is unaffected since
it only seeks the unescaped closing `"`.

**The numeric grammar, ruled in full (R238, closing G2).** Four rules, each chosen against a
known footgun rather than for taste:

- **Radix prefixes are `0x`, `0b`, `0o`, lowercase only.** `0X`/`0B`/`0O` are lex errors —
  there is nothing to gain from two spellings of a prefix. Hex *digits* stay either case
  (`0xDEADBEEF` is idiomatic), and the exponent marker stays either case (`1E10`); where two
  spellings do carry idiom, the formatter canonicalizes rather than the lexer forbidding.
- **Leading zeros are a lex error**, with an explicit error production (`0[0-9_]+`) rather than
  a gap in the grammar, because §1.1 collects lexical errors and a gap would silently yield
  adjacent `INT` tokens for `007` instead of a diagnosis. `0755` means octal to a C or Python-2
  reader and 755 to a machine that allows it; Luna admits neither reading, and `0o755` is how
  the intent is spelled. Bare `0`, `0.5`, and `0x0` are unaffected.
- **`_` separates digits and nothing else.** One underscore, strictly between two digits, in
  any radix and on either side of a point: `1_000`, `0b0100_0001`, `3.141_592`. Rejected
  everywhere else — `_1` (an identifier), `1_`, `1__0`, and `0x_FF` (Go permits that last one;
  Luna does not, a digit must follow the prefix). A doubled or dangling separator is a typo,
  not an intent.
- **No leading or trailing point.** `.5` is written `0.5` and `5.` is written `5.0`. The
  trailing ban is load-bearing, as above; the leading ban is symmetry, and `0.5` reads better.

Exponents take an optional sign and **plain digits only** — no separators inside an exponent,
and no hex-float form (`0x1p3` is not Luna). **Literal magnitude is not the lexer's business**
(R238): the lexer accepts any digit string, and a value too large to be an `int` is caught in
parsing, which can decide it without type information precisely because there are no
wider-type literals — every integer literal is an `int` (numeric-tower R216, reaffirmed).
Assigning an in-range `int` literal to a narrower target (`let b: byte = 300;`) is a separate,
later check that belongs to analysis. A `double` literal whose value **overflows to infinity**
(`1e400`) is likewise a compile error, since `inf` is the explicit spelling and a silent
finite-to-infinite jump is a wrong value, not a rounding; ordinary rounding — including
underflow — is normal IEEE behaviour and is not diagnosed.

## 5. Operators and punctuation

Ordered longest-first within each family; §8 gives the global order.

| Token | Name | Go regex (RE2) |
|-|-|-|
| `???=` | `NULL_COALESCE_ASSIGN` | `\?\?\?=` |
| `???` | `NULL_COALESCE` | `\?\?\?` |
| `??=` | `COALESCE_ASSIGN` | `\?\?=` |
| `??` | `COALESCE` | `\?\?` |
| `?->` | `OPT_PROTO_ACCESS` | `\?->` |
| `?.` | `OPT_ACCESS` | `\?\.` |
| `?` | `QUESTION` | `\?` |
| `...` | `SPREAD` | `\.\.\.` |
| `..<` | `RANGE_EXCL` | `\.\.<` |
| `..` | `RANGE` | `\.\.` |
| `.` | `DOT` | `\.` |
| `\|\|` | `OR` | `\x7c\x7c` |
| `\|` | `BAR` | `\x7c` |
| `&&` | `AND` | `&&` |
| `&` | `AMP` | `&` |
| `->` | `ARROW` | `->` |
| `-=` | `MINUS_ASSIGN` | `-=` |
| `-` | `MINUS` | `-` |
| `=>` | `FAT_ARROW` | `=>` |
| `==` | `EQ` | `==` |
| `=` | `ASSIGN` | `=` |
| `!=` | `NEQ` | `!=` |
| `!` | `BANG` | `!` |
| `<=` | `LE` | `<=` |
| `<` | `LT` | `<` |
| `>=` | `GE` | `>=` |
| `>` | `GT` | `>` |
| `+=` | `PLUS_ASSIGN` | `\+=` |
| `+` | `PLUS` | `\+` |
| `*=` | `STAR_ASSIGN` | `\*=` |
| `*` | `STAR` | `\*` |
| `/=` | `SLASH_ASSIGN` | `/=` |
| `/` | `SLASH` | `/` |
| `%=` | `PERCENT_ASSIGN` | `%=` |
| `%` | `PERCENT` | `%` |
| `@@` | `AT_AT` | `@@` |
| `@` | `AT` | `@` |
| `#[` | `ATTR_OPEN` | `#\[` |
| `(` | `LPAREN` | `\(` |
| `)` | `RPAREN` | `\)` |
| `{` | `LBRACE` | `\{` |
| `}` | `RBRACE` | `\}` |
| `[` | `LBRACKET` | `\[` |
| `]` | `RBRACKET` | `\]` |
| `,` | `COMMA` | `,` |
| `;` | `SEMICOLON` | `;` |
| `:` | `COLON` | `:` |

Notes. The pipe tokens are written with the hex escape `\x7c` (Go regex for `|`) so the
patterns survive markdown table rendering byte-for-byte. There is
no unary `+`, no `===`/`!==`, no ternary, no bitwise tokens, and no `&&=`
/`||=` (numeric-operators §1.1; associativity §4; int §8). `:` serves annotations
(`x: int`), slice bounds (`xs[1:3]`, `xs[:]`, tables §3), and default-bearing signatures.
`#` occurs only as part of `ATTR_OPEN` (attributes §3), or as the leading `#!` of a
first-line shebang (§2) — a bare `#` anywhere else is a lex error. (`*` was the import glob until R136 retired it; bare-path imports mean "everything" now.) `SLASH` and `SLASH_ASSIGN`
compete only with the two comment forms, and the **next** byte decides between them (`//`, `/*`,
`/=`, `/`) — no context, no lookbehind. `~` is not an operator: its sole appearance is opening a
`REGEX` literal (`~"…"`, R237, F2), and a `~` not followed by `"` is a lex error, exactly as a
bare `#` is.

## 6. Interpolation sub-tokens (modes `DQ_STRING`, `REGEX_BODY`, `COMMAND`)

| Token | Name | Go regex (RE2) |
|-|-|-|
| `${` | `INTERP_OPEN` | `\$\{` — pushes `INTERP_EXPR` |
| `$name` | `INTERP_IDENT` | `\$[A-Za-z_][A-Za-z0-9_]*` — `DQ_STRING` only; longest identifier wins (string §5) |
| `\n`, `\$`, `\"`, … | `ESCAPE_PAIR` | `(?s)\\.` — one backslash-pair; `DQ_STRING`, `REGEX_BODY`, and `COMMAND` (R150 — commands escape `` \` `` `\\` `\$`, command §2). Decoding per the authoritative table, string §5.1 (R150) |
| text run (dq) | `DQ_TEXT` | `[^"\\$]+` |
| lone `$` | text (fallback, all modes) | `\$` — a `$` not starting an interp form is literal text (in commands: `$` not followed by `{`, command §2.2) |
| text run (regex) | `REGEX_TEXT` | `[^"\\$]+` |
| text run (command) | `CMD_TEXT` | ``[^`\\$]+`` (backslash excluded since R150 — commands have escapes) |
| `${...expr}` | `INTERP_OPEN` + `SPREAD` + … | spread-splice in commands and literals (command §3, spread §5); the `...` lexes as `SPREAD` inside `INTERP_EXPR` |

**Opening delimiters are tokens in their own right** (R235, closing an inventory gap this file
carried: the closers were named, the openers only described): `"` (`DQ_OPEN`), `~"`
(`REGEX_OPEN`, R237), `` ` `` (`CMD_OPEN`). They are emitted in `DEFAULT` or
`INTERP_EXPR`, push their mode, and are **mode-path only** — where §4's span regex applies, the
whole literal is one `STRING_DQ` / `REGEX` / `COMMAND` token and no opener or closer is emitted.
So the same source construct reaches the parser in one of two shapes, single token or delimited
sequence, and the parser accepts both; the split is F1/F3's fast path, not two grammars.

Closing delimiters end the mode: `"` (`DQ_CLOSE`), unescaped `"` plus `[imsxb]*`
(`REGEX_CLOSE`), `` ` `` (`CMD_CLOSE`). Inside `INTERP_EXPR` the lexer counts `LBRACE` /
`RBRACE`; the `RBRACE` that returns the count to zero is emitted as `INTERP_CLOSE` and
pops back to the enclosing literal mode.

## 7. Identifiers

| Token | Name | Go regex (RE2) |
|-|-|-|
| `foo`, `snake_case`, `camelCase` | `IDENT` | `[A-Za-z_][A-Za-z0-9_]*` |
| `_` | `WILDCARD` | `_\b` |

The identifier grammar is now formal (lexical-structure §2): ASCII bytes only, no Unicode
identifiers, source must be valid UTF-8. Hyphens are
explicitly not identifier characters (functions §5.6: `-` is always subtraction). `_` is
identifier-shaped but is the discard/wildcard (wildcard spec, keywords §4); lexing it as a
distinct token keeps the parser's pattern grammar simple, and `_\b` is safe in RE2 (word
boundary, no lookahead needed) so `_foo` still lexes as one `IDENT`.

## 8. Ordering (maximal munch)

Attempt order within `DEFAULT` / `INTERP_EXPR`:

1. `SHEBANG` (only at byte offset 0), then `WHITESPACE`, `LINE_COMMENT`, `BLOCK_COMMENT` —
   comments before anything `/`-initial.
2. `BYTES` — before `IDENT`, or `b"…"` lexes as `IDENT(b)` + string.
3. `REGEX` — `~"`-initial and self-identifying (R237): `~` is a token in no other
   position, so no ordering constraint applies and no context is consulted.
4. `DOUBLE` (both rows), then `INT_HEX`, `INT_BIN`, `INT_OCT`, then the leading-zero error
   production, then `INT_DEC` — doubles first so `1.5` is one token; the radix prefixes
   before decimal so `0x10` doesn't lex as `INT(0)` + `IDENT(x10)`; and the error
   production before `INT_DEC` so `0755` is diagnosed rather than split into adjacent
   integers (§4, R238).
5. Operators, longest lexeme first: `???=` › `???` › `??=` › `??` › `?->` › `?.` › `?`; `...` ›
   `..<` › `..` › `.`; `||` › `|`; `&&` › `&`; `=>` and `==` › `=`; `->` and
   `-=` › `-`; `@@` › `@`; `!=` › `!`; `<=` › `<`; `>=` › `>`; `#[` before any bare `#`.
6. Keywords (with `KW_MATCH_BANG` before `KW_MATCH`, and `KW_YIELD_FROM` before
   `KW_YIELD`), then `WILDCARD`, then `IDENT` — or, equivalently, `IDENT` first with a
   keyword lookup, plus a one-token peek for `match!` and a whitespace-only peek for
   `yield from`.

## 9. Token positions (R236)

Every token carries a **byte offset and byte length** into the validated source — a span, not a
copy of the text, the source buffer being retained for anyone who needs the lexeme. Byte offsets
are the stored form for four reasons, none of them convenience: the scanner is byte-native (RE2
over bytes that the ingress check has already proven good UTF-8, lexical-structure §1); §2's
tiling invariant is exact in bytes and merely approximate in anything else; cutting a source line out for an error snippet
needs byte indices regardless; and the language server (`luna -l`, compiler §0.1) wants **UTF-16
code units**, a third encoding — so a conversion layer exists no matter what, and it is cheapest
with a single canonical origin to convert *from*. The corpus already leans this way:
lexical-structure §1 refuses to strip a BOM precisely to keep byte offset 0 meaningful.

**Line and column are computed, never stored.** Diagnostics report a **rune** column, which is
what a person counts, but nothing on the common path pays for it: a table of line-start offsets
is built once per file, a byte offset becomes a line by binary search, and the column is a rune
count over that line's prefix alone — bounded by line length, never file length. A compile that
emits no diagnostic never runs the conversion at all.

**The pure-ASCII fast path falls out of a pass that already exists.** The UTF-8 validation of
lexical-structure §1 visits every byte before tokenizing, establishing validity "exactly once, up
front"; recording whether any byte was ≥ `0x80` costs nothing there,
and in a file where none was — the overwhelming majority — the rune column *is* the byte column
and the conversion is subtraction. Even in a file that has non-ASCII, lexical-structure §1
confines it to string, `command`, `regex`, and comment content, so the counting path only ever
runs for a diagnostic on a line carrying such content.

**Tab handling is open.** Whether a tab advances one column or to the next tab stop is a
rendering question for the diagnostics spec to settle, and it does not reach the lexer: byte
spans are unaffected either way, and the renderer holds the source line.

---

## Flagged: complex, context-sensitive, or non-regular tokens

The brief asked for anything very complex or backtracking-heavy to be flagged. One
framing note first: Go's `regexp` is **RE2**. It never backtracks — every pattern runs in
time linear in the input, and backreferences/lookaround don't exist in the engine. So
nothing in this file can be slow at runtime; the real hazards are tokens that are
**context-sensitive or non-regular**, which no RE2 pattern (indeed no regex) can express
alone. Those are flagged below, along with two patterns that would be ReDoS-shaped in a
backtracking engine but are safe here.

**F1 — Double-quoted strings with `${expr}` are not regular.** Strings §13 allows an
arbitrary expression between the braces, *including nested double-quoted strings* (its own
example: `"${x ?? "none"}"`). The `STRING_DQ` span regex closes at the `"` before `none`
and mis-tokenizes everything after. There is no regex fix — nested delimiters plus nested
braces require counting. Use the `DQ_STRING` / `INTERP_EXPR` modes of §1/§6: the span
regex in §4 may be used only as a fast path when a scan finds no `${` before the closing
quote.

**F2 — regex literals are prefixed, and they interpolate.** A regex literal is `~"…"` (R237,
regex §2). The sigil is what keeps this simple: `~` is a token in no other position, so
`REGEX` is self-identifying, `/` is unconditionally division-or-comment (decided by the
*next* byte, never by context), and the lexer carries **no** regex-allowed state at all.

This flag now records history, because the alternative is a well-known trap that should not
be re-proposed. A bare `/…/` literal makes `/` three-way ambiguous — division, comment, or
regex — resolvable only from the **previous significant token**, which RE2 cannot express (it
has no lookbehind) and which therefore becomes lexer state. R235 specified that state exactly:
a 25-token "division set" of everything that can end a value, consulted on every `/` and
threaded per-frame through the mode stack. **R237 superseded the entire apparatus** by
changing one character of surface syntax. The decisive cost was never the table's size but
its failure mode — a misclassification toward regex sent the scanner hunting for the next
unescaped `/`, swallowing arbitrary source into one token, where the opposite error merely
produced a parse error in the right place.

The interpolation half stands. `${expr}` inside a regex literal (regex §7, comptime-only) may
itself contain a `"` — inside a nested string in the splice — which would falsely terminate
the §4 span regex; `REGEX_BODY` mode handles it, and §4's pattern is a no-`${` fast path
exactly as for strings (F1) and commands (F3). The span pattern is otherwise sound, including
multi-line `x`-verbose patterns, since it only hunts the unescaped closing `"`. Two collisions
the old delimiter forced are simply gone: `//` is now unambiguously a line comment with no
empty-regex reading — retiring regex §2's `/(?:)/` workaround, an empty pattern being `~""` —
and a `#` comment inside an `x`-verbose pattern can no longer terminate the literal.

**F3 — Command literals with `${expr}` are not regular**, for the same reason as F1: the
splice is a full expression (command §3) and may contain backticks inside nested strings
or nested command literals. The §4 span regex is a no-`${` fast path only; otherwise use
the `COMMAND` / `INTERP_EXPR` modes. Command literals have **no escape
sequences** by ruling (command §2.2, resolving G5): `\` is an ordinary byte, and literal
`` ` `` / `${` are written via interpolation — so the span regex and `CMD_TEXT` correctly
treat `\` as plain text, and no `ESCAPE_PAIR` token exists in `COMMAND` mode.

**F4 — Block comments.** `(?s)/\*.*?\*/` uses a lazy quantifier — a classic backtracking
red flag in PCRE, but linear and safe in RE2. Nesting is ruled **out** (lexical-structure §3,
resolving G3): the first `*/` ends the comment, which is exactly what the lazy quantifier
implements, so this regex is the complete rule and no depth counter is needed.

**F5 — Would-be ReDoS shapes, pre-empted.** The naive spelling of every span pattern is
`(escape-pair or normal-char)*` — an alternation under a star with overlapping
first-sets, the textbook catastrophic-backtracking shape on adversarial input such as a
long backslash run with no closing quote. RE2 would run it in linear time anyway, but
this file uses the unrolled-loop form `[^X\\]*(?:\\.[^X\\]*)*` throughout, which is
equivalent, is linear even in a backtracking engine, and keeps the patterns portable if
the lexer ever leaves Go's `regexp`. Net: nothing in this file can exhibit extensive
backtracking, under RE2 by engine guarantee and elsewhere by construction.

**F6 — Ordering is load-bearing.** The maximal-munch chains in §8 are correctness
requirements, not style: `???=`/`??`/`?.`, `...`/`..<`/`..`, `||`/`|`, `@@`/`@`,
`b"` before `IDENT`, and `DOUBLE`-requires-digits-after-the-point (which is what keeps
`1..5` and `1.toDouble()` unambiguous without lookahead — RE2 has none to offer).

## Cross-reference notes: gaps found, and their resolutions

Seven gaps surfaced during cross-referencing; **all seven are now ruled** (G2, the last, by R238).

- **G1 — resolved, then superseded (R223).** `from` was required by the import grammar
  (`import { x } from m`, modules §4) but missing from the keywords.md inventory; the
  first resolution reserved it everywhere. R223 re-ruled it **unreserved**: `from` lexes
  as `IDENT`, the import grammar treats it contextually, and delegation is the compound
  token `KW_YIELD_FROM` (§3), so the keywords.md inventory carries no `from` row.
- **G2 — resolved (R238).** The numeric literal grammar is ruled in §4: `0x`/`0b`/`0o`
  lowercase-only prefixes with **octal added**, leading zeros a lex error with an explicit
  error production, `_` strictly between digits, no leading or trailing point, plain digits in
  exponents. The working assumptions this file carried were adopted almost unchanged — octal
  is the one addition, the leading-zero ban the one thing that was genuinely open — and the
  magnitude question the assumptions never addressed is answered too: a literal too large for
  `int` is caught in **parsing**, not lexing. Nothing here needed a wider-type literal form,
  so numeric-tower's R216 deferral is reaffirmed rather than spent.
- **G3 — resolved.** Comments are now specified (lexical-structure §3): `//` line comments
  and C-style `/* … */` block comments that do **not** nest, so §2's regex is the complete
  rule. There is no doc-comment syntax — attributes carry documentation — and the corpus's
  doc-comment references are gone.
- *(**G4 — resolved by R150.** The full escape table is authoritative in string §5.1:
  per-context rows for `"…"`/`'…'`/`b"…"`/`` `…` ``/`/…/`, with `\u{…}` added (the
  strings-side codepoint escape; `\xNN` stays bytes-only on the UTF-8-validity split),
  `\r` confirmed, the exemplified `\0` retired (no shorthand; `\u{0}`), and unknown
  escapes ruled lex errors. The token *span* is unaffected — `\\.` covers any pair —
  except `\u{…}`, whose braces ride the existing brace-depth machinery.)*
- *(**G5 — superseded by R150.** The earlier resolution gave command literals no escapes,
  spelling a literal backtick through interpolation (``${'`'}``, joined by the adjacency
  rule) — ceremony where one escape pair suffices, and the mode table's "unescaped
  `` ` ``" terminator had implied the escape reading all along. Commands now escape
  `` \` `` `\\` `\$` (command §2, string §5.1); `COMMAND` mode gains `ESCAPE_PAIR`
  and `CMD_TEXT` excludes the backslash, §4.)*
- **G6 — resolved.** The identifier grammar is now formal (lexical-structure §2): ASCII
  `[A-Za-z_][A-Za-z0-9_]*` as actual bytes, no Unicode identifiers, and all source files
  must be valid UTF-8 (lexical-structure §1) — the lexer rejects anything else up front.
- **G7 — resolved.** `match!` stays a single token (`KW_MATCH_BANG`), as tokenized here;
  the `KW_MATCH` + `BANG` alternative is retired.
