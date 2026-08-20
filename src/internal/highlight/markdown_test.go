package highlight

import (
	"strings"
	"testing"

	"luna/internal/spec"
)

func TestMarkdownRewritesOnlyLunaFences(t *testing.T) {
	const md = "# Title\n\nProse.\n\n```luna\nlet x = 1;\n```\n\n```go\nx := 1\n```\n\nEnd.\n"

	out, problems := Markdown("t.md", md)
	if len(problems) != 0 {
		t.Fatalf("unexpected problems: %v", problems)
	}
	if !strings.Contains(out, `<span class="tok-decl">let</span>`) {
		t.Errorf("luna fence was not highlighted:\n%s", out)
	}
	if !strings.Contains(out, "```go\nx := 1\n```") {
		t.Errorf("the go fence should be untouched:\n%s", out)
	}
	if strings.Contains(out, "```luna") {
		t.Errorf("the luna fence should be gone:\n%s", out)
	}
	for _, keep := range []string{"# Title", "Prose.", "End."} {
		if !strings.Contains(out, keep) {
			t.Errorf("lost surrounding prose %q:\n%s", keep, out)
		}
	}
}

// TestMarkdownLocatesProblemsInTheDocument is the reason renderBlock takes the fence index:
// a diagnostic reported at the block's own line 2 is useless to someone editing a 900-line
// spec file. The offset arithmetic is easy to get wrong by one and impossible to notice.
func TestMarkdownLocatesProblemsInTheDocument(t *testing.T) {
	// Fence on line 3, so the block's line 1 is document line 4 and line 2 is line 5.
	const md = "# T\n\n```luna\nlet ok = 1;\nlet bad = \"oops;\n```\n"

	_, problems := Markdown("t.md", md)
	if len(problems) != 1 {
		t.Fatalf("want 1 problem, got %d: %v", len(problems), problems)
	}
	p := problems[0]
	if p.Line != 5 {
		t.Errorf("problem at document line %d, want 5 (fence line 3 + block line 2)", p.Line)
	}
	if p.Path != "t.md" {
		t.Errorf("path %q, want t.md", p.Path)
	}
	if p.Code == "" || p.Message == "" {
		t.Errorf("problem carries no code or message: %+v", p)
	}
}

func TestMarkdownReportsAnUnclosedFence(t *testing.T) {
	const md = "# T\n\n```luna\nlet x = 1;\n"

	_, problems := Markdown("t.md", md)
	var fence bool
	for _, p := range problems {
		if p.Code == "fence" {
			fence, _ = true, p
			if p.Line != 3 {
				t.Errorf("fence problem at line %d, want 3", p.Line)
			}
		}
	}
	if !fence {
		t.Errorf("an unclosed ```luna fence went unreported: %v", problems)
	}
}

// TestMarkdownPreservesNonBlockText round-trips a document with no luna fences at all. Split
// and rejoin on "\n" is exact only if nothing else touches the lines, and a rewriter that
// quietly drops a trailing newline corrupts every file it is run over.
func TestMarkdownPreservesNonBlockText(t *testing.T) {
	for _, md := range []string{
		"",
		"\n",
		"# T\n\nprose\n",
		"no trailing newline",
		"```\nplain fence\n```\n",
	} {
		out, problems := Markdown("t.md", md)
		if out != md {
			t.Errorf("document changed\n got: %q\nwant: %q", out, md)
		}
		if len(problems) != 0 {
			t.Errorf("%q: unexpected problems: %v", md, problems)
		}
	}
}

// TestSpecRendersWithoutBuildProblems runs the gate over the real corpus.
//
// Every ```luna block in the spec goes through the oracle. A block that does not lex is
// either a mistake in the spec or an example that means to be broken, and either way the
// spec is where it gets settled, so this failing is informative rather than annoying.
func TestSpecRendersWithoutBuildProblems(t *testing.T) {
	blocks, err := spec.LunaBlocks()
	if err != nil {
		t.Fatalf("reading the spec corpus: %v", err)
	}
	if len(blocks) == 0 {
		t.Fatal("the spec corpus is empty: check the src/specs symlink")
	}

	for _, b := range blocks {
		_, problems := renderBlock(b.Path, b.Line-1, b.Source)
		for _, p := range problems {
			t.Errorf("%s:%d:%d: %s: %s", p.Path, p.Line, p.Column, p.Code, p.Message)
		}
	}
}
