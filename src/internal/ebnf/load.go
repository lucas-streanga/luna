package ebnf

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"luna/internal/spec"
)

// SpecPath is grammar.md, relative to the repository root.
const SpecPath = "specs/build/grammar.md"

// FenceLabel is the fence grammar.md's productions carry. It is deliberately not `luna`:
// R258 made every ```luna block a complete Luna file, and productions are not Luna, so a
// distinct label keeps the corpus tools (internal/spec, internal/highlight) unaware of them.
const FenceLabel = "```ebnf"

// Load reads grammar.md and returns the grammar it defines.
//
// It reads the spec through the `src/specs` symlink, as every other spec reader here does,
// and for the same reason: resolving the link would put the file outside the module and make
// `go test` cache a pass over a spec it never opened (internal/spec's findRoot).
func Load() (*Grammar, error) {
	prods, err := LoadProductions()
	if err != nil {
		return nil, err
	}
	return New(prods), nil
}

// LoadProductions returns the productions without indexing them, for callers that want to
// count or inspect before building.
func LoadProductions() ([]Prod, error) {
	root, err := spec.Root()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(root, SpecPath))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", SpecPath, err)
	}
	src, n := fencedBlocks(string(raw))
	if n == 0 {
		return nil, fmt.Errorf("%s: no %s blocks found; the fence label changed and every "+
			"check over the grammar would have passed while reading nothing", SpecPath, FenceLabel)
	}
	return Parse(src)
}

// BlockCount reports how many production blocks grammar.md carries, which the tests assert
// against §0's nine sections — a reader that silently found fewer would make every check
// below vacuous, the same fail-open internal/spec guards with its own armed check.
func BlockCount() (int, error) {
	root, err := spec.Root()
	if err != nil {
		return 0, err
	}
	raw, err := os.ReadFile(filepath.Join(root, SpecPath))
	if err != nil {
		return 0, err
	}
	_, n := fencedBlocks(string(raw))
	return n, nil
}

// fencedBlocks concatenates every ```ebnf block, and counts them. Fence recognition matches
// internal/spec exactly — an unindented opening line, closed by the next line beginning with
// a fence — because the two readers walk the same corpus and a divergence between them would
// be invisible.
func fencedBlocks(md string) (string, int) {
	var b strings.Builder
	n := 0
	lines := strings.Split(md, "\n")
	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != FenceLabel {
			continue
		}
		n++
		for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```"); i++ {
			b.WriteString(lines[i])
			b.WriteByte('\n')
		}
	}
	return b.String(), n
}
