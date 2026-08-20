package parser

import (
	"testing"

	"luna/oracle/token"
)

// The nonterminal functions' specs, as an event table.
//
// Events rather than trees, because that is the seam §4.2 kept these tests for: a tree cannot
// show *where* a tier opened its node, only that it did, and mark/precede is the subtle part.
// The goldens check the tree; these check the stream that produces it.
//
// A token event carries a **full-stream** index, so a source with whitespace in it names indices
// with gaps. Most sources here have none, on purpose.

var nonterminalSpecs = []struct {
	fn   string
	call func(*parser)
	src  string
	want string
}{
	// §0.1: file and declarations
	{"file", func(p *parser) { p.file() }, "x;",
		"open(File) open(Prelude) close open(Statement) open(SimpleStmt) token(0) close token(1) close close"},
	// Even here the Prelude pair is emitted: the parser always opens it and splice elides the
	// empty node (§6.1). File survives because the file's trailing trivia is content.
	{"file", func(p *parser) { p.file() }, "// c\n",
		"open(File) open(Prelude) close close"},
	{"prelude", func(p *parser) { p.prelude() }, "import a;",
		"open(Prelude) open(PreludeItem) token(0) open(ImportSpec) open(ModulePath) token(2) close close token(3) close close"},
	{"topLevelItem", func(p *parser) { p.topLevelItem() }, "x;",
		"open(Statement) open(SimpleStmt) token(0) close token(1) close"},
	{"declaration", func(p *parser) { p.declaration() }, "let x=1;",
		"open(Declaration) open(BindingDecl) token(0) open(Binder) token(2) close token(3) open(Initializer) token(4) close token(5) close close"},
	{"attribute", func(p *parser) { p.attribute() }, "#[a]",
		"open(Attribute) token(0) token(1) token(2) close"},

	// §0.2: statements
	{"block", func(p *parser) { p.block() }, "{}",
		"open(Block) token(0) token(1) close"},
	{"statement", func(p *parser) { p.statement() }, "x;",
		"open(Statement) open(SimpleStmt) token(0) close token(1) close"},
	{"simpleStmt", func(p *parser) { p.simpleStmt() }, "x",
		"open(SimpleStmt) token(0) close"},
	{"bindTarget", func(p *parser) { p.bindTarget() }, "x",
		"token(0)"},

	// §0.3: the tier spine. Each tier fires, and only the tier that fired opens.
	{"expr", func(p *parser) { p.expr() }, "a",
		"token(0)"},
	{"assignment", func(p *parser) { p.assignment() }, "a=b",
		"open(Assignment) open(AssignTarget) token(0) close token(1) token(2) close"},
	{"assignTarget", func(p *parser) { p.assignTarget() }, "a",
		"open(AssignTarget) token(0) close"},
	{"wordPrefix", func(p *parser) { p.wordPrefix() }, "copy a",
		"open(WordPrefix) token(0) token(2) close"},
	{"conditional", func(p *parser) { p.conditional() }, "a?b:c",
		"open(Conditional) token(0) token(1) token(2) token(3) token(4) close"},
	{"coalesce", func(p *parser) { p.coalesce() }, "a??b",
		"open(Coalesce) token(0) token(1) token(2) close"},
	{"disjunction", func(p *parser) { p.disjunction() }, "a||b",
		"open(Disjunction) token(0) token(1) token(2) close"},
	{"conjunction", func(p *parser) { p.conjunction() }, "a&&b",
		"open(Conjunction) token(0) token(1) token(2) close"},
	{"equality", func(p *parser) { p.equality() }, "a==b",
		"open(Equality) token(0) token(1) token(2) close"},
	{"comparison", func(p *parser) { p.comparison() }, "a<b",
		"open(Comparison) token(0) token(1) token(2) close"},
	// `is` reaches the type grammar, and `Type` survives where its tiers do not (§5).
	{"comparison", func(p *parser) { p.comparison() }, "a is b",
		"open(Comparison) token(0) token(2) open(Type) token(4) close close"},
	{"rangeExpr", func(p *parser) { p.rangeExpr() }, "a..b",
		"open(RangeExpr) token(0) token(1) token(2) close"},
	{"additive", func(p *parser) { p.additive() }, "a+b",
		"open(Additive) token(0) token(1) token(2) close"},
	// A left-associative run is one node over the whole chain, not one per operator.
	{"additive", func(p *parser) { p.additive() }, "a+b+c",
		"open(Additive) token(0) token(1) token(2) token(3) token(4) close"},
	{"multiplicative", func(p *parser) { p.multiplicative() }, "a*b",
		"open(Multiplicative) token(0) token(1) token(2) close"},
	{"prefixExpr", func(p *parser) { p.prefixExpr() }, "-a",
		"open(PrefixExpr) token(0) token(1) close"},
	{"applyExpr", func(p *parser) { p.applyExpr() }, "a apply P",
		"open(ApplyExpr) token(0) token(2) open(ProtoInit) token(4) close close"},
	{"postfixExpr", func(p *parser) { p.postfixExpr() }, "a.b",
		"open(PostfixExpr) token(0) open(Postfix) token(1) token(2) close close"},
	{"postfix", func(p *parser) { p.postfix() }, ".b",
		"open(Postfix) token(0) token(1) close"},
	{"subscript", func(p *parser) { p.subscript() }, "[]",
		"open(Subscript) token(0) token(1) close"},
	{"argList", func(p *parser) { p.argList() }, "a,b",
		"open(ArgList) open(Arg) token(0) close token(1) open(Arg) token(2) close close"},
	{"arg", func(p *parser) { p.arg() }, "a",
		"open(Arg) token(0) close"},

	// §0.4: primaries. Primary is a tier: it survives only for the parenthesized form.
	{"primary", func(p *parser) { p.primary() }, "a",
		"token(0)"},
	{"primary", func(p *parser) { p.primary() }, "(a)",
		"open(Primary) token(0) token(1) token(2) close"},
	{"tableLit", func(p *parser) { p.tableLit() }, "[]",
		"open(TableLit) token(0) token(1) close"},
	{"variantLit", func(p *parser) { p.variantLit() }, ".{a}",
		"open(VariantLit) token(0) token(1) open(VariantName) token(2) close token(3) close"},
	{"fnLit", func(p *parser) { p.fnLit() }, "fn()=>1",
		"open(FnLit) token(0) token(1) token(2) token(3) token(4) close"},
	{"genLit", func(p *parser) { p.genLit() }, "gen{}",
		"open(GenLit) token(0) open(Block) token(1) token(2) close close"},
	{"matchExpr", func(p *parser) { p.matchExpr() }, "match{}",
		"open(MatchExpr) token(0) token(1) token(2) close"},
	{"tryCatchExpr", func(p *parser) { p.tryCatchExpr() }, "try{}catch(e){}",
		"open(TryCatchExpr) token(0) open(Block) token(1) token(2) close open(CatchClause) token(3) token(4) open(CatchBinder) token(5) close token(6) open(Block) token(7) token(8) close close close"},

	// §0.5: declaration literals
	{"declLit", func(p *parser) { p.declLit() }, "enum{}",
		"open(EnumLit) token(0) token(1) token(2) close"},
	{"enumLit", func(p *parser) { p.enumLit() }, "enum{}",
		"open(EnumLit) token(0) token(1) token(2) close"},
	{"useClause", func(p *parser) { p.useClause() }, "use(io)",
		"open(UseClause) token(0) token(1) open(CapList) token(2) close token(3) close"},
	{"protoInit", func(p *parser) { p.protoInit() }, "P",
		"open(ProtoInit) token(0) close"},

	// §0.6: types. `Type` is the elision rule's one override.
	{"typ", func(p *parser) { p.typ() }, "int",
		"open(Type) token(0) close"},
	{"typ", func(p *parser) { p.typ() }, "a|b",
		"open(Type) open(UnionType) token(0) token(1) token(2) close close"},

	// §0.7: patterns
	{"pattern", func(p *parser) { p.pattern() }, "a",
		"token(0)"},
	{"pattern", func(p *parser) { p.pattern() }, "a|b",
		"open(AltPattern) token(0) token(1) token(2) close"},
	{"destructurePattern", func(p *parser) { p.destructurePattern() }, "[a]",
		"open(DestructurePattern) token(0) open(DestrEntries) open(DestrEntry) token(1) close close token(2) close"},

	// §0.8: literals
	{"literal", func(p *parser) { p.literal() }, "1",
		"token(0)"},
	{"stringLit", func(p *parser) { p.stringLit() }, "'a'",
		"open(StringLit) token(0) close"},
}

