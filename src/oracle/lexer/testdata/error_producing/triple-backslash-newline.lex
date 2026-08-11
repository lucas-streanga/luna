let s = """
    a\
    """;
---
KW_LET 0..3 "let"
WHITESPACE 3..4 " "
IDENT 4..5 "s"
WHITESPACE 5..6 " "
ASSIGN 6..7 "="
WHITESPACE 7..8 " "
TRIPLE_DQ_OPEN 8..12 "\"\"\"\n"
MARGIN 12..16 "    "
DQ_TEXT 16..17 "a"
ESCAPE_PAIR 17..19 "\\\n"
!L0005 17..19
TRIPLE_DQ_CLOSE 19..26 "    \"\"\""
SEMICOLON 26..27 ";"
WHITESPACE 27..28 "\n"
