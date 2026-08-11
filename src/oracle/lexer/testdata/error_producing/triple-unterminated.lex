let s = """
    a
let t = 1;
---
KW_LET 0..3 "let"
WHITESPACE 3..4 " "
IDENT 4..5 "s"
WHITESPACE 5..6 " "
ASSIGN 6..7 "="
WHITESPACE 7..8 " "
TRIPLE_DQ_OPEN 8..12 "\"\"\"\n"
!L0009 8..11
DQ_TEXT 12..17 "    a"
DQ_TEXT 17..18 "\n"
DQ_TEXT 18..28 "let t = 1;"
DQ_TEXT 28..29 "\n"
