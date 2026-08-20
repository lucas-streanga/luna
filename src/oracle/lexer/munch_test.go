// The exhaustive maximal-munch sweep (lexer-testing-plan §3).
//
// Every ordered pair of DEFAULT-reachable tokens, three times over: written adjacently,
// separated by a space, and separated by a block comment. Exhaustive rather than random,
// because at this size exhaustive is affordable and randomness would only obscure which
// pair failed.
//
// What is under test is §0's own convention, "longest match wins, and where patterns
// overlap the attempt order in §8 is normative", which F6 calls vital rather than
// stylistic. Every rule below is a consequence of it: word juxtaposition is longest-match
// on identifiers, numeric continuation on numbers, the regex flag on `[imsxb]*`, `match!`
// on a compound keyword. Only the comment opener is ordering rather than length.
//
// Two things are checked, and the second is the one §8 exists for.
//
// **The metamorphic relation.** Separated by anything, a pair must lex as exactly those
// two tokens. That says the tokenization of A does not depend on what follows it, which
// is what makes the lexer a function of its input rather than of its context, and is the
// property R237 bought by retiring `/…/`.
//
// **The fusion set.** Written adjacently, most pairs still lex as two tokens; the ones
// that do not must each fall under a rule stated below with its spec citation. An
// unclassified fusion fails, which is what makes maximal munch checkable rather than
// asserted.
//
// This is combinatorial and metamorphic, not mutation testing: nothing here mutates the
// lexer. Mutation was used *on* this sweep, to confirm it bites: a spurious operator
// table entry surfaces as an unclassified fusion, and teaching the `yield from` fold to
// skip comments breaks the metamorphic relation, which no other test in the package sees.
package lexer_test

import (
	"strings"
	"testing"

	"luna/oracle/lexer"
	"luna/oracle/source"
	"luna/oracle/token"
)

// A sample is a concrete lexeme that must lex to exactly one token of the given kind.
// Some kinds carry more than one, where a second shape reaches a rule the first cannot:
// `from` is an IDENT, but it is the only one that can complete `yield from`.
type sample struct {
	kind   token.Kind
	lexeme string
}

// nonKeyword is every DEFAULT-reachable sample that is not a keyword. Keywords are derived
// instead, from their §0 names, so adding one to the spec extends this sweep without anyone
// remembering to.
var nonKeyword = []sample{
	{token.IntDec, "42"}, {token.IntDec, "0"},
	{token.IntHex, "0x1F"}, {token.IntBin, "0b01"}, {token.IntOct, "0o7"},
	{token.Double, "3.14"}, {token.Double, "1e9"},
	{token.StringSq, `'s'`}, {token.StringDq, `"s"`},
	{token.Bytes, `b"s"`}, {token.Regex, `~"s"`}, {token.Command, "`s`"},

	{token.NullCoalesceAssign, "???="}, {token.NullCoalesce, "???"},
	{token.CoalesceAssign, "??="}, {token.Coalesce, "??"},
	{token.OptProtoAccess, "?->"}, {token.OptAccess, "?."}, {token.Question, "?"},
	{token.Spread, "..."}, {token.RangeExcl, "..<"}, {token.Range, ".."}, {token.Dot, "."},
	{token.Or, "||"}, {token.Bar, "|"}, {token.And, "&&"}, {token.Amp, "&"},
	{token.Arrow, "->"}, {token.MinusAssign, "-="}, {token.Minus, "-"},
	{token.FatArrow, "=>"}, {token.Eq, "=="}, {token.Assign, "="},
	{token.Neq, "!="}, {token.Bang, "!"},
	{token.Le, "<="}, {token.Lt, "<"}, {token.Ge, ">="}, {token.Gt, ">"},
	{token.PlusAssign, "+="}, {token.Plus, "+"},
	{token.StarAssign, "*="}, {token.Star, "*"},
	{token.SlashAssign, "/="}, {token.Slash, "/"},
	{token.PercentAssign, "%="}, {token.Percent, "%"},
	{token.AtAt, "@@"}, {token.At, "@"},

	{token.AttrOpen, "#["},
	{token.LParen, "("}, {token.RParen, ")"},
	{token.LBrace, "{"}, {token.RBrace, "}"},
	{token.LBracket, "["}, {token.RBracket, "]"},
	{token.Comma, ","}, {token.Semicolon, ";"}, {token.Colon, ":"},

	{token.Ident, "foo"}, {token.Ident, "from"}, {token.Wildcard, "_"},
}

