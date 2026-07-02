# Exec

`exec` runs a `command` (command spec). Building a command is pure and inert; **running** it
reaches outside the program (it spawns a process and does I/O), so execution is a
**capability**, and its result is governed by the error model. This document specifies the
`exec` capability, the two run functions (`run` and `capture`), the `commandError` raised on
failure, and the pipefail semantics. The command value itself, its construction, backtick
literal, interpolation, and `|>` pipelines, is in the command spec.

---

## 1. Execution is a capability

Running a command is an outside-reaching effect, so `exec` is a `nocopy` capability reached
only through `use` (functions §5.6), like `io` and `system`. Consequences:

- **Building is free; running is gated.** Constructing and composing a `command` needs no
  capability (it is pure data, command spec). A function that *runs* a command holds `exec`
  in its `use` clause, so "does this execute programs" is visible in the signature.
- **Comptime-safe by the same invariant.** Comptime forbids `use`, so it cannot hold `exec`,
  so a command can be *constructed* at compile time but never *run* at compile time. No
  exec-specific comptime rule is needed; it falls out of the capability model.

`exec` is an **ordinary** capability, not `unsafe-`. The structured-command design (command
spec) means running a command does not suspend Luna's guarantees: there is no shell, no
injection surface, no memory or type unsafety. It is a bounded, structured effect like `io`,
so it does not carry the `unsafe-` prefix (functions §5.6). (The genuinely unsafe
shell-string path is separate; §5.)

A command argument that is a `secret` (secret spec) is `reveal`ed **internally by `exec`**,
at the point the argument is handed to the spawned process, and nowhere else. `exec` is the
canonical infrastructure boundary where a raw secret legitimately crosses out of the program:
user code wraps a credential with `secret(...)`, passes the command around (redacted in every
log and error), and `exec` reveals it once, at the syscall, so the raw value never appears in
user-visible output.

```
const countLines = fn (path: string) use (exec): !int => {
  let out = try exec.run(`wc -l ${path}`);
  return parseFirstInt(out);
};
```

---

## 2. `run`: stdout, or throw on failure

```
fn run(cmd: command): !string
```

`run` executes `cmd`, and:

- **On success (exit 0, and every pipeline stage exit 0),** returns the command's **stdout**
  as a string.
- **On failure (any non-zero exit),** throws a **`commandError`** (§4) carrying the failing
  stage, its exit code, and its stderr.

`run` is errorable (`!string`): a caller handles failure with `try` or an errorable binding.
This is the common form, "run it, give me the output, and if it fails, fail", and it makes a
failed command impossible to ignore, because failure propagates as an error rather than
sitting in a return value to be checked (contrast the `$?`-inspection footgun of shell
scripts).

```
let files = try exec.run(`ls ${dir}`);              // stdout, or a thrown commandError
let code  = files.exitCode if (files is commandError);  // inspect the failure via narrowing
```

The exit code is **not** in `run`'s success result, because a successful `run` always exited
0 (any non-zero threw). The code is meaningful only on failure, where it rides on the thrown
`commandError`. To reach the exit code without throwing, use `capture` (§3).

---

## 3. `capture`: the full result, never throwing on non-zero

```
fn capture(cmd: command): !commandResult
```

`capture` executes `cmd` and returns a **result value** describing what happened, **without
throwing on a non-zero exit**:

```
commandResult = {
  stdout:   string;      // captured standard output
  stderr:   string;      // captured standard error
  exitCode: int;         // the exit status (0 or non-zero; not an error here)
};
```

`capture` is for commands whose **non-zero exit is information, not failure**: `grep` exits 1
for "no match", `diff` exits 1 for "files differ", `test` exits 1 for "false". Under `run`,
those would throw for an ordinary, expected outcome; `capture` hands back the exit code so
the caller decides what it means:

```
let r = exec.capture(`grep ${pattern} ${file}`);
matched(r.stdout)    if (r.exitCode == 0);      // 0: found
noMatch()            if (r.exitCode == 1);      // 1: no match (not an error)
throw error('grep failed') if (r.exitCode > 1); // >1: a real failure, the caller's call
```