func TestNonterminalEvents(t *testing.T) {
	for _, s := range nonterminalSpecs {
		t.Run(s.fn+"/"+s.src, func(t *testing.T) {
			expects(t, func(t *testing.T) {
				p := parserFor(t, s.src)
				s.call(p)
				if got := events(p.events); got != s.want {
					t.Errorf("events\n got %s\nwant %s", got, s.want)
				}
				if p.pos != len(p.tokens) {
					t.Errorf("stopped at token %d of %d: a nonterminal consumes the whole of what "+
						"it matched", p.pos, len(p.tokens))
				}
			})
		})
	}
}

// The two predicates decide before consuming anything, which is what keeps §7.3's "no
// backtracking anywhere" true (§4.7).
func TestPredicatesDecideWithoutConsuming(t *testing.T) {
	for _, c := range []struct {
		fn   string
		call func(*parser) bool
		src  string
		want bool
	}{
		{"assignedImportAhead", (*parser).assignedImportAhead, "const fs = import a;", true},
		{"assignedImportAhead", (*parser).assignedImportAhead, "export const fs = import a;", true},
		{"assignedImportAhead", (*parser).assignedImportAhead, "const n = 1;", false},
		{"assignedImportAhead", (*parser).assignedImportAhead, "import a;", false},
		{"assignedImportAhead", (*parser).assignedImportAhead, "const fs = imported;", false},

		{"assignTargetAhead", (*parser).assignTargetAhead, "a=b", true},
		{"assignTargetAhead", (*parser).assignTargetAhead, "a.b[c](d).e = 1", true},
		{"assignTargetAhead", (*parser).assignTargetAhead, "[a,b] = t", true},
		{"assignTargetAhead", (*parser).assignTargetAhead, "_ = 1", true},
		{"assignTargetAhead", (*parser).assignTargetAhead, "a.b[c](d).e + 1", false},
		{"assignTargetAhead", (*parser).assignTargetAhead, "a+b=c", false},
		{"assignTargetAhead", (*parser).assignTargetAhead, "&x = 1", false},
	} {
		t.Run(c.fn+"/"+c.src, func(t *testing.T) {
			expects(t, func(t *testing.T) {
				p := parserFor(t, c.src)
				before := p.pos
				if got := c.call(p); got != c.want {
					t.Errorf("%s on %q is %v, want %v", c.fn, c.src, got, c.want)
				}
				if p.pos != before || len(p.events) != 0 {
					t.Errorf("%s consumed: pos %d → %d, %d events", c.fn, before, p.pos, len(p.events))
				}
			})
		})
	}
}

func TestIsPathSegmentAdmitsEveryKeyword(t *testing.T) {
	expects(t, func(t *testing.T) {
		for _, k := range []token.Kind{token.Ident, token.Wildcard, token.KwLet, token.KwModuleof} {
			if !isPathSegment(k) {
				t.Errorf("isPathSegment(%s) is false; a path segment is an IDENT, a WILDCARD, or "+
					"any keyword (R252)", k)
			}
		}
		for _, k := range []token.Kind{token.Semicolon, token.Plus, token.IntDec} {
			if isPathSegment(k) {
				t.Errorf("isPathSegment(%s) is true", k)
			}
		}
	})
}
