// Command gengrammar writes the editor highlighting grammars from lexer §0.
//
//	go run ./cmd/gengrammar          # write the three artifacts under tooling/
//	go run ./cmd/gengrammar -check   # report drift, write nothing
//
// Run it after any ruling that touches the token table. TestGeneratedFilesAreCurrent runs
// -check's comparison in the suite, so forgetting is a test failure rather than a grammar that
// quietly colours a retired keyword, which is what cmd/grammarcheck found all three of these
// doing.
//
// grammar.js is deliberately not generated. Changing it means regenerating parser.c through
// the pinned container (tooling/generate-grammar.sh), so it stays hand-written and
// cmd/grammarcheck stays pointed at it.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"luna/internal/grammar"
	"luna/internal/spec"
)

func main() {
	check := flag.Bool("check", false, "report which files are stale and exit non-zero; write nothing")
	flag.Parse()

	root, err := spec.Root()
	if err != nil {
		fail(err)
	}
	files, err := grammar.Files()
	if err != nil {
		fail(err)
	}

	stale := 0
	for _, f := range files {
		path := filepath.Join(root, f.Path)
		old, readErr := os.ReadFile(path)
		current := readErr == nil && bytes.Equal(old, f.Content)

		switch {
		case current:
			fmt.Printf("  ok       %s\n", f.Path)
		case *check:
			stale++
			fmt.Printf("  STALE    %s\n", f.Path)
		default:
			if err := os.WriteFile(path, f.Content, 0o644); err != nil {
				fail(err)
			}
			fmt.Printf("  written  %s\n", f.Path)
		}
	}

	if stale > 0 {
		fmt.Fprintf(os.Stderr, "\ngengrammar: %d file(s) stale; run `go run ./cmd/gengrammar`\n", stale)
		os.Exit(1)
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "gengrammar: %v\n", err)
	os.Exit(1)
}
