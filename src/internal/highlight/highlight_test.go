package highlight

import (
	"html"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"luna/internal/spec"
	"luna/oracle/source"
	"luna/oracle/token"
)

// TestEveryKindHasAClass is the totality pin. §0's inventory is the set of things that can
// appear in a rendered page, so a kind missing from the table is a construct that would
// render uncoloured, or, since classOf panics, not at all. Walking token.All means adding
// a kind to §0 fails here rather than in a docs build.
func TestEveryKindHasAClass(t *testing.T) {
	for _, k := range token.All() {
		if _, ok := classes[k]; !ok {
			t.Errorf("no class for %s", k)
		}
	}
	if len(token.All()) == 0 {
		t.Fatal("token.All is empty: this test checked nothing")
	}
	if _, ok := classes[token.Unset]; ok {
		t.Error("UNSET has a class; it is not a token and must never be rendered")
	}
}

// TestUnknownKindPanics confirms the assert classOf's doc claims. The branch is unreachable
// for every kind §0 defines (TestEveryKindHasAClass is what makes it so) and an
// unreachable branch that silently returns something plausible is how a gap gets shipped.
func TestUnknownKindPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("classOf returned normally for a kind with no class")
		}
	}()
	classOf(token.Kind(250), "x")
}

// TestEveryClassIsStyled pins the renderer to the stylesheet. A class with no rule is a
// token that renders in the body colour, which looks like a highlighting bug and is
// invisible in any test that only checks the HTML.
func TestEveryClassIsStyled(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range classes {
		if c == classPlain || seen[c] {
			continue
		}
		seen[c] = true
		if !strings.Contains(StyleSheet, "."+c+" ") {
			t.Errorf("class %q is emitted but has no rule in StyleSheet", c)
		}
	}
	if len(seen) == 0 {
		t.Fatal("no classes found: this test checked nothing")
	}
}

// TestRenderIsLossless is the property a regex grammar cannot offer.
//
// Tokens tile the input (R242), so the rendering is a partition of the source: stripping
// the markup must give back exactly the bytes that went in, not merely something that
// looks the same, and with no allowance for a dropped byte in a construct the renderer
// mishandled. Run over the whole spec corpus it covers every construct anyone has written
// down, which is a wider net than any set of cases written here.
func TestRenderIsLossless(t *testing.T) {
	blocks, err := spec.LunaBlocks()
	if err != nil {
		t.Fatalf("reading the spec corpus: %v", err)
	}
	if len(blocks) == 0 {
		t.Fatal("the spec corpus is empty: check the src/specs symlink")
	}

	for _, b := range blocks {
		f, err := source.New(b.Path, b.Source)
		if err != nil {
			continue // ingress rejected it; there is no stream to render
		}
		out, _ := Render(f)
		if got := strip(out); got != b.Source {
			t.Errorf("%s:%d: render is lossy\n got: %q\nwant: %q", b.Path, b.Line, got, b.Source)
		}
	}
}

// TestRenderIsLosslessOnBrokenInput extends that to source no production claims. R242 tiles
// with INVALID precisely so the stream stays total over garbage, and a docs build is where
// garbage actually turns up -- a half-written snippet in a draft.
func TestRenderIsLosslessOnBrokenInput(t *testing.T) {
	cases := []string{
		"let x = \"unterminated;\n",
		"let x = ~\"a;\n",
		"#^@\n",
		"let x = 'a\\q';\n",
		"\"\"\"\n  hi\n",
		"let x = `cmd ${\n",
		"",
		"\n\n\n",
		"let x = <&>;\n",
	}
	for _, src := range cases {
		f, err := source.New("t.luna", src)
		if err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		out, _ := Render(f)
		if got := strip(out); got != src {
			t.Errorf("%q: render is lossy\n got: %q", src, got)
		}
	}
}

