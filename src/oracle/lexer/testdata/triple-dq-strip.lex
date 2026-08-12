let s = """
    trailing spaces stripped   
    kept via escape\u{20}\u{20}
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
DQ_TEXT 16..40 "trailing spaces stripped"
WHITESPACE 40..43 "   "
DQ_TEXT 43..44 "\n"
MARGIN 44..48 "    "
DQ_TEXT 48..63 "kept via escape"
ESCAPE_PAIR 63..69 "\\u{20}"
ESCAPE_PAIR 69..75 "\\u{20}"
TRIPLE_DQ_CLOSE 75..83 "\n    \"\"\""
SEMICOLON 83..84 ";"
WHITESPACE 84..85 "\n"
