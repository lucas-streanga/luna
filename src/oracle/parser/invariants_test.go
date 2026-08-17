// The invariant battery: what holds of **every** tree and **every** splice, whatever was parsed.
//
// Two entry points, four groups. They take a tree and a token stream rather than anything this
// phase owns, so the drivers can differ while the properties do not: hand-written events today,
// the golden corpus today, the spec corpus and the fuzz targets next, and `Parse` itself when
// Phase 2 lands — at which point nothing here changes. That is the point of writing it as a
// battery rather than as assertions inside each test. A driver costs nothing to add, and an
// invariant added here applies to every driver at once.
//
// What it deliberately does not do is compare a tree against grammar.md. That is the goldens'
// job, and the division is the whole design: **goldens pin shape, invariants pin properties.** A
// golden says this file produces this tree; the battery says no tree, from any input, is ever
// malformed. Neither substitutes for the other, and only the second scales to a fuzzer.
package parser

import (
	"strings"

	"luna/oracle/token"
)

// reporter is the slice of *testing.T the battery uses, and it is an interface for one reason:
// **a test helper that cannot fail is worse than no helper at all.** Standing a recorder in for T
// is what lets invariants_probe_test.go feed each group a deliberately malformed tree and check
// that the right assertion fires. testing.TB would be the natural spelling and cannot be
// implemented outside the testing package, which is why this is written out.
type reporter interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// --- the tree ----------------------------------------------------------------------------

// assertTreeInvariants asserts every property a tree has by construction.
//
// **The precondition is the one Parse always meets: the tree was built from a spliced stream.**
// Three of the four groups turn on it — a tree built from the parser's own events holds no
// trivia, so its root does not span the file and its leaves are not the token stream. A test
// comparing the two readings calls the groups it wants instead.
func assertTreeInvariants(t reporter, tree *Tree, tokens []token.Token, src string) {
	t.Helper()

	// §6.1's iff, which build cannot enforce alone: it sees one stream and cannot know whether
	// the trivia was spliced in, so "no tree exactly when the file is empty" is asserted here.
	if tree == nil {
		if len(tokens) > 0 {
			t.Fatalf("no tree for a file of %d bytes in %d tokens", len(src), len(tokens))
		}
		return
	}
	if len(tokens) == 0 {
		t.Fatalf("a tree of %d nodes for a file with no tokens", tree.Len())
	}

	assertArenaIsATree(t, tree)
	assertSpansNest(t, tree, src)
	assertLeavesAreTheFile(t, tree, tokens, src)
	assertTriviaIsNeverAtAnEdge(t, tree)
}

