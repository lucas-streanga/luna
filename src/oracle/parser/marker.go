package parser

// Markers: how a nonterminal function says where its node begins.
//
// A node's `open` and its `close` are separated by everything between them, so the parser needs a
// handle on the first to emit the second. A marker is that handle, and it is **an index into the
// event stream** — the flattest thing that can be one, and available because §4 chose a stream
// over a tree.
//
// Two ways to make one, because §0's productions come in two shapes:
//
//   - **`open(k)`** for a nonterminal that knows its name before its children — `Block`, `FnLit`,
//     `IfStmt`, `Type`, and every other production whose first symbol commits it. The event is
//     emitted now and closing is a patch-free append.
//   - **`mark()` then `precede(m, k)`** for the precedence tiers, which know their name and do not
//     yet know whether they have a node at all. `Additive` is an `Additive` only once a `PLUS`
//     arrives; before that it is whatever `Multiplicative` returned, and §5's elision is exactly
//     that a tier which did not fire never existed. `mark` emits nothing, so the tier below writes
//     straight into the enclosing node, and `precede` retroactively wraps the run when the operator
//     fires.
//
// **The kind is given at `open` and `precede`, never at `complete`.** rust-analyzer's shape is the
// other one — `start()` emits a placeholder and `complete(kind)` patches it — and it does not pay
// here: `evOpen` would then have a transient state naming nothing, which `build` already rejects
// (`isNode(Unset)` is false), so every reader of a partial stream, the debug dump included, would
// have to know that an open may not mean anything yet. What that buys is deciding a node's name
// after seeing its children, and no §0 production wants it. A tier knows its name; it does not know
// whether it fires.
//
// **There is no `abandon`, and the case for one has no members.** A speculative open either ends up
// empty or it does not. Empty, splice already deletes it (§2.2, §6.1) — that is what makes an
// `open(Prelude)` on a file with no imports free, and it is pinned by the splice fuzzers rather
// than by a second implementation of the same rule here. Not empty — the tier that consumed its
// operand and then did not fire — is served by `mark`, which never emitted the open there was to
// abandon. Abandoning a *non-empty* open is unwrapping rather than deleting, which the stream has
// no event for and would need a tombstone and a fixup pass to acquire. Nothing in §0 asks, so
// nothing is built; if Phase 3's recovery ever does, the answer is to look again at `precede`
// before adding a state to the stream.

// A marker is a position in the event stream: where a node's open sits, or where one could be
// inserted. It is meaningful only to the parse that made it.
type marker int

// open starts a node of kind k at the cursor, emitting the open now. For every nonterminal whose
// first symbol commits it, which is all of them but the tiers.
//
// Opening speculatively is free (§6.1): an open whose close arrives with nothing between them is
// dropped by splice, so `ParamList` on `fn ()` and `Prelude` on a file with no imports cost an
// event pair and no node. The rule that makes it free is the same one that forbids an empty node
// reaching the builder, so this is not a licence to be careless — it is the one place where being
// careless is already handled.
func (p *parser) open(k Kind) marker {
	panic("parser: open is unimplemented")
}

// mark remembers where a node would begin, and emits nothing.
//
// The tier idiom, and the reason a golden's tree section is literally the tree:
//
//	m := p.mark()
//	p.multiplicative()
//	if !p.at(token.Plus) && !p.at(token.Minus) {
//		return // did not fire: the operand passes through, and Additive never existed
//	}
//	w := p.precede(m, Additive)
//	for p.at(token.Plus) || p.at(token.Minus) {
//		p.bump()
//		p.multiplicative()
//	}
//	p.complete(w)
func (p *parser) mark() marker {
	panic("parser: mark is unimplemented")
}

// precede opens a node of kind k at a mark taken earlier, wrapping everything emitted since it,
// and returns the marker that must be completed.
//
// It is an insertion into the event slice, so it costs the events emitted since the mark — the
// left operand's subtree, once, at the moment the operator is seen. Left-associative runs are
// loops rather than recursion, so a chain of any length pays it once; the shape that pays it
// repeatedly is a tier firing inside a tier inside parentheses, where the input has to nest to
// make the cost grow. Measured against the alternative — rust-analyzer's forward-parent link and
// the fixup pass that resolves it — that is a real O(1) traded for a state in the stream and a
// fourth pass, and the trade is not worth making before a profile asks for it.
//
// **Its precondition is that nothing opened at or after m is still open**, or the inserted open
// would land outside a node whose close has not arrived and the stream would stop nesting. It
// panics rather than checks quietly: the caller is our own parser, and a misnested stream is a
// panic in `build` a long way from the mistake.
func (p *parser) precede(m marker, k Kind) marker {
	panic("parser: precede is unimplemented")
}

// complete closes the node m opened.
//
// It takes the marker although a close names no node, because the parser keeps the open markers on
// a stack and can therefore assert that the one being closed is the one on top. That turns every
// misnesting into a panic at the call site that made it, where the alternative is a panic in the
// builder — or, when the misnesting happens to balance, a wrong tree and a golden diff.
func (p *parser) complete(m marker) {
	panic("parser: complete is unimplemented")
}
