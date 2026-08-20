package diagnostic

// The parse diagnostics: grammar.md §11.2's two tables, one constant per **allocated** row.
//
// §11.2 lists its diagnostics and numbers only some of them, which is the point rather than an
// oversight: R250, R251 and R267 all rule that a code allocated before there is a check to raise
// it and a test to pin it is a code nothing checks. A published code is also a promise the
// compiler can produce it, and `luna explain` would be answering for a diagnostic that cannot
// occur.
//
// The unnumbered rows are not forgotten, they are waiting: §11.2 carries them with an em dash in
// the code column, TestParseCodesMatchSpec counts them, and each takes its number when the
// check that raises it lands. R272 is why the rule is worth keeping: it retired one of §11.2's
// named rules outright, and under R240's never-reuse rule an eagerly allocated number would now be
// burned on a diagnostic the language cannot produce.
const (
	// The engine (§11.2). Raised by the parser's expect-sites, §7.2 layer 1, the only recovery
	// that needs no judgement, and therefore the only one Phase 2 can raise.
	MissingToken Code = "P0001" // §11.2, R267
)

// parseTitles is the title fixed to each parse code, §11.2's Title column verbatim.
var parseTitles = map[Code]string{
	MissingToken: "Missing token",
}
