# Golden file format (`.parse`)

The format is fixed here **before the parser exists**, deliberately: the same corpus feeds two
tools, and the second one (`internal/ebnf`) is already running. Fixing the shape first is what
lets the recognizer start consuming goldens now and the recursive-descent parser join later
without a migration. This file also records the findings from designing it — the parts that are
decisions rather than mechanics.

Nothing here is a language decision, so nothing here is a `CHANGES.md` ruling; it sits beside its
data exactly as `oracle/lexer/testdata/FORMAT.md` does.

The format's reader and writer are `../golden.go` and `../golden_render.go` — in this package
rather than under `internal/`, because the goldens are the **parser's** expectations and only
incidentally something `internal/ebnf` also reads. Exported names there are `Golden`-scoped
(`ReadGolden`, `LexGolden`, `RenderGolden`) so that a bare `Parse` and `Node` can arrive beside
them without collision.

---

## 0. Why one corpus and two tools

The EBNF tool is a **recognizer**. Its whole output for a case is two bits — accepted, and derived
exactly once — which is an assertion, not a golden. The parser's output is a tree and diagnostics,
which is worth pinning. So the shared artifact is the **input**, and the directory discriminates,
exactly as `error_producing/` already does for the lexer.

| | `testdata/` | `testdata/error_producing/` |
|-|-|-|
| **`internal/ebnf`** | accepted ∧ **exactly one derivation** | **rejected** |
| **the parser** | tree matches ∧ no diagnostics ∧ CST reconstructs the source | diagnostics match ∧ partial tree matches |

The table buys a property neither tool has alone, and it is the reason to build this:

> **The set of inputs the grammar rejects and the set the parser diagnoses are the same set.**

compiler §1.3 makes parse a pure context-free pass, so that equality is a real invariant. A `P`
code with no grammar rejection behind it means the parser invented a rule; a grammar rejection
with no `P` code means an input dies with no diagnostic. Nothing checks either today.

**Every golden is a whole file**, parsed from `File` — one start symbol, the same discipline the
431-block spec corpus follows (R258, R265). The spec corpus stays separate and stays free
coverage: the parser should run all of it for *parses ∧ reconstructs*, with no goldens. These
files are for pinning **shape**, and for the adversarial cases nobody writes in a spec.

---

## 1. The file

Three sections, separated by a `---` line: source, tree, diagnostics. A clean case has two
sections and no trailing separator.

```
let x: int = 1 + 2;
---
File 0..20
  BindingDecl 0..19
    KW_LET 0..3 "let"
    Binder 4..5
      IDENT 4..5 "x"
    COLON 5..6 ":"
    Type 7..10
      IDENT 7..10 "int"
    ASSIGN 11..12 "="
    Additive 13..18
      INT_DEC 13..14 "1"
      PLUS 15..16 "+"
      INT_DEC 17..18 "2"
    SEMICOLON 18..19 ";"
```

Leaf lines are `FORMAT.md`'s, unchanged: `NAME start..end "lexeme"`, half-open byte spans, Go
`%q`, **spec names not Go identifiers**. Interior lines are the same without the lexeme.
Indentation is two spaces per level.

- **Names are grammar.md §0 nonterminals.** The parser is recursive descent over §0, so it has a
  function per nonterminal and printing the name is free — and the golden is therefore reviewable
  against the spec, which is the same principle `FORMAT.md` states for token names. No separate
  AST vocabulary is invented; if one is ever specified for §1.4's benefit, it is a second view,
  not a replacement for this one.
- **Trivia is dropped**, and stays dropped: an indented tree is readable against §0 and a tree
  interleaved with whitespace nodes is not. grammar.md is defined over the trivia-filtered
  stream, and the 85 `.lex` goldens already pin every trivia token.
  What no golden here can see is *placement* — which node a comment lands in — so that is
  asserted directly rather than displayed: **index coverage** over the spliced event stream and
  the **never-first-or-last-child** invariant over the tree, both run on every case and the whole
  corpus (`../parser-implementation.md` §2.3). Losslessness is therefore checked without being
  dumped.
- **`File`'s span is the whole file**, and it is the one line here that dropping trivia would
  otherwise turn into a lie. Trivia are nodes in the CST, and §2.1 pushes them outward into the
  innermost node already open — so a file's leading and trailing trivia land in `File`, and a
  node's span is the extent of its children with that trivia counted. The confinement is exact:
  trivia is never the first or last child of anything but `File`, so `File` is the **only** node
  whose span differs from the extent of its non-trivia tokens, and every other line here is the
  same number under either reading. It is therefore always `0..len(source)`, which the harness
  asserts rather than leaving to the eye. The example above ends in a newline, which is why its
  `File` runs one byte past its `BindingDecl`; `comment-only-file` is the extreme, 64 bytes of
  comment under a tree section of one line.

  The alternative was to keep printing the extent of the non-trivia tokens, which is what a
  derivation yields and what these files held until the parser needed an answer. It loses on the
  point of the section: the tree section is a **transcript of the tree**, and a span no node has
  cannot catch the off-by-one it exists to catch. What it costs is that the `File` line now pins
  nothing but the file's length — deliberately, since the root's real span is checked by
  reconstruction, which is stronger than any line in a golden.

  `0..0` can no longer appear: a zero-byte file has no tree at all (`../parser-implementation.md`
  §6.1), and no golden could express one anyway, since a case's source section is never empty.
