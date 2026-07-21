# The pipeline operator `|>` (retired)

**`|>` is retired in full (R146).** Not moved, not narrowed — the operator no longer
exists, its token is gone from the lexer, and its precedence tier is a tombstone
(associativity §1).

## Why

The operator's own §1 was its undoing. It opened by condemning a general pipe as "a
second spelling of what `.`-chaining already does — redundant syntax splits idiom for
no gain," and reserved `|>` for dataflow, "a notion UFCS cannot express." That claim
was true when written and **went false at R91–R93**: the catalogue redesign made every
transformer a lazy, kind-following, stream-taking free function, so

```
f.lines() |> filter(isError) |> take(10)     // then
f.lines().filter(isError).take(10)           // now: identical semantics
```

are the *same operation* — lazy-start, pull-driven, short-circuiting, source-taking,
effects checked at the pull. None of those properties were ever the operator's (its own
§5.1/§5.2 said so); and mechanically, `s |> filter(p)` had to inject the left operand
as the call's first argument, which is UFCS with different punctuation. The stream half
of `|>` had become exactly the redundant second spelling its spec was written to
forbid.

The command half (`a |> b` wiring stdout to stdin) was genuine semantics — neither side
is a function — but one function's worth: it is now **`pipe(...)`** (command §4), a
built-in variadic, keeping every property (structured, shell-free, injection-safe,
inert, immutable operands) without spending an operator on one domain.

## Was → is

| Was | Is |
|-|-|
| `s \|> map(f) \|> take(n)` | `s.map(f).take(n)` — the catalogue chain, identical semantics (stream §7) |
| `cmd1 \|> cmd2 \|> cmd3` | `pipe(cmd1, cmd2, cmd3)` (command §4) |
| pipeline §4 inertness | unchanged: lazy-start streams (stream §1.2), inert commands (command §2) |
| pipeline §5.1 the enforced move | unchanged, and never the operator's: every catalogue call **takes** its stream operands (iterable-functions §1.5, stream §7.3) |
| pipeline §5.2 effects at the pull, failing stages panic | unchanged, and never the operator's: capability rules and the no-error-channel rule (stream §7.2, std.io §8) |
| pipeline §3.1 the stream↔command bridge | unchanged: an explicit `exec`-level API, still pending (exec spec) |
| the `PIPELINE` token, tier 11 | removed; the tier number is kept as a tombstone so tier-12 citations stand |
| `any` spec's `\|>`-needs-narrowing rule | moot: `pipe` is an ordinary typed function under the existing UFCS rule (any §2) |

No content here is authoritative; cite stream §7 and command §4.
