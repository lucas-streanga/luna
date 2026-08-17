// The invariant battery: what holds of **every** tree and **every** splice, whatever was parsed.
//
// It takes a tree and a token stream rather than anything this phase owns, so drivers can differ
// while the properties do not — hand-written events and the golden corpus today, the fuzz targets
// next, `Parse` when Phase 2 lands, with nothing here changing. A driver then costs nothing to
// add, and an invariant added here reaches every driver at once.
//
// **The tree has two tiers and their union**, split where the one precondition bites: some of
// this is true of any tree the builder returns, the rest only of one built from a **spliced**
// stream. Every driver today meets that precondition and wants the union; the fuzz targets that
// feed `build` arbitrary streams do not, and asserting the spliced tier there would report the
// input rather than a defect.
//
// It never compares a tree against grammar.md, which is the division the whole design rests on:
// **goldens pin shape, invariants pin properties.** Only the second scales to a fuzzer.
package parser

import (
	"strings"

	"luna/oracle/token"
)

// reporter is an interface for one reason: **a test helper that cannot fail is worse than no
// helper at all**, and standing a recorder in for T is what lets invariants_probe_test.go prove
// each group fires. testing.TB is the natural spelling and cannot be implemented outside the
// testing package.
type reporter interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// --- the tree ----------------------------------------------------------------------------

// assertTreeInvariants is both tiers, and what every driver that parses a whole file wants: the
// tree is well formed, *and* it is this file.
func assertTreeInvariants(t reporter, tree *Tree, tokens []token.Token, src string) {
	t.Helper()
	assertTreeIsWellFormed(t, tree, src)
	assertTreeIsTheFile(t, tree, tokens, src)
}

// assertTreeIsWellFormed holds of **any** tree the builder returns, from any stream. Nothing here
// reads the token stream, which is what makes it safe to assert about events nobody spliced — the
// case the contract fuzz target lives in. Whether a nil tree should have been nil is the other
// tier's question, since only the token stream can answer it.
func assertTreeIsWellFormed(t reporter, tree *Tree, src string) {
	t.Helper()
	if tree == nil {
		return
	}
	assertArenaIsATree(t, tree)
	assertSpansNest(t, tree, src)
}

// assertTreeIsTheFile needs the precondition: **the tree was built from a spliced stream.** Every
// claim here is about the tree standing for a particular file, and each is simply false of one
// built from the parser's own events, which hold no trivia at all.
func assertTreeIsTheFile(t reporter, tree *Tree, tokens []token.Token, src string) {
	t.Helper()

	// §6.1's iff, which build cannot enforce alone: it cannot tell a spliced stream from one
	// whose trivia is simply absent.
	if tree == nil {
		if len(tokens) > 0 {
			t.Fatalf("no tree for a file of %d bytes in %d tokens", len(src), len(tokens))
		}
		return
	}
	if len(tokens) == 0 {
		t.Fatalf("a tree of %d nodes for a file with no tokens", tree.Len())
	}

	// The absolute anchor, where assertSpansNest checks only relative ones (golden.md §1).
	if offset, end := tree.Root().Span(); offset != 0 || end != len(src) {
		t.Errorf("the root spans %d..%d, want 0..%d — File owns the file's leading and trailing "+
			"trivia, and is the only node whose span differs from its non-trivia extent",
			offset, end, len(src))
	}

	assertLeavesAreTheFile(t, tree, tokens, src)
	assertTriviaIsNeverAtAnEdge(t, tree)
}

