let s = """
		a
		b
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
TRIPLE_DQ_OPEN 8..12 "\"\"\"\n"
MARGIN 12..14 "\t\t"
DQ_TEXT 14..15 "a"
DQ_TEXT 15..16 "\n"
MARGIN 16..18 "\t\t"
DQ_TEXT 18..19 "b"
TRIPLE_DQ_CLOSE 19..25 "\n\t\t\"\"\""
SEMICOLON 25..26 ";"
WHITESPACE 26..27 "\n"
KW_LET 27..30 "let"
WHITESPACE 30..31 " "
IDENT 31..32 "t"
WHITESPACE 32..33 " "
ASSIGN 33..34 "="
WHITESPACE 34..35 " "
TRIPLE_SQ_OPEN 35..39 "'''\n"
MARGIN 39..40 "\t"
RAW_TEXT 40..41 "a"
TRIPLE_SQ_CLOSE 41..46 "\n\t'''"
SEMICOLON 46..47 ";"
WHITESPACE 47..48 "\n"
