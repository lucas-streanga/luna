package parser

import (
	"testing"

	"luna/oracle/token"
)

// commaList's specs. The item is `bump` throughout, so one token is one item and the events read
// directly: what is being pinned is the *separator* handling, which is the whole reason the helper
// exists.

func TestCommaListWrapsASingleItem(t *testing.T) {
	expects(t, func(t *testing.T) {
		p := parserFor(t, "a)")
		p.commaList(ArgList, token.RParen, p.bump)
		if got, want := events(p.events), "open(ArgList) token(0) close"; got != want {
			t.Errorf("events are %q, want %q", got, want)
		}
	})
}

func TestCommaListSeparatesItems(t *testing.T) {
	expects(t, func(t *testing.T) {
		p := parserFor(t, "a,b,c)")
		p.commaList(ArgList, token.RParen, p.bump)
		want := "open(ArgList) token(0) token(1) token(2) token(3) token(4) close"
		if got := events(p.events); got != want {
			t.Errorf("events are %q, want %q — the separators are children like anything else", got, want)
		}
	})
}

// The trailing comma is admitted everywhere and required nowhere, and telling it from a missing
// item is what the closer is for.
func TestCommaListAdmitsATrailingComma(t *testing.T) {
	expects(t, func(t *testing.T) {
		p := parserFor(t, "a,)")
		p.commaList(ArgList, token.RParen, p.bump)
		if got, want := events(p.events), "open(ArgList) token(0) token(1) close"; got != want {
			t.Errorf("events are %q, want %q", got, want)
		}
	})
}

// The closer belongs to the caller's production, so the list must leave it alone.
func TestCommaListLeavesTheCloser(t *testing.T) {
	expects(t, func(t *testing.T) {
		p := parserFor(t, "a,b)")
		p.commaList(ArgList, token.RParen, p.bump)
		if want := indexOf(p.tokens, 3); p.pos != want {
			t.Errorf("stopped at %d, want %d — the closer is the parent's to consume", p.pos, want)
		}
	})
}

// One list, two closers: `CapList` is `RPAREN` under a UseClause and `RBRACE` under a
// CapabilityLit, which is why the closer cannot be a property of the list.
func TestCommaListTakesTheCallersCloser(t *testing.T) {
	expects(t, func(t *testing.T) {
		p := parserFor(t, "io,exec}")
		p.commaList(CapList, token.RBrace, p.bump)
		want := "open(CapList) token(0) token(1) token(2) close"
		if got := events(p.events); got != want {
			t.Errorf("events are %q, want %q", got, want)
		}
	})
}

// Truncated input must stop the loop rather than spin: every iteration past the first consumes a
// comma, so the list always advances (§6.4).
func TestCommaListStopsAtEndOfInput(t *testing.T) {
	expects(t, func(t *testing.T) {
		p := parserFor(t, "a,b,")
		p.commaList(ArgList, token.RParen, p.bump)
		if p.pos != len(p.tokens) {
			t.Errorf("stopped at %d of %d without reaching the end", p.pos, len(p.tokens))
		}
		if got, want := events(p.events), "open(ArgList) token(0) token(1) token(2) token(3) close"; got != want {
			t.Errorf("events are %q, want %q", got, want)
		}
	})
}
