# Command

A `command` is a structured, inert description of a program (or pipeline of programs) to
run: a program name, an argument list, and pipeline structure. It is its **own primitive
type**, built with a backtick literal, and it does **not** run when constructed. Running a
command is a separate, capability-gated effect specified in **exec**; this document is only
about building and composing the command value.

The defining property, and the reason `command` is structured rather than a string, is
**safety by construction**: a command is a program plus an argument *list*, executed without
a shell, so an interpolated value is always a single argument and can never be re-parsed as
shell syntax. Shell injection is therefore impossible, the same way the regex engine makes
ReDoS impossible and the capability sandbox makes comptime exfiltration impossible: the
vulnerability is closed at the architecture level, not patched with escaping.

---

## 1. `command` is its own type

`command` is a distinct primitive type: a compiled, inert description of what to run, not a
string and not a table. Making it a type gives the same benefits as `regex` being a type:

- **Inert.** Constructing a command does not run it. A command value can be built, stored,
  passed, and run zero or more times.
- **Reusable.** The same command value may be run repeatedly, or composed into a larger
  pipeline (§4), before it is ever executed.
- **Type-enforced.** A function taking `command` cannot be handed an arbitrary string; a
  value of type `command` is a validated structured command, never a shell string that
  something might have injected into.

A command is an **opaque** primitive (like `regex`, host-backed, not a table): its internal
structure, the program name and argument vector, is not user-inspectable and cannot be
manipulated as general data. That opacity is what guarantees the argument list is built only
through the safe backtick and interpolation path (§2, §3); a transparent table, by contrast,
would expose the arguments as ordinary list data that could be reassembled outside that
path, reopening the injection surface the structured design closes. So `command` is its own
sealed type rather than a table precisely because the safety property depends on its guts
being controlled, not open.

```luna
let listing = `ls -la`;      // a command, not run yet
run(listing);                // run it (exec spec) ...
run(listing);                // ... and again; it is reusable
```

---

## 2. The backtick literal builds a structured command

A command literal is delimited by backticks. It parses into a **program and an argument
list**, not a shell string:

```luna
_ = `grep -n foo file.txt`;
// program: "grep"
// args:    ["-n", "foo", "file.txt"]
```

- The literal's escapes are `` \` `` (a literal backtick), `\\`, and `\$` (a literal
  dollar) — the authoritative table is string §5.1 (R150, superseding the earlier
  no-escapes ruling, lexer G5: the `${'\u{60}'}`-style interpolation workaround for a
  literal backtick was ceremony where one escape pair suffices).
- **A command literal may not span lines** (R244, lexer §4): a raw newline ends it and raises
  `L0009`, and a trailing `\` does not continue it. Nothing is lost — the literal is split
  into a program and arguments by whitespace anyway, so a line break was never more than a
  separator.
- The literal is tokenized into a program name and arguments by whitespace, like a shell
  argument vector, but it is **never handed to a shell**. Execution uses a direct
  program-and-arguments call (exec spec), the equivalent of `execve` / Go's `exec.Command`,
  so there is no shell parsing step at all.
- Because no shell runs, **shell-language features are not available inside the literal**:
  no glob expansion (`*.txt` is the literal string `*.txt`, passed as one argument), no
  `$VAR` or `~` expansion, no shell pipes or redirection as text. These are provided
  structurally instead: pipelines by the `pipe` function (§4), and other features by
  functions and methods rather than by shell string syntax. This is the deliberate trade
  that buys injection-safety.

### 2.1 Why no shell

Handing a command line to `/bin/sh -c` means the shell re-parses the string, which is the
root of every shell injection: an interpolated value containing `;`, `|`, `` ` ``, `$()`,
or whitespace becomes *syntax*, not data. By building a structured program-plus-arguments
value and executing it without a shell, an interpolated value is always exactly one
argument, whatever characters it contains, so it can never become a second command, a pipe,
or a substitution. Injection is impossible by construction (§3).

For the genuine, rare case that needs an actual shell (running a user-supplied shell
one-liner, or needing shell globbing), a separate `unsafeShellExec` exists (exec
spec); it invokes a real shell, is therefore genuinely unsafe, and is marked so on both axes
(the `unsafe` capability prefix and the `shell` in its name). The default `command` path
never touches a shell.

---

## 3. Interpolation: values are single arguments, always

A backtick literal may interpolate `${expr}`. Each interpolated value becomes **exactly one
argument** in the argument list, never spliced into a parsed string:

```luna
let name = "my file; rm -rf /";
_ = `rm ${name}`;
// program: "rm"
// args:    ["my file; rm -rf /"]     <- one argument, not two commands
```

