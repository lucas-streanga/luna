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


### 1.1 What a panic carries, and what the recover adds

**One `I` code, not one per site.** A code is a name, and an internal panic already has a better
one: its message, which lives on the same line as the check it describes and cannot drift from
it. Splitting `I` per panic site would put a second, hand-maintained copy of that taxonomy in a
registry — the argument §6.3 used to refuse eleven `Error` kinds, one level up. It also fails all
three jobs compiler §3.1 gives a code: no test ever pins *which* invariant broke, since every one
is unreachable; `luna explain` would have the same page for all of them; and unique codes fragment
a user's search so a variant of a known crash does not find the known issue. Against that, per-site
codes would make every `panic()` acquire a permanent number and a permanent title under §3.1's
append-only rule, and retiring an assertion would retire a number forever.

If a second `I` is ever wanted it should be **per detection mechanism**, not per site. There is one
candidate: compiler §1.7 guarantees the emitted Go compiles, so a failure there is also `I` — but
it arrives with no panic and its report wants the generated Go rather than a stack. That code
allocates when §1.8 exists to raise it.

**The panic is spelled `panic(diagnostic.Bugf(…))`**, which keeps the format template. That is the
one part of a crash report that cannot be added later: rendering happens at the call site, so
`event 42 …` and `event 7 …` are one bug wearing two faces unless the template is captured where
it is written.

The rule is enforced over the whole oracle by a test in `oracle/diagnostic` that walks syntax
trees rather than text, so it sees `panic(` in code and never in a comment or a string — which
matters, since this section names the form. **The driver is exempt**, and deliberately: it is the
layer that *catches* a Bug, so its own panics answer to a design still being written (§1.2).

Three mechanics, verified rather than assumed, that the recover can rely on:

- **`debug.Stack()` inside the deferred function still sees the panic site.** The frames are on
  the stack while the defer runs, so the capture includes the line that panicked. Nothing is
  needed at the call site.
- **A Go fault classifies itself.** A nil-map write recovers as a value satisfying `runtime.Error`,
  where a `diagnostic.Bug` does not, so "our assertion" and "the compiler tripped over Go" are
  distinguishable with a type switch and no cooperation from either.
- **`recover` only works in the goroutine that panicked.** compiler §2 parses modules in parallel,
  so the recover belongs in each worker, not around the driver's top-level call. This is the
  mistake that would kill the process on exactly the case the mechanism exists for.

And one rule that is not a mechanic: **the recovered unit's output is discarded, never consumed.**
A panic mid-construction leaves a half-built tree, and handing that to §1.4 is worse than having
no tree.

### 1.2 Still to decide

- ~~The `I` code, or codes.~~ **Decided in §1.1: one, and per detection mechanism if ever more.**
  What remains is the number itself, and R250's rule applies: it is allocated when the recover
  lands with a test to pin it, and not before.
- **What the diagnostic's span is.** A panic carries no position. The file is known; the offset is
  not, and "the whole file" may be the honest answer.
- **Whether the recovered panic's message reaches the user.** It names an internal invariant, which
  is meaningless to a user and essential to a bug report. A `luna` flag, or the message going to
  stderr while the diagnostic stays generic, are both plausible and neither is chosen.
- **Whether a recovered panic is retried differently**, e.g. the LSP falling back to the previous
  good tree for that file. That wants the LSP to exist first.
