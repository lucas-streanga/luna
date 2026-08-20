package grammar

import (
	"fmt"
	"sort"
	"strings"

	"luna/internal/highlight"
	"luna/internal/spec"
	"luna/oracle/escape"
	"luna/oracle/token"
)

// Rule is one TextMate rule. The json tags are the tmLanguage schema, and the Shiki
// grammar is the same object in a TypeScript encoding: one model, two emitters, which is
// why those two can no longer disagree.
type Rule struct {
	Name          string           `json:"name,omitempty"`
	Match         string           `json:"match,omitempty"`
	Begin         string           `json:"begin,omitempty"`
	End           string           `json:"end,omitempty"`
	Captures      map[string]Scope `json:"captures,omitempty"`
	BeginCaptures map[string]Scope `json:"beginCaptures,omitempty"`
	EndCaptures   map[string]Scope `json:"endCaptures,omitempty"`
	Patterns      []Rule           `json:"patterns,omitempty"`
	Include       string           `json:"include,omitempty"`
}

// Scope is the tmLanguage capture form, `{"name": "..."}`.
type Scope struct {
	Name string `json:"name"`
}

// RuleSet is one repository entry.
type RuleSet struct {
	Patterns []Rule `json:"patterns"`
}

// Grammar is the whole tmLanguage document.
type Grammar struct {
	Name       string             `json:"name"`
	ScopeName  string             `json:"scopeName"`
	FileTypes  []string           `json:"fileTypes"`
	Patterns   []Rule             `json:"patterns"`
	Repository map[string]RuleSet `json:"repository"`
}

// keywordScopes maps internal/highlight's classes to the TextMate scopes themes key on.
//
// The two vocabularies stay separate deliberately. A theme that has never heard of Luna
// still colours `storage.type.luna` correctly, because it matches on the `storage` prefix,
// emitting `tok-decl` as a scope name would render every keyword unstyled in every theme but
// ours. What crosses over is the *grouping*, which is the judgement worth sharing: whether
// `spawn` reads as control flow or as a declaration is decided once, in highlight, and this
// only renames it for a different audience.
var keywordScopes = map[string]string{
	"tok-decl":  "storage.type.luna",
	"tok-kw":    "keyword.control.luna",
	"tok-const": "constant.language.luna",
	"tok-var":   "variable.language.luna",
}

const (
	scopeType    = "support.type.luna"
	scopeIdent   = "variable.other.luna"
	scopeInterp  = "punctuation.section.interpolation.luna"
	scopeEscape  = "constant.character.escape.luna"
	scopeBadEsc  = "invalid.illegal.unknown-escape.luna"
	scopeInterpV = "variable.other.interpolated.luna"
)

// File is one generated artifact, with the path it belongs at relative to the repository
// root.
type File struct {
	Path    string
	Content []byte
}

// Files generates all three artifacts. One entry point for cmd/gengrammar and for the
// regeneration check, so what the test compares is exactly what the command writes.
func Files() ([]File, error) {
	root, err := spec.Root()
	if err != nil {
		return nil, err
	}
	p, err := loadPatterns()
	if err != nil {
		return nil, err
	}
	have, err := loadNodeTypes(root)
	if err != nil {
		return nil, err
	}
	g := build(p)

	tm, err := g.TmLanguage()
	if err != nil {
		return nil, err
	}
	ts, err := g.ShikiTS()
	if err != nil {
		return nil, err
	}

	return []File{
		{Path: "tooling/vscode-luna/syntaxes/luna.tmLanguage.json", Content: tm},
		{Path: "tooling/shiki-luna.ts", Content: ts},
		{Path: "tooling/zed-luna/languages/luna/highlights.scm", Content: g.HighlightsSCM(p, have)},
		{Path: grammarJSPath, Content: buildJS(p).GrammarJS()},
	}, nil
}

// grammarJSPath is tree-sitter's grammar source. Unlike the other three it is not the file
// anything consumes: `tree-sitter generate` compiles it into src/, and src/ is what Zed
// clones and builds. Regenerating this alone changes nothing a user sees.
const grammarJSPath = "tooling/tree-sitter-luna/grammar.js"

