package lexer

import "luna/oracle/escape"

// literalForm is which delimited literal the scanner is reading, and the four facts that
// follow from it: which escapes it admits, whether `${…}` splices, whether `$name` does,
// and whether a raw newline is content or the end of the literal.
//
// One type rather than four parameters, and keyed on the *form* rather than on
// escape.Context, because these came apart once already. `"""` shares `"…"`'s escape table
// (R248) while spanning lines (R246), so deriving line-spanning from the escape context
// was correct until the multi-line forms landed and silently wrong afterwards. Gathered
// here, an impossible combination cannot be named at a call site — which is the whole of
// what the type is for.
type literalForm uint8

const (
	formSq       literalForm = iota // '…'
	formDq                          // "…"
	formTripleDq                    // """…"""
	formBytes                       // b"…" and b'…'
	formCommand                     // `…`
	formRegex                       // ~"…"
)

// rules is what a form decides. Read once and held, never re-derived per byte.
type rules struct {
	escapes escape.Context

	// splices is `${…}`; splicesIdent is the shorter `$name`, which string §5 gives to
	// double-quoted strings alone — in a command or a regex a `$` not followed by `{` is
	// ordinary DOLLAR_TEXT.
	splices      bool
	splicesIdent bool

	// spansLines is R244's rule in its final form: a literal may span lines exactly when
	// its opener is more than one byte.
	spansLines bool
}

// formRules is the table those facts live in. `”'` is deliberately absent: it admits no
// escapes and no interpolation, and lexTripleSq handles its newlines itself, so it would
// be a row nothing reads — the kind of dead entry that misleads whoever scans the table
// next.
var formRules = [...]rules{
	formSq:       {escapes: escape.StringSq},
	formDq:       {escapes: escape.StringDq, splices: true, splicesIdent: true},
	formTripleDq: {escapes: escape.StringDq, splices: true, splicesIdent: true, spansLines: true},
	formBytes:    {escapes: escape.Bytes},
	formCommand:  {escapes: escape.Command, splices: true},
	formRegex:    {escapes: escape.Regex, splices: true, spansLines: true},
}

func (f literalForm) rules() rules { return formRules[f] }

// noun names the form for an unterminated-literal message, which §11 requires to say
// which kind of literal went unclosed.
func (f literalForm) noun() string {
	switch f {
	case formBytes:
		return "bytes"
	case formCommand:
		return "command"
	case formRegex:
		return "regex"
	}
	return "string"
}
