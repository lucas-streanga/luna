# Function Representation

How Luna stores function values at runtime. The fourth internals sibling
(value-representation, string-representation, table-representation): it describes the payload
an `fn`-typed `lval`'s `ptr` points at (R204). The semantics are owned elsewhere — the value
model and captures by the functions spec (§2), calling and named arguments by functions §3.3
(R108), the capability check by capabilities §3.1, the introspection surface by introspection
§4.5–§4.6 (R130), signature identity by functions §3.1–§3.2 and value-representation §4.2
(R131). This file records how the hosted runtime realizes them, and why.

---

## 1. The shape: one pointer, two halves

A function value is **one pointer** to a **closure block**:

```
fnValue:      *closureBlock
closureBlock: { desc *fnDescriptor, captures... }     // one allocation
```

This is Go's own closure representation (`funcval`: a pointer to a code-plus-captures block),
adopted deliberately: one word boxes into `lval.ptr` trivially, `==` on functions is the
ruled identity compare (equality §2) as a pointer compare, and R203's type-directed rule
applies with nothing fn-specific — a statically-typed `fn` binding passes the raw pointer, an
`fn` in an `any` slot boxes.

The deep structure is the **per-literal / per-value split** — the shared-static-descriptor
pattern's third use (protocol descriptors and const-table metadata being the others,
table-representation §4, §5): everything that is a fact of the *literal* is identical for
every closure minted from it and lives once, in an emitted-const descriptor; a value carries
only *which literal* (`desc`) and *this instance's captures*.

### 1.1 Capture-free literals: a static block, and per-literal identity (R205)

A literal with an **empty capture set** (`const somefn = fn () => {};`) has nothing
per-value at all, so its closure block is **emitted as const data** — one static block in
read-only memory, every use of the value the same pointer. **Zero allocations, ever**: not
at creation, not at passing, not at boxing (the pointer enters `lval.ptr` as any fn does).
This is Go's own representation for capture-free closures (a static `funcval`), and the
block joins the immortal class (no lifetime, no sharing questions — fn values never had
counts to begin with).

The observable consequence is **ruled, not accidental**: identity canonicalizes **per
literal**. N evaluations of one capture-free literal yield N pointer-equal — hence
`==`-equal — values, where a *capturing* literal mints a distinct identity per evaluation
(each evaluation snapshots, §3). Nothing in the equality spec promises distinct identities
per evaluation; Go behaves identically; and the clinching argument is **phase invariance**
(compiler §6): a comptime-folded capture-free literal and its runtime evaluation are
trivially the same value under canonicalization, and awkwardly different values otherwise.
The ruling in one line: **a capture-free literal denotes one value; its evaluations are
identical.**

## 2. The descriptor: per literal, emitted const

```
fnDescriptor:
  nativeEntry                  // the typed Go function (§4)
  dynEntry                     // the lval-ABI trampoline (§4)
  typeid                       // the interned signature-plus-errorability (R131)
  requirementMask              // capability bitset (below)
  names       []string         // the name of parameter i at index i (R206)
  defaults    []value          // emitted-const default values, trailing-aligned (R206)
  flags       uint32           // two live bits: generator, hasVariadic (R206)
```

The field representations are ruled, each by a lesson this file's siblings taught (R206):

- **`names` is a positional slice, never a Go map**, for two independent reasons: parameter
  counts are tiny, and table-representation §1.5 *just ruled* that ≤8-entry name lookup is a
  scan, not a hash — a map for three parameters would be the small-table mistake imported
  into our own runtime (named-argument binding scans the slice; short names are inline-string
  word-compares) — and, disqualifying on its own, **Go maps cannot be emitted as static
  data** (they are runtime-initialized), which would break descriptor-as-RODATA outright.
  The position mapping needs no storage at all: the name of parameter `i` *is* `names[i]`.
- **`defaults` holds values, not code**, because defaults are ruled **comptime-known
  constants** (functions §3.3.1, R206) — an emitted-const array, trailing-aligned with the
  parameter list. Fn-valued defaults cost nothing special: a named function's default is a
  pointer to its static block (§1.1).
- **The derived fields are not stored** — derive-don't-cache (table-representation §1.5),
  five times over: `paramCount` = `len(names)`; `minArity` = `len(names) − len(defaults)`;
  **comptime eligibility ⇔ `requirementMask & NON_COMPTIME_CAPS == 0`** — the R43 theorem
  doing representation work (ineligibility sources are *exactly* capabilities, and `use`
  propagates transitively by declaration, capabilities §5/R33, so the mask already carries
  transitive effect-freedom), refined by R213: **comptime capabilities exist** (functions
  §5.5, capabilities §7.1) and occupy mask bits, so the test masks them out first —
  `NON_COMPTIME_CAPS` is a link-time constant, and the read stays one masked compare (an
  earlier form said `mask == 0`, forgetting the comptime-capability bits); errorability is
  in the `typeid`; post-variadic named-only-ness follows from `hasVariadic` plus the
  trailing-rest rule (R108).
- **`flags` has exactly two live bits**: `generator` (the evaluator and emitter need
  "calling this constructs a generator machine") and `hasVariadic` (load-bearing for §4's
  binder). Thirty bits honestly reserved.

