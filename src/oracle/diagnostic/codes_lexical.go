package diagnostic

// The lexical diagnostics — lexer §11's table, one constant per row, in one file so
// it can be diffed against the spec at a glance.
//
// Two are raised at ingress by package source, before any tokenizing; the other ten
// by the lexer. They live together anyway, because §11 is one table and splitting the
// series by which package happens to raise it would defeat the point of the file.
//
// Numbering is append-only and never reused (R240). A retired check's code stays
// retired: search results outlive the check that prompted them.
const (
	// Ingress, before tokenizing (lexical-structure §1).
	InvalidUTF8   Code = "L0001"
	ByteOrderMark Code = "L0002"

	// The lexer proper.
	LeadingZero               Code = "L0003" // §0, R238
	UppercaseRadixPrefix      Code = "L0004" // §0, R238
	UnknownEscape             Code = "L0005" // string §5.1, R150
	InvalidCodepointEscape    Code = "L0006" // string §5.1, R150
	UnexpectedHash            Code = "L0007" // §5, R85
	UnexpectedTilde           Code = "L0008" // §5, R237
	UnterminatedLiteral       Code = "L0009" // §1
	UnterminatedBlockComment  Code = "L0010" // §2, F4
	UnterminatedInterpolation Code = "L0011" // §6
	UnexpectedCharacter       Code = "L0012" // §0
	MalformedCodepointEscape  Code = "L0013" // §0, string §5.1, R245
	MalformedByteEscape       Code = "L0016" // bytes §7, string §5.1, R248
	InsufficientIndentation   Code = "L0014" // §4, R246
	ContentAfterTripleOpen    Code = "L0015" // §4, R246
)

// lexicalTitles is the title fixed to each lexical code.
//
// A title is part of a code's identity (R240) — it is what `luna explain L0003`
// prints, and unlike a description it does not vary per instance. These are §11's
// Title column verbatim, and a test pins them against it.
var lexicalTitles = map[Code]string{
	InvalidUTF8:               "Invalid UTF-8",
	ByteOrderMark:             "Byte-order mark",
	LeadingZero:               "Leading zero",
	UppercaseRadixPrefix:      "Uppercase radix prefix",
	UnknownEscape:             "Unknown escape",
	InvalidCodepointEscape:    "Invalid codepoint escape",
	UnexpectedHash:            "Unexpected `#`",
	UnexpectedTilde:           "Unexpected `~`",
	UnterminatedLiteral:       "Unterminated literal",
	UnterminatedBlockComment:  "Unterminated block comment",
	UnterminatedInterpolation: "Unterminated interpolation",
	UnexpectedCharacter:       "Unexpected character",
	MalformedCodepointEscape:  "Malformed codepoint escape",
	InsufficientIndentation:   "Insufficient indentation",
	ContentAfterTripleOpen:    "Content after a multi-line opener",
	MalformedByteEscape:       "Malformed byte escape",
}
