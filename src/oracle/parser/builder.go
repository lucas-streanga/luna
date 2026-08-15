package parser

import (
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
func build(f *source.File, toks []token.Token, evs eventStream) *Tree {
	panic("parser: build is unimplemented")
}