// skipped are the kinds this sweep cannot pair, each for a structural reason rather than
// for convenience. Trivia would vanish under the non-trivia comparison the sweep makes;
// the delimiters, interp, and content kinds are mode-path only (§6) and unreachable from
// DEFAULT; INVALID is what the sweep would be reporting, not an input to it.
var skipped = map[token.Category]bool{
	token.Trivia: true, token.Delimiter: true, token.Interp: true,
	token.Content: true, token.Error: true,
}

func samples(t *testing.T) []sample {
	t.Helper()

	out := append([]sample(nil), nonKeyword...)
	for _, k := range token.All() {
		if k.Category() != token.Keyword {
			continue
		}
		out = append(out, sample{k, keywordLexeme(k)})
	}

	// Every kind the sweep should reach must have a sample. Without this a token added to
	// §0 would simply go unswept, and the sweep would keep passing over a smaller corpus
	// than it claims, the shape of fail-open this project keeps finding.
	have := map[token.Kind]bool{}
	for _, s := range out {
		have[s.kind] = true
	}
	for _, k := range token.All() {
		if k != token.Shebang && !skipped[k.Category()] && !have[k] {
			t.Errorf("%s is DEFAULT-reachable but has no sample", k)
		}
	}
	return out
}

// keywordLexeme recovers a keyword's spelling from its §0 name: KW_LET is `let`. The two
// compounds are not single words and are named outright (§3, R223).
func keywordLexeme(k token.Kind) string {
	switch k {
	case token.KwMatchBang:
		return "match!"
	case token.KwYieldFrom:
		return "yield from"
	}
	return strings.ToLower(strings.TrimPrefix(k.String(), "KW_"))
}

// TestMunchSamples checks the sweep's own inputs before the sweep trusts them: each
// sample must lex, alone, to exactly the one token it claims. A sample that does not would
// poison every pair it appears in, and there are hundreds of those.
func TestMunchSamples(t *testing.T) {
	for _, s := range samples(t) {
		got := kinds(t, s.lexeme)
		if len(got) != 1 || got[0] != s.kind {
			t.Errorf("sample %q lexes to %v, want [%s]", s.lexeme, got, s.kind)
		}
	}
}

func TestMaximalMunch(t *testing.T) {
	all := samples(t)
	fusions := map[string]int{}

	for _, a := range all {
		for _, b := range all {
			// Separated, the pair must be exactly [A, B]: whatever follows A cannot change
			// what A is. The one exception is the `yield from` fold, whose whitespace-only
			// regex is normative (§0); a comment between the words defeats it, which is
			// the same fact from the other side.
			for _, sep := range []string{" ", "/*c*/"} {
				got := kinds(t, a.lexeme+sep+b.lexeme)
				want := []token.Kind{a.kind, b.kind}
				switch {
				case a.kind == token.KwYield && b.lexeme == "from" && sep == " ":
					want = []token.Kind{token.KwYieldFrom}
				case a.kind == token.Slash && sep == "/*c*/":
					// The separator itself fuses: `/` before `/*` is `//`, so the pair
					// opens a line comment and the rest of the line is trivia. §5's rule
					// that the *next* byte decides between the four `/` forms, working:
					// asserted rather than excepted, since it is a fact about the language.
					want = nil
				}
				if !equal(got, want) {
					t.Errorf("%q + %q + %q lexes to %v, want %v",
						a.lexeme, sep, b.lexeme, got, want)
				}
			}

			// Adjacent, a pair may fuse. Every fusion must have a rule.
			got := kinds(t, a.lexeme+b.lexeme)
			if equal(got, []token.Kind{a.kind, b.kind}) {
				continue
			}
			rule := classify(a, b, got, first(t, a.lexeme+b.lexeme))
			if rule == "" {
				t.Errorf("unclassified fusion: %q + %q lexes to %v",
					a.lexeme, b.lexeme, got)
				continue
			}
			fusions[rule]++
		}
	}

	// The counts are the reviewable artifact §3 asks for: a shift in the shape of the
	// fusion set shows up here even when every individual pair still has a rule.
	for _, rule := range []string{wordJuxtaposition, numericContinues, maximalMunch,
		regexFlag, commentOpener, compoundKeyword} {
		t.Logf("%-24s %5d pairs", rule, fusions[rule])
	}
}

