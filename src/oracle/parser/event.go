package parser

import (
	"fmt"
	"strings"
)

// The event stream (§4), internal by §4.1: exporting it would make tuning recovery a breaking
// change, and no consumer wants it.

// evMissing is §7.2 layer 1's synthesised leaf. It cannot be a token event because there is no
// token to index, and it carries no position because the builder's cursor is the one offset that
// cannot break the tiling. Where no open intervenes that puts it before trivia pending at the same
// moment; where one does, releasing the open flushes first and the leaf follows those bytes.
type eventKind uint8

const (
	evOpen eventKind = iota
	evToken
	evMissing
	evClose
)

type event struct {
	kind eventKind
	node Kind // evOpen and evMissing
	// evToken only, and an index into the **full** stream: the filtered view the parser walks is
	// index-parallel with it, so splice needs no mapping table (§2.2).
	tok int
}

type eventStream []event

// String is the debug dump §4.2 permits: test output and a flag, no compatibility promise.
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
