# Luna — Grammar Specification

The context-free grammar the parser implements (compiler §1.3), collected from every spec in
the repository. This file is **authoritative for the shape of the language**; each topic spec
remains authoritative for what a form *means* and keeps its own examples (R253). Where prose
and a production here disagree, the production is the shape and the disagreement is a ruling.

Sources: `associativity.md` for both precedence tables, `match.md` §2.1 for pattern position,
`lexer.md` §0 for every terminal, and the owning spec named against each section below.

Conventions used throughout:

- **Terminals are lexer §0 token names**, never spellings: `KW_MATCH`, `LBRACE`, `FAT_ARROW`.
  This is what makes the two specs compose, and it removes a collision the metasyntax would
  otherwise have — `|` is alternation here and `BAR` is the token, so they never contend.
- **Nonterminals are `UpperCamel`.** `::=` defines, `|` alternates, `?` `*` `+` are the usual
  quantifiers, and `( )` groups. Following lexical-structure §2's lone production, which is the
  corpus's only prior use of this notation.
- **`IDENT("text")` is an `IDENT` whose lexeme is exactly `text`** — the positional
  spelling-match Luna already uses for `from` (R223), `get` / `set` (R232), and
  `identityEquality` (equality §4.4). It is not a keyword and does not reserve the word.
- **`!TERMINAL` is a guard**: a zero-width assertion that the next token is not that terminal
  (R270). It consumes nothing, derives nothing, and appears in no tree — it only forbids, and it
  takes no quantifier. One rule in this file needs one, *a leading token claimed by one reading*:
  `: type` puts a binding's right-hand side in type position (R269). Restricting a nonterminal by
  its first token is an intersection with a regular set, so the grammar stays context-free and
  `internal/ebnf` runs it unchanged; what the notation buys is that §0 **states** a rule that was
  prose, which is what makes it checkable — a rule a CFG cannot express is a rule the grammar
  over-accepts, and the parse goldens' reject-set invariant is exactly that gap. It was written
  for two rules; R272 retired the other by changing the syntax it adjudicated, which is the
  better repair where it is available.
- **The grammar is defined over the trivia-filtered token stream.** `WHITESPACE`, `MARGIN`,
  `SHEBANG`, `LINE_COMMENT` and `BLOCK_COMMENT` are emitted by the lexer (R236) and dropped by
  the parser (compiler §1.1), so they appear in no production. Re-attaching them to the lossless
  CST is compiler §1.3's and tooling §2's business, not this file's.
- **Every comma-separated list has one shape**, `Item (COMMA Item)* COMMA?`, wrapped in a named
  nonterminal and made optional by its parent where the list may be empty. A trailing comma is
  admitted everywhere and required nowhere. The shape is uniform on purpose: putting the comma
  on the *item* instead — `Item ::= … COMMA?` inside `Item*` — makes the separator optional
  altogether and derives `[10 20]`, which eight productions did until R263 (see the
  cross-reference notes).
- **Productions are fenced ` ```ebnf `**, so an extractor can find them and no corpus tool that
  keys on ` ```luna ` (R258) is disturbed.
- **The grammar is permissive by rule**: a restriction lives here only when it is decidable with
  no symbol knowledge. §9 lists what that admits and who rejects it.

---

## 0. The grammar

Every production, in one place. The sections that follow keep the **notes** — the rationale,
the orderings, the hazards — but not a second copy of these rules.

### 0.1 File and declarations

```ebnf
File          ::= Prelude TopLevelItem*
Prelude       ::= PreludeItem*
PreludeItem   ::= KW_IMPORT ImportSpec SEMICOLON
                | KW_EXPORT? KW_CONST IDENT ASSIGN KW_IMPORT ImportSpec SEMICOLON
TopLevelItem  ::= Declaration | Statement

ImportSpec    ::= ModulePath
                | LBRACE ImportNames RBRACE IDENT("from") ModulePath
ImportNames   ::= ImportName (COMMA ImportName)* COMMA?
ImportName    ::= IDENT (KW_AS IDENT)?
ModulePath    ::= PathSegment (DOT PathSegment)*
PathSegment   ::= IDENT | WILDCARD | Keyword

Declaration   ::= Attribute* BindingDecl
                | TestDecl
BindingDecl   ::= KW_EXPORT? BindingKw Binder COLON IDENT("type") ASSIGN Type SEMICOLON
                | KW_EXPORT? BindingKw Binder (COLON !IDENT("type") Type)? ASSIGN Initializer SEMICOLON
Initializer   ::= KW_COMPTIME? DeclLit | Expr
BindingKw     ::= KW_CONST | KW_LET | KW_VAR
Binder        ::= IDENT (QUESTION | BANG)? | DestructurePattern
TestDecl      ::= KW_TEST STRING_SQ UseClause? Block

Attribute     ::= ATTR_OPEN IDENT (LPAREN AttrArgs? RPAREN)? RBRACKET
AttrArgs      ::= AttrArg (COMMA AttrArg)* COMMA?
AttrArg       ::= (Expr FAT_ARROW)? Expr
```

### 0.2 Statements

```ebnf
Block         ::= LBRACE BlockItem* RBRACE
BlockItem     ::= Declaration | Statement

Statement     ::= SimpleStmt Modifier? SEMICOLON
                | CompoundStmt
                | DeferStmt
SimpleStmt    ::= Expr
                | KW_RETURN Expr?
                | KW_BREAK
                | KW_CONTINUE
                | KW_YIELD Expr (FAT_ARROW Expr)?
                | KW_YIELD_FROM Expr
CompoundStmt  ::= Block | IfStmt | WhileStmt | ForeachStmt
DeferStmt     ::= KW_DEFER (Block | SimpleStmt) SEMICOLON

Modifier      ::= KW_IF LPAREN Expr RPAREN
                | KW_WHILE LPAREN Expr RPAREN
                | KW_FOREACH LPAREN ForeachBinder KW_IN Expr RPAREN

IfStmt        ::= KW_IF LPAREN Expr RPAREN Block (KW_ELSE (IfStmt | Block))?
WhileStmt     ::= KW_WHILE LPAREN Expr RPAREN Block
ForeachStmt   ::= KW_FOREACH LPAREN ForeachBinder KW_IN Expr RPAREN Block
ForeachBinder ::= BindTarget (FAT_ARROW BindTarget)?
BindTarget    ::= IDENT | WILDCARD | DestructurePattern
```

### 0.3 Expressions — thirteen tiers, loosest first

