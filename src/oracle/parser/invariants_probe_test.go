// The battery, tested in both directions, because **a test helper that cannot fail is worse than
// no helper at all**: it reads as coverage while asserting nothing.
//
// Corruptions are applied to a hand-written arena rather than produced by build, the point being
// to describe trees the builder cannot make. The probes call each group directly so that a group
// 2 test fails when group 2 is broken and at no other time; one test at the end covers the
// dispatch that skips past.
package parser

import (
	"fmt"
	"strings"
	"testing"

	"luna/oracle/token"
)

// --- standing in for T -------------------------------------------------------------------

// recorder captures what an assertion reports instead of failing the test that is watching it.
type recorder struct{ reports []string }

func (r *recorder) Helper() {}

func (r *recorder) Errorf(format string, args ...any) {
	r.reports = append(r.reports, fmt.Sprintf(format, args...))
}

// Fatalf unwinds as testing.T.Fatalf does: an assertion that declares the input beyond further
// judgement must not carry on judging it.
func (r *recorder) Fatalf(format string, args ...any) {
	r.Errorf(format, args...)
	panic(fatal{})
}

type fatal struct{}

// probe re-raises any panic but the recorder's own: build and splice panic with strings, and
// swallowing one here would turn a contract violation into a silent pass.
func probe(assert func(reporter)) []string {
	r := &recorder{}
	func() {
		defer func() {
			if v := recover(); v != nil {
				if _, ok := v.(fatal); !ok {
					panic(v)
				}
			}
		}()
		assert(r)
	}()
	return r.reports
}

// assertReported is both directions: an empty want means nothing may be said at all.
func assertReported(t *testing.T, reports []string, want string) {
	t.Helper()
	if want == "" {
		if len(reports) > 0 {
			t.Fatalf("a well-formed input drew %d reports:\n%s",
				len(reports), strings.Join(reports, "\n"))
		}
		return
	}
	for _, r := range reports {
		if strings.Contains(r, want) {
			return
		}
	}
	if len(reports) == 0 {
		t.Fatalf("nothing was reported; want a report containing %q", want)
	}
	t.Fatalf("no report contained %q; got:\n%s", want, strings.Join(reports, "\n"))
}

// --- the trees the probes corrupt ---------------------------------------------------------

// probeTree is tree_test.go's arena for "x;\n", freshly copied for each probe to corrupt.
func probeTree(t *testing.T) (*Tree, []token.Token, string) {
	t.Helper()
	_, tokens := lexFixture(t, "probe.luna", handSource)
	return handTree(t), tokens, handSource
}

// triviaInsideStatement puts the newline under the Statement — what a splice that flushed at
// every close would build. It is well formed in every other respect, so only group 4 may object.
func triviaInsideStatement(t *testing.T) *Tree {
	tree := probeTreeWithNodes(t, []node{
		{kind: File, parent: 0, size: 5, offset: 0, end: 3},
		{kind: Statement, parent: 0, size: 4, offset: 0, end: 3},
		{kind: Kind(token.Ident), parent: 1, size: 1, offset: 0, end: 1},
		{kind: Kind(token.Semicolon), parent: 1, size: 1, offset: 1, end: 2},
		{kind: Kind(token.Whitespace), parent: 1, size: 1, offset: 2, end: 3},
	})
	return tree
}

// triviaFirstInStatement is the other edge, over " x;": a node whose span would start at trivia.
func triviaFirstInStatement(t *testing.T) *Tree {
	return probeTreeWithNodes(t, []node{
		{kind: File, parent: 0, size: 5, offset: 0, end: 3},
		{kind: Statement, parent: 0, size: 4, offset: 0, end: 3},
		{kind: Kind(token.Whitespace), parent: 1, size: 1, offset: 0, end: 1},
		{kind: Kind(token.Ident), parent: 1, size: 1, offset: 1, end: 2},
		{kind: Kind(token.Semicolon), parent: 1, size: 1, offset: 2, end: 3},
	})
}

