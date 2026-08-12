package spec

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadCodes reads one spec file's error-summary table — the `| Code | Title |` grid every
// stage keeps (lexer §11, modules §12, R240).
//
// Separate from LoadFrom because that one is the *lexer's* reader and insists on §0's token
// table and §10's counts alongside. Every other stage has a code table and nothing else of
// that shape, so the table parse is what generalizes.
func LoadCodes(relPath string) ([]CodeRow, error) {
	root, err := findRoot()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(root, relPath)

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("spec: %w", err)
	}
	// Read-only: a Close error carries no information, nothing having been buffered.
	defer func() { _ = f.Close() }()

	var rows []CodeRow
	inCodes := false
	sc := bufio.NewScanner(f)
	for line := 0; sc.Scan(); {
		line++
		text := strings.TrimSpace(sc.Text())

		switch {
		case strings.HasPrefix(text, "| Code | Title |"):
			inCodes = true
			continue
		case !inCodes:
			continue
		case strings.HasPrefix(text, "|-"):
			continue // the header separator
		case !strings.HasPrefix(text, "| "):
			inCodes = false // a blank line or prose ends the table
			continue
		}

		r, err := parseCodeRow(text, line)
		if err != nil {
			return nil, fmt.Errorf("spec: %s:%d: %w", relPath, line, err)
		}
		rows = append(rows, r)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("spec: reading %s: %w", path, err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("spec: no error-summary table found in %s", relPath)
	}
	return rows, nil
}
