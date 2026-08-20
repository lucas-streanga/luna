package grammar

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"luna/internal/spec"
)

// TestGeneratedFilesAreCurrent is the regeneration check, and the whole point of generating
// these.
//
// cmd/grammarcheck could only ask whether a grammar still named a retired keyword, a probe
// so it found what it was told to look for and nothing else. This asks the total question
// instead: are the bytes on disk the bytes this package produces? A ruling that moves a §0
// row now fails the suite, and so does a hand edit to a generated file.
func TestGeneratedFilesAreCurrent(t *testing.T) {
	root, err := spec.Root()
	if err != nil {
		t.Fatalf("locating the repository: %v", err)
	}
	files, err := Files()
	if err != nil {
		t.Fatalf("generating: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no files generated: this test checked nothing")
	}

	for _, f := range files {
		got, err := os.ReadFile(filepath.Join(root, f.Path))
		if err != nil {
			t.Errorf("%s: %v", f.Path, err)
			continue
		}
		if !bytes.Equal(got, f.Content) {
			t.Errorf("%s is stale (%d bytes on disk, %d generated) — run `go run ./cmd/gengrammar`\n%s",
				f.Path, len(got), len(f.Content), firstDiff(string(got), string(f.Content)))
		}
	}
}

// knownDefective are the §0 rows whose pattern is wrong pending a ruling. Every one of
// them the `\x7c` confusion described on defects.
//
// Listing them keeps the suite green without hiding them: the test asserts each one still
// fails, so when the ruling lands and the row is corrected, this test fails demanding the
// entry be deleted. A known issue that cannot be quietly fixed and forgotten.
//
// DQ_TEXT and RAW_TEXT carry the same defect and are absent because their first column is
// prose, so there is no example to match and nothing here can see it. ESCAPE_PAIR is here
// but not in defects, since the generator builds escape rules from oracle/escape and never
// reads that row.
var knownDefective = map[string]bool{
	"INT_DEC":     true,
	"DOUBLE":      true,
	"ESCAPE_PAIR": true,
}

// contextual are rows whose pattern deliberately covers more than the lexeme its first
// column shows, so the example cannot match and its failing to would mean nothing.
//
// Both triple openers own to the end of their line (R246), so their patterns end in `\n`
// while the column sensibly illustrates the delimiter alone.
var contextual = map[string]bool{
	"TRIPLE_DQ_OPEN": true,
	"TRIPLE_SQ_OPEN": true,
}

// TestSpecPatternsMatchTheirExamples is what found the `\x7c` defect.
//
// §0's first column is examples of the token and its fourth is the pattern that must match
// them. Nothing had ever run one against the other: the lexer is hand-written from the prose
// and its tests are written from the lexer, so a pattern could be wrong in the table for
// years and no check would notice, which is exactly what happened.
func TestSpecPatternsMatchTheirExamples(t *testing.T) {
	inv, err := spec.Load()
	if err != nil {
		t.Fatalf("reading §0: %v", err)
	}

	checked := 0
	stillBad := map[string]bool{}
	for _, r := range inv.Rows {
		if !r.IsToken() || contextual[r.Name] {
			continue
		}
		pattern := extract(r.Pattern)
		if pattern == "" {
			continue // a state rule, described in prose
		}
		// The row keeps its own pattern here even where defects overrides it downstream: the
		// point is to watch the spec, not the correction.
		re, err := regexp.Compile(pattern)
		if err != nil {
			t.Errorf("§0:%d %s: pattern does not compile: %v", r.Line, r.Name, err)
			continue
		}

		for _, ex := range examples(r.Token) {
			checked++
			switch {
			case re.MatchString(ex):
			case knownDefective[r.Name]:
				stillBad[r.Name] = true
			default:
				t.Errorf("§0:%d %s: pattern %s does not match its own example %q",
					r.Line, r.Name, pattern, ex)
			}
		}
	}

	if checked == 0 {
		t.Fatal("no examples were checked: §0's first column changed shape")
	}
	for name := range knownDefective {
		if !stillBad[name] {
			t.Errorf("%s now matches its examples: the spec was fixed, so remove it from "+
				"knownDefective (and from defects, if it is there)", name)
		}
	}
	t.Logf("%d examples checked; %d rows defective pending a ruling", checked, len(stillBad))
}

