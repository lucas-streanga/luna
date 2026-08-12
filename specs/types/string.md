# String

The `string` type itself: immutability and what it means for bindings, the units
doctrine, the operator surface, the builder's place, and interpolation with its escape
table. The function catalogue is **string-api** (its own document, R227); the internal
layout (inline vs. descriptor, borrowed slices, UTF-8 validity) is
**string-representation**.

---

## 1. Immutable, and what that means for bindings

**Strings are immutable, so every operation is pure.** No method mutates its
receiver; each returns a new string (or a view, number, bool, or table). There is no
`&`-write-back *into* a string the way there is into a table, because there is no
interior to write into; a `&` reference to a **`var`** string binding follows the
general rule (variables §5.1) and can only **rebind the slot** to a new string.

**Binding modes follow the variables spec exactly; strings add no carve-out.** `var s`
may be rebound to a new string; `let s` fixes the binding and may **never** be rebound
(`reassignmentError`, variables §1.1), the "new value, not a mutation" reading is
wrong, rebinding is precisely what `let` forbids; `const s` is the same as `let` here,
since an immutable string has no interior for `const` to additionally freeze
(variables §3.1). Since strings have no in-place mutation at all, the practical ladder
is: `var` to reassign, `let`/`const` (equivalent) otherwise.

---

## 2. Units: no length, no integer indexing

There is no `length` and no integer indexing, because "the length of a string" and
"the character at position i" each mean three different things in UTF-8 and silently
picking one is the most common source of Unicode bugs. Instead the units are explicit —
the three counts `byteLength` / `codepointCount` / `graphemeCount` — and positions are
**byte offsets**, the only unit that is O(1) to act on. The count functions and the
byte-offset conventions every signature assumes are the catalogue's (string-api §2).

---

## 3. There is no concatenation operator

An infix `.` concatenation (and its `.=` compound) existed in earlier drafts and is
**removed**, vestigial (operators §0.1). Two reasons beyond redundancy: infix ` . `
collides grammatically with member access `a.b`, making whitespace load-bearing at a parser
choice point, and its implicit `toString` coercion of both operands was the one silent
coercion in a language that forbids them. Luna joins strings with exactly two mechanisms,
each already the better tool at its scale:

- **Interpolation** (§5) for a fixed, small number of joins: `"n=$x"`, `"hi $name!"`,
  `"${a}${b}"`. Conversion is visible (interpolation renders via `toString`, conversion
  spec, by stated rule rather than operator side-effect), and the join is one allocation.
- **The builder** (§4) for accumulation, and **`join`** (string-api §8) for a known
  collection; a
  loop of pairwise concatenations was O(n^2) under the old operator and the builder is the
  answer the old section already pointed at.

Arithmetic `+` stays strictly numeric: `"3" + 4` is a compile error, never a concat and
never a coercion.

---

## 4. String builder

Because strings are immutable, repeated concatenation reallocates on every step, an O(n^2)
loop. The **string builder** is the mutable, amortized-O(1)-append accumulator that solves
this: you append into it freely, then materialize a `string` once. It is the one place in
the string story where mutation lives.

The builder is realized as a table with the applied `stringBuilder` protocol (an element-empty
table whose growable buffer is an ungranted per-table protocol member), so its operations
are protocol functions reached with `->`:

```luna
var b = builder();
&b->append("Hello, ")->append(name)->append("!");   // chainable; & writes back
let greeting = b->build();                           // immutable string
```

The full builder API, construction, the surface (`append`, `appendAll`,
`appendCodepoint`, `reserve`, `byteLength`, `isEmpty`, `clear`, `build`), reference
passing, performance, and concurrency, is specified in **string-builder** (its own
document; `take` is retired there, R99 — `build()` is COW-cheap and build-and-drop *is*
the zero-copy path). Interpolation (§5) lowers to a builder, so an interpolated literal
is one builder pass rather than a chain of reallocating concatenations.

---

## 5. Interpolation

Interpolation is lexer-level surface syntax that lowers to builder calls (§4), so a
`"$a$b$c"` literal costs one builder pass rather than a chain of reallocating `.`
joins. The rules follow PHP for quoting and Perl for what may be spliced:

- **Double quotes interpolate and honor escapes.** `"$name"`, `"${expr}"`, and escapes
  like `"\n"`, `"\t"`, `"\$"`, `"\u{1F600}"` are all live in a double-quoted literal —
  the complete set is §5.1's table, the one authority.
- **Single quotes are literal.** `'...'` neither interpolates nor processes escapes;
  `'$name\n'` is the eight characters `$`, `n`, `a`, `m`, `e`, `\`, `n` (and a literal
  `$`). The only single-quote escape is `\'` for a quote itself (and `\\` for a
  backslash), matching PHP.
- **`$name`** splices a bare variable (longest valid identifier wins).
- **`${expr}`** splices an **arbitrary expression** (Perl-style): `"${a.b + c}"`,
  `"${items.count()}"`, `"${x ?? "none"}"`. The braces bound the expression, so it may
  contain anything, including nested double-quoted strings.
- Every spliced value is rendered with `toString`, the same coercion `.` uses.

