package parser

// Markers: where a node begins (§4.6). Two ways to make one, because §0's productions come in
// two shapes — a nonterminal whose first symbol commits it opens eagerly, and a precedence tier,
// which knows its name but not yet whether it has a node at all, marks and precedes.
//
// There is no abandon, and §4.6 has the argument: an empty speculative open is splice's to drop
// (§6.1), and a non-empty one is what mark exists to avoid emitting.

// A marker is a position in the event stream, meaningful only to the parse that made it.
type marker int

// open emits the open now. Doing so speculatively is free: an open whose close arrives with
// nothing between them is dropped by splice (§6.1), so `ParamList` on `fn ()` costs an event pair
// and no node.
func (p *parser) open(k Kind) marker {
	panic("parser: open is unimplemented")
}

// mark emits nothing, so the tier below writes straight into the enclosing node. The tier idiom:
//
//	m := p.mark()
//	p.multiplicative()
//	if !p.at(token.Plus) && !p.at(token.Minus) {
//		return // did not fire: Additive never existed
//	}
//	w := p.precede(m, Additive)
//	…
//	p.complete(w)
func (p *parser) mark() marker {
	panic("parser: mark is unimplemented")
}

// precede inserts, so it costs the events emitted since the mark — the left operand's subtree,
// once, when the operator is seen. Left-associative runs are loops, so a chain of any length pays
// it once. rust-analyzer's forward-parent link buys a real O(1) for a state in the stream and a
// fourth pass, and that trade wants a profile first.
//
// **Its precondition is that nothing opened at or after m is still open**, or the inserted open
// lands outside a node whose close has not arrived. It panics: the caller is our own parser, and
// the alternative is a panic in build a long way from the mistake.
func (p *parser) precede(m marker, k Kind) marker {
	panic("parser: precede is unimplemented")
}

// complete takes the marker although a close names no node, so that the parser's stack can assert
// the node being closed is the one on top — turning a misnesting into a panic at the call site
// that made it rather than in the builder, or a wrong tree where it happens to balance.
func (p *parser) complete(m marker) {
	panic("parser: complete is unimplemented")
}
