// Package highlight renders Luna source as HTML, coloured from the oracle's own token
// stream rather than from a regex approximation of it.
//
// The three shipping grammars under tooling/ each re-derive §0 by hand, and cmd/grammarcheck
// exists because they drift. This is the other half of that answer: for a consumer with no
// latency budget (documentation, rendered at build time) there is no reason to approximate
// the lexer when the lexer is right there.
//
// Two properties the oracle has and a TextMate grammar cannot promise make this almost
// trivial:
//
//   - Tokens tile the input (R242). Every byte belongs to exactly one token, trivia and
//     unlexable bytes included, so the rendering is a straight walk with no gaps to paper
//     over and no catch-all rule. Lossless is a *test* here, not a hope.
//   - Trivia is a token (R236). Comments and whitespace arrive as kinds, so they need no
//     separate pass.
//
// # What this deliberately does not do
//
// The map from token to class is pure, one kind to one class, plus the builtin type names
// §0 cannot distinguish from any other identifier. Nothing here looks at neighbouring
// tokens. A TextMate grammar fakes shallow parsing with lookahead, colouring `foo` as a
// function because a `(` follows, or a name as a type because it sits after `:`, and gets
// it wrong exactly as often as the heuristic is wrong.
//
// That line is drawn on purpose. The lexer knows what it knows; the rest waits for a
// parser, which will know it properly. The visible cost is that call sites, declaration
// names and type positions render as plain identifiers.
package highlight

import (
	"fmt"
	"html"
	"sort"
	"strings"

	"luna/oracle/diagnostic"
	"luna/oracle/lexer"
	"luna/oracle/source"
	"luna/oracle/token"
)

// The CSS classes emitted. Short and few, because a docs theme has to style all of them and a
// class per token kind would be 133 rules describing a dozen real distinctions.
//
// classPlain is the empty class, meaning no span at all: whitespace and ordinary
// identifiers are written as bare text. It is a named constant because "" appearing in the
// table below should read as a decision rather than as a gap.
const (
	classPlain    = ""
	classComment  = "tok-com"
	classKeyword  = "tok-kw"   // control flow and the word-spelled operators
	classDecl     = "tok-decl" // the binding and type-introducing forms
	classConst    = "tok-const"
	classVar      = "tok-var" // self, the wildcard, and an interpolated name
	classType     = "tok-type"
	classNumber   = "tok-num"
	classString   = "tok-str"
	classEscape   = "tok-esc"
	classRegex    = "tok-regex"
	classCommand  = "tok-cmd"
	classInterp   = "tok-interp"
	classOperator = "tok-op"
	classPunct    = "tok-punc"
	classAttr     = "tok-attr"
	classError    = "tok-err"
)

