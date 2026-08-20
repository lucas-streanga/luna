// The grammar pin and the parse gate.
//
// These read specs/build/grammar.md and run it. TestGrammarReaderIsArmed guards the guards,
// exactly as internal/spec's does: every check below compares against what the reader
// extracted, so a reader that extracted nothing would make them all pass while verifying
// nothing.
package ebnf_test

import (
	"fmt"
	"strings"
	"testing"

	"luna/internal/ebnf"
	"luna/internal/spec"
	"luna/oracle/lexer"
	"luna/oracle/source"
	"luna/oracle/token"
)

// §10's claims, which this file is the enforcement of.
const (
	wantBlocks        = 9   // §0's production groups
	wantNonterminals  = 129 // §10's total
	wantKeywordAlts   = 50  // lexer §10's keyword count, reached through Keyword
	wantStartUnreach  = "File"
	corpusFloorBlocks = 400 // a walk that finds fewer has broken, not shrunk
)

func load(t *testing.T) *ebnf.Grammar {
	t.Helper()
	g, err := ebnf.Load()
	if err != nil {
		t.Fatalf("loading the grammar: %v", err)
	}
	return g
}

func TestGrammarReaderIsArmed(t *testing.T) {
	n, err := ebnf.BlockCount()
	if err != nil {
		t.Fatalf("counting blocks: %v", err)
	}
	if n != wantBlocks {
		t.Fatalf("found %d ```ebnf blocks, want %d — §0 was reorganized and every check "+
			"below would be reading a partial grammar", n, wantBlocks)
	}
	g := load(t)
	if got := len(g.Nonterminals()); got < 50 {
		t.Fatalf("only %d nonterminals parsed; the metasyntax reader is not working", got)
	}
}

// TestNonterminalCount pins §10's headline number. It counts only the names grammar.md writes
// (synthetics from the desugar carry an interpunct and are excluded), so the assertion is about
// the spec rather than about this package's rewriting.
func TestNonterminalCount(t *testing.T) {
	g := load(t)
	var spec []string
	for _, n := range g.Nonterminals() {
		if !strings.ContainsRune(n, '·') {
			spec = append(spec, n)
		}
	}
	if len(spec) != wantNonterminals {
		t.Errorf("grammar.md defines %d nonterminals, §10 claims %d", len(spec), wantNonterminals)
	}
}

// TestEveryTerminalIsAToken is R253's terminals rule, enforced: a production may not name a
// token that does not exist. Without it a typo becomes a terminal nothing can ever match, and
// the production silently stops deriving.
func TestEveryTerminalIsAToken(t *testing.T) {
	real := map[string]bool{}
	for _, k := range token.All() {
		real[k.String()] = true
	}
	for _, name := range load(t).Terminals() {
		if !real[name] {
			t.Errorf("%s is named as a terminal but is no lexer §0 token", name)
		}
	}
}

// TestNoUndefinedOrUnreachable: a nonterminal used and never defined is a hole; one defined
// and never reached is dead grammar. §10 makes both failures rather than review questions.
func TestNoUndefinedOrUnreachable(t *testing.T) {
	g := load(t)
	if missing := g.Undefined(); len(missing) > 0 {
		t.Errorf("used but never defined: %v", missing)
	}
	dead := g.Unreachable()
	if len(dead) != 0 {
		t.Errorf("defined but unreachable from %s: %v", g.Start, dead)
	}
	// And the converse of reachability: File must be the only nonterminal on no right-hand
	// side. A second one would be dead grammar that Unreachable cannot see, because it is
	// reachable from itself.
	used := map[string]bool{}
	for _, p := range g.Prods {
		for _, s := range p.RHS {
			if !s.IsTerminal {
				used[s.Name] = true
			}
		}
	}
	for _, n := range g.Nonterminals() {
		if strings.ContainsRune(n, '·') {
			continue
		}
		if !used[n] && n != wantStartUnreach {
			t.Errorf("%s appears on no right-hand side and is not the start symbol", n)
		}
	}
}

// TestKeywordClassMatchesLexer is the pin R252's path-segment rule needs: a keyword added to
// the lexer and forgotten in `Keyword` would make `import newkw.x;` silently fail to parse.
func TestKeywordClassMatchesLexer(t *testing.T) {
	g := load(t)
	if got := g.Alternatives("Keyword"); got != wantKeywordAlts {
		t.Errorf("Keyword has %d alternatives, lexer §10 has %d keywords", got, wantKeywordAlts)
	}

	inClass := map[string]bool{}
	for _, p := range g.Prods {
		if p.LHS == "Keyword" && len(p.RHS) == 1 {
			inClass[p.RHS[0].Name] = true
		}
	}
	for _, k := range token.All() {
		if k.Category() != token.Keyword && inClass[k.String()] {
			t.Errorf("%s is in Keyword but is not a keyword token", k)
		}
		if k.Category() == token.Keyword && !inClass[k.String()] {
			t.Errorf("%s is a keyword token and is missing from Keyword", k)
		}
	}
}

// --- the parse gate --------------------------------------------------------------------

// lexBlock runs a corpus block through the real lexer and drops trivia, which is the token
// stream grammar §0 is defined over.
func lexBlock(src string) ([]ebnf.Token, error) {
	f, err := source.New("block.luna", src)
	if err != nil {
		return nil, err
	}
	toks, diags := lexer.Lex(f)
	if len(diags) > 0 {
		return nil, fmt.Errorf("%d lexical diagnostics", len(diags))
	}
	var out []ebnf.Token
	for _, tk := range toks {
		if tk.Kind.IsTrivia() {
			continue
		}
		out = append(out, ebnf.Token{Kind: tk.Kind, Text: f.Slice(tk.Offset, tk.Len)})
	}
	return out, nil
}

// TestCorpusParses is the parse gate lexer-testing-plan §9 names as the strong one: a lexing
// gate is permissive by construction (`pub`, `caps.io`, `!T` all lex clean), where a parse
// gate rejects a retired spelling the moment it stops being grammatical.
//
// It is reported per block, named by file and line, because "seed#312 failed" tells nobody
// which block broke.
func TestCorpusParses(t *testing.T) {
	blocks, err := spec.LunaBlocks()
	if err != nil {
		t.Fatalf("reading the corpus: %v", err)
	}
	if len(blocks) < corpusFloorBlocks {
		t.Fatalf("only %d blocks found, want at least %d — the corpus walk is broken, and a "+
			"gate that finds nothing passes", len(blocks), corpusFloorBlocks)
	}
	g := load(t)

	for _, b := range blocks {
		name := fmt.Sprintf("%s:%d", b.Path, b.Line)
		t.Run(name, func(t *testing.T) {
			toks, err := lexBlock(b.Source)
			if err != nil {
				t.Fatalf("lexing: %v", err)
			}
			r := g.Recognize(toks)
			if !r.Accepted {
				t.Errorf("does not derive: %s", r.Explain(toks))
			}
		})
	}
}

// TestCorpusIsUnambiguous is the question associativity §4 asserted and nothing ever tested.
// It is separate from the gate above so an ambiguity does not read as a parse failure: a
// block that derives two ways still derives.
func TestCorpusIsUnambiguous(t *testing.T) {
	blocks, err := spec.LunaBlocks()
	if err != nil {
		t.Fatalf("reading the corpus: %v", err)
	}
	g := load(t)

	for _, b := range blocks {
		toks, err := lexBlock(b.Source)
		if err != nil {
			continue // reported by TestCorpusParses
		}
		if r := g.Recognize(toks); r.Ambiguous {
			t.Errorf("%s:%d derives more than one way", b.Path, b.Line)
		}
	}
}
