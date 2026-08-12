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

## 0. The token table

Every token, in one place: what it looks like, its name, its category, and its RE2 pattern.
The sections that follow keep the **notes** that explain each group — the rationale, the
orderings, the flagged hazards — but not a second copy of these rows. `§n` in the category
column points at the section that owns the explanation. Attempt order is **not** given by this
table's row order; §8 gives it for `DEFAULT`/`INTERP_EXPR` and §6 for the literal modes.

| Token | Name | Category | Go regex (RE2) |
|-|-|-|-|
| spaces, tabs, newlines | `WHITESPACE` | trivia §2 | `[ \t\r\n]+` — one token per maximal run. Also the stripped trailing whitespace of a `"""` line (R246) |
| a line's leading margin | `MARGIN` | trivia §2 | the closing delimiter's exact indentation bytes, at the start of every content line of a triple literal (R246); a non-blank line not beginning with them is `L0014` |
| `#!…` (first line only) | `SHEBANG` | trivia §2 | `\A#![^\n]*` |
| `// …` | `LINE_COMMENT` | trivia §2 | `//[^\n]*` |
| `/* … */` | `BLOCK_COMMENT` | trivia §2 | `(?s)/\*.*?\*/` |
| `var` | `KW_VAR` | keyword §3 | `\bvar\b` |
| `let` | `KW_LET` | keyword §3 | `\blet\b` |
| `const` | `KW_CONST` | keyword §3 | `\bconst\b` |
| `fn` | `KW_FN` | keyword §3 | `\bfn\b` |
| `gen` | `KW_GEN` | keyword §3 | `\bgen\b` |
| `constraint` | `KW_CONSTRAINT` | keyword §3 | `\bconstraint\b` |
| `proto` | `KW_PROTO` | keyword §3 | `\bproto\b` |
| `enum` | `KW_ENUM` | keyword §3 | `\benum\b` |
| `error` | `KW_ERROR` | keyword §3 | `\berror\b` |
| `capability` | `KW_CAPABILITY` | keyword §3 | `\bcapability\b` |
| `attribute` | `KW_ATTRIBUTE` | keyword §3 | `\battribute\b` |
| `test` | `KW_TEST` | keyword §3 | `\btest\b` |
| `export` | `KW_EXPORT` | keyword §3 | `\bexport\b` |
| `import` | `KW_IMPORT` | keyword §3 | `\bimport\b` |
| `if` | `KW_IF` | keyword §3 | `\bif\b` |
| `else` | `KW_ELSE` | keyword §3 | `\belse\b` |
| `foreach` | `KW_FOREACH` | keyword §3 | `\bforeach\b` |
| `in` | `KW_IN` | keyword §3 | `\bin\b` |
| `while` | `KW_WHILE` | keyword §3 | `\bwhile\b` |
| `break` | `KW_BREAK` | keyword §3 | `\bbreak\b` |
| `continue` | `KW_CONTINUE` | keyword §3 | `\bcontinue\b` |
| `return` | `KW_RETURN` | keyword §3 | `\breturn\b` |
| `yield from` | `KW_YIELD_FROM` | keyword §3 | `\byield[ \t\r\n]+from\b` |
| `yield` | `KW_YIELD` | keyword §3 | `\byield\b` |
| `match!` | `KW_MATCH_BANG` | keyword §3 | `\bmatch!` |
| `match` | `KW_MATCH` | keyword §3 | `\bmatch\b` |
| `where` | `KW_WHERE` | keyword §3 | `\bwhere\b` |
| `defer` | `KW_DEFER` | keyword §3 | `\bdefer\b` |
| `by` | `KW_BY` | keyword §3 | `\bby\b` |
| `try` | `KW_TRY` | keyword §3 | `\btry\b` |
| `catch` | `KW_CATCH` | keyword §3 | `\bcatch\b` |
| `throw` | `KW_THROW` | keyword §3 | `\bthrow\b` |
| `copy` | `KW_COPY` | keyword §3 | `\bcopy\b` |
| `spawn` | `KW_SPAWN` | keyword §3 | `\bspawn\b` |
| `await` | `KW_AWAIT` | keyword §3 | `\bawait\b` |
| `comptime` | `KW_COMPTIME` | keyword §3 | `\bcomptime\b` |
| `comptype` | `KW_COMPTYPE` | keyword §3 | `\bcomptype\b` |
| `is` | `KW_IS` | keyword §3 | `\bis\b` |
| `as` | `KW_AS` | keyword §3 | `\bas\b` |
| `apply` | `KW_APPLY` | keyword §3 | `\bapply\b` |
| `declared` | `KW_DECLARED` | keyword §3 | `\bdeclared\b` |
| `moduleof` | `KW_MODULEOF` | keyword §3 | `\bmoduleof\b` |
| `use` | `KW_USE` | keyword §3 | `\buse\b` |
| `true` | `KW_TRUE` | keyword §3 | `\btrue\b` |
| `false` | `KW_FALSE` | keyword §3 | `\bfalse\b` |
| `null` | `KW_NULL` | keyword §3 | `\bnull\b` |
| `undefined` | `KW_UNDEFINED` | keyword §3 | `\bundefined\b` |
| `nan` | `KW_NAN` | keyword §3 | `\bnan\b` |
| `inf` | `KW_INF` | keyword §3 | `\binf\b` |
| `self` | `KW_SELF` | keyword §3 | `\bself\b` |
| `42`, `1_000` | `INT_DEC` | literal §4 | `0\x7c[1-9](?:_?[0-9])*` |
| `0x1F`, `0xdead_beef` | `INT_HEX` | literal §4 | `0x[0-9a-fA-F](?:_?[0-9a-fA-F])*` |
| `0b0100_0001` | `INT_BIN` | literal §4 | `0b[01](?:_?[01])*` |
| `0o755`, `0o1_7` | `INT_OCT` | literal §4 | `0o[0-7](?:_?[0-7])*` |
| `3.14`, `2.5e10` | `DOUBLE` (point form) | literal §4 | `(?:0\x7c[1-9](?:_?[0-9])*)\.[0-9](?:_?[0-9])*(?:[eE][+-]?[0-9]+)?` |
| `6.022e23`, `1e-9` | `DOUBLE` (exponent form) | literal §4 | `(?:0\x7c[1-9](?:_?[0-9])*)[eE][+-]?[0-9]+` |
| `'literal'` | `STRING_SQ` | literal §4 | `'[^'\\\n]*(?:\\[^\n][^'\\\n]*)*'` — newline-bounded (R244); attempted **after** `'''` (§8) |
| `"text $x ${e}"` | `STRING_DQ` | literal §4 | `"[^"\\\n]*(?:\\[^\n][^"\\\n]*)*"` — newline-bounded (R244); **only valid when the literal contains no `${`**, otherwise mode-lex (F1) |
| `b"GET \x89"` | `BYTES` (dq) | literal §4 | `b"[^"\\\n]*(?:\\[^\n][^"\\\n]*)*"` — newline-bounded (R244) |
| `b'GET '` | `BYTES` (sq) | literal §4 | `b'[^'\\\n]*(?:\\[^\n][^'\\\n]*)*'` — newline-bounded (R244) |
| `~"\d+"im` | `REGEX` | literal §4 | `(?s)~"[^"\\]*(?:\\.[^"\\]*)*"[imsxb]*` — self-identifying since R237; interpolation-limited (F2). **The one form that may span lines** (R244), for the `x` flag (§4) |
| `` `grep ${x} f` `` | `COMMAND` | literal §4 | ``` `[^`\\\n]*(?:\\[^\n][^`\\\n]*)*` ``` — newline-bounded (R244); **only valid when the literal contains no `${`**, otherwise mode-lex (F3). Escapes per R150 (command §2.2) |
| `0755`, `0_1` | `INVALID` | error §11 | `0[0-9_]+` — a leading zero is a **lex error** (R238, `L0003`), never silent decimal and never C-style octal |
| `0X1F`, `0B1` | `INVALID` | error §11 | `0[XBO]` — radix prefixes are lowercase only (R238, `L0004`); without this production `0X1F` would split into `INT_DEC` + `IDENT` and diagnose as a syntax error instead |
| `???=` | `NULL_COALESCE_ASSIGN` | operator §5 | `\?\?\?=` |
| `???` | `NULL_COALESCE` | operator §5 | `\?\?\?` |
| `??=` | `COALESCE_ASSIGN` | operator §5 | `\?\?=` |
| `??` | `COALESCE` | operator §5 | `\?\?` |
| `?->` | `OPT_PROTO_ACCESS` | operator §5 | `\?->` |
| `?.` | `OPT_ACCESS` | operator §5 | `\?\.` |
| `?` | `QUESTION` | operator §5 | `\?` |
| `...` | `SPREAD` | operator §5 | `\.\.\.` |
| `..<` | `RANGE_EXCL` | operator §5 | `\.\.<` |
| `..` | `RANGE` | operator §5 | `\.\.` |
| `.` | `DOT` | operator §5 | `\.` |
| `\|\|` | `OR` | operator §5 | `\x7c\x7c` |
| `\|` | `BAR` | operator §5 | `\x7c` |
| `&&` | `AND` | operator §5 | `&&` |
| `&` | `AMP` | operator §5 | `&` |
| `->` | `ARROW` | operator §5 | `->` |
| `-=` | `MINUS_ASSIGN` | operator §5 | `-=` |
| `-` | `MINUS` | operator §5 | `-` |
| `=>` | `FAT_ARROW` | operator §5 | `=>` |
| `==` | `EQ` | operator §5 | `==` |
| `=` | `ASSIGN` | operator §5 | `=` |
| `!=` | `NEQ` | operator §5 | `!=` |
| `!` | `BANG` | operator §5 | `!` |
| `<=` | `LE` | operator §5 | `<=` |
| `<` | `LT` | operator §5 | `<` |
| `>=` | `GE` | operator §5 | `>=` |
| `>` | `GT` | operator §5 | `>` |
| `+=` | `PLUS_ASSIGN` | operator §5 | `\+=` |
| `+` | `PLUS` | operator §5 | `\+` |
| `*=` | `STAR_ASSIGN` | operator §5 | `\*=` |
| `*` | `STAR` | operator §5 | `\*` |
| `/=` | `SLASH_ASSIGN` | operator §5 | `/=` |
| `/` | `SLASH` | operator §5 | `/` |
| `%=` | `PERCENT_ASSIGN` | operator §5 | `%=` |
| `%` | `PERCENT` | operator §5 | `%` |
| `@@` | `AT_AT` | operator §5 | `@@` |
| `@` | `AT` | operator §5 | `@` |
| `#[` | `ATTR_OPEN` | punctuation §5 | `#\[` |
| `(` | `LPAREN` | punctuation §5 | `\(` |
| `)` | `RPAREN` | punctuation §5 | `\)` |
| `{` | `LBRACE` | punctuation §5 | `\{` |
| `}` | `RBRACE` | punctuation §5 | `\}` |
| `[` | `LBRACKET` | punctuation §5 | `\[` |
| `]` | `RBRACKET` | punctuation §5 | `\]` |
| `,` | `COMMA` | punctuation §5 | `,` |
| `;` | `SEMICOLON` | punctuation §5 | `;` |
| `:` | `COLON` | punctuation §5 | `:` |
| `"""` | `TRIPLE_DQ_OPEN` | delimiter §6 | `"""[ \t\r]*\n` — pushes `TRIPLE_DQ_STRING` (R246); the opener owns to the end of its line, and content after it is `L0015` |
| `<margin>"""` | `TRIPLE_DQ_CLOSE` | delimiter §6 | `\n[ \t]*"""` — pops `TRIPLE_DQ_STRING`; the token spans the preceding newline, which is why that newline is not content (R246) |
| `'''` | `TRIPLE_SQ_OPEN` | delimiter §6 | `'''[ \t\r]*\n` — pushes `TRIPLE_SQ_STRING` (R246) |
| `<margin>'''` | `TRIPLE_SQ_CLOSE` | delimiter §6 | `\n[ \t]*'''` — pops `TRIPLE_SQ_STRING` |
| `"` | `DQ_OPEN` | delimiter §6 | `"` — pushes `DQ_STRING`; mode-path only; attempted **after** `"""` (§8) |
| `~"` | `REGEX_OPEN` | delimiter §6 | `~"` — pushes `REGEX_BODY` (R237); mode-path only |
| `` ` `` | `CMD_OPEN` | delimiter §6 | ``` ` ``` — pushes `COMMAND`; mode-path only |
| `"` | `DQ_CLOSE` | delimiter §6 | `"` — pops `DQ_STRING` |
| `"im` | `REGEX_CLOSE` | delimiter §6 | `"[imsxb]*` — pops `REGEX_BODY`, flags included |
| `` ` `` | `CMD_CLOSE` | delimiter §6 | ``` ` ``` — pops `COMMAND` |
| `${` | `INTERP_OPEN` | interp §6 | `\$\{` — pushes `INTERP_EXPR` |
| `$name` | `INTERP_IDENT` | interp §6 | `\$[A-Za-z_][A-Za-z0-9_]*` — the two double-quoted modes, `DQ_STRING` and `TRIPLE_DQ_STRING` (§1); longest identifier wins (string §5) |
| `}` | `INTERP_CLOSE` | interp §6 | the `}` returning brace depth to zero; pops to the enclosing literal mode |
| `\n`, `\$`, `\u{1F600}`, … | `ESCAPE_PAIR` | content §6 | `\\u\{[0-9a-fA-F]{1,6}\}\x7c\\[^\n]` in `DQ_STRING`, `\\[^\n]` in `COMMAND`, `(?s)\\.` in `REGEX_BODY` — one backslash-pair, except that `DQ_STRING` matches the **whole** `\u{…}` (R245), that being the one context where the codepoint escape is legal (string §5.1); elsewhere a bare `\u` is an ordinary unknown escape and one pair is the right span for it. A backslash-newline is not a pair in a newline-bounded form (R244), so a trailing `\` cannot continue the literal. R150 gives commands `` \` `` `\\` `\$` (command §2); decoding per the authoritative table, string §5.1 |
| text run (dq) | `DQ_TEXT` | content §6 | `[^"\\$\n]+` in `DQ_STRING`, newline-bounded (R244); `[^\\$\n]+\x7c\n` in `TRIPLE_DQ_STRING`, where a line break is content and **a `"` is too** — the closer is recognized only at a line start after the margin, so the run never competes with it (R246) |
| text run (raw triple) | `RAW_TEXT` | content §6 | `[^\n]+\x7c\n` — `TRIPLE_SQ_STRING` only: no escapes, no interpolation, so `\`, `$`, and `'` are all ordinary bytes (R246) |
| lone `$` | `DOLLAR_TEXT` | content §6 | `\$` — a `$` that starts no interp form is literal content; one name across all three modes, as `ESCAPE_PAIR` is (in commands: `$` not followed by `{`, command §2.2) |
| text run (regex) | `REGEX_TEXT` | content §6 | `[^"\\$]+` — newlines included, regex being the one form that may span lines (R244) |
| text run (command) | `CMD_TEXT` | content §6 | ``[^`\\$\n]+`` — newline-bounded (R244); backslash excluded since R150, commands having escapes |
| `foo`, `snake_case`, `camelCase` | `IDENT` | identifier §7 | `[A-Za-z_][A-Za-z0-9_]*` |
| `_` | `WILDCARD` | identifier §7 | `_\b` |
| any byte beginning no token | `INVALID` | error §11 | *(no pattern: the catch-all, attempted last)* — one byte, with `L0012`. What makes the lexer **total**: every byte begins a token, so §2's tiling holds on invalid input too (R242) |
## 1. Lexer modes

Three literal forms admit `${expr}` interpolation whose body is a full Luna expression —
including nested strings, braces, and (in principle) nested literals of the same kind — so
those literals are **not regular languages** and cannot be tokenized by any single RE2
pattern (see flags F1–F3). The lexer therefore runs a small mode stack:

| Mode | Entered by | Left by |
|-|-|-|
| `DEFAULT` | start of file | — |
| `DQ_STRING` | `"` (`DQ_OPEN`, §6) | unescaped `"` (`DQ_CLOSE`), **or a raw newline** — `L0009` (R244) |
| `TRIPLE_DQ_STRING` | `"""` ending its line (`TRIPLE_DQ_OPEN`, R246) | a line whose margin is followed by `"""` (`TRIPLE_DQ_CLOSE`) |
| `TRIPLE_SQ_STRING` | `'''` ending its line (`TRIPLE_SQ_OPEN`, R246) | a line whose margin is followed by `'''` (`TRIPLE_SQ_CLOSE`) |
| `REGEX_BODY` | `~"` (`REGEX_OPEN`, R237) | unescaped `"`, then flags (`REGEX_CLOSE`) — the one mode a newline does not end |
| `COMMAND` | `` ` `` (`CMD_OPEN`) | unescaped `` ` `` (`CMD_CLOSE`), **or a raw newline** — `L0009` (R244) |
| `INTERP_EXPR` | `${` inside any of the three modes above | the `}` that returns brace depth to zero (depth counted by the lexer), or the enclosing literal's newline (R244) |

`INTERP_EXPR` lexes with the full `DEFAULT` rule set. Single-quoted strings and `b"..."`
bytes literals do **not** interpolate (string §5, bytes §7) and are handled by a single
regex each. Inside a splice **margin checking is suspended** (R246): the content there is
code, where newlines are insignificant (lexical-structure §1), so a `${…}` may span lines
and its lines carry no margin obligation — the alternative would impose an offside rule in
the one place Luna has none.

The two triple modes are the multi-line literals (R246). `TRIPLE_DQ_STRING` interpolates and
escapes exactly as `DQ_STRING` does — **exactly**, including the bare `$name` form, which §0
and §6 formerly restricted to `DQ_STRING` and which is corrected with R262; `TRIPLE_SQ_STRING`
does neither, and holds a single `RAW_TEXT` run per line. Neither has a whole-literal fast path: a triple always has margins
to tokenize, so it always takes the delimited shape (§6).

## 2. Whitespace and comments — the trivia tokens

`WHITESPACE`, `MARGIN`, `SHEBANG`, `LINE_COMMENT`, `BLOCK_COMMENT` (§0, categorized
`trivia §2`). **All five are emitted, not skipped (R236)**, and collectively they are the
**trivia** tokens. They carry no meaning — whitespace is insignificant and comments are inert
(lexical-structure §1, §3) — so every consumer but one drops them; the lexer emits them anyway,
because the formatter (`luna -f`, compiler §0.1) cannot reproduce what the lexer discarded, and
it is the only component that needs them. The parser filters trivia from its view, which is one
predicate over the stream and cheaper than maintaining a second lexer mode that suppresses them.
Fidelity rides on the **span**, not on token kinds: a trivia token carries a byte range into the
retained source (§9), so one `WHITESPACE` run reports its exact bytes — tabs versus spaces, `\n`
versus `\r\n` — with no need for per-character token kinds to distinguish them.

`MARGIN` is the newest and the least obvious, and it belongs here by the same definition
(R246): the indentation stripped from a multi-line literal's content lines is *layout*, not
data, so it carries no meaning and only the formatter needs it — which is why reindenting a
triple literal is the ordinary trivia rewrite rather than a special case. Emitting it is what
makes stripping a **classification** rather than a transformation: nothing removes the margin,
the decoder simply concatenates content and skips trivia, so the rule exists in one place
instead of being implemented twice in two places that must agree. Stripped trailing whitespace
in a `"""` line is ordinary `WHITESPACE` for the same reason, and the whole `"""`-versus-`'''`
difference reduces to whether a line's trailing whitespace is classified as trivia or content.
Escapes are safe from all of this by construction: trivia is only ever assigned to literal
whitespace bytes, and an `ESCAPE_PAIR` is content, so no rule is needed to protect `\t`.

Two consequences are worth stating. **Token spans tile the source exactly** — monotonic, no
gaps, summing to the file length — and since R242 that holds on **invalid** input as well:
every byte begins a token, bytes no real production claims being covered by `INVALID` (§0).
So the stream reconstructs what the scanner saw without consulting the diagnostics, which is
what leaves those purely presentational; and the scanner's rule, one token per step covering at
least one byte, makes termination structural rather than a property to be tested for. And **trivia attachment is deliberately not decided here**: the
stream is flat, with no binding of a comment to a neighbouring token as leading or trailing
matter, because whether a trailing `// …` belongs to the line above or the line below is a
formatting policy and the formatter spec is pending. A flat stream lets that spec decide; an
attached one would freeze the answer before it is asked.

Both comment forms are specified in lexical-structure §3. Block comments are C-style and do
**not** nest, by ruling, so §0's non-nesting pattern is the complete rule (F4). There
is no doc-comment syntax; documentation rides on attributes.

A **shebang** — `#!` as the first two bytes of the file — runs to the byte *before* the next
newline as one `SHEBANG` trivia token (emitted, then dropped by every consumer but the
formatter, R236), so a `.luna` file can be made executable and run directly (R85). The newline
itself is left outside, joining the `WHITESPACE` run that follows, exactly as `LINE_COMMENT`
leaves it: §0's two patterns are both `[^\n]*`-tailed, and §9 gives the reason — a newline
needs no token of its own, its bytes being recoverable from whatever run contains them. It is recognized
**only** at byte offset 0 (the `\A` anchor) and **only** as `#!`; a bare `#` is never a
comment (the `#`-for-comments spelling was weighed and rejected, R85), and `#` otherwise
appears solely in `#[` (attributes §3, §5).

## 3. Keywords

Reserved words per keywords.md §1–§4: 48 word-shaped keywords plus two compound tokens,
`match!` and `yield from` (R223). Recommended implementation: lex an `IDENT` (§7) and
promote via a lookup table, with the one-token peeks of §8 for the compounds, rather
than running 50 patterns; §0 gives the per-word regex for completeness. The word-shaped
keywords all follow `\bword\b`; the two compounds carry their own patterns there.

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

The literal tokens are in §0, categorized `literal §4`; this section owns their rules.

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
unary `+`, so positive infinity is written `inf`, never `+inf`. Regex flags are exactly `i m s x b` (regex §3).

**A literal may span lines exactly when its opener is more than one byte.** That is R244's
rule in the form R246 left it: `~"`, `"""`, and `'''` may; `"`, `'`, and the backtick may
not. The correspondence is the rationale rather than a coincidence — a multi-byte opener is
typed deliberately, where a stray single quote is the ordinary typo, and it is the stray one
that would otherwise swallow the rest of a file.

**A raw newline ends every literal but the regex (R244).** `'…'`, `"…"`, `b"…"`, `b'…'`, and
the backtick command literal are bounded by the line they open on; a newline before the closing
delimiter raises `L0009` with its caret on the *opener*, and the newline itself is left to lex as
`WHITESPACE`, so the following line lexes as code. The rule is error locality rather than
recovery: quotes pair greedily, so an unterminated `"` in a file with more strings below closes on
the *next* quote instead of running to end of file, mis-tokenizing every line between and
surfacing its diagnostic — if at all — on some later, innocent literal. Bounding at the newline is
what puts the caret on the typo. A `${…}` splice inherits its literal's rule, and a
backslash-newline is not an escape pair (§0), so a trailing `\` cannot continue the literal
either. **The `x`-flag verbose form is the exemption**: spelling a long pattern across lines is
that flag's purpose (regex §4), so `REGEX` keeps `(?s)` and its span pattern is unaffected,
seeking only the unescaped closing `"` — safe here because `~"` is a deliberate two-byte opener
rather than the single stray byte the rule exists to contain.

**Multi-line literals, ruled in full (R246).** `"""` is `"…"` with more lines — same escape
table (string §5.1), same interpolation — and `'''` is raw: no escapes, no interpolation, `\`
and `$` ordinary bytes. Six rules:

- **The opener ends its line**; trailing whitespace is tolerated, then the newline. Anything
  may precede it, so `const s = """` is ordinary. Content after it is `L0015`. This is what
  makes the first content line unexceptional, so the margin applies uniformly from line one.
- **The closer begins its line**, after the margin; anything may follow it, so `""";` and
  `""".trim()` and `""", x)` all work. So content may contain `"""` anywhere *except* at the
  start of a line at exactly the margin.
- **The margin is the closer's indentation**, matched as a **byte prefix** and never as a
  column — a column comparison needs a tab width and §9 refuses to pick one. A non-blank
  content line that does not begin with those exact bytes is `L0014`; a line indented deeper
  keeps the excess as data; blank and whitespace-only lines are exempt.
- **The newline before the closer belongs to the delimiter**, so the closing token spans
  `\n<margin>"""` and a value ends without a trailing newline unless a blank line supplies
  one. `\<newline>` is accordingly an unknown escape (`L0005`), not a continuation — `\n`
  already spells the intent, and adding the continuation later stays backward-compatible.
- **`"""` strips each line's trailing whitespace; `'''` keeps it.** Trailing whitespace is not
  durable in source — editors, `.editorconfig`, and CI hooks delete it — so a `"""` value
  never depends on bytes that get taken away, and `\u{20}` is the durable spelling. `'''` is
  where whitespace-sensitive content goes, and saying so at the call site is the signal a
  reader and a formatter need. The **`\r` of a CRLF line ending follows the same split**
  (R249): in `"""` it is trailing whitespace and is stripped, so a value reads the same from
  a CRLF checkout as from an LF one; in `'''` it is content, so the value differs. Raw means
  raw, and a project depending on a `'''` value pins its line endings in `.gitattributes` —
  the obligation is a repository's, not the language's.
- **A splice suspends the margin** (§1): inside `${…}` the content is code, so it may span
  lines with no margin obligation.

Bytes literals have **no triple form**: binary data is not line-oriented, so `b"""` is an
empty `BYTES` followed by a quote, exactly as it is today.

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

§0 lists these as `operator §5` (37) and `punctuation §5` (10), grouped longest-first within
each family; §8 gives the normative global order.

Notes. The pipe tokens are written with the hex escape `\x7c` (Go regex for `|`) so the
patterns survive markdown table rendering byte-for-byte. There is
no unary `+`, no `===`/`!==`, no bitwise tokens, and no `&&=`
/`||=` (numeric-operators §1.1; associativity §4; int §8). The **conditional** `c ? a : b`
does exist (R254, associativity §1 tier 11) and needs no token of its own: it is `QUESTION`
and `COLON`, both already here, resolved by position like every other dual-role token — which
is why this note previously and wrongly listed it among the absent forms. `:` serves
annotations (`x: int`), slice bounds (`xs[1:3]`, `xs[:]`, tables §3), default-bearing
signatures, and the conditional's arm separator.
`#` occurs only as part of `ATTR_OPEN` (attributes §3), or as the leading `#!` of a
first-line shebang (§2) — a bare `#` anywhere else is a lex error. (`*` was the import glob until R136 retired it; bare-path imports mean "everything" now.) `SLASH` and `SLASH_ASSIGN`
compete only with the two comment forms, and the **next** byte decides between them (`//`, `/*`,
`/=`, `/`) — no context, no lookbehind. `~` is not an operator: its sole appearance is opening a
`REGEX` literal (`~"…"`, R237, F2), and a `~` not followed by `"` is a lex error, exactly as a
bare `#` is.

## 6. Interpolation sub-tokens (modes `DQ_STRING`, `REGEX_BODY`, `COMMAND`)

**Opening delimiters are tokens in their own right** (R235, closing an inventory gap this file
carried: the closers were named, the openers only described): `"` (`DQ_OPEN`), `~"`
(`REGEX_OPEN`, R237), `` ` `` (`CMD_OPEN`). They are emitted in `DEFAULT` or
`INTERP_EXPR`, push their mode, and are **mode-path only** — where the literal's span regex (§0) applies, the
whole literal is one `STRING_DQ` / `REGEX` / `COMMAND` token and no opener or closer is emitted.
So the same source construct reaches the parser in one of two shapes, single token or delimited
sequence, and the parser accepts both; the split is F1/F3's fast path, not two grammars.

Closing delimiters end the mode: `"` (`DQ_CLOSE`), unescaped `"` plus `[imsxb]*`
(`REGEX_CLOSE`), `` ` `` (`CMD_CLOSE`). Inside `INTERP_EXPR` the lexer counts `LBRACE` /
`RBRACE`; the `RBRACE` that returns the count to zero is emitted as `INTERP_CLOSE` and
pops back to the enclosing literal mode.

**Spread-splice composes from existing tokens**, and needs no row of its own: `${...expr}` is
`INTERP_OPEN` followed by `SPREAD` inside `INTERP_EXPR` (command §3, spread §5), the `...`
being an ordinary operator once the splice has pushed the expression mode.

**Attempt order inside a literal mode** (§8 covers `DEFAULT` / `INTERP_EXPR`; this is the
mode-internal half, and `DOLLAR_TEXT` is why it must be stated): the mode's closing delimiter,
then `ESCAPE_PAIR`, then `INTERP_OPEN`, then `INTERP_IDENT` (the two double-quoted modes, §1), then
`DOLLAR_TEXT`, then the mode's text run. The `$` chain is the load-bearing part —
`DOLLAR_TEXT`'s pattern `\$` would match at *every* `$` if tried first, and is correct only
because the two interpolation forms are attempted before it. This is also why the text runs
exclude `$` from their classes: a run that could swallow `$` would consume `${` before
`INTERP_OPEN` ever saw it.

**The triple modes add two line-shaped steps** (R246), and they come first because both are
about position rather than content. At a **newline**, the mode's closer is attempted before
anything else: if the following line's indentation is followed by `"""` (or `'''`), that whole
run — `\n`, margin, delimiter — is the closing token, which is how the last newline stops being
content. Otherwise the newline is content and the next line begins with a `MARGIN` trivia token,
whose bytes must be the closer's indentation exactly; a non-blank line that does not begin with
them is `L0014`. Then the ordinary chain above runs, with one addition in `TRIPLE_DQ_STRING`:
whitespace immediately before a newline is split off as `WHITESPACE` trivia rather than joining
the text run, which is the whole of "`\"\"\"` strips trailing whitespace" (§4).
`TRIPLE_SQ_STRING` skips the `$` and escape chain entirely — it has neither — and its line is one
`RAW_TEXT` run, trailing whitespace included.

## 7. Identifiers

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
2. `TRIPLE_DQ_OPEN` and `TRIPLE_SQ_OPEN` — before `DQ_OPEN`/`STRING_DQ` and
   `STRING_SQ` respectively (R246), or `"""` lexes as an empty string followed by a
   quote. Maximal munch, and the only ordering the triples need.
3. `BYTES` — before `IDENT`, or `b"…"` lexes as `IDENT(b)` + string.
4. `REGEX` — `~"`-initial and self-identifying (R237): `~` is a token in no other
   position, so no ordering constraint applies and no context is consulted.
5. `DOUBLE` (both rows), then `INT_HEX`, `INT_BIN`, `INT_OCT`, then the leading-zero error
   production, then `INT_DEC` — doubles first so `1.5` is one token; the radix prefixes
   before decimal so `0x10` doesn't lex as `INT(0)` + `IDENT(x10)`; and the error
   production before `INT_DEC` so `0755` is diagnosed rather than split into adjacent
   integers (§4, R238).
6. Operators, longest lexeme first: `???=` › `???` › `??=` › `??` › `?->` › `?.` › `?`; `...` ›
   `..<` › `..` › `.`; `||` › `|`; `&&` › `&`; `=>` and `==` › `=`; `->` and
   `-=` › `-`; `@@` › `@`; `!=` › `!`; `<=` › `<`; `>=` › `>`; `#[` before any bare `#`.
7. Keywords (with `KW_MATCH_BANG` before `KW_MATCH`, and `KW_YIELD_FROM` before
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
is built **lazily, on the first diagnostic that needs one**, a byte offset becomes a line by
binary search, and the column is a rune count over that line's prefix alone — bounded by line
length, never file length. A compile that emits no diagnostic builds no table and runs no
conversion; one that does emit is already about to do IO, so the scan is off the path that
matters. Line starts are the offsets **just after each `\n`**, with `\r` left as ordinary
content on the preceding line, so CRLF needs no special case. Newlines accordingly need no token
of their own: folding them into a `WHITESPACE` run (§2) loses nothing, the run's bytes being
recoverable from its span, and splitting runs at newlines would roughly double the trivia count
for a fact no consumer reads — the parser least of all, statements being `;`-terminated.

**The pure-ASCII fast path falls out of a pass that already exists.** The UTF-8 validation of
lexical-structure §1 visits every byte before tokenizing, establishing validity "exactly once, up
front"; recording whether any byte was ≥ `0x80` costs nothing there,
and in a file where none was — the overwhelming majority — the rune column *is* the byte column
and the conversion is subtraction. Even in a file that has non-ASCII, lexical-structure §1
confines it to string, `command`, `regex`, and comment content, so the counting path only ever
runs for a diagnostic on a line carrying such content.

**Tabs need no decision.** For the reported *location* a tab is one column, already implied by
the rune count above since a tab is one scalar (R236) — which is also what keeps diagnostic
tests free of any tab-width setting (testing-strategy §2). For *rendering*, a caret line built
by copying the source prefix and replacing every non-tab character with a space, tabs left as
tabs, aligns under any tab width without one ever being chosen; only wide characters (one rune,
two cells) need more than that, and they are a renderer refinement rather than a language
decision.

## 10. Token inventory

Every token this file defines, by owning section. **Names and counts only — the patterns live
in their sections and are not duplicated here**, so this table is an index rather than a second
source of truth. Its purpose is mechanical: a count that can be asserted in a test, and a list
that makes an omission visible. R232 fixed a "47 patterns" claim standing over a 49-row table;
this section exists so that class of drift fails loudly instead of hiding.

**134 tokens over 138 rows.** By §0's category column: **5** trivia, **50** keyword (48
word-shaped plus the compounds `KW_MATCH_BANG` and `KW_YIELD_FROM`), **10** literal, **37**
operator, **10** punctuation, **10** delimiter (five openers, five closers), **3** interp,
**6** content, **2** identifier, **1** error.

**Rows exceed names in three places**, which is where a naive recount goes wrong. `DOUBLE`
owns two rows (point and exponent form), `BYTES` owns two (`b"…"`, `b'…'`), and `INVALID` owns
three — the two error productions and the catch-all — so those seven rows are three names.
`STRING_SQ`/`STRING_DQ` look like the same case and are not: two rows, two genuine names. §3's
50 matches keywords.md §1–§4 exactly.

Every row now names a token (R242). The arithmetic used to carry a third case, rows that were
not tokens at all, and it is gone: the error productions emit `INVALID` alongside their
diagnostic rather than emitting nothing, which is what makes §2's tiling total.

**Not tokens, and deliberately absent:** the seven *modes* of §1 (`DEFAULT`, `DQ_STRING`,
`TRIPLE_DQ_STRING`, `TRIPLE_SQ_STRING`, `REGEX_BODY`, `COMMAND`, `INTERP_EXPR` — note
`COMMAND` names both a mode and a token, the one collision in the vocabulary). Nothing else: since R242 every §0 row names a token, including
the three that also raise a diagnostic.

`DOLLAR_TEXT` was **found by compiling this table** (R239): §6's lone-`$` row carried a pattern
and no name, the same gap R235 closed for the openers, one row further down. Only `""`, `~"…"`,
and `` `…` `` are affected — a single-quoted string is one span regex with no mode (§1), so `$`
is ordinary content inside `[^'\\]` there and never reaches this question.

---


## 11. Error summary (R240)

Every lexical error, with the code that names it. Codes are `L` + four digits, allocated
append-only and never reused (compiler §3.1). Each has a fixed **title**; the description is
per-instance and volatile. Tests pin the code and the primary span, never the prose
(testing-strategy §2).

| Code | Title | Raised when | Authority |
|-|-|-|-|
| `L0001` | Invalid UTF-8 | A source byte sequence is not valid UTF-8; rejected at ingress, before tokenizing | lexical-structure §1 |
| `L0002` | Byte-order mark | A file begins with U+FEFF, which is an ordinary codepoint here and never stripped | lexical-structure §1 |
| `L0003` | Leading zero | A decimal literal begins `0` followed by another digit or `_` (`0755`, `0_1`) — write `0o755` for octal | §0, R238 |
| `L0004` | Uppercase radix prefix | `0X`, `0B`, or `0O`; prefixes are lowercase only | §0, R238 |
| `L0005` | Unknown escape | A `\` followed by a character absent from its context's row in the escape table | string §5.1, R150 |
| `L0006` | Invalid codepoint escape | `\u{…}` naming a surrogate (`D800`–`DFFF`) or a value above `10FFFF` | string §5.1, R150 |
| `L0007` | Unexpected `#` | A `#` that is neither `#[` nor a first-line `#!` | §5, R85 |
| `L0008` | Unexpected `~` | A `~` not followed by `"`; the sigil exists only to open a regex literal | §5, R237 |
| `L0009` | Unterminated literal | A raw newline inside a string, bytes, or command literal (R244), or end of file inside any literal, the regex included — the description names which | §1, §4 |
| `L0010` | Unterminated block comment | End of file with no closing `*/`; block comments do not nest, so the first `*/` would have closed it | §2, F4 |
| `L0011` | Unterminated interpolation | End of file with `INTERP_EXPR` brace depth above zero | §6 |
| `L0012` | Unexpected character | A byte that begins no token in the current mode (`^`, a bare `\`, a `$` in `DEFAULT`) | §0 |
| `L0013` | Malformed codepoint escape | `\u` not followed by `{`, 1–6 hex digits, `}` — `\u`, `\u{}`, `\u{XYZ}`, `\u{41` (R245). Distinct from `L0006`, which is a **well-formed** escape naming an invalid scalar | §0, string §5.1 |
| `L0014` | Insufficient indentation | A non-blank content line of a `"""` or `'''` literal that does not begin with the margin — the closing delimiter's exact indentation bytes (R246). Mixed tabs and spaces land here, byte-matching being what lets §9 keep refusing to pick a tab width | §4, R246 |
| `L0015` | Content after a multi-line opener | Anything but whitespace between a `"""` or `'''` opener and its newline; the opener owns the rest of its line (R246), which is what makes the first content line unexceptional | §4, R246 |
| `L0016` | Malformed byte escape | `\x` not followed by exactly two hex digits, in a context where `\xNN` is legal at all — bytes literals only (bytes §7). `\x` in a string is `L0005` instead, `x` being absent from that row entirely (R248) | §4, string §5.1, R248 |

**An `INVALID` token covers bytes no other production claims** (§0, R242) — which is a test
about the *stream*, not about this table: a diagnostic does not oblige the lexer to emit
anything, and several of these raise no token of their own (R243). `L0003`, `L0004`, `L0007`,
`L0008`, and `L0012` do, nothing else claiming the bytes they condemn; `L0009` and `L0010` do
on the span-regex fast path, where a literal or comment that never closed leaves the
single-token reading unable to complete — deliberately, and not as a fallback waiting to be
written (R247). That path is taken *precisely* when the literal holds no `${`, so there is no
interior structure to preserve, and one `INVALID` says what is true where a text token would
assert the bytes are string content. The mode path emits real tokens and no `INVALID` for the
same failure, which is not an inconsistency: it is reached only when a splice is present, and
the splice really was lexed correctly. `L0005`, `L0006`, `L0013`, and `L0016` do **not** — an escape sits inside
a well-formed literal, already covered by its `STRING_SQ` or its `ESCAPE_PAIR`, and an `INVALID`
over it would overlap. Nor does `L0011`, every byte of the splice having been emitted as it was
scanned. `L0001` and `L0002` are raised at ingress, where there is no stream at all.

Diagnostics and tokens are separate channels and stay separate: the stream records what the
scanner consumed, the diagnostic records what to tell the author, and neither has to do the
other's job — in either direction (R243). That is what frees a primary span to be a *caret
position* — `L0009` wants its caret on the opening delimiter, not spread across everything up
to end of file — and it is why §2's tiling holds on invalid input.

`L0012` is the catch-all that makes the lexer **total**: every byte begins a token, and any
byte that begins no other one begins an `INVALID`. The scanner's rule is therefore one token
per step covering at least one byte, which makes progress structural — the classic lexer bug,
looping forever on a character it does not recognize, is unwritable rather than untested.
None of these aborts — §1.1 collects lexical errors and the compile stops at the phase
boundary — so each must also leave the scanner able to make progress, `L0009`/`L0010`/`L0011`
being the ones where recovery is least obvious, since the mode stack must be unwound.

No lexical error has a runtime counterpart: all sixteen are compile-time only, and none of them
corresponds to a catchable type in the errors §2 hierarchy.

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
quote. Since R244 that scan is **line-local** — the literal cannot outlive the line it opens
on — so the choice between §6's two shapes is made from bounded lookahead rather than from a
scan that may run to end of file.

The multi-line forms have **no fast path at all** (R246), and neither does the command literal's
triple counterpart, there being none. A `"""` or `'''` literal always takes the delimited shape,
because its margins are tokens (§2) and there is therefore always interior structure to emit —
one fewer decision rather than one more. Their lookahead is a different question and still
bounded: the scanner must find the closing line to learn the margin *before* lexing the body,
which is the same shape as the `${` probe above, run once per literal.

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

**R244 closed the other half of that indictment.** R237 removed the context-sensitivity but not
the swallowing: an unclosed `"` still consumed the rest of the file, the failure mode above with
a different opener. Bounding every literal but the regex at its line ends it. `~"…"` is the
exemption — the `x` flag exists to span lines (§4) — and that is affordable for the same reason
the sigil was: a two-byte opener is typed deliberately, where a stray `"` is the ordinary typo.

The interpolation half stands. `${expr}` inside a regex literal (regex §7, comptime-only) may
itself contain a `"` — inside a nested string in the splice — which would falsely terminate
the span regex; `REGEX_BODY` mode handles it, and §0's `REGEX` pattern is a no-`${` fast path
exactly as for strings (F1) and commands (F3). The span pattern is otherwise sound, including
multi-line `x`-verbose patterns, since it only hunts the unescaped closing `"`. Two collisions
the old delimiter forced are simply gone: `//` is now unambiguously a line comment with no
empty-regex reading — retiring regex §2's `/(?:)/` workaround, an empty pattern being `~""` —
and a `#` comment inside an `x`-verbose pattern can no longer terminate the literal.

**F3 — Command literals with `${expr}` are not regular**, for the same reason as F1: the
splice is a full expression (command §3) and may contain backticks inside nested strings
or nested command literals. The §0 span regex is a no-`${` fast path only, line-local since
R244; otherwise use the `COMMAND` / `INTERP_EXPR` modes. Command literals **do** have escape
sequences — `` \` ``, `\\`, `\$` (command §2.2, R150 superseding G5's original no-escapes
resolution) — so `CMD_TEXT` excludes the backslash and `ESCAPE_PAIR` is emitted in `COMMAND`
mode exactly as in `DQ_STRING`. This paragraph asserted the opposite until R244's sweep, the
last site where G5's retired reading survived.

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
  and C-style `/* … */` block comments that do **not** nest, so §0's `BLOCK_COMMENT` regex is
  the complete rule. There is no doc-comment syntax — attributes carry documentation — and the corpus's
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
