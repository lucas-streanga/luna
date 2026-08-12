let s = """
    a """ b ''' c
    """;
let t = '''
    raw \n $x """ and ''' mid-line
    ''';
---
KW_LET 0..3 "let"
WHITESPACE 3..4 " "
IDENT 4..5 "s"
WHITESPACE 5..6 " "
ASSIGN 6..7 "="
WHITESPACE 7..8 " "
TRIPLE_DQ_OPEN 8..12 "\"\"\"\n"
MARGIN 12..16 "    "
DQ_TEXT 16..29 "a \"\"\" b ''' c"
TRIPLE_DQ_CLOSE 29..37 "\n    \"\"\""
SEMICOLON 37..38 ";"
WHITESPACE 38..39 "\n"
KW_LET 39..42 "let"
WHITESPACE 42..43 " "
IDENT 43..44 "t"
WHITESPACE 44..45 " "
ASSIGN 45..46 "="
WHITESPACE 46..47 " "
TRIPLE_SQ_OPEN 47..51 "'''\n"
MARGIN 51..55 "    "
RAW_TEXT 55..85 "raw \\n $x \"\"\" and ''' mid-line"
TRIPLE_SQ_CLOSE 85..93 "\n    '''"
SEMICOLON 93..94 ";"
WHITESPACE 94..95 "\n"