// build assembles the grammar from §0, the token inventory, and highlight's grouping.
func build(p *patterns) *Grammar {
	g := &Grammar{
		Name:      "luna",
		ScopeName: "source.luna",
		FileTypes: []string{"luna"},
		Patterns: []Rule{
			// Attempt order. Triples precede their single-quoted forms and `b"` precedes
			// identifiers, both for the reason §8 gives: first match wins here, so a shorter
			// production listed first would win a prefix it should have lost. Numbers precede
			// operators so `3.14` is one token rather than three.
			{Include: "#trivia"},
			{Include: "#attributes"},
			{Include: "#triple-strings"},
			{Include: "#bytes"},
			{Include: "#regex"},
			{Include: "#strings"},
			{Include: "#commands"},
			{Include: "#numbers"},
			{Include: "#keywords"},
			{Include: "#types"},
			{Include: "#identifiers"},
			{Include: "#operators"},
		},
		Repository: map[string]RuleSet{},
	}

	g.Repository["trivia"] = RuleSet{Patterns: []Rule{
		{Name: "comment.line.shebang.luna", Match: p.pattern("SHEBANG")},
		{Name: "comment.line.double-slash.luna", Match: p.pattern("LINE_COMMENT")},
		// §0 gives BLOCK_COMMENT as one `(?s)/\*.*?\*/` pattern, which cannot survive the
		// translation: TextMate matches within a line, so a multi-line comment has to be a
		// begin/end pair. F4 settles the only question that raises: block comments do not
		// nest, so the first `*/` closes it and no `patterns` are needed inside.
		{Name: "comment.block.luna", Begin: `/\*`, End: `\*/`},
	}}

	g.Repository["attributes"] = RuleSet{Patterns: []Rule{{
		Name:          "meta.annotation.luna",
		Begin:         p.pattern("ATTR_OPEN"),
		End:           `\]`,
		BeginCaptures: map[string]Scope{"0": {Name: "punctuation.definition.annotation.luna"}},
		EndCaptures:   map[string]Scope{"0": {Name: "punctuation.definition.annotation.luna"}},
		Patterns: []Rule{
			{Include: "#strings"},
			{Include: "#numbers"},
			{Name: "entity.name.function.annotation.luna", Match: p.pattern("IDENT")},
		},
	}}}

	g.Repository["triple-strings"] = RuleSet{Patterns: []Rule{
		{
			// R246's opener owns to the end of its line, so the begin pattern ends at `$`
			// rather than consuming a newline §0 writes into the token.
			Name:     "string.quoted.triple.luna",
			Begin:    `"""[ \t\r]*$`,
			End:      `^[ \t]*"""`,
			Patterns: interpolated(p, escape.StringDq),
		},
		{
			// Raw: no escapes, no interpolation, so `\`, `$` and `'` are ordinary bytes
			// (R246, R249). The absent Patterns is the whole of that rule.
			Name:  "string.quoted.triple.raw.luna",
			Begin: `'''[ \t\r]*$`,
			End:   `^[ \t]*'''`,
		},
	}}

	g.Repository["bytes"] = RuleSet{Patterns: []Rule{
		{Name: "string.quoted.double.bytes.luna", Begin: `b"`, End: `"|$`, Patterns: escapeRules(escape.Bytes)},
		{Name: "string.quoted.single.bytes.luna", Begin: `b'`, End: `'|$`, Patterns: escapeRules(escape.Bytes)},
	}}

	g.Repository["regex"] = RuleSet{Patterns: []Rule{{
		Name:  "string.regexp.luna",
		Begin: p.pattern("REGEX_OPEN"),
		// REGEX_CLOSE carries the flags (§0), and the regex is the one form that may span
		// lines (R244), hence no `$` alternative in the end pattern.
		End:         p.pattern("REGEX_CLOSE"),
		EndCaptures: map[string]Scope{"0": {Name: "keyword.other.regexp-flags.luna"}},
		Patterns:    interpolated(p, escape.Regex),
	}}}

	g.Repository["strings"] = RuleSet{Patterns: []Rule{
		{
			// `"|$` is R244: a raw newline ends the literal, so the rule must not run on to
			// colour the rest of the file as string.
			Name:     "string.quoted.double.luna",
			Begin:    `"`,
			End:      `"|$`,
			Patterns: interpolated(p, escape.StringDq),
		},
		{
			Name:     "string.quoted.single.luna",
			Begin:    `'`,
			End:      `'|$`,
			Patterns: escapeRules(escape.StringSq),
		},
	}}

	g.Repository["commands"] = RuleSet{Patterns: []Rule{{
		Name:     "string.interpolated.command.luna",
		Begin:    "`",
		End:      "`|$",
		Patterns: interpolated(p, escape.Command),
	}}}

	g.Repository["numbers"] = RuleSet{Patterns: []Rule{{
		Name: "constant.numeric.luna",
		// The one place row order is *not* attempt order, so it is named outright from §8.5:
		// doubles first so `1.5` is one token rather than `1` `.` `5`, the radix prefixes
		// before decimal so `0x10` does not lex as INT_DEC(0) + IDENT(x10). §0 lists INT_DEC
		// first, and taking that order would break both.
		Match: p.alternation(numericOrder),
	}}}

	g.Repository["keywords"] = RuleSet{Patterns: keywordRules(p)}

	g.Repository["types"] = RuleSet{Patterns: []Rule{{
		Name:  scopeType,
		Match: `\b(?:` + strings.Join(highlightedTypes(p), "|") + `)\b`,
	}}}

	g.Repository["identifiers"] = RuleSet{Patterns: []Rule{
		{Name: "variable.language.wildcard.luna", Match: p.pattern("WILDCARD")},
		{Name: scopeIdent, Match: p.pattern("IDENT")},
	}}

	g.Repository["operators"] = RuleSet{Patterns: []Rule{
		{Name: "keyword.operator.luna", Match: p.alternation(p.namesInOrder("operator", nil))},
		{Name: "punctuation.separator.luna", Match: p.alternation(p.namesInOrder("punctuation", notAttrOpen))},
	}}

	return g
}

