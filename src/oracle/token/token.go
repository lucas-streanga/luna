package token

// Token is one lexeme the lexer emitted: which kind it is, and where it sits.
//
// The text is not here. Tokens carry byte spans into the retained source (lexer §9),
// so a lexeme is recovered with source.File.Slice rather than copied per token —
// which is also what lets a trivia token's exact bytes reach the formatter (R236).
//
// Offset and Len are the span. Because trivia are emitted, a file's tokens tile its
// bytes exactly: no gaps, no overlaps, summing to the file's length. That invariant
// is the strongest property the lexer's fuzz suite has, and it holds on invalid input
// too — every byte either begins a token or raises L0012.
type Token struct {
	Kind   Kind
	Offset int
	Len    int
}

// End is the offset one past the token, so a token spans [Offset, End).
func (t Token) End() int { return t.Offset + t.Len }

// IsTrivia reports whether the token is whitespace, a comment, or the shebang — the
// four kinds §2 emits and every consumer but the formatter drops.
func (t Token) IsTrivia() bool { return t.Kind.IsTrivia() }
