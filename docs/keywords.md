# Keywords

The complete reserved-word inventory, collected from every spec, with each word's governing
document. This is lexer-facing reference: §1–§4 are what the tokenizer reserves, §5 is what
it deliberately does not, §6 is the flag list of words whose definitions need work.

## 1. Declaration keywords

| Keyword | Function | Spec |
|-|-|-|
| `var` | mutable, rebindable binding | variables §1 |
| `let` | fixed binding (never rebinds) | variables §1 |
| `const` | deep-frozen binding | variables §1, §3 |
| `fn` | function literal / function type | functions |
| `constraint` | constraint declaration | constraints |
| `proto` | protocol declaration | protocols |
| `enum` | enum declaration | enum |
| `error` | error declaration **and** the root error type (dual role, position-resolved) | errors §3 |
| `capability` | capability declaration (`const io = capability`) | capabilities §1 |
| `attribute` | attribute declaration | attributes §2 |
| `test` | test declaration (string-literal name, zero params, implicit `undefined!`) | tests |
| `export` | module export (was `pub`, R19) | modules |
| `import` | module import | modules |

## 2. Control-flow keywords

| Keyword | Function | Spec |
|-|-|-|
| `if` / `else` | conditional (block and postfix-modifier forms) | control-flow |
| `foreach` | iteration; binder is **`in`** (`foreach (k => v in xs)`), ruled explicitly, `as` was never the form | control-flow §1 |
| `in` | the `foreach` binder (contextual: only inside `foreach` heads) | control-flow §1 |
| `while` | loop (block and postfix) | control-flow |
| `break` / `continue` | loop exit / next iteration | control-flow |
| `return` | function return | functions |
| `yield` | generator suspension (`yield v`, `yield k => v`); a function containing `yield` is a generator | stream §2 |
| `match` | pattern dispatch (valued and open-ended) | match |
| `where` | guard (match arms) **and** predicate clause (constraints); two homes, one meaning, "holds when"; never part of a type, which is what lets it terminate one | match §3, constraints §1 |
| `defer` | scope-exit statement | defer |
| `by` | range step clause (`lo..hi by s`, contextual: only after a range) | range §3, §4a |
| `try` / `catch` | error recovery expression / block | errors §8 |
| `throw` | raise an error (word prefix, tier 12) | errors §4 |

## 3. Word operators (expression position; associativity §1 tier 12 unless noted)

| Keyword | Function | Spec |
|-|-|-|
| `copy` | deep copy | variables §5.2 |
| `try` | error-to-value recovery (also in §2 as `try`/`catch` blocks) | errors §8 |
| `spawn` | start a task, yielding its `promise` | concurrency |
| `await` | park until a task completes; take the result | await |
| `comptime` | compile-time evaluation | functions §5 |
| `comptype` | declaration descriptor operator **and** its type (dual, like `error`) | reflection §3.2 |
| `is` | value-against-type test (tier 6; the single meaning) | is |
| `as` | checked narrowing (tier 6); also the import alias (`import { parse as jsonParse }`). **Never a binder**: the constraint form is `constraint { i: int where ... }`, not `int as i` (R87) | as, modules §8 |
| `apply` | protocol application: the expression operator (`[] apply P(name: v)`, `@P`-typed, never errorable) **and** the requirement declaration inside a `proto` block (`apply otherProto;`). The dynamic form is the free function `apply()`, an ordinary call, not a keyword use | protocols §4, §7 |
| `declared` | a binding's declared type | type §4 |
| `use` | referential capture, in two positions with one meaning (capabilities flow where `use` names them): the declaration clause on a `fn` / `test` header, and the **call-site delegation clause** (`f(x) use (io)`, R112) | functions §2.2, capabilities §4, §5.2 |

## 4. Value and contextual keywords

