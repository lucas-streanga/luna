package highlight

import (
	"html"
	"strings"

	"luna/oracle/source"
)

// Problem is something worth failing a docs build over, located in the markdown file
// rather than in the extracted block, which is the only location a person editing the
// document can act on.
//
// Code is the L#### of a lexical diagnostic, or a short word for a problem this rewriter
// found itself. The two share a channel because they share a consumer: a build that stops.
type Problem struct {
	Path    string
	Line    int
	Column  int
	Code    string
	Message string
}

// Markdown rewrites every ```luna block in a document into highlighted HTML, returning the
// new document and everything wrong with the blocks.
//
// Highlighting docs through the real lexer buys a second thing besides colour, and it is
// arguably the bigger one: a snippet that does not lex is now a build failure. A regex
// grammar cannot notice, colouring the broken snippet just as confidently, so today the
// spec's examples are checked by eye. Every block passes through the oracle here, and
// cmd/highlight -strict is what turns that into a gate.
//
// Fence recognition matches internal/spec exactly: a line whose trimmed text is ```luna, so
// a fence indented inside a list item counts, closed by the next line whose first
// non-space is a backtick fence. Keeping the two identical means the corpus the tests lex
// and the corpus the docs render are the same set of blocks, and neither can quietly
// include one the other misses.
func Markdown(path, md string) (string, []Problem) {
	var (
		out      []string
		problems []Problem
	)
	lines := strings.Split(md, "\n")

	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) != "```luna" {
			out = append(out, lines[i])
			continue
		}

		open := i
		var body []string
		for i++; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```"); i++ {
			body = append(body, lines[i])
		}
		if i >= len(lines) {
			// Reaching the end without a closer means every line after the fence was swallowed
			// as code. A markdown renderer reads it the same way, so the block still renders --
			// but a whole document turning into one code block is a typo, not an intent.
			problems = append(problems, Problem{
				Path: path, Line: open + 1, Column: 1, Code: "fence",
				Message: "unclosed ```luna fence: everything to end of file was taken as code",
			})
		}

		html, probs := renderBlock(path, open, strings.Join(body, "\n")+"\n")
		out = append(out, html)
		problems = append(problems, probs...)
	}
	return strings.Join(out, "\n"), problems
}

// renderBlock highlights one block and relocates its diagnostics into the markdown file.
//
// The block is lexed as a file of its own, so every offset it reports is block-relative;
// open is the fence's 0-based index, which makes the document line of the block's line L
// exactly open+1+L. Reporting the block-relative line instead would be the more obvious
// code and the less useful output.
func renderBlock(path string, open int, src string) (string, []Problem) {
	f, err := source.New(path, src)
	if err != nil {
		// Ingress rejects invalid UTF-8 and a leading BOM before there is anything to tokenize
		// (lexical-structure §1). Nothing can be highlighted, so the block is passed through as
		// plain preformatted text and the reason is reported.
		return `<pre class="luna"><code>` + html.EscapeString(src) + `</code></pre>`, []Problem{{
			Path: path, Line: open + 1, Column: 1, Code: "source",
			Message: err.Error(),
		}}
	}

	rendered, errs := Render(f)
	var problems []Problem
	for _, d := range errs.Sorted() {
		pos := f.Position(d.Primary.Offset)
		problems = append(problems, Problem{
			Path:    path,
			Line:    open + 1 + pos.Line,
			Column:  pos.Column,
			Code:    string(d.Code),
			Message: d.Description,
		})
	}
	return rendered, problems
}
