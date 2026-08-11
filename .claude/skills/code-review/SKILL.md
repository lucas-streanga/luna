---
name: code-review
description: Review source code against this project's eight criteria — structure, naming, comments, API, function complexity, performance, type usage, and over-abstraction. Use when asked to review code, audit a package, or check a change before committing. Produces a prioritized report; makes no edits unless separately asked.
---

# Code review

Review the named code against the eight criteria below. **Produce a report. Make no edits**
unless the user asks for them separately — deciding what to change is theirs, and a review
that quietly rewrites its subject cannot be disagreed with.

## Method

1. **Read all of it first.** Reviewing file-by-file as you read produces findings about the
   first file and none about the interactions, which is where the real defects are.
2. **Map the functions before judging them** — one line each, with file and line. It makes
   size outliers and naming inconsistencies visible as a list rather than as a feeling.
3. **Verify every claim you intend to report.** Grep for the call site, run the case, check
   the branch is reachable. A confident wrong finding costs more than a missed one, and
   this applies double to anything a subagent or tool reported to you.
4. **Cite `file:line`.** A finding without a location is an opinion.

## The criteria

1. **Structure.** Are the directories and modules laid out properly? Is there enough
   separation of concerns? Watch for a file that has quietly become the place things go —
   size is the symptom, an inaccurate name is the confirmation.

2. **Naming.** Are symbols readable, concise, and accurate? The worst pairs are ones that
   differ only in the order or case of their fragments; a reader must hold both to tell
   them apart. If two functions do genuinely different jobs, the difference belongs in the
   name — not in which file they happen to live in.

3. **Comments.** Compact, concise, accurate. A comment that says *why*, and names the
   alternative that lost, survives a rewrite. A comment restating the code is noise, and a
   comment that has gone stale is worse than none — check every one that cites a filename,
   a count, or a symbol that has moved.

4. **APIs.** Easy to understand, concise. Look for behavior that is real but undocumented:
   what a returned slice aliases, what happens on a path the doc does not mention, what a
   caller must not do.

5. **Function complexity.** Is each function easy to read and understand on its own? Length
   alone is not the test — a long function that reads top-down as one ordered procedure is
   fine. The test is whether you had to *re-derive* the logic instead of reading it.

6. **Performance bugs.** An inefficient algorithm where a simpler, obvious one would do.
   This is not a hunt for micro-optimizations. Do report work repeated per byte or per
   element that could be hoisted, and predicates ordered so the expensive test runs first.

7. **Type usage.** Are types used to control error conditions, mutability, and side
   effects? The highest-value finding in this category is **a type answering questions it
   is not authoritative for** — where two facts happen to coincide today, so one is derived
   from the other, and the derivation becomes silently wrong the moment they diverge. Also
   look for: redundant state that must stay in sync (a bool beside a sentinel that already
   encodes it), mutable pointers into growable storage, and exported mutable fields.

8. **Over-abstraction.** Is any abstraction chosen only to enforce DRY, at the cost of
   complexity? **Clean-code principles are not the end-all-be-all.** Duplication of three
   trivial lines is cheaper than a shared helper with four parameters. Judge an abstraction
   by whether it makes a wrong combination *unrepresentable*, not by how much text it saved.

## What counts as a finding

A finding is something that will **cost someone later**: a latent trap, a wrong claim, a
name that will be misread, work that is quadratic where it could be linear.

A preference is not a finding. If you cannot say what it costs, leave it out or mark it a
nit. Do not pad the report — but do not conclude "no issues" without having looked for the
categories in 7 and 8, which are the ones that hide.

Rank the **latent** above the **present**. Code that works today but whose types permit a
wrong future call is a better finding than a long function, because tests cover the second
and nothing covers the first.

## Report shape

Organize by criterion, then close with a prioritized table:

| | finding |
|-|-|
| **Fix** | live traps and wrong claims |
| **Worth doing** | real improvements with a clear cost of leaving |
| **Nits** | one line each, no elaboration |

State plainly what is good, briefly and once — a review with no positive signal gives the
reader no calibration for the negative. Then spend the words on the findings.

## Recurring traps in this codebase

Checked first, because each has been found here at least once:

- **A fact derived from something authoritative for a different fact.** Deriving
  line-spanning from an escape context was correct until a literal form shared one and not
  the other.
- **Dead table rows and unreachable default branches.** An empty row reads as a claim
  ("nothing is allowed here") rather than as an absence. An unreachable branch should
  assert, not return a plausible value.
- **Docs that name files, counts, or symbols.** They go stale when code moves. A count
  that is tested elsewhere should not be repeated in prose at all.
- **Loop invariants re-derived per element**, and predicate chains that run a prefix scan
  before the byte test that would have ruled it out.
- **Two mechanisms for one fact** — a guard method *and* a sentinel value, a bool *and* a
  zero value. One of them will be updated alone.
