package highlight

// builtins are the type names types.md lists, and the one place this package knows
// something §0 does not.
//
// To the lexer every one of these is an IDENT (§7): `list` is spelled exactly like a
// variable called `list`, and no lexical rule separates them. Highlighting them anyway is
// a *convention* rather than a fact about the token stream, the same convention every
// language's editor support follows, and the reason the three tooling grammars colour these
// words today. Dropping them would have made this renderer visibly worse than the grammars
// it is meant to replace, which is the only argument for the set existing.
//
// It is written out rather than read from the spec at runtime: a rendered page must not
// depend on the spec tree being on disk. TestBuiltinsMatchSpec pins it to the four tables in
// specs/overview/types.md instead, so a type added there fails the suite rather than going
// quietly uncoloured, the same three-party discipline §0's inventory is held to.
//
// Some of these lex as keywords, never as identifiers: `null`, `proto`, `error` and
// `capability` are in §3. They stay in the set because the set's definition is "the names
// those tables list", which is what the test can check; classOf consults it only for an
// IDENT, so the overlap decides nothing.
var builtins = map[string]bool{
	// Primitive and value types
	"int": true, "double": true, "float": true, "string": true, "bool": true,
	"null": true, "regex": true, "command": true, "secret": true, "bytes": true,
	"byte": true, "duration": true, "instant": true, "type": true,

	// Structured types
	"table": true, "list": true, "sink": true, "fn": true, "stream": true,
	"promise": true, "stringBuilder": true,

	// Declaration forms
	"proto": true, "error": true, "capability": true,

	// The top and bottom types
	"any": true, "never": true,
}
