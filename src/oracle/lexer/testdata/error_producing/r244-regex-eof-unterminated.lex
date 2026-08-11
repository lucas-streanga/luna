let r = ~"abc
---
KW_LET 0..3 "let"
WHITESPACE 3..4 " "
IDENT 4..5 "r"
WHITESPACE 5..6 " "
ASSIGN 6..7 "="
WHITESPACE 7..8 " "
INVALID 8..14 "~\"abc\n"
!L0009 8..10
