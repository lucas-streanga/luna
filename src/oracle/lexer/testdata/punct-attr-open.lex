#[jsonTag('id')]
const currentId: string;
---
ATTR_OPEN 0..2 "#["
IDENT 2..9 "jsonTag"
LPAREN 9..10 "("
STRING_SQ 10..14 "'id'"
RPAREN 14..15 ")"
RBRACKET 15..16 "]"
WHITESPACE 16..17 "\n"
KW_CONST 17..22 "const"
WHITESPACE 22..23 " "
IDENT 23..32 "currentId"
COLON 32..33 ":"
WHITESPACE 33..34 " "
IDENT 34..40 "string"
SEMICOLON 40..41 ";"
WHITESPACE 41..42 "\n"
