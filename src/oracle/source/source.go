// Package source is the ingress and position layer: bytes on disk become a validated
// buffer the lexer may scan, and a byte offset becomes a location a person can read.
//
// Validity is established once, up front (lexical-structure §1), so nothing
// downstream re-checks — the lexer matches ASCII bytes precisely because the rest has
// already been proven well-formed UTF-8. Two conditions are rejected here: invalid
// UTF-8 (L0001), and a leading byte-order mark (L0002), refused rather than stripped
// so byte offset 0 stays meaningful for the shebang rule.
//
// Positions are computed, never stored (lexer §9, R236): tokens carry byte spans into
// the retained text, and line and rune column derive on demand from a lazily built
// line-start table.
//
// NOTE: declarations only. Bodies are unimplemented — the tests come first.
package source

import "fmt"

// Diagnostic codes this package raises (lexer §11).
const (
	CodeInvalidUTF8   = "L0001"
	CodeByteOrderMark = "L0002"
)

// Error is an ingress failure, carrying the code that names it and the offset it was
// found at. It deliberately does not depend on a diagnostic package: rendering a
// diagnostic needs the source line to draw a caret under, so a renderer must sit
// above both, and source must stay a leaf.
type Error struct {
	Code   string
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
	lines []int32 // lazily built; nil until the first Position call
}

// New validates in-memory text and returns the file it describes. Text that is not
// valid UTF-8, or that begins with a byte-order mark, is rejected with an *Error.
func New(name, text string) (*File, error) {
	panic("unimplemented")
}

// Load reads and validates a file from disk. An IO failure is an ordinary error; only
// a lexical condition is reported as an *Error.
func Load(path string) (*File, error) {
	panic("unimplemented")
}

// Name is the file's identity, which R240 makes part of every span.
func (f *File) Name() string { panic("unimplemented") }

// Text is the whole validated buffer.
func (f *File) Text() string { panic("unimplemented") }

// IsASCII reports whether the file contained no byte >= 0x80. Recorded free during
// validation, and what lets Position compute a rune column by subtraction.
func (f *File) IsASCII() bool { panic("unimplemented") }

// Slice returns the text of a span. This is how a lexeme is recovered: tokens carry
// spans, not copies.
func (f *File) Slice(offset, length int) string { panic("unimplemented") }

// Position resolves a byte offset to a line and rune column. Offsets in [0, len] are
// valid; len itself is the end-of-file position. An offset outside that range is a
// caller bug, not a diagnostic, and panics.
func (f *File) Position(offset int) Position { panic("unimplemented") }