```ebnf
Expr          ::= Assignment
Assignment    ::= WordPrefix
                | AssignTarget AssignOp Assignment
AssignTarget  ::= (IDENT | WILDCARD) Postfix*
                | DestructurePattern
AssignOp      ::= ASSIGN | PLUS_ASSIGN | MINUS_ASSIGN | STAR_ASSIGN | SLASH_ASSIGN
                | PERCENT_ASSIGN | COALESCE_ASSIGN | NULL_COALESCE_ASSIGN

WordPrefix    ::= WordOp WordPrefix
                | KW_DECLARED IDENT
                | KW_MODULEOF IDENT
                | FnLit
                | Conditional
WordOp        ::= KW_COPY | KW_TRY | KW_SPAWN | KW_AWAIT | KW_COMPTIME | KW_COMPTYPE | KW_THROW

Conditional   ::= Coalesce (QUESTION Coalesce COLON Coalesce)?
Coalesce      ::= Disjunction (CoalesceOp Coalesce)?
CoalesceOp    ::= COALESCE | NULL_COALESCE
Disjunction   ::= Conjunction (OR Conjunction)*
Conjunction   ::= Equality (AND Equality)*
Equality      ::= Comparison ((EQ | NEQ) Comparison)?
Comparison    ::= RangeExpr (CompOp RangeExpr | (KW_IS | KW_AS) Type)?
CompOp        ::= LT | LE | GT | GE
RangeExpr     ::= Additive ((RANGE | RANGE_EXCL) Additive? (KW_BY Additive)?)?
Additive      ::= Multiplicative ((PLUS | MINUS) Multiplicative)*
Multiplicative ::= PrefixExpr ((STAR | SLASH | PERCENT) PrefixExpr)*
PrefixExpr    ::= (BANG | MINUS | AT | AT_AT) PrefixExpr | ApplyExpr
ApplyExpr     ::= PostfixExpr (KW_APPLY ProtoInit)*
PostfixExpr   ::= AMP? Primary Postfix* UseClause?

Postfix       ::= LPAREN ArgList? RPAREN
                | Subscript
                | DOT IDENT
                | OPT_ACCESS IDENT
                | ARROW IDENT
                | OPT_PROTO_ACCESS IDENT
Subscript     ::= LBRACKET RBRACKET
                | LBRACKET Expr RBRACKET
                | LBRACKET Expr? COLON Expr? RBRACKET

ArgList       ::= Arg (COMMA Arg)* COMMA?
Arg           ::= IDENT COLON Expr
                | SPREAD Expr
                | Expr
```

### 0.4 Primary expressions

```ebnf
Primary       ::= Literal
                | IDENT
                | WILDCARD
                | KW_SELF
                | KW_ERROR
                | TableLit
                | VariantLit
                | GenLit
                | MatchExpr
                | TryCatchExpr
                | LPAREN Expr RPAREN

TableLit      ::= LBRACKET TableEntries? RBRACKET
TableEntries  ::= TableEntry (COMMA TableEntry)* COMMA?
TableEntry    ::= Attribute* (Expr FAT_ARROW)? Expr
                | SPREAD Expr
VariantLit    ::= DOT LBRACE VariantName Expr? RBRACE
VariantName   ::= IDENT (DOT IDENT)?

FnLit         ::= KW_FN LPAREN ParamList? RPAREN UseClause? (COLON Type)? FAT_ARROW FnBody
FnBody        ::= Block | Expr
GenLit        ::= KW_GEN UseClause? Block
ParamList     ::= Param (COMMA Param)* COMMA?
Param         ::= AMP? BindTarget QUESTION? (COLON Type)? (ASSIGN Expr)?
                | SPREAD IDENT (COLON Type)?

MatchExpr     ::= MatchKw LPAREN Expr RPAREN LBRACE MatchArms? RBRACE
                | MatchKw LBRACE GuardArms? RBRACE
MatchKw       ::= KW_MATCH | KW_MATCH_BANG
MatchArms     ::= MatchArm (COMMA MatchArm)* COMMA?
MatchArm      ::= Pattern (KW_WHERE Expr)? FAT_ARROW ArmBody
GuardArms     ::= GuardArm (COMMA GuardArm)* COMMA?
GuardArm      ::= Expr FAT_ARROW ArmBody
ArmBody       ::= Block | Expr

TryCatchExpr  ::= KW_TRY Block CatchClause+
CatchClause   ::= KW_CATCH LPAREN CatchBinder RPAREN Block
CatchBinder   ::= (IDENT | WILDCARD) (COLON Type)?
```

### 0.5 Declaration literals

```ebnf
DeclLit       ::= ProtoLit | EnumLit | ErrorLit | ConstraintLit | CapabilityLit | AttributeLit

ProtoLit      ::= KW_PROTO LBRACE ProtoItem* RBRACE
ProtoItem     ::= KW_APPLY IDENT SEMICOLON
                | IDENT("identityEquality") SEMICOLON
                | MemberDecl
MemberDecl    ::= BindingKw Grants IDENT QUESTION? COLON Type (ASSIGN Expr)? SEMICOLON
                | BindingKw Grants IDENT QUESTION? ASSIGN Expr SEMICOLON
Grants        ::= IDENT("get")? IDENT("set")?

EnumLit        ::= KW_ENUM LBRACE VariantDecls? RBRACE
VariantDecls   ::= VariantDecl (COMMA VariantDecl)* COMMA?
VariantDecl    ::= IDENT (COLON VariantPayload)?
VariantPayload ::= PayloadShape | Type
PayloadShape   ::= LBRACKET PayloadFields? RBRACKET
PayloadFields  ::= PayloadField (COMMA PayloadField)* COMMA?
PayloadField   ::= StringLit FAT_ARROW Type

ErrorLit      ::= KW_ERROR (COLON IDENT)? LBRACE ErrorField* RBRACE
ErrorField    ::= IDENT QUESTION? COLON Type SEMICOLON

ConstraintLit ::= KW_CONSTRAINT IDENT COLON Type (KW_WHERE Expr)+
CapabilityLit ::= KW_CAPABILITY (LBRACE CapList RBRACE)?
AttributeLit  ::= KW_ATTRIBUTE LBRACKET AttrParams? RBRACKET
AttrParams    ::= AttrParam (COMMA AttrParam)* COMMA?
AttrParam     ::= StringLit FAT_ARROW Type (ASSIGN Expr)?

UseClause     ::= KW_USE LPAREN CapList RPAREN
CapList       ::= IDENT (COMMA IDENT)* COMMA?
ProtoInit     ::= IDENT (LPAREN InitList? RPAREN)?
InitList      ::= IDENT COLON Expr (COMMA IDENT COLON Expr)* COMMA?
```

### 0.6 Types

```ebnf
Type          ::= FnType | UnionType
FnType        ::= KW_FN LPAREN TypeList? RPAREN COLON Type
TypeList      ::= Type (COMMA Type)* COMMA?
UnionType     ::= IntersectType (BAR IntersectType)*
IntersectType ::= PostfixType (AMP PostfixType)*
PostfixType   ::= PrimaryType (BANG | QUESTION)*
PrimaryType   ::= IDENT
                | AT IDENT
                | KW_SELF
                | KW_ERROR
                | KW_COMPTYPE
                | KW_PROTO
                | KW_NULL
                | KW_UNDEFINED
                | KW_FN
                | EnumLit
                | LPAREN Type RPAREN
```

