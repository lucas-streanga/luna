#!/usr/bin/env bash
#
# generate-grammar.sh: regenerate tree-sitter-luna's parser in a one-shot
# podman container (no local npm/node; pinned CLI via tooling/Containerfile).
# Does NOT touch git: publish-grammar.sh is the commit/push/repoint half.

set -euo pipefail
cd "$(dirname "$0")"

rm -rf tree-sitter-luna/src
podman compose build
podman compose run --rm treesitter tree-sitter generate

echo "Generated. Next: tooling/publish-grammar.sh to commit, push, and repoint zed-luna."