func probeTreeWithNodes(t *testing.T, nodes []node) *Tree {
	t.Helper()
	tree := handTree(t)
	tree.nodes = nodes
	return tree
}

// --- group 1: the arena is a tree -----------------------------------------------------------

func TestArenaInvariants(t *testing.T) {
	tests := []struct {
		name    string
		corrupt func(*Tree)
		want    string
	}{{
		name:    "a well-formed arena",
		corrupt: func(*Tree) {},
		want:    "",
	}, {
		name:    "a size of zero",
		corrupt: func(tree *Tree) { tree.nodes[3].size = 0 },
		want:    "has size 0",
	}, {
		name:    "a size reaching past the arena",
		corrupt: func(tree *Tree) { tree.nodes[4].size = 9 },
		want:    "claims 9 nodes in an arena of 5",
	}, {
		// With size 4 the walk lands exactly on 4 and the trailing WHITESPACE is in no subtree.
		name:    "a root that does not span the arena",
		corrupt: func(tree *Tree) { tree.nodes[0].size = 4 },
		want:    "the root spans 4 of the arena's 5 nodes",
	}, {
		// What a close that patched the wrong frame leaves. It stays inside the arena, so only
		// the tiling walk can see it.
		name:    "a child reaching past its parent",
		corrupt: func(tree *Tree) { tree.nodes[2].size = 3 },
		want:    "node 1 (Statement) spans 3 nodes but its children reach 4",
	}, {
		name:    "a parent that names the wrong node",
		corrupt: func(tree *Tree) { tree.nodes[2].parent = 0 },
		want:    "node 2 is a child of node 1 and names 0 as its parent",
	}, {
		name:    "a root with a parent",
		corrupt: func(tree *Tree) { tree.nodes[0].parent = 1 },
		want:    "the root names node 1 as its parent",
	}, {
		// What an empty interior node that survived §6.1 looks like, and the only trace it leaves.
		name:    "a childless node carrying a nonterminal's kind",
		corrupt: func(tree *Tree) { tree.nodes[4].kind = Modifier },
		want:    "node 4 is a childless Modifier",
	}, {
		name:    "a leaf's kind on a node with children",
		corrupt: func(tree *Tree) { tree.nodes[1].kind = Kind(token.Semicolon) },
		want:    "node 1 is a SEMICOLON with 2 children",
	}, {
		name:    "a leaf carrying Unset",
		corrupt: func(tree *Tree) { tree.nodes[2].kind = Unset },
		want:    "node 2 is a childless UNSET",
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tree, _, _ := probeTree(t)
			tc.corrupt(tree)
			assertReported(t, probe(func(r reporter) { assertArenaIsATree(r, tree) }), tc.want)
		})
	}
}

// --- group 2: spans -------------------------------------------------------------------------

func TestSpanInvariants(t *testing.T) {
	tests := []struct {
		name    string
		corrupt func(*Tree)
		want    string
	}{{
		name:    "well-formed spans",
		corrupt: func(*Tree) {},
		want:    "",
	}, {
		name:    "a span that runs backwards",
		corrupt: func(tree *Tree) { tree.nodes[3].offset = 3 },
		want:    "node 3 (SEMICOLON) spans 3..2",
	}, {
		name:    "a span past the end of the file",
		corrupt: func(tree *Tree) { tree.nodes[4].end = 4 },
		want:    "in a file of 3 bytes",
	}, {
		name:    "a parent starting after its first child",
		corrupt: func(tree *Tree) { tree.nodes[1].offset = 1 },
		want:    "node 1 (Statement) starts at 1, its first child at 0",
	}, {
		name:    "a parent ending before its last child",
		corrupt: func(tree *Tree) { tree.nodes[1].end = 1 },
		want:    "node 1 (Statement) ends at 1, its last child at 2",
	}, {
		name:    "a child that overlaps its sibling",
		corrupt: func(tree *Tree) { tree.nodes[3].offset = 0 },
		want:    "SEMICOLON starts at 0, back inside the child that ended at 1",
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tree, _, src := probeTree(t)
			tc.corrupt(tree)
			assertReported(t, probe(func(r reporter) { assertSpansNest(r, tree, src) }), tc.want)
		})
	}
}

