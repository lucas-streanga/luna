# Views (retired)

The `view` type is **retired** (R95). Views existed to carry "meta space" context so that
`.` on a view meant meta call, and to name which protocol an access meant. Both jobs are
now done directly by `->` (protocols §3): all protocol member access goes through `->`,
resolved at compile time against the closed member space, so there is no intermediate
value to carry and nothing for `.` to re-interpret.

What replaced each piece:

| Was (views) | Is now |
|-|-|
| `tab->P` producing a `view` value | nothing — `tab->member`, qualified `tab->P.member` when ambiguous (protocols §3.1) |
| `.` on a view = meta call | `->` calls directly; `.` is element access / UFCS, always (protocols §3.5) |
| `: self` / `: view` return-type chaining | chaining via `->` on returned values; `self` = the receiver's type `@CurrentProto` (protocols §2.4) |
| `view(tab, proto)` constructor | nothing — no view values exist |
| view→table coercion, view equality | moot |
| `tab->P` as `undefined` capability probe | bare member read is `undefined` when unapplied; hard use panics; `?->` is the soft form (protocols §3.2) |
| `@@sb` (a view's protocol) | moot; `@@tab` (a table's applied protocols) is specified in protocols §8 |
| bare `tab->name()` = built-in protocol call | the built-in protocol is dead (R91); the catalogue is free functions (iterable-functions, indexable-functions) |

Do not reintroduce the `view` type, the `view()` constructor, or `: view` returns; the
dispatch table lives in protocols §3.5.
