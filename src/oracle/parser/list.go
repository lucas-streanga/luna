package parser

import "luna/oracle/token"

// commaList is §0's one list shape, `Item (COMMA Item)* COMMA?`, written once.
//
// Fifteen productions carry it over sixteen sites, and each hand-written copy would be another
// chance to admit `[10 20]` — which is exactly the defect R263 had to fix in the grammar, in eight
// productions at once. One implementation is one place to get the trailing comma right.
//
// **The closer is a parameter because `CapList` has two of them** — `RPAREN` under a `UseClause`
// and `RBRACE` under a `CapabilityLit` — so it cannot be a property of the list. It is also what
// separates a trailing comma from a missing item, and what stops the loop at end of input.
//
// **Optionality stays with the caller**, because that is where §0 puts it: the `?` in
// `TableEntries?` is on `TableLit`, not on the list. So a caller writes the guard its production
// shows, and the two lists §0 does *not* mark optional — `ImportNames` and `CapList` — get no
// guard and report an empty list as the error it is.
//
// Termination needs no argument beyond the loop: every iteration past the first consumes a comma,
// so the list advances even when item consumes nothing (§6.4).
func (p *parser) commaList(k Kind, closer token.Kind, item func()) {
	panic("parser: commaList is unimplemented")
}
