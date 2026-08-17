package parser

import (
	"fmt"

	"luna/oracle/source"
	"luna/oracle/token"
)

// build turns a spliced event stream into the tree, and is the only thing that constructs one,
// which is what makes §4.3's immutability an invariant rather than a convention. Its own rule is
// §4.2's, that a node's span is its children's extent; §6.1's elision belongs to splice, which
// holds an open until content arrives, so an empty node arriving here is a violation to reject.
//
// **Every precondition is checked and every violation panics.** The stream is our own parser's,
// so a violation is a programmer error, and a corrupt tree is undetectable downstream.
//
// Coverage is the exception, deliberately: it belongs to splice, and not requiring it here is
// what lets a test build from an unspliced stream and compare the two readings.
func build(f *source.File, tokens []token.Token, events eventStream) *Tree {
	b := builder{tree: &Tree{src: f}, lastTok: -1}
	for i, e := range events {
		if b.done {
			panic(fmt.Sprintf("parser: event %d follows the root's close", i))
		}
		switch e.kind {
		case evOpen:
			if !isNode(e.node) {
				panic(fmt.Sprintf("parser: event %d opens %s, which is not a node kind", i, e.node))
			}
			b.open(e.node)
		case evToken:
			tk := tokens[b.index(e.tok, i, len(tokens))]
			b.leaf(Kind(tk.Kind), tk.Offset, tk.End(), i)
		case evMissing:
			if !isSynthesisable(e.node) {
				panic(fmt.Sprintf("parser: event %d synthesises %s, which is not a terminal the "+
					"parser could have expected", i, e.node))
			}
			b.leaf(e.node, b.pos, b.pos, i) // §6.1: zero width, at the insertion point
		case evClose:
			b.close(i)
		default:
			panic(fmt.Sprintf("parser: event %d has kind %d", i, e.kind))
		}
	}
	if n := len(b.stack); n > 0 {
		panic(fmt.Sprintf("parser: the stream ends inside %s, %d deep",
			b.tree.nodes[b.stack[n-1].id].kind, n))
	}
	if len(b.tree.nodes) == 0 {
		return nil // the empty file, and only it (§6.1)
	}
	return b.tree
}

// builder walks the stream. Storage is pre-order, so an open is an append and a close a patch:
// a node's size is how far the arena grew while it was on the stack.
type builder struct {
	tree    *Tree
	stack   []frame
	pos     int  // the end of the last leaf: where a synthesised one goes
	lastTok int  // so the ascent is checked rather than assumed
	done    bool // catches a second root and any trailing event with one flag
}

func (b *builder) index(tok, at, n int) int {
	if tok < 0 || tok >= n {
		panic(fmt.Sprintf("parser: event %d is token(%d) of a stream of %d", at, tok, n))
	}
	if tok <= b.lastTok {
		panic(fmt.Sprintf("parser: event %d is token(%d) after token(%d): the stream consumes "+
			"the file in order, each token once", at, tok, b.lastTok))
	}
	b.lastTok = tok
	return tok
}

// frame is an open node's children's extent so far. filled is §6.1's rule: a frame that closes
// without one produces nothing.
type frame struct {
	id          NodeID
	offset, end int
	filled      bool
}

func (f *frame) cover(offset, end int) {
	if !f.filled {
		f.offset, f.filled = offset, true
	}
	f.end = end
}

func (b *builder) open(k Kind) {
	parent := NodeID(0) // the root is its own parent; Parent reports false there
	if n := len(b.stack); n > 0 {
		parent = b.stack[n-1].id
	}
	b.stack = append(b.stack, frame{id: NodeID(len(b.tree.nodes))})
	b.tree.nodes = append(b.tree.nodes, node{kind: k, parent: parent})
}

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

func (b *builder) close(at int) {
	n := len(b.stack)
	if n == 0 {
		panic(fmt.Sprintf("parser: event %d closes a node that was never opened", at))
	}
	fr := b.stack[n-1]
	b.stack = b.stack[:n-1]
	if n == 1 {
		b.done = true
	}
	if !fr.filled {
		panic(fmt.Sprintf("parser: event %d closes %s with no children: splice drops an empty "+
			"node rather than emitting one (§6.1)", at, b.tree.nodes[fr.id].kind))
	}
	d := &b.tree.nodes[fr.id]
	d.size = uint32(len(b.tree.nodes)) - uint32(fr.id)
	d.offset, d.end = uint32(fr.offset), uint32(fr.end)
	if n > 1 {
		b.stack[n-2].cover(fr.offset, fr.end)
	}
}
