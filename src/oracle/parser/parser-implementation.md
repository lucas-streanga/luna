# Parser implementation

The decisions taken before the parser was written, and why. compiler §1.3 and tooling §2 fix
**what** it must produce; this file fixes **how**. Nothing here is a language decision, so nothing
here is a `CHANGES.md` ruling — it sits beside its code as `testdata/golden.md` does.

Fixed elsewhere, and assumed throughout:

- **A lossless CST** from which the exact source bytes are reconstructable, retaining all trivia
  and original spelling; the AST the compiler analyses is a **view** over it, and the CST is the
  shared root (tooling §2). A "now, not later" decision there: a lossy parser cannot be made
  lossless without being rewritten.
- **Error tolerance is a property of the pass, aborting is a property of the driver** (tooling
  §3): the parser always produces a best-effort partial tree, the batch driver discards it at the
  phase boundary, the LSP driver consumes it.
- **The grammar is grammar.md §0**, and the diagnostics are its §11 — an engine of five plus six
  named rules, titles fixed, numbers allocated as checks land (R267).

---

## 1. The tree is kind-tagged, not a struct per construct

One node type carrying a `Kind` tag, rather than 130 typed structs with an interface over them.

**The decisive argument is this repository's own.** The node kinds already exist and are already
pinned: grammar.md §0's nonterminals plus lexer §0's token kinds. A `Kind` enum can be asserted
against §0 **both ways** — every surviving nonterminal has a kind, every kind has a nonterminal —
exactly as `token.Kind` is asserted against lexer §0 and `TestNonterminalCount` already asserts
§0's own arithmetic. A hand-written struct per production cannot be pinned that way: nothing
would catch a production added to §0 with no struct behind it, which is the R232 defect class
one level up.

Three supporting reasons:

- **Error tolerance destroys the guarantee typed structs are bought for.** Their selling point is
  that `fnLit.Body` is a real body. But tooling §3 requires a tree for `const f = fn () =>` with
  nothing after it, so `Body` must be optional — and then so must every field everywhere. The
  types are paid for and the guarantee is exactly the one error tolerance forbids.
- **Everything generic would have to know all 130 types.** The formatter, the LSP's folding,
  selection ranges and node-under-cursor, and this package's own golden renderer all walk the
  tree without caring what is in it.
- **Go has no sum types.** The typed encoding is an interface plus a type switch, with no
  exhaustiveness check — so the safety is nominal. And the cautionary example is the backend
  itself: `go/ast` is typed structs, and comment handling is its most-complained-about corner,
  `go/printer` reattaching comments by position heuristics.

**The cost, accepted:** no static shape. Mitigated by typed accessors over the untyped tree —
written **where the compiler needs them**, not upfront for all 130.

## 2. Trivia are nodes in the tree, not attachments on tokens

Whitespace, comments and the shebang are ordinary children, not leading/trailing baggage carried
by the token beside them.

- **It makes losslessness structural rather than a discipline.** R236 already made trivia emitted
  tokens whose spans tile the source with no gaps; keeping them as nodes carries that property
  into the tree, so "concatenate the leaves and you get the file back" is a one-line invariant and
  the same check the `.lex` goldens already run. Attachment would make losslessness a rule every
  node type has to remember.
- **It defers the attachment question rather than answering it early.** Which node owns a comment
  becomes a *query over the tree* instead of a fact frozen at parse time, so the formatter can
  change its mind without a reparse, and the rules can be written when the formatting rules are
  (tooling §2 defers those deliberately).

**Rejected: Roslyn's leading/trailing attachment.** It answers ownership at parse time, which is
useful when you have already decided the answer. We have not, and this design does not require us
to.

### 2.1 Placement: the builder's rule, and why that closes the question

§4 does most of the work. The parser runs on the **trivia-filtered** stream, because that is what
grammar.md §0 is defined over, so the parser never sees trivia at all; the builder re-inserts it
by walking the full stream. What is left is where trivia goes when an `open` or a `close` falls in
the same gap — given `foo(); // c` then `bar();`, the comment could land before the `close`,
between the two statements, or after the next `open`. One rule settles it:

> **`close` happens before pending trivia is flushed, and `open` is deferred until it has been.**

Both directions push trivia outward, into the innermost node that was *already* open when it
occurred. That yields an invariant worth asserting: **trivia is never the first or last child of
any node except `File`**.

The invariant is the point, not a side effect. If trivia could be a node's first child, that
node's span would start at the preceding comment, and getting a tight span back would need
Roslyn's `Span`/`FullSpan` split — two span accessors on every node, forever. Pushing outward
keeps inner spans tight and leaves the gap covered by the enclosing node, which covers it anyway.

So **attachment as a stored fact is settled**. Attachment as a *semantic* question — which
declaration a comment documents — stays a query over contents and neighbours, which is what
making trivia nodes bought.

### 2.2 The splice pass: where §2.1's rule actually lives

Three stages, not two: **parser → splice → builder**. The parser emits events over the filtered
stream; a small pass fills trivia in; the builder is then uniform, turning open/token/close into a
tree with no idea that trivia is special.

