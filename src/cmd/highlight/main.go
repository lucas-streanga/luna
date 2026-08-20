// Command highlight renders Luna as HTML, coloured by the oracle rather than by a grammar.
//
//	go run ./cmd/highlight a.luna                  # one file -> a <pre> block
//	go run ./cmd/highlight -md specs/build/lexer.md   # rewrite every ```luna fence
//	go run ./cmd/highlight -css > luna.css         # the stylesheet those classes want
//
// This is the docs half of the answer to grammar drift. cmd/grammarcheck measures how far
// the three hand-written grammars under tooling/ have slid from §0; this removes the need
// for one of them, because documentation is rendered at build time and has no reason to
// approximate a lexer it could simply run.
//
// -strict is the part worth having beyond colour. Every block goes through the real lexer,
// so a snippet with an unterminated literal or a stray byte fails the build instead of being
// confidently mis-coloured, a check the spec's examples have never had.
//
// # Wiring
//
// The site serves the stylesheet once and pipes each document through -md:
//
//	go run ./cmd/highlight -css > public/luna.css
//	for f in $(grep -rl '```luna' ../specs); do go run ./cmd/highlight -md -strict "$f"; done
//
// Nothing here knows about Shiki, Astro or any other renderer, which is the point: the interface
// is a markdown file in and a markdown file out, so the site's pipeline keeps whatever it does
// with everything that is not Luna.
//
// Classes rather than the inline colours Shiki bakes into each span, so light and dark are
// one stylesheet instead of two renders, and retheming does not mean regenerating pages.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"luna/internal/highlight"
	"luna/oracle/source"
)

func main() {
	var (
		css    = flag.Bool("css", false, "write the default stylesheet and exit")
		md     = flag.Bool("md", false, "treat input as markdown and rewrite its ```luna fences")
		strict = flag.Bool("strict", false, "exit non-zero if any block has lexical diagnostics")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr,
			"usage: highlight [-md] [-strict] [file]\n       highlight -css\n\nreads stdin if no file is given\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *css {
		fmt.Print(highlight.StyleSheet)
		return
	}

	name, text, err := read(flag.Args())
	if err != nil {
		fail(err)
	}

	if *md {
		out, problems := highlight.Markdown(name, text)
		fmt.Print(out)
		report(problems, *strict)
		return
	}

	f, err := source.New(name, text)
	if err != nil {
		// Ingress rejects invalid UTF-8 and a leading BOM before any tokenizing
		// (lexical-structure §1), so there is no stream to colour.
		fail(err)
	}
	out, errs := highlight.Render(f)
	fmt.Println(out)

	var problems []highlight.Problem
	for _, d := range errs.Sorted() {
		pos := f.Position(d.Primary.Offset)
		problems = append(problems, highlight.Problem{
			Path: name, Line: pos.Line, Column: pos.Column,
			Code: string(d.Code), Message: d.Description,
		})
	}
	report(problems, *strict)
}

// report writes the problems to stderr in the file:line:col form every editor and CI log
// already knows how to jump to, and decides the exit status.
//
// Without -strict they are advisory: a page still renders, which is what an author editing
// prose around a half-written snippet wants. With it they are fatal, which is what a
// release build wants. Neither is the right default for the other, so the flag exists.
func report(problems []highlight.Problem, strict bool) {
	for _, p := range problems {
		fmt.Fprintf(os.Stderr, "%s:%d:%d: %s: %s\n", p.Path, p.Line, p.Column, p.Code, p.Message)
	}
	if len(problems) > 0 && strict {
		fmt.Fprintf(os.Stderr, "highlight: %d problem(s)\n", len(problems))
		os.Exit(1)
	}
}

func read(args []string) (name, text string, err error) {
	if len(args) == 0 {
		b, err := io.ReadAll(os.Stdin)
		return "<stdin>", string(b), err
	}
	b, err := os.ReadFile(args[0])
	return args[0], string(b), err
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "highlight: %v\n", err)
	os.Exit(1)
}
