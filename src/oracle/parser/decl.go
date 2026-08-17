package parser

// grammar.md **§0.1 — File and declarations**, eighteen productions:
//
//	File · Prelude · PreludeItem · TopLevelItem · ImportSpec · ImportNames · ImportName ·
//	ModulePath · PathSegment · Declaration · BindingDecl · Initializer · BindingKw · Binder ·
//	TestDecl · Attribute · AttrArgs · AttrArg
//
// `TopLevelItem`, `PathSegment` and `BindingKw` are pure alternations: they dispatch and never
// reach a tree (§5), so `topLevelItem` opens nothing and the other two are token tests inside
// `modulePath` and `bindingDecl`. Everything else here opens its node.
//
// The group's cross-boundary callers are `declaration` (from `BlockItem`, stmt.go) and `attribute`
// (from `TableEntry`, primary.go); it reaches out to `block` and `bindTarget` (stmt.go), `expr`
// (expr.go), `typ` and `typeOnly` (type.go), `declLit` and `useClause` (decllit.go), and
// `destructurePattern` (pattern.go).

// file is `File ::= Prelude TopLevelItem*`, the start symbol and the parse's only unconditional
// open: a file of nothing but comments derives no tokens at all, and `File` still has to be there
// for splice to fill (§6.1). It is also the one node whose span is the whole file, its leading and
// trailing trivia having nowhere further out to go (§2.1).
func (p *parser) file() {
	panic("parser: file is unimplemented")
}

// prelude is `Prelude ::= PreludeItem*`, and it is a node rather than a filter: an import after
// any other top-level item does not derive, so a misplaced one is a syntax error by construction
// (grammar.md §1). A file with no imports opens it and closes it empty, and splice deletes it —
// which is why `empty-forms.parse` has no `Prelude` line and `import-forms.parse` does.
//
// `Prelude` was counted a pure alternation until a tree was built from the goldens; §5 has the
// correction and the reason its single symbol stands for any number of children.
func (p *parser) prelude() {
	panic("parser: prelude is unimplemented")
}

// assignedImportAhead reports whether the cursor begins the assigned import form,
// `KW_EXPORT? KW_CONST IDENT ASSIGN KW_IMPORT ImportSpec SEMICOLON` — modules §5's second cell,
// which R250 put in the prelude and grammar.md spells out here rather than deferring to
// `BindingDecl`.
//
// **This is the junction grammar.md's Flagged list names**, and it is decided here by a predicate
// that peeks at most five tokens and consumes nothing, rather than by left-factoring the two
// productions and naming the run afterwards. The factoring reads free and is not:
//
//   - The two shapes are **not prefix-identical in the tree**. `PreludeItem` holds a bare `IDENT`
//     where `BindingDecl` holds a `Binder` around one, and `KW_IMPORT ImportSpec` where
//     `BindingDecl` holds an `Initializer`. `assigned-import-lookahead.parse` prints both. So the
//     factored parser has to `precede` a `Binder` as well as the item, which is only correct while
//     the decision lands *before* `ASSIGN` is consumed and the `IDENT` is still the last event.
//   - It would put the seam **inside** the most-edited production in the file. `BindingDecl` would
//     no longer parse its own beginning; `KW_EXPORT? BindingKw Binder` would live in the caller,
//     and every other caller of `declaration` would need a second entry point or the same split.
//     §4 wants a function per nonterminal that opens, consumes and closes.
//   - Its **failure mode is worse**. On `export const x =` at end of input the factored parser has
//     already committed to a shape and must invent one; the predicate simply fails — `nth(4)` is
//     no token — and the input is a `BindingDecl` missing its initializer, which is both the more
//     common reading and the better diagnostic.
//
// What it costs is that the peek is four or five tokens where the factored form needs two. That is
// a depth, not a buffer: `nth` is a scan over a cursor that already exists (cursor.go), so nothing
// is materialized and nothing is consumed. grammar.md's "two tokens" is a statement about the
// grammar after left-factoring, not an instruction about where the parser puts its seam — the same
// file rules that recovery points are the implementation's business (§11).
func (p *parser) assignedImportAhead() bool {
	panic("parser: assignedImportAhead is unimplemented")
}

// topLevelItem is `TopLevelItem ::= Declaration | Statement`, a pure alternation, so it opens
// nothing and dispatches. A bare `Statement` derives at top level and is rejected by semantic
// analysis rather than here (§9, R257), so the dispatch is on what can begin a `Declaration` —
// `ATTR_OPEN`, a `BindingKw`, `KW_TEST`, and `KW_EXPORT` before a `BindingKw` — with everything
// else a statement.
func (p *parser) topLevelItem() {
	panic("parser: topLevelItem is unimplemented")
}

// declaration is `Declaration ::= Attribute* BindingDecl | TestDecl`. It opens: only its two
// alternatives are single symbols, and the first is two.
//
// `BindingDecl`'s two forms are decided at the **annotation** and nowhere else (R269): a
// `COLON IDENT("type") ASSIGN` selects the type-alias form, whose right-hand side is a `Type`,
// and the `!IDENT("type")` guard on the other production is what makes the choice exclusive.
// Three fixed tokens, no scan, and the right-hand side is never consulted —
// `type-in-value-position.parse` pins five cases, including the parenthesized annotation
// `let x: (type | null) = 5;` the rule requires.
func (p *parser) declaration() {
	panic("parser: declaration is unimplemented")
}

// attribute is `Attribute ::= ATTR_OPEN IDENT (LPAREN AttrArgs? RPAREN)? RBRACKET`. It is reached
// from `Declaration` here and from `TableEntry` in primary.go, the payload being table-shaped
// rather than argument-shaped (attributes §3).
func (p *parser) attribute() {
	panic("parser: attribute is unimplemented")
}