// The fusion rules. Each names why two adjacent lexemes do not lex as two tokens, and
// cites what makes that correct.
const (
	// Two identifier-shaped lexemes written together are one longer word. §7's grammar has
	// no separator, so `let` + `if` is the identifier `letif` and nothing else.
	wordJuxtaposition = "word juxtaposition"

	// A numeric literal absorbs what can continue it: more digits, a radix digit, a
	// separator, an exponent, a point followed by a digit (§4, R238).
	numericContinues = "numeric continuation"

	// §8.6's maximal-munch chains: `?` + `?` is `??`, `!` + `=` is `!=`. The fused token
	// must itself be an operator or punctuation, which is what ties this back to §0.
	maximalMunch = "maximal munch"

	// `[imsxb]*` is greedy, so a regex literal eats an adjacent flag letter (§0, §4).
	regexFlag = "regex flag"

	// A `/` followed by `/` or `*` is a comment, not division: §5's rule that the *next*
	// byte decides between the four `/` forms, and §8.1's comments-first ordering. The
	// pair becomes trivia, or an INVALID when the block comment never closes (F4, L0010).
	commentOpener = "comment opener"

	// `match` + `!` is the single token KW_MATCH_BANG (§3, G7): `\bmatch!` is its own
	// production, so `match!=` is the compound followed by ASSIGN rather than KW_MATCH
	// followed by NEQ.
	compoundKeyword = "compound keyword"
)

// classify names the rule that explains a fusion, or "" when none does, which fails the
// sweep. fused is the lexeme of the first token the concatenation produced.
func classify(a, b sample, got []token.Kind, fused string) string {
	switch {
	case a.kind == token.Slash && (b.lexeme[0] == '/' || b.lexeme[0] == '*'):
		return commentOpener
	case a.kind == token.KwMatch && b.lexeme[0] == '!':
		return compoundKeyword
	case wordShaped(a.lexeme) && identContinues(b.lexeme):
		return wordJuxtaposition
	case numeric(a.kind) && numericContinues0(b.lexeme):
		return numericContinues
	case a.kind == token.Regex && isRegexFlagByte(b.lexeme[0]):
		return regexFlag
	case operatorish(a.kind) && len(got) > 0 && operatorish(got[0]) && knownOperator[fused]:
		// The fused lexeme must be one §0 actually lists. Without that this rule would
		// accept *any* operator-to-operator fusion, so a spurious entry in the operator
		// table would classify itself as maximal munch and pass unnoticed.
		return maximalMunch
	}
	return ""
}

// knownOperator is every operator and punctuation lexeme, taken from the samples so the
// two cannot disagree.
var knownOperator = func() map[string]bool {
	m := map[string]bool{}
	for _, s := range nonKeyword {
		if operatorish(s.kind) {
			m[s.lexeme] = true
		}
	}
	return m
}()

func wordShaped(s string) bool {
	c := s[0]
	return c == '_' || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}

func identContinues(s string) bool {
	c := s[0]
	return wordShaped(s) || ('0' <= c && c <= '9')
}

func numeric(k token.Kind) bool {
	switch k {
	case token.IntDec, token.IntHex, token.IntBin, token.IntOct, token.Double:
		return true
	}
	return false
}

// numericContinues0 is what may extend a number: a digit, a letter (a radix digit or an
// exponent marker), a separator, or a point.
func numericContinues0(s string) bool {
	c := s[0]
	return ('0' <= c && c <= '9') || ('a' <= c && c <= 'z') ||
		('A' <= c && c <= 'Z') || c == '_' || c == '.'
}

func isRegexFlagByte(c byte) bool {
	return c == 'i' || c == 'm' || c == 's' || c == 'x' || c == 'b'
}

func operatorish(k token.Kind) bool {
	return k.Category() == token.Operator || k.Category() == token.Punctuation
}

// kinds lexes src and returns the non-trivia kinds. Trivia is dropped because the
// separators are trivia: comparing past them is what makes the metamorphic relation say
// something about the tokens rather than about the spacing.
func kinds(t *testing.T, src string) []token.Kind {
	t.Helper()
	f, err := source.New("munch", src)
	if err != nil {
		t.Fatalf("%q is not valid source: %v", src, err)
	}
	toks, _ := lexer.Lex(f)

	out := make([]token.Kind, 0, len(toks))
	for _, tok := range toks {
		if !tok.IsTrivia() {
			out = append(out, tok.Kind)
		}
	}
	return out
}

// first is the lexeme of src's first non-trivia token, or "".
func first(t *testing.T, src string) string {
	t.Helper()
	f, err := source.New("munch", src)
	if err != nil {
		t.Fatalf("%q is not valid source: %v", src, err)
	}
	toks, _ := lexer.Lex(f)
	for _, tok := range toks {
		if !tok.IsTrivia() {
			return f.Slice(tok.Offset, tok.Len)
		}
	}
	return ""
}

func equal(a, b []token.Kind) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