// assertArenaIsATree is group 1: the storage really is one, and nothing else in the package
// checks it.
//
// Children navigates *by* size, so a wrong size does not fail — it silently returns a wrong child
// list, and every other assertion in this file would still pass, because they all reach the tree
// through that same accessor. Parent is worse: it is stored, read by one method, and no test
// walks upward far enough to notice. This group is what makes the rest trustworthy rather than
// self-consistent, which is why it reads the arena directly.
func assertArenaIsATree(t reporter, tree *Tree) {
	t.Helper()

	// Sizes first and on their own, because the tiling walk below steps *by* them: a size of
	// zero would spin it forever rather than fail it.
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

	// Every node is in the root's subtree, which in a pre-order arena is one comparison. Without
	// it a short root size hides the nodes past its end from every walk below — they are then in
	// no child list, and the tiling check that would have caught it never reaches them.
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
		// Stepping child by child must land exactly on the node after the subtree. Short means
		// a size that swallows a sibling, long means one that reaches past its parent, and both
		// give Children a wrong answer with no other symptom.
		if i != end {
			t.Fatalf("node %d (%s) spans %d nodes but its children reach %d",
				id, n.kind, n.size, i-id)
		}

		// A leaf carrying a nonterminal's kind is an empty interior node that survived — the
		// only trace §6.1's rule can leave in an arena where size 1 *means* leaf.
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

// assertSpansNest is group 2: a node's span is exactly its children's extent, and the children
// tile it without gap or overlap.
//
// Three checks rather than one because they fail differently. A parent narrower than its children
// is a cover that did not widen; a gap between siblings is a leaf that went missing locally; an
// extent that is right at the edges and wrong in between is a cover that widened over something
// it should not have. Containment and ordering both follow, so neither is asserted separately.
func assertSpansNest(t reporter, tree *Tree, src string) {
	t.Helper()

	if offset, end := tree.Root().Span(); offset != 0 || end != len(src) {
		t.Errorf("the root spans %d..%d, want 0..%d — File owns the file's leading and trailing "+
			"trivia, and is the only node whose span differs from its non-trivia extent",
			offset, end, len(src))
	}

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

// assertLeavesAreTheFile is group 3: losslessness, in the strong form.
//
// Concatenating the leaves is the familiar half and the weaker one — it passes with every leaf
// mislabelled, since a kind is not bytes. The half with teeth is that the **positive-width leaves
// are the token stream**: same kinds, same spans, same order, none missing and none invented.
// That is what catches a botched Kind conversion, a misalignment in the single kind space, or a
// real token quietly replaced by a synthesised one of the same width.
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
			// A zero-width leaf stands for a token that is not there, so it answers to §6.1
			// rather than to the stream: it must be a terminal the parser could have expected.
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

// assertTriviaIsNeverAtAnEdge is group 4 over the tree: §2.3's second half, and the half index
// coverage cannot see — a comment placed in the wrong node still preserves order and still
// reconstructs.
//
// The invariant is what keeps inner spans tight. A node whose first child were a comment would
// start at that comment, and recovering a tight span would need Roslyn's Span/FullSpan split, two
// accessors on every node forever. File is the exception because the file's own leading and
// trailing trivia have nowhere further out to go.
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

// assertSpliceInvariants asserts what is true of every splice: it inserts trivia, and does
// nothing else at all.
//
// It wants both streams because the contract is a relation between them. Coverage alone would
// pass a pass that reordered the parser's events, and the placement rule alone would pass one
// that dropped half of them.
func assertSpliceInvariants(t reporter, tokens []token.Token, before, after eventStream) {
	t.Helper()
	assertIndexCoverage(t, tokens, after)
	assertSpliceOnlyInserts(t, tokens, before, after)
	assertTriviaIsNeverAtAnEventEdge(t, tokens, after)
}

// assertIndexCoverage is §2.3's first half, and the whole of losslessness a stage before the tree
// exists: after splicing, the token indices are exactly {0..n-1}, each once and in order. It is
// far easier to read than a tree diff when trivia goes missing, which is the reason it is checked
// here as well as on the tree.
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

// assertSpliceOnlyInserts is the pass's entire contract in one scan: delete the events splice
// added and the parser's own stream is back, in order and unrenumbered. It may not reorder, drop,
// renumber or invent anything — only interleave trivia.
//
// The greedy match is exact because of the check on the first line of the loop: the parser walks
// the trivia-filtered stream, so none of its events is a trivia token, so an inserted event can
// never be mistaken for one of the parser's.
func assertSpliceOnlyInserts(t reporter, tokens []token.Token, before, after eventStream) {
	t.Helper()
	trivia := func(e event) bool {
		return e.kind == evToken && e.tok >= 0 && e.tok < len(tokens) && tokens[e.tok].IsTrivia()
	}

	j := 0
	for i, want := range before {
		if trivia(want) {
			t.Errorf("the parser's event %d is token(%d), which is trivia: it walks the filtered "+
				"stream, and splice is what puts trivia back", i, want.tok)
		}
		for j < len(after) && after[j] != want {
			if got := after[j]; !trivia(got) {
				t.Fatalf("splice put %s before the parser's event %d (%s): it may insert trivia "+
					"and nothing else", got, i, want)
			}
			j++
		}
		if j == len(after) {
			t.Fatalf("splice dropped the parser's event %d (%s)", i, want)
		}
		j++
	}
	for ; j < len(after); j++ {
		if got := after[j]; !trivia(got) {
			t.Fatalf("splice put %s after the parser's last event", got)
		}
	}
}

// assertTriviaIsNeverAtAnEventEdge is §2.1's invariant one stage earlier than the tree can see
// it: a trivia event may abut the root's open and close, and no other node's.
//
// Both forms are worth having. This one needs no tree, runs before the builder, and names the
// offending event; the tree form stays authoritative, because it also catches the case where an
// empty node was opened and deleted between the open and the trivia, which is invisible here.
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
