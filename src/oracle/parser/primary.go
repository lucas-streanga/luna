package parser

// grammar.md **§0.4 — Primary expressions**.
//
// `FnBody`, `ArmBody` and `MatchKw` are pure alternations and never reach a tree (§5): a block
// body shows a `Block`, an expression body shows the expression, and `match` against `match!` is
// the token. `Primary` is a tier, kept only when it fires — for `LPAREN Expr RPAREN`.
//
// **Patterns land before match**: a `MatchArm` begins with a `Pattern`, so `matchExpr` has
// nothing to parse until pattern.go exists.

// primary is the spine's widest dispatch, and one junction needs a token of lookahead: **`error`
// has three roles** — a `Primary`, a `PrimaryType`, and the head of an `ErrorLit`. In expression
// position `LBRACE` or `COLON` after it opens the literal, anything else is the root error type
// as a value or callee (`error-three-roles.parse`). Nothing collides, a named argument's head
// being an `IDENT` and `KW_ERROR` not being one.
func (p *parser) primary() {
	panic("parser: primary is unimplemented")
}

// tableLit decides an entry's key form by scanning for the `FAT_ARROW`, not by the key's shape: a
// key that is not a string or an int is admitted here and rejected by semantics (§9).
func (p *parser) tableLit() {
	panic("parser: tableLit is unimplemented")
}

// variantLit opens `DOT LBRACE`, and that leading dot is the whole of R272: sharing no first token
// with `Block`, it needs no parentheses in a body position and no guard anywhere. It took three
// rulings — `FnBody ::= Block | Expr` was prose-annotated as an ordered choice a CFG cannot
// express, R268 ruled for the block and parenthesized the literal, R270 stated that in §0 with a
// guard, and R272 removed the collision instead.
func (p *parser) variantLit() {
	panic("parser: variantLit is unimplemented")
}

// fnLit is reached from `WordPrefix`, not `Primary`: it binds looser than every operator below the
// word prefixes, so `fn () => a + b` takes the whole sum as its body. `KW_FN` also begins a
// `FnType` in type position, which has no `FAT_ARROW`.
func (p *parser) fnLit() {
	panic("parser: fnLit is unimplemented")
}

func (p *parser) genLit() {
	panic("parser: genLit is unimplemented")
}

// matchExpr is left-factored on the keyword and decided by the token after it: `LPAREN` is the
// scrutinee form, whose arms are patterns, and `LBRACE` the guard form, whose arms are
// expressions (`match-forms.parse`).
func (p *parser) matchExpr() {
	panic("parser: matchExpr is unimplemented")
}

// tryCatchExpr shares `KW_TRY` with a `WordOp`, separated by the token after it: `LBRACE` is this,
// anything else is the word prefix over an expression.
func (p *parser) tryCatchExpr() {
	panic("parser: tryCatchExpr is unimplemented")
}
