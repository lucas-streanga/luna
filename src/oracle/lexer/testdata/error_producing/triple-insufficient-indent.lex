let s = """
    ok
 hi!
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
DQ_TEXT 16..18 "ok"
DQ_TEXT 18..19 "\n"
DQ_TEXT 19..23 " hi!"
!L0014 19..20
TRIPLE_DQ_CLOSE 23..31 "\n    \"\"\""
SEMICOLON 31..32 ";"
WHITESPACE 32..33 "\n"
