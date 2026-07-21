# Cross-reference consistency audit

Scope: every file under `specs/`, audited 2026-07-21 on branch `spec-refine-2` (HEAD `4796c4f`).
Method: twelve topic-scoped sweeps of the corpus (strings, numerics, streams, absence, errors,
functions/capabilities, tables/enums/constraints, type operations, concurrency, grammar/build,
std modules, orientation/examples), plus a mechanical checker that resolved all ~3,300
`<spec> §N.M` cross-references against actual headings, validated every `R<n>` citation against
CHANGES.md, and grepped for every retired spelling. Every finding below was verified against the
quoted lines; line numbers are as of HEAD above.

**What is clean.** No retired spelling survives anywhere: zero hits for `caps.*`, `use (&…)`,
`pub`, `fail`, prefix `!T`, `unsafe-ffi`, uppercase error names, or stray `docs/` paths; all
`u64` and `|>` mentions are deliberate "renamed R226" / "retired R146" annotations. Every cited
ruling number exists in CHANGES.md (the log itself never assigned R52–R58 and R77–R78 — a log
fact, not a defect). Roughly 3,000 of 3,300 section references resolve correctly.

**Where the drift is.** Four defect classes, in descending severity: (1) contradictions with no
governing ruling — both sides are "current" and a ruling is needed; (2) partial sweeps — a ruling
settled the question but listed sites were missed; (3) cross-references broken by renumbering
(concentrated on recently-refined files: protocols, channels, command, string, range); (4) the
orientation layer and worked examples predating many rulings.

---

## 1. Contradictions that need a ruling (no clear current side)

**1.1 String builder: value or single-owner?** `stringBuilder.md:36` — copying a builder
(`var b2 = b`) is an ordinary COW table copy yielding an independent accumulator; `:34` — `const`
binds and deep-freezes it (so does `variables.md:227`). Against: `concurrency.md:81-83` — builders
"cannot be copied and cannot be `const`", crossing by ownership transfer; `functions.md:136`
repeats it. R99 ruled the COW model; R117/R119/R121/R222 keep reaffirming the transferred/taken
class. The two models drive different answers for capture, spawn, and equality.

**1.2 Static enforcement of consumption.** `stream.md:476-478`, `iterable-functions.md:67-68`,
and `concurrency.md:248-249` all promise "a compile error where statically evident" for
use-after-taken / double-await; `compiler.md:236-238` (§1.4.1) rules the opposite — "No move,
linearity, or use-after-consume analysis… **runtime** properties… not through static
move-tracking", with static detection at most an optional lint. One side must give.