**Token events carry the full-stream index**, not a filtered one. They can: the filtered view is
index-parallel with the full token slice, so each consumed token already knows its real index and
no mapping table is needed.

The splice pass keeps `next`, the first unemitted full index:

| Event | Action |
|-|-|
| `token(i)` | emit `token` for every index in `next..i-1` — all trivia by construction — then `token(i)`; set `next = i+1` |
| `open(k)` | **if the stack is non-empty**, emit trivia from `next` while it stays trivia; then `open(k)` |
| `close` | emit `close`. **No flush** — this is the half of §2.1 that pushes trivia outward |
| `missing(k)` | emit it. **No flush**, for the same reason and one of its own: it covers no bytes, so it belongs before the trivia it precedes rather than after |
| end | emit the remaining trivia, then close the root |

The last row is the root's `close`, which is where a file ends in a balanced stream: the parser
opens and closes `File` itself, §6.1 turning on its doing so.

Trivia needs **no new event kind**: the pass emits ordinary `token` events for trivia indices, and
the builder makes them leaves like anything else. **A synthesised leaf does need one**, and it is
the only other member: §7.2's layer 1 emits a zero-width leaf of the kind it expected (§6.1), and
that cannot be a `token` event because there is no entry in the lexer's stream for it to index. It
carries the kind alone — its position is the builder's cursor, the end of the last leaf emitted,
which is the one offset that cannot break the tiling. Trivia pending at that moment has not been
flushed, so the zero-width leaf lands before those bytes rather than after them.

The stack-non-empty guard is what keeps leading file trivia inside `File`. It is not flushed
before `File` opens, because nothing is open yet; it flushes before the first top-level item's
`open` and lands as `File`'s first child — the one place §2.1's invariant permits it.

**Rejected: the parser emits trivia events.** It would have to *see* trivia, contradicting the
basis of §2.1 — that the parser is the filtered stream, which is what makes it exactly
grammar.md §0. Auto-emitting at `bump()` inside the event emitter does not save it: that fires
*after* the `open`, so trivia lands as a first child, which is precisely what the invariant
forbids.

**Rejected: the builder does it inline.** The decision logic is identical — flush before `token`,
flush before `open`, never before `close` — and it is one fewer concept. What decides it is where
the test seam goes: as a pass, §2.1's rule is the entire content of one function whose test is
**events in, events out**, the flattest comparison available and exactly what §4.2 committed to.
Inside the builder it becomes three conditions to reconstruct from reading, tested only through
tree shape.

### 2.3 How the rule is pinned

Neither golden format can see this rule. The tree section drops trivia (`golden.md` §1, and it
should keep doing so — an indented tree stays readable against §0), and no *event* the parser
emits mentions trivia at all. So §2.1 is asserted directly, in the two places the two halves live:

- **Index coverage, on the spliced events.** After splicing, the multiset of token indices is
  exactly `{0..n-1}`, each once, in order. One line, no tree required, and it *is* the
  reconstruction invariant — available a stage earlier than the tree and far easier to diagnose
  when it fails.
- **The structural invariant, on the tree**: trivia is never the first or last child of any node
  except `File`. This is the half index coverage cannot see, because a comment placed in the
  wrong node still preserves order and still reconstructs.

Both run over every golden and over the whole corpus, not over a chosen sample.

## 3. Positions are absolute offsets

Every node stores its own `[offset, end)` in bytes.

- Span queries are O(1) and context-free: a node can say where it is without a path from the
  root, which is what diagnostics, the golden renderer, and every LSP request want.
- The code is markedly simpler, and nothing in it is subtle.

**Rejected: widths plus a cursor — the green/red split (Roslyn, rowan).** Storing a *width*
instead of a position makes a subtree position-independent, which is what allows identical
subtrees to be shared and an edit to reuse most of the previous tree; the "red" layer is a cursor
created on demand that adds parent pointers and absolute offsets as it descends. Roslyn and
rust-analyzer need this because their files are large and the IDE reparses per keystroke.

**Nothing in Luna's specs asks for sub-file incrementality.** The build cache is module-granular
(incremental-compilation-build-cache.md), and a full reparse of one Luna file is microseconds.

**What that forecloses, stated plainly:** cheap edit-time reuse. Retrofitting means touching every
node construction — contained in one package, but real work. The asymmetry that makes this
acceptable is tooling §2's own: *losslessness* is the property that cannot be retrofitted, and it
is being paid for now. Incrementality is not in that category, and buying an option nobody has
asked to exercise is how a frontend gets complicated.

### 3.1 Storage is an arena, not linked nodes

Nodes live in one slice and refer to each other by index. A `NodeID` is an integer.

- **Stable, cheap keys.** §8's typed AST is this tree plus side tables of resolved symbols and
  types, keyed by node. An integer key makes those a slice or a compact map; a pointer key makes
  them a `map[*Node]T`, which is heavier and cannot be written to disk.
- **Trivial serialization**, which the module-granular build cache
  (incremental-compilation-build-cache.md) may well want: an arena is already a flat buffer, and
  a tree of pointers is not.
- **Locality, and no GC pressure** from a few million individually allocated nodes — the ordinary
  reason compilers reach for this.

