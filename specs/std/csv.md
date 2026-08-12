# `std.csv`

```luna
import std.csv;
```

The CSV module, following the pattern `std.json` establishes in full (constraint-typed
boundary, pure functions, comptime-eligible, per-format module so imports pull only what
they use).

## 1. The `csv` type

```luna
export const csv = constraint s: string where isValidCsv(s);
```

An invariant, entry-only constraint over `string` (constraints §7); the type is the
escaping/validity decision at every boundary (std.json §1.1). `isValidCsv` is
**deferred**, and dialect-relative by nature:
delimiter, quoting, embedded newlines; the predicate (or the constraint itself) likely
wants a dialect parameter (§3).

## 2. Reading

```luna
export const fromCsv = fn (v: csv): table! => {};
```

Pure, comptime-eligible, source-agnostic; the entry into `csv` is the validation, and the
`!` arm carries semantic failures beyond well-formedness. File-to-table:
`try fromCsv(readAll(fd) as csv)`.

## 3. Writing: `toCsv`, the comptime generator

```luna
export const toCsv = comptime fn (ct: comptype): fn (any): csv => {};
```

The writing side exists (R173), shaped exactly as json §2's canonical generator — R157
ruled this at the serialization level (one generator per format, each its own comptime
generator; the format-parameterized writer is rejected there), and this module now
carries its member of that family: `comptype` in, runtime serializer out, the
specialization living in `const`-captured plain data (attributes §4's mechanics, not
repeated here). The generated function's result enters `csv` typed — a writer produces
valid CSV by construction, so the boundary constraint costs nothing on the way out.

Two things are deliberately *not* committed with it: the **column story** — how fields
map to columns, whether a header row is emitted, and the tag-attribute vocabulary for
column names (a `jsonTag` analog) — is §4's headers question, one question shared by
both directions; and a **dynamic walker** (`toCsvDynamic`, the `toJsonDynamic` mirror)
is deferred pending use — a runtime `any`-walker needs the headers and dialect answers
first.

## 4. Open questions

Valid opens, pending an implementation:

- **Dialects**: parameterizing delimiter/quote/escape, and whether that is a predicate
  argument or a family of constraints. Applies to both directions (§2, §3).
- **Headers**: whether the first row maps to keys or `fromCsv` yields a list of rows —
  and, on the writing side, whether `toCsv` emits one and where column names come from
  (§3). One question, both directions.
