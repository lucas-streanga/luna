# Release script: driving other programs

The shell script every project has — check the tree is clean, tag, build, push — written
where a failed command cannot be ignored and a credential cannot leak into a log.

```luna
import std.io;
import std.process;
import { exec, run, capture, commandError } from std.exec;

const main = fn () use (io, exec, argv, env): int! => {
  let arguments = args();
  die("usage: ${arguments[0]} <version>") if (arguments.count() != 2);

  let version = arguments[1] as string;
  let tag   = "v${version}";
  let image = "ghcr.io/acme/gadget:${version}";
  let notes = "release ${version}";

  let token = envVars()['GHCR_TOKEN'];
  die('GHCR_TOKEN is not set') if (token == undefined);

  die('working tree is dirty') if (!run(`git status --porcelain`).isEmpty());

  let branch = run(`git rev-parse --abbrev-ref HEAD`).trim();
  die("releases come from master, not ${branch}") if (branch != 'master');
  die("${tag} already exists") if (capture(`git rev-parse --verify ${tag}`)->exitCode == 0);

  _ = run(`git tag -a ${tag} -m ${notes}`);
  println("tagged ${tag}");

  try {
    _ = run(`docker login ghcr.io -u acme-ci -p ${token as secret}`);
    _ = run(`docker build -t ${image} .`);
    _ = run(`docker push ${image}`);
    _ = run(`git push origin ${tag}`);
  } catch (e: commandError) {
    printerr("${e.stage.debugJson()} exited ${e.exitCode}: ${e.stderr}");
    _ = run(`git tag -d ${tag}`);          // leave the repository as we found it
    return 1;
  };

  println("released ${image}");
  return 0;
};
```

What it exercises:

- **`run` throws, `capture` returns the code** (std.exec §2, §3), and the choice is made per
  call. Everything here is `run`, because a failed `git status` or `docker push` really is a
  failure — except `git rev-parse --verify`, which exits non-zero to *say* the tag is
  absent. That one exit code is data, so it is the one `capture`. The `$?`-inspection
  footgun has no spelling: a `run` whose command failed cannot proceed, because the failure
  is on the channel the caller must handle, not in a status variable to forget.
- **`_ =` on every command whose output is unused** (variables, no-discard): `run` returns
  stdout, so ignoring it is a visible, greppable act. The four in the `try` block are the
  ones this script genuinely does not read.
- **The literal is a program and an argument vector, never a shell string** (command §2):
  `${tag}` and `${notes}` are each exactly **one** argument no matter what they contain, so
  a version string with a space or a `;` is an odd argument and never a second command.
  There is no shell, so there is nothing to quote and nothing to escape.
- **The credential is a `secret` from the moment it exists** (std.process §2, secret §5):
  `envVars()` hands back `key => secret`, so `token` is already wrapped — listing the
  environment is `use (env)`, and reading a value's *content* is a separate gate. This file
  never gets that far: **there is no `use (revealSecret)` in it**, and the run functions
  reveal the argument internally, at the syscall (std.exec §1). The token exists in no Luna
  frame's readable form.
- **`debugJson` renders a failed command safely** (command §5.2): structured JSON, one array
  element per argument, and **a `secret` argument renders as `<secret>`** — so the error
  path of a `docker login` cannot print the token, and there is no shell history for it to
  land in either. Rendering a command to an executable shell string is not on the surface at
  all (§5.3).
- **A typed `catch` for the rollback** (errors §8.3): `catch (e: commandError)` selects that
  subtree and lets everything else keep unwinding. The block form catches panics too, which
  is why the narrowing matters — a `typeError` in this script should reach the top, not
  trigger a tag deletion.
- **The rollback is itself errorable, and propagates** (errors §7): `run` inside the `catch`
  can throw, and `main` is `fn!`, so a failed cleanup fails the program rather than being
  swallowed by the handler that was cleaning up. Deliberate: the operator needs to know the
  tag is still there.
- **Capabilities are the audit** (capabilities §3.1): `use (io, exec, argv, env)` on one
  line says this program prints, runs other programs, reads its arguments, and lists its
  environment — and, by omission, that it does not touch the filesystem structure and cannot
  reveal a secret.
