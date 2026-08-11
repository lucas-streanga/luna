`ls -la` `a\`b` ``
---
COMMAND 0..8 "`ls -la`"
WHITESPACE 8..9 " "
COMMAND 9..15 "`a\\`b`"
WHITESPACE 15..16 " "
COMMAND 16..18 "``"
WHITESPACE 18..19 "\n"
