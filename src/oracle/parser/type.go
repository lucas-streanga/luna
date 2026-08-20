package parser

// grammar.md **§0.6: Types**.
//
// `UnionType`, `IntersectType`, `PostfixType` and `PrimaryType` are tiers and elide when they pass
// through. **`Type` is a pure alternation that survives anyway**, the one override in the elision
// rule (§5): eliding it leaves a bare `IDENT` in type position indistinguishable from an
// expression's, which is the distinction R256 exists to make.

// typ is spelled short because `type` is a Go keyword; it is the only §0 name that does not
// survive transliteration.
//
// What ends a type is what cannot continue one, so `x is int && y` splits where `x is int & y`
// does not. It is also `BindingDecl`'s alias right-hand side since R269, which retired the
// `TypeOnly` this group used to carry. That production required a type operator, which kept the
// two binding forms apart at the cost of putting the deciding token past the whole right-hand
// side, where no bounded lookahead reached it.
func (p *parser) typ() {
	panic("parser: typ is unimplemented")
}