// TestRenderColoursTheObviousThings is the sanity floor: losslessness is satisfied by
// emitting the source with no spans at all, so something has to assert that colour happens
// and lands where it should.
func TestRenderColoursTheObviousThings(t *testing.T) {
	const src = `// note
let n = 1 + count;
let s = "a ${n} b\n";
let t: list = f(~"x"i);
`
	f, err := source.New("t.luna", src)
	if err != nil {
		t.Fatal(err)
	}
	out, errs := Render(f)
	if !errs.Empty() {
		t.Fatalf("unexpected diagnostics: %v", errs)
	}

	want := []string{
		`<span class="tok-com">// note</span>`,
		`<span class="tok-decl">let</span>`,
		`<span class="tok-num">1</span>`,
		`<span class="tok-op">+</span>`,
		`<span class="tok-interp">${</span>`,
		`<span class="tok-esc">\n</span>`,
		`<span class="tok-type">list</span>`,
		// One span, not three: the literal holds no splice, so §0's span pattern matches it
		// whole (F1) and the flags ride along on the same token.
		`<span class="tok-regex">~&#34;x&#34;i</span>`,
	}
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("missing %s\nin: %s", w, out)
		}
	}
	// Ordinary identifiers must stay bare. A renderer that wrapped every token would satisfy
	// every check above and say nothing about the one distinction §7 actually makes.
	if strings.Contains(out, `>count</span>`) {
		t.Errorf("identifier `count` should render bare, got: %s", out)
	}
}

// TestBuiltinsMatchSpec pins the one set this package maintains by hand.
//
// The guards matter more than the comparison. An extractor that silently matches nothing --
// a renamed heading, a reformatted table -- would report perfect agreement, which is the
// fail-open this repo keeps finding. So every section must be present and every section
// must contribute.
func TestBuiltinsMatchSpec(t *testing.T) {
	sections := []string{
		"Primitive and value types",
		"Structured types",
		"Declaration forms (and their union supertypes)",
		"The top and bottom types",
	}

	root, err := spec.Root()
	if err != nil {
		t.Fatalf("locating the spec: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "specs", "overview", "types.md"))
	if err != nil {
		t.Fatalf("reading types.md: %v", err)
	}

	found := map[string]bool{}
	perSection := map[string]int{}
	heading := ""
	for _, line := range strings.Split(string(raw), "\n") {
		if after, ok := strings.CutPrefix(line, "## "); ok {
			heading = strings.TrimSpace(after)
			continue
		}
		if !wanted(sections, heading) || !strings.HasPrefix(line, "| `") {
			continue
		}
		name, _, ok := strings.Cut(line[3:], "`")
		if !ok {
			t.Errorf("%s: unclosed backtick in row: %s", heading, line)
			continue
		}
		found[name] = true
		perSection[heading]++
	}

	for _, s := range sections {
		if perSection[s] == 0 {
			t.Fatalf("section %q contributed no type names: types.md changed shape and this "+
				"check stopped reaching anything", s)
		}
	}
	for name := range found {
		if !builtins[name] {
			t.Errorf("types.md lists %q; builtins does not", name)
		}
	}
	for name := range builtins {
		if !found[name] {
			t.Errorf("builtins has %q; types.md no longer lists it", name)
		}
	}
}

func wanted(all []string, s string) bool {
	for _, a := range all {
		if a == s {
			return true
		}
	}
	return false
}

// strip recovers the source from rendered HTML.
//
// Scanning for `<` is enough because every byte of source text has been through
// html.EscapeString by the time it lands in the output, so a `<` in the source is `&lt;`
// there and the only unescaped angle brackets are the renderer's own tags.
func strip(out string) string {
	var b strings.Builder
	for i := 0; i < len(out); i++ {
		if out[i] != '<' {
			b.WriteByte(out[i])
			continue
		}
		for i < len(out) && out[i] != '>' {
			i++
		}
	}
	return html.UnescapeString(b.String())
}