// classes maps every kind in §0's inventory. A map rather than the keyed array token uses,
// because the property worth testing here is *presence*: classPlain is a legitimate value,
// so in an array an unmapped kind and a deliberately-bare one would both read as "". With a
// map the totality test can tell them apart, and it does.
var classes = map[token.Kind]string{
	// Trivia (§2). Whitespace carries no colour, so it is written bare; the margin a triple
	// literal strips (R246) is whitespace too, and showing it as string content would claim
	// it survives into the value, which is the one thing R246 settled that it does not.
	token.Whitespace:   classPlain,
	token.Margin:       classPlain,
	token.Shebang:      classComment,
	token.LineComment:  classComment,
	token.BlockComment: classComment,

	// Keywords (§3), split the way a reader uses them rather than the way §0 lists them.
	token.KwVar:        classDecl,
	token.KwLet:        classDecl,
	token.KwConst:      classDecl,
	token.KwFn:         classDecl,
	token.KwGen:        classDecl,
	token.KwConstraint: classDecl,
	token.KwProto:      classDecl,
	token.KwEnum:       classDecl,
	token.KwError:      classDecl,
	token.KwCapability: classDecl,
	token.KwAttribute:  classDecl,
	token.KwTest:       classDecl,
	token.KwExport:     classDecl,
	token.KwImport:     classDecl,

	token.KwIf:        classKeyword,
	token.KwElse:      classKeyword,
	token.KwForeach:   classKeyword,
	token.KwIn:        classKeyword,
	token.KwWhile:     classKeyword,
	token.KwBreak:     classKeyword,
	token.KwContinue:  classKeyword,
	token.KwReturn:    classKeyword,
	token.KwYieldFrom: classKeyword,
	token.KwYield:     classKeyword,
	token.KwMatchBang: classKeyword,
	token.KwMatch:     classKeyword,
	token.KwWhere:     classKeyword,
	token.KwDefer:     classKeyword,
	token.KwBy:        classKeyword,
	token.KwTry:       classKeyword,
	token.KwCatch:     classKeyword,
	token.KwThrow:     classKeyword,
	token.KwCopy:      classKeyword,
	token.KwSpawn:     classKeyword,
	token.KwAwait:     classKeyword,
	token.KwComptime:  classKeyword,
	token.KwComptype:  classKeyword,
	token.KwIs:        classKeyword,
	token.KwAs:        classKeyword,
	token.KwApply:     classKeyword,
	token.KwDeclared:  classKeyword,
	token.KwModuleof:  classKeyword,
	token.KwUse:       classKeyword,

	token.KwTrue:      classConst,
	token.KwFalse:     classConst,
	token.KwNull:      classConst,
	token.KwUndefined: classConst,
	token.KwNan:       classConst,
	token.KwInf:       classConst,
	token.KwSelf:      classVar,

	// Literals (§4). The four integer bases are one colour: the base is a spelling, not a
	// different kind of value, and a theme that distinguished them would be saying otherwise.
	token.IntDec:   classNumber,
	token.IntHex:   classNumber,
	token.IntBin:   classNumber,
	token.IntOct:   classNumber,
	token.Double:   classNumber,
	token.StringSq: classString,
	token.StringDq: classString,
	token.Bytes:    classString,
	token.Regex:    classRegex,
	token.Command:  classCommand,

	// Operators (§5).
	token.NullCoalesceAssign: classOperator,
	token.NullCoalesce:       classOperator,
	token.CoalesceAssign:     classOperator,
	token.Coalesce:           classOperator,
	token.OptProtoAccess:     classOperator,
	token.OptAccess:          classOperator,
	token.Question:           classOperator,
	token.Spread:             classOperator,
	token.RangeExcl:          classOperator,
	token.Range:              classOperator,
	token.Dot:                classOperator,
	token.Or:                 classOperator,
	token.Bar:                classOperator,
	token.And:                classOperator,
	token.Amp:                classOperator,
	token.Arrow:              classOperator,
	token.MinusAssign:        classOperator,
	token.Minus:              classOperator,
	token.FatArrow:           classOperator,
	token.Eq:                 classOperator,
	token.Assign:             classOperator,
	token.Neq:                classOperator,
	token.Bang:               classOperator,
	token.Le:                 classOperator,
	token.Lt:                 classOperator,
	token.Ge:                 classOperator,
	token.Gt:                 classOperator,
	token.PlusAssign:         classOperator,
	token.Plus:               classOperator,
	token.StarAssign:         classOperator,
	token.Star:               classOperator,
	token.SlashAssign:        classOperator,
	token.Slash:              classOperator,
	token.PercentAssign:      classOperator,
	token.Percent:            classOperator,
	token.AtAt:               classOperator,
	token.At:                 classOperator,

	// Punctuation (§5). `#[` gets its own class: it is the one punctuation mark that opens a
	// construct a reader scans *past*, and a theme wants to dim it.
	token.AttrOpen:  classAttr,
	token.LParen:    classPunct,
	token.RParen:    classPunct,
	token.LBrace:    classPunct,
	token.RBrace:    classPunct,
	token.LBracket:  classPunct,
	token.RBracket:  classPunct,
	token.Comma:     classPunct,
	token.Semicolon: classPunct,
	token.Colon:     classPunct,

	// Delimiters (§0). Coloured as the literal they bound, which is what every theme does
	// with a quote: the delimiter is part of the string to a reader, whatever §0 says.
	token.TripleDqOpen:  classString,
	token.TripleSqOpen:  classString,
	token.DqOpen:        classString,
	token.TripleDqClose: classString,
	token.TripleSqClose: classString,
	token.DqClose:       classString,
	token.RegexOpen:     classRegex,
	token.RegexClose:    classRegex,
	token.CmdOpen:       classCommand,
	token.CmdClose:      classCommand,

	// Interpolation (§6). The `${`, the `}` and a bare `$name` are the splice's own marks
	// and take the splice colour; the expression between them lexes as ordinary Luna and is
	// coloured as such, which is the whole benefit of a mode stack over a nested regex.
	token.InterpOpen:  classInterp,
	token.InterpIdent: classVar,
	token.InterpClose: classInterp,

	// Content (§0). Bodies take their literal's colour, except an escape, which is the one
	// part of a string that a reader must see is not itself.
	token.EscapePair: classEscape,
	token.DqText:     classString,
	token.DollarText: classString,
	token.RawText:    classString,
	token.RegexText:  classRegex,
	token.CmdText:    classCommand,

	// Identifiers (§7). Bare, unless the name is one types.md lists (see builtins).
	token.Ident:    classPlain,
	token.Wildcard: classVar,

	token.Invalid: classError,
}

