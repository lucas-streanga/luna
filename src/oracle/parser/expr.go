package parser

// grammar.md **§0.3 — Expressions**, twenty-four productions:
//
//	Expr · Assignment · AssignTarget · AssignOp · WordPrefix · WordOp · Conditional · Coalesce ·
//	CoalesceOp · Disjunction · Conjunction · Equality · Comparison · CompOp · RangeExpr ·
//	Additive · Multiplicative · PrefixExpr · ApplyExpr · PostfixExpr · Postfix · Subscript ·
//	ArgList · Arg
//
// **The tier spine is sixteen tiers, loosest first** — `Expr`, `Assignment`, `WordPrefix`,
// `Conditional`, `Coalesce`, `Disjunction`, `Conjunction`, `Equality`, `Comparison`, `RangeExpr`,
// `Additive`, `Multiplicative`, `PrefixExpr`, `ApplyExpr`, `PostfixExpr` and `Primary`, the last of
// them in primary.go with the forms it dispatches to. §0.3's own heading says thirteen and so does
// golden.md §4; §4.9 has the count, what thirteen is counting, and why nothing here corrects it.
//
// **A tier opens its node only when its operator fires**, and that single rule is what makes the
// tree literally equal a golden's tree section: `1` is sixteen tiers deep in the grammar and one
// leaf in the tree, `a + b` is an `Additive`, `x is T` a `Comparison`. `Expr` is a pure alternation
// and never survives; the other fifteen survive exactly when they fire (§5).
//
// The infix and postfix tiers get there with `mark` and `precede` — parse the tier below into a
// remembered position, and wrap the run only once an operator arrives:
//
//	m := p.mark()
//	p.multiplicative()
//	if !p.at(token.Plus) && !p.at(token.Minus) {
//		return
//	}
//	w := p.precede(m, Additive)
//	for p.at(token.Plus) || p.at(token.Minus) {
//		p.bump()
//		p.multiplicative()
//	}
//	p.complete(w)
//
// **The prefix tiers are the exception and use `open`**: `WordPrefix` and `PrefixExpr` are decided
// by their own first token, so there is nothing to decide retroactively. `PostfixExpr` is both —
// `AMP` decides it up front, a `Postfix` or a `UseClause` decides it afterwards.
//
// **Four of the twenty-four get no function.** `AssignOp`, `WordOp`, `CoalesceOp` and `CompOp` are
// pure alternations over terminals: their names never reach a tree (§5), and the operator token is
// what carries the distinction (§8.2). A tier tests the token set and bumps.
//
// **The spine is not self-contained**, and these are the seams the later groups fill:
//
//	Assignment  → DestructurePattern                          (§0.7, pattern.go)
//	WordPrefix  → FnLit                                       (§0.4, primary.go)
//	Comparison  → Type                                        (§0.6, type.go)
//	ApplyExpr   → ProtoInit                                   (§0.5, decllit.go)
//	PostfixExpr → UseClause                                   (§0.5, decllit.go)
//	Primary     → Literal (§0.8) · TableLit · VariantLit · GenLit · MatchExpr · TryCatchExpr

// expr is `Expr ::= Assignment`, a pure alternation: it never reaches a tree and exists so that
// every caller wanting "an expression" names one thing.
func (p *parser) expr() {
	panic("parser: expr is unimplemented")
}

// assignment is `Assignment ::= WordPrefix | AssignTarget AssignOp Assignment`, right-associative
// through its own recursion, and **the junction no fixed lookahead decides**.
//
// Both alternatives can begin with `IDENT` or `WILDCARD`, and they stay identical for as long as
// the target runs: `a.b[c](d).e = 1` against `a.b[c](d).e + 1` differ at the token after the whole
// postfix chain. grammar.md's Flagged list names it (R271) and leaves §0 alone, which is right:
// nothing here is ambiguous and an LR parser would need no help. It is the LL/LR gap, and this is
// the side of it that pays.
//
// `assignTargetAhead` is what pays it — a **token-level scan** that consumes nothing and produces
// nothing, so the two productions stay whole and §7.3's "no backtracking anywhere" holds.
//
// **Rejected: parse the expression, then rename the node.** It looks free — the wrapper is the
// only difference for `_ = x`, where the tiers all collapse and `precede` supplies an
// `AssignTarget` around a bare `WILDCARD`. It breaks on `[a, b] = t`, where the expression branch
// has already built `TableLit → TableEntries → TableEntry` and the target wants
// `DestructurePattern → DestrEntries → DestrEntry`: a different subtree over the same tokens, not
// a different name for the same one. `destructuring-binder.parse` and `spread-forms.parse` print
// the two shapes side by side.
func (p *parser) assignment() {
	panic("parser: assignment is unimplemented")
}

