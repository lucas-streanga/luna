// Pseudo-random streams (lexer-testing-plan §6).
//
// The same five properties FuzzLexer asserts, over a deliberately opposite distribution:
// blind uniform draws rather than coverage-guided mutation from real Luna, reaching shapes
// no golden would contain and no mutator starting from valid source would find quickly.
//
// It is also the only random exploration a plain `go test` performs. Go gives a fuzz
// target its seed corpus and nothing more unless -fuzz is passed, so without this the
// properties would be checked on 537 curated inputs and on nothing else until somebody
// remembered to fuzz. Sharing checkProperties rather than restating it is what keeps the
// two from drifting.
//
// In a memory-safe language the failure worth hunting is a panic, not a corruption, and
// every one the lexer could plausibly raise is a bug in *it*: an index past the end, a
// negative length, or Next's own progress assertion, which fires when a step consumes no
// bytes and would otherwise loop forever (R242).
//
// Internal, because property 5 reads s.modes.
package lexer

import (
	"strconv"
	"testing"

	"luna/oracle/source"
)

const (
	randomIterations = 100_000
	randomMaxLen     = 48 // short inputs reach more edge cases per byte than long ones
)

// lexerBytes is the alphabet the scanner actually branches on, plus a little ordinary
// text. Drawing from it rather than from all 256 is what gets past ingress and into the
// mode stack: a uniform byte is almost never valid UTF-8, so most of those inputs are
// rejected before the lexer sees them.
const lexerBytes = " \t\n\r?.|&-=!<>+*/%@#[](){},;:'\"`~$\\^_0123456789abxoeimsxbXBOu{}" +
	"letmatchyieldfromconst"

func TestRandomStreams(t *testing.T) {
	for _, c := range []struct {
		name     string
		alphabet string // empty means the full byte range
	}{
		{"uniform bytes", ""},
		{"lexer alphabet", lexerBytes},
	} {
		t.Run(c.name, func(t *testing.T) {
			// A fixed seed per subtest, so a failure names an input that reproduces by
			// rerunning — no flake, and no "works on my machine".
			r := rng(0x5eed)
			lexed := 0
			for i := range randomIterations {
				if checkOne(t, i, draw(&r, c.alphabet)) {
					lexed++
				}
			}

			// Reported rather than assumed. If ingress ever began rejecting everything,
			// this test would still pass while exercising nothing — the fail-open shape
			// that keeps turning up in this package.
			t.Logf("%d of %d inputs reached the lexer", lexed, randomIterations)
			if c.alphabet != "" && lexed != randomIterations {
				t.Errorf("the lexer alphabet is all ASCII, so every input should be valid UTF-8")
			}
		})
	}
}

// checkOne runs the shared property checks over one input, reporting whether ingress
// accepted it. The recover exists only so a panic names its input: without it the failure
// is a stack trace with nothing to reproduce from.
func checkOne(t *testing.T, i int, data []byte) (reached bool) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("panic on iteration %d, input %s:\n\t%v", i, strconv.Quote(string(data)), r)
		}
	}()

	checkProperties(t, data)
	return accepted(data)
}

// accepted asks the question source.New answers, for the reach count alone — a wrong
// answer here misreports a number rather than letting a broken input through.
func accepted(data []byte) bool {
	_, err := source.New("random", string(data))
	return err == nil
}

// draw builds one input. From alphabet when it is non-empty, otherwise from the full byte
// range, which is mostly not valid UTF-8 and so mostly tests that ingress refuses it
// cleanly.
func draw(r *rng, alphabet string) []byte {
	out := make([]byte, r.next()%(randomMaxLen+1))
	for i := range out {
		if alphabet == "" {
			out[i] = byte(r.next())
			continue
		}
		out[i] = alphabet[r.next()%uint64(len(alphabet))]
	}
	return out
}

// rng is splitmix64, written out rather than imported.
//
// Two reasons, and the second is the one that matters. The linter forbids math/rand in
// tests (testing-strategy §7) because a suite drawing on ambient randomness is not
// reproducible. And math/rand does not promise its sequence across Go releases, so a fixed
// seed there would silently start meaning something else after a toolchain bump — five
// lines here means a reported failure reproduces exactly, for as long as the file exists.
type rng uint64

func (r *rng) next() uint64 {
	*r += 0x9e3779b97f4a7c15
	z := uint64(*r)
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}