### 0.7 Patterns

```ebnf
Pattern        ::= AltPattern
AltPattern     ::= PrimaryPattern (BAR PrimaryPattern)*
PrimaryPattern ::= WILDCARD (COLON Type)?
                 | IDENT (COLON Type)?
                 | RangePattern
                 | LiteralPattern
                 | TablePattern
                 | VariantPattern
                 | LPAREN Pattern RPAREN
LiteralPattern ::= MINUS? (IntLit | DOUBLE | KW_INF)
                 | StringLit | KW_TRUE | KW_FALSE | KW_NULL | KW_UNDEFINED | KW_NAN
RangePattern   ::= LiteralPattern (RANGE | RANGE_EXCL) LiteralPattern
TablePattern    ::= LBRACKET TablePatEntries? RBRACKET
TablePatEntries ::= TablePatEntry (COMMA TablePatEntry)* COMMA?
TablePatEntry   ::= (KeyLit FAT_ARROW)? Pattern
                  | SPREAD (IDENT | WILDCARD)
VariantPattern  ::= DOT LBRACE VariantName Pattern? RBRACE

DestructurePattern ::= LBRACKET DestrEntries? RBRACKET
DestrEntries    ::= DestrEntry (COMMA DestrEntry)* COMMA?
DestrEntry      ::= (KeyLit FAT_ARROW)? BindTarget
                  | SPREAD (IDENT | WILDCARD)

KeyLit         ::= StringLit | MINUS? IntLit
```

### 0.8 Literals

```ebnf
Literal       ::= IntLit | DOUBLE | StringLit | BytesLit | RegexLit | CommandLit
                | KW_TRUE | KW_FALSE | KW_NULL | KW_UNDEFINED | KW_NAN | KW_INF
IntLit        ::= INT_DEC | INT_HEX | INT_BIN | INT_OCT
BytesLit      ::= BYTES

StringLit     ::= STRING_SQ
                | STRING_DQ
                | DQ_OPEN DqPiece* DQ_CLOSE
                | TRIPLE_DQ_OPEN DqPiece* TRIPLE_DQ_CLOSE
                | TRIPLE_SQ_OPEN RAW_TEXT* TRIPLE_SQ_CLOSE
DqPiece       ::= DQ_TEXT | ESCAPE_PAIR | DOLLAR_TEXT | INTERP_IDENT | Splice

RegexLit      ::= REGEX | REGEX_OPEN RegexPiece* REGEX_CLOSE
RegexPiece    ::= REGEX_TEXT | ESCAPE_PAIR | DOLLAR_TEXT | Splice

CommandLit    ::= COMMAND | CMD_OPEN CmdPiece* CMD_CLOSE
CmdPiece      ::= CMD_TEXT | ESCAPE_PAIR | DOLLAR_TEXT | Splice

Splice        ::= INTERP_OPEN SPREAD? Expr INTERP_CLOSE
```

### 0.9 The keyword class

`Keyword` is every keyword token, needed in exactly one position: a module path segment
(R252). It is written out rather than abbreviated because §10 asserts its size.

```ebnf
Keyword       ::= KW_VAR | KW_LET | KW_CONST | KW_FN | KW_GEN | KW_CONSTRAINT | KW_PROTO
                | KW_ENUM | KW_ERROR | KW_CAPABILITY | KW_ATTRIBUTE | KW_TEST | KW_EXPORT
                | KW_IMPORT | KW_IF | KW_ELSE | KW_FOREACH | KW_IN | KW_WHILE | KW_BREAK
                | KW_CONTINUE | KW_RETURN | KW_YIELD_FROM | KW_YIELD | KW_MATCH_BANG
                | KW_MATCH | KW_WHERE | KW_DEFER | KW_BY | KW_TRY | KW_CATCH | KW_THROW
                | KW_COPY | KW_SPAWN | KW_AWAIT | KW_COMPTIME | KW_COMPTYPE | KW_IS | KW_AS
                | KW_APPLY | KW_DECLARED | KW_MODULEOF | KW_USE | KW_TRUE | KW_FALSE
                | KW_NULL | KW_UNDEFINED | KW_NAN | KW_INF | KW_SELF
```

---

## 1. File structure and the prelude

A file is a module (modules §1). `Prelude` is the import run, and it is a distinct production
rather than a filter over `TopLevelItem*` because the prelude rule is structural: an `import`
after any other top-level item does not derive, so a misplaced one is a syntax error by
construction (compiler §1.3). §1.2 rejects it from the token stream first (R250), which makes
this a structural invariant rather than a path taken — and not dead code to be tidied away.

**Both cells of modules §5's grid are prelude members** (R250): the statement form and the
assigned form. The assigned form is spelled out here rather than deferred to `BindingDecl`
because at `KW_EXPORT? KW_CONST IDENT ASSIGN` the two readings part only on the token after the
`=` — see §11's flagged list, which records what that costs a parser and what it does not. It is
`KW_CONST` only, and admits no type annotation; modules §6 forbids the annotation too, and that
half is semantic.

`TopLevelItem` admits a `Statement`, and semantic analysis rejects it (R257). The grammar is
permissive here for the reason §9 gives, and the payoff is direct: every corpus example parses
from one start symbol.

**`PathSegment` admits any keyword** (R252). The path never becomes a name, so a segment
collides with nothing, and `test/` and `error/` are ordinary directory names. The lexer is
untouched — it still emits `KW_TEST` — and only this position's view of that token changes.
`WILDCARD` is a segment too, `_` being an ordinary directory name.

**`from` is matched by spelling, not reserved** (R223). After a braced import list the
from-clause is the only legal continuation, so the match is unambiguous and `from` stays usable
as a binding or a module name — `import { parse as from } from m;` parses.

## 2. Declarations

Every declaration is a binding: Luna has no free-standing function declaration, and a named
function is a lambda bound to a name (functions §1). `TestDecl` is the one exception in form —
a test is named by a **string**, so there is nothing to bind — and it is a declaration rather
than a statement because a test is a module-level artifact (tests §1). Its name is
`STRING_SQ` and not any `StringLit`: interpolating a test name would buy nothing and would
break `grep`, which is how a test is found. Its body is mandatory, so `test 'empty' {}` is the
floor and there is no body-less form.

**The declaration literals of §0.5 are primary expressions**, not declaration forms. `proto`,
`enum`, `error`, `constraint`, `capability` and `attribute` all appear as the right-hand side of
a binding, so making them expressions keeps one production for "declare a name" and costs
nothing: each commits on its keyword. That they are legal *only* as a `const` initializer
(R126, R137) is a restriction with no grammatical content — it needs the binding keyword and the
position, both of which semantic analysis has — so it lives there under §9's rule.

**Attributes attach to a declaration or a table-literal entry**, and to nothing else
(attributes §3.1 names exactly two sites). A `test` is neither, which is why `TestDecl` carries
no `Attribute*`. Both sites that do carry it take `Attribute*`, so stacking falls out.

