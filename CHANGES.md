# Changes: capability model, capture model, bare `fn`, and consistency sweep

Four rulings, applied across the corpus. Every edit below traces to one of them.

**R1 — Capabilities live on the value; capability-holding function values are
second-class.** A function value's capability set is fixed at its creation site
(literal's `use` clause ∪ callees' requirements), checked there against the enclosing
scope, and the value can then never be aliased, passed, stored, or returned, only
called directly by its declared name or spawned. All violations are compile errors at
the offending assignment or call (not panics: a capful value never reaches a slot, so
no dynamic case exists). Consequence: every function value in any function-typed slot
is capability-free by construction; indirect calls need no capability reasoning;
capability inference stays one-pass and exact. Principle recorded in functions §3/§7
and capabilities §3.1: an **error is data** (rides the return value, so the type must
carry it or it smuggles); a **capability is authority** (rides nothing, so it is
confined at the value instead of described in the type).

**R2 — Bare `fn` binds any *non-errorable* function; `fn!` is the errorable
wildcard.** Signature erasure no longer erases errorability. Ladder:
`fn (A): R <: fn <: fn!` and `fn (A): !R <: fn!`; `fn (A): !R` into `fn` is never
legal. Reasoning: no laundering errors, a per-call check can restore signature safety
but not a caller's handling obligation.

**R3 — Capture is an implicit deep-`const` binding of the current value.** COW stays
COW (snapshot logically copies, physically shares until the *original* writes), const
stays const (deep, read-only, `const` bindings captured share with no copy). No
reference capture of ordinary bindings exists; `use` names capabilities and nothing
else. Mutation crosses into a function only as a `&` parameter at call time. Stated
non-goals: stateful closures (`once`, `memoize`, counters); the accumulate-into-outer-
binding idiom (use fold shapes, `&` parameters, or `foreach` statements).

**R4 — Streams: pass, or the capture moves them.** A directly captured stream binding
is a compile error (pass it as a parameter). A stream reachable inside a captured
table is handled exactly like the spawn boundary (concurrency §2.1/§2.3): copyable
parts snapshot, the stream referent transfers and is marked moved-from; later access
through any outer alias panics.

---

## File-by-file

**bindings/variables.md** — replaced the mis-saved copy of functions.md with the real
variables spec, then updated three passages to R3: §3.1 (a closure has no mutable
contents at all), §5.1 scalar references (the escaping-`use`-capture boxing case no
longer exists; scalar references never allocate), §5.1 spawn boundary (`&` arguments
are the only forbidden crossing; any closure is spawnable).

**types/functions.md** — §2 rewritten (R3, R4): §2.1 capture = implicit deep-`const`
bind, with the one-law framing (call, closure, spawn cross identically), the
compile-error examples, and the non-goals stated as decisions; §2.2 `use` = capability
channel only, `&` parameters as the mutation manifest; §2.3 the stream rule. §3 gained
the capabilities-are-not-in-the-type paragraph and the errors-vs-authority principle
(R1). §3.1 rewritten for R2 (`fn` non-errorable, `fn!` wildcard, subtype ladder,
no-laundering rationale; capful values need no wildcard, they fit no slot). §5
rationale line updated (`use` is the sole referential capture); §5.6 and §6 wording
aligned; §7 gained the closing principle paragraph; §8 first resolved note re-resolved
(confinement makes type-surfacing vacant); §9 simplified (binding-versus-slot
dissolves for every binding mode). `use (unsafe-ffi)` → `use (caps.unsafe-ffi)`.

**declarations/capabilities.md** — §3 aliasing sentence rewritten (the alias is
impossible, not merely ineffective). §3.1 extended: the never-enters-a-value-slot
doctrine now covers capability-holding function values (the second-class rule, R1),
with the declaration-binding-versus-alias distinction, the `foreach`-statement idiom
for effectful iteration, and the errors asymmetry. New §5.1: the creation-site check,
and why indirect calls contribute the empty requirement exactly. §8: the sandbox also
holds by existence (no non-comptime instance exists before runtime). §10: the
capability-set-polymorphism question resolved by §3.1 (nothing is forwarded); new open
question, comptime-eligibility may leave the typeid once the everything-outside-is-a-
capability audit closes. `use (reveal)` → `use (caps.reveal)`.

**concurrency/concurrency.md** — §2 captures bullet and §2.1 references bullet
rewritten (R3): closure environments are frozen at creation and cross as `const` data;
the `use`-captured-`var` spawn error is deleted because the capture it forbade no
longer exists; every closure is spawnable; `&` arguments remain the only forbidden
crossing; `spawn` noted as a direct-call form, not a value slot (R1).

**type-operations/match.md** — §11 rewritten: match captures `const` snapshots; `use`
on a match names capabilities only; example updated; stream note added (R3, R4).

**types/strings.md** — binding doctrine aligned with variables §1.1: `let` never
rebinds (the "new value, not a mutation" reading removed), `var s: string` is legal
and is how a string binding is reassigned, `let`/`const` coincide for strings
(variables §3.1); `.=` requires `var`. `&` on a `var` string follows the general rule
and can only rebind the slot.

**build/compiler.md, concurrency/exec.md, types/functions.md (§5.6)** — the three
stale "comptime forbids `use`" statements updated to the current rule: comptime
forbids using any **non-comptime** capability (functions §5.5), now also vacuously
true before runtime (capabilities §8). `use (exec)` → `use (caps.exec)`.

**overview/high-level-overview.md, types/never.md, types/undefined.md** — `use (&io)`
→ `use (caps.io)` (canonical spelling per capabilities §4/§5; `&` inside `use` was
always wrong and is now actively misleading, since `use` performs no `var` capture).

---

# Changes, round 2: the `try`/`!` error model (F2)

**R5 — The `try` expression catches the `UserError` subtree only; `try` is not
total.** Panics unwind through a `try` expression to a `try`/`catch` **block** (which
catches everything) or to the task/program boundary. errors.md §8.1 was already the
correct text; the two opposing claims are fixed: value-representation §3.1 no longer
says "`try` is total" (it converts every *UserError* throw; a Panic unwinds through,
and the arm is therefore exactly base `UserError`, neither a subtype nor the root),
and int.md §3 is rewritten, `let sum! = try bigA + bigB` was never an overflow check
under this model, since `OverflowError` is a `Panic`; anticipated overflow is handled
at a `try`/`catch` block (`catch OverflowError e`), with the success path inside the
block (this also removes §3's reliance on flow-narrowing prose). The no-`checkedAdd`
doctrine survives with the block as its mechanism; intended-alternative arithmetic
stays with the wrapping/saturating operations (§4).

**R5a — forced consequence: `!` desugars to `| UserError`, and the throwaway error is
base `UserError`, not base `error`.** With `try` catching only the `UserError`
subtree, a root-typed throwaway (`throw error('msg')`, old §5.2) would have been an
*expected* failure that `!` does not declare and `try` does not catch, and errors.md
§2/§7 disagreed with §8.1 about the `!` arm (`| error` vs `| UserError`). Resolution,
taking §8.1's side throughout: the root `error` is **abstract** (never constructable;
every concrete error value is a `Panic` or a `UserError`, making the two subtree
policies total); the throwaway is **base `UserError`** (`throw UserError('disk
full')`, bare `throw UserError`, `UserError('msg', dataTable)`); `T!` is
`T | UserError` (`string?!` is `string | null | UserError`). Root `error` remains the
widest catch, the widest declared arm, and the static type of a block-caught error;
it is just not what `!` means and not a value's exact type. Swept: errors.md (§2
diagram and abstract-root note, §2.1, §5.2, §7, §8.1 example, §10 ledger),
value-representation §3/§3.1, int.md §2–§3, strings.md (`toInt`/`toDouble` arms),
functions.md §3.1 ladder, operators.md `T!` row, never.md and exec.md `throw error`
sites. If the short throwaway spelling matters more than the subtree-total design,
the alternative is to keep `throw error(...)` and redefine the policies as
"everything outside the Panic subtree," at the cost of an imprecise root-typed `try`
arm; the spec now takes the precise side.

**R6 — the hierarchy is flattened: `UserError` is removed** (supersedes R5a's
throwaway/desugar direction; R5, `try` is not total, stands unchanged). New hierarchy:
`error` (root, constructable, the throwaway) with one distinguished subtree, `Panic`
(sealed, ambient, undeclarable); user errors extend the root directly (the new default
parent, errors §4). Every policy that named the `UserError` subtree now names the
complement of `Panic`, defined once in errors §2 as the **declarable error** category
(a category, not a type): `!` is required iff the body can raise a declarable error;
the `try` expression catches declarable errors only (one negated interval test);
inheritance is open everywhere outside the sealed subtree. This restores
`throw error('disk full')` and the `!` desugar `T!` = `T | error` (which is what §2's
diagram and §7 originally said, §8.1's arm-precision paragraph was the one text that
needed the node).

Accepted costs, recorded in the spec: (1) the `!`/`try` arm is spelled root `error`,
which statically includes `Panic` even though a panic can never dynamically land there
via `try`, so a manually stored block-caught panic can legally sit in an `error`-typed
slot and `is Panic` on a `try` arm is legal-but-always-false (errors §7); (2) there is
no one-word block catch for "declarable errors only", a boundary that handles expected
errors while letting panics unwind spells it by ordering, `catch panic p { throw p; }`
then bare `catch e` (errors §8.3). Files swept (33 sites beyond errors.md): overview
types + high-level, strings, double, bytes, never, functions, bool, int, operators,
exec, concurrency, conversion, as, value-representation. errors.md §10's
root-construction question is re-resolved for the root; base-`Panic`/subtype user
constructability remains open.

**R7 — `Panic` is sealed in both directions: no user inheritance (already so) and no
user origination.** No `Panic` type is constructable in user code (errors §5, §9); the
runtime mints every panic at its own failing operation, so `Panic` means exactly "a
language or runtime error" and cannot be faked. Consequences: every *originating*
`throw` requires `fn!` with no per-throw category test (errors §6, §7; functions §4),
since everything user code can construct is declarable; the sole `Panic`-typed act in
user code is **re-throwing** a caught one (`catch panic p { throw p; }`), which is
propagation, appends a breadcrumb, and does not require `fn!`; a root-`error`-typed
`throw` is conservatively treated as origination (requires `fn!`) even if dynamically a
panic. errors §10's constructing-the-roots question is now fully resolved.