// TestChildrenTileInvariant is the completeness half of the spans, one tier up: a gap between two
// children is a token in the file and in no leaf, caught locally where reconstruction reports it
// file-wide.
func TestChildrenTileInvariant(t *testing.T) {
	t.Run("children that abut", func(t *testing.T) {
		tree, _, _ := probeTree(t)
		assertReported(t, probe(func(r reporter) { assertChildrenTile(r, tree) }), "")
	})
	t.Run("a gap between two children", func(t *testing.T) {
		tree, _, _ := probeTree(t)
		tree.nodes[2].end = 0 // IDENT shrinks; SEMICOLON no longer abuts it
		assertReported(t, probe(func(r reporter) { assertChildrenTile(r, tree) }),
			"SEMICOLON starts at 1 where the previous child ended at 0")
	})
}

// TestElidedEvents is the one oracle in the battery, and so the one thing here that could pass a
// bad splice by agreeing with it: assertSpliceOnlyInserts believes whatever this marks. Marking
// everything is caught by the lockstep comparison, but only because it desynchronises the whole
// stream — a *targeted* over-mark would not be, so the marks are checked directly.
//
// The pattern is one character per event: `x` where splice must drop it, `.` where it must not.
func TestElidedEvents(t *testing.T) {
	tests := []struct {
		name   string
		events eventStream
		want   string
	}{{
		name:   "nothing empty",
		events: eventStream{openEv(File), openEv(Statement), tokEv(0), closeEv, closeEv},
		want:   ".....",
	}, {
		name:   "an empty node",
		events: eventStream{openEv(File), tokEv(0), openEv(Modifier), closeEv, closeEv},
		want:   "..xx.",
	}, {
		name: "empty nodes nested in each other",
		events: eventStream{
			openEv(File), tokEv(0), openEv(Block), openEv(Modifier), closeEv, closeEv, closeEv,
		},
		want: "..xxxx.",
	}, {
		// A synthesised leaf is content, so the node holding one is not empty (§6.1).
		name:   "a node holding only a synthesised leaf",
		events: eventStream{openEv(File), openEv(Initializer), missingEv(Error), closeEv, closeEv},
		want:   ".....",
	}, {
		// The root is never marked: trailing trivia releases it, and splice alone can see that.
		name:   "a root with no leaf in it",
		events: eventStream{openEv(File), closeEv},
		want:   "..",
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got strings.Builder
			for _, drop := range elidedEvents(tc.events) {
				if drop {
					got.WriteByte('x')
					continue
				}
				got.WriteByte('.')
			}
			if got.String() != tc.want {
				t.Errorf("elided %s, want %s\nover %s", got.String(), tc.want, tc.events)
			}
		})
	}
}

// TestReadingsAgreeInvariant is golden.md §1's claim. Its violating fixture is the shape the
// parse fuzzer found in splice: a Statement whose span was widened over the newline after it,
// which is exactly a node reading differently with trivia counted and without.
func TestReadingsAgreeInvariant(t *testing.T) {
	t.Run("one number under either reading", func(t *testing.T) {
		tree, _, _ := probeTree(t)
		assertReported(t, probe(func(r reporter) { assertReadingsAgree(r, tree) }), "")
	})
	t.Run("a node widened over trivia", func(t *testing.T) {
		tree := triviaInsideStatement(t)
		assertReported(t, probe(func(r reporter) { assertReadingsAgree(r, tree) }),
			"node 1 (Statement) spans 0..3 counting trivia and 0..2 without it")
	})
}

// --- group 3: the leaves are the file --------------------------------------------------------

