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

```
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

An `import` is a **compile-time, top-level** statement. It **must** appear at module top level,
never inside a function, a block, or a conditional. There are no exceptions.

This is not a restriction users miss, there is no real use for conditional loading, and it is
what makes the rest of the system work: because every import is unconditional and resolved at
compile time, the entire import graph (§2) is known statically, and every name's origin (§7) is
a fixed compile-time fact rather than something that depends on runtime control flow.

---

## 5. Importing brings public names into scope

`import` brings a module's `export` names into the current scope. It does **not** create a module
object or a namespace construct; Luna is data-focused, and where a namespace is wanted, an
ordinary **table** serves (§6). The forms:

```
import { parse, split } from text.strings          // bring in parse and split
import { parse as strParse } from text.strings      // bring in with a rename
import * from text.strings                          // bring in every export name
```

- **Selective** (`{ names }`): brings the named exports into scope. The common form.
- **Aliased** (`{ name as newName }`): renames on import, the tool for resolving collisions
  (§8).
- **Glob** (`* from`): brings **all** `export` names into scope. Available but used sparingly,
  since it couples the importer to the module's entire (and future) export surface.

Each of these is a **statement that dumps names**; none produces a value. A namespace, when
wanted, is a table (§6).

---

## 6. Collecting a module into a table

To get a namespaced handle instead of loose names, **assign a glob import**, which collects the
module's `export` exports into an ordinary **table**:

```
const strings = import * from text.strings;    // strings : table, holding all export exports
strings.parse(x);                                    // ordinary table access
```

- The glob import **in assignment position** yields a `table` whose fields are the module's
  `export` exports. `strings` is that table.
- Its type is `table`, annotatable (`const strings: table = import * from ...`) and inferable.
  This is the proof that **a module is not a value or a type**: what you get is a plain table,
  not a "module object." After compilation the module itself is gone (§9); only this table, if
  you built one, remains, as data.

So there are two ways to get namespaced access, and they are the author's or the importer's
choice: the **author** exports a single table (`export const strings = { parse: ..., ... }`, then
`import { strings } from ...`), or the **importer** collects loose exports with `const ns =
import * from ...`. Both yield `ns.parse`; both are just table access. No module-namespace
mechanism is needed because tables already are namespaces.

The distinction in one line: a **bare** `import ... from X` is a statement that dumps names; an
**assigned** `import * from X` is an expression that yields a table. Dumping creates no
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

```
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

```
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

## 11. Open questions

The module system's own design is complete; what remains open is its integration with an
external package ecosystem, deferred as a unit:

- **Packaging and distribution.** How separately-compiled module trees, both third-party
  libraries and the standard library (§10), are mounted under a compilation and rooted.
  Root-is-`main` (§3) is application-scoped: within one compilation there is a single empty-path
  root, but consuming a separately-built library means mounting its tree under some path in the
  consumer, which will likely want a project-marker root (a root file) rather than the
  `main`/`--library` discovery used today. The full **organization of the standard library** (its
  module set and grouping under the reserved `std` root, §10) is part of this and is deferred
  deliberately, it is important to get right.
- **Dynamic loading.** Loading a module chosen at runtime is deliberately excluded today (§4),
  which is what makes the import graph fully static. Whether a controlled dynamic-import facility
  is ever added, and how it would interact with the static DAG and with capabilities, is a later
  question.
