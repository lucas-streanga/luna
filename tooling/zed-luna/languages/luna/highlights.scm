; Generated from keywords.md (R32-R47). Keyword classification via #match?,
; the standard technique for lexical-grade grammars.

(comment) @comment
(string) @string
(command) @string.special
(attribute) @attribute
(number) @number
(punctuation) @operator

((identifier) @keyword
 (#match? @keyword "^(var|let|const|fn|constraint|proto|enum|error|capability|attribute|meta|export|import|test)$"))

((identifier) @keyword.control
 (#match? @keyword.control "^(if|else|foreach|while|in|break|continue|return|yield|match|where|defer|try|catch|throw|by)$"))

((identifier) @keyword.operator
 (#match? @keyword.operator "^(copy|spawn|await|comptime|comptype|is|as|apply|declared|use)$"))

((identifier) @type
 (#match? @type "^(int|double|bool|string|bytes|table|list|stream|promise|view|never|any|regex|command|type|byte|number|json|csv|yaml|xml|path|file|secret|panic)!?$"))

((identifier) @constant.builtin
 (#match? @constant.builtin "^(true|false|null|undefined|self)$"))

((identifier) @variable
 (#match? @variable "^_$"))