func TestLeafInvariants(t *testing.T) {
	tests := []struct {
		name    string
		corrupt func(*Tree)
		want    string
	}{{
		name:    "well-formed leaves",
		corrupt: func(*Tree) {},
		want:    "",
	}, {
		// The case reconstruction cannot see: identical bytes, wrong kind.
		name:    "a leaf with the wrong kind",
		corrupt: func(tree *Tree) { tree.nodes[2].kind = Kind(token.KwLet) },
		want:    "node 2 is KW_LET 0..1, want token 0: IDENT 0..1",
	}, {
		name:    "a leaf with the wrong span",
		corrupt: func(tree *Tree) { tree.nodes[3].end = 3 },
		want:    "node 3 is SEMICOLON 1..3, want token 1: SEMICOLON 1..2",
	}, {
		// The tree stays internally consistent, which is why this needs the stream to detect.
		name: "a token missing from the tree",
		corrupt: func(tree *Tree) {
			tree.nodes = tree.nodes[:4]
			tree.nodes[0].size, tree.nodes[0].end = 4, 2
		},
		want: "the tree holds 2 of the file's 3 tokens",
	}, {
		name: "a zero-width leaf that no expect-site could have wanted",
		corrupt: func(tree *Tree) {
			tree.nodes[4].offset, tree.nodes[4].end = 3, 3 // the newline, synthesised
		},
		want: "node 4 is a zero-width WHITESPACE",
	}, {
		name:    "leaves that do not reconstruct the file",
		corrupt: func(tree *Tree) { tree.nodes[4].offset, tree.nodes[4].end = 0, 1 },
		want:    `the leaves reconstruct "x;x"`,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tree, tokens, src := probeTree(t)
			tc.corrupt(tree)
			assertReported(t,
				probe(func(r reporter) { assertLeavesAreTheFile(r, tree, tokens, src) }), tc.want)
		})
	}
}

// --- group 4: trivia placement ---------------------------------------------------------------

func TestTriviaPlacementInvariant(t *testing.T) {
	tests := []struct {
		name string
		tree func(*testing.T) *Tree
		want string
	}{{
		// The positive case that matters most: a check that forgot File's exception fires here.
		name: "trivia at the end of File",
		tree: handTree,
		want: "",
	}, {
		name: "trivia as a node's last child",
		tree: triviaInsideStatement,
		want: "WHITESPACE at 2..3 is the last child of Statement",
	}, {
		name: "trivia as a node's first child",
		tree: triviaFirstInStatement,
		want: "WHITESPACE at 0..1 is the first child of Statement",
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tree := tc.tree(t)
			assertReported(t,
				probe(func(r reporter) { assertTriviaIsNeverAtAnEdge(r, tree) }), tc.want)
		})
	}
}

// --- the tree battery as a whole -------------------------------------------------------------

// TestTreeBatteryDispatches is what the direct calls above skip past: that the entry point runs
// all four groups, across both tiers. One corruption per group, each chosen to be reported rather
// than fatal, so none of them can mask the next.
func TestTreeBatteryDispatches(t *testing.T) {
	tests := []struct {
		group   string
		corrupt func(*Tree)
		want    string
	}{
		{"1", func(tree *Tree) { tree.nodes[2].parent = 0 }, "names 0 as its parent"},
		{"2", func(tree *Tree) { tree.nodes[1].offset = 1 }, "starts at 1, its first child at 0"},
		{"3", func(tree *Tree) { tree.nodes[2].kind = Kind(token.KwLet) }, "want token 0: IDENT"},
		{"4", func(tree *Tree) {
			tree.nodes[1].size, tree.nodes[1].end = 4, 3
			tree.nodes[4].parent = 1
		}, "is the last child of Statement"},
	}

	for _, tc := range tests {
		t.Run("group "+tc.group, func(t *testing.T) {
			tree, tokens, src := probeTree(t)
			tc.corrupt(tree)
			assertReported(t,
				probe(func(r reporter) { assertTreeInvariants(r, tree, tokens, src) }), tc.want)
		})
	}
}

