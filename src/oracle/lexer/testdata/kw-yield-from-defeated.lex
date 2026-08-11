yield /*c*/ from;
yield

from;
---
KW_YIELD 0..5 "yield"
WHITESPACE 5..6 " "
BLOCK_COMMENT 6..11 "/*c*/"
WHITESPACE 11..12 " "
IDENT 12..16 "from"
SEMICOLON 16..17 ";"
WHITESPACE 17..18 "\n"
KW_YIELD_FROM 18..29 "yield\n\nfrom"
SEMICOLON 29..30 ";"
WHITESPACE 30..31 "\n"
