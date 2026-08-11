~"\d+"im ~"" ~"a\"b"x
---
REGEX 0..8 "~\"\\d+\"im"
WHITESPACE 8..9 " "
REGEX 9..12 "~\"\""
WHITESPACE 12..13 " "
REGEX 13..21 "~\"a\\\"b\"x"
WHITESPACE 21..22 "\n"