// TestCompiledParserIsCurrent watches the one link in the chain that a command cannot run.
//
//	lexer.md §0 ──[ gengrammar, Go ]──► grammar.js ──[ tree-sitter, podman ]──► src/
//
// TestGeneratedFilesAreCurrent guards the first arrow. This guards the second, and it exists
// because src/ is *committed*: Zed fetches the grammar by repo and rev and compiles
// src/parser.c with its own toolchain, having no tree-sitter-cli, so the generated parser
// is the shipped artifact, not a build intermediate (zed-luna/README.md, R60).
//
// Both ends of that arrow being tracked is what makes the lag committable. git cannot tell
// you: it sees two modified files and has no idea one is derived from the other. So this
// asks the question directly: grammar.json embeds every pattern verbatim, so a pattern
// present in the source and absent from the compiled form means the container step has not
// been run since the source last changed.
//
// The remedy is tooling/generate-grammar.sh, which needs podman. That makes this the one
// check in the suite whose fix is not a Go command, and the reason it is worth the
// awkwardness is that the failure it catches is invisible in review and ships.
func TestCompiledParserIsCurrent(t *testing.T) {
	root, err := spec.Root()
	if err != nil {
		t.Fatalf("locating the repository: %v", err)
	}
	p, err := loadPatterns()
	if err != nil {
		t.Fatal(err)
	}
	js := buildJS(p)
	if len(js.patterns) == 0 {
		t.Fatal("the grammar emitted no patterns: this test checked nothing")
	}

	built, err := compiledPatterns(filepath.Join(root, "tooling/tree-sitter-luna/src/grammar.json"))
	if err != nil {
		t.Fatal(err)
	}

	var stale int
	for _, pat := range js.patterns {
		if !strings.Contains(built, pat) {
			stale++
			t.Errorf("pattern is in grammar.js but not in the compiled parser:\n  %s", pat)
		}
	}
	if stale > 0 {
		t.Errorf("\n%d pattern(s) stale. src/ is what Zed compiles, so grammar.js alone "+
			"changes nothing.\nRun: tooling/generate-grammar.sh", stale)
	}
}

// compiledPatterns returns every string in the compiled grammar, joined.
//
// Decoded rather than searched raw, because the two files escape differently: grammar.js
// writes `b?"` where grammar.json writes `b?\"`. Decoding normalizes that away, which is the
// trap that made an earlier version of this check report `#[` missing from three files that
// all had it.
func compiledPatterns(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}

	var out []string
	var walk func(any)
	walk = func(v any) {
		switch t := v.(type) {
		case string:
			out = append(out, t)
		case []any:
			for _, e := range t {
				walk(e)
			}
		case map[string]any:
			for _, e := range t {
				walk(e)
			}
		}
	}
	walk(doc)
	if len(out) == 0 {
		return "", fmt.Errorf("%s: no strings found; the file changed shape", path)
	}
	return strings.Join(out, "\n"), nil
}

// TestQueryNamesOnlyRealNodes is the check that keeps Zed from losing Luna entirely.
//
// A tree-sitter query naming a node type the parser does not produce fails as a whole with
// TSQueryErrorNodeType, not that one pattern but the whole file, so highlights.scm getting
// ahead of parser.c takes highlighting down for the language rather than degrading. The
// generator gates on node-types.json for that reason; this asserts the result independently,
// so a hand edit to the query is caught too.
func TestQueryNamesOnlyRealNodes(t *testing.T) {
	root, err := spec.Root()
	if err != nil {
		t.Fatalf("locating the repository: %v", err)
	}
	have, err := loadNodeTypes(root)
	if err != nil {
		t.Fatal(err)
	}
	scm, err := os.ReadFile(filepath.Join(root, "tooling/zed-luna/languages/luna/highlights.scm"))
	if err != nil {
		t.Fatal(err)
	}

	// The trailing `@` is what separates a node pattern from an alternation inside a #match?
	// predicate: `"^(self)$"` is a regex in a string, not a node named `self`.
	named := regexp.MustCompile(`\(([a-z_]+)\)\s*@`).FindAllStringSubmatch(string(scm), -1)
	if len(named) == 0 {
		t.Fatal("no node patterns found in highlights.scm: this test checked nothing")
	}
	for _, m := range named {
		if !have[m[1]] {
			t.Errorf("highlights.scm captures (%s), which %s does not list — "+
				"run tooling/generate-grammar.sh, then go run ./cmd/gengrammar",
				m[1], nodeTypesPath)
		}
	}
}

// TestKeywordGroupsCoverEveryKeyword pins the three vocabularies together. A keyword whose
// class has no TextMate scope or no tree-sitter capture panics during generation, which is
// the right behaviour but a poor error message; this names the gap instead.
func TestKeywordGroupsCoverEveryKeyword(t *testing.T) {
	p, err := loadPatterns()
	if err != nil {
		t.Fatalf("reading §0: %v", err)
	}

	names := p.namesInOrder("keyword", nil)
	if len(names) == 0 {
		t.Fatal("§0 lists no keywords: this test checked nothing")
	}

	total := 0
	for _, r := range keywordRules(p) {
		total += strings.Count(r.Match, "|") + 1
	}
	if total != len(names) {
		t.Errorf("keyword rules cover %d alternatives, §0 has %d keywords", total, len(names))
	}
}

