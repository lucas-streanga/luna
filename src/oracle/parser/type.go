package parser

// grammar.md **§0.6 — Types**, seven productions:
//
//	Type · FnType · TypeList · UnionType · IntersectType · PostfixType · PrimaryType
//
// Four are tiers and elide when they pass through: `UnionType`, `IntersectType`, `PostfixType`,
// `PrimaryType`. **`Type` is a pure alternation and survives anyway** — the one override in the
// elision rule (§5, golden.md §2). Eliding it would leave a bare `IDENT` in type position
// indistinguishable from an expression's, and telling those apart is what R256 exists for:
// `let x: int = y;` would print two bare `IDENT` lines with nothing recording which is which.
//
// The group is reached from `Comparison`'s `is` and `as` (expr.go), from the annotations in
// `BindingDecl`, `Param`, `MemberDecl`, `ErrorField`, `CatchBinder`, `PayloadField`, `AttrParam`
// and the pattern forms, and from `FnLit`'s result. It reaches back into decllit.go for `EnumLit`,
// which `PrimaryType` admits.

// typ is `Type ::= FnType | UnionType`. Spelled `typ` because `type` is a Go keyword; it is the
// only nonterminal in §0 whose name does not survive the transliteration, and renaming the others
// to match would cost more than it saves.
//
// `is` and `as` take a whole `Type`, which is why `v is int | string` needs no parentheses: `BAR`
// binds inside the type. What ends a type is what cannot continue one — `AND` is one token and
// cannot extend `int &`, so `x is int && y` splits at `Conjunction` where `x is int & y` does not
// (`is-intersection-vs-and.parse`).
//
// It is also the right-hand side of `BindingDecl`'s type-alias form since R269, which retired the
// `TypeOnly` this group used to carry. That production required its right-hand side to hold at
// least one type operator — enough to keep the alias and the annotated binding apart, and it put
// the deciding token past the whole right-hand side, where no bounded lookahead reached it. The
// decision is now at the annotation, three tokens (`COLON IDENT("type") ASSIGN`), and this
// function is what runs after it.
func (p *parser) typ() {
	panic("parser: typ is unimplemented")
}
