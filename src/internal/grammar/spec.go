// Package grammar generates the editor highlighting grammars from the oracle.
//
// Three artifacts under tooling/ tokenize Luna outside the compiler, and cmd/grammarcheck
// exists because all three had drifted from §0, one still naming a keyword retired in
// R145, all four missing R237's `~"…"` and R246's triples. Every one of those findings was
// a *table*: a keyword list, a delimiter form, a set of operators. Tables are exactly what
// a generator gets right for free.
//
// # What is derived and what is not
//
// The vocabulary is read from the oracle and the spec at generation time:
//
//   - Patterns come from §0's fourth column, verbatim. internal/spec already parses it, and
//     since R237 those patterns are RE2-clean, that ruling having removed the division-set
//     lookbehind precisely because RE2 could not express it, which is also what makes them
//     portable to oniguruma without translation.
//   - Attempt order comes from §0's row order, which §8 already sorts longest-first. A
//     TextMate `patterns` array is first-match-wins in order, so maximal munch transfers as
//     a straight transcription. That coincidence is what makes the whole idea work.
//   - Keyword *grouping* comes from internal/highlight, so the docs renderer and the editor
//     grammars classify `spawn` the same way because they read one table, not because two
//     people made the same call.
//   - Escape sets come from oracle/escape, the same rows the lexer raises L0005 against.
//   - Builtin type names come from types.md via internal/highlight.
//
// The rule *structure* is not derived, and cannot be. §0 describes a mode stack; a TextMate
// grammar is begin/end/patterns. Both are pushdown machines and the mapping is faithful, but
// it is a translation between formalisms rather than a transcription, so the skeleton in
// textmate.go is written by hand. That is the piece a new literal form makes you edit,
// roughly a once-a-ruling event, against tables that refresh themselves every run.
//
// # What generation does not buy
//
// A skeleton with the wrong begin/end nesting is still wrong, and nothing here would notice:
// the check in grammar_test.go proves the files on disk match what this produces, not that
// what this produces is right. Verifying that needs the grammars actually run over the spec
// corpus and compared against the oracle span by span, which is a separate job.
package grammar

import (
	"fmt"
	"regexp"
	"strings"

	"luna/internal/spec"
)

// patterns is §0's fourth column, keyed by token name.
type patterns struct {
	byName map[string][]string // a name may own several rows (DOUBLE, BYTES)
	order  []string            // token names in §0 row order, duplicates collapsed
	rows   []spec.Row
}

// loadPatterns reads §0 and extracts a usable regex from every row that has one.
func loadPatterns() (*patterns, error) {
	inv, err := spec.Load()
	if err != nil {
		return nil, err
	}

	p := &patterns{byName: map[string][]string{}, rows: inv.Rows}
	for _, r := range inv.Rows {
		if !r.IsToken() {
			continue
		}
		if fixed, bad := defects[r.Name]; bad {
			// One entry replaces every row of the name, DOUBLE owning two, so a defect is
			// corrected once rather than per row.
			if _, seen := p.byName[r.Name]; !seen {
				p.order = append(p.order, r.Name)
				p.byName[r.Name] = []string{fixed}
			}
			continue
		}
		re := extract(r.Pattern)
		if re == "" {
			continue // a row whose column is prose, not a pattern — see pattern
		}
		if _, seen := p.byName[r.Name]; !seen {
			p.order = append(p.order, r.Name)
		}
		p.byName[r.Name] = append(p.byName[r.Name], re)
	}
	return p, nil
}

// pattern is the single regex for name, and fails loudly when there is not exactly one.
//
// Three §0 rows carry prose where a pattern would go: MARGIN's "the closing delimiter's
// exact indentation bytes", INTERP_CLOSE's "the `}` returning brace depth to zero", and
// INVALID's catch-all, because each is a rule about lexer state rather than about bytes.
// None is reachable from a TextMate grammar anyway. Asking for one of them is a bug in the
// caller, and a renamed or reformatted row is a bug in the spec sweep; both surface here
// rather than as a silently empty rule in a shipped grammar.
func (p *patterns) pattern(name string) string {
	got := p.byName[name]
	switch len(got) {
	case 1:
		return got[0]
	case 0:
		panic(fmt.Sprintf("grammar: §0 has no usable pattern for %s", name))
	default:
		panic(fmt.Sprintf("grammar: §0 has %d patterns for %s; use alternation", len(got), name))
	}
}

