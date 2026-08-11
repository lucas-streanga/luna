// No L0013 below: R245 makes `\u{}` split into ESCAPE_PAIR + DQ_TEXT, which is
// the tokenization this pins, but which phase validates escapes is open (R243).
let s = "a\u{}b${x}";
---
LINE_COMMENT 0..79 "// No L0013 below: R245 makes `\\u{}` split into ESCAPE_PAIR + DQ_TEXT, which is"
WHITESPACE 79..80 "\n"
LINE_COMMENT 80..160 "// the tokenization this pins, but which phase validates escapes is open (R243)."
WHITESPACE 160..161 "\n"
KW_LET 161..164 "let"
WHITESPACE 164..165 " "
IDENT 165..166 "s"
WHITESPACE 166..167 " "
ASSIGN 167..168 "="
WHITESPACE 168..169 " "
DQ_OPEN 169..170 "\""
DQ_TEXT 170..171 "a"
ESCAPE_PAIR 171..173 "\\u"
DQ_TEXT 173..176 "{}b"
INTERP_OPEN 176..178 "${"
IDENT 178..179 "x"
INTERP_CLOSE 179..180 "}"
DQ_CLOSE 180..181 "\""
SEMICOLON 181..182 ";"
WHITESPACE 182..183 "\n"