The dangerous-looking `name` is passed to `rm` as a single (strange) filename argument. No
shell parses it, so the `;` and `rm -rf /` are inert text, not a second command.

Because safety comes from the *structure* (one value, one argument), not from validating the
value, **interpolation is safe with any value at runtime**, including untrusted user input.
This is unlike regex, where interpolation into a literal is restricted to comptime-known
values (regex spec §7); a command literal has no such restriction, because a structured
argument cannot inject regardless of when it is known:

```luna
_ = `grep ${userInput} ${userFile}`; // safe: userInput and userFile are each one argument
```

Both **comptime and runtime** interpolation are permitted and equally safe. A command
literal may be constructed at compile time (it is just data) or at runtime; only *running*
it requires a capability (exec spec), and running is never a comptime operation.

To interpolate **multiple** arguments from a collection (not one argument), a list-splicing
form is used so that each element becomes its own argument:

```luna
let flags = ["-l", "-a", "-h"];
_ = `ls ${...flags}`; // args: ["-l", "-a", "-h"], three arguments
```

`${expr}` is one argument; `${...expr}` spreads a list into many arguments (spread spec §5,
one element one argv entry, `toString`-rendered, never re-tokenized; the
semantics are what matter here.)

---

## 4. Pipelines: the `pipe()` function

**A `command` is an immutable value.** This was implicit (inert descriptions, pure
composition, no mutator anywhere in this spec) and is now the rule: a command is
argv-and-structure, a *description*, with no live cursor, no OS handle, and no consumption
state, the process does not exist until `exec`. All "modification" is construction, a new
literal, a `pipe` composition, or future `withEnv`-style functions returning new commands.
Two consequences, both features: **piping does not move a command** (an immutable value
shares like any other), `let grep404 = \`grep 404\`;` composes into any number of
pipelines and stays valid; and **`exec` does not consume a command**, running a
description twice is two process trees, like calling a function twice.

Commands compose into pipelines with **`pipe`**, a built-in variadic function (R146; the
`|>` operator is retired — retired/pipeline.md) that connects each stage's stdout to the
next stage's stdin and yields a **new `command` value** (a pipeline is itself a command):

```luna
const pipe = fn (first: command, second: command, ...rest: command): command => {};
```

```luna
_ = pipe(`cat access.log`, `grep 404`, `sort`, `uniq -c`);
```

- **At least two stages, structurally**: the two leading required parameters make a
  one-stage "pipeline" unrepresentable rather than checked (functions §3.3.3's variadic
  is the tail).
- The name evokes the shell pipe it replaces without spending an *operator* on one
  domain (R146: the stream half of `|>` had become a redundant second spelling of
  catalogue chains after R91; the command half was real semantics, but one function's
  worth).
- Because `pipe` takes command *values*, it composes literals and variables alike:

```luna
let stage1 = `producer`;
let stage2 = `consumer`;
let pipeline = pipe(stage1, stage2);     // a new command value, still inert
```

- A pipeline is a `command`, so it is inert and reusable like any command, and it runs
  through the same `exec` surface (exec spec). The pipeline structure is explicit in the
  Luna source, one argument per stage, so which stage fails is identifiable when it runs
  (exec spec, `commandError.stage`).

The pipeline connects stages by their standard streams structurally; there is no shell
pipe, so, as with a single command, no shell parsing occurs at any stage.

---

## 5. Introspection

Introspection reads a command's structure. It is **structural, never textual**: you may read
the program, arguments, and pipeline stages as structured values, but there is **no**
function that renders a command back to an executable shell string, because that string is
the injectable form the type exists to avoid (§2.1). Structure-in, structure-out preserves
the argument boundaries that make a command safe; a rendered shell string would collapse
them.

Introspection is **pure**, no effect, no capability, so it lives with the command type as
**builtin free functions** (the type-catalogue convention, functions §3.4 — there is no
"command module"; std.exec owns only the effect, R172/R188), reached by UFCS
(`c.argsOf()` is `argsOf(c)`).

### 5.1 Structural introspection

Reading structure preserves argument boundaries, so it is always safe:

```luna
const isPipeline = fn (c: command): bool => {};      // whether c has more than one stage
const stageCount = fn (c: command): int => {};       // number of stages (1 for a single command)
const stages = fn (c: command): list => {};          // the stages, a list of commands ([c] for a single command)
const program = fn (c: command): string => {};       // the program name of a single-stage command
const argsOf = fn (c: command): list => {};          // the argument list of a single-stage command
```

