// The scaffold.s own check, and the one file here meant to shrink: every name Phase 2 will
// implement exists, and every one of them panics.
//
// A stub that returned a zero value would not report "not written yet" — it would report a tree
// disagreeing with a golden, and the diff would be read against grammar.md §0 for a defect that is
// not there. That is the fail-open check.sh guards against when it treats a missing tool as a
// failure rather than a skip.
//
// It is also what keeps the scaffold compiling under `unused`, where a `//nolint` on each stub
// would suppress a check where this asserts one. An entry leaves the table when its function
// acquires a body, and the file goes with the last of them.
package parser

import (
	"strings"
	"testing"

	"luna/oracle/source"
	"luna/oracle/token"
)

func TestScaffoldIsUnimplemented(t *testing.T) {
	p := &parser{}
	stubs := []struct {
		name string
		call func()
	}{
		// parser.go — the spine's entry and its state
		{"parse", func() { parse(nil, nil) }},
		{"newParser", func() { newParser(nil, nil) }},

		// cursor.go — the trivia-filtered view
		{"atEnd", func() { p.atEnd() }},
		{"at", func() { p.at(token.Semicolon) }},
		{"nth", func() { p.nth(1) }},
		{"lexeme", func() { p.lexeme(0) }},
		{"atWord", func() { p.atWord("from") }},
		{"bump", func() { p.bump() }},
		{"eat", func() { p.eat(token.Comma) }},
		{"eatWord", func() { p.eatWord("get") }},
		{"expect", func() { p.expect(token.Semicolon) }},
		{"expectWord", func() { p.expectWord("from") }},
		{"errorToken", func() { p.errorToken() }},

		// marker.go — where a node begins
		{"open", func() { p.open(File) }},
		{"mark", func() { p.mark() }},
		{"precede", func() { p.precede(marker(0), Additive) }},
		{"complete", func() { p.complete(marker(0)) }},

		// decl.go — §0.1 file and declarations
		{"file", func() { p.file() }},
		{"prelude", func() { p.prelude() }},
		{"assignedImportAhead", func() { p.assignedImportAhead() }},
		{"topLevelItem", func() { p.topLevelItem() }},
		{"declaration", func() { p.declaration() }},
		{"attribute", func() { p.attribute() }},

		// stmt.go — §0.2 statements
		{"block", func() { p.block() }},
		{"statement", func() { p.statement() }},
		{"simpleStmt", func() { p.simpleStmt() }},
		{"bindTarget", func() { p.bindTarget() }},

		// expr.go — §0.3 the tier spine
		{"expr", func() { p.expr() }},
		{"assignment", func() { p.assignment() }},
		{"assignTargetAhead", func() { p.assignTargetAhead() }},
		{"assignTarget", func() { p.assignTarget() }},
		{"wordPrefix", func() { p.wordPrefix() }},
		{"conditional", func() { p.conditional() }},
		{"coalesce", func() { p.coalesce() }},
		{"disjunction", func() { p.disjunction() }},
		{"conjunction", func() { p.conjunction() }},
		{"equality", func() { p.equality() }},
		{"comparison", func() { p.comparison() }},
		{"rangeExpr", func() { p.rangeExpr() }},
		{"additive", func() { p.additive() }},
		{"multiplicative", func() { p.multiplicative() }},
		{"prefixExpr", func() { p.prefixExpr() }},
		{"applyExpr", func() { p.applyExpr() }},
		{"postfixExpr", func() { p.postfixExpr() }},
		{"postfix", func() { p.postfix() }},
		{"subscript", func() { p.subscript() }},
		{"argList", func() { p.argList() }},
		{"arg", func() { p.arg() }},

		// primary.go — §0.4 primary expressions
		{"primary", func() { p.primary() }},
		{"tableLit", func() { p.tableLit() }},
		{"variantLit", func() { p.variantLit() }},
		{"fnLit", func() { p.fnLit() }},
		{"genLit", func() { p.genLit() }},
		{"matchExpr", func() { p.matchExpr() }},
		{"tryCatchExpr", func() { p.tryCatchExpr() }},

		// decllit.go — §0.5 declaration literals
		{"declLit", func() { p.declLit() }},
		{"enumLit", func() { p.enumLit() }},
		{"useClause", func() { p.useClause() }},
		{"protoInit", func() { p.protoInit() }},

		// type.go — §0.6 types
		{"typ", func() { p.typ() }},

		// pattern.go — §0.7 patterns
		{"pattern", func() { p.pattern() }},
		{"destructurePattern", func() { p.destructurePattern() }},

		// literal.go — §0.8 literals
		{"literal", func() { p.literal() }},
		{"stringLit", func() { p.stringLit() }},

		// keyword.go — §0.9 the keyword class
		{"isPathSegment", func() { isPathSegment(token.Ident) }},
	}

	for _, s := range stubs {
		t.Run(s.name, func(t *testing.T) {
			got, ok := recovered(s.call)
			if !ok {
				t.Fatalf("%s returned instead of panicking: an unimplemented stub that answers is "+
					"a wrong answer, reported as a golden diff rather than as missing work", s.name)
			}
			// The name is checked, not merely the panic: a stub that panics with another stub's
			// message is a copy-paste that would send a reader to the wrong function.
			if want := "parser: " + s.name + " is unimplemented"; got != want {
				t.Errorf("panicked with %q, want %q", got, want)
			}
		})
	}
}

// recovered runs call and reports the panic value as a string, or ok=false if it returned.
func recovered(call func()) (msg string, ok bool) {
	defer func() {
		if v := recover(); v != nil {
			msg, ok = strings.TrimSpace(toString(v)), true
		}
	}()
	call()
	return "", false
}

func toString(v any) string {
	if s, is := v.(string); is {
		return s
	}
	if e, is := v.(error); is {
		return e.Error()
	}
	return ""
}

// TestScaffoldParserState is a shape check, there being no behaviour yet: the struct is this
// phase.s reviewable artifact, and an unread field is a field nobody noticed was wrong.
func TestScaffoldParserState(t *testing.T) {
	f, err := source.New("scaffold.luna", "x;")
	if err != nil {
		t.Fatalf("source.New: %v", err)
	}
	toks := []token.Token{{Kind: token.Ident, Offset: 0, Len: 1}, {Kind: token.Semicolon, Offset: 1, Len: 1}}

	p := &parser{f: f, tokens: toks}
	if p.f != f {
		t.Errorf("the parser holds a different file; it is carried for the spelling-matched terminals")
	}
	if len(p.tokens) != len(toks) {
		t.Errorf("the parser holds %d tokens, want the full stream's %d", len(p.tokens), len(toks))
	}
	if p.pos != 0 {
		t.Errorf("the cursor starts at %d, want 0", p.pos)
	}
	if len(p.events) != 0 || len(p.stack) != 0 || len(p.diags) != 0 {
		t.Errorf("a fresh parse holds %d events, %d open nodes and %d diagnostics; all three are "+
			"accumulated, none is seeded", len(p.events), len(p.stack), len(p.diags))
	}
}
