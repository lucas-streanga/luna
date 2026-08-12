# Associativity and precedence

The binding table the parser implements. Sources: the operator catalogue (operators §0), the
positional dual-role rule (operators: `&`, `!`, `@`, `error`, `comptype` resolved by
position), and the rulings in the specs cited per row. Expression grammar and **type
grammar** are separate tables, because type position is its own grammatical world (operators
§0, type spec §3.1).

## 1. Expression precedence, tightest to loosest

| Tier | Operators | Associativity | Notes |
|-|-|-|-|
| 1 postfix | `f(...)` `x[...]` `x.name` `x->name` `x->P.m` `x?.name` `x?->name` | left | one chainable postfix tier: `a->P.b(c)[d]` parses left-to-right; `x.name(` is UFCS, `x.name` is access (functions §3.4); `->` reaches protocol space, bare or qualified, assignable where the member grants `set` (protocols §3); `?->` is its soft form, short-circuiting like `?.` (protocols §3.2) |
| 1a structure | `x apply P(...)` | left | the `apply` operator (R158): the left operand is a complete tier-1 postfix expression; the right side is the operator's **own closed grammar** — a proto name plus an optional initializer list, never an expression (protocols §4.2) — so only the left edge needs a tier; chains left (`[] apply person(...) apply employee(...)`, protocols §7) and binds tighter than every comparison, so `x apply P is @P` needs no parens |
| 2 prefix, symbolic | `!x` `-x` `@x` `@@x` | right (stack) | `!` is logical not, prefix, expression position only (operators §0; the postfix `T!` lives in the type grammar) |
| 3 multiplicative | `*` `/` `%` | left | |
| 4 additive | `+` `-` | left | `+` is numeric only; there is **no** concat operator (string §3), interpolation joins |
| 5 range | `..` `..<` `by` | **none** | non-chainable: `a..b..c` is a parse error; arithmetic binds tighter, so `0..n-1` is `0..(n-1)` and `0..n by k+1` steps by `k+1`; `by` is part of the range production (range §3, §4a), one per range |
| 6 comparison | `<` `<=` `>` `>=` `is` `as` | **none** | non-chainable, `a < b < c` is a compile error, comparisons yield `bool` and re-comparing a bool to a number is a base mismatch anyway (equality §1); `is`/`as` take a **type expression** on the right (table §2), so no right-side ambiguity, and they sit here so `x is int == y` demands parens |
| 7 equality | `==` `!=` | **none** | non-chainable; there is **no** `===` / `!==` (§4, resolved) |
| 8 conjunction | `&&` | left | short-circuits |
| 9 disjunction | `\|\|` | left | short-circuits |
| 10 coalescing | `??` `???` | right | `a ?? b ?? c` is `a ?? (b ?? c)`, the natural chain; `???` (null-or-absent coalesce, coalescing spec) sits on the same tier and mixes freely |
| 11 conditional | `c ? a : b` | **none** | non-chainable: `a ? b : c ? d : e` is a parse error, because match §6's open-ended form *is* the guard chain and a chained ternary would be a second spelling for it (the `===`/`\|>` argument, §4). Nesting is by parentheses, either arm. Looser than `??`, so `x ?? y ? a : b` is `(x ?? y) ? a : b`; tighter than the word prefixes, so `try c ? a : b` is `try (c ? a : b)`. The `?` is position-resolved like `&` and `!`: expression position is the conditional, declaration position is the optional marker (`let n?: T`, `name?: T = null`, a proto member's `name[?]:`), and `T?` is the type grammar's. (This tier was R146's retired pipeline slot; the retirement is recorded in §4, and tier-12 citations still stand.) |
| 12 prefix, word | `copy` `try` `spawn` `await` `comptime` `comptype` `throw` `declared` | right | see §3: word prefixes bind the **whole expression** below assignment; `declared` is the degenerate member (R158) — its operand is exactly **one binding name**, never a general expression (type §4), so precedence barely bites, but it lives here as the word prefix it is |
| 13 assignment | `=` `+=` `-=` `*=` `/=` `%=` `??=` `???=` | right, statement-ish | compound `a op= b` is `a = a op b` with the **target evaluated once** (`t[f()] += 1` calls `f` once); `??=` assigns on absence, `???=` on absence or `null` (coalescing spec); requires the same rebindability as `=` (a `var`, or an element write); there is no `&&=`/`||=` for now; `.=` died with `.` (string §3) |

