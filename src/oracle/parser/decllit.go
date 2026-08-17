package parser

// grammar.md **§0.5 — Declaration literals and the closed sub-grammars**, twenty-three
// productions:
//
//	DeclLit · ProtoLit · ProtoItem · MemberDecl · Grants · EnumLit · VariantDecls · VariantDecl ·
//	VariantPayload · PayloadShape · PayloadFields · PayloadField · ErrorLit · ErrorField ·
//	ConstraintLit · CapabilityLit · AttributeLit · AttrParams · AttrParam · UseClause · CapList ·
//	ProtoInit · InitList
//
// `DeclLit` and `VariantPayload` are pure alternations and never reach a tree (§5), so an
// `Initializer` shows the `ProtoLit` or the `EnumLit` directly.
//
// A declaration literal outside a `const` initializer derives and is rejected by semantic analysis
// (§9, R126, R137): the check needs the binding keyword, which semantics has and the grammar does
// not. The parser therefore reaches `declLit` from `Initializer` alone and does not police it.

// declLit is
// `DeclLit ::= ProtoLit | EnumLit | ErrorLit | ConstraintLit | CapabilityLit | AttributeLit`, a
// pure alternation over six keywords: it dispatches on one token and opens nothing.
func (p *parser) declLit() {
	panic("parser: declLit is unimplemented")
}

// enumLit is `EnumLit ::= KW_ENUM LBRACE VariantDecls? RBRACE`, reached from `DeclLit` here and
// from `PrimaryType` in type.go — an enum is a type literal as well as a declaration literal.
//
// Its payload shape is deliberately scoped: `PayloadShape ::= LBRACKET PayloadFields? RBRACKET`
// lives under `VariantDecl` rather than in `Type`, or a bracketed form would be granted across the
// whole language and undo a deferral by accident (grammar.md's cross-reference notes,
// `enum-payload-shape.parse`).
func (p *parser) enumLit() {
	panic("parser: enumLit is unimplemented")
}

// useClause is `UseClause ::= KW_USE LPAREN CapList RPAREN`, the capability list a function names
// (capabilities §3.1). Reached from `PostfixExpr` as the call-site delegation clause (R112) and
// from `FnLit`, `GenLit` and `TestDecl` as the declaration's own.
func (p *parser) useClause() {
	panic("parser: useClause is unimplemented")
}

// protoInit is `ProtoInit ::= IDENT (LPAREN InitList? RPAREN)?`, the right side of the `apply`
// operator and a closed grammar rather than an expression: a proto name and an optional
// initializer list, never a general expression (protocols §4.2, `apply-operator.parse`).
func (p *parser) protoInit() {
	panic("parser: protoInit is unimplemented")
}
