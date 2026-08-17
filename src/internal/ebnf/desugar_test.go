// Tests for the metasyntax reader and its desugar to BNF.
//
// These come first and matter most. The desugar is the one place this package can disagree
// with grammar.md without saying so: if `A B?` rewrites wrongly, the tool tests a language the
// spec does not describe and every corpus result afterwards is quietly worthless. So each
// rewrite is checked against a hand-computed language — the strings it must accept and the
// strings it must reject — rather than against its own output.
package ebnf_test

import (
	"testing"

	"luna/internal/ebnf"
	"luna/oracle/token"
)

// toks builds an input from token kinds, using kinds whose spelling is irrelevant here.
func toks(kinds ...token.Kind) []ebnf.Token {
	out := make([]ebnf.Token, len(kinds))
	for i, k := range kinds {
		out[i] = ebnf.Token{Kind: k, Text: k.String()}
	}
	return out
}

func build(t *testing.T, src string) *ebnf.Grammar {
	t.Helper()
	prods, err := ebnf.Parse(src)
	if err != nil {
		t.Fatalf("parsing the grammar: %v", err)
	}
	return ebnf.New(prods)
}

// accepts asserts the exact language of a small grammar: every string in yes must derive and
// every string in no must not. Checking both directions is the point — a desugar that admits
// too much passes a yes-only test.
func accepts(t *testing.T, src string, yes, no [][]token.Kind) {
	t.Helper()
	g := build(t, src)
	for _, in := range yes {
		if r := g.Recognize(toks(in...)); !r.Accepted {
			t.Errorf("should accept %v: %s", in, r.Explain(toks(in...)))
		}
	}
	for _, in := range no {
		if r := g.Recognize(toks(in...)); r.Accepted {
			t.Errorf("should reject %v, but it derived", in)
		}
	}
}

func TestOptional(t *testing.T) {
	// `A B? C` is exactly {AC, ABC}.
	accepts(t, "File ::= LPAREN RPAREN? COMMA",
		[][]token.Kind{
			{token.LParen, token.Comma},
			{token.LParen, token.RParen, token.Comma},
		},
		[][]token.Kind{
			{token.LParen, token.RParen, token.RParen, token.Comma},
			{token.Comma},
		})
}

func TestOptionalsMultiply(t *testing.T) {
	// Two optionals in one sequence give four strings, not two: the quantifier multiplies
	// the alternatives it appears in. A desugar that expanded them independently would miss
	// the both-absent or both-present case.
	accepts(t, "File ::= LPAREN? COMMA RPAREN?",
		[][]token.Kind{
			{token.Comma},
			{token.LParen, token.Comma},
			{token.Comma, token.RParen},
			{token.LParen, token.Comma, token.RParen},
		},
		[][]token.Kind{
			{token.LParen, token.RParen},
			{token.LParen, token.Comma, token.Comma},
		})
}

func TestStar(t *testing.T) {
	// `A*` includes the empty string.
	accepts(t, "File ::= COMMA*",
		[][]token.Kind{
			{},
			{token.Comma},
			{token.Comma, token.Comma},
			{token.Comma, token.Comma, token.Comma},
		},
		[][]token.Kind{{token.LParen}})
}

func TestPlus(t *testing.T) {
	// `A+` excludes it — the difference `*` and `+` exist to express.
	accepts(t, "File ::= COMMA+",
		[][]token.Kind{
			{token.Comma},
			{token.Comma, token.Comma},
		},
		[][]token.Kind{{}})
}

func TestGroupWithAlternatives(t *testing.T) {
	// A parenthesized choice binds only its own alternatives, so the group must not leak the
	// `|` out to the enclosing sequence: this is `L (C|R) L`, never `L C` or `R L`.
	accepts(t, "File ::= LPAREN (COMMA | RPAREN) LPAREN",
		[][]token.Kind{
			{token.LParen, token.Comma, token.LParen},
			{token.LParen, token.RParen, token.LParen},
		},
		[][]token.Kind{
			{token.LParen, token.Comma},
			{token.RParen, token.LParen},
			{token.LParen, token.Comma, token.RParen, token.LParen},
		})
}

