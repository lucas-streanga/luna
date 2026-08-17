package parser

// grammar.md **§0.2 — Statements**, twelve productions:
//
//	Block · BlockItem · Statement · SimpleStmt · CompoundStmt · DeferStmt · Modifier · IfStmt ·
//	WhileStmt · ForeachStmt · ForeachBinder · BindTarget
//
// `BlockItem`, `CompoundStmt` and `BindTarget` are pure alternations and never reach a tree (§5),
// so `Block` prints a `Statement` over an `IfStmt` with nothing between them, and a `foreach`
// binder that is a bare `IDENT` prints the token under `ForeachBinder`. `Statement` and
// `SimpleStmt` are **not** in that set and print even over a single child —
// `declared-and-moduleof.parse` shows both.
//
// `block` is the group's busiest export: `TestDecl` (decl.go), `GenLit`, `FnBody`, `ArmBody` and
// `CatchClause` (primary.go) and `DeferStmt` all take one. `statement` is reached from
// `TopLevelItem` and `BlockItem`, and `bindTarget` from `Param` (primary.go) and `AssignTarget`'s
// destructuring form.
//
// Two of R268's four `!LBRACE` guards are here — on `Statement`'s simple-statement arm and on
// `DeferStmt`'s — and each is one `p.at(token.LBrace)` at the head of the function it guards.
// The other two are `FnBody` and `ArmBody` in primary.go.

// block is `Block ::= LBRACE BlockItem* RBRACE`. The loop needs `atEnd` beside the closing brace:
// a file that ends inside a block must leave the loop, or the parse does not terminate (§6.4).
func (p *parser) block() {
	panic("parser: block is unimplemented")
}

// statement is
// `Statement ::= SimpleStmt Modifier? SEMICOLON | CompoundStmt | DeferStmt`.
//
// **The dispatch is on one token, `LBRACE` included** (R268). `CompoundStmt`'s `Block` and
// `Primary`'s `VariantLit` both begin there, and §0 now settles it in the production itself — the
// `!LBRACE` guard on this alternative — so a `{` at a statement head opens a block and a variant
// literal there is parenthesized, `({read});`. Before the guard both readings derived and neither
// the grammar nor this function could say which was meant.
//
// The postfix `Modifier` is sugar the parser keeps (§9): `x = 5 if (c);` stays a `Statement` with a
// `Modifier` child, and §11.2's named rules — a declaration or a `defer` carrying one, an `else` on
// one, two of them chained — are messages Phase 3 selects, not shapes in the tree (§6.3).
func (p *parser) statement() {
	panic("parser: statement is unimplemented")
}

// simpleStmt is
// `SimpleStmt ::= Expr | KW_RETURN Expr? | KW_BREAK | KW_CONTINUE | KW_YIELD Expr (FAT_ARROW Expr)?
// | KW_YIELD_FROM Expr`.
//
// It opens over a single `Expr` child, which is the tier spine's entry from statement position and
// the shape all three of the spine's goldens have under `Statement`.
func (p *parser) simpleStmt() {
	panic("parser: simpleStmt is unimplemented")
}

// bindTarget is `BindTarget ::= IDENT | WILDCARD | DestructurePattern`, a pure alternation: it
// dispatches, opens nothing, and its bracket form is pattern.go's.
func (p *parser) bindTarget() {
	panic("parser: bindTarget is unimplemented")
}
