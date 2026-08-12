# `std.process`

```luna
import std.process;
```

The process-self module: the program's own arguments and environment, under two
capabilities (**`argv`**, **`env`**). Small on purpose — a process's identity is data
you read, not machinery. Born in R134's split of the retired `std.system` name
(retired/system.md): the filesystem half is `std.filesystem`; everything process-shaped
lives here.

## 1. Arguments

```luna
export const argv = capability;
export const args = fn () use (argv): list => {};
```

`args()` returns the program's arguments as a list of strings, `args()[0]` the program
name (the C convention the examples already use). `argv` **is** a capability (R43,
capabilities §9): arguments are program *input*, and no arbitrary function — no
arbitrary *dependency* — should read them unbidden; `main` declares `use (argv)` like
any other authority. This module is the home that capability row's flag promised.

## 2. Environment

```luna
export const env = capability;
export const envVars = fn () use (env): table => {};   // 'PATH' => secret, 'HOME' => secret, ...
```

Designed in exec (R42), **relocated here** (R134): it sat there because exec passes
environments to children, but reading your *own* environment is process-self, not
command-running — exec composes with this module when it needs to. The ruling carries
unchanged: values are **`secret`**, so enumeration and reading are **separately
gated** — `use (env)` lists what exists, extracting a value's content requires
`revealSecret` — the double gate falling out of machinery that already exists (secret spec).

## 3. Refusals, recorded

- **No `chdir`, ever.** The working directory is process-global mutable state: one
  task changing it silently re-resolves every relative path in every other task — a
  data race by OS design, exactly the shared-mutable-state class the language forbids
  (concurrency §2). Relative paths resolve against the working directory **at process
  start**, always. (A read-only `cwd(): path` may land with `std.filesystem` if real
  use demands it.)
- **No `exit()`.** The exit code is `main`'s return value (`fn () use (…): int!` —
  the examples' shape); early termination is `die` or error propagation. Structured
  exit only: no function tears down the process from the middle of a frame, past
  every pending `defer`.

## 4. Deferred

`pid`, hostname, username — process-identity facts, none alpha-blocking; each becomes
a gated read here when real use arrives (hostname may belong to net).