func TestGroupWithQuantifier(t *testing.T) {
	// `(A B)*` repeats the pair, not either half.
	accepts(t, "File ::= (LPAREN RPAREN)*",
		[][]token.Kind{
			{},
			{token.LParen, token.RParen},
			{token.LParen, token.RParen, token.LParen, token.RParen},
		},
		[][]token.Kind{
			{token.LParen},
			{token.LParen, token.RParen, token.LParen},
		})
}

// TestListShape is the shape grammar.md standardized on in R263, and the defect it replaced.
// The separator belongs to the list, not the item; putting it on the item inside a repetition
// makes it optional altogether.
func TestListShape(t *testing.T) {
	const good = `
File ::= LBRACKET Items? RBRACKET
Items ::= IDENT (COMMA IDENT)* COMMA?
`
	accepts(t, good,
		[][]token.Kind{
			{token.LBracket, token.RBracket},
			{token.LBracket, token.Ident, token.RBracket},
			{token.LBracket, token.Ident, token.Comma, token.Ident, token.RBracket},
			{token.LBracket, token.Ident, token.Comma, token.RBracket}, // trailing comma
		},
		[][]token.Kind{
			// The comma-less list: what the old shape admitted and this one must not.
			{token.LBracket, token.Ident, token.Ident, token.RBracket},
			{token.LBracket, token.Comma, token.RBracket},
		})

	// And the defect itself, pinned so the difference is visible rather than asserted.
	const bad = `
File ::= LBRACKET Item* RBRACKET
Item ::= IDENT COMMA?
`
	g := build(t, bad)
	in := toks(token.LBracket, token.Ident, token.Ident, token.RBracket)
	if r := g.Recognize(in); !r.Accepted {
		t.Error("the retired shape should still admit the comma-less list; " +
			"if it does not, this test no longer demonstrates what R263 fixed")
	}
}

func TestSpellingMatchedTerminal(t *testing.T) {
	// IDENT("from") matches the lexeme, not just the kind — the positional match that leaves
	// `from` unreserved (R223).
	g := build(t, `File ::= IDENT("from") IDENT`)

	ok := []ebnf.Token{{Kind: token.Ident, Text: "from"}, {Kind: token.Ident, Text: "m"}}
	if r := g.Recognize(ok); !r.Accepted {
		t.Errorf("should accept `from m`: %s", r.Explain(ok))
	}
	bad := []ebnf.Token{{Kind: token.Ident, Text: "unto"}, {Kind: token.Ident, Text: "m"}}
	if r := g.Recognize(bad); r.Accepted {
		t.Error("`unto m` derived, so the lexeme is not being matched")
	}
}

func TestAmbiguityIsDetected(t *testing.T) {
	// The property no other technique in the plan provides. This grammar derives one token
	// two ways, and the recognizer must say so rather than picking one.
	//
	// The nonterminals are UpperCamel deliberately: a name with no lowercase letter is a
	// *terminal* by symbolFor's rule, so `A ::= IDENT` would define a production for a token
	// kind and never be reached. grammar.md's names are all UpperCamel, so this only bites
	// in hand-written test grammars — here, once.
	g := build(t, `
File ::= Left | Right
Left ::= IDENT
Right ::= IDENT
`)
	r := g.Recognize(toks(token.Ident))
	if !r.Accepted {
		t.Fatal("should accept")
	}
	if !r.Ambiguous {
		t.Errorf("ambiguity went undetected")
	}
}

// TestAmbiguityBelowTheStartSymbol is the case that exposed the first implementation as
// unsound. Counting completed start items cannot see this: both derivations finish through
// the *same* File production, and Earley deduplicates items, so the count says one. Only
// recording why each item entered the chart — and walking back from the accepting item —
// makes an ambiguous node visible.
func TestAmbiguityBelowTheStartSymbol(t *testing.T) {
	g := build(t, `
File ::= LPAREN Inner RPAREN
Inner ::= Left | Right
Left ::= IDENT
Right ::= IDENT
`)
	in := toks(token.LParen, token.Ident, token.RParen)
	r := g.Recognize(in)
	if !r.Accepted {
		t.Fatalf("should accept: %s", r.Explain(in))
	}
	if !r.Ambiguous {
		t.Error("ambiguity one level below the start symbol went undetected")
	}
}

