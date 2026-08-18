package parser

import "testing"

// newParser and parse: the two ends of the pass.

// newParser's one job beyond assignment is pos's invariant (§4.5), and the leading-trivia case is
// the only one that can get it wrong.
func TestNewParserSeedsThePastLeadingTrivia(t *testing.T) {
	expects(t, func(t *testing.T) {
		f, toks := lexFor(t, "// a comment\nx;")
		p := newParser(f, toks)
		if want := indexOf(toks, 0); p.pos != want {
			t.Errorf("pos is %d, want %d — the cursor starts past the file's leading trivia", p.pos, want)
		}
		if len(p.events) != 0 || len(p.diags) != 0 || len(p.stack) != 0 {
			t.Error("a fresh parse is not empty; all three sinks accumulate, none is seeded")
		}
	})
}

func TestNewParserOnATriviaOnlyFileStartsAtTheEnd(t *testing.T) {
	expects(t, func(t *testing.T) {
		f, toks := lexFor(t, "// nothing but this\n")
		if p := newParser(f, toks); p.pos != len(toks) {
			t.Errorf("pos is %d of %d; with no non-trivia token there is nowhere else to be",
				p.pos, len(toks))
		}
	})
}

// parse's contract is splice's preconditions, and the way to assert them is to run splice: it
// panics on each one. Building and running the battery afterwards makes this the whole pass.
func TestParseProducesASpliceableStream(t *testing.T) {
	expects(t, func(t *testing.T) {
		src := "// leading\nlet x = 1 + 2;\n"
		f, toks := lexFor(t, src)
		evs, diags := parse(f, toks)
		if len(diags) != 0 {
			t.Errorf("%d diagnostics on input the grammar derives", len(diags))
		}
		spliced := splice(toks, evs)
		assertSpliceInvariants(t, toks, evs, spliced)
		assertTreeInvariants(t, build(f, toks, spliced), toks, src)
	})
}

// §6.1's iff, from the top: the empty file is the only input with no tree.
func TestParseGivesTheEmptyFileNoTree(t *testing.T) {
	expects(t, func(t *testing.T) {
		f, toks := lexFor(t, "")
		if tree, _ := Parse(f, toks); tree != nil {
			t.Errorf("the empty file produced a tree of %d nodes; no tree reconstructs the empty "+
				"string exactly", tree.Len())
		}
	})
}

// A file of nothing but trivia is the neighbouring case, and it must *not* be nil: the trailing
// trivia is content, so it releases File and becomes its children.
func TestParseGivesATriviaOnlyFileATree(t *testing.T) {
	expects(t, func(t *testing.T) {
		src := "// just this\n"
		f, toks := lexFor(t, src)
		tree, _ := Parse(f, toks)
		if tree == nil {
			t.Fatal("no tree for a comment-only file; a comment is bytes, and losing it would lose them")
		}
		assertTreeInvariants(t, tree, toks, src)
	})
}
