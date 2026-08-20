// Package source is the ingress and position layer: bytes on disk become a validated
// buffer the lexer may scan, and a byte offset becomes a location a person can read.
//
// Validity is established once, up front (lexical-structure §1), so nothing
// downstream re-checks: the lexer matches ASCII bytes precisely because the rest has
// already been proven well-formed UTF-8. Two conditions are rejected here: invalid
// UTF-8 (L0001), and a leading byte-order mark (L0002), refused rather than stripped
// so byte offset 0 stays meaningful for the shebang rule.
//
// Positions are computed, never stored (lexer §9, R236): tokens carry byte spans into
// the retained text, and line and rune column derive on demand from a lazily built
// line-start table.
package source

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"luna/oracle/diagnostic"
)

// Error is an ingress failure: the code that names it and the offset it was found at.
//
// It is not a diagnostic.Diagnostic, because at ingress there is no *File to point a
// span at, New rejecting text before it returns one. The caller converts, supplying the
// file name it already has. The dependency runs one way, source to diagnostic, which
// is what keeps diagnostic a leaf and lets a renderer sit above both.
type Error struct {
	Code   diagnostic.Code
	Offset int
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s at byte %d", e.Code, e.Offset)
}

// Position is a human-facing location: 1-based line, 1-based **rune** column. A tab
// is one column (R236); rendering a caret under it is the renderer's problem, not a
// question this type answers.
type Position struct {
	Line   int
	Column int
}

// File is a validated source file.
type File struct {
	name  string
	text  string
	ascii bool

	// The line index is built on first use: §9 wants a compile that emits no
	// diagnostic to build no table. Guarded, because compiler §1.1 lexes files in
	// parallel and a *File can be reached both from its own lexer and from whatever
	// later renders a collected diagnostic. -race would catch the unguarded version
	// (testing-strategy §7), but sync.Once costs one atomic load after the first call
	// which is cheaper than finding out.
	once  sync.Once
	lines []int32 // byte offset where each line starts
}

// New validates in-memory text and returns the file it describes. Text that is not
// valid UTF-8, or that begins with a byte-order mark, is rejected with an *Error.
func New(name, text string) (*File, error) {
	// The BOM is checked first so that a file which is both BOM-led and later
	// malformed reports the earlier problem.
	if strings.HasPrefix(text, "\ufeff") {
		return nil, &Error{Code: diagnostic.ByteOrderMark, Offset: 0}
	}
	ascii, bad, ok := scan(text)
	if !ok {
		return nil, &Error{Code: diagnostic.InvalidUTF8, Offset: bad}
	}
	return &File{name: name, text: text, ascii: ascii}, nil
}

// Load reads and validates a file from disk. An IO failure is an ordinary error; only
// a lexical condition is reported as an *Error.
func Load(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("source: %w", err)
	}
	return New(path, string(b))
}

// scan makes the single pass lexical-structure §1 describes: it proves the text is
// valid UTF-8 and, because it is visiting every byte anyway, records whether any byte
// was non-ASCII. On failure bad is the offset of the first byte that broke validity:
// the diagnostic points at where the text stopped being well-formed, not at the file.
func scan(text string) (ascii bool, bad int, ok bool) {
	ascii = true
	for i := 0; i < len(text); {
		if text[i] < utf8.RuneSelf {
			i++
			continue
		}
		ascii = false
		// DecodeRuneInString reports (RuneError, 1) for a byte that begins no valid
		// sequence, and (RuneError, 3) for a correctly encoded U+FFFD, so the size is
		// what separates a real replacement character from a malformed one.
		r, size := utf8.DecodeRuneInString(text[i:])
		if r == utf8.RuneError && size <= 1 {
			return ascii, i, false
		}
		i += size
	}
	return ascii, 0, true
}

// Name is the file's identity, which R240 makes part of every span.
func (f *File) Name() string { return f.name }

// Text is the whole validated buffer.
func (f *File) Text() string { return f.text }

// IsASCII reports whether the file contained no byte >= 0x80. Recorded free during
// validation, and what lets Position compute a rune column by subtraction.
func (f *File) IsASCII() bool { return f.ascii }

// Slice returns the text of a span. This is how a lexeme is recovered: tokens carry
// spans, not copies.
func (f *File) Slice(offset, length int) string { return f.text[offset : offset+length] }

// Position resolves a byte offset to a line and rune column. Offsets in [0, len] are
// valid; len itself is the end-of-file position. An offset outside that range is a
// caller bug, not a diagnostic, and panics.
func (f *File) Position(offset int) Position {
	if offset < 0 || offset > len(f.text) {
		panic(diagnostic.Bugf("source: offset %d out of range [0, %d] in %s", offset, len(f.text), f.name))
	}
	f.once.Do(f.buildLines)

	// The last line start at or before the offset. lines[0] is 0 and offset is
	// non-negative, so this never selects -1.
	i := sort.Search(len(f.lines), func(i int) bool { return f.lines[i] > int32(offset) }) - 1
	start := int(f.lines[i])

	if f.ascii {
		return Position{Line: i + 1, Column: offset - start + 1}
	}
	return Position{Line: i + 1, Column: utf8.RuneCountInString(f.text[start:offset]) + 1}
}

// buildLines records the offset each line begins at. Keying only on '\n' is what lets
// CRLF need no special case: the '\r' stays ordinary content on the line it ends. A
// trailing newline yields a final entry at len(text), which is the empty last line
// every editor shows.
func (f *File) buildLines() {
	lines := make([]int32, 1, 1+strings.Count(f.text, "\n"))
	for i := 0; i < len(f.text); i++ {
		if f.text[i] == '\n' {
			lines = append(lines, int32(i+1))
		}
	}
	f.lines = lines
}
