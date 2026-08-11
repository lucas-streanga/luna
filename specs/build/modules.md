# Modules

Luna's module system is deliberately small. A **file is a module**, a module's **path is its
identity**, imports are **static** and form a **directed acyclic graph**, and importing simply
brings a module's public names into scope. There is no module keyword, no module name
declaration, no runtime module object, and no cyclic dependencies. The whole system is a few
rules that compose, chosen so it stays easy to reason about rather than accreting mechanics.

---

## 1. A file is a module

Every source file **is** a module. There is no `module { ... }` wrapper and no module-name
declaration:

- The file's **path is its identity** (§3). A module is never given a name; naming would be
  redundant with the path and could disagree with it, so it is not allowed.
- **Top-level `export` declarations are the module's exports**; top-level declarations without
  `export` are private to the module.

```luna
// file: text/strings.luna   , this file is the module `text.strings`
export const parse = fn (...) => ...;      // exported
export const split = fn (...) => ...;      // exported
const helper = fn (...) => ...;         // private
```

The file with the `main` function is the **main module**, implicitly; it needs no declaration
either, and it defines the project root (§3).

### 1.1 `export` on the root module is allowed, dormant on an app root

`export` means the same thing everywhere: "exported if this module is imported." On a **library
root** (§9.1), `export` is essential, it is exactly how the library exposes its API to consumers.
On an **application root** (the `main` file), nothing imports the module, so a `export` binding is
exported to no one: a harmless **no-op**, not an error.

This is deliberate. `export` is not *wrong* in the app root, it is merely **dormant in this
configuration**, its inertness following from the fact that nothing imports the root, not from
anything broken in the code. Erroring would special-case the keyword by module role and create a
refactoring wall (extracting the file into a library, or removing `main`, would flip the same
`export` between error and required). So the rule is uniform: `export` marks exports everywhere, and an
export goes nowhere when nothing imports the module. Hard errors are reserved for genuinely
broken code (unresolved names, type errors, cycles), not for inert-but-harmless declarations,
and there is no warning either, dormant `export` is simply allowed and ignored.

---

## 2. No cycles: the import graph is a DAG

Modules **may not** import each other cyclically. The import graph is a **directed acyclic
graph**, known in full at compile time. This is a hard rule, not a warning, and it buys three
things:

- **Parallel compilation.** Modules with no import path between them compile independently; a
  topological sort compiles them in waves across cores.
- **Deterministic initialization.** A DAG has a clean topological init order, so there is no
  "which module initializes first" ambiguity that cycles create.
- **Incremental rebuilds.** Changing a module requires recompiling only it and its transitive
  dependents.

The cost, stated plainly: **mutually dependent code must live in the same module.** If two
modules genuinely need each other, the shared part is extracted into a third module they both
import, or they are merged. This is treated as a design improvement, not a limitation: cyclic
module dependencies are a structuring mistake, and the rule forces the fix.

---

## 3. Paths are relative to the root, which has the empty path

Every module's path is resolved from a single **root**, so a module's path is the same
everywhere it is referenced (no relative-path ambiguity where the same module could be reached
by two different paths). Crucially, paths are **relative to the root and carry no project name**:
`strings.luna` under the root is the module `strings`, `utils/parse.luna` is `utils.parse`. The
**root module itself has the empty path** `""`.

**The root module cannot be imported** (R251). Its file sits in the tree like any other, so
`import app;` would resolve to it and hand one file a second identity — `""` and `app` — which
is the two-paths ambiguity this section exists to prevent, arriving through the one module
exempted from the naming rule. An import resolving to the root's file is an error of its own,
raised by import validation (compiler §1.2). It is deliberately *not* reported as a cycle,
though it closes one: a cycle report sends the reader hunting for a dependency to invert, when
the fix is to stop importing the entry.

This makes module identity **project-name-independent and rename-safe**. No module path contains
a project name, so renaming or moving the project as a whole breaks nothing: every internal path
(`utils.parse`, and so on) is unchanged. The project name is external metadata with no bearing
on the program's own structure, so it is deliberately kept out of module identity entirely. (It
is also what the import syntax already assumes, `import { parse } from utils` names a
root-relative path with no project prefix.)

