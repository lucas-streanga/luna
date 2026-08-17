package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"luna/internal/ebnf"
	"luna/oracle/lexer"
	"luna/oracle/source"
	"luna/oracle/token"
)

// The `.parse` golden format, described by `testdata/golden.md`. It was fixed before the parser
// existed because one corpus feeds two tools — `internal/ebnf` answers whether grammar.md
// derives the source section, exactly once, and the parser is held to the tree and the
// diagnostics — and fixing the shape first is what let the second join without a migration.

// GoldenSeparator is the line that divides a case's sections.
const GoldenSeparator = "---"

// GoldenExtension is the golden file suffix.
const GoldenExtension = ".parse"

// GoldenErrorDir is the subdirectory holding cases that must raise a diagnostic. The split is
// checked rather than trusted (golden.md §1): a misfiled case still passes, it just stops
// meaning what its directory says.
const GoldenErrorDir = "error_producing"

// Golden is one `.parse` file: Luna source, the tree it must produce, and the diagnostics it
// must raise. A section that is absent is distinguished from one that is present and empty,
// because "not yet pinned" and "pinned as nothing" are different claims.
type Golden struct {
	Path        string
	Source      string
	Tree        string
	Diagnostics string
	HasTree     bool
	HasDiags    bool
}

// Name is the case's file name without the extension, which is what a test reports.
func (c *Golden) Name() string {
	return strings.TrimSuffix(filepath.Base(c.Path), GoldenExtension)
}

// ExpectsDiagnostics reports whether the case lives in GoldenErrorDir.
func (c *Golden) ExpectsDiagnostics() bool {
	return filepath.Base(filepath.Dir(c.Path)) == GoldenErrorDir
}

// ReadGolden parses one golden file.
func ReadGolden(path string) (*Golden, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c := &Golden{Path: path}
	sections := splitGoldenSections(string(raw))
	switch len(sections) {
	case 3:
		c.Diagnostics, c.HasDiags = sections[2], true
		fallthrough
	case 2:
		c.Tree, c.HasTree = sections[1], true
		fallthrough
	case 1:
		c.Source = sections[0]
	default:
		return nil, fmt.Errorf("%s: %d sections; a case has source, tree, and optionally diagnostics",
			path, len(sections))
	}
	if c.Source == "" {
		return nil, fmt.Errorf("%s: empty source section", path)
	}
	return c, nil
}

// splitGoldenSections cuts on lines that are exactly the separator. The source keeps its
// trailing newline — everything before the first separator is the input, and real files end
// with one.
func splitGoldenSections(s string) []string {
	var out []string
	var cur strings.Builder
	for _, line := range strings.SplitAfter(s, "\n") {
		if strings.TrimSuffix(line, "\n") == GoldenSeparator && strings.HasSuffix(line, "\n") {
			out = append(out, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteString(line)
	}
	out = append(out, cur.String())
	return out
}

// Bytes renders the case back to file form, so that a regenerated golden and a hand-written one
// are byte-identical when they mean the same thing.
func (c *Golden) Bytes() []byte {
	var b strings.Builder
	b.WriteString(c.Source)
	if c.HasTree {
		b.WriteString(GoldenSeparator + "\n")
		b.WriteString(c.Tree)
	}
	if c.HasDiags {
		b.WriteString(GoldenSeparator + "\n")
		b.WriteString(c.Diagnostics)
	}
	return []byte(b.String())
}

// ReadGoldenDir returns every case under dir, including GoldenErrorDir, sorted by path.
func ReadGoldenDir(dir string) ([]*Golden, error) {
	var out []*Golden
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, GoldenExtension) {
			return nil
		}
		c, err := ReadGolden(path)
		if err != nil {
			return err
		}
		out = append(out, c)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// --- lexing ----------------------------------------------------------------------------

// LexedGolden is one source in both shapes a case needs: the recognizer runs over the filtered
// view grammar §0 is defined over, the builder over the full stream Parse takes (§4.4).
type LexedGolden struct {
	File   *source.File
	Tokens []token.Token // every token, trivia and all
	Input  []ebnf.Token  // the filtered view, kind plus lexeme, which is what a terminal matches

	// Filtered position → index into Tokens, since a derivation numbers tokens in the filtered
	// stream and an event carries the real index. Dies with the bridge; the parser needs no such
	// table (§2.2).
	index []int
}

// LexGolden runs the real lexer and builds both views.
func LexGolden(name, src string) (*LexedGolden, error) {
	f, err := source.New(name, src)
	if err != nil {
		return nil, err
	}
	tokens, diags := lexer.Lex(f)
	if len(diags) > 0 {
		return nil, fmt.Errorf("%d lexical diagnostics, the first at offset %d",
			len(diags), diags[0].Primary.Offset)
	}
	out := &LexedGolden{File: f, Tokens: tokens}
	for i, tk := range tokens {
		if tk.Kind.IsTrivia() {
			continue
		}
		out.Input = append(out.Input, ebnf.Token{Kind: tk.Kind, Text: f.Slice(tk.Offset, tk.Len)})
		out.index = append(out.index, i)
	}
	return out, nil
}
