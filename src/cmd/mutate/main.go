// Command mutate is a small mutation-testing harness for the oracle.
//
//	go run ./cmd/mutate            # run every mutant
//	go run ./cmd/mutate -list      # print them without running
//	go run ./cmd/mutate -run margin
//
// It tests the *tests*. Each mutant is a deliberate small defect; the suite is run against
// it; and a mutant the suite still passes is a **survivor** — which is a hole in the suite,
// because something broke and nothing noticed.
//
// Coverage spans every package with behaviour worth breaking: the lexer, the escape table,
// source, token, diagnostic, and the two module phases with the driver that wires them.
//
// Survivors are the output that matters, and the tool exits non-zero when there are any.
//
// Mutants are written by hand rather than generated. Two reasons, both learned the hard
// way in this package. A generated mutant is often *equivalent* — semantically identical
// to the original, unkillable by anything, and only distinguishable by a human — so a
// generator mostly produces triage work. And a hand-written mutant can say which test
// ought to catch it, which turns "something failed" into "the test I believed in failed",
// the difference between coverage and understanding.
//
// The uniqueness check below is not ceremony either: an inert mutation, one that applies
// but cannot be reached, looks exactly like a suite that is too weak. That mistake was made
// twice by hand before this existed.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"luna/internal/spec"
)

// A mutant is one deliberate defect: a unique substring of a file, what to replace it
// with, and which test is expected to notice.
type mutant struct {
	name   string // what the defect is, in a few words
	file   string // relative to the module root
	old    string // must appear exactly once
	new    string
	expect string // the test that should kill it
}