The root is **discovered** differently for applications and libraries, but this does not affect
its identity:

- **For an application**, the root is the directory containing the **`main`** file. Obvious, no
  configuration. (The cost: `main` must sit at the top of the tree, and moving it moves the
  root.)
- **For a library** (no `main`), the root is given to the compiler: `luna --library
  src/lib.luna` establishes `src/` as the root. `--library` subsumes a separate `--root` flag,
  naming the entry file *is* naming the root (its directory).

So the same rule, "root is the directory of the entry file, and the root module has the empty
path," serves both; applications find the entry via `main` implicitly, libraries name it
explicitly. **App and library roots are identical in identity** (both have path `""`); they
differ only in discovery (and in role: §9.1). `main` is a special *function* (the execution
entry, §1), not a module identity: the root module is uniformly "the empty-path top module,"
and an application's root merely happens to contain `main`.

**A path segment may be a keyword** (R252): `import test.helpers;` and `import error.codes;`
are ordinary imports, and `_` is a segment like any other. A segment is not a name — §5 makes
the path bind nothing — so `test` here collides with nothing, while `test/` and `error/` are
among the most ordinary directory names there are. Only the grammar's view of this one
position changes; the lexer still reads `test` as a keyword. The boundary is that a braced
name list (§5) and the `const` name of an assigned import (§6) *do* bind, so both stay
identifiers.

A path maps directly to the filesystem: `text.strings` is `text/strings.luna` under the root,
and `utils.parse` is `utils/parse.luna`. Module loading is **filesystem-based** and **static**
(§4); dynamic loading is a possible later addition, not part of this system. Modules are
distributed as **source**; a platform-agnostic bytecode form is deliberately not adopted, to
avoid binding the language to a bytecode representation.

Within a single compilation there is exactly **one** root (one empty-path module); mounting one
project's tree under another (consuming a library as a dependency) is the deferred package
concern (§10), not something the single-compilation model needs to resolve.

---

## 4. Imports are static and top-level only

An `import` is **compile-time and top-level** — and imports form the module's **prelude**:
every `import` must precede **all other top-level declarations** (R190). An import after any
non-import declaration is a compile error, raised by import validation (compiler §1.2, R250).