- **The capability requirement set is a bitmask, not a list**: capabilities are a closed
  compile-time universe (each declaration an index), and the ruled dynamic check is already
  phrased as "requirement set ⊆ the executing frame's granted set, **one bitmask compare**"
  (match §2.3). What is deliberately *not* stored: **grants** — capability tokens erase to
  nothing at runtime (compiler §7.5); the value carries requirements only, and the grant
  side of the compare is the executing frame's.
- **The descriptor is introspection's backing store**: `params(f)`, `paramTypes(f)`,
  `capabilitiesOf(f)` (R130) read exactly these fields — the introspection surface and the
  call machinery share one source of truth by construction.
- **Comptime eligibility needs no runtime read**: comptime never runs at runtime, so the
  flag serves the IR evaluator (compiler §6) and no emitted path ever consults it.

## 3. The environment: const-snapshot captures

The captures in the closure block are the functions-spec model realized (§2.1): implicit
deep-`const` snapshots taken at creation, read-only thereafter. Representation follows R203
inside the block too — a captured `int` is an `int64` field, not an `lval` — and the block's
allocation is Go's business: the emitter creates it and **the Go backend's escape analysis**
decides its placement, the standing no-Luna-escape-analysis division (compiler §1.4.1). The
one deliberate exception is unchanged from value-representation §1: an escaping `use`-capture
of a mutable scalar boxes the scalar so binding and closure share one cell. Captured
capability references erase (zero-data, §2), so authority in a closure costs no bytes — the
requirement mask is the whole runtime record.

## 4. The two ABIs: native entry and dynamic trampoline

Each literal is emitted twice — the R203 two-tier economics applied to calls:

- **The native entry** is an ordinary Go function with a **typed Go signature** — unboxed
  parameters and result, a Go closure where captures exist. Every statically-resolved Luna
  call compiles to a plain Go call of this entry: the hot path is indistinguishable from
  handwritten Go, which is the devirtualization the compiler spec's optimization passes
  lean on.
- **The dynamic entry** is a generated **trampoline** with one uniform ABI
  (`func(block, argv, named) lval`-shaped), used only where a call goes through an
  `fn`-typed slot. In order, it **binds** — the first
  `paramCount` argv entries positionally, defaults filling the deficit tail (an unfillable
  deficit is the `arityError`, R108), named arguments resolved through `paramNames`
  (`namedArgumentError` on unknown or double-bound, R108) — then unboxes to the native
  parameters, calls the native entry, and boxes the result.

**The capability check sits at the call site, not in the trampoline, and its frame operand
is a compile-time constant** (R205). A frame's granted set is **lexical**: its function's
own `use` clause, plus any R112 call-site delegation — both static — and no runtime grant
state exists anywhere (no capability stack, no frame walking; tokens erase, compiler §7.5).
So the emitter knows every dynamic call site's grant as an **immediate**, and the entire
runtime capability system compiles to, at fn-slot call sites only:

```go
if desc.requirementMask &^ SITE_GRANT_CONST != 0 { panic(...) }   // one load, one and-not, one branch
```

Direct calls pay **nothing**: a by-name call to a `use`-requiring function from an
ungranted frame is a *compile error* (capabilities §5's static propagation), so the check
exists only where the callee is statically unknown. The audit backbone of the language has
a runtime footprint of one masked compare on the rare path — free precisely because grants
were designed lexical (no `implicit`, R33; no dynamic capability creation), which is what
lets the constant fold.

**Surplus arguments are dropped by never being consulted** — the ruled drop (functions
§3.3, spread §4) needs no mechanism: binding is callee-driven, the binder reads
`len(names)` entries, and later argv entries simply never bind. **Unless `hasVariadic`**
(R206): a variadic callee's surplus positional arguments are not surplus at all — the
binder branches on the flag and collects them into the trailing rest list (functions
§3.3.3), which is exactly why that flag is stored rather than derived. In static calls the
same rules apply at compile time. In both paths, surplus argument *expressions* are
evaluated as any argument is (they are expressions, evaluated in call order) — dropping is
a binding fact, never an evaluation fact, so effects cannot vanish based on callee arity.

## 5. Signature tests and identity

`@f` reads the descriptor's `typeid` — the interned signature — and `f is fn (A): R`
dispatches per value-representation §4.2: the `fn`/`fn!` ladder by interval, signature
leaves by the link-time pairwise table (R131).

**The optimistic-`as` claim is representable today, for free** (R213, closing functions
§3.2's loop): after `f as fn (int): int`, the **claimed** signature rides the value's
`lval` typeid — re-typing the lval is what `as` *is* — while the **real** signature stays
in the descriptor. The two-sided per-call checks then read their natural sources: the
contravariant argument checks and the binder consult the **descriptor** (the real
parameters, §4), and the covariant result check consults the **lval's claimed** typeid.
No new state, no claim table: the claim travels wherever the value does, exactly as `as`
semantics require. `==` is identity: one pointer compare on the
closure block (equality §2 — extensional function equality is undecidable, so identity is
the only sound answer, already ruled). Two *capturing* closures from one literal are unequal
(distinct blocks, one per evaluation); one closure aliased twice is equal to itself; and a
**capture-free** literal's evaluations are all equal — one static block, per-literal
identity (§1.1, R205).