**The internal indexing is deliberately not fixed here.** Nodes plus a child-index array, or
pre-order storage where a subtree is a contiguous range and each node carries its subtree size,
are both arenas; the choice belongs at the keyboard, not in this file.

**The cost, accepted:** a `NodeID` means nothing without its `*Tree`, so the two travel together.
That is the API's problem to hide — a cursor carrying both reads exactly like a node pointer —
and hiding it is cheap, which is why the cost is acceptable rather than merely tolerated.

**IDs are stable within one parse, not across parses.** §3 reparses whole files, so there is no
edit-to-edit identity to preserve and side tables are per-tree. Anything wanting cross-parse
identity — an LSP holding a symbol across a keystroke — needs its own stable name, not a
`NodeID`.

## 4. The parser emits events; a builder constructs the tree

The parser produces a flat stream — `open(Kind)`, `token`, `close` — and a separate builder turns
it into a tree. (The shape rust-analyzer uses.)

- **The parser reads like §0.** A function per nonterminal that opens, consumes, and closes, with
  no tree-building interleaved.
- **Recovery becomes a stream operation.** "Close everything down to a synchronisation point" is
  something you can do to a stack of open events; doing it to a half-built tree is surgery.
- **It decouples the representation.** §3 could be revisited later without the parser changing at
  all, which is most of what made §3's rejection affordable.
- **It is the better test surface**, which is why it was ruled. An event stream is flat, ordered
  and directly comparable — the builder can be tested against hand-written event sequences with no
  parser, and the parser against expected events with no tree, so a failure names which of the two
  is wrong instead of leaving one diff over a nested structure.

### 4.1 The events are internal

The stream is **not** exported. It exists so the parser can be written and tested as a separate
thing from the builder, and for nothing else.

**No consumer wants it.** Every candidate turns out to want something else: the formatter and the
LSP want the CST, which is already full-fidelity (tooling §2); syntax highlighting runs off the
**lexer** and does today (`internal/highlight`); incremental reparse does not exist (§3);
serialization belongs to the arena (§3.1); lowering starts from the AST (§1.5).

**And exporting it would freeze decisions §10 deliberately leaves open.** Recovery *is* an
event-stream behaviour by §7.2's own design — "close everything down to a synchronisation point"
is a statement about a stack of open events. The quiet period's *N* and the mismatched-bracket
heuristic both change which events are emitted, so a supported stream would make tuning recovery a
breaking change, which is exactly backwards for the two things we said we would tune against
goldens once they exist. A public surface here does not cost nothing; it costs option value on a
decision still in flight.

### 4.2 Events are tested by unit tests, not by goldens

The seam §4 wanted is a **unit-test** seam, and internal is enough for it: hand-written event
sequences drive the builder with no parser, and expected event sequences check the parser with no
tree, both in-package. A failure names which of the two is wrong without either being exported.

**There is no event golden corpus**, and the reason is that events cannot see the half of the
system most in need of pinning:

- **Trivia placement is builder-only and invisible to events.** The parser runs on the
  trivia-filtered stream — that is what makes it exactly grammar.md §0 — so *no event mentions
  trivia at all*. §2.1's whole rule, the subtlest in this document and the one the formatter
  depends on, is executed by the builder while walking the full stream. An event golden could
  not observe it.
- **Node spans are the builder's arithmetic.** Events carry token spans; a node's span is its
  children's extent. An off-by-one there is exactly what a golden is for, and would not appear in
  an event dump.
- **§6.1's zero-width leaves and the no-empty-interior-nodes rule** are builder behaviour too.

There is a real argument the other way, and it loses on a detail worth recording. A flat stream
diffs better: adding a wrapper node re-indents a whole subtree in the tree format but is two lines
in an event stream. Against that — golden.md §3's discipline is that **the diff is read against
grammar.md §0**, which an indented tree supports and a flat open/close stream does not; and the
reindentation only happens when a wrapper is added or removed, which means a grammar change, which
is exactly when you want to look hard. The noise is aligned with the significance.

**A debug dump exists, and is not API**: a `String()` over the event slice, used in test-failure
output and available behind a flag. Readable when you are diagnosing, with no compatibility
promise attached.

One door left open on purpose: **`error_producing/`**. Recovery is an event behaviour, and when
such a case goes wrong the raw open/close sequence may explain it where the flattened tree does
not. Adding an events section to that directory's cases is a contained change, to be made from
real cases rather than guessed at now.

### 4.3 The tree is immutable once built

There is no mutation API. The builder produces a tree and then nothing changes it.

- **compiler §2 parses modules in parallel** and the LSP reads concurrently. An immutable tree
  needs no synchronisation and can be shared by reference across as many readers as there are.
- Annotations do not need mutability: §8's typed AST puts resolved symbols and types in **side
  tables** keyed by `NodeID`, so the analysing phases add information without touching the tree.
- It is cheap to assert now and expensive to reclaim later. One caller mutating in place makes
  the invariant everyone else was relying on unavailable, and nothing about that failure is local.

### 4.4 What `Parse` consumes

**Tokens, and a `*source.File`. It does not lex.**

