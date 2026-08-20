// The scaffold's behavioural specs: what each function must do, written before the bodies.
//
// **A spec is not a skip.** Every one runs for real; a stub's panic is caught and the spec is
// reported *pending on that stub*, which is checked against `scaffoldStubs`, the same table
// TestScaffoldIsUnimplemented pins. So a spec can only be pending for a function the scaffold
// agrees is unwritten, and the moment a body lands its name leaves that table and every spec
// waiting on it becomes a real assertion, failing loudly if the body is wrong. Nothing has to be
// remembered, and nothing passes by not running.
//
// Any other panic propagates: a stub that panics with the wrong message, or an implementation that
// panics where it should return, is a failure and not a pending.
package parser

import (
	"strings"
	"testing"

	"luna/oracle/source"
	"luna/oracle/token"
)

// expects runs one expectation against the scaffold.
func expects(t *testing.T, body func(t *testing.T)) {
	t.Helper()
	defer func() {
		v := recover()
		if v == nil {
			return
		}
		name, ok := scaffoldPanic(v)
		if !ok {
			panic(v)
		}
		if !scaffoldStubs[name] {
			t.Fatalf("pending on %s, which is not in scaffoldStubs: either the stub table is "+
				"stale or a body panics where it should return", name)
		}
		t.Logf("pending on %s", name)
	}()
	body(t)
}

func scaffoldPanic(v any) (string, bool) {
	s, ok := v.(string)
	if !ok || !strings.HasPrefix(s, "parser: ") || !strings.HasSuffix(s, " is unimplemented") {
		return "", false
	}
	return strings.TrimSuffix(strings.TrimPrefix(s, "parser: "), " is unimplemented"), true
}

// --- fixtures ------------------------------------------------------------------------------

// parserFor builds a parse in progress **without** newParser, so that a spec for the cursor is
// not blocked on the constructor. It computes pos the way §4.5 states the invariant rather than
// the way newParser will, which is what makes newParser's own spec worth having.
func parserFor(t *testing.T, src string) *parser {
	t.Helper()
	f, toks := lexFor(t, src)
	p := &parser{f: f, tokens: toks}
	for p.pos < len(toks) && toks[p.pos].IsTrivia() {
		p.pos++
	}
	return p
}

func lexFor(t *testing.T, src string) (*source.File, []token.Token) {
	t.Helper()
	lexed, err := LexGolden("spec.luna", src)
	if err != nil {
		t.Fatalf("lexing %q: %v", src, err)
	}
	return lexed.File, lexed.Tokens
}

// indexOf is the full-stream index of the n'th non-trivia token, which is what a token event
// carries (§2.2) and therefore what an expectation has to name.
func indexOf(toks []token.Token, n int) int {
	for i, tk := range toks {
		if tk.IsTrivia() {
			continue
		}
		if n == 0 {
			return i
		}
		n--
	}
	return -1
}

// events renders a stream on one line, so an expectation reads as a sentence rather than a slice
// literal. The debug dump §4.2 permits is exactly this.
func events(s eventStream) string {
	out := make([]string, 0, len(s))
	for _, e := range s {
		out = append(out, e.String())
	}
	return strings.Join(out, " ")
}

// expectsPanic is expects for a contract the scaffold enforces by panicking. A stub's panic is
// still a pending; any other panic must name the contract, and returning normally is a failure.
func expectsPanic(t *testing.T, want string, body func()) {
	t.Helper()
	got, panicked := recovered(body)
	if !panicked {
		t.Fatalf("returned instead of panicking; want a panic naming %q", want)
	}
	if name, ok := scaffoldPanic(got); ok {
		if !scaffoldStubs[name] {
			t.Fatalf("pending on %s, which is not in scaffoldStubs", name)
		}
		t.Logf("pending on %s", name)
		return
	}
	if !strings.Contains(got, want) {
		t.Errorf("panicked with %q, want it to name %q", got, want)
	}
}