// TestTreeTierBoundary is the split itself, in the case it exists for: a tree built from the
// parser's own events, with nothing spliced into them.
//
// It is a perfectly well-formed tree — the arena is sound and every span nests — and it is not
// the file: it has no trivia, so its root stops short of the end and two of the three tokens are
// in no leaf. Both statements are true at once, which is why the two tiers are separate
// functions and why the contract fuzz target can assert the first without meeting the
// precondition of the second.
func TestTreeTierBoundary(t *testing.T) {
	f, tokens := lexFixture(t, "unspliced.luna", handSource)
	tree := build(f, tokens, eventStream{
		openEv(File),
		openEv(Statement), tokEv(0), tokEv(1), closeEv,
		closeEv,
	})

	assertReported(t, probe(func(r reporter) { assertTreeIsWellFormed(r, tree, handSource) }), "")

	reports := probe(func(r reporter) { assertTreeIsTheFile(r, tree, tokens, handSource) })
	for _, want := range []string{
		"the root spans 0..2, want 0..3",
		"the tree holds 2 of the file's 3 tokens",
		`the leaves reconstruct "x;"`,
	} {
		assertReported(t, reports, want)
	}
}

// TestTreeBatteryPassesWellFormedTrees is the other direction at the entry point. The goldens
// cover it thirty times over, but by a route that goes quiet if the corpus ever stops reaching
// the battery — and the damaged tree here is one no golden holds at all.
func TestTreeBatteryPassesWellFormedTrees(t *testing.T) {
	t.Run("the hand-written tree", func(t *testing.T) {
		tree, tokens, src := probeTree(t)
		assertReported(t,
			probe(func(r reporter) { assertTreeInvariants(r, tree, tokens, src) }), "")
	})

	// §6.1's zero-width leaf is legal, and a group that rejected it would fail every recovery
	// test in Phase 2 for the wrong reason.
	t.Run("a tree with a synthesised leaf", func(t *testing.T) {
		const src = "x"
		f, tokens := lexFixture(t, "missing.luna", src)
		tree := build(f, tokens, eventStream{
			openEv(File),
			openEv(Statement), tokEv(0), missingEv(Kind(token.Semicolon)), closeEv,
			closeEv,
		})
		assertReported(t,
			probe(func(r reporter) { assertTreeInvariants(r, tree, tokens, src) }), "")
	})

	// §6.1's iff: the only input for which no tree is the right answer.
	t.Run("the empty file", func(t *testing.T) {
		assertReported(t, probe(func(r reporter) { assertTreeInvariants(r, nil, nil, "") }), "")
	})
	t.Run("no tree for a file with tokens", func(t *testing.T) {
		_, tokens := lexFixture(t, "probe.luna", handSource)
		assertReported(t,
			probe(func(r reporter) { assertTreeInvariants(r, nil, tokens, handSource) }),
			"no tree for a file of 3 bytes")
	})
}

// --- the splice battery ----------------------------------------------------------------------

// probeSplice is the parser's stream for "x;\n" and what splice makes of it. The corruptions
// below are applied to the output, since splice itself is the thing being described.
func probeSplice(t *testing.T) (tokens []token.Token, before, after eventStream) {
	t.Helper()
	_, tokens = lexFixture(t, "probe.luna", handSource)
	before = eventStream{
		openEv(File),
		openEv(Statement), tokEv(0), tokEv(1), closeEv,
		closeEv,
	}
	return tokens, before, splice(tokens, before)
}

// withEvent and withoutEvent describe a corrupted stream as an edit to a real one, which is
// easier to read than a rebuilt literal and cannot alias the stream it came from.
func withEvent(events eventStream, at int, e event) eventStream {
	out := append(eventStream{}, events[:at]...)
	out = append(out, e)
	return append(out, events[at:]...)
}

func withoutEvent(events eventStream, at int) eventStream {
	out := append(eventStream{}, events[:at]...)
	return append(out, events[at+1:]...)
}