// assignTargetAhead reports whether the cursor begins an `AssignTarget` followed by an `AssignOp`.
// It walks `(IDENT | WILDCARD) Postfix*` or a bracketed `DestructurePattern` over the token kinds
// alone, counting brackets to step over a subscript or an argument list, and stops at the first
// token that cannot continue a target.
//
// It is unbounded in the input and bounded by the statement, which is the same shape §7.2 layer 3
// gives recovery: a linear pass that matches `()[]{}` over the token stream. Whether the two share
// one scaffold is Phase 3's to decide, once the scaffold exists.
func (p *parser) assignTargetAhead() bool {
	panic("parser: assignTargetAhead is unimplemented")
}

// assignTarget is `AssignTarget ::= (IDENT | WILDCARD) Postfix* | DestructurePattern`. It is not a
// pure alternation, so it opens and prints even over a single child — `AssignTarget` over a bare
// `WILDCARD` is what all three of the spine's goldens begin with.
func (p *parser) assignTarget() {
	panic("parser: assignTarget is unimplemented")
}

// wordPrefix is
// `WordPrefix ::= WordOp WordPrefix | KW_DECLARED IDENT | KW_MODULEOF IDENT | FnLit | Conditional`.
//
// A prefix tier: the leading token decides it, so it opens rather than precedes, and it recurses on
// itself for `await try f()`. `KW_DECLARED` and `KW_MODULEOF` are the degenerate members (R158,
// R261) — each takes exactly one binding name and never a general expression — which is why
// `declared-and-moduleof.parse` shows `WordPrefix` over a keyword and a bare `IDENT` with no tier
// beneath it.
func (p *parser) wordPrefix() {
	panic("parser: wordPrefix is unimplemented")
}

// conditional is `Conditional ::= Coalesce (QUESTION Coalesce COLON Coalesce)?`. Non-chainable by
// production shape: `a ? b : c ? d : e` does not derive (R254), so there is no loop and no
// associativity to choose. The `?` is position-resolved — the ternary here, the optional marker in
// a declaration, the `T?` suffix in type position (`ternary-vs-optional-type.parse`).
func (p *parser) conditional() {
	panic("parser: conditional is unimplemented")
}

// coalesce is `Coalesce ::= Disjunction (CoalesceOp Coalesce)?`, right-associative through its own
// recursion rather than a loop, so `a ?? b ?? c` is `a ?? (b ?? c)`.
func (p *parser) coalesce() {
	panic("parser: coalesce is unimplemented")
}

// disjunction is `Disjunction ::= Conjunction (OR Conjunction)*`, left-associative, one node over
// the whole run.
func (p *parser) disjunction() {
	panic("parser: disjunction is unimplemented")
}

// conjunction is `Conjunction ::= Equality (AND Equality)*`. `AND` is one token, so it cannot
// extend a type: `x is int && y` ends the type at `int` and splits here, where `x is int & y` is
// an intersection inside the type (`is-intersection-vs-and.parse`, grammar.md Flagged).
func (p *parser) conjunction() {
	panic("parser: conjunction is unimplemented")
}

// equality is `Equality ::= Comparison ((EQ | NEQ) Comparison)?`, non-chainable by shape.
func (p *parser) equality() {
	panic("parser: equality is unimplemented")
}