var mutants = []mutant{
	// --- trivia (§2) ---
	{
		name:   "whitespace run stops at carriage return",
		file:   "oracle/lexer/core.go",
		old:    `func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\r' || c == '\n' }`,
		new:    `func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' }`,
		expect: "TestGolden",
	},
	{
		name:   "block comment closes at the last */ rather than the first",
		file:   "oracle/lexer/trivia.go",
		old:    `if i := strings.Index(s.src[s.pos+2:], "*/"); i >= 0 {`,
		new:    `if i := strings.LastIndex(s.src[s.pos+2:], "*/"); i >= 0 {`,
		expect: "TestGolden",
	},
	{
		name:   "line comment swallows its newline",
		file:   "oracle/lexer/trivia.go",
		old:    "\t\ts.pos += i\n\t} else {",
		new:    "\t\ts.pos += i + 1\n\t} else {",
		expect: "TestGolden",
	},

	// --- words (§3, §7) ---
	{
		name:   "match! no longer folds",
		file:   "oracle/lexer/word.go",
		old:    `case word == "match" && s.peek(0) == '!':`,
		new:    `case false:`,
		expect: "TestMunchSamples",
	},
	{
		// The fold is whitespace-only by §0's regex, so a comment between the words must
		// defeat it. Dropping `i == s.pos` instead is an *equivalent* mutant and was tried
		// here first: lexWord has already eaten every identifier byte, so src[s.pos:] can
		// never begin with `from` unless whitespace was skipped, and the guard is dead
		// weight rather than a check. Left recorded, because the next person will reach
		// for the same one.
		name: "yield from folds across a comment",
		file: "oracle/lexer/word.go",
		old:  "\tfor i < len(s.src) && isSpace(s.src[i]) {\n\t\ti++\n\t}",
		new: "\tfor i < len(s.src) {\n\t\tif isSpace(s.src[i]) {\n\t\t\ti++\n\t\t\tcontinue\n\t\t}\n" +
			"\t\tif strings.HasPrefix(s.src[i:], \"/*\") {\n" +
			"\t\t\tif j := strings.Index(s.src[i+2:], \"*/\"); j >= 0 {\n\t\t\t\ti += 2 + j + 2\n\t\t\t\tcontinue\n\t\t\t}\n\t\t}\n\t\tbreak\n\t}",
		expect: "TestMaximalMunch",
	},
	{
		name:   "the wildcard becomes an ordinary identifier",
		file:   "oracle/lexer/word.go",
		old:    `	case word == "_":`,
		new:    `	case false:`,
		expect: "TestMunchSamples",
	},

	// --- numbers (§4, R238) ---
	{
		name:   "DOUBLE no longer requires a digit after the point",
		file:   "oracle/lexer/number.go",
		old:    `if s.peek(0) == '.' && isDigit(s.peek(1)) {`,
		new:    `if s.peek(0) == '.' {`,
		expect: "TestGolden",
	},
	{
		name:   "an incomplete exponent is consumed anyway",
		file:   "oracle/lexer/number.go",
		old:    "\tif !isDigit(byteAt(s.src, i)) {\n\t\treturn false\n\t}",
		new:    "\tif false {\n\t\treturn false\n\t}",
		expect: "TestGolden",
	},
	{
		name:   "digit separators may double up",
		file:   "oracle/lexer/number.go",
		old:    "\t\tj := i\n\t\tif src[j] == '_' {\n\t\t\tj++\n\t\t}",
		new:    "\t\tj := i\n\t\tfor j < len(src) && src[j] == '_' {\n\t\t\tj++\n\t\t}",
		expect: "TestGolden",
	},
	{
		name:   "the leading-zero error production is dropped",
		file:   "oracle/lexer/number.go",
		old:    `if s.src[start] == '0' && isDigitOrSep(s.peek(0)) {`,
		new:    `if false {`,
		expect: "TestGolden",
	},

	// --- operators (§5, §8) ---
	{
		name:   "the operator table is tried shortest-first",
		file:   "oracle/lexer/core.go",
		old:    "\tfor _, e := range opsByByte[s.src[s.pos]] {",
		new:    "\tfor i := len(opsByByte[s.src[s.pos]]) - 1; i >= 0; i-- {\n\t\te := opsByByte[s.src[s.pos]][i]",
		expect: "TestMunchSamples",
	},

	// --- literals and escapes (§4, R248) ---
	{
		name:   "every escape is accepted, whatever its context",
		file:   "oracle/escape/escape.go",
		old:    `	case strings.IndexByte(allowed[ctx], c) < 0:`,
		new:    `	case strings.IndexByte(allowed[ctx], c) < -1:`,
		expect: "TestGolden",
	},
	{
		name:   "surrogates pass as valid scalars",
		file:   "oracle/escape/escape.go",
		old:    `	if v > 0x10FFFF || (v >= 0xD800 && v <= 0xDFFF) {`,
		new:    `	if false {`,
		expect: "TestGolden",
	},
	{
		name:   "\\xNN accepts a single hex digit",
		file:   "oracle/escape/escape.go",
		old:    `	if !isHex(byteAt(src, i+2)) || !isHex(byteAt(src, i+3)) {`,
		new:    `	if !isHex(byteAt(src, i+2)) {`,
		expect: "TestGolden",
	},
	{
		name:   "a literal may span lines whatever its form",
		file:   "oracle/lexer/form.go",
		old:    `	formDq:       {escapes: escape.StringDq, splices: true, splicesIdent: true},`,
		new:    `	formDq:       {escapes: escape.StringDq, splices: true, splicesIdent: true, spansLines: true},`,
		expect: "TestGolden",
	},

	// --- modes (§1, §6) ---
	{
		name:   "the $ chain tries DOLLAR_TEXT before ${",
		file:   "oracle/lexer/literal_mode.go",
		old:    "\tcase s.peek(1) == '{':\n\t\ts.push(mode{kind: modeInterp, open: s.pos, openLen: 2})",
		new:    "\tcase false:\n\t\ts.push(mode{kind: modeInterp, open: s.pos, openLen: 2})",
		expect: "TestGolden",
	},
	{
		name:   "brace depth never decrements",
		file:   "oracle/lexer/core.go",
		old:    "\ts.modes[i].depth--\n\treturn token.RBrace",
		new:    "\treturn token.RBrace",
		expect: "TestGolden",
	},
	{
		name:   "frames open at end of input are not reported",
		file:   "oracle/lexer/lexer.go",
		old:    "\ts.finished = true\n\tfor _, m := range s.modes[1:] {",
		new:    "\ts.finished = true\n\tfor _, m := range s.modes[1:1] {",
		expect: "TestRandomStreams",
	},

	// --- triples (R246) ---
	{
		name:   "the margin is always empty",
		file:   "oracle/lexer/triple.go",
		old:    "\t\tif strings.HasPrefix(src[j:], delim) {\n\t\t\treturn src[i:j]\n\t\t}",
		new:    "\t\tif strings.HasPrefix(src[j:], delim) {\n\t\t\treturn \"\"\n\t\t}",
		expect: "TestGolden",
	},
	{
		name: "insufficient indentation goes unreported",
		file: "oracle/lexer/triple.go",
		old: "\ts.error(diagnostic.InsufficientIndentation, s.pos, 1,\n" +
			"\t\t\"this line is not indented by the closing delimiter's margin (%d bytes)\", len(m.margin))",
		new:    "\t_ = m",
		expect: "TestGolden",
	},
	{
		name:   "a triple's closing line need not match the margin",
		file:   "oracle/lexer/triple.go",
		old:    "\tif !strings.HasPrefix(s.src[i:], m.margin) {\n\t\treturn -1\n\t}",
		new:    "\tif false {\n\t\treturn -1\n\t}",
		expect: "TestGolden",
	},
	{
		name:   "trailing whitespace in a triple stays content",
		file:   "oracle/lexer/triple.go",
		old:    "\t\tfor end > s.pos && isHorizontalSpace(s.src[end-1]) {\n\t\t\tend--\n\t\t}",
		new:    "\t\tfor false {\n\t\t\tend--\n\t\t}",
		expect: "TestGolden",
	},

	// --- ingress (lexical-structure §1) ---
	{
		name:   "a leading BOM is accepted",
		file:   "oracle/source/source.go",
		old:    `	if strings.HasPrefix(text, "\ufeff") {`,
		new:    `	if false {`,
		expect: "TestRejectsLeadingBOM",
	},
	{
		name:   "invalid UTF-8 passes ingress",
		file:   "oracle/source/source.go",
		old:    `		if r == utf8.RuneError && size <= 1 {`,
		new:    `		if r == utf8.RuneError && size < 1 {`,
		expect: "TestRejectsInvalidUTF8",
	},
	{
		name:   "the pure-ASCII flag is never cleared",
		file:   "oracle/source/source.go",
		old:    "\t\tascii = false\n",
		new:    "",
		expect: "TestASCIIFlag",
	},
	{
		name:   "the line search is off by one",
		file:   "oracle/source/source.go",
		old:    `	i := sort.Search(len(f.lines), func(i int) bool { return f.lines[i] > int32(offset) }) - 1`,
		new:    `	i := sort.Search(len(f.lines), func(i int) bool { return f.lines[i] > int32(offset) })`,
		expect: "TestPosition",
	},
	{
		name:   "a line starts on its newline rather than after it",
		file:   "oracle/source/source.go",
		old:    `			lines = append(lines, int32(i+1))`,
		new:    `			lines = append(lines, int32(i))`,
		expect: "TestPosition",
	},

	// --- the token inventory (§0, §10) ---
	{
		name:   "IsTrivia names the wrong category",
		file:   "oracle/token/kind.go",
		old:    `func (k Kind) IsTrivia() bool { return k.Category() == Trivia }`,
		new:    `func (k Kind) IsTrivia() bool { return k.Category() == Keyword }`,
		expect: "TestMaximalMunch",
	},
	{
		name:   "All includes the Unset zero value",
		file:   "oracle/token/kind.go",
		old:    `	for k := 1; k < len(infos); k++ {`,
		new:    `	for k := 0; k < len(infos); k++ {`,
		expect: "TestUnsetIsNotAToken",
	},

	// --- diagnostics (§11, R240) ---
	{
		name:   "code 0000 is accepted",
		file:   "oracle/diagnostic/diagnostic.go",
		old:    "\treturn !zero\n",
		new:    "\treturn true\n",
		expect: "TestCodeValid",
	},
	{
		name:   "a code with no title validates",
		file:   "oracle/diagnostic/diagnostic.go",
		old:    `	case d.Code.Title() == "":`,
		new:    `	case false:`,
		expect: "TestValidate",
	},
	{
		name:   "Sorted reorders the caller's own list",
		file:   "oracle/diagnostic/diagnostic.go",
		old:    `	out := slices.Clone(l)`,
		new:    `	out := l`,
		expect: "TestListSorted",
	},

	// --- more of the DEFAULT dispatch (§8) ---
	{
		name:   "a shebang is recognized anywhere, not only at offset 0",
		file:   "oracle/lexer/core.go",
		old:    `	case s.pos == 0 && s.has("#!"):`,
		new:    `	case s.has("#!"):`,
		expect: "TestGolden",
	},
	{
		name:   "#[ is no longer an attribute opener",
		file:   "oracle/lexer/core.go",
		old:    "\t\tif s.has(\"#[\") {\n\t\t\ts.pos += 2\n\t\t\treturn token.AttrOpen\n\t\t}",
		new:    "\t\tif false {\n\t\t\ts.pos += 2\n\t\t\treturn token.AttrOpen\n\t\t}",
		expect: "TestGolden",
	},
	{
		name:   "b\" no longer beats IDENT",
		file:   "oracle/lexer/core.go",
		old:    `	case c == 'b' && (s.peek(1) == '"' || s.peek(1) == '\''):`,
		new:    `	case false:`,
		expect: "TestMunchSamples",
	},
	{
		name:   "braces count depth in DEFAULT as well as in a splice",
		file:   "oracle/lexer/core.go",
		old:    `	if s.modes[i].kind != modeInterp {`,
		new:    `	if false {`,
		expect: "TestRandomStreams",
	},
	{
		name:   "a bare ~ is not diagnosed",
		file:   "oracle/lexer/literal_fast.go",
		old:    "\tif !s.has(`~\"`) {",
		new:    "\tif false {",
		expect: "TestGolden",
	},

	// --- more numbers (§4) ---
	{
		name:   "an uppercase radix prefix is not diagnosed",
		file:   "oracle/lexer/number.go",
		old:    `		case 'X', 'B', 'O':`,
		new:    `		case 'Q':`,
		expect: "TestGolden",
	},
	{
		name:   "a radix prefix needs no digit after it",
		file:   "oracle/lexer/number.go",
		old:    "\tif i >= len(s.src) || !digit(s.src[i]) {\n\t\treturn token.Unset\n\t}",
		new:    "\tif false {\n\t\treturn token.Unset\n\t}",
		expect: "TestGolden",
	},
	{
		name:   "binary literals accept hex digits",
		file:   "oracle/lexer/number.go",
		old:    `		kind, digit = token.IntBin, isBinDigit`,
		new:    `		kind, digit = token.IntBin, isHexDigit`,
		expect: "TestGolden",
	},

	// --- more of the fast path (F1, F3) ---
	{
		name:   "a ${ no longer forces the mode path",
		file:   "oracle/lexer/literal_fast.go",
		old:    `		case r.splices && src[i] == '$' && byteAt(src, i+1) == '{':`,
		new:    `		case false:`,
		expect: "TestGolden",
	},
	{
		name:   "an unterminated literal swallows its newline",
		file:   "oracle/lexer/literal_fast.go",
		old:    "\t\tcase !r.spansLines && src[i] == '\\n':\n\t\t\tp.end = i",
		new:    "\t\tcase !r.spansLines && src[i] == '\\n':\n\t\t\tp.end = i + 1",
		expect: "TestGolden",
	},
	{
		name:   "regex flags are left outside the literal",
		file:   "oracle/lexer/literal_fast.go",
		old:    "\tfor end < len(s.src) && isRegexFlag(s.src[end]) {\n\t\tend++\n\t}",
		new:    "\tfor false {\n\t\tend++\n\t}",
		expect: "TestGolden",
	},

	// --- more of the literal modes (§6) ---
	{
		name:   "$name splices on a digit as well as a letter",
		file:   "oracle/lexer/literal_mode.go",
		old:    `	case form.rules().splicesIdent && isIdentStart(s.peek(1)):`,
		new:    `	case form.rules().splicesIdent && isIdentPart(s.peek(1)):`,
		expect: "TestGolden",
	},
	{
		name:   "a command's text run swallows $",
		file:   "oracle/lexer/literal_mode.go",
		old:    "\treturn s.textRun(token.CmdText, \"`\\\\$\\n\")",
		new:    "\treturn s.textRun(token.CmdText, \"`\\\\\\n\")",
		expect: "TestGolden",
	},
	{
		name:   "a regex body stops at a newline",
		file:   "oracle/lexer/literal_mode.go",
		old:    `	return s.textRun(token.RegexText, "\"\\$")`,
		new:    `	return s.textRun(token.RegexText, "\"\\$\n")`,
		expect: "TestGolden",
	},

	// --- more triples (R246) ---
	{
		name:   "content after a triple's opener is not diagnosed",
		file:   "oracle/lexer/triple.go",
		old:    "\tif j < len(s.src) && s.src[j] != '\\n' {",
		new:    "\tif false {",
		expect: "TestGolden",
	},
	{
		name:   "blank lines must carry the margin too",
		file:   "oracle/lexer/triple.go",
		old:    `	if m.margin == "" || blankLine(s.src, s.pos) {`,
		new:    `	if m.margin == "" {`,
		expect: "TestGolden",
	},
	{
		name:   "every position counts as a line start",
		file:   "oracle/lexer/triple.go",
		old:    `func (s *Scanner) atLineStart() bool { return s.pos > 0 && s.src[s.pos-1] == '\n' }`,
		new:    `func (s *Scanner) atLineStart() bool { return s.pos > 0 }`,
		expect: "TestGolden",
	},

	// --- the Scanner API ---
	{
		name:   "Errors hands back the live slice",
		file:   "oracle/lexer/lexer.go",
		old:    `func (s *Scanner) Errors() diagnostic.List { return slices.Clone(s.errors) }`,
		new:    "func (s *Scanner) Errors() diagnostic.List { _ = slices.Clone(s.errors); return s.errors }",
		expect: "TestScannerErrorsIsACopy",
	},

	// --- discovery: path resolution (modules §3) ---
	{
		name:   "std is not reserved",
		file:   "oracle/modules/discover.go",
		old:    `return module == std || strings.HasPrefix(module, std+".")`,
		new:    `return false`,
		expect: "TestReservedRoot",
	},
	{
		name:   "std matches by prefix, so `standard` is reserved too",
		file:   "oracle/modules/discover.go",
		old:    `return module == std || strings.HasPrefix(module, std+".")`,
		new:    `return strings.HasPrefix(module, std)`,
		expect: "TestReservedRoot",
	},
	{
		name:   "a module path's dots do not become directories",
		file:   "oracle/modules/discover.go",
		old:    `return strings.ReplaceAll(module, ".", "/") + ext`,
		new:    `return module + ext`,
		expect: "TestModulePaths",
	},
	{
		name: "the entry is not the root module",
		file: "oracle/modules/discover.go",
		old: `	if file == entry {
		return ""
	}`,
		new: `	if false {
		return ""
	}`,
		expect: "TestModulePaths",
	},

	// --- discovery: the walk (§1.0) ---
	{
		// Bounded on purpose. Dropping the !seen check outright loops forever, which the
		// harness can only report as a timeout — no named test fires, so it says nothing
		// about which check was watching. Un-seeding the entry instead lets it be reached
		// exactly twice, which a file-set assertion catches precisely.
		name:   "the visited set does not start with the entry",
		file:   "oracle/modules/discover.go",
		old:    `seen := map[string]bool{entry: true}`,
		new:    `seen := map[string]bool{}`,
		expect: "TestGraphShape",
	},
	{
		name:   "a missing entry is not an error",
		file:   "oracle/modules/discover.go",
		old:    "return Result{}, &Error{Code: diagnostic.MissingEntry, Path: entry}",
		new:    "_ = &Error{Code: diagnostic.MissingEntry, Path: entry}\n\t\t\t\t\tcontinue",
		expect: "TestErrors",
	},
	{
		name: "an ingress-rejected file is dropped from the file set (R251)",
		file: "oracle/modules/discover.go",
		old: `			res.Files = append(res.Files, File{Path: file, Module: module})
			continue`,
		new:    `			continue`,
		expect: "TestIngressRejectedFileIsListed",
	},
	{
		name:   "an import's recorded span loses its length",
		file:   "oracle/modules/discover.go",
		old:    `Offset: imported.offset, Len: imported.len,`,
		new:    `Offset: imported.offset,`,
		expect: "TestEdgeSpansThePath",
	},

	// --- discovery: the prelude reader (§5's grid, R136/R250/R252) ---
	{
		name:   "trivia is not skipped, so a comment ends the prelude",
		file:   "oracle/modules/discover.go",
		old:    `if !p.ok || !p.tok.IsTrivia() {`,
		new:    `if true {`,
		expect: "TestPreludeEnd",
	},
	{
		name:   "keywords are not path segments (R252)",
		file:   "oracle/modules/discover.go",
		old:    `	case token.Identifier, token.Keyword:`,
		new:    `	case token.Identifier:`,
		expect: "TestEveryKeywordIsAPathSegment",
	},
	{
		name: "the wildcard is not a path segment",
		file: "oracle/modules/discover.go",
		old: `	case token.Identifier, token.Keyword:
		return true
	}`,
		new: `	case token.Keyword:
		return true
	case token.Identifier:
		return p.tok.Kind != token.Wildcard
	}`,
		expect: "TestKeywordSegmentPositions",
	},
	{
		name: "an assigned import may not be annotated (modules §6)",
		file: "oracle/modules/discover.go",
		old: `		if p.at(token.Colon) && !p.skipAnnotation() {
			return importRef{}, false
		}`,
		new: `		if false {
			return importRef{}, false
		}`,
		expect: "TestFormSpace",
	},
	{
		name:   "export may not precede an assigned import",
		file:   "oracle/modules/discover.go",
		old:    `	p.accept(token.KwExport)`,
		new:    `	_ = token.KwExport`,
		expect: "TestFormSpace",
	},
	{
		name:   "a braced name list is not scanned past",
		file:   "oracle/modules/discover.go",
		old:    `		for !p.at(token.RBrace) {`,
		new:    `		for false {`,
		expect: "TestImportForms",
	},
	// An `if false` on the from-clause check is *equivalent*, not a gap: the p.advance()
	// after it still consumes whatever followed the brace list, so `import { a } dep;`
	// leaves the reader on `;`, path() fails, and the item is rejected either way.
	{
		name:   "an import needs no terminating semicolon",
		file:   "oracle/modules/discover.go",
		old:    `	if !ok || !p.accept(token.Semicolon) {`,
		new:    `	if !ok {`,
		expect: "TestPreludeEnd",
	},
	{
		name:   "a dotted path stops after its first segment",
		file:   "oracle/modules/discover.go",
		old:    `	for p.accept(token.Dot) {`,
		new:    `	for false {`,
		expect: "TestModulePaths",
	},
	{
		name:   "a file of pure imports reports its prelude ending at zero",
		file:   "oracle/modules/discover.go",
		old:    `			return len(f.Text()), imports`,
		new:    `			return 0, imports`,
		expect: "TestPreludeEnd",
	},

	// --- validation: resolution (§1.2, R251) ---
	{
		name:   "the root is found by list position rather than its empty path",
		file:   "oracle/modules/validate.go",
		old:    `		if f.Module == "" {`,
		new:    `		if false {`,
		expect: "TestRootImport",
	},
	{
		name:   "std edges are reported as unresolved imports",
		file:   "oracle/modules/validate.go",
		old:    `		if reserved(e.To) {`,
		new:    `		if false {`,
		expect: "TestStdIsNeverUnresolved",
	},
	{
		name: "importing the root is not its own error (R251)",
		file: "oracle/modules/validate.go",
		old:  `		case target == entry:`,
		// `&& false` rather than replacing the comparison: `entry` is used nowhere else, so
		// dropping it produces an unused variable and a mutant that only tests the compiler.
		new:    `		case target == entry && false:`,
		expect: "TestRootImport",
	},
	{
		name:   "an unresolved import goes unreported",
		file:   "oracle/modules/validate.go",
		old:    `		case !v.known(e.To):`,
		new:    `		case false:`,
		expect: "TestUnresolvedImport",
	},
	{
		name:   "duplicate imports are not deduped, so one cycle reports twice",
		file:   "oracle/modules/validate.go",
		old:    `			if !slices.Contains(v.imports[e.From], e.To) {`,
		new:    `			if true || slices.Contains(v.imports[e.From], e.To) {`,
		expect: "TestCycles",
	},

	// --- validation: cycles and layers ---
	{
		name:   "a back edge does not report a cycle",
		file:   "oracle/modules/validate.go",
		old:    `				v.reportCycle(stack, next)`,
		new:    `				_ = next`,
		expect: "TestCycles",
	},
	{
		name:   "a cycle's path is truncated to the module it closed on",
		file:   "oracle/modules/validate.go",
		old:    `	loop := append(append([]string{}, stack[start:]...), back)`,
		new:    `	loop := append(append([]string{}, stack[start:start]...), back)`,
		expect: "TestCycles",
	},
	{
		name:   "a layer is emitted before its dependencies are placed",
		file:   "oracle/modules/validate.go",
		old:    `		if !placed[imp] {`,
		new:    `		if placed[imp] && false {`,
		expect: "TestLayers",
	},
	// Changing PreludeEnd`s `>=` to `>` also survives, and is a question rather than a gap:
	// see the note in validate.go. A mutant that is arguably a fix does not belong here.
	{
		name:   "a missing token stream is tolerated rather than a driver bug",
		file:   "oracle/modules/validate.go",
		old:    `			panic("modules: no token stream for " + f.Path)`,
		new:    `			continue`,
		expect: "TestMissingTokenStreamPanics",
	},

	// --- the driver (driver.md §3, §4) ---
	{
		name:   "lexed files are merged in completion order",
		file:   "oracle/driver/driver.go",
		old:    `			units[i] = lexOne(fsys, f)`,
		new:    `			units[len(files)-1-i] = lexOne(fsys, f)`,
		expect: "TestOrderFollowsTheFileSet",
	},
	{
		name:   "a lexical error does not stop the compile at §1.1's boundary",
		file:   "oracle/driver/driver.go",
		old:    `	if !res.Diagnostics.Empty() {`,
		new:    `	if false {`,
		expect: "TestErrorsAcrossPhasesDoNotMix",
	},
	{
		name:   "an ingress failure is swallowed instead of reported",
		file:   "oracle/driver/driver.go",
		old:    `		u.diags.Add(fromSourceError(err, f.Path))`,
		new:    `		_ = fromSourceError(err, f.Path)`,
		expect: "TestIngressIsReported",
	},
	{
		name: "a file vanishing mid-build is not an error",
		file: "oracle/driver/driver.go",
		old: `		if u.err != nil {
			return Result{}, u.err
		}`,
		new: `		if false {
			return Result{}, u.err
		}`,
		expect: "TestFileVanishingMidBuildIsAnError",
	},
	{
		name:   "a completed build does not report reaching validation",
		file:   "oracle/driver/driver.go",
		old:    `	res.Graph, res.Diagnostics, res.Reached = graph, vdiags, PhaseValidate`,
		new:    `	res.Graph, res.Diagnostics, res.Reached = graph, vdiags, PhaseLex`,
		expect: "TestReachedNamesThePhase",
	},
}

