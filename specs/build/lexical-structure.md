# Luna — Lexical Structure

The rules that govern Luna source **before** tokenizing: how a file is encoded, what an
identifier is, and how comments and shebangs are written. This spec is the authority for
those three; `lexer.md` is the authority for *how* the tokens are recognized (modes,
ordering, the per-lexeme patterns), and `keywords.md` for *which* identifiers are reserved.
Literal forms (numbers, strings, bytes, `regex`, `command`) are owned by their type specs
and tokenized per lexer §4/§6; this file owns only source text, identifiers, and comments.

---

## 1. Source text and encoding

A Luna source file is a sequence of bytes that **must be valid UTF-8**. Invalid UTF-8 is
rejected **before** tokenizing — a compile error at ingress, not a runtime surprise and not
a silent substitution. Validity is established exactly once, up front, so every downstream
pass operates on known-good text (the same discipline strings use, string-representation §8).
That pass visits every byte, so it also records whether the file is **pure ASCII** — free here,
and what lets rune columns be computed from byte offsets by subtraction in the common case
(lexer §9, R236).

All of Luna's lexical **syntax** is ASCII: keywords, operators, punctuation, identifiers,
and every literal delimiter are ASCII bytes, so after the UTF-8 check the lexer matches on
ASCII bytes alone (lexer conventions). Non-ASCII codepoints appear **only** inside literal
and comment content — string, `command`, and `regex` bodies and comment text — where they
pass through as UTF-8 and are never interpreted as syntax.

- **No byte-order mark.** A leading U+FEFF is an ordinary codepoint, neither recognized nor
  stripped; a BOM at the start of a file is therefore a lex error. This keeps **byte offset
  0** meaningful, which the shebang rule (§3) depends on.
- **Whitespace is insignificant** except as a token separator and inside literals. Spaces,
  tabs, carriage returns, and newlines (`[ \t\r\n]+`, lexer §2) carry no meaning of their
  own; there is no offside rule and no significant indentation. The multi-line literals
  (`"""`, `'''`, R246) are worth naming as the apparent exception they are not: their
  **margin** is whitespace used as *measurement* — a ruler declaring where a line's data
  begins — which is a fourth role beyond separator, data, and structure, and it is confined
  entirely between the delimiters. Nothing about it decides where a statement or a block
  begins, and it cannot reach the grammar.
- **Newlines are not significant.** Statements are `;`-terminated (every spec example), so a
  line break is just whitespace. A newline is load-bearing in exactly four places: it ends a
  line comment, it ends a shebang line (§3), it ends a string, bytes, or command literal
  (R244, lexer §4), and it separates the content lines of a multi-line literal (R246). The
  third is why an unclosed quote costs one diagnostic on the line it was opened on rather
  than the remainder of the file; the fourth is available only to the literals whose opener
  is more than one byte — `~"`, `"""`, `'''` — which is the same fact stated forwards.
- The grammar imposes **no** line-length, identifier-length, or file-size limit.

Identifiers are **case-sensitive** (§2); nothing else in the source text is.

---

## 2. Identifiers

An identifier is ASCII:

```
IDENT  ::=  [A-Za-z_][A-Za-z0-9_]*
```

A letter or underscore, then letters, digits, and underscores — as actual bytes, tokenized
as `IDENT` (lexer §7). A word that fits this grammar but is **reserved** (keywords §1–§4) is
not an identifier; `keywords.md` is the sole authority for the reserved set.

- **No Unicode identifiers, deliberately.** Names are ASCII-only. This keeps the lexer total
  and ASCII (§1), and sidesteps the confusable-character and normalization hazards that a
  Unicode identifier grammar carries — a "safe by construction" concern, not an oversight.
  Human-language text lives in `string`/`bytes` values, never in names. (Character *ranges*
  are unavailable for the same family of reasons — no `char` type, range §10.)
- **Hyphens are not identifier characters.** `-` is always subtraction; a hyphenated
  identifier would make whitespace load-bearing (the disease removed with infix `.`), so a
  name that would otherwise hyphenate is camelCase instead (`unsafeShellExec`, not `unsafe-shell-exec`)
  (functions §5.6, keywords §6).
