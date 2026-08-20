package grammar

import (
	"fmt"
	"strings"
)

// jsRule is one entry in grammar.js's `rules` object, already formatted.
type jsRule struct{ name, body string }

// jsGrammar is grammar.js as data, so the same construction serves two consumers: the
// emitter below, and TestCompiledParserIsCurrent, which needs the list of patterns to look
// for in the parser tree-sitter built.
type jsGrammar struct {
	rules    []jsRule
	patterns []string // every regex source emitted, JS-escaped as it appears in the file
}

// buildJS assembles grammar.js from §0.
//
// The same split as the TextMate skeleton, for the same reason: §0 supplies the patterns and
// their order, and the rule structure is a translation this cannot derive. Three rules are
// translations rather than transcriptions, each noted where it happens.
//
// Ordering inside an alternation does not matter here, unlike TextMate. tree-sitter's
// lexer takes the longest match across every token rule, so `0x10` beats `0` whatever order
// the alternatives sit in. §0's order is kept anyway, being longest-first already, so the
// file reads correctly under either rule and does not depend on knowing which applies.
func buildJS(p *patterns) *jsGrammar {
	g := &jsGrammar{}

	// Trivia. LINE_COMMENT transcribes; BLOCK_COMMENT cannot, §0 writing it `(?s)/\*.*?\*/`
	// and tree-sitter compiles to a DFA, which has no lazy quantifier. The replacement is the
	// standard non-nesting construction (content, stars, anything-but-slash, repeat), which
	// F4 licenses by settling that block comments do not nest.
	g.add("comment", g.choice(
		p.pattern("LINE_COMMENT"),
		`/\*[^*]*\*+([^/*][^*]*\*+)*/`,
	))

	// §0 gives ATTR_OPEN as `#\[` alone, the rest being the parser's business. A highlighting
	// grammar wants the whole annotation as one node, so the body and closer are this file's.
	g.add("attribute", g.token(`#\[[^\]]*\]`))

	g.add("regex", g.token(tsRegex(p.pattern("REGEX"))))

	// The triples are §6 mode rules, not §0 patterns, so both are written here. Each interior
	// is the "cannot contain three consecutive delimiters" construction: a unit is a
	// non-quote, or one quote then a non-quote, or two then a non-quote. Written any looser,
	// longest-match runs one token from the first triple's opener to the last one's closer.
	//
	// The single-line forms transcribe, with `b?` prefixed: §0 gives BYTES its own rows, but
	// internal/highlight classes BYTES and STRING_DQ alike, so a separate node could only ever
	// be captured @string.
	g.add("string", g.choice(
		`"""([^"\\]|\\[\s\S]|"[^"\\]|""[^"\\])*"""`,
		`'''([^']|'[^']|''[^'])*'''`,
		"b?"+p.pattern("STRING_DQ"),
		"b?"+p.pattern("STRING_SQ"),
	))

	g.add("command", g.token(p.pattern("COMMAND")))
	g.add("number", g.token(p.alternation(numericOrder)))

	// `!?` is appended so `match!` arrives as one identifier for highlights.scm to match on.
	// §0 makes it KW_MATCH_BANG, a keyword; grammar.js has no keywords, so the `!` has to ride
	// on the identifier or the predicate can never see it.
	g.add("identifier", g.token(p.pattern("IDENT")+"!?"))

	// `~` is appended although §0 gives it no operator row: it opens a regex and nothing else
	// (L0008). The regex rule is longer and wins wherever it applies, but without a fallback a
	// stray `~` begins no token at all and tree-sitter drops into error recovery, which is
	// how a highlighting grammar loses a whole file over one character.
	ops := p.alternation(p.namesInOrder("operator", nil))
	punct := p.alternation(p.namesInOrder("punctuation", notAttrOpen))
	g.add("punctuation", g.token(ops+"|"+punct+"|~"))

	return g
}

// add appends a rule.
func (g *jsGrammar) add(name, body string) { g.rules = append(g.rules, jsRule{name, body}) }

// token wraps one pattern as `token(/…/)`, recording the source for the staleness check.
func (g *jsGrammar) token(pattern string) string {
	return fmt.Sprintf("_ => token(%s),", g.literal(pattern))
}

