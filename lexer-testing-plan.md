# lexer-testing-plan

The test plan for the oracle's lexer — Phase 0, human-owned, `src/oracle/{token,source,lexer}`
with diagnostics in `src/oracle/diagnostic`. Scoped to lexing alone: the lexer is testable standalone
because compiler §1.1 makes it a complete phase that runs before parsing and consults no symbol
knowledge. Sits under `testing-strategy.md`, which owns the whole-program spine (differential,
metamorphic, fuzz, non-determinism); this file owns one component.

Harness flags are §7's: `go test -race -shuffle=on -count=1 -timeout=<T>`.

---

## 0. Why the lexer is testable now

Five rulings landed between sketching this plan and writing it, and each removed work:

- **R236** made trivia emitted tokens, which promotes *spans tile the source exactly* from a
  partial invariant to a **total** one — and fixed the position model at byte offsets with line
  and rune column computed lazily (lexer §9).
- **R237** replaced bare `/…/` with `~"…"`, **deleting the regex-allowed flag**. That was the
  single hardest surface in this plan: no division-set matrix to enumerate, and no
  swallow-to-next-slash failure mode to hunt.
- **R238** ruled the numeric grammar, so literal tests no longer freeze provisional assumptions.
- **R239** named `DOLLAR_TEXT` and wrote down §6's mode-internal attempt order, which the
  interpolation tests check against.
- **R240** gave diagnostics codes, which is what makes §5's error tests writable at all;
  lexer §11 allocates `L0001`–`L0012`.

Two spec artifacts are the plan's inputs: **§0**, the single token table, and **§10**, the counts.

---

## 1. Inventory pin (build first)

Assert §10's numbers against the implementation: **127 tokens over 131 rows** (R242), and the
per-category counts. Then assert the correspondence both ways — every token constant has a §0
row, every §0 row has a constant. The same shape pins §11's thirteen error codes and their
titles (R240, R245).

Cheapest possible first signal, and it catches a real defect class: R232 fixed a "47 patterns"
claim standing over a 49-row table, which a count assertion would have failed on the day it
appeared. Two of the three error productions and the `DOLLAR_TEXT` gap were both found by
compiling the inventory rather than by reading code.

**These tests read the spec, so the repository carries a `src/specs -> ../specs` symlink and
the reader refuses to walk past `go.mod` without it.** Both halves are load-bearing. `go help
test`: a cached result is reused unless a file the test opened *within the package's module*
has changed — and `specs/` sits outside the module, so reading it by its real path made every
pin here silently cacheable. Edit `lexer.md`, run `go test ./...`, get a stale `ok (cached)`
that verified nothing; confirmed by mutating a §11 title, where the cached run passed and
`-count=1` caught it. The symlink puts the spec inside the module so Go tracks it. The refusal
to walk past the module root is what stops a checkout *without* the symlink from quietly
reverting to the old behaviour — the walk would otherwise find the real directory and every pin
would go on passing against a file the cache ignores, which is the same fail-open wearing a
different hat. `make-archive.sh` passes `zip -y` so the link is archived as a link rather than
resolved into a second copy of the spec tree.

## 2. Golden corpus, as data

Input text plus expected token dump in `testdata/`, **generated from §0** so it cannot drift, and
held as *data files* rather than test functions so it survives the API shifting underneath it.
Golden files are reviewable once and diffable forever; generated assertions are neither.

## 3. Exhaustive adjacency sweep

Every ordered pair of `DEFAULT`-mode tokens, times three separators: nothing, a space, a comment.
At ~126 kinds that is under 50k cases and runs in well under a second — **exhaustive beats
random** at this size, and randomness would only obscure which pair failed.

The expected output is `[A, B]` **except** for the fusion set, which is exactly §8's maximal-munch
chains (`?` + `?` → `??`, `.` + `.` → `..`, `!` + `=` → `!=`, `1` + `.5` → one `DOUBLE`). Writing
that set down as a reviewable artifact is what makes F6's "ordering is load-bearing" testable
rather than aspirational.

The separator variants double as a **metamorphic relation**: inserting whitespace between two
tokens never changes the token sequence — with exactly one documented exception, `yield from`,
where §3's whitespace-only regex is normative and a comment between the words deliberately defeats
the fold. That single exception is what makes the relation worth testing: it must hold everywhere
else.

## 4. The mode stack is the real surface

The token table is the easy part. The lexer's genuine complexity is a **state machine** — a mode
stack with per-frame brace depth (§1, §6) — and the interesting bugs live there.

Cases that must be in the corpus from day one:

- **Brace counting**: `"${ {point} }"` — an enum-variant literal inside an interpolation, whose
  `}` must not close the splice. With tables bracket-delimited (R237's neighbourhood), enum
  literals and `match` are the *only* ways to get a brace inside a splice, which makes this the
  canonical case rather than a contrived one.
- **Nested same-kind literals**: `"${x ?? "none"}"` — F1's own example, and the reason the mode
  stack exists at all.
- **Depth**: interpolation nested several levels, each level a different mode.
- **Per-frame state** (R239): entering `INTERP_EXPR` resets what the frame carries and popping
  restores it.
- **Mode-internal order** (§6): the closing delimiter, `ESCAPE_PAIR`, `INTERP_OPEN`,
  `INTERP_IDENT`, `DOLLAR_TEXT`, then the text run — where `DOLLAR_TEXT`'s `\$` is correct *only*
  because the interpolation forms are attempted ahead of it.

## 5. Error tests

Pin the **code** and the **primary span's file and line**; never the prose (testing-strategy §2,
R240). Secondary spans are opt-in per test — they are the half that churns as diagnostics improve.

