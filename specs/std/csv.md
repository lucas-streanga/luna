# `std.csv`

```
import std.csv;
```

The CSV module, following the pattern `std.json` establishes in full (constraint-typed
boundary, pure functions, comptime-eligible, per-format module so imports pull only what
they use).

## 1. The `csv` type

```
export const csv = constraint s: string where isValidCsv(s);
```

An invariant, entry-only constraint over `string` (constraints §7); the type is the
escaping/validity decision at every boundary (std.json §1.1). `isValidCsv` is
**deferred**, and dialect-relative by nature:
delimiter, quoting, embedded newlines; the predicate (or the constraint itself) likely
wants a dialect parameter (§3).

## 2. Reading

```
export const fromCsv = fn (v: csv): table!;
```

Pure, comptime-eligible, source-agnostic; the entry into `csv` is the validation, and the
`!` arm carries semantic failures beyond well-formedness. File-to-table:
`try fromCsv(readAll(fd) as csv)`.

## 3. Open questions

- **Dialects**: parameterizing delimiter/quote/escape, and whether that is a predicate
  argument or a family of constraints.
- **Headers**: whether the first row maps to keys or `fromCsv` yields a list of rows.
- **Writing side** (`toCsv`): whether the attribute-driven generator pattern
  (std.json §2, attributes §4) transfers wholesale, pending use.