// comparison is `Comparison ::= RangeExpr (CompOp RangeExpr | (KW_IS | KW_AS) Type)?`,
// non-chainable, and the spine's reach into type.go: `is` and `as` take a whole `Type`, which is
// why `v is int | string` needs no parentheses.
func (p *parser) comparison() {
	panic("parser: comparison is unimplemented")
}

// rangeExpr is `RangeExpr ::= Additive ((RANGE | RANGE_EXCL) Additive? (KW_BY Additive)?)?`.
// Non-chainable, and both tails are optional: `a..`, `a..b`, `a..b by c` and `a.. by c` all derive
// (`range-by.parse`).
func (p *parser) rangeExpr() {
	panic("parser: rangeExpr is unimplemented")
}

// additive is `Additive ::= Multiplicative ((PLUS | MINUS) Multiplicative)*`.
func (p *parser) additive() {
	panic("parser: additive is unimplemented")
}

// multiplicative is `Multiplicative ::= PrefixExpr ((STAR | SLASH | PERCENT) PrefixExpr)*`.
func (p *parser) multiplicative() {
	panic("parser: multiplicative is unimplemented")
}

// prefixExpr is `PrefixExpr ::= (BANG | MINUS | AT | AT_AT) PrefixExpr | ApplyExpr`. A prefix
// tier: it opens on its own first token and recurses, so `!-x` stacks right. The `!` here is
// logical not; the postfix `T!` is type.go's, and only position separates them.
func (p *parser) prefixExpr() {
	panic("parser: prefixExpr is unimplemented")
}

// applyExpr is `ApplyExpr ::= PostfixExpr (KW_APPLY ProtoInit)*`, left-associative. The right side
// is the operator's own closed grammar — a proto name and an optional initializer list, never an
// expression (protocols §4.2) — so only the left edge is a tier and `ProtoInit` is decllit.go's.
func (p *parser) applyExpr() {
	panic("parser: applyExpr is unimplemented")
}

// postfixExpr is `PostfixExpr ::= AMP? Primary Postfix* UseClause?`, and it is the tier that
// decides both ways: `AMP` fires it before `Primary` is parsed, a `Postfix` or a `UseClause` fires
// it after. `AMP` binds the **base** rather than the postfix chain — the first draft made it a
// prefix tier over the whole chain, which yields a reference to the chain's result
// (`amp-binds-the-base.parse`, grammar.md's cross-reference notes).
func (p *parser) postfixExpr() {
	panic("parser: postfixExpr is unimplemented")
}

// postfix is
// `Postfix ::= LPAREN ArgList? RPAREN | Subscript | DOT IDENT | OPT_ACCESS IDENT | ARROW IDENT |
// OPT_PROTO_ACCESS IDENT`.
//
// One kind for six access forms, told apart by the leading token alone (§8.2), which is why
// punctuation cannot be filtered out of the AST view. `x->P.m` is **two** postfixes rather than one
// qualified access: whether `P` names a proto is symbol knowledge, and letting the grammar decide
// made `->a.b` derive two ways (§9).
func (p *parser) postfix() {
	panic("parser: postfix is unimplemented")
}

// subscript is
// `Subscript ::= LBRACKET RBRACKET | LBRACKET Expr RBRACKET | LBRACKET Expr? COLON Expr? RBRACKET`.
// The empty form is the bytes append target `b[] = 65` (bytes §3); the slice form has both bounds
// optional. Three forms, one token of lookahead each after the bracket.
func (p *parser) subscript() {
	panic("parser: subscript is unimplemented")
}

// argList is `ArgList ::= Arg (COMMA Arg)* COMMA?`, the shape every comma-separated list in §0 has
// since R263: the separator is on the list, never on the item, or `f(a b)` derives.
func (p *parser) argList() {
	panic("parser: argList is unimplemented")
}

// arg is `Arg ::= IDENT COLON Expr | SPREAD Expr | Expr`. The named form is decided at one token —
// an `IDENT` followed by a `COLON` — which is what keeps it out of the expression grammar
// (functions §3.3.2).
func (p *parser) arg() {
	panic("parser: arg is unimplemented")
}
