package parser

// grammar.md **§0.8 — Literals**, ten productions:
//
//	Literal · IntLit · BytesLit · StringLit · DqPiece · RegexLit · RegexPiece · CommandLit ·
//	CmdPiece · Splice
//
// Six of the ten are pure alternations and never reach a tree (§5): `Literal`, `IntLit`,
// `BytesLit`, `DqPiece`, `RegexPiece`, `CmdPiece`. A decimal integer is an `INT_DEC` leaf with
// nothing over it, and an interpolation piece is the token it is. `StringLit`, `RegexLit`,
// `CommandLit` and `Splice` survive.
//
// **This is where the grammar stops being a tree of tokens and becomes one connected component**:
// `Splice ::= INTERP_OPEN SPREAD? Expr INTERP_CLOSE`, so a string reaches the whole expression
// language and the whole expression language reaches back (`interpolation-splice.parse`). The lexer
// has already done the hard half — R236's modes emit `DQ_OPEN`, `DQ_TEXT`, `INTERP_OPEN` and their
// kin as ordinary tokens — so the parser only loops over pieces.

// literal is
// `Literal ::= IntLit | DOUBLE | StringLit | BytesLit | RegexLit | CommandLit | KW_TRUE | KW_FALSE
// | KW_NULL | KW_UNDEFINED | KW_NAN | KW_INF`, a pure alternation: it dispatches on one token and
// opens nothing.
func (p *parser) literal() {
	panic("parser: literal is unimplemented")
}

// stringLit is
// `StringLit ::= STRING_SQ | STRING_DQ | DQ_OPEN DqPiece* DQ_CLOSE | TRIPLE_DQ_OPEN DqPiece*
// TRIPLE_DQ_CLOSE | TRIPLE_SQ_OPEN RAW_TEXT* TRIPLE_SQ_CLOSE`.
//
// It survives even over a single `STRING_SQ`, and has to: `KeyLit` and `PayloadField` name it
// positionally, so the node is what says a string is in key position rather than value position.
// Reached from `Literal`, from `KeyLit` and `LiteralPattern` (pattern.go), and from `PayloadField`
// and `AttrParam` (decllit.go).
func (p *parser) stringLit() {
	panic("parser: stringLit is unimplemented")
}
