package spec

import (
	"os"
	"path/filepath"
	"strings"
)

// Block is one fenced code block labelled `luna` in the spec.
//
// The corpus is worth three things (lexer-testing-plan §9) and this type serves all of
// them: a regression corpus, fuzz seeds, and (once a parser exists) a parse gate, which
// is the strong one. Path and Line are carried for that third use, where a failure has to
// name the block it came from.
type Block struct {
	Path   string // the .md file, relative to the repository root
	Line   int    // 1-based line of the opening fence
	Source string // the block's contents, with a trailing newline
}

// LunaBlocks reads every ```luna block in the spec.
//
// It walks from Root, so it carries the same requirement Load does: the `src/specs`
// symlink must exist, or the spec is invisible to Go's test cache and every check over it
// silently goes stale. findRoot refuses rather than reaching past the module.
func LunaBlocks() ([]Block, error) {
	root, err := Root()
	if err != nil {
		return nil, err
	}

	var out []Block
	if err := walkMarkdown(filepath.Join(root, "specs"), root, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// walkMarkdown recurses with os.ReadDir rather than filepath.WalkDir, because `specs` is
// the `src/specs` symlink and WalkDir does not follow one: it lstats the link, sees a
// non-directory, and returns having found nothing.
//
// Reading *through* the link matters for more than finding the files. Resolving it to the
// real path would put every file outside the module, and `go help test` tracks only files
// a test opened *within* its module, so the corpus checks would go silently cacheable,
// which is the exact fail-open the symlink was added to prevent.
func walkMarkdown(dir, root string, out *[]Block) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if e.IsDir() {
			if err := walkMarkdown(path, root, out); err != nil {
				return err
			}
			continue
		}
		if filepath.Ext(path) != ".md" {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		*out = append(*out, blocksIn(rel, string(raw))...)
	}
	return nil
}

// blocksIn extracts the labelled blocks of one file. An unclosed fence yields whatever
// followed it, which is the reading a markdown renderer takes and needs no error of its
// own: the spec's fences are checked by eye, not by this.
func blocksIn(path, md string) []Block {
	var out []Block
	lines := strings.Split(md, "\n")
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "```luna" {
			continue
		}
		open := i
		var body []string
		for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```"); i++ {
			body = append(body, lines[i])
		}
		out = append(out, Block{
			Path:   path,
			Line:   open + 1,
			Source: strings.Join(body, "\n") + "\n",
		})
	}
	return out
}
