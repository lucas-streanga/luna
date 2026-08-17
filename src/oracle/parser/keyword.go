package parser

import "luna/oracle/token"

// grammar.md **§0.9 — The keyword class**, one production:
//
//	Keyword ::= KW_VAR | KW_LET | … | KW_SELF
//
// Fifty alternatives, every one of them a terminal, needed in exactly one position: a module path
// segment, so that `import moduleof.x;` derives (R252). It is a pure alternation and never reaches
// a tree, and so is the `PathSegment` that names it — `import-forms.parse` prints a `ModulePath`
// over bare `IDENT` and `KW_MODULEOF` leaves.
//
// It therefore gets **no parse function**: written out, §0.9 is a token set, and the one place that
// reads it is `modulePath` deciding whether to bump. It gets its own file because §0 groups it
// separately and §10 counts it, so a reader looking for the ninth group finds it saying why there
// is nothing here rather than finding nothing.

// isPathSegment reports whether a token may be a `PathSegment` — `IDENT`, `WILDCARD`, or any
// keyword.
//
// It asks the token's own category rather than listing §0.9's fifty, and that is safe because the
// correspondence is already pinned twice over: `internal/ebnf` asserts that every terminal §0 names
// is a lexer §0 token, and that §0.9's own count equals lexer §10's fifty (grammar.md §10). A
// keyword added to the lexer and forgotten in §0.9 fails there, which is the check that would
// otherwise have to be duplicated here.
func isPathSegment(k token.Kind) bool {
	panic("parser: isPathSegment is unimplemented")
}