func main() {
	list := flag.Bool("list", false, "print the mutants without running them")
	filter := flag.String("run", "", "only mutants whose name contains this")
	flag.Parse()

	root, err := spec.Root()
	if err != nil {
		fmt.Fprintf(os.Stderr, "mutate: %v\n", err)
		os.Exit(1)
	}

	selected := mutants[:0:0]
	for _, m := range mutants {
		if *filter == "" || strings.Contains(m.name, *filter) {
			selected = append(selected, m)
		}
	}
	if *list {
		for _, m := range selected {
			fmt.Printf("%-52s %-28s %s\n", m.name, m.expect, m.file)
		}
		return
	}

	var survivors, broken, unexpected []mutant
	start := time.Now()
	for i, m := range selected {
		fmt.Printf("[%2d/%2d] %-52s ", i+1, len(selected), m.name)
		switch outcome, by := run(root, m); outcome {
		case killed:
			// Whether the *expected* test noticed, not merely whether something did. A
			// kill by some other test is still a kill, but it means the suite's shape is
			// not what its author believed — and that is worth seeing.
			if !contains(by, m.expect) {
				fmt.Printf("killed by %s — but not by %s\n", strings.Join(by, ", "), m.expect)
				unexpected = append(unexpected, m)
			} else {
				fmt.Printf("killed by %s\n", strings.Join(by, ", "))
			}
		case hung:
			fmt.Println("killed — the suite hung, which is a kill by timeout")
		case survived:
			fmt.Println("SURVIVED")
			survivors = append(survivors, m)
		case uncompilable:
			fmt.Println("did not compile — the mutant is malformed, not the suite")
			broken = append(broken, m)
		}
	}

	fmt.Printf("\n%d mutants in %s: %d survived, %d malformed, %d killed by the wrong test\n",
		len(selected), time.Since(start).Round(time.Second),
		len(survivors), len(broken), len(unexpected))
	for _, m := range unexpected {
		fmt.Printf("  UNEXPECTED %-51s %s did not fire\n", m.name, m.expect)
	}
	for _, m := range survivors {
		fmt.Printf("  SURVIVED  %-52s %s expected to catch it\n", m.name, m.expect)
	}
	for _, m := range broken {
		fmt.Printf("  MALFORMED %-52s %s\n", m.name, m.file)
	}
	if len(survivors) > 0 || len(broken) > 0 || len(unexpected) > 0 {
		os.Exit(1)
	}
}