// assertArenaIsATree is group 1, and it reads the arena directly because every other assertion
// here reaches the tree through Children — which navigates *by* size, so a wrong size returns a
// wrong child list rather than failing, and the rest of the file would agree with it. Parent is
// worse: stored, read by one method, and checked by nothing.
func assertArenaIsATree(t reporter, tree *Tree) {
	t.Helper()

	// Sizes first and alone, because the walk below steps *by* them: a zero would spin it.
	for id, n := range tree.nodes {
		if n.size < 1 {
			t.Fatalf("node %d (%s) has size %d; the smallest subtree is the node itself",
				id, n.kind, n.size)
		}
		if id+int(n.size) > len(tree.nodes) {
			t.Fatalf("node %d (%s) claims %d nodes in an arena of %d",
				id, n.kind, n.size, len(tree.nodes))
		}
	}

	// Reachability, which a pre-order arena buys for one comparison: a short root size hides the
	// nodes past its end from every walk below, including the tiling check that would catch it.
	if got := int(tree.nodes[0].size); got != len(tree.nodes) {
		t.Fatalf("the root spans %d of the arena's %d nodes; an arena holds exactly one tree",
			got, len(tree.nodes))
	}

	if p := tree.nodes[0].parent; p != 0 {
		t.Errorf("the root names node %d as its parent; it is its own", p)
	}

	for id, n := range tree.nodes {
		end := id + int(n.size)
		children := 0
		i := id + 1
		for i < end {
			if got := tree.nodes[i].parent; got != NodeID(id) {
				t.Errorf("node %d is a child of node %d and names %d as its parent", i, id, got)
			}
			i += int(tree.nodes[i].size)
			children++
		}
		// Short swallows a sibling, long reaches past the parent, and both give Children a wrong
		// answer with no other symptom.
		if i != end {
			t.Fatalf("node %d (%s) spans %d nodes but its children reach %d",
				id, n.kind, n.size, i-id)
		}

		// A leaf carrying a nonterminal's kind is an empty interior node that survived, and the
		// only trace §6.1's rule can leave where size 1 *means* leaf.
		if children == 0 {
			leafKind := n.kind == Error || (n.kind.IsToken() && n.kind != Unset)
			if !leafKind {
				t.Errorf("node %d is a childless %s: an interior node with no children should "+
					"have been deleted, and a leaf carries a token's kind or Error", id, n.kind)
			}
			continue
		}
		if !isNode(n.kind) {
			t.Errorf("node %d is a %s with %d children: a token kind belongs to a leaf",
				id, n.kind, children)
		}
	}
}

// assertSpansNest is group 2. Three checks rather than one because they fail differently: a
// parent narrower than its children is a cover that did not widen, a gap between siblings is a
// leaf gone missing locally, and an extent right at the edges but wrong between them is a cover
// that widened over something it should not have. Containment and ordering follow from them.
//
// Every check is **relative**, a node against its own children, which is why the group needs no
// spliced stream; src is only the bound no span may cross.
func assertSpansNest(t reporter, tree *Tree, src string) {
	t.Helper()

	for id := range tree.Len() {
		n := tree.At(NodeID(id))
		offset, end := n.Span()
		if offset > end || end > len(src) {
			t.Fatalf("node %d (%s) spans %d..%d, in a file of %d bytes",
				id, n.Kind(), offset, end, len(src))
		}

		children := n.Children()
		if len(children) == 0 {
			continue
		}
		if first, _ := children[0].Span(); first != offset {
			t.Errorf("node %d (%s) starts at %d, its first child at %d",
				id, n.Kind(), offset, first)
		}
		if _, last := children[len(children)-1].Span(); last != end {
			t.Errorf("node %d (%s) ends at %d, its last child at %d", id, n.Kind(), end, last)
		}
		at := offset
		for _, child := range children {
			childOffset, childEnd := child.Span()
			if childOffset != at {
				t.Errorf("node %d (%s): %s starts at %d where the previous child ended at %d",
					id, n.Kind(), child.Kind(), childOffset, at)
			}
			at = childEnd
		}
	}
}

