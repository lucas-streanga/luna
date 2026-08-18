# Driver implementation

The decisions taken while building the driver, and why. compiler §1–§3 own the pipeline and the
error model; this records how we build against them, as `oracle/parser/parser-implementation.md`
does for its package. Nothing here is a language decision, so nothing here is a `CHANGES.md`
ruling.

**This file overlaps the repository-root `driver.md`**, which is the older plan for the same
subject — plural drivers, per-unit passes, fan-out and merge order, `Unit` bookkeeping. The two
want merging, and the merge is not done: this file holds what has been decided since, and
`driver.md` still holds the rest. Recorded so that neither is read as the whole story.

---

## 1. A panic is an `I` diagnostic, and converting it is the driver's job

> **The parser and every other pass may panic. A panic is never about the user's input** — it is
> compiler §3.1's `I`, "an invariant violated, this is a compiler bug"
> (`oracle/parser/parser-implementation.md` §7.8). **The driver is what turns one into a
> diagnostic**, and each driver turns it into a different outcome.

- The **LSP driver** recovers per request, reports the `I` against the file it was working on, and
  **stays up**. A language server that dies on one malformed file is worse than one that says it
  could not read it.
- The **batch driver** aborts. A compiler bug should stop a build, and tooling §3 already has it
  aborting at the phase boundary; this is the same abort with a different cause.

**Why the recover is here and not in the pass.** Putting it in `Parse` would be worse at both
ends. The fuzzers and the unit tests call passes directly, so a recover inside a pass would hide
exactly the bugs `FuzzParse` exists to find; and a pass that swallows its own invariant violation
has to return *something*, which means handing a corrupt tree to the next phase. Recovering at the
driver keeps panics loud where they are useful and contained where they are not — and it puts the
choice in the same place tooling §3 already puts abort-versus-continue, so no new axis is
introduced.

**Why not an error return instead of a panic.** Threading an `error` through every nonterminal
function to report a condition that cannot happen would cost the parser its shape — §4's "a
function per nonterminal that opens, consumes, and closes" — for a path no input reaches. A panic
is the cheaper spelling of "unreachable", and the recover is the one place that has to know it.

### 1.1 Still to decide

- **The `I` code, or codes.** Whether one code covers every escaped panic or the drivers
  distinguish (parser bug, analysis bug, driver bug) is open. R250's rule applies either way: a
  code is allocated when there is an implementation to raise it and a test to pin it, so the first
  `I` lands with the first recover and not before.
- **What the diagnostic's span is.** A panic carries no position. The file is known; the offset is
  not, and "the whole file" may be the honest answer.
- **Whether the recovered panic's message reaches the user.** It names an internal invariant, which
  is meaningless to a user and essential to a bug report. A `luna` flag, or the message going to
  stderr while the diagnostic stays generic, are both plausible and neither is chosen.
- **Whether a recovered panic is retried differently**, e.g. the LSP falling back to the previous
  good tree for that file. That wants the LSP to exist first.