type outcome int

const (
	killed outcome = iota
	survived
	hung
	uncompilable
)

// run applies one mutant, runs the suite, and puts the file back.
//
// The original is held in memory and restored by a defer, so an interrupted run leaves the
// tree as it found it — this rewrites checked-in source, and that is the one thing it must
// never get wrong.
func run(root string, m mutant) (outcome, []string) {
	path := filepath.Join(root, m.file)
	original, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nmutate: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = os.WriteFile(path, original, 0o644) }()

	// Exactly one occurrence, or the mutation is ambiguous — and an ambiguous one applied
	// somewhere unreached is indistinguishable from a weak suite.
	if n := strings.Count(string(original), m.old); n != 1 {
		fmt.Fprintf(os.Stderr, "\nmutate: %q matches %d times in %s, want 1\n", m.name, n, m.file)
		os.Exit(1)
	}
	mutated := strings.Replace(string(original), m.old, m.new, 1)
	if err := os.WriteFile(path, []byte(mutated), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "\nmutate: %v\n", err)
		os.Exit(1)
	}

	cmd := exec.Command("go", "test", "-count=1", "-timeout", "60s", "./...")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		return survived, nil
	}
	text := string(out)
	switch {
	case strings.Contains(text, "[build failed]") || strings.Contains(text, "syntax error"):
		return uncompilable, nil
	case strings.Contains(text, "test timed out"):
		return hung, nil
	}
	return killed, failures(text)
}

var failRe = regexp.MustCompile(`--- FAIL: (\w+)`)

// failures names every test that noticed, deduplicated. Every one, not the first: Go runs
// tests in file order, so "the first failure" would report whichever test happens to sort
// earliest rather than the one that was meant to catch this.
func failures(out string) []string {
	seen := map[string]bool{}
	var out2 []string
	for _, m := range failRe.FindAllStringSubmatch(out, -1) {
		if name := m[1]; !seen[name] {
			seen[name] = true
			out2 = append(out2, name)
		}
	}
	if len(out2) == 0 {
		return []string{"the build or vet"}
	}
	return out2
}

func contains(all []string, want string) bool {
	for _, s := range all {
		if s == want {
			return true
		}
	}
	return false
}