// numericOrder is §8.5's attempt order for the numeric literals, verbatim. The leading-zero
// error production it names between the radix forms and INT_DEC is omitted: it exists to
// raise L0003, and a highlighting grammar has no way to say "this is a diagnostic".
var numericOrder = []string{"DOUBLE", "INT_HEX", "INT_BIN", "INT_OCT", "INT_DEC"}

// notAttrOpen keeps `#[` out of the punctuation alternation: it is the annotation rule's
// begin pattern, and matching it here first would colour the opener and leave the body to
// be lexed as ordinary code.
func notAttrOpen(name string) bool { return name != "ATTR_OPEN" }

// keywordRules emits one rule per class group, preserving §0's row order inside each.
//
// Order within a group is what carries maximal munch: `\bmatch!` must be attempted before
// `\bmatch\b`, and §0 already lists it that way. Order *between* groups does not matter,
// because every keyword pattern is `\b`-anchored, so `in` cannot claim the front of
// `import` however the groups are arranged. The groups are emitted sorted by class so the
// generated file is stable across runs.
func keywordRules(p *patterns) []Rule {
	byClass := map[string][]string{}
	for _, name := range p.namesInOrder("keyword", nil) {
		k := kindByName(name)
		class := highlight.Class(k)
		scope, ok := keywordScopes[class]
		if !ok {
			panic(fmt.Sprintf("grammar: keyword %s has class %q with no TextMate scope", name, class))
		}
		byClass[scope] = append(byClass[scope], name)
	}

	scopes := make([]string, 0, len(byClass))
	for s := range byClass {
		scopes = append(scopes, s)
	}
	sort.Strings(scopes)

	out := make([]Rule, 0, len(scopes))
	for _, s := range scopes {
		out = append(out, Rule{Name: s, Match: p.alternation(byClass[s])})
	}
	return out
}