Lowering: a double-quoted literal with interpolations becomes a builder that appends
the literal runs and the `toString` of each splice in order, then `build()`s once. So
interpolation and manual builder use share one mechanism, and interpolation carries no
hidden O(n^2) cost.

### 5.1 The escape table (R150)

The complete escape set, per literal context — this table is the one authority
(lexical-structure §4 and lexer G4 point here; bytes §7 and command §2 defer to it for
their shared rows):

| Context | Escapes |
|-|-|
| `"…"` double-quoted string | `\n` `\t` `\r` `\\` `\"` `\$` (a literal dollar — suppresses interpolation) `\u{H…}` (1–6 hex digits, a Unicode scalar value) |
| `'…'` single-quoted string | `\'` `\\` only (above — literal strings stay literal) |
| `b"…"` bytes literal | `\n` `\t` `\r` `\\` `\"` `\xNN` (one raw byte) — no `\$` (no interpolation), no `\u{}` (bytes §7) |
| `` `…` `` command literal | `` \` `` `\\` `\$` |
| `~"…"` regex literal | the regex spec's own escape language (RE2's), passed through undecoded; Luna decodes exactly one escape, `\"`, so a quote can appear inside the delimiters (regex §2, R237) |

**A backslash-newline is not an escape pair** (R244, R246). A raw newline ends a single-quote
or double-quote string literal — and bytes and command literals with them — so a trailing `\`
cannot carry the literal onto the next line; the newline raises `L0009` with its caret on the
opening quote. That holds inside `"""` too, where `\<newline>` is an unknown escape rather
than a line continuation (`L0005`): `\n` already spells the intent, and leaving it an error
keeps the continuation addable later without breaking anything.

**Multi-line text has its own forms** (R246, lexer §4): `"""…"""`, which is a double-quoted
string with more lines — same escape table, same interpolation — and `'''…'''`, which is raw:
no escapes at all, no interpolation, `\` and `$` ordinary bytes. Both strip a **margin**, the
closing delimiter's indentation, from every content line; `"""` also strips each line's
trailing whitespace, where `'''` preserves it, which is what makes `'''` the form for
whitespace-sensitive content.

That split reaches line endings too (R249). A CRLF's `\r` is trailing whitespace, so `"""`
strips it and a value reads the same however the file was checked out, while `'''` keeps it
and the value differs between a CRLF and an LF checkout. **Raw means raw**: the guarantee
`'''` makes is one sentence — every byte between the delimiters, minus the margin — and
"every byte except a `\r` before a newline" would not be, would be invisible in the source,
and would leave no way to write a literal CRLF. So the obligation sits with the repository,
which pins line endings in `.gitattributes` as it would for any byte-exact fixture; `"""` is
the right default for text that should not vary by checkout. `'''` needs no escapes because the closer must begin its line,
so a mid-line `'''` is ordinary content and the containment problem `\'` solves for `'…'`
does not arise.

Three rules, each ruled (R150):

- **`\u{…}` is the strings-side codepoint escape, and the `\xNN` split is safety, not
  taste**: a raw byte in a *string* could break the UTF-8 validity guaranteed at
  ingress (lexical-structure §1 — a lone `\xFF` is not valid UTF-8), so `\xNN` is
  **bytes-only**, while `\u{…}` encodes a codepoint to valid UTF-8 *by construction*.
  Surrogates (`\u{D800}`–`\u{DFFF}`) and values above `\u{10FFFF}` are lex errors
  (`L0006`). Without `\u{…}`, control and invisible characters would be unwritable in
  strings, since the raw-byte door is (correctly) closed. **Malformed is a separate
  error from invalid** (`L0013`, R245): `\u{D800}` is well formed and names a scalar
  that does not exist, where `\u`, `\u{}`, `\u{XYZ}`, and `\u{41` are not escapes at
  all. Two mistakes, two fixes, two messages — and the split is what lets the lexer
  make the token cover the escape's whole extent (lexer §0).
**The check is staged, and the lexer runs it** (R248, closing R243's open question). Three
questions about one backslash pair, in order, each reachable only by passing the one before:
**is the escape character in this context's row?** — if not, `L0005`, which covers `\q` and
also a legal-looking escape in the wrong literal, `'\n'` or `b"\u{41}"`; **is its shape well
formed?** — `\u{…}` wants one to six hex digits in braces (`L0013`), `\xNN` wants exactly two
(`L0016`); **is its value legal?** — a surrogate or a value above `10FFFF` is `L0006`. The
ordering is what keeps the codes disjoint: `\x` in a double-quoted string is `L0005` rather
than `L0016`, `x` being absent from that row entirely. The regex row needs no check at all,
its escapes passing through to RE2 undecoded. The lexer owns this because it alone knows the
literal form, which is this table's key; the same table later answers "what is its value" for
whoever decodes.

- **An unknown escape is a lex error.** Any `\` followed by a character not in its
  context's row is a compile error — never PHP's silent pass-through (`"\q"` staying
  `\q`), which is a silent-wrong-value.
- **No `\0` shorthand, no octal**: `\u{0}` spells NUL when genuinely wanted; octal
  escapes are a C-ism with no constituency. (An earlier §5 example showed `"\0"`; it
  is retired by this table.)
