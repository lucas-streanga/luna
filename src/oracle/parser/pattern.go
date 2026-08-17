package parser

// grammar.md **§0.7 — Patterns**, thirteen productions:
//
//	Pattern · AltPattern · PrimaryPattern · LiteralPattern · RangePattern · TablePattern ·
//	TablePatEntries · TablePatEntry · VariantPattern · DestructurePattern · DestrEntries ·
//	DestrEntry · KeyLit
//
// `Pattern` is a pure alternation and `AltPattern` and `PrimaryPattern` are tiers, so all three
// elide when they pass through and only `AltPattern` survives, when a `BAR` fires it.
//
// **This group lands before match.** `MatchArm ::= Pattern (KW_WHERE Expr)? FAT_ARROW ArmBody`, so
// primary.go's `matchExpr` has nothing to parse until `pattern` exists.
//
// Two sub-grammars, not one, and they are not interchangeable: `DestructurePattern` is a **binding**
// form reached from `Binder`, `BindTarget` and `AssignTarget`, and `TablePattern` is a **matching**
// form reached from `PrimaryPattern`. They share a bracket and nothing else — `DestrEntry` ends in a
// `BindTarget` where `TablePatEntry` ends in a `Pattern` — which is why an expression already
// parsed as a `TableLit` cannot be renamed into either (expr.go's `assignment`).

// pattern is `Pattern ::= AltPattern`, the entry every match arm names.
func (p *parser) pattern() {
	panic("parser: pattern is unimplemented")
}

// destructurePattern is `DestructurePattern ::= LBRACKET DestrEntries? RBRACKET`, with
// `DestrEntry ::= (KeyLit FAT_ARROW)? BindTarget | SPREAD (IDENT | WILDCARD)`. It is the bracketed
// binding form of `Binder`, `BindTarget` and `AssignTarget` (`destructuring-binder.parse`).
func (p *parser) destructurePattern() {
	panic("parser: destructurePattern is unimplemented")
}