// assertLeavesAreTheFile is group 3, losslessness in the strong form. Concatenation is the weaker
// half: it passes with every leaf mislabelled, a kind not being bytes. The half with teeth is
// that the **positive-width leaves are the token stream**, which catches a botched Kind
// conversion, a misaligned kind space, or a token replaced by a synthesised leaf of equal width.
func assertLeavesAreTheFile(t reporter, tree *Tree, tokens []token.Token, src string) {
	t.Helper()

	var text strings.Builder
	next := 0
	for id := range tree.Len() {
		n := tree.At(NodeID(id))
		if len(n.Children()) > 0 {
			continue
		}
		offset, end := n.Span()
		text.WriteString(n.Text())

		if offset == end {
			// No token behind it, so it answers to §6.1 rather than to the stream.
			if !isSynthesisable(n.Kind()) {
				t.Errorf("node %d is a zero-width %s, which is not a terminal the parser could "+
					"have expected", id, n.Kind())
			}
			continue
		}
		if next >= len(tokens) {
			t.Fatalf("node %d (%s %d..%d) is a leaf past the end of the token stream",
				id, n.Kind(), offset, end)
		}
		tk := tokens[next]
		if n.Kind() != Kind(tk.Kind) || offset != tk.Offset || end != tk.End() {
			t.Errorf("node %d is %s %d..%d, want token %d: %s %d..%d",
				id, n.Kind(), offset, end, next, tk.Kind, tk.Offset, tk.End())
		}
		next++
	}

	if next != len(tokens) {
		t.Errorf("the tree holds %d of the file's %d tokens; the rest are in no leaf",
			next, len(tokens))
	}
	if got := text.String(); got != src {
		t.Errorf("the leaves reconstruct %q, want %q", got, src)
	}
}

// assertTriviaIsNeverAtAnEdge is group 4, §2.3's half that index coverage cannot see: a comment
// in the wrong node still preserves order and still reconstructs. What it protects is tight inner
// spans, whose loss would mean Roslyn's Span/FullSpan split on every node forever. File is
// excepted because its own leading and trailing trivia have nowhere further out to go.
func assertTriviaIsNeverAtAnEdge(t reporter, tree *Tree) {
	t.Helper()
	for id := range tree.Len() {
		n := tree.At(NodeID(id))
		children := n.Children()
		if len(children) == 0 || n.Kind() == File {
			continue
		}
		for i, edge := range []Node{children[0], children[len(children)-1]} {
			if !isTrivia(edge.Kind()) {
				continue
			}
			where := "first"
			if i == 1 {
				where = "last"
			}
			offset, end := edge.Span()
			t.Errorf("%s at %d..%d is the %s child of %s: trivia belongs to the node that was "+
				"already open, not to the one it abuts", edge.Kind(), offset, end, where, n.Kind())
		}
	}
}

// --- the splice ---------------------------------------------------------------------------

// assertSpliceInvariants wants both streams because the contract is a relation between them:
// coverage alone would pass a splice that reordered the parser's events, and the placement rule
// alone would pass one that dropped half of them.
func assertSpliceInvariants(t reporter, tokens []token.Token, before, after eventStream) {
	t.Helper()
	assertIndexCoverage(t, tokens, after)
	assertSpliceOnlyInserts(t, tokens, before, after)
	assertTriviaIsNeverAtAnEventEdge(t, tokens, after)
}

// assertIndexCoverage is §2.3's first half: losslessness a stage before the tree exists, where it
// reads far better than a tree diff when trivia goes missing.
func assertIndexCoverage(t reporter, tokens []token.Token, events eventStream) {
	t.Helper()
	next := 0
	for i, e := range events {
		if e.kind != evToken {
			continue
		}
		if e.tok != next {
			t.Fatalf("event %d is token(%d) where token(%d) was due: the indices must be "+
				"{0..%d}, each once and in order", i, e.tok, next, len(tokens)-1)
		}
		next++
	}
	if next != len(tokens) {
		t.Fatalf("the stream carries %d of the file's %d tokens: the rest are in no leaf, and "+
			"the tree cannot reconstruct the source", next, len(tokens))
	}
}

