# Luna for Zed

Zed highlights via **tree-sitter**, not TextMate, so the vscode-luna folder does not
apply here. This extension pairs with `tooling/tree-sitter-luna/` (a deliberately
lexical, highlighting-grade grammar; keyword classification lives in
`highlights.scm`, generated from keywords.md).

## Stopgap (30 seconds, no extension)

Zed settings (`cmd-,`) — map the extension onto an existing language:

    "file_types": { "JavaScript": ["luna"] }

`const`/`let`, `=>`, `//` comments, both quote styles, and backticks all read
correctly; it is wrong about Luna's keywords but immediately useful.

## The real install

1. **Generate the parser** (once, and after grammar.js changes):

       cd tooling/tree-sitter-luna
       npx tree-sitter-cli generate

   (`tree-sitter.json` is included, so this produces ABI 15; without it the CLI
   warns and falls back to ABI 14, which Zed also accepts, the warning is
   cosmetic.)

2. **Make it a git repo and push it** (Zed fetches grammars by repo + rev; the
   generated `src/` must be committed):

       git init && git add -A && git commit -m "tree-sitter-luna v0"
       git remote add origin git@github.com:YOURNAME/tree-sitter-luna.git
       git push -u origin main

3. **Point the extension at it**: edit `extension.toml`, set `repository` to your
   repo URL and `rev` to the pushed commit sha (a sha is more reliable than a
   branch name).

4. **Install as a dev extension**: in Zed, `zed: install dev extension` (command
   palette), select the `tooling/zed-luna` folder. Zed fetches the grammar repo,
   compiles it, and `.luna` files highlight. `zed: reload extensions` after changes.

**Monorepo alternative to a separate repo**: Zed's grammar config takes an optional
`path` into the cloned repo, so the grammar can live inside the main project:

    [grammars.luna]
    repository = "file:///home/you/projects/luna"   # or the pushed URL
    rev = "<main-repo sha>"
    path = "tooling/tree-sitter-luna"

One trap: if you previously `git init`-ed **inside** `tree-sitter-luna`, the outer
repo treats it as a gitlink and its commits contain none of the grammar files;
remove the inner `.git`, commit the grammar (including generated `src/`) into the
main repo, and use that sha. Every regeneration is then a main-repo commit and a
`rev` bump. A separate repo remains the better shape if the grammar is ever
published (the tree-sitter ecosystem assumes repo-root grammars).

Local-only alternative for step 2/3: a `file://` URL to the local git repo works in
recent Zed for dev extensions, with two gotchas that both surface as the generic
compile error: the URL needs **three** slashes for an absolute path
(`file:///home/...`; two slashes makes the first segment a hostname), and it must
point at the **repo root** (the directory with `.git`, `grammar.js`, `src/`), never
at `src/` itself, Zed clones the repo and resolves `src/parser.c` from its root. If
your build rejects `file://` outright, the pushed-repo path always works.

## Troubleshooting "failed to compile grammar 'luna'"

The message is generic; `zed: open log` (command palette) has the real error. In
practice, three causes cover it, most likely first:

1. **`rev` is a branch name or the URL is the placeholder.** Zed wants a pinned
   commit: `git rev-parse HEAD` after pushing, paste the full sha into
   `extension.toml`, reinstall.
2. **`src/` not committed at that sha.** Zed clones and compiles the committed
   `src/parser.c`; it never runs `generate`. Verify:
   `git show <sha>:src/parser.c | head -3`.
3. **ABI 15 on an older Zed.** With `tree-sitter.json` present the CLI emits
   ABI 15, which only recent Zed builds accept. Compatibility move:
   `npx tree-sitter-cli generate --abi 14`, commit, update the sha.

4. **A malformed or root-missing `file://` URL** (see the local-only note above):
   three slashes, repo root, never `src/`.

5. **A stale clone in the extension folder.** Zed clones grammars into
   `zed-luna/grammars/<name>` during dev installs; after changing the
   `repository` URL, the leftover clone from the old URL blocks the install
   ("already exists, but is not a git clone of ..."). Fix:
   `rm -rf tooling/zed-luna/grammars`, reinstall. Add that directory to
   `.gitignore`, it is Zed's scratch space, never yours to commit.

If none of these match, the log line is the thing to read (and to send): command
palette `zed: open log`, or `~/.local/share/zed/logs/Zed.log` on Linux
(`~/Library/Logs/Zed/Zed.log` on macOS).

## Scope note

This grammar is for *highlighting*: it tokenizes exactly and imposes no structure,
so it cannot mis-parse and never breaks on half-typed code. String interpolation is
not highlighted inside strings in v0 (needs finer string rules; add when it
matters). The compiler's real parser is a separate artifact with a different job.
