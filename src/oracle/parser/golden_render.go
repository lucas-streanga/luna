package parser

import (
	"fmt"
	"strings"
)

// Rendering a tree into a golden's tree section (`testdata/golden.md` §1).
//
// It walks the CST and does one thing to it: **skips trivia**. That is the whole transform,
// and it is the same one §8's AST view makes — an indented tree stays readable against
// grammar.md §0 and a tree interleaved with whitespace nodes does not, while the placement no
// golden can display is asserted directly instead (§2.3).
//
// Everything else a dump would normally have to do was decided upstream. Elision happened when
// the events were emitted, so a tier that did not fire never reached the tree; empty nodes were
// deleted by the builder (§6.1); and spans are the builder's arithmetic, read off the node
// rather than recomputed here. That is why File's span needs no special case any more: trivia
// is never the first or last child of anything else, so File is the only node whose extent
// includes the file's leading and trailing trivia, and it comes out 0..len(source) on its own.

// RenderGolden prints a tree in the golden format. A nil tree renders as nothing — the empty
// file, which no golden can hold anyway, since a case's source section is never empty.
func RenderGolden(t *Tree) string {
	var b strings.Builder
	if t != nil {
		writeGolden(&b, t.Root(), 0)
	}
	return b.String()
}

func writeGolden(b *strings.Builder, n Node, depth int) {
	offset, end := n.Span()
	kids := n.Children()
	b.WriteString(strings.Repeat("  ", depth))
	if len(kids) == 0 {
		// A leaf prints its lexeme, which for a synthesised one is "" — width being the whole
		// distinction between a missing token and a real one (§6.1).
		fmt.Fprintf(b, "%s %d..%d %q\n", n.Kind(), offset, end, n.Text())
		return
	}
	fmt.Fprintf(b, "%s %d..%d\n", n.Kind(), offset, end)
	for _, kid := range kids {
		if isTrivia(kid.Kind()) {
			continue
		}
		writeGolden(b, kid, depth+1)
	}
}
