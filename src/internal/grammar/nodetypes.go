package grammar

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// nodeTypesPath is tree-sitter's own manifest of what the committed parser produces. It is
// generated beside parser.c by `tree-sitter generate`, so it describes the parser that
// actually ships rather than the grammar.js someone has edited since.
const nodeTypesPath = "tooling/tree-sitter-luna/src/node-types.json"

// loadNodeTypes reads the node types the committed parser can produce.
//
// Reading the generated manifest rather than parsing grammar.js is the whole point: grammar.js
// is the input to a generation step this repo runs by hand, so the two disagree for exactly
// as long as it takes someone to run tooling/generate-grammar.sh. A query built from
// grammar.js would be correct about the intent and wrong about the parser, which is the
// direction that breaks — see HighlightsSCM.
func loadNodeTypes(root string) (map[string]bool, error) {
	raw, err := os.ReadFile(filepath.Join(root, nodeTypesPath))
	if err != nil {
		return nil, fmt.Errorf("grammar: %w", err)
	}

	var entries []struct {
		Type  string `json:"type"`
		Named bool   `json:"named"`
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("grammar: %s: %w", nodeTypesPath, err)
	}

	have := make(map[string]bool, len(entries))
	for _, e := range entries {
		// Only named nodes are addressable as `(name)` in a query; anonymous ones are matched
		// by their literal text and are not what nodeCaptures names.
		if e.Named {
			have[e.Type] = true
		}
	}
	if len(have) == 0 {
		return nil, fmt.Errorf("grammar: %s lists no named node types", nodeTypesPath)
	}
	return have, nil
}