Every one of §11's twelve codes needs at least one case. Two deserve more than one:

- **`L0012` unexpected character** is the catch-all that makes the lexer *total* — every byte
  either begins a token or raises it — so it is what lets §3's tiling invariant hold on invalid
  input as well as valid.
- **`L0009`/`L0010`/`L0011`**, the unterminated family, are where recovery is least obvious:
  §1.1 collects errors rather than aborting, so the scanner must unwind the mode stack and keep
  making progress. Recovery behaviour needs its own cases, not just detection.

## 6. Fuzzing — four properties, no oracle needed

A lexer is an unusually good fuzz target because the interesting properties need no reference:

1. **Never panics** on arbitrary bytes — invalid UTF-8, NULs, lone `$`, unterminated everything.
2. **Always terminates** — structural since R242, not merely asserted: the scanner emits exactly
   one token per step covering at least one byte, so the classic
   bug, looping on an unrecognized character, is unwritable rather than untested.
3. **Spans tile the input exactly** — monotonic, gapless, summing to the input length. Total since
   R242: bytes no production claims are covered by `INVALID`, so it holds on invalid input too,
   which is where a fuzzer operates. The single strongest assertion available.
4. **The mode stack is empty at EOF**, or an error explains why it isn't.

Seeds: the **436 `luna`-labeled corpus blocks** — real Luna rather than generated noise — plus
every example in the spec's own text.

## 7. A spec-literal reference lexer

Transcribe §0's patterns, attempted in §8's order, into perhaps fifty lines of Go that are
obviously correct by inspection because they *are* the spec. Differential-test it against the fast
implementation §3 recommends (lex an `IDENT`, promote via lookup table, peek for the compounds).

This is `testing-strategy.md`'s oracle philosophy applied one level down, and it is what makes §3's
optimization safe to perform at all. It covers `DEFAULT` mode with no interpolation — where the
modes begin, regexes stop being sufficient, which is §4's territory.

---

## 8. Build order

Bottom-up along the state, testing each layer as it lands:

1. **`source`** — UTF-8 validation at ingress (lexical-structure §1), the pure-ASCII flag, the
   lazy line index (§9). A clean separate boundary, since validation happens *before* tokenizing.
2. **`token`** — §0's inventory as constants. §1's pin lands here.
3. **`lexer`, `DEFAULT` mode** — §8's attempt order. §2, §3, §7 land here.
4. **The mode stack** — §1/§6. §4 lands here.
5. **`diagnostic` integration** — §11's codes and spans. §5 lands here.

## 9. What this gate does not catch

Recorded because the opposite was claimed earlier and is wrong. **A lexing gate is permissive.**
Every retired spelling lexes cleanly: `pub` is an `IDENT`, `caps.io` is `IDENT DOT IDENT`,
`use (&io)` is `KW_USE LPAREN AMP IDENT RPAREN`, `!T` is `BANG IDENT`. None produces a lexical
error; they fail at parse or semantic analysis. So "every `luna` block lexes clean" would **not**
have caught the drift R232 found by hand.

What it does catch is malformed *lexemes* — unterminated literals, unknown escapes, leading zeros,
`0X`, stray non-ASCII outside comments and strings. The corpus is already lexically clean, so the
gate is mostly a **regression guard for the implementation** rather than a discovery tool for the
spec: two hits across 436 blocks, both since fixed.

The second hit narrows the claim above, though, and is worth stating precisely (R243). A retired
spelling is invisible to the gate only when it stays *lexically valid* — which is the usual case,
the four above included. **R237 is the exception**: retiring `/…/` for `~"…"` made the old spelling
lex as `SLASH LPAREN …` with `L0012` on every `\`, so the one site the R237 sweep missed
(`string-api.md` §5) fell out the moment a real lexer ran the corpus. The gate catches a retirement
exactly when the retirement changed what is lexable, and no more.

The corpus labeling is therefore worth three things and not a fourth: a regression corpus, fuzz
seeds, and — once the parser exists — a **parse** gate, which *is* strong and would catch stale
spellings.

## 10. Open

- **The parse gate needs a fragment convention.** 31 labeled blocks use `...` as an elision
  (`try { ... } catch (e) { ... }`). That lexes fine — `...` is `SPREAD` — but will not parse, so
  a parse gate needs a second label distinguishing complete programs from fragments. Not a
  decision for now; the count is recorded so it is not a surprise later.
- **`tests/` `:ro` versus `_test.go` beside the code.** claude-agent-plan §B mounts `tests/`
  read-only during implement so an agent cannot edit its way past a failure, but Go's convention
  puts unit tests beside the code, under the `rw` repo mount. Phase 0 is human-owned so it does
  not bite yet; the loop will surface it immediately.
- **`golangci-lint` must be built against the pinned Go.** Fedora's package is built with 1.25 and
  refuses a module targeting `go 1.26`; the working copy is `go install`ed into the `GOPATH`
  volume. The Containerfile still installs the unusable one (an R233 amendment).
