package parser

import "luna/oracle/token"

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
func splice(toks []token.Token, evs eventStream) eventStream {
	panic("parser: splice is unimplemented")
}
