#!/usr/bin/env bash
#
# regen-grammar.sh: regenerate tree-sitter-luna (in a one-shot podman
# container; see tooling/compose.yaml), commit, push, and repoint zed-luna at
# the new rev. Run from anywhere inside the repo. Requires podman + git only.
#
# After it finishes: in Zed, `zed: reload extensions` (or `zed: install dev
# extension` on first install).

set -euo pipefail

root="$(git -C "$(cd "$(dirname "$0")" && pwd)" rev-parse --show-toplevel)"
cd "$root"

GRAMMAR="tooling/tree-sitter-luna"
EXT="tooling/zed-luna"

# 1) Clear the old generated parser for a clean regen. Deliberately NOT
#    touching $EXT/grammars: after a dev install that directory holds the LIVE
#    compiled grammar Zed loads in place, and deleting it breaks the extension
#    until reinstall (R70). Zed refreshes it itself on 'reload extensions' when
#    the rev changes. Nuke it manually only after changing the repository URL
#    (README triage §5).
rm -rf "$GRAMMAR/src"

# 2) Regenerate, inside a one-shot container (no local npm/node). The build is
#    the "re-compose": cached and instant when the Containerfile is unchanged,
#    and the pinned tree-sitter-cli makes generation reproducible. Run and
#    remove; nothing stays resident.
(cd tooling \
  && podman compose build \
  && podman compose run --rm treesitter tree-sitter generate)

# 3) Commit the grammar and push, so the rev we are about to pin exists remotely.
git add "$GRAMMAR"
git commit -m 'regenerated tree-sitter-luna'
git push

rev="$(git rev-parse HEAD)"

# 4) Repoint the extension: derive the repo URL from origin (ssh -> https, Zed
#    clones anonymously), and regenerate extension.toml whole.
url="$(git remote get-url origin | sed -E 's#^git@([^:]+):#https://\1/#; s#\.git$##')"

# 4b) Reconcile the clone cache with the URL (R73, unifying R65/R70): Zed's
#     clone in $EXT/grammars is the LIVE grammar and must survive routine
#     regens, but a clone from a DIFFERENT url blocks reinstall ("already
#     exists, but is not a git clone of ..."). Nuke it only on a url change.
clone="$EXT/grammars/luna"
if [ -d "$clone" ]; then
  old_url="$(git -C "$clone" remote get-url origin 2>/dev/null || echo '')"
  if [ "$old_url" != "$url" ]; then
    echo "grammar clone points at '$old_url', repointing to '$url': clearing cache"
    rm -rf "$EXT/grammars"
  fi
fi

cat > "$EXT/extension.toml" <<EOF
id = "luna"
name = "Luna"
version = "0.0.1"
schema_version = 1
authors = ["Luna spec"]
description = "Syntax highlighting for the Luna programming language."

[grammars.luna]
repository = "$url"
rev = "$rev"
path = "$GRAMMAR"
EOF

# 5) Commit the repoint and push again.
git add "$EXT"
git commit -m 'point zed-luna at regenerated grammar'
git push

echo
echo "grammar rev: $rev"
echo "repository:  $url (path: $GRAMMAR)"
echo "Now in Zed: 'zed: reload extensions' (or 'zed: install dev extension' first time)."