- **Diagnostics are a third section**, one per line, `!P0001 12..13` — the code and the primary
  span, never the prose (grammar.md §11). The leading `!` is redundant here (unlike in `.lex`,
  where diagnostics interleave with tokens) and is kept anyway so that `grep -rn '!P0' testdata/`
  spans both formats.
- **The harness checks the directory against the file**, rather than trusting it: a case in
  `error_producing/` with no diagnostics section, or one outside it with a diagnostics section,
  is a failure. A misfiled golden is otherwise invisible — it still passes, it just stops meaning
  what its directory says.

## 2. Elision, and the hazard that shapes it

A dump that prints every nonterminal is unreadable: `1` is thirteen nested tiers. So a
nonterminal that passes through with a single child collapses, for one of two reasons.

**Pure alternations**, *computed from the grammar rather than listed*. A nonterminal whose every
production is one symbol — `AssignOp`, `CompOp`, `WordOp`, `BindingKw`, `MatchKw`, `Keyword`,
`TopLevelItem`, `BlockItem`, `PathSegment`, `Literal`, `IntLit`, `StringLit` and their kin — is
pure **dispatch**: its name says which of several shapes follows, and the child it yields already
says the same thing. Nothing else about the grammar distinguishes them, which is what makes the
set computable (`ebnf.PureAlternations`) instead of a list somebody maintains — a new operator
class added to grammar.md joins by being written, not by being remembered.

One symbol means one the grammar wrote. `Prelude ::= PreludeItem*` reads as a single symbol only
because the desugar made the repetition an `LHS·n` helper, and that helper stands for any number
of children — so `Prelude` is **not** in the set, and it prints whenever the file has imports.
The set is read for "always exactly one child", which is a property of the arity and not of the
production's length (`../parser-implementation.md` §5).

**The precedence tiers**, which are listed, because no shape distinguishes them:

- the expression tiers — `Expr`, `Assignment`, `WordPrefix`, `Conditional`, `Coalesce`,
  `Disjunction`, `Conjunction`, `Equality`, `Comparison`, `RangeExpr`, `Additive`,
  `Multiplicative`, `PrefixExpr`, `ApplyExpr`, `PostfixExpr`, `Primary`
- the type tiers — `UnionType`, `IntersectType`, `PostfixType`, `PrimaryType`
- the pattern tiers — `Pattern`, `AltPattern`, `PrimaryPattern`

Everything else always prints — and **`Type` prints despite being a pure alternation**, the one
override, for the reason below.

**The hazard, which is why `Type` is an override and why the tiers are a list.** The obvious
blanket rule — *elide any node with one child and no token of its own* — is right for the
expression tiers, where the surviving name is the tier that fired (`Additive`) and the collapsed
ones said nothing. Applied uniformly it also deletes five type nonterminals in a row (`Type →
UnionType → IntersectType → PostfixType → PrimaryType → IDENT`), and then `let x: int = y;`
prints two bare `IDENT` lines with **nothing recording that one of them is in type position**.
That is the one distinction R256 exists to make, erased at exactly the leaves. So `Type` is
overridden back, and the wrappers that are not pure alternations — `Binder`, `Param`,
`Initializer`, `MemberDecl` — keep themselves, at one line per occurrence. It is what the
`is-intersection-vs-and` case turns on: `x is int & y` puts `int & y` inside one `Type`, while
`x is int && y` ends the `Type` at `int` and splits at `Conjunction`. The two trees differ where
the claim in grammar.md §11 says they do.

`ArmBody` and `FnBody` are **not** in that group, though they read as though they should be: both
are `Block | Expr`, so both are pure alternations and both collapse. Nothing is lost, because what
they wrapped says it instead — a block body shows a `Block`, an expression body shows the
expression — and the ordered choice §11 flags stays legible in the tree either way.

Most delimited forms need no exception: `Block`, `TableLit`, `DestructurePattern`, `FnLit` and
their kin carry their own tokens, so they survive any rule. That is why the exception list is
short.

**A second consequence, worth stating so nobody expects otherwise.** Elision makes goldens
insensitive to tier insertion and removal: adding a fourteenth precedence tier changes no golden.
That is mostly a feature — no churn on a refactor — but it means goldens do **not** pin the tier
count. `TestNonterminalCount` in `internal/ebnf` does, and is the thing to reach for when that is
the question.