## 3. Statements

The statement grammar's shape is set by the **postfix modifier**, control-flow §5:

- **A modifier takes one `SimpleStmt`**, never a block — the block form is the block form, and
  postfix exists for the one-statement case.
- **Modifiers never chain.** `Modifier?` is one optional slot, so `expr if (c) foreach (h)` does
  not derive (control-flow, the postfix desugar).
- **There is no postfix `else`**, which falls out of `Modifier` naming no `KW_ELSE`: a
  conditional *value* is `match`, `??` or the tier-11 conditional, and an else-bearing
  conditional is the block form (control-flow §4, §5.1).
- **Declarations take no modifier** (R159). `Declaration` and `Statement` are separate
  alternatives of `BlockItem`, and only `Statement` has the slot, so `let x = 5 if (c);` does not
  derive — a conditional declaration is unrepresentable rather than diagnosed.
- **`defer` takes no modifier** (R159), which is why `DeferStmt` sits outside the modifier
  production entirely. This one is a trap rather than nonsense: `defer close(fd) if (cond);`
  would desugar to a defer inside a sugar block, whose exit is immediate. The conditional-cleanup
  idiom is one level in — `defer { close(fd) if (cond); };` — and that derives, because the
  modifier attaches to the inner statement.
**A `{` is always a block, and no rule says so** (R272). `CompoundStmt`'s `Block` is the only
thing that opens with `LBRACE` anywhere a statement, a `defer` body, a `FnBody` or an `ArmBody`
may appear, because a variant literal opens with `DOT LBRACE` (§0.4). There is nothing to
disambiguate and therefore nothing to state: `{}` is an empty block, `{x;}` is a block, and
`.{read}` is a literal wherever an expression may go, body positions included.

This replaces R268, which ruled the collision the other way — the block won, and a variant
literal in any of the four positions was parenthesized, `=> ({read})`. That rule needed a guard
in each of the four productions and an enumerated list of what "wherever a block may appear"
meant, so a fifth block-bearing form added later would have reintroduced the over-accept by
omission. R272 removes the collision instead of adjudicating it, and the four guards went with
it.

**`throw` and assignment are expressions, not statements** (associativity §1, tiers 12 and 13),
so both reach statement position through `SimpleStmt ::= Expr`. **`yield` is a statement and
never an expression** (keywords §2), so it has its own alternative.

**`else` and `if` are two tokens**, not a compound. The precedent for a compound would be
`KW_YIELD_FROM`, which exists so `from` can stay unreserved (R223) and which carries a
documented hazard: the fold is whitespace-only, so a comment defeats it (lexer-testing-plan §3).
`if` is already a keyword, so a compound buys no reservation and would make `else /*c*/ if`
fail to parse.

## 4. Expressions

§0.3 is `associativity.md` §1 written as a stratified grammar: thirteen tiers, thirteen
nonterminals, loosest first. Stratification is R253's ruling, and the argument is the
**non-associative** tiers — 5 (range), 6 (comparison), 7 (equality) and 11 (conditional) — whose
operands are the *next tier down* rather than themselves. `a < b < c` does not derive, and
neither does `a ? b : c ? d : e` (R254). Those rejections are the production shape rather than a
check somebody has to remember to write.

Tier by tier, the notes that are not obvious from the shape:

- **Tier 1 postfix** is one chainable run, so `a->P.b(c)[d]` parses left to right. `ARROW`
  reaches protocol space bare or qualified (`MemberRef`), and `OPT_PROTO_ACCESS` is its
  short-circuiting form.
- **The `use` clause is a tail, not a postfix** (capabilities §5.2): `UseClause?` sits after
  `Postfix*`, which is exactly the rule that no postfix may follow it. `f() use (io).g()` does
  not derive; operators and modifiers compose outside it, as `f() use (io) ?? d` shows.
- **Tier 1a `apply`** takes a complete tier-1 expression on the left and its own closed grammar
  on the right (§7), never an expression, so only the left edge needs a tier. It chains.
- **Tier 5 range** makes `by` part of the production, one per range (range §3), and its right
  operand is optional — `(1..)` is the infinite range (range §5), and a step may still be given.
- **Tier 6** takes a `Type` on the right of `is` / `as`, which is where the type grammar is
  entered from expression position. Everything else there is an expression.
- **Tier 12 word prefixes bind the whole expression below assignment** (associativity §3), which
  is the right-recursion in `WordPrefix`. `declared` and `moduleof` are the degenerate members —
  each takes exactly one binding name, never an expression (type §4, modules §7.1, R261).
- **`FnLit` is a word prefix too** (R264), and that is where its ambiguity went. A literal with
  an *expression* body is greedy — `fn () => n * 2` is `fn () => (n * 2)` — so nothing may
  follow it, exactly as nothing may follow `copy` or `try`. Leaving it in `Primary` gave every
  tier between there and assignment a second reading: `(fn () => n) * 2`, `(fn () => t).count`,
  `(fn () => x) = 5`. Sitting at tier 12 it has no such competitor, and `fn (…) =>` reads as
  what associativity §3 already describes — a prefix taking the entire expression to its right.
  The cost is that an **immediately-invoked literal is parenthesized**, `(fn () => { … })()`,
  which is the same rule the `use` clause carries one production above (capabilities §5.2).

**`AMP` is not a tier, and that is why associativity §1 never lists it.** A reference "exists
only as an argument" (variables §5.1), so `&` is argument-position punctuation rather than an
operator, and §0.3 attaches it to the **base** of a postfix chain — the `AMP?` on
`PostfixExpr` — not to the chain's result.

`&myTable.sort()` is therefore `sort(&myTable)`: the `&` marks the receiver, and UFCS
(functions §3.4) lifts that receiver into the argument slot where a reference belongs. The
alternative — `&` as a tier-2 prefix over the whole chain — would mean a reference to the
*result* of sorting, a temporary, which §5.1 says cannot be formed. `&myStream.map(…)` settles
it independently: §5.1 records that the stream is consumed *for everyone*, which is mutation
through the reference, not a reference to a return value.

Two consequences. `&` no longer stacks with the symbolic prefixes, so `&-x` — a reference to a
negation — does not derive, while `-&x` does and semantics rejects it. And the tree needs no
downstream rewrite: it already says which binding the reference is to, where a tier-2 shape
would have made semantic analysis push the reference inward to find the receiver.

The grammar **cannot** enforce "only as an argument", because `&myTable.sort();` puts the `&` at
statement head and only UFCS lowering moves it into argument position. So `&` is admitted at the
base of any postfix expression and semantics checks the enclosing construct (§9).

## 5. Types

§0.6 is `associativity.md` §2, and tier 0 is grouping. `Type` is entered from exactly four
places, and nowhere else: after a `COLON` in an annotation, parameter, member, field or typed
binder; on the right of `KW_IS` / `KW_AS`; inside a declaration literal that takes one; and
after a `: type` annotation in a binding, which is how a type expression reaches **value**
position at all (R256).