// assertSpliceOnlyInserts is the pass's whole contract in one scan: **splice may insert trivia and
// drop empty nodes, and may do nothing else.** Everything the parser emitted comes through in
// order and unrenumbered, or is an open/close pair with nothing between it.
//
// The greedy match is exact *because* of the check on the loop's first line — the parser emits no
// trivia event, so an inserted one can never be mistaken for one of its own.
func assertSpliceOnlyInserts(t reporter, tokens []token.Token, before, after eventStream) {
	t.Helper()
	trivia := func(e event) bool {
		return e.kind == evToken && e.tok >= 0 && e.tok < len(tokens) && tokens[e.tok].IsTrivia()
	}

	for i, e := range before {
		if trivia(e) {
			t.Errorf("the parser's event %d is token(%d), which is trivia: it walks the filtered "+
				"stream, and splice is what puts trivia back", i, e.tok)
		}
	}
	if len(after) == 0 {
		if len(tokens) > 0 {
			t.Fatalf("splice emitted nothing for a file of %d tokens", len(tokens))
		}
		return
	}

	// Lockstep, skipping what each side is allowed to differ by. Matching greedily instead would
	// misalign the moment two identical opens appear with one of them elided — and closes are
	// identical always.
	drop := elidedEvents(before)
	i, j := 0, 0
	for i < len(before) || j < len(after) {
		switch {
		case i < len(before) && drop[i]:
			i++
		case j < len(after) && trivia(after[j]):
			j++
		case i == len(before):
			t.Fatalf("splice put %s at %d, past the parser's last event", after[j], j)
		case j == len(after):
			t.Fatalf("splice dropped the parser's event %d (%s)", i, before[i])
		case before[i] != after[j]:
			t.Fatalf("the parser's event %d is %s where splice has %s at %d: it may insert trivia "+
				"and drop empty nodes, and nothing else", i, before[i], after[j], j)
		default:
			i, j = i+1, j+1
		}
	}
}

// elidedEvents marks the events §2.2 drops: an open with no leaf between it and its close, and
// that close. It is computed from the parser's stream rather than copied from splice — "this node
// is empty" is a property of the input, and the point is to check the pass against it.
//
// The root is the exception splice makes for trailing trivia, so it is never marked; a file with
// no content at all produces no events, which the caller has already handled.
func elidedEvents(before eventStream) []bool {
	drop := make([]bool, len(before))
	var opens []int
	var filled []bool
	for i, e := range before {
		switch e.kind {
		case evOpen:
			opens, filled = append(opens, i), append(filled, false)
		case evClose:
			if len(opens) == 0 {
				continue // unbalanced, which splice itself rejects
			}
			at, content := opens[len(opens)-1], filled[len(filled)-1]
			opens, filled = opens[:len(opens)-1], filled[:len(filled)-1]
			switch {
			case at == 0: // the root
			case !content:
				drop[at], drop[i] = true, true
			case len(filled) > 0:
				filled[len(filled)-1] = true // a surviving child is content for its parent
			}
		default:
			if len(filled) > 0 {
				filled[len(filled)-1] = true
			}
		}
	}
	return drop
}

// assertTriviaIsNeverAtAnEventEdge is §2.1's invariant a stage before the tree: it needs no
// builder and names the offending event. Since §2.2 drops empty nodes here rather than in the
// builder, the two forms now see the same thing — which is the point of holding the open, and was
// not true when a node could vanish between the trivia and the tree.
func assertTriviaIsNeverAtAnEventEdge(t reporter, tokens []token.Token, events eventStream) {
	t.Helper()
	depth := 0
	for i, e := range events {
		switch e.kind {
		case evOpen:
			depth++
			continue
		case evClose:
			depth--
			continue
		case evToken:
			if e.tok < 0 || e.tok >= len(tokens) || !tokens[e.tok].IsTrivia() {
				continue
			}
		default:
			continue
		}
		if depth <= 1 {
			continue // File's own, the one place the invariant admits trivia at an edge
		}
		if i > 0 && events[i-1].kind == evOpen {
			t.Errorf("event %d is trivia directly after %s: it would be that node's first child",
				i, events[i-1])
		}
		if i+1 < len(events) && events[i+1].kind == evClose {
			t.Errorf("event %d is trivia directly before a close: it would be that node's last "+
				"child", i)
		}
	}
}
