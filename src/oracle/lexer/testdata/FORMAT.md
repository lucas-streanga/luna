# Golden file format (`.lex`)

One case per file: Luna source, a `---` line, then the token stream it must produce.

**Cases that raise a diagnostic live in `error_producing/`**; everything directly in this
directory lexes clean. So "here is what valid Luna tokenizes to" is readable without stepping
over the error cases, and the error cases are countable. The harness checks the split rather
than trusting it — a golden whose directory disagrees with whether it diagnoses is a failure,
because a misfiled one is otherwise invisible: it still passes, it just stops meaning what its
directory says.

```
let x = 1;
---
KW_LET 0..3 "let"
WHITESPACE 3..4 " "
IDENT 4..5 "x"
...
```

- **One token per line**: `NAME start..end "lexeme"`.
- **Spans are half-open byte offsets** — `0..3` is `text[0:3]`, matching Go slice syntax.
- **The lexeme is Go `%q`**, so `"\n"`, `"\t"`, and `"\xff"` are unambiguous. Necessary rather
  than decorative: whitespace tokens are invisible otherwise, and error cases carry bytes that
  are not valid UTF-8.
- **Names are lexer §0's**, not Go identifiers — a golden is read against the spec table, so it
  speaks the table's vocabulary. `Kind.String()` returns these.
- **Diagnostics appear inline**, in source order, as `!CODE start..end`. §1.1 collects errors
  and keeps scanning, so a case can hold both, and interleaving them keeps the file reading the
  way the scanner ran.

## Two properties worth relying on

**The dump covers every byte.** Trivia are emitted tokens (R236), so spans tile the source with
no gaps — concatenating the lexeme column reproduces the input exactly. A reviewer can check
that by eye; the harness asserts it.

**The span column is redundant, deliberately.** The lexeme is sliced *from* the span, so a wrong
span already yields a wrong lexeme. Spans stay because `expected 4..5, got 4..6` is a far more
legible failure than a diff of two quoted strings.

## Limits and conventions

**The input keeps its trailing newline**, because everything before `---` is the input and real
files end with one. A file *without* a trailing newline cannot be represented here — the same
limitation `txtar` has — so those cases belong in Go table tests instead.

**There is no comment syntax, and none is needed.** A case that needs explaining carries a real
Luna `//` comment, which then appears in the token stream and is itself coverage. Anything
shorter goes in the filename.

**Columns are not aligned.** Alignment scans better, but one long token name reflows every line
and buries the real change — and the diff is the review surface.

## The rule

A golden is never trustworthy merely because the lexer produced it. Committing tool output as
the expectation asserts only that the lexer still does what it did first, bugs included. A
regenerate flag is fine; the discipline is that the resulting **diff** is read against §0 before
it is committed.
