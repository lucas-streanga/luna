package parser

import (
	"fmt"
	"strings"
)

// The event stream (§4). Parsing into a flat sequence keeps the parser reading like grammar.md
// §0, makes recovery a stream operation — "close everything down to a synchronisation point" is
// something you can do to a stack of open events and not to a half-built tree — and leaves the
// representation decoupled, which is most of what made §3's rejection affordable.
//
// It stays internal (§4.1). No consumer wants it, and exporting it would freeze what §11 leaves
// open: the quiet period's N and the mismatched-bracket heuristic both change which events are
// emitted, so a supported stream would make tuning recovery a breaking change.

// eventKind has deliberately no member for trivia: splice emits ordinary token events at trivia
// indices, and the builder makes them leaves like anything else (§2.2).
//
// evMissing is the exception to "open, token, close", and it is §7.2 layer 1's: at an
// expect-site the parser knows exactly one terminal is required, so it emits a zero-width leaf
// of that kind (§6.1). It cannot be a token event, because a synthesised leaf has no entry in
// the lexer's stream to index.
type eventKind uint8

const (
	evOpen eventKind = iota
	evToken
	evMissing
	evClose
)

// event is one step of the stream. A token event indexes the **full** token slice rather than
// the filtered view the parser walks: each consumed token already knows its real index, so
// splice needs no mapping table (§2.2).
//
// A missing event carries only its kind. Its position is the builder's cursor — the end of the
// last leaf emitted — which is the one offset that cannot break the tiling: trivia pending at
// that moment has not been flushed yet (splice does not flush at a missing, exactly as it does
// not at a close), so the zero-width leaf sits before those bytes rather than after them.
type event struct {
	kind eventKind
	node Kind // evOpen and evMissing: the node being opened, or the terminal that was expected
	tok  int  // evToken only: an index into the full token stream
}

// eventStream is one parse's whole output, before splicing and before building.
type eventStream []event

// String is the debug dump §4.2 permits — test-failure output and a flag, with no
// compatibility promise. Unindented, because flatness is the property the stream was chosen
// for.
func (s eventStream) String() string {
	var b strings.Builder
	for _, e := range s {
		b.WriteString(e.String())
		b.WriteByte('\n')
	}
	return b.String()
}

func (e event) String() string {
	switch e.kind {
	case evOpen:
		return fmt.Sprintf("open(%s)", e.node)
	case evToken:
		return fmt.Sprintf("token(%d)", e.tok)
	case evMissing:
		return fmt.Sprintf("missing(%s)", e.node)
	case evClose:
		return "close"
	default:
		return fmt.Sprintf("event(%d)", e.kind)
	}
}