- **`_` is the discard.** It is identifier-shaped but is the wildcard/blank, lexed as its own
  token (`WILDCARD`, `_\b`, lexer §7), not an ordinary identifier (wildcard spec, keywords
  §4). A name with a leading underscore and more (`_tmp`) is an ordinary identifier.
- **Predeclared ≠ reserved.** Builtin type names (`int`, `string`, `table`, …), `panic`, and
  std exports (`io`, `json`, …) are ordinary **identifiers** — shadowable, resolved by scope —
  not keywords (keywords §5). They fit this grammar and obey it.

**Casing carries no semantics.** The compiler enforces no capitalization rule; the house
conventions (camelCase for bindings and functions; error and panic type names camelCase too,
ruled R122, keywords §6) are style, not grammar.

---

## 3. Comments and shebang

Two comment forms. Both are **emitted as trivia tokens** and dropped by every consumer except
the formatter (R236, lexer §2) — the lexer no longer discards them, because a formatter cannot
reproduce text it never received:

| Form | Syntax | Ends at |
|-|-|-|
| Line comment | `// …` | the next newline (exclusive) |
| Block comment | `/* … */` | the first `*/` |

- **Block comments do not nest.** The first `*/` closes the comment, so the non-nesting span
  is the complete rule (G3, lexer F4). Nesting — which would let a block comment wrap code
  that itself contains block comments — was **considered and deferred**: it is cheap to
  implement (a depth counter, the shape the lexer already runs for interpolation braces) but
  changes the failure mode (an unbalanced `/*` or `*/` inside commented-out string content
  would miscount), and the need has not arisen. Revisit if it does.
- **Comment interiors are not tokenized.** Everything from the opening delimiter to the
  closing one is raw bytes: a `/`, `//`, `"…"`, or `/foo/` inside a comment is inert. So any
  code — including regex and string literals — can be commented out; only the closing `*/`
  (block) or the newline (line) ends the comment.
- **No doc-comment syntax.** Documentation is carried by **attributes** (attributes spec),
  deliberately — one declaration-metadata mechanism, not a second parallel one.

**Shebang.** A `#!` beginning the **first line** of a file (byte offset 0) runs through
the next newline as one trivia token (R236, lexer §2) and carries no meaning, so a `.luna`
file can be marked executable and run directly, matching Luna's run-a-file-like-a-script model. It is recognized **only** at offset 0 and **only** as
`#!`; a bare `#` is otherwise a lex error, and `#` appears elsewhere solely in `#[` attribute
application (R85, lexer §2/§5).

### 3.1 Comments cannot collide with `regex` (R237)

They no longer share a starting character. A regex literal is `~"…"` (regex §2), so a leading
`/` is only ever a comment (`//`, `/*`) or division — decided by the **next** byte, with no
context consulted — and a regex pattern may begin with any character at all, `/` and `*`
included.

This subsection previously carried an invariant and a workaround, both now retired with the
delimiter that forced them: a regex literal "can never begin with `/` or `*`", and the empty
pattern spelled `/(?:)/` because a bare `//` was a line comment. Under `~"…"` the empty
pattern is `~""`, and `~"/usr/local"` needs no escaping. The history is kept because the
collision is the standard argument for moving comments to `#` — an exchange R85 already
declined, and one this ruling makes moot from the other side.

---

## 4. Resolved and open

**Resolved.**

- UTF-8 required and checked at ingress; BOM forbidden; whitespace and newlines
  insignificant (§1).
- ASCII identifiers only, no Unicode, hyphen is subtraction, `_` is the discard (§2, G6).
- `//` line and non-nesting `/* … */` block comments; documentation via attributes; shebang
  at offset 0 (§3; G3; R85).

**Open.**

- *(The full **escape-sequence table** is **closed by R150** — string §5.1, the one
  authority: per-context rows, `\u{…}` added on the UTF-8-validity split (`\xNN` stays
  bytes-only), unknown escapes are lex errors, no `\0`/octal, and command literals gained
  their three escapes, superseding lexer G5.)*
- Block-comment **nesting** stands deferred (§3), not rejected — **reaffirmed R150**: do not
  implement; the depth counter is cheap but the failure-mode change (miscounting on unbalanced
  delimiters inside commented-out content) has still not earned its keep.
- *(The **casing convention** for builtin error types was **resolved by R122** — camelCase
  everywhere, keywords §6; this bullet had gone stale.)*
