let s = """oops
    """;
---
KW_LET 0..3 "let"
WHITESPACE 3..4 " "
IDENT 4..5 "s"
WHITESPACE 5..6 " "
ASSIGN 6..7 "="
WHITESPACE 7..8 " "
TRIPLE_DQ_OPEN 8..16 "\"\"\"oops\n"
!L0015 11..12
TRIPLE_DQ_CLOSE 16..23 "    \"\"\""
SEMICOLON 23..24 ";"
WHITESPACE 24..25 "\n"
