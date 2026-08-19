// The spec corpus through Parse and into the invariant battery, the last driver
// invariants_test.go was written to take.
//
// It complements the goldens on the battery's own division: goldens pin shape, invariants pin
// properties. A golden's tree section drops trivia (golden.md §1), so §2.1's placement rule is
// checked here or nowhere, and so are strong losslessness and index coverage, which lives on the
// event stream a stage before any tree.
//
// In-package because the event stream is (§4.1).
package parser

import (
	"fmt"
	"testing"

	"luna/internal/spec"
	"luna/oracle/diagnostic"
)

// parseRun keeps every stage: the splice invariants are a relation between two streams rather
// than a property of either, which is why goldenRun does the same.
type parseRun struct {
	lexed     *LexedGolden
	unspliced eventStream
	events    eventStream
	tree      *Tree
	diags     []diagnostic.Diagnostic
}

// runParse drives one source through the whole pass. Nil with no message is **pending** and not a
// skip: the panic must be parse's own sentinel, so a block becomes a live assertion the moment a
// body lands. Nil with one is a **violated invariant**, which §7.8 makes a parser bug; catching it
// attributes it to this block instead of aborting the run on the first, and which blocks break is
// the whole diagnosis. Anything else propagates.
func runParse(lexed *LexedGolden) (run *parseRun, bug string) {
	defer func() {
		v := recover()
		if v == nil {
			return
		}
		if s, ok := v.(string); ok && s == "parser: parse is unimplemented" {
			run = nil
			return
		}
		if b, ok := v.(diagnostic.Bug); ok {
			run, bug = nil, b.Message
			return
		}
		panic(v)
	}()
	events, diags := parse(lexed.File, lexed.Tokens)
	spliced := splice(lexed.Tokens, events)
	return &parseRun{
		lexed:     lexed,
		unspliced: events,
		events:    spliced,
		tree:      build(lexed.File, lexed.Tokens, spliced),
		diags:     diags,
	}, ""
}

func TestSpecCorpusThroughParse(t *testing.T) {
	blocks, err := spec.LunaBlocks()
	if err != nil {
		t.Fatalf("reading the spec corpus: %v", err)
	}
	if len(blocks) < specCorpusFloor {
		t.Fatalf("found %d blocks, expected at least %d; the corpus walk is broken",
			len(blocks), specCorpusFloor)
	}

	parsed, pending, broke, nodes := 0, 0, 0, 0
	for _, b := range blocks {
		lexed, err := LexGolden("spec.luna", b.Source)
		if err != nil {
			continue // a lexical failure is oracle/lexer's to report, not this test's
		}
		run, bug := runParse(lexed)
		if bug != "" {
			broke++
			t.Errorf("%s:%d violated an invariant: %s", b.Path, b.Line, bug)
			continue
		}
		if run == nil {
			pending++
			continue
		}
		parsed++
		if run.tree != nil {
			nodes += run.tree.Len()
		}
		t.Run(fmt.Sprintf("%s:%d", b.Path, b.Line), func(t *testing.T) {
			// Every block derives, which internal/ebnf's TestCorpusParses proves, so the parser
			// must raise nothing on any of them: the parser's half of the reject-set invariant
			// (golden.md §0).
			if len(run.diags) != 0 {
				t.Errorf("%d diagnostics on a block the grammar derives:\n%v", len(run.diags), run.diags)
			}
			assertSpliceInvariants(t, run.lexed.Tokens, run.unspliced, run.events)
			assertTreeInvariants(t, run.tree, run.lexed.Tokens, b.Source)
		})
	}

	t.Logf("%d parsed, %d pending, %d broke, %d nodes built", parsed, pending, broke, nodes)
	if parsed+pending+broke < specCorpusFloor {
		t.Errorf("only %d blocks reached the parser: the machinery is being checked against "+
			"almost nothing", parsed+pending+broke)
	}
}
