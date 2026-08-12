// Package token defines Luna's token inventory: the kinds the lexer emits and the
// categories that group them.
//
// The inventory is lexer §0's table, and nothing here may drift from it —
// inventory_test.go pins the names, the categories, and the counts against the spec
// itself. Names returned by Kind.String are §0's names, not Go's, so token dumps and
// golden files stay reviewable against the spec.
//
// §0 has 137 rows and 133 tokens: DOUBLE and BYTES each own two rows, and INVALID owns
// three — the two error productions and the catch-all (R242). Every row names a token.
package token

// Category groups kinds as §0's category column does. The zero value is invalid.
type Category uint8

const (
	CategoryUnset Category = iota
	Trivia
	Keyword
	Literal
	Operator
	Punctuation
	Delimiter
	Interp
	Content
	Identifier
	Error
)

var categoryNames = [...]string{
	CategoryUnset: "unset",
	Trivia:        "trivia",
	Keyword:       "keyword",
	Literal:       "literal",
	Operator:      "operator",
	Punctuation:   "punctuation",
	Delimiter:     "delimiter",
	Interp:        "interp",
	Content:       "content",
	Identifier:    "identifier",
	Error:         "error",
}

func (c Category) String() string {
	if int(c) >= len(categoryNames) {
		return "invalid"
	}
	return categoryNames[c]
}

// Kind identifies a token class. The zero value, Unset, is not a token: it is what an
// unassigned Kind reports, so an uninitialized token never masquerades as a real one. It is
// distinct from Invalid, which IS emitted — for bytes no real production claims (R242).
type Kind uint8

const (
	Unset Kind = iota

	// trivia
	Whitespace
	Margin
	Shebang
	LineComment
	BlockComment

	// keyword
	KwVar
	KwLet
	KwConst
	KwFn
	KwGen
	KwConstraint
	KwProto
	KwEnum
	KwError
	KwCapability
	KwAttribute
	KwTest
	KwExport
	KwImport
	KwIf
	KwElse
	KwForeach
	KwIn
	KwWhile
	KwBreak
	KwContinue
	KwReturn
	KwYieldFrom
	KwYield
	KwMatchBang
	KwMatch
	KwWhere
	KwDefer
	KwBy
	KwTry
	KwCatch
	KwThrow
	KwCopy
	KwSpawn
	KwAwait
	KwComptime
	KwComptype
	KwIs
	KwAs
	KwApply
	KwDeclared
	KwModuleof
	KwUse
	KwTrue
	KwFalse
	KwNull
	KwUndefined
	KwNan
	KwInf
	KwSelf

	// literal
	IntDec
	IntHex
	IntBin
	IntOct
	Double
	StringSq
	StringDq
	Bytes
	Regex
	Command

	// operator
	NullCoalesceAssign
	NullCoalesce
	CoalesceAssign
	Coalesce
	OptProtoAccess
	OptAccess
	Question
	Spread
	RangeExcl
	Range
	Dot
	Or
	Bar
	And
	Amp
	Arrow
	MinusAssign
	Minus
	FatArrow
	Eq
	Assign
	Neq
	Bang
	Le
	Lt
	Ge
	Gt
	PlusAssign
	Plus
	StarAssign
	Star
	SlashAssign
	Slash
	PercentAssign
	Percent
	AtAt
	At

	// punctuation
	AttrOpen
	LParen
	RParen
	LBrace
	RBrace
	LBracket
	RBracket
	Comma
	Semicolon
	Colon

	// delimiter
	TripleDqOpen
	TripleSqOpen
	DqOpen
	RegexOpen
	CmdOpen
	TripleDqClose
	TripleSqClose
	DqClose
	RegexClose
	CmdClose

	// interp
	InterpOpen
	InterpIdent
	InterpClose

	// content
	EscapePair
	DqText
	DollarText
	RegexText
	CmdText
	RawText

	// identifier
	Ident
	Wildcard

	// error
	Invalid
)

type info struct {
	name string
	cat  Category
}

