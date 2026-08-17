package parser

import (
	"fmt"

	"luna/oracle/token"
)

// splice fills trivia into a parse's event stream, which is what lets the parser run on the
// trivia-filtered stream grammar.md §0 is defined over and the builder stay unaware that trivia
// is special. §2.2 has the rule in table form; it reduces to one sentence:
//
//	close happens before pending trivia is flushed, and open is deferred until it has been.
//
// Both halves push trivia outward, into the innermost node already open when it occurred, and
// the guard that skips the flush while nothing is open is what keeps a file's leading trivia
// inside File. That keeps inner spans tight — a node whose first child were a comment would
// start at that comment, and recovering a tight span would need Roslyn's Span/FullSpan split,
// two accessors on every node forever — and it buys one invariant: **trivia is never the first
// or last child of any node except File**.
//
// It is a pass rather than three conditions inside the builder because of where that puts the
// test seam: here the rule is one function tested events in, events out (§4.2), where inside
// the builder it would be reachable only through tree shape.
//
// Neither golden format can see the rule, so §2.3 asserts it directly — after splicing the
// token indices are exactly {0..n-1}, each once and in order.
//
// The table's last row — "at the end, emit the remaining trivia, then close the root" — is
// implemented as the close that returns the depth to zero, because that is where the end of the
// file is in a balanced stream. The parser opens and closes File itself: §6.1 turns on its doing
// so, an empty file being File opened and closed with nothing between.
//
// It checks the two things it depends on and panics on either, for the reason build does: the
// stream is our own parser's, so a violation is a programmer error. **Token indices must be in
// range and strictly ascending**, or the flush emits the wrong run and coverage breaks silently;
// **the stream must be balanced**, or the depth never returns to zero and the file's trailing
// trivia is dropped without trace. Kinds it never inspects, so it never checks them — build does,
// and one violation caught once by the pass that depends on it is the whole rule.
func splice(toks []token.Token, evs eventStream) eventStream {
	out := make(eventStream, 0, len(evs)+len(toks))
	next, depth, last := 0, 0, -1

	// flush emits the run of trivia at next, which is what an open defers to and a close does
	// not do at all. Both halves push trivia outward, into the innermost node already open.
	flush := func() {
		for next < len(toks) && toks[next].IsTrivia() {
			out = append(out, event{kind: evToken, tok: next})
			next++
		}
	}

	for i, e := range evs {
		switch e.kind {
		case evToken:
			if e.tok < 0 || e.tok >= len(toks) {
				panic(fmt.Sprintf("parser: event %d is token(%d) of a stream of %d",
					i, e.tok, len(toks)))
			}
			if e.tok <= last {
				panic(fmt.Sprintf("parser: event %d is token(%d) after token(%d): the parser "+
					"consumes the file in order, each token once", i, e.tok, last))
			}
			last = e.tok
			for ; next < e.tok; next++ { // trivia by construction: the parser skipped nothing else
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
		default: // evMissing consumes no index and no bytes, so there is nothing to flush before it
			out = append(out, e)
		}
	}
	if depth != 0 {
		panic(fmt.Sprintf("parser: the stream ends at depth %d: every node the parser opened "+
			"must be closed, or the file's trailing trivia has nowhere to go", depth))
	}
	return out
}
