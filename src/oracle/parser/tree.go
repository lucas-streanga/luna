package parser

import (
	"luna/oracle/diagnostic"
	"luna/oracle/source"
)

// NodeID is a node's index into its tree's arena (§3.1). An integer rather than a pointer so
// that §8's side tables are slices instead of maps that cannot be written to disk, and so a few
// million nodes are not a few million allocations.
//
// IDs are stable within one parse and not across parses: §3 reparses whole files, so anything
// wanting identity across a keystroke needs a name of its own.
type NodeID uint32

// Tree is one parsed file: tooling §2's lossless CST, trivia and all.
//
// It is immutable once built (§4.3). Compiler §2 parses modules in parallel and the LSP reads
// concurrently, so an immutable tree needs no synchronisation and can be shared by reference;
// annotations do not need mutability either, §8 keying side tables by NodeID.
type Tree struct {
	src   *source.File
	nodes []node
}

// node is one arena slot. Storage is pre-order — the choice §3.1 left to the keyboard — so that
// a subtree is the contiguous range [id, id+size): the builder's open/close becomes
// append-and-patch with no child-index array beside it, and "every node" is a loop.
//
// Spans are absolute offsets (§3) rather than widths against a cursor, which forecloses the
// green/red split's edit-time reuse and buys context-free span queries. Nothing here asks for
// sub-file incrementality: the build cache is module-granular and a full reparse is
// microseconds. The parent is stored so that ascent costs no scan.
type node struct {
	kind        Kind
	parent      NodeID
	size        uint32 // nodes in this subtree, itself included; a leaf is 1
	offset, end uint32 // the node's own [offset, end) in bytes
}

// Node is a cursor: a NodeID and the tree it belongs to, so that it reads exactly like a node
// pointer while the storage stays an arena (§3.1). The zero Node belongs to no tree.
type Node struct {
	t  *Tree
	id NodeID
}

// Source returns the file this tree was parsed from. The parser is handed it and never lexes
// (§4.4).
func (t *Tree) Source() *source.File { return t.src }

// Len is the number of nodes. IDs are dense and pre-order, so a loop over 0..Len()-1 is a
// whole-tree walk — which is how §2.3's invariants are checked without a traversal.
func (t *Tree) Len() int { return len(t.nodes) }

// Root is the File node, always ID 0. Every tree has one: an empty file has no File node and no
// tree either (§6.1).
func (t *Tree) Root() Node { return t.At(0) }

// At returns a cursor onto one node. It panics on an unknown ID: that is a caller mixing trees
// or holding one across a reparse, and neither is recoverable (§3.1).
func (t *Tree) At(id NodeID) Node {
	if int(id) >= len(t.nodes) {
		panic(diagnostic.Bugf("parser: no node %d in a tree of %d", id, len(t.nodes)))
	}
	return Node{t: t, id: id}
}

// ID is the key §8's side tables use.
func (n Node) ID() NodeID { return n.id }

// Kind is what the node is.
func (n Node) Kind() Kind { return n.t.nodes[n.id].kind }

// Span is the node's half-open byte range, an interior node's being its children's extent. Zero
// width is meaningful rather than degenerate: it is how a missing token is represented, and on
// Error it is the classification (§6.1, §6.2).
func (n Node) Span() (offset, end int) {
	d := n.t.nodes[n.id]
	return int(d.offset), int(d.end)
}

// Text is the source the node spans. Trivia are nodes like any other (§2), so the root's text
// is the file — that identity is losslessness, stated as one comparison.
func (n Node) Text() string {
	offset, end := n.Span()
	return n.t.src.Slice(offset, end-offset)
}

// Children returns the node's children in source order. Only a leaf has none: the builder emits
// no empty interior nodes (§6.1).
func (n Node) Children() []Node {
	d := n.t.nodes[n.id]
	var out []Node
	for i := n.id + 1; i < n.id+NodeID(d.size); i += NodeID(n.t.nodes[i].size) {
		out = append(out, Node{t: n.t, id: i})
	}
	return out
}

// Parent returns the node's parent, and false at the root.
func (n Node) Parent() (Node, bool) {
	if n.id == 0 {
		return Node{}, false
	}
	return Node{t: n.t, id: n.t.nodes[n.id].parent}, true
}
