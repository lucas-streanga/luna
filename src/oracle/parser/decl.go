package parser

// grammar.md **§0.1: File and declarations**.
//
// `TopLevelItem`, `PathSegment` and `BindingKw` are pure alternations that never reach a tree
// (§5), so they dispatch rather than open; the last two are token tests inside their callers.

// file is the parse's only unconditional open: a file of nothing but comments derives no tokens
// at all, and `File` still has to be there for splice to fill (§6.1). It is also the one node
// whose span is the whole file, its outer trivia having nowhere further to go (§2.1).
func (p *parser) file() {
	panic("parser: file is unimplemented")
}

// prelude is a node rather than a filter, because an import after any other top-level item does
// not derive and a misplaced one is therefore a syntax error by construction (grammar §1). A file
// with no imports opens it empty and splice deletes it, which is why `empty-forms.parse` has no
// `Prelude` line and `import-forms.parse` does.
func (p *parser) prelude() {
	panic("parser: prelude is unimplemented")
}

// assignedImportAhead decides grammar.md's flagged prelude junction with a predicate that peeks at
// most five tokens and consumes nothing, rather than by left-factoring `PreludeItem` against
// `BindingDecl`. §4.7 has the argument; the short version is that the two are not
// prefix-identical in the *tree*, a bare `IDENT` against a `Binder`, so factoring costs a seam
// inside `BindingDecl` and fails worse at end of input.
func (p *parser) assignedImportAhead() bool {
	panic("parser: assignedImportAhead is unimplemented")
}

// topLevelItem dispatches on what can begin a `Declaration`; everything else is a statement. A
// bare `Statement` derives here and semantic analysis rejects it (§9, R257).
func (p *parser) topLevelItem() {
	panic("parser: topLevelItem is unimplemented")
}

// declaration selects `BindingDecl`'s type-alias form at the **annotation** and nowhere else
// (R269): `COLON IDENT("type") ASSIGN`, three fixed tokens, with the `!IDENT("type")` guard on the
// other production making the choice exclusive. The right-hand side is never consulted.
func (p *parser) declaration() {
	panic("parser: declaration is unimplemented")
}

// attribute takes a **table-shaped** payload, not an argument-shaped one (attributes §3), which
// is why `TableEntry` can carry one too.
func (p *parser) attribute() {
	panic("parser: attribute is unimplemented")
}