The driver owns the phase pipeline (compiler §1), and a pass that lexes is a pass that has
absorbed another phase — the batch driver could no longer accumulate `L` codes and abort at the
lex boundary, and the LSP driver could no longer reuse a token stream it already has. Passes are
reusable (tooling §1) precisely because they do one phase each.

The `*source.File` is still required, and not for lexing: grammar.md has **spelling-matched
terminals** — `IDENT("from")`, `IDENT("get")`, `IDENT("type")`, `IDENT("identityEquality")` — so
the parser compares lexeme text and needs `Slice`.

```go
func Parse(f *source.File, tokens []token.Token) (*Tree, []diagnostic.Diagnostic)
```

`tokens` is the **full** stream, trivia included. The parser walks the filtered view of it; §2.2's
splice pass needs the rest.

## 5. The `Kind` enum: what never survives into a tree is not a kind

**What never survives is exactly the pure alternations** — a nonterminal whose every production is
a single symbol the desugar did not mint, so it always passes through with one child and always
collapses (`testdata/golden.md` §2; the set is computed by `ebnf.PureAlternations`, not listed, so
a new operator class joins by being written into §0). There are **25** of §0's 130:

```
ArmBody AssignOp BindTarget BindingKw BlockItem BytesLit CmdPiece CoalesceOp
CompOp CompoundStmt DeclLit DqPiece Expr FnBody IntLit Keyword Literal MatchKw
PathSegment Pattern RegexPiece TopLevelItem Type VariantPayload WordOp
```

**`Prelude` was the twenty-sixth until a tree was built from the goldens.** `Prelude ::=
PreludeItem*` has one symbol on its right, but the *desugar* put it there: `*` becomes an `LHS·n`
helper, so that one symbol stands for any number of children and a three-import file yields three
of them. The corpus had shown it all along — `import-forms.parse` prints a `Prelude` line over
three `PreludeItem`s — while this section said the name never reaches a tree, and nothing caught
the disagreement because the golden renderer printed derivation names and needed no `Kind` behind
them. Building the tree needed one and the pin refused to supply it, which is Phase 1's version
of the R253 pattern: the reading was wrong and only the writing could say so.
`ebnf.PureAlternations` now excludes a production whose single symbol is synthetic, which is the
property its callers actually read the set for — not "one symbol" but "one child".

**`Type` is the one that stays anyway.** It is a pure alternation by shape (`FnType | UnionType`)
and the one place that is wrong: eliding it leaves a bare `IDENT` indistinguishable from an
expression's, which is the distinction R256 exists to make. So **24 drop, and 106 node kinds come
from §0**.

**The precedence tiers stay, and this corrects the ruling as it was put.** They collapse only when
they pass through with a *single* child; they survive whenever they fire — `a + b` is an
`Additive`, `x is T` a `Comparison`, `a ? b : c` a `Conditional`, and the goldens show all three.
Of the 23 tiers only `Expr` and `Pattern` never survive, and both are pure alternations, so the
one rule already catches them. A tree that could not name `Additive` could not represent `a + b`.

**One kind space, not two.** Leaves carry token kinds and interior nodes carry node kinds, and a
homogeneous tree wants one tag: `Kind`'s low range mirrors `token.Kind`'s 134 values so that
converting a token's kind is a no-op, and its high range holds the 106. It should be `uint16`
rather than `uint8` — 240 allocated leaves 15 spare, which is not room to work in.

**There is exactly one parser-only kind, `Error`** — §6 has the reasoning, and the short version
is that a missing token reuses the kind it was missing.

**The pin.** The enum is asserted against §0 in both directions, the same shape as the lexer's
inventory pin: every surviving nonterminal has a kind, every node kind has a surviving
nonterminal. That assertion is the whole reason §1 chose a tag over a type.

**The constants are hand-written, not generated**, and the pin is what makes that safe. It
follows `token.Kind`'s own precedent — a written table checked against the spec — and it keeps
this repository's rule that **the diff is the review surface**: 105 generated constants are a
file nobody reads, where 105 written ones are a file somebody reviewed once against §0. The
typing is the cost and the pin removes the risk, which is the trade the lexer already made.

One alignment the pin must also cover: §5's single kind space only works if `Kind`'s low range
holds the *same numeric values* as `token.Kind`, so that `Kind(tk.Kind)` is a conversion and not
a translation. A test asserts that for every `token.All()`, not merely that both enums have 134
entries.

---

## 6. How a damaged tree is represented

There are **two** failures, not one, and they differ on whether bytes exist:

- **Absence** — a required token is not there. `const f = fn () =>` with nothing after it. There
  is nothing to cover, so a zero-width node is the only honest span.
- **Excess** — the input carries tokens the grammar cannot place. `let x = 1 @@@ ;`. Those bytes
  exist, and if they are not leaves somewhere then reconstruction fails and §2's losslessness is
  gone. So this node has **positive** width and wraps the skipped tokens.

Both are needed. Zero-width alone cannot represent garbage; error nodes alone cannot represent a
missing `;`.

### 6.1 Absence: the expected kind, at zero width