| Keyword | Function | Spec |
|-|-|-|
| `true` / `false` | booleans | bool |
| `null` | the chosen nothing | undefined/null |
| `undefined` | the structural absence | undefined |
| `nan` | the IEEE not-a-number double value | double §1, §2.2 |
| `inf` | the IEEE infinity double value (`-inf` is `MINUS KW_INF`; there is no unary `+`) | double §1.1 |
| `self` | contextual, inside a `proto` block: the receiver's type `@CurrentProto` in return-type position, the receiver value in bodies | protocols §2.4 |
| `get` / `set` | contextual, inside `proto` member declarations only: external access grants, canonical order `get set`; a grant that can never be exercised is a definition error | protocols §2.2 |
| `panic` | not a keyword: the lowercase **type** at the root of the sealed panic subtree (like `error`); `catch (p: panic)` is an ordinary typed binder | errors §9, §6 below |
| `_` | not a keyword: the discard identifier (`_ =`, match wildcard, `_: T` type test) | errors §8.1, match, wildcard |

The six **value keywords** (`true`, `false`, `null`, `undefined`, `nan`, `inf`) are **keywords by
lexing and literals by role**, and the distinction is worth stating because it is easy to mistake.
They are not **literal tokens** (lexer §4): a literal token is recognized by a regex over an
open-ended set of lexemes (`42`, `3.14`, `"text"`), while these six are a fixed, finite set of
words that would otherwise lex as `IDENT`. They are not **predeclared identifiers** (§5) either,
and that is load-bearing rather than tidy: a bare identifier in a pattern position is always a
fresh binding (match §2.1, R87), so were `nan` an `IDENT`, the arm `match (x) { nan => ... }`
(double §2.2) would bind rather than match. Reserving them is what keeps them matchable. Type
names, by contrast, never appear bare in a pattern (they follow a `:`), so they stay ordinary
shadowable identifiers (§5) with no hazard.

`undefined` is the one **special** member, and only in what a program may *do* with it, never in
how it lexes: it is language-produced, so a program may compare against it (`x == undefined`, an
`undefined` match arm) but never conjure it as a value (undefined spec). `nan` and `inf` carry no
such restriction, they are ordinary producible doubles (`let x = nan;` is fine), and `true`,
`false`, and `null` likewise.

## 5. Predeclared names, deliberately not keywords

Builtin type names (`int`, `double`, `bool`, `string`, `bytes`, `table`, `list`, `iterable`,
`stream`, `promise`, `never`, `any`, `regex`, `command`, `type`, `byte`, `number`) and builtin
values/functions are **predeclared identifiers**, resolved by ordinary scope, not reserved
words, which keeps the lexer small and the keyword set closed. Whether they are
**shadowable** is flagged (§6). Std exports (`io`, `json`, `path`, `file`, ...) are ordinary
bindings.

## 6. Rulings applied (were flags), and what remains

All seven original flags are ruled (R33):

- **Builtin-name shadowing: shadowable.** Predeclared names occupy an outermost universe
  scope, ordinary scoping applies, no reserved status (modules spec); `let int = 5;` is
  legal and locally self-punishing.
- **`panic` is lowercase, the type itself.** The distinguished sealed subtree root is
  **`panic`**, matching the lowercase root `error`; `catch (p: panic)` is now an ordinary typed
  binder (match §2.2, R87), and the "contextual keyword" entry this file briefly had is gone,
  there is nothing contextual about it. (Follow-up flag, small: the builtin children remain
  PascalCase, `typeError`, `outOfMemory`, while the std families are camelCase, `ioError`,
  `fileNotFound`; the casing convention for builtin error types deserves one ruling.)
- **`implicit` is scratched** (capabilities §6): every capability is explicit, a
  required-but-undeclared `use` is a compile error, no inference tier, no modifier, no
  keyword.
- **`yield` classification is lexical, per function literal**: a literal is a generator iff
  its own body, excluding nested literals, contains `yield`; a nested `fn` with `yield` is
  its own generator (stream §2). No flow analysis.
- **`by` is reserved now** for future stepped-range syntax; rejecting it as an identifier
  today costs nothing and avoids a breaking change (associativity §4, R28).
- **`unsafe` is a camelCase naming prefix** (`unsafeExec`), never a hyphen: `-` is
  subtraction, hyphenated identifiers would make whitespace load-bearing (the disease
  removed with infix `.`), and this is the near-universal answer in infix-minus languages
  (functions §5.6).
- **`in` remains spoken for** by `foreach`; noted against future membership-operator
  proposals.

### Reserved for future use

*(empty; `by` graduated to a keyword in R47)*
