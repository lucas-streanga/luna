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
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
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

	// --- splice: §2.1's placement and §2.2's deferral ---
	//
	// The parser is not written yet, so these reach the machinery under it. `golden_bridge.go` is
	// deliberately not mutated: it is scaffolding, deleted when Parse lands.
	//
	// **Several of these panic rather than fail**, because the package checks its preconditions
	// always and on purpose — and a panic ends the test binary, so only the first test to reach it
	// gets to report. Where that happens `expect` names *that* test rather than the one which owns
	// the rule, and the narrow test never runs at all. It is worth knowing which mutants are in
	// that class, so each says so.
	{
		name:   "a close flushes pending trivia",
		file:   "oracle/parser/splice.go",
		old:    "\t\t\tif depth == 0 {\n\t\t\t\tflush()\n\t\t\t}\n\t\t\tout = append(out, e)",
		new:    "\t\t\tflush()\n\t\t\tout = append(out, e)",
		expect: "TestSplicePlacesTrivia",
	},
	{
		// Panics: reverting deferral sends empty nodes to the builder, which rejects them.
		name:   "an open is emitted rather than held",
		file:   "oracle/parser/splice.go",
		old:    "\t\tcase evOpen:\n\t\t\theld = append(held, e.node)",
		new:    "\t\tcase evOpen:\n\t\t\tif depth > 0 {\n\t\t\t\tflush()\n\t\t\t}\n\t\t\tout = append(out, e)\n\t\t\tdepth++",
		expect: "TestRandomShapes",
	},
	{
		name:   "a release emits its opens before the trivia",
		file:   "oracle/parser/splice.go",
		old:    "\t\tflush()\n\t\tfor _, k := range held[start:] {",
		new:    "\t\tdefer flush()\n\t\tfor _, k := range held[start:] {",
		expect: "TestSplicePlacesTrivia",
	},
	{
		// Both of these make a *precondition* fire, and a panic ends the test binary — so the
		// only test that gets to report is whichever runs first, by file name. The narrow tests
		// that own these rules (TestSpliceHoldsOpensUntilContent, TestSplicePlacesTrivia) never
		// run at all. `expect` therefore names the first test to reach it rather than the one
		// that means the most, and that is a property of the harness rather than a hole: `failures`
		// collects every FAIL precisely because a *failing* mutant reports several, and a
		// panicking one cannot.
		name:   "the root is not released by trailing trivia",
		file:   "oracle/parser/splice.go",
		old:    `		if len(held) == 1 && depth == 0 && next < len(tokens) {`,
		new:    `		if false {`,
		expect: "TestGoldenCorpusInvariants",
	},
	{
		name:   "the file's leading trivia is flushed outside File",
		file:   "oracle/parser/splice.go",
		old:    "\t\tstart := 0\n\t\tif depth == 0 {",
		new:    "\t\tstart := 0\n\t\tif false {",
		expect: "TestRandomShapes",
	},
	{
		// Panics: the index is emitted twice, which the builder meets as a token out of order.
		name:   "a consumed token is left unconsumed",
		file:   "oracle/parser/splice.go",
		old:    "\t\t\tout = append(out, e)\n\t\t\tnext = e.tok + 1",
		new:    "\t\t\tout = append(out, e)\n\t\t\tnext = e.tok",
		expect: "TestRandomShapes",
	},
	{
		name:   "splice accepts a token event naming trivia",
		file:   "oracle/parser/splice.go",
		old:    `			if tokens[e.tok].IsTrivia() {`,
		new:    `			if false {`,
		expect: "TestSpliceRejects",
	},
	{
		name:   "splice accepts a token the parser never reached",
		file:   "oracle/parser/splice.go",
		old:    "\tif next != len(tokens) {",
		new:    "\tif false {",
		expect: "TestSpliceRejects",
	},

	// --- the builder: spans (§4.2) and the arena (§3.1) ---
	{
		name:   "a node starts at its last child rather than its first",
		file:   "oracle/parser/builder.go",
		old:    "\tif !f.filled {\n\t\tf.offset, f.filled = offset, true\n\t}",
		new:    "\tf.offset, f.filled = offset, true",
		expect: "TestGoldens",
	},
	{
		// Panics, and unavoidably: `filled` doubles as "this frame has a child", so *any* mutant
		// that misroutes cover leaves some node empty and trips §6.1 before the run ends. Clamping
		// to the root is still worth it over a bare `n-3` — it keeps File covered long enough for
		// the span assertions to report, which is what this mutant is for.
		name:   "a closed node widens its grandparent",
		file:   "oracle/parser/builder.go",
		old:    "\tif n > 1 {\n\t\tb.stack[n-2].cover(fr.offset, fr.end)\n\t}",
		new:    "\tif n > 1 {\n\t\tb.stack[max(n-3, 0)].cover(fr.offset, fr.end)\n\t}",
		expect: "TestRandomShapes",
	},
	{
		// Undersized rather than oversized, for the same reason: too *large* a size walks Children
		// off the end of the arena, and the index-out-of-range ends the binary before the arena
		// invariant can report anything.
		name:   "a subtree claims one node too few",
		file:   "oracle/parser/builder.go",
		old:    `	d.size = uint32(len(b.tree.nodes)) - uint32(fr.id)`,
		new:    `	d.size = uint32(len(b.tree.nodes)) - uint32(fr.id) - 1`,
		expect: "TestGoldenCorpusInvariants",
	},
	{
		name:   "the cursor does not follow the last leaf",
		file:   "oracle/parser/builder.go",
		old:    "\tb.stack[n-1].cover(offset, end)\n\tb.pos = end",
		new:    "\tb.stack[n-1].cover(offset, end)",
		expect: "TestBuildZeroWidthLeaf",
	},
	{
		name:   "the builder accepts an empty node",
		file:   "oracle/parser/builder.go",
		old:    "\tif !fr.filled {\n\t\tpanic(fmt.Sprintf(\"parser: event %d closes %s with no children",
		new:    "\tif false {\n\t\tpanic(fmt.Sprintf(\"parser: event %d closes %s with no children",
		expect: "TestBuildRejects",
	},
	{
		name:   "the root's close is not noticed",
		file:   "oracle/parser/builder.go",
		old:    "\tif n == 1 {\n\t\tb.done = true\n\t}",
		new:    "\tif false {\n\t\tb.done = true\n\t}",
		expect: "TestBuildRejects",
	},
	{
		name:   "a leaf is parented to the root rather than to its node",
		file:   "oracle/parser/builder.go",
		old:    "\t\tparent: b.stack[n-1].id,\n\t\tsize:   1,",
		new:    "\t\tparent: 0,\n\t\tsize:   1,",
		expect: "TestGoldenCorpusInvariants",
	},

	// --- the kind predicates and the tree API ---
	{
		name:   "trivia may be synthesised",
		file:   "oracle/parser/kind.go",
		old:    `	return k == Error || (k.IsToken() && k != Unset && !isTrivia(k))`,
		new:    `	return k == Error || (k.IsToken() && k != Unset)`,
		expect: "TestBuildRejects",
	},
	{
		name:   "a token kind may be opened",
		file:   "oracle/parser/kind.go",
		old:    `func isNode(k Kind) bool { return !k.IsToken() && k <= Error }`,
		new:    `func isNode(k Kind) bool { return k <= Error }`,
		expect: "TestBuildRejects",
	},
	{
		name:   "nothing is trivia",
		file:   "oracle/parser/kind.go",
		old:    `func isTrivia(k Kind) bool { return k.IsToken() && token.Kind(k).IsTrivia() }`,
		new:    `func isTrivia(k Kind) bool { return false }`,
		expect: "TestGoldens",
	},
	{
		name:   "children are walked one node at a time",
		file:   "oracle/parser/tree.go",
		old:    `	for i := n.id + 1; i < n.id+NodeID(d.size); i += NodeID(n.t.nodes[i].size) {`,
		new:    `	for i := n.id + 1; i < n.id+NodeID(d.size); i++ {`,
		expect: "TestTreeNavigation",
	},
	{
		name:   "a node's text stops one byte short",
		file:   "oracle/parser/tree.go",
		old:    `	return n.t.src.Slice(offset, end-offset)`,
		new:    `	return n.t.src.Slice(offset, max(end-offset-1, 0))`,
		expect: "TestTreeLeavesTileTheSource",
	},
	{
		name:   "the golden renderer keeps trivia",
		file:   "oracle/parser/golden_render.go",
		old:    "\t\tif isTrivia(kid.Kind()) {\n\t\t\tcontinue\n\t\t}",
		new:    "\t\tif false {\n\t\t\tcontinue\n\t\t}",
		expect: "TestGoldens",
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

	// Only from here on does anything get written, so this is where the tree becomes
	// something that has to be put back.
	watchSignals()

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
		case interrupted:
			fmt.Println("not run — interrupted")
		}
		if sig, yes := stopping(); yes {
			// Leave here rather than falling through to the summary. An interrupted run
			// has established nothing, and printing "0 survived" over the handful that
			// did run would report a pass for work that was never done — the same
			// fail-open check.sh's --count=1 and missing-tool rules exist to refuse.
			fmt.Fprintf(os.Stderr, "\nmutate: interrupted after %d of %d; nothing is concluded\n",
				i+1, len(selected))
			os.Exit(128 + int(sig))
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
	interrupted
)

