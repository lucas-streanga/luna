package parser

// grammar.md **§0.5 — Declaration literals and the closed sub-grammars**.
//
// `DeclLit` and `VariantPayload` are pure alternations and never reach a tree (§5), so an
// `Initializer` shows the `ProtoLit` or `EnumLit` directly.
//
// A declaration literal outside a `const` initializer derives and semantics rejects it (§9, R126,
// R137): the check needs the binding keyword, which the grammar does not have.

func (p *parser) declLit() {
	panic("parser: declLit is unimplemented")
}

// enumLit is reached from `DeclLit` and from `PrimaryType`, an enum being a type literal as well
// as a declaration literal. Its payload shape is scoped to `VariantDecl` rather than living in
// `Type`, or a bracketed form would be granted across the whole language and undo a deferral by
// accident (§4's cross-reference notes).
func (p *parser) enumLit() {
	panic("parser: enumLit is unimplemented")
}

// useClause has two meanings for one production: the declaration.s own capabilities on a `FnLit`,
// `GenLit` or `TestDecl`, and the call-site delegation clause on a `PostfixExpr` (R112).
func (p *parser) useClause() {
	panic("parser: useClause is unimplemented")
}

// protoInit is the `apply` operator's own closed grammar — a proto name and an optional
// initializer list, never an expression (protocols §4.2).
func (p *parser) protoInit() {
	panic("parser: protoInit is unimplemented")
}
