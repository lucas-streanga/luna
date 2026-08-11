let a = /* one
two */ 2;
---
KW_LET 0..3 "let"
WHITESPACE 3..4 " "
IDENT 4..5 "a"
WHITESPACE 5..6 " "
ASSIGN 6..7 "="
WHITESPACE 7..8 " "
BLOCK_COMMENT 8..21 "/* one\ntwo */"
WHITESPACE 21..22 " "
INT_DEC 22..23 "2"
SEMICOLON 23..24 ";"
WHITESPACE 24..25 "\n"
