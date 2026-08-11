// Package spec reads the lexer specification's own tables so tests can assert
// against them directly rather than against a transcription.
//
// The spec is the source of truth for the token inventory (lexer §0) and for the
// counts that summarize it (lexer §10). Anything that hardcodes either will drift,
// and drift between prose and table is a defect this project has already shipped
// once: R232 fixed a "47 patterns" claim standing over a 49-row table. Reading the
// markdown is what lets a test hold all three parties — table, prose, and code — to
// the same numbers.
//
// Test-only in practice, but an ordinary package: the spec-literal reference lexer
// (lexer-testing-plan §7) consumes the same rows, and a helper defined in another
// package's tests would be unreachable from it.
package spec

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// LexerSpecPath is the spec's location relative to the repository root.
const LexerSpecPath = "specs/build/lexer.md"

// Row is one row of lexer §0's token table.
//
// §0 has more rows than tokens, deliberately, and the difference is what Name and
// Note record. DOUBLE and BYTES each own two rows (point/exponent form, dq/sq), so
// two rows share one Name. Error productions have no Name at all: they are
// recognized shapes that raise a diagnostic rather than producing a token.
type Row struct {
	Line     int    // 1-based line in lexer.md, so a failure can name the row
	Token    string // column 1, the lexeme illustration
	Name     string // column 2's backticked name; empty for an error production
	Note     string // column 2's parenthetical, e.g. "point form"
	Category string // column 3's category word, without the section
	Section  string // column 3's section, e.g. "§4"
	Pattern  string // column 4, pattern plus any trailing commentary
}

// IsToken reports whether the row defines a token. Rows that do not are the error
// productions, which §0 categorizes "error" precisely so a count can exclude them.
func (r Row) IsToken() bool { return r.Name != "" }

// Counts holds lexer §10's claimed totals — the prose summary, not the table.
// Comparing these against the table is the check that catches the R232 defect.
type Counts struct {
	Tokens     int
	Rows       int
	ByCategory map[string]int
}

// Inventory is lexer §0's table together with §10's claimed counts.
type Inventory struct {
	Path   string
	Rows   []Row
	Claims Counts
}

// Tokens returns the distinct token names in first-appearance order, paired with
// their category. Rows sharing a Name collapse to one entry; error productions are
// omitted.
func (inv *Inventory) Tokens() []Row {
	seen := make(map[string]bool, len(inv.Rows))
	out := make([]Row, 0, len(inv.Rows))
	for _, r := range inv.Rows {
		if !r.IsToken() || seen[r.Name] {
			continue
		}
		seen[r.Name] = true
		out = append(out, r)
	}
	return out
}

// Actual computes the counts the table really carries, for comparison with Claims.
func (inv *Inventory) Actual() Counts {
	c := Counts{Rows: len(inv.Rows), ByCategory: map[string]int{}}
	for _, r := range inv.Tokens() {
		c.Tokens++
		c.ByCategory[r.Category]++
	}
	return c
}

// Load finds the repository root by walking up from the working directory and reads
// the lexer spec from it. Tests run with the working directory set to their own
// package, so walking is what keeps this working when a package moves.
func Load() (*Inventory, error) {
	root, err := findRoot()
	if err != nil {
		return nil, err
	}
	return LoadFrom(filepath.Join(root, LexerSpecPath))
}

