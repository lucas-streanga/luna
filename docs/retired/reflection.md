# Reflection — retired name, moved (R127)

This spec moved to **`std/introspection.md`** and the surface it describes became the
standard module **`std.introspection`** (R127). The rename is deliberate and
load-bearing: "reflection" is the industry's name for the mutable surface Luna
structurally refuses (runtime type mutation, accessibility overrides), and the
introspection spec's §1 principles are the codified refusal. The built-in operators
(`@`, `@@`, `declared`, `comptype`, `is`, `as`) are untouched by the move — operators
are language, functions are library (introspection §2).

Section map (was → is):

| This file (was) | introspection.md (is) |
|-|-|
| §1 The two tiers, §2 compile-time-known | §3 |
| §3.1 Runtime tier | §4.1 |
| §3.2 Comptime tier, `comptype` | §4.2 |
| §3.3 `TypeKind` | §4.3 (renamed `kind`, function `kindOf`, R128) |
| §3.4 Protocol queries | §4.4 |
| §3.5 `@P` decomposition | §4.5 |
| §4 Canonical use | §5 |
| §5 Operators complement | §2 |
| §6 Open questions | §7 |

No content is authoritative here; cite `introspection §N`.