Postfix **statement modifiers** (`expr if (c)`, `expr foreach (...)`, `expr while (...)`,
control-flow spec) are statement grammar, not operators, and sit outside this table; `where`
exists only inside `match` arms (match §3) and `constraint` bodies (constraints §1), and can
never extend a type expression, which is what lets it terminate the type in `n: int where n > 10`
(match §2.1); `=>` is arrow/arm punctuation, not an operator; `name:` at the head of a call
argument is named-argument punctuation (functions §3.3.2), decided at one token;
`expr use (caps)` is the call-site delegation clause (capabilities §5.2, R112), wrapping one
complete postfix expression — no postfix may follow it, and operators and statement modifiers
compose outside it;
string interpolation is lexical, not an operator; `...` is pattern punctuation (destructuring §1.2), never an expression operator.

## 2. Type-position precedence, tightest to loosest

| Tier | Forms | Associativity | Notes |
|-|-|-|-|
| 0 grouping | `( T )` | — | a parenthesized type is that type; the parentheses group and mean nothing else. Numbered 0 so tiers 1–5 keep their numbers and every citation stands. Two uses, one optional and one not: **readable emphasis** where the tiers already agree (`(@P & @Q) \| null`, the overview's convention, tier 4), and **required** wherever a tier binds looser than intended — chiefly tier 5, whose result type extends greedily right, so `(fn (): int) \| string` is a union of two types where `fn (): int \| string` is one function type returning a union. There is no tuple type, so `( T )` is never a one-element anything |
| 1 postfix | `T!` `T?` | left, stackable | `string?!` ≡ `string!?` ≡ `string \| null \| error` (errors §7, value-representation §3.1), order-independent by canonicalization |
| 2 refinement | `@P` | prefix | application refinement; `@X` on a non-protocol is a compile error (protocols spec) |
| 3 intersection | `&` | left, canonical | commutative after interning (`@Q & @P` ≡ `@P & @Q`, type §3.1) |
| 4 union | `\|` | left, canonical | `&` binds tighter than `\|`: `@P & @Q \| null` is `(@P & @Q) \| null`; the overview's parenthesized spelling stays as the readable convention |
| 5 fn | `fn (params): T` | , | the result type extends greedily right through tiers 1-4: `fn (): int \| string` returns the union; parenthesize the `fn` type itself to embed it in a union |

Position decides which table applies: `&x` in an argument is a reference (variables §5.1),
`A & B` in an annotation is the meet; `!x` in an expression negates, `T!` in a type adds the
error arm; same rule as `error` and `comptype` (dual keyword/type, errors §3, introspection
§4.2).

**Type position is entered from exactly four places**, and nowhere else (R256): after a `:` in
an annotation, parameter, member, field, or typed binder; on the right of `is` / `as`; inside a
declaration form that takes one (`constraint i: T`, an `enum` variant payload, a `catch` head);
and after `: type` in a binding, which is how a type expression reaches **value** position at
all — none of this table's constructors (`|`, `&`, postfix `?` / `!`, `fn (params): T`) has an
expression production, so `const number: type = int | double;` parses because of the
annotation, not in spite of it (type §2). In expression position `fn` therefore always begins a
function *literal*, never a type, which is what lets `fn` commit its production one token in
(functions §3, R45).

**Grouping is not one of those four**, and this is the corollary that catches people: tier 0's
`( T )` and the expression grammar's `( expr )` are different productions sharing a glyph, and
which one a `(` opens is settled by the position already in force — a parenthesis never
*switches* grammars. So `@(int | double)` does not parse: `@`'s operand is value position, the
parenthesis opens an expression, and `int | double` is not one. Nor would it mean anything if
it did — `@` on something already a type is a compile error (types overview), and the question
it is reaching for is `x is (int | double)`, where `is` has put the right-hand side in type
position and the parentheses are optional. **Pattern position is a third grammar**, specified in match §2.1: it is neither of the
tables above, a type occurs in it only after a `:`, and `|` is therefore the union operator
inside a type and the alternation separator outside one (§4).

## 3. Word-prefix binding, the one designed decision in this file

Symbolic prefixes (tier 2) bind tight, as everywhere. **Word prefixes bind loose**: `copy`,
`try`, `spawn`, `comptime`, `comptype`, `throw` take the **entire expression** to their
right, down to (but not including) assignment and statement punctuation. So `try a + b` is
`try (a + b)`; `copy t.name` is `copy (t.name)` (postfix is inside the expression);
`spawn f(x)` spawns the call; `_ = try cleanup()` reads as errors §8.1 wrote it. Rationale:
a word reads like a clause head, and the alternatives are worse, tight binding would make
`try a + b` mean `(try a) + b`, adding an error-typed value to `b`, a base-mismatch error
that would *usually* be caught but is a trap where it isn't. Nesting is by parentheses:
`try (copy t)` and `copy (try f())` both parse.

## 4. Resolved drift and resolved rulings — nothing open (R158)

**Resolved here** (each previously inconsistent between specs):

- **`===` / `!==` do not exist.** bool.md used `!==` for null checks while the catalogue
  never defined it (the F11 drift). Under erasure equality (equality §1) `!= null` is exact
  and total, and an identity operator would duplicate what `==` already means for identity
  types (equality §2); bool.md is fixed to `!=`.
- **No infix `.`**, no `.=` (string §3): removes the whitespace-sensitive collision with
  member access at tier 1.
- **`@int` in a pattern is a compile error** (match §2): pattern-type position is type
  position, table §2 applies.

**Resolved by ruling (were open):**

- **The conditional operator `c ? a : b` exists** (R254), tier 11 above, **non-chainable**.
  It was specified nowhere and used at four sites, while lexer §5 listed it among the tokens
  Luna does *not* have — the grammar sweep's find. Non-chainability is the same argument that
  retired `===` and `|>`: match §6's open-ended form already *is* the multi-way conditional
  expression, so a chained ternary would be a second spelling for one mechanism, and Luna
  prefers the tier-5/6/7 answer (`none`) wherever a chain has another spelling. It occupies
  the tier number R146 vacated when the pipeline `|>` was retired (retired/pipeline.md); the
  retirement stands, and tier-12 citations are unaffected either way.
- **Compound assignments exist**: `+=` `-=` `*=` `/=` `%=` `??=` `???=`, tier 13 above, sugar with
  single evaluation of the target. `&&=`/`||=` excluded for now (no demonstrated need;
  addable later without breakage).
- **Ranges never reverse.** `a..b` with `b < a` is the **empty range**, total and loop-safe;
  the counterexample that decides it is `0..n-1` at `n == 0`, which under implicit-descending
  would silently iterate `0, -1` in the most common loop header in the language. Descending
  iteration is **explicit**: a negative step, `10..0 by -2` (range §3, landed in R47, the
  "possible later" of this note), or `reverse(r)` for an existing stream. `0..-1` parses as `0..(-1)`
  (tier 2 inside tier 5) and is empty, consistently. Companion ruling: **unary negation of
  `int.min` panics** as overflow, joining `INT_MIN / -1` in int §overflow.
- **`&` outside argument position is a semantic error**, not a grammar production: the parser
  accepts prefix `&` uniformly and semantic analysis rejects non-argument uses (variables
  §5.1), keeping the grammar regular and the diagnostic precise.
- **`await` is defined** (concurrency/await.md): word prefix, tier 12; parks the green
  thread, moves the result out, consumes the promise, surfaces the task's error or panic at
  the collection point.
- **`throw ... if (...)` parses** as tier-12 prefix under a statement modifier; pinned.
- **Pattern grouping**: ~~at pattern top level `|` is always the or-pattern separator; an inline
  union *type* pattern requires parentheses, `(int | string) n`~~. **Retracted, and the question
  it answered no longer arises.** A type appears in a pattern only after a `:` (match §2.1), so
  **`|` is the union operator inside a type and the alternation separator everywhere else**, and
  the two readings never occupy the same position. `n: int | string` is a union; `1 | 2` is an
  alternation. No parenthesization rule is needed, and the grammar stays LR(1) with one-token
  decisions for a better reason than a carve-out: at an `IDENT` or `_`, peek for `:`. The
  parentheses survive only for the rare inverse, using a *typed* pattern as an alternative,
  `(_: string) | 5` (match §2.1, §5).