// highlightedTypes is types.md's list minus the names §3 also reserves as keywords,
// `null`, `fn`, `proto`, `error`, `capability`.
//
// highlight can afford to keep them: it consults the set only for a token the lexer already
// classified IDENT, so a keyword never reaches it. A grammar has no such guarantee. Both
// rules can match `error`, and which wins is a question of rule order in TextMate and of the
// host's convention in tree-sitter: Neovim takes the last capture, Zed the first. Removing
// the overlap answers it the same way everywhere, and it answers it correctly: those words
// lex as keywords, so colouring them as types is never right.
func highlightedTypes(p *patterns) []string {
	reserved := map[string]bool{}
	for _, name := range p.namesInOrder("keyword", nil) {
		reserved[p.lexeme(name)] = true
	}

	var out []string
	for _, t := range highlight.BuiltinTypes() {
		if !reserved[t] {
			out = append(out, t)
		}
	}
	return out
}

// kindByName resolves a §0 name to its Kind. Linear over 133 kinds, run once per keyword at
// generation time; a map would be faster and less obviously correct.
func kindByName(name string) token.Kind {
	for _, k := range token.All() {
		if k.String() == name {
			return k
		}
	}
	panic(fmt.Sprintf("grammar: §0 names %s, the token inventory does not", name))
}

// interpolated is the pattern list for a literal that admits `${…}`, the three forms of
// F1-F3, plus the cooked triple since R246.
//
// The nested `$self` is how a TextMate grammar answers the non-regularity those flags name.
// The engine tries the inner patterns and the end pattern together and takes whichever
// matches first, so in `"${x ?? "none"}"` the `${` rule wins over the closing quote and
// consumes through its own `}`, the same answer the mode stack gives, reached differently.
func interpolated(p *patterns, ctx escape.Context) []Rule {
	rules := escapeRules(ctx)
	rules = append(rules,
		Rule{
			Name:          "meta.interpolation.luna",
			Begin:         p.pattern("INTERP_OPEN"),
			End:           `\}`,
			BeginCaptures: map[string]Scope{"0": {Name: scopeInterp}},
			EndCaptures:   map[string]Scope{"0": {Name: scopeInterp}},
			Patterns:      []Rule{{Include: "$self"}},
		},
	)
	// The bare `$name` form is DQ_STRING only (§0, string §5); a command's `$` that starts no
	// interp form is ordinary content (command §2.2).
	if ctx == escape.StringDq {
		rules = append(rules, Rule{Name: scopeInterpV, Match: p.pattern("INTERP_IDENT")})
	}
	return rules
}

// escapeRules turns string §5.1's row for ctx into a valid-escape rule and an illegal one.
//
// Flagging the illegal case is the part no hand-written grammar here has ever done: they
// all match `\\.` and colour every escape alike, so `\q` reads as legitimate right up until
// the compiler rejects it. Reading the row from oracle/escape means the grammar marks
// exactly what the lexer raises L0005 for.
func escapeRules(ctx escape.Context) []Rule {
	set := escape.Allowed(ctx)
	if set == "" {
		// The regex passes escapes through undecoded, so nothing there can be unknown and
		// there is no illegal form to flag (§0's REGEX_BODY row).
		return []Rule{{Name: scopeEscape, Match: `\\.`}}
	}

	var alts []string
	var plain []byte
	for i := 0; i < len(set); i++ {
		switch c := set[i]; c {
		case 'u':
			// `u` is in the row, but a bare `\u` is malformed rather than valid (L0013), so it
			// cannot go in the character class: the braces and digits are part of the escape.
			alts = append(alts, `\\u\{[0-9a-fA-F]{1,6}\}`)
		case 'x':
			alts = append(alts, `\\x[0-9a-fA-F]{2}`) // exactly two (bytes §7)
		default:
			plain = append(plain, c)
		}
	}
	if len(plain) > 0 {
		alts = append(alts, `\\[`+charClass(string(plain))+`]`)
	}

	return []Rule{
		{Name: scopeEscape, Match: strings.Join(alts, "|")},
		{Name: scopeBadEsc, Match: `\\.`},
	}
}

// charClass escapes the four characters that carry meaning inside `[...]`.
func charClass(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if c := s[i]; c == '\\' || c == ']' || c == '^' || c == '-' {
			b.WriteByte('\\')
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