**A missing token is a leaf of the kind that was expected, with width zero** — not a distinct
`Missing` kind. A missing terminator *is* a `SEMICOLON` leaf of width 0, so an accessor looking
for a `SEMICOLON` child finds one and the tree keeps the shape it would have had. That is what
leaves `Error` as the only new kind in §5.

Width alone distinguishes a synthesised leaf from a real one, on one condition, which is therefore
a rule: **the builder emits no empty interior nodes** — an `open` immediately followed by its
`close` produces nothing. This costs nothing, because it is the same rule the golden renderer
already applies to absent optionals; but it has to be stated, or a zero-width `Prelude` becomes
indistinguishable from a missing token.

Where the absence is a *construct* rather than a token — `let x = ;`, where §11.2's "expected a
construct" fires and no single terminal was wanted — the marker is a zero-width `Error` at the
insertion point.

**A node holding only that marker survives, at zero width**, and the rule above is why rather
than an exception to it. The deletion is of nodes with *no* children, and a node whose one child
was synthesised has one: `let x = ;` yields an `Initializer` of width zero over a zero-width
`Error`, between the `=` and the `;`. Nothing is ambiguous by then — the ambiguity the rule
guards against is between a synthesised leaf and an empty node, and an empty node is already
gone — and what is left is a distinction worth keeping: a node the parser *reached* and found
nothing to put in is not a node it never reached. §6.2's width still says everything a consumer
acts on.

**The rule has no exception, and the root is where that shows.** A file of zero bytes lexes to
no tokens, so `File` opens and closes with nothing between it; the rule deletes it and `Parse`
returns **no tree**. That is the honest answer rather than a corner to be carved out: a tree
exists so the source can be reconstructed from it, and no tree reconstructs the empty string
exactly. Nothing goes with it — the file name belongs to the `*source.File` the caller is
already holding.

The condition is an *iff*, which is what makes it safe to rely on. Tokens tile the source (R236,
R242), so a file with any bytes in it has at least one token, and §2.3's index coverage puts
every token in the tree as a leaf: **`File` is empty exactly when the file is.** A file of only
whitespace or comments still parses — its trivia are `File`'s children, by the end row of
§2.2's table — so the case this deletes is precisely the empty one, and `nil` from `Parse` has
one meaning rather than two.

### 6.2 `Error`'s width is the classification

One kind, two shapes, and the shape is the distinction anything downstream acts on:

| Width | Means |
|-|-|
| `0` | something should be here |
| `n` | this should not be here |

That is enough for the compiler to treat "a subtree missing a piece" differently from "a subtree
with tokens nobody could place", which is the only classification a consumer has asked for.

### 6.3 One `Error` kind, not one per diagnostic

Splitting `Error` into the five engine diagnostics, or the six named rules, of grammar.md §11.2
was considered and rejected.

**It would put a second, unpinned copy of §11.2 in the `Kind` enum.** The diagnostic already
carries the code, the title, the primary span and the secondaries; encoding the class in the kind
as well gives two sources of truth for one taxonomy with nothing holding them together — the R232
defect class exactly. And it is worse than an ordinary duplication here, because the two halves
would be pinned against *different* tables: §1 chose a tag over a type so that `Kind` could be
asserted against grammar.md §0 both ways, and error kinds are already the single exception. One
exception is a footnote. Eleven of them is a second vocabulary inside the pinned one, checkable
only against §11.2 — a table R267 deliberately left provisional, with numbers allocated as the
parser raises them. Baking in a taxonomy that was ruled not to be frozen freezes it.

**The six named rules would produce no distinct shape anyway.** `let x = 5 if (c);` does not
derive, because a `Declaration` has no `Modifier` slot — only `Statement` does — so the parser
meets it as "expected `;`, found `if`". The named diagnostic comes from *recognising the
situation* and choosing a better message; the tree is a `BindingDecl` with a zero-width
`SEMICOLON` either way. The six differ in message selection, which is parser behaviour, not in
representation.

**And the redundancy would show up in the artifact where it matters.** A golden's third section
already lists the diagnostics, so naming the class in the tree section too states the taxonomy
twice in one file with nothing checking that the two agree.

Prior art points the same way: rust-analyzer has a single `ERROR`; Roslyn keeps the taxonomy
wholly in the diagnostic; tree-sitter's `ERROR`/`MISSING` split is **excess versus absence** —
§6.2's axis — and not a classification of causes.

**If a consumer ever needs the class from the tree**, the answer is a *field* holding the
diagnostic code, not a kind: the same value the diagnostic carries rather than a parallel encoding
of it, addable without touching any existing kind. Not now — correlating by span is free and
nothing is asking.

### 6.4 Costs accepted

- **Node-under-cursor is ambiguous at a zero-width node**, which sits inside everything at that
  offset. The LSP needs a tie-break rule.
- **The tiling invariant restates as "concatenate the *non-empty* leaves."** Still one line.
- **A missing token's diagnostic needs a real primary span.** compiler §3.1 mandates exactly one
  and a zero-length caret renders as nothing, so the convention — the end of the previous token,
  or the start of the next — belongs to the diagnostic layer, not the tree.
