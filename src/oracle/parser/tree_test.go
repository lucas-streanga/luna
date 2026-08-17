// The arena and the navigation API (§3.1), against a tree written by hand — the same seam §4.2
// gives the builder one stage up, so the storage is judged by use without waiting for a parser.
package parser

import (
	"strings"
	"testing"

	"luna/oracle/source"
	"luna/oracle/token"
)

// The trailing WHITESPACE hangs off File rather than off Statement — §2.1 in miniature, since
// close happens before pending trivia is flushed.
const handSource = "x;\n"

func handTree(t *testing.T) *Tree {
	t.Helper()
	f, err := source.New("hand.luna", handSource)
	if err != nil {
		t.Fatalf("building the source file: %v", err)
	}
	return &Tree{
		src: f,
		nodes: []node{
			{kind: File, parent: 0, size: 5, offset: 0, end: 3},
			{kind: Statement, parent: 0, size: 3, offset: 0, end: 2},
			{kind: Kind(token.Ident), parent: 1, size: 1, offset: 0, end: 1},
			{kind: Kind(token.Semicolon), parent: 1, size: 1, offset: 1, end: 2},
			{kind: Kind(token.Whitespace), parent: 0, size: 1, offset: 2, end: 3},
		},
	}
}

func TestTreeNavigation(t *testing.T) {
	tree := handTree(t)
	if got := tree.Len(); got != 5 {
		t.Fatalf("Len is %d, want 5", got)
	}

	root := tree.Root()
	if root.Kind() != File {
		t.Errorf("root is %s, want File", root.Kind())
	}
	if _, ok := root.Parent(); ok {
		t.Error("the root reports a parent")
	}
	if got := root.Text(); got != handSource {
		t.Errorf("root text is %q, want the whole file %q", got, handSource)
	}

	kids := root.Children()
	if len(kids) != 2 {
		t.Fatalf("File has %d children, want Statement and WHITESPACE", len(kids))
	}
	if kids[0].Kind() != Statement || kids[1].Kind() != Kind(token.Whitespace) {
		t.Fatalf("File's children are %s and %s", kids[0].Kind(), kids[1].Kind())
	}

	stmt := kids[0]
	if o, e := stmt.Span(); o != 0 || e != 2 {
		t.Errorf("Statement spans %d..%d, want 0..2 — its children's extent, not the file's", o, e)
	}
	leaves := stmt.Children()
	if len(leaves) != 2 {
		t.Fatalf("Statement has %d children, want IDENT and SEMICOLON", len(leaves))
	}
	for i, want := range []string{"x", ";"} {
		if got := leaves[i].Text(); got != want {
			t.Errorf("leaf %d is %q, want %q", i, got, want)
		}
		if !leaves[i].Kind().IsToken() {
			t.Errorf("leaf %d has node kind %s", i, leaves[i].Kind())
		}
		if p, ok := leaves[i].Parent(); !ok || p.ID() != stmt.ID() {
			t.Errorf("leaf %d's parent is not the Statement", i)
		}
	}
}

// TestTreeAtRejectsAnUnknownID pins the documented panic: a NodeID means nothing without its
// tree, so one from another tree — or from a previous parse, IDs being stable within one only —
// is a caller error rather than something to return zero for.
func TestTreeAtRejectsAnUnknownID(t *testing.T) {
	tree := handTree(t)
	assertPanics(t, "no node 5 in a tree of 5", func() { tree.At(NodeID(tree.Len())) })
}

// TestTreeParentClimbsToTheRoot walks the direction §8's view will: Parent is stored rather than
// derived, so nothing else in the package would notice it going wrong more than one hop up.
func TestTreeParentClimbsToTheRoot(t *testing.T) {
	tree := handTree(t)
	n, hops := tree.At(3), 0 // the SEMICOLON, two levels down
	for {
		parent, ok := n.Parent()
		if !ok {
			break
		}
		n, hops = parent, hops+1
		if hops > tree.Len() {
			t.Fatalf("the parent chain from node 3 does not reach the root")
		}
	}
	if hops != 2 {
		t.Errorf("the SEMICOLON is %d hops from the root, want 2", hops)
	}
	if n.ID() != 0 || n.Kind() != File {
		t.Errorf("the chain ends at node %d (%s), want the File at 0", n.ID(), n.Kind())
	}
}

// TestTreeLeavesTileTheSource is the reconstruction invariant, in the form it will run over
// every golden. It holds only because trivia are nodes rather than attachments on tokens (§2),
// which is what makes losslessness structural instead of a rule every node type must remember.
func TestTreeLeavesTileTheSource(t *testing.T) {
	tree := handTree(t)
	var b strings.Builder
	for id := range tree.Len() {
		n := tree.At(NodeID(id))
		if len(n.Children()) == 0 {
			b.WriteString(n.Text())
		}
	}
	if got := b.String(); got != handSource {
		t.Errorf("the leaves reconstruct %q, want %q", got, handSource)
	}
}
