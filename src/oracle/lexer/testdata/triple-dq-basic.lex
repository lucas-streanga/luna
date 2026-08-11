fn query() {
    let sql = """
        SELECT *
        FROM users
        """;
}
---
KW_FN 0..2 "fn"
WHITESPACE 2..3 " "
IDENT 3..8 "query"
LPAREN 8..9 "("
RPAREN 9..10 ")"
WHITESPACE 10..11 " "
LBRACE 11..12 "{"
WHITESPACE 12..17 "\n    "
KW_LET 17..20 "let"
WHITESPACE 20..21 " "
IDENT 21..24 "sql"
WHITESPACE 24..25 " "
ASSIGN 25..26 "="
WHITESPACE 26..27 " "
TRIPLE_DQ_OPEN 27..31 "\"\"\"\n"
MARGIN 31..39 "        "
DQ_TEXT 39..47 "SELECT *"
DQ_TEXT 47..48 "\n"
MARGIN 48..56 "        "
DQ_TEXT 56..66 "FROM users"
TRIPLE_DQ_CLOSE 66..78 "\n        \"\"\""
SEMICOLON 78..79 ";"
WHITESPACE 79..80 "\n"
RBRACE 80..81 "}"
WHITESPACE 81..82 "\n"