// classOf is the class for a kind, and the only reader of the table.
//
// An unmapped kind panics rather than falling back to plain. The totality test makes that
// unreachable for every kind §0 defines, so reaching it means a kind was added to the
// inventory and not to the table, a gap that would otherwise show up as one silently
// uncoloured construct in the rendered docs, which is precisely the class of drift this
// package exists to end.
func classOf(k token.Kind, text string) string {
	c, ok := classes[k]
	if !ok {
		panic(fmt.Sprintf("highlight: no class for %s", k))
	}
	if k == token.Ident && builtins[text] {
		return classType
	}
	return c
}

// Class is the CSS class a kind renders as, or "" where it renders bare.
//
// Exported for the grammar generator, which needs the *grouping* rather than the class
// string: which keywords are declarations, which are control flow, which are constants.
// That split is a presentation judgement made once, here, and a generated editor grammar
// reading it is how the docs and the editors end up agreeing about `spawn` by construction
// instead of by two people making the same call twice.
//
// The builtin type names are not visible through this: they refine an IDENT by spelling,
// not by kind, so a caller wanting them asks BuiltinTypes.
func Class(k token.Kind) string { return classOf(k, "") }

// BuiltinTypes is the type names types.md lists, sorted. See builtins.
func BuiltinTypes() []string {
	out := make([]string, 0, len(builtins))
	for name := range builtins {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Render highlights a whole file, returning the HTML and whatever the lexer had to say
// about it.
//
// The diagnostics are returned rather than rendered. A docs build wants them on stderr with
// the file and line, not smuggled into the page as a tooltip nobody hovers; cmd/highlight
// decides, and -strict makes them fatal.
func Render(f *source.File) (string, diagnostic.List) {
	toks, errs := lexer.Lex(f)
	return RenderTokens(f, toks), errs
}

// RenderTokens is Render over an already-lexed stream, for a caller that has one.
func RenderTokens(f *source.File, toks []token.Token) string {
	var b strings.Builder
	// One span per token is the worst case and most tokens get one; the source itself is the
	// rest. Wrong for a file that is mostly whitespace, which costs a regrow and nothing else.
	b.Grow(len(f.Text())*2 + len(toks)*24)

	b.WriteString(`<pre class="luna"><code>`)
	// Runs of one class become one span. Adjacent tokens sharing a colour are the common
	// case rather than the exception. A string is its two delimiters plus its text and an
	// operator often abuts another, so emitting per token would roughly triple the markup
	// for every literal in the document while rendering identically.
	//
	// Slicing the run as start..end rather than concatenating each token's text is only
	// valid because the stream tiles (R242): adjacent tokens leave no gap, so the bytes
	// between the first token's start and the last one's end are exactly the run's.
	for i := 0; i < len(toks); {
		class := classOf(toks[i].Kind, f.Slice(toks[i].Offset, toks[i].Len))
		start := toks[i].Offset
		end := toks[i].End()
		for i++; i < len(toks); i++ {
			if classOf(toks[i].Kind, f.Slice(toks[i].Offset, toks[i].Len)) != class {
				break
			}
			end = toks[i].End()
		}

		text := html.EscapeString(f.Text()[start:end])
		if class == classPlain {
			b.WriteString(text)
			continue
		}
		b.WriteString(`<span class="`)
		b.WriteString(class)
		b.WriteString(`">`)
		b.WriteString(text)
		b.WriteString(`</span>`)
	}
	b.WriteString(`</code></pre>`)
	return b.String()
}
