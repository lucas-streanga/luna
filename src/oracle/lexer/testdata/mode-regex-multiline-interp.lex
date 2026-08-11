// A regex that both spans lines and interpolates — the intersection of R244's exemption
// and F2's splice. REGEX_TEXT carries newlines through where every other text run stops.
let r = ~"
  ^${p}\d{4}  # year
  -\d{2}
"xm;
---
LINE_COMMENT 0..90 "// A regex that both spans lines and interpolates — the intersection of R244's exemption"
WHITESPACE 90..91 "\n"
LINE_COMMENT 91..180 "// and F2's splice. REGEX_TEXT carries newlines through where every other text run stops."
WHITESPACE 180..181 "\n"
KW_LET 181..184 "let"
WHITESPACE 184..185 " "
IDENT 185..186 "r"
WHITESPACE 186..187 " "
ASSIGN 187..188 "="
WHITESPACE 188..189 " "
REGEX_OPEN 189..191 "~\""
REGEX_TEXT 191..195 "\n  ^"
INTERP_OPEN 195..197 "${"
IDENT 197..198 "p"
INTERP_CLOSE 198..199 "}"
ESCAPE_PAIR 199..201 "\\d"
REGEX_TEXT 201..216 "{4}  # year\n  -"
ESCAPE_PAIR 216..218 "\\d"
REGEX_TEXT 218..222 "{2}\n"
REGEX_CLOSE 222..225 "\"xm"
SEMICOLON 225..226 ";"
WHITESPACE 226..227 "\n"
