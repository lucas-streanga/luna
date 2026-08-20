package lexer

import "luna/oracle/token"

// The two lookup tables §3 and §8 ask for. They are kept together and apart from the
// scanning code because they are the parts a reviewer diffs against the spec directly:
// one against keywords.md §1–§4, the other against §0's operator and punctuation rows
// and §8.5's maximal-munch chains.

// keywords promotes an identifier to its keyword, which is §3's recommended
// implementation: one IDENT scan and a lookup, rather than a pattern per keyword tried
// in order. The two compound keywords are absent; `match!` and `yield from` are folded
// by the peeks in lexWord, because neither is a plain word.
//
// `panic`, `_`, the proto grant modifiers `get` and `set`, and every builtin type name
// are deliberately not here: they lex as IDENT (keywords §4–§5).
var keywords = map[string]token.Kind{
	"var":        token.KwVar,
	"let":        token.KwLet,
	"const":      token.KwConst,
	"fn":         token.KwFn,
	"gen":        token.KwGen,
	"constraint": token.KwConstraint,
	"proto":      token.KwProto,
	"enum":       token.KwEnum,
	"error":      token.KwError,
	"capability": token.KwCapability,
	"attribute":  token.KwAttribute,
	"test":       token.KwTest,
	"export":     token.KwExport,
	"import":     token.KwImport,
	"if":         token.KwIf,
	"else":       token.KwElse,
	"foreach":    token.KwForeach,
	"in":         token.KwIn,
	"while":      token.KwWhile,
	"break":      token.KwBreak,
	"continue":   token.KwContinue,
	"return":     token.KwReturn,
	"yield":      token.KwYield,
	"match":      token.KwMatch,
	"where":      token.KwWhere,
	"defer":      token.KwDefer,
	"by":         token.KwBy,
	"try":        token.KwTry,
	"catch":      token.KwCatch,
	"throw":      token.KwThrow,
	"copy":       token.KwCopy,
	"spawn":      token.KwSpawn,
	"await":      token.KwAwait,
	"comptime":   token.KwComptime,
	"comptype":   token.KwComptype,
	"is":         token.KwIs,
	"as":         token.KwAs,
	"apply":      token.KwApply,
	"declared":   token.KwDeclared,
	"moduleof":   token.KwModuleof,
	"use":        token.KwUse,
	"true":       token.KwTrue,
	"false":      token.KwFalse,
	"null":       token.KwNull,
	"undefined":  token.KwUndefined,
	"nan":        token.KwNan,
	"inf":        token.KwInf,
	"self":       token.KwSelf,
}

type op struct {
	lit  string
	kind token.Kind
}

// operators is every token recognized by its bytes alone: §0's operator rows and most of
// its punctuation, grouped by first byte and **longest lexeme first within
// a group**. The within-group order is §8.5's chains transcribed verbatim, and it is a
// correctness requirement rather than a style (F6): `??` listed before `?` is what
// makes `??` one token. Order *between* groups is irrelevant, since dispatch is on the
// first byte.
//
// Three punctuation rows are absent, each needing a decision bytes alone cannot make:
// `#[`, because a `#` may also open a shebang and is otherwise `L0007`; and `{` / `}`,
// because inside `INTERP_EXPR` their depth is what ends the splice (§6).
var operators = []op{
	{"???=", token.NullCoalesceAssign}, {"???", token.NullCoalesce},
	{"??=", token.CoalesceAssign}, {"??", token.Coalesce},
	{"?->", token.OptProtoAccess}, {"?.", token.OptAccess}, {"?", token.Question},

	{"...", token.Spread}, {"..<", token.RangeExcl}, {"..", token.Range}, {".", token.Dot},

	{"||", token.Or}, {"|", token.Bar},
	{"&&", token.And}, {"&", token.Amp},

	{"->", token.Arrow}, {"-=", token.MinusAssign}, {"-", token.Minus},
	{"=>", token.FatArrow}, {"==", token.Eq}, {"=", token.Assign},
	{"!=", token.Neq}, {"!", token.Bang},
	{"<=", token.Le}, {"<", token.Lt},
	{">=", token.Ge}, {">", token.Gt},

	{"+=", token.PlusAssign}, {"+", token.Plus},
	{"*=", token.StarAssign}, {"*", token.Star},
	{"/=", token.SlashAssign}, {"/", token.Slash},
	{"%=", token.PercentAssign}, {"%", token.Percent},
	{"@@", token.AtAt}, {"@", token.At},

	{"(", token.LParen}, {")", token.RParen},
	{"[", token.LBracket}, {"]", token.RBracket},
	{",", token.Comma}, {";", token.Semicolon}, {":", token.Colon},
}

// opsByByte indexes operators by first byte, preserving table order, so the longest-first
// property above survives into the lookup and matching walks a handful of candidates
// rather than the whole table.
var opsByByte [256][]op

func init() {
	for _, e := range operators {
		opsByByte[e.lit[0]] = append(opsByByte[e.lit[0]], e)
	}
}
