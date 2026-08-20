package parser

import "testing"

// The markers' specs. They pin **behaviour, not representation**: a marker is an index today
// (§4.6), and what the parser depends on is where an open lands and which node a close closes.

func TestMarkerOpenAndCompleteBracketTheEvents(t *testing.T) {
	expects(t, func(t *testing.T) {
		p := parserFor(t, "x")
		m := p.open(Block)
		p.bump()
		p.complete(m)
		if got, want := events(p.events), "open(Block) token(0) close"; got != want {
			t.Errorf("events are %q, want %q", got, want)
		}
		if len(p.stack) != 0 {
			t.Errorf("%d nodes left open after complete", len(p.stack))
		}
	})
}

// An open that never gets content still emits its pair: dropping it is splice's job (§6.1), not
// the parser's, which is what makes a speculative open free.
func TestMarkerOpenEmitsEvenWhenEmpty(t *testing.T) {
	expects(t, func(t *testing.T) {
		p := parserFor(t, "x")
		p.complete(p.open(Prelude))
		if got, want := events(p.events), "open(Prelude) close"; got != want {
			t.Errorf("events are %q, want %q — the parser emits the pair and splice elides it", got, want)
		}
	})
}

func TestMarkerMarkEmitsNothing(t *testing.T) {
	expects(t, func(t *testing.T) {
		p := parserFor(t, "x")
		p.mark()
		if len(p.events) != 0 {
			t.Errorf("mark emitted %d events; the tier below must write into the enclosing node",
				len(p.events))
		}
		if len(p.stack) != 0 {
			t.Errorf("mark opened %d nodes; nothing is open until it fires", len(p.stack))
		}
	})
}

// The tier idiom, end to end: the open lands *before* the operand that was already emitted.
func TestMarkerPrecedeWrapsWhatWasEmittedSinceTheMark(t *testing.T) {
	expects(t, func(t *testing.T) {
		p := parserFor(t, "a + b")
		m := p.mark()
		p.bump() // the left operand, emitted before anything is opened
		w := p.precede(m, Additive)
		p.bump()
		p.bump()
		p.complete(w)
		want := "open(Additive) token(0) token(2) token(4) close"
		if got := events(p.events); got != want {
			t.Errorf("events are %q, want %q", got, want)
		}
	})
}

// A tier that does not fire leaves no trace at all, the rule that makes the tree equal a
// golden's tree section (§4.8).
func TestMarkerAMarkThatNeverPrecedesCostsNothing(t *testing.T) {
	expects(t, func(t *testing.T) {
		p := parserFor(t, "a")
		p.mark()
		p.bump()
		if got, want := events(p.events), "token(0)"; got != want {
			t.Errorf("events are %q, want %q", got, want)
		}
	})
}

// Nesting: an inner precede inserts at a higher index than an outer mark, so the outer mark stays
// valid. That is why a tier chain needs no mark fixups, and it is worth pinning because nothing
// in the signature says so.
func TestMarkerAnInnerPrecedeLeavesAnOuterMarkValid(t *testing.T) {
	expects(t, func(t *testing.T) {
		p := parserFor(t, "a + b == c")
		outer := p.mark()
		inner := p.mark()
		p.bump() // a
		iw := p.precede(inner, Additive)
		p.bump() // +
		p.bump() // b
		p.complete(iw)
		ow := p.precede(outer, Equality)
		p.bump() // ==
		p.bump() // c
		p.complete(ow)
		want := "open(Equality) open(Additive) token(0) token(2) token(4) close token(6) token(8) close"
		if got := events(p.events); got != want {
			t.Errorf("events are %q, want %q", got, want)
		}
	})
}

// complete's argument is a self-check: closing out of order is a misnesting, and catching it here
// names the call site rather than leaving build to panic far away.
func TestMarkerCompleteRejectsAMisnesting(t *testing.T) {
	expectsPanic(t, "parser:", func() {
		p := parserFor(t, "x")
		outer := p.open(Block)
		p.open(Statement)
		p.complete(outer) // the inner node is still open
	})
}

// precede's precondition: nothing opened at or after the mark may still be open, or the inserted
// open lands outside a node whose close has not arrived.
func TestMarkerPrecedeRejectsAnOpenNodeAfterTheMark(t *testing.T) {
	expectsPanic(t, "parser:", func() {
		p := parserFor(t, "x")
		m := p.mark()
		p.open(Block) // still open when precede is asked to insert before it
		p.precede(m, Additive)
	})
}