That last one is why `Type` appears in no `Primary` alternative. None of the type grammar's
constructors — `BAR`, `AMP`, postfix `BANG` / `QUESTION`, `FnType` — has an expression
production, so a bare `int | double` in value position derives nothing. `const number: type =
int | double;` parses because the annotation put the right-hand side in type position.

**And the annotation commits** (R269): `: type` before an `=` selects the first `BindingDecl`
production, whose right-hand side is a `Type` and nothing else, which is the rule R256 already
stated in prose — "decided at one token and before the `=` is even reached" (type §2). The
`!IDENT("type")` on the second production is what makes it exclusive, and without it both derive
`const t: type = int;`: one reading a type, the other a value annotated with the type named
`type`. They mean the same thing, which is why the ambiguity is harmless to a reader and fatal to
a parser.

Three forms therefore stop deriving, and each is one the language has a better spelling for. A
**function literal** cannot carry the annotation — `const t: type = fn (a): b => x;` — because a
literal is not a type; drop the annotation and it is an ordinary binding. An **expression that
computes a type** cannot either — `const t: type = comptype x;` — and type §2 already says these
need no annotation, `comptype x` being an expression whose value is a type. And a **type
expression whose first token is `type`** must be parenthesized in an annotation:
`let x: (type | null) = 5;`. That last is the same medicine `{` takes one section up, and for the
same reason: where a leading token is claimed, parentheses are how the other reading is spelled.

This is what retired `TypeOnly`, which existed only to keep the two productions apart by
requiring the alias's right-hand side to carry at least one type operator. It worked, and it put
the deciding token past the whole right-hand side — arbitrarily far right (`enum { a, b } | null`)
and arbitrarily deep (`(a|b)` is a type, `(a)` is an expression). The guard moves the decision
back to the annotation, where R256 put it.

**`KW_FN` begins a literal in expression position and a type in type position**, and never both
in one place, which is what lets `FnLit` commit on its keyword (R45's LL(1) claim, licensed by
R256). Within the type grammar, `KW_FN` still needs one token: `LPAREN` follows for `FnType`,
anything else leaves bare `fn`.

## 6. Patterns

§0.7 is match §2.1, restated over token names. Two properties carry it:

- **A pattern's kind is decided by syntax alone**, one token of lookahead: at an `IDENT` or a
  `WILDCARD`, peek for `COLON`. The type universe is never consulted, so a bare `list` or `count`
  is a binding wherever it appears and no import elsewhere can turn a binding arm into a type
  test.
- **A type appears in a pattern only after `COLON`**, so `BAR` is the union operator inside a
  type and the alternation separator everywhere else. The two never occupy one position, and no
  parenthesization rule is needed to keep them apart.

**Keys in both pattern forms are literals** (`KeyLit`), never expressions. A match pattern is a
*test*, and an expression key would mean evaluating an expression to decide whether an arm
matches — so under first-match-wins, arms above the matching one could evaluate keys as a side
effect of failing. `DestructurePattern` takes the same restriction rather than the freer one it
could carry, because match §4.3 states that a match pattern **is** a destructuring pattern with
literals and type tests added, and the two "differ in exactly one place" — the absent named key.
A second difference would falsify both claims. Table *literals* are unaffected and keep `Expr`
keys: construction is not matching.

**Two positions take a restricted pattern.** A `CatchBinder` is a parenthesized binder and
nothing else (errors §8.3) — the parentheses are required, and they are what terminate the type
before a `{` that would otherwise look like `error { … }`. A `ConstraintLit`'s head is a typed
binder alone (constraints §1), which is why shape and literal patterns are unavailable there and
Luna gains no shape types by the back door.

## 7. The closed sub-grammars

Five forms have an interior that is **not** an expression, and naming them as a category is what
keeps an expressions-everywhere grammar from swallowing them:

- **`ProtoInit`** — the `apply` operator's own grammar (protocols §4.2): a proto name and an
  optional initializer list of `name: value`, never an expression. It reads identically to named
  arguments deliberately; the binding target differs.
- **`UseClause`** — a list of capability names, in both its positions (functions §2.2,
  capabilities §5.2). Names, not expressions.
- **`Arg`'s named form** — `IDENT COLON Expr` (functions §3.3.2). The `IDENT COLON` head is
  what distinguishes it, taken by left-factoring against a plain `Expr` that also begins with
  `IDENT`.
- **`AttrArg`** — an attribute's payload, which is **table-shaped and not argument-shaped**.
  attributes §3 gives two forms: positional (`#[jsonTag('user_name')]`) and keyed
  (`#[route('path' => '/users', 'method' => 'POST')]`, "mirroring table construction"). So the
  keyed spelling is `FAT_ARROW`, not `IDENT COLON`, and `#[jsonTag()]` and `#[jsonTag]` are both
  legal for an all-defaulted attribute. It is narrower than `TableEntry`: no nested attributes,
  no spread, attributes being never dynamic (§3.2).
- **`MemberDecl`, `VariantDecl`, `ErrorField`, `AttrParam`** — the declaration bodies. Each is a
  row list with its own shape, and none is a table literal even where it wears brackets. A
  member's grants are `IDENT("get")? IDENT("set")?` in that order and at most once each:
  protocols §2.2 fixes the canonical order, and there is nothing a repeated grant could mean.

**`PayloadShape` is the sharpest.** An enum variant's payload may be a *shaped table* —
`circle: ['radius' => int]` — and enum §2.3 calls that "**the one place shape-typed tables exist
in Luna**, variant-scoped, never general." So it is a production of `VariantDecl` and
deliberately **not** an alternative of `Type`: adding a bracketed form to the type grammar would
grant shape types everywhere, which is precisely the deferral
`deferred-constructs/shape-type.md` records. The payload may equally be an ordinary type, a
named table constraint included (`circle: circleTable`), so `VariantPayload` offers both and
`LBRACKET` decides at one token.

## 8. Literals with interior structure

A string is **not** a terminal. Since R236 and R239 an interpolating literal reaches the parser
in one of two shapes, and §0.8 admits both:

- **The fast path** is one token — `STRING_DQ`, `REGEX`, `COMMAND` — taken exactly when the
  literal holds no splice, so its span regex applies (lexer §6).
- **The mode path** is a delimited sequence, opener through closer, with content and escape
  tokens between and `Splice` for each `${…}`.

R247's split is a lexer optimization, not two languages: the parser accepts both shapes for the
same construct. The triples have no fast path — a triple always has margins to tokenize — so
they appear only in delimited form. `TRIPLE_SQ_STRING` neither escapes nor interpolates, which
is why its content is `RAW_TEXT` alone, while `TRIPLE_DQ_STRING` interpolates and escapes
exactly as `DQ_STRING` does (lexer §1) and therefore shares `DqPiece`.

