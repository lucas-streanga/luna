package parser

import (
	"fmt"
	"strings"
)

// Rendering a tree into a golden's tree section (`testdata/golden.md` §1).
//
// Skipping trivia is the whole transform, everything else having been decided upstream: tiers
// that did not fire never reached the tree, empty nodes were deleted (§6.1), and spans are read
// off the node. File's span needs no special case for the same reason: §2.1 confines trivia at
// an edge to File alone, so it comes out 0..len(source) on its own.

// RenderGolden prints a tree in the golden format. A nil tree renders as nothing: the empty file,
// which no golden can hold, since a case's source section is never empty.
func RenderGolden(tree *Tree) string {
	var b strings.Builder
	if tree != nil {
		writeGolden(&b, tree.Root(), 0)
	}
	return b.String()
}

func writeGolden(b *strings.Builder, n Node, depth int) {
	offset, end := n.Span()
	kids := n.Children()
	b.WriteString(strings.Repeat("  ", depth))
	if len(kids) == 0 {
		// A synthesised leaf prints "": width is the whole distinction (§6.1).
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
