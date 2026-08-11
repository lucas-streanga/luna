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

Three rules, each ruled (R150):

- **`\u{…}` is the strings-side codepoint escape, and the `\xNN` split is safety, not
  taste**: a raw byte in a *string* could break the UTF-8 validity guaranteed at
  ingress (lexical-structure §1 — a lone `\xFF` is not valid UTF-8), so `\xNN` is
  **bytes-only**, while `\u{…}` encodes a codepoint to valid UTF-8 *by construction*.
  Surrogates (`\u{D800}`–`\u{DFFF}`) and values above `\u{10FFFF}` are lex errors.
  Without `\u{…}`, control and invisible characters would be unwritable in strings,
  since the raw-byte door is (correctly) closed.
- **An unknown escape is a lex error.** Any `\` followed by a character not in its
  context's row is a compile error — never PHP's silent pass-through (`"\q"` staying
  `\q`), which is a silent-wrong-value.
- **No `\0` shorthand, no octal**: `\u{0}` spells NUL when genuinely wanted; octal
  escapes are a C-ism with no constituency. (An earlier §5 example showed `"\0"`; it
  is retired by this table.)
