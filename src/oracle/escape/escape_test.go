package escape_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"luna/internal/spec"
	"luna/oracle/escape"
)

// TestTableMatchesSpec pins `allowed` against string §5.1, which the spec calls "the one
// authority" and which lexical-structure §4, bytes §7 and command §2 all defer to.
//
// It was the last spec table without a pin. §0's tokens have one and §11's and §12's codes
// have one; this is the table whose drift is hardest to see from anywhere else, because the
// lexer's goldens were generated from current behaviour and would move with it rather than
// object.
func TestTableMatchesSpec(t *testing.T) {
	contexts := map[string]escape.Context{
		`"…"`:  escape.StringDq,
		`'…'`:  escape.StringSq,
		`b"…"`: escape.Bytes,
		"`…`":  escape.Command,
		`~"…"`: escape.Regex,
	}

	root, err := spec.Root()
	if err != nil {
		t.Fatalf("locating the spec: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "specs", "types", "string.md"))
	if err != nil {
		t.Fatalf("reading string.md: %v", err)
	}

	checked := 0
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(line, "| ") {
			continue
		}
		cells := strings.SplitN(strings.Trim(line, "| "), " | ", 2)
		if len(cells) != 2 {
			continue
		}
		key := firstSpan(cells[0])
		ctx, ok := contexts[key]
		if !ok {
			continue // a row of some other table, or §5.1's header
		}
		checked++

		// The regex row is prose, not a set: its escapes are RE2's, passed through undecoded,
		// and Luna decodes exactly one. Allowed reports "" for it on purpose, since an empty row
		// would read as "nothing is allowed here", the opposite of the truth. So what there
		// is to check is that it stays that way, and Check's passthrough is what tests the
		// behaviour.
		if ctx == escape.Regex {
			if got := escape.Allowed(ctx); got != "" {
				t.Errorf("%s: Allowed returned %q; the regex has no table by design", key, got)
			}
			continue
		}

		want := escapeChars(cells[1])
		if extra, ok := deviations[ctx]; ok {
			want += extra
		}
		if got := escape.Allowed(ctx); sorted(got) != sorted(want) {
			t.Errorf("%s: Go allows %q, §5.1 (with known deviations) gives %q",
				key, sorted(got), sorted(want))
		}
	}

	if checked != len(contexts) {
		t.Fatalf("matched %d of §5.1's %d context rows; the table changed shape",
			checked, len(contexts))
	}
}

// Both filters below are needed, for different rows. The double-quoted row puts an em dash
// *inside* a parenthetical and then lists `\u{H…}` after it, so cutting at the dash first
// would drop a real escape. The bytes row uses a dash to introduce escapes it **forbids**
// ("no `\$`, no `\u{}`"), which read exactly like the positive ones to a naive scan.
var (
	parenthetical = regexp.MustCompile(`\([^)]*\)`)
	fence         = regexp.MustCompile("`+")
)

func escapeChars(cell string) string {
	cell = parenthetical.ReplaceAllString(cell, "")
	if i := strings.Index(cell, "—"); i >= 0 {
		cell = cell[:i]
	}

	var out []rune
	for _, span := range spans(cell) {
		if r := []rune(span); len(r) >= 2 && r[0] == '\\' {
			out = append(out, r[1])
		}
	}
	return string(out)
}

// spans pulls the code spans out of a cell, honouring variable fence widths. The command
// row is written with a doubled fence because its escape contains a backtick.
func spans(cell string) []string {
	var out []string
	for {
		open := fence.FindStringIndex(cell)
		if open == nil {
			return out
		}
		mark, rest := cell[open[0]:open[1]], cell[open[1]:]
		end := strings.Index(rest, mark)
		if end < 0 {
			return out
		}
		out = append(out, strings.TrimSpace(rest[:end]))
		cell = rest[end+len(mark):]
	}
}

func firstSpan(cell string) string {
	if s := spans(cell); len(s) > 0 {
		return s[0]
	}
	return ""
}

func sorted(s string) string {
	r := []rune(s)
	for i := 1; i < len(r); i++ {
		for j := i; j > 0 && r[j] < r[j-1]; j-- {
			r[j], r[j-1] = r[j-1], r[j]
		}
	}
	return string(r)
}

// deviations are characters Go allows that §5.1's row does not list.
//
// One entry, and it is a spec gap rather than a bug: bytes §7 rules that the quote style
// does not matter, and a literal must be able to escape whichever delimiter closes it, so
// `b'don\'t'` is legal for the same reason `b"say \""` is. escape.go calls that "an
// inference from two rules rather than a third rule stated outright", and §5.1, which the
// spec calls the *one authority*, does not state it.
//
// Recorded here rather than fudged into the comparison, so that adding `\'` to §5.1's bytes
// row fails this test and asks for the entry to be deleted.
var deviations = map[escape.Context]string{
	escape.Bytes: "'",
}
