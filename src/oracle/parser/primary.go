package parser

// grammar.md **§0.4 — Primary expressions**, twenty-one productions:
//
//	Primary · TableLit · TableEntries · TableEntry · VariantLit · VariantName · FnLit · FnBody ·
//	GenLit · ParamList · Param · MatchExpr · MatchKw · MatchArms · MatchArm · GuardArms ·
//	GuardArm · ArmBody · TryCatchExpr · CatchClause · CatchBinder
//
// `FnBody`, `ArmBody` and `MatchKw` are pure alternations and never reach a tree (§5): a block body
// shows a `Block` and an expression body shows the expression, and `match` against `match!` is the
// token. `Primary` is a tier — elided when it passes through, kept when it fires, which it does
// only for `LPAREN Expr RPAREN` (`iife-parenthesized.parse`).
//
// **Patterns land before `match`**: `MatchArm ::= Pattern (KW_WHERE Expr)? FAT_ARROW ArmBody`, so
// `matchExpr` has nothing to parse until pattern.go exists.

// primary is
// `Primary ::= Literal | IDENT | WILDCARD | KW_SELF | KW_ERROR | TableLit | VariantLit | GenLit |
// MatchExpr | TryCatchExpr | LPAREN Expr RPAREN`.
//
// The bottom of the tier spine and its widest dispatch. One junction needs a token of lookahead
// and grammar.md's Flagged list settles it: **`KW_ERROR` has three roles** — a `Primary`, a
// `PrimaryType`, and the head of an `ErrorLit`. In expression position, `LBRACE` or `COLON` after
// it opens an `ErrorLit`; anything else is the root error type as a value or a callee, which is
// `throw error;` against `throw error('disk full')`. Nothing collides there, a named argument's
// head being an `IDENT` and `KW_ERROR` not being one (`error-three-roles.parse`).
func (p *parser) primary() {
	panic("parser: primary is unimplemented")
}

// tableLit is `TableLit ::= LBRACKET TableEntries? RBRACKET`, with
// `TableEntry ::= Attribute* (Expr FAT_ARROW)? Expr | SPREAD Expr`. The key form is decided by
// scanning for the `FAT_ARROW`, not by the key's shape: a table key that is not a string or an int
// is admitted here and rejected by semantic analysis (§9).
func (p *parser) tableLit() {
	panic("parser: tableLit is unimplemented")
}

// variantLit is `VariantLit ::= DOT LBRACE VariantName Expr? RBRACE`, and the leading `DOT` is the
// whole of R272: it shares no first token with `Block`, so `.{read}` is a literal wherever an
// expression may go and `{` opens a block wherever a block may appear, with nothing to decide.
// `variant-literal-dot-brace.parse` prints it in all four body positions, qualified, with a
// payload, as a pattern, and in a ternary.
//
// It took three rulings to get here. `FnBody ::= Block | Expr` was prose-annotated as an "ordered
// choice" a CFG cannot express, so §0 derived `=> {read}` as a literal while the prose called it a
// block; R268 ruled for the block and parenthesized the literal; R270 stated that in §0 with a
// guard; R272 removed the collision instead, and the guards went with it.
func (p *parser) variantLit() {
	panic("parser: variantLit is unimplemented")
}

// fnLit is
// `FnLit ::= KW_FN LPAREN ParamList? RPAREN UseClause? (COLON Type)? FAT_ARROW FnBody`, reached
// from `WordPrefix` rather than from `Primary` — it binds looser than every operator below the
// word prefixes, so `fn () => a + b` takes the whole sum as its body.
//
// `KW_FN` also begins a `FnType` in type position (`fn (int): bool`), which has no `FAT_ARROW`.
// Position decides, and where it does not — `const t: type = fn (int): bool;` — the junction is
// decl.go's flagged one.
func (p *parser) fnLit() {
	panic("parser: fnLit is unimplemented")
}

// genLit is `GenLit ::= KW_GEN UseClause? Block`.
func (p *parser) genLit() {
	panic("parser: genLit is unimplemented")
}

// matchExpr is
// `MatchExpr ::= MatchKw LPAREN Expr RPAREN LBRACE MatchArms? RBRACE | MatchKw LBRACE GuardArms?
// RBRACE`.
//
// Left-factored on the keyword and decided by the token after it: `LPAREN` is the scrutinee form
// whose arms are `Pattern`s, `LBRACE` the guard form whose arms are `Expr`s. `match!` is the same
// two shapes under a different token (`match-forms.parse`).
func (p *parser) matchExpr() {
	panic("parser: matchExpr is unimplemented")
}

// tryCatchExpr is `TryCatchExpr ::= KW_TRY Block CatchClause+`, and `KW_TRY` is also a `WordOp`, so
// the two are told apart by the token after it: a `LBRACE` is this, anything else is the word
// prefix over an expression.
func (p *parser) tryCatchExpr() {
	panic("parser: tryCatchExpr is unimplemented")
}