// TestUnreachableAmbiguityIsNotReported is the other side: Earley explores derivations that
// lead nowhere, and an item reached two ways off the successful path is not an ambiguity of
// the input. Reporting it would make the check cry wolf on ordinary grammars.
func TestUnreachableAmbiguityIsNotReported(t *testing.T) {
	// `Dead` is ambiguous, and no accepted parse goes through it.
	g := build(t, `
File ::= IDENT COMMA
Dead ::= Ambig
Ambig ::= IDENT | IDENT
`)
	r := g.Recognize(toks(token.Ident, token.Comma))
	if !r.Accepted {
		t.Fatal("should accept")
	}
	if r.Ambiguous {
		t.Error("reported ambiguity from a subtree no parse uses")
	}
}

func TestUnambiguousReportsOneParse(t *testing.T) {
	// The other direction: a grammar with one derivation must not report ambiguity, or the
	// check above would be satisfied by a recognizer that always says "many".
	g := build(t, `File ::= IDENT COMMA IDENT`)
	r := g.Recognize(toks(token.Ident, token.Comma, token.Ident))
	if !r.Accepted || r.Ambiguous {
		t.Errorf("accepted=%v ambiguous=%v, want accepted and unambiguous", r.Accepted, r.Ambiguous)
	}
}

func TestLeftRecursion(t *testing.T) {
	// Earley is chosen partly because it takes left recursion unchanged; the desugar emits
	// left-recursive pairs for every `*`, so this is not a hypothetical.
	accepts(t, `
File ::= List
List ::= IDENT | List COMMA IDENT
`,
		[][]token.Kind{
			{token.Ident},
			{token.Ident, token.Comma, token.Ident},
			{token.Ident, token.Comma, token.Ident, token.Comma, token.Ident},
		},
		[][]token.Kind{
			{},
			{token.Ident, token.Comma},
		})
}

func TestNullableNonterminal(t *testing.T) {
	// A nonterminal that derives ε must let the item waiting on it advance in the same set.
	// Miss this and `A B? C` silently stops parsing — the classic Earley nullable bug.
	accepts(t, `
File ::= LPAREN Maybe RPAREN
Maybe ::= IDENT | Nothing
Nothing ::=
`,
		[][]token.Kind{
			{token.LParen, token.RParen},
			{token.LParen, token.Ident, token.RParen},
		},
		[][]token.Kind{{token.LParen, token.Ident, token.Ident, token.RParen}})
}

func TestContinuationLines(t *testing.T) {
	// grammar.md writes alternatives on their own `|` lines; folding them into the rule is
	// the reader's job, and getting it wrong would silently drop alternatives.
	accepts(t, `
File ::= LPAREN
       | RPAREN
       | COMMA
`,
		[][]token.Kind{{token.LParen}, {token.RParen}, {token.Comma}},
		[][]token.Kind{{}, {token.LParen, token.RParen}})
}

func TestSyntheticNamesCannotCollide(t *testing.T) {
	// Synthetics are named with an interpunct, which no spec name may contain, so a
	// synthetic can never shadow or be mistaken for a grammar.md nonterminal.
	prods, err := ebnf.Parse("File ::= (LPAREN | RPAREN)* COMMA")
	if err != nil {
		t.Fatal(err)
	}
	g := ebnf.New(prods)
	for _, n := range g.Nonterminals() {
		if n != "File" && !containsRune(n, '·') {
			t.Errorf("generated nonterminal %q carries no synthetic marker", n)
		}
	}
}

func containsRune(s string, r rune) bool {
	for _, c := range s {
		if c == r {
			return true
		}
	}
	return false
}

// TestGuardForbidsALeadingToken is R270's notation: `!TERMINAL X` derives what X derives, minus
// the strings that begin with that terminal. This is the shape R268 needs — a `{` opens a block
// wherever a block may appear, so the expression arm may not start with one.
func TestGuardForbidsALeadingToken(t *testing.T) {
	// Body ::= Block | !LBRACE Expr, over a toy expression that may or may not start with `{`.
	src := `File ::= Body
Body ::= Block | !LBRACE Expr
Block ::= LBRACE RBRACE
Expr ::= IDENT | LBRACE IDENT RBRACE`
	accepts(t, src,
		[][]token.Kind{
			{token.LBrace, token.RBrace}, // the block
			{token.Ident},                // the expression, not brace-initial
		},
		[][]token.Kind{
			{token.LBrace, token.Ident, token.RBrace}, // the guard's whole purpose
		})
}

