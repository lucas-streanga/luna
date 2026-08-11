let s = """
    a
    """;
let t = '''
    a
    ''';
---
KW_LET 0..3 "let"
WHITESPACE 3..4 " "
IDENT 4..5 "s"
WHITESPACE 5..6 " "
ASSIGN 6..7 "="
WHITESPACE 7..8 " "
TRIPLE_DQ_OPEN 8..13 "\"\"\"\r\n"
MARGIN 13..17 "    "
DQ_TEXT 17..18 "a"
WHITESPACE 18..19 "\r"
TRIPLE_DQ_CLOSE 19..27 "\n    \"\"\""
SEMICOLON 27..28 ";"
WHITESPACE 28..30 "\r\n"
KW_LET 30..33 "let"
WHITESPACE 33..34 " "
IDENT 34..35 "t"
WHITESPACE 35..36 " "
ASSIGN 36..37 "="
WHITESPACE 37..38 " "
TRIPLE_SQ_OPEN 38..43 "'''\r\n"
MARGIN 43..47 "    "
RAW_TEXT 47..49 "a\r"
TRIPLE_SQ_CLOSE 49..57 "\n    '''"
SEMICOLON 57..58 ";"
WHITESPACE 58..60 "\r\n"
