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

```
let listing = `ls -la`;      // a command, not run yet
run(listing);                // run it (exec spec) ...
run(listing);                // ... and again; it is reusable
```

---

## 2. The backtick literal builds a structured command

A command literal is delimited by backticks. It parses into a **program and an argument
list**, not a shell string:

```
`grep -n foo file.txt`
// program: "grep"
// args:    ["-n", "foo", "file.txt"]
```

- The literal is tokenized into a program name and arguments by whitespace, like a shell
  argument vector, but it is **never handed to a shell**. Execution uses a direct
  program-and-arguments call (exec spec), the equivalent of `execve` / Go's `exec.Command`,
  so there is no shell parsing step at all.
- Because no shell runs, **shell-language features are not available inside the literal**:
  no glob expansion (`*.txt` is the literal string `*.txt`, passed as one argument), no
  `$VAR` or `~` expansion, no shell pipes or redirection as text. These are provided
  structurally instead: pipelines by the `|>` operator (§4), and other features by
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
one-liner, or needing shell globbing), a separate `unsafe-system.shellExec` exists (exec
spec); it invokes a real shell, is therefore genuinely unsafe, and is marked so on both axes
(the `unsafe-` capability prefix and the `shell` in its name). The default `command` path
never touches a shell.

---

## 3. Interpolation: values are single arguments, always

A backtick literal may interpolate `${expr}`. Each interpolated value becomes **exactly one
argument** in the argument list, never spliced into a parsed string:

```
let name = "my file; rm -rf /";
`rm ${name}`
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

```
`grep ${userInput} ${userFile}`      // safe: userInput and userFile are each one argument
```

Both **comptime and runtime** interpolation are permitted and equally safe. A command
literal may be constructed at compile time (it is just data) or at runtime; only *running*
it requires a capability (exec spec), and running is never a comptime operation.

To interpolate **multiple** arguments from a collection (not one argument), a list-splicing
form is used so that each element becomes its own argument:

```
let flags = ["-l", "-a", "-h"];
`ls ${...flags}`                     // args: ["-l", "-a", "-h"], three arguments
```

`${expr}` is one argument; `${...expr}` spreads a list into many arguments. (The exact
spread syntax is subject to the destructuring / spread grammar, still to be specified; the
semantics are what matter here.)

---

## 4. Pipelines: the `|>` operator

Commands compose into pipelines with `|>`, which connects the stdout of the left command to
the stdin of the right and yields a **new `command` value** (a pipeline is itself a
command):

```
`cat access.log` |> `grep 404` |> `sort` |> `uniq -c`
```

- `|>` is the pipeline operator. It is **not** `=>`, which is already used for lambda
  bodies, match arms, and table-literal pairs; `|>` avoids that collision and evokes the
  shell pipe `|` it replaces.
- Because `|>` operates on command *values*, it composes literals and variables alike. Two
  command variables pipe into a new one with no special handling:

```
let stage1 = `producer`;
let stage2 = `consumer`;
let pipeline = stage1 |> stage2;     // a new command value, still inert
```

- A pipeline is a `command`, so it is inert and reusable like any command, and it runs
  through the same `exec` surface (exec spec). The pipeline structure is visible in the
  Luna source, one `|>` per stage, so which stage fails is identifiable when it runs (exec
  spec, `commandError.stage`).

The pipeline connects stages by their standard streams structurally; there is no shell
pipe, so, as with a single command, no shell parsing occurs at any stage.

---

## 5. What a command is not

- **Not a string.** A command is structured; it never round-trips through a shell string. A
  string is not a command and is not accepted where a `command` is required.
- **Not run when built.** Construction and composition (`` `...` ``, `|>`) are pure with
  respect to effects; nothing executes until an `exec` function runs the command (exec
  spec). A command may be built at comptime.
- **Not a shell.** No `/bin/sh` is involved on the `command`/`exec` path. Shell-string
  execution is the separate, `unsafe-`marked `unsafe-system.shellExec` (deferred).

---

## 6. Open questions

- **Spread syntax:** the exact form for spreading a list into multiple arguments
  (`${...flags}` above) depends on the destructuring / spread grammar, still to be
  specified. The semantics (each element becomes one argument) are fixed.
- **Environment and working directory:** how a command carries environment variables and a
  working directory (as builder-style methods on the command value, `.env(...)`, `.cwd(...)`,
  versus arguments to the run functions), pending the exec spec's execution model.
- **Stdin as data:** how a fixed input string or byte buffer is attached to a command
  (feeding stdin without a pipeline predecessor), pending the exec spec.
- **Redirection:** whether output redirection to a file is a structural method on the
  command, or purely an exec-time concern.