**The prelude is all four cells of §5's grid**, the assignment forms among them (R250): `const
fs = import std.filesystem;` and `export const fs = import …;` are prelude members exactly as
the statement forms are, and a declaration after one of them ends the prelude just the same.
This is stated rather than inferred because the alternative reading is silently wrong — a
`const`-assigned import taken for an ordinary declaration would end the prelude *at itself*,
so discovery would drop its edge and never lex the module, and nothing would catch it: the
form is legal, so the parser has nothing to reject.

The motivations, recorded: there is no use for a late import besides hurting readability (the
rule every formatter would impose anyway), and the prelude is what lets the compiler's
**discovery** stage read a file's imports by scanning only its head (compiler §1.0 —
imports-only lexing stops at the first non-import declaration, O(file-head), with §1.2's error
backstopping the early stop). It **must** appear at module top level,
never inside a function, a block, or a conditional. There are no exceptions.

This is not a restriction users miss, there is no real use for conditional loading, and it is
what makes the rest of the system work: because every import is unconditional and resolved at
compile time, the entire import graph (§2) is known statically, and every name's origin (§7) is
a fixed compile-time fact rather than something that depends on runtime control flow.

---

## 5. Importing brings public names into scope

`import` brings a module's `export` names into the current scope. It does **not** create a module
object or a namespace construct; Luna is data-focused, and where a namespace is wanted, an
ordinary **table** serves (§6). The whole surface is a **two-axis grid** (R136): *position*
decides dump-versus-collect, *braces* decide all-versus-some — and that grid is the motivation
stated as a table:

| | All exports | Some exports |
|-|-|-|
| **Statement** (dumps names) | `import std.filesystem;` | `import { stat, exists } from std.filesystem;` |
| **Assignment** (collects a table, §6) | `const fs = import std.filesystem;` | `const fs = import { stat, exists } from std.filesystem;` |

```luna
import text.strings                                 // every export name, bare
import { parse, split } from text.strings           // exactly these names
import { parse as strParse } from text.strings      // with a rename (collisions, §8)
```

- **Bare** (`import path;`): brings **all** `export` names into scope — the happy path for
  `std.*` and your own modules. The coupling cost is real and stays stated: it ties the
  importer to the module's entire (and future) export surface, which is worth a thought for
  third-party dependencies. **The path never becomes a name**: `import std.filesystem;` binds
  nothing called `filesystem` — this is not Python's `import os`; namespacing is exclusively
  the assignment form's job (§6). This sentence is load-bearing beyond its own paragraph: it
  is the whole reason a path segment may be a keyword (§3, R252), so a proposal to make a path
  bind retires that with it.
- **Selective** (`{ names } from`): brings exactly the named exports. The precise form.
  `from` is **not reserved** (R223, lexer §3): it lexes as an ordinary identifier and the
  parser matches its spelling here, unambiguous because after a braced import list the
  from-clause is the only legal continuation — so `from` stays usable as a binding or
  module name (`import { parse as from } from m;` parses).
- **Aliased** (`{ name as newName }`): renames on import, the collision tool (§8).

There is deliberately **no `*`**: bare-path already means "everything," so a glob sigil would
be a second spelling for the first cell (the pre-R136 `import * from p` is retired). Each
statement form **dumps names** and produces no value; `import` is the language's one bare
directive, and that is fitting — it is mechanics, not a value. Assignment position changes
everything (§6).

**An unused import is a compile error** (R159, the sibling of the unused-binding rule,
variables §4.1): a *selective* import none of whose names are used, or a *bare* import
none of whose dumped names are used, is rejected. Beyond the Go alignment that forces
both rules (compiler §1.7's no-ICE contract; Go rejects unused imports), this one earns
its keep independently: a phantom import is a phantom **dependency edge**, and the
interface-hash cache (build-cache §1) would rehash the importer on every interface
change of a module it never uses.

---

## 6. Collecting a module into a table

To get a namespaced handle instead of loose names, **assign the import** (the grid's second
row, §5), which collects exports into an ordinary **table**:

```luna
const strings = import text.strings;                 // all exports, as a table
const fs = import { stat, exists } from std.filesystem;   // just these two, as a table
strings.parse(x);                                    // ordinary table access
fs.stat(p);
```

- **`const` only** (R136), and this is load-bearing, not style: a `const` table with
  comptime-known members is what lets `fs.stat` **fold** — statically resolved member
  access, statically checked capabilities on the call (the `platform.lineEnding`
  precedent). A `let`/`var`-assigned import would demote every call through it to the
  dynamic frontier for nothing; it is a compile error. Assignment-position imports are
  top-level only, exactly as the statement forms (§4) — and they are **prelude members**
  (R250), so they sit with the other imports at the head of the file and a declaration after
  one of them ends the prelude. `export const fs = import …;` is legal and is a prelude member
  too: re-exporting a collected table is ordinary.
- **Partial collection** (R136): the braced assignment form collects exactly the named
  exports; an alias **renames the key** (`const t = import { parse as jsonParse } from
  …` yields `t.jsonParse`) — §8's one mechanism, one more consumer.
- Collection gathers every export **that can inhabit a table slot** (R135): a
  **capability** export is excluded from the *bare* assigned form, because capability
  tokens never inhabit value slots (capabilities §3.1, the anti-laundering rule) — and
  nothing is lost, because a `use` clause names *bindings* resolved at compile time
  (R19), so a capability is only ever useful arriving through the statement forms (§5),
  which create bindings, not slots. The asymmetry is the design: authority travels by
  name, data travels by value. **Naming a capability in a *braced* assigned form is a
  compile error** (R136), not a silent exclusion: an explicit request for what slots
  cannot hold fails loudly, with the fix in the message (import it as a statement).
- The assigned import yields a `table` whose fields are the collected exports; its type
  is `table`, annotatable and inferable. This is the proof that **a module is not a
  value or a type**: what you get is a plain table, not a "module object." After
  compilation the module itself is gone (§9); only this table, if you built one,
  remains, as data.

So there are two ways to get namespaced access, and they are the author's or the importer's
choice: the **author** exports a single table (`export const strings = { parse: ..., ... }`, then
`import { strings } from ...`), or the **importer** collects loose exports with `const ns =
import ...`. Both yield `ns.parse`; both are just table access. No module-namespace
mechanism is needed because tables already are namespaces.

The distinction in one line: a **statement** `import` dumps names; an
**assigned** `import` is an expression that yields a table. Dumping creates no
variable; collecting creates one (an ordinary table).

---

## 7. Provenance: where a binding comes from

A binding's **origin module** is tracked, but only as **strippable metadata for tooling**,
never as part of the type. This is what lets "where does this name come from" be answerable
without making modules a type.

- Provenance is attached to the **binding**, not the value or the type. Importing `parse`
  records "this binding came from `text.strings`"; the function value and its type are
  unchanged and origin-free.
- It is **semantically inert and strippable**: like a source location, it can be dropped in a
  release build, and its presence never affects type checking, dispatch, or behavior. That is
  the test that it is metadata, not type, remove it and nothing semantic changes.
- It powers **diagnostics** (a collision error naming both source modules, §8) and **tooling**
  ("go to definition," debugger provenance), not language semantics.

Because provenance lives beside the type and not inside it, module identity is observable by
tools but invisible to the type system, so modules never become types through this back door.

### 7.1 `moduleof`: the provenance operator

`moduleof` is a **unary, compile-time prefix operator** (not a function) that yields a `table`
describing the module a binding is defined in:

```luna
moduleof parse           // the module `parse` is defined in: e.g. { path: "strings" }
moduleof someLocal       // a local's defining module is the current module
moduleof mainFn          // a root-module binding: { path: "" } (empty path == the root)
```

- **It is an operator, not a function.** Its operand is a **single identifier** that resolves
  lexically to a binding, not an argument list, not a dotted access, not an expression, not a
  literal. `moduleof a, b`, `moduleof x.y`, and `moduleof expr` are ungrammatical: an operator
  has exactly one operand. It is paren-free, consistent with the other compile-time operators
  (`@`, `@@`, `as`, `is`).
- **It resolves at compile time** via lexical name resolution, so it is zero-cost and never
  runtime-dependent. This is why conditional imports would break it and are forbidden (§4): with
  static top-level imports, every name resolves to exactly one binding with one origin,
  regardless of runtime control flow. `moduleof` reads which *binding* a name refers to (a
  compile-time, lexical fact), not what *value* it currently holds (a runtime fact).
- **It returns the defining module**: the **source module** for an imported binding, and the
  **current module** for a local binding or parameter. So it is total over bindings, every
  binding is defined somewhere. A binding defined in the **root module** has `path: ""`; the
  **empty path is itself the root signal** (no non-root module can have an empty path, since
  every non-root path has at least one segment), so no separate "is root" flag is needed. A blank
  path reads as "the root" and is distinct from a null or missing value.
- **It rejects member access** (`moduleof strings.parse`). Not because dot access is dynamic (it
  is static), but because it is **semantically incoherent**: a table field has no module origin.
  A collected-module table (§6) is indistinguishable from a hand-built table (both are `table`),
  so `strings.parse` is just a field of a table, and table fields do not carry module
  provenance. Supporting the collected-module case would require tables to retain per-field
  module metadata, contradicting the plain-table rule (§6). If you want a binding's provenance,
  import it as a binding (`import { parse } from X`, then `moduleof parse`), which *does* have an
  origin, rather than collecting it into a table field, which does not.

The result is an ordinary `table`, so even provenance introspection produces data, not a module
value (§9). `moduleof` is intended for the rare diagnostic or tooling case; it is wordy on
purpose, because it is almost never needed and does not warrant a scarce sigil.

---

## 8. Collisions are compile-time errors, resolved by aliasing

Because imports dump names, two imports can bring in the same name. A collision is a
**compile-time error**, never a silent shadow, and it is resolved by **aliasing**:

```luna
import { parse } from text.legacy;
import { parse as jsonParse } from json.parser;      // alias avoids the collision
```

The diagnostic names both source modules (provenance, §7), so the conflict is clear. Aliasing
(`as`) or collecting one side into a table (§6, so it is `json.parse` rather than bare `parse`)
resolves it. Since the whole import graph is static (§2, §4), every collision is detected at
compile time; none can arise at runtime.

---

## 9. Modules are not values or types

A module is a **compile-time construct**. It has no runtime representation and is not a value or
a type:

- You cannot assign a module, pass it, store it, or return it. (You can collect its *exports*
  into a table, §6, but that table is plain data, not the module.)
- A module has no type; there is no `module` type, and `moduleof` yields a `table`, not a
  module.
- After compilation, modules **disappear**: what remains is the compiled code, plus any tables
  you built from exports, plus strippable provenance metadata (§7). Nothing runtime-observable
  *is* a module.

This is the property that keeps the system simple: modules are purely a way to organize and
resolve source at compile time, and they leave no runtime entity behind to reason about.

### 9.1 App root and library root: same identity, different role

The root module has the same *identity* whether it belongs to an application or a library (path
`""`, §3). The only difference is its *role*, which is inherent to what applications and
libraries are and needs no mechanism:

- An **application root** is a **sink**: it imports other modules and is imported by none (it is
  where execution starts, via `main`).
- A **library root** is a **source**: it is what consumers import (`import { parse } from
  <the library>`), the entry point of consumption.

This role difference does not affect the root's identity, its path, or `moduleof`; it is simply
how the two are used. There is no app-versus-library asymmetry in the module system itself, only
in whether the root is a starting point for execution or for consumption.

### 9.2 Visibility is exactly two levels

A binding is either `export` (exported) or unmarked (module-private). There is **no third level**:
no package-private, no per-module selective exposure, no re-export. This is deliberate, the
public/private binary is the whole visibility model.

- **Re-exports are not provided.** A module exports only its own `export` declarations; it cannot
  re-export another module's names. To re-expose something, define it `export` in the module that
  should own it. (A facade `export import` form is additive if a real need appears.)
- **Selective cross-module exposure is not provided.** A binding is not "private except to
  module B." A need to share internals between modules almost always means the boundary is in the
  wrong place, the shared part should be its own module both import, or the two should be one
  module, the same remedy as for cycles (§2). When multi-file modules exist, files within one
  module directory share a scope and so already see each other's private names, which covers the
  in-module-sharing case without a separate mechanism.

Keeping visibility to two levels avoids turning each binding's visibility into a set of modules
and avoids a second coupling graph layered over the import DAG.

---

## 10. Builtins and the standard library

Two kinds of code are available beyond a project's own modules: **builtins** (the language's own
vocabulary) and the **standard library** (shipped modules). They differ in how they are reached,
and the difference is exactly language-versus-library.

- **Builtins are ambient in every module, always, and are not optional.** The primitive types
  and their full APIs, `string`, `int`, `double`, `table`, `regex`, `command`, `bytes`, `secret`,
  are available everywhere with no import; capabilities are not among them, they are ordinary
module exports (capabilities §4). Predeclared names occupy an **outermost universe scope**
and are **shadowable** by ordinary scoping (`let int = 5;` is legal and makes `x: int` an
error in that scope, self-inflicted and local; typeids interned at declaration are
unaffected), no special case, no reserved-word status (keywords §5). Builtin-ness
  follows the **type**: a builtin type's API is builtin regardless of how large it is, so the
  extensive `string` API is builtin because `string` is. There is no builtin-free build mode,
  builtins are what the language *is*, not a library that could be omitted, so "without builtins"
  is not a leaner build, it is a different language. (The standard library, by contrast, is
  opt-in per import.)

- **The standard library is just modules, under a reserved `std` virtual root.** `std` is a
  **virtual directory**, always available, that the compiler resolves to the shipped standard
  library; it is not a real directory in the project. It behaves like any path prefix in the
  ordinary module system, so **`std` is reserved**: a project cannot have a `std` directory at
  its root. (This is standard practice and near-zero cost, and it prevents the shared-namespace
  hazard where a user file could shadow a stdlib name.)

Because the standard library is ordinary modules, it needs **no special mechanism**:

- Importing from it is an ordinary import: `import { makeBuilder } from std.stringBuilder`. As
  with any import, this **dumps** names into scope (§5), so usage is bare (`makeBuilder(...)`);
  `std.` appears only in an **import path**, never at a call site. There is no `std.`-qualified
  access.
- Standard-library modules **use builtins ambiently** (like all modules) and **import each other**
  by ordinary path (`std.json` may `import ... from std.stringBuilder`). The `std.*` tree is a
  normal acyclic DAG (§2).
- What is a builtin versus a `std.*` library module follows the type rule above: a primitive type
  and its API are builtin (`string`, `regex`, `command`); a non-primitive helper or compound is a
  library module (`stringBuilder`, and the like).

Dependency flows **one direction**: user modules import `std.*`, `std.*` modules import only
builtins and other `std.*` modules, and builtins sit beneath both. The standard library **cannot
import user code**, it is shipped and compiled independently of any project, so user modules do
not exist from its point of view. This makes user-to-standard-library cycles structurally
impossible (not merely forbidden), so the layering user then `std` then builtins is automatic.

The detailed **organization** of the standard library, exactly which modules exist, how they are
grouped under `std`, and how they are versioned and distributed, is deferred (§11); it is
important to get right and is left to the standard library's own specification. What is settled
here is only the mechanism: builtins are ambient and non-optional, and the standard library is
ordinary modules under a reserved, one-directional `std` virtual root.

---

## 11. Deferred by decision

The module system's own design is complete. Its integration with an external package
ecosystem is **deferred as a unit** — and the deferrals are decisions with directions
fixed, not open questions (R151):

- **Packaging and distribution: source-based, ruled; the rest deferred.** Packages will
  distribute as **source trees**, not compiled artifacts — the direction is fixed now
  because so much of the corpus already leans on it: comptime evaluation folds imported
  **bodies and const values** into dependents (build-cache §1.2, R149), so a binary
  package would have to carry them anyway; artifacts are **per-compiler-version and
  per-target** (R149), so binary distribution would mean a version × target matrix
  where source is one tree; and the capability audit is a *source* audit — `use`
  clauses are read, not trusted (capabilities spec). What stays deferred: how a
  package's tree is mounted and rooted under a consumer (likely a project-marker root
  file rather than today's `main`/`--library` discovery, §3), and the full
  **organization of the standard library** under the reserved `std` root (§10) —
  deliberately, it is important to get right.
- **Dynamic loading: excluded, deferred — not open.** Loading a module chosen at
  runtime is deliberately excluded (§4); that exclusion is what makes the import graph
  fully static, which the DAG (§2), the interface-hash cache (build-cache §1), and the
  comptime sandbox all lean on. This is a standing decision, revisited only if a
  concrete need survives contact with those three dependents — none is on any horizon.

---

## 12. Error summary (R240)

Every module error, with the code that names it. Codes are `M` + four digits, allocated
append-only and never reused (compiler §3.1). Each has a fixed **title**; the description is
per-instance and volatile. Tests pin the code and the primary span, never the prose
(testing-strategy §2).

Discovery (compiler §1.0) raises none of these — it answers *which files* and nothing else
(R250). Every row below is import validation's (§1.2), except `M0005`, which travels as the
code on the error `Discover` returns: at that point there is no file to anchor a span to, which
is the shape `source.Error` already established for ingress.

| Code | Title | Raised when | Authority |
|-|-|-|-|
| `M0001` | Unresolved import | An import path names no file under the root; `std.*` is excluded, reaching no file by construction | §3, §10, R251 |
| `M0002` | Root import | An import resolves to the entry's file, which would give one file two identities | §3, R251 |
| `M0003` | Import cycle | Modules import one another; the description carries the full path, and every cycle is reported, not merely the first | §2, R251 |
| `M0004` | Import outside the prelude | An `import` after the prelude — late at top level, or inside a function, a block, or a conditional | §4, R250 |
| `M0005` | Missing entry | The entry names no file, so there is no root module and no compilation to begin | §3 |

**Deliberately uncoded.** A `std` directory at the project root is forbidden (§10), but nothing
looks for one: every `std.*` import resolves to the virtual root, so such a directory is merely
unreferenced, and detecting it would mean listing the root to catch a name no import can reach.
A malformed path handed to discovery is a caller bug and belongs to the `I` stage. An I/O
failure on a file that exists is environmental rather than a claim about the program, and stays
a plain error (R250).
