package parser

import (
	"fmt"

	"luna/oracle/token"
)

// splice fills trivia back into a parse's event stream, which is what lets the parser run on the
// filtered stream grammar.md §0 is defined over while the builder stays unaware trivia exists.
// §2.2 has the rule and its table; it reduces to one sentence:
//
//	close happens before pending trivia is flushed, and open is deferred until it has been.
//
// It is a pass rather than three conditions in the builder because that is where the test seam
// goes: here the rule is one function tested events in, events out (§4.2).
//
// The table's last row is implemented as the close returning the depth to zero, which is where
// a balanced stream ends — the parser opens and closes File itself, as §6.1 requires.
//
// **It panics on the three preconditions it needs**, for the reason build does. Indices must be
// in range and ascending, the stream balanced, and every token accounted for. The last is the one
// splice needs to *deliver* coverage rather than one it reads, and it fails silently without a
// check: an unconsumed token is still emitted, still tiles, still reconstructs — it just lands in
// a node nobody put it in. It holds by design, §7.2 skipping a token *into* an Error node (§6.2)
// rather than past one. Kinds it never inspects, so it never checks them; build does.
func splice(tokens []token.Token, events eventStream) eventStream {
	out := make(eventStream, 0, len(events)+len(tokens))
	next, depth, last := 0, 0, -1

	// An open defers to this and a close never runs it, which is what pushes trivia outward.
	flush := func() {
		for next < len(tokens) && tokens[next].IsTrivia() {
			out = append(out, event{kind: evToken, tok: next})
			next++
		}
	}

	for i, e := range events {
		switch e.kind {
		case evToken:
			if e.tok < 0 || e.tok >= len(tokens) {
				panic(fmt.Sprintf("parser: event %d is token(%d) of a stream of %d",
					i, e.tok, len(tokens)))
			}
			if e.tok <= last {
				panic(fmt.Sprintf("parser: event %d is token(%d) after token(%d): the parser "+
					"consumes the file in order, each token once", i, e.tok, last))
			}
			last = e.tok
			for ; next < e.tok; next++ {
				if !tokens[next].IsTrivia() {
					panic(fmt.Sprintf("parser: event %d skips token %d (%s): the parser consumes "+
						"every token, one it cannot place into an Error node rather than past it",
						i, next, tokens[next].Kind))
				}
				out = append(out, event{kind: evToken, tok: next})
			}
			out = append(out, e)
			next = e.tok + 1
		case evOpen:
			if depth > 0 { // nothing is open before File, so the file's leading trivia is its
				flush()
			}
			out = append(out, e)
			depth++
		case evClose:
			depth--
			if depth < 0 {
				panic(fmt.Sprintf("parser: event %d closes a node that was never opened", i))
			}
			if depth == 0 {
				flush()
			}
			out = append(out, e)
		default: // evMissing covers no bytes, so there is nothing to flush before it
			out = append(out, e)
		}
	}
	if depth != 0 {
		panic(fmt.Sprintf("parser: the stream ends at depth %d: every node the parser opened "+
			"must be closed, or the file's trailing trivia has nowhere to go", depth))
	}
	// The flush above stopped at the first token that is not trivia, so anything left is one the
	// parser never reached.
	if next != len(tokens) {
		panic(fmt.Sprintf("parser: the stream ends with token %d (%s) in no event: every token "+
			"reaches a leaf, or the tree cannot reconstruct the file",
			next, tokens[next].Kind))
	}
	return out
}