// choice wraps several as `token(choice(…))`, one per line.
func (g *jsGrammar) choice(patterns ...string) string {
	var b strings.Builder
	b.WriteString("_ => token(choice(\n")
	for _, p := range patterns {
		fmt.Fprintf(&b, "      %s,\n", g.literal(p))
	}
	b.WriteString("    )),")
	return b.String()
}

// literal renders a pattern as a JS regex literal and records its source.
//
// The recorded form is the escaped body, what sits between the slashes, because that is
// what JavaScript's RegExp.prototype.source returns and therefore what tree-sitter writes
// into grammar.json. Recording the unescaped form would make the staleness check miss every
// pattern containing a slash, which is most of the operators.
func (g *jsGrammar) literal(pattern string) string {
	body := escapeSlashes(pattern)
	g.patterns = append(g.patterns, body)
	return "/" + body + "/"
}

// escapeSlashes escapes each `/` that would otherwise end the `/…/` literal.
//
// Character classes are tracked and left alone, which matters for more than tidiness. A `/`
// inside `[…]` does not terminate a JavaScript regex literal, so escaping it there is
// unnecessary, and tree-sitter re-parses the pattern with Rust's regex-syntax rather than
// with JavaScript's engine, so an escape that is merely redundant in one is a gamble in the
// other. Emitting `[^/*]` keeps the class to characters both engines certainly agree on.
//
// An already-escaped character is copied with its backslash and skipped, so a `\\` before a
// `/` cannot be mistaken for escaping it.
func escapeSlashes(p string) string {
	var b strings.Builder
	inClass := false
	for i := 0; i < len(p); i++ {
		switch {
		case p[i] == '\\' && i+1 < len(p):
			b.WriteByte(p[i])
			i++
			b.WriteByte(p[i])
		case p[i] == '[':
			inClass = true
			b.WriteByte(p[i])
		case p[i] == ']':
			inClass = false
			b.WriteByte(p[i])
		case p[i] == '/' && !inClass:
			b.WriteString(`\/`)
		default:
			b.WriteByte(p[i])
		}
	}
	return b.String()
}

// tsRegex translates a §0 pattern into one tree-sitter reads the same way.
//
// Only `(?s)` needs handling. JavaScript has no inline flag syntax: `s` is a flag on the
// literal, which tree-sitter does not carry into its automaton, so the dot has to be
// widened where the flag was doing the work. `\\.` is an escape pair, and `[\s\S]` is the
// portable spelling of "any character including a newline". REGEX is the one row this
// applies to: it is the only literal that may span lines (R244), for the x flag.
func tsRegex(pattern string) string {
	rest, found := strings.CutPrefix(pattern, "(?s)")
	if !found {
		return pattern
	}
	return strings.ReplaceAll(rest, `\\.`, `\\[\s\S]`)
}

// GrammarJS renders the tree-sitter grammar.
func (g *jsGrammar) GrammarJS() []byte {
	var b strings.Builder
	fmt.Fprintf(&b, `/**
 * tree-sitter-luna: a HIGHLIGHTING-GRADE grammar, deliberately lexical.
 *
 * %s
 * %s
 *
 * This is not the Luna parser (that is the compiler's job, per the spec's grammar
 * pins); it tokenizes accurately and imposes almost no structure, so it never
 * mis-parses valid code and never breaks highlighting on partial code while typing.
 * Keyword classification happens in highlights.scm via #match? predicates.
 *
 * AFTER REGENERATING: run tooling/generate-grammar.sh, which regenerates src/ from
 * this file and then runs gengrammar again so highlights.scm can pick up any new
 * node type. Until it does, the checked-in parser.c still implements the previous
 * version and nothing here takes effect — src/ is what Zed compiles, not this.
 */
module.exports = grammar({
  name: 'luna',

  extras: $ => [/\s/, $.comment],

  rules: {
    source_file: $ => repeat($._token),

    _token: $ => choice(
`, bannerLine1, bannerLine2)

	for _, r := range g.rules {
		if r.name == "comment" {
			continue // an extra, reached through `extras` rather than as a token
		}
		fmt.Fprintf(&b, "      $.%s,\n", r.name)
	}
	b.WriteString("    ),\n")

	for _, r := range g.rules {
		fmt.Fprintf(&b, "\n    %s: %s\n", r.name, r.body)
	}

	b.WriteString("  },\n});\n")
	return []byte(b.String())
}
