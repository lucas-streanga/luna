// The inventory pin (lexer-testing-plan §1).
//
// Three parties must agree about the token inventory: lexer §0's table, lexer §10's
// prose summary, and this package's constants. Checking code against the table alone
// would miss the defect this project has already shipped once: R232 fixed a "47
// patterns" claim standing over a 49-row table, a prose-vs-table drift that no
// two-party check sees.
//
// These tests read the spec rather than a transcription of it, which means they can
// fail *open* if the spec is reformatted and the reader silently finds nothing.
// TestSpecReaderIsArmed exists to make that impossible: it asserts the reader found
// a plausible §0 and a complete §10 before any comparison is trusted.
package token_test

import (
	"sort"
	"testing"

	"luna/internal/spec"
	"luna/oracle/token"
)

// categoryCount is the number of token categories in §0, error included: since R242 the
// error productions emit INVALID rather than nothing, so every category has a token.
const categoryCount = 10

func loadSpec(t *testing.T) *spec.Inventory {
	t.Helper()
	inv, err := spec.Load()
	if err != nil {
		t.Fatalf("reading the lexer spec: %v", err)
	}
	return inv
}

// TestSpecReaderIsArmed guards the guards. Every other test here compares against
// what the reader extracted, so a reader that extracted nothing would make them all
// pass while checking nothing. §10's summary is one wrapped paragraph in which a
// count can be separated from its category by a line break, so it is the fragile
// part and the one asserted hardest.
func TestSpecReaderIsArmed(t *testing.T) {
	inv := loadSpec(t)

	if len(inv.Rows) == 0 {
		t.Fatal("no §0 rows parsed; the table header or row format changed")
	}
	if inv.Claims.Rows == 0 || inv.Claims.Tokens == 0 {
		t.Fatal("§10's totals not parsed; the summary phrasing changed")
	}
	if got := len(inv.Claims.ByCategory); got != categoryCount {
		t.Fatalf("§10 yielded %d per-category counts, want %d — the summary was reformatted "+
			"and the count check would silently pass; claims=%v",
			got, categoryCount, inv.Claims.ByCategory)
	}

	// The row-versus-kind arithmetic depends on rows the reader must tell apart: the ones
	// sharing INVALID, and the ones sharing DOUBLE and BYTES. If it stops distinguishing
	// them the totals still add up by accident, so assert the shapes directly. Since
	// R242 no row is nameless: every §0 row names a token.
	var nameless, noted, invalid int
	for _, r := range inv.Rows {
		switch {
		case !r.IsToken():
			nameless++
		case r.Name == "INVALID":
			invalid++
		}
		if r.Note != "" {
			noted++
		}
	}
	if nameless != 0 {
		t.Errorf("found %d rows naming no token, want 0 (R242)", nameless)
	}
	if invalid != 3 {
		t.Errorf("found %d INVALID rows, want 3 (two error productions and the catch-all)", invalid)
	}
	if noted != 4 {
		t.Errorf("found %d rows with a name qualifier, want 4 (DOUBLE ×2, BYTES ×2)", noted)
	}
}

// TestKindsMatchSpecNames pins the inventory itself: every §0 token has a Kind and
// every Kind has a §0 row, compared by the spec's own names.
func TestKindsMatchSpecNames(t *testing.T) {
	inv := loadSpec(t)

	inSpec := map[string]bool{}
	for _, r := range inv.Tokens() {
		inSpec[r.Name] = true
	}
	inCode := map[string]bool{}
	for _, k := range token.All() {
		if inCode[k.String()] {
			t.Errorf("two kinds share the name %q", k.String())
		}
		inCode[k.String()] = true
	}

	if missing := diff(inSpec, inCode); len(missing) > 0 {
		t.Errorf("in §0 but not in token.go: %v", missing)
	}
	if extra := diff(inCode, inSpec); len(extra) > 0 {
		t.Errorf("in token.go but not in §0: %v", extra)
	}
}

// TestCategoriesMatchSpec pins each token's category. A kind in the right set with
// the wrong category would pass the name check above, and would break the parser's
// trivia predicate (R236) without any count moving.
func TestCategoriesMatchSpec(t *testing.T) {
	inv := loadSpec(t)

	want := map[string]string{}
	for _, r := range inv.Tokens() {
		want[r.Name] = r.Category
	}
	for _, k := range token.All() {
		w, ok := want[k.String()]
		if !ok {
			continue // reported by TestKindsMatchSpecNames
		}
		if got := k.Category().String(); got != w {
			t.Errorf("%s: category %q in token.go, %q in §0", k, got, w)
		}
	}
}

// TestCountsAgree is the three-party check. §0's table, §10's prose, and this
// package must report the same totals, which is the shape of the R232 defect.
func TestCountsAgree(t *testing.T) {
	inv := loadSpec(t)
	actual, claims := inv.Actual(), inv.Claims

	if actual.Rows != claims.Rows {
		t.Errorf("row count: §0 has %d, §10 claims %d", actual.Rows, claims.Rows)
	}
	if actual.Tokens != claims.Tokens {
		t.Errorf("token count: §0 has %d, §10 claims %d", actual.Tokens, claims.Tokens)
	}
	if got := len(token.All()); got != actual.Tokens {
		t.Errorf("token count: token.go has %d, §0 has %d", got, actual.Tokens)
	}

	byCategory := map[string]int{}
	for _, k := range token.All() {
		byCategory[k.Category().String()]++
	}

	// Compared over the union of key sets, so a category present in one party and
	// absent from another is a failure rather than an unvisited comparison.
	for _, cat := range union(actual.ByCategory, claims.ByCategory, byCategory) {
		a, c, g := actual.ByCategory[cat], claims.ByCategory[cat], byCategory[cat]
		if a != c || a != g {
			t.Errorf("category %q: §0 has %d, §10 claims %d, token.go has %d", cat, a, c, g)
		}
	}
}

// TestUnsetIsNotAToken pins the zero value. Kind's zero must not name a real
// token, or an uninitialized token silently reads as WHITESPACE (the first kind declared) and
// every span check downstream inherits the lie.
func TestUnsetIsNotAToken(t *testing.T) {
	if got := token.Unset.String(); got != "UNSET" {
		t.Errorf("Unset.String() = %q, want %q", got, "UNSET")
	}
	if got := token.Unset.Category(); got != token.CategoryUnset {
		t.Errorf("Unset.Category() = %v, want CategoryUnset", got)
	}
	for _, k := range token.All() {
		if k == token.Unset {
			t.Fatal("All() includes Unset")
		}
	}
}

func diff(a, b map[string]bool) []string {
	var out []string
	for k := range a {
		if !b[k] {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

func union(ms ...map[string]int) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range ms {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
	}
	sort.Strings(out)
	return out
}