// pending is the file this tool has rewritten and not yet put back, held so that a path
// which cannot run a defer can still restore it. Guarded because the signal handler reads
// it from its own goroutine while run writes it from main's.
//
// One mutant is live at a time — run is sequential, deliberately, since two mutants at once
// would each be tested against the other's defect — so a single slot is the whole state.
//
// stopping is the half that is easy to leave out, and the test caught its absence: restoring
// on a signal is useless on its own, because main is still running and will apply the *next*
// mutant a moment later, after the handler has already tidied up. The flag is what makes the
// restore final.
var pending struct {
	sync.Mutex
	path     string
	original []byte
	sig      syscall.Signal // non-zero once a signal has been seen
}

// apply records the file and writes the mutation, both under one lock, so an interrupt can
// only land wholly before or wholly after — never in the window between a write and the
// note saying it happened.
//
// It reports false once a signal has been seen, which is how the loop learns to stop.
func apply(path string, original []byte, mutated string) (bool, error) {
	pending.Lock()
	defer pending.Unlock()
	if pending.sig != 0 {
		return false, nil
	}
	pending.path, pending.original = path, original
	if err := os.WriteFile(path, []byte(mutated), 0o644); err != nil {
		// A partial write is exactly what the original is kept for.
		_ = os.WriteFile(path, original, 0o644)
		pending.path, pending.original = "", nil
		return false, err
	}
	return true, nil
}

