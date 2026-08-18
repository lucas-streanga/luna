package parser

import "luna/oracle/token"

// grammar.md **§0.9 — The keyword class**, one production, needed in one position: a module path
// segment, so that `import moduleof.x;` derives (R252). It is a pure alternation and so is the
// `PathSegment` naming it, so neither reaches a tree and neither gets a function — §0.9 is a token
// set, and `modulePath` is the only thing that reads it.
//
// The file exists so that a reader looking for the ninth group finds it saying why there is
// nothing here.

// isPathSegment asks the token's category rather than listing §0.9's fifty, which is safe because
// `internal/ebnf` already pins that every terminal §0 names is a lexer §0 token and that §0.9's
// count equals lexer §10's fifty (§10). A keyword added to the lexer and forgotten in §0.9 fails
// there, so duplicating the list here would only add a second thing to forget.
func isPathSegment(k token.Kind) bool {
	panic("parser: isPathSegment is unimplemented")
}