// LoadFrom reads the lexer spec from an explicit path.
func LoadFrom(path string) (*Inventory, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("spec: %w", err)
	}
	// Read-only: a Close error carries no information, nothing having been buffered.
	defer func() { _ = f.Close() }()

	inv := &Inventory{Path: path}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	inTable := false
	// §10's summary is one wrapped paragraph, and a category can be split from its
	// count across a line break ("**37**" ending one line, "operator" opening the
	// next). So the paragraph is accumulated whole and parsed once.
	var counts []string
	inCounts := false

	for line := 1; sc.Scan(); line++ {
		text := sc.Text()

		if inCounts {
			if strings.TrimSpace(text) == "" {
				inv.Claims = parseCounts(strings.Join(counts, " "))
				inCounts = false
			} else {
				counts = append(counts, text)
				continue
			}
		} else if totalsRe.MatchString(text) {
			inCounts, counts = true, []string{text}
			continue
		}

		switch {
		case strings.HasPrefix(text, "| Token | Name | Category |"):
			inTable = true
			continue
		case inTable && strings.HasPrefix(text, "|-"):
			continue // the header separator
		case inTable && !strings.HasPrefix(text, "| "):
			inTable = false // a blank line or prose ends the table
		}

		if inTable {
			r, err := parseRow(text, line)
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %w", path, line, err)
			}
			inv.Rows = append(inv.Rows, r)
		}
	}
	if inCounts { // the summary ran to end of file
		inv.Claims = parseCounts(strings.Join(counts, " "))
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("spec: reading %s: %w", path, err)
	}
	if len(inv.Rows) == 0 {
		return nil, fmt.Errorf("spec: no §0 table found in %s", path)
	}
	if inv.Claims.Tokens == 0 {
		return nil, fmt.Errorf("spec: no §10 count summary found in %s", path)
	}
	return inv, nil
}

var (
	// Column 2 is `NAME`, or `NAME` (note), or *(error production)*.
	nameRe = regexp.MustCompile("^`([A-Z][A-Z_0-9]*)`(?:\\s+\\((.+)\\))?$")
	// Column 3 is "category §n".
	catRe = regexp.MustCompile(`^([a-z]+)\s+(§[0-9.]+)$`)
	// §10's header: "**126 tokens over 130 rows.**"
	totalsRe = regexp.MustCompile(`\*\*(\d+) tokens over (\d+) rows\.\*\*`)
	// §10's per-category counts: "**49** keyword".
	perCatRe = regexp.MustCompile(`\*\*(\d+)\*\*\s+([a-z]+)`)
)

// parseRow splits one table row.
//
// Splitting on " | " rather than "|" is load-bearing: the OR and BAR rows carry
// escaped pipes (`\|\|`) inside column 1, and splitting on the bare character
// shreds them. SplitN caps the count at four so a pattern containing " | " would
// land intact in the last field rather than overflowing into a fifth.
func parseRow(text string, line int) (Row, error) {
	body := strings.TrimSuffix(strings.TrimPrefix(text, "| "), " |")
	f := strings.SplitN(body, " | ", 4)
	if len(f) != 4 {
		return Row{}, fmt.Errorf("want 4 columns, got %d: %q", len(f), text)
	}

	r := Row{Line: line, Token: f[0], Pattern: f[3]}

	col2 := strings.TrimSpace(f[1])
	if col2 != "*(error production)*" {
		m := nameRe.FindStringSubmatch(col2)
		if m == nil {
			return Row{}, fmt.Errorf("unrecognized name column: %q", col2)
		}
		r.Name, r.Note = m[1], m[2]
	}

	m := catRe.FindStringSubmatch(strings.TrimSpace(f[2]))
	if m == nil {
		return Row{}, fmt.Errorf("unrecognized category column: %q", f[2])
	}
	r.Category, r.Section = m[1], m[2]

	return r, nil
}

// parseCounts reads §10's prose summary from the whole accumulated paragraph.
//
// The totals are matched as a phrase ("126 tokens over 130 rows") rather than as
// two bold numbers, so they cannot be confused with the per-category counts that
// follow; and the per-category pattern requires the count and the category word to
// be adjacent, so a bolded number elsewhere in the paragraph contributes nothing.
func parseCounts(text string) Counts {
	c := Counts{ByCategory: map[string]int{}}
	if m := totalsRe.FindStringSubmatch(text); m != nil {
		c.Tokens, _ = strconv.Atoi(m[1])
		c.Rows, _ = strconv.Atoi(m[2])
	}
	for _, pc := range perCatRe.FindAllStringSubmatch(text, -1) {
		n, _ := strconv.Atoi(pc[1])
		c.ByCategory[pc[2]] = n
	}
	return c
}

// findRoot walks up from the working directory looking for the lexer spec.
func findRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, LexerSpecPath)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("spec: no " + LexerSpecPath + " in any parent directory")
		}
		dir = parent
	}
}
