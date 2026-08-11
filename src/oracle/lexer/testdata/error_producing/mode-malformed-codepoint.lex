// `\u{}` is malformed, so R245 splits it into ESCAPE_PAIR + DQ_TEXT and R248's
// stage-two check raises L0013 over the `\u` alone — the span R245 arranged for.
let s = "a\u{}b${x}";
---
LINE_COMMENT 0..79 "// `\\u{}` is malformed, so R245 splits it into ESCAPE_PAIR + DQ_TEXT and R248's"
WHITESPACE 79..80 "\n"
LINE_COMMENT 80..163 "// stage-two check raises L0013 over the `\\u` alone — the span R245 arranged for."
WHITESPACE 163..164 "\n"
KW_LET 164..167 "let"
WHITESPACE 167..168 " "
IDENT 168..169 "s"
WHITESPACE 169..170 " "
ASSIGN 170..171 "="
WHITESPACE 171..172 " "
DQ_OPEN 172..173 "\""
DQ_TEXT 173..174 "a"
ESCAPE_PAIR 174..176 "\\u"
!L0013 174..176
DQ_TEXT 176..179 "{}b"
INTERP_OPEN 179..181 "${"
IDENT 181..182 "x"
INTERP_CLOSE 182..183 "}"
DQ_CLOSE 183..184 "\""
SEMICOLON 184..185 ";"
WHITESPACE 185..186 "\n"
