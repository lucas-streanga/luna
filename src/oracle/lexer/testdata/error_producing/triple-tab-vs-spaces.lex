let s = """
	ab
    """;
---
KW_LET 0..3 "let"
WHITESPACE 3..4 " "
IDENT 4..5 "s"
WHITESPACE 5..6 " "
ASSIGN 6..7 "="
WHITESPACE 7..8 " "
TRIPLE_DQ_OPEN 8..12 "\"\"\"\n"
DQ_TEXT 12..15 "\tab"
!L0014 12..13
TRIPLE_DQ_CLOSE 15..23 "\n    \"\"\""
SEMICOLON 23..24 ";"
WHITESPACE 24..25 "\n"