`capture` still errors (`!commandResult`) for failures that are **not** just a non-zero exit,
the process could not be spawned at all (program not found, permission denied). Those are
`commandError`s like any other; a non-zero *exit* is not, because the process ran and
reported a status. So the split is precise: **a process that ran and exited is always a
`commandResult` (any code); a process that could not run is a `commandError`.**

### 3.1 Choosing `run` vs `capture`

The distinction is whether a non-zero exit means *error* for your use:

- **`run`** treats non-zero as failure: it throws. Reach for it when any non-zero exit is a
  problem (most commands).
- **`capture`** treats non-zero as data: it returns the code. Reach for it when the exit code
  is an expected result (grep, diff, test), or when you want stderr and the code regardless
  of success.

The exit code lives on `capture`'s result and on `run`'s thrown error, and nowhere else:
you can see the code exactly when you have chosen a form that surfaces it, which keeps a
failed `run` from being silently ignored.

---

## 4. `commandError` and pipefail

A command or pipeline fails when any stage exits non-zero (for `run`) or cannot be spawned
(for either function). Failure is a `commandError`, a `UserError` subtype (errors §4):

```
commandError = error {
  stage:    command;     // which stage of the pipeline failed
  exitCode: int;         // that stage's non-zero exit status
  stderr:   string;      // that stage's captured standard error
};
// plus inherited message, stacktrace, cause, data (errors §2.1)
```

**Pipefail is the default and the only behavior.** In a pipeline (`a |> b |> c`, command
spec §4), if **any** stage exits non-zero, the pipeline fails, and `commandError.stage`
identifies which stage. There is no mode to set (unlike bash, where `pipefail` is opt-in);
a pipeline that partially fails is a failure, full stop. Because the pipeline structure is
explicit in the source, the failing stage is precisely reportable.

This maps shell exit-code checking onto Luna's error model: a failed stage **throws** (under
`run`) and propagates like any error, rather than leaving a status code to be inspected and
forgotten. `try` / `catch` and `is`-narrowing (errors §8) handle a `commandError` exactly
like any other error.

---

## 5. The `unsafe-system.shellExec` escape hatch (deferred)

The `command` / `exec` path never invokes a shell, which is what makes it injection-safe
(command spec §2.1). For the genuine, rare need to run an actual **shell string**, running a
user-supplied shell one-liner, or needing shell features like globbing and `$VAR` expansion,
a separate operation will exist: **`unsafe-system.shellExec`**, which hands a string to a
real shell (`/bin/sh -c`).

It is deliberately marked dangerous on both axes:

- **`unsafe-` capability prefix**, because handing a string to a shell reintroduces
  injection: the shell re-parses the string, so an interpolated value can become syntax.
  This suspends the safety guarantee, which is exactly what `unsafe-` denotes (functions
  §5.6).
- **`shell` in the name**, because it genuinely runs a shell, unlike the shell-less
  `command`/`exec` path.

So the safe, structured path is the easy default (`exec.run` / `exec.capture` on a
`command`), and the dangerous shell-string path is possible but explicitly opt-in and
visibly marked. `unsafe-system` is a large, complex module (raw syscalls and shell access)
and is **specified separately, later**; this section only records that `shellExec` is its
home for shell-string execution and why it is unsafe.

---

## 6. Open questions

- **Streaming output:** whether `run` / `capture` also have streaming variants that yield
  stdout as a `stream` rather than a buffered `string`, for large or long-running output,
  pending the stream spec.
- **Environment, cwd, stdin:** how environment variables, working directory, and a fixed
  stdin input attach to a run, as methods on the `command` value (command spec open
  questions) or as options to `run` / `capture`.
- **Concurrency:** running commands from multiple green threads, and whether `exec` exposes a
  spawn-and-await form distinct from the synchronous `run` / `capture`, pending the
  concurrency model.
- **`capture` and spawn failure:** confirm the boundary that a process which *ran* yields a
  `commandResult` for any exit code, while a process that *could not start* yields a
  `commandError`, once the exec implementation details are settled.
