package parser

// grammar.md **§0.8 — Literals**.
//
// `Literal`, `IntLit`, `BytesLit`, `DqPiece`, `RegexPiece` and `CmdPiece` are pure alternations
// and never reach a tree (§5): a decimal integer is an `INT_DEC` leaf with nothing over it.
//
// **This is where the grammar becomes one connected component**: a `Splice` holds an `Expr`, so a
// string reaches the whole expression language and it reaches back. The lexer has already done the
// hard half (R236's modes), so the parser only loops over pieces.

func (p *parser) literal() {
	panic("parser: literal is unimplemented")
}

// stringLit survives even over a single `STRING_SQ`, and has to: `KeyLit` and `PayloadField` name
// it positionally, so the node is what says a string is in key position rather than value
// position.
func (p *parser) stringLit() {
	panic("parser: stringLit is unimplemented")
}