- **The classic loop hazard**: recovery that synthesises a missing token and retries can spin. The
  guard is that every error consumes at least one real token.

### 6.5 Lexical errors are excess, and are never reported twice

The parser **runs even when lexing produced diagnostics** — tooling §3's error tolerance is a
property of the pass, and the LSP driver never aborts.

`token.Invalid` is a real kind in the lexer's `Error` category, and **no production names it**, so
it can never be scanned. It is therefore always *excess* in §6's sense: §7.2's layer 2 finds no
ancestor that accepts it and falls to "consume one token into an `Error` and retry", which is the
minimal-damage path rather than a concession.

**An `INVALID` token gets no `P` code.** The lexer already raised an `L` for it, and reporting
again is §7.6's double-report across a phase boundary. One conditional, and the kind of rule that
is forgotten exactly once and then reports every bad byte twice.

The cost of all this is bounded in a way worth knowing: **the batch driver never reaches the
parser with lexical errors at all**, since compiler §3 aborts at the phase boundary. Only the LSP
gets here, and there a mostly-`Error` tree is the desired outcome — the valid parts still parse.

### 6.6 The format already carries both

No change is needed to `testdata/golden.md`: a missing terminator renders `SEMICOLON 18..18 ""`
and skipped garbage renders `Error 12..15` with its tokens beneath, both inside the existing three
sections. `error_producing/` becomes writable as soon as the synchronisation points are chosen.

---

## 7. Recovery: synchronisation and cascade

§6 says what a damaged tree *looks* like. This says how it comes to be.

Everything hard here follows from one fact: at the moment of an error the parser is inside a
stack of half-open constructs, and it does not know which of them the author meant to close. In
`const f = fn (a, b => a;` the missing `)` is met while `ParamList`, `FnLit`, `BindingDecl`,
`Declaration` and `File` are all open, and the right answer is to close exactly one. Close too
few and the parse never recovers; close too many and a single mistake swallows the file.

### 7.1 What grammar.md gives us for free

- **`;` terminates almost everything, at every nesting level** — `Statement`, `BindingDecl`,
  `DeferStmt`, `ErrorField`, `MemberDecl`. One anchor works inside a `proto` block, inside an
  `error` declaration, and at file level alike.
- **`}` closes the rest** — `CompoundStmt` and `TestDecl` end there rather than in `;`.
- **Brackets nest**, and can be matched in one linear pass over the tokens.
- **No semicolon insertion, no significant whitespace, and no multi-line command literals**
  (R244), so there are no layout ambiguities to reason about.

One caveat, from §9: a bare `Statement` derives at top level, so the file-level FIRST set is not
merely the declaration keywords. Privileging `import` / `export` / `const` / `let` / `var` /
`test` / `#[` as anchors is a heuristic about real files, not a fact about the grammar, and
should be commented as such where it is written.

### 7.2 The strategy, in four layers

1. **At an expect-site, synthesise.** The parser knows exactly one terminal is required — this is
   §11.1's 274 committed expect-sites — so it emits a zero-width leaf of that kind (§6.1),
   reports a *missing token*, and continues. No search, no heuristic, no frontier computation:
   the call site already holds the answer.
2. **At a recursion site, do not guess.** Nothing single is expected there, and §11.1 measured
   those frontiers at 26–84 tokens wide. Close to the **nearest ancestor that accepts the current
   token**; if no ancestor does, consume one token into an `Error` node and retry.
3. **Bracket scaffold bounds both.** A linear pre-pass matches `()[]{}` over the token stream, and
   recovery may never skip past the closer of the innermost open bracket — so damage inside a
   nested construct stays inside it.
4. **Panic mode is the floor, never the first resort**: skip to `;`, `}`, or a declaration
   keyword, and pop to a frame that accepts it.

### 7.3 Why this is the good outcome at the cheap price

This is local repair's *result* — "missing `)`" rather than "unexpected `=>`", with the insertion
point a code action needs — obtained without local repair's *search*. Burke–Fisher style recovery
tries deleting, inserting and replacing at the error point and keeps whichever advances furthest;
it gets good messages by guessing well. We do not have to guess, because §11.1's measurement
found the line: **the 274 expect-sites are exactly where the repair is unambiguous, and the 429
recursion sites are exactly where guessing would be reckless.** The grammar hands over the safe
repairs and we decline the rest.

The consequence worth stating: **no backtracking is required anywhere.** Layer 1 synthesises
without lookahead, layers 2–4 only ever close frames or consume tokens. The parser stays single
pass over the token stream, which was not the reason for §4's event stream but does mean its
rewind capability goes unused rather than load-bearing.

### 7.4 Rejected

- **Error productions in the grammar.** Structurally foreclosed, not merely disliked: grammar.md
  is the authority and `internal/ebnf` reads it, so a production admitting a mistake would make
  the grammar *derive* invalid programs — breaking the reject-set invariant the parse goldens
  rest on (`testdata/golden.md` §0).
- **Panic mode as the primary mechanism.** Fifty lines and entirely predictable, but it discards
  everything between the error and the anchor: the example above loses the whole signature, and
  every message stays at "unexpected token".