**`Splice` admits a leading `SPREAD`** (spread §5, command §3): `${...xs}` is `INTERP_OPEN`
then an ordinary `SPREAD` operator, the splice having already pushed the expression mode. It
needs no production of its own beyond that option.

## 9. What the grammar deliberately admits

**A restriction belongs here only when it is decidable with no symbol knowledge.** Everything
below derives and is rejected later, and each is deliberate — the parser stays regular and the
diagnostic stays precise (associativity §4's rule for `&`, generalized).

| Admitted | Rejected by | Why not here |
|-|-|-|
| `AMP` on a base that is not a `var` binding, or outside a call | semantic analysis (variables §5.1) | "is this a `var`" is symbol knowledge; and the call is only visible after UFCS lowering |
| A `Statement` at `TopLevelItem` | semantic analysis (modules §1, R257) | the check needs no structure, and the grammar's permissiveness is what collapses the corpus to one start symbol |
| `KW_LET` / `KW_VAR` at module level | semantic analysis (variables §4) | module level is `const`-only; positional, but R250 and R257 both put a rule of this shape in semantics for the diagnostic |
| `KW_IMPORT` after the prelude | import validation §1.2 (R250) | it does not derive, and §1.2 aborts first; the rejection is a structural invariant |
| A declaration literal outside a `const` initializer | semantic analysis (R126, R137) | needs the binding keyword, which semantics has |
| A type annotation on an assigned import | semantic analysis (modules §6) | one binding production, one rule elsewhere |
| A non-constant attribute payload | semantic analysis (attributes §3.2) | "comptime-evaluable" needs to know what the callee is |
| A table key that is not a string or an int | semantic analysis (tables §3.1) | no grammar checks a value's type |
| `KW_SELF` outside a proto block | semantic analysis (protocols §2.4) | position is not enough; the enclosing declaration is |
| `x->P.m` as two postfixes rather than one qualified access | semantic analysis (protocols §3.1) | whether `P` names a proto is symbol knowledge; letting the grammar decide made `->a.b` derive two ways |
| `IDENT("get")` / `IDENT("set")` as ordinary names | nothing — they are ordinary names (R232) | recognized positionally in a member head only |
| An empty `File` | nothing | a module with no declarations is legal and useless |

## 10. Production inventory

Every nonterminal this file defines, by owning section. **Names and counts only** — the
productions live in §0 and are not duplicated here — so this table is an index rather than a
second source of truth. Its purpose is mechanical: a count that a test can assert, and a list
that makes an omission visible, exactly as lexer §10 does for tokens.

**129 nonterminals over nine groups.** By §0's grouping: **18** file and declarations, **12**
statements, **24** expressions, **21** primaries, **23** declaration literals and closed
sub-grammars, **7** types, **13** patterns, **10** literals, **1** keyword class.

A guard is not a nonterminal and adds none: `!TERMINAL` names a token, so it answers to the
terminal pin below rather than to this count.

**Every ` ```luna ` block in the corpus derives, and derives exactly once**, which
`internal/ebnf` checks by running §0 over each of them. That is the question associativity
§4 asserted and nothing tested until R264; it is a check rather than a proof, since it speaks
only for the inputs the corpus contains, but it is the strongest statement available and it
failed on 31 blocks before the productions below were straightened.

**Each is defined exactly once**, and `File` is the only one that appears on no right-hand
side — it is the start symbol, and a second such nonterminal would be dead grammar.

**Every terminal named in §0 is a lexer §0 token name**, and the correspondence is asserted both
ways: no production may name a token that does not exist, and §10's own count of `Keyword`'s
alternatives must equal lexer §10's **50**. That second pin is what catches a keyword added to
the lexer and forgotten here, which R252's path-segment rule would silently mis-parse.

**Reachability**: every nonterminal is reachable from `File`, and every one is defined. A
nonterminal that is defined and unreachable is dead grammar; one that is reachable and undefined
is a hole. Both are test failures rather than review questions.

## 11. Error summary (R240)

Every syntax error, with the code that names it. Codes are `P` + four digits, allocated
append-only and never reused (compiler §3.1). Each has a fixed **title**; the description is
per-instance and volatile. Tests pin the code and the primary span, never the prose.

**One code is allocated, and nine rows wait with an em dash.** R250 and R251 established the rule
this follows: a code allocated before there is an implementation to raise it and a test to pin it
is a code nothing checks — and worse, a published code is a promise the compiler can produce it.
`P0001` is `expect`'s, §7.2 layer 1 being the only recovery that needs no judgement and therefore
the only one a parser without recovery can raise. The other nine take their numbers as the checks
that raise them land.

R272 is why the rule earns its keep rather than merely sounding careful: it retired one of §11.2's
named rules outright, by changing the syntax so the situation cannot arise. Under R240's
never-reuse rule, a number allocated eagerly would now be burned on a diagnostic the language
cannot produce.

What is already fixed, ahead of the numbers:

- **The parser is error-tolerant** (compiler §1.3): it accumulates and produces a best-effort
  partial tree. The batch driver discards it and aborts at the phase boundary; the tooling
  drivers consume it (tooling §3).
- **`P` covers syntax only.** A rule that needs a symbol table is an `S` (compiler §3.1), and
  §9's table is the list of things that therefore are not `P` codes.
- **Recovery points are an implementation concern**, not a grammar one, and belong with the
  parser rather than here.

### 11.1 How large the space is, measured (R267)

The question "what must the parser diagnose" has a mechanical answer, and it has three of them,
because it is three questions. Each number below is derived from §0 by `internal/ebnf`, not
estimated.

| Enumeration | Count | What it answers |
|-|-|-|
| **Dot positions** | 1,119 over 548 desugared productions | where a parser can *be* when it fails. A true bound and a useless one: most dots share a message. |
| **Committed expect-sites** | 275, over **31 distinct terminals** | where the parser writes `expect(X)` — a dot past position 0 with a terminal next. This is the inventory of required tokens. |
| **Recursion sites** | 418, over 170 nonterminals | where a nonterminal is predicted, so failure is reported one level down as "expected a *thing*". |
| **Frontier classes** | **50** | the distinct answers to "what may come next", over every prefix of every four-token program (12,558 of them). |

A **guard** occupies none of these positions. `!TERMINAL` consumes nothing, so it is no dot a
parser rests at and no token it can be missing; it is folded into the dispatch that chooses the
production, which is the point of having it.

Ten terminals carry 222 of the 275 expect-sites — `COLON` 39, `RPAREN` 26, `IDENT` 24,
`SEMICOLON` 22, `ASSIGN` 21, `LPAREN` 21, `FAT_ARROW` 20, `RBRACKET` 18, `RBRACE` 16,
`COMMA` 15 — and twelve terminals are wanted at exactly one site, where a message can name its
construct with no ambiguity.

**The frontier is bimodal, and that is what shapes the diagnostics.** Its sizes run 1, 2, 4, 5,
6 — then a gap — then 11, 14, 15, 26 … 63, with the mass at 26–63. Only three frontiers are
singletons (`IDENT`, `SEMICOLON`, `STRING_SQ`). Below the gap the expected set **is** the
message ("expected `;`"); above it the set is sixty-odd ways to begin an expression, and
printing it is noise where naming the *nonterminal* is not. The two halves of §11.2's engine
table are those two modes.

**Every figure moved with R268–R272, and the directions are worth recording.** R269 dropped the
widest frontier from 84 to 62, `const t: type =` no longer having both a type and an expression
live after it. R272 then added four expect-sites and four dot positions — the `LBRACE` in
`DOT LBRACE …` is a required terminal past position 0, twice over in `VariantLit` and
`VariantPattern` — and put one frontier class back, at 63. That is the whole parsing cost of
`.{}`: four expect-sites, in exchange for the four guards R268 needed and the named diagnostic
§11.2 no longer carries.

**What the grammar cannot supply, and the parser must.** A frontier is the union over every live
item; a recursive-descent parser has a **stack**, and the stack is where the good message lives.
The grammar knows `RPAREN` is expected — only the parser knows *which* `(` is unclosed, which is
the secondary span that makes the diagnostic worth reading. Nor can the grammar choose whom to
blame: at the end of `let x = 1` the frontier holds `SEMICOLON` and every operator, and "missing
`;`" is the parser preferring the innermost incomplete production. So this section sizes and
bounds the space; it does not write the messages.

What it *does* buy is a completeness check, and one worth building: for every valid prefix, the
token a parser's diagnostic names must appear in the grammar's frontier there. A parser that
invents an expectation §0 does not have is caught by construction — the same shape as the
reject-set invariant the parse goldens carry (`oracle/parser/testdata/golden.md`).

### 11.2 The diagnostics, by title

Titles are fixed and are part of a code's identity (compiler §3.1); descriptions are
per-instance. **Numbers are allocated when the parser raises the check and a test pins it**, in
the order that happens, so this table is deliberately unnumbered and deliberately not in
allocation order.

The split is between an **engine** — five diagnostics that cover every one of the 1,140 sites —
and the **named rules**, each of which is a place the language deliberately excludes something
and a spec sentence says why. A rule with a reason earns a page; the engine's five do the rest.

**The engine.**

| Code | Title | Raised when | Frontier |
|-|-|-|-|
| — | Unexpected token | a token appears where §0 admits none, and no single construct was under way | large |
| — | Unexpected end of input | input ended with a production incomplete | any |
| — | Unclosed delimiter | a committed `(`, `[` or `{` never met its closer; the opener is a **secondary span** | small |
| `P0001` | Missing token | a required terminal is absent; the description names it (the 275 expect-sites) | small |
| — | Expected a construct | a nonterminal was predicted and nothing could begin it; the description names which — expression, type, pattern, statement, declaration (the 418 recursion sites) | large |

**The named rules.** Each is unrepresentable in §0 on purpose, so the generic message would be
technically right and useless — `set get` reports "expected `:`", which does not name the actual
mistake.

There were six until R272. The sixth was *"variant literal here needs parentheses"*, and it is
gone because the situation it explained no longer arises: `.{read}` collides with nothing, so
there is no mistake to name. A named rule retired by a syntax change rather than by a better
message is the outcome to prefer — R267 chose these five because a rule with a reason earns a
page, and the reason there was a collision the language has since stopped having.

| Code | Title | The rule | Owner |
|-|-|-|-|
| — | Declaration with a postfix modifier | `let x = 5 if (c);` would scope the binding inside the sugar block, where it is provably unusable | control-flow §5.1 |
| — | `defer` with a postfix modifier | `defer close(fd) if (c);` would run the cleanup **immediately**, at the sugar block's exit | control-flow §5.1 |
| — | `else` on a postfix form | an else-bearing conditional is the block form | control-flow §5.1 |
| — | Chained postfix modifiers | `expr if (c) foreach (h)` poses the which-nests-which trap | control-flow §5 |
| — | Grant order must be `get set` | one spelling, so `set get` does not derive | protocols §2.2 |

Two boundaries the count depends on, both already ruled elsewhere. **Import-after-the-prelude is
an `M`, not a `P`**: §1.2 rejects it before parsing (R250), so the structural invariant §1 records
never raises a code here. And the corpus's 204 "compile error" sites are overwhelmingly `S` — §9's
table is the explicit list of what the grammar admits and semantics rejects, which is to say the
list of things that are *not* on this page.

---

## Flagged: hazards and the corners

**Two junctions need more than one token, and they need different amounts** (R271). The LL(1)
claims elsewhere in this file are otherwise unqualified, and the earlier version of this note
claimed there was one such place, which writing the parser disproved.

**The prelude junction needs two tokens.** At `KW_EXPORT? KW_CONST IDENT ASSIGN`, the parser must
see whether `KW_IMPORT` follows to know whether it is in `PreludeItem` or `BindingDecl`. R250
predicted this in as many words — "the stopping condition is a parse decision rather than a token
test: `const` and `export` need lookahead to `= import`". It left-factors cleanly as a grammar,
though a parser may prefer not to: the two shapes are not prefix-identical in the *tree*, a
`PreludeItem` holding a bare `IDENT` where a `BindingDecl` holds a `Binder`, so factoring costs a
seam inside `BindingDecl` that a five-token predicate does not
(`oracle/parser/parser-implementation.md` §4.7).

**The assignment junction needs a bracket-matched scan, and no fixed number of tokens.**
`Assignment ::= WordPrefix | AssignTarget AssignOp Assignment` has two alternatives that both
begin with `IDENT` or `WILDCARD` and stay identical for as long as the target runs:
`a.b[c](d).e = 1` against `a.b[c](d).e + 1` part at the token after the whole postfix chain,
which is unbounded. It is decidable and cheap — the prefix `(IDENT | WILDCARD) Postfix*` is
recognized exactly by a scan that matches `()[]{}` and then asks whether an `AssignOp` follows,
consuming nothing — and it is decidable *exactly*, since `AssignOp` appears in no other
production. This is the ordinary gap between LL and LR rather than a defect in §0: an LR parser
needs none of it, and nothing here is ambiguous. It is recorded because the claim above used to
say it did not exist.

**The collisions this section used to adjudicate are settled in §0 rather than here.** `: type`
before an `=` puts the right-hand side in type position, which R270's guard states in the
production (R269, §5). The `{` collision is gone outright: a variant literal opens `.{` (R272,
§0.4), so `LBRACE` in an expression position derives nothing and there is no choice to make.
Both were prose in this section, and prose is exactly the form in which a rule over-accepts —
the grammar derived `fn () => {read}` and `const t: type = int;` under readings the language does
not have.

**`&&` versus `&` after `is`.** In type position `AMP` is intersection, so `x is int & y` is the
type `int & y`, not a conjunction. `x is int && y` is safe by maximal munch — `AND` is one token
and cannot extend a type — so the type ends there. Decided, not ambiguous, and worth knowing.

**`KW_ERROR` is a `Primary` and a `PrimaryType` and heads `ErrorLit`**, which is three roles
for one token and needs one token to separate them. In type position it is the root type
(`catch (e: error)`, `r is error`). In expression position, peek: `LBRACE` or `COLON` opens
`ErrorLit`; anything else is the root error type as a value or callee — `throw error;` and
`throw error('disk full')` (errors §5.2). Nothing collides at that junction, because a named
argument's head is an `IDENT` and `KW_ERROR` is not one.

**`Subscript`'s empty form is the bytes append target** (`b[] = 65`, bytes §3), which is why
`LBRACKET RBRACKET` derives. It is meaningless anywhere else and semantic analysis says so.

**`VariantLit` and `Block` no longer share a first token, and that is a ruling rather than a
happy fact** (R272). A variant literal opens `DOT LBRACE` — `.{read}`, `.{circle ['radius' =>
5]}`, `.{hand.north}` — so `LBRACE` in an expression position derives nothing and `FnBody ::=
Block | Expr` is decided by one token with no rule attached.

**The history is worth keeping, because it cost three rulings to get here.** `FnBody ::= Block |
Expr` was written as an alternation and annotated in prose as an ordered choice, "the only one in
this file". A CFG has no ordered choice, so §0 *derived* `fn () => {read}` as a variant literal
while the prose called it a block: the grammar accepted an input the language rejects, at the one
site this section flagged, and a parser obeying the prose would have diagnosed an input the
grammar derives — breaking the reject-set invariant the parse goldens rest on
(`oracle/parser/testdata/golden.md` §0). R268 closed the gap by ruling for the block in all four
positions and parenthesizing the literal, with R270's `!LBRACE` guard stating it inside §0. R272
went further and removed the collision: a rule that must *enumerate* the places a block may
appear is a rule that a later block-bearing form silently breaks, and `.{}` needs no list.

What the dot costs is one character and two adjacency corners, below.

**A match arm's body is `Block | Expr` too**, and the reason survives the change: functions §3
pins the `{`-opens-a-block rule and says outright that it "governs match arms", which only holds
if an arm may carry a block at all. `ArmBody` is therefore `FnBody`'s twin. What an arm with a
block body *yields* is `undefined` — a block has no value (R254), and match §9 already admits
`undefined` into a match's result type, so nothing new enters the type system by it.

**`.{` has two adjacency corners, and both are maximal munch rather than grammar** (R272). The
lexer takes the longest token, so a `.` that abuts a preceding operator is eaten by it:

- **`c ?.{on} : .{off}` does not parse** — `?.` is `OPT_ACCESS`, which wants an `IDENT`. The
  conditional is written `c ? .{on} : .{off}`, with the space. This is the one worth knowing,
  since a ternary over two variants is an ordinary thing to write; the diagnostic should name
  `?.` rather than report a missing identifier.
- **`1...{b}` does not parse** — `...` is `SPREAD`. The range is written `1 .. .{b}`, and nobody
  writes a range ending in a variant anyway.

Neither is ambiguous and neither mis-parses: both are rejections, and a space fixes each. `??` is
unaffected — `x ??.{fallback}` parses, `COALESCE` being two characters that do not reach the dot.

## Cross-reference notes: gaps found, and their resolutions

Writing and then auditing this file surfaced ten things. Nine are fixed; the tenth is recorded
against the spec that owns it.

- **The type grammar had no grouping production.** `associativity.md` §2 listed five tiers and
  no parenthesization, while the overview wrote `(@proto1 & @proto2) | null` and tier 5's own
  note instructed parenthesizing a `fn` type to embed it in a union. Fixed there as tier 0, and
  `PrimaryType` carries it here.
- **`is` / `as` take a whole `Type`**, which is why `v is int | string` (is §2) needs no
  parentheses: `BAR` binds inside the type. The parenthesized spelling is legal and redundant.
- **The prelude is two productions, not a filter.** Stating it as `Prelude` ahead of
  `TopLevelItem*` is what makes a misplaced import fail to derive, which compiler §1.3 asserts
  and nothing previously wrote down.
- **`error` was missing from `Primary`** in the first draft — a token with three roles that is
  easy to enumerate as two, caught by tracing `throw error('wrong count')`.
- **The enum payload shape is scoped, and saying so is load-bearing.** Putting a bracketed form
  in `Type` to make the enum parse would have granted shape types across the whole language,
  undoing a deferral (`shape-type.md`) by accident. It became `PayloadShape` under `VariantDecl`.
- **The corpus had one bracketed type**, `awaitAny`'s `[int, any]!` (concurrency §5.1) — that
  same non-existent form used in earnest, corrected to `list!` with the pair named in the
  comment, as channels §1 already spells the identical shape of value.
- **`moduleof` was an operator with no token** (R261): specified in modules §7.1 as the sibling
  of `declared`, and present in none of keywords.md §3, lexer §0, or operators.md §0. It is a
  keyword now, the fiftieth.
- **`&` was placed as a prefix tier and is not one.** The first draft made it tier 2, over the
  whole postfix chain, which yields a reference to the chain's *result*. §4 has the correction
  and the reasoning; the short version is that associativity §1's silence about `&` was right,
  because `&` is not an operator.
- **An attribute's payload is table-shaped, not argument-shaped.** The first draft reused
  `ArgList`, so it accepted `#[route(path: '/users')]` — a spelling the language does not have —
  and rejected `#[route('path' => '/users')]`, which attributes §3 gives verbatim.
- **Eight list productions made their separator optional** (R263). `TableEntry`, `MatchArm`,
  `GuardArm`, `VariantDecl`, `PayloadField`, `AttrParam`, `TablePatEntry` and `DestrEntry` each
  carried a trailing `COMMA?` while appearing inside an `Item*` repetition, which admits the
  comma-less list: `[10 20]`, `enum { a b c }`, `match (x) { 1 => "a" 2 => "b" }` all derived.
  Seven other lists already used the correct `Item (COMMA Item)* COMMA?` shape, so the file
  carried two spellings for one job and one of them was wrong. It is a **wrong-accept**, not an
  ambiguity, so the corpus gate would have stayed green over it — found instead by scoping the
  spec-literal parser, which is the argument for building that tool before trusting this file.
- **Capture cannot reach module scope, and the spec says it does.** functions §2.1 says a
  function captures "every copyable binding it references" as a deep-`const` snapshot at
  closure-creation time, with no scope qualifier. Applied to module level that breaks
  **recursion**: `const walk = fn (…) => { … yield from walk(c); … }` (stream §1.5) would
  capture a `walk` that does not exist yet, and enum §2.2 relies on the same property ("a
  recursive function's name is in its body"). Module-level bindings must therefore be
  *referenced*, not captured. This is a `functions.md` correction, not a grammar one, and it is
  recorded here because writing the grammar is what surfaced it.