// TestGuardIsZeroWidth: the guard consumes nothing, so what follows it still has to be there.
// A guard that ate a token would make the first case below derive.
func TestGuardIsZeroWidth(t *testing.T) {
	accepts(t, "File ::= !LBRACE Expr\nExpr ::= IDENT IDENT",
		[][]token.Kind{{token.Ident, token.Ident}},
		[][]token.Kind{{token.Ident}, {}})
}

// TestGuardMatchesTheLexemeToo: `!IDENT("type")` forbids one spelling and not the kind, which is
// what R269 needs — an annotation may be any type but the bare identifier `type`.
func TestGuardMatchesTheLexemeToo(t *testing.T) {
	g := build(t, `File ::= COLON !IDENT("type") IDENT ASSIGN`)

	ok := []ebnf.Token{
		{Kind: token.Colon, Text: ":"}, {Kind: token.Ident, Text: "int"},
		{Kind: token.Assign, Text: "="},
	}
	if r := g.Recognize(ok); !r.Accepted {
		t.Errorf("should accept `: int =`: %s", r.Explain(ok))
	}
	bad := []ebnf.Token{
		{Kind: token.Colon, Text: ":"}, {Kind: token.Ident, Text: "type"},
		{Kind: token.Assign, Text: "="},
	}
	if r := g.Recognize(bad); r.Accepted {
		t.Error("`: type =` derived, so the guard is matching the kind and not the lexeme")
	}
}

// TestGuardDisambiguates is the property both rulings are for: two productions that overlap
// become disjoint, so an input that derived twice derives once. Without the guard this grammar
// is ambiguous on `IDENT`, which is exactly the state grammar.md's BindingDecl was in.
func TestGuardDisambiguates(t *testing.T) {
	ambiguous := build(t, "File ::= Left | Right\nLeft ::= IDENT\nRight ::= IDENT")
	if r := ambiguous.Recognize(toks(token.Ident)); !r.Ambiguous {
		t.Fatal("the control grammar is not ambiguous; the test proves nothing")
	}
	guarded := build(t, "File ::= Left | !IDENT Right\nLeft ::= IDENT\nRight ::= IDENT")
	r := guarded.Recognize(toks(token.Ident))
	if !r.Accepted || r.Ambiguous {
		t.Errorf("accepted=%v ambiguous=%v; the guard should leave exactly one derivation",
			r.Accepted, r.Ambiguous)
	}
}

// TestGuardLeavesNoNodeInATree: a guard is an assertion, not a symbol, so a derivation must not
// carry one — the parse goldens would otherwise print a child for a token nobody consumed.
func TestGuardLeavesNoNodeInATree(t *testing.T) {
	g := build(t, "File ::= !LBRACE IDENT SEMICOLON")
	root, err := g.Derive(toks(token.Ident, token.Semicolon))
	if err != nil {
		t.Fatalf("deriving: %v", err)
	}
	if len(root.Children) != 2 {
		t.Fatalf("File has %d children, want 2: the guard left a node behind", len(root.Children))
	}
	for i, want := range []string{"IDENT", "SEMICOLON"} {
		if root.Children[i].Name != want {
			t.Errorf("child %d is %s, want %s", i, root.Children[i].Name, want)
		}
	}
}

// TestGuardIsRejectedWhereItCannotMean: a guard names a terminal and takes no quantifier, and
// both are refused at parse time rather than given a meaning nobody asked for.
func TestGuardIsRejectedWhereItCannotMean(t *testing.T) {
	for _, src := range []string{
		"File ::= !Expr IDENT\nExpr ::= IDENT", // a nonterminal: not a regular restriction
		"File ::= !LBRACE? IDENT",              // a quantifier on an assertion
		"File ::= !LBRACE !LBRACKET IDENT",     // two assertions at one position
		"File ::= !",                           // nothing to negate
	} {
		if _, err := ebnf.Parse(src); err == nil {
			t.Errorf("%q parsed, and it has no meaning", src)
		}
	}
}