**1.3 The ternary operator does not exist — and is used.** `lexer.md:223` ("no ternary"),
`operators.md` §0 (no row), `associativity.md` (no tier), `control-flow.md:176-177` ("conditional
*values* are `match` and `??`") vs live uses `type.md:208` (`cond() ? 5 : 5.0`),
`secret.md:225`, `json.md:173-174` (`canReveal(s) ? reveal(s) : '<secret>'`), and
`optional-access-and-coalescing.md:151`, which even anchors the coalescers' precedence "just
above the ternary".

**1.4 Coalescer associativity and mixing.** `optional-access-and-coalescing.md:151` —
left-associative, unparenthesized `??`/`???` mixing is a parse error. `associativity.md:23`
(tier 10) — **right**-associative, the two "mix freely". Legal-program vs parse-error difference;
R27/R29 support the associativity table.

**1.5 Stream operands are taken vs the threaded-rng idiom.** `iterable-functions.md:66-68` (§1.5):
passing a stream to any catalogue function, "as the primary or as an operand", **takes** it.
`std/random.md:14-16` reuses `rng` after `shuffle(deck, rng)` and keeps calling `nextInt(rng, …)`
(`:78` — "`next*` consumes from the stream — visibly, since streams are by-reference"). Under
§1.5 the first `shuffle` takes `rng` and every later use panics `useAfterTaken`. R139 (rng
threading) and R92/R222 (taking) were never reconciled.

**1.6 Match scrutinizing a stream.** `match.md:577-578`: "scrutinizing a stream consumes it by
the ordinary single-pass rules". `destructuring.md:101-103`: "Match arms do not accept streams…
streams are identity values to `match`, never structure" (R103; `equality.md` §2 agrees). The
identity model exists precisely so match can never consume a stream.

**1.7 `int`↔`uint` crossing spelled two ways.** `numeric-tower.md:165` — `someInt.toUint()`
("convert with an explicit function"); `numeric-tower.md:214` and `as.md` §5 — `int as uint` is a
checked `as`. `conversion.md:22-24` rules "no conversion is ever spelled with `as`" and its
catalogue has no `toUint`; both spellings postdate R226.

**1.8 Sized-int subtype chain vs no-implication.** `numeric-tower.md:35` — "`i8 <: i16 <: i32 <:
int` by the constraint-subtype relation". `constraints.md:223-225` — constraints are **never**
subtypes of one another by predicate implication, and `int.md:153-156` declares `i8`/`i16`/`i32`
each flat on base `int` (siblings, not a chain). Either the declarations become nested-base or
the chain claim goes.

**1.9 `byteLength` / `count` over `bytes`.** `binary.md:62-64` makes "the O(1) `byteLength`
check" the caller's bounds guard over a `bytes`, but the only `byteLength` is
`string-api.md:56` (`byteLength(str)`), `bytes` is deliberately not `iterable` (so no `count`),
and bytes.md's catalogue has no length entry at all.

**1.10 One name, one signature — "satisfied in full" is false.** `functions.md:637-639` claims
the std surface fully satisfies the no-overloading rule, but always-in-scope name collisions
exist between the string API and the catalogues: `count` (`string-api.md:151` vs
`iterable-functions.md:136`), `find` (`:173` vs `:165`), `isEmpty` (`:128` vs `:128`), `slice`
(`:220` vs `indexable-functions.md:72` vs `bytes.md:92`), `reverse` (`:295` vs `:152`),
`replace` (`:286` vs `iterable-functions.md:359`), `cString` (`:399` vs `bytes.md:94`). Also two
std `now`s: `time.md:118` (`now(): instant`) vs `datetime.md:94` (`now(zone): @datetime`) —
R188 applied §3.4 *across* modules when it renamed command's `args` to `argsOf`.

**1.11 `unsafeSystem` vs `unsafeShellExec`.** `capabilities.md:519` assigns shell-string
execution to the `unsafeSystem` capability; `exec.md:182` (and command.md) name the operation's
capability `unsafeShellExec`. Both deferred, but the inventory and the module disagree on the
name. Relatedly `functions.md:934` cites a capability `unsafeReveal` that nothing defines —
revelation is the ordinary `reveal*` family (`revealSecret`, R219), not `unsafe*`.

**1.12 If-as-expression.** `never.md:118` types `if (c) { x } else { die("no x") }` as "the type
of `x`"; `control-flow.md:176-177` rules conditionals are statements and conditional values are
`match`/`??`.

**1.13 Enum variant selection outside braces.** `enum.md:239-241` fences `Name.variant` **inside
construction braces** ("outside braces `.` is field access"), yet `enum.md:344-345` uses bare
`@x == shape.circle` / `x is shape.circle` in expression position, and `introspection.md:184`
uses `@shape.circle` — which also collides with the value-position `@` rule
(`overview/types.md:137`: `@` of a type-valued binding is `type`, so `baseOf(@shape.circle)`
would be `null`, not `shape`; the working spelling is `baseOf(shape.circle)`).

**1.14 `platform.arch` vocabulary.** `platform.md:13` documents `'x86-64'`; `platform.md:40`
rules the values are Go's GOOS/GOARCH vocabulary — GOARCH spells it `amd64`.

**1.15 CLI flags used but not in the ruled flag table.** `modules.md:90-91` requires
`luna --library` (library-root discovery) and `incremental-compilation-build-cache.md:166`
requires `--clean` (manual full rebuild); `compiler.md:36-47`'s "Known flags" table (R41)
carries neither. Either the table grows or the features lose their flags.

**1.16 Compile-target set.** `compiler.md:27` — "targets **`linux-x86-64` only, for now**"
(echoed by `io-errors.md:4` and build-cache §3) vs `compiler.md:356-357` — "Targets are 64-bit
only: `amd64` and `arm64` at alpha (R193)". R193 swept §1.8/§6.3 but not §0 or the echoes; if
§0's "for now" means pre-alpha sequencing it should say so against the ruled alpha set.

---

## 2. Partial sweeps — a ruling settled it; these sites were missed

### 2.1 The `toJson` generator split (R157/R179: `toJson` takes a `comptype`; the value writer is `toJsonDynamic`)

- `std/json.md:173-174` — its own example calls `toJson(cfg, …)` on a value **and** delegates
  `use (…)` onto a comptime call (a compile error, R114, `capabilities.md:333`).
- `declarations/protocols.md:471` — `toJson(v, skipFunctions: true)`.
- `types/tables.md:224` — `tab[t.toJson()]` composite-key idiom.
- `std/datetime.md:198` — `toJson(['created' => …])`.
- `std/time.md:47`, `types/complex.md:123` — value-level `toJson` phrasing.
- `declarations/serialization.md:42-43` — restates both writers **without** the R96/R113/R125
  flag parameters; the whole file is a diverged duplicate of json §2 (R24 recorded it dissolved)
  and is also the one spec file missing from index.md's table.
- `declarations/attributes.md:212` — cites "json spec §3" for `toJsonDynamic`; json §3 is Reading.

### 2.2 R231 (undefined refine) — swept only undefined.md

- `types/any.md:19` — "`@v`: … always answerable" (an `any` may hold `undefined`; `@` on it panics).
- `bindings/variables.md:387-392` — `@` of a missing-key read returns type `undefined`, with a
  worked `typeName` example; R231 makes that `@` a use → panic.
- `declarations/errors.md:550-551` — "*using* a void function's result … panics"; statically
  known undefined is a **compile error** (`undefined.md:60-61`, `compiler.md:231-232`);
  `undefined.md:186-188` itself still carries the old phrasing once.
- `overview/high-level-overview.md:52-53` — "`undefined` … unstorable" (it is storable in
  bindings; unstorable only in tables — overview/types.md states it correctly).
- `bindings/optional-access-and-coalescing.md:10` — the literal "two productions" count R231
  reframed as two production *classes*.
- `types/string-api.md:448` — "`undefined` is unspellable in a return type" vs written
  `: undefined` / `undefined!` signatures in `std/io.md:146-150`, `std/time.md:143`,
  `std/filesystem.md:106-109`, `std/net.md:76`, `concurrency/channels.md:92,118`,
  `declarations/tests.md:30,75`, `std/io-errors.md:73`. One convention must win.

### 2.3 R222/R221/R223 (stream enforcement, `gen`, `yield from`) — lexer and bindings missed

- `bindings/variables.md:325` — consumed stream foreach shown yielding "nothing" (silent empty
  pass); R222 makes it a `useAfterConsumed` panic — and the same example (`:322-323`) applies
  `&` to a `let`, which `variables.md:293` itself rules a compile error.
- `build/lexer.md:82,119-120` — `from` still reserved (`KW_FROM`, "gap G1, now resolved");
  R223 unreserved it (`keywords.md:36`, `stream.md:232-234`) and made `yield from` one compound
  token the lexer never defines.
- `build/lexer.md` keyword table — no `gen` row, though R221 reserved it (`keywords.md:15`).
- `build/lexer.md:146,238,317-318` — command literals "have **no escape sequences** by ruling
  (command §2.2, resolving G5)"; R150 gave commands three escapes (`` \` ``, `\\`, `\$` —
  `command.md:59`, string §5.1), the cited command §2.2 no longer exists, and the `COMMAND` span
  regex cannot skip an escaped backtick.
- `keywords.md:58` and `match.md:570-571` — the `use`-position inventory says two positions
  (fn/test header, call-site delegation); R221 added the `gen use (…)` head as a third.

### 2.4 R142/R117/R119 (promises, channels, timeouts) — concurrency neighbors missed

- `concurrency/concurrency.md:611` — "**Channels: deferred**, not necessary" vs `:581` of the
  same file: "**Channels: designed, R119**".
- `concurrency/concurrency.md:462-463` — timeouts/`awaitAny` "stay deferred" vs `:573` of the
  same file: "**Timeouts and `awaitAny`: landed, R142**"; `std/time.md:178-180` keeps the
  same pre-landing framing ("now unblocked", net "behind it").
- `concurrency/await.md:26-27` — a second `await` "**panics**"; R142: compile error where
  statically evident, `doubleAwait` panic only on the dynamic path (`concurrency.md:248-250`).
- `bindings/variables.md:273-274` — "The sole exception is `stream`" (by-reference class);
  R142 added `promise` to that class (`concurrency.md:245-246`).
- `declarations/tests.md:54` — the runner pushes promises into a table and iterates it later;
  R117 bans promises in retained storage (`concurrency.md:221-223`). (Also `->push` — see 2.7.)
- `concurrency/concurrency.md:244` — "the `argv` precedent, capabilities §1 — … not a
  capability"; R43 reversed argv to a capability, so the cited precedent asserts the opposite.
- `types/stream-api.md:124-125` — spawn-in-map example without the `toStream` R146 made
  load-bearing (`concurrency.md:322-325`: a table primary would yield the banned table of
  promises).
- `index.md:176` — Await row still says "cancellation deferred" (R115 specified it; R142 closed
  the surface deliberately).

### 2.5 R43/R39/R33/R213 (capability model) — capabilities.md and compiler.md carry dead models

- `capabilities.md:80-82` and `:381-382` — "argv … **not** a capability" vs `:522` ("argv **is**
  a capability, ruled R43") in the same file; `std/process.md:21` agrees with R43.
- `capabilities.md:93-96` — capability-holding function values "confined … cannot be bound to a
  new name … the rename is a compile error" vs `:146` (`let f = println; f("hi")` legal) and
  R39's first-class rule the same paragraph half-acknowledges.
- `capabilities.md:200-201` — `use` is "the same `use` that captures any nocopy value" vs
  `functions.md:116` — "`use` has exactly one meaning: … a **capability**".
- `capabilities.md:525-526` — "explicit unless deliberately `implicit`" vs `:43-44` — the
  `implicit` modifier removed (R33); `:521` even notes the tier column is gone for that reason.
- `capabilities.md:77-78` — "comptime-excluded (comptime forbids capturing nocopy values, §8)"
  stated categorically vs `:470-471` — comptime may hold `comptime` capabilities.
- `capabilities.md:507-519` — the §9 inventory omits `entropy` (R139, `random.md:37`) and
  `revealStackTrace` (R113, `errors.md:127-128`).
- `capabilities.md:26,401` — composed-set examples grant `net` and `fs`, capabilities that do
  not exist (`egress`/`ingress` per R143; `filesystem` per R134/R135).
- `functions.md:214` (and `:228,754`, `capabilities.md:549`) — eligible iff "requirement set is
  **empty**" vs `functions.md:712-713` (R213) — eligible iff every capability in the set is
  `comptime`; `internal-representation-of-functions.md` §2 records the mask form and calls the
  empty-set form the earlier mistake.
- `build/compiler.md:257-258` — eligibility/capabilities are "call-graph fixpoints" vs
  `functions.md:739-740` (§5.1 "no fixpoint exists, R213") — locality is the ruled model.
- `declarations/tests.md:75` — `'requirements' => [io]` stores a capability token in a table,
  which `capabilities.md:113-114` forbids; the inert form is capability *types* (R130).

### 2.6 R45/R213 (fn literal grammar: mandatory `=>`)

Arrowless `fn` literals survive in `capabilities.md:229-233,379`, `std/io.md:10`, and
`stream.md:31,39,204-205` — stream.md §1.4 even calls `fn (): stream { … }` "the generator's own
canonical spelling" while `functions.md:177-178` (R45) makes the `=>` mandatory (range.md's
desugar at `:139` writes the arrow). Needs either a sweep or a carve-out ruling.

### 2.7 R91/R92/R102/R106/R107 (catalogue respellings) — scattered remnants

- `expressions/spread.md:203` — `join(', ', xs)` glue-first; ruled signature is iterable-first
  (`iterable-functions.md:430`).
- `types/functions.md:104` — `fold(xs, 0, fn …)` as the accumulator; `fold` is the string
  case-folder (`string-api.md:278`) and the accumulator is `reduce` (whose argument order the
  example also fails to match).
- `types/regex.md:290` — `findAll` "as a stream or table"; ruled stream-only
  (`string-api.md:174`).
- `expressions/associativity.md:90` — "`reverse(r)` for an existing stream"; `reverse` is
  table-only (`indexable-functions.md:152`); the ruled spelling is `r.collect().reverse()`.
- `types/bytes.md:180-181` — `[0x89, …] as bytes`; a repack is `toBytes` (R107;
  `conversion.md:22-24` — "no conversion is ever spelled with `as`").
- `types/bytes.md:176` — "build a `string` and take its `bytes()`"; that direction is
  `toBytes()` since R102.
- `internal-representation-of-strings.md:205,119` — `bytes()` elements described as borrowed
  views / inline strings; R102 typed them ints 0..255 (`string-api.md:350`).
- `stringBuilder.md:77`, `protocols.md:612`, `equality.md:209` — `var buf: bytes = bytes();`
  — a nullary `bytes()` no spec defines (empty bytes is `[]` or `b""`); `equality.md:210` also
  narrows `append` to `s: string` vs the ruled `value: any` (`stringBuilder.md:79`).
- `expressions/control-flow.md:98` — `while (…) { handle(queue.pop()); }`: removers return the
  shortened table (`tables.md:330-332`), so this neither mutates `queue` (infinite loop) nor
  passes an element; needs `handle(queue.last()); &queue.pop();`.
- Functions used but defined nowhere: `push` (`functions.md:107`, `tests.md:54`,
  `attributes.md:180` — the latter two via `->push`, which is protocol space and doubly wrong
  for a catalogue verb), `reset` (`variables.md:374`; the stream op is `restart`),
  `makeBuilder` (`modules.md:381-382`; the constructor is `builder()`).

### 2.8 Error-model remnants

- `expressions/range.md:141` — the §4a desugar (`throw typeError('zero step')`) constructs and
  originates a `panic` type in "no magic" user-level Luna; `errors.md:255-257` makes both
  impossible (panics unconstructable; originating throw requires `fn!`).
- `expressions/numeric-operators.md:101` — overflow "still catchable with `try`";
  `int.md:64-66`/errors §8.1: the `try` **expression** never catches a panic (block form only).
  `build/compiler.md:723-724` has the same unqualified "try recovers it" phrasing.
- `types/double.md:209-210` — policy verbs fail with "a declarable error … or a panic";
  `conversion.md:63-64` (R106) puts all failures on the `int!` channel.
- `types/stream.md:81` — `catch (e) => { yield fallback; }`; the ruled catch grammar takes a
  parenthesized binder then a block, no arrow (`match.md:141-142`, errors §8.3).
- `std/exec.md:87` — "`try run(…)` — stdout, or the thrown commandError **propagates**": `try`
  recovers to a value, never propagates; `exec.md:60-62,121-123` then use the try-bound union
  unnarrowed (the exact R49 bug class), as does `tests.md:17` (`var fd = try openFile(…)` then
  `close(fd)`).
- `errors.md:477-478` — "the same `self!` a protocol's `apply` carries … (protocols §7.5)":
  no §7.5 exists; operator apply is never errorable and dynamic `apply()` is `table!`
  (`functions.md:690-692`, protocols §4).
- `std/io.md:225-226` — "Malformed content surfaces as the parser's declarable error" vs
  `json.md:189-191`: well-formedness fails at the `as json` entry (a `typeError` panic); the `!`
  arm is semantic-only. `index.md:29`'s comment ("bad JSON propagates too") repeats the wrong
  channel.

### 2.9 Tables/constraints remnants

- `constraints.md:407` — "a **shape** constraint like `list` never panics through a wider path,
  it retags" vs `:325` — the retag-on-write "shape class" was "considered and rejected"
  (R11: breaking writes panic through every path).
- `enum.md:115` — recursion terminates "because … tables are reference values"; tables are COW
  **value** types (`tables.md:299`, R200).
- `indexable-functions.md:118` — `unset` "O(n) if a list must reindex" vs `tables.md:124` —
  "Removal does not silently reindex" (R196 tombstones).
- `tables.md:688` — the const-table perfect hash "also fixes an iteration order" vs
  `internal-representation-of-tables.md:395` — insertion order is spec; the hash never reorders.
- `tables.md:199-200` — "there is no positional order" for non-list tables vs
  `equality.md:154-155` — "Tables are ordered values"; note both equality.md and the internals
  delegate the insertion-order guarantee to "the tables spec", which never states it.
- `tables.md:169-170` — order-preserving producers "typed to return `list`" vs the catalogue's
  `values(it: iterable): iterable` + kind-following rule (`iterable-functions.md:250,41`).
- `undefined.md:241-242` — "a table's storage never needs to represent it" vs the tombstone
  representation that stores an `isUndefined`-flagged entry
  (`internal-representation-of-tables.md:84`) — wording, but worth aligning.
- `internal-representation-of-variables.md:480-481` and
  `internal-representation-of-tables.md:348-350` — spawn "deep-copies … captures" vs
  `concurrency.md:47-49` — captures are frozen at creation and cross shared-by-reference.
- `bindings/destructuring.md:237,247` — protocol-typed sources typed "by the protocol's
  declared, enforced element members (protocols §5.4)": protocols retired element members
  entirely (R95; `protocols.md:49-50,498`) and have no §5.4.
- `bindings/variables.md` retired-model cluster: `:145` ("the table protocol, whose method
  operations are pure and write back through a reference" — R91 retired it), `:227-229`
  ("meta members", "a mutating meta call (one taking `&self`)" — no protocol function takes
  `&self`, `protocols.md:171-172`), `:408` ("a table-protocol violation at runtime (tables §5)"
  — contradicts its own §3 `typeError` panic), `:66-71` (`File|stream`, `File.modeRead`,
  `"File"` typeName — current form is lowercase `file`, `path`-typed argument, `{read}` modes).

### 2.10 Operator/keyword/token inventory drift

- `build/lexer.md:79` — `meta` still tokenized (`KW_META`); R96 retired the keyword
  (`protocols.md:51-52` — "`meta` is no longer a keyword"; keywords.md has no row).
- `moduleof` — defined as a unary compile-time prefix operator (`modules.md:244`, live per
  R171) but absent from all four inventories that claim completeness: `operators.md` §0
  ("every operator in Luna"), `associativity.md`'s tiers, `keywords.md`, and `lexer.md`'s
  token table (its sibling `declared` appears in all four).
- `..<` — established by R47 (`range.md` §1.1, `associativity.md:18`, `lexer.md:181`) but
  missing from `operators.md`'s catalogue rows for ranges (`:62-63`).
- Prefix `&x` — an expression operator per `operators.md:53`, but `associativity.md:15`'s
  symbolic-prefix tier (`!x -x @x @@x`) omits it, leaving it without a precedence row.
- `build/lexer.md:122-123` vs `:253` — the same file lexes `_` two ways: "not keywords…
  they lex as `IDENT`" vs the dedicated `WILDCARD` token row.
- `build/compiler.md:889-895` — §11's "Open" list still names block-scoped defer lowering
  (ruled, R148 — `:732`), green-thread mapping "pending the concurrency model" (complete —
  `:719-720`, R115–R119/R142), and static-unboxing boundaries (ruled, R203 per §7.1.1).

### 2.11 Misc std remnants

- `std/filesystem.md:53-54` — `extension(p) ?? ''` can never fire: `extension` returns a `null`
  miss (`:46`) and `??` is absent-only (`optional-access-and-coalescing.md:37`); the idiom
  needs `???`.
- `std/filesystem.md:135-136` — claims `alreadyExists` is filesystem's addition; it is in
  openFile's original errno family (`io.md:117-118`, `io-errors.md:31`).
- `std/net.md:52` — `chunks(conn)` omits the required `size` parameter
  (`io.md:165` — no default).
- `std/introspection.md:496` — calls `fn (...args: any): any` "the `println` shape"; println is
  a typed three-parameter signature (`io.md:146-147`).
- `std/introspection.md:83,522` — imports `from introspection`, not `std.introspection`.
- `equality.md:112` — cites "constraints §10" as the home of `port` and `i8…`; §10 holds only
  `byte`/`nat`/`list` (`port` is std.net's, sized ints are int §6.1).

---

## 3. Broken or stale section references (mechanical)

Confirmed hard breaks (target section does not exist; the content moved):

| Site | Says | Reality |
|-|-|-|
| `bindings/destructuring.md:237,247` | protocols §5.4 | protocols §5 has no subsections; concept retired (R95) |
| `declarations/errors.md:478` | protocols §7.5 | no §7.5; see functions §4 / protocols §4.4 |
| `build/lexer.md:146,238,318` | command §2.2 | command §2 has no subsections; superseded by R150 anyway |
| `build/lexer.md:291` | Strings §13 | string.md ends at §5.1 post-split (content now string §5) |
| `build/lexical-structure.md:56` | range §10 | range.md ends at §8 |
| `concurrency/concurrency.md:409` | channels §2.1 | channels.md is flat §1–§7; sink/MPSC content is §3 |
| `declarations/capabilities.md:327` | (own) §5.5 | capabilities.md has no §5.5 (functions §5.5 is the live half) |
| `keywords.md:42`, `operators.md:69` | throw → errors §4 | errors §4 is Inheritance; Throwing is §6 |
| `declarations/tests.md:33` | errors §4 | originating-throw rule is errors §6/§7 |
| `examples/one-billion-rows.md:52` | errors §5 | errors §5 is Construction; errorable main is §8.2 |
| `operators.md:70`, `keywords.md:122-123` | generator classification → stream §2 | that is §1; §2 is single-pass consumption |
| `declarations/attributes.md:212` | toJsonDynamic → json §3 | json §3 is Reading; writers are §2 |
| `equality.md:112` | port/i8 → constraints §10 | §10 holds byte/nat/list only |
| `build/lexer.md:225` | slice bounds → tables §3 | tables §3 is Keys and access; slicing is §2.5 |
| `build/lexer.md:345-347` | G1: `from` "now listed there (§1)" in keywords.md | keywords.md §1 has no `from` row (R223 unreserved it) |

Bare same-file references whose real target is another spec (the name was elided; the section
does not exist in the citing file): `introspection.md:379` ("§2.2's theorem" → protocols §2.2),
`await.md:99` ("the §5 composition" → concurrency §5), `serialization.md:85` ("(§2:" → json §2),
`shape-type.md:66` ("§7 entry … §9.2 value-carried" → constraints §7/§9.2),
`internal-representation-of-streams.md:141` ("the §9.5 shape" → constraints §9.5).

Notation note: several files cite "§0" for another spec's unnumbered preamble
(`introspection §0`, `json §0`, `constraints §0`); only compiler.md and operators.md actually
number a §0. Harmless but inconsistent.

---

## 4. Orientation layer and examples

**index.md**
- `:26,29` — front-page example: `as path` used without importing std.filesystem (R135 moved
  `path` there; same gap in log-scan, one-billion-rows, testing examples), and the "bad JSON
  propagates" comment routes well-formedness through the wrong channel (see 2.8).
- `:89-91` — `-d` glossed `--debug` "builds in debug mode"; the ruled flag is `-d/--debugger`,
  run under the debugger (`compiler.md:46`, R41); a debug-info build is `-d -c`.
- `:176` — Await row: "cancellation deferred" (superseded, R115/R142).
- `:177` — Associativity row: "parser-blocking questions" (its §4: "nothing open", R158).
- `:199` — Range row: "`lo..hi` … a slice bound" — `..` is never a slice bound (R166;
  `range.md:14-16,193`).
- Missing rows: `declarations/serialization.md` (see 2.1) and `retired/json-duplicate.md` in
  the retired table.

**overview/high-level-overview.md**
- `:24-25` — claims the front-page example shows "`in`-bound iteration"; it contains none.
- `:52-53` — "`undefined` … unstorable" (see 2.2) and the numeric set "deferred past alpha"
  with no R187/R226 carve-out (`uint`, 16/32 constraints, `nat` are alpha).
- `:68-70` — lists `attribute` under "Declaration forms introduce user-defined types";
  attributes are explicitly not types (`attributes.md:232`).

**overview/types.md**
- `:31-33` vs `:169-172` — internal contradiction on the same R187 carve-out.
- `:83` — "`@` yields the enum type"; enum §6 rules `@x` is the **variant**, never widened.
- `:20` — secret row omits the `table` payload (R111; `secret.md:9-10`).

**examples/**
- `log-scan.md:15` — opens `args()[0]` (the program name, per `process.md:20-21`) as the log.
- `one-billion-rows.md:14-16` — `count() != 1` and `arguments[0]` are off by one under the same
  convention (index.md's example does it right); `:22-34` — arithmetic, ordering, `.min`
  member access, and typed UFCS on un-narrowed `any` elements throughout, which any §2 (F22)
  makes compile errors (log-scan narrows with `as string` and cites the rule).

**keywords.md**
- `:4-5` — "§6 is the flag list of words whose definitions need work" — all seven flags are
  ruled (`:106`, R33); index row `:172` echoes the stale framing.
- `:126-127` — "`by` is reserved **now** for future stepped-range syntax" — `by` landed (R47,
  range §3); the file's own §2 row says so.

---

## 5. Minor / informational

- CHANGES.md ruling sequence gaps: R52–R58, R77–R78 were never assigned (log fact only).
- `stringBuilder.md:283-285` still calls `bytes` "pending its own design"; bytes.md is fully
  specced.
- `tables.md:736-737` (§A.5) mentions "a table frozen at *runtime* (built mutably, then
  sealed)"; sealing was removed (R109).
- `operators.md:65` — `where` guards "a match arm / comprehension"; no comprehension construct
  exists (keywords.md gives `where` two homes: match arms and constraints).
- `regex.md:248` — `/\d+/.source` accessor is defined nowhere (regex is opaque).
- `type.md:292-293` and `incremental-compilation-build-cache.md:74-75` — both say compile-time
  `match` exhaustiveness consumes `variants(t)` / enum variant sets; match §9.1 rules the only
  exhaustiveness question is `_`-presence (R182), so the claimed dependency does not exist.
- Error declarations are written binder-less (`myError = error { … };` — `errors.md:189`,
  `io-errors.md:16`) while the sibling declaration forms were each ruled const-only
  (`const p = proto` R126, `const b = constraint` R137) and the variables ladder defines no
  binder-less declaration statement; no spec states which binder `error` takes.
- `build/modules.md:146-148` — three import examples lack the terminating `;` that
  lexical-structure §1 and modules.md's own §5 grid require.