// alternation joins every §0 pattern of the named tokens with `|`, in §0 row order.
//
// Joining the spec's own patterns rather than rebuilding one from the lexemes is what keeps
// the compounds honest: `KW_YIELD_FROM` is `\byield[ \t\r\n]+from\b` and `KW_MATCH_BANG` is
// `\bmatch!` with no closing boundary, neither of which survives being reduced to a word and
// wrapped in `\b(?:…)\b`. Since §0 already orders them before `KW_YIELD` and `KW_MATCH`,
// row order gives first-match-wins the munch order it needs.
func (p *patterns) alternation(names []string) string {
	var parts []string
	for _, n := range names {
		parts = append(parts, p.byName[n]...)
	}
	if len(parts) == 0 {
		panic("grammar: alternation over no patterns")
	}
	return strings.Join(parts, "|")
}

// lexeme is a token's spelling, from §0's first column: `var`, `match!`, `yield from`.
//
// The first column rather than the name, because the name loses the spelling: KW_MATCH_BANG
// says nothing about the `!` and KW_YIELD_FROM nothing about the space. Rows whose first
// column is a description rather than a code span ("spaces, tabs, newlines") yield "".
func (p *patterns) lexeme(name string) string {
	for _, r := range p.rows {
		if r.Name != name {
			continue
		}
		if m := backtickRe.FindStringSubmatch(strings.TrimSpace(r.Token)); m != nil {
			for _, g := range m[1:] {
				if g != "" {
					return strings.TrimSpace(g)
				}
			}
		}
		return ""
	}
	return ""
}

// namesInOrder is the §0 names of a category, in row order, that satisfy keep.
func (p *patterns) namesInOrder(category string, keep func(string) bool) []string {
	var out []string
	seen := map[string]bool{}
	for _, r := range p.rows {
		if !r.IsToken() || r.Category != category || seen[r.Name] {
			continue
		}
		if keep != nil && !keep(r.Name) {
			continue
		}
		seen[r.Name] = true
		out = append(out, r.Name)
	}
	return out
}

// backtickRe takes the pattern column's leading code span. Triple backticks come first
// because the command rows are written that way: their pattern contains a backtick.
var backtickRe = regexp.MustCompile("^(?:```(.+?)```|``(.+?)``|`(.+?)`)")

// defects are §0 rows whose pattern does not compile to what the row means, with the
// reading the rest of the spec makes unambiguous.
//
// §0 writes a pipe as the hex escape `\x7c`, and §0's own note at line 389 explains why: a
// bare `|` would end the markdown table cell. But the escape produces a regex matching a
// *literal* pipe, and these three rows use it where **alternation** is meant. `0\x7c[1-9]…`
// as written matches the three-character string `0|1`; it matches neither `42` nor `0`,
// which are the examples in its own first column. The OR and BAR rows, which do mean a
// literal pipe, are correct, so the notation carries two incompatible meanings and nothing
// distinguishes them.
//
// The correction is not in doubt: §4 and every worked example read these as alternation, and
// the lexer implements them that way. What is in doubt is how §0 should spell it, which is a
// ruling rather than a guess, so the fix is quarantined here instead of applied to the spec.
//
// TestSpecPatternsMatchTheirExamples is what keeps this honest: it fails for any row whose
// pattern does not match its own examples, and an entry here that is no longer needed fails
// too. When the ruling lands, this map empties and the test says so.
var defects = map[string]string{
	"INT_DEC": `0|[1-9](?:_?[0-9])*`,
	"DOUBLE": `(?:0|[1-9](?:_?[0-9])*)\.[0-9](?:_?[0-9])*(?:[eE][+-]?[0-9]+)?` +
		`|(?:0|[1-9](?:_?[0-9])*)[eE][+-]?[0-9]+`,
}

// extract pulls the regex out of one pattern cell.
//
// The cell is a code span followed, on many rows, by an em-dash and commentary about
// orderings and rulings. Taking only the leading span drops the prose; a cell that opens
// with something else (the three state-rule rows) yields "".
//
// `\x7c` is deliberately left alone. It is a valid hex escape in both RE2 and oniguruma, so
// the OR and BAR rows carry across untouched; rewriting it to a bare `|` would silently
// turn those two from literal matches into empty alternations. The rows that meant
// alternation are handled by defects above, where the substitution is visible.
func extract(cell string) string {
	m := backtickRe.FindStringSubmatch(strings.TrimSpace(cell))
	if m == nil {
		return ""
	}
	for _, g := range m[1:] {
		if g != "" {
			return strings.TrimSpace(g)
		}
	}
	return ""
}
