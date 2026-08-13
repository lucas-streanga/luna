// Command ambiguity enumerates every sentence grammar.md derives up to a length and reports
// the ones that derive more than one way.
//
// This is a **proof over a fixed grammar**, not a regression over changing code: its answer can
// only change when grammar.md does. So it is an opt-in step (`./check.sh --ambiguity`) rather
// than part of the gate, in the same category as fuzzing and mutation — the gate keeps a
// three-token sweep, which costs half a second and would still catch a grammar broken outright.
//
//	ambiguity -sweep                          the documented table, ~40s
//	ambiguity -start Expr -len 4              one sub-language, deeper
//	ambiguity -start ImportSpec -len 6 -spellings
//
// The cost is exponential in -len — about sixteen times per token on this grammar — so the
// reachable length differs between one start symbol and another, and finding that out is an
// interactive job rather than a CI one.
//
// Exit status is 1 if anything was found, or if a run was not exhaustive: a capped run proves
// nothing by being clean, so it is not reported as a pass.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"luna/internal/ebnf"
)

// sweep is the documented table: what an opt-in run covers, and the lengths each start reaches
// before the shared table outgrows the machine. Every entry has been run to completion here.
//
// The starts overlap on purpose — the grammar is one connected component, so a smaller start
// symbol narrows the output rather than the work, and its budget therefore buys depth in that
// sub-language that File cannot reach at any affordable length.
var sweep = []ebnf.Bound{
	{Start: "File", MaxLen: 4},
	{Start: "Expr", MaxLen: 4},
	{Start: "Statement", MaxLen: 4},
	{Start: "Type", MaxLen: 4},
	{Start: "Pattern", MaxLen: 4},
	{Start: "Primary", MaxLen: 4},
	// The narrowest reachable set in the grammar (21 cells), which is what affords the depth to
	// search the positional-keyword class properly: every IDENT("from") collision available in
	// five tokens, with ordinary identifiers free to be spelled `from` too.
	{Start: "ImportSpec", MaxLen: 5, Spellings: true},
}

func main() {
	all := flag.Bool("sweep", false, "run the documented table instead of a single start")
	start := flag.String("start", "File", "nonterminal to enumerate from")
	maxLen := flag.Int("len", 4, "longest sentence to generate, in tokens")
	perCell := flag.Int("cap", 0, "cap on sentences kept per (nonterminal, length); 0 is uncapped")
	spellings := flag.Bool("spellings", false, "also emit required lexemes in unconstrained positions")
	list := flag.Bool("list", false, "print every sentence generated, not just the findings")
	flag.Parse()

	g, err := ebnf.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "loading grammar.md:", err)
		os.Exit(2)
	}

	bounds := sweep
	if !*all {
		bounds = []ebnf.Bound{{
			Start:      *start,
			MaxLen:     *maxLen,
			MaxPerCell: *perCell,
			Spellings:  *spellings,
		}}
	}

	began := time.Now()
	clean := true
	for i := range bounds {
		bounds[i].KeepSentences = *list
		if !run(g, bounds[i]) {
			clean = false
		}
	}
	fmt.Printf("total %s\n", time.Since(began).Round(time.Millisecond))

	if !clean {
		os.Exit(1)
	}
}

func run(g *ebnf.Grammar, b ebnf.Bound) bool {
	began := time.Now()
	rep, err := g.Enumerate(b)
	if err != nil {
		fmt.Fprintln(os.Stderr, "enumerating:", err)
		os.Exit(2)
	}
	for _, s := range rep.All {
		fmt.Println(s)
	}
	fmt.Print(rep)
	fmt.Printf("  elapsed %s\n", time.Since(began).Round(time.Millisecond))

	// A bound that generated nothing is the fail-open shape this whole tool is aimed at: it
	// reports clean while having looked at not one sentence. MemberDecl needs five tokens
	// before it derives at all, which is exactly how that arrives by accident.
	if rep.Sentences == 0 {
		fmt.Fprintf(os.Stderr, "  %s at %d tokens derives nothing; this bound checked nothing\n",
			b.Start, b.MaxLen)
		return false
	}
	return len(rep.Ambiguous) == 0 && len(rep.Unrecognized) == 0 && rep.Exhaustive()
}
