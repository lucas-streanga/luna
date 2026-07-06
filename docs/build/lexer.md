# Luna — Lexer Specification

Token inventory for the Luna lexer, cross-referenced from every spec in the repository
(keywords.md is the keyword authority; lexical-structure.md the encoding, identifier,
and comment authority; operators.md and associativity.md the operator catalogue; literal
forms from int.md §7, double.md §8, strings.md §13, bytes.md §7, regex.md §2–3,
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
| `DQ_STRING` | `"` | unescaped `"` |
| `REGEX_BODY` | `/` in regex-allowed context (F2) | unescaped `/`, then flags |
| `COMMAND` | `` ` `` | unescaped `` ` `` |
| `INTERP_EXPR` | `${` inside any of the three modes above | the `}` that returns brace depth to zero (depth counted by the lexer) |

`INTERP_EXPR` lexes with the full `DEFAULT` rule set. Single-quoted strings and `b"..."`
bytes literals do **not** interpolate (strings §13, bytes §7) and are handled by a single
regex each.

## 2. Whitespace and comments

| Token | Name | Go regex (RE2) |
|-|-|-|
| spaces, tabs, newlines | `WHITESPACE` (skipped) | `[ \t\r\n]+` |
| `// …` | `LINE_COMMENT` (skipped) | `//[^\n]*` |
| `/* … */` | `BLOCK_COMMENT` (skipped) | `(?s)/\*.*?\*/` |

Both forms are specified in lexical-structure §3. Block comments are C-style and do
**not** nest, by ruling, so the non-nesting pattern above is the complete rule (F4). There
is no doc-comment syntax; documentation rides on attributes.

## 3. Keywords

Reserved words per keywords.md §1–§4. Recommended implementation: lex an `IDENT`
(§7) and promote via a lookup table, rather than running 47 patterns; the per-word regex
is given for completeness. All follow the shape `\bword\b`.

| Token | Name | Go regex (RE2) |
|-|-|-|
| `var` | `KW_VAR` | `\bvar\b` |
| `let` | `KW_LET` | `\blet\b` |
| `const` | `KW_CONST` | `\bconst\b` |
| `fn` | `KW_FN` | `\bfn\b` |
| `constraint` | `KW_CONSTRAINT` | `\bconstraint\b` |
| `proto` | `KW_PROTO` | `\bproto\b` |
| `enum` | `KW_ENUM` | `\benum\b` |
| `error` | `KW_ERROR` | `\berror\b` |
| `capability` | `KW_CAPABILITY` | `\bcapability\b` |
| `attribute` | `KW_ATTRIBUTE` | `\battribute\b` |
| `test` | `KW_TEST` | `\btest\b` |
| `meta` | `KW_META` | `\bmeta\b` |
| `export` | `KW_EXPORT` | `\bexport\b` |
| `import` | `KW_IMPORT` | `\bimport\b` |
| `from` | `KW_FROM` | `\bfrom\b` |
| `if` | `KW_IF` | `\bif\b` |
| `else` | `KW_ELSE` | `\belse\b` |
| `foreach` | `KW_FOREACH` | `\bforeach\b` |
| `in` | `KW_IN` | `\bin\b` |
| `while` | `KW_WHILE` | `\bwhile\b` |
| `break` | `KW_BREAK` | `\bbreak\b` |
| `continue` | `KW_CONTINUE` | `\bcontinue\b` |
| `return` | `KW_RETURN` | `\breturn\b` |
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
| `self` | `KW_SELF` | `\bself\b` |

Notes. `in`, `by`, and `self` are contextual per keywords.md (foreach heads, range steps,
protocol bodies) but reserved everywhere, so the lexer treats them uniformly. `from` is
reserved for the import-source clause (keywords §1, modules §4) — gap G1, now resolved.
`match!` is listed as its own form in operators §0 and is tokenized as one unit here,
confirmed as the ruling (G7). `panic`, `_`, and every builtin type name (`int`, `string`, `table`, …) are
**not** keywords (keywords.md §4–§5); they lex as `IDENT`.

## 4. Literals

| Token | Name | Go regex (RE2) |
|-|-|-|
| `42`, `1_000` | `INT_DEC` | `[0-9](?:_?[0-9])*` |
| `0x1F`, `0xdead_beef` | `INT_HEX` | `0x[0-9a-fA-F](?:_?[0-9a-fA-F])*` |
| `0b0100_0001` | `INT_BIN` | `0b[01](?:_?[01])*` |
| `3.14`, `2.5e10` | `DOUBLE` (point form) | `[0-9](?:_?[0-9])*\.[0-9](?:_?[0-9])*(?:[eE][+-]?[0-9]+)?` |
| `6.022e23`, `1e-9` | `DOUBLE` (exponent form) | `[0-9](?:_?[0-9])*[eE][+-]?[0-9]+` |
| `'literal'` | `STRING_SQ` | `(?s)'[^'\\]*(?:\\.[^'\\]*)*'` |
| `"text $x ${e}"` | `STRING_DQ` | `(?s)"[^"\\]*(?:\\.[^"\\]*)*"` — **only valid when the literal contains no `${`**; otherwise mode-lex (F1) |
| `b"GET \x89"` | `BYTES` (dq) | `(?s)b"[^"\\]*(?:\\.[^"\\]*)*"` |
| `b'GET '` | `BYTES` (sq) | `(?s)b'[^'\\]*(?:\\.[^'\\]*)*'` |
| `/\d+/im` | `REGEX` | `(?s)/[^/\\]*(?:\\.[^/\\]*)*/[imsxb]*` — context-gated and interpolation-limited (F2) |
| `` `grep ${x} f` `` | `COMMAND` | ``(?s)`[^`]*` `` — **only valid when the literal contains no `${`**; otherwise mode-lex (F3). `\` is an ordinary byte, by ruling (command §2.2) |

All span patterns use the alternation-free "unrolled loop" form
(`[^X\\]*(?:\\.[^X\\]*)*`) rather than `(escape or normal)*`, for two reasons: it keeps
regex-alternation bars out of markdown table cells (where cell-escaping mangles them),
and it is the ReDoS-safe spelling should the lexer ever be ported off RE2 (F5). The two
forms accept exactly the same strings.

Notes. A literal with neither point nor exponent is an `int` (double §8); the `DOUBLE`
pattern requires a digit on **both** sides of the point, which is what makes `1..5` lex as
`INT RANGE INT` and `1.toDouble()` as `INT DOT IDENT` with no special cases. Booleans,
`null`, and `undefined` are keywords (§3), not literal tokens. Minus is never part of a
numeric token: `-7` is `MINUS INT_DEC` (unary minus, tier 2; `0..-1` parses as `0..(-1)`,
associativity §4). Regex flags are exactly `i m s x b` (regex §3). The `x`-flag verbose
form spans lines and contains `#` comments; the `REGEX` span pattern is unaffected since
it only seeks the unescaped closing `/`. Several of the exact numeric rules are open in
the spec and were resolved by assumption here — see G2.

## 5. Operators and punctuation

Ordered longest-first within each family; §8 gives the global order.

| Token | Name | Go regex (RE2) |
|-|-|-|
| `???=` | `NULL_COALESCE_ASSIGN` | `\?\?\?=` |
| `???` | `NULL_COALESCE` | `\?\?\?` |
| `??=` | `COALESCE_ASSIGN` | `\?\?=` |
| `??` | `COALESCE` | `\?\?` |
| `?.` | `OPT_ACCESS` | `\?\.` |
| `?` | `QUESTION` | `\?` |
| `...` | `SPREAD` | `\.\.\.` |
| `..<` | `RANGE_EXCL` | `\.\.<` |
| `..` | `RANGE` | `\.\.` |
| `.` | `DOT` | `\.` |
| `\|>` | `PIPELINE` | `\x7c>` |
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
patterns survive markdown table rendering byte-for-byte; `\x7c>` ≡ `\|>` to Go. There is
no unary `+`, no `===`/`!==`, no ternary, no bitwise tokens, and no `&&=`
/`||=` (numeric-operators §1.1; associativity §4; int §8). `:` serves annotations
(`x: int`), slice bounds (`xs[1:3]`, `xs[:]`, tables §3), and default-bearing signatures.
`#` occurs only as part of `ATTR_OPEN` (attributes §3) — a bare `#` in `DEFAULT` mode is a
lex error. `*` is also the import glob (`import * from m`). `SLASH` and `SLASH_ASSIGN`
compete with `REGEX` and comments; see F2.

## 6. Interpolation sub-tokens (modes `DQ_STRING`, `REGEX_BODY`, `COMMAND`)

| Token | Name | Go regex (RE2) |
|-|-|-|
| `${` | `INTERP_OPEN` | `\$\{` — pushes `INTERP_EXPR` |
| `$name` | `INTERP_IDENT` | `\$[A-Za-z_][A-Za-z0-9_]*` — `DQ_STRING` only; longest identifier wins (strings §13) |
| `\n`, `\$`, `\"`, … | `ESCAPE_PAIR` | `(?s)\\.` — one backslash-pair; `DQ_STRING` and `REGEX_BODY` only (commands have no escapes, command §2.2). Decoding is a later pass (G4) |
| text run (dq) | `DQ_TEXT` | `[^"\\$]+` |
| lone `$` | text (fallback, all modes) | `\$` — a `$` not starting an interp form is literal text (in commands: `$` not followed by `{`, command §2.2) |
| text run (regex) | `REGEX_TEXT` | `[^/\\$]+` |
| text run (command) | `CMD_TEXT` | ``[^`$]+`` |
| `${...expr}` | `INTERP_OPEN` + `SPREAD` + … | spread-splice in commands and literals (command §3, spread §5); the `...` lexes as `SPREAD` inside `INTERP_EXPR` |

Closing delimiters end the mode: `"` (`DQ_CLOSE`), unescaped `/` plus `[imsxb]*`
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

1. `WHITESPACE`, `LINE_COMMENT`, `BLOCK_COMMENT` — comments before anything `/`-initial.
2. `BYTES` — before `IDENT`, or `b"…"` lexes as `IDENT(b)` + string.
3. `REGEX` — only when the regex-allowed flag is set (F2); before `SLASH`/`SLASH_ASSIGN`.
4. `DOUBLE` (both rows), then `INT_HEX`, `INT_BIN`, then `INT_DEC` — doubles first so
   `1.5` is one token; hex/bin before decimal so `0x10` doesn't lex as `INT(0)` +
   `IDENT(x10)`.
5. Operators, longest lexeme first: `???=` › `???` › `??=` › `??` › `?.` › `?`; `...` ›
   `..<` › `..` › `.`; `|>` and `||` › `|`; `&&` › `&`; `=>` and `==` › `=`; `->` and
   `-=` › `-`; `@@` › `@`; `!=` › `!`; `<=` › `<`; `>=` › `>`; `#[` before any bare `#`.
6. Keywords (with `KW_MATCH_BANG` before `KW_MATCH`), then `WILDCARD`, then `IDENT` — or,
   equivalently, `IDENT` first with a keyword lookup, plus a one-token peek for `match!`.

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

**F2 — `/` is three-way ambiguous, and regex literals interpolate.** A leading `/` can be
division (`SLASH`, `SLASH_ASSIGN`), a comment (`//`, `/*`), or a `REGEX` literal, and the
choice needs the **previous significant token** (the JavaScript problem): after a value
(ident, literal, `)`, `]`, a postfix) it is division; after operators, `(`, `[`, `{`, `,`,
`;`, `=>`, `=`, `return`, and the other prefix positions it opens a regex. RE2 has no
lookbehind, so this is lexer state, not a pattern. Additionally, `${expr}` inside a regex
literal (regex §7, comptime-only) can itself contain `/` — division inside the splice —
which would falsely terminate the §4 span regex; the `REGEX_BODY` mode handles it. The
span regex is otherwise sound, including multi-line `/x` verbose patterns, since it only
hunts the unescaped closing `/`. Note `//` (an empty pattern) is also a line comment,
which the §8 ordering resolves in favor of the comment.

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
requirements, not style: `???=`/`??`/`?.`, `...`/`..<`/`..`, `|>`/`||`/`|`, `@@`/`@`,
`b"` before `IDENT`, and `DOUBLE`-requires-digits-after-the-point (which is what keeps
`1..5` and `1.toDouble()` unambiguous without lookahead — RE2 has none to offer).

## Cross-reference notes: gaps found, and their resolutions

Seven gaps surfaced during cross-referencing; six are now ruled, one stays open.

- **G1 — resolved.** `from` was required by the import grammar (`import { x } from m`,
  modules §4) but missing from the keywords.md inventory. It is now listed there (§1) as
  the import-source clause, contextual to `import` statements but reserved everywhere.
- **G2 — deliberately deferred.** The numeric literal grammar remains open (int §7–§8,
  double §8) by choice. This file's working assumptions stand until it is ruled: `_` only
  between digits, lowercase `0x`/`0b` only, no octal, no leading or trailing point (the
  trailing-point ban is also what makes ranges lex cleanly), plain digits in exponents.
- **G3 — resolved.** Comments are now specified (lexical-structure §3): `//` line comments
  and C-style `/* … */` block comments that do **not** nest, so §2's regex is the complete
  rule. There is no doc-comment syntax — attributes carry documentation — and the corpus's
  doc-comment references are gone.
- **G4 — open.** The double-quote escape set is still only exemplified (`\n`, `\t`, `\0`,
  `\$`, plus `\"`/`\\` implied; bytes adds `\xNN`). The token *span* is unaffected (`\\.`
  covers any pair) but the string decoder needs the full table (`\r`? `\u{…}`?).
- **G5 — resolved.** Command literals have **no escape sequences** (command §2.2): `\` is
  an ordinary byte, and a literal `` ` `` or `${` is written through interpolation
  (``${'`'}``, `${'${'}`), which the new adjacency rule (command §3) joins into the
  surrounding argument. The lexer's `COMMAND` mode is unchanged and correct as specified.
- **G6 — resolved.** The identifier grammar is now formal (lexical-structure §2): ASCII
  `[A-Za-z_][A-Za-z0-9_]*` as actual bytes, no Unicode identifiers, and all source files
  must be valid UTF-8 (lexical-structure §1) — the lexer rejects anything else up front.
- **G7 — resolved.** `match!` stays a single token (`KW_MATCH_BANG`), as tokenized here;
  the `KW_MATCH` + `BANG` alternative is retired.
