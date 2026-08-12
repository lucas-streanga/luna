// The spec corpus gate (lexer-testing-plan §9).
//
// Every ```luna block in the spec must lex with no diagnostic at all. That is a stronger
// claim than the fuzz seeds make — they assert only the structural properties, because a
// mutated seed is expected to be broken — and it is the claim worth attaching a name to,
// so a failure says which block in which file rather than "seed#312".
//
// What this gate does *not* catch is worth stating, because the opposite was once assumed
// here. A lexing gate is permissive: retired spellings lex perfectly well. `pub` is an
// IDENT, `caps.io` is IDENT DOT IDENT, `use (&io)` is KW_USE LPAREN AMP IDENT RPAREN. The
// one exception is R237, which made `/…/` lexically invalid rather than merely retired —
// and that exception is exactly how the site R237's own sweep missed was found.
//
// So this is a regression guard for the implementation, plus a discovery tool for the one
// class of spec drift that changes what is lexable.
package lexer_test

import (
	"fmt"
	"strings"
	"testing"

	"luna/internal/spec"
	"luna/oracle/lexer"
	"luna/oracle/source"
)

func TestSpecCorpus(t *testing.T) {
	blocks, err := spec.LunaBlocks()
	if err != nil {
		t.Fatalf("reading the spec corpus: %v", err)
	}
	// A floor rather than a non-empty check: a reader that silently found a handful would
	// otherwise pass while covering nothing, which is the failure this project keeps
	// meeting. The count is ~436 and grows; the floor only has to be high enough that a
	// broken walk cannot clear it.
	if len(blocks) < 400 {
		t.Fatalf("found %d luna blocks, expected ~436 — the corpus walk is broken", len(blocks))
	}

	for _, b := range blocks {
		// Slashes nest the subtests, so `-run 'TestSpecCorpus/specs/types'` selects one
		// directory and the -v listing reads as the tree it is.
		t.Run(fmt.Sprintf("%s:%d", b.Path, b.Line), func(t *testing.T) {
			f, err := source.New(b.Path, b.Source)
			if err != nil {
				t.Fatalf("not valid source: %v", err)
			}

			toks, errs := lexer.Lex(f)
			for _, d := range errs.Sorted() {
				p := f.Position(d.Primary.Offset)
				t.Errorf("%s at %d:%d — %s\n%s", d.Code, p.Line, p.Column, d.Description,
					excerpt(f, p.Line))
			}

			// Tiling, checked here too: it costs one pass and it means a failure names the
			// block rather than arriving as an anonymous fuzz counterexample.
			next := 0
			for _, tok := range toks {
				if tok.Offset != next {
					t.Fatalf("%s starts at %d, previous ended at %d", tok.Kind, tok.Offset, next)
				}
				next = tok.End()
			}
			if next != len(b.Source) {
				t.Fatalf("spans end at %d, block is %d bytes", next, len(b.Source))
			}
		})
	}
}

// excerpt is the offending line, indented, so a failure reads without opening the spec.
func excerpt(f *source.File, line int) string {
	lines := strings.Split(f.Text(), "\n")
	if line < 1 || line > len(lines) {
		return ""
	}
	return "    " + lines[line-1]
}
