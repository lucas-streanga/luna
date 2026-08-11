# `std.xml`

```luna
import std.xml;
```

The XML module, following the pattern `std.json` establishes in full (constraint-typed
boundary, pure functions, comptime-eligible, per-format module so imports pull only what
they use).

## 1. The `xml` type

```luna
export const xml = constraint s: string where isValidXml(s);
```

An invariant, entry-only constraint over `string` (constraints §7); the type is the
escaping/validity decision at every boundary (std.json §1.1). `isValidXml` is
**deferred**: it means **well-formedness
exactly**, never validity against a DTD or schema, which is a document-level question no
string constraint should answer (§3).

## 2. Reading

```luna
export const fromXml = fn (v: xml): table!;
```

Pure, comptime-eligible, source-agnostic; the entry into `xml` is the validation, and the
`!` arm carries semantic failures beyond well-formedness. File-to-table:
`try fromXml(readAll(fd) as xml)`.

## 3. Open questions

- **Shape**: how attributes, children, text nodes, and namespaces map onto one `table`.
- **Schema validity**: out of scope for the constraint by design; whether a separate
  validator belongs anywhere in std at all.
- **Writing side** (`toXml`): whether the attribute-driven generator pattern
  (std.json §2, attributes §4) transfers wholesale, pending use.