- **Full local-repair search.** Bought nothing over layer 1 for the cases that matter, and its
  furthest-advance heuristic is a source of confidently wrong suggestions.

### 7.5 Structural cascade: minimal

**Synthesise only at expect-sites recovery actually reached.** Nodes may lack children, and that
is the tree telling the truth about the file.

**Rejected: liberal synthesis (Roslyn's).** Filling in every missing child keeps each node
shape-complete, which sounds like it makes accessors total — but §1 already established that error
tolerance forces every accessor to handle absence anyway. So liberal synthesis buys a totality
that was never available, and pays for it in structure the file does not have: an LSP that
cheerfully shows a `;` nobody wrote.

### 7.6 Diagnostic cascade: one rule now, the rest tuned later

One real mistake tends to produce a storm as everything downstream misparses. **Ruled now, being
the cheapest and safest: never report two errors at the same token position.** It kills most
storms and cannot suppress a genuine second finding, since two findings at one token are one
finding twice.

Three further devices are available and deliberately not chosen yet — a quiet period of *N*
tokens after an error, suppression inside an already-errored subtree until the next sync point,
and a per-file cap with a "too many errors" tail. The tension that makes them a later decision:
the LSP genuinely wants several *independent* errors, because a file mid-edit has more than one,
so a quiet period trades real second findings for calm and its *N* should be tuned against the
`error_producing/` goldens rather than picked now.

### 7.7 How we will know it works

Recovery is usually judged by feel. It does not have to be here.

- **The `error_producing/` goldens** pin partial trees exactly, so a change in recovery is a
  reviewable diff rather than an impression.
- **Perturbation, mechanically.** `ebnf.Enumerate` already produces every sentence the grammar
  derives up to a length. Delete, insert or replace one token in each and assert two things: the
  parser reports **exactly one** error, and its tree still covers most of the input. That is a
  quantitative recovery metric over hundreds of thousands of cases, built from machinery that
  exists. It is the harness to build early — it turns "is our recovery good" into a number that
  moves when the sync sets change.
- **The reject-set invariant** (`testdata/golden.md` §0): what the parser diagnoses and what the
  grammar rejects are the same set.

---

## 8. The AST view is an API, not a second tree

> **The AST view is the CST traversed with trivia skipped.** It is not a transformation and there
> is no second tree. §1.4's *typed* AST is this same tree plus side tables of resolved symbols and
> types, keyed by `NodeID` (§3.1).

### 8.1 Why the gap is one category wide

A CST normally carries four things a compiler does not want. Three of them are never built here,
so the view has almost nothing to do:

| Normally present | Our status |
|-|-|
| Trivia | present (§2) — **the only remainder** |
| Unit-cascade wrappers (`Expr → Assignment → … → Primary`) | never built: §5 made pure alternations non-kinds, and a tier is opened only when it fires |
| Desugar artifacts (`LHS·n`) | never built: the parser loops where the EBNF→BNF rewrite nests |
| Empty optionals | never built (§6.1) |

A consequence worth having: the parse goldens' tree section is **literally the compiler's view**,
so one corpus tests both consumers.

### 8.2 Trivia only — punctuation cannot be filtered, and §5 is why

"Trivia and punctuation" is the intuitive filter and it is wrong here. Because §5 elided all 26
pure alternations, the **operator tokens became the sole carriers of their distinctions**:

- `=` / `+=` / `??=` — `AssignOp` is gone, the token is the operator
- `match` / `match!` — likewise `MatchKw`
- `.` / `?.` / `->` / `?->` — four access forms, one `Postfix` kind, told apart only by the
  leading token
- `!` / `-` / `@` / `@@` in a prefix; `..` / `..<`; `!` / `?` on a `PostfixType`

Filter punctuation and the tree stops meaning anything. Two decisions interacting: §5 bought a
smaller enum by making punctuation load-bearing, and this is the bill.

### 8.3 Two rules that keep the view honest

- **Error nodes and zero-width missing tokens stay visible.** §1.4 needs to know a subtree is
  damaged so it can skip semantic checks on it rather than emit a cascade of `S` diagnostics —
  §7.6's instinct, one phase up.
- **The view never filters by importance.** Trivia is the only category with a definition;
  everything else is a judgement, and judgements drift. Mechanical and total, or not a view.

### 8.4 Rejected

- **A second tree built by a lowering pass.** The compiler's structure could then be exactly what
  it needs and desugaring could be baked in — control-flow §5's postfix forms are an *exact*
  desugar (R46), so an AST collapsing them would be faithful. But it costs two structures, an
  invariant between them, and a span mapping on every diagnostic; and the desugaring argument is
  answered elsewhere: **compiler §1.5 already exists to lower to IR**, which is where desugaring
  belongs. Doing it at parse time would cost the CST its fidelity and the formatter its job.
- **A projection** — AST nodes each holding a CST reference. The second tree's cost plus an
  indirection, and it only pays when the AST's *shape* differs from the CST's. Ours does not.
- **No view at all**, each pass walking the CST directly. That is this design without the
  discipline: every pass reinvents "skip trivia" slightly differently.

---

## 9. Sugar is normalized by accessors, never by rewriting

Luna has a good deal of syntactic sugar — postfix modifiers, `T?`, `T!`, compound assignment,
UFCS, `else if`, multi-clause `where`. The parser collapses **none** of it.

> **The frontend keeps the syntax the user wrote, through §1.4.** Sugar is normalized by
> *accessors*, never by rewriting the tree. **§1.5 is where sugar dies** — after the last
> user-facing diagnostic has been issued.

### 9.1 Why the parser cannot do it

tooling §2 requires the CST to be full-fidelity: *the exact source bytes are reconstructable*.
Collapse `x = 5 if (c);` into `if (c) { x = 5; }` and that is gone — and since the formatter reads
the CST, it would rewrite every postfix form into a block form. That is not a formatter, it is a
rewriter.

So desugaring needs a **second structure**, which is §8's rejected option A, rejected for exactly
this reason: **compiler §1.5 already exists to lower to IR**.

One precision, because the spec sentence reads like an instruction and is not one. Control-flow §5
says the postfix form **is** an exact desugar (R46) and that "everything follows from that
identity rather than from new rules". That fixes the **meaning**, not the mechanism — the identity
holds whether or not any tree is transformed.

### 9.2 Three cases that break rather than merely cost fidelity

- **Compound assignment cannot be desugared naively.** Associativity §1: `a op= b` is
  `a = a op b` *with the target evaluated once*, so `t[f()] += 1` calls `f` once. A tree-level
  rewrite has to introduce a temporary, and the AST then holds **nodes with no source behind
  them** — §7.5's phantom structure arriving through a different door, with every diagnostic on
  one having to point somewhere plausible.
- **UFCS cannot happen in the parser at all.** `x.map(f)` → `map(x, f)` requires knowing whether
  `map` is a free function or `x.map` is an element access holding one. That is symbol knowledge,
  which grammar.md §9's table exists to keep out of the grammar. §1.4 at the earliest.
- **`T?` → `T | null` costs diagnostics measurably.** "expected `int | null`, got `string`" where
  the author wrote `int?` is worse, and recovering the spelling for the message means keeping it
  anyway.

### 9.3 The benefit is real; accessors deliver it without the cost

The honest argument *for* desugaring is uniformity: a check written once against the collapsed
form cannot diverge between spellings, so you never get the block form reporting well and the
postfix form reporting badly.

That is obtainable from a **normalized accessor** rather than a normalized tree —

```go
// one accessor, both shapes
func IfLike(t *Tree, n NodeID) (cond, body, els NodeID, ok bool)
```

— answered by both `IfStmt` and a `Statement` carrying an `if` `Modifier`, with the checker
written once. It is precisely §8's "typed accessors written where the compiler needs them", so the
mechanism is already chosen: no second tree, no spanless nodes, and the formatter keeps its job.

### 9.4 The principle, and an honest concession

**Desugar only after you have said everything you need to say about the original syntax.** `S`
codes are the last thing a user reads about their own code and they belong to §1.4; lowering
belongs to §1.5. The phase boundary is already in the right place.

The concession: the *diagnostic* cost of desugaring in the frontend would be small — mostly
`T?`-shaped renaming, not lost information. What decides this is not diagnostics. It is that
desugaring reintroduces the second tree §8 spent a decision eliminating, and takes the formatter
with it.

---

## 10. Implementation order

Not a design decision — a sequence chosen so that **each step has a test before the next one
starts**, and so that the API is judged by a real caller before the parser exists.

1. **`Kind`, and its two pins.** Hand-written constants (§5), then the assertions: every surviving
   §0 nonterminal has a kind and the reverse, and `Kind(k)` matches numerically for every
   `token.All()`. Both are real tests that pass before any tree exists — lexer-testing-plan §1's
   "inventory pin, build first", and the cheapest first signal available here.
2. **The arena and the navigation API** (§3.1): `Tree`, `NodeID`, children, spans, kind, immutable
   once built (§4.3).
3. **Events, splice and builder** (§4, §2.2), events unexported. Unit-tested against hand-written
   sequences (§4.2), and §2.3's index-coverage assertion lands here.
4. **Port `golden_render.go` onto the new tree.** The first real caller, and the step that makes
   this a plan rather than a hope: it exercises kinds, traversal, spans and leaf text against
   **thirty goldens that already exist**, so the API is tried by use rather than by inspection.
   `internal/ebnf` then drops out of this package exactly as `golden_render.go`'s own header note
   predicts, and the goldens stop testing only the grammar and start testing the tree too.
5. **The parser proper**, a production group at a time, goldens going green as they go.
6. **Recovery** (§7), and with it `error_producing/` and the perturbation harness (§7.7) — the
   thing that turns recovery quality from a judgement into a number.

## 11. Still open

- **The quiet period's *N*** (§7.6), and whether the other two cascade suppressors are wanted at
  all. Tuned against the `error_producing/` goldens once they exist, not chosen now.
- **The mismatched-bracket heuristic** (§7.2 layer 3). The scaffold is well defined when brackets
  nest; what it should do with `([)]` is not, and the answer wants real cases.
