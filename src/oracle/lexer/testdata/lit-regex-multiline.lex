let r = ~"
  \d{4}  # year
"x;
let b = 1;
---
KW_LET 0..3 "let"
WHITESPACE 3..4 " "
IDENT 4..5 "r"
WHITESPACE 5..6 " "
ASSIGN 6..7 "="
WHITESPACE 7..8 " "
REGEX 8..29 "~\"\n  \\d{4}  # year\n\"x"
SEMICOLON 29..30 ";"
WHITESPACE 30..31 "\n"
KW_LET 31..34 "let"
WHITESPACE 34..35 " "
IDENT 35..36 "b"
WHITESPACE 36..37 " "
ASSIGN 37..38 "="
WHITESPACE 38..39 " "
INT_DEC 39..40 "1"
SEMICOLON 40..41 ";"
WHITESPACE 41..42 "\n"
