// Command lexdump prints the token stream for a file or for stdin.
//
// The tool you want when a golden surprises you and you would rather try three variants than
// write three files:
//
//	go run ./cmd/lexdump file.luna
//	echo 'let x = ~"a"bar;' | go run ./cmd/lexdump
//	go run ./cmd/lexdump -golden file.luna > oracle/lexer/testdata/case.lex
//
// Token lines use testdata/*.lex notation (FORMAT.md), which is what these are reviewed
// against, so there is no second vocabulary to learn. Diagnostics also carry their line,
// column and description, which a golden does not record; -golden drops those so the output
// pastes straight into a case.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"

	"luna/oracle/diagnostic"
	"luna/oracle/lexer"
	"luna/oracle/source"
	"luna/oracle/token"
)

func main() {
	golden := flag.Bool("golden", false, "emit exactly the testdata/*.lex dump format")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: lexdump [-golden] [file]\n\nreads stdin if no file is given\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	name, text, err := read(flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "lexdump: %v\n", err)
		os.Exit(1)
	}

	f, err := source.New(name, text)
	if err != nil {
		// Ingress rejects invalid UTF-8 and a leading BOM before any tokenizing
		// (lexical-structure §1), so there is no stream to print.
		fmt.Fprintf(os.Stderr, "lexdump: %v\n", err)
		os.Exit(1)
	}

	toks, errs := lexer.Lex(f)
	if *golden {
		fmt.Print(text)
		fmt.Println("---")
	}
	for _, line := range dump(f, toks, errs, *golden) {
		fmt.Println(line)
	}

	// Exit non-zero when the file has lexical errors, so the tool composes with a shell
	// the way any other checker does.
	if !errs.Empty() {
		os.Exit(1)
	}
}

// dump renders tokens and diagnostics as one sequence in source order.
//
// Interleaving is not presentation: it is what testdata/*.lex records, so -golden output
// that appended the diagnostics would not round-trip through the harness. Ties go to the
// token, matching the harness, which appends tokens first and sorts stably.
func dump(f *source.File, toks []token.Token, errs diagnostic.List, golden bool) []string {
	type entry struct {
		at   int
		text string
	}

	out := make([]entry, 0, len(toks)+len(errs))
	for _, t := range toks {
		out = append(out, entry{t.Offset, fmt.Sprintf("%s %d..%d %s",
			t.Kind, t.Offset, t.End(), strconv.Quote(f.Slice(t.Offset, t.Len)))})
	}
	for _, d := range errs {
		text := fmt.Sprintf("!%s %d..%d", d.Code, d.Primary.Offset, d.Primary.End())
		if !golden {
			p := f.Position(d.Primary.Offset)
			text += fmt.Sprintf("  %d:%d  %s: %s", p.Line, p.Column, d.Title(), d.Description)
		}
		out = append(out, entry{d.Primary.Offset, text})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].at < out[j].at })

	lines := make([]string, len(out))
	for i, e := range out {
		lines[i] = e.text
	}
	return lines
}

// read takes the single optional file argument, falling back to stdin. The name matters:
// it is what every diagnostic span carries (R240).
func read(args []string) (name, text string, err error) {
	if len(args) > 1 {
		return "", "", fmt.Errorf("expected at most one file, got %d", len(args))
	}
	if len(args) == 0 {
		b, err := io.ReadAll(os.Stdin)
		return "<stdin>", string(b), err
	}
	b, err := os.ReadFile(args[0])
	return args[0], string(b), err
}