func TestSpliceInvariants(t *testing.T) {
	tests := []struct {
		name string
		// Which of the three is probed, so a failure blames one rather than the battery.
		assert  func(r reporter, tokens []token.Token, before, after eventStream)
		corrupt func(tokens []token.Token, before, after eventStream) (eventStream, eventStream)
		want    string
	}{{
		name:   "a well-formed splice",
		assert: assertSpliceInvariants,
		want:   "",
	}, {
		name:   "a trivia token left out",
		assert: assertSpliceInvariants,
		corrupt: func(_ []token.Token, before, after eventStream) (eventStream, eventStream) {
			return before, withoutEvent(after, 5) // the trailing WHITESPACE
		},
		want: "carries 2 of the file's 3 tokens",
	}, {
		name:   "a token carried twice",
		assert: assertSpliceInvariants,
		corrupt: func(_ []token.Token, before, after eventStream) (eventStream, eventStream) {
			return before, withEvent(after, 3, after[2])
		},
		want: "event 3 is token(0) where token(1) was due",
	}, {
		name:   "the parser's last event dropped",
		assert: spliceOnlyInserts,
		corrupt: func(_ []token.Token, before, after eventStream) (eventStream, eventStream) {
			return before, withoutEvent(after, len(after)-1)
		},
		want: "splice dropped the parser's event 5 (close)",
	}, {
		// §2.2 drops empty nodes, so the oracle expects this open to survive: what it holds is
		// what makes it not empty.
		name:   "a node dropped that was not empty",
		assert: spliceOnlyInserts,
		corrupt: func(_ []token.Token, before, after eventStream) (eventStream, eventStream) {
			return before, withoutEvent(after, 1) // the Statement's open
		},
		want: "the parser's event 1 is open(Statement) where splice has token(0) at 1",
	}, {
		name:   "an event splice had no business adding",
		assert: spliceOnlyInserts,
		corrupt: func(_ []token.Token, before, after eventStream) (eventStream, eventStream) {
			return before, withEvent(after, 1, openEv(Modifier))
		},
		want: "where splice has open(Modifier) at 1",
	}, {
		// A bug one stage upstream: splice is what puts trivia back.
		name:   "the parser emitting trivia of its own",
		assert: spliceOnlyInserts,
		corrupt: func(_ []token.Token, before, after eventStream) (eventStream, eventStream) {
			return withEvent(before, 1, tokEv(2)), after
		},
		want: "the parser's event 1 is token(2), which is trivia",
	}, {
		name:   "trivia flushed straight after an open",
		assert: triviaNeverAtAnEventEdge,
		corrupt: func(_ []token.Token, before, after eventStream) (eventStream, eventStream) {
			// open(File) open(Statement) WS token(0) token(1) close close
			return before, eventStream{
				after[0], after[1], tokEv(2), after[2], after[3], after[4], after[6],
			}
		},
		want: "event 2 is trivia directly after open(Statement)",
	}, {
		name:   "trivia flushed just before a close",
		assert: triviaNeverAtAnEventEdge,
		corrupt: func(_ []token.Token, before, after eventStream) (eventStream, eventStream) {
			// the shape a splice that flushed at every close would emit
			return before, eventStream{
				after[0], after[1], after[2], after[3], tokEv(2), after[4], after[6],
			}
		},
		want: "event 4 is trivia directly before a close",
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokens, before, after := probeSplice(t)
			if tc.corrupt != nil {
				before, after = tc.corrupt(tokens, before, after)
			}
			assertReported(t,
				probe(func(r reporter) { tc.assert(r, tokens, before, after) }), tc.want)
		})
	}
}

// The two sub-assertions with signatures of their own, adapted so one table can drive all three.
func spliceOnlyInserts(r reporter, tokens []token.Token, before, after eventStream) {
	assertSpliceOnlyInserts(r, tokens, before, after)
}

func triviaNeverAtAnEventEdge(r reporter, tokens []token.Token, _, after eventStream) {
	assertTriviaIsNeverAtAnEventEdge(r, tokens, after)
}
