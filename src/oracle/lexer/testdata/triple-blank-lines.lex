let s = """
    a
  
        

    b
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
DQ_TEXT 17..18 "\n"
WHITESPACE 18..20 "  "
DQ_TEXT 20..21 "\n"
MARGIN 21..25 "    "
WHITESPACE 25..29 "    "
DQ_TEXT 29..30 "\n"
DQ_TEXT 30..31 "\n"
MARGIN 31..35 "    "
DQ_TEXT 35..36 "b"
TRIPLE_DQ_CLOSE 36..44 "\n    \"\"\""
SEMICOLON 44..45 ";"
WHITESPACE 45..46 "\n"
