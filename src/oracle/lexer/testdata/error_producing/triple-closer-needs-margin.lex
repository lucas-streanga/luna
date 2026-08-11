// The `"""` here sits at the margin's offset but the line is not indented by the margin,
// so it is content and L0014 — not a closer. The next line closes.
let s = """
    a
nope"""x
    """;
---
LINE_COMMENT 0..89 "// The `\"\"\"` here sits at the margin's offset but the line is not indented by the margin,"
WHITESPACE 89..90 "\n"
LINE_COMMENT 90..159 "// so it is content and L0014 — not a closer. The next line closes."
WHITESPACE 159..160 "\n"
KW_LET 160..163 "let"
WHITESPACE 163..164 " "
IDENT 164..165 "s"
WHITESPACE 165..166 " "
ASSIGN 166..167 "="
WHITESPACE 167..168 " "
TRIPLE_DQ_OPEN 168..172 "\"\"\"\n"
MARGIN 172..176 "    "
DQ_TEXT 176..177 "a"
DQ_TEXT 177..178 "\n"
DQ_TEXT 178..186 "nope\"\"\"x"
!L0014 178..179
TRIPLE_DQ_CLOSE 186..194 "\n    \"\"\""
SEMICOLON 194..195 ";"
WHITESPACE 195..196 "\n"