- **`argsOf`, not `args`** (R188): bare `args` is taken — std.process exports
  `args() use (argv)` (R134) — and one name has one signature (functions §3.4). The `*Of`
  suffix is the established introspection vocabulary (`kindOf`, `baseOf`,
  `capabilitiesOf`), and command introspection is introspection. `program` and the rest
  collide with nothing and keep their bare names.
- `argsOf` returns the arguments as **separate elements**, so a dangerous-looking argument
  stays isolated as one element (`["my file; rm -rf /"]`), never merged into a string where
  its boundary could be lost. This is the safety property (§3) preserved in the read path.
- `program` and `argsOf` describe a single command. For a pipeline, decompose with `stages`
  first and read each stage; each stage is itself a `command`.

### 5.2 Diagnostic rendering: `debugJson`

For logging and debugging, a command renders to **structured JSON**, not a shell string:

```luna
const debugJson = fn (c: command): json => {};
```

The return is **`json`, not bare `string`** (R188): a JSON-producing function returning an
untyped string is the exact footgun the `json` constraint exists to close (json §1.1), and
the output is valid by construction, so the entry check elides (json §1, the
generator-output rule). (`debugJson` is also a per-type instance of the deferred general
debug-rendering design, conversion §6 — if a corpus-wide `inspect` ever lands, this is the
command's arm of it.)

`debugJson` emits the command as structured data, the program and arguments as a JSON array,
and a pipeline as an array of such stages, so **every argument is a distinct JSON string with
its boundary explicit**:

```luna
_ = `rm ${userFile}`.debugJson();
// {"program":"rm","args":["my file; rm -rf /"]}
```

Because the arguments stay separate in the output, the interpolation boundaries (the "holes"
where values were substituted) are explicit in the rendering: you see one argument per slot,
not a concatenated line. This is deliberately **not** a shell string and **not**
round-trippable to execution; it is for display, and its structure makes clear "this is one
argument," rather than a concatenated line that could be re-parsed or re-run. It is the safe
way to record what a command is.

An argument that is a `secret` (secret spec) renders as `<secret>` here, not its value, so
`debugJson` never leaks a credential passed as an argument. Sensitivity travels with the
value, so no redaction flag on `debugJson` is needed; a secret argument redacts itself.

### 5.3 No executable shell string here

There is intentionally **no** function on the command surface that renders a command to an
executable shell string. Producing that string is exactly the injectable artifact the
structured design avoids (§2.1), so it does not belong in ordinary introspection. The one
place that conversion legitimately lives is behind the `unsafe` marking:
**`unsafeCommandToString`** (deferred, part of the not-yet-specified unsafeSystem
module) will render a command to a shell string, its `unsafe` prefix and name signalling
that using the result reopens injection. Ordinary code introspects structurally (§5.1) or
diagnostically (§5.2); only explicitly-unsafe code obtains a shell string.

---

## 6. What a command is not

- **Not a string.** A command is structured; it never round-trips through a shell string. A
  string is not a command and is not accepted where a `command` is required.
- **Not run when built.** Construction and composition (`` `...` ``, `pipe`) are pure with
  respect to effects; nothing executes until an `exec` function runs the command (exec
  spec). A command may be built at comptime.
- **Not a shell.** No `/bin/sh` is involved on the `command`/`exec` path. Shell-string
  execution is the separate, `unsafe`-marked `unsafeShellExec` (deferred).

---

## 7. Open questions

- *(**Spread interpolation: resolved** — `${...expr}` in a command literal is fully
  specified: one element, one argv entry, `toString`-rendered, never re-tokenized
  (spread §5), lexing as `INTERP_OPEN` + `SPREAD` (lexer §6). This bullet's text was
  also mangled by an earlier edit; restored as the resolution it had become, R188.)*
- **Environment and working directory:** how a command carries environment variables and a
  working directory (as builder-style methods on the command value, `.env(...)`, `.cwd(...)`,
  versus arguments to the run functions) — the same open std.exec §6 records from the
  effect side; one question, two files, resolved together when ruled.
- **Stdin as data:** how a fixed input string or byte buffer is attached to a command
  (feeding stdin without a pipeline predecessor), pending std.exec (§6 there).
- **Redirection:** whether output redirection to a file is a structural method on the
  command, or purely an exec-time concern.
- **`debugJson` and secret arguments:** a `secret` argument (secret spec) redacts to
  `<secret>` in `debugJson` automatically. What remains open is whether *non*-secret
  arguments should ever be maskable for logging (a value not marked `secret` but still
  sensitive in context), or whether the rule is simply "mark it `secret` and it redacts
  everywhere," which is the current, preferred answer.
