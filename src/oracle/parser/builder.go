package parser

import (
	"fmt"

	"luna/oracle/source"
	"luna/oracle/token"
)

// build turns a spliced event stream into the tree. Being the only thing that constructs a Tree
// is what makes §4.3's immutability an invariant rather than a convention, and it is uniform on
// purpose: every token event becomes a leaf, trivia included, since all the placement thinking
// happened in splice.
//
// Two rules are its alone. **No empty interior nodes**, which is what lets width distinguish a
// synthesised leaf from a real one — a missing token is a zero-width leaf of the kind expected
// (§6.1), so a zero-width Prelude beside it would be indistinguishable. And **a node's span is
// its children's extent**, the arithmetic §4.2 keeps goldens over the tree for, an off-by-one
// in it appearing in no event dump.
//
// The first has no exception, the root included: a zero-byte file lexes to no tokens, so File is
// empty, the rule deletes it, and build returns nil. §6.1 carries the reasoning and the iff that
// keeps nil unambiguous.
//
// evs must be spliced. An unspliced stream builds a tree that drops every comment, which
// reconstruction catches only after the fact.
//
// A malformed stream panics rather than building something shaped wrongly: an unopened close, a
// leaf outside every node, a second root, or an unclosed node at the end are all the parser
// having lost its stack, and none of them is recoverable here.
func build(f *source.File, toks []token.Token, evs eventStream) *Tree {
	b := builder{tree: &Tree{src: f}}
	for i, e := range evs {
		switch e.kind {
		case evOpen:
			b.open(e.node, i)
		case evToken:
			tk := toks[e.tok]
			b.leaf(Kind(tk.Kind), tk.Offset, tk.End(), i)
		case evMissing:
			b.leaf(e.node, b.pos, b.pos, i) // zero width, at the insertion point (§6.1)
		case evClose:
			b.close(i)
		default:
			panic(fmt.Sprintf("parser: event %d has kind %d", i, e.kind))
		}
	}
	if len(b.stack) > 0 {
		panic(fmt.Sprintf("parser: the stream ends with %d nodes still open", len(b.stack)))
	}
	if len(b.tree.nodes) == 0 {
		return nil // the empty file, and only it (§6.1)
	}
	return b.tree
}

// builder is the walk's state. Storage being pre-order means an open is an append and a close is
// a patch: a node's size is how far the arena grew while it was on the stack, so no child-index
// array is needed and a subtree stays the contiguous range the navigation API reads.
type builder struct {
	tree  *Tree
	stack []frame
	pos   int // the end of the last leaf: where a synthesised one goes, and the only cursor kept
}

// frame is an open node's span so far — its children's extent, which is what a node's span is.
// filled is the whole of §6.1's rule: a frame that closes without one produces nothing.
type frame struct {
	id          NodeID
	offset, end int
	filled      bool
}

// cover widens the frame to include a child.
func (f *frame) cover(offset, end int) {
	if !f.filled {
		f.offset, f.filled = offset, true
	}
	f.end = end
}

// open appends the node and pushes it. The span is left for close, which is the first moment it
// is known.
func (b *builder) open(k Kind, at int) {
	parent := NodeID(0) // the root is its own parent; Parent reports false there
	if n := len(b.stack); n > 0 {
		parent = b.stack[n-1].id
	} else if len(b.tree.nodes) > 0 {
		panic(fmt.Sprintf("parser: event %d opens a second root", at))
	}
	b.stack = append(b.stack, frame{id: NodeID(len(b.tree.nodes))})
	b.tree.nodes = append(b.tree.nodes, node{kind: k, parent: parent})
}

// leaf appends a leaf and widens its parent over it. Trivia arrives here like anything else,
// which is what §2.2 bought by splicing rather than by teaching the builder about it.
func (b *builder) leaf(k Kind, offset, end, at int) {
	n := len(b.stack)
	if n == 0 {
		panic(fmt.Sprintf("parser: event %d is a leaf outside every node", at))
	}
	b.tree.nodes = append(b.tree.nodes, node{
		kind:   k,
		parent: b.stack[n-1].id,
		size:   1,
		offset: uint32(offset),
		end:    uint32(end),
	})
	b.stack[n-1].cover(offset, end)
	b.pos = end
}

// close patches the node's size and span, or deletes it. Deletion is safe precisely because it
// is empty: nothing was appended after it, so truncating the arena to its own index removes it
// and nothing else.
func (b *builder) close(at int) {
	n := len(b.stack)
	if n == 0 {
		panic(fmt.Sprintf("parser: event %d closes a node that was never opened", at))
	}
	fr := b.stack[n-1]
	b.stack = b.stack[:n-1]
	if !fr.filled {
		b.tree.nodes = b.tree.nodes[:fr.id]
		return
	}
	d := &b.tree.nodes[fr.id]
	d.size = uint32(len(b.tree.nodes)) - uint32(fr.id)
	d.offset, d.end = uint32(fr.offset), uint32(fr.end)
	if n > 1 {
		b.stack[n-2].cover(fr.offset, fr.end)
	}
}
