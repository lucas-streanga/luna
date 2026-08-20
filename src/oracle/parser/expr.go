package parser

// grammar.md **§0.3: Expressions**, and the tier spine (§4.8).
//
// **A tier opens its node only when its operator fires**, which is the single rule that makes the
// tree literally equal a golden's tree section: `1` is many tiers deep in the grammar and one
// leaf in the tree. Infix and postfix tiers reach that with mark/precede (§4.6); the prefix tiers,
// `WordPrefix` and `PrefixExpr`, are decided by their own first token and so open eagerly.
// `PostfixExpr` is both: `AMP` decides it up front, a `Postfix` or `UseClause` afterwards.
//
// `AssignOp`, `WordOp`, `CoalesceOp` and `CompOp` get **no function**: they are pure alternations
// over terminals, so a tier tests a token set and bumps, and the operator token is what carries
// the distinction downstream (§8.2).
//
// §0.3's heading and the tier count it actually defines disagree; §4.9 has the arithmetic and
// why nothing here corrects it.
//
// **The spine is not self-contained.** `Assignment` reaches `DestructurePattern`, `WordPrefix`
// reaches `FnLit`, `Comparison` a `Type`, `ApplyExpr` a `ProtoInit`, `PostfixExpr` a `UseClause`,
// and `Primary` the literals and the delimited forms: the seams later groups fill.

func (p *parser) expr() {
	panic("parser: expr is unimplemented")
}

// assignment is the junction no fixed lookahead decides: `WordPrefix` and `AssignTarget` both
// begin with `IDENT` or `WILDCARD` and stay identical until the token past the whole target:
// `a.b[c](d).e = 1` against `a.b[c](d).e + 1`. Not a defect in §0 but the ordinary LL/LR gap,
// recorded in Flagged by R271; §4.7.1 has the reasoning and the rejected cover grammar.
func (p *parser) assignment() {
	panic("parser: assignment is unimplemented")
}

// assignTargetAhead scans `(IDENT | WILDCARD) Postfix*` or a bracketed pattern over token kinds
// alone, counting brackets, and consumes nothing, so the two productions stay whole and §7.3's
// "no backtracking anywhere" holds. It is exact in both directions, `AssignOp` appearing in no
// other production. Whether it and §7.2 layer 3's bracket scaffold are one thing is Phase 3's.
func (p *parser) assignTargetAhead() bool {
	panic("parser: assignTargetAhead is unimplemented")
}

// assignTarget is not a pure alternation, so it prints even over a single child: `AssignTarget`
// over a bare `WILDCARD` is what all three of the spine's goldens begin with.
func (p *parser) assignTarget() {
	panic("parser: assignTarget is unimplemented")
}

// wordPrefix recurses on itself for `await try f()`. `declared` and `moduleof` are the degenerate
// members (R158, R261): one binding name each, never an expression.
func (p *parser) wordPrefix() {
	panic("parser: wordPrefix is unimplemented")
}

// conditional is non-chainable by production shape, `a ? b : c ? d : e` not deriving (R254), so
// there is no loop and no associativity to choose.
func (p *parser) conditional() {
	panic("parser: conditional is unimplemented")
}

// coalesce is right-associative through its own recursion rather than a loop.
func (p *parser) coalesce() {
	panic("parser: coalesce is unimplemented")
}

func (p *parser) disjunction() {
	panic("parser: disjunction is unimplemented")
}

// conjunction is where `x is int && y` splits, `AND` being one token that cannot extend a type
// where `x is int & y` stays an intersection (`is-intersection-vs-and.parse`).
func (p *parser) conjunction() {
	panic("parser: conjunction is unimplemented")
}

func (p *parser) equality() {
	panic("parser: equality is unimplemented")
}

// comparison is the spine's entry into the type grammar: `is` and `as` take a whole `Type`, which
// is why `v is int | string` needs no parentheses.
func (p *parser) comparison() {
	panic("parser: comparison is unimplemented")
}

// rangeExpr has both tails optional, so `a..`, `a..b`, `a..b by c` and `a.. by c` all derive
// (`range-by.parse`).
func (p *parser) rangeExpr() {
	panic("parser: rangeExpr is unimplemented")
}

func (p *parser) additive() {
	panic("parser: additive is unimplemented")
}

func (p *parser) multiplicative() {
	panic("parser: multiplicative is unimplemented")
}

// prefixExpr stacks right, and its `!` is logical not; the postfix `T!` is type.go's, separated
// by position alone.
func (p *parser) prefixExpr() {
	panic("parser: prefixExpr is unimplemented")
}

// applyExpr takes the operator's own closed grammar on the right, never an expression (protocols
// §4.2), so only its left edge is a tier.
func (p *parser) applyExpr() {
	panic("parser: applyExpr is unimplemented")
}

// postfixExpr binds `AMP` to the **base** rather than the postfix chain: over the chain it would
// yield a reference to the chain's result (`amp-binds-the-base.parse`, §4's cross-reference notes).
func (p *parser) postfixExpr() {
	panic("parser: postfixExpr is unimplemented")
}

// postfix is one kind for six access forms, told apart by the leading token alone, which is why
// punctuation cannot be filtered out of the AST view (§8.2). `x->P.m` is **two** postfixes, since
// whether `P` names a proto is symbol knowledge (§9).
func (p *parser) postfix() {
	panic("parser: postfix is unimplemented")
}

// subscript's empty form is the bytes append target `b[] = 65` (bytes §3).
func (p *parser) subscript() {
	panic("parser: subscript is unimplemented")
}

// argList carries the separator on the list, never on the item, or `f(a b)` derives (R263).
func (p *parser) argList() {
	panic("parser: argList is unimplemented")
}

// arg decides its named form at one token, `IDENT` then `COLON`, which is what keeps named
// arguments out of the expression grammar (functions §3.3.2).
func (p *parser) arg() {
	panic("parser: arg is unimplemented")
}
