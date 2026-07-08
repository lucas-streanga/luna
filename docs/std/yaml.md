# `std.yaml`

```
import std.yaml;
```

The YAML module, following the pattern `std.json` establishes in full (constraint-typed
boundary, pure functions, comptime-eligible, per-format module so imports pull only what
they use).

## 1. The `yaml` type

```
export const yaml = constraint { s: string where isValidYaml(s) };
```

An invariant, entry-only constraint over `string` (constraints §7); the type is the
escaping/validity decision at every boundary (std.json §1.1). `isValidYaml` is
**deferred**, and version-relative (1.1 vs 1.2
differ on scalars like `no` and `on`); the predicate must pin a version (§3).

## 2. Reading

```
export const fromYaml = fn (v: yaml): table!;
```

Pure, comptime-eligible, source-agnostic; the entry into `yaml` is the validation, and the
`!` arm carries semantic failures beyond well-formedness. File-to-table:
`try fromYaml(readAll(fd) as yaml)`.

## 3. Open questions

- **Version**: which YAML the predicate means, and whether anchors/aliases and multiple
  documents are in scope for `fromYaml`.
- **Writing side** (`toYaml`): whether the attribute-driven generator pattern
  (std.json §2, attributes §4) transfers wholesale, pending use.