**R8 — `==` erases constraints (F3).** The typeid-first rejection rule made every
constrained value unequal to its base (`someByte == 65` always false, breaking bytes
§6's own filter example and any list-vs-table content comparison). New algorithm
(equality spec, intro and §1): flags; same typeid → fast path unchanged; different
typeids → compare the two types' **`valueBase`** (a new one-word precomputed field on
every `typeinfo`, value-representation §4: constraint types erase transitively to
their plain root, all other types are their own base) — different bases → `false`
(so `1 == 1.0`, `"1" == 1`, `circle == square` all stay false), same base → payload
comparison under the base. Cost: two indexed loads on the mismatch path, and zero in
statically-typed code, where the compiler folds the erasure and emits the base compare
or constant `false` directly. Consistency obligations recorded: hashing for `==`-keyed
structures is over (`valueBase`, payload); type values are exempt (`@someByte ==
@someInt` remains false, erasure applies to tags, never to type-valued payloads);
nominal identity (enum variants, error types) never erases; relational operators
already widened and now agree with `==`. Also: an enum-value row added to equality §6
(same variant, then structural payload; F3's missing-enum note), and error-value
equality logged as a new open question in equality §7.

**R9 — constraints split into two classes; `list` is the built-in shape constraint
(F4).** The contradiction: tables §2.2 said list-breaking writes *drift* the value to
`table`, while constraints §7/§10 said constrained-table mutations *panic*. Resolution
(constraints §7, new §7.1): all constraints share one machinery, minted typeid, value-
carried, mutation interception keyed off the value's typeid so it fires through widened
aliases, but the **outcome** of a breaking write is per-class. **Invariant** constraints
(`byte`, `port`, every user `constraint {}`) panic with the value unchanged, forced for
soundness: under retagging, a `byte`-declared binding reached through `&x: int` would
silently hold 300 against its own declared type. **Shape** constraints (`list`) retag
the value to its base per tables §2.2's operation rules, "a list is a list until it's
not," and are sound because shape is never a maintained invariant: `list` in declared
position (bindings, parameters, returns; tables §2.4) is an **entry assertion** over the
maintained base `table`, re-checked at each constrained entry, legal to drift from
between them. Criterion recorded: panic when the write is wrong, retag when the value
merely changed kind. Swept: constraints §7/§7.1/§9.4/§10/§11 (user shape constraints
logged open), tables §2.2/§2.4 bridges. tables §8.3's panic bullets were already
correctly scoped to general (invariant) table constraints and stand unchanged; R8's
equality erasure is unaffected (`list` erases to `table` in `==` either way).

**R10 — `table` is the union `list | hashmap`; declared `list` is maintained
(supersedes R9's entry-assertion model).** `hashmap` (any non-list shape) is
unreachable, cannot be declared, demanded, or matched, and unreported (`@t.typeName`
stays `"table"` for the non-list member, `"list"` for lists; tables §2.1). The framing
makes drift precise: a shape-breaking mutation is a value moving between union members,
legal exactly because the other member can never be demanded. Outcomes are keyed to the
**write path's declared type** (constraints §7.1, tables §2.2): through `table`-typed
or untyped paths the value retags ("declare table, can break into a table"); through a
`list`-typed path the write is a declaration violation, a compile error where
statically evident (`xs['k'] = 1` on `xs: list`), a panic where index-dependent
("declare list, no silent conversion"). Two supporting rules the design forces, both
adopted: **`&` references are invariant** (variables §5.1, functions §3.2: `&list`
never fits `&table`, `&byte` never fits `&x: int`, the classic mutable-reference
variance unsoundness, closing §9.4's widened-`&` scenario statically; §9.4 and tables
§8.3 re-exampled onto the surviving wider-container-slot path where value-carried panic
still protects invariant constraints), and **inference widens shape constraints**
(variables §1, tables §2.1: `var t = [1,2,3]` infers `table`, or the spec's own
`var myTable = []; myTable['element'] = 1` example would be a compile error; invariant
constraints infer as-is, a validated commitment stays). Enforcement-fact placement
recorded in §7.1: invariant constraints ride the value (same thing everywhere),
shape constraints ride the declaration (the same value legally drifts under one
declaration and not another). Write-back methods (`&tab.sort()`) are unaffected, they
desugar to by-value call plus rebind (variables §2), with tables §2.4's list-typed
returns keeping the rebind legal.

**R11 — `table` is the sole primitive; `list` is an ordinary invariant constraint
(supersedes R10; simplifies R9 to one class).** The union framing and `hashmap` are
removed, the reachable-hashmap variant was rejected because it does not partition
(string keys fit neither member as defined), creates the symmetric violation (filling
the last gap exits a `hashmap` declaration), and required the silent coercion the
language forbids. Instead: list-ness is a **fact** every table maintains as the O(1)
bit, queryable as `isList()`, freely varying on unconstrained tables ("a list is a list
until it's not"); `list` the **type** is the built-in invariant table-level constraint
over that predicate, so a `list`-declared position is a **promise**, breaking writes
panic through every path, value-carried (constraints §9.4), compile error where
statically evident ("no silent conversion"). `list` is special among table constraints
only in cost: its per-mutation predicate reads the maintained bit, O(1) where general
table constraints pay their predicate, which licenses the contiguous-representation
optimizations. Consequences: constraints §7.1 collapses to one outcome (the fact/promise
split delivers both behaviors with one mechanism; the rejected shape class recorded
there); the shape-widening inference exception is deleted (a literal carries no
constraint, so `var t = [1,2,3]` infers `table` with no special rule; constraint-carrying
producers infer as-is, `var xs = t.values()` infers `list`); reference invariance (R10)
is retained, demoted from load-bearing to hardening, it turns §9.4's `&`-path runtime
panic into a compile error; `@` reports `"list"` only on constraint-carrying values,
`"table"` otherwise, type = commitment, method = fact. Equality (R8) unaffected: the
`list` typeid still erases to `table` in `==`. Deltas vs R10, accepted: protection
extends through container paths (consistent with `pair`), and shape is reported by
`isList()` rather than by `typeName` on unconstrained tables.

**R12 — attributes never touch the type: value-representation's contrary bullet
fixed (part of F8).** Ratified: attributes are compile-time-only constructs, erased at
runtime, with no mechanism to add them dynamically, and an attributed `int` *is*
`int`. attributes.md already specified exactly this (§1, §5, citing
value-representation §4.1); the one contradicting text was value-representation §4's
typeinfo bullet ("each distinct combination of base type and attributes is its own
typeinfo, and therefore its own id"), now rewritten to the correct story: attributes
are a compiler-side sidecar on the *declaration*, absent from `typeinfo` and `typeid`,
so same-shape declarations intern to one id and `@a == @b` is never perturbed by tags.
attributes §3.1's "the literal's type records them" phrasing tightened to "the
compiler's record of the declaration" to avoid re-implying type-carriage. Honest
residual logged in attributes §6 rather than papered over: since same-shape
declarations share a typeid, a bare `type` value can key attributes only for named
declarations (typeid ↔ declaration 1:1); how comptime reaches the attributes of an
anonymous table literal (the remaining prong of F8's serializer pipeline) is undecided
and needs its own ruling.

**R13 — `comptimeType(v)`: the comptime/runtime type split, with equality preserved
(resolves R12's residual, a further F8 prong).** Ratified insight: a comptime type
holds strictly more information than the runtime type (attributes), the two are
separate things, and equality semantics must be identical in both phases, for values
and for `type` values. Mechanism (reflection §3.2): the extra information rides the
value during comptime evaluation as a **provenance tag** (originating declaration),
preserved through copies and `comptime fn` parameters (what makes generator pipelines
work), fresh and attribute-free on computed values, erased at runtime lowering, and
**invisible to `==`**. The one bridge is `comptime fn comptimeType(v: any): table`,
returning a **declaration descriptor as an ordinary table** (`type` + `fields` with
attributes + `attributes`), deliberately **not** a `type`: returning `type` would
either leak attributes into type identity (re-admitting `int + attr != int`,
phase-shifted) or make equal `type` values observationally distinguishable. Descriptor
comparison is structural table `==`, attribute-inclusive by construction. Honest cost
stated in-spec: `comptimeType` is not a function of the value alone (two equal values
may differ in provenance), which is exactly why it is comptime-only. `fields(t)` /
`attributes(t)` on a bare `type` re-scoped to named declarations (typeid ↔ declaration
1:1), compile error on anonymous types directing to `comptimeType`. attributes §6 and
value-representation §4 updated to point at the resolution.

**R14 — `comptype` is an operator and a type (supersedes R13's function-returning-
table surface; provenance mechanism unchanged).** `comptype v` is a prefix word
operator in the `copy`/`try` family; `comptype` in type position names the built-in
descriptor type, dual-role like `error` (errors §3), which answers parsability by
existing precedent. A `comptype` value is a nominal record (`.type`, `.fields`,
`.attributes`, ordinary `.` access like error fields), deliberately **not** a subtype
of `table` so it cannot leak into runtime through table-typed slots, with structural,
attribute-inclusive `==` over its three fields, while `type` and value equality remain
phase-identical per R13. **Comptime-confinement rides the type**, not just the
operator: the operator is a compile error outside comptime, and additionally no
`comptype`-typed value may flow into anything that survives lowering (a runtime const
splice, a stored element, an `any` slot outliving comptime), each a compile error at
the offending point — the type-level rule is what makes "no runtime existence"
airtight where an operator-only rule would leak through `any`. Gains over R13's
surface: generators demand their input statically (`comptime fn (ct: comptype):
string`, called `toJson(comptype User)`), giving the F8 pipeline a typed intermediate.
operators.md catalogue row added; reflection §3.2, attributes §6, value-representation
§4 updated.

**R15 — the serializer pipeline types end to end; the F8 dependent-return gap closed
without dependent types.** attributes §4's canonical sketch (`comptime fn (T: type):
fn (v: T): string`, a type-parameterized signature nothing could express) is replaced
by the ratified pattern: `const toJson = comptime fn (ct: comptype): fn (any): string`,
called `toJson(comptype User)`. The specialization lives in **captured plain data**
(field/tag pairs extracted from the descriptor into an ordinary table, const-snapshot
captured per R3), not in the parameter's type, so no staged generics are needed; and
R14's confinement **enforces** the extraction, capturing `ct` itself into the returned
closure is a comptype value surviving lowering, a compile error at the capture. New
supporting rules made explicit because they were implicit and load-bearing: **phase
invariance** (functions §5.5: a comptime-eligible function callable in both phases must
yield phase-independent results, or folding changes behavior; `comptype`-taking
functions are vacuously exempt, being uncallable at runtime) and the **splice rule**
(compiler spec: a comptime-produced `fn` value lowers iff its environment is
confinement-free plain data). The **split API is kept deliberately**: `toJsonDynamic(v:
any)` is the attribute-blind structural walk (trivially implementable, the primitive set
is closed), and the proposed unification `toJson(ct: comptype | any)` is rejected in-spec
with the two reasons: the arms differ in observable output (tags honored vs erased), and
the unified function would be callable in both phases while phase-divergent, exactly
what phase invariance forbids.

**R16 — new spec file `types/json.md`: the `json` constraint and the serialization
surface.** `const json = constraint { string as str where isValidJson(str) }`, an
ordinary invariant constraint over the immutable base `string`, so it is **entry-only**
checked (constraints §7): validated at most once per value, then trusted forever, no
mutation surface exists, which is the precise form of "very cheap because strings are
immutable" (the honest cost, an O(n) parse at entry, once, elidable where provable, is
stated rather than waved away). `toJson` and `toJsonDynamic` return `json`, not bare
`string`: returning `string` discards the fact the serializer just established, forcing
downstream code to re-validate or trust blindly, while `json` keeps the fact in the
type, with `json <: string` making it free for consumers and `input as json` falling
out as the boundary-validation idiom with no new machinery. Dependency flagged in-spec:
the predicate is a function call inside `where`, so constraints §11 must resolve to
admit pure comptime-eligible calls, and `isValidJson` is recorded as the motivating
instance. attributes §4's generator example re-typed to return `json`; index.md row
added; fromJson deliberately out of scope, logged as open.

**R15b — serialization spec added.** New file `declarations/serialization.md`: the
`json` type as an ordinary invariant constraint on `string` (`constraint { string as
str where isValidJson(str) }`), entry-only because the base is immutable, one
validation at the boundary, elided where provable, and the typed-splicing payoff (a
`json` value embeds verbatim, a `string` gets escaped, the type is the escaping
decision). `toJson` and `toJsonDynamic` both return `json`, never bare `string`;
attributes §4's example updated to match; indexed.

**R16 — statement `apply` does not retype (F5).** The architecture was already in
protocols §9 (`@` is types, `@@` is protocols-as-data, `@P` is the static refinement;
wearing is a value property); the defect was one comment in §10's example claiming
`b apply stringBuilder` gives `b` type `@stringBuilder`. Fixed and made normative in
§10: **expression**-position `apply` yields a `@P`-typed value, so declarations get
static checking (`var c: @stringBuilder = [] apply stringBuilder`); **statement**-
position `apply` is a runtime mutation of the worn set with zero static effect and
zero CFA, and protocol access through a non-`@P` binding is resolved against the
value's worn set at runtime, panic if absent, the fact/promise split of constraints
§7.1 applied to protocols. The split is sound with no check on `apply` itself because
application is **monotone**; that condition is now recorded in §12's Removal question:
if `unapply` is ever added, `@P` declarations become breakable promises and removal
needs invariant-panic treatment, it cannot be a free mutation. tables §5.2.1's
no-flow-typing argument stands unchanged and is no longer contradicted. Still open,
adjacent: `@P` typeid canonicalization (F21) and const's reach over meta space /
view-path mutation (F25).

**R17 — `toString` dispatch fixed to the qualified view form (F7).** conversion §3's
dispatch was written as an unqualified meta call (`value->hasMeta('stringify')`,
`value->stringify()`), which the protocol model forbids: bare `->member` reaches only
the built-in pool and cross-protocol unqualified lookup is impossible by design
(protocols §3.3, §12). Resolution: `stringify` is a **well-known protocol**, not a
floating meta-function name, and the dispatch is `value is @stringify ?
value->stringify.toString() : builtinRender(value)`, membership test on the worn set,
then the qualified view form, then a member call, every step already permitted, no
exception to the lookup rules. Two details pinned: the wear-test is `is @stringify`
(never view-truthiness), and the protocol's member is contractually non-errorable
(`toString = meta fn (b: table): string`), enforced at application, so
`toString`-the-conversion keeps its totality guarantee through the extension point
rather than hoping for well-behaved implementations. How a wearer supplies its
rendering is deliberately left to the ordinary protocol mechanism.

**R18 — the two-tier subtype test, and `<:` scratched from source (F9).** The
preorder-interval test is provably tree-only: intervals of one numbering are laminar,
so `int <: int | double` and `int <: int | string` with the two unions incomparable
admits no interval assignment, the multiple-supertype DAG value-representation §4.2's
own parenthetical warned about, arising for every type with two union supertypes.
Resolution, now in §4.2: intervals are **scoped to the nominal tree** (errors,
constraints, single-parent edges), and **unions decide by member decomposition** over
the flat canonicalized member list already in the union's `typeinfo`, `T <: (A|B|…)`
iff `T <:` some member, union-on-the-left distributing; honest cost O(1) for tree
edges, O(members) for unions, foldable at compile time between static pairs. **`is`
keeps exactly one meaning**: a value against a type, running the two-tier check on the
value's current typeid (always concrete, never a union), which is.md §2/§4 already
specified correctly, including the ruling that the type-to-type relation gets no
operator, its answer between statically-known types is a compile-time constant, served
by `isSubtype` in reflection. The corpus review found the offenders and fixed them:
type.md §5 (listed `t <: u` as an operator running "the interval check" for all
tests), reflection §3.1 and its closing summary (called `is`/`<:` "the operator forms
of isSubtype"), is.md's cost sentence (now cites the two-tier rule) and its `:<`
spelling slip, constraints §9.1's ambiguous operator list. `<:` survives everywhere
else as what it was doing real work as: notation for the relation in the documents,
never grammar.

**R19 — `caps.*` axed; `pub` is `export`.** Capabilities are ordinary `const`s that
their defining module exports (`io` from `std.io`, `reveal` from `std.secret`), named
in `use` clauses as imported bindings; no namespace, no registry, audit unchanged
because modules resolve at compile time to one canonical declaration (capabilities §4
rewritten; 33 `caps.*` sites swept; the ambient-`caps` builtin mention removed from
modules.md). `pub` → `export` throughout modules.md (26 sites).

**R20 — `std.io`, the first standard module, and the enum-literal context rule.** New
file `std/io.md`, built entirely from ratified machinery: one exported capability;
`file` is a table wearing `fileDescriptor` on the string-builder precedent
(identityEquality, transferred crossing class, so opened files are race-free by
ownership with no locks); the standard handles are shared `const file` values, the
runtime's sink lock behind them being the module's only lock, buying line-level
atomicity; `openFile` returns `!file` with a small extensible `fileError` family;
print family returns `undefined` (the no-discard rule kept whole); `lines` is
text-only and `seek`/`write` binary-only (symmetric misuse panics); `chunks` added for
binary; `dump` renamed `readAll`; `close`/`flush` with the `defer` idiom and a
GC backstop that is explicitly not a contract; the failure model summarized in one
table (open = declarable, misuse = panic, mid-stream = panic). Call sites name
anonymous-enum variants with the fenced literal (`openFile('x', {read})`), grounded by
a new rule in enum §3.3: **a fenced variant literal takes its enum from the expected
type at its position**, anonymous enums interning structurally so the context always
resolves to one canonical enum; no context, compile error. `platform` stubbed as a
comptime-known `const` table for default parameters.

**R21 — io consistency pass, and F13 closed by fiat.** Four fixes: the never-
returning thrower is **`die`**, not `fail` (never.md, the overview `main` example);
**`file` is defined**, an exported type alias `export const file = @fileDescriptor`
(aliases are pure sugar, type spec), so the module's two names for the type resolve to
one; **`path`** is an invariant constraint over `string` in the `json` pattern
(`constraint { string as p where isValidPath(p) }`), `isValidPath` deferred to
`std.platform` as platform-dependent, the constraint existing now so `openFile`'s
boundary is typed from day one (`fileName: path`, `fileError.path: path`); and the
errorable-type spelling is **postfix `T!`** everywhere, `file!`, `int!`, `self!`,
`result!`, which resolves the F13 prefix/postfix drift on the side operators.md and
errors §7 always documented, 46 prefix sites swept across eleven files including
functions.md's own grammar block and the `fn (A): R! <: fn!` ladder.

**R22 — the format-constraint family and the file-level parsers.** serialization §1
generalized from `json` to the family: `csv`, `yaml`, `xml`, each an exported invariant
constraint over `string`, predicates deferred with the nuance named per format (CSV is
dialect-relative, YAML version-relative, XML means well-formedness only, schema validity
excluded by design). `std.io` §7.1 adds `parseCsv` / `parseJson` / `parseYaml` /
`parseXml`, each `fn (fd: file) use (io): table!`: malformed content is a declarable
error (the canonical expected environmental failure), binary-mode input is a panic
(misuse, the `lines` rule), and the constraint types the seam without doubling work,
the parser is the validator, the entry check elided as provably redundant (constraints
§9.5). Document-shape questions (scalar-rooted JSON vs `table!`, XML attribute mapping,
CSV headers) logged in serialization §4 rather than silently decided.

**R23 — parsers move out of io (supersedes R22's file-level helpers).** Ratified
reasoning: parsing is format knowledge with no io in it, the io-touching part was one
`readAll` line. serialization §3 now owns the `from*` family, `fromJson(j: json):
table!` and siblings, taking the **constraint types** as parameters so the entry *is*
the validation (no double work, nothing checked where the value is already
format-typed), **pure and comptime-eligible** (config literals parse at build time,
impossible for any file-taking signature), and source-agnostic (file, network, argv,
builder). std.io §7.1 replaced by the composition idiom, `try fromJson(readAll(fd) as
json)`, where the `as` narrows the `string | bytes` union and runs the format entry in
one visible step; binary-mode misuse now fails at the `as` naturally. io stays a leaf
with zero format knowledge. Module granularity (one `std.serialization` vs per-format
`std.json` etc., so a JSON import never pulls an XML parser) logged as an open
question, leaning per-format.

**R24 — per-format modules.** `declarations/serialization.md` dissolved into
`std/json.md` (the full worked module: the `json` type and §1.1's the-type-is-the-
escaping-decision rationale, `toJson` / `toJsonDynamic` with the split-API reasoning,
`fromJson`, open questions) plus `std/csv.md`, `std/yaml.md`, `std/xml.md`, thin
modules on the same pattern, each carrying its own deferred-predicate nuance (CSV
dialects, YAML versions, XML well-formedness-only) and per-format open questions
(headers, anchors, attribute mapping). Importing a JSON parser never pulls an XML one.
Cross-references swept (attributes §4, std.io, index).

**R25 — wearer refinements are first-class types, and `&` is the general
intersection (F21).** The definition was stranded at the overview tier (`&` declared
"the intersection operator," `(@P & @Q) | null` shown, then scoped to refinements with
no representational spec below it, while type.md §5 kept `@P` out of the type universe
entirely). Resolution on the pattern R18 established for unions, **identity from
interning, membership not by intervals**: `@P` and canonicalized protocol sets intern
typeids (typeinfo records the sorted set, `@Q & @P` ≡ `@P & @Q`), so refinements sit in
unions, alias (`export const file = @fileDescriptor` now grounded), and compare by
typeid, while membership stays the O(1) worn-set test and `@x` never reports `@P`
(wearing remains a value property; `valueBase` = `table`, so `==` is unperturbed).
type.md §5 rewritten, the two-category split dissolved. New type.md §3.1: **`&` is
general**, total by normalization at interning: distribute over `|`; meet tree atoms
(lower type, or uninhabited and dropped); merge refinements over one base (constraints
conjoin, the type-position spelling of constraints §6's where-composition with the same
base-match and no-implication rules; protocol sets union; the two mix, `list &
@drawable`); a *written* type normalizing to uninhabited is a compile error (`never`
only by writing `never`); **`fn` types never intersect**, that would be overloading in
a costume, compile error. Membership decomposes by conjunction, the dual of R18's union
rule, and `(A & B) <: A` is free structurally (conjunct elimination, not predicate
reasoning, so constraints §6's no-solver stance is untouched). Theorem recorded: in a
single-inheritance tree, meets of ordinary atoms always collapse, so `&` is productive
exactly on the multi-membership axes, which is why the old scoping felt natural and why
lifting it costs nothing. Swept: protocols §9, value-representation §4.2 (the AND rule),
overview, operators catalogue (`A & B`, position-disambiguated from prefix `&`),
constraints §6 cross-ref.

**R26 — the io error hierarchy, the target pin, and the `!` position fix.** Three
items. The operator catalogue's logical-not row spelled itself `a!` while its own
description said prefix; fixed to `!a`, prefix, expression position, the clean split
against postfix type-position `T!` (R21) that makes both parseable by position alone.
compiler §0 (new): the compiler targets **linux-x86-64 only, for now**, a sequencing
decision that lets platform-relative surfaces (std.platform, the `path` predicate, io's
errnos) be specified concretely; widening revisits exactly those named surfaces. New
`std/io-errors.md`, verified against the open(2)/errno man pages: the `ioError`
hierarchy (`fileNotFound`, `notADirectory`, `isADirectory`, `permissionDenied` with
child `readOnlyTarget`, `alreadyExists`, `invalidPath`, `tooManyOpenFiles`,
`outOfSpace`) under `ioError = error { path: path?, errno: int? }`, grouped by
errors §5.2's catch-by-name rule with errno carried for diagnostics never dispatch,
plus the **fates partition** every implementer needs in one view: declarable at open;
`ENOMEM` routed to the existing `OutOfMemory` panic; `EIO` and write-side failures to
the mid-operation panic ruling (revisit flag kept); `EINTR`/`EAGAIN` absorbed by the
runtime; `EFAULT`/`EBADF`/flag-`EINVAL` impossible by construction in a memory-safe,
typed API. `invalidPath` documented as the runtime complement of the static `path`
constraint (symlink loops and per-fs limits no string predicate can see).

**R27 — the parser-blocking rulings: F10, F14, F20 closed; associativity spec
created; F11 resolved.** **F10**: infix `.` concatenation and `.=` are removed as
vestigial (strings §11 rewritten with the two rationales: the whitespace-sensitive
collision with member access, and the one silent `toString` coercion in a no-coercion
language); interpolation, the builder, and `join` are the joining mechanisms; `cString`
redefined via interpolation; seven example sites swept. **F14**: a match type pattern
**is a type expression** (bare `int`, unions, constraints, `@P`), optional trailing
binder, semantics exactly `is` plus narrowing; pattern-type position is type position,
so `@int` errors by the existing `@X`-on-non-protocol rule; match §1's `@T` spelling
was self-refuting (`@string` as an expression is `typeof(string)` = `type`) and the
`match (@x)` dispatch idiom, which never worked under honest semantics, is replaced by
matching the value; or-pattern examples converted; is.md example fixed. **F20**: UFCS
specified as functions §3.4, resolved statically **by shape**: `x.name(` with a call is
UFCS (free-function lookup, one function per name), `x.name` without is element/member
access, stored-fn calls spell `t['f']()` or `(t.f)()`, `->` stays the disjoint protocol
channel; the std-surface consequence fixed as a rule (one name, one signature, receiver
first; union params or distinct names where drafts implied variants, exact signatures
flagged as std work). **F11**: `===`/`!==` do not exist; bool.md's `!==` fixed to `!=`,
exact under erasure equality. New `expressions/associativity.md`: the twelve-tier
expression table (comparisons, equality, and ranges non-chainable; `??` right; `&&`/`||`
short-circuit left), the five-tier type-position table (`&` over `|`; postfix `!`/`?`
order-independent; greedy `fn` results), the word-prefix ruling (`try a + b` is
`try (a + b)`, with the trap the alternative sets), and six flagged open questions
(compound assignments, recommended deferred; unary minus vs `int.min` and ranges; `&`
rejection site; `await`'s missing catalogue row; the `throw ... if` idiom pinned;
or-pattern vs union-pattern grammar split).

**R28 — the parser-freeze rulings: compounds, ranges, `&`, `await`, pattern
grouping.** All six associativity §4 questions closed. **Compound assignments exist**
(`+=` `-=` `*=` `/=` `%=` `??=`), tier 12, sugar with single evaluation of the target
(`t[f()] += 1` calls `f` once), `??=` assigning only on null, `&&=`/`||=` excluded for
now. **Ranges never reverse**: `a..b` with `b < a` is the empty range, decided by the
counterexample `0..n-1` at `n == 0` (implicit descending would silently iterate `0, -1`
in the language's most common loop header); descending is explicit via `reverse`;
companion ruling, unary negation of `int.min` panics, joining `INT_MIN / -1` in int's
overflow list. **`&` outside argument position rejects semantically**, keeping the
grammar regular. **`await` specified** in new concurrency/await.md: word prefix that
parks the green thread (no function coloring, the green-thread dividend stated as
deliberate), result **moved out** as the symmetric half of spawn's copy-in (task done,
no alias can exist, transfer free), promise **consumed** (moved-from on second await,
joining streams/builders/files), task errors surface at the await behind `try`, panics
propagate fail-fast; never-awaited promises run anyway and a dropped declarable error
elevates to a panic at task exit (nothing unhandleable vanishes silently).
**Cancellation deferred by decision, with the shape on record**: never preemptive (the
async-exceptions trap), cooperative and suspension-point-delivered as runtime-minted
`Cancelled` under `Panic`, unwinding through `defer`s, slotting into R7's origination
rules and io-errors §4's EINTR note. **Pattern grouping**: `|` at pattern top level is
always the or-pattern separator; inline union type patterns take parens
(`(int | string) n`), LR(1) with one-token decisions, near-costless since the
or-pattern binding rule already types the idiomatic spelling. Catalogue rows added for
`await` and the compounds; match §5 and int.md updated.

**R29 — the operator audit, and `|>` hardened.** Corpus-wide operator sweep: the
catalogue was fuller than feared (`???` and `???=` were already rowed in the coalescing
family; ranges, `defer`, `spawn` present), and the gaps were `|>` (in neither catalogue
nor precedence table) plus two R28 omissions in the associativity table (`???` joins
`??` at the coalescing tier, `???=` joins the compounds). `|>` now has its catalogue
row and its own precedence tier, left-associative, between coalescing and the word
prefixes (tiers renumbered: word prefixes 12, assignment 13), so `try src() |> stage`
wraps the whole pipe. The soundness challenge found the operator sound in four of five
dimensions, pipeline.md had pre-empted general-application redundancy (UFCS owns
`f(x)`), cross-kind laundering (closed over kind, bridging is explicit `exec`), hidden
effects (inert construction), and unbounded memory (pull-driven), and one real flaw,
created by this session's own rulings: piping was "a discipline, not an enforced move"
while R20/R28/concurrency §2.3 made enforced moved-from the house style for every other
single-owner resource, and the soft version was worse than stale style, the piped-from
handle and the pipeline shared a live cursor, so interleaved pulls made elements
silently vanish between consumers. **Fixed: `a |> t` moves `a`**, moved-from panics,
compile error where static (pipeline §5.1, stream §7.3 rewritten). Two properties
surfaced as consequences of existing rules rather than new ones (pipeline §5.2):
stages are `use`-free by construction (second-class capability closures cannot be
passed, capabilities §3.1), so a pipeline's effects live only at its endpoints; and a
failing stage panics at the pull (streams have no error channel, the mid-stream
ruling). Plain re-iteration of an exhausted stream stays a discipline, with the
distinction argued (deterministic emptiness vs live-cursor aliasing) and the
should-it-also-enforce question flagged in stream §8.

**R30 — commands are immutable values; the pipe move is class-based, not
operator-based (refines R29).** The challenge found the belief underspecified rather
than wrong: command.md never declared command's value class, and R29 made the class
load-bearing. Ratified in command §4: a `command` is an **immutable value**,
argv-and-structure, a description with no cursor, handle, or consumption state; all
modification is construction. Two consequences stated as features: piping does not
move a command (a named stage composes into any number of pipelines and stays valid),
and `exec` does not consume one (running a description twice is two process trees).
pipeline §5.1 reframed accordingly: **`|>` has no move rule of its own**, operands
behave per their class exactly as function arguments do, streams move because
transferred single-owner resources always move, commands share because immutable
values always share, so the apparent asymmetry is the one rule every call site
already follows, and the analysis en route established that even mutable-COW commands
would have been share-safe, immutability was ratified because it matches the spec's
actual surface (no mutators exist) and is the stronger guarantee.

**R31 — terminology standardization: tasks, consumable, taken, application.** Four
verbiage rulings swept corpus-wide (34 files). **Tasks**: the unit of concurrency is
the task, `spawn f()` starts a task and returns its `promise` (the handle type is
unchanged); "green thread" survives as the substrate term in exactly one definitional
sentence (concurrency §1: a task is implemented as a green thread) and in runtime-
internal discussion, never as the prose workhorse ("parks the current task").
**Consumable**: named as the stream property word at stream §2 ("streams are
consumable: consumption is single-pass..."). **Taken**: the state of a value whose
ownership has moved is "taken" everywhere "moved-from" stood (34 sites): a stream
crossing spawn is taken, a promise after await is taken, a piped-from stream is taken,
use of a taken value panics. **Application**: the protocol relation speaks apply-family
vocabulary, matching the operator: a table is "an application of P" / "has P applied,"
the question is "is this protocol applied," `@P` is the **application refinement**
(formerly wearer refinement), the applied set (formerly worn set), the wear metaphor
retired at ~160 sites with grammar-checked rewrites.

**R32 — keywords.md: the reserved-word inventory, with flags.** New top-level
`keywords.md` collecting every keyword with its governing spec, split into declaration
keywords, control flow, word operators, value/contextual keywords (`self`, `panic`, and
`_`-as-identifier), and the deliberate non-keywords (builtin type names are predeclared
identifiers, keeping the lexer's reserved set closed). One drift fixed en route:
stream-api.md's `foreach (s as v)` spelling violated control-flow §1's explicit ruling
that the binder is `in`; swept to `foreach (v in s)` / `foreach (k => v in s)`. Seven
flags raised for rulings: builtin-name shadowing (recommend shadowable via ordinary
scope, Go's answer); the `panic`/`Panic` selector-vs-type pair, used but never
reconciled; **`implicit`'s written form, which no example anywhere shows**, blocking
the implicit capability tier from being declarable in real source; `yield` inside
nested closures (should be illegal, needs the sentence); reserving `by` now against
future stepped ranges; the `unsafe-` hyphen convention vs identifier lexing; and a note
that `in` is spoken for if a membership operator is ever floated.

**R33 — the keyword rulings: implicit scratched, `panic` lowercase, shadowing,
lexical `yield`, `by` reserved, `unsafe` camelCase.** All seven keywords.md flags
ruled. **`implicit` removed entirely** (capabilities §6 rewritten): every capability is
explicit, a required-but-undeclared `use` is a compile error, no inference tier, the
rationale recorded, silent capability appearance is the exact invisibility the system
exists to prevent, purchased to save one `use (io)`. **`panic` is the lowercase type
itself** (91 sites swept), matching root `error`, so `catch panic p` is ordinary
catch-by-type and the contextual-keyword reading dissolves; follow-up flagged, builtin
children casing (`TypeError` vs the std `ioError` style) deserves one ruling.
**Builtin names are shadowable** (modules spec): outermost universe scope, ordinary
scoping, `let int = 5;` legal and locally self-punishing, typeids unaffected.
**Generator classification is lexical, per function literal** (stream §2): a literal is
a generator iff its own body, excluding nested literals, contains `yield`; nested
`yield` makes the nested fn its own generator, the PHP rule, one tree walk, no CFA.
**`by` reserved** for stepped ranges. **`unsafe` is a camelCase prefix**: hyphens in
identifiers would make whitespace load-bearing against infix `-` (the disease removed
with infix `.`), the near-universal answer in infix-minus languages; capability and
module names swept, `unsafeFfi`, `unsafeSystem`, `unsafeShellExec` (functions §5.6,
capabilities §8, exec, command).

**R34 — F16, F18, F22, F25 closed; error-children casing unified.** **Casing**: the
five builtin panic children join the language's one convention, `typeError`,
`outOfMemory`, `reassignmentError`, `incompatibleTypeError`, `cancelled` (20 files),
by the same argument that lowercased `panic`, no PascalCase island. **F16**: the u8
drift located exactly, int.md's table said "constraint on `int`" while numeric-tower
said "constraint on `u64`"; ruled per the ratified u16 semantics, `u8` sits in the
unsigned tower over `u64` (`u8 <: u16 <: u32 <: u64`), and the table row now also
states the byte/u8 division of labor, same range, different bases, `byte` is the
`int`-based bytes-element and IO workhorse, crossing them is a signed/unsigned family
crossing needing explicit conversion. **F18**: the truthiness leaks fixed, table-api's
`some`/`every` predicates are typed `fn (any): bool`, and views §3.2's `if (tab->P)`
example, a view is not a bool, becomes `tab is @P`. **F22**: new types/any.md, the
surgical rule, `any` supports exactly the universal operations (`==`, `is`, `as`,
`match`, `@`, `toString`/interpolation, store/pass/copy) and type-specific operations
(arithmetic, ordering, indexing, typed UFCS, `|>`, `&`) are compile errors requiring
narrowing first; total disallowal analyzed and rejected with the breakage named
(heterogeneous `foreach`, `toJsonDynamic`'s walk, error/log payloads). **F25**: the
forced ruling, `const`'s freeze covers **meta space and the applied set**, mutating
meta calls on const-rooted values are compile errors where static and panics through
dynamic paths, statement `apply` on const is an error (variables §3, protocols §10),
forced because const values cross tasks shared by reference and a mutable meta side
door would be the data race the isolation model exists to prevent; std handles remain
coherent, their buffers are runtime-side (std.io §2.1). En-route fix: std.io's own
`defer close(&fd)` example used `let fd`, but `&` requires `var` (R1); corrected.

**R35 — destructuring rulings.** Four of §7's five questions ruled. **Mixed patterns
are legal** (§3 rewritten), with the justification upgraded from tolerance to
principle: the pattern grammar is the table-literal grammar with targets in value
position, and the literal already mixes, so bare targets consume implicit integer keys
in order while `key =>` entries bind independently, positional part under §1's
exactness rules, keyed part under §2's, what the literal can build, the pattern can
take apart. **Rest is trailing-only**, ruled: `[a, ...mid, z]` is a parse error, a
mid-pattern rest would force length arithmetic with no carried use case. **Patterns
bind in every binding-introducing position**: declarations, function parameters, and
`foreach` heads (the PHP lineage), identical semantics and typing everywhere, with one
guard, a destructured parameter is by-value only, `&` marks whole named parameters,
never patterns. **Nested destructuring deferred by decision**, flat patterns plus a
second statement cover it until real code demands depth. En-route: `...` had been live
in destructuring §1.2 since before the R29 operator audit and had no catalogue row;
added (pattern punctuation, never an expression operator, noted in associativity's
exclusions).

**R36 — spread ruled and its missing spec written.** The corpus cited a "spread
spec" from tables, operators, and command that did not exist; new
`expressions/spread.md` founds it on the four rulings. **Streams spread**: `[...s]` is
the eager materialization, foreach-class consumption to exhaustion, with the two
deliberate costs named (unbounded stream, unbounded memory, bound it with `take`
first; spreading is the chosen opposite of pipeline laziness). **One level, always**:
an entry that is a table arrives as one value, depth is a policy and policies get
functions, the existing `flatten(tab, depth, preserveKeys)` is the explicit API.
**Call-site spread** (`f(...args)`, PHP lineage): list-shaped required (panic
otherwise, string keys have no positional meaning and named arguments do not exist),
fills positionally with defaults covering the tail, runtime arity check where length
is dynamic. **Interpolation spread** (`"$...xs"`, `"${...expr}"`): each element
rendered by `toString` in sequence, total, exactly the loop of `"$x"`s it abbreviates;
the proposed panic-unless-string rule was pushed back on and recorded as rejected, it
would make this the one non-total rendering form and diverge from the scalar
interpolation it abbreviates, strictness is spelled `as` upstream (one-line flip if
overruled). Table-literal spread §1 states the concat/merge unification (list part
reindexes positionally, keyed part merges later-wins) grounding operators §0.1's
no-`+`-on-tables rationale. Command-literal `${...flags}` is the same form, one
element one argv entry, never re-tokenized. The duplicate `...` catalogue rows (R29's
and R35's) consolidated into one position-disambiguated row; command.md's
still-to-be-specified notes resolved.

**R37 — wildcard rulings.** wildcard.md §8's three questions closed. `_` in nested
destructuring is subsumed by R35's deferral of nesting itself (flat rules compose at
each level when depth lands, nothing new to define). Trailing `_` parameters are legal
and kept: behaviorally identical to omission (surplus arguments drop anyway) but
retained because the signature then *documents* the deliberately-ignored argument,
explicitness that costs nothing. Whole-value discard is `_ = expr` and nothing else,
one spelling for one thing, the errors §8.1 form.

**R38 — tests as a language feature.** New `declarations/tests.md` from the ratified
vision, with the semantics assembled from existing rulings rather than new mechanism:
`test 'name' { ... }` declares a zero-parameter function whose return is **implicitly
`undefined!`** (forced: throwing is how a test fails and R7 makes every originating
throw require errorability; no annotation written or permitted), with ordinary `use`
clauses and **the runner as the granting entry point, exactly as `main` is**. Failure
is any throw, declarable or panic, and the runner's collection is `spawn` + `try
await` verbatim, R28's await already delivering both failure kinds at the collection
point; isolation and parallelism are the concurrency model as-is, with R34's
const-meta freeze making "parallel tests cannot race on Luna state" provable. One
soundness catch in the proposal, resolved: a const table of test *closures* would
launder capabilities past `use` propagation (a capability-declaring test is a
second-class function value, capabilities §3.1), so the exposed `tests` table holds
metadata rows, capability-free tests carry a callable `run`, capability-declaring
tests are runner-invocable only, the same posture as `main`, a consequence rather
than a policy. Names are compile-time-unique identities; duplicate names are compile
errors. Runner: `luna -t`/`--test`, per-test report, nonzero exit on failure,
declaration-order reporting, scheduler-order execution; discovery conventions,
fixtures, temp-path helpers, `assert`, and comptime tests logged open. `test` added
to keywords.md.

**R39 — second-class function values repealed; the requirement-set check installed
(revises R4, simplifies R38).** Ratified: capability-holding function values are
**first-class**. The old confinement existed because no dynamic check did; the new
model supplies it, a function value carries its **requirement set** (fixed at creation,
a bitmask over the closed capability universe, one word on the value, not a captured
token, capability tokens themselves remain confined and never enter value slots), and
**calls through fn-typed slots check requirement ⊆ the executing frame's grant**,
panic on shortfall; **direct calls stay statically checked**, unchanged; `spawn` on a
value checks against the spawner. The laundering theorem survives and is now stated in
capabilities §3.1: every actual exercise of a capability occurs under a declared `use`
on the executing path, because the dynamic caller must hold the grant, so possession of
a closure is possession of inert data, never of authority, and the `use (io)` audit is
exactly as true as before. Honest costs stated: value-mediated calls can fail at
runtime rather than compile time (the price of first-classness, paid only on the
dynamic frontier), and pipeline §5.2's "stages effect-free by construction" weakens to
"stage effects ambient-checked at the pull," more honest anyway. Free wins: the
comptime sandbox preserves itself (no grants exist at comptime, requirement-carrying
values fail vacuously), callback APIs need no special cases, and tests §4 simplifies,
**every test gets a callable `run`** with a `requirements` field, invocation checking
the caller's grant, so an io test runs anywhere `use (io)` is held and panics cleanly
anywhere else. functions §3.2 and §5's second-class rationales rewritten to the
value-carried check.

**R40 — test open questions ruled.** Discovery is any-module, no file magic,
conventions are the codebase's business. Zero parameters is final, fixture injection
off the table, fixtures are ordinary bindings. `assert` is ruled out: `throw
error('...') if (cond)` is the one way to fail, and expression-capture niceties do not
buy a second mechanism. "Comptime tests" resolved as pipeline composition, not a
language feature: `luna -t -c someProgram` runs the tests then produces the build,
ordinary runtime programs under the runner's grant, the comptime sandbox uninvolved.
Per-test temp resources deferred by decision pending `std.system`.

**R41 — the `luna` binary's flag table (compiler §0.1).** The suite's known flags in
one table: bare run (via a **content-addressed binary cache**, keyed on the resolved
module set's sources plus compiler version, sound because comptime determinism makes
same-inputs-same-binary a theorem), `-c/--compile` with `-o/--output` (default output
is the program name; an existing path is an error, never a silent overwrite, refined
with a build marker so a Luna-built previous artifact is fair to replace while foreign
files are never clobbered), `-t/--test`, `-f/--format` (formatter spec pending),
`-l/--lsp`, `-d/--debugger`. Composition ruled with a fixed canonical pipeline order
(format → build → test → artifact/run/debug) regardless of flag order: `-t -f -c`
compose freely with the artifact gated on test success; `-d` composes with `-c` (debug
builds compile first anyway, per the user's own reconsideration) and excludes `-l`;
**`-l` composes with nothing**, advised: a language server is a long-running JSON-RPC
process owning stdio, editor-launched, so batch-flag combination is incoherent by
nature, not by policy. Debugging a single test (`-d -t 'name'`) flagged as attractive,
deferred with the debugger spec.

**R42 — batch rulings: concurrency, exec, attributes (compiler / incremental /
modules / tooling / await questions skipped by decision).** **Concurrency**: the
result-collecting surface is **`await` over a stream of promises**, a lazy stream of
results (each pull awaits and takes one promise; force with spread; a failed task
panics at the pull since streams have no error channel, per-task error handling awaits
individually where the declarable channel exists), await §1.1; capability scoping at
spawn is **none**, the spawner must hold the spawned function's requirements, one
rule, the call rule; channels, backpressure, and scheduler tuning deferred by
decision. **exec**: streaming output ruled yes (stdout as a `stream`, the natural
`|>` producer); environment variables get a **new capability `env`** with one surface,
`envVars() use (env): table` of `key => secret`, so enumeration and reading are
separately gated (`reveal` extracts values), the double gate falling out of the secret
machinery; the `commandResult`/`commandError` boundary confirmed (ran = result for any
exit code, could-not-start = declarable error); command concurrency and cwd/stdin
attachment deferred. **attributes** §6 restructured into resolved/ruled/open:
attributes **cannot declare targets** (any attribute attaches anywhere, misapplication
cannot exist, consumers read or ignore); payloads may be **arbitrarily nested plain
data** (nothing in the machinery cared about flatness); other declaration forms
deferred; duplicate-application policy the one genuinely open remainder.

**R43 — capabilities open questions ruled; `argv` is a capability; eligibility
leaves the function typeid.** All five of capabilities §10's items closed. **`argv`
reversed from data to capability**: program input is authority, arbitrary dependencies
have no business reading it unbidden; surface mirrors env's (`args() use (argv):
list`), `main` declares `use (argv)`, the overview example updated (which also flushed
two stale spellings there, the old `File.modeRead` enum form and the pre-`path`
signature). The §9 table gains `argv` and `env` rows and **loses its tier column**,
stale since R33 removed `implicit` (the table still said `io | implicit`). Set
declaration confirmed already-defined (§7.1). The polymorphism resolution
**re-grounded**: its old text cited the repealed second-class rule; the standing
resolution is R39's, no annotation can be needed because the requirement set never
appears in a type, higher-order functions are transparently polymorphic over
authorities they never see. **Comptime-eligibility leaves the function typeid**
(functions §3 rewritten): the fn typeid is signature plus errorability and nothing
else; eligibility is a derived value fact, requirement set empty, the same one word
R39 installed, so `@f == @g` no longer distinguishes eligibility and the
comptime-boundary coercion reads the value's set; the standing audit assumption
(every ineligibility source is a capability) recorded with its candidates to keep
honest as std grows.

**R44 — capabilities are never receivers.** Five sites (the overview, never.md,
undefined.md, functions.md) called through the capability token, `io->println(...)`,
treating a zero-data capability as a table wearing a protocol. Wrong on both sides of
the arrow: `->` is protocol meta-navigation on tables (a capability is not a table and
applies no protocol), and the std surface is **free functions** that *declare* the
capability (`println(...)` inside a `use (io)` scope, std.io). All five swept to the
free-function form; never.md's process-exit example additionally rehomed from `io` to
the `system` capability (process control is not stream io), spelled `exitProcess(code)
use (system)`, deferred with std.system.

**R45 — the function-literal header pinned, and the `=> {` rule.** Confirmed sound
and LL(1): `fn` commits the production, each header junction is decided by its next
token (`use`, `:`, `=>`), and the `=>` reuse across lambdas, match arms, table pairs,
`foreach`, and `yield` is harmless because every context is committed before the arrow
arrives. Two findings pinned as grammar (functions §3): the **header order** is
`fn (params) [use (...)] [: returnType] => body`, `use` before the return type, which
22 of 23 corpus sites already followed (the one colon-first straggler, never.md,
swept); and **after `=>`, `{` always opens a block**, in lambdas and match arms alike,
because the fenced variant literal also uses braces and the parser must choose without
types, so a variant-literal expression body is parenthesized (`=> ({read})`), noted in
enum §3.3 beside the literal it disambiguates.

**R46 — postfix modifiers pinned: exact desugar, AST-order resolution, one per
statement.** The question, can the parser handle a postfix-`foreach` body referencing
names the head binds later in the text, splits into a trivial half and a real half:
parsing needs no binding knowledge at all (identifiers parse as identifiers, the
modifier is clean statement grammar behind a non-operator keyword), and resolution is
AST-order, not text-order, head first, body inside the head's scope, the
comprehension precedent. Pinned in control-flow: the postfix form **is** the block
form (`expr foreach (h);` ≡ `foreach (h) { expr; }`), so scoping, shadowing,
evaluate-source-once, and the per-iteration no-discard rule all inherit from the
desugar instead of being restated; head names bind within the body only; and **one
postfix modifier per statement**, chaining is a compile error, the which-nests-which
trap belongs to the block forms that already answer it.

**R47 — ranges desugared explicitly; `by` generalized; a live contradiction
reconciled.** Found en route: range.md §4 still said `10..1` descends, flatly against
R28's ratified empty-range rule (applied to associativity, never swept into range.md).
Reconciled by the ratified generalization, which is better than either old text:
**bounds never determine direction** (`b < a` is empty, the `0..n-1` counterexample
recorded in range §4), and **`by` takes any int expression, evaluated once, whose sign
is the direction**, so `10..0 by -2` is the explicit descending form R28's own note
floated, no loop header descends by accident because writing the sign is the
explicitness; step `0` panics (the infinite-loop guard, checked once). New range §4a:
**the desugaring, explicit**, a range is sugar for an immediately-invoked generator
(evaluate step once, guard zero, `while` with the sign-selected comparison, `yield`,
step), and the desugar *proves* the rules rather than accompanying them, `0..-1` runs
zero iterations, `0..10 by 3` yields `0,3,6,9` (settling §8's alignment question,
removed), `..<` is the strict comparison, `lo..` drops the condition, `restart` is
re-invocation. Non-constant progressions stay generator functions, the step is a
value, never a per-iteration computation. `by` graduated from reserved-for-future to
keyword (keywords.md); the associativity range tier, its R28 resolution note, and the
operator catalogue row updated.

**R48 — bounds-implied descending: the rejection recorded.** The `10..0 → by -1`
desugar was probed a second time (its mechanics were never the issue) and the user
retracted it after the consequences analysis; range §4 now carries the full rejection
record so a third probe reads instead of re-derives: the silent, data-dependent return
of the `0..n-1` bug; runtime-opaque direction for non-literal bounds; the
bounds-vs-sign conflict rule the current design never needs; and the frequency
asymmetry (the ubiquitous case must be safe by default, the deliberate case can say
so). The literals-only middle ground rejected on phase-invariance grounds, hoisting a
literal into a `const` must not change loop behavior. `downTo` noted as the future
sugar over the explicit form if `by -1` ever proves heavy. The ruling itself is
unchanged from R47: no implicit descent, `by -1` is the explicit form.

**R49 — idiomatic `main`, the try-idiom corrections, and `examples/`.** The
overview's `main` refined per ruling: errorable (`int!`, sanctioned by errors §5's
fn!-main), bare `openFile` propagating, `var fd` + `defer close(&fd)` per iteration,
the inner line loop postfix, the outer loop block-form; the map-of-errorable-opens
construction (which iterated `file!` values without handling them) is gone. En route,
a real semantic bug in my own R20/R23 texts: `try f()` yields the **union** (errors
§8, `v : string | error`), not propagation, so `var fd = try openFile(...)` bound
`file | error` and every downstream use was ill-typed; io.md's defer example and the
io/json composition idioms corrected to bare propagating calls with the try-recovery
alternative noted. New `examples/` directory, four worked programs verified against
the ruling set: **one-billion-rows** (lazy line fold, `??=` first-sight init,
element-path compound writes, the honest parallelization note about the
binary-chunking/text-lines tension), **log-scan** (pipelines, pull-driven bounding,
`fn (any): bool` predicates, universal interpolation), **serialization** (the
comptype/attributes/toJson flagship, the split API's two documents from one value),
and **testing** (throw-to-fail, try-as-recovery asserted on `is error`, capability
tests under the runner's grant). Indexed.

**R50 — `main`'s return fixed.** `ExitCodes.success` was a dangling reference
(defined nowhere in the corpus) whose implied enum could not satisfy `int!` anyway,
enums are nominal, never ints, no coercion. Fixed per the ruling: **`exitCode`**, a
`const` table of named int codes declared beside `main`
(`const exitCode = ['success' => 0, 'usage' => 64]`), so `exitCode.success` is an
`int` by element access and type-checks against `int!` with zero special machinery,
and the snippet now also teaches the named-constants idiom, a const table is Luna's
enum-of-ints, coercion-free. exec.md's `exitCode` *field* on `commandResult` is
unrelated and untouched.

**R51 — the `main` example moved to the front.** The "A taste" section (the
canonical `main`, its `const exitCode` companion, and the shapes-on-display note)
relocated from the overview to `index.md`, directly under the opening paragraph, first
thing a reader sees; the overview keeps a one-line pointer in its place.

**R52 — Shiki grammar for Astro/MDX highlighting.** New `tooling/shiki-luna.ts`: a
TextMate grammar as a Shiki `LanguageRegistration`, wired via
`markdown.shikiConfig.langs` in astro.config (the wiring is in the file header),
enabling ```luna fences in .md/.mdx with no plugin and no LSP, highlighting is
lexical and the lexical surface is fully ratified, so the grammar is *generated from*
keywords.md's four keyword sets, the operator catalogue longest-first, interpolated
strings with `$x`/`${expr}`/`$...xs`, backtick command literals, `#[attributes]`,
`@P`/`@@` refinements, and the predeclared type names (styled though not reserved,
keywords §5). Each pattern group cites its governing section so regeneration tracks
the spec. The LSP remains the ceiling above this, semantic tokens in editors through
its own channel, irrelevant to static docs.

**R53 — the TS2307 fix and the `<LunaCode>` component.** The error is the
transitive-dependency gap: shiki ships inside Astro at runtime but TypeScript needs
it as a direct dependency for the `LanguageRegistration` type, so `npm i -D shiki`
(dev-only, the import is type-erased); noted in the grammar's header. New
`tooling/LunaCode.astro`: a self-contained code frame using Astro's built-in `<Code>`
component, which accepts the grammar object directly as `lang`, so the component
needs **zero config** (fences remain the config path; the component is the props
path, for code arriving as data). Deliberately quiet chrome: optional filename
caption, hover-revealed Copy button (active-voice label, Copied confirmation),
CSS-custom-property theming with defaults so the host site restyles it without
edits, `:focus-visible` outline and `prefers-reduced-motion` respected. Usage
documented in the component header.

**R54 — `.luna` files as strings.** Vite's `?raw` suffix already does this at
runtime for any extension (`import hello from './hello.luna?raw'`); new
`tooling/luna-files.d.ts` supplies the ambient `*.luna?raw` module declaration that
makes TypeScript agree (place beside env.d.ts or add to tsconfig include), documents
the `import.meta.glob(..., { query: '?raw' })` bulk pattern for an examples index
page, and deliberately declares **no** bare `*.luna` module, a raw-less import has no
defined meaning until a compiler-backed loader exists, and staying untyped keeps that
mistake loud instead of silently `any`. LunaCode.astro's header now shows the
file-import path as the primary usage.

**R55 — `<LunaCode name="...">`.** The component now loads snippets itself: a
root-absolute eager `?raw` glob over `/src/assets/snippets/*.luna` (immune to page
depth and to where the component lives, the exact failure mode of the ENOENT just
hit), so pages write `<LunaCode name="a-taste-of-luna" />` with no import at all.
Title defaults to the filename; exactly one of `name`/`code` is enforced; an unknown
name **fails the build** with the available snippet names listed, an error that says
what to do next. Explicit `code` retained for one-off inline snippets.

**R56 — LunaCode restyled to the host theme; dual-theme highlighting.** github-dark
replaced with the **everforest-light / everforest-dark** pair (matched to the site's
sage-green/warm-brown palette; one word to swap). Automatic light/dark switching uses
the site's own mechanism rather than JS: Shiki renders with `themes` + `defaultColor:
false`, emitting `--shiki-light`/`--shiki-dark` per token with no hardcoded colors,
and one rule, `color: light-dark(var(--shiki-light), var(--shiki-dark))`, follows
`color-scheme`, which the site's `html[data-theme]` rules already set, so the toggle,
the system preference, and the code blocks stay in lockstep by construction. Frame
chrome rewritten onto the site's tokens with standalone fallbacks (`--color-border`,
`--color-surface-sunken`, `--color-text-muted`, `--radius`, `--space-*`, `--shadow`,
`--text-sm`); the custom focus outline and reduced-motion block dropped in favor of
the site's global rules, which already cover both.

**R57 — the monochrome fix.** Diagnosis from the render: Astro's `<Code>` renames
Shiki's `.shiki` class to `.astro-code`, so R56's `light-dark()` routing rule matched
nothing, and with `defaultColor: false` the token colors live *only* in custom
properties until CSS reads them, every span stayed colorless and the site's inline-
`code` color inherited through (the salmon monotone). Fixed by selecting **elements,
not classes**: `pre`/`span` under the component's own body scope, immune to whatever
Shiki or Astro names the wrapper, with the site's `code` styling explicitly
neutralized inside the frame (`background: none`, inherit font/color) and the pre
forced to full width (`max-width: none`) so prose styles cannot narrow the code
surface, the dead right gutter in the render.

**R58 — the contrast fix.** The worry is measurable: everforest-light's string
tokens are olive-yellow on cream, roughly 2.4:1, far under WCAG AA's 4.5:1. Light
theme swapped to **vitesse-light**, which keeps the green-keyword brand identity on a
near-white ground with token inks around 4.5-5:1; dark mode keeps everforest-dark,
which renders on its own background where contrast is fine and whose warmth fits the
site's brown dark theme. Rationale left as a comment beside the theme choice, with
the honest caveat that comments stay muted by design in every theme and the
maximal-legibility alternative (github-light-high-contrast) named for if that ever
matters. The principle recorded: contrast is fixed by theme choice, not CSS surgery,
token palettes are tuned to their own backgrounds.

**R59 — the cutoff trimmed at the source; editor highlighting shipped.** The
apparent truncation was two things at once: the pre scrolls (macOS hides the
scrollbar) and, more honestly, the taste snippet's comment column was aligned for the
spec's wide fixed-width context, forcing ~100 columns onto a ~72-column web frame;
the index example's comments trimmed and the long postfix line broken, and the
canonical snippet extracted to `tooling/a-taste-of-luna.luna` for copying into
`src/assets/snippets/` verbatim. The zero-highlighting observation identified
correctly as **editor**-side (the site render is fully tokenized): inline fences
would not help, the editor knows `luna` no better than `.luna`. New
`tooling/vscode-luna/` extension, no marketplace, no build, copy the folder into
`~/.vscode/extensions/` and reload: contributes the language id (so `.luna` files
*and* markdown ```luna fences highlight in the editor, VS Code builds fence patterns
from registered ids), a language configuration (comments, brackets, auto-close), and
a tmLanguage **generated from `shiki-luna.ts`** by a small extraction script, one
grammar as source of truth, site and editor in lockstep by regeneration rather than
discipline (the extractor needed a real quote-tokenizer; regexes paired apostrophes
across contexts, a fitting way for this session to end up writing a lexer after all).

**R60 — Zed support.** Zed highlights via tree-sitter, not TextMate, so vscode-luna
cannot port; two paths shipped. The 30-second stopgap: Zed's `file_types` mapping
onto JavaScript (const/let, `=>`, `//`, both quote styles, and backticks all read
correctly). The real path: `tooling/tree-sitter-luna/`, a deliberately **lexical,
highlighting-grade** grammar (tokenizes exactly, imposes no structure, cannot
mis-parse half-typed code; keyword classification via `#match?` predicates in
`highlights.scm`, generated from keywords.md's categories), plus
`tooling/zed-luna/`, the extension (extension.toml, language config with brackets
and comments, the queries) and a README walkthrough: `npx tree-sitter-cli generate`,
commit the generated parser to a repo, point extension.toml's repository/rev at it,
`zed: install dev extension`. Scope honestly noted: v0 does not highlight
interpolation inside strings, and the compiler's real parser remains a separate
artifact with a different job.

**R61 — tree-sitter.json.** Generation succeeded on the user's machine with the
CLI's no-manifest warning (ABI 14 fallback, harmless, Zed accepts it);
`tooling/tree-sitter-luna/tree-sitter.json` added (the 0.24+ manifest: grammar name,
scope, file-types, injection regex, metadata) so regeneration produces ABI 15
silently; README updated to say the warning was cosmetic.

**R62 — the grammar-compile triage.** Zed's "failed to compile grammar" is a
generic wrapper; the README gains a troubleshooting section with the three real
causes ranked: `rev` must be a pinned commit sha, not a branch or the scaffold's
placeholder; `src/parser.c` must be committed at that sha, Zed clones and compiles,
never generates; and ABI 15 (which R61's manifest now produces) requires a recent
Zed, with `generate --abi 14` as the compatibility move. `zed: open log` named as
the source of the real error text.

**R63 — the file:// URL fix.** The user's failure diagnosed from the config
itself: `file://home/...` parses the first path segment as a hostname (three
slashes needed for absolute paths), and the URL pointed at `src/` rather than the
repo root, which Zed clones before resolving `src/parser.c`, so the clone failed and
surfaced as the generic compile error; the rev and committed parser were verified
fine. README's local-path note expanded with both gotchas, a fourth triage entry
added, and the log locations (palette command plus on-disk paths for Linux and
macOS) written in.

**R64 — monorepo grammar option.** Answered: the grammar can live in the main repo
via the grammar config's optional `path` field (the tree-sitter-typescript
monorepo precedent), with the user's specific trap named, their nested `.git`
makes the outer repo's commits contain no grammar files (gitlink), so the inner
repo must be removed and the grammar committed into the main repo before the
outer sha can serve as `rev`; regeneration becomes a main-repo commit plus rev
bump, and a separate repo stays the recommended shape if the grammar is ever
published. README's install section gains the monorepo block.

**R65 — the stale-clone triage entry.** The user's log identified the real failure:
Zed clones grammars into the extension's own `grammars/` directory during dev
installs, and a leftover clone from the earlier `file://` attempts blocked the
retry against the new GitHub URL. Fix is `rm -rf tooling/zed-luna/grammars` and
reinstall; a `.gitignore` for that directory added to the scaffold (Zed's scratch
space, never committed) and the README gains triage entry five with the exact
error text quoted for grepability.

**R66 — regen-grammar.sh, the .gitignore, and the keep/delete ruling on tooling
files.** New `tooling/regen-grammar.sh`: clears the stale bits (Zed's clone cache
and the old generated `src/`), regenerates, commits ('regenerated
tree-sitter-luna'), pushes so the rev exists remotely before anything pins it,
derives the repository URL from `origin` (ssh normalized to https, Zed clones
anonymously), regenerates `extension.toml` whole with the fresh sha and the
monorepo `path`, recommits and pushes; runnable from anywhere in the repo, ends by
naming the Zed reload command. Root `.gitignore` added: the Zed scratch dir,
node_modules, wasm/build artifacts (generated `src/` explicitly *is* committed, the
comment says so), the make-archive zip, and environment noise. File-retention
question answered in-chat: shiki-luna.ts **stays** (source of truth for the
tmLanguage extraction and the canonical machine-readable lexical surface);
luna-files.d.ts and LunaCode.astro are website-scoped and safe to delete here.

**R67 — containerized regeneration.** regen-grammar.sh no longer touches local
npm: generation runs as a one-shot `podman compose run --rm` against a new
`tooling/compose.yaml` (node:22-alpine, the smallest official node image;
working_dir bind-mounted with `:z` for SELinux hosts; a named volume caches
npx's tree-sitter-cli between runs so only the first invocation downloads), with
`podman compose pull --quiet` first, the requested auto-re-compose, so the image
stays current and nothing stays resident. Rootless-podman ownership noted in the
compose comments (container root maps to the invoking user, generated files come
out correctly owned). Host requirements are now podman and git, nothing else.

**R68 — node:22-slim.** The compose image swapped from alpine to Debian-based
node:22-slim per the ruling, matching the website's own base image and avoiding
musl's occasional misbehavior with native tooling; comment updated with the
rationale. First run after the switch re-pulls and re-warms the npm cache volume;
everything else is unchanged.

**R69 — the Containerfile.** The question exposed a real gap, not just a missing
convention: npx-at-runtime floats the tree-sitter-cli version, and the CLI version
shapes the generated parser, so generation was not reproducible from repo state.
New `tooling/Containerfile` (`FROM node:22-slim`, `npm install -g
tree-sitter-cli@0.26.10`, the version the user's own successful run resolved);
compose.yaml switches from a bare image to `build:` with a named local image, the
npm cache volume deleted (nothing downloads at runtime anymore), and the script's
step 2 becomes `podman compose build` (cached, instant when unchanged, and the
honest meaning of "auto re-compose") followed by the one-shot run calling the
baked-in `tree-sitter generate` directly. The project's same-inputs-same-output
philosophy now applies to its own tooling.

**R70 — the live-artifact fix.** Diagnosed from the log: R66's script carried the
R65 stale-clone nuke (`rm -rf zed-luna/grammars`) into routine regeneration, but
after a dev install that directory holds the **live compiled grammar** Zed loads
in place, so the script deleted the wasm out from under the registered language
("No such file or directory"). The nuke was correct only for its original case, a
repository-URL change invalidating the clone; in the steady state Zed fetches the
new rev into its existing clone and recompiles on reload. Script no longer touches
the directory (comment explains why, pointing at triage §5 for the URL-change
case); README's entry five rewritten to scope the nuke to URL changes with the new
error text quoted; recovery is one `zed: install dev extension`.

**R71 — the template made obviously invalid.** The user's "failed to fetch
revision main" (their branch is master) traced to the archive's extension.toml
template still carrying the original `rev = "main"` placeholder, which their
drop-in of updated tooling files copied over the script-generated sha version, a
plausible-looking placeholder failing later and stranger than an invalid one
would. The template is now explicitly generated-file-marked with deliberately
invalid values (an all-zeros rev, a RUN-the-script repository) so any
non-generated copy fails at first contact with an error pointing at the script;
README triage entry one gains the fetch-revision symptom and the
extension.toml-is-generated-never-copied warning. Immediate recovery: run
regen-grammar.sh, reinstall.

**R72 — website files pruned; the glibc fix.** LunaCode.astro and luna-files.d.ts
deleted from tooling/ per the ruling (website-scoped, live copies are on the site;
shiki-luna.ts stays as source of truth), index row updated. The container failure
diagnosed from the error text: tree-sitter-cli 0.26's prebuilt binary requires
GLIBC_2.39, and node:22-slim is bookworm-based (Debian 12, glibc 2.36); base image
switched to **node:22-trixie-slim** (Debian 13, glibc 2.41), same node major, same
slim variant, rationale committed as a Containerfile comment with the exact error
quoted for grepability. The base-image change busts the build cache automatically,
so the script's `podman compose build` step rebuilds without intervention; note
that `podman compose up` is not part of this flow, the pipeline is build plus
one-shot `run --rm`.

**R73 — the conditional nuke, reconciling R65 and R70.** The stale-clone error
returned legitimately: the user's install history crossed a url change (file:// to
the origin-derived GitHub url), the exact case R65's nuke existed for and R70's
removal re-exposed. Both were half-right; the script now holds the synthesis: it
compares the existing clone's `remote get-url origin` against the url it is about
to write and clears the cache **only on mismatch**, announcing the repoint, so
routine regens preserve the live grammar and url changes self-heal, with the
manual command remaining documented for installs outside the script. Immediate
user fix this once: manual nuke plus reinstall.

**R74 — the Shiki grammar rederived from the lexer spec.** The user delivered a
full lexer specification (token inventory, modes, maximal-munch ordering, RE2
targeting, non-regularity flags); shiki-luna.ts regenerated against it as the new
source-of-truth citation, gaining what the approximation lacked: hex, binary, and
exponent numeric forms (the spec's exact patterns, including the
digit-on-both-sides-of-the-point rule that keeps `1..5` clean); `b"…"`/`b'…'`
bytes literals ordered before identifiers; `/…/imsxb` regex literals with the F2
division ambiguity approximated by a lookbehind heuristic (oniguruma affords what
RE2 does not; the caveat is in the header); block comments; `match!` as a unit
(its own pattern, since a trailing `\b` after `!` cannot match); `from`; and the
spec's unrolled-loop span forms adopted verbatim for portability. The vscode
tmLanguage re-extracted in the same pass, so both derived artifacts track the
lexer spec through one file. Noted in-chat: build/lexer.md is the natural home
for the spec document itself, on the user's word.

**R75 — canonical tree adopted; grammar promoted to the G-rulings; the script
split.** The user's uploaded zip is the working copy now: specs under `docs/`,
their `make-archive.sh` (git-ls-files-driven, gitignore-respecting) replaces mine,
the root and zed-luna .gitignores restored (zip tools drop dotfiles), CHANGES.md
carried forward. **Grammar**: shiki-luna.ts regenerated against
docs/build/lexer.md's G-rulings revision, with the header now citing rulings
rather than assumptions, `from` reserved (G1), block comments non-nesting so the
begin/end pair is the complete rule (G3), command literals escape-free with the
no-escape fact commented at the pattern it shapes (G5), formal ASCII identifiers
over UTF-8 sources (G6), `match!` one token (G7); G2 stands deferred with the
working assumptions unchanged, G4 affects decoding not spans; the vscode
tmLanguage re-extracted in lockstep. **Scripts**: the monolithic regen script
retired per instruction and split as requested into
`tooling/generate-grammar.sh` (the containerized parser regeneration, no git)
and `tooling/publish-grammar.sh` (commit the grammar, push so the rev exists
remotely, R73's conditional clone-cache reconciliation, regenerate
extension.toml, marked generated-do-not-copy, recommit, push, print the Zed
reload step).

**R76 — the phantom dependency removed.** The spec repo has no node_modules, so
the `import type { LanguageRegistration } from 'shiki'` was a permanent TS2307 in
any editor opening the repo, flagging a dependency that was only ever
documentation. Replaced with a local structural type (TmRule plus LunaGrammar),
Shiki accepts grammars structurally, so the file is now self-contained and
editor-quiet everywhere, and the real type check relocates to the one place shiki
actually exists, the website, via an optional documented one-liner
(`const _check: LanguageRegistration = lunaGrammar;`). The extraction pipeline is
unaffected, it parses the object literal, not the types.

**R77 — the website grammar sync.** The website's copy of shiki-luna.ts stops
being a fork: `tooling/sync-luna-grammar.sh` (templated here, lives in the
website's scripts/) shallow-clones the luna repo on `npm run dev`/`build` via
package.json pre-scripts and overwrites the website copy, stamping it with a
GENERATED header naming source, rev, and syncing script (the R71 lesson applied
forward). Two safety refinements over the raw proposal: `--depth 1`, and an
offline fallback that keeps the existing copy with a warning when the repo is
unreachable, hard-failing only when no copy exists, so no-wifi never blocks the
dev server. Committing the synced file recommended (generated-but-vendored, the
tree-sitter src/ precedent): fresh clones and offline CI always build, and
upstream grammar changes surface as reviewable diffs.

**R78 — the sync script rewritten for the containerized website; multi-arch
answered.** The website's Containerfile revealed node:22-slim, which ships
neither git nor curl, so R77's bash+git script could not run where npm runs;
replaced by `sync-luna-grammar.mjs`, pure Node built-in fetch (>=18), raw-URL
fetch with a best-effort GitHub-API sha for the GENERATED stamp, DEST corrected
to src/lib/, an unchanged-content short-circuit so watch-mode tooling stays
quiet, the same offline fallback semantics, and a 10s timeout on every fetch
because the image's CMD makes a hung predev a hung container, the failure class
the user just killed. The luna tooling template swapped to the .mjs; the
standalone file delivered for the astro repo. Multi-arch ruled possible as-is:
node:22-slim is a multi-arch manifest (amd64+arm64), podman machine on Apple
silicon runs linux/arm64 natively and pulls the right variant, and the compose
file's anonymous-volume node_modules mask already gives each machine its own
in-container native binaries, the classic cross-arch trap dodged by structure;
avoid --platform emulation, unnecessary and slow.

**R79 — gated secrets: per-capability secret authority, zero new machinery.** The
hole named by the user, one `reveal` capability opening every secret, closed at the
effect site. Ratified signature (the user's variadic insight, corrected):
`secret(raw: string|bytes, ...gates: type): secret`, gates are capability
**typeids** (`@dbCred`, tokens stay confined, the `&capability` params in the
sketch would have violated §3.1), zero gates means the default `[@reveal]` with no
spelling for an empty set, multiple gates are **AND**. The gate set rides the
value, and `reveal`/`revealBytes` drop their static `use (reveal)` for the dynamic
check, gate set ⊆ executing frame's grant, panic on shortfall: the **third
application of the one R39 check** (value-mediated calls, spawn, now secrets),
same bitmask word, same subset test. The laundering theorem extends verbatim;
the audit becomes per-gate (`use (dbCred)` lists exactly who can see the database
password) and `use (reveal)` is demoted to the default gate's key; `gatesOf(s)`
for check-before-reveal; re-gating is reveal-then-rewrap so changing gates
requires authority over the current ones by construction; `as secret` ≡
`secret(raw)`. The user's original constraint-based proposal (`can` inside
`where`) rejected for the reason now made a stated principle in constraints:
**predicates are functions of the value alone**, frame state is an illegal input,
because checked constraints become facts that ride values across frames and
frame-dependent truth would mint facts that stop being true in transit. The
`can` expression outside constraints deferred on YAGNI; compiler-sandboxing
recorded in-chat as latent (restricted root grant + the R39 check binds even
smuggled closures; missing pieces are an embedding API and resource limits).

**R80 — `can` scrapped entirely; sandboxing's mechanical core recorded.** The
`can` expression, left half-open by R79's YAGNI deferral, is now scrapped in every
form: no frame-inspecting operator exists anywhere in the language, grants stay
invisible to programs and are checkable only by exercising them (constraints,
principle paragraph updated). Compiler-driven capability revocation deferred with
its CLI/embedding API undecided, but the mechanical core is on the record in
capabilities: **the compiler modifies `main`'s `use` set and everything falls out
for free**, static propagation refuses what the root never granted, R39's dynamic
check binds smuggled closures, R79's gates cover secrets, no second enforcement
mechanism needed; resource limits remain the genuinely separate missing piece.

---

## Not changed (out of scope of these rulings, still open from the review)

 `list` drift vs
panic (F4); `as` algebra exceptions (F6); union subtyping vs interval
test (F9); `any` pipelines (F22); view interior mutability (F25). The variables.md-internal
tension (§3 "not a runtime seal" vs "seals the copy" vs the runtime arm in §7's error
table) is also left for a ruling.
