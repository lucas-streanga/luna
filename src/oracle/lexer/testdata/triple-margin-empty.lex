let s = """
no margin at all
second line
""";
---
KW_LET 0..3 "let"
WHITESPACE 3..4 " "
IDENT 4..5 "s"
WHITESPACE 5..6 " "
ASSIGN 6..7 "="
WHITESPACE 7..8 " "
TRIPLE_DQ_OPEN 8..12 "\"\"\"\n"
DQ_TEXT 12..28 "no margin at all"
DQ_TEXT 28..29 "\n"
DQ_TEXT 29..40 "second line"
TRIPLE_DQ_CLOSE 40..44 "\n\"\"\""
SEMICOLON 44..45 ";"
WHITESPACE 45..46 "\n"