// infos is indexed by Kind. Keyed entries, not positional, so reordering the constant
// block above cannot silently shift a name onto the wrong kind.
var infos = [...]info{
	Unset:              {"UNSET", CategoryUnset},
	Invalid:            {"INVALID", Error},
	Whitespace:         {"WHITESPACE", Trivia},
	Margin:             {"MARGIN", Trivia},
	Shebang:            {"SHEBANG", Trivia},
	LineComment:        {"LINE_COMMENT", Trivia},
	BlockComment:       {"BLOCK_COMMENT", Trivia},
	KwVar:              {"KW_VAR", Keyword},
	KwLet:              {"KW_LET", Keyword},
	KwConst:            {"KW_CONST", Keyword},
	KwFn:               {"KW_FN", Keyword},
	KwGen:              {"KW_GEN", Keyword},
	KwConstraint:       {"KW_CONSTRAINT", Keyword},
	KwProto:            {"KW_PROTO", Keyword},
	KwEnum:             {"KW_ENUM", Keyword},
	KwError:            {"KW_ERROR", Keyword},
	KwCapability:       {"KW_CAPABILITY", Keyword},
	KwAttribute:        {"KW_ATTRIBUTE", Keyword},
	KwTest:             {"KW_TEST", Keyword},
	KwExport:           {"KW_EXPORT", Keyword},
	KwImport:           {"KW_IMPORT", Keyword},
	KwIf:               {"KW_IF", Keyword},
	KwElse:             {"KW_ELSE", Keyword},
	KwForeach:          {"KW_FOREACH", Keyword},
	KwIn:               {"KW_IN", Keyword},
	KwWhile:            {"KW_WHILE", Keyword},
	KwBreak:            {"KW_BREAK", Keyword},
	KwContinue:         {"KW_CONTINUE", Keyword},
	KwReturn:           {"KW_RETURN", Keyword},
	KwYieldFrom:        {"KW_YIELD_FROM", Keyword},
	KwYield:            {"KW_YIELD", Keyword},
	KwMatchBang:        {"KW_MATCH_BANG", Keyword},
	KwMatch:            {"KW_MATCH", Keyword},
	KwWhere:            {"KW_WHERE", Keyword},
	KwDefer:            {"KW_DEFER", Keyword},
	KwBy:               {"KW_BY", Keyword},
	KwTry:              {"KW_TRY", Keyword},
	KwCatch:            {"KW_CATCH", Keyword},
	KwThrow:            {"KW_THROW", Keyword},
	KwCopy:             {"KW_COPY", Keyword},
	KwSpawn:            {"KW_SPAWN", Keyword},
	KwAwait:            {"KW_AWAIT", Keyword},
	KwComptime:         {"KW_COMPTIME", Keyword},
	KwComptype:         {"KW_COMPTYPE", Keyword},
	KwIs:               {"KW_IS", Keyword},
	KwAs:               {"KW_AS", Keyword},
	KwApply:            {"KW_APPLY", Keyword},
	KwDeclared:         {"KW_DECLARED", Keyword},
	KwModuleof:         {"KW_MODULEOF", Keyword},
	KwUse:              {"KW_USE", Keyword},
	KwTrue:             {"KW_TRUE", Keyword},
	KwFalse:            {"KW_FALSE", Keyword},
	KwNull:             {"KW_NULL", Keyword},
	KwUndefined:        {"KW_UNDEFINED", Keyword},
	KwNan:              {"KW_NAN", Keyword},
	KwInf:              {"KW_INF", Keyword},
	KwSelf:             {"KW_SELF", Keyword},
	IntDec:             {"INT_DEC", Literal},
	IntHex:             {"INT_HEX", Literal},
	IntBin:             {"INT_BIN", Literal},
	IntOct:             {"INT_OCT", Literal},
	Double:             {"DOUBLE", Literal},
	StringSq:           {"STRING_SQ", Literal},
	StringDq:           {"STRING_DQ", Literal},
	Bytes:              {"BYTES", Literal},
	Regex:              {"REGEX", Literal},
	Command:            {"COMMAND", Literal},
	NullCoalesceAssign: {"NULL_COALESCE_ASSIGN", Operator},
	NullCoalesce:       {"NULL_COALESCE", Operator},
	CoalesceAssign:     {"COALESCE_ASSIGN", Operator},
	Coalesce:           {"COALESCE", Operator},
	OptProtoAccess:     {"OPT_PROTO_ACCESS", Operator},
	OptAccess:          {"OPT_ACCESS", Operator},
	Question:           {"QUESTION", Operator},
	Spread:             {"SPREAD", Operator},
	RangeExcl:          {"RANGE_EXCL", Operator},
	Range:              {"RANGE", Operator},
	Dot:                {"DOT", Operator},
	Or:                 {"OR", Operator},
	Bar:                {"BAR", Operator},
	And:                {"AND", Operator},
	Amp:                {"AMP", Operator},
	Arrow:              {"ARROW", Operator},
	MinusAssign:        {"MINUS_ASSIGN", Operator},
	Minus:              {"MINUS", Operator},
	FatArrow:           {"FAT_ARROW", Operator},
	Eq:                 {"EQ", Operator},
	Assign:             {"ASSIGN", Operator},
	Neq:                {"NEQ", Operator},
	Bang:               {"BANG", Operator},
	Le:                 {"LE", Operator},
	Lt:                 {"LT", Operator},
	Ge:                 {"GE", Operator},
	Gt:                 {"GT", Operator},
	PlusAssign:         {"PLUS_ASSIGN", Operator},
	Plus:               {"PLUS", Operator},
	StarAssign:         {"STAR_ASSIGN", Operator},
	Star:               {"STAR", Operator},
	SlashAssign:        {"SLASH_ASSIGN", Operator},
	Slash:              {"SLASH", Operator},
	PercentAssign:      {"PERCENT_ASSIGN", Operator},
	Percent:            {"PERCENT", Operator},
	AtAt:               {"AT_AT", Operator},
	At:                 {"AT", Operator},
	AttrOpen:           {"ATTR_OPEN", Punctuation},
	LParen:             {"LPAREN", Punctuation},
	RParen:             {"RPAREN", Punctuation},
	LBrace:             {"LBRACE", Punctuation},
	RBrace:             {"RBRACE", Punctuation},
	LBracket:           {"LBRACKET", Punctuation},
	RBracket:           {"RBRACKET", Punctuation},
	Comma:              {"COMMA", Punctuation},
	Semicolon:          {"SEMICOLON", Punctuation},
	Colon:              {"COLON", Punctuation},
	TripleDqOpen:       {"TRIPLE_DQ_OPEN", Delimiter},
	TripleSqOpen:       {"TRIPLE_SQ_OPEN", Delimiter},
	DqOpen:             {"DQ_OPEN", Delimiter},
	RegexOpen:          {"REGEX_OPEN", Delimiter},
	CmdOpen:            {"CMD_OPEN", Delimiter},
	TripleDqClose:      {"TRIPLE_DQ_CLOSE", Delimiter},
	TripleSqClose:      {"TRIPLE_SQ_CLOSE", Delimiter},
	DqClose:            {"DQ_CLOSE", Delimiter},
	RegexClose:         {"REGEX_CLOSE", Delimiter},
	CmdClose:           {"CMD_CLOSE", Delimiter},
	InterpOpen:         {"INTERP_OPEN", Interp},
	InterpIdent:        {"INTERP_IDENT", Interp},
	InterpClose:        {"INTERP_CLOSE", Interp},
	EscapePair:         {"ESCAPE_PAIR", Content},
	DqText:             {"DQ_TEXT", Content},
	DollarText:         {"DOLLAR_TEXT", Content},
	RegexText:          {"REGEX_TEXT", Content},
	CmdText:            {"CMD_TEXT", Content},
	RawText:            {"RAW_TEXT", Content},
	Ident:              {"IDENT", Identifier},
	Wildcard:           {"WILDCARD", Identifier},
}

// String returns the kind's §0 name (KW_VAR, INT_DEC, DOLLAR_TEXT), not its Go
// identifier. Token dumps and golden files are read against the spec, so they carry the
// spec's vocabulary.
func (k Kind) String() string {
	if int(k) >= len(infos) {
		return "INVALID"
	}
	return infos[k].name
}

// Category reports which of §0's groups the kind belongs to.
func (k Kind) Category() Category {
	if int(k) >= len(infos) {
		return CategoryUnset
	}
	return infos[k].cat
}

// IsTrivia reports whether the kind is one of §2's five trivia tokens. The parser
// filters on this: trivia are emitted so the formatter can see them (R236), and
// dropped by everything else.
func (k Kind) IsTrivia() bool { return k.Category() == Trivia }

// All returns every kind in §0's order, excluding only Unset — the zero value, which
// names no token. Invalid is included: since R242 it is a token like any other, and §10's
// counts are what this feeds.
func All() []Kind {
	ks := make([]Kind, 0, len(infos)-1)
	for k := 1; k < len(infos); k++ {
		ks = append(ks, Kind(k))
	}
	return ks
}
