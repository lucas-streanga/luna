package parser

// grammar.md **§0.7 — Patterns**.
//
// `Pattern` is a pure alternation and `AltPattern` and `PrimaryPattern` are tiers, so only
// `AltPattern` survives, and only when a `BAR` fires it.
//
// **This group lands before match**, which cannot parse an arm without it.
//
// Two sub-grammars, not one, and they are not interchangeable: `DestructurePattern` is a
// **binding** form reached from `Binder`, `BindTarget` and `AssignTarget`, while `TablePattern` is
// a **matching** form under `PrimaryPattern`. They share a bracket and nothing else — which is why
// an expression already parsed as a `TableLit` cannot be renamed into either (§4.7.1).

func (p *parser) pattern() {
	panic("parser: pattern is unimplemented")
}

func (p *parser) destructurePattern() {
	panic("parser: destructurePattern is unimplemented")
}
