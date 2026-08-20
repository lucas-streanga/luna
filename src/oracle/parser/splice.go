package parser

import (
	"luna/oracle/diagnostic"
	"luna/oracle/token"
)

// splice fills trivia back into a parse's event stream, which is what lets the parser run on the
// filtered stream grammar.md §0 is defined over while the builder stays unaware trivia exists.
// §2.2 has the rule and the alternatives it beat; it reduces to one sentence:
//
//	an open is held until content arrives, and pending trivia is flushed when it is released.
//
// Holding is what keeps trivia out of a node that never existed. Flushing eagerly at each open
// was correct for every open that produces a node, and §6.1 does not promise one. The trivia
// then ended up as the *enclosing* node's last child, widening its span over bytes it does not
// own. §6.1's elision therefore lives here: an empty node is dropped rather than built and
// deleted, and the same rule at the root is "no tree for the empty file".
//
// It is a pass rather than conditions inside the builder because that is where the test seam
// goes: here the rule is one function tested events in, events out (§4.2).
//
// **It panics on the three preconditions it needs**, for the reason build does. Indices must be
// in range and ascending, the stream balanced, and every token accounted for. The last is the one
// splice needs to *deliver* coverage rather than one it reads, and it fails silently without a
// check: an unconsumed token is still emitted, still tiles, still reconstructs; it just lands in
// a node nobody put it in. It holds by design, §7.2 skipping a token *into* an Error node (§6.2)
// rather than past one. Kinds it never inspects, so it never checks them; build does.
func splice(tokens []token.Token, events eventStream) eventStream {
	out := make(eventStream, 0, len(events)+len(tokens))
	next, depth, last := 0, 0, -1
	var held []Kind // opens waiting for content

	flush := func() {
		for next < len(tokens) && tokens[next].IsTrivia() {
			out = append(out, event{kind: evToken, tok: next})
			next++
		}
	}

	// release emits the held opens, and first the trivia that was pending when the outermost of
	// them was held, which belongs to the node enclosing them all. The root inverts that: nothing
	// is open before File, so it is emitted first and the file's leading trivia flushes inside it,
	// the one place §2.1's invariant admits trivia at an edge.
	release := func() {
		if len(held) == 0 {
			return
		}
		start := 0
		if depth == 0 {
			out = append(out, event{kind: evOpen, node: held[0]})
			depth, start = 1, 1
		}
		flush()
		for _, k := range held[start:] {
			out = append(out, event{kind: evOpen, node: k})
			depth++
		}
		held = held[:0]
	}

	for i, e := range events {
		switch e.kind {
		case evToken:
			if e.tok < 0 || e.tok >= len(tokens) {
				panic(diagnostic.Bugf("parser: event %d is token(%d) of a stream of %d",
					i, e.tok, len(tokens)))
			}
			if e.tok <= last {
				panic(diagnostic.Bugf("parser: event %d is token(%d) after token(%d): the parser "+
					"consumes the file in order, each token once", i, e.tok, last))
			}
			// The parser walks the filtered stream, so it never consumes trivia, and a flush is
			// bounded only by that: it runs while the tokens stay trivia, so one aimed at a trivia
			// token would consume past it and emit the index twice.
			if tokens[e.tok].IsTrivia() {
				panic(diagnostic.Bugf("parser: event %d is token(%d), which is %s: the parser walks "+
					"the filtered stream, and splice is what puts trivia back",
					i, e.tok, tokens[e.tok].Kind))
			}
			last = e.tok
			release()
			for ; next < e.tok; next++ {
				if !tokens[next].IsTrivia() {
					panic(diagnostic.Bugf("parser: event %d skips token %d (%s): the parser consumes "+
						"every token, one it cannot place into an Error node rather than past it",
						i, next, tokens[next].Kind))
				}
				out = append(out, event{kind: evToken, tok: next})
			}
			out = append(out, e)
			next = e.tok + 1
		case evOpen:
			held = append(held, e.node)
		case evMissing:
			release() // a synthesised leaf is content, so the node it lands in survives
			out = append(out, e)
		case evClose:
			// Trailing trivia is content too, and has nowhere further out to go: it releases the
			// root even when the parse put nothing in it, which is what keeps a comment-only file
			// from vanishing along with its comments.
			if len(held) == 1 && depth == 0 && next < len(tokens) {
				release()
			}
			if len(held) > 0 {
				held = held[:len(held)-1] // the node never had content, so it never existed
				continue
			}
			depth--
			if depth < 0 {
				panic(diagnostic.Bugf("parser: event %d closes a node that was never opened", i))
			}
			if depth == 0 {
				flush()
			}
			out = append(out, e)
		default:
			out = append(out, e)
		}
	}
	if n := depth + len(held); n != 0 {
		panic(diagnostic.Bugf("parser: the stream ends at depth %d: every node the parser opened "+
			"must be closed, or the file's trailing trivia has nowhere to go", n))
	}
	// The flush above stopped at the first token that is not trivia, so anything left is one the
	// parser never reached.
	if next != len(tokens) {
		panic(diagnostic.Bugf("parser: the stream ends with token %d (%s) in no event: every token "+
			"reaches a leaf, or the tree cannot reconstruct the file",
			next, tokens[next].Kind))
	}
	return out
}
