package parser

// grammar.md **§0.2 — Statements**.
//
// `BlockItem`, `CompoundStmt` and `BindTarget` are pure alternations and never reach a tree (§5),
// so a `Block` prints a `Statement` over an `IfStmt` with nothing between. `Statement` and
// `SimpleStmt` are **not** in that set and print even over a single child.
//
// This group carried two of R268's four `!LBRACE` guards. R272 deleted all four by giving the
// variant literal its own opener, so `LBRACE` here means `Block` and nothing else.

// block's loop needs `atEnd` beside the closing brace, or a file ending inside a block does not
// terminate (§6.4).
func (p *parser) block() {
	panic("parser: block is unimplemented")
}

// statement keeps the postfix `Modifier` as sugar (§9); §11.2's named rules — a declaration or a
// `defer` carrying one, an `else` on one, two chained — are messages Phase 3 selects, not shapes
// in the tree (§6.3).
func (p *parser) statement() {
	panic("parser: statement is unimplemented")
}

func (p *parser) simpleStmt() {
	panic("parser: simpleStmt is unimplemented")
}

func (p *parser) bindTarget() {
	panic("parser: bindTarget is unimplemented")
}
