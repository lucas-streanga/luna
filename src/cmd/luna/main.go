// Command luna is a thin main over oracle/driver (driver.md §1).
//
//	go run ./cmd/luna app.luna
//
// It runs the batch pipeline as far as the passes go (§1.0 discover, §1.1 lex, §1.2 import
// validation) and prints the diagnostics as JSON.
//
// The JSON is throwaway. Diagnostic rendering is its own concern and gets its own module:
// line and column, a caret under the span, `luna explain <code>`. This exists so there is
// something to run today, and every field it prints is one the real renderer will read from
// the same Diagnostic.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"luna/oracle/driver"
)

type jsonReport struct {
	Reached     string           `json:"reached"`
	Files       []string         `json:"files"`
	Layers      [][]string       `json:"layers"`
	Diagnostics []jsonDiagnostic `json:"diagnostics"`
}

type jsonDiagnostic struct {
	Code    string `json:"code"`
	Title   string `json:"title"`
	File    string `json:"file"`
	Offset  int    `json:"offset"`
	Length  int    `json:"length"`
	Message string `json:"message"`
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: luna <entry.luna>")
		os.Exit(2)
	}
	entry := os.Args[1]

	// modules §3: the root is the directory of the entry file, for an application and a
	// library alike. os.DirFS of that directory is what makes a file's path within the FS its
	// module path.
	res, err := driver.Build(os.DirFS(filepath.Dir(entry)), filepath.Base(entry))
	if err != nil {
		fmt.Fprintf(os.Stderr, "luna: %v\n", err)
		os.Exit(1)
	}

	report := jsonReport{Reached: res.Reached.String(), Layers: res.Graph.Layers}
	for _, f := range res.Files {
		report.Files = append(report.Files, f.Path)
	}
	for _, d := range res.Diagnostics {
		report.Diagnostics = append(report.Diagnostics, jsonDiagnostic{
			Code:    string(d.Code),
			Title:   d.Title(),
			File:    d.Primary.Filename,
			Offset:  d.Primary.Offset,
			Length:  d.Primary.Length,
			Message: d.Description,
		})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintf(os.Stderr, "luna: %v\n", err)
		os.Exit(1)
	}
	if len(report.Diagnostics) > 0 {
		os.Exit(1)
	}
}