// TestTmLanguageIsValidJSON guards the one failure that would ship silently: VSCode drops a
// grammar it cannot parse and falls back to no highlighting at all, with no error a user
// sees.
func TestTmLanguageIsValidJSON(t *testing.T) {
	files, err := Files()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if filepath.Ext(f.Path) != ".json" {
			continue
		}
		var v map[string]any
		if err := json.Unmarshal(f.Content, &v); err != nil {
			t.Fatalf("%s: %v", f.Path, err)
		}
		if v["scopeName"] != "source.luna" {
			t.Errorf("%s: scopeName is %v, want source.luna — package.json references it by name",
				f.Path, v["scopeName"])
		}
	}
}

// TestEveryPatternCompilesAsRE2 checks what generation actually emits, defect corrections
// included. RE2 is the strictest of the three engines involved: oniguruma runs the
// TextMate grammars and Rust's regex-syntax runs tree-sitter's, and both accept everything
// RE2 does, so compiling here is the nearest thing to evidence the grammars load.
//
// Nearest, not proof. Nothing available locally can tell whether generated grammar.js is
// well-formed *JavaScript*, because that is settled by node inside the container; a pattern
// error is caught here, a structural one is not.
func TestEveryPatternCompilesAsRE2(t *testing.T) {
	p, err := loadPatterns()
	if err != nil {
		t.Fatal(err)
	}
	g := build(p)

	n := 0
	for _, pat := range buildJS(p).patterns {
		n++
		if _, err := regexp.Compile(pat); err != nil {
			t.Errorf("grammar.js: %q: %v", pat, err)
		}
	}
	var walk func(rules []Rule, where string)
	walk = func(rules []Rule, where string) {
		for i, r := range rules {
			for _, pat := range []string{r.Match, r.Begin, r.End} {
				if pat == "" {
					continue
				}
				n++
				if _, err := regexp.Compile(pat); err != nil {
					t.Errorf("%s[%d]: %q: %v", where, i, pat, err)
				}
			}
			walk(r.Patterns, where)
		}
	}
	for name, set := range g.Repository {
		walk(set.Patterns, name)
	}
	if n == 0 {
		t.Fatal("no patterns walked: this test checked nothing")
	}
}

// examples pulls the code spans out of §0's first column.
//
// Hand-scanned rather than matched with a regex, because a code span's fence is however many
// backticks it opened with and RE2 has no backreference to say "the same number again". The
// command rows need exactly that: their example is a backtick, so the column is written with
// a doubled fence. A single-backtick-delimited regex reads the padding spaces as the example
// and reports three rows failing for a reason that is entirely its own.
//
// Rows describing their token in prose ("spaces, tabs, newlines") yield none, and so do
// spans carrying an ellipsis or a placeholder (`#!…`, `<margin>"""`) which illustrate a
// shape rather than a lexeme.
func examples(col string) []string {
	var out []string
	for i := 0; i < len(col); {
		if col[i] != '`' {
			i++
			continue
		}
		open := i
		for i < len(col) && col[i] == '`' {
			i++
		}
		fence := col[open:i]

		end := strings.Index(col[i:], fence)
		if end < 0 {
			break
		}
		ex := col[i : i+end]
		i += end + len(fence)

		// CommonMark strips one space of padding from each side, which is how a span holds a
		// backtick of its own.
		if len(ex) > 1 && strings.HasPrefix(ex, " ") && strings.HasSuffix(ex, " ") {
			ex = ex[1 : len(ex)-1]
		}
		// The first column is prose, so a pipe in it is markdown-escaped, unlike the pattern
		// column, which uses `\x7c` for the same purpose. The OR and BAR rows show `\|\|`.
		ex = strings.ReplaceAll(ex, `\|`, "|")

		if ex == "" || strings.ContainsAny(ex, "…<") {
			continue
		}
		out = append(out, ex)
	}
	return out
}

// firstDiff locates where two generated files part company, so a stale-file failure names a
// line instead of printing two thousand of them.
func firstDiff(a, b string) string {
	la, lb := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := range max(len(la), len(lb)) {
		x, y := at(la, i), at(lb, i)
		if x != y {
			return "  line " + itoa(i+1) + "\n    on disk:   " + trunc(x) + "\n    generated: " + trunc(y)
		}
	}
	return ""
}

func at(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return "(end of file)"
}

func trunc(s string) string {
	if len(s) > 120 {
		return s[:120] + "…"
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}