## 3. The rule

Carried from `FORMAT.md` verbatim, because it is the whole discipline:

> A golden is never trustworthy merely because the parser produced it. Committing tool output as
> the expectation asserts only that the parser still does what it did first, bugs included. A
> regenerate flag is fine; the discipline is that the resulting **diff** is read against
> grammar.md §0 before it is committed.

---

## 4. Findings, and what is deferred

**Goldens do not find ambiguity. They pin it once found.** Ambiguity lives in the input nobody
thought to write — all five fixed in R264 came from running the spec corpus, not from imagining
cases. A hand-written hazard corpus is a regression net and worth having; it is not the
search.

**The search is bounded exhaustive generation, and it now exists**: `internal/ebnf/generate.go`,
run in full by `./check.sh --ambiguity`. It enumerates every sentence the grammar derives up to a
length in tokens and routes each through the same Earley recognizer, so the package keeps one
ambiguity oracle rather than two — and a sentence it generates that the recognizer rejects is a
report line of its own, since both read the same productions. It is opt-in rather than gated
because it is a proof over a fixed grammar: the answer moves only when grammar.md does, so the
gate keeps a three-token sweep and the deep table is run when the grammar changes.

**Its ceiling is structural, and it decides the division of labour with this file.** Two facts
about grammar.md set it. The thirteen expression tiers each store the whole expression language
at every length, so the table holds the same strings a dozen times over. And the grammar is **one
connected component** — `StringLit` admits interpolation, `Splice` holds an `Expr` — so even
enumerating `Type` drags in every expression form there is: naming a sub-language narrows the
output and almost never the work. The table is 227k strings at length 3 and 3.7M at length 4, a
factor of sixteen per token, which puts length 5 out of reach.

So generation covers **everything through four tokens** (and five from `ImportSpec`, whose
reachable set is 21 cells), completely — 212,845 sentences on the gate, and 186,630 from `Expr`
alone at length 4 in a longer run. The corners §11 flags are mostly **six to eight** tokens —
the parenthesized IIFE, `x->P.m`, `fn () => {}` against `fn () => .{read}` — so they sit past the
ceiling and are exactly what the goldens below are for. Neither instrument reaches where the
other does.

## 5. The hazard corpus, and how its trees were written

Thirty cases, seeded from grammar.md's own flagged list (§9 and §11) — the corners the file
already knows about, which are therefore the ones worth holding still. **Every one derives, and
derives exactly once**, which is the ambiguity result for the six-to-eight-token range that
generation cannot reach.

**Their trees were written by the grammar, before any parser existed.** `ebnf.Derive`
reconstructs the single derivation from the Earley chart and refuses an ambiguous input rather
than picking one of its trees — so the expectation in each file came out of §0's productions
rather than out of an implementation of them. That is the provenance §3's rule asks for, in its
strongest available form: the diff *is* read against grammar.md, because grammar.md wrote it.
Regenerate with `go test ./oracle/parser -update`, and read the diff before committing it.

The consequence to be clear about: when the recursive-descent parser lands, these trees are its
**target**, not a transcript of it. A disagreement is the parser failing to implement §0, or §0
failing to say what was meant — never the golden needing a refresh to match new behaviour.

`error_producing/` is empty and stays that way until the parser exists: a rejected input has no
derivation to render, and its partial tree and `P` codes are the parser's to produce (§0's table).
The grammar half of those cases — that they are rejected — is checked the moment they are written.

The seeded corners:

- `FnBody ::= Block | Expr` and its twin `ArmBody` — the file's only ordered choice
- `.{` opens a variant literal and `{` a block, so the two never contend (R272; R268 had ruled it the other way, with parentheses)
- `KW_ERROR` in three roles: `Primary`, `PrimaryType`, and the head of `ErrorLit`
- `&` versus `&&` after `is` — in type position `AMP` is intersection
- the one two-token junction: `KW_EXPORT? KW_CONST IDENT ASSIGN` needing to see `KW_IMPORT`
- `?` as the ternary against `?` as the optional-type suffix; `!` prefix against `T!`
- `x->P.m` as two postfixes (§9: qualification is semantics')
- the parenthesized IIFE, and `AMP` binding the base rather than the postfix chain
- postfix modifiers against their block forms, including the two `Statement` kinds §5.1 excludes
- `Subscript`'s three forms, including the empty `b[]` and the slice `[a:b]`
- trailing commas in all fifteen list positions, and the comma-less lists R263 used to admit
- `match` with a scrutinee against `match` with guards; `match!`
- the empty cases: empty `File`, empty `Block`, empty `TableLit`, `fn ()` against `fn (x)`