// disarm puts the file back, if it is still out. Idempotent, because the ordinary return
// path and the signal path both call it and either may come first. A failed write is
// swallowed: this runs during shutdown, where there is nothing useful left to do with an
// error and reporting it would only delay the restore.
func disarm() {
	pending.Lock()
	defer pending.Unlock()
	restoreLocked()
}

// stop marks the run as ending and puts back whatever is out. After this, apply refuses,
// so the tree stays as it was found no matter how long the process takes to die.
func stop(s syscall.Signal) {
	pending.Lock()
	defer pending.Unlock()
	pending.sig = s
	restoreLocked()
}

func restoreLocked() {
	if pending.path == "" {
		return
	}
	_ = os.WriteFile(pending.path, pending.original, 0o644)
	pending.path, pending.original = "", nil
}

// stopping reports the signal seen, if any, so the loop can leave promptly rather than
// running the suite once per remaining mutant with nothing mutated — and so it can exit
// with the status that signal deserves.
func stopping() (syscall.Signal, bool) {
	pending.Lock()
	defer pending.Unlock()
	return pending.sig, pending.sig != 0
}

// watchSignals restores the live mutant when the run is interrupted, then re-raises.
//
// This exists because a **terminating signal runs no deferred function**, and Ctrl-C is the
// likely interruption: mutation is the minutes-long step of check.sh, so it is the one a
// person actually stops. Without this, the mutant applied at that moment stays applied — a
// checked-in source file quietly holding a deliberate defect, which is the single failure
// this tool must never have. (It has happened: `source.go`'s BOM check was found reading
// `if false`, and the three failures it caused looked like a design bug rather than a
// leftover.)
//
// Restoring is only half of it, and the half that is easy to mistake for the whole: this
// was written first as a bare restore, and the test caught main calmly applying the *next*
// mutant a moment later, after the handler had already tidied up. stop() sets the flag that
// makes the restore final.
//
// The exit status stays honest from both directions: main leaves with 128+signal as soon as
// it is between mutants, and this goroutine re-raises for the case where it is not. A run
// stopped by Ctrl-C must not look like one that finished and found nothing.
func watchSignals() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	go func() {
		s := <-sigs
		sig, ok := s.(syscall.Signal)
		if !ok {
			sig = syscall.SIGINT
		}
		stop(sig)
		fmt.Fprintf(os.Stderr, "\nmutate: interrupted; the tree is back as it was\n")

		// main leaves by the same status as soon as it notices, which is immediately
		// between mutants. This path is for when it cannot: it is inside a `go test` that
		// may take the full timeout to return. Re-raise so the wait is bounded by the
		// signal rather than by the suite.
		signal.Reset(s)
		_ = syscall.Kill(os.Getpid(), sig)
		time.Sleep(5 * time.Second)
		os.Exit(128 + int(sig))
	}()
}

// run applies one mutant, runs the suite, and puts the file back.
//
// The original is held in memory and restored on every path out: a defer for the ordinary
// and panicking ones, an explicit disarm before each os.Exit (which skips defers), and
// watchSignals for interruption (which skips them too). This rewrites checked-in source,
// and that is the one thing it must never get wrong.
func run(root string, m mutant) (outcome, []string) {
	path := filepath.Join(root, m.file)
	original, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nmutate: %v\n", err)
		os.Exit(1)
	}

	// Exactly one occurrence, or the mutation is ambiguous — and an ambiguous one applied
	// somewhere unreached is indistinguishable from a weak suite. Checked before arming,
	// because nothing has been written yet and there is nothing to put back.
	if n := strings.Count(string(original), m.old); n != 1 {
		fmt.Fprintf(os.Stderr, "\nmutate: %q matches %d times in %s, want 1\n", m.name, n, m.file)
		os.Exit(1)
	}

	defer disarm()

	mutated := strings.Replace(string(original), m.old, m.new, 1)
	switch applied, err := apply(path, original, mutated); {
	case err != nil:
		fmt.Fprintf(os.Stderr, "\nmutate: %v\n", err)
		os.Exit(1)
	case !applied:
		// Interrupted between mutants. Nothing was written, so there is nothing to judge.
		return interrupted, nil
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
