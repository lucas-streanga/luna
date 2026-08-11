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
created by these rulings: piping was "a discipline, not an enforced move"
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
builds compile first anyway, on reconsideration) and excludes `-l`;
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
desugar was probed a second time (its mechanics were never the issue) and it was
retracted after the consequences analysis; range §4 now carries the full rejection
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

**R59 — the canonical snippet extracted; editor highlighting shipped.** The
canonical `main` snippet was extracted to `tooling/a-taste-of-luna.luna` (its
comment column trimmed and the long postfix line broken so it fits a narrow frame),
the single copy of the front-page example. Syntax highlighting then shipped to
editors as well as static docs: new `tooling/vscode-luna/` extension, no
marketplace, no build, copy the folder into `~/.vscode/extensions/` and reload: it
contributes the language id (so `.luna` files *and* markdown ```luna fences
highlight in the editor, VS Code builds fence patterns from registered ids), a
language configuration (comments, brackets, auto-close), and a tmLanguage
**generated from `shiki-luna.ts`** by a small extraction script, one grammar as
source of truth, docs and editor in lockstep by regeneration rather than discipline.

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

**R61 — tree-sitter.json.** Generation succeeded with the
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

**R63 — the file:// URL fix.** The failure diagnosed from the config
itself: `file://home/...` parses the first path segment as a hostname (three
slashes needed for absolute paths), and the URL pointed at `src/` rather than the
repo root, which Zed clones before resolving `src/parser.c`, so the clone failed and
surfaced as the generic compile error; the rev and committed parser were verified
fine. README's local-path note expanded with both gotchas, a fourth triage entry
added, and the log locations (palette command plus on-disk paths for Linux and
macOS) written in.

**R64 — monorepo grammar option.** Answered: the grammar can live in the main repo
via the grammar config's optional `path` field (the tree-sitter-typescript
monorepo precedent), with the specific trap named: a nested `.git`
makes the outer repo's commits contain no grammar files (gitlink), so the inner
repo must be removed and the grammar committed into the main repo before the
outer sha can serve as `rev`; regeneration becomes a main-repo commit plus rev
bump, and a separate repo stays the recommended shape if the grammar is ever
published. README's install section gains the monorepo block.

**R65 — the stale-clone triage entry.** The log identified the real failure:
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
comment says so), the make-archive zip, and environment noise. File retention
ruled: shiki-luna.ts **stays** (source of truth for the tmLanguage extraction and
the canonical machine-readable lexical surface).

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
node:22-slim per the ruling, avoiding
musl's occasional misbehavior with native tooling; comment updated with the
rationale. First run after the switch re-pulls and re-warms the npm cache volume;
everything else is unchanged.

**R69 — the Containerfile.** The question exposed a real gap, not just a missing
convention: npx-at-runtime floats the tree-sitter-cli version, and the CLI version
shapes the generated parser, so generation was not reproducible from repo state.
New `tooling/Containerfile` (`FROM node:22-slim`, `npm install -g
tree-sitter-cli@0.26.10`, the version a known-good run resolved);
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

**R71 — the template made obviously invalid.** A "failed to fetch
revision main" error (the branch is master) traced to the archive's extension.toml
template still carrying the original `rev = "main"` placeholder, which the
drop-in of updated tooling files copied over the script-generated sha version, a
plausible-looking placeholder failing later and stranger than an invalid one
would. The template is now explicitly generated-file-marked with deliberately
invalid values (an all-zeros rev, a RUN-the-script repository) so any
non-generated copy fails at first contact with an error pointing at the script;
README triage entry one gains the fetch-revision symptom and the
extension.toml-is-generated-never-copied warning. Immediate recovery: run
regen-grammar.sh, reinstall.

**R72 — the glibc fix.** The container failure diagnosed from the error text:
tree-sitter-cli 0.26's prebuilt binary requires GLIBC_2.39, and node:22-slim is
bookworm-based (Debian 12, glibc 2.36); base image switched to
**node:22-trixie-slim** (Debian 13, glibc 2.41), same node major, same slim
variant, rationale committed as a Containerfile comment with the exact error quoted
for grepability. The base-image change busts the build cache automatically, so the
script's `podman compose build` step rebuilds without intervention; note that
`podman compose up` is not part of this flow, the pipeline is build plus one-shot
`run --rm`.

**R73 — the conditional nuke, reconciling R65 and R70.** The stale-clone error
returned legitimately: an install history that crossed a url change (file:// to
the origin-derived GitHub url), the exact case R65's nuke existed for and R70's
removal re-exposed. Both were half-right; the script now holds the synthesis: it
compares the existing clone's `remote get-url origin` against the url it is about
to write and clears the cache **only on mismatch**, announcing the repoint, so
routine regens preserve the live grammar and url changes self-heal, with the
manual command remaining documented for installs outside the script. Immediate
fix, once: manual nuke plus reinstall.

**R74 — the Shiki grammar rederived from the lexer spec.** A
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
lexer spec through one file. Separately, build/lexer.md is the natural home
for the spec document itself.

**R75 — canonical tree adopted; grammar promoted to the G-rulings; the script
split.** The canonical zip is the working copy now: specs under `docs/`,
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
editor-quiet everywhere, and the real type check relocates to wherever shiki is
actually installed, via an optional documented one-liner
(`const _check: LanguageRegistration = lunaGrammar;`). The extraction pipeline is
unaffected, it parses the object literal, not the types.

**R79 — gated secrets: per-capability secret authority, zero new machinery.** The
hole named, one `reveal` capability opening every secret, closed at the
effect site. Ratified signature (the variadic form, corrected):
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
`secret(raw)`. The original constraint-based proposal (`can` inside
`where`) rejected for the reason now made a stated principle in constraints:
**predicates are functions of the value alone**, frame state is an illegal input,
because checked constraints become facts that ride values across frames and
frame-dependent truth would mint facts that stop being true in transit. The
`can` expression outside constraints deferred on YAGNI; compiler-sandboxing
recorded as latent (restricted root grant + the R39 check binds even
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

**R81 — the front-page example rewritten: fromJson plus match.** "A taste" now
reads a JSON config through the composition idiom (`fromJson(readAll(fd) as
json)`, bare-propagating in the errorable main) and dispatches on the result with
a four-arm `match` demonstrating the ratified semantics, type pattern with binder
(`table t`), `where` guards refining structure, a literal `undefined` arm for the
missing key, and `_` for chosen exhaustiveness, first-match-wins noted in prose.
The shapes-on-display note rewritten to name what the example now shows. En
route, one drift fixed, the old example's postfix `if arguments.empty()` lacked
the parentheses the `if` grammar requires (R46's desugar is `expr if (c);`).
The ask to match on the table structure surfaced a real gap now flagged
in match §12: structural table patterns in arms would slot in syntactically via
the R35 grammar but not semantically, destructuring's absent-key-binds-undefined
rule would make `['host' => h]` match hostless tables, so arms need a presence
rule before it lands, and the guard form is the recorded idiom meanwhile. The
canonical snippet regenerated.

**R82 — the R81 flag retracted; shape patterns were already ruled; the front page
gets them.** A challenge forced the review R81 skipped, and match.md §4
("Table and shape patterns") had ruled everything the flag worried about, all
along: presence semantics for named keys (§4.3's one deliberate difference, with
the extract-vs-test duality stated verbatim), keyed-partial with extras ignored,
unbounded nesting with clean fall-to-next-arm, literals and typed binders in
value position, as-patterns deliberately omitted. Flag retracted in §12 with the
honest reason, raised without reviewing the very spec it flagged. The forced
review paid twice: §4's summary line still said sub-patterns may be "`@T`
(type-tested)", pre-R27 drift (F14 ruled `@int` a compile error, type patterns
are bare types), fixed; and §4.3's "noted in both specs" claim was half-true,
the destructuring side's cross-note now exists. One nuance kept and flagged
rather than silently changed: the "for lists, partial works the same way" reading
diverges from §4's positional-exact rule, where prefix matching is spelled
explicitly (`[a, b, ..._]`), strictly more expressive since exactly-two remains
sayable; standing rule kept pending override. The front page upgraded to the
structural arms the example always wanted (`['host' => string h, 'port' => int
p]`), the shapes note rewritten, the snippet regenerated.

**R83 — publish-grammar.sh survives no-op regeneration.** Diagnosed on a fresh
checkout: `set -e` plus an empty commit, generation reproduced the already-committed
parser byte-for-byte (pinned CLI doing its job), `git commit` exited nonzero on the
empty stage, and the script died **before the extension.toml write**, which is
precisely what a fresh checkout needs even when the grammar is a no-op. Both commit
steps now guard on `git diff --cached --quiet` and announce the no-op; the
extension.toml regeneration is reached unconditionally, which also self-heals the
committed placeholder that full-archive syncs reintroduce (the R71 hazard in sync
form). Secondary note: the retired R75 monolith `regen-grammar.sh` was still being
run; delete it, the pair is `generate-grammar.sh` then `publish-grammar.sh`.

**R84 — the `const` "runtime seal" tension in variables.md resolved.** Three
passages read as a contradiction: §3 called `const` immutability "not a runtime
seal," yet the same section and §7's error table said a mutation "raises a
table-protocol violation at runtime," and §3 also described const-binding as
"sealing the copy." The conflict was wording, not design. `const` is a
**compile-time property of the binding** (tables §5.2, Amendment A); its runtime arm
is not a `const` flag being tested but the value already sitting in its
**permanently-immutable representation** — frozen storage with no mutation machinery
— so a dynamic-path write meets an ordinary table-protocol violation and the value
still never changes. "Not a runtime seal" is sharpened to "not a *revocable*
runtime seal," explicitly separated from the removed `freeze`/`thaw` machinery and
the program-settable `close`/`neverOpen` growth seals; "seals the copy" reworded to
"makes that copy deeply immutable"; §7's arm reworded to "on a dynamic path."
Swept: variables.md §3 (two paragraphs) and §7. The sweep was local — no other file
asserted a const *runtime* seal, and concurrency's "deep-frozen `const`, shared by
reference" and tables' "the const seal" already use freeze/seal in the compile-time
sense.

**R85 — comment syntax stays `//` + `/* */`; the `#`-comment switch rejected; shebang
and empty-regex ruled.** The open question: `//` names both a line comment and an empty
regex literal, indistinguishable in isolation. The proposal weighed was to retire `//` as a
comment in favor of `#` (freeing `//` for the empty regex, and buying `#!` shebang), which
would force attributes off `#[`. Rejected, for three reasons. **First, it targets the wrong
cost.** The genuinely hard part of the `/` overload is regex-vs-division, which needs
previous-significant-token lexer state (lexer F2), and `/*` block comments keep `/`
three-way regardless; dropping `//`-as-comment removes only the cheapest, already-settled
sub-case and leaves the state machine intact. **Second, the stated blocker was false.** `#`
line comments and `#[…]` attributes coexist lexically by the same attempt-order lever §8
already uses for `//` over `/*` (try `#[` before the `#`-comment), so attributes would need
no new spelling; the real objection is aesthetic (no established language pairs `#` comments
with `#[]` attributes, and `#[` becomes a jarring exception to "`#` starts a comment") and
not worth a surface change. **Third, the price is a 59-file, 4-grammar sweep**
(tree-sitter, VSCode, Zed, Shiki all encode `//`) against settled syntax, for a marginal
gain. The `//`-empty-regex clash was already resolved the right way (F2, §8: the comment
wins); this ruling makes the consequence explicit — an **empty pattern is `/(?:)/`** (an
explicit empty group, RE2-accepted, the spelling JavaScript adopted for exactly this
reason) or `regex("")`, never `//`. The two independent wins the proposal bundled are taken
**without** the switch: a **first-line `#!` is a shebang**, recognized only at byte offset 0
and only as `#!` (a bare `#` elsewhere stays a lex error, so `#[` and the shebang are the
sole uses of `#`), giving a directly-executable `.luna` script; and the empty-regex hatch
above. Swept: regex.md §2 (empty-pattern note), lexer.md §2 (shebang token + rule), §5 (the
bare-`#` note now excepts the shebang), §8 (shebang tried first, only at offset 0), and F2
(comment-wins sharpened, `/(?:)/` named, the rejected `#` path cited). **Companion work,
deferred to its own ruling:** the lexer cites a *`lexical-structure §1–§3`* authority for
encoding, identifiers, and comments that **has no file**; a new `comments.md` will become
the comment authority and retire the dead citation, and the **block-comment nesting**
question (currently non-nesting, G3) is reopened there — both out of scope here.

**R86 — `lexical-structure.md` created; the dangling encoding/identifier/comment authority
resolved; block-comment nesting deferred.** R85 deferred the comment authority to "a new
`comments.md`"; on inspection the lexer's dead citations were to *`lexical-structure §1–§3`*
and spanned **three** topics — encoding (§1), identifiers (§2), comments (§3) — so a
comments-only file would have left §1 and §2 dangling. The broader file is the right fix and
is what R85's forward reference should have named. `lexical-structure.md` now exists with
those exact three sections, so every `lexical-structure §N` citation in lexer.md resolves
unchanged (no lexer edits needed). Its content **consolidates already-ruled facts** rather
than inventing: UTF-8 required and checked at ingress (string-representation §8's discipline),
ASCII-only identifiers with no Unicode and hyphen = subtraction (G6, functions §5.6, keywords
§6), `_` the discard (wildcard spec), predeclared ≠ reserved (keywords §5), `//` plus
non-nesting `/* */` with documentation carried by attributes (G3), and the offset-0 `#!`
shebang (R85). Three small decisions were made where the corpus was silent: a **UTF-8 BOM is
forbidden** (a leading U+FEFF is an ordinary codepoint, not stripped, hence a lex error,
which keeps byte offset 0 meaningful for the shebang); **casing carries no semantics** (the
compiler enforces no capitalization, consistent with shadowable predeclared names — the
builtin error-type *convention* stays open, keywords §6); and the **regex/comment invariant**
is stated outright (§3.1): because `//` and `/*` are decided on the two-char prefix before the
regex-vs-division rule (lexer F2), a `regex` literal can never begin with `/` or `*`, so
`/**/` is an empty block comment and the two forms never contend. Block-comment **nesting** is
recorded as **deferred, not rejected** (§3), by decision — cheap to build (a depth counter)
but it changes the failure mode, and the need has not arisen. Swept: `docs/index.md` gained a
*Lexical structure* row and a *Lexer* row (the latter was missing from the map entirely).

**R87 — a type in a pattern is written after `:`; a bare identifier is always a binding.**
The corpus spelled a match arm's typed position prefix, `int n`. Two spellings were argued, and
readability decided neither. Builtin type names are ordinary **shadowable identifiers** (keywords §5,
modules §7), so under the prefix spelling a bare pattern position could be a type test, a binding, or
(via `nan`) a value test, and which one it was depended on **name resolution**: `match (x) { list =>
... }` type-tests while `match (x) { rows => ... }` binds, and exporting a `const cursor = constraint
{...}` upstream silently converts every `cursor =>` binding arm downstream into a type test. That is
Rust's const-pattern hazard without Rust's capitalization convention to survive it, and it is safe by
discipline, which the first commitment forbids. Marking the **type** rather than the binder makes a
pattern's kind decidable by syntax alone with one token of lookahead, and no declaration anywhere can
change what an arm means. Two alternatives were argued and dropped. **(a) Bare identifier = type
test, binders colon'd** (`x: any` to bind anything): it breaks match §4's own claim that "a match
pattern *is* a destructuring pattern with literals and type tests added", since `let ['x' => x] = tab`
binds while `match (tab) { ['x' => x] => ... }` would type-test, the same syntax meaning opposite
things in the two constructs §4.3 exists to reconcile; it is lossy, because a typed binder *supplies*
its type (as §2, §4), so `['name' => n: any]` on a `@person` source binds `any` where `['name' => n]`
binds `string` (destructuring §5); and it taxes the commonest atom (bare binders outnumber anonymous
type tests roughly 9:4 in the spec's own examples) to shorten the rarest. **(b) `is` instead of `:`**
(`_ is int`, `n is int`): it costs four exceptions where `:` costs one. `is`'s left operand is a
**value** in every existing use (is §2) and would become a binder; `x is int` is *already* a legal
open-ended-match arm (match §6), so deleting a scrutinee would silently reinterpret every arm, where
`x: int` is not an expression and fails to parse; the pattern `is` and the guard `is` would then
collide inside one arm (`x is int where x is int`), destroying §3's pedagogy; and it inverts is.md's
headline, "`is` never narrows," restated in six other files. The `:` costs exactly one exception, and
a small one: in a pattern `:` tests where in a declaration it asserts. Nothing is hidden by the reuse,
because a pattern's whole job is to be a predicate, so the arm's selection **is** the check, and the
invariant the annotation protects, *an annotation never lies*, holds in both positions (as §4).

The grammar is now stated, as match **§2.1**: `_`, `_: T`, `name`, `name: T`, literal, range, shape,
`{tag pattern}`, alternation, parentheses. A bare binder **inherits** the value's type; a typed binder
**tests and supplies** one; `_: T` tests and discards, and is `_` in *binding* position carrying an
annotation (wildcard §5's `fn (_, index)`, now typed), not `_` in type position (wildcard §7). The
semantics, match **§2.2**: `name: T` is `value is T` (total, never panics) followed by a fresh binding
of static type `T` (as §7). One emitted check, `is`'s cost per kind (type §7.1), the bind free. Two
consequences fall out of `as`'s existing rules rather than new machinery: a **disjoint** type is a
compile error, not a dead arm (as §5), and a **supertype** is irrefutable with its check elided, so
`n: any` is legal and lossy, which is exactly why the bare binder exists. The `|` collision
**dissolves** rather than being ruled: because a type occurs only after a `:`, `|` is the union
operator inside a type and the alternation separator outside one, and the two never share a position.
`n: int | string` is a union, `1 | 2` an alternation. **R28's pattern-grouping ruling is retracted**,
`(int | string) n` is gone, and parentheses survive only for the inverse, a typed pattern used as an
alternative, `(_: string) | 5`.

Three positions take patterns, two restricted. A **match arm** takes the full grammar. A **`catch`
clause** takes a **parenthesized** binder pattern: `catch (e)` inherits the root `error` and so
catches everything, `catch (e: diskError)` narrows, `catch (_: diskError)` narrows while saying out
loud that the value is discarded, `catch (e: ioError | typeError)` unions, and `catch (p: panic)`
replaces `catch panic p`. `catch error` and `catch someType` are **retired**, a bare name in a binder
position is a binding, never a type. The parentheses are new and required: they make `catch` the same
shape as every other clause head (`if (c)`, `while (c)`, `foreach (…)`, `match (x)`), where it was the
sole exception, and they resolve a real ambiguity the typed binder introduces, since `error` is both
the declaration keyword and the root type (errors §3), so an unparenthesized `catch _: error { ... }`
puts a `{` directly after a type where `error { ... }` is also the error-declaration form. The `)`
closes the type, which is also why `{` is absent from match §2.1's type-terminator set. A
**`constraint` body** takes a typed binder and nothing else:
`constraint { i: int where i >= 0 && i <= 255 }`. `int as i` is retired, which was the one place `as`
was a binder; `as` is now purely checked narrowing and the import alias, and match §4.2's as-pattern
rationale ("`as` is a checked-narrowing operator, not a binder") becomes true as written. A constraint
body is therefore an arm's `pattern where guard` with the pattern narrowed to one typed binder and the
body dropped, diverging in exactly one place: a constraint admits several `where` clauses because it
*reports which one failed*, and an arm takes one because a failing arm has nothing to report.

Prerequisite, and a standing gap it closed: **`nan` and `inf` are now keywords** (lexer §3), joining
`true`/`false`/`null`/`undefined` as the six **value keywords**. They had **no token at all**, absent
from lexer.md, keywords.md, and every declaration, while match §7 was built on
`match (x) { nan => ... }`; under the new rule a bare `IDENT` binds, so that arm would have bound a
fresh `nan`. They are **keywords by lexing and literals by role** (keywords §4): not literal tokens,
which are regexes over open-ended lexeme sets, and not predeclared identifiers, which would bind. The
spelling is lowercase `nan`/`inf` rather than `NaN`/`Infinity`, matching the other five and leaving
the reserved set with no capitalized member; `-inf` is `MINUS KW_INF`, and there being no unary `+`,
positive infinity is `inf`. Of the six, only `undefined` restricts what a program may *do* with it
(compare, never conjure); `nan` and `inf` are ordinary producible doubles. Named constants generally
are **not** patterns: `where c == exitCode.success` is the spelling, Rust's const-pattern rule
deliberately rejected. Type names stay shadowable identifiers, which is now harmless, because they
never appear bare in a pattern.

The forced sweep paid four times. **type.md §7** matched a bare `@stringBuilder` and then claimed it
"narrows `x`", using `x` in the arm body: a type test with no binder narrows nothing (as §7), so the
example was unsound; it is now `b: @stringBuilder => b->stringBuilder.append("!")`, with the general
rule stated, **the binder is what narrows**. **overview/types.md** said applying `@` to something
already a type "is an error", unqualified, contradicting type.md §1.1: it is an error in **type**
position only, and in value position `@int` is reflection yielding `type`, which is precisely why the
retired `@int n` spelling was self-refuting (R27's F14). **reflection.md**'s comptime sketch matched
`kind(t)`, a `TypeKind` **enum**, with bare `table =>` / `enumType =>` arms rather than the `{table}` /
`{enumType}` variant patterns enum §4 requires. **tables.md** called `list` a protocol ("a `@list`
match pattern"); `list` is a type, written bare after the `:`.

Swept: `match.md` (§2 rewritten, §2.1 and §2.2 added, §3, §4, §4.2, §4.3, §5; three internal section
refs were off by one, §6→§7, §7→§8, §8→§9, all consistent with §6 having been inserted after they were
written); `associativity.md` (pattern-grouping retracted, pattern position named as the third grammar,
the two stale "match §1" refs, `where`'s second home); `constraints.md` (§1, §3, §6, §10); `errors.md`
(§2, §5.2, §6, §8.2, §8.3 rewritten, §9); `keywords.md` (`as`, `where`, `panic`, `_`, the value
keywords, "constraints §2"→§1); `lexer.md` (§3, §4); `overview/types.md`; `type.md` (§1, §7, §7.1);
`as.md` (§4, §7); `wildcard.md` (§4, §7); `destructuring.md` (§5); `is.md`; `any.md`; `reflection.md`;
`enum.md`; `tables.md`; `int.md`; `double.md`; `bytes.md`; `json.md`; `serialization.md`;
`protocols.md`; `io-errors.md`; `io.md`, `csv.md`, `yaml.md`, `xml.md` (constraint bodies);
`internal-representation-of-variables.md`; `index.md` and `tooling/a-taste-of-luna.luna` (the
front-page example).

Not changed, deliberately: **function-type patterns.** `_: fn`, `n: fn`, and `n: fn (int): string` are
unspecified, and type §7.1's kind table has no row for them. The signature case is the one `is` that
is not O(1), and it collides with `as`'s per-call deferral, whose rationale in as.md §5.1 and
functions.md §3.2, *"a function's conformance is observable only when it runs"*, is **wrong**: function
types are not erased (overview/types), so the runtime value carries its signature. `as` defers because
it permits a claim that is not a subtype relation, which functions.md §3.2 says out loud ("allowed
*optimistically*"), not because conformance is unobservable. Flagged in type.md §7.1; it is the next
ruling. Also unchanged: the builtin error types' casing (`OverflowError`, `ArityError`, `IOError` vs
`typeError`, `ioError`), already flagged at keywords §6; and the tree-sitter grammar, which is
highlighting-only and carries no pattern rules.

**R88 — function narrowing: obligations eagerly, values per call; `is` on a signature is a
pairwise table.** The rationale the corpus gave for deferring a function `as` to each call was
false. It said a function's conformance to `fn (A): R` is "observable only when it runs" (as §5.1,
functions §3.2), but **function types are not erased**: the typeid is *signature plus errorability
and nothing else* (functions §3), so the real parameters, the real result, and whether the function
throws are all in hand the moment the `as` runs. Deferral was never forced by ignorance. `as` defers
because **`as` is licensed to assert a claim that is not a subtype relation**, which functions §3.2
already said out loud ("allowed *optimistically*") three lines below the false rationale, and such a
claim can only be tested against the calls that occur. The license has exactly one boundary, and it
is the ruling: **`as` may be optimistic about *values*, never about *obligations*.** An argument type
and a result type are beliefs about the calls that will occur, and being wrong is caught at the call.
Errorability is a caller obligation: you cannot believe a function will not throw and be checked
afterwards, because the check would arrive after the throw. A `&` parameter is an obligation too, a
write channel, and the calls that occur bound what a callee reads, never what it writes back. So
**kind, errorability, reference positions, and hopelessness are eager**; **argument and result
conformance are deferred**. Locality tightens with it: hopelessness is detectable at the `as` even
through an erased `fn`, since the typeid knows the real signature, so it panics **there** rather than
at some first call that may never come; the "an optimistic narrowing never called never panics" grace
survives, for optimistic narrowings only.

**Hopelessness is a disjunction, and the old gloss was wrong twice.** functions §3.2 made a
statically-known narrowing a compile error only when parameters *and* results were disjoint. That let
`fn (): int as fn (): string` through (a zero-parameter function has no disjoint parameters at all)
and `fn (int | string): R as fn (double): R` too, both of which panic on every call. **Or**, not
**and**: a narrowing is hopeless when *any* position is, a disjoint parameter, a disjoint result, a
differing `&` position, or a **required** parameter of the real function beyond the claimed arity
(§3.3.1's defaults decide it). The gloss "no function could inhabit both" is separately wrong: a
function value has exactly one signature typeid, so *any* two distinct signatures are value-set
disjoint, including the `fn (int): int` / `fn (int): int | string` pair the optimistic rule exists to
permit. The criterion is "no **call** could satisfy," never "no value could inhabit." New functions
§3.2.1 carries the narrowing table, headed by the fact that inverts most first guesses: **widening a
parameter narrows the function type**, so `fn (int | string): R as fn (int): R` and `... as fn` are
free widenings that can never fail, while the two forms that look harmless, widening a claimed
parameter and narrowing a claimed result, are the real narrowings and are exactly the ones that defer.

**`is` on a signature needs a third shape, and it is the one value-representation §4.2 already
names.** R87 made a pattern's typed binder `is T`, total and non-panicking, so `match (f) { g: fn
(int): string => ... }` must **decide** conformance rather than assert it. The wildcard ladder is a
tree (`fn (A): R <: fn <: fn!`, with `fn (A): R! <: fn!` alone), so function typeids occupy one
contiguous region with the non-errorable signatures as a prefix and `is fn` / `is fn!` stay interval
checks. The **leaves** are a DAG: signature subtyping is structural and contravariant in the
parameters, so `fn (int | string): (int | string)` is a subtype of both `fn (int): (int | string)` and
`fn (int | string): any`, which are incomparable, exactly the multiple-supertype shape §4.2 says
laminar intervals cannot encode. **The resemblance to errors is superficial and is refused outright**:
errors carry subtype rules too, but they are *single inheritance by construction*, a nominal tree
declared node by node, which is why the interval numbering serves them to the leaves; nothing about a
function type is declared, signatures being interned structurally, so there is no tree to number.
Signatures therefore take the other escape §4.2's own parenthetical names, a **pairwise table**: the
type universe is closed, function types are interned, so a program's distinct function typeids are a
finite set `F` enumerable at link time, `S <: T` is folded once structurally, and the runtime test is
one indexed load. `F` counts distinct written signatures, not call sites; a thousand of them is 125 KB.
Rejected: **typeid identity**, O(1) and trivially wrong, since it contradicts is §2's own definition
("is `x`'s current type a subtype of `T`") and would make `match` *weaker* than `as`, failing to match
an `f: fn (int | string): int` genuinely usable as a `fn (int): int`.

**The payoff, and its two boundaries** (new match §2.3). `as` claims and pays per call; `is` decides.
When `g: fn (int): string` matches, `real <: claim` holds, so the argument is within the claimed
parameter which the real parameter accepts by contravariance, the result is within the real result
which is within the claim by covariance, arity and errorability agree, and `&` positions are identical:
**every per-call signature check folds away.** A bare `g: fn` buys nothing, there being no signature in
the type to decide, so calls through it keep all four. And **no pattern discharges the capability
check**: a function's requirement set rides the value, not the typeid, so `is` never inspected it, and
a call through any `fn`-typed slot still verifies *requirement set ⊆ the executing frame's grant*, one
bitmask compare, panicking on shortfall. Narrowing a signature is a claim about **data**; authority is
not data. Capabilities and comptime-eligibility are confirmed **erased from the typeid** and borne on
the value, reflection-visible in principle; the laundering worry closes as capabilities §3.1's
**laundering theorem** already stated, a closure travels through capability-free code as inert data,
never as exercisable authority, and the shortfall panic fires at the **indirect call**, before the
body runs, never at the `use` inside it. Possession of the value is not possession of the grant.

**Two unswept rulings surfaced across four sites, one of them denying the panic this ruling
documents.** **R39** left residue in three places. *functions §3.1* asserted that capability-holding
values "cannot enter a `fn`, `fn!`, or signed slot at all, so every value reaching any function-typed
slot is capability-free": the pre-R39 second-class rule, repealed ninety lines earlier in the same
file. *capabilities §5.1*, which §3.1 **cites** as the authority for "fixed at creation," still said
"after creation, no further check ever runs," "an indirect call needs no check at all, because a
capability-holding value cannot enter a slot," and "no runtime capability state anywhere", all three
false under R39, and the second is precisely the dynamic frontier this ruling relies on. *capabilities
§8* propped the comptime sandbox on the same repealed rule ("a slot could not hold a capability-holding
value even at runtime"); the sandbox conclusion never depended on it, and R39's own argument is
stronger, at comptime the executing frame's grant is empty, so a requirement-carrying value invoked
there fails the dynamic check **vacuously**. The check's ordering is now stated where it belongs
(capabilities §5.1): a shortfall panics **before the callee's body begins**, so it refuses an
unauthorized call at the boundary rather than halting an effect in progress, which is what makes the
laundering theorem a safety property and not merely a detection one. **R43** left residue in one place.
*functions §5.2* was headed "It lives in the function type"
and said "the eligibility bit is part of the `fn` type," contradicting §3's post-R43 "comptime
eligibility left the typeid, joining capabilities on the value." R43 is also the only sound reading:
were eligibility in the typeid, `f as comptime fn (int): int` would be an optimistic claim about an
obligation, which this ruling forbids; being *derived* from the requirement set, which lives on the
value where `as` cannot reach it, it is un-launderable for free, and it needs no place in the type
because a comptime call requires a statically-known `const` callee in any case (§5.2's own next
sentence), so the compiler always holds the function itself.

**A corpus-wide check for the newly-ruled facts paid a sixth time.** Three specs asserted that *the*
subtype test is the interval check, which value-representation §4.2 has never said (unions decompose)
and which R88 makes doubly false (signatures index a table): `reflection.md`'s `isSubtype` was "the
interval check," `compiler.md` said "subtype tests compile to the two integer comparisons of the
interval check," both now dispatch by the shape of the target. Two specs claimed **`is` narrows**,
which is.md §3's headline denies and which R87 §2.2 and as §7 depend on: `exec.md`'s "`is`-narrowing"
and `errors.md` §8.3's "(`is SomeType`, or a following `catch`)" for recovering a concrete subtype;
both now name `as`, a typed `catch` clause, or a match arm as the narrowing form, with `is` as the
boolean gate. `is.md` placed `isSubtype` "in comptime reflection" two lines after citing reflection
**§3.1, the runtime tier**, and reflection.md and type.md both agree it is runtime; is.md is corrected.
And `pipeline.md` §5.2's **heading** still read "Stages are effect-free by construction" while its own
body, swept at R39, said that claim "rested on the repealed second-class rule" and weakened to the
ambient check; the heading is retitled. Nothing in the corpus now contradicts either ruling.

Swept: `functions.md` (§3.1's R39 residue, §3.2 rewritten, §3.2.1 added, §5.2 rewritten);
`as.md` (§5.1 rewritten and retitled); `capabilities.md` (§5.1 and §8, both R39 residue);
`internal-representation-of-variables.md` (§4.2 gains the function-signature shape);
`is.md` (§2's dispatch is by shape, not "two-tier"; §4 and §5's `isSubtype` tier);
`type.md` (§7.1 gains the wildcard-callable and function-signature rows; its R87 "open" flag retired);
`match.md` (§2 wording, new §2.3); `reflection.md` (`isSubtype`'s dispatch); `compiler.md` (typetable
emission); `exec.md` and `errors.md` (`is` does not narrow); `pipeline.md` (§5.2's heading).

Not changed, and flagged: **reflection has no named query for a function's requirement set.**
functions §3 says the fact is "readable at any boundary that needs it" and this ruling leans on that
for the capability story, but reflection §3's runtime tier lists `typeName`, `kind`, `isNullable` and
nothing else. Also unchanged: the builtin error types' casing (keywords §6), still open since R87.

**R89 — spread: one spec, and sequence contexts require a `list`.** R36 opened by saying the
corpus cited "a spread spec that did not exist" and wrote `expressions/spread.md`. The spec
did exist, in `bindings/`, which is why every citation said "spread spec" and none said a
path. So R36 founded a **second** spread spec and swept neither the first nor the two `Spread`
rows in `index.md`'s single table, leaving `bindings/spread.md` still listing as **open** the
three questions R36 had just ruled (streams spread, deep spread, call-site spread) and the two
files disagreeing about what an integer key does. Partial application, the one defect this
process exists to catch, and it hid a semantic contradiction for eighty-odd rulings. One file
now, `expressions/spread.md`; `bindings/spread.md` is deleted, its fold-with-running-index and
worked example, list-ness-as-emergent, and comptime spread merged in (none of it duplicated
elsewhere; its `{...}` gloss corrected to the bracketed table literal). The keeper is the
expressions one because **spread contributes values**: destructuring is the pattern-side
counterpart and keeps `bindings/`. Splitting one token's two halves across two directories is
what let them drift.

**The ruling that organizes the file: a table literal is the only spread context with a key
slot.** Keys survive it, under the fold. **Sequence contexts**, call arguments, command argv,
and interpolation, have no key slot, so they require a `list`. This derives all four contexts
from one distinction and adds no mechanism. **Argument spread therefore requires a `list`, and
spread contributes no rule to enforce it**: `f(...xs)` demands exactly what `someFn(xs)`
demands of a `list` parameter, because narrowing is never implicit (tables §2.3). A static
`table` is a **compile error**, the `someFn(tab)` error tables §2.3 already prints; a value
reached dynamically takes the `as list` check at the call, `typeError` on failure, the same
panic `as list` raises. Rejected: **take the values**, spreading whatever the table holds in
iteration order. It is the simpler sentence and it is what a spread token naively suggests, but
it would make `...` **the one implicit `table → list` narrowing in the language**: `someFn(tab)`
is a compile error while `someFn(...tab)` would silently reindex, so writing the token would
*loosen* the check it looks like it tightens. tables §2.3 has already ruled the two spellings and
each says what it means, `t as list` **asserts** that a non-list here is a bug, `t.values()`
**produces** a list from any table, and its own words are "nothing reindexes silently." Silent
reindexing is the bug class; ergonomics is not the axis. The consequence is recorded rather than
apologized for: `f(...t)` is deliberately stricter than `[...t]`, which reindexes `[5=>50]` to
`[50]` without complaint, because a literal is *building* a table, where integer keys carry no
positional contract, and a parameter list has one.

**Three further defects in R36's own text.** *Arity was wrong in the surplus direction.* §4 said
"panic on mismatch," but functions §3.3 **drops** surplus arguments (the rule that lets a callback
declare fewer parameters than the protocol supplies) and raises `ArityError` only on a deficit. A
spread of ten elements into a two-parameter function is legal, exactly as the written ten-argument
call is; corrected. *The rationale for positional-only spread was false.* §4 justified it with
"named arguments do not exist," while table-api's notation defines a keyword-only tail after a
`*,` marker and `merge`, `diff`, `intersect`, and `replace` all use one. Replaced with the reason
that is true and is this ruling's principle: an argument list has no key slot. *Spread is not the
materialization.* `collect(s)` **preserves** a key-value stream's integer keys (stream-api) while
the fold reindexes them, so the equation was false for exactly the streams it mattered for;
`[...s]` is `[...collect(s)]`, and `values(s)` for a values-only stream. Streams flow into
sequence contexts and into variadics **values-only**, that being the stream analogue of a `list`
(stream §1.1); a key-value stream needs `.values()` first, precisely as a keyed table does.

**`"$...xs"` does not lex, and is deleted.** lexer §6's `INTERP_IDENT` is
`\$[A-Za-z_][A-Za-z0-9_]*`, which `$.` cannot match, and the spread splice is defined only inside
`${ }` (`INTERP_OPEN` + `SPREAD`). `"${...xs}"` is the one spelling and had to be, since bare
`$name` is `DQ_STRING`-only while spread must also work in command literals, where no bare form
exists. R36's rejection of a panic-unless-every-element-is-a-`string` **rendering** rule stands
untouched and is now visibly a different axis from this one: that rule is about elements, the
`list` requirement is about shape.

**Stale citations, two of them R36's own and one inherited.** `operators §0.1` is the `.` token;
the no-`+`-on-tables rationale is `operators §0.4` ("What some tokens deliberately do NOT do"),
twice. `command §4` is the pipeline operator; `${...flags}` is `command §3`, twice, and `lexer.md`
cited §3 correctly, so the corpus disagreed with itself. `variables §5.2` is the `copy` operator;
copy-on-write value semantics are `tables §4`. And `flatten`'s cited signature had shed two of its
five parameters, restored, the fuller signature strengthening the very argument it was cited for:
four policy knobs (`depth`, `preserveKeys`, `onNoGet`, `asStream`) do not fit in a token, which is
why depth is a function and `[......t]` is not a thing.

Swept: `expressions/spread.md` (rewritten, absorbing the deleted file); `bindings/spread.md`
(deleted); `index.md` (its two `Spread` rows, in one table, collapsed to one);
`operators.md` (the `...x` catalogue row, which said "spreads a table/stream" of argument
lists too, and had no note of the third position); `table-api.md` (`merge`, R90). Verified
unmoved by the section renumbering: `command.md`'s and `lexer.md`'s citations of **spread §5**
(interpolation and command literals), and `await.md`'s **spread §2** for
`[...await promises]`, which is a values-only stream and so needs no `.values()`.

**R90 — `merge` appends: `preserveKeys` defaults to `false`.** `table-api.md`'s `merge` was
internally contradictory: its prose, "Appends `tabs` onto `tab` in order," says integer keys
renumber, while its signature defaulted `preserveKeys: bool = true`, which keeps them, so they
collide with `tab`'s and overwrite. Both cannot hold, and under the default the *documented*
behavior was the one you could not get: `[10, 20].merge([30])` was `[0=>30, 1=>20]`, not
`[10, 20, 30]`. The prose is right and the default was the error. Forced independently by R89:
spread §1.3 states `[...a, ...b]` is `a.merge(b)`, and operators §0.4 rests the whole
no-`+`-on-tables ruling on spread "already expressing merge" — under the old default that equation
was false in its integer-key half, and concatenating two lists silently overwrote. Three claims,
one of them with no argument behind it; the default gives way. `preserveKeys: true` survives as
the *other* operation, layering `tabs`' entries at their own integer keys so they collide, it is
simply not what merge means. The generalization is worth naming because it decides the rest of
that audit: **combiners reindex by default; per-element reshapers preserve by default.** `flatten`
already defaults `false`, "since keys collide across levels"; `merge` has the identical collision,
across sources rather than levels, and defaulted the other way.

Deliberately not swept, and now flagged: the **rest of `table-api.md`.** `preserveKeys` is a
per-method flag defined nowhere globally and still defaults `true` on `chunk`, `partition`,
`random`, and others; and the notation section leans on three parameter forms `functions.md` §3.3
does not define — the `...tabs` variadic, the `*,` keyword-only tail, and the `name?` optional
parameter. That is the table-api revision, not this one.

**R91 — the built-in table protocol is retired; the catalogue is built-in free functions.**
The corpus held three incompatible accounts of how a table operation is called: table-api §1
said "you may call `tab.map(...)`" — a dot-reached protocol member, impossible since functions
§3.4 ruled that `x.name(` is UFCS and that "UFCS never searches protocol meta pools"; tables
§3.3 said the catalogue lives in meta space behind bare `->` (`tab->map()`); and tables §4's
own examples write `myTable.sort()` and `&myTable.pop()`. Meanwhile `map` existed twice — a
stream free function and a table protocol member with different signatures — against §3.4's
own rule, "one name, one signature, first parameter is the receiver," whose tail flagged
exactly this std-shaping work. The ruling: **there is no built-in protocol.** Every operation
is a built-in free function, receiver first, reached by call or UFCS — the account the stream
API already followed and the examples already assumed. Free availability: no import, no
module; a local binding shadows the built-in, and a UFCS call through the shadow is a compile
error (not callable) — loud, and accepted. Dead with the protocol: bare `->` (the operator now
reaches only *named* user protocols), views §7.2's special case (nothing left to exclude from
`@@tab`), the "represented virtually" implementation note, and the members-not-assignable
rule. `&`-write-back is untouched: `&` at the call site assigns the return value back (tables
§4), however the call is spelled. Swept: `table-api.md` (stubbed to the successor map plus
deferrals), `iterable-functions.md` / `indexable-functions.md` (created, R92), `index.md`.
The rest of the corpus is a flagged remainder (Still open).

**R92 — one vocabulary over `iterable`; whole-input retention takes a `table`.** `iterable`
is a built-in type, the union `table | stream`, written bare like `list`. The catalogue
splits on one test: an operation whose semantics need only **ordered traversal of
`key => value` pairs** — lazy, short-circuiting, or bounded-memory — takes `iterable`
(`iterable-functions.md`: 47 functions, including `take` / `skip` / `takeWhile` / `dropWhile`,
formerly stream-only, now on tables too); an operation needing keyed access, table state,
positional mutation, **or the entire input at once** takes `table`
(`indexable-functions.md`: 22). The retention rule is the load-bearing half: `sort`,
`reverse`, `shuffle`, `groupBy`, `partition` cannot emit their first element before seeing
their last, so on a stream they would be a *silent* collect — hiding exactly the O(n) the
explicit bridge exists to show — and streams may be infinite, so the type keeps unbounded
input out of unbounded buffering; the stream spelling is `s.collect().sort()`. `random` is
the deliberate exception (reservoir sampling, O(num), single pass) and stays iterable;
`reduce` is the deliberate escape hatch for building retained data by hand. Output kind
follows the primary operand (first operand, for constructor-style `combine`): table in →
table out, stream in → stream out, and the `asStream` flag is retired from every signature.
Rejected alternative: an output-kind parameter for "merge two tables, get a stream" —
unnecessary, because `asStream(tab)` is O(1) and lazy, so bridging the primary
(`a.asStream().merge(b)`) already spells it with no new mechanism; the bridges' costs match
their explicitness (`asStream` free, `collect` the visible O(n), both total on `iterable`
and identity on their own kind). A stream passed as *any* argument is **taken**, extending
the `|>` transfer discipline (stream §7). `onNoGet` / `onNoSet` are dropped from every
signature and deferred — with `TableReadViolationError` / `TableMutationViolationError` and
the `canGet` / `canSet` grant semantics — to the protocol redesign; the deferrals are
recorded in the `table-api.md` stub. R90's audit, `preserveKeys` half: **discharged** — the
surviving defaults follow R90's rule (combiners `merge` / `flatten` reindex; reshapers and
pickers `chunk` / `partition` / `random` / `slice` preserve); its parameter-forms half stays
open below.

**R93 — every stream is keyed; the name mergers.** The values-only / key-value stream
dichotomy (stream spec §1.1) is erased: every iterable yields `key => value` pairs, and a
generator yielding bare values has **implicit keys** `0, 1, 2…` — a stream is to a table
exactly what a list is to a keyed table, and implicit keys behave precisely as list keys
(preserved by per-element functions, hence sparse after `filter`; reindexed by `values`).
The symmetry makes every key-facing function (`keys`, `keyOf`, `flip`, `keyFirst` /
`keyLast`, mode `{keys}`) total over `iterable`, and closes stream-api §9's key-aware
`peek` / `first` question: `peek` / `first` return values, `keyFirst` / `keyLast` return
keys, `foreach (k => v)` gives pairs. The mergers, one name per operation: **`empty` →
`isEmpty`** (joins the `isList` / `isConsumed` predicate family; bare `empty` reads as a
verb, which is `clear`'s job); **`concat` → `merge`** (on streams, merge is lazy
concatenation — duplicate string keys flow through in order and resolve at `collect`, last
wins, the same table `merge` would build); **`enumerate` → nothing** (it only made implicit
keys explicit, and they are always present); **the stream `values` collector → `collect`**
(`values` is everywhere the kind-preserving reindexing transform; a stream with undisturbed
implicit keys collects to a `list`, so the old collector's behavior falls out of the one
bridge instead of being a second one). Retired spellings are tabulated in
iterable-functions §3; do not reintroduce.

**R94 — the table→stream bridge is `toStream`, not `asStream`.** Renamed in-session, before
any sweep landed on the old name. `as` is the narrowing operator (as spec): `x as T`
reinterprets a value the expression already has, producing no new one. The bridge *builds* a
new value — a stream adapting the table's elements — and that is the conversion family's
territory, where every member is spelled `to*` and is an ordinary UFCS-reachable function
(conversion spec: `toInt`, `toDouble`, `toBool`, …). `tab.asStream()` claimed a narrowing
that is not happening; `tab.toStream()` says what it does. Two names deliberately keep their
spelling: the retired `asStream: bool` *flag* (R92) stays `asStream` in the log and the
retired-spellings table, since it names a dead thing; and `collect` does **not** become
`toTable`, because it is a consumer, not a conversion — it exhausts its source and retains
O(n), and its name carries that cost (R92's explicitness rule) where `toTable` would read as
free. Swept: `iterable-functions.md` (§1.3, §2.11, and the §3 retired-spellings table, which
now lists the old name); the Still-open grep list (the old corpus's `asStream` — stream-api
§7's function and every `asStream:` flag — now sweeps to `toStream` / the R92 kind rule).

**R95 — `->` is all protocol member access; protocol space is closed; views are retired.**
The protocol model is rebuilt on one move: a protocol's members live in **protocol space**
— per-protocol namespaced, reached only by `->`, and **closed at compile time** (every
proto block fixes its member set at definition) — and element space (`.` / `[]`) is
protocol-free, pure user data. Everything the old model fought follows from ending the
shared space: declared-element-member collisions (old §6.3), the `@P & @Q` disjointness
constraint (old §7.2), the declared-vs-dynamic install machinery (old §5.1–5.2), and the
built-in-name avoidance rule (old §6.4) are all structurally impossible rather than
checked. `->` resolves statically against in-scope protocols: bare `tab->m` when exactly
one declares `m`, qualified `tab->P.m` otherwise, unknown names a compile error — so
`undefined` never masks a typo. The one runtime question left is application (a dynamic
fact): a bare read of an unapplied protocol's member is `undefined` (two absences, one
rule, matching element space), a hard use — write or call — **panics** (a silent no-op
call is Objective-C nil messaging, the failure mode that parses as success; rejected),
and `?->` is the explicit soft form, short-circuiting argument evaluation like `?.`.
This ruling also settles a live contradiction: old protocols §10 said an unapplied reach
panics while §4.4 and views §3.2 said `undefined` — resolved as above (soft read, hard
panic). **Views are retired with the space they navigated**: `->` yields values directly,
chaining rides return values, `self` is the receiver typed `@CurrentProto`, `tab->P.m` is
pure qualification syntax producing no intermediate value, and `@@` (tables only) moves
into protocols §8. The §5.4.2 widening/`&`-alias hole closes by construction: no `.`
write can touch a protocol member, no `->` write can touch element data, and grants
travel with the member, not the binding. Swept: `protocols.md` (rewritten), `views.md`
(stubbed to a replacement map), `index.md`.

**R96 — the proto block is member declarations only: the ladder plus grants.** A protocol
contributes exactly one kind of thing, members, declared
`<const|let|var> [get] [set] name[?]: type [= default]` — three existing mechanisms
composing with their ordinary meanings (binding keyword = mutability, variables §1
including the §1.2 write-once optional; `?` = type; default presence = apply-time
omissibility, the same rule as function parameters, so there is no `required` marker).
The `meta` keyword is **retired**; its old sense (per-table private state) is an
ungranted member, and protocol-level constants fall out of **`const`'s one-binding
rule**: a `const` with a default is bound at *definition* — uniform across tables, a
protocol fact, reachable off the proto value (`person->species`), excluded from
equality, serialization, and initializers, and never per-table overridable (hence **no
virtual dispatch**: fn-typed members cannot be overridden per table); a `const` without
a default takes its one binding from a required apply initializer — a per-table
immutable, the record field. Mutable protocol-level state is inexpressible; shared
mutable state belongs to a task, per the concurrency commitments. **Functions are
`const` fn-typed members** — no separate category: `get` makes one public, ungranted is
a private helper, receiver first by value with caller-side `&`-write-back (the R91
catalogue convention; no `&` parameters). `P->fn` yields the bare function value
(receiver-first, so it composes with `map` directly); `tab->fn` uncalled is a compile
error (a bound method would be a stale snapshot under capture §2.1 or an alias, both
rejected; bake a receiver with an explicit closure). Grants are external-only (a
protocol's own functions always reach their own members), canonically ordered `get set`,
orthogonal (write-only is legal), and **a grant that can never be exercised is a
definition error** (`const … set`; `let set` on a scalar) — likewise **required ⇒
granted is a theorem** (no default + no grant = unbindable = ill-formed). The surfaces
sharpen to one boundary: **per-table `get` members = the equality surface = the
serialization surface; granted per-table members = the initializer surface**;
definition-fixed members are vacuous everywhere; `identityEquality` is retained for
hidden-state types. And ruled here: **fn values do not serialize** (no representation,
no way back, a security hazard) — `toJson` raises `typeError` on a fn in the
serialization surface, with `skipFunctions: true` to omit fn-valued slots instead.
Swept: `protocols.md`.

**R97 — application is pure machinery; initializers are data; requirements auto-apply.**
Custom `apply` functions are **dead**: application runs no user code, ever — it attaches
the protocol and binds its per-table members from initializers or defaults, atomically
(all-or-nothing, no observable partial state). What `apply` bodies did is covered by
constraints (validation), initializers (population), or ordinary factory functions
returning `@P` (everything else — no constructor concept, hence none of the constructor
failure modes: no init-order semantics, no partially built values, no overloads).
Initializers are a named, typed, compile-checked value list — the **apply operator's own
grammar**, deliberately not function named-arguments (that question stays open,
R89/R90 tail) — supplying required members and overriding `let`/`var` defaults;
definition-fixed and ungranted members may not appear. Two forms with a clean split:
the **operator** (`[] apply person(name: "Lucas")`, onto any table expression) is
**never errorable** — under this model a table's element data is *irrelevant* to
application, so the old shape-dependence (§7.4) and its whole errorability tier system
(§7.5–7.6) collapse; the **free function** (`&tab.apply(p, inits)`, R91-style with
write-back) is the dynamic form and carries the model's **single residual error**,
`ApplyError`: missing required initializer, unknown/unbindable initializer key,
initializer value failing type or constraint, or re-application with initializers. Bare
**re-application is a no-op** (idempotent, state kept — which keeps application monotone
and the fact/promise split sound); re-applying **with initializers is an error** (silent
data loss or broken idempotency otherwise). **Requirements**: a proto block may state
`apply otherProto;` — the same keyword because the semantics *are* auto-application
(transitive, idempotent, safe precisely because application is machinery); `require` as
a new keyword was rejected as both costly and a misnomer. The requirement graph is a
**DAG** (modules §2's argument verbatim), and `@B <: @A` falls out: a `@B` provably has
`A` applied. One restriction keeps initializer lists single-protocol: a requirement with
*required* members cannot be auto-applied — the target must have it applied already
(compile error in the operator form naming the missing requirement). Swept:
`protocols.md`; resolves old §11's apply-reach question (a built-in free function, per
R91) and its applied-check question (the closed space plus the O(1) tag test).

**R98 — the element-permission model is deleted; the R92 deferrals resolve by deletion.**
With protocols out of element space (R95) and grants compile-checked (R96), per-key
element permissions have nothing left to govern: a bare table was always fully
accessible to its holder, and protocol members answer to static grants. Deleted, not
deferred: **`onNoGet` / `onNoSet`** (bulk operations traverse element space, which
carries no permissions — the enums return to no signature, ever),
**`TableReadViolationError` / `TableMutationViolationError`** (grant violations are
compile errors, not runtime events), and **`canGet` / `canSet`** (retired from
indexable-functions; `has` covers presence, and there is no runtime grant to query).
This discharges the deferrals R92 parked in `table-api.md` and empties tables §6's
runtime model (the tables.md sweep itself remains flagged below). Swept: `table-api.md`
(deferral section rewritten as resolved), `indexable-functions.md` (§1, §5),
`iterable-functions.md` (§1.6, §3).

**R99 — the R95–R98 sweep, first pass: tables, the builder, secret, keywords.** Applying
the protocol redesign where it bit deepest, with three consequences the sweep forced into
the open rather than deciding silently. **`tables.md`**: §3.3 rewritten to the two-space
model (element vs. protocol space; `->` now legally appears left of `=` where a member
grants `set`, retiring the old "never assignable" absolute); §6's per-key permission
model deleted bodily (R98) — the section now states the positive rule (element space
carries no permissions; encapsulation lives in protocol space, compile-checked) and
absence collapses from three cases to two (absent → `undefined`, present `null` →
`null`; the denial case is gone); §4's write-back prose de-protocoled (no *built-in or
protocol function* takes a `&` receiver — the R91/R96 convention stated once); §8.1's
constraint examples respelled to free functions (`count(t)`, `values(t)`). **Forced
ruling one, §7 unified: derived tables are born open *and bare*.** A transformer reads
element space only, so it cannot reproduce a protocol's per-table member state, and a
protocol without its state is incoherent — so transformers shed protocols, uniformly.
This also resolves a tension the old text carried (access "propagates by key identity"
in §7.2 while §6.4 said derived values are born protocol-free): the shed side wins,
identity copies keep everything, and re-attaching behavior is an explicit `apply` that
must re-supply what the protocol requires. **`stringBuilder.md`**: rewritten to the
member model — `buf` is an ungranted per-table `var` member (under new-`meta` it would
have been one shared buffer for every builder in the program, the exact hazard R96's
retirement of the keyword was for), the surface is `const get` fn members, chaining is
`&b->append(...)->append(...)`, and `identityEquality` is declared. **Forced ruling two:
`take()` is retired.** It is inexpressible under the pure-receiver convention (one
return channel cannot carry both the string and the reset receiver) and redundant under
COW: `build()` shares the buffer with the produced string and the O(n) copy happens only
if the builder is appended to again — so build-and-drop *is* the zero-copy path, and
interpolation's lowering (build on a dropped temp) gets it by construction. Rejected
alternative: a `[string, @stringBuilder]` pair return, which buys nothing over
`build()` + `&clear()`. **Forced ruling three: protocol members may take catalogue
names.** The builder keeps `isEmpty` — `b->isEmpty()` (buffer) and `b.isEmpty()` (UFCS,
element space, vacuously true) coexist because the spaces are disjoint; the old §6.4
name-avoidance rule died with the built-in protocol, and the doc now demonstrates the
split instead of legislating around it. **`secret.md`**: no protocol content after all —
swept for a latent defect instead: two sections were both numbered §3.1; gated
construction is now §3.2. **`keywords.md`**: the `meta` row deleted (R96); `apply`'s row
rewritten (expression operator + proto-block requirement declaration; the dynamic form
is a free function, not a keyword use); `self`'s row updated (type in return position,
value in bodies — no view); `get` / `set` added as contextual modifiers; and in the
predeclared-name list, `view` removed (R95) and **`iterable` added** — an R92 sweep item
that had no home until now. The `?->` token is deliberately not added yet (deferred by
decision, with the lexer/associativity work). Also swept, discovered by the verification
greps: `indexable-functions.md` §2's stale pointer into old tables §6;
`concurrency.md`'s value-carried-enforcement cite (old tables §6.4 → §6.2, and its
protocol half dropped — grant enforcement is compile-time now, so only constraints
still "follow the value"); `operators.md`'s `->`, `@@`, and `apply` rows plus its
operators-run-no-user-code passage (`->` is protocol-member *access*; the call parens
are what run a protocol function, exactly as with UFCS; the statement-form
`tab apply proto` row respelled to the expression operator, the dynamic form being the
free function per R97); and `spread.md` end to end (its old flatten signature respelled
without `onNoGet` / `asStream`; `table-api` cites → the catalogue files; and every
"values-only stream" translated to its R93 equivalent, a **list-like** stream — one
with implicit, undisturbed integer keys — including the R89 variadic-fill note and the
§2 fold-vs-collect distinction, which now bites only for *explicit* integer keys).

**R100 — the R95–R98 sweep, third pass: coalescing, `toString`, json, functions.** Four
files, and the one genuinely load-bearing translation of the whole sweep.
**`optional-access-and-coalescing.md`**: rewritten to the two-axis model. The R99 flag on
`???` resolves in its favor — its cut (absent-or-null vs. value) is entirely on the
*value* axis, which R98 never touched, so both coalescers survive verbatim; only the
denial axis dies, and the file now says where it went: **not deleted but moved to compile
time** (a forbidden access does not compile, protocols §3.1). The write-side tables lose
their grant rows; a `???=` overwrite is unconditionally legal, so both compound forms
share exactly one failure, the add-path `OpenViolationError`; the read-to-classify
`TableReadViolationError` caveat and the permission-throws-through-coalescers ruling are
deleted (the latter's three-row table is two rows now); a stale `freeze` disclaimer went
with them. Added: a **protocol space** section — the unapplied protocol is one more
absence under the same rules (`undefined` on bare read, coalescable; `?->` for calls,
argument-short-circuiting like `?.`), members of an applied protocol are **never absent**
(application binds every member, so `??=` has no protocol-space role), and a member's
*value* being `null` is the ordinary `???` case. **`conversion.md` — the forced
translation: the interface pattern.** Old `stringify` declared "its one member:
`toString = meta fn`", assuming each applier implements a protocol function — an
*abstract member*, which the member model does not have (protocol functions are
definition-fixed, R96; there is no per-applier override, deliberately). The faithful
translation uses only ruled machinery: **a required fn-typed member**,
`const get toString: fn (any): string;`, bound per application as an apply initializer
(protocols §4.2), immutable after, with non-errorability still contract-not-courtesy
(the initializer is checked against the declared member type, so no `!`-typed renderer
can enter). Dispatch respelled `value->stringify.toString(value)` — a qualified fn-member
read-and-call (protocols §3.4), not a protocol-function call, which is precisely what
lets every application carry a different renderer; the "earlier draft ill-formed" note
rewritten (bare `->toString` is no longer impossible, merely scope-dependent, which is
reason enough to qualify a dispatch entry point). This is the pattern for every
interface-shaped contract under the member model, and the spec now says so. It also
closes conversion §6's first open question ("`stringify` signature and totality
contract", which waited on "the protocol spec's meta-function conventions"): the
conventions arrived, the answer is the declared member type, and the bullet is removed;
the `fromString` question stays open, respelled as the pattern in reverse. One honest
consequence recorded: a renderer is a `get` fn-typed per-table member, so it sits in the
serialization surface and meets the fn rule — `toJson` on a `@stringify` value is a
`typeError` unless `skipFunctions` is set. **`json.md`**: §2.1 added — the serialization
surface stated per R96 (element space + `get`-granted per-table members;
definition-fixed and ungranted members never serialize, with the round-trip argument),
`skipFunctions: bool = false` added to both writers (per-call on `toJsonDynamic`, baked
into the generated writer on `toJson`), fn values raise `typeError`; open questions gain
the protocol-member nesting shape, and the `fromJson`-into-shapes item now names its
mechanism (apply + initializers over the granted surface). **`functions.md`**: §3.3's
arity example respelled (`myTab.filter`, UFCS); §3.4's `->` bullet rewritten (bare or
qualified `->`, no view form, no "meta pools"); §3.4's flagged std-shaping tail marked
**discharged** (the catalogues satisfied it, R91–R92); §4's errorability cross-reference
replaced (the dead custom-`apply` `: self!` and §7.5 cites → an errorable protocol
function and the dynamic `apply()`'s `table!`); §5's capability citation corrected
(protocols spec → capabilities spec).

**R101 — the sweeps completed: equality, the stream family, `?->`, and the tail.** The
final pass; with it, **the R91–R93 and R95–R98 sweeps are complete** — no spec file still
speaks the pre-catalogue or pre-redesign language (the examples were verified already
clean). **`equality.md`**: §4.4's proto example modernized to the member model, and
**§4.5 added** — the R96 equality surface as a numbered rule: applied-protocol sets must
match *as sets* (order not load-bearing), each protocol's `get`-granted per-table members
compare by `==`, ungranted members never do (incidental by declaration —
`identityEquality` is that rule's other half, not an exception), definition-fixed members
are vacuous, fn-typed members compare by identity; §4's opening and the summary table row
carry the addition, and the `view`-equality open question is retired as mooted (R95).
**`stream.md`**: §1.1 rewritten — "values-only vs. key-value" is erased for implicit vs.
explicit keys (R93), with the implicit keys stated to be *real* (key-facing functions see
them, transforms preserve them, `values` reindexes them, undisturbed they collect to a
`list`); §5.1's bridges respelled (`toStream` / `collect`, the retired flags noted); §7.3
extended with the R92 rule that passing a stream to *any* catalogue function takes it.
**`stream-api.md`**: restructured to the stream-only surface it actually owns —
producing, `peek` / `isConsumed`, `foreach`, restart — plus an orientation table over the
shared catalogue; the absorbed transform/collect/bridge sections are gone,
the key-aware-peek question is recorded closed (values via `peek`/`first`, keys via
`keyFirst`/`keyLast`, pairs via `foreach`), and the open questions trim to three, the
infinite-consumer guard staying open (iterable-functions §1.4 cites it, now at §6).
**`range.md`** and **`control-flow.md`**: enumeration prose respelled to implicit keys —
observable behavior unchanged (`foreach (k => v in 10..20)` always gave the index), but
the model is now uniform and the keys are real, `keys` and `keyOf` included.
**`associativity.md`** tier 1: `x->P` generalized to `x->name` (bare protocol-member
access), `x?->name` added, notes updated for assignability-where-granted. **`lexer.md`**:
the deferred **`?->` token lands** — `OPT_PROTO_ACCESS`, regex `\?->`, slotted into the
maximal-munch ordering (`??` › `?->` › `?.` › `?`). **`wildcard.md`**: the unused-parameter
example moves from `map` to `reduce` — under the catalogue signatures the old
`map(fn (_, index) => ...)` would need a `mode` argument (and would arity-error without
one, functions §3.3); `reduce`'s two-argument callback (`fn (_, item)`) makes the point
with no mode at all. **`regex.md`**: `text->find(pattern)` → `text.find(pattern)` (UFCS).
**`constraints.md`** §7: its cross-reference respelled — "protocol *element* types
(protocols §5.4)" → "protocol *member* types (protocols §3.3)"; the
checked-on-write / trusted-on-read analogy survives intact, since member writes still
check the declared type wherever the site cannot discharge it statically. `index.md`'s
Stream API row updated to the slimmed scope.

**R102 — producers produce streams; the string API sheds its flags.** The convention gets
its name: everything that generates a sequence from a non-sequence source — generators,
ranges, `io`'s `lines()`/`chunks()`, and now the string API — returns a **stream**, with
retention always the explicit `collect()`. `strings.md` was the last holdout (eleven
`asStream: bool` flags, a `stream | table` return convention, and a "materialize by
default" stance nothing else shared); all eleven flags are deleted and
`findAll` / `split` / `lines` / `codepoints` / `graphemes` / `characters` return `stream`,
with `lines`' inexplicable `table` return fixed by the same stroke and `slice`'s
vestigial flag dropped (one string in, one string out). The cost is one word in the small
case (`s.split(' ').collect()[1]`) — and the compiler may fuse `collect(split(...))` into
the eager build, an elision, not a semantics. **`bytes()` splits along the
producer/conversion seam**: `bytes(str): stream` is the iteration view (octets as ints);
`toBytes(str): bytes` is the conversion to the packed value (`to*` family, R94), the
inverse of `fromBytes`. Also swept while in the file: §12's builder summary — a **R99
straggler**, missed because the grep pattern "meta fn" does not match "meta functions" —
respelled to `&b->append(...)`, `take` noted retired; `join`'s glue-first receiver
deleted in favor of the ruled catalogue signature (the exact "receiver position
wandering" functions §3.4 flagged); `toInt`/`toDouble` respelled `int!`/`double!`
(postfix-errorable convention); `isValidUtf8`'s "future `bytes` type" updated (it
exists); §14's build-transfer question marked resolved by R99. **Discovered and flagged,
not fixed**: the conversion family (`toInt` over `string`/`bool`/`double` with differing
signatures and errorability across `strings.md` and `conversion.md`) still has the
per-receiver variants that functions §3.4's one-name-one-signature rule forbids — the
catalogues' half was discharged (R100); the conversion family's half is open below.

**R103 — stream destructuring: take-what-you-bind.** Positional patterns accept a stream
source, pulling **exactly as many elements as they bind**. Exhaustion binds the remaining
targets to `undefined` — the stream's ordinary absence, the answer `peek` already gives —
and surplus is simply left in the stream; neither is an error. This is deliberately *not*
destructuring §1.1's exact-length rule, and the ruling names why the asymmetry is
principled: the table rule is a **shape assertion**, meaningful because a table's length
is checkable in O(1) before binding; a stream's length is unknowable without consumption
and may be infinite, so a stream pattern is a **take-request**, and the no-silent-loss
philosophy does not bite (nothing is asserted, and the tail remains recoverable via a
rest binding or `restart`). **`...rest` binds the remaining stream, not a list** — lazy
head/tail decomposition with nothing buffered (on a table, rest collects; on a stream,
collecting would defeat the point). **The destructure takes the source** (the `|>`
transfer discipline): `rest`, if bound, is the one live handle. Two guards: keyed
patterns reject streams (no random access — compile error where statically known,
`typeError` otherwise), and **match arms reject streams** (a failed pattern test would
have consumed elements it cannot restore; streams are identity values to `match`, never
structure). Swept: `destructuring.md` (§1.4 added; intro, §5's rest type, §6),
`stream.md` (§2.1 added).

**R104 — bytes: `foreach` yes, `iterable` no.** The criterion that excluded strings from
iteration becomes the stated rule: **`foreach` consumes anything with an unambiguous
element sequence.** `bytes` passes — every element is unambiguously a byte — so
`foreach (x in b)` (and `foreach (i => x in b)`, offsets as keys) is direct; a bare
`string` fails (bytes, codepoints, or graphemes?) and keeps choosing its unit through
producers. But **`iterable` stays exactly `table | stream`**: admitting `bytes` would
break kind-follows-primary at every transform — `map`'s output cannot promise `0..255`,
`filter` cannot leave a sparse packed buffer — costing either ~47 per-function special
cases or an implicit bytes→stream conversion, and Luna does neither. The catalogue is
reached by the explicit bridge instead: **`toStream` widens to `iterable | bytes`** (a
union parameter, the documented no-overloading pattern), the single place `bytes` touches
the catalogue, yielding ints; `collect` of a byte stream is a **list of ints** (no
special case), and repacking is an explicit build — a `toBytes(iterable)` conversion is
flagged open below. `bytes.md`'s promised direct `b.map` / `b.filter` (§2, §6) are
**retracted** accordingly. This closes the bytes-iterability flag standing since R91's
landing. Swept: `bytes.md`, `control-flow.md` (the foreach-source rule),
`iterable-functions.md` §2.11.

**R105 — restartability: the immutable-snapshot rule.** `canRestart` is decided by one
rule: **a stream whose source is an immutable retained snapshot is restartable** — string
producers, ranges, and `toStream` over a table or `bytes` (the COW capture *is* a
snapshot, the case easy to miss); generator functions stay source-dependent per stream
§4, since a generator over a socket cannot promise replay. The rule's cost, asked and
answered: a restartable stream **pins its snapshot** for its lifetime. Not a real
problem — it is ordinary value lifetime, identical in kind to a closure's deep-`const`
capture (functions §2.1) and to a string slice pinning its parent's buffer (string-api
§6, the in-corpus precedent), and the escape is the ordinary one: `collect` the small
result and drop the stream. Swept: `stream.md` §4, `stream-api.md` §4, `range.md`.

**R106 — the conversion family unified: three prefixes, three contracts.** The last
standing violation of functions §3.4's one-name-one-signature rule — `toInt` in three
per-receiver variants (`string → int!`, `double → int!`, `bool → int`), `toDouble` in two
— resolves by the **distinct-names** arm, not the union arm, because this family is the
union pattern's weak spot: errorability varies with the input, so a union signature
(`toInt(v: string|bool|double): int!`) would force the `try` tax onto the provably-total
`bool` path — a signature that lies in the safe direction is still a signature that lies.
The resolution recognizes **three different operations wearing one verb**, and gives each
its prefix, tabulated in conversion §2: **`to*` is total, always** (`toString`, `toInt`,
`toDouble`, `toBytes`, `toJson`, `toStream` — seeing `to` means no error handling);
**`parse*` acquires from bare text, always `!`, and names the *target*** (`parseInt`,
`parseDouble`, `parseBool` — the input type, universal `string`, says nothing, so the
name must); **`from*` decodes a typed carrier, always `!`, and names the *source***
(`fromJson`, `fromBytes`, `fromYaml` — the output, ambient `table`/`string`, says
nothing, so the name echoes the input). The principle: *the name carries whichever end
the signature doesn't make obvious.* The families never compete — `parseJson` would mean
"text → `json`", which already has its one spelling (`raw as json`, the constraint
entry), so it does not exist and `fromJson` keeps its name; the `parseFromJson`
contortion dies with the false dilemma. Noted, not papered over: `from*` is uniform about
naming, not about where validation runs (`fromJson` trusts its constraint-validated
input; `fromBytes` validates raw octets itself). **`double → int` is a policy, not a
conversion**: the retired `toInt(d)` silently truncated; it becomes the **policy verbs**
`trunc` / `round` (ties away from zero) / `floor` / `ceil`, each `fn (d: double): int!`,
deliberately outside the prefix families — they are decisions, not conversions, and the
choice is now visible in the source. Survivors, each exactly once and total:
`toInt(b: bool): int`, `toDouble(n: int): double`. Swept: `conversion.md` (§2 rewritten
around the contract table, §4, §5), `double.md` §7 (the four verbs), `strings.md` §4
(`parseInt` / `parseDouble`) and §14, `functions.md` §3.4 (the discharge note now true in
full), `bool.md` (already conformant — `toInt(b)` *is* the surviving `toInt`), and the
examples, where the sweep surfaced two latent drifts fixed with cause:
`one-billion-rows.md` **indexed a `split` result** (unindexable since R102 made it a
stream) — rewritten to R103 stream destructuring, which the example now showcases; and
`testing.md`'s `parsePort` used `n as int` on a `double`, violating the as-never-converts
rule (as spec §3) — rewritten to a direct `parseInt`. `fromString`-as-open-coercion stays
open (conversion §6), unchanged.

**R107 — `toBytes` widens; the `to*` contract is precise about panics.** The R104 open —
repacking an iterable of ints into a `bytes` — resolves into the **existing name**:
`fn toBytes(src: string | iterable): bytes`. Two observations make the name legal:
constraint violations are **panics**, not declarable errors (constraints §7.1, bytes §2),
and panics are signature-exempt (functions §4, "just about every function can panic") —
so the iterable arm, which checks each element against the `byte` constraint, carries no
`!`, and the union R106 forbade for `toInt` is clean here: *both* arms are `!`-free (the
string arm cannot even panic — a string's octets are bytes by construction), so neither
forces a `try` tax on the other. `parseBytes`, the floated alternative, is rejected by
R106's own table: it would be the one `parse*` taking no text and carrying no `!`,
eroding both halves of a one-ruling-old contract — and "parse" misdescribes an operation
that decodes nothing (the ints are already values; only the packing changes). Semantics:
the iterable arm is a **bulk append**, morally the `b[] = x` loop it abbreviates,
inheriting its per-element `typeError` panic — and panic is *right*: a non-byte int out
of your own transform is an invariant bug, not the data-dependent recoverable failure
that makes `fromBytes` errorable (world octets may not be UTF-8; your map's range is
your contract). A stream argument is taken (iterable-functions §1.5). Sharpened
alongside, in conversion §2: the contract table's "**never**" means never **`!`** —
panics stay possible in `to*` as everywhere, with `toJson`'s `typeError` on fn values
(json §2.1) and `toBytes`' constraint panic as the in-family examples; the contract is
"no error *handling*," not "cannot fail on misuse." Swept: `strings.md` §9,
`conversion.md` §2, `bytes.md` §6; the R104 open is retired below.

**R108 — named arguments and variadics: the call surface completed.** The R89/R90 flag —
three parameter notations used across the std surface and defined nowhere — closes as
functions §3.3.1–§3.3.3. **Named arguments** (§3.3.2), on the PHP rules: always optional,
pure ergonomics (skipping defaulted parameters; `a.merge(b, preserveKeys: true)`),
positional-first (a positional after a named is a compile error), usable with any
function including a type-erased `fn` — parameter names ride fn values as runtime
metadata, and are therefore **contract**: renaming a std parameter is a breaking change,
stated in the spec. Double-binding and unknown names error on the established
static/dynamic split: compile error against a known signature, the new
**`NamedArgumentError`** panic (errors §2's tree, the sibling of `ArityError` — arity is
the count-mismatch binding panic, this is the name-mismatch one) through erased calls.
One deliberate asymmetry named: surplus *positional* arguments stay dropped (§3.3, the
callback idiom) while a surplus *name* panics — an extra positional means the callee
ignores it; a wrong name always means the caller was aiming at something. The PHP 8.1
associative-spread-becomes-named-arguments behavior is **rejected** ("an evil, evil
footgun", ruled): spread into arguments remains a list-only sequence context (spread
§4); names are visible at call sites, never manufactured from data. Function resolution
is untouched — names bind arguments, never select functions. **Variadics** (§3.3.3):
`...name: T`, collected as a **`list`** always (zero arguments → `[]`, each element
checked against `T`, bare is `any`, no declarable default) — and the R35 unification
**now holds**: the variadic is destructuring's trailing rest in the *positional*
parameter sublist, with what follows outside positional space. **Only defaulted
parameters may follow a variadic**, named-only by construction — a *required* named-only
parameter would make named arguments mandatory at every call, contradicting their
always-optional charter. The `*,` keyword-only marker is **retired as redundant**
("after the variadic" already says named-only); the four catalogue signatures shed it.
Call-site spread composes as the identity (`f(...xs)` re-collects to `rest == xs`,
COW-cheap; list-like streams fill variadics, R89). **`name?` settled** (§3.3.1): `?` is
only ever the type (`T | null`, uniform with proto members, R96); a parameter `p?: T`
with no written default reads as `= null` — sugar, not a divergence. And the convergence
R97 hedged on is embraced: apply initializers and named arguments share one surface
(`name: value`) by design, binding members and parameters respectively — protocols §4.2
and §10 updated, the initializer-grammar open closed. Swept: `functions.md`
(§3.3.1–§3.3.3), `errors.md` (the panic tree), `iterable-functions.md` (§1.6, four
signatures), `spread.md` (§4's rejection bullet, §7 resolved), `protocols.md` (§4.2,
§10), `associativity.md` (argument punctuation).

**R109 — the growth seal is removed: tables carry no seal state at all.** `open()` /
`close()` / `neverOpen()` are deleted, and `OpenViolationError` / `InvalidOpenError`
with them — §5.2's freeze/thaw argument transferred verbatim to the growth axis: under
copy-on-write value semantics a callee receives a copy and tasks never share a mutable
table, so nobody can add a key to *your* table but you, and a growth seal protected a
table only from its own holder — a three-state runtime flag, a revocability
distinction, and two panic types spent encoding *discipline* in a
safety-by-construction language. The old §5 design note had already confessed the
direction ("better expressed as a compile-time contract… under review"); the
replacements now exist, and the removal record names them (tables §5.1): a fixed shape
as contract → a **protocol** (compile-checked member space, R95–R96); an element-space
invariant including a fixed key-set → a **table-level constraint** (tables §8,
value-carried, elided where provable, `list` the built-in instance); the `neverOpen`
optimization claim → **`const` tables** (Amendment A) and the deferred `toImmutable()`
(§5.2.1). One use has no zero-setup replacement, deliberately: accidental key creation
(`tab['tpyo'] = v`) is now guarded only by a *declared* invariant — an invariant you
care about is declared, not toggled. What the deletion buys: **element-space operations
have no runtime errors at all** (indexable §5's inventory is empty — R98 took the
permission errors, R109 takes the seal errors; ambient panics only, as everywhere);
**`??=` and `???=` are total** (the seal was their only failure mode; both compound
tables lose their error rows); **`&`-write-back is ordinary assignment** (§4.2's
"flag-respecting structural update" had no flags left to respect — governed by the
binding's mutability and declared constraints, like any assignment); and tables §7
simplifies to "derived tables are born bare." Swept: `tables.md` (intro, §4.1.1's
seal parenthetical → the declared-invariant note, §4.2 rewritten, §5 restructured as
the two-axis removal record with §5.2 / §5.2.1 preserved and subsection numbers stable,
§7), `indexable-functions.md` (intro, §2 → removal record with its section number kept
so references land, §3's legality note, `splice` / `fill`, §5 — where a stale "denial"
mention from before R100 was also caught), `iterable-functions.md` (intro,
`prepend` / `append`), `optional-access-and-coalescing.md` (write-side totality, both
tables, the sealed-tables section deleted, the laziness ruling), `bindings/variables.md`
(the `const` paragraph — where two stale "table-protocol violation" phrases, R95–R98
residue, were discovered and respelled as `typeError` panics), `index.md` (two rows).
`indexable-functions` drops from 22 functions to 19; `defer.md` and the build-cache
spec's `open()` / `close()` are file handles and untouched.

**R110 — error equality: the identity surface; `toTable`.** Equality §7's oldest open
resolves as **structural-minus-trace, made principled**: `a == b` for errors iff the
same error **type** (nominal, no erasure — the enum-variant rule) and the **identity
surface** deep-matches by `==` — `message`, the declared fields, `data`, and `cause`
(recursively, each level by its own surface) — with **`stacktrace` never comparing**:
the trace is runtime-attached provenance, written by `throw`, recording *where* the
error happened rather than *what* it is, so two equal errors thrown at different sites
are equal, which is exactly the tests-and-dedup case `==` exists for. This is the
**surface principle's third instance** (protocols' granted members, R96; tables'
element space), now stated as one sentence: *equality compares what the author
declared; what the runtime attaches is not identity.* A premise from the discussion
corrected on the way: the stacktrace is **not** a `secret` and cannot be one
(`stacktrace: table`, sealed but readable, errors §2.1 — with the documented
`stacktrace.isEmpty()` idiom; and `secret` wraps `string | bytes` only, secret §6), so
the secrets-never-equal trap does not fire — though **secret contagion** is now stated
explicitly where it was only derivable: a container whose compared surface holds a
secret (a table element, an error field) is never equal, including to itself, the same
family as nan; guidance recorded (don't put secrets in compared fields). **`toTable`**
lands as the surface reified: `fn toTable(e: error): table` — total (`to*` contract),
*the* definition of the surface (`==` is typeid plus `toTable` equality), and the
shape-matching bridge (`match (e.toTable()) { ['code' => 11] => … }` rides ordinary
table patterns while the type axis stays with typed binders / `is` / `@`). It claims
the global `toTable` name under one-name-one-signature; nothing else wants it. Swept:
`errors.md` (§2.2 added), `equality.md` (§2's list, §5's error bullet and the secret
contagion line, §6's row, §7 resolved), `conversion.md` (§2's table, §5's catalogue).

**R111 — secrets widen to `table`; the gate-constraint idiom; the stacktrace becomes a
secret.** Three rulings in one motion. **The payload set gains `table`** — §6's old "there
is no meaningful secret table" met its counterexample: data whose *structure itself* is
the disclosure, the stacktrace (frames leak paths and internal shape) being the motivating
case. The kind tag goes three-way, `revealTable` joins the extractor family, and the
doctrine is re-scoped rather than deleted: **wrap the leaves** when only values are
sensitive (the credentials object stays the canonical example), **wrap the table** when
the shape is the secret — the whole-table secret is the exception, not the default. The
challenge was run and found obligations, not blockers, all discharged in the sweep: the
`stacktrace.isEmpty()` idiom (unreadable through a secret) is replaced by
**`wasThrown(e: error): bool`**, a dedicated predicate disclosing one bit and no
contents; the crash reporter becomes a named reveal boundary (secret §5.1's
infrastructure pattern — and user-facing display now redacts traces *by construction*,
the display concern from R110's discussion solved as a feature); and **the R110/R111
interlock is stated in three files**: because R110 already excluded the trace from the
identity surface, wrapping it in a secret adds no equality contagion — had equality been
ruled structural-in-full, this widening would have made every error never-equal to
itself. Discharged in passing: the `revealBytes` deferral (`bytes` exists). **The
gate-constraint idiom** delivers "secrets parameterized over capabilities" with zero new
mechanism — no generics, the `json` pattern over the immutable base `secret`:
`export const dbSecret = constraint { s: secret where gatesOf(s).exists(@dbCred) };`
entry-checked once, then `fn (cred: dbSecret) use (dbCred): conn!` tells the whole story
(authority in `use`, material in the type, `reveal` joining them at the effect site with
the existing requirement-⊆-grant test, statically discharged through constraint-typed
parameters in matching frames); convention recorded — a module exports its secret
constraint beside its capability. **A membership operator was considered and scratched**:
the draft spelling `@dbCred in gatesOf(s)` would have made `in` a membership operator,
which keywords §6 explicitly reserved against ("`in` remains spoken for by `foreach`") —
and the survey surfaced that protocols §8's `stringBuilder in @@b` example (inherited
from the retired views spec) had been illegal under that ruling all along; both spellings
land on the catalogue's `exists`, the note stands, and `in` stays foreach-only. The
trace's gate is the default `[@reveal]` for now, with the dedicated-trace-capability
question flagged (errors §10). Swept: `secret.md` (intro, §3, §3.1, §3.3 added, §5, §6),
`errors.md` (§2.1, §2.2, §6.1, §10), `equality.md` (the interlock clause),
`protocols.md` (§8's example).

**R112 — call-site delegation: `use` on a call; the dynamic check stays frame-local.**
The R39 contradiction is resolved by *addition*, not by picking a side. Capabilities §5.1
said value-mediated calls check against "the executing frame's granted set (the frame's
declared `use`)" — frame-local, under which a `use`-free function invoking an effectful
closure panics, the guarantee that a declaration bounds its dynamic extent. §3.1
simultaneously promised "callback-taking APIs accept effectful functions with no special
cases," which frame-locality makes false (`each`'s frame holds nothing). Two earlier
resolutions were examined and rejected: an **ambient / task-root grant** (the check
nearly vacuous within a task — "every function implicitly holds `main`'s capabilities"
for the dynamic frontier, extent purity lost, recoverable only by an opt-in `pureFn`
constraint, purity-as-opt-in in a language whose charter is authority-as-opt-in); and a
**dynamic-grant statement** (`grant io { … }` — ambient authority manipulation,
degrading the audit from "read the declarations" to "trace the grants"). The synthesis:
**the frame-local check stands, and a frame's grant is its declared `use` plus what
callers explicitly delegated into it** — a clause on the call, in the same spelling:
`someFn(cb)` panics when `cb` needs io (the negative case, now a worked example);
`someFn(cb) use (io)` extends io into that call's dynamic extent, legal only where the
delegator itself holds io (statically checked). Delegation is extent-scoped, feeds the
**dynamic frontier only** (value-mediated invocations and reveal gates — named calls
still require declared `use`, everywhere, at compile time), and `spawn f() use (io)` is
the explicit spelling for granting a task. §3.1's promise becomes true as written: the
*API* has no special cases; the caller authorizes at the site. The audit strictly
improves: `grep "use (io)"` now returns declarations *and* delegations, one search for
the complete authority story. **Reuse of `use` over a new `with` keyword, ruled**: the
unified reading is real (capabilities flow where `use` names them — a body's extent or a
call's), position-resolved dual roles are the established device (`apply`, `error`,
`comptype`), the grep unification *requires* one spelling, and `with` would add a
keyword with JS's worst associations. Grammar: the clause wraps one complete postfix
expression, no postfix may follow it, operators and statement modifiers compose outside;
decided at one token (`use (` after a postfix expression vs. after a keyword-introduced
header), LR(1). Every corpus phrase "the executing frame's granted set" stays true —
its *definition* gains the delegation component in one place. Swept: `capabilities.md`
(§3.1 ×2, §5.1, §5.2 added), `keywords.md` (`use`'s row), `associativity.md` (the
clause). R88's flagged reflection query (`requirementsOf`) stays open but is now
**decoupled from safety**: extent purity is the default again, so no `pureFn` bandaid is
needed.

**R113 — secrets completed: union `reveal`, `canReveal`, `revealStackTrace`,
`<secret>`, delegated serialization.** Five rulings landing on R112's foundation.
**`reveal` returns the union** `string | bytes | table` — one extractor, superseding
R111's `revealBytes` / `revealTable` days after they landed (the log preserves the
chain): the "no overloading; unions instead" doctrine applied to reveal itself, with the
payload narrowed like any union (`as`, `match`) and §3.1's static kind tag dissolving
into ordinary union typing — less static precision, full uniformity. **`canReveal(s):
bool`** is the probe form to reveal's assertion form — the same gate ⊆ grant test
returned instead of enforced, the hard/soft pairing of `.`/`?.` and `->`/`?->`, and the
panic-free render-or-redact idiom (`canReveal(s) ? reveal(s) : '<secret>'`). **The
`reveal*` convention**: capabilities whose purpose is revelation are `reveal*`-prefixed —
`reveal`, and now **`revealStackTrace`**, the trace's dedicated gate (resolving R111's
open; the crash reporter holds it; `grep "use (revealStackTrace)"` audits who may see
internal structure) — so `grep "use (reveal"` returns every revelation authority,
declarations and delegations alike. **The placeholder is `<secret>`**, replacing
`<redacted>` on every display path (toString, interpolation, `debugJson`, error display):
it names *what kind of thing* was concealed, so a reader knows to look for a gate rather
than wonder which redaction policy fired. **Serialization ruled** (the flag standing
since R96): `toJson` renders every secret as `'<secret>'` — serialization is a display
path and concealment-on-display is the secret's designed behavior, so no error (contrast
fn values, a structural impossibility) and no skip flag; lossy by design. Revealed
serialization is **explicit twice over**: both writers take
`revealSecrets: fn (s: secret): string | bytes | table = null`, and the idiomatic call
delegates the gates at the site —
`toJson(cfg, revealSecrets: r) use (dbCred, revealStackTrace)` — with revealed tables
serialized recursively, revealed bytes routed through `string.fromBytes` (the one
revealer-path error), and declining spelled by returning the placeholder. Swept:
`secret.md` (§3.1, §4, §5, §7), `errors.md` (§2.1, §10 resolved), `json.md` (§2's
signatures, §2.1), `command.md` (two placeholder mentions; prose uses of "redacted" as a
verb stay).

**R114 — call-site delegation does not breach the comptime sandbox (amends R112).** The
review question: did R112 break the "comptime needs no sandbox because effects are
structurally impossible" guarantee (capabilities §8)? **It did** — the hole is
`comptime render(tpl) use (exec)`: `render` is eligible (its declared set is empty, it
only *invokes* a closure argument), the delegator-holds check passes at module top level
under `main`'s grant, and the value-mediated check passes because the delegated `exec` is
now *in* the frame grant — so `exec` reaches inside a comptime evaluation. The guarantee
had an unstated premise: **`use` appeared only in declarations, which eligibility
inspects.** R112 added a second `use` site (the call) that eligibility does not inspect,
and the premise silently failed — the "no sandbox needed" assertion would have been false
after R112 without a fix. Closed by three rules, all now in the spec: **(1)** a comptime
boundary **resets the frame grant to ∅**, regardless of what the enclosing runtime frame
declared or was delegated — a hard floor no `use` of either kind can raise, and now
stated as the premise §8 rests on; **(2)** **`comptime f() use (X)` is a compile error**
(ruled a hard error, not a vacuous no-op: `use` in that position is purely runtime and
holds zero compile-time meaning) — the `comptime`/delegation interaction R112 never
considered, the sibling of `spawn`'s narrowing; **(3)** delegation is **invisible to
comptime-eligibility**, which reads declared `use` only — it *could not* contribute
soundly, since eligibility is a fixed property of a value (§5.1) while a delegation is a
property of the caller, so letting it flip a callee's eligibility would make a value fact
depend on who calls it (the same reason it stays out of §6's inference). The laundering
theorem holds at comptime again: the only non-comptime authority in reach is one the
compile-time frame was never granted, and neither `use` site can manufacture it. A second,
independent §8 argument (no non-comptime capability *instance* exists at comptime, so the
dynamic check fails vacuously) was already correct and is unaffected — belt, brace, and
bolt still agree. Swept: `capabilities.md` (§5.2's two carve-outs, §8's boundary-reset
premise and the vacuous-check phrasing), `functions.md` §5.5 (eligibility reads declared
`use` only).

**R115 — cancellation is specified alpha semantics, runtime-initiated only.** The
concurrency review found the family's largest contradiction: `concurrency.md` built
fail-fast (§4), structured lifetime (§6), and the guarantees (§7) *on* cancellation as
current semantics, while `await.md` §3 said "there is no cancellation in the alpha."
Resolved in concurrency.md's favor — the alternative was examined and found untenable:
without cancellation, fail-fast degrades to fail-slow (scope exit waits on the slowest
unrelated sibling), a task blocked on a dead socket hangs program shutdown *permanently
with no in-language remedy*, timeouts become impossible forever (§8's own no-time-limits
resolution cites cancellation existing), and structured concurrency loses the co-feature
that makes scoped tasks worth having. The semantics (new §6.1): **cooperative,
suspension-point-delivered, refused-on-entry** — `cancelled` (a runtime-minted panic,
now in errors §2's tree) is delivered *instead of* performing the suspension point's
operation, the same before-the-effect principle as the R39 check, and a parked task is
interrupted at the park (the blocked-io fix); **observable, never stoppable** —
catchable like any panic (uniformity kept), but re-delivered at the next suspension
point, the pending flag uncloseable by user code, with the safety proof recorded:
catch-resistance is exactly as strong as never suspending, which is the
already-conceded compute-hang carve-out, so zero new attack surface; **defers run
uncancelled, unconditionally** — and are therefore *the compensation context* (a catch
block's io gets re-delivered; a defer's completes), with body-maintained progress state
as the exit-reason channel and the misbehaving-defer hang accepted as a residual beside
logic-bug hangs; **runtime-initiated only** — fail-fast and scope exit cancel, no user
`cancel(p)` exists, and timeouts / `awaitAny` / deadlines stay deferred *as surface*
over these semantics. The Luna-specific asymmetry is stated in §6.1: the classic
async-exceptions blast radius is a shared-state blast radius, and Luna deleted the
shared state — a cancelled task's partials are invisible copies, its handles already
taken, its promise resolves; only the external world remains, the existing carve-out.
Swept: `concurrency.md` (§6.1 added, §7's two carve-out bullets, §8's cancellation open
resolved), `await.md` (§3 rewritten from full deferral to surface-only deferral),
`errors.md` (the `cancelled` row).

**R116 — error collection is per-promise; the stream-of-`T!` form is retired.** The
second contradiction: concurrency §4 promised "await into a stream of `T!`" for
collecting failures, while await §1.1 ruled streams have no error channel (a failed task
panics at the pull). await.md's side wins — it is the newer text and the one consistent
with the stream model: the stream-collecting `await` is fail-fast in stream form, and
collection is **individual** (`try await p` per promise, where the declarable channel
exists), kept honest by ordinary errorable-value rules. Swept: `concurrency.md` §4 (the
paragraph rewritten, the retirement recorded inline).

**R117 — promise confinement wins; the circulation rule made precise.** The third
contradiction: concurrency §3.1 ruled promises confined (compile error crossing spawn
boundaries — the premise of the await-DAG deadlock proof), while await §4's open floated
"it moves" (transfer + taken, like streams). Confinement wins, because the deadlock
guarantee cites it; and the review forced the precise circulation rule: a promise flows
through **bindings and streams only** (single-pass, scope-local — the §5
`map(fn (x) => spawn f(x))` composition, load-bearing for await §1.1) and may **not**
enter retained storage — a table element would let a copyable, boundary-crossing
container carry it — nor be captured, passed into, or returned from a task; `collect` on
a promise stream is likewise an error (await the stream, then collect the *results*).
Compile error where statically evident, panic otherwise. Swept: `concurrency.md` §3.1,
`await.md` §4 (the open resolved).

**R118 — the task root; concurrency aligned with R112; three opens closed.** The
R42-era closing entry — "capability scoping at spawn: none… no narrowing-or-widening
mechanism" — was falsified by R112 (`spawn f() use (io)` *is* the widening mechanism).
Superseded in place: a task's **root grant** is the spawned function's declared
requirement set **plus spawn-site delegation**, checked against the spawner's grant at
the spawn; frames inside follow the ordinary R112 rules from that root. The §5 examples,
which predated the catalogue and the pipeline spec, are respelled legal:
`map(spawn expensiveThing)` (spawn is a word-prefix operator, not a passable value) →
`map(fn (x) => spawn expensive(x))` with the R112 delegation annotation where the work
is effectful, and `|> await` (await is not a pipeline stage) →
`await (xs |> map(…))` per await §1.1; await.md's tier number corrected (12, not 11).
Closed by pointing at the now-correct text: stream-api §6's parallel-transformers open
(concurrency is opt-in per stage; no transformer is implicitly parallel), stream §8's
parallel-consumption open (streams transfer with the taken state; two live cross-task
handles cannot exist), and concurrency §8's capability-scoping open (the task root is
the mechanism). The await-surface open is trimmed to its one genuinely-open remainder,
`awaitAsCompleted` versus fail-fast. Swept: `concurrency.md` (§5, §8, the closing
entry), `await.md` (§1), `stream-api.md` §6, `stream.md` §8.

**R119 — channels: the stream receive end, the shared `sink`, the owner-task pattern.**
The last major missing piece lands as `channels.md`, on the shape §8's old open predicted
and this design confirmed. **Creation**: `let [tx, rx] = channel(capacity: int = 0)` — a
destructured two-list; `0` is rendezvous, `n` buffers with backpressure. **The receive
end is literally a `stream`** — the whole catalogue applies unmodified, parking on empty
is ordinary stream consumption, `canRestart` is `false` by R105's own rule, and
exhaustion *is* the closed signal (receive-from-closed is a stream ending, not an
error). **The `sink`** (a new predeclared type) is the one deliberate exception to
single-ownership in the stateful class, governed by the principle that makes the
exception precise — **ownership follows readability**: a stream is taken because reading
is stateful consumption; a sink has no readable surface (no peek, no count — "once sent,
it's gone"), so sharing it shares no readable state. Sinks are **shared** on every copy
and crossing (the taxonomy row is capabilities', for capabilities' reason; the channel
interior is runtime machinery beside the scheduler), which is fan-in — the feature; the
residual mutual observables (interleaving, backpressure timing, finished-ness) are all
synchronization-class, never data-class, so §7's no-data-races survives verbatim. MPSC
now; select and MPMC deferred. **Sent values cross by the spawn taxonomy unchanged**
(eager deep copy for mutables — §2's non-atomic-refcount argument forces it at this
boundary too — `const` shares, streams/builders transfer, promises forbidden per R117);
"gone, poof" is ruled as no-shared-view, in-flight values owned by the runtime alone.
**Parked sends and receives are suspension points** (the R115 list grows), so
cancellation delivers there, refused-on-entry. **Completion is per-handle**: `finish(tx)`
relinquishes one handle, the stream ends when the last is finished or scope-dropped, and
there is **no whole-channel close** — which dissolves multi-producer close coordination
entirely (nobody can close a channel out from under a sibling). The name is `finish`
because `close` is unavailable under one-name-one-signature: io's
`fn close(&fd: file) use (io)` differs in reference mode *and* requirement set, which a
union cannot carry per-arm (and defer.md's `f.close()` vs. io's `close(&fd)` drift is
flagged for the io sweep). Send after finish, or to a departed receiver, panics —
**`ClosedChannelError`**, new in the panic tree — rare under structured lifetime, since
producers parked on sends receive `cancelled` at the park when the consumer's scope
fails. **The owner-task pattern is now real**: the counter example in channels §5 is
R96's "mutable static" as an owning task with reply sinks riding request tables —
protocols §2.1's pointer finally has a referent, and the R95–R98 open closes as *an
answer, not a deferral* (statics stay inexpressible by design; the owner task is the
expression). **`sync` variables considered again and rejected again**, citing
concurrency §5's own words: reintroduced shared state, false safety on compound
operations, hidden cost — the owner task is `sync` with the synchronization made
explicit and visible. **The honest cost, stated in bold in both specs**: §7's "no
deadlocks" weakens to "no lock or await deadlocks" — **channel-wait cycles exist** (two
tasks parked sending to each other's full channels), classed with logic-bug hangs,
contained by scope-bounding, cancellable at the parks, not structurally prevented; and
one leak shape is flagged (a sink sent through its own channel keeps it alive forever).
Swept: `channels.md` (created), `concurrency.md` (§2.1's sink row, §6.1's suspension
list, §7's deadlock guarantee, §8's channels open resolved), `protocols.md` §2.1,
`errors.md` (the tree), `keywords.md` §5 (`sink`), `index.md`.

**R120 — the BEAM scrutiny: five deferrals named, one softened.** The concurrency model
was audited against Elixir/BEAM, the SOTA. Where it stands: even on isolation (both
share-nothing-by-copy), **ahead** on structured lifetime (BEAM processes free-float and
leak; Luna orphans are structurally impossible), effect containment (BEAM has no
capability analogue), bounded-by-default channels (BEAM's unbounded mailboxes are its
classic overload death; Luna's rendezvous default makes backpressure the default), and
failure defaults (unawaited-error elevation, fail-fast); **behind** on hang *recovery* —
BEAM survives its own deadlocks because call timeouts are ubiquitous, and Luna currently
cannot express "wait, but not forever." Hence the notes, each now on the ledger:
**(1)** timeouts + `awaitAny` / select are deferred and **named the top post-alpha
priority** (concurrency §8) — *safety, not convenience*: the admitted channel-wait-cycle
deadlock has no in-language recovery until they land; a deadline is a cancellation and a
select is a race, riding R115. **(2)** **`std/time.md` created as a full deferral
record**: nothing in the corpus can read a clock, sleep, or build a timer (a
retry-with-backoff is unwritable today); two constraints are ruled now (`sleep` is a
suspension point; the timeout family is race-against-`sleep`, so the module and the
surface land together), everything else deferred. **(3)** capacity-as-decoupling
guidance in channels §1 (rendezvous maximizes both backpressure and wait-cycle risk).
**(4)** the stdlib patterns layer recorded in channels §7 — `call(tx, req)` (the
`GenServer.call` shape), the supervisor loop, the registry task; library, not language,
wanted especially once timeouts land. **(5)** unconditional kill (BEAM's
`Process.exit(:kill)`) recorded as nonexistent and **extremely unlikely** ever to exist —
softened from "never" at Lucas's direction, since the Go backend (which cannot kill a
goroutine) is the forecloser and a backend change could reopen it; concurrency §7 and
await §3 both carry the phrasing. Also acknowledged in the audit, not actioned: the
crossing taxonomy's six rows are the model's honest complexity concentration (BEAM has
one row — everything copies), the taken state is a runtime lesson where Rust teaches at
compile time, and task observability (BEAM's observer) has no story yet. Swept:
`concurrency.md` (§7, §8), `await.md` (§3), `channels.md` (§1, §7), `std/time.md`
(created), `index.md`.

**R121 — the io review: the stream convention for files, creation-authorized lazy
effects, three deferral records.** The io family's verdict: the failure model is the
best-reasoned corner of the corpus (the three-category table; io-errors' five-fates
errno partition, where every errno the target can return has exactly one row) — but §2
missed the protocol redesign, the `&fd` convention contradicted the settled model, and
the review surfaced one genuine capability-model gap. Fixed: **(1)** io §2's "meta state
(protocol-private)" — an R99-family straggler — respelled as ungranted per-table `var`
members, the builder's shape. **(2) The `&` is gone from every file signature** (`close`,
`flush`, `append`, `appendLine`, `write`, `seek`): a file is a **referent-stateful**
value in the transferred/taken class, "like a stream or a builder" by io's own words, and
`close` *marks the referent's terminal state* — the concurrency §2.3 model — so file
operations follow the **stream** convention (take the handle by value, mutate the
referent, no write-back), not the COW-table convention. The R119-flagged
`&fd`-vs-`f.close()` drift resolves in **defer.md's favor** — it was right all along;
seven call sites swept (`index.md`'s front page, both big examples, log-scan, tests,
await §3's quote), and channels §5's `finish`-not-`close` rationale re-grounded on the
capability axis alone (which was always sufficient). **(3) Lazy io ruled
creation-authorized** — the interesting find: `lines(fd)` performs its reads at *pull*
time, possibly in a `use`-free frame, with no invocation check (streams are consumed by
syntax, not called), which made the laundering theorem's literal statement false for
lazy carriers. Ruled (option (a)): the theorem's honest general form is now stated in
capabilities §3.1 — *every capability exercise is authorized by a declared `use`, at the
exercising call or at the creation of the value that carries it* — with the stream as a
pre-authorized effect in motion, the §5.2 trust model verbatim; io §6 carries the
companion paragraph. **(4)** io-errors §4's `interrupted` open **closed by R115**:
`EINTR` absorption and cancellation are separate mechanisms (syscalls restart
transparently; cancel-pending is checked at the park, `cancelled` refused-on-entry) —
user code sees neither. **(5)** file streams ruled **non-restartable** (a live cursor is
not an immutable snapshot; R105) in io §6 and stream §4, whose "a file can be re-opened"
example was the drift; re-traversal is explicit, `seek(fd, 0)` plus a fresh view.
**(6)** the `sink` wordage collision with R119's type reworded in io §2.1/§4. **(7)**
Three more absences become decisions: **`std/platform.md`** (the most load-bearing stub
in the corpus — `println`'s *default parameter* reads it today), **`std/system.md`**
(the metadata boundary io §9 names, under the separate `system` capability), and
**`std/net.md`** (the largest missing std family — with its gating dependency stated
plainly: no network API before the timeout surface, the R120 lesson applied
prospectively). Healthy and untouched: the capability story, the `path` constraint, the
printing surface, the per-format composition seam, and exec's secret boundary. Swept:
`std/io.md`, `std/io-errors.md`, `capabilities.md` §3.1, `stream.md` §4,
`channels.md` §5, `await.md` §3, `index.md` + five example/spec call sites;
`platform.md` / `system.md` / `net.md` created and indexed.

**R122 — error casing: camelCase everywhere; the R87-era flag closes.** The oldest small
flag in the file resolves the way the evidence always leaned: **every error and panic
type name is camelCase**, matching the roots (`error`, `panic`), the always-camel
builtins (`typeError`, `outOfMemory`, `cancelled`), and the std family (`ioError`,
`fileNotFound`, `commandError`). The
eight live PascalCase holdouts renamed in place across twelve files: `arityError`,
`namedArgumentError`, `overflowError`, `divisionByZero`, `closedChannelError`,
`applyError`, `writeOnceViolationError`, `stringBoundaryError`. **Historical mentions
keep their dead spellings deliberately**: the retirement records (tables §5.1's
`OpenViolationError` / `InvalidOpenError`, the R98-retired violation errors) name things
that no longer exist, and their names are frozen in the record — renaming a tombstone
would falsify the history it preserves; the same holds for this log itself. One
collision caught in passing: protocols §2.2's write-only example member was named
`sink`, a type name since R119 — renamed `output`. Swept: `errors.md` (the tree),
`functions.md`, `match.md`, `spread.md`, `concurrency.md`, `channels.md`,
`numeric-operators.md`, `defer.md`, `int.md`, `protocols.md`, `variables.md`,
`strings.md`, `keywords.md` §6 (the flag itself, now a resolution record).

**R123 — removal exists: `unapply`, a checked write, never a free mutation.** The
R95–R98 open closes on the side of existence, and the standing condition (§6.3: removal
must get invariant-constraint treatment, constraints §7.1) turned out to be the design,
not an obstacle. The soundness audit found the hard part already built: tables are
copy-on-write values (tables §4) and Luna refuses flow narrowing (`as` §7), so no third
party ever holds your table (`let q = t as @person` is a value copy a later `unapply` on
`t` cannot reach) and no scoped static assumption exists to falsify — the only promises
in the universe are the written-back binding's declared type and the value-carried
constraint typeids, both checkable at the one place removal happens, the write. And the
post-removal state needs no new semantics: a stripped protocol is an unapplied protocol,
protocols §3.2's fully-specified world (`undefined` on bare read, panic on hard use,
`?->` to ask). The shape: **a built-in free function mirroring dynamic `apply`** —
`fn unapply(tab: table, protocol: proto): table!`, `&t.unapply(p)` write-back — no
keyword, no operator: typed *construction* is common and fully compile-time-checkable,
typed *deconstruction* is neither, and a static form would buy refinement-subtraction
type algebra for a rare payoff. Failure taxonomy: **not applied → no-op** (the mirror of
idempotent re-apply, §4.3); **still required by an applied requirer → `applyError`** —
stripping `person` under an applied `employee` would leave `@employee <: @person`
claiming members that no longer exist, so unapply refuses and the caller peels in
reverse requirement order (**cascade removal rejected**: silently removes more than
asked; **a minted `unapplyError` rejected**: one name for the protocol subsystem's
runtime-data failures, `applyError` widened to application/removal); **a breaking write
→ `typeError` panic**, never the error union, compile error where statically evident
(`&p.unapply(person)` on `p: @person` always is — the operand's declared type contains
what is being removed). Per-table member state is destroyed, apply-initialized `const`s
included — data loss is the point — and re-apply after removal is a genuine fresh apply
(the "already has" tests never fire); §2.1's heading is honest now: **"one binding per
application," not "ever."** The audit also surfaced a **latent hole predating this
ruling**: constraints §7 defined the checked-mutation class enumeratively (key writes,
element writes), but predicates can observe protocol facts — negatively too (`where
!(t is @tagged)`) — so a dynamic `apply` through an untyped container slot could break a
constraint unchecked; monotonicity protects `@P` promises, never arbitrary predicates.
Fixed by defining the class semantically: **every mutation a predicate can observe** —
element-key writes, writes to `get`-granted protocol members through any path (a
predicate's reach is the granted surface, so ungranted writes are exempt), and
applied-set changes in either direction. Swept: `protocols.md` (§2.1 heading and body,
§4.3, §4.4, new §4.6, §6.3 soundness rewritten, §10 resolved), `constraints.md` §7 (the
mutation class), `errors.md` §2 (`applyError` annotated), `keywords.md` §3 (the `apply`
row; the keyword table itself untouched — `unapply` is a predeclared identifier, §5),
`operators.md` (the `apply` row).

**R124 — the F-series closes: F6 ruled (`as` is lossless), F4/F9/F22 were already
dead.** The audit first: three of the four F-entries this file's tail still carried were
stale, closed long ago — F4 (`list` drift vs panic) by R9/R10's fact/promise split keyed
to the write path's declared type; F9 (union subtyping vs the interval test) by R18's
two-tier test; F22 (`any` pipelines) by R34's types/any.md (universal operations work,
`|>` demands narrowing). The tail now says so. F6 — "the `as` algebra exceptions" — was
real: as.md §3 ruled "same bits, never a conversion" while the numeric tower granted
`as` powers beyond subset-narrowing: `double as float` (rounds — a silently computed new
value), `decimal as int` (panic-unless-exact over a precision-losing direction), and the
representation-crossing `int as u64` / fixed-to-arbitrary entry. **Ruling: the criterion
is losslessness, not representation.** "Same bits" was wrong on both sides — `int as
u64` legally changes representation while preserving the value exactly, and a
bit-preserving reinterpretation that changed the value would be exactly what `as`
forbids. `as` may move a value between representations precisely when the value, where
accepted, is preserved exactly, so the only possible failure is a membership/range check
(panic), never precision. Consequences: `int as i8`, `u64 as u8`, `int as u64`, and
`int as decimal` (total; §3.1's spelling updates from the `n.toDecimal()` placeholder —
the cost argument there was against *implicit* widening, and `as` is explicit, so the
rationale survives untouched) are all legal `as`. **`double as float` does not exist**:
rounding is a computation, so it is the conversion `toFloat(d: double): float`, total
(IEEE round-to-nearest-even, overflow to `float` inf, `nan` to `nan`), landing with
`float`; `toF16` likewise when `f16` lands. **`decimal as int` does not exist**: a
fractional `decimal` has no lossless `int` reading and picking a nearby one is a
rounding *decision* — exactly the double→int question R106 answered with policy verbs —
so `trunc` / `round` / `floor` / `ceil` widen to `double | decimal` when `decimal`
lands. Corroboration that the algebra was already at work unnamed: R106 made int→double
the *function* `toDouble` because it is lossy above 2^53, and R94 renamed `asStream` →
`toStream` on the same instinct. En-route finds, both fixed: as.md §8's lone open ("`as`
on secret payloads, pending `bytes`") was **resolved by R113** — `reveal`'s union
narrows with ordinary `as string` / `match` (secret §5) — and is now recorded closed;
and variables.md still used the retired `values()`-as-collector idiom
(`(0..50).values()` "materializes… a list" — `values` has been the kind-preserving
reindexer since R92, the materializer is `collect()`) plus an inference example claiming
`t.values()` infers `list` against the catalogue's `iterable` signature — re-exampled
onto a declared `fn (): list` producer. Swept: `as.md` §3 (the criterion, the table
row), §6 (the numeric known-uses bullet), §8 (resolved); `numeric-tower.md` §1.3, §2,
§3.1, §4 (all three narrowing rows rewritten), §5 footer; `conversion.md` §2 (the
verbs-widen note), §5 (the deferred `toFloat` / verb-widening entry); `variables.md`
(two sites); this file's tail (the stale F-list cleaned).

**R125 — protocol serialization: the `"@@"` section, off by default.** The JSON-nesting
open (R95–R98) closes. **Shape**: with `includeProtocols: true`, both writers emit the
protocol surface under one reserved key, `"@@"` — the axis's own operator name — mapping
each applied protocol's name to an object of its `get`-granted per-table members,
recursively serialized, sections in **application order** — not aesthetics but the
requirement-safe replay order (a requirer never precedes its requirement, protocols §7),
so the document doubles as an apply script. Empty sections emit: a marker protocol's
presence is equality-bearing data. **Default: excluded** (`includeProtocols: bool =
false` on both writers, baked into the generated writer like `skipFunctions`).
Serialization is an interop boundary first — the common call feeds consumers that know
nothing of protocols — and the default keeps `fromJson |> modify |> toJson` total over
foreign documents, a literal `"@@"` key included. The honest cost, recorded in protocols
§5: the one-boundary claim weakens to "the fourth consumer on request" — by default two
non-`==` tables can serialize identically; equality and the initializer surface are
untouched. A quiet win the default buys: the fn-member `typeError` narrows to protocol
mode, so a table wearing an interface-pattern protocol (conversion §3) serializes its
data cleanly by default. **Opt-in refusals, both `typeError`**: an element key literally
`"@@"` (in default mode it emits verbatim and no question arises — default-exclude is
what made refusal affordable, since the foreign-JSON objection lives entirely on the
default path), and two applied protocols sharing a name (legal composition, §6.1, but
duplicate section keys are illegal JSON; module-qualifying would make data depend on
file layout — refusal is cheaper, the case degenerate). **The read side stays
caller-driven**: no registry (R19), a name string cannot summon a proto, so `fromJson`
parses `"@@"` as ordinary nested data; rebuilding is the caller's — proto values in
hand, apply in section order, each section the initializer table; a reordered document
may fail with `applyError` (stated, not defended against). Round-trip is thereby
**`==`-faithful, not state-faithful** (ungranted members re-default, `==` reads the
granted surface — the one-boundary theorem), with two recorded exceptions:
`identityEquality` protocols (never `==` anything but themselves) and interface-pattern
protocols (the required fn member raises `typeError`, and `skipFunctions` omits exactly
what `apply` then demands — they do not round-trip; fn has no data form). Deferred,
named: whether `jsonTag` attributes reach member declarations inside a `proto` block,
riding with the generated read side (json §4). Swept: `json.md` §2 (signatures), §2.1
(rewritten around flag and shape), §3 (the read-side bullet), §4 (nesting resolved, the
`jsonTag` open added); `protocols.md` §5 (boundary reworded, serialization bullet), §10
(open closed); this file's tail.

**R126 — `@@` is total; protos are const-declared brands; a pre-R95 fossil deleted.**
The last R95–R98 open closes. **Operand surface**: `@@` is total over `any` — on a value
with no protocol axis (everything but a table) it yields `[]`, joining any.md §1's
universal-question set beside its sibling `@`. The coherence argument decided it:
`5 is @person` is already `false`, not an error — the language answers protocol
questions about non-tables with "no," and `@@` is the same question through the sibling
operator; refusing statically would make the two answers disagree. The result value
pinned: a **fresh snapshot `list`** of `proto` values in application order (value
semantics — mutating the copy touches nothing; after `unapply` (R123) the next `@@`
reflects the shrunk set), carrying the `list` commitment into inference. **Protos are
const-declared**: a `proto` block is legal in exactly one position, the initializer of a
`const` declaration (the `capability` precedent), and the binding identifier *is* the
protocol's name — captured at declaration, carried by the value, never renamed by
aliases or reexports. Anonymous protos do not exist (small use case; may be added
later). One declaration, one identity, one name makes a proto a **brand**, and the brand
settles equality: **protos join the identity-equality class** (equality §2) — two
structurally identical protos from different modules are deliberately distinct, so
structural proto comparison is meaningless. Honest cost, recorded in protocols §1:
membership-shape tests ("does this table wear something *shaped like* X") cannot be
spelled structurally — you need the proto value in hand — and that is not the hot path.
Both rulings are what R125 quietly needed: `protoName`, now the JSON section key and
collision comparator, is well-defined for every proto that can exist. **The fossil**:
reflection.md missed the R95–R98 sweep in six places — §3.4's pre-R95 query set
(`elementMembers`, `metaMembers`, `metaFunctions`, `hasApply`), views citations in
§3.4/§3.5 and the §5 operator summary, a `view` variant still in §3.3's `TypeKind`
enum, the closing-summary bullet still routing meta members through the deleted
queries, and two §6 opens premised on protocol-installed *element members* (a thing
R95 made structurally impossible — both dissolved, the `fields`-ordering half
surviving alone) — all reflecting a protocol model (meta space, declared element
members, custom apply bodies) that R95–R98 retired. **Deleted, not renamed** (per the tombstone rule
the deletions are recorded here and in place); `protoName` and
`declaresIdentityEquality` survive; the member-model query surface (members with
binding keyword / grants / type / default-presence; requirements) is **explicitly
deferred to the reflection deep pass**, with the two boundaries any future surface must
keep recorded in place: keyed on the proto, not the table; declarations, never values.
Swept: `protocols.md` §1 (const-only, the brand paragraph), §2/§7/§10 examples
(`const` added), §8 (totality and snapshot pins), §10 (open closed); `any.md` §1
(`@@v` universal); `equality.md` §2 (proto in the identity list); `reflection.md` §3.3 (the `view`
variant), §3.4 (rebuilt short), §3.5 (views cites), §5 (summary bullets), §6 (two
opens dissolved); `operators.md` (`@@` row); `keywords.md` (`proto`
row); `overview/types.md` (the literal); `stringBuilder.md` and `conversion.md` §3
(example spellings); this file's tail.

**R127 — introspection: the name, the module, the principles.** The reflection deep pass
opens with structure rather than API: the surface moves to a standard module,
**`std.introspection`** (new file `std/introspection.md`; `reflection.md` is a
retirement stub with a was→is section map), and the name change is load-bearing, not
cosmetic — names carry the contract (R94/R106's discipline), "reflection" is the
industry's name for the mutable surface Luna structurally refuses (runtime type
mutation, accessibility overrides, the PHP/Ruby failure), and *introspection* is the
read-only word; the module's own name declines the feature requests. **The split:
operators are language, functions are library.** `@`, `@@`, `declared`, `comptype`,
`is`, `as` stay built-in — grammar, the hot path; every named query imports from
`std.introspection`, and **the import is an audit signal**: a file that introspects says
so on its first lines, the same greppable-declaration property `use` gives effects.
**The module is capability-free, recorded as a theorem, not a choice**: every export is
a pure read of static declaration data, and a surface that can neither mutate nor
resolve names has no authority to protect. Comptime-tier exports fold exactly as
builtins (modules resolve at compile time, the R19 argument). **The five principles**
(introspection §1), each already latent in scattered rulings, now pillars: (1)
introspection, never reconstruction — rebuilding is `apply` plus initializers (R125),
never a query; (2) **no name→value resolution, ever** — no registry, no `forName`
(R19), every query takes a value already held; (3) declarations, never values — the
R126 encapsulation boundary, promoted; (4) results are inert value-semantic snapshots —
no live link into the closed type universe; (5) **nothing grantable, nothing
bypassed** — capability-shaped results are inert descriptors (the `gatesOf` precedent),
never authority, and no query opens a side door: grants hold, constraints check,
secrets stay sealed, the closed member space stays closed (dynamic member access by
runtime name never exists). Plus the home rule: kind-specific value probes live in
their kind's spec (`gatesOf`/`canReveal` in secret, `toTable`/`wasThrown` in errors);
this module owns the declaration level. The sound content carried over intact (the two
tiers, `comptype` and its confinement, `@P`-by-decomposition); the audit-condemned rows
carried **with explicit in-place flags** naming their re-derivation slices (§7):
`TypeKind` re-derived from the closed universe, `fields`/`attributes` re-grounded
(errors/enums/constraints; the dead `@P` domain), the proto member surface, the
fn-value cluster (R88's capability set + R108's parameter names, inert descriptors),
`baseOf` (subsuming `constraintBase`), the `unionMembers`/`T!`/alias-`typeName` pins,
and the §5 worked example rewritten onto the ratified `comptype` pipeline. En-route
fixes: enum.md's live `IOError` → `ioError` (missed by R122's sweep — enum.md was not
on its file list); overview/types.md's `@@`-over-"table or view" and two stray "views"
cites; internals' "`super` companion" → `declared` and "view types" → `@P` refinements;
constraints §8's dangling "detailed type-reflection API is deferred" now points at the
module. Swept (cites `reflection §3.x` → `introspection §4.x` and prose):
`serialization.md`, `any.md`, `json.md`, `attributes.md`, `is.md`, `enum.md`,
`operators.md`, `keywords.md`, `type.md`, `compiler.md`, `functions.md`,
`constraints.md`, `errors.md`, `protocols.md` §8 (retitled), `associativity.md`,
`variables.md`, `internals/internal-representation-of-variables.md`,
`overview/types.md`, `overview/high-level-overview.md`, `examples/serialization.md`,
`index.md` (stub row + std row), this file's tail (the R88 open now names its slice).

**R128 — the `kind` enum: nineteen closed variants, `kindOf`, and a fourth fossil
layer.** The first API re-derivation slice. **Names**: the enum is **`kind`**
(lowercase — `TypeKind` was a PascalCase island, the R122 argument) and the function
is **`kindOf`** — the two cannot share one name (a module exports one binding per
name), and `kindOf` joins the `*Of` query family (`gatesOf`, `baseOf` pending).
**Spelling**: the old enum was illegal by Luna's own lexer — `fn`, `capability`, and
`constraint` are reserved words (keywords §1) and cannot be variant names. The rule:
**where the natural name is a reserved keyword, the variant appends `Type`** (`fnType`,
`errorType`, `enumType`, `constraintType`, `capabilityType`); everything else keeps its
bare name — including `type` and `protocol`, which are **not** reserved (`type` is a
predeclared identifier, keywords §5; the keyword is `proto`), so the old `typeType`
dodge was over-caution. Predeclared names are safe as variants because variant
references are **fenced** (enum §3.3, R20) and resolve against the expected enum, never
lexical scope — the precedent being the catalogue mode enum's `values` variant beside
the live `values` function. **The derivation** (from keywords §5's closed predeclared
list, no ellipsis — a closed universe deserves a closed, exhaustively matchable enum):
`scalar` (int, double, bool, **string** — ruled a scalar, the enum dispatches
structural queries and a string walks like an atom — null, undefined, never, and the
committed tower primitives as they land), `bytes`, `table`, `stream`, `sink`,
`promise`, `command`, `regex`, `secret`, the five `*Type` dodges, `protocol`,
`refinement`, `union`, `type`, `any`. **`union` retained over the initial instinct to
drop it**: `@x` never yields a union (R18, concrete typeids — the instinct was exactly
right for the `@` path), but `declared x` does, `number` and `iterable` are predeclared
union aliases, every `T!` is one, and `unionMembers`' guard *is* `kindOf(t) ==
{union}`. `list` correctly absent (a constraint, R10 — and the reconciliation with
tables §2.1 recorded: a value that entered `list` carries the constraint typeid, so
`typeName` says `"list"` while `kindOf` says `{constraintType}`, a constraint name, not
a kind). No `intersection` variant: `&` normalizes at interning (type §3.1, R25), with
**one pin deferred** — the kind of the mixed normal form (`list & @drawable`).
**The fourth fossil layer**: the carried §4.5 claimed "`@P` is not a `type` value, it
has no `typeid`" while *citing* type.md §5, whose literal title is "Every type-position
form is a `type` value, **including `@P`**" — a pre-R25 relic contradicting R25,
type §5, and protocols §6 (refinements intern canonicalized protocol-set typeids).
§4.5 rewritten on the corrected foundation: `kindOf(@P)` answers `{refinement}`; what
survives is that membership stays the applied-set test on the value, `@x` never
*reports* a refinement, structural queries have nothing to walk (decomposition via
§4.4), and dispatch is `match`. En route, more pre-R95 residue cleaned from type.md:
`view` in §5's universe list, "views spec" cites, a stale "protocols §9" cite, and
§7's meta/view-vocabulary passage (the old `b->P`-produces-a-view story) rewritten in
member-model terms with `?->` and the qualified form. Swept: `introspection.md` §2,
§3, §4.1 (`kindOf` signature and bullet), §4.3 (rewritten), §4.5 (rewritten), §5, §7
(slice closed, pins extended: mixed-intersection kind, refinement protocol-set query);
`type.md` §5, §6 (the Kind bullet), §7/§7.1 (de-viewed); `reflection.md` (stub map row
annotated).

**R129 — the protocol member surface: declarations fully visible, granted values
readable, ungranted values sealed.** The second and third re-derivation slices land
together. **The foundational split** the ruling turns on: "introspecting a table's
members" conflates two reads with different safety profiles. Member **declarations** —
ungranted included — are fully visible: a declaration is source structure the author
published by writing it, and knowing `stringBuilder` has a private `var buf: bytes`
discloses nothing about any table (pillar 3's exact line, now exercised). Member
**values** split by grant. **Ungranted values stay sealed** — the ruling's one refusal,
against the initial trial balloon that reads are harmless because mutation is
impossible. The mutation half is indeed structurally dead three times over (immutable
typetable declarations, inert results, no spelling for a grant change — no
`setAccessible` is *possible*), but disclosure is an independent, live danger the
corpus already legislates against twice: serialization excludes ungranted members
*because* emitting them would disclose (protocols §5), and secrets gate reads, not
writes. A read-side bypass would make `get` advisory — privacy by discipline, not
construction — and because grants never change retroactively, an ungranted member is
*permanently* private by declaration; introspection reading it would be the one place
the language lets that declaration lie. **Granted values become generically readable**
via the `read` accessor: each `get`-granted row of `members(p)` carries
`fn (t: table): any`, the `constraintPredicate` pattern (the declaration hands you its
public reader *to run*), mirroring `->` exactly (`undefined` on an unapplied table,
§3.2; the uniform value for definition-fixed members). No name→value resolution occurs
(the accessor comes from the declaration you hold, pillar 2) and **no setter accessors
exist** — introspection is read-only, and generic writing is not a walker's need
because rebuilding is `apply` plus initializers (R125). This closes the R127
asymmetry: user-space generic walkers (pretty-printers, differs) now have exactly the
power of R125's builtin dynamic writer — the granted surface, nothing more. **The
surface**: `members(p)` (runtime tier — `@@t` protos are runtime values; rows: name,
`binding`, `get`/`set` bools, type, `required` = no-default, `definitionFixed` =
const-with-default, `read`, `attributes` reserved against the `jsonTag` deferral) and
`requirements(p)` (**direct** requirements only, proto values held never summoned; the
transitive closure is a caller's fold), both in **declaration order** — resolving the
last pre-R95 ordering pin (deterministic generated output; tables are ordered, proto
blocks read top to bottom). The **`binding` enum** (`constBinding` / `letBinding` /
`varBinding`) dodges the reserved ladder words per R128's suffix discipline; a
semantic vocabulary (`{mutable, fixed, frozen}`) was rejected as a second name-set for
a ladder users know by keyword. **The re-groundings**: `fields(t)` scoped to **error
types** (the one nominal declaration form with fields; empty everywhere else — enums
via `variants`, constraints via `constraintBase`/`constraintPredicate`, refinements
decompose, anonymous shapes via `comptype`), and `attributes(t)` scoped to
errors/enums/constraints (protos are not types). Swept: `introspection.md` §4.2 (both
flagged bullets rewritten), §4.4 (the surface), §7 (two slices closed, ordering pin
resolved); `protocols.md` §8 (the `members` pointer line).

**R130 — the function axis: `capabilitiesOf`, `params`, signature decomposition; the
`fn ≡ fn (...any): any` equation rejected.** The fn-value cluster lands
(introspection §4.6), and with it the **last review-era open closes** — R88's
"reflection-visible in principle" becomes an actual query. **The governing split was
already law**: functions §3.2 says verbatim that the capability "requirement set rides
the value, not the type," and the ruling generalizes it — *the signature lives in the
type; names and capabilities ride the value* — with one sharpening recorded: **erasure
is a declared-position phenomenon, never a value phenomenon** (function types are not
erased, `@f` reports the full signature, typeids are concrete, R18; a bare-`fn` slot
erases what the slot knows, not what the value is), and errorability sits in the type
even at the wildcard tier (`fn`/`fn!` split on it alone). **`capabilitiesOf(f)`**
returns the *declared* requirement set as **capability types** — `gatesOf`'s exact
twin, inert by construction (a type can never appear in a `use` clause, so nothing
grantable leaks; the user's own framing: nothing important leaks because the values
cannot be used) — with delegation invisible (R112) and frame grants absent (not part
of the value). **`params(f)`** is the home R108 promised its metadata: names are value
metadata *by force* — structural typing makes `fn (x: int)` and `fn (y: int)` one
type, yet named arguments bind through erased values at runtime, so the names
demonstrably ride the value — rows name/type/optional/variadic in declaration order
(R129). **`paramTypes(t)` / `returnType(t)`** decompose fn *types* for
no-value-in-hand codegen (walking `members` rows' fn-typed member types), `null` on
the wildcards — the existential has no signature to report. **The rejected equation**,
recorded in place: bare `fn` is not `fn (...any): any`. The property the equation
seems to buy already holds operationally (erased calls are statically accepted,
dynamically checked — `arityError` / `typeError` / `namedArgumentError`), but the
equation is unsound twice over: contravariance would make *no* concrete function a
subtype of the wildcard (every argument list would have to fit each concrete
signature), and the spelling belongs to a real citizen — `fn (...args: any): any` is
R108's declarable `println` shape, the *most-accepting* signature, where the wildcard
means the opposite quantifier, an existential, soundly formalized as the interval over
the function typeid region (type §7.1). One spelling would conflate ∃ with the top
signature: **`fn` calls like `(...any): any`; it is not typed as it.** Swept:
`introspection.md` (intro, new §4.6, §7 slice closed), `functions.md` §3.2 (the
`capabilitiesOf` pointer on the requirement-set law), this file's tail (the review-era
opens list now reads *none*).

**R131 — `baseOf`, `{intersection}`, `protocolsOf`, the sugar pins: the introspection
re-derivation closes.** The final slice; introspection §7 is now a closure record and
**nothing is open in that spec**. **`baseOf(t): type?`** is the general
refinement-parent query — a constraint's base (`byte` → `int`, `list` → `table`,
`json` → `string`), an error type's parent (`fileNotFound` → `ioError`; the root
answers `null`), an enum variant's enum (`@Shape.circle` → `Shape`, resolving enum.md's
standing recovery deferral on the **general** side, `enumOf` retired unminted), and a
refinement's or mixed intersection's base (`@person` → `table`); atoms, unions, and
`any` answer `null`. It **subsumes `constraintBase`** — one query, one question — with
every cite swept. **The mixed-intersection pin resolves with a new kind**:
`{intersection}` names exactly the mixed constraint-and-protocol normal form
(`list & @drawable`), the *only* form that survives R25's normalization as an
intersection (pure constraint meets conjoin to `{constraintType}`, pure protocol meets
to `{refinement}`); it is genuinely both, so its kind privileges neither and dispatch
queries both halves. The proposed third variant **`{complex}`** ("union and
intersection both involved") was **rejected**: `&` distributes over `|` at interning,
so the outermost form is always a union whose members are intersection-free-or-mixed —
`kindOf` reports the outermost constructor and `unionMembers` recurses, which answers
what `{complex}` would double-encode — and the name collides with the committed
numeric type `complex` (a future `{scalar}`); the enum is now **twenty** variants,
still closed. **`protocolsOf(t): list`** completes decomposition: a refinement's set,
a mixed intersection's protocol half, `[]` for everything else — total, mirroring
`@@`'s R126 totality, protos held through the type in hand (pillar 2). **The sugar
pins**: `unionMembers` decomposes the sugar because the sugar *is* the union mechanism
(`?` is `| null`, `!` adds the error arm — the governing small-surface idea) —
`unionMembers(int!)` is `[int, error]`, `unionMembers(int?)` is `[int, null]`, no
special case because no special type. And **`typeName` never shows alias names** —
the answer the "could `baseOf` own this?" musing dissolves into: **forced, not
chosen**. Aliases are pure sugar (R21), `iterable` and `table | stream` are one
typeid, one typeid has one name, and with two aliases for one type a display-name slot
would have no sound owner; `baseOf` cannot carry it either, because a union refines
nothing (`baseOf` answers `null`, not a spelling). The alias name lives in source; the
canonical structural spelling lives in output. **The §5 worked example** is rewritten
on the module's own surface — a ten-line generic `describe` walker (imports as audit
signal, `@@`'s totality, `members` rows, the granted-only `read` accessor) whose
closing observation is the whole design: it *could not* see more if it tried. Swept:
`introspection.md` §4.1 (`typeName` alias rule, `unionMembers` sugar rule,
`constraintBase` → `baseOf`), §4.2/§4.3 (cites, the twenty-variant enum, the
`{intersection}`/`{complex}` note), §4.5 (`protocolsOf`), §5 (the example), §7 (the
closure record, external riders named: `jsonTag` reach, json §4/attributes §6.3; the
deferred tower's numeric questions); `constraints.md` §8; `enum.md` (the deferral
resolved).

**R132 — `std.time`: built-in `duration`/`instant`, one monotonic clock, `sleep`; the
timeout surface is unblocked.** The R120 deferral record becomes a real module — the
named prerequisite of the top post-alpha priority delivered. **The types are
built-ins, and that is forced**: the essence of a chrono-style library is dimensional
safety (`instant + instant` must not compile), Luna has no generics and no operator
overloading, and the R120 record's own lean ("the constraint idiom, as everywhere")
does not survive contact with arithmetic — a constraint over `int` erodes on first use
(`byte + byte` widens) and can forbid nothing dimensional. So the built-in-only
operator rule becomes the feature: only built-ins get operators, make them built-ins —
the path the tower already reserves (breaking-change-before-1.0, universe-growth
tolerance, tower §6). The **dimensional operator table** (time §2) is closed and its
compile errors are the safety: `duration ± duration`, `-duration`,
`duration * int`, `duration / int`, `duration / duration = int` (the ratio),
`duration % duration`, `instant - instant = duration`, `instant ± duration`; nothing
else — no point addition, no time-squared, no bare-number mixing, no `double` scaling
(a policy question deferred with datetime's needs). **Representation: 64-bit signed
nanoseconds** (one inline word, ±292 years, overflow panics never wraps, signed
either-order subtraction). **One clock, monotonic** — "high resolution" ruled a
quality of implementation, not a second type, killing cross-clock subtraction by
construction; `now() use (time): instant`, arbitrary epoch, order-and-subtract only.
An `instant` has no data form that survives the process, so `toJson` refuses it (the
`fn` precedent); a `duration` serializes as its canonical string, round-trippable via
`parseDuration`. **The `time` capability gates both effects** (clock read and
`sleep`) — recorded as a theorem, not a taste: eligibility is empty-requirement-set,
every ineligibility source is a capability (R43, capabilities §10), and a build must
not depend on when it runs; the record's is-sleep-gated open closes on the same
argument. Corollary recorded: `now: fn (): instant` parameters make virtual time the
path of least resistance. **`sleep(d) use (time)`** is a suspension point *always*
(cancellation delivers, R115; zero/negative returns immediately but still suspends —
`sleep(seconds(0))` is the portable yield point), sleeps at least `d` monotonically,
absorbs `EINTR` (R121). **Wall-clock time is exiled to `std.datetime` entirely**
(user-ruled: nothing is lost — "what time is it" was always a calendar question),
which kills the monotonic-versus-wall conflation bug structurally, and the seam is
fixed: `duration` is the shared currency datetime will consume. Units: constructors
`nanoseconds`…`hours` (total, pure, comptime-foldable; the ladder stops at `hours` —
a "day" is a calendar claim); extraction is the `whole*` family (truncation named in
the word; no `toSeconds` — one name one signature, and `to*` would hide the loss the
spelling states, the R106 discipline); `parseDuration(s): duration!` joins the
`parse*` family; `toString` is Go-style compound units, exact. Deliberately absent,
each with its reason: days-and-up, a second clock, `tick` streams (an R102 stream
producer, deferred so its shape follows the timeout surface), deadline objects
(patterns layer). Swept: `std/time.md` (rewritten in full), `keywords.md` §5
(`duration`/`instant` predeclared), `introspection.md` §4.3 (the `{scalar}`
as-they-land clause), `operators.md` (`+` and `-a` rows), `concurrency.md` §8 (the
timeout open now reads **unblocked**), `index.md` (the row). The timeout / `awaitAny`
/ select session is next, sitting directly on `sleep`.

**R133 — `std.datetime`: mandatory timezones, two arithmetic families, the DST
policies ruled.** The calendar module lands on R132's seam exactly as contracted
(`duration` the currency; wall reading exiled here). **Three commitments**: every
datetime has a timezone, always — no zoneless value exists or ever will (C#'s
`DateTimeKind.Unspecified` is the recorded cautionary tale; the one legitimate
pressure, recurring civil events, is deferred as a future `date`/`timeOfDay`
component pair, never a zoneless datetime); **immutable by grants** — the `datetime`
protocol's members (`epochSeconds: int`, `nano: int`, `zone: @timezone`, all
`const get`, no `set` anywhere) make interface immutability free (protocols §2.2 +
COW), derivations returning new values via the factory pattern; **no operators** —
named functions (`difference`, `isBefore`/`isAfter`, `add*`), consistent with
protocol-tables and the built-in-only rule. **Representation**: seconds-since-epoch +
nanosecond-of-second (two integer members, ~24 bytes of heap-backed protocol state
like any table's — the Go model, the PHP size) — a single `duration`-since-epoch was
rejected because int64 nanos spans only 1678–2262, amputating history.
**Strict `==` includes the zone** (the one-boundary rule: all three members granted),
with `sameMoment(a, b)` as the named zone-blind instant question. **The default zone
is UTC and the language decided it**: `localZone()` is an environment read under the
`time` capability, so a pure constructor structurally cannot default to local —
"local" is always a visible opt-in in the `use`-auditable position; the user's
local-vs-UTC split dissolved on Luna's own rules. **Zones versus offsets**: one
`timezone` type, two origins (`zone(id)!` with full IANA rules; `offset(d)` fixed and
DST-blind; `isFixed` distinguishes), with the ISO-text trap recorded (parsed
datetimes hold offset-zones; `withZone` is the upgrade). **The tzdb is bundled with
the runtime** (the Go backend's `time/tzdata` makes it nearly free), which makes
`zone` **pure and comptime-eligible** — `const chicago = zone('America/Chicago')`
folds and a typo is a compile error; a `timezone` **enum was rejected** (~600 ids,
renames in nearly every tzdb release = perpetual breaking changes; ids arrive as
runtime data anyway; comptime folding already delivers the enum's static checking
without its breakage). **The two arithmetic families** — absolute (`add`/`subtract`
by `duration`, exact physics) versus calendar (`addDays`/`addWeeks`/`addMonths`/
`addYears`, zone-aware) — with the three policies ruled: **month-end clamps** (Jan 31
+ 1 month = Feb 28/29, the C# answer); **DST gap: lenient shift-forward**; **DST
overlap: earlier wins** (the first occurrence in real time — the java.time/NodaTime
convention; the user's earlier-or-later question answered on convergence and
physicality), overridable by named argument (`onAmbiguous: {later}`), policy visible
per R106/R108. **Derivation**: `next`/`previous` over the **`weekday` enum** (ISO,
Monday-first — ruled once, keeping the module out of locale space; months stay `int`
1–12, arithmetic operands more often than names), `startOf*`/`endOf*`, `withZone`.
**Construction**: `create(...): @datetime!` (errorable — the user's call, wide
data-shaped failure surface), `now(zone = utc) use (time)`, `fromUnixSeconds!`
(R106's `from*`), `parseDatetime!` (ISO 8601/RFC 3339; **PHP's format specification
is the planned, deferred adoption** for custom formats — noted, stolen later).
**Serialization is deliberately not special** (user-ruled): a datetime serializes
like any protocol table (R125), and the wire idiom is `toString` into the field you
mean — ISO 8601 with offset, round-trippable. **Leap seconds do not exist** (unix
semantics); the calendar is **proleptic Gregorian**, named as such. One capability
(`time`) covers both effects (`now`, `localZone`); everything else folds. Swept:
`std/datetime.md` (new, the full module), `std/time.md` (three deferred-pointers now
landed-pointers), `index.md` (the row). Deferred with reasons: PHP format spec,
sequences/recurrence (a separate stream-shaped library), floating civil values,
locale, non-Gregorian calendars.

**R134 — `std.system` retired: `std.process` lands, `std.filesystem` named.** The
alpha-modules audit found the old grab-bag hiding two modules, and the split follows
**R121's own argument**: the record justified separating metadata from `io` because
contents and structure are "different authorities" — and the authority *is the
filesystem*, which is what a `use` clause should say to an audit. So the metadata
surface becomes **`std.filesystem`** under a **`filesystem` capability** (superseding
R121's capability-name half; the io-stays-contents-only boundary stands; the surface
itself is the next ruling, R121's composition constraints carrying unchanged), keeping
every effect module **1:1 with its authority** (`io`/`io`, `time`/`time`) — the
C++/Rust fine-grained precedent over Go's `os` grab-bag. Everything process-shaped
lands now as **`std.process`** (new file): **`args()` under `argv`** (the capability
R43 ruled; the "module home flagged with std.system" row in capabilities §9 resolves
here) and **`envVars()` under `env`, relocated from exec §6** — it sat there because
exec passes environments to children, but reading one's *own* environment is
process-self; the R42 ruling carries unchanged (secret-valued entries, enumeration
and reading separately gated through `reveal`). Two refusals recorded: **no `chdir`,
ever** — the working directory is process-global mutable state, one task's change
re-resolving every relative path in every other task, a data race by OS design, the
shared-mutable-state class the language forbids; relative paths resolve against the
start-of-process directory, always — and **no `exit()`** — the exit code is `main`'s
return, teardown is structured, and no call unwinds the process past pending `defer`s
from mid-frame. En-route finds, both fixed: capabilities §9's table **was missing the
`time` row entirely** (R132 never added it) and its `system` row still claimed the
clock (moved to `time` in R132) — row replaced by `filesystem`, `time` added; and
never.md's worked example was a user-written `exit(code) use (system)` wrapping a
deferred `exitProcess` — contradicting both halves of this ruling — re-exampled onto
a `die`-based `fatal` helper with the no-exit rationale stated in place. Swept:
`std/process.md` (new), `std/system.md` (rewritten as the split record),
`exec.md` §6 (the env bullet now points home), `capabilities.md` §9 (four rows),
`io.md` §9 (the boundary names `filesystem`), `tests.md` (temp-resources deferral
repointed), `never.md` §1, `index.md` (two rows, plus `import std.process;` in the
front-page example), `examples/one-billion-rows.md` and `examples/log-scan.md`
(imports). `pid`/hostname/username deferred inside the module. `std.filesystem`'s
surface is next.

**R135 — `std.filesystem`: the structure surface, the collect-idiom convention, and
the slot-inhabitant rule.** The alpha-blocking module lands. **The import convention
is the ruling's front door**, two lines, each forced by an existing rule: `import
{ filesystem, path } from std.filesystem;` plus `const fs = import * from
std.filesystem;` — names like `delete`/`copy`/`join` are too valuable to dump bare, so
the **importer-collects idiom** (modules §6, the mechanism the user half-remembered)
is the documented convention, keeping the author's exports free functions (the
author-side forced table was rejected: modules §6 assigns namespacing to the
importer's choice, selective import stays available to scripts, and static capability
checking rides direct references). The user's storability challenge produced **the
slot-inhabitant rule, now ruled in modules §6**: collection gathers every export that
can inhabit a table slot — a **capability export is excluded**, because capability
tokens never inhabit value slots (capabilities §3.1, the anti-laundering rule already
on the books) and nothing is lost: `use` clauses name *bindings* (R19), so authority
is only ever useful arriving through the name-dumping forms. Authority travels by
name; data travels by value. Protos, types, enums, fns collect fine (all already
load-bearing as table contents — `@@`, `requirements`, `unionMembers`). Adjacent
wrinkle recorded: annotations take type *names*, never element accesses, so `path`
also arrives bare (pinned against type.md if contested). **The surface**: `path`
**relocated from io §2.0** (io imports it; `isValidPath` still rides std.platform);
the pure half (`join` variadic, `dirname`, `basename`, `extension: string?` — null,
coalesce if you want text — `normalize`), comptime-eligible, the
pure-half/gated-half module shape for the third time running; the gated half under
`use (filesystem)` — `exists` (the probe form, never errors, `false` covers absent
*and* inaccessible, **advisory by declaration**: check-then-open is a race, the
open's own error is the truth), `stat` following symlinks into a get-only
`@fileInfo` (`size`, `modified: @datetime` UTC — the R133 payoff — `kind` as the
`entryKind` enum; **permissions deliberately absent**, the model deferred whole),
`entries`/`walk` as non-restartable stream producers yielding **paths only** (ruled:
smallest surface; the runtime may cache d-type internally) under the
creation-authorized-carrier law (R121), `walk` **never following directory symlinks**
(cycle safety is not optional), `createDir(recursive:)`, **`delete(p, recursive:
true)` as the visibly-flagged rm-rf** (no separately-named `deleteAll` to reach for by
accident), `rename`, `copy` (file-only; recursive deferred with its
metadata-preservation question), and `tempDir`/`tempFile` — the primitive tests.md
waited on, with the soundness note (entropy inside an already-gated effectful
function is as lawful as `now()`'s nondeterminism) and `tempFile` returning the
opened `file`, not a path (a path would be a TOCTOU invitation). **Errors extend the
`ioError` family** rather than minting a tree (errors classify what failed;
capabilities classify who may — orthogonal): `alreadyExists` already existed
(EEXIST), `directoryNotEmpty` added (ENOTEMPTY). **chmod deferred whole** with the
reasoning recorded: a separate capability rejected (R121's split was
contents-versus-structure and permissions are structure; `delete` is strictly more
destructive and lives here), `exec chmod` the existing escape hatch, setuid flagged
for the eventual permissions scrutiny. Deferred with owners: symlink axis, watching,
recursive copy, `cwd()`. Swept: `std/filesystem.md` (new), `modules.md` §6 (the
slot-inhabitant rule), `io.md` §2.0 (relocation pointer), `io-errors.md` (the new
arm, the moved cite), `capabilities.md` §9 (row closed), `std/system.md` (surface
landed), `tests.md` (primitives exist; only machinery remains open), `index.md` (the
row).

**R136 — the import grid: four forms, two axes, no `*`.** The module-import surface
resolves into a **two-axis grid**, now stated as a table in modules §5 (the user's
request — the table *is* the motivation): *position* decides dump-versus-collect
(statement dumps names; assignment collects a table), *braces* decide all-versus-some.
Statement-all is the bare **`import std.filesystem;`** — legalizing the form every
example already wrote, which modules §5 had never defined (the wrinkle R135 parked
resolves as ratification, not repair) — statement-some is the existing braced form,
assignment-all is `const fs = import std.filesystem;`, and assignment-some is **new**:
`const fs = import { stat, exists } from std.filesystem;`, partial collection with
aliases renaming keys (`t.jsonParse` — §8's one mechanism, one more consumer). Under
the grid, **`import * from p` is retired entirely**: bare-path already means
"everything," so the glob sigil was a second spelling for the first cell — the `*`
disappears from the import grammar (lexer note updated). Four supporting rules:
**assigned imports are `const` only**, load-bearing not style — a const table with
comptime-known members is what lets `fs.stat` fold to statically resolved, statically
capability-checked calls (the `platform.lineEnding` precedent), so `let`/`var` would
demote every call to the dynamic frontier for nothing; assignment-position imports are
**top-level only** like the statement forms (§4, reaffirmed); **naming a capability in
a braced assigned form is a compile error**, not a silent exclusion — an explicit
request for what slots cannot hold (R135) fails loudly, with the fix in the message —
while the bare assigned form keeps R135's silent exclusion (nothing you named is
missing); and **the path never becomes a name** — `import std.filesystem;` binds
nothing called `filesystem`, this is not Python's `import os`, and namespacing is
exclusively the assignment form's job. The §5 coupling warning survives, rescoped:
bare import is the happy path for `std.*` and your own modules; the
entire-export-surface coupling stays worth a thought for third-party dependencies.
`import` is the language's one bare directive, and that is fitting — mechanics, not a
value. Swept: `modules.md` §5 (the grid table, the forms), §6 (partial collection,
const-only, the loud error, the one-line distinction de-globbed), `std/filesystem.md`
(the canonical import updated), `lexer.md` (the `*`-glob note now a retirement
record).

**R137 — constraints go braceless; inline `where` considered and rejected.** The user's
parse observation held all the way down: the braces in `constraint { i: int where … }`
had **no function**. Not parse — the keywords table already carried the proof, `where`
is "never part of a type, which is what lets it terminate one," so the type ends and
the predicate begins with no delimiter, the declaration's `;` ends the predicate, and
a further `where` (reserved, never an expression operator) unambiguously starts the
next conjunct — multi-clause survives braceless with its reason intact (which clause
failed). Not purity — §2's "enforced by the form" was always the *declaration form's*
property, not the braces'. Not content — every other brace-former holds a list (proto
members, enum variants, error fields); a constraint holds one typed binder, and
`capability`, the other fixed-shape form, never had braces. And dropping them
*strengthens* §1's own claim that a constraint body "is exactly a match arm" — arms
are braceless. New form: **`const byte = constraint i: int where i >= 0 && i <= 255;`**
The companion ruling that makes the parse airtight: **constraint literals are
const-initializer-only, stated explicitly** (the R126 proto precedent — §1 said
"bound to a const" but never forbade expression position), with **name-capture now
stated**: the binding identifier is the constraint's name, what `typeName` reports and
a failing check cites — a rule the corpus relied on (`"byte"`, `"json"`) without ever
writing down. **Inline `where` in signatures — anonymous constraints,
`fn (n: int where n > 0)` — considered and rejected** (constraints §1.1, a tombstone
because PHP/TS-lineage users will ask), three times over: it breaks R79
*structurally* (the inline predicate sees sibling parameters, so
`fn (lo: int, hi: int where lo <= hi)` — not a function of the value alone — becomes
representable and needs a rule, where the named form makes it unrepresentable by
construction); it forces a typeid identity crisis (per-occurrence minting makes
identical signatures different types against R130's structural rule; interning
demands alpha-equivalence over predicate ASTs — permanent machinery for a
convenience); and it optimizes away the feature (the name is the API — `typeName`,
the panic's citation, the third caller's reuse). Rejection costs one reusable line,
cheaper now. Swept (28 sites, 14 files): `constraints.md` (§1 rewritten with the
braceless rationale, const-only, name-capture, §1.1; §2's wording), `json.md`,
`types/json.md`, `serialization.md`, `csv.md`, `yaml.md`, `xml.md`, `filesystem.md`
(`path`), `secret.md` (`dbSecret`), `double.md`, `int.md` (the sized-int family),
`bytes.md`, `tables.md` §8, `overview/types.md`, `keywords.md` (the `constraint` row,
the `as` row's R87 note, the `where` row crediting its terminator property as the
enabler).

**R138 — `std.platform`: one export, target facts, the tension inverted.** The most
load-bearing stub in the corpus becomes the smallest module in std, deliberately: one
export, the `const` **target-facts** table (`os`, `arch`, `lineEnding`,
`pathSeparator`). **The comptime tension dissolves on one distinction**: these are
facts about the *target* — an input to the build; in script mode the host is the
target — never the build machine's ambient environment. So comptime access is not a
portability hazard but **conditional compilation done right**: the same source
compiled for linux folds `"\n"`, for windows `"\r\n"`, each binary correct for its
target, reproducible under the only definition that survives cross-compilation (same
source + same target = same binary); and the flip side is decisive — without
comptime-visible platform facts, portable libraries are unwritable. No capability,
recorded as **the R43 theorem run in reverse**: capabilities mark what must not fold;
deterministic per-build inputs are eligible, nothing to gate. **`os`/`arch` are
strings, not enums** (the Go `GOOS`/`GOARCH` vocabulary): an exhaustive `match` over
an `os` enum would break on every toolchain release that adds a target — the R133
enum-growth hazard arriving through slow growth rather than volatility; strings grow
silently, and one matches an open set with a wildcard arm. **Module over ambient
global** (considered and rejected): predeclaring the table would erase the audit
trail — platform-dependence is exactly the cross-cutting concern a portability review
wants greppable, the R127 import-as-audit-signal argument — and the cost is nothing,
since under R136's grid `import std.platform;` dumps exactly one name and the table
is its own namespace, pollution-free by construction (the user's
global-const-table instinct, landed as the module's shape). **The debtors settle**:
`isValidPath`'s deferral chain (io → filesystem → platform) terminates — it is
`std.filesystem`'s export, comptime-branched on `platform.os`, and for the sole
current target the rule is real: nonempty, no NUL byte, ≤ 4096 bytes; errno
portability stays io-errors' own question. Deferred: endianness, word size, CPU
features — FFI-era facts; a row is added only when a *target* fact earns it. Swept:
`std/platform.md` (rewritten from the R121 record), `std/filesystem.md` §1
(`isValidPath` ruled), `std/io.md` §2.0 and §10 (the stub note now a landed
pointer), `index.md` (the row).

**R139 — `std.random`: seeding is the effect, generation is pure; the catalogue's
`randFn` was doubly unsound.** The last non-network alpha gap closes on one idiom —
one capability-gated entropy read at the top, a pure deterministic stream everywhere
after — whose consequence is a feature no bolted-on RNG gets: **every randomized run
is replayable by logging one int**. **Two findings fixed the shape.** First, the known
one: the catalogue's optional `randFn?` on `random`/`shuffle` defaulted the randomness
source to nothing specced, and an ungated nondeterministic default breaks the R43
theorem — a capability-free `shuffle(t)` would fold a "random" order into the binary.
Second, the new one: **a function-shaped PRNG is unimplementable in Luna** — a PRNG
carries mutable state between calls, closures capture a `const` snapshot (functions
§2.1), so a stateful counter closure cannot exist; stateful sequence generation has
exactly one home, the generator, which is to say a **`stream`** (the user's phrasing,
now the spec's: stateful functions *are*, semantically, streams — and O(1) state
besides). So `randFn` was mis-typed the day it was written, and the catalogue
parameter becomes **`rng: stream`, required** (`random`/`shuffle` respecced; the
retired-spellings table gains the row) — optional-with-default was the unsoundness,
since a pure function cannot conjure entropy and a `use` set cannot be conditional on
an argument's absence. **The surface**: the **`entropy`** capability (not `random` —
the catalogue's picker owns that name, and the authority *is* the entropy source);
gated `randomSeed`, `randomBytes` (the secure path — OS entropy, never the PRNG;
tokens wrap `as secret`), and **`randomStream()`** — the everyday one-call spelling,
which exists because the convenient alternatives died under scrutiny: **the seed
default was considered and rejected** (§6, a tombstone) — an entropy-read default is
structurally impossible (a capability set cannot be conditional on argument presence;
defaults are comptime-known, functions §3.3.1), and a constant default is the C
`rand()` footgun verbatim, the same sequence every program every run, shipped as the
convenient spelling. Pure side: **`prng(seed)` with the algorithm pinned as contract —
PCG-64** (Go `math/rand/v2`'s choice, backend-free; pinning matters because
comptime-folded values and seeded tests must not change across toolchain releases),
**no engine zoo** (PHP's Mt19937/Pcg/Xoshiro/Secure menagerie is compat history not
inherited — one pinned engine plus one secure source); `nextInt` (unbiased, rejection
sampling — nobody ships dice that roll low), `nextDouble` ([0,1), 53-bit), `nextBool`,
each visibly consuming the by-reference stream. **Restart is replay, reseed is
rebinding**: a PRNG's source is its seed, an immutable int, so `restart(rng)` replays
the identical sequence — the *exemplar* of R105's immutable-snapshot rule (a range
restarts from `lo`, a PRNG from its seed), holding for `randomStream` too (its
snapshot is the seed drawn at creation — restart the rng, replay the exact randomized
execution); fresh randomness is an ordinary new stream, never an in-place reseed —
"same sequence again" and "different sequence now" never share a spelling. `prng`
with a fixed seed is comptime-eligible (reproducible test data folds); the gated
three are ineligible by the same theorem, landing the R43 fix exactly where it
should. Deferred: distributions and sampling utilities (library territory over
`prng`/`nextDouble`), cryptographic constructions (`randomBytes` covers secure
material). Swept: `std/random.md` (new), `iterable-functions.md` §2.9 (`random`
respecced; the retired-spellings row), `indexable-functions.md` §4 (`shuffle`
respecced), `index.md` (the row).

**R140 — the PCG security analysis on the record; `std.crypto` named and deferred,
deliberately.** The user's lock-in scrutiny of PCG-64 produced three permanent
records. **PCG-64's security is zero, settled, and irrelevant by design**: it is a
statistical generator with no cryptographic claim, and prediction is not an open
question — practical full-state recovery from a handful of outputs is published
(Bouillaguet, Martinez & Sauvage, 2020). **The seed theorem** (random §3.1, new):
`prng(seed: int)` has a 64-bit seed, so *no* engine in that slot could be secure — an
attacker brute-forces 2⁶⁴ seeds offline regardless of the algorithm, and the module's
headline feature (every run replayable from one logged int) is precisely the property
a security RNG must never have; the slot is structurally non-secure, so engine choice
is a statistics-and-speed question only, which is what makes the PCG pin a *low*
lock-in risk (statistically battle-tested; the surface is engine-agnostic; the
contract-pin makes any future change a visible versioned break in the pre-1.0
window, never silent drift). **The Go attribution precised** (random §3): PCG is
`math/rand/v2`'s *seedable* engine; Go's auto-seeded default is ChaCha8, a
misuse-hardening chosen because Go's rand API predates the secure/statistical
split — legacy pressure Luna does not have, because this module *is* the split.
**`std.crypto` is named and deferred in full** (new record, std/crypto.md; the
user's rationale verbatim in spirit: crypto surfaces are really tight stuff and poor
decisions last a very long time — an ecosystem inherits its crypto API's mistakes
for decades, the mcrypt precedent) — designed as its own dedicated effort, never in
passing. The record fixes what exists today (`randomBytes` for secure material, the
`secret` type for containment, the exec hatch) and the constraints any future design
inherits: capability-gated effects, `secret`-shaped key material, and the permanent
statistical/secure split — nothing in `std.crypto` will ever be seedable-for-replay,
nothing in `std.random` will ever claim security. Swept: `std/random.md` §3
(attribution), §3.1 (new), §7 (crypto pointed home), `std/crypto.md` (new),
`index.md` (the row).

**R141 — `std.math`: the alpha standard library completes.** The shortest real module
after platform, everything pure, capability-free, comptime-eligible, IEEE-sentinel
throughout (`sqrt(-1.0)` is `nan`, nothing errors, nothing panics — double.md's
stance applied). **What it deliberately does not duplicate** frames the module: the
catalogue owns collection aggregates (`sum`/`average`/`product`/`min`/`max`/`mode` —
so `mean` is not added, it is `average`), double.md owns the special-value probes and
the rounding **policy verbs** — and a double-returning `floor`/`ceil` family is
**recorded absent**: the policy verbs own rounding, and a parallel float family would
put two meanings behind one name. **The surface**: `pi`/`e`; `abs`/`sign`/`clamp`
over `number` with **kind following the operand** (the R92 precedent; `abs(minInt)`
panics, the int rule); **`lerp`** (user-added — simple, very useful for graphics)
with the C++20 endpoint discipline (`lerp(a,b,0)` and `lerp(a,b,1)` exact; `t`
unclamped, extrapolation is the graphics convention, compose with `clamp`);
`sqrt`/`hypot`/`pow`/`exp` — `hypot` stating the module's inclusion criterion, a
function earns a row when the obvious composition is a trap (`sqrt(x*x+y*y)`
overflows); **`ln`, not `log`** (bare "log" is ambiguous between mathematics and
engineering, so no export bears it; `log2`/`log10` round out three
precision-optimized names, a general base being a one-liner); trig in **radians
only** with `toRadians`/`toDegrees` at the boundary (R106's `to*` contract exactly);
and **statistics split by the R92 retention rule**, which slots them perfectly —
`variance`/`stddev` take `iterable` (single-pass Welford, stream-legal, `sample:
bool = false` for Bessel's correction), `median`/`percentile` take `table` only (they
sort; whole-input family), empty input yielding `nan`, the IEEE answer to an
undefined statistic. Deferred until earned: hyperbolics, `gcd`/`lcm`, combinatorics,
integer `pow`. Swept: `std/math.md` (new), `index.md` (the row). **With this, the
alpha standard library is complete below the network tier**: io, json, time,
datetime, introspection, process, filesystem, platform, random, math, secret, exec,
channels — all landed; what remains is the designed-last networking sequence, the
timeout / `awaitAny` / select session and `std.net` behind it.

**R142 — the timeout surface: `awaitAny` is the one primitive, scope exit was the
cancel all along; the top priority delivered.** The ruling twenty rulings built
toward (concurrency §5.1, new). **The central proof**: timeout is a *library
function* with zero new cancellation machinery, because R115's scope-exit rule was
the missing cancel primitive — `timeout(f, d)` spawns the work and the `sleep(d)`
timer *in its own frame*, races them, and its **return is its scope exit**, so the
loser is cancelled by the already-ratified rule; there is **still no user-facing
`cancel(p)`** (R115 reaffirmed — the race adds no third canceller, only a new reason
for scope exit to fire), and raw `spawn` stays timeout-free (the user's instinct:
deadlines live in scope-owning wrappers, because scope ownership is what makes loser
reclamation fall out for free). **The primitive**: `awaitAny(...ps: promise): [int,
any]!` — first completion wins (already-completed → immediate; ties by position),
winner's error propagates on §4's per-promise channel, **losers not consumed and not
cancelled** (observed, still awaitable, still scope-owned). **The family over it**:
`awaitTimeout(p, d)` (timer frame-local and reclaimed; `p` consumed if it wins,
untouched and still the caller's if it loses) and `receiveTimeout(rx, d)` — the
channel-recovery form the BEAM scrutiny demanded: the one admitted deadlock shape,
channel-wait cycles, is now *escapable*, not merely contained (channels §6 amended;
the `GenServer.call`-with-deadline pattern is buildable, channels §7). All three
yield **`timeoutError`** — declarable, never a panic: a timeout is expected,
recoverable, data-shaped (errors §2). **The contract, stated loudly**: timeout
bounds *waiting*, never *execution* — the caller always unblocks; the loser stops at
its next suspension point, and a suspension-point-free loop has none (§7's accepted
cost, the Go backend cannot kill, R120; `sleep(seconds(0))` is the yield-point
discipline) — the one place Luna is honestly weaker than BEAM, traded knowingly.
**Go-style `select`: mostly dissolved, remainder deferred** — Go needs select
because bare channels are the primitive and fan-in requires choosing; Luna's MPSC
sinks make **fan-in one channel with N senders**, the owner-task pattern *is* the
select loop, residual heterogeneous races get a merge task or `awaitAny` over
wrappers, and a dedicated construct waits for a case that survives the merge idiom —
recorded as a decision, not a gap. **Two supporting rulings in §3.1**: the
**carrier-list extension by the rule's own criterion** — a carrier is legal iff it
structurally cannot retain, duplicate, or export; the variadic's frame-bound
argument list passes the same audit bindings and streams passed, enforcement is
type-directed (`...ps: promise` makes the element type static; existing confinement
checks fire transitively; no new check family, no special runtime representation —
stock Go select underneath), and provenance is closed (user code cannot construct a
promise-bearing list, the literal is the banned store; the variadic calling
convention is the only producer, joining apply-initializer lists and fenced enum
literals in the grammar-constructed-ephemera family). And **a promise is `nocopy`**
(the user's call; the `argv` precedent, legal because built-ins own their binding
semantics): `let q = p;` is a compile error — one handle, one name; parameter
passing is **by reference**, joining the consumable class streams defined (variables
§5); `await` consumes, a second await is a compile error where evident and a
**`doubleAwait` panic** (errors §2, new leaf) through the one dynamic path nocopy
leaves open. Swept: `concurrency.md` §3.1 (both rulings), §5.1 (new), §8 (the
top-priority open is a landed record); `await.md` §1/§3/§4 (surface landed,
no-cancel reaffirmed, `awaitAll` dissolved into §1's stream form); `channels.md` §6
(recovery), §7 (select dissolved-record; patterns layer unblocked); `errors.md` §2
(`timeoutError`, `doubleAwait`); `std/time.md` §5 (the seam closed as designed);
`std/net.md` (**the gate is open** — pending real use alone).

**R143 — `std.net`: TCP and UDP under `egress`/`ingress`; the alpha closes.** The
last module, shaped by two principles stated as its header. **Zero timeout
parameters, anywhere**: every net operation is a suspension point (R121), so every
wait is bounded by the R142 family — Go threads `SetDeadline` through its entire net
API because its timeouts do not compose; Luna's do, and this module collects that
payoff by shipping no deadline in any signature. **Alpha net is plaintext, loudly**:
TLS is gated on `std.crypto` (R140, deferred deliberately), and the chain is fixed
and recorded — **crypto → tls → http**, no link before the one under it.
**Two capabilities, not one** (superseding R121's provisional single `net`):
**`egress`** (originate — `dial`, `send`) and **`ingress`** (bind and accept —
`listen`, `udpBind`), because "may phone home" and "may open listening ports" are
different authorities — the distinction every firewall draws, drawn in the `use`
clause where Luna draws everything; splitting later would break, splitting now cost
two good names. Bytes on established connections stay **`io`'s** — the R134 symmetry
completed: `filesystem` : structure :: `egress`/`ingress` : connections :: `io` :
contents. **A `connection` wears `fileDescriptor`** (its proto requires it, §7), so
io's entire byte surface — `chunks`, `write`, `close`, `defer close` — works on
connections with **no new functions**, per R121's referent-stateful no-`&`
convention; `connection`'s own members are the socket facts, identityEquality,
single-owner, transferred class. **Accept is a stream** — `connections(l)`, R102 —
making the server three lines (`foreach` + `spawn`, handlers scope-bounded), lazy
and creation-authorized (the listener carries the `ingress` grant, the laundering
rule), non-restartable (a socket is the canonical non-replayable source), bounded by
`receiveTimeout` like any wait. **UDP**: `udpBind` / `send` / `datagrams` — the
datagram is the unit (rows of from/port/data), and the capability shape is honest:
one socket can need both authorities, `use (ingress, egress)`, each word meaning
itself. **`port` becomes real**: the constraint that has been the corpus's running
example since R9 is exported by the module that owns it (constraints §1 notes the
graduation). **Errors extend `ioError`** (the R135 orthogonality rule):
`connectionRefused`, `connectionReset`, `hostUnreachable`, `addressInUse`,
`dnsError` — all declarable (network failure is expected, recoverable, data-shaped —
the `timeoutError` argument). Free by composition, stated: backpressure (pull-based
streams — no buffering-policy surface exists because none is needed), cancellation
(every park is a suspension point; R142's contract holds verbatim), credentials as
secrets at the boundary. **`std.http` deferred by decision** (new record,
std/http.md): a whole other beast atop net, its secure half gated two modules deep
by the chain; today's answers are hand-spoken HTTP/1.1 over net or the exec hatch;
future constraints fixed (no new authority — HTTP is a protocol, not a new reach;
R142 timeouts; stream bodies; secret-shaped credentials). Also deferred with owners:
socket options, unix domain sockets, half-close, explicit IPv6 surface. Swept:
`std/net.md` (rewritten from the R121 record), `std/http.md` (new deferral record),
`io-errors.md` (five arms), `capabilities.md` §9 (two rows), `constraints.md` §1
(the graduation note), `index.md` (both rows). **With this ruling the alpha surface
is complete**: every module on the ledger is landed or deliberately deferred with
its reason recorded, and the corpus stands at R143 with no open contradiction.

**R144 — housekeeping: `docs/retired/`, and one more fossil.** Routine cleanup, on
the ledger so future greps understand the moves. The four retirement stubs leave the
live directories for **`docs/retired/`** (git-moved, history preserved):
`table-api.md` (R91–R93), `views.md` (R95), `reflection.md` (R127), `system.md`
(R134) — each a was→is tombstone map, none authoritative, and the rule stated in the
index's new **Retired** section, which replaces their four scattered rows. Path
references swept: `process.md` (the split-record pointer), `indexable-functions.md`
(the R98 retirement cite), `index.md` (consolidated). The deep name-sweep caught
**one live fossil the R95 passes missed**: compiler.md's optimization pass still
read "protocol-dispatch devirtualization (protocols, **views** specs)… resolve the
**meta-function**… instead of a runtime **view lookup**" — pre-R95 on all three
counts, and conceptually stale besides: the member model has **no virtual dispatch
to devirtualize** (protocols §2.1), so the pass is rewritten as **protocol-member
resolution** (static applied-set at `@P`-typed sites → direct access; otherwise the
dynamic check, protocols §3.2/§6.2), with the correction recorded in place. Frozen
history untouched, as always: retired names inside `retired/` stubs and CHANGES
entries keep their spellings.

**R145 — housekeeping: the spec directory is `specs/`.** `docs/` renamed to `specs/`
(a plain `git mv`, its own commit, history preserved — git detects renames at diff
time, so `--follow` and blame walk through unbroken). More accurate: the directory
holds the specification, not documentation; `user-docs/` remains the planned name for
the latter. References swept: `CLAUDE.md` (the repo map, plus its stale
README-mismatch note — README had since been fixed and needed only the rename — and
the orientation-layer paths corrected to `specs/overview/`), `README.md` (three path
lines), `tooling/shiki-luna.ts` (the source-of-truth comment; `make-archive.sh`
needed nothing — it is path-agnostic via `git ls-files`). Two pieces of drift caught
in README's Taste example en route: `defer close(&fd)` predated R121's de-`&`'d io
surface (now `close(fd)`), and the imports predated R134 (`import std.process;`
added for `args()`/`argv`). Rulings before R145 cite `docs/` paths and are frozen
history, per the tombstone rule.

**R146 — the pipeline operator `|>` is retired; commands get `pipe()`.** The
scrutiny's finding: the operator's own §1 condemned a general pipe as "a second
spelling of what `.`-chaining already does" and reserved `|>` for dataflow, "a notion
UFCS cannot express" — **a claim that went false at R91–R93**, when the catalogue made
every transformer a lazy, kind-following, stream-taking free function. Since then
`s |> map(f)` and `s.map(f)` have been the *same operation* (lazy-start, pull-driven,
short-circuiting, source-taking, effects at the pull — none of which were ever the
operator's, as its own §5.1/§5.2 admitted), and mechanically `s |> filter(p)` had to
inject the left operand as the call's first argument — UFCS with different
punctuation. The stream half had become exactly the redundant second spelling its spec
was written to forbid. The command half (`a |> b`, stdout→stdin — genuine semantics,
neither side a function) was one function's worth: it is now **`pipe(first: command,
second: command, ...rest: command): command`** (command §4) — variadic, the two
leading required parameters making a one-stage pipeline *unrepresentable* rather than
checked, every property kept (structured, shell-free, injection-safe, inert, immutable
operands, `commandError.stage` per argument), the name evoking the shell pipe without
spending an operator on one domain. The spec moved to **retired/pipeline.md** with the
why and the was→is map; the `PIPELINE` token is gone (lexer §4, munch lists);
**associativity tier 11 is a tombstone** (the number kept so tier-12 citations
stand); operators.md's two rows deleted; any.md's `|>`-needs-narrowing rule is moot
(`pipe` is an ordinary typed function under the existing UFCS rule). **Stream §7 is
rewritten as "Chains are pipelines"** — same section numbers (widely cited), same
semantics, now stated as what chains always had; §7.1 keeps the explicit
stream↔command bridge rule. **A precision the sweep forced** (concurrency §5): the
old `xs |> map(fn (x) => spawn …)` examples respell as `xs.toStream().map(…)`, and
the `toStream` is load-bearing, not style — kind follows the primary
(iterable-functions §1.3), so a spawning map over a *table* would yield a table of
promises, the retained storage §3.1 bans; the operator's spelling had been hiding the
kind question, and the honest spelling surfaces it. Swept (18 files):
`retired/pipeline.md` (the stub), `command.md` §2/§4/§8, `stream.md` §7/§8,
`iterable-functions.md` (four sites), `stream-api.md`, `any.md` (rule deleted),
`bytes.md`, `range.md` (both examples), `spread.md`, `channels.md`, `json.md` (the
figurative pipe), `concurrency.md` (three sites plus the toStream note),
`exec.md` (three sites, `pipe(a, b, c)` spelling), `associativity.md` (the
tombstone tier), `lexer.md` (token, escape note, two munch lists),
`examples/log-scan.md` (rewritten to the chain), `index.md` (live row → the Retired
table). One process note, honestly: a careless sed corrupted three lines of
concurrency.md mid-sweep; restored from git and redone with exact-match edits — the
committed-before-sweeping discipline paid for itself.

**R147 — nested destructuring lands; the deferral's cost had already collapsed.** The
opens-combing pass begins, and the first question answers itself from the corpus:
destructuring §7 deferred nesting "until real code demands depth," but **match §4 had
already built everything** — the nested shape grammar, the recursion, the binding
semantics, and the declaration that a match pattern is the *strict superset* of a
destructuring pattern. So a `let` reusing that grammar costs nothing new, and the
standing asymmetry (the same pattern legal in a `match` arm, a syntax error in a
`let`, for data shaped exactly like the language's bread and butter) was a wart, not
an economy. **The one genuinely new question** — statements have no next arm to fall
to — resolves by the flat rules' own personalities, stated as one recursion rule
(destructuring §3.1): *a nested pattern destructures the value at its position under
its own mode's rules.* **Keyed stays partial and absence propagates** — a nested
keyed pattern under an absent or `undefined` value binds all its names `undefined`
(§2.1's philosophy extended; what the flat spelling plus a second statement would
have produced; `??` recovers) — while **positional stays exact and asserts at every
level** (§1.1's error, `typeError` panic or compile error where static, at any
depth): `['server' => ['host' => h]]` navigates, `[[a, b], c]` asserts, each mode
keeping the personality it always had. **Tests stay match's**: statement patterns
exclude literals and typed binders at every depth — a failed test needs a next arm —
which is precisely what keeps the superset relation a relation rather than two
homographs (match §4.3). `_` and `...rest` compose per level with flat rules
unchanged (rest trailing-only within its level); streams at any depth follow §1.4's
take-what-you-bind. Swept: `destructuring.md` §3.1 (new), §7 (the open resolved);
`match.md` §4 (the superset note gains the back-reference).

**R148 — defer on the Go backend: implementable, two lowerings ruled; three defer.md
opens close.** The combing reaches compiler.md's hard question — Luna `defer` versus
Go `defer`, especially around goroutines — and the answer is **yes, cleanly**, with
the differences smaller than feared: registration-on-reach with by-value capture
(defer §4) is *exactly* Go's argument-evaluation rule, LIFO matches, panic-in-defer
supersession with remaining-defers-run (§6) is Go's own behavior, and return-value
mutation never arises (we never emit named returns). The one real difference is the
known one — **block-scoped versus function-scoped** — so Go `defer` is unusable at
Luna-*function* granularity only. **Two sound lowerings recorded** (compiler §7.3):
(1) *blocks become functions* — each defer-carrying block wraps in an
immediately-invoked Go func holding real Go `defer`s, every property delivered
natively, control-signal returns for `return`/`break`/`continue` crossing the
wrapper; and (2) **the zero-cost hybrid, the target** — normal exit edges get inlined
guarded cleanup (the destructor-flag lowering; the pending set is statically known),
panic paths use a **per-task defer list with block-depth markers** drained at
recovery sites. **The constraint that decides where to drain is `catch (p: panic)`**:
panics are catchable mid-stack (errors §8), so defers between throw and catch must
run before the catch body — every catch is a Go `recover` boundary anyway, so its
handler drains to the catch's depth marker, and the goroutine-entry trampoline —
which *already exists*, because a task panic must resolve its promise (concurrency
§4.1) — drains the remainder. Same trampoline, one added drain. **Goroutine
composition is free by construction**: the defer list is per-task state beside the
cancellation flag and promise; scope-bounding and const-snapshot capture mean
nothing crosses tasks; cancellation unwinds through the same machinery (R115), with
the one addition R115 demands — the **shield flag**, a per-task in-defer bit
suppressing `cancelled` delivery inside defer bodies, checked exactly where delivery
already checks; a task leaked on an uncancellable loop runs its defers at its
eventual suspension point, consistent not special. **The combing bonus — defer §8
was stale on three of four bullets**: cancellation cleanup was resolved by R115 two
eras ago (now a landed record); early stream cleanup was answered by the R121 file
model (the owner-scope `defer close(f)` pattern — the file, not the stream, owns the
descriptor, so abandoned short-circuited chains leave nothing unclosed; recorded
closed); and the panicking-defer residue is **resolved by R148 with machinery that
now exists** — supersession is *chained, not lossy*: the displaced panic rides the
superseding one as its **`cause`** (R110's identity surface), recursively, so control
flow moves on and the record loses nothing. The top-level-defer question stays open,
correctly (pending the process-exit model). Also clarified: §1.6's "(defer spec,
§7.3)" cite read as a defer-spec section that does not exist; it now says what it
meant (compiler's own §7.3). Swept: `compiler.md` §7.3 (the lowering record), §1.6
(the cite); `defer.md` §6 (the `cause` chain), §8 (three closures).

**R149 — the build cache audited against the current language: two miscompile holes
closed, the target dimension added, two fossil layers cleaned.** The
incremental-compilation spec predated most of R91–R148, and its own correctness
standard ("never serve a stale artifact") is what convicts the drift. **Fossils**:
§1.2's protocol clause still hashed "the full meta-function surface and element
contract" (pre-R95 — now the member surface: binding keywords, grants, types,
required-versus-defaulted, defaults and definition-fixed values reachable as `P->m`,
the requirement set, `identityEquality` — R129's row shape exactly), and its function
clause claimed comptime-eligibility is "type identity" (pre-R43/R130 — eligibility is
a value fact; the interface now lists the signature *plus the declared capability
set*, interface because dependents observe it twice: comptime eligibility and `use`
obligations). **The two silent-miscompile holes**, both caught by §1.2's own logic
run against the current language: exported **const values** (a dependent
comptime-folds them into its binary; §1.2 hashed only the types) and **attributes on
exported declarations** (a dependent's generated serializer reads `jsonTag` at
comptime, attributes §4/json §2) — closed under the generalized rule the corpus can
now state precisely: **the interface is everything comptime-observable through a
dependent**. Which also grounds §5's serialization open: the extraction *is* the
introspection surface computed at compile time (R127–R131 — `members(p)` rows,
signatures, `capabilitiesOf`, `fields`, `variants`), so cache, tooling, and
introspection agree on what an interface is **by construction**, and only byte-level
details remain open. **The missing key dimension**: R138 made platform facts target
facts that fold at comptime, so per-target artifacts differ *by design* — the cache
namespace (§3) and the run-cache key (compiler §0.1) now carry **compiler version
and compile target**, with "version" noted to cover bundled data (the tzdb R133, the
PCG pin R139); latent while one target exists, recorded so the second target finds
it waiting. **The R136 coupling cost gains its build face**, noted in §1.3: a bare
or bare-assigned import couples a dependent to the whole export surface, so adding
any export recompiles every bare-importing dependent — correct, and a real argument
for selective imports in hot dependency cones. **Sibling fossils in compiler.md**:
"protocol devirtualization" survived R144 in three places (the load-bearing-choices
intro, the §1 pipeline diagram, §5's pass list — all now "protocol-member
resolution"; §4's IR note respelled onto applied-set facts) and §7.3's "the full
concurrency model is pending" was four eras stale (the model completed R115–R119 and
R142; the bullet now enumerates the per-task runtime state the defer lowering
already fixed). Eviction tuning stays open, correctly. Swept:
`incremental-compilation-build-cache.md` §1.2 (both clauses, the new
comptime-observable bullet), §1.3 (the coupling note), §3 (version-and-target), §5
(the grounded open); `compiler.md` intro, §1 diagram, §0.1 (the key), §4, §5, §7.3.

**R150 — the escape table lands; lexer G4 closes, G5 is superseded; two lexical
opens settle.** The lexical-structure combing: block-comment nesting **reaffirmed
deferred** (do not implement — the depth counter is cheap but the failure-mode
change has still not earned its keep), the error-casing bullet closed as stale
(resolved by R122; this file missed that sweep), and **the escape-sequence table is
ready and ruled** — the corpus had already fixed most rows piecewise (bytes §7's
enumeration, strings §13's single-quote rule, regex's own language, the COMMAND
mode's "unescaped `` ` ``" terminator), and assembling them surfaced the three
decisions now blessed. **The authoritative table is strings §13.1**, per context:
`"…"` gets `\n` `\t` `\r` `\\` `\"` `\$` and **`\u{H…}`**; `'…'` keeps its ruled
pair; `b"…"` gets the shared set plus `\xNN`, minus `\$` and `\u{}`; `` `…` `` gets
`` \` `` `\\` `\$`; `/…/` passes RE2's language through. **The three rulings**: (1)
the **`\xNN`/`\u{…}` split is safety, not taste** — a raw byte in a string could
break the UTF-8 validity guaranteed at ingress, so `\xNN` is bytes-only, while
`\u{…}` encodes valid UTF-8 by construction (surrogates and >`10FFFF` are lex
errors; without it, control characters would be unwritable in strings since the
raw-byte door is correctly closed); (2) **an unknown escape is a lex error**, never
PHP's silent pass-through; (3) **no `\0` shorthand, no octal** — `\u{0}` spells NUL,
and §13's exemplified `"\0"` is retired by the table. **Two internal contradictions
resolved en route**: lexer G4 closes (span unaffected — `\\.` covers any pair, and
`\u{…}`'s braces ride the existing depth machinery; decoding reads §13.1), and
**lexer G5 is superseded** — its no-command-escapes resolution (a literal backtick
via the ``${'`'}`` interpolation workaround) was ceremony where one escape pair
suffices, and the mode table's "unescaped `` ` ``" terminator had implied the escape
reading all along; `COMMAND` mode gains `ESCAPE_PAIR`, `CMD_TEXT` excludes the
backslash. Swept: `strings.md` §13 (the example list de-`\0`'d) and §13.1 (new, the
authority); `bytes.md` §7 (repointed, the bytes-only rationale in place);
`command.md` §2 (the three escapes, G5's supersession noted); `lexer.md` §4 (two
rows), G4/G5 (records); `lexical-structure.md` §2 (casing), §4 (three bullets:
closed, reaffirmed-R150, stale-closed).

**R151 — modules §11: two opens become decisions.** The combing reaches modules.md,
and both bullets convert from questions to directions-fixed deferrals. **Packaging
and distribution: source-based, ruled** — packages will distribute as source trees,
not compiled artifacts, and the direction costs nothing to fix now because the
corpus already leans on it three ways: comptime folds imported bodies and const
values into dependents (R149's interface rule — a binary package would carry the
source-equivalent anyway), artifacts are per-version-and-per-target (R149 — binary
distribution would be a version × target matrix where source is one tree), and the
capability audit is a *source* audit. What stays deferred, deliberately: mounting
and rooting (likely a project-marker root file), and the standard library's
organization under `std`. **Dynamic loading: excluded and deferred, not open** — the
exclusion is what makes the import graph fully static, which the DAG, the
interface-hash cache, and the comptime sandbox all lean on; a standing decision,
revisited only if a concrete need survives contact with those three dependents.
Swept: `modules.md` §11 (retitled "Deferred by decision", both bullets rewritten).

**R152 — tooling §7: four deferrals marked, and the trivia question dissolves.** The
formatter, LSP, finer-grained incrementality, and debugger designs are **deferred by
decision** to their own dedicated specs — tooling.md fixes the foundations they slot
into (the pass library, the lossless CST, error tolerance), not their surfaces. **The
trivia-in-interface-extraction question dissolves** on a stance the corpus already
held: it assumed a doc-comment subset of trivia, and **no such class exists** — Luna
deliberately has no doc-comment syntax (lexical-structure §3), so comments are
uniformly ordinary trivia, uniformly excluded from the extracted interface and its
hash (build-cache §1.3), never semantic. Hover documentation, when tooling wants it,
arrives through the one declaration-metadata mechanism the language has — attributes —
and **the question's sharp residue is recorded at that future feature's home**
(attributes §6.3): a documentation attribute must pick its **observability class**,
because R149 made comptime-observable attributes interface-hash-bearing, so a doc
edit under today's one attribute class recompiles every dependent — the exact cascade
§1.1 exists to fight; a tooling-only class avoids it but is a new attribute category.
The constraint is fixed now so the eventual design chooses knowingly. Swept:
`tooling.md` §7 (retitled, four deferrals, the dissolution record);
`attributes.md` §6.3 (the doc-attribute constraint).

**R153 — channels §7: nothing open, confirmed and marked.** The combing verifies the
user's read exactly: select was resolved by R142 (the dissolution record already in
place), and the remaining three are deferrals with directions fixed — **MPMC /
work-stealing** deferred by decision (one consumer per stream is the model's grain,
ownership-follows-readability; fan-out has `spawn`/`await`; revisited only if a real
workload outgrows the owner-task pattern), **channel-of-channels** deferred to
practice (nothing forbids them; idioms documented as they prove out, never
pre-specified), and **the stdlib patterns layer** deferred as library work, fully
unblocked since R142, pending write-up only. Section retitled "Resolved and
deferred — nothing open." Swept: `channels.md` §7.

**R154 — attributes §6.3: duplicates are a compile error; the two deferrals
reaffirmed.** **Duplicate application of one attribute on one declaration is
rejected outright** — the alternatives each fail a house rule: last-wins silently
discards data (the no-silent-drop instinct), and a list forces every consumer to
handle multiplicity nobody asked for; the declaration is malformed, so say so.
Trivial, and ruled as such. **Attributes on other declaration forms** stay deferred
by decision pending a concrete need, with the likeliest first customer named (the
`jsonTag`-on-proto-members rider, json §4); **the documentation attribute** stays
deferred with its R152 observability-class constraint standing. Section retitled
"Ruled and deferred." Swept: `attributes.md` §6.3.

**R155 — constraints §11: three ruled, one deferred; stacking lands as §6.1.**
**Predicate expressiveness ruled full**: any expression whose calls are all
capability-free — the statically decidable meaning of *pure* (R43's theorem), with
R79's value-alone requirement discharged by machinery already on the books
(capability-freedom bars ambient effects; const-snapshot capture freezes referenced
environments, so nothing a legal predicate reads can go stale) — and **no cost
carve-out** (the runtime-checked stance already accepts per-entry cost; an expensive
predicate is the user's code costing what it costs). **Static elision deferred by
decision** (the §9.5 mechanism is fixed; the provable-case catalogue is compiler
work). **Constrained bases ruled yes** (new §6.1) — the user's bottom-up instinct
upgraded to a *typing necessity*: `constraint i: byte where i <= 127` binds `i: byte`,
so the predicate may assume base membership, which is only honest if the base chain
ran first; the spelling desugars to §6's own conjunction (`… where byte where …`),
**delta checking falls out of the fact model** (a value already typed `byte` skips
`byte`'s predicate — `b as asciiByte` runs one conjunct, not two), representation was
already ready (nested intervals §9.1; chain-implicit widening §5; `baseOf` answers
the immediate parent, R131), and the base-match rule reads through the chain.
**Other bases confirmed by shipping practice** — the open never closed while the
corpus filled with its answers (`json`, `path`, `probability`, `finiteDouble`); §7
already splits machinery by base mutability. Swept: `constraints.md` §6.1 (new), §11
(retitled, four records).

**R156 — protocols §10: nothing open, confirmed.** The user's read verified: four of
the six bullets were already closure records (R123 removal, R108 initializer grammar,
R125 serialization nesting, R126 `@@` totality); the two stragglers convert — the
**`?->` token bullet was stale** (R101 landed `OPT_PROTO_ACCESS` with the
`??` › `?->` › `?.` munch order two eras before the bullet stopped saying "the
build-spec sweep's concern") and **bound functions** is marked the standing rejection
it always was (§3.4; no concrete need has survived the explicit-closure idiom). The
list retitled "Resolved and rejected — nothing open." The protocol spec — the
conversation's first major redesign — now carries no open question at all. Swept:
`protocols.md` §10.

**R157 — serialization §3: `fromJson` was never still open, and formats get one
generator each.** The user's recall question exposed a doubly stale bullet:
`fromJson` landed eras ago as `fn (j: json): table!` (json §3), but serialization.md
still listed it deferred *and* remembered a drifted signature (`: any`). The bullet
is now a record pointing at the one genuinely deferred piece — the read-side
generator, json §4. **One generator per format, ruled**: `toJson`, `toCsv`, `toYaml`
each their own comptime generator; the format-parameterized alternative rejected
because formats legitimately differ in semantics (flat-tabular csv,
attributes-versus-elements xml, yaml anchors) — one parameterized signature would
express the union of every format's semantics, the same argument that made
`toJson`/`toJsonDynamic` deliberately two names, and the ruling matches the
per-format module layout already on the books (one format, one module, one
generator). Swept: `serialization.md` §3 (retitled, both records).

**R158 — associativity: `apply` and `declared` join the table; the stale heading
retitles.** The make-sure pass found the table current on every existing row (the
R95–R101 tier 1, the R146 tombstone, the R112/R108/R137 prose) — and **two operators
missing entirely**. **`apply` gets its own tier, 1a**: keywords §3's "tier 12 unless
noted" default mis-implied a prefix word, but `apply` is *infix* — and half its
precedence question does not exist, because the right side is the operator's own
closed grammar (a proto name plus initializer list, never an expression, protocols
§4.2), so only the left edge needed a rule: a complete tier-1 postfix expression,
chaining left (protocols §7's own `[] apply person(...) apply employee(...)`),
binding tighter than every comparison so `x apply P is @P` needs no parens; the
keywords row now notes the tier. **`declared` joins tier 12** as the degenerate
member: a word prefix whose operand is exactly one binding name (type §4), so
precedence barely bites, but it lives where it belongs. And §4's heading — "Resolved
drift, and open questions" — promised opens it did not contain; retitled "nothing
open." Swept: `associativity.md` §1 (two rows), §4 (the heading); `keywords.md` (the
`apply` row's tier note).

**R159 — postfix modifiers get their exclusions; unused bindings and imports become
errors.** The user's gap (`let x = 5 if (cond);` desugars to a binding scoped inside
the sugar block — nonsense) is closed **at the grammar, not by a lint**: declarations
take no postfix modifier, compile error — a conditional declaration is nonsense by
construction, so it is unrepresentable, while the line sits one notch over exactly
right (assignment with a modifier is legal and useful: `x = 5 if (cond);` writes the
outer `x`). **The analysis found a trap worse than the nonsense case**: `defer f() if
(cond);` would desugar to defer inside the sugar block, whose exit *is* the
trigger — the cleanup would run **immediately**, silently, at the wrong time — so
`defer` takes no postfix modifier either, and the conditional-cleanup idiom is the
same syntax one level in (`defer { f() if (cond); };` — registration unconditional,
execution conditional, captured at registration per defer §4). Pins completing the
form: no `else` on postfix; postfix `if` is statement grammar, never an expression
(conditional values are `match` and `??`); the desugar-is-the-semantics and
one-modifier rules were already ruled (R46) and stand. **The unused rules, and Luna
was forced to rule them**: compiler §1.7's no-ICE contract ("valid IR implies valid
Go; a Go compile failure is always a compiler bug") meets Go's rejection of unused
locals and imports — so Luna either errors at its own level or launders dead code
through emitted silencers. Ruled: **an unused local binding is a compile error**
(variables §4.1 — the discard is explicit, `_` or don't bind), **an unused import is
a compile error** (modules §5 — earning its keep independently: a phantom import is a
phantom dependency edge the interface-hash cache would rehash against), **unused
parameters are not an error** (signature conformance is legitimate; `_`-name them),
and module-level unexported-unused stays legal for alpha (dead-code elimination's
territory). Swept: `control-flow.md` §4.1 (new), `variables.md` §4.1 (new),
`modules.md` §5.

**R160 — defer §8 fully closes: the last bullet resolves by composition.** The
validation pass confirms R148's three closures and finds the fourth — top-level
defer, "pending the program-entry and process-exit model" — **now answerable, because
the model it waited on landed since**: module top level admits only declarations
(modules §1; execution enters through `main`), so the program's entry block *is*
`main`'s body and a defer there is §1's own function-scoped case, running at `main`'s
return before the process exits with its `int!` code; and R134 sealed the other
path — no `exit()` exists, nothing unwinds the process past pending defers — so
every process end is either `main` returning (defers run, §1/§5) or a panic/`die`
unwinding through `main` (defers run, §2). The honest residue is external and out of
any language's scope (`SIGKILL` runs nothing; signals generally are their own future
question, not defer's). Section retitled "Resolved — nothing open." Swept:
`defer.md` §8.

**R161 — `decimal` specced: exact arithmetic, policy-explicit division, the Java
traps killed by construction.** The committed tower member gets its spec (new
decimal.md; delivery stays post-alpha with the extended tower, §6 unchanged). **The
model**: `unscaledInteger × 10⁻ˢᶜᵃˡᵉ` with an arbitrary-precision integer —
string-like exactness without string storage; the user's arbitrary-vs-fixed question
was already answered by the tower's committed row ("grows, no overflow"), and the
*real* question underneath was division. **The centerpiece ruling**: `+`/`-`/`*` are
exact operators, always; **`/` and `%` are omitted from the operator table** —
compile error naming `div` — because `1/3` has no finite decimal expansion, so
decimal division *is* a rounding decision, and a rounding decision hidden in an
operator is what R106's policy-verb discipline exists to prevent:
`div(a, b, scale, rounding: roundingMode = {halfEven})`, scale required, banker's
rounding the default, `{halfUp}`/`{trunc}`/`{floor}`/`{ceiling}` completing the
closed enum. The exact-division desire has a committed home that is not this type —
`rational` — the family split doing the work. **Python's ambient context rejected
permanently** (frame-dependent arithmetic, the R79-family violation, comptime
poison): every rounding is written at its site. **Equality is normalized value
equality** (`1.10 == 1.1` true — scale is representation; Java's
`equals`-vs-`compareTo` split, the industry's most infamous decimal footgun, is
unrepresentable; display scale is formatting, never identity). **Boundaries**:
`parseDecimal(s): decimal!` is primary (text is how exact decimals arrive), and
**comptime dissolves the literal question** — `const price = parseDecimal("19.99")`
folds, literal ergonomics with zero grammar; `int as decimal` stands (R124);
**`double as decimal` rejected**, resolving R124's hedge to *no* — it would embed
the double's true dyadic value (`0.1` → `0.1000000000000000055…`, the
`new BigDecimal(0.1)` trap), and the deliberate crossing is
`parseDecimal(toString(d))`, the `valueOf` behavior with the lossy moment visible;
`decimal as int` stays nonexistent with the R124 promise honored (the policy verbs
widen to `double | decimal` on landing). **Serialization is the canonical string**
(the R132 duration precedent — a JSON number would round-trip through doubles and
destroy the point), `parseDecimal(toString(d)) == d` the round-trip law.
**Deliberately absent**: transcendentals (an exact type cannot hold inexact results
honestly — that is `double`'s arithmetic), contexts, scale-preserving display.
Swept: `decimal.md` (new), `numeric-tower.md` §3.1 (the `double as decimal`
rejection), §6 (specced note), §7 (the rounding-and-context open resolved),
`index.md` (the row), `keywords.md` §5 (`decimal` predeclared, post-alpha noted).

**R162 — `rational` specced: exact division, the mirror table, and the
two-integers-not-two-decimals model.** The tower's second exact type gets its spec
(new rational.md; delivery post-alpha, §6 unchanged), closing decimal's hole exactly
as R161's family split promised: **decimal is exact radix-10 arithmetic; rational is
exact division — now a theorem, not a pointer**, because the operator tables are
deliberate mirror images: rational **owns `/`** (all four operations exact, always,
re-reducing to canonical form; `/0` panics per the committed row) and **omits `%`**
with the shortest rationale in the tower — *exact division leaves no remainder*.
**The representation question the user probed lands precisely**: a rational is *not*
two decimals — it is **two of the thing inside a decimal**, two arbitrary-precision
integers on the same internal bignum decimal's unscaled value needs (no user-facing
`bigint`, still), held in **canonical form as an invariant** (gcd-reduced,
denominator positive, sign on numerator) — two-decimals could not even be the
semantics, since the scales are redundant degrees of freedom canonicalization
immediately clears, and one-representation-per-value is what makes equality
structural and `1/2`-vs-`2/4` unrepresentable; the tower §7 normalization-and-
overflow open resolves with it (the wide option won, cheaply — the committed row
said "grows" all along). **The crossing trio** (the user's decimal-from-rational
question, R106-clean): `toRational(d): rational` total and exact (a decimal already
*is* rational-shaped, `n/10ˢ`); **`exactDecimal(r): decimal!`** the errorable demand
(finite expansion iff the reduced denominator's primes are only 2 and 5), shelved
beside the policy verbs outside the prefix families; and **`toDecimal(r, scale,
rounding = {halfEven}): decimal`** — total *because* scale is required, so the
`to*`-means-total contract survives untouched — decimal's `div` philosophy arriving
from the other side. The policy verbs widen once more (`double | decimal |
rational`, completing R124's promise through R161's extension). **Boundaries**:
`parseRational!` (`"2/3"`, integer and decimal text; `"1/0"` is a parse *error*, not
a panic — no division was attempted), comptime-folded literals (the R161 story
verbatim), `int as rational` lossless (R124), and **`double as rational` rejected**
with the trap sharper than decimal's — the embedding would be *mathematically exact*
(every finite double is a dyadic rational), which is precisely the problem;
`parseRational(toString(d))` is the visible crossing. Canonical-string
serialization, third application of the R132 precedent; integral rationals print
without `/1`. Deliberately absent: accessors for alpha (their honest return type is
the nonexistent `bigint` — recorded, not forgotten), reciprocal (three tokens),
transcendentals (decimal §7's argument verbatim). `complex` is now the tower's last
unspecced member. Swept: `rational.md` (new), `numeric-tower.md` §3.1 (the
symmetric rejection), §6, §7 (the open resolved), `conversion.md` §2/§5 (the
widening completed, the trio pointed), `index.md`, `keywords.md` §5.

**R163 — the exact types map onto `math/big`: recorded, with the three wrapper
divergences named.** The implementation question answered and pinned (compiler §7.5,
the R148 backend-record pattern): the shared internal bignum is **`big.Int`** (pure
Go, platform-deterministic — what keeps comptime folds and R149's cache keying
sound); **`big.Rat` backs `rational` nearly wholesale** — its always-normalized
invariant is verbatim rational §1's canonical form, `SetString` accepts
`parseRational`'s text forms, `RatString` matches the no-`/1` display; **`decimal`
has no Go counterpart** (`big.Float` is arbitrary-precision *binary*, a semantic
mismatch never to be touched) and is a thin runtime struct
`{unscaled: big.Int, scale: int32}` — the shape of Go's dominant third-party
decimal, well-trodden. Three divergences, all wrapper-level, none semantic:
zero-denominator `Rat` panics in Go where `parseRational("1/0")` is a Luna error
(one validation); `FloatString` rounds half-away-from-zero only, so the
`roundingMode` enum is implemented over quotient/remainder primitives;
`math/big`'s mutable receiver API is emitter plumbing that becomes an
allocation-reuse optimization where uniqueness is proven. Consequence worth the
record: the exact-types implementation cost is far lower than the specs'
sophistication suggests — relevant to post-alpha scheduling. Swept: `compiler.md`
§7.5 (the mapping bullet).

**R164 — `complex` specced: a pair of doubles, IEEE per component, and the tower's
one unordered member.** The last unspecced tower member exists (complex.md): the
one-sentence model is **`double`'s semantics on a plane** — unlike its shelf-mates
it is *inexact*, and what it adds is closure (roots for negatives), not precision.
Component type is **always `double`, permanently** (numeric-tower §7's last
tower-open resolved): `float`-component complex is an array-storage optimization
for an audience Luna does not serve, exact-component (Gaussian rational) complex
has none at all, and no parameterization mechanism exists to express either — and
one component type makes the backend **Go's native `complex128`, boxed**, zero
wrapper divergences, the cheapest tower member to deliver (compiler §7.5). All
four arithmetic operators, `/` included — the deliberate anti-mirror of decimal:
decimal banished `/` because rounding is a policy decision hidden in an exact
type; complex is already inexact, so `/` hides nothing `double`'s own does not,
and division by complex zero is IEEE inf/nan per component, never a panic (the
float family's rule, not `rational`'s `divisionByZero`). `%` omitted (no meaning
on the plane). **No ordering: `<`/`<=`/`>`/`>=` are compile errors — a theorem,
not taste** (were `i > 0`, `i·i = -1 > 0`; were `i < 0`, the same), making
`complex` the tower's first type without comparison operators, noted in
operators §2's ordering row. `==` is componentwise IEEE, nan-contagious,
non-reflexive like `double`. Accessors `real`/`imag`/`conj`/`magnitude` —
**`magnitude` deliberately not `abs`**: math's `abs` contract is
kind-follows-the-operand (R92), which complex-in-double-out cannot honor. The
literal story is the R161 story verbatim: the pure constructor
`complex(re, im)` comptime-folds; an `i`-suffix literal is deferred, with the
finding recorded that numeric-operators §1.1's illustrative `-3-4i` could never
typecheck under the family rules (`int` minus `complex`) — a literal form means
untyped-constant machinery (rejected as a mechanism) or a fused lexical form, and
the constructor makes the question idle; that passage now reads
`-complex(3.0, 4.0)`. Crossings: **`double as complex` legal** — the asymmetry
against R161/R162's rejections is the point, the component is carried bit-for-bit,
not reinterpreted, so R124's lossless criterion is met with no trap; explicit
because it allocates (§3.1's rule). No `complex as double` (a projection is an
accessor, `real(z)`, never a narrowing); **no exact-type interop, ever**
(rational §6's parked item resolved as *never*): components are doubles, and
`double`'s exact-type crossings are already ruled string-mediated — a direct path
would smuggle the lossy moment out of sight, twice. `toString` canonical
(`"3+4i"`, both components always, `i` always); `toJson` is the canonical string,
fourth application of the R132 precedent; `parseComplex!` at the boundary.
Transcendentals and polar form deferred (not decimal §7's exactness argument —
an inexact type could hold them honestly; the surface waits for its audience).
Two stale sites found and fixed en route: numeric-tower §1.4's backend paragraph
still recommended `big.Float` (contradicting R163's never-used ruling — now
points at compiler §7.5), and decimal §7 still called rational interop "deferred
with `rational` itself" (delivered by R162's trio). Swept: `complex.md` (new),
`numeric-tower.md` §1.4 ×2, §3.1, §6, §7 (the open resolved), `operators.md` §2
(the ordering row's exception), `numeric-operators.md` §1–§3 (the per-type
operator-table exclusions stated in §1 — an omitted operator is a compile error;
the §1.1 rewrite, including the fix of `complex` mislabeled an *exact* type; §2's
violation taxonomy completed from two shapes to four — the exact types are "safe
by growth", `rational`'s `/0` alone panics, `complex` is IEEE per component; §3's
opens resolved, the pointed-at type-set questions having landed with
R161/R162/R164), `rational.md` §6,
`decimal.md` §7, `conversion.md` §5 (verbs do not widen to complex; `parseComplex`
joins the family), `compiler.md` §7.5, `index.md`, `keywords.md` §5, and the
`divisionByZero` naming made uniform — `int.md` §5 now names the panic (it said
only "a panic") and the errors §2 panic-tree annotation widened from int-only to
tower-wide (int `/` `%`, rational `/`, decimal `div`).

**R165 — operators.md catalogue validated against the corpus: no new decisions,
ten repairs.** A full row-by-row pass of the master catalogue (§0) against
everything ruled through R164, requested as validation; every fix applies an
existing ruling, none makes a new one. The finds: **`await` appeared twice**
(two rows, drifted independently — merged into one, which now also names the
`doubleAwait` second-await panic, R142); **the compound-assign row contradicted
the coalescing rows** — it glossed `??=` as "assigns only when null", but `??=`
is *absent*-assign and `???=` (which the row omitted entirely) is null-assign;
**two `&`-intersection rows** — the older protocol-only row predated the general
type meet (type §3.1) and was absorbed into it; **the `@@` row still cited the
retired `reflection` spec** and used the retired read-write word ("reflect",
R127's whole point) — now "applied protocols", protocols §8/introspection; **the
spread row called variadics "unspecified"** (resolved by R108, functions §3.3);
**the `?->` token was missing outright** (landed R101, present in five specs,
absent from the table claiming every operator); **`try`, `throw`, `comptime`,
and `yield` had no rows** though the catalogue includes their siblings (`match`,
`defer`, `copy`, `comptype`) and keywords §2–§3 lists all four — added;
**`comptype`'s kind said "reflection"** — now "introspection", and the §0 intro's
kind enumeration was reconciled with the kinds the table actually uses (it
promised "bitwise", which has no rows — deferred, int §8 — and omitted
structure/introspection/comptime); **the `/` and `%` rows** gained the extended
tower's exclusions (decimal has no `/`; `%` omitted for the exact types and
complex) and their `duration` arms (std.time §2); **§2's tail** still called
`decimal`'s representation an open question (R161) — rewritten to point at the
tower's real remainders (literal forms, bitwise). Not touched deliberately:
"green thread" in the `spawn` row is corpus-legitimate terminology (overview,
compiler §7), not drift. Swept: `operators.md` §0 (the catalogue and intro),
§2 (the tail) — single-file by nature; the catalogue is a mirror, and the
corpus it mirrors was already consistent.

**R166 — range.md validated; its "opens" reclassified as defers; one summary
fossil and one cross-spec slicing contradiction fixed.** The §8 items were never
open questions — nothing in them awaits a decision; each waits on a trigger — so
the section is now "Deferred by decision" (more positions: pending experience;
character ranges: deferred with the char/codepoint treatment, §6; a reified
bounds pair: for want of a need — match reads endpoints syntactically). The
validation's finds: **§7's summary bullet said "descending when `lo > hi`"** —
the bounds-implied descending that §4 itself headline-rejects (R28, R48; empty,
never descending) had survived in the file's own summary; the bullet now states
the actual rules (sign is direction, `0` panics, `lo > hi` empty, descending
explicit). And **bytes §4 spelled slicing `b[start..end]`** — inclusive `..`
inside brackets — contradicting the ruled corpus-wide convention that `:` slices
half-open and `..` ranges inclusive, the two never mixing (tables §2.5, range
§2.1, which itself cites `bytes[2:]` as the example). bytes.md never engaged
the question — casual notation predating the convention — and now reads
`b[start:end]`, half-open, with the convention cited. The rest of range.md
checked clean: the §4a desugar, the R93 implicit-keys bullet, R105
restartability, the match §5 membership split, int-only elements. Flagged for a
future bytes.md pass, not fixed here (an API-naming call, not mechanical
drift): `asList(b)` **copies** — under R106's prefix contract a copying total
conversion is `to*`-shaped, so the `as*` name is suspect. Swept: `range.md` §7,
§8; `bytes.md` §4 ×2.

**R167 — `asList` renamed `toList`: the last live `as*`-prefixed function falls
to the R106 prefix contract.** R166's flag, ruled: `asList(b: bytes): list`
**copies** the packed buffer into boxed `lval`s — a total, value-to-value
conversion, which is `to*` by R106's own table, and the `as*` spelling was
worse than merely old: `as` means lossless re-typing without a new value (as
spec, R124), so the name falsely suggested a free view over what is an O(n)
expanding copy — precisely the cost-visibility the prefix contract exists to
carry. The name was pre-R106 convention (its own parenthetical defended
`asList`-vs-`values`, a question from before the prefixes were unified), the
same fossil class as `asStream` → `toStream` (R102). `toList` was unclaimed, so
it lands as *the* `toList`, one name, one signature (functions §3.4); no
collision with `collect`, since `bytes` is deliberately not `iterable` (R104).
The grep confirms this was the **last** live `as*` function — the only surviving
mention is iterable-functions §3's retired-spellings guard table, where it
belongs. Swept: `bytes.md` §2, §4 ×2 (the signature, the bullet — its naming
note now states the R106 rationale), `conversion.md` §5 (the canonical summary
gains the `toList` row beside `toTable`).

**R168 — spread.md validated; §7 reclassified as one resolution and one
deferral; one intro fossil fixed.** §7 was never open: the variadic item is
R108's resolution (already marked so in place) and the `bytes`/`string`-spread
item waits on a use case, not a decision — the section is now "Resolved and
deferred," and the deferral's pressure is noted as low since `toList(b)` (R167)
already spells the explicit form, so deferring costs a name, not a capability
(the Still-open tail's tracking line reworded to match). The validation's one
find: **the intro's own parenthetical still said the parameter-list `...name`
position "is not yet specified"** — contradicting §7's R108 resolved marker
three sections down, the same fossil class R165 caught in the operators
catalogue; it now cites functions §3.3.3. Everything else checked clean against
the corpus: the §1 fold and its `merge` agreement (iterable-functions §2.7,
`preserveKeys = false`), the §2 stream-spread/`collect` distinction (§2.11,
R93), the `flatten` signature (verbatim match, §2.5), §4's list-only rule and
R108 named-argument rejection, §5's lexer claims verified against lexer §6
(`INTERP_IDENT` is `DQ_STRING`-only; `${...expr}` is `INTERP_OPEN` + `SPREAD`
in commands and literals), and §6's "Amendment A" cite (live in tables.md).
Swept: `spread.md` intro, §7; the CHANGES tail line.

**R169 — string internals brought current; the allocator review recorded (three
optimizations rejected, one theorem kept); the R27 concat leaks swept.** The
user proposed four immutability-driven optimizations; the review's outcomes are
now §11.1 of the string-representation spec. **Inline-in-`lval`** (8 bytes + a
flags bit) was already the ruled spec verbatim — tier 1, the string-inline field
of value-representation §2.2 — a proposal and its prior acceptance meeting. **A
separate string heap** is rejected: the expensive-to-GC premise mostly dissolves
on the Go backend (pointer-free `[]byte` buffers live in noscan spans — the
collector never scans string bytes, it marks the 16-byte descriptor and
sweeps), and a real private heap means stalled arena experiments or `unsafe`
manual memory — reintroducing exactly the two problems §3 records deleting, as
the first unsafe memory in the system. The Java analogy corrected in place: no
modern JVM has a separate string heap (the interned pool moved to the ordinary
heap in Java 7); the live feature, G1 string deduplication, is GC-time
invisible interning — the spec's existing §4 stance. **Deliberate close
packing** is rejected as riding on the same allocator ownership while degrading
§7: an arena-packed slice pins the whole arena, promoting `copy` from
optimization to memory-correctness obligation. **Naive-refcount completeness is
a true theorem, kept and rejected**: the string reference graph is acyclic by
construction (inline and owned reference nothing; a borrowed slice references
exactly one pre-existing buffer; immutability adds no edge after birth), so
naive RC is *complete* — no cycle collector ever — recorded for a hypothetical
self-hosted backend; rejected today because it buys nothing under Go's GC and
costs an inc/dec on every `lval` copy/drop, **atomic** ones, since immutable
strings are precisely the values shared by reference across tasks —
cross-core contention purchased for a collector we do not need. The fossil
pass on the same file: `bytes`-doesn't-exist-yet claims ×3 (§1, §8, §9 — bytes
exists; the bridge is `toBytes()`), §9's `stream | table` return shape (R102:
producers produce streams; restartable free off an immutable source, R105),
§10's "repeated `+` in a loop" (no concat operator exists — R27; joining is
interpolation/`join`/builder), §11's builder open (resolved — stringBuilder.md
exists in full). One vocabulary reversal from the pre-bless analysis, checked
and kept deliberately: "views" stays — strings §9 is *titled* "UTF-8 views",
live usage; R95 retired the table-view model, a different thing. And the
en-route discovery swept: **R27 (F10) removed `.` concatenation, but three
files still spoke it as live** — conversion §3.1 ("`.` concatenation, strings
§11" — citing as definition the very section that denies it) and
stringBuilder.md ×4 (the intro's "concatenation operator", §3.1's "`.`
concatenation operator uses", §6's "`join` or concatenation", §7's
"reallocating `.` concatenations") — all now name interpolation/`join`/pairwise
joining, with the operator's absence cited where load-bearing. Grep confirms
the only surviving "concatenation operator" mentions are denials and strings
§11's own tombstone. Swept: `internal-representation-of-strings.md` §1 ×2, §8,
§9 ×2, §10, §11 (+ new §11.1), `conversion.md` §3.1, `stringBuilder.md` §0,
§3.1, §6, §7.

**R170 — the 24-byte `lval` confirmed forced under stock Go; the escape hatches
pressure-tested and recorded; the GC fork scoped and declined; the
value-representation fossil layer cleared.** The question was whether any way
around the three-word hosted `lval` exists; the answer is no, and §1.1 now
carries the review so no hatch is re-attempted piecemeal: scalars in an
`unsafe.Pointer` word (actively fatal — `invalidptr` throws, and bit patterns
aliasing live spans silently retain arbitrary objects); pointers in `uintptr`
(freed under you; non-moving-today is non-contractual); NaN-boxing/tagging (the
same two rows in costume — they need a collector that reads the tag); Go's own
`any` (a 16-byte tagged union whose payload word is always a pointer — every
stored scalar allocates, and 48-bit typeid + flags + string-inline don't fit a
Go type word); handle/slab indirection (the slab pins everything — a memory
manager built to avoid one, plus a double-hop per read); off-heap cgo/mmap
(off-heap-to-heap pointers are GC-invisible). Two stock-Go recoveries recorded
beyond static unboxing: **traced-word-first physical order** (`ptrdata` = 8 —
the GC scans one word in three; 24 is an exact size class) and **homogeneous
table-storage specialization** (a provably-scalar list stores as noscan
parallel words, 16 or even 8 bytes per element with zero scanning — beating
the C layout where bulk data lives; Amendment A is the compile-time version;
the honest residue is 2.67-vs-4 lvals per cache line on genuinely mixed data
only). **The fork verdict**: a conditional-pointer slot permeates gcdata,
`typePointers`/`scanobject`, the write barrier (a *conditional* barrier is
compiler codegen, not runtime), stack maps, and span classes — a permanent
fork of Go's most safety-critical code, damaging the Go-source-to-Go-toolchain
premise and R149's determinism. Ruling: **forking the GC is the self-hosted
runtime on an installment plan** — declined now, reserved beside R169's string
refcount-completeness theorem for a future self-hosted backend that would take
the 16-byte union and string RC together. En-route finds, both fixed:
**compiler §7's error-model bullet claimed an "error bit"** on the errorable
`lval` — contradicting value-representation §2.1's ruling that error-ness is
never a flag (derived, `currentType <: error`, the §4.2 interval test) — now
states the derived check; and the value-representation file carried an
**11-site `IOError` PascalCase layer** (R122-missed; the corpus-wide grep now
returns zero live PascalCase error names). The four unqualified "16-byte
`lval`" sites (value-representation §1, string-representation §3/§10, compiler
§7) now defer to §1.1's logical/physical split. Checked and left standing:
`let v!: string` (the binder-suffix errorable form) is specified in errors
§463, not drift. Swept: `internal-representation-of-variables.md` §1, §1.1
(the review), casing ×11; `compiler.md` §7 (error model);
`internal-representation-of-strings.md` §3, §10.

**R171 — the orientation layer brought current: both overview files, same
fossil classes, swept together.** high-level-overview.md and overview/types.md
each still carried a **`view` row in the structured-types table** — the R95
retirement's most visible survivors, types.md's even pointing its Spec column
at the retired `views` file — both replaced with the missing **`sink`** row
(the send end of a channel, channels §3), which had never been added when
channels landed. The other repairs, applied to whichever file carried them:
the wider-numeric sentence updated from "committed but deferred" to committed
**and fully specced** (R161/R162/R164, delivery post-alpha) in both;
**`duration`/`instant` rows added** to both value-type tables (committed with
std.time, R132 — alpha types absent from the orientation layer the whole
time); `constraint` added to high-level-overview's declaration-forms list (it
was the one form missing); the stale multi-apply example `[] apply proto1,
proto2` corrected to the ruled chaining form `apply proto1 apply proto2`
(R158's grammar — the comma form was this file's alone, corpus-wide); the
"how a table composes *capabilities*" phrase de-collided to "roles"
(capability means the effect token, R43 vocabulary, nothing protocol-shaped);
types.md's `Shape` enum example re-cased to `shape` (the corpus's enum names
are camelCase: `roundingMode`, `weekday`, `kind`); the `float` row's cite
moved to numeric-tower §1.3; the `any` row's Spec pointer fixed to the `any`
spec (it pointed at value-representation); and types.md's closing "Deferred
types" section — which still deferred the module system, destructuring,
spread-into-calls, and operator precedence, all long since ruled (R136, R147,
spread §4, R28/R158) — rewritten to the honest remainder: the extended tower
is delivery-not-design, and only need-gated `bytes` API surface stays later.
Verified live before citing: `moduleof` (modules §7.1), the slice/range rows
(already R166-consistent). Swept: `high-level-overview.md` ×5,
`overview/types.md` ×6.

**R172 — exec is `std.exec`: the module home ruled, the file moved, the
`exec.run` wrinkle dissolved by construction.** exec.md lived in
`concurrency/`, which was never its subject; the question was std module
versus built-in capability + free functions. Ruled: **`std.exec`**, on three
converging precedents — every capability in the corpus is an std-module
export, none predeclared (`reveal` ← std.secret, `time` ← std.time, `entropy`
← std.random, `filesystem` ← std.filesystem, `egress`/`ingress` ← std.net,
`env` ← std.process), and a built-in `exec` would be the lone exception while
deleting the import-as-audit signal (`import { exec, run } from std.exec` is
the grep-able "this file can run programs" line); the **built-in type,
std-module effect** split is exactly `secret`/`reveal`'s shape (`command`
stays types/command.md with literals and `pipe()`; the effect moves); and the
backend itself splits `os` from `os/exec` — std.process (R134, process-self)
plus std.exec mirrors it. Folding into std.process was rejected (children are
a third concern; importing std.process for `argv` must not look exec-capable).
The placement dissolves the parked spelling wrinkle: with free `run`/`capture`
and the capability co-exported, `exec.run` cannot arise — an assigned import
collects the functions but never the capability (the slot-inhabitant rule,
R135/R136), so the file's own internal contradiction (free-function
signatures beside `exec.run(...)` examples) is resolved on the free-function
side, with `use (exec)` now on both signatures (the R143 std convention). The
rewrite's other repairs: "like `io` and `system`" — `system` died in R134 —
now `filesystem`; §2's broken example (element access on `string | error`
plus `is`-as-narrowing, violating the file's own §4 note) replaced with
propagation + a catch pointer; **`commandResult` re-expressed as the
`commandResult` protocol** (`@commandResult`, const-get members) — the
declaration used a record syntax Luna does not have, and R135's `@fileInfo`
is the ruled pattern for typed read-only results (access is `->`, examples
fixed) — flagged for review as the one substantive re-expression; §5's
internal naming drift (`shellExec` vs `unsafeShellExec`) unified on
`unsafeShellExec`, and the three generic `unsafeExec` illustrations of the
prefix rule (keywords §6, lexical-structure, functions §5.6's table) aligned
to the real planned name. Swept: `git mv concurrency/exec.md → std/exec.md`
(history preserved), the full rewrite, `index.md` (row moved from Concurrency
& effects to the std section), `capabilities.md` §9 (the grid row's home →
std.exec), `keywords.md` §6, `lexical-structure.md`, `functions.md` §5.6.
Command.md's seven "(exec spec)" cites are name-based and stand unchanged.

**R173 — `toCsv` committed: csv.md's writing side lands as the R157-family
comptime generator.** The bless completes what R157 already ruled at the
serialization level — one generator per format, each its own comptime
generator, `toCsv` named in that ruling's own list — so csv.md now carries its
member of the family, shaped exactly as json §2's canonical generator:
`export const toCsv = comptime fn (ct: comptype): fn (any): csv;` — `comptype`
in, runtime serializer out, specialization in `const`-captured plain data
(attributes §4), the result entering `csv` typed because a writer produces
valid CSV by construction. Deliberately *not* committed with it: the column
story (field-to-column mapping, header-row emission, the `jsonTag`-analog tag
vocabulary) — folded into the headers open as one question shared by both
directions — and a dynamic walker (`toCsvDynamic`, the `toJsonDynamic`
mirror), deferred pending use since a runtime `any`-walker needs the headers
and dialect answers first. The remaining opens (dialects, headers) stand as
valid, per direction: both now noted as applying to reader and writer alike.
The rest of the file validated clean: `fromCsv` is R106-conformant
(typed-carrier `from*`, always `!`), the constraint-boundary pattern matches
std.json §1.1, the bare `import std.csv;` is the R136 form. Swept: `csv.md`
§3 (new)/§4 (renumbered opens), `index.md` (the per-format row notes the
writer).

**R174 — io.md validated: current except its own §9, which had gone stale
against §4 and the io-errors spec; one cross-spec contradiction discovered
and left for ruling.** The finds, fixed: §9's taxonomy bullet asked about
"the exact **`fileError`** children (disk full on write? interrupted?)" —
the family is `ioError`, named by this file's own §4, whose list already
contains `outOfSpace`, and "interrupted" is answered by io-errors' fates
partition (`EINTR` absorbed by the runtime, never surfaced); the bullet is
now a resolved marker, with the one genuinely-open remainder promoted to its
own bullet — **write-side failures** (whether disk-full *during* a write ever
warrants a declarable arm instead of the §8 mid-operation panic; io-errors'
kept revisit flag). §9's filesystem-boundary bullet stated R134/R135's own
answer while sitting under "Open questions" — now a resolved marker. The
encoding-set and buffering opens stand as valid. Everything else checked
current: the R121 layer (no-`&`, creation-authorized lazy reads, canRestart
false), the R138 platform defaults, `defer close(fd)`, the §2.1 sink-naming
note (already channels-aware), §7.1's composition example, the §8 category
table, camelCase error names throughout. **Discovered and NOT ruled — the
`@P` value-position contradiction** (added to the still-open tail): type.md
§1.1 says `let t = @stringBuilder` (value position) is introspection on the
proto value "and so yields `proto`", while type.md §5, protocols §7's tail,
introspection §5, and io §2 all bless `export const file = @fileDescriptor`
as binding the **application refinement** — the same spelling, two claimed
meanings; §5's is the load-bearing one (a public type must alias), and the
static-protohood steer §1.1 already applies *within* type position extends
naturally to value position, but that is a ruling, not a sweep. Swept:
`io.md` §9 ×2. *(Ruled the next day: R175.)*

**R175 — `@P` in value position yields the induced refinement: the
static-protohood steer extended, the §1.1/§5 contradiction resolved on §5's
side.** The ruling: `@X` in an expression, where `X` is **statically a proto
binding**, yields the **application refinement as a first-class `type` value**
— the same interned typeid type position denotes — and introspection
otherwise; never an error in value position. The unifying story is stated in
§1.1: `@` hands over *the type associated with the operand* — for an ordinary
value the type it **has**, for a proto the type it **induces**; a proto is a
type-maker, and `@` on a type-maker hands over the made type. This is the
steer §1.1 already used *within* type position ("@X is a refinement only when
X is a protocol — a static fact"), now applied on both sides of the position
split, so for protos the two positions **agree** and the disambiguation
remains fully static (closed universe, value-representation §4.1 — the
existing semantic-analysis paragraph gains the value-position arm). What it
buys: the alias idiom is legal by construction (`export const file =
@fileDescriptor;` — an initializer is value position), and §5's `@P == @P`
one-typeid comparison means what it says. What it forecloses: the `@`
spelling for "the type the proto value itself has" — near-useless, its real
content being proto-membership, already spelled `x is proto`. The rejected
alternative — uniform value-position introspection — would have broken the
aliasing idiom at four blessed sites and demanded a new refinement-as-value
spelling. En-route: §1's example block gains the proto line, and its
`@someShape // Shape` comment was re-cased to `shape` (the R171 enum-casing
class). Swept: `type.md` §1 (intro sentence, examples, the reflects-a-value
note), §1.1 (both bullets, the agreement paragraph, the semantic-analysis
paragraph), §5 (the alias parenthetical); `overview/types.md` (the
value-position bullet's new arm). protocols §7, introspection §5, and io §2
already spoke the ruled side and stand unchanged; the still-open tail item
is discharged.

**R176 — as.md validated: one unwritable example, one internally-contradictory
rationale, and the R124 criterion brought up to its own later sharpenings.**
The finds, fixed: §1's subtype-narrowing examples included **`capability as
reveal`** — the corpus's only occurrence, and unwritable since R135: the
slot-inhabitant rule means no `capability`-typed value slot can exist, so the
wider operand of that narrowing can never be held (replaced with `ioError as
fileNotFound`, a real mid-tree narrowing). §6's secret bullet called `"text"
as secret` "**widening** a string into a secret… a coercion" — contradicting
this file's own §1 (widening is implicit; `as` is reserved for narrowing) and
R124's vocabulary; the spelling itself was verified current (secret §3 rules
`as secret`, with `secret(...)` a separate R79 *gated* constructor, now also
noted), and the bullet is rewritten as what it is: a **lossless entry** of the
`int as decimal` class, explicit-though-infallible because the crossing should
be seen (secret §3's own searchability argument). §3's criterion paragraph
predated the exact types: it now carries the **R161/R162 sharpening** —
lossless is *necessary, not sufficient*; the preserved value must be the value
the source type presents, so `double as decimal`/`double as rational` are
rejected though mathematically lossless (the faithful-embedding trap) while
`double as complex` (R164) is the accepted contrast (bit-for-bit, nothing
reinterpreted) — and §6's tower bullet gains the same three moves. §8 marked
none-open. Checked and clean: the §5.1 function-narrowing model (matches
functions §3.2/§3.2.1, both cites verified), the §7 no-flow-narrowing story,
the §4 pattern-position note, the R106 conversion split. Swept: `as.md` §1,
§3, §6 ×2, §8.

**R177 — is.md validated: the semantics sentence overclaimed subtype, exactly
as suspected; the dispatch paragraph self-contradicted on constraints; one
wrong cite.** The user's instinct ("`is` is subtype membership, but only
sometimes") named the defect precisely. §2's opening sentence said `x is T`
"reports whether `x`'s current type is a **subtype** of `T`" — **false for
constraints**: `200 is byte` is `true` while `int` is not a subtype of `byte`
(the relation runs the other way, `byte <: int`); the test runs the predicate.
The ruled meaning, now stated: `is` answers **membership in `T`'s value set**
("would `x` seat in a `T`-declared position") — one question, answered by
whichever mechanism the type's shape requires: typeid-subtype *coincides* with
membership for nominal tree types, constraints answer by **predicate over the
base** (constraints §7's own "admits exactly the base-type values that satisfy
the predicate"), and `@P` answers by the applied-set test, a value property
never in the typeid (type §5, the same axis R175 just walked). The dispatch
paragraph had the matching internal contradiction — it listed "a constraint"
among the **interval-check** tree nodes while its own tail said "a constraint
runs its predicate"; the constraint arm is now stated correctly (valueBase +
predicate, with an in-interval current typeid as the fast path that skips the
re-run, entry-only checking having already paid it). And the applied-set cite
pointed at **protocols §9** ("Extensions are functions") — now §6, the section
that actually holds `x is @P`. Verified real before letting them stand:
`isSubtype` (introspection §4.1 — exists, with introspection's own §0 stating
the value-vs-type-question split verbatim), the fn-ladder interval and
signature pairwise-table claims (value-representation §4.2), the §3
no-narrowing story (compiler §1.4.1). A corpus grep confirms the
subtype-overclaim phrasing existed nowhere else. Swept: `is.md` §2 ×2.

**R178 — `is` soundness recorded: static dispatch proven, the never-panics
contract given its honest asterisk, predicate non-totality ruled at the
predicate's home.** The soundness probe, answered and pinned (is §2.1): the
mechanism is chosen at **compile time**, never by runtime inspection of `T` —
the right operand is a type expression (associativity tier 6), type position
resolves statically (closed universe; every binding's kind statically fixed,
type §1.1), so the compiler emits the mechanism and no shape falls through;
the corollary stated: `x is t` with `t` a **`var` holding a `type` value** is
not writable (a `var`'s kind is *value*, unusable in type position) — dynamic
membership belongs to introspection, the R127 operators-are-language split.
Termination: decomposition cannot recurse (canonical pre-flattening),
intervals/pairwise loads constant, applied-set runs no user code — **the one
unbounded cell is the constraint predicate**, and that is not `is`-specific
(the identical predicate runs at every entry; a diverging predicate breaks
its constraint everywhere equally). The genuine gap found by the probe, now
ruled in both homes: **purity is not totality** (new constraints §2.1) — a
predicate can panic *ambiently* (`i * i > 0` overflows at large `i`:
`overflowError` from inside the predicate, arriving before any membership
answer and, at entry sites, before any `typeError`), and such a panic
**propagates from every checking site, never swallowed into `false`/"check
failed"** — suppression would hide the bug and make the answer a silent
wrong value; and a predicate can **diverge** (pure function calls, §11; no
termination checker exists). Both are authoring bugs in the constraint, not
holes in the checking model, whose guarantee is *where* checks run and that
verdicts are durable — never that an arbitrary pure computation is cheap,
terminating, or panic-free. is §1's "never panics" now carries the
conversion-§2-pattern precision: no `is`-**specific** failure, not "nothing
can panic while the test runs." Swept: `is.md` §1, §2.1 (new);
`constraints.md` §2.1 (new).

**R179 — conversion.md aligned to the R157 writer split and completed; and
the validation uncovered a two-headed json spec, flagged for merge.** The
alignment: §2's `to*` exemplar row listed **`toJson`** as a value→value total
conversion — post-R157 `toJson` is the *comptime generator*
(`comptype → fn (any): json`); the value→value writer is `toJsonDynamic` —
the row now names `toJsonDynamic` (and `toList`, R167's own row, was missing
from its own file's exemplars), with the generator noted as honoring the
contract through its *product*; the §2 precision paragraph and §3's
flagged-call parenthetical (`toJson(v, includeProtocols: true)` — a call
shape `toJson` cannot take, its first parameter being a `comptype`) fixed the
same way. §5 gains the two missing family rows — **`toBytes(src: string |
iterable)`** (strings §9, the R107 per-element panic cross-noted) and
**`toStream(src: iterable | bytes)`** (the lazy O(1) bridge, R102) — and the
deferred bullet now names the full landing `parse*` family
(`parseDecimal`/`parseRational`/`parseComplex`), not just complex's. Checked
clean: §1's dividing line, §2's naming principle and policy-verb paragraph,
§3's stringify machinery (protocols §3.1 *is* the qualified-form section,
§3.4 *is* fn-typed-member-on-proto — both verified), §3.1/§3.2, §4's
open/closed split, §6's three opens (all valid). **The discovery**: the
corpus carries **two diverged `json.md` files** — `std/json.md` (181 lines,
the R125 flag signatures, R146-touched; most "json §" cites resolve here:
§2.1 what-serializes, §3 reading) and `types/json.md` (142 lines, untouched
since the R145 rename, **but holding unique content other specs cite**: the
§1.2 entry-cost rules and §1.3 predicate-dependency-on-constraints-§11) —
with **both** indexed (index.md rows 125 and 180, each claiming
`toJson`/`toJsonDynamic`). Divergence in the flesh: types' `toJsonDynamic`
signature lacks the R125 flags std's carries. Not resolved here — a merge is
structural surgery awaiting direction (tracked in the still-open tail).
Swept: `conversion.md` §2 ×2, §3, §5 ×2.

**R180 — the two-headed json spec collapsed: `std/json.md` is the one json
spec; the orphan retired with a was→is map.** R179's flag, executed. The
merge carried types/json.md's unique, still-true content into std/json.md as
**new subsections of §1 — so no existing "json §" cite renumbers**: §1 gains
the value-carried and equality-erases constraint-rule bullets (constraints
§9.2, equality §1 — `@s` reports `json` through widening, `someJson == "{}"`
compares contents), §1.1 gains the boundary-idiom block, and **§1.2 "The
cost, precisely"** (once per value then free; the O(n) entry parse as the
honest price; §9.5 elision; O(1) `is`/`@` after entry) and **§1.3 "Predicate
dependency"** (`isValidJson` as constraints §11's motivating instance) are
carried whole. §4's opens absorb the orphan's two extra details: the strict
RFC 8259 expectation, and a promoted **number-fidelity** open (`nan`/inf
have no JSON representation — sharpened, not settled, by the exact types'
canonical-string rulings). Everything else was subsumed: std's §2 already
carried the generator and walk with the R125 flags the orphan's signatures
lacked, its §2.1/§3 the what-serializes and reading stories the orphan
deferred. The orphan's §5 "parsing deliberately not specified here" bullet
was simply obsolete (std §3 specifies `fromJson`). Mechanics: `git mv
types/json.md → retired/json-duplicate.md` (history preserved), content
replaced with the R146-pipeline-style tombstone (what it was, why retired,
a was→is table); index.md's types-section JSON row removed (the std.json
row remains, the one row); greps confirm zero external cites to the folded
§1.2/§1.3/§5 (they were internally cited only), zero dangling `types/json`
path references. Swept: `std/json.md` §1 ×2, §1.1, §1.2 (new), §1.3 (new),
§4; `retired/json-duplicate.md` (new tombstone); `index.md`.

**R181 — equality.md validated: two types had no ruled equality at all
(`bytes`, `sink` — both now ruled by §2's own dividing principle), the alpha
types `duration`/`instant` and `proto` were missing from the summary table,
the capability rows described an unwritable comparison, and the
non-reflexive census predated `complex`.** The two rulings, made loudly and
flagged for review: **`bytes == bytes` is content equality** (length then
bytes, a `memcmp`; the shared-storage fast path applies) — nothing had
specified whole-buffer equality anywhere; §2's principle answers mechanically
(contents finite, comparable, meaningful), and it is the already-ruled
element rule (`byte`-vs-`int` erasure, bytes §6) bulked. **`sink == sink` is
identity** — also specified nowhere; channels §3's own "no readable surface"
principle decides it (contents are not a coherent question for a write end),
and the rule is now recorded at both homes. The mechanical repairs:
**`duration`/`instant` rows added** (value equality on the payload; ruled in
time §2's operator table since R132, never mirrored); a **`proto` row added**
(§2 already ruled identity, R126 — the table just lacked the row, with the
`@P == @P`-compares-refinement-typeids distinction noted, R175); the
**`capability` rows** (§5 bullet, §6 row) gained the R176-class honesty note
— the comparison is *unwritable in user code* (tokens never enter a value
slot, capabilities §3.1/R135, and `==`'s operands are value positions), kept
because the relation is real internally (the dynamic grant check is a subset
test over capability identities); and the **extended tower's equality is now
forward-noted** after the table (decimal normalized, R161; rational
structural-because-canonical, R162; complex componentwise IEEE, R164), with
the closing census corrected: the non-reflexive set is `double`, `secret`,
**and `complex` when it lands** — "the two non-reflexive rows" was true when
written and wrong since R164. Checked clean: §1's erasure story
(value-representation §4 verified), §4's structural rules and the §4.1
acyclicity argument, §4.4/§4.5's protocol surfaces (json §2.1 cite still
valid post-R180), §5's string/error/secret/command/regex bullets. §7,
holding only resolved markers, retitled "Resolved" (the R166/R168
reclassification class). Swept: `equality.md` §2, §5 ×2, §6 (five rows + the
tower note + the census), §7; `bytes.md` §6; `channels.md` §3.

**R182 — match.md validated: two mechanical gaps fixed; two semantic finds
raised for ruling, not decided.** The load-bearing core checked clean against
every ruling it leans on: §2's typed-binding dispatch already speaks R177's
corrected mechanism list verbatim (interval, union decomposition, applied-set,
constraint predicate, signature table); §2.2/§2.3's `is`-not-`as` machinery
and the signature-discharge argument match as §5.1 and functions §3.2; §2.1's
type-terminator set carries R137's `where` property; §4's destructuring
consistency holds at both ends (destructuring §2.1 has its half of the
absent-key note, §3.1 the R147 back-flow); §9.1's no-coverage-analysis rule is
exactly what numeric-tower §6 requires of exhaustiveness under a growing
universe; §7's total-order literals, §10, and the `@int`-in-patterns rule
(unchanged by R175 — `int` is not a proto) all stand. The mechanical fixes:
§2's prose literal list lacked **`inf`** (its own §2.1 grammar row has it, and
keywords §4's reserve-them-to-match-them argument covers it identically), and
§2's pattern-kind bullets lacked the **enum-variant pattern** outright — the
grammar row existed, the prose enumeration skipped it; added with the enum §4
cite. The two finds, parked in the still-open tail: **negative numeric
literals are unwritable as patterns** (literals are non-negative and the sign
is an operator, numeric-operators §1.1 — so `match (x) { -5 => ... }` has no
grammar; yet `-inf` is a blessed lexical pair, keywords §4, suggesting
sign-folding in pattern position is nearly forced); and **§11's match-capture
model is anomalous** — `match use (io)` appears nowhere else in the corpus,
keywords §3's `use` inventory has no match position, no other non-callable
construct owns a grant frame, and deep-const snapshot capture for an
immediately-evaluated expression diverges observably from inline evaluation
(live reads during guard sequences). Both need rulings. Swept: `match.md`
§2 ×2. *(Both ruled the same day: R183, R184.)*

**R183 — signed numeric literal patterns: the leading `-` folds in pattern
position.** R182's first flag, ruled *admit*: a match pattern's numeric
literal takes an optional leading `-` — `-5`, `-1.5`, `-inf`, range endpoints
(`-10..-1`), alternation members (`-1 | 1`) — folded by the **parser** from
`MINUS` + literal into one signed-literal pattern node; no lexer change, no
negative-literal token, numeric-operators §1.1's sign-is-an-operator rule
untouched everywhere expressions live. The parsability question was raised
and answered: the fold is unambiguous **because patterns admit no
operators** — pattern position is its own closed grammar, so a leading `-`
has no competing reading, and one token of lookahead (must be `INT`,
`DOUBLE`, or `inf`) decides; LL(1)-clean, and the mechanism is exactly the
`MINUS KW_INF` pair keywords §4 already blessed for `-inf`, generalized.
Three edges inherited rather than invented: the most-negative-`int` literal
form is recognized as at the expression boundary (numeric-operators §1.1's
special case); `-0.0` as a pattern matches both zeros (the total order
merges them, match §7 — the sign adds no distinction); **no `-nan`** (a nan
carries no meaningful sign at the language level). Swept: `match.md` §2 (the
literal bullet), §2.1 (the grammar's literal row), §5 (range endpoints);
`numeric-operators.md` §1.1 (the fold noted beside the operator rule);
`keywords.md` §4 (the `inf` row generalizes its own precedent).

**R184 — match is inline: no capture, no `use` clause, the enclosing frame's
grant; §11's lambda-capture model retired as a fossil.** R182's second flag,
ruled as recommended: a match expression evaluates **immediately, inline, in
the enclosing frame** — it captures nothing (guards and arm bodies read
surrounding bindings **live**, so a write-back between guard evaluations is
visible to later guards, exactly as in an `if` branch), and it takes no
`use` clause: authority is the enclosing frame's grant, covering arm bodies
precisely as it covers any inline branch (capabilities §5). `match use (...)`
is **retired** — no non-callable construct owns a grant frame, and keywords
§3's two-position `use` inventory (header; call-site delegation, R112) is
complete without it. The old §11 (deep-`const` snapshots plus a match-level
`use`) was a fossil of treating the arms as a deferred closure; the
distinction that mattered survives untouched — a `fn` literal *inside* an
arm body is an ordinary closure and captures by snapshot (functions §2): the
function captures, never the match. Grep confirms no other spec referenced
match-capture or `match use`, so the rewrite is single-file. Swept:
`match.md` §11 (rewritten).

**R185 — any.md validated: the F22 answer stands; the universal and excluded
lists each gained the entries the corpus had since ruled around them.** The
core checked clean — the surgical rule (universal operations work,
type-specific narrow first), §3's rejection rationale, §4's honest-signature
note, the R126 `@@` line, and `@v` unchanged by R175 (an `any` binding is a
value binding, so introspection applies). The completions, each derived from
the file's own criterion rather than new policy: §1 gains **`??` / `???`
coalescing** (the absence/null tests read the per-value flags every `lval`
carries, value-representation §2 — a flag test is defined for every value)
and **the introspection runtime tier** (`typeName`/`kindOf`, introspection
§4.3 — whose own text says "the runtime tier makes `any` inspectable,"
aligning this spec with the overview's inspectable-not-a-dead-end claim);
§2 gains **protocol access** (`v->name`, `v?->name`) beside element access —
protocol space belongs to tables (protocols §3), so an `any` receiver is a
compile error by §2's own meaning-depends-on-type rule, previously stated
only for `.`/`[]`. Swept: `any.md` §1, §2.

**R186 — bool.md: `parseBool`'s spellings ruled, the bitwise open deferred,
and a prefix/postfix fossil caught.** The spellings: **exactly the two words
`true` and `false`, case-insensitively** — `"tRue"`, `"FALSE"` parse; one
word, any case, because booleans arrive from a casing zoo (JSON `true`,
Python `True`, SQL/INI `TRUE`) and the value set is two unambiguous words.
Two rejections, each deliberate: **`"1"`/`"0"` error** — ints inside strings;
accepting them would be the truthiness policy this spec refuses arriving at
the text boundary, and the spelling is the visible composition
(`parseInt(s)`, then the comparison you mean) — the no-int-to-bool rule (§3)
holding at parse time; and **surrounding whitespace errors** — parsing is
exact, trimming is the caller's explicit `s.trim()`, stated as the `parse*`
family's general stance (the function parses the text it is given, never a
cleaned-up version). **Non-short-circuit boolean operators: deferred**,
alongside the integer bitwise design (int §8, operators §0.4) — most code
wants short-circuit, explicit sequencing spells the rest. The validation
find, fixed: §2 illustrated logical not as **`true!`** — the *postfix*
spelling, which is the errorable-type suffix position; logical not is prefix
(`!true`), ruled corpus-wide, and the bullet now names the position split
(operators §0, type §1.1). Also aligned: the intro's "value word" → "scalar
word" (the R170 three-word layout's vocabulary). §5 retitled Resolved and
deferred. Swept: `bool.md` intro, §2, §3, §5.

**R187 — typed multi-byte reads land as `std.binary`: the exported `endian`
enum, six named reads, the size enum rejected, and five tower typeids pulled
into alpha.** The reviewed sketch (`readIntFromBytes(b, endianness,
size: enum { b16, b32, b64 }): i16|i32|int`) was sound in type but rejected
in shape, on three grounds now recorded in the module: it lacked an
**`offset`** (a positionless read parses nothing); its **union return
couples the result type to an argument value** the type system cannot see,
forcing a narrowing at every call site — resolved by the policy-verb/R157
two-names discipline, the width living in the *name*
(`readI16`/`readU16`/`readI32`/`readU32`/`readI64`/`readU64`, each
`(b, offset: int, endianness: endian)`); and it was **signed-only**, when
lengths and magic numbers read unsigned at least as often. Placement ruled
std, not bytes surface: endianness is an *encoding* concern, not a property
of the buffer — the std.math pure-domain precedent (R141), the module name
the backend's own (`encoding/binary`, the R172 mirror move), and `std.bytes`
rejected for the type-name shadow. **`endian` is a named export** (the
bless's caveat): `export const endian = enum { little, big };` — its own
small type so libraries and user functions can share the vocabulary; call
sites unchanged (fenced literals target-type). **No endianness default,
anywhere**: a default bakes a silent portability bias. Bounds panic (the
bytes indexing-misuse class); reads are pure and comptime-eligible.
**Option (a) ruled on the tower coupling**: `u64` and the
`i16`/`u16`/`i32`/`u32` constraints are **pulled into the alpha surface** —
scheduling, not design (numeric-tower §6's own "new typeids under existing
rules"); widened-then-tightened signatures rejected as a compatibility wart,
and `readU64` must not ship broken. Deferred deliberately: the write family
(its own pass — it touches the mutation surface), `readI8` (`b[i]` covers
unsigned; signed-8 reinterpretation cannot be `as` and is rare), varints.
Swept: `binary.md` (new), `bytes.md` §9 (the bullet resolved), 
`numeric-tower.md` §6 (the carve-out), `overview/types.md` (the deferred
paragraph), `index.md` (the std.binary row), `keywords.md` §5.

**R188 — command.md validated: a nonexistent "command module," a real name
collision, an untyped JSON return, and a mangled open — all fixed.** The
younger layers checked clean (§4's `pipe` is R146-current with the R158/R146
rationale intact; §2's escapes are the R150 table; the `unsafeShellExec`
references are R172-consistent; §3's interpolation and spread semantics match
spread §5). The finds: §5 claimed introspection "lives with the `command`
module," reached "module-qualified (`command.args(c)`)" — **no command module
exists**; `command` is a built-in type, and the spelling would be key access
on a type value. The functions are **builtin free functions** via UFCS, the
type-catalogue convention, with std.exec owning only the effect (R172). **The
`args` collision, forced and fixed**: std.process has exported
`args() use (argv)` since R134, and one name has one signature (functions
§3.4), so the command reader is **`argsOf(c)`** — the `*Of` suffix being the
established introspection vocabulary (`kindOf`, `baseOf`, `capabilitiesOf`),
which command introspection is; `program`/`stages`/`stageCount`/`isPipeline`
collide with nothing and keep bare names (the asymmetry accepted as minimal
change). **`debugJson` now returns `json`, not `string`**: a JSON-producing
function returning an untyped string is the exact footgun json §1.1 closes,
the output is valid by construction so the entry check elides, and the
connection to conversion §6's deferred general debug-rendering is noted.
`stages`/`argsOf` return types tightened `table` → `list` (their keys are
exactly `0..n-1`). §7's first bullet was **textually mangled** (an orphaned
fragment missing its head) and had *become* resolved — `${...expr}` is fully
specified (spread §5, lexer §6) — restored as the resolution; the
environment/cwd and stdin bullets re-pointed at std.exec §6, named as the
same opens recorded there from the effect side. secret §158's `debugJson`
mention verified unaffected by the return-type fix. Swept: `command.md`
§5 ×2, §5.1, §5.2, §7.

**R189 — double.md: the four opens sorted into their real states, and §6's
float-conversion claim corrected against the tower.** The opens, as
suspected, were largely-but-not-fully resolved — now each is placed: **a
total-equality operator resolves as *no*** (`===`/`!==` are dead, R27's F11,
associativity §4 — no second equality operator ever; the total order's
explicit form is the §2.2 comparator function, used implicitly by
`match`/`sort`, and the nan-scrutinee subtleties were settled by match §7);
**`float`: the spec deferred, the rules ruled** (the family and both
conversion directions are the tower's, §1.3/R124; only the type's own spec
and delivery wait); **the rounding functions resolved** (`trunc`/`round`/
`floor`/`ceil` are the core policy verbs, §7/R106 — never a library
question) with **fma and explicit rounding modes staying deferred** (math §5
omits them today, a std.math extension when the audience arrives); and **the
decimal bullet resolved by R161, its lean wrong twice** — `decimal` exists
and is *built-in*, not the "likely a library type" the bullet guessed
(operators need the built-in line, numeric-tower §1.4). The body fix: **§6
claimed double/float conversion is "explicit and lossy" in both directions**
— contradicting tower §1.3, where `float` → `double` widens *implicitly and
losslessly* (every binary32 value embeds exactly) and only the downward
direction is the `toFloat` function; §6 now states the split. Also
sharpened: §3's "library round-trip stringification" named — it is
`toString`, ruled the shortest round-trip rendering (the property the
exact-type crossings lean on, decimal §5). The rest checked clean: §1/§1.1
IEEE-vs-int-panic (numeric-operators §2's shapes), §2/§2.2 (the
equality/match passes verified these from the other side, R181/R182), §5
`finiteDouble`, §7's verbs, §8's literal-grammar deferral (tower §7's open).
§9 retitled Resolved and deferred. Swept: `double.md` §3, §6, §9.

**R190 — the pipeline's bootstrap resolved: a discovery stage 0
(imports-only lexing), import validation split from it, and the prelude rule
(imports precede all other top-level declarations).** The tension named and
fixed: compiler §1 ordered `lex → module resolution` without saying which
files the lexer had — but the file set is written in imports, which only
lexing can read. Ruled (the user's option 3, refined): **stage 0 discovery**
— from the source roots, an **imports-only mode of the real lexer** reads
each file's imports, BFS with a visited set, yielding the file set *and*,
as a free byproduct, the raw edge list (following an edge and retaining it
cost the same, which dissolves the option's one stated con). The rejected
alternatives recorded in place: "import analysis before lexing" is this
stage under another name (reading a path *is* lexing), and lex-and-follow
interleaving serializes on graph depth and entangles the R149 cache with
traversal order. Three soundness rules pinned: **discovery shares the
lexer's implementation** (a second scanner is a miscompilation seed — naive
scans false-positive in comments and interpolation-nested strings; Go's
`parser.ImportsOnly` shares for the same reason); **the stage is sound by
module-system construction** — imports static, top-level, literal-pathed
(modules §4, R136, R151), so the file set is decidable by lexing alone,
recorded as a fence any dynamic-import proposal must answer to; and **the
prelude rule** — imports precede all other top-level declarations, so
discovery scans O(file-head), with a violating late import a **parse
error**, which is what licenses the early stop (an under-scanned file
cannot survive to analysis). The motivation recorded at modules §4: no use
for a late import besides hurting readability. The old "module resolution"
phase becomes **§1.2 import validation** — discovery *finds*, validation
*judges* (path resolution, cycle diagnosis with the full path from the
retained edges, topological order) — and the parallelism model now names
its two gating artifacts: the **file set** unlocks lex+parse (unordered
parallel — the context-free-parser investments paying off), the **DAG**
orders only semantic layering. Swept: `compiler.md` §1 (diagram), §1.0
(new), §1.1, §1.2 (rewritten), §1.3, §2; `modules.md` §4. Grep confirms no
outside spec cited the old phase name.

**R191 — the back half of the pipeline ordered phase by phase, and the
emitted program is one Go package per Luna module, built by one `go build`.**
The validation questions answered and pinned. **The three middle phases are
not uniform** (each now states its ordering): semantic analysis is
DAG-layered on *signatures* (unchanged, §1.4); **lowering is unordered and
pipelined** — every §1.5 transformation is local to the module's own typed
AST, so a module lowers the moment its own analysis finishes, no layer
barrier (the pleasant surprise: the rich-AST phase parallelizes freely,
because the unit is the module and per-module ASTs are disjoint); and
**optimization is DAG-ordered on *bodies*** — the constraint the spec had
never stated: comptime evaluation *executes* imported pure functions
(`sqrt(2.0)` folds by running `sqrt`), needing the dependency's IR, one
notch stronger than signatures — exactly why R149's cache interface includes
const values — with the local passes (elision, DCE) free within each module.
Emission softens from its all-modules barrier to per-module pipelining
(cross-module facts were resolved *into* the IR by §1.6). **The mapping
ruled**: one Go package per Luna module, the module DAG mirrored as Go's
package graph (legal by theorem — Luna acyclic, Go requires acyclic), one
`go build` invocation, never per-package driving — Go's scheduler
parallelizes along the graph and its **per-package build cache** makes an
unchanged Luna module a skipped native compile: **maximal incremental
builds, the deciding argument**, two cache layers composing (R149 above,
Go's below), neither ours to build. The flat-package alternative rejected
with the Go fact recorded for the non-Go-fluent reader: Go has no file-level
imports — the package is the unit, same-package files share one namespace —
so "flat" means one compilation unit, dead caching, whole-program compiler
memory. Clarified in place: Go *modules* (go.mod) are versioning machinery,
not packages — the emitted program is exactly one, static, networkless.
Mechanical consequences named: capitalization mangling for cross-package
identifiers; §7.4's explicit init sequence unchanged. Swept: `compiler.md`
§1.5, §1.6, §1.7, §1.8, §2.

**R192 — the comptime engine confirmed as the IR evaluator, the evaluator
doubled as the conformance oracle, generate-and-run rejected in full, and
the FMA landmine recorded.** The implementation question re-derived what §6
already ruled (interpret the IR, never generate-and-run Go mid-compile) —
and the discussion surfaced three things the spec lacked, now §6.1/§6.2.
**The oracle** (previously living only in out-of-corpus planning notes): the
IR evaluator has two duties, one artifact — comptime engine and **reference
implementation for differential testing** of the compiled path — which turns
§6's phase-invariance requirement into a *test harness by construction*; its
unoptimizedness is a feature twice (rich unreordered comptime errors; an
oracle you optimize is an oracle you doubt), and the two-implementations
cost is bounded by the shared lowered IR — the divergence surface is two
backends' data operations, never two readings of Luna. **Generate-and-run's
four rejection grounds recorded** so the option is never half-reopened: the
marshaling wall (a comptime-produced `fn` — the generator pattern's product
— is a code pointer plus const environment, unserializable across a process
boundary, so that path contains an evaluator anyway); cross-compilation (a
second toolchain configuration mid-build, where the evaluator runs
in-process with target facts injected — R138's story assumes it); pipeline
serialization (a toolchain invocation inside §1.6's DAG loop, R191); and
sandbox by construction (the evaluator does not implement effect operations
— structural unreachability, not a property to prove of an artifact).
**§6.2, the genuinely evil one**: Go permits FMA contraction within single
expressions on some architectures (arm64, ppc64) — fused rounds once,
unfused twice — so single-expression evaluator float paths would fold the
same Luna expression to different bits on different *host* machines,
silently poisoning §8 determinism and R149 cache keys. The Go spec
guarantees rounding at explicit assignments, so: the evaluator's float
arithmetic is written fusion-proof (explicit intermediates), and the
emitter's per-node emission — which already prevents contraction — is
promoted from accident to **load-bearing invariant**: phase invariance
requires runtime floats to equal comptime folds, so no future emitter
optimization may merge float operations into single Go expressions without
answering to §6.2. Swept: `compiler.md` §6.1 (new), §6.2 (new).

**R193 — comptime host-independence audited channel by channel; targets ruled
64-bit only; cross-compilation recorded with its cgo/FFI fence.** The
question — can comptime folded on an x86 host be trusted by an arm64 build —
answered with the audit now standing as compiler §6.3, and the striking fact
recorded: **most channels were closed by rulings made for other reasons**.
Integer width — closed by the spec (no platform-sized integer exists in the
surface; `int` is i64, Go computes it identically everywhere: the 32-vs-64
poison has no channel). Float arithmetic — IEEE + §6.2 (correctly-rounded ops
bit-identical; amd64 is SSE2, Go removed x87). Endianness — **R187's
no-default rule turns out to carry a second leg**: no endian-implicit read
exists, so the host's byte order is simply unobservable (recorded at both
homes, with a **permanent fence** in std.binary §3: a native-endian
read/write may never be added). Word-size observables — R138 (no `sizeof`;
`platform.*` are injected target facts). Iteration order — insertion-ordered
tables, plus the evaluator discipline rule (no bare Go-map iteration where a
result could observe it). **The one genuinely remaining channel**:
non-correctly-rounded math functions (transcendentals) — pure-Go math is
identical everywhere, but Go carries per-arch assembly overrides on exotic
ports (s390x), so the determinism contract is **scoped to the ruled target
set**, with re-audit required per future port. **Targets: 64-bit only,
`amd64` and `arm64` at alpha** (§1.8), with the honest rationale relocated:
comptime arithmetic does *not* force this (i64 is spec-fixed; Go emulates it
on 32-bit correctly — recorded so nobody later "fixes" comptime to enable
32-bit, which would not help); the value representation does (8-byte words
throughout — the three-word `lval`, 48-bit typeids, string-inline), plus
audit scope. **Cross-compilation recorded** (§1.8): two environment
variables (`GOOS`/`GOARCH`), no toolchains, trivial exactly as long as the
program is pure Go — which it is by construction — with the fence named:
cgo breaks it, the future FFI surface rides cgo, so FFI forfeits trivial
cross-compilation, a recorded cost, not a surprise. Swept: `compiler.md`
§1.8, §6.3 (new); `binary.md` §1 (the second leg), §3 (the fence).

**R194 — the table representation recorded: the third internals sibling
(`internal-representation-of-tables.md`), from the hashmap brainstorm.** The
shape: **one ordered entries array plus key→index Go maps** — values never
enter a map; the maps hold `int32` indexes only, split `map[int64]`/
`map[string]` because Luna's two key types hit Go's specialized fast paths
exactly; deletion tombstones with lazy compaction — **the zend_array design**,
the layout validated for decades by the same workloads Luna's table semantics
descend from. The Go-map assessment recorded so it is not re-litigated: C++'s
`unordered_map` is slow *by API contract* (pointer stability forces node
chaining); Go's opposite bet (no interior pointers) bought open addressing
and, in Go 1.24, Swiss Tables over extendible hashing — the restrictive-API
lesson being the language's own. Structural wins recorded: **the map is never
iterated** (iteration is the entries array), so §6.3's no-bare-map-iteration
discipline is satisfied by construction; the never-shrinks gotcha is defused
(index-sized residue); tiny tables may skip the maps for linear scan (a
measurement knob). **Protocol state: structs, never hashmaps** — `->` is
compile-time resolved, no dynamic protocol index exists, so each applied
proto's state is a fixed field block with **unboxed** fields per declared
member types (the ~24-byte `datetime` sizing, R133, falls out); the three
dynamic surfaces (dynamic apply/unapply R123, `@@` serialization R125,
introspection read R129) are all served by **one shared static descriptor
per proto**, never per-value structures; the honest refinement — the applied
*set* is dynamic, a small `(protoId, ptr)` vector, O(#applied) ≈ O(1).
**Const tables: the struct-vs-perfect-hash dichotomy dissolved** — a PH map
storing *offsets* is the struct viewed from the dynamic path: one
insertion-ordered data block (unboxed slots where the shape pins types), one
key array (required regardless — a minimal PH accepts any input, so
membership needs stored keys), one small index; static path devirtualizes to
a field read, dynamic path runs PH → verify → offset → box-on-read, the
boxing cost accepted on the by-definition-rare path. Two open knobs, both
measurement-driven: compaction ratio, tiny-table threshold. Swept:
`internal-representation-of-tables.md` (new), `index.md` (the internals row).

**R195 — the table capacity cap stated as contract: at most 2³¹ − 1 entries
(tables §1.1).** R194's `int32` indexes had written a hard limit implicitly;
ruled explicit and *guaranteed*, in both directions — programs may rely on
the limit being this and no smaller, the runtime on it being this and no
larger, which contractualizes the 4-byte index width (half the memory of an
8-byte scheme, table-representation §1). Sharpened en route: the cap is
**one `INT32_MAX` total, not 2× across key spaces** — both maps index the
one shared entries array, so key mix never changes the limit. Precisions
pinned: the cap is on **entry count, never key magnitude** (keys remain full
`int64`; `[5_000_000_000 => x]` is one legal entry; a list's largest index is
bounded by consequence); **insertion past the cap panics `outOfMemory`** —
resource-exhaustion-shaped, the structure cannot allocate another slot —
flagged for reversal if a dedicated arm is preferred; and the sanity anchor:
≥85 GB for one table at the cap — a database's or a stream's job — with the
cap industry-normal (JVM, .NET, V8 all near 2³¹). Swept: `tables.md` §1.1
(new), `internal-representation-of-tables.md` §1 (the contractual note).

**R196 — the three storage modes, the two header flags, and the free
tombstone: table-representation §1.1.** The mode ladder: **list** (`isList`
⇒ `isContiguous`; maps *unallocated*; `t[i]` is `entries[i]` — zero hashing,
which is what "lists are stored contiguously" means operationally; the flag
is tables §2.2's ruled O(1) property implemented), **contiguous-with-holes**
(the invariant pinned: live key ≡ entries index, holes are tombstones,
iteration order ≡ index order — deleting from a list lands here with
identity addressing intact, the filter-by-delete workload staying fast), and
**mapped** (the general shape). The flip triggers found by challenge, not
threshold alone: **re-inserting a previously deleted key breaks the mode
immediately** — insertion order demands the re-add iterate *last*, so
refilling its old slot would iterate it mid-order — as do string-key and
out-of-order int inserts; the hole-ratio threshold flips lazily; the flip is
one O(n) compact-and-populate. PHP's packed arrays are the precedent a
second time (`IS_UNDEF` holes, hash on violation). **Flag placement ruled by
the corpus's own principle in disguise**: value-representation §2.1's
"one shared fact must not have per-slot copies that can disagree" (written
for `taken`) applies verbatim — the flags are referent facts on the table
header, beside sharing count, live count, hole count, next-append index.
**The tombstone design**: in-line, zero-cost, no list — the marker is the
entry value's own `isUndefined` flag, unambiguous *because* `undefined` is
unstorable in a table (a semantic rule doing representation work); the
closure recorded: in contiguous mode **the tombstone is its own correct
return value** (`t[k]` on a holed key reads the undefined-flagged slot, and
absent keys are supposed to read `undefined` — no special case exists on the
read path); and a free-list is purposeless **by semantics** — insertion
order makes slot reuse illegal (a new entry can only append), so which slots
are dead is never needed, only how many (the header counter feeding the
threshold). Honest cost kept: one predicted branch per slot and dead
cache-line pollution until compaction, bounded by the knob. Swept:
`internal-representation-of-tables.md` §1.1 (new).

**R197 — the layout's cost story closed: contiguous iteration, and sorting's
honest price (table-representation §1.2).** The crown jewel recorded:
in-order iteration is a **linear memory walk** (~48-byte in-line entries,
one to two per cache line), dereferencing **nothing at all** for the hot
element types — inline scalars, and ≤8-byte strings being
string-representation's tier-1 inline, so a table of ints or short strings
iterates as one sequential stream (contrast: node-per-element maps miss
cache per element). The flip side accepted with its accounting sharpened
three ways beyond the original framing: sort was **never cheap here**
(order is the load-bearing axis — every index changes, so the key→index
maps rebuild wholesale; entry movement is a constant factor on an O(n)
rebuild); the **common case dodges even that** (list sort renumbers
`0..n-1`, maps are nil — pure permutation; and a COW-produced sort result
is ordinary construction in sorted order); and the **standard escape is
free until needed** (sort an index permutation, apply in one pass — each
entry moves exactly once; sort-internals, no representation change). The
rejected `[]*entry` double-boxing gets its indictment: the `unordered_map`
disease imported voluntarily — per-entry allocation, GC scan pressure,
a miss per access, paid on every iteration forever to subsidize the rare
sort — and it kills the homogeneous noscan path besides. Swept:
`internal-representation-of-tables.md` §1.2 (new).

**R198 — key representation closed: the general key cannot shrink (two doors
closed by our own rulings), and the common key costs zero bytes
(table-representation §1.3).** The can-keys-be-smaller question answered
with reasons stronger than the obvious one: **16 bytes is the forbidden
union** — a `{meta, word}` key needs word to be sometimes-scalar,
sometimes-pointer, R170's precise-GC theorem striking a second time (the
inline-string argument was never what closed the door) — and **8 bytes died
by R195's own precision**: "never key magnitude" means full-range `int64`
keys, so a tagged word has not one bit to steal. One genuine shrink recorded
and deferred to the §6 knobs: the 16-byte side-array-index scheme (payload
word holds the int value, inline bytes, or a scalar *index* into a
heap-string-descriptor side array — an index, not a pointer, so the GC
objection vanishes; costs an extra hop and a lifecycle, saves 8B per stored
key). **The real optimization**: in the keyless modes (list, contiguous —
R196), key ≡ index is the invariant, so the key column is *derivable* —
entries split into values + keys columns with the keys column **nil** in
those modes, the third instance of the allocate-on-flip pattern (maps ×2,
now keys). A list entry is **24 bytes, not 48** — iteration density doubles
for the most common table shape — `foreach` yields the index as `k` (R93's
own shape), the flip's compact-and-populate pass materializes the column,
and the **homogeneous-noscan composition completes**: a scalar list is one
pointer-free block, no key column to spoil the classification. Swept:
`internal-representation-of-tables.md` §1.3 (new), §1.2 (the sizing), §6
(the key-scheme knob).

**R199 — the header in one cache line, and the empty-table singleton
(table-representation §1.4).** The overhead accounting corrected twice and
then closed: slice headers are 24 bytes (not 8), and the sketch had omitted
R198's keys column and §3's sharing count — but both dissolve: the keys
column becomes an 8-byte pointer allocated at the flip (the pattern's fourth
instance), and the flag word genuinely holds everything *because two header
fields are derivable* (live count = len − holes; next-append = len, since
contiguous mode only ever appends), the packed counts fitting because R195
caps entries at 2³¹. Result: **56 bytes → size class 64 → the whole header
is one cache line**; a list entry is 24 bytes against PHP 7.4's 32-byte
packed buckets, the pre-7.4 disaster excluded by construction. **The empty
singleton formalized**: `[]` points at one global immortal empty block —
zero allocations until first write, `[] == []` instant via the shared-storage
fast path, PHP's own precedent — with the load-bearing subtlety recorded:
the singleton is shared by every task and therefore **may not carry a
maintained sharing count** (§3's counts are non-atomic because mutable
tables are task-confined), so it joins value-representation §6.1's
count-free class (const tables): flagged always-shared, splitting
unconditionally on write, no count write ever. Considered-not-taken:
small-table inline storage (smallvec) — complicates the COW split for one
saved allocation. Swept: `internal-representation-of-tables.md` §1.4 (new).

**R200 — the small-table state needs no representation: nil-ness is the
mode, the mode bits are dropped, and the sharing count is specified
precisely (table-representation §1.5, §3).** The ≤8 design landed as the
maps' allocation gate **decoupling** from the mode flip: an order violation
materializes the keys column, the maps materialize only at len > 8, and the
dominant shape (a small string-keyed record) lives its whole life between —
where the scan wins by the **swiss-group argument** (a ≤8 table is one
swiss group; the hash exists to pick a group, and with one group scan-only
is the degenerate swiss table minus the pointless half), word-shaped besides
(inline keys are single 64-bit words, §1.3 — eight masked compares against
contiguous memory). The dispatch pseudocode recorded verbatim; the `len`
gate noted as resolving the nil-map ambiguity ("no such keys" vs "not yet
indexed") and as hysteresis-proof (the scan is always correct). **Zero mode
bits, ruled derive-don't-cache**: all four rungs are functions of nil-ness
and two counts (list = keys nil ∧ holes 0; contiguous = keys nil ∧ holes >
0; small = keys ≠ nil ∧ len ≤ 8; mapped = otherwise) — cached bits that can
disagree with the pointer are value-representation §2.1's smell in
miniature, a bug class that cannot exist when the pointer *is* the mode;
§1.4's flag word empties to immortal bit + 31-bit share + 32-bit holes.
Transitions and hysteresis pinned (violation: allocate + identity-fill +
apply; insert #9: populate only existing key spaces' maps; compaction:
rebuild non-nil maps; maps never freed). **§3 rewritten as the sharing
count's precise contract**: the COW discriminator, not GC — alias++/drop−−,
count-1 writes in place, shared writes split — with the **asymmetric
failure directions** recorded as its most important property (overcount =
spurious split, safe; undercount = aliased in-place write, unsound — the
emitter's discipline is a one-directional soundness obligation, "count
high" always legal; the sticky-flag degenerate is the all-safety extreme;
31-bit saturation degrades to exactly it, safe), the non-atomic rationale
(task confinement via the transitively-deep spawn copy), and the two
count-free citizens (const, the R199 singleton). The fingerprint word
joined the §6 knobs. Swept: `internal-representation-of-tables.md` §1.1
(the header enumeration de-staled), §1.4 (the flag word), §1.5 (new), §3
(rewritten), §6 (the knob). *(The fingerprint knob was short-lived: ruled
against the same day, R201.)*

**R201 — the fingerprint word ruled against: a knob demoted to a rejection
with a narrow revival condition.** R200 had shelved it as a measurement
knob; ruled instead as **rejected now** (table-representation §1.5), on the
economics: the swiss control word filters comparisons that are *expensive*
(dereference + memcmp on distant lines), while the small-state scan compares
single words on prefetched lines — a three-op SWAR filter in front of
~1-cycle compares saves approximately nothing, and it adds a parallel
structure maintained on every insert and delete: real complexity and a real
bug surface for a hypothetical win. Recorded as a textbook
**mis-optimization shape** — an optimization imported from a context whose
economics do not transfer. The narrow revival condition pinned: only if
measurement ever shows small tables dominated by heap (>8-byte) string keys,
where the filtered compare is genuinely a dereference plus memcmp — and the
one preserved sweetener: the header's 64-byte size class holds a spare word
of allocation slack (§1.4 is 56 bytes), so revival would cost cycles only,
zero marginal memory. Honestly noted in the spec's own words: not expected
to be implemented. §6's knob shelf now carries an explicit non-entry (a knob
implies an expected experiment; this one has a rejection instead). Swept:
`internal-representation-of-tables.md` §1.5, §6.

**R202 — interleaved entries considered and rejected: the parallel columns
stand (table-representation §1.3).** The proposal — two entry layouts,
values-only for lists and key-beside-value for mapped tables, motivated by
keyed iteration — reviewed and rejected on a corrected premise plus an
accounting table now in the spec. The premise: the keys column is **not an
indirection** (`keys[i]` is direct parallel indexing), and the only real
pointer-follow — a heap string key reaching its descriptor — is
**layout-invariant**, so interleaving changes nothing but stream count for
one operation. The accounting: one marginal win (keyed mapped iteration,
two prefetch streams → one, and prefetchers eat two streams) against two 2×
losses — mapped value-only iteration (keys dragged through cache) and the
§1.5 small-state scan (packed keys ≈ 3 lines vs 48-byte-stride ≈ 6). Since
iteration was the stated first priority, the columns win on the proposer's
own criterion. The structural cost seals it: two entry layouts fork every
value-touching operation by mode (equality walk, COW split, serialization,
spread's fold — all mode-blind today over the uniform values column) and
spoil noscan classification by mixing key representations into the value
block — the two-representations smell §1.5's derive-don't-cache ruling
already guards against. The layout question is now closed the way §1.2
closed double-boxing and R201 closed the fingerprint: with the rejected
alternative's full grounds on the record. Swept:
`internal-representation-of-tables.md` §1.3.

**R203 — the boxing policy named and bounded: type-directed boxing, with
escape analysis rejected as the alternative (compiler §7.1.1).** The
brainstorm's two options resolved into the third the corpus had implied:
**representation follows the static type, nothing else** — the box boundary
is **entry into a dynamically-typed slot**, never a function call as such
(an `int` through `fn (n: int)` travels raw forever; the same `int` into an
`any` slot boxes there, greedily). Four clarifications recorded with it:
**boxing is not allocation** (the Java instinct does not transfer — an
`lval` is a value; boxing is two-three stores with the tag half a
compile-time constant, no heap object, no indirection added — which is why
the greedy boundary needs no cleverness); **escape analysis as boxing
policy rejected**, chaining §1.4.1's standing "no Luna-level escape
analysis" — Luna does type-directed representation, Go does escape, and a
Luna escape pass would duplicate downstream machinery to shave stores off a
boundary that costs stores, with results neither local nor obviously
deterministic (R149 keying and §6.1 evaluator-emitter agreement both prefer
the type rule's locality); **the reflection worry dissolved as a theorem** —
introspection never forces a box the type system didn't already force
(`@x` on a static binding folds to a constant typeid; runtime-typeid reads
happen only on `any` values, boxed by the type rule before introspection
arrived — reflection consumes boxes, never creates them); and **the honest
residue** named (`any`-heavy code mass-materializes lvals inherently, with
the table representation carrying the mitigation). Swept: `compiler.md`
§7.1.1.

**R204 — the function representation recorded: the fourth internals sibling
(`internal-representation-of-functions.md`).** The brainstorm's value-rep
sketch landed with its two corpus-forced corrections and the structural
split that reshaped it. **A fn value is one pointer** to a closure block
`{desc, captures…}` — Go's own `funcval` shape adopted deliberately: boxes
into `lval.ptr` trivially, `==`-identity is a pointer compare (equality §2),
R203 applies with nothing fn-specific. **The per-literal/per-value split**
(the shared-static-descriptor pattern's third use, after protocol
descriptors and const-table metadata): names→positions, arity and defaults,
the capability set, and flags are facts of the *literal*, living once in an
emitted-const `fnDescriptor` — which is simultaneously **introspection's
backing store** (`params`/`paramTypes`/`capabilitiesOf`, R130, read exactly
these fields: one source of truth by construction). The corrections:
**capabilities are a bitmask, not a list** (the closed capability universe;
match §2.3's own "one bitmask compare" — and grants store nothing, tokens
erasing per compiler §7.5: the value carries requirements, the frame carries
grants); and **comptime eligibility needs no runtime flag** (comptime never
runs at runtime — the flag is evaluator-facing). Captures: the functions §2.1
const-snapshot model with R203 inside the block (captured ints are `int64`
fields), placement deferred to Go's escape analysis (§1.4.1's standing
division), capability captures costing zero bytes. **The two ABIs** — R203's
two-tier economics applied to calls: a typed **native entry** (statically
resolved calls are plain Go calls, the devirtualization target) and a
generated **dynamic trampoline** (capability bitmask check → positional bind
of `paramCount` → defaults fill the deficit or `arityError` → named binding
via the descriptor table or `namedArgumentError`, R108 → unbox, call native,
box). **Surplus arguments drop by never being consulted** — callee-driven
binding needs no drop mechanism, compile-time truncation being the same rule
statically — with the semantic line pinned: surplus argument *expressions*
still evaluate (dropping is a binding fact, never an evaluation fact, so
effects cannot vanish based on callee arity). Signature tests read the
descriptor typeid into the R131 machinery (ladder interval; pairwise
leaves). Swept: `internal-representation-of-functions.md` (new), `index.md`
(the internals row).

**R205 — capture-free literals are static singletons, and the capability
check is one masked compare against a lexical constant (function-rep §1.1,
§4).** The two deeper questions, answered and recorded. **Capture-free
literals** (`const somefn = fn () => {};`): nothing is per-value, so the
closure block is emitted as **const data** — one static block, every use the
same pointer, **zero allocations ever** (Go's own static-`funcval` trick),
joining the immortal class. The observable consequence ruled, not left as
accident: **identity canonicalizes per literal** — N evaluations of a
capture-free literal are pointer-equal, hence `==`-equal, where a capturing
literal mints per evaluation — with the clinching argument being **phase
invariance** (a comptime-folded capture-free literal and its runtime
evaluation are trivially the same value under canonicalization, awkwardly
different otherwise; Go behaves identically; the equality spec promises no
per-evaluation minting). One line: *a capture-free literal denotes one
value; its evaluations are identical.* §5's identity note amended to match.
**The capability check**: the questioner's premise corrected first — the
runtime check exists **only for fn-slot calls** (a direct by-name call from
an ungranted frame is a compile error, capabilities §5) — and then the
pretty theorem: **the executing frame's granted set is a lexical constant**
(its function's `use` clause plus R112 site delegation, both static; no
runtime grant state exists, tokens erasing per §7.5), so the emitter
materializes each dynamic site's grant as an immediate and the entire
runtime capability system compiles to `reqMask &^ SITE_GRANT != 0 → panic`
— one load, one and-not, one branch, at dynamic sites only. The check moved
**out of the trampoline to the call site** (the site owns the constant; the
trampoline takes no grant argument — R204's §4 amended). Recorded with its
philosophical due: the audit backbone costs one masked compare on the rare
path, free precisely because grants were designed lexical (no `implicit`,
R33; no dynamic capability creation) — which is what lets the constant
fold. Swept: `internal-representation-of-functions.md` §1.1 (new), §4
(the check relocated, the theorem), §5 (identity note).

**R206 — defaults ruled comptime-known constants (with fn values expressly
legal); the descriptor's fields ruled, with five derivations and two live
flag bits.** The semantic ruling, at its home (functions §3.3.1): **a
default is a comptime-known constant, never a per-call expression** — the
grounds recorded: dynamic defaults are trivially expressible where they
belong (`p?: T` + coalesce-and-call in the body, under the function's own
capabilities, visibly); the classic traps cannot exist (Python's
mutable-default dead by value semantics — a table default is COW-copied per
call; capability-in-default-position never arises; phase-invariant by
construction); and the representation falls out (const values in the
descriptor, not prologue code). **Fn-valued defaults deliberately legal**
(PHP forbids this and it is legitimately annoying): the default is the fn
*value*, nothing is called — and the ruling composes with R205: a named
top-level function is a const binding to a static closure block, literally
a RODATA constant; capability-requiring fn defaults fine (the value carries
requirements; the eventual call checks). The representation rulings
(function-rep §2): **`names` is a positional slice, never a Go map** — the
R200 small-scan lesson applied to our own runtime (a map for three
parameters), and disqualifying alone: Go maps cannot be emitted as static
data, breaking descriptor-as-RODATA; the position mapping is the index
itself. **Five derived fields are not stored** (derive-don't-cache, five
times): `paramCount` = len(names); `minArity` = len(names) − len(defaults);
**comptime eligibility ⇔ requirementMask == 0 — the R43 theorem doing
representation work** (ineligibility sources are exactly capabilities; `use`
propagates transitively, so the mask carries transitive effect-freedom);
errorability in the typeid; post-variadic named-only-ness by rule. **Flags:
two live bits** — `generator`, and `hasVariadic`, the latter load-bearing:
§4's surplus-drop rule amended — a variadic callee's surplus positional
args are not surplus, the binder branches on the flag and collects the
trailing rest list (functions §3.3.3), which is exactly why that bit is
stored rather than derived. Thirty bits honestly reserved. Swept:
`functions.md` §3.3.1 (the ruling), `internal-representation-of-functions.md`
§2 (the descriptor), §4 (the variadic branch).

**R207 — streams: generator defers ruled (exhaustion, mark-first, two
riders), and the representation recorded as the fifth internals sibling —
state machines, with goroutines and range-over-func rejected in full.** The
semantic ruling (stream §1.3): **a generator body's top-level defers run on
exhaustion**, where exhaustion is every body-exit path (completion, early
`return`, body panic), during the final pull, synchronously with
consumption — and the before-vs-on question answered precisely: the
orderings are observable in **exactly one corner** (a panicking defer whose
pull site catches with `try`), and that corner forces **mark-exhausted
first, then defers, then unwind** — the only order leaving the stream
coherent (done, defers run exactly once, re-pull reporting exhausted rather
than resuming a finished frame; a panicked body likewise marks done before
unwinding — a broken generator is never resumable). The riders:
**`yield` in a defer body is a compile error** (runs at exhaustion when
yielding is definitionally over; lexically claims the generator while never
legally executing); **an abandoned stream never runs its defers — a stated
contract** (no finalizers, the backstop-is-not-a-contract discipline), with
the idiom recorded: resources belong to the consumer's defer, which the
R121 creation-authorization model already enforces structurally (the fd is
the owner's). defer §2 cross-notes the host-special-cased exit path. **The
representation** (`internal-representation-of-streams.md`): the stream
block (state; flags; the **one-`lval` peek slot** — nothing in the API
demands more lookahead; `taken` referent-side per value-rep §2.1's own
ruling; the R206 generator bit's native entry constructing the block);
**generator bodies compile to state machines** on lowered IR — lazy start
*is* `state == 0`, abandonment is just garbage, **restart is two stores**
(R105's replay, gated by canRestart — impossible under goroutines, which
cannot be killed), chains are uniform pull-through stages, and
defer-on-exhaustion costs one hoisted store. **Goroutine-per-generator
rejected on four grounds** (the unfixable-politely leak — abandonment is
normal usage and Go cannot kill a blocked sender; 10–20× per-element cost;
confinement fragility — one-task-one-thread soundness resting on a
lockstep accident any buffering breaks; and the pinning escape does not
exist — Go has no co-scheduling primitive). **Range-over-func rejected as
the representation** (push cannot serve a pull surface — `peek`, `zip`,
`merge` pull from multiple sources alternately), allowed as a possible
foreach-boundary emission detail. Swept: `stream.md` §1.3 (new),
`defer.md` §2 (the cross-note),
`internal-representation-of-streams.md` (new), `index.md`.

**R208 — the state-machine lowering pinned pre-IR: the Go-goto constraint
forces the shape, the algorithm is five steps, and two interactions with
existing rulings were uncovered (stream-representation §2.1).** The
hypothetical question paid concretely. **The shape is forced, not chosen**:
Go's `goto` may not jump into a block, so the textbook Duff-style
switch-into-loops emission is illegal Go — the general form is the
**flattened dispatch loop** (every basic block a case, every jump a state
assignment; regenerator's same forced move for JS), with the honest cost
(Go's loop optimizations die in the dispatch loop) recovered by
**structured islands**: only the yield spine flattens; maximal yield-free
subtrees emit as real structured Go. The algorithm pinned in five steps
(CFG; cut suspension edges; **liveness → hoist only locals live across a
suspension**, hoist-all as the correct v1 knob; regions + pc-loop; append
the R207 exhaustion tail), its inputs small *because* §1.5's lowering
already desugared everything. **The two discoveries**: (1) **R148's defer
machinery relocates for generator frames** — the per-task defer list
assumed frames that do not outlive their activation, and a generator frame
suspends, so its pending defers live in the **stream block**, surviving
pulls and handoffs, drained by the exhaustion states (compiler §7.3 carries
the carve-out); (2) **`try` spanning a `yield` is the transform's hardest
corner** — a Luna `try` around a yield spans multiple resume invocations
while Go's `recover` is per-frame, so protection re-establishes from state
via a *handler-range table* (C#'s iterator exception design,
known-implementable) — with the cheap alternative (**forbid `yield` inside
`try`**, a parse restriction) recorded, and the choice **parked in the
still-open tail awaiting ruling** (handler table the default
recommendation). Also recorded: the **push-can't-suspend theorem**
sharpening §3's range-over-func rejection (early stop is not
suspend-resume; delivery-after-stop is resume-by-replay, O(n²), restartable
sources only); and the R192 parity closure — **the evaluator needs none of
this** (a suspended generator is a saved interpreter frame), so the
transform is emitter-only, sitting exactly on the divergence surface the
oracle patrols, with comptime generator folding running transform-free.
Swept: `internal-representation-of-streams.md` §2.1 (new), §3 (the
theorem); `compiler.md` §7.3 (the carve-out).

**R209 — generator `return` must be bare; `getReturn` refused; and
generator-ness confirmed unreadable-off-the-type by design (stream §1).**
The returning-a-stream analysis validated against R33's standing rule —
classification is a parse-time lexical scan per literal, never the return
type, and *couldn't* be the type: a generator `fn (): stream` and an
ordinary function returning streams built elsewhere (a stored stream; an
invoked nested generator, where the invocation is what constructs — the PHP
shape) are **observably identical to callers**, both handing back an
unstarted lazy stream — generator-ness is a private descriptor bit (R206),
and the `stream|table` mixed-return example proves the typeid could never
carry it. The safety note recorded: forgetting the IIFE invocation is a
compile error (`fn` does not fit `stream`), the trap static types close.
**The ruling the example was one brace away from**: in a generator body,
**`return` must be bare** — `return;` ends the stream (the early end,
without `break` shenanigans through enclosing loops, taking the R207
exhaustion path: mark done, run defers), while `return expr;` is a
**compile error** — the caller already received the stream at construction,
so a returned value has **no recipient**. PHP's `getReturn()` (the
out-of-band second result channel you must know to poll) deliberately
refused; the lexical rule kept airtight (a body mixing `yield` with valued
`return` is rejected, never ambiguously classified); the diagnostic
teaches the structural fix (decision in an outer ordinary function,
construction in the nested generator). Swept: `stream.md` §1 (the two new
passages), §1.3 (the bare-return cross-precision).

**R210 — `yield` inside a `try` block ruled out at parse: option B in full,
on the abandonment argument; the transform's hardest corner deleted rather
than built.** The R208 parked item, ruled. The decisive argument, found by
costing the rewrite: **a spanning catch never had power, only grouping** —
after any catch runs, the rest of its try body is abandoned (that is what
catch means, in every language), so "recover and *continue* yielding" was
never expressible with a spanning try under either option; per-element
recovery always required per-element trys. What the restriction forbids is
one spelling of "shared recovery for a prefix-run that then ends," and both
replacement spellings are already-ruled idioms: the **per-element
try-expression** with the R209 bare return (`let v = try parse(x);` — the
try-expression cannot contain a yield *structurally*, expressions not
containing statements, so the workhorse is untouched) and the
**consumer-side supervisor** (a resume panic propagates out of the pull;
`try` around consumption is the boundary errors §8.2 and io §6 already
designate). The rule's edges pinned: **catch blocks are unrestricted**
(post-recovery code is pc-shaped — `catch (e) => { yield fallback; }` is
legal, and a yielding catch inside an outer try is rejected transitively by
the same rule, self-consistent); **defer bodies are the parallel ban**
(R207) — the two places a yield can never run are the two rejected at
parse, in the same lexical walk that classifies the generator, one
try-depth counter, near-zero cost. Stream-representation §2.1's
handler-range passage rewritten as **the road not taken** (the C#-style
table, multiplied by nested trys, per-state defer-drain depths, and
rethrow — deleted rather than built); every resume frame's protection stays
ordinary R148 machinery. Swept: `stream.md` §1 (the ruling),
`internal-representation-of-streams.md` §2.1 (the passage rewritten); the
still-open tail item discharged.

**R211 — enum.md: the variant syntax gains its `:`, the payload shape ruled
the deliberate exception with constraints composing not competing, the
PascalCase field re-cased, and §9 sorted.** **The syntax**: a variant is
`name: payloadType` — the earlier juxtaposition (`circle ['radius' => int]`)
was the grammar's lone name-then-type-by-adjacency, every other
type-introducing position using `:` (parameters, bindings, proto members,
the constraint binder, match's typed binders); parses with no new machinery
(`,`/`}` already terminate types, R137). The proposed `=` rejected with the
proposer's own model turned around: **protocols use `:` for member types**
(`=` there is initializers/values, the corpus-wide split), and
`= ['radius' => int]` would even parse as a table *value* holding a type —
a misreading inviting defaults semantics variants do not have. Construction
and patterns unchanged (inside the brace fence the adjacent thing is a
value/pattern — a closed own-grammar). **The shape-vs-constraint question
ruled as composition** (new §2.3): the shaped payload is the one place
shape-typed tables exist — variant-scoped, justified not tolerated, because
two consumers *read* the contract rather than run it (the construction
checker's field-level diagnostics; the match binder inheriting declared
field types — `r: int` in `{circle ['radius' => r]}` *because* the shape
says so). A constraint cannot replace it — any shape *check* is expressible
but as an **opaque boolean**: static field types die (binders at `any`),
diagnostics collapse to "constraint failed," closedness becomes discipline,
and R137's const-only rule makes per-variant ceremony — while constraints
**compose within it** (field types may be constraints,
`['radius' => positiveInt]`, §2.1's own rule: shape owns structure,
constraints own value refinement, each doing what the other cannot) and the
whole-payload predicate contract stays available as explicit opt-in
(`circle: circleTable`). **The casing field**: ~30 PascalCase sites re-cased
(`shape`, `event`, `expr`, `direction`, `hand`, `tree`, `loggable`, `foo` —
the R171 class, now at the source), with the four outside stragglers swept
(introspection's `baseOf(@shape.circle)`, type.md ×3). **§9 retitled
Resolved and deferred**: parameterized enums reworded from "out of scope"
to *deferred with generics* (it *is* parametric typing; no enum-local
answer exists); the R131 marker already stood. Swept: `enum.md` §0, §1,
§2 (examples, prose, the ruling note), §2.1, §2.2, §2.3 (new), §8, §9,
casing throughout; `overview/types.md` (the declaration example's colon);
`introspection.md`; `type.md` ×3.

**R213 — functions.md's close audit: ten findings, all fixed — three
internal contradictions (one a whole fossil model), three stale
representation claims, three internals-compatibility gaps, and the `!=>`
spelling normalized out of existence.** The contradictions: **§7's opening
was the pre-R43 fossil** — it still claimed comptime-eligibility
"encoded into the typeid" with "comptime-eligible variants [as] distinct
typeids," flatly against §3/§5.2/R43; rewritten as the two-placement story
(errorability in the typeid — not per-value, type-derivable; eligibility on
the value — per-value, derived from the requirement set, never cached).
**§5.1's whole-program fixpoint was a fossil contradicting R33 itself**:
with no inference tier — every capability explicitly declared, propagation
by declaration (capabilities §5, verified) — the transitive information is
already in each function's own declared set, so eligibility is a **local
read**, the same shape as errorability's check; the §5 rule's redundant
second conjunct ("every callee eligible") deleted, slot-calls noted as
sound via the comptime-empty-grant panic (§5.4), and §4's parenthetical —
which cited the fossil as its contrast — now records all three checks
(errorability, capabilities, eligibility) sharing the
local-against-declarations shape. **§5.2 misstated the binding ladder**
("a `let` that could be reassigned" — `let` never rebinds, keywords §1):
the rule and example now say `var`, with `let` noted as coinciding with
`const` for functions (§8). The stale claims: §1's "16-byte value" (the
R170 sweep's fifth missed site — now the logical/physical split), §1's
"everything lives in the type" (wrong §6 cite, and imprecise post-R43 —
now type-or-descriptor, never lval flags), §3.3.2's "function values pay a
small fixed metadata cost" (names ride the shared per-literal descriptor;
a value pays one pointer, R204/R206). The compatibility gaps closed at
both ends: **the eligibility formula refined** in function-representation
§2 — comptime capabilities exist (§5.5) and occupy mask bits, so the test
is `mask & NON_COMPTIME_CAPS == 0`, a link-time constant, still one
compare (R206's `mask == 0` had forgotten them); **the optimistic-`as`
claim shown representable for free** (function-representation §5): the
claimed signature rides the lval typeid — re-typing the lval is what `as`
*is* — the real one stays in the descriptor, and §3.2's two-sided checks
read their natural sources; **R205's per-literal identity folded into
functions.md §2.1** (observable `==` semantics were living only in an
internals file): a capture-free literal denotes one value, its evaluations
identical, phase invariance the clinching ground. And **`!=>` is ruled a
non-token**: the errorability `!` belongs to the return type
(`: int! => {`), the detached `int !=>` habit recorded as a drafting
accident — technically the same parse, stylistically wrong — and all six
corpus sites normalized (functions §4, exec, capabilities ×3,
stringBuilder). Swept: `functions.md` §1, §2.1, §3 (the R45 note), §3.3.2,
§4 (the example + the parenthetical), §5, §5.1 (rewritten), §5.2, §7
(rewritten); `internal-representation-of-functions.md` §2, §5;
`exec.md`, `capabilities.md` ×3, `stringBuilder.md` (the spelling).

**R212 — general shape types deferred, with the full design recorded in a
new home: `deferred-constructs/`, for deferred core-language constructs.**
The brainstorm's outcome, recorded so it is never re-derived
(`deferred-constructs/shape-type.md`): **the sharpened case for** (the real
payoff is not sealing — expressible today as a table constraint plus §7's
mutation machinery — but *static field access*, a contract accessors and
binders read, enum §2.3's two-readers lesson generalized); **the sharpened
case against, which currently wins** (the corpus's records trajectory is
protos — `@fileInfo`, `@commandResult`, `datetime`, and json §4's planned
generated read side all reached for them — so shapes would open a second
record mechanism beside one in motion, the abuse vector made concrete);
**the impossibility result** (static-only checking cannot exist: dynamic
data — `fromJson` output, the flagship case — has nothing static to check,
so any shape type is forced onto the constraint model, static-when-provable
plus runtime entry plus mutation class — the distinction from constraints
being only *what the compiler reads*, never *when it checks*); **the forced
design if revived** (`const circle = shape ['radius' => int];` — const-only
per R137's discipline, sugar over a table constraint plus retained readable
structure, exact and closed with width subtyping refused, inline anonymous
shapes rejected permanently); and **the revival condition** (only if
post-alpha experience shows protos too heavy for plain data records — the
apply ceremony, nominal identity, or serialization split proving a real
tax). The new folder's charter stated in the index: deferred *language*
constructs, distinct from deferred std libraries (recorded in modules) and
deferred decisions (recorded where they arose). Swept:
`deferred-constructs/shape-type.md` (new), `index.md` (the new section).

**R214 — never.md validated: the `die` contradiction resolved (R134 forces
always-throws), `die` given its ruled home, and two fossils fixed.** The
finds: **never.md contradicted itself about `die`** — §1 called it "the
primitive case" of `fn (): never` (the exit form) while §2 defined it
`never!` (always-throws), and `die` was defined *nowhere* (errors.md
silent; usage only in examples). **R134 decides it**: a `die` that exited
the process would *be* the abolished `exit()`, while `die` as `never!`
composes exactly — "exit with a message" *is* "throw an error nobody
catches," unwinding through pending `defer`s (structured teardown, R134's
own requirement), reaching `main`, the runtime reporting and terminating
(functions §4's `main` story); a *caught* `die` is an ordinary handled
error — both correct, chosen by the caller, which no true process-exit
could offer. Ruled and homed: **`fn die(msg: string): never!` ≡
`throw error(msg)`**, a builtin free function beside the throwaway it
sugars (errors §5.2). The corollary recorded in never §1: since users
cannot construct panic values (errors §9), `fn (): never`'s only honest
user-written inhabitants are **divergers** (event loops) — §1's `fatal`
example, which called the throwing `die` from a non-`!` function (a
functions-§4 containment violation besides), moved to §2 as the `never!`
wrapper it must be. The fossils: **§2.1 said errorability "is propagated
over the call graph"** — the exact phrasing functions §4 refuses (declared,
locally verified, never propagated) — fixed; and **§3's lattice example
called `exit()`**, the R134-dead function, contradicting §1's own
parenthetical two sections up (the earlier combing fixed §1's mention and
missed §3) — now `die("no x")`, with the value-arm/error-channel split
noted. Checked clean: §1.1/§1.2's unverified-claim-plus-runtime-trap
design, §3's algebra, §4's asymmetry, §5's both resolutions. Swept:
`never.md` §1, §2, §2.1, §3; `errors.md` §5.2 (the `die` ruling).
*(The `die` half is superseded the same day: R215.)*

**R215 — `die` re-ruled: a true panic, `fn die(msg: string): never` with no
`!` — superseding R214's declarable-thrower answer, whose R134 argument
conflated *must unwind* with *must be declarable*.** The correction owned in
the entry: both channels unwind through defers with structured teardown, so
R134 never forced the declarable form — it forbids only non-unwinding
process exit. What decides the channel is die's **use profile**, and it is
panic-shaped on every axis: assertions, impossible states, and usage-bail
are exceptional conditions *outside the caller's contract*, so the
declarable form would have spread **`fn!` contagion** through every
die-using codebase (exactly what the panic exemption exists to prevent,
functions §4); as a true panic, `die` is **catchable at deliberate
boundaries** (a bare `catch (e)` catches everything by the single-root
design; supervisors catch `panic`-typed) so die-using code survives
supervised contexts without systematic deletion; and the `!` is not merely
unnecessary but **false** — `!` names the declarable channel, and a
panic-raiser has nothing on it. Mechanically: `die` raises the new **`died`
panic type** (carrying the message; joins the tree beside `cancelled`,
whose non-`*Error` name is the precedent — rename open to the user),
runtime-minted on the caller's behalf, so "users construct no panic values"
stands unbroken. The pleasant reversal recorded: never §1's *original*
text ("die is itself the primitive case") was right all along — R214's
restructure swung the wrong way — and `fn (): never`'s inhabitants are now
two kinds, divergers *and* always-panickers, the latter existing because
`die` does; `fatal` returns to §1 as an always-panicker, §2's example
renamed (`rejectAll`) as the deliberately-not-die declarable shape, and
errors §5.2 now pairs **the two one-line failure forms, one per channel**:
`throw error('msg')` (declarable, `fn!` required) and `die('msg')` (panic,
no signature change). R214's fossil fixes (the §2.1 propagation phrasing,
§3's `exit()`) stand. Swept: `errors.md` §5.2 (rewritten), the panic tree
(the `died` arm); `never.md` §1, §2, §3.

**R216 — numeric-tower.md closed out: the last two opens deferred, §7
retitled, and §6's stale status sentence corrected.** The file was indeed
mostly correct (the R164/R187 sweeps had kept it current); the remainders:
**literals for the wider types — deferred**, cheaply, because the standing
answer already covers the need (R161's comptime-folded-constructor ruling,
applied three times since: `parseDecimal`, `parseRational`,
`complex(re, im)` all fold at build time — a suffix would buy spelling, not
capability, and waits until it earns grammar); **bit operations —
deferred** with the bitwise spec as a whole (int §8, operators §0.4 — `&`
and `|` are spoken for, the surface needs its own pass, nothing in alpha
demands it). §7 retitled Resolved and deferred: three resolved markers
(R161/R162/R164) stand, nothing open. The one staleness found: §6's opener
"the current specced primitives are `int`, `double`, and `byte`" — stale
twice, since the exact types *are* specced (specced ≠ delivered) and R187
pulled five typeids into alpha — now states the **alpha-delivered core**
(int, double, byte, plus the R187 five) with the carve-out below it
unchanged. Swept: `numeric-tower.md` §6, §7.

**R217 — named captures ruled (canonical `(?<name>...)`, the PHP-shaped
both-key-space match table), and the `regex.escape` module fossil normalized
to the one `regexEscape`.** The brainstorm's outcome. **Syntax** (regex
§5.4): `(?<name>...)` canonical — the form JavaScript, .NET, and Go 1.22+
share — with RE2's classic `(?P<name>...)` an accepted engine synonym; the
feature costs the language nothing (the literal passes its interior as
pattern source; the only literal-special characters are `/` and `${`), works
identically in plain literals, verbose mode, and `regex()`, and a bad group
name in a literal errors at the literal per §2's standing rule. **The match
table** (string-api §5): one table, **both key spaces** — int keys the
positional groups (0 the whole match), a named group under **both** its
number and its name — PHP's `preg_match` shape, proven by the lineage
Luna's tables descend from; `m['year']` is the access path, keyed
destructuring composes for free, and no accessor function exists or is
needed (the proposed `.regexCaptures()` reinterpreted: regex-side
introspection, deferred as `captureNames(r): list` until need). Positions
deliberately not in the table (a position-returning variant deferred).
Named backreferences (`\k<name>`) ride the `b` engine's deferral. **The
fossil** (R188's class, regex edition): §7 spelled `regex.escape` "in the
regex module," and string-api §5's entry was a "delegating alias"
(`const regexEscape = regex.escape`) — but `regex` is a built-in type, no
module exists, and there was never anything to delegate. Ruled as the user
spelled it: **`fn regexEscape(str: string): string`**, one builtin free
function, one name one signature, pure and comptime-eligible (which is what
lets it run inside literal interpolation, regex §7) — both files rewritten
onto it. regex §9's named-captures open resolved; the b-engine, step-budget,
and delimiter opens stand. Swept: `regex.md` §5.4 (new), §7, §9;
`strings.md` §5 (the match-shape ruling; the `regexEscape` entry rewritten).

**R218 — secret.md closed out: `reveal` stays a name, mark-preserving operations
dead, never-equal confirmed; two missed sweep sites fixed.** The three opens,
ruled. **`reveal` is not a keyword**: the guarantee never lived in the name — it
lives in the gate ⊆ frame-grant check at the effect site, which no aliasing can
launder (the laundering theorem, secret §5). Keyword-ness would harden the wrong
half: `reveal` the default gate is an ordinary capability const, deliberately
aliasable and delegatable like every capability (capabilities §5.2), so freezing
the extractor's spelling while the authority it checks stays a value protects
nothing; the audit that matters is `use (reveal…)` in signatures, not `reveal(`
at call sites — and keywords.md never listed it, so the ruling formalizes the de
facto corpus. **No mark-preserving operations** — concatenation specifically is
dead, and the invariant is stated positively: **`reveal` is the sole consumer of
a secret's payload**. Three grounds: the combined gate set is invented policy no
matter how it is chosen, while reveal-then-rewrap (`secret(reveal(a) .
reveal(b), @g)`) already requires the combined authority at the site, proven by
machinery that exists (the re-gating shape, §3.2); admitting even one string
operation for secret operands is the flagged-string failure mode §1 refuses; and
payloads are `string | bytes | table`, so "concatenation" is meaningless for a
third of the set. **Never equal, including the same secret** — confirmed, not
newly ruled: equality §5 already carried the full position (constant `false`,
non-reflexive beside IEEE nan, the same contagion, `command` identity-equal
precisely to avoid dragging secret comparison in); secret §7 had simply gone
stale behind it. Stated on the record: the payloads are never consulted at all,
so `==` is not a timing oracle either; token checking is reveal-and-compare
inside the granted frame, with the constant-time form named in std.crypto's
deferred scope (R140), which inherits "key material is `secret`-shaped." **Two
missed sweep sites fixed in passing**: §3.2's constructor still read `raw:
string|bytes` — R79's signature, missed by R111's widening, and
self-contradictory since R113 made the stacktrace a gated *table* secret
(`@revealStackTrace`) that the runtime itself constructs; now
`string|bytes|table`. And §6 still promised per-kind extractors "checked at
compile time" — R111's plural-extractor language, superseded by R113's one union
`reveal`, contradicting §3.1 and §5 in the same file; rewritten onto the union
story. Swept: `secret.md` (§3.2 two sites, §6, §7 → Resolved).

**R219 — the default gate renamed `revealSecret`; the capability/extractor
collision dissolved and the namespacing deferral retired.** The default gate was
spelled `reveal`, the same identifier as the extractor it keys, and capabilities
§1 held the pair apart with "different namespaces (the exact namespacing
mechanism is the module system's, deferred)" — a two-namespace resolution rule
(`use`-position vs expression position) the module system would have owed, since
both names are exports of `std.secret` and both appear bare in one signature
(`use (reveal)` over a body calling `reveal(s)`). Renamed: the capability is
**`revealSecret`**; the function stays `reveal`. Grounds. **The family already
names authorities apart from the functions they gate** — `env` gates
`envVars()`, `argv` gates `args()`, `egress` gates `dial`/`send` — and the
`reveal`/`reveal` pair was the lone exception, so the rename joins the standing
style instead of inventing one. **The R113 `reveal*` convention survives
verbatim**, being prefix-anchored: `grep "use (reveal"` still returns every
revelation authority, and `revealSecret` is the convention's own shape, reveal +
the material opened (default-gated secrets are exactly the ones that are just "a
secret"). The name is the default gate's key, not a skeleton key — R113's
demotion stands, restated against the new spelling at secret §5. **The rejected
alternative** — keeping `reveal` for the capability and renaming the function
(`expose`) — would orphan the convention's verb (capabilities named for an act
the language no longer spells), drag `canReveal` with it, and churn every
example call site in the corpus, for no audit gain: R218 already placed the
audit in `use (reveal…)` signatures, not call-site spellings. Beyond the
collision, the rename buys a real deletion: capabilities §1's deferral dies —
capability names are chosen distinct from the functions they gate, so no
namespacing rule is owed anywhere. Adjacency noted and accepted: json's
`revealSecrets` revealer parameter (json §2.1) is plural, a delegated closure,
not a capability. Swept: `capabilities.md` (intro, §1 declarations + the
coexistence passage rewritten, §2 subtype example, §3.1 illegal-forms ×4, §4
export line + example, §5 absence example + honest-limit, §7.2 impersonation,
§9 table — `reveal` and `env` rows), `secret.md` (§3.2 default set ×2, §5
default-gate key + convention list, §7 header + R218 bullet + new bullet),
`overview/types.md` (capability bullet ×2 + subtype line), `index.md` (std.exec
row), `exec.md` (§0 shape cite), `process.md` (§2 double gate),
`constraints.md` (§2 purity list). The extractor's own mentions (`reveal(`,
`canReveal`, prose verbs) stand unchanged throughout, as does
`revealStackTrace`. One misstatement fixed in passing: secret §7's R218 bullet
called the default gate "deliberately aliasable and delegatable like every
capability" — capability aliasing is illegal (nocopy, capabilities §3.1);
aliasable-without-laundering describes the *extractor*, delegatable the
capability, and the bullet now says which is which (R218's entry above stands
as written, frozen history).

**R220 — the gate-naming constraint re-shaped exact and compiler-blessed;
§3.3's elision claim made sound; `secret use (…)` rejected as syntax.** The
question was compiler legibility: the constraint idiom (R111) names a gate
through a runtime predicate the compiler treats as opaque, and a dedicated form
was proposed — `const dbSecret = secret use (dbCred)`. **Rejected, three
grounds.** *Grammar*: `use (` after a completed postfix expression *is*
call-site delegation, decided at one token (R112, associativity §1), and
`secret` is a predeclared identifier, not a keyword (keywords §5), so
`secret use (dbCred)` already parses today as delegation over the expression
`secret`; discriminating a second meaning needs either reserving `secret` or
value-dependent parsing — two settled principles spent on one form. *Keyword
semantics*: `use`'s two positions share one meaning, capture — capabilities
flow where `use` names them (keywords §3); a gate stamp captures nothing and
demands rather than holds, which is why gates are spelled as typeid *data*
(`@dbCred`, secret §3.2) in the first place — and it would dilute the
`use (dbCred)` audit (capabilities §5) with hits that neither exercise nor
extend authority. *And it would not buy the analysis*: the blocker was never
predicate opacity. R111's conventional predicate,
`gatesOf(s).exists(@dbCred)`, is a lower bound — a
`{@dbCred, @prodAccess}`-gated value satisfies it, enters, and panics at
`reveal` under `use (dbCred)` — so §3.3's claim that the compiler "can prove
the gate check passes and elide it" was **unsound as written**, on exactly that
counterexample, under any surface syntax. **Ruled instead: the blessed shape is
exact** — `constraint s: secret where gatesOf(s) == [@dbCred]`, superseding
R111's `exists` convention. An upper bound is what elision needs: a frame's
grant is a superset of its declared `use` (delegation only adds, capabilities
§5.2), so pinned-gates ⊆ declared-`use` proves gates ⊆ grant and the
reveal-site check is provably redundant — elided in constraints §9.5's
meaning-preserving sense (the comptime corner is unreachable: a frame declaring
a non-comptime capability is comptime-ineligible, functions §5.5). The exact
predicate also moves the wrong-material failure to the entry boundary, where
the mismatch is legible; the `exists` shape admitted values that were doomed to
panic mid-body. Mechanics: list `==` is strict structural equality (equality
§4) and type values compare by identity, so the predicate is one identity
compare per gate; `gatesOf` order is now pinned as **construction order**
(secret §5, previously unstated and newly load-bearing), knowable by the
predicate's author through the pairing convention — the module that constructs
the material writes its constraint beside it. A set-semantics probe
(`gatesAre`) was considered and **deferred** until a real order mismatch shows
up. The sugar `constraint secret use (dbCred)` — keyword-introduced,
second-token-decided against the binder form, desugaring to the exact
predicate — is recorded as **considered and deferred**: grammatically clean,
sugar over one existing mechanism, but the blessed predicate alone serves
until demand exists. Fixed in passing: `secret` was missing from keywords §5's
predeclared-names list while every other builtin type is present; added.
Swept: `secret.md` (§3.3 heading, predicate, example comment, all three
bullets, §5 `gatesOf` order note), `keywords.md` (§5).

**R221 — inline streams ruled: the `gen` block, a keyword-introduced literal;
`stream {}` rejected.** The stream spec's oldest open, closed. The form:
`gen { body }`, an expression, optionally `gen use (io) { body }` — **pure
sugar** over the immediately-invoked anonymous generator
(`(fn () => { body })()`, the exact shape §1's R209 discussion already
exhibited), so lazy-start, const-snapshot capture, creation-site capability
check (the R121 carrier story), R207/R209/R210, and §1.1 keys all inherit with
zero new semantics. One strengthening: a `gen` block is a generator **by
form** — the keyword is the lexical marker, no yield-scan, and a yield-free
`gen {}` is the canonical empty stream. The spelling question was decided by
parse cost. `stream {}` — the blessed candidate — fails twice: in
return-annotation position it collides with the generator's own canonical
spelling (`fn (): stream { ... }` — the brace must be the body), and
`stream use (io) {` re-runs the delegation-grammar objection R220 just
sustained against `secret use (…)`, with `stream` a predeclared identifier
whose shadowability is a standing flag (keywords §6) — contextual parsing
would hang on an unresolved question. `gen` is a full keyword and only ever
the literal former: **one-token decision, zero carve-outs**, `use (` after a
`gen` head joining `fn`/`test` in R112's list verbatim. The former≠type split
is not new doctrine but the house pattern: backticks construct a `command`,
slashes a `regex`, `gen` a `stream` — and the corpus already names the concept
(generator functions, generator bodies; fn : function :: gen : generator; the
same spelling Rust chose for the same construct). Also rejected: `yield { }`
(one keyword meaning construct at the head and suspend inside, two lines
apart), compound formers (`fn stream`), a sigil literal (command and regex
earned theirs from universal outside precedent; generators have none to
borrow). Swept: `stream.md` (§1 intro, §1.4 new, §8), `keywords.md` (§1 new
row, §6 classification note), `internal-representation-of-streams.md` (the
parity note's example generator was named `gen()` — now a keyword; renamed
`naturals()` in passing).

**R222 — single-pass enforced everywhere; the `useAfter` panic family;
granularity-via-subtyping recorded as a standing convention.** §8's
consistency review ran, and the empty second pass lost: **re-consuming an
exhausted stream panics** (`useAfterConsumed`) — the silent empty pass hid
exactly the double-consumption bugs single-pass exists to surface, and every
neighboring construct (`spawn`, `await`, `close`, every catalogue call)
already enforced its move. Consumption means `foreach`, destructuring, spread,
and the catalogue; the panic is about **handle reuse, never emptiness** (a
first pass over an empty stream is ordinary and zero iterations). Probes stay
total — the hard/soft pairing: `isConsumed` now answers "can this handle still
produce?" (true on exhausted **and** taken, the exact guard for the panic),
joining `taken()` as a query-not-a-use, legal on taken handles; `peek`/
`isEmpty` panic on a taken handle (they must run a body another owner holds)
and stay honest on an exhausted one. The panic taxonomy: parent **`useAfter`**
(use of a spent handle — the use-after-free of a language without free), the
prefix naming the family exactly as `reveal*`/`unsafe*` do (R113, R219), with
children **`useAfterTaken`** (moved — naming what was previously a bare
`panic`, concurrency §2.3) and **`useAfterConsumed`** (ended; the participle
pair matches the probes: `taken()` / `isConsumed()`). Siblings, not a chain: a
prospective `useAfterClose` (file after close, today a plain io failure) fits
a family of siblings but not a chain rooted at taken — recorded as a deferred
candidate. `doubleAwait` **reparented** under `useAfterTaken` (a second await
is precisely a taken use; the name survives as the finer instance) — the new
convention applied to an existing type on day one. The convention itself,
recorded at errors §2: **granularity via subtyping** — one recovery boundary,
several causes ⇒ finer panic types under a catchable parent, never one coarse
type; catch the parent for the family, a child for the cause; new runtime
panics join or found a family rather than widening a catch-all. Swept:
`stream.md` (§2 rewritten, §3 probes, §7.3, §8, §2.1/§6.1 stale "discipline"
cites), `errors.md` (§2 tree + convention paragraph), `concurrency.md` (§2.3 —
the "consumed yields empty, no panic" sentence, the direct casualty),
`spread.md` (§2), `stream-api.md` (§2 — `isConsumed`/`isEmpty` probe wording),
`index.md` (the stream row refreshed across R221–R223).

**R223 — `yield from` ruled: one compound token, pure sugar; the implicit key
unified as the running next-integer-index.** Delegation:
`yield from src;` ≡ `foreach (k => v in src) { yield k => v; }` — the desugar
*is* the spec, so table iteration, laziness (one element per outer pull), the
R210/R207 bans (the desugar contains `yield`), and stream-taking (a stream
operand transfers, iterable-functions §1.5 extended to the syntax) all
inherit. Lexing: `yield from` is a single compound token; `from` stays
unreserved, exactly as contextual as import's `from` — the one casualty,
bare-yielding a binding named `from`, parenthesizes (`yield (from);`). What
makes pure sugar *possible* is the key ruling: §1.1's implicit key is
redefined as a **running next-integer-index over the whole yield sequence** —
a bare yield emits at the counter; an explicit or delegated integer key is
emitted verbatim and advances the counter past it (`max(counter, k+1)`);
string keys never touch it. This is the table-literal fold's counter
(spread §1) minus its renumbering, and the divergence is principled: a table's
keys are an index and must be unique, a stream's keys are **flowing data**
(duplicates representable, passing through), with uniqueness enforced only at
materialization by `collect` applying the table's own write rules. PHP's
delegation key-collision wart dies as a corollary of the counter rule, not as
a carve-out on delegation. Swept: `stream.md` (§1.1 rewritten, §1.5 new),
`keywords.md` (§2 yield row).

**R224 — stream.md's remaining opens closed: bidirectional axed, element
typing is the table rules, cleanup is R207.** **Bidirectional generators:
axed.** `channel()` already returns the two-way pair (`[sink, stream]`,
channels §1); a value-returning yield would be a second, *implicit* channel
mechanism — against small-surface — and it composes wrong: a sent-back value
cannot thread through `map`/`filter`, the wart PHP's `send()` and Python's
both carry into every pipeline. Corollary banked immediately: **`yield` is a
statement**, never an expression — `let x = yield v` is unrepresentable rather
than forbidden, the hole closed by construction. **Element typing:** a stream
carries no static element typing; elements and keys are per-element dynamic
exactly as table members (the §6 sentence stood; the open closes onto it), no
generics by doctrine (secret §3.3), transformers track nothing, narrowing is
the consumer's `is`/`as`/`match`. One precision added: keys are **`int` or
`string` and nothing else** — the table key rule verbatim, enforced at
`yield k => v` (`typeError`; compile error where statically evident) — without
which `collect` and the stream↔table parallel (list : keyed table :: implicit
: explicit, §1.1) break. **Cleanup:** no scoped-cleanup convenience rides the
stream; R207 is complete — defers on exhaustion at the final pull, abandoned
streams run nothing by stated contract, resources belong to the consumer's
`defer` (io §6). Swept: `stream.md` (§1.1 key rule, §1.5 statement note, §8 →
Resolved).

**R225 — the fused lowering (stream-representation §2.2): the as-if license
and the syntax-directed catalogue.** Streams that cannot be observed *as
values* need never exist: producer and consumer compile into one loop — no
streamBlock, no dispatch — `foreach (x in (1..n).filter(p).take(k))` emitting
the counting loop a programmer would write. The license extends elision's
doctrine (constraints §9.5, "never changes meaning") from checks to
representation, and it is *sound because the semantics already paid for it*:
pull-driven single-pass **is** loop semantics (the observable order — steps
alternating per pull, defers at the final pull, panics from the pull site —
is the loop's own), and the language outlaws observation exactly where fusion
would show (stream §7.3's enforced move; R222's exhausted-handle upgrade).
Identification respects the compiler's refusal of flow-sensitivity (compiler
§1.4.1): a **syntax-directed catalogue** in the §9.5 shape — mechanism fixed,
catalogue pending implementation. Tier 1: the chain wholly visible at its
consumption site (a `foreach` head, spread, or `collect` argument), source a
range / `toStream` / `gen` block / generator call, stages catalogue
transformers with literal arguments — nothing bound, nothing escapes, nothing
observes, decided from the parse tree alone; the bound-once tier is deferred.
Fusion is a fast path, never load-bearing. The name: "flattening" was already
taken in the same file (§2's flattened dispatch loop), "stream elision" would
stretch a term the corpus uses precisely for check-removal — **fusion** is the
literature-exact word (the Haskell lineage) under the file's own §2.1 word,
**lowering**. Distinct from comptime generator folding (same file), which
evaluates rather than emits. Swept:
`internal-representation-of-streams.md` (§2.2 new), `stream.md` (§7.2
cross-reference).

**R226 — `u64` renamed `uint`; `nat` minted as the non-negative `int`
constraint; the two jobs of "unsigned" split by family.** The trigger was
strings.md's sentinel review: `indexOf` wants a structurally non-negative
index type, the obvious name was `uint`, and a full-width merge of that name
with `u64` was proposed — one 64-bit unsigned type for both jobs. **The merge
is rejected on the tower's own rulings.** A full-width unsigned type cannot
subtype `int` — int §6.2's top-bit argument is representation, not policy —
so merged-`uint` indexes would live in the *other* integer family, where the
ruled semantics bite in sequence: `idx + 1` is an explicit-crossing compile
error (§3; literals are int-family), `idx == 0` is *silently false* (the
`1 == 1.0` rule — the exact silent-wrong-value class the language closes),
`idx - 1` panics at zero (unsigned computes at its own width with no wider
signed space for intermediates — the `size_t` footgun, panic-flavored), and
an offset could not key a table (`int | string`) or enter any `int`-taking
signature without ceremony. The empirical record concurs: Go — the backend —
has a full-width `uint` and deliberately types `len()` as `int`. **Ruled
instead, the split**: *the rename* — the unsigned primitive is **`uint`**,
the chain `u8 <: u16 <: u32 <: uint` the exact mirror of
`i8 <: i16 <: i32 <: int`, each family topped by its width-unsuffixed 64-bit
primitive, so the name now promises exactly what the type delivers (`int`'s
full-width unsigned twin — the aesthetic complaint that triggered the merge
proposal is *dissolved*, not argued away); std.binary's `readU64` keeps the
wire width in the function name (R187's width-in-the-name rule names what is
read, not what is returned) and returns `uint`; `toU64` follows R106's
target-naming to `toUint`. And *the mint* — **`nat`** =
`constraint i: int where i >= 0` (constraints §10, predeclared, keywords §5,
alpha beside `byte`): the signed family's one **range** refinement, range
`0..2^63 - 1`, `nat <: int`, compute-at-`int`-width with entry checks, so a
`nat` flows into every `int` surface with zero crossing ceremony — the
property indexes need and the unsigned family structurally cannot offer. The
division of labor, stated: **`uint` is the boundary type** (binary formats
whose values need the top half), **`nat` is the domain refinement** (indexes,
counts, sizes). The string-API adoption of `nat` that motivated all this is
**deferred by instruction** — a follow-up ruling sweeps that surface. One
contradiction fixed in passing: int §6.1's code block still declared
`u8`/`u32` as constraints on *`int`* with a `u8 == byte` note — flatly
contradicting §6.4's "`u8` is not `byte`" row and numeric-tower §1.2's
constraints-on-the-unsigned-primitive rule; stale since the family split,
rewritten signed-only (and its `someU8` example, now wrong-family, moved to
`i16`). Swept: `numeric-tower.md` (§1.1 `nat` paragraph, §1.2 chain + mirror
note, §1.4, §2/§3 examples incl. `toUint`, §4, §5 table — `uint (u64)` row
mirroring `int (i64)`, new `nat` row, §6 + carve-out), `int.md` (§6.1
rewritten, §6.2 renamed, §6.3, §6.4 table + `nat` row, §8), `binary.md` (§2
signature + readU64 note, §2.1), `as.md` (×4), `overview/types.md` (×3),
`overview/high-level-overview.md`, `constraints.md` (§10 `nat`),
`keywords.md` (§5), `index.md` (std.binary row), `introspection.md` (comment),
`compiler.md` (§7.5 width row), `internal-representation-of-tables.md` (R201
note's `u64` was a machine word, reworded "64-bit word").

**R227 — strings.md split: `string.md` (the type) and `string-api.md` (the
catalogue), the stream/stream-api shape.** The corpus had already voted:
cross-references throughout said "string-api §N" while the file was
`strings.md`, and the file titled "String API" carried type-level content —
immutability and binding, the units doctrine, the no-concat rule,
interpolation and the R150 escape table — fused to the function catalogue.
Split along that seam, mirroring the existing stream.md / stream-api.md pair.
**string.md** takes the type: §1 immutability and binding, §2 the units
doctrine (the count functions and offset conventions stay catalogue-side), §3
no concatenation operator, §4 the builder's place, §5 interpolation with §5.1
the escape table — the R150 one-authority moves with its section, and every
pointer at it moves too. **string-api.md** keeps the catalogue with **§1–§10
numbering preserved**, which is what made the split's direction obvious:
every pre-existing "string-api §N" citation (regex ×8, bytes, stream,
stringBuilder, control-flow, conversion) now lands correctly with no edit.
§1 keeps the catalogue rules (no-overloading, UFCS, naming, return shapes)
with pointers to the moved type facts; the opens renumber §14 → §11. **No
function content changed** (by instruction): the review findings — `cString`'s
R150-stale `"\0"`, the `-1` sentinels, the slice-length sentinel — stay
recorded in §11 for their own ruling. The file rename is a git move
(history preserved on the catalogue half). Swept (file-name and section
fixes): `associativity.md` (×3 → string §3), `operators.md` (×2),
`conversion.md` (×3), `lexer.md` (×6 → string §5 / §5.1),
`lexical-structure.md`, `command.md` (§2), `bytes.md` (×2 + §9 name),
`bool.md`, `spread.md`, `functions.md` ("strings spec" → string-api §1),
`type.md` (same), `stringBuilder.md` (×7),
`internal-representation-of-strings.md` (×3), `one-billion-rows.md`,
`overview/types.md` (string row), `index.md` (one row → two).

**R228 — string-api closed out: `nat` adopted across the surface, both
sentinels retired, the three failure channels recorded; `stringBoundaryError`
defined and homed; two R150-stale `\0` fixed; the builder's `bytes` parameter
renamed.** The half R226 deferred by instruction, landed. **The convention,
recorded at §1**: a **miss** is an answer — `null`, typed `T?` — a **failure**
is the error channel (`parseInt: int!`), a **misuse** panics. The miss half's
type-system ground: `undefined` is unspellable in a return type, so
undefined-returners degrade to `any` (`keyOf`, `peek`), while null keeps
`indexOf: nat?` precise; regex `find` was already `table | null`, now spelled
`table?`, the house sugar. (`keyOf` and iterable `find` under the same
convention: a recorded follow-up for iterable-functions' own ruling.) **The
adoption**: `indexOf`/`lastIndexOf` return `nat?` and take `from: nat` — the
`-1` sentinel dies structurally; the counts (`byteLength`, `codepointCount`,
`graphemeCount`, `count`, `cStringLength`) return `nat`;
`repeat(times: nat)` (a negative count was a silent `""`), pad widths,
`chr(codepoint: nat)`, `split`'s `limit: nat` (its documented `0` = uncapped
stands; a negative computed limit, formerly silently uncapped, now panics at
entry). **`slice`**: `offset: nat`, and `length: nat? = null` — null is "to
the end" (the `finalGlue` pattern: absence spelled as absence), retiring the
`length <= 0` sentinel, under which a negative *computed* length silently
returned the whole tail. **`stringBoundaryError`** — name kept — gains a real
definition: a leaf in errors §2's panic tree (the §6 cite previously pointed
at §10, C-string interop, and the type existed nowhere); and the hard/soft
pair completes with the probe **`isCodepointBoundary(str, offset: int):
bool`** — deliberately `int`, never panicking: a guard must be askable with
exactly the arithmetic-produced offset it guards, a negative one answering
`false` (`byteLength` itself is a boundary, the end). **`chr`** panics
(`typeError`) on a non-scalar (surrogates, above `0x10FFFF`), compile error
where statically evident — the runtime mirror of `\u{…}`'s lex-time rejection.
**Two R150 sweep misses fixed**: §10 defined `cString` as exactly `"$str\0"`
and exampled `"ab\0cd"` — `\0` is an R150 lex error whose tombstone sits in
string §5.1; both now `\u{0}`. **The `sliceBytes` ghost deleted**: §2 named
it beside `slice`, no catalogue entry ever existed — a fossil of the
pre-byte-slice draft. **The builder**: `capacityHint` is **bytes** (the unit
`reserve` already speaks) and `nat`; `reserve`'s parameter, literally named
`bytes`, renamed **`numBytes`** — shadowing a predeclared type name in a std
signature is legal (keywords §5, shadowability itself a standing flag) and
terrible, so the surface will not model it. **Opens** (§11 → Resolved): the
slice unit confirmed (byte-offset, boundary-checked, no `graphemeSlice`);
`cString()` returns a `string` and no distinct C-string type exists
(unnecessary — NUL is one valid codepoint, validity holds, interior-NUL
honesty lives in `cStringLength`, the deferred FFI consumes a `string`);
error-vs-sentinel resolved by the convention; regex interpolation resolved at
regex §7 (comptime-only). **One newly open, held from this ruling**: the `""`
separator — currently an error pointing at `graphemes()`; challenged in
review: an empty separator matches at every position, so "split on `""`" *is*
"split everywhere," and any non-error meaning must silently pick the unit
(JS's code-unit answer severs surrogate pairs) — the silent choice string §2
exists to refuse — against the convenience of a runtime-computed separator
that happens to be empty; recorded at §11, pending decision. Swept:
`string-api.md` (§1 return shapes, §2 counts + conventions ×2, §4 `repeat` +
`chr`, §5 `indexOf`/`lastIndexOf`/`count`/`find`, §6 `slice` + the new probe,
§8 `split` limit + pad widths, §10 ×3, §11 rewritten), `stringBuilder.md`
(§2 signature + unit sentence, protocol sketch, member table),
`errors.md` (§2 tree).

**R229 — the `""` separator: ban kept, widened to the every-match class, and
named `emptyNeedle`.** R228's held-open item, closed on the ground the
challenge itself sharpened. The empty string occurs at every position, so
"split on `""`" *is* "split everywhere," and advancing past a zero-width
match forces a silently chosen unit — JS's code-unit answer severs surrogate
pairs, PHP's `str_replace` answers "matches nowhere"; the cross-language
chaos is the ambiguity's signature — exactly the silent pick string §2 exists
to refuse. **The principled boundary is not `split`**: the disease is
every-match *enumeration*, so the ban covers the trio — `split`, `count`,
`replace` — and `split`'s **int arm joins** (previously unspecified): a chunk
width `<= 0` is a zero-byte step, the same emptiness. **Single-match
operations are defined, not banned** — a single answer needs no unit, and the
definitions are the universal ones: `indexOf(s, "") = 0`,
`lastIndexOf(s, "") = byteLength(s)` (both ends are boundaries),
`contains`/`startsWith`/`endsWith` trivially `true`, `replaceFirst` inserts
at offset 0, `before`/`after` follow `indexOf`. **The name: `emptyNeedle`** —
a leaf beside `divisionByZero` in errors §2, the same shape: a bare
condition-name for one argument-degeneracy, spelled in the catalogue's own
parameter vocabulary (§5's `needle`); no family to join or found (R222's
convention consulted). Rejected spellings: `emptySeparator` /
`invalidSeparator` (split-only framing, and "invalid" says nothing the tree's
names don't say better), `zeroWidthMatch` (mechanism-named, and the panic
fires before any match exists), the `*Error` suffixes (the condition-name kin
go bare). The diagnostic names the three explicit spellings of "every
position" — `graphemes()`, `codepoints()`, `bytes()`. Swept: `string-api.md`
(§5 the empty-needle rule + `count` note, §6 `before`/`after` note, §7
`replace`/`replaceFirst`, §8 `split`, §11 open → resolved), `errors.md`
(§2 tree).

**R230 — type.md tidied: the `is` position restored, the binding ladder
stated, §5's enumeration completed, `baseOf` reconciled, the `declared` open
closed.** Five smalls from a review pass. **(1)** §1.1's type-position list
omitted `is` — `x is @P` is a type position, as overview/types.md already
states; one word restored. **(2)** §2 read "a `type` binding is `const`"
while §1.1's own examples bind with `let` — resolved by the strings ladder
(string §1): an inline immutable primitive has no interior for `const` to
additionally freeze, so **`let` and `const` are equivalent and both legal**;
`var` stays disallowed. **(3)** §5's enumeration of type-position forms
omitted **error and capability types** — the two interval-heavy tree families
(typed `catch` binders name one; `use` clauses and gate-set typeids name the
other) — completed, with `nat` joining the constraint examples (R226).
**(4)** The `baseOf` three-way wobble resolved in favor of the facts:
introspection §4.1 **defines** `baseOf` (R131 — exported signature and full
entry), and constraints already cites it as in place; the one stale site was
introspection's own `kindOf` note still reading "`baseOf` pending, §7" —
written before R131 landed in the same section and never updated; type.md
§6's constraint-base bullet now cites `baseOf` directly. **(5)** §9's open —
`declared` on nested and destructured bindings — closes with **zero new
machinery**: a destructured binder is an ordinary binding with an inferred
type, and §4 already makes written and inferred indistinguishable, so the
answer is uniform; a table **field is not a binding**, and §4 already defines
`declared` on bindings only, so a field is a compile error by the
definition's own consequence, not by new rule. Swept: `type.md` (§1.1, §2,
§5, §6, §9 → Resolved), `introspection.md` (§4.1 note).

**R231 — undefined.md closed out: `any`-held undefined is uniform, `@` on
undefined is a use, taken is neither undefined nor a type change; the
production count made honest.** Both opens, ruled. **`undefined` in `any`**:
`any` binds anything by definition, so an `any`-typed binding may hold
`undefined` — and nothing changes, because the rule was never about the
binding's type: holding is never the error, and **no operations are defined
on an undefined value**. The sanctioned surface is exhaustive and short —
hold, `== undefined` (equality), coalesce (`??`/`?.`/`???`), and `declared`
on the *binding* (a compile-time binding fact, not a value use; type §4).
Everything else is a use, panicking at runtime and rejected at compile time
where statically evident — **including `@`**: asking the type of a value
that is not there is a use, which settles the open's reflection half (`@u`
panics; `declared u` answers — the binding/value split of type §4 doing the
work). The open's "generic positions" phrase is dropped: Luna has no
generics. **Taken vs undefined**: confirmed distinct, and sharpened with the
type angle — a taken stream is **still a `stream`**; takenness is referent
*state*, deliberately not type-determining (value-representation §2.1's
discipline), read by the total queries (`taken`, `isConsumed`, R222) and
enforced by `useAfterTaken` (errors §2). The behavioral split is the design:
absence **coalesces away** (a routine question), taken **panics through** (a
moved value is a bug). Productions stay closed at the two classes; a future
producer needs its own ruling, never a quiet extension. **Fixed in passing**:
§1 claimed exactly *two productions* (missing key, void return) while the
corpus already produced `undefined` from the catalogue's miss and empty
answers (`keyOf`/`find` misses, `first`/`last`/`peek` on empty,
iterable-functions §2; a destructuring shortfall, stream §2.1) — an
undercount, not a contradiction in meaning; reframed as two production
**classes** (the absent read, the void return) with the catalogue family
enumerated under the first. Also verified, no change needed: §3.2's
`remove`/`unset` matches tables §4.1 (the static `delete` form is §4.1.1's
recorded rejection), and §5's `var x?: T = null` is the ruled optional
spelling (variables §1.2). Swept: `undefined.md` (§1, §7 → Resolved).

**R232 — lexer.md brought current: `gen` and `yield from` land, `meta` and
`from` leave, `get`/`set` ruled identifiers; the keyword count made honest.**
The pre-implementation audit of lexer.md against its authorities found four
rulings that never reached it — the token inventory is the first thing an
implementation consumes, so the sweep debt came due. **In**: `KW_GEN`
(`\bgen\b`, R221 — keywords.md §1 had the row, the lexer did not) and
`KW_YIELD_FROM` (R223's compound token), pattern `\byield[ \t\r\n]+from\b`,
attempted before `KW_YIELD` exactly as `KW_MATCH_BANG` precedes `KW_MATCH`;
the two words separate by whitespace only, the regex normative — a comment
between them defeats the fold — which is the lexical fact behind stream
§1.5's parenthesize-to-bare-yield-`from` casualty. **Out**: `KW_META`
(retired R96, its keywords.md row deleted in R99 — but R101's lexer sweep
landed `?->` without removing the row, a missed site standing since) and
`KW_FROM` with its "reserved everywhere" note (G1's first resolution,
superseded by R223: `from` is unreserved, `IDENT` to the lexer, contextual
to the import grammar; the lexer's G1 bullet now records both halves of the
history). **Ruled here**: `get` / `set` — R99 added them to keywords §4 as
contextual modifiers without fixing their lexing, and the lexer was silent;
they follow the `panic` pattern, **identifiers, never reserved**, recognized
positionally by the parser in proto member heads
(`<const|let|var> [get] [set] name`), which keeps `get` and `set` usable as
ordinary member and binding names. The alternative — reserving them like
`in`/`by`/`self` — was rejected: those three sit where expressions do and
must stay unbindable, while `get`/`set` occupy one closed declaration head
where position already disambiguates, and reserving two catalogue-plausible
names buys nothing. **The count**: §3 claimed "47 patterns" over a 49-row
table; the inventory is now stated as 47 word-shaped keywords plus the two
compound tokens, 49 in all, matching keywords.md §1–§4 exactly. Discovered
in the same pass and fixed: operators.md's `yield` row predated R223 and
showed only suspension — delegation added, with the §1.5 cite. Import,
deliberately, gets **no** compound-token analogue: after a braced import
list the from-clause is the only legal continuation, so the parser matches
the identifier by spelling (the ECMAScript mechanism — `from` is unreserved
there too), whereas `yield` is followed by an arbitrary expression, which is
exactly why delegation alone needed the fold and carries the parenthesize
casualty; the mechanism is now stated at the syntax it governs (modules §5).
Not swept:
the tooling grammars (shiki-luna.ts, the vscode tmLanguage, tree-sitter) —
their keyword alternation is R85-era (`meta`, `from`, `view`; no `gen`) and
stale across many rulings, not just these; derived artifacts, regenerated as
a batch when the grammar is next published, not per-ruling. Swept:
`lexer.md` (§3 intro, table, notes, §8 ordering, G1 bullet), `keywords.md`
(§4 `get`/`set` row), `operators.md` (§0 yield row), `modules.md` (§5
selective bullet).

**R233 — the Go toolchain is bundled, not required: one pin serves as both
build toolchain and emitted-`go.mod` floor, three components are dropped
from the shipped bundle, and Go's build cache moves under
`$HOME/.lunalang/`.** The implementation's first decision forced a spec
question the corpus had left unstated: "the backend is Go source handed to
the Go toolchain" (compiler §0) never said *whose* toolchain, and the answer
decides whether installing Luna means installing Go. **It does not** — the
pinned toolchain ships inside the `luna` binary. That is what makes the
single-binary claim (index, "the whole toolchain, runner, compiler,
formatter, and language server") true rather than nearly true, and it is the
difference between a language that emits Go and a language whose users must
know that it does. **The pin is dual-purpose**, which is what promotes it
from packaging to design: it is simultaneously the toolchain that builds a
program and the language floor of the single static `go.mod` the emitter
writes (compiler §1.8), and because the toolchain travels with it that
floor has **no user-facing version to negotiate** — nobody downstream can be
too old. Pinned at **Go 1.26.5**, matching the sandbox that builds the
compiler (claude-agent-plan §A.1), bumped deliberately, never by drift.
**Licensing, checked rather than assumed**: Go is BSD-3-Clause, which
permits redistribution in binary form inside a closed product, with an
attribution notice in the accompanying materials and no endorsement implied
— but a stock distribution ships **33 license files**, not one, and three
components carry terms the rest do not. All three are **excluded from the
bundle**: `cmd/vendor/github.com/google/pprof` (Apache-2.0, reachable only
through `go tool pprof`), `crypto/internal/boring` (an
OpenSSL/SSLeay/ISC/Intel-BSD composite whose SSLeay half carries the
advertising clause, inert unless built with `GOEXPERIMENT=boringcrypto`),
and the race detector's `.syso` blobs (built from LLVM `compiler-rt`, dual
NCSA/MIT, and the one component Go ships with no license file beside it).
None is reachable when emitting machine code, so each would attach an
obligation to every `luna` distribution in exchange for code that never
runs. The rejected alternative was **shipping the tarball intact**, and its
argument is real and was weighed: an unmodified tree carries its own 33
license files and satisfies the source-redistribution clause automatically,
where a curated bundle must curate notices to match. It loses anyway,
because the notice artifact is required either way — intactness buys
convenience, not compliance — and convenience is a poor reason to ship an
advertising clause and an unlicensed-in-tree binary blob. The exclusions are
a *distribution* boundary, not a development one: the sandbox keeps all
three, so testing-strategy §7's `-race` gate is untouched. **The consequence
that reaches the cache**: current Go distributions ship **no precompiled
standard library**, so the bundle must carry Go's `src/` (which R193's
two-environment-variable cross-compilation needs regardless) and the first
build on a user's machine compiles the standard-library packages it touches
before anything is warm. That cost is paid once and cached — and the cache
is **Luna's**, rooted under `$HOME/.lunalang/` rather than the user's
default `GOCACHE`, so the two layers compiler §1.8 already composes are
evicted by one owner; a bundled toolchain writing to a cache Luna does not
own would leak build state the eviction model cannot see. Root only: the
layout beneath it, and whether Go's cache ages out on the incremental
spec's §4.2 schedule, stay open and are recorded there. **Not ruled here**,
deliberately: the notice artifact's contents, and the trademark question
(the Go marks may be retained on substantially unmodified redistribution,
but the exclusions above make this bundle modified — a `trademark@go`
question before any binary ships, not a spec question). Swept: `compiler.md`
(§0.1 new bundled-toolchain paragraph, §1.8 assemble-and-build), `index.md`
(distribution paragraph, Backend, Using the compiler),
`incremental-compilation-build-cache.md` (intro, §5 open question).

**R234 — Luna self-hosts, and that splits R192's one artifact in two: the
comptime evaluator follows the compiler into Luna, the oracle stays Go,
test-only and permanent, and the compiler may never import it.** The
decision is to **self-host** — the production compiler written in Luna —
taken for a reason stronger than dogfooding: testing-strategy §1 concedes
that the alpha front-end is shared between oracle and backend and therefore
*not* differential-tested, "closable later by an independent front-end," and
a Luna-written front-end **is** that front-end. Self-hosting also puts the
language's own performance on the critical path of every build, which is the
point: the overflow-enforcement tax and the cost of tree-walking over tagged
unions stop being speculation and become findings. **Bootstrap late, not
early**, and the rejected alternative is recorded with its argument intact —
bootstrapping early on a minimal subset, to avoid translating a large Go
compiler later. It fails on its premise: nothing is ever *translated*,
because the **spec** is the source of truth and the differential harness
grades a from-scratch Luna implementation automatically, so the cost being
avoided does not exist, while the cost incurred — every ruling applied to two
implementations through the churn years — is real and recurring. It also
fights itself, since a compiler written in a deliberately minimal subset
dogfoods the subset, not the language. So: idiomatic full Luna, after the
surface is frozen and std has grown what a compiler needs (filesystem, exec,
process, a hash, platform — `claude-agent-plan` §F's "io, stringBuilder, and
perhaps 1–2 others" is about a third of it). **Vocabulary, fixed before it
collides**: this corpus already uses **self-hosted backend** for *native
codegen replacing Go* (R170, string-representation §11.1), a different axis
entirely. **Self-hosting** is the compiler being written in Luna, and it
removes no Go whatever — the backend still emits Go source and R233 still
bundles the Go toolchain. **The split.** R192 gave one artifact two duties.
Self-hosting cannot hold both: the **evaluator** is a component of the
compiler, in-process, producing values that land in the compiler's own IR and
typetable, so it follows the compiler into Luna; the **oracle's** entire
value is being an implementation the compiler under test did not produce, so
it stays Go. R192's four grounds against generate-and-run survive untouched —
they concern comptime being in-process — but its economy does not: "the
two-implementations cost is bounded by structure" was the argument for one
artifact, and the duplication is now the product, bought deliberately for
independence. **The oracle is a test instrument that ships exactly once**: it
is alpha v0, the first shippable Luna, and stops shipping when a stable
Luna-written compiler exists. It is nonetheless **permanent**, because after
self-hosting testing-strategy §3's script-vs-compiled pair degenerates — one
compiler drives both paths, so it tests the run/cache path, not two readings
of the semantics — leaving oracle-vs-compiled as the only pair with two
independent implementations behind it; retiring the oracle would end
differential testing outright. **"Frozen" is re-read, not relaxed**: frozen
*to the agent loop*, the `:ro` mount that stops an agent editing its way past
a failing differential, never feature-complete — a reference that cannot
track rulings drifts from the spec on the first one after it is written, and
then every divergence is ambiguous, compiler bug or stale oracle. **The
compiler may never import the oracle**, gated on the build graph rather than
on intention. The **Go-FFI route** — a Luna compiler calling the Go oracle
for comptime — is rejected in full, first because it would convert the oracle
from a check on the compiler into a dependency of it, making an oracle bug a
*production miscompilation* and paying for two implementations to get one's
worth of testing; and on three mechanical grounds besides: `unsafeFfi` is a
**C** FFI riding cgo, forfeiting R193's two-environment-variable
cross-compilation for the one program that most needs both targets; it is not
even a foreign boundary, since Luna compiles *to* Go and both sides would
share one runtime and one GC; and `unsafeFfi` is deferred to its own spec,
which would put an unwritten spec on self-hosting's critical path.
**Bifurcated into §6.2/§6.3**, both of which were written as hand-discipline
about *Go* code: fusion-proof float arithmetic stays a discipline for the Go
oracle and becomes **structural** for a Luna evaluator (per-node emission
never puts two float operations in one Go expression), and the
no-bare-Go-map-iteration fence becomes *unviolatable* for a Luna evaluator,
which has no Go map to reach for. One requirement replaces what the
unification gave free: evaluator and oracle must fold floats **to the same
bits**, a disagreement there being indistinguishable from a real differential
failure. Transcendental agreement survives — a Luna evaluator reaches Go's
`math` through `std.math`, longer chain, identical callee. Swept:
`compiler.md` (§6.1 rewritten, §6.2 float fence, §6.3 order fence and the
transcendental channel), `internal-representation-of-streams.md` (§2.1's R192
parity note, §2.2's folding-vs-fusion bullet — both said "oracle" where they
meant the evaluator). Also swept, outside the spec corpus:
`testing-strategy.md` (§1, §3), `claude-agent-plan.md` (Phase 0.2). **Not
ruled here**: when the bootstrap happens, and whether Phase 1's Go-written
compiler is kept alongside the interpreter as a second reference.

**R235 — F2's predecessor set enumerated, inverted: the division side is the
closed one, `}` and the three mode-path closers join it, and the mode openers
finally get names.** The pre-implementation pass over lexer.md found F2's
regex-vs-division rule stated as a list of *regex* positions — "after
operators, `(`, `[`, `{`, `,`, `;`, `=>`, `=`, `return`, **and the other
prefix positions**" — which is not a rule an implementation or a test can
consume, and which had left `}` unclassified in either direction. The defect
is structural, not sloppiness: the regex-allowed side is open-ended (every
operator, every declaration keyword, every opener), so any enumeration of it
trails off. **Inverted, the rule closes**: `/` is division exactly when the
previous significant token can end a value, otherwise it opens a regex — and
the division side is **25 tokens**, written out in F2 as a table. The other
85 allow a regex, as do the two no-predecessor positions, start-of-file and
just after `INTERP_OPEN`. *Significant* excludes what §2 skips. **`RPAREN` is
unconditional, and Luna earns that where JavaScript cannot**: there, `)` is
undecidable from the token alone because a brace-less body lets an expression
follow it (`if (c) /re/.test(x)`), which is parser knowledge. Luna has no
such form, every control-flow construct being either block (`{` follows) or
postfix (`;` follows), so no `)` precedes an expression. **`RBRACE` is
unconditional as a consequence of tables being bracket-delimited** (`['name'
=> 'Lucas']`): the braces that close a *value* are enum-variant literals
(`{point}`) and `match`, both ordinary, while the brace that closes a *block*
is a statement boundary where a regex could begin in principle — except the
no-discard rule (errors §540) makes a regex-leading statement unreachable,
since the legal spelling is `_ = /re/…`. Tracking block-vs-value on the brace
stack was **considered and rejected as not lexer-decidable**: `{point}` and
`{point;}` diverge arbitrarily far past the `{`, and §1.1 grants the lexer no
parser feedback, lexing being a complete phase over all files in parallel.
**The three mode-path closers are the entries most easily missed**, because
the two halves of the fact live in different sections: §4's span regexes are
the no-`${` fast path only (F1, F3), so an interpolated literal ends not at
`STRING_DQ`/`REGEX`/`COMMAND` but at `DQ_CLOSE`/`REGEX_CLOSE`/`CMD_CLOSE` —
omit them and `"a${x}b" / 2` opens a regex. **`KW_SELF` likewise**: a
value-ender that §3's value-keyword list does not name, that list enumerating
the six literal-denoting words. **`WILDCARD` and `QUESTION` are placed on the
safe side of an asymmetry** rather than on a semantic argument, neither being
plausibly followed by a regex: misclassifying toward division is benign — one
`SLASH`, a parse error in place, §1.1's collect-don't-abort recovery
unharmed — while misclassifying toward regex is destructive, the scanner
hunting the next unescaped `/` and swallowing arbitrary source into a ruined
token stream. Anything genuinely indifferent therefore belongs on the
division side. The one token where this reasoning does **not** license
conservatism is `BANG`: `!` is prefix logical-not and a regex literal is an
expression, so `!` directly followed by one is grammatically constructible,
and unlike the other dual-role tokens (`&`, `@`, `?`) its
regex-allowed classification is a requirement. Recorded with it: **the flag
is per-frame state on the mode stack, not a global** — `INTERP_EXPR` lexes
with the full `DEFAULT` rule set, so entering a splice resets it to
regex-allowed and popping restores the enclosing frame's; one global boolean
gets either `"${x}" / 2` or `"${/re/.source}"` wrong depending on which way
it leaks. **The opener gap, closed in the same pass**: §6 named the three
mode *closers* and only described the openers, in a file otherwise exact
enough about its inventory to have caught a 47-vs-49 keyword discrepancy
(R232). They are now `DQ_OPEN`, `REGEX_OPEN`, `CMD_OPEN` — mode-path only,
none a division position — and the consequence is stated where it was
previously implicit: the same source construct reaches the parser as
**one token or a delimited sequence** depending on whether the fast path
applied — F1/F3's optimization, not two grammars.
Swept: `lexer.md` (§1 mode table, §5 slash note, §6 openers and
closers, F2 rewritten). Not swept: the tooling grammars, on R232's precedent:
their alternation is stale across many rulings and is regenerated as a batch
when the grammar is next published. **Not ruled here**, both live and both
touching this file: whether comments and whitespace are *retained* for the
formatter (§2 still marks all four trivia forms "skipped", which a formatter
cannot work from), and the position model (byte offsets in tokens versus rune
columns at the diagnostic boundary, with LSP wanting UTF-16 regardless).

**R236 — the two lexer opens closed together: whitespace and comments
become emitted *trivia* tokens, and tokens carry byte spans with line and
rune column computed on demand.** Both were flagged unruled by R235 and
both are settled here, because they are the same question asked twice —
what a token stream must carry for consumers the compiler itself is not.
**Trivia.** §2's four forms (`WHITESPACE`, `SHEBANG`, `LINE_COMMENT`,
`BLOCK_COMMENT`) were marked *skipped*; they are now **emitted**,
collectively the **trivia** tokens. The forcing argument is the formatter:
`luna -f` (compiler §0.1) cannot reproduce text the lexer discarded, and it
is the sole consumer that needs it — the parser filters trivia out with one
predicate over the stream. The rejected alternative was a
**trivia-suppressing lexer mode**, off for compilation and on for the
formatter and language server. It buys a shorter stream and costs the thing
this file just spent R235 arguing against: two token streams for one source
construct, hence two behaviours to test and a standing risk that formatter
and compiler disagree about what a file contains. One stream, one filter.
Two consequences recorded where they land: **spans now tile the source
exactly** — monotonic, gapless, summing to file length — promoting a
partial invariant into a total one for any consumer checking positions; and
**trivia attachment is deliberately left undecided**, the stream staying
flat with no leading/trailing binding of a comment to a neighbouring token,
because whether a trailing `// …` belongs to the line above or below is
formatting policy and the formatter spec is pending. A flat stream lets
that spec choose; an attached one would freeze the answer before the
question is asked. One interaction swept in passing: `KW_YIELD_FROM`'s fold
**consumes** the run between the two words, so its span covers them both
and no `WHITESPACE` token is emitted inside the compound — the same fact
R223's "a comment between them defeats the fold" states from the other
side. **Positions.** Tokens carry a **byte offset and length** — a span,
not a copy, the source buffer being retained for anyone wanting the lexeme
— and line/rune column are **computed, never stored**. Bytes win on four
counts, none of them convenience: the scanner is byte-native over text the
ingress check has already validated (lexical-structure §1); the tiling
invariant above is exact in bytes and only approximate in anything else;
slicing a line out for an error snippet needs byte indices regardless; and
the language server wants **UTF-16 code units**, a third encoding, so a
conversion layer exists no matter what and is cheapest with one canonical
origin to convert from. The corpus already leaned this way without saying
so: lexical-structure §1 refuses to strip a BOM *precisely* to keep byte
offset 0 meaningful for the shebang rule. Diagnostics nonetheless report a
**rune** column, being what a person counts, and it stays off the common
path — a line-start table built once per file, binary search to the line,
then a rune count over that line's prefix alone, bounded by line length
rather than file length, and never run at all by a compile that emits no
diagnostic. **The pure-ASCII fast path costs nothing** because the pass
that enables it already exists: lexical-structure §1's validation visits
every byte "exactly once, up front", so recording whether any byte was ≥
`0x80` is free, and in a file where none was the rune column *is* the byte
column. Even where non-ASCII appears, §1 confines it to string, `command`,
`regex`, and comment content, so the counting path runs only for a
diagnostic on a line carrying some. Swept: `lexer.md` (§2 retitled and
rewritten, §3's `yield from` note, F2's *significant* definition, new §9
Token positions), `lexical-structure.md` (§1 pure-ASCII record, §3's "both
skipped"), `compiler.md` (§1.1 Lex output). **Not ruled here**: tab
handling in a column count — one column or advance to the next tab stop —
which is a rendering question for the diagnostics spec and reaches neither
byte spans nor the lexer; recorded as open in §9 so it is not decided by
whatever the first implementation happens to do.

**R237 — regex literals take a sigil: `~"…"` replaces bare `/…/`, which
deletes F2's context rule and supersedes R235's division set outright.** A
bare `/…/` literal is the single most expensive piece of surface syntax in
the lexer: it makes `/` three-way ambiguous — division, comment, or regex —
resolvable only from the **previous significant token**, which RE2 cannot
express (no lookbehind), so it becomes lexer state threaded per-frame
through the mode stack. R235 had just specified that state precisely, as a
closed 25-token division set. **R237 deletes the mechanism rather than the
specification of it**: `~` is a token in no other position in the language,
so `~"` is self-identifying, `/` is unconditionally division-or-comment
decided by the *next* byte, and the lexer consults no context whatsoever.
R235 is superseded on this point and kept in the log; its other half —
naming the mode openers `DQ_OPEN`/`REGEX_OPEN`/`CMD_OPEN` — stands. **The
decisive argument was the failure mode, not the table size.**
Misclassifying toward division is benign: one `SLASH`, a parse error in the
right place, §1.1's collect-don't-abort recovery unharmed. Misclassifying
toward regex is destructive: the scanner hunts the next unescaped `/` and
swallows arbitrary source into a single token. A design whose worst case is
"swallow the rest of the file" sits badly beside safe-by-construction, and
the fix costs one character. **Two rejected alternatives, both with real
arguments, recorded so neither is re-proposed.** An **identifier-shaped
prefix** (`r/…/`, on the `b"…"` precedent) fails outright, and the reason
is worth stating because the precedent looks exact and is not: `b"…"` is
safe because `"` cannot follow a value, so juxtaposition is unavailable and
the prefix reading is the only reading — but `/` *can* follow a value, that
being division itself. So `let a = r/2; let b = s/3;` lexes `r/2; let b =
s/` as one regex, and worse, whether it does depends on whether a `/`
appears later in the file. Non-local lexing is strictly worse than the
ambiguity it was meant to cure. **Arbitrary or multiple delimiters**
(`~#…#`, Perl-style) were considered more seriously and declined on three
grounds: RE2 has no backreferences, so "the same byte that opened it" is
not expressible as a pattern and the file's table idiom would break (a
restricted set of N delimiters would mean N static rows, which does work);
`#` specifically collides with the `x` flag's comment syntax (regex §3–4),
so the most natural alternate delimiter is the one that cannot be used; and
a canonical formatter (`luna -f`) would be forced either to normalize —
rewriting the author's pattern text and re-escaping — or to preserve,
leaving the corpus permanently heterogeneous. Reversibility settles the
rest: `~#…#` is a lex error today, so a second delimiter can be added
compatibly later and can never be removed once used. **The delimiter is `"`
and the pattern is near-raw.** Luna decodes exactly one escape, `\"`, which
is what lets a quote sit inside; every other backslash sequence — `\d`,
`\w`, `\n`, `\\`, `\p{L}` — reaches the engine verbatim. This is not a new
concept: R150's escape table already ruled the regex context "passed
through undecoded", so only the delimiter escape changed, `\/` becoming
`\"`. The trade is deliberate and favourable — `/` is common in patterns
(paths, URLs) and now needs no escaping at all, while `"` is rare and costs
`\"`. Consequences that fall out and were swept: an **empty pattern is
`~""`**, retiring the `/(?:)/` workaround that existed only because `//`
was a line comment; **`#` comments in `x`-verbose patterns can no longer
terminate a literal**; **lexical-structure §3.1's "comments never collide
with regex" invariant is moot**, comments and regex literals no longer
sharing a starting character; and regex §9's **alternate-delimiter open
question is dissolved rather than answered**, since slash-heavy patterns no
longer pay anything. A bare `~` not followed by `"` is a lex error, as a
bare `#` is. Swept: `regex.md` (§1–§5, §7 examples and prose throughout,
§9's open question), `lexer.md` (§1 mode table, §4 `REGEX` row, §5 slash
and sigil notes, §6 opener/closer, §8 ordering item 3, F2 rewritten to
history), `lexical-structure.md` (§3.1), `string.md` (§5.1's regex row),
`string-api.md` (§7 cross-reference), `index.md` (spec table),
`overview/types.md` (type table). Not swept: the tooling grammars, on
R232's precedent — their alternation is stale across many rulings and
regenerates as a batch. **Not ruled here**: whether a second delimiter is
ever admitted, which regex §9 now carries, reversibility note attached.

**R238 — the numeric literal grammar, ruled in full: octal joins hex and
binary, leading zeros become a lex error, and literal magnitude is
parsing's problem, not lexing's.** G2 was the last gap lexer.md carried
open, deferred "by choice" behind a set of working assumptions the file had
been running on unratified. Six of them are adopted as they stood; two
questions were genuinely open and are decided here. **Octal `0o` is
added**, and the honest accounting is that its only modern constituency —
Unix file modes — is a *deferred* consumer: filesystem §5 defers the
permission model whole (chmod, chown, mode bits), with `exec` running
`chmod` as the standing escape hatch. What earns octal a place now anyway
is entanglement, not need: the leading-zero question had to be answered
regardless, and its answer is only coherent once `0o` exists. **Leading
zeros are a lex error**, and specified as an explicit **error production**
(`0[0-9_]+`) rather than as a hole in the grammar — the distinction matters
because §1.1 collects lexical errors rather than aborting, so a mere hole
would silently lex `007` into three adjacent `INT` tokens and hand the
parser garbage instead of handing the author a diagnosis. The substance:
`0755` means octal to a C or Python-2 reader and 755 to a language that
permits it, which is a silent wrong value of exactly the class Luna exists
to close, and `0o755` now spells the intent unambiguously. Bare `0`, `0.5`,
and `0x0` are untouched. **Prefixes are lowercase only** — `0X`/`0B`/`0O`
are lex errors, since two spellings of a prefix buy nothing — while hex
*digits* stay either case (`0xDEADBEEF` is idiomatic) and the exponent
marker stays either case (`1E10`); where two spellings genuinely carry
idiom, the formatter canonicalizes rather than the lexer forbidding, which
is the general principle this file should follow whenever the question
recurs. **`_` separates digits and nothing else**: one underscore, strictly
between two digits, in any radix and on either side of a point. `_1` is an
identifier, `1_` and `1__0` are typos rather than intents, and `0x_FF` is
rejected too — Go permits that one, Luna does not, a digit must follow the
prefix. **No leading or trailing point**: `.5` is written `0.5` and `5.` is
written `5.0`. The trailing ban was already load-bearing and merely gets
ratified — requiring a digit on both sides is what makes `1..5` lex as `INT
RANGE INT` and `1.toDouble()` as `INT DOT IDENT` with no lookahead, which
RE2 could not supply anyway — and the leading ban is symmetry plus
legibility. **Exponents take an optional sign and plain digits**, at least
one, with separators legal in the significand and never inside the
exponent, and no hex-float form. **Literal magnitude is not lexical** (the
question the working assumptions never reached): the lexer accepts any
digit string, and a value too large for `int` is caught in **parsing**.
That this needs no type information is a consequence of the last decision
rather than an accident — because **no wider-type literal form exists**,
every integer literal is an `int` and its range is always i64. Assigning an
in-range literal to a narrower target (`let b: byte = 300;`) is a different
check and belongs to analysis, where the target type is known. Extended by
symmetry to doubles: a literal that **overflows to infinity** (`1e400`) is
a compile error, because `inf` is a keyword and the explicit spelling
exists, so a finite literal turning infinite is a wrong value rather than a
rounding; ordinary rounding, underflow included, stays undiagnosed, `1.1`
being inexact too. **R216's deferral is reaffirmed, not spent.** It rode
explicitly on "the literal grammar", which has now happened — and happened
*without* adding suffixes or context-driven forms, so the standing answer
holds unchanged: the comptime-folded constructor is the literal story
(`parseDecimal("19.99")`, `parseRational("1/3")`, `complex(3.0, -4.0)`), a
suffix would buy spelling rather than capability, and it may still be
considered later. Recording the reaffirmation matters because a deferral
whose trigger has fired and gone unmentioned is indistinguishable from an
oversight. Swept: `lexer.md` (§4 table gains `INT_OCT` and the error
production, `INT_DEC` and both `DOUBLE` rows re-anchored against leading
zeros, §4 notes gain the four-rule grammar, §8 ordering item 4, G2 marked
resolved and the gaps preamble corrected to seven of seven), `int.md` (§7
literals, §8's separator bullet), `double.md` (§8 literals),
`numeric-tower.md` (§9's R216 bullet). Not swept: the tooling grammars, on
R232's precedent. **Not ruled here**: tab handling in a column count, which
is lexer §9's remaining open item and belongs to the diagnostics spec.

**R239 — the lone `$` gets a name, `DOLLAR_TEXT`, and the mode-internal
attempt order is written down.** Compiling §10's inventory surfaced a row
in §6 carrying a pattern and no token name: a `$` that starts no
interpolation form, described only as "text (fallback, all modes)". It is
not an edge case — `$` is the regex end-of-line anchor (`~"^\d+$"m` is the
spec's own example), `"costs $5"` is an ordinary string, and `INTERP_IDENT`
is `DQ_STRING`-only so `` `echo $HOME` `` hits it too. And it cannot be
absorbed silently, because §2's spans tile the source with no gaps while
`DQ_TEXT`, `REGEX_TEXT`, and `CMD_TEXT` all **exclude** `$` from their
classes — an exclusion that is itself load-bearing, since a text run able
to swallow `$` would consume `${` before `INTERP_OPEN` saw it. The rejected
alternative was to emit the byte **as the enclosing mode's own text
token**, which adds no name and keeps the inventory at 125; it loses
because the emitted token would not match its own documented pattern,
forcing all three text classes to become `[^X\\$]+\x7c\$` with an
order-dependent branch apiece. In a file whose method is one token, one
name, one readable pattern, three smuggled alternations cost more than one
honest row. **`ESCAPE_PAIR` settles the shape**: it is already one name
with one pattern shared across `DQ_STRING`, `REGEX_BODY`, and `COMMAND`, so
a single `DOLLAR_TEXT` across the same three modes follows precedent rather
than breaking symmetry, and the three-per-mode alternative (`DQ_DOLLAR` and
friends) is what nobody wants. Ruled out first, and recorded because it
looks attractive: **requiring `\$` for a literal dollar**, deleting the
case entirely. It is fatal to regex under R237's near-raw rule — Luna
decodes only `\"`, so `~"^\d+\$"` would hand the engine `\$`, which *is* a
literal dollar to the engine, leaving the end-of-line anchor unwritable —
and it would break `"costs $5"` for nothing, string §5.1's `\$` already
being available and optional. **Swept in with it**: §6 now states the
**mode-internal attempt order** (closing delimiter, `ESCAPE_PAIR`,
`INTERP_OPEN`, `INTERP_IDENT`, `DOLLAR_TEXT`, text run), which §8 never
covered — §8 is explicitly the `DEFAULT`/`INTERP_EXPR` order — and which
`DOLLAR_TEXT` makes necessary to state, its pattern `\$` being correct only
because the two interpolation forms are attempted ahead of it. **Unaffected
by construction**: single-quoted strings and `b"…"` bytes literals do not
interpolate and are matched by one span regex with no mode (§1), so `$` is
ordinary content in `[^'\\]` there, `\$` is correctly absent from their
escape rows (string §5.1), and neither ever reaches this decision. Swept:
`lexer.md` (§6 row and new ordering paragraph, §10 inventory —
literal content 4 → 5, total 125 → **126**).

**R240 — every diagnostic carries a code: one-letter stage prefix plus four
digits, per-stage numbering, and a span model with one mandatory primary.**
testing-strategy §2 pinned diagnostic tests to "the ruled error **type** +
source location", which was unwritable for most of the compiler:
`errors.md` §2's hierarchy names *runtime* types, and the phases that
produce most diagnostics — lex, parse, semantic — named nothing at all.
lexer.md had at least twelve ruled error conditions and no vocabulary for
any of them, so the error half of the test plan could not be written
without inventing names and freezing them, the same trap G2 posed before
R238. **The scheme**: a one-letter prefix naming the stage that *defined*
the check, then four digits — `L0003`, `S0143`, `M0011` — with the prefix a
**separate numbering space**, so `L0001` and `P0001` are unrelated.
Per-stage rather than flat is chosen for a reason particular to this
project: `claude-agent-plan` implements slices in parallel, and independent
number spaces remove the central registry those agents would otherwise
contend on. MSVC's `C####`/`LNK####` split is the precedent; ten thousand
per stage is far beyond what any compiler has spent (Rust is near six
hundred after a decade). **Nine prefixes**, derived from §1's phases: `L`
lexical, `P` syntax, `S` semantic, `M` modules (§1.0 discovery and §1.2
import validation together, since a user cannot tell which noticed an
unresolved import), `C` comptime, `B` build (assemble, toolchain
invocation, the incremental cache), `F` format, `T` tooling (LSP, debugger,
test runner), `I` internal — an invariant violated, "this is a compiler
bug". **No prefix exists for lowering or emission**, and the omission is a
theorem rather than an oversight: §1.7 guarantees the emitted Go compiles,
so a failure there is not a user diagnostic but a compiler bug, and lands
in `I`. No severity axis exists either, §3's "**No warnings, ever**" having
already settled it — every code is an error. **Allocation is append-only**,
starting at `0001`; `0000` is never allocated because too much tooling
reads zero as "no error". A retired check's code is **retired, never
reused** — search results outlive the check that prompted them. Numbers
carry no meaning, with **no ranges reserved by topic**, because a range
always overflows and then lies. And **a code never changes prefix**: when a
check migrates to an earlier phase, as checks do, it keeps its original
code and is reported by whichever phase now runs it. That is deliberately
impure — the prefix records where a check was *defined*, not where it lives
— and it is what makes per-stage numbering safe, since renumbering would
collide with codes already allocated. **A code is not an error type, and
both are needed.** A code identifies a *diagnostic*: a message about a
program that will not be built, uncatchable, with no runtime existence. A
type identifies a *value*: `useAfterConsumed` (errors §2) is catchable
because the program ran and the check failed dynamically. The same
condition frequently has both, the compiler proving what it can and
deferring the rest — exactly what variables §7's "compile (runtime when
branch-dependent)" column has been recording all along — so a static
use-after-consume is `S0143` while its dynamic twin stays
`useAfterConsumed`, and a code with a runtime counterpart names it so `luna
explain` can say so. Runtime panics get no codes; their type name is
already the stable referent. **Prose splits by lifetime**: the **title** is
fixed per code and part of its identity ("Use-after-consume"), the
**description** is per-instance and volatile (naming the binding, the file,
the type). Only the title is documentable, which is what makes a page per
code possible while instance text churns — and what makes
`claude-agent-plan` §F's "diagnostics are volatile in alpha" safe rather
than merely tolerated. **Spans**: exactly **one primary span**, mandatory,
the caret site, so "the location" is never ambiguous; plus zero or more
**labeled secondary spans** ("declared here", "consumed here") carrying the
narrative. A span is `(file, byte offset, length)` — file identity
required, not optional, because a secondary span routinely lives in another
module. Byte offsets fall out of R236 at no cost: the diagnostic layer
stores offsets only and lexer §9's lazy line index resolves line and rune
column at render time. Notes and hints are prose and never load-bearing,
but a hint **may carry a structured suggestion** (a span plus replacement
text), designed in now because it is what lets `luna -l` surface code
actions later without every hint being rewritten. **testing-strategy §2 is
amended accordingly**: tests pin the **code** plus the **primary** span's
file and line, with secondary spans opt-in per test — they are the half
that churns as diagnostics improve. Runtime errors continue to be matched
by type. **Applied immediately as lexer §11**, the worked example every
later spec can copy: twelve `L00xx` codes for conditions already ruled,
from `L0001` invalid UTF-8 through `L0012` unexpected character — the
catch-all that makes the lexer **total**, since every byte now either
begins a token or raises a code, which is what lets §2's tiling invariant
hold on invalid input as well as valid. Writing it surfaced a hole R238
left: R238 declared `0X`/`0B`/`0O` lex errors but added no error
production, so `0X1F` would have split into `INT_DEC` + `IDENT` and
diagnosed as a syntax error; §0 gains the `0[XBO]` production, and the
inventory's row count moves 129 → 130 while the token count stays 126.
Swept: `compiler.md` (§3's thin codes bullet replaced by new §3.1),
`lexer.md` (§0 error production, §10 counts, new §11),
`testing-strategy.md` (§2). **Not ruled here**: codes for the error-summary
tables that predate the scheme (variables §7 and elsewhere), which acquire
them as their specs are next revisited; and `luna explain <code>`, which
the scheme makes possible but compiler §0.1's flag table does not yet carry.

---

## Still open (out of scope of these rulings)

The F-series is **closed** (audited by R124): F4 (`list` drift vs panic) was resolved
by R9/R10, F9 (union subtyping vs the interval test) by R18, and F22 (`any` pipelines)
by R34 — this list had simply gone stale while carrying them — and F6 (the `as` algebra
exceptions) is resolved by R124 itself: **lossless is the criterion**. Also closed along
the way: view interior mutability (F25, **mooted by R95** — views no longer exist) and
the builtin error types' casing (**resolved by R122** — camelCase everywhere). Still
genuinely open from that review era: *(none — the last one, R88's function-value
capability query, is **closed by R130**: `capabilitiesOf`, introspection §4.6.)*

From R89 and R90, once the largest flag in this file, now closed:

- **Variadic parameter declaration and named arguments: resolved by R108** (functions
  §3.3.1–§3.3.3). The R35 unification holds after all — the variadic is the positional
  sublist's trailing rest, post-variadic parameters are defaulted and named-only, the
  `*,` marker and the undefined `name?` form are retired/defined, and named arguments
  landed on the PHP rules with `NamedArgumentError` as the binding panic. (The R90
  audit's other half, the `preserveKeys` defaults, was discharged by R92.)

And from R91–R93, the two big flagged remainders:

- **The R91–R93 and R95–R98 sweeps are COMPLETE** (R99–R101). Every site the two
  remainder lists named is done: the catalogue split and its retirements (R99: tables,
  builder, secret, keywords, operators, spread, concurrency; R100: coalescing, toString,
  json, functions; R101: equality, stream, stream-api, range, control-flow,
  associativity, lexer, wildcard, regex, constraints §7's cross-ref), the `?->` token
  landed (R101), and the examples verified clean. The retired-spelling greps
  ("built-in protocol", "meta fn", "values-only", "asStream", "onNoGet", "canGet",
  "ViolationError", "enumerate", the old `->` call shapes) return only deliberate
  historical mentions. The one discovery that pass recorded — `strings.md`'s eleven
  `asStream` flags — is **resolved by R102** (producers produce streams), which also
  settled the string/bytes-iterability question it raised (R104: `foreach` yes for
  `bytes`, `iterable` stays `table | stream` exact). Future edits: iterable-functions
  §3's retired-spellings table is the guard against reintroduction.
- **Opens from R95–R98: all closed.** (`@@`'s type surface is **closed by R126** —
  total over `any`, `[]` off tables, protos const-declared identity-class brands; the
  JSON nesting shape is **closed by R125** — the `"@@"` section, off
  by default; removal/`unapply` is **closed by R123** — the free function, refusal where
  an applied requirer still requires,
  wrong-write checks for the rest, the §6.3 condition repaid as stated; the `?->` token landed in R101;
  the initializer-grammar spelling closed with R108; and **mutable protocol-level state
  closed with R119** — the task-ownership story exists now, the owner-task pattern of
  channels §5, so "statics stay inexpressible" is an answer with a referent, not a
  deferral.)
- **Opens from R102–R105: all resolved.** The conversion-family unification by R106
  (three prefixes, three contracts; the policy verbs; `parseInt` / `parseDouble`); the
  `toBytes`-over-iterable repack by R107 (the union arm, constraint-panic-checked per
  element and therefore `!`-free; `parseBytes` rejected by R106's own table).
- **Opens from R111: both resolved by R113** — the trace's gate is the dedicated
  `revealStackTrace` capability (the `reveal*` convention), and `toJson` renders secrets
  as `'<secret>'` with explicit, call-site-delegated revealed serialization
  (`revealSecrets` + `use` on the call, R112). No `skipSecrets` flag exists; the
  placeholder is the graceful default.

One small deferral tracked here (reclassified from "open" with the spread.md validation,
R168): spread of `bytes` / `string` — whether `[...someBytes]` yields `byte` elements or
errors waits on a use case, not a decision, and `toList(b)` (R167) already spells the
explicit form (spread §7).

Two R233 deferrals sit outside the corpus and are tracked here so they are not lost, both
pre-ship rather than pre-freeze: the **contents of the third-party notice artifact** that
must accompany the `luna` distribution, and **trademark clearance** — the Go marks may be
retained on a substantially *unmodified* redistribution, and R233's three exclusions make
this bundle modified, so it is a `trademark@go` question to settle before any binary is
published, not one the spec can answer.

R234's two deferrals, likewise tracked: **when** the bootstrap happens (the ruling fixes the
order — after the oracle, after the surface freezes, after std grows — not a date), and
**whether Phase 1's Go-written compiler is kept alongside the interpreter** as a second
independent reference. The second is a real choice with a real cost: it is the faster and more
complete of the two Go artifacts and the more expensive to maintain, and R234 commits only to
keeping the interpreter.

*(The `@P` value-position contradiction R174's validation surfaced here is **ruled:
R175** — the static-protohood steer extends to value position, `@P` on a proto yields the
induced refinement everywhere, the alias idiom legal by construction.)*

*(The two-headed json spec R179 flagged here is **resolved: R180** — merged into
`std/json.md`, the orphan retired as `retired/json-duplicate.md`, one index row, no
cite renumbered.)*

*(R182's two match.md flags are **ruled: R183** — signed numeric literal patterns, the
parser-level fold, LL(1)-clean because patterns admit no operators — and **R184** —
match is inline, no capture, no `use` clause, the enclosing frame's grant; §11
rewritten.)*

*(R208's try-spanning-yield item is **ruled: R210** — the parse restriction, on the
abandonment argument: a spanning catch never had power, only grouping; the handler-range
table recorded as the road not taken.)*
