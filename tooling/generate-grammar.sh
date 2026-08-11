#!/usr/bin/env bash
#
# generate-grammar.sh: run the whole grammar chain.
#
#   lexer.md §0 --[ gengrammar, Go ]--> grammar.js --[ tree-sitter, podman ]--> src/
#
# Both halves, because grammar.js is now generated too and running one without the
# other leaves the repo in a state nothing detects by eye: src/ is what Zed clones
# and compiles, so a regenerated grammar.js on its own changes nothing a user sees.
#
# Does NOT touch git: publish-grammar.sh is the commit/push/repoint half.
#
# Requires Go on the host as well as podman. That is a change from when this only
# drove the container -- the tree-sitter CLI is still pinned in Containerfile and
# still the only thing that needs node, but the generator is Go and runs locally,
# like every other check under src/.

set -euo pipefail
cd "$(dirname "$0")"

# 1) §0 -> grammar.js (and the three grammars that need no compilation).
(cd ../src && go run ./cmd/gengrammar)

# 2) grammar.js -> src/. Deleted first so a rule removed upstream cannot survive
#    in a stale generated file.
rm -rf tree-sitter-luna/src
podman compose build
podman compose run --rm treesitter tree-sitter generate

# 3) Again, now that src/node-types.json exists. highlights.scm names a node only
#    once the committed parser can produce it -- a query naming one it cannot
#    fails as a WHOLE, taking Luna highlighting down in Zed rather than degrading
#    -- so the query can only catch up after step 2. Usually a no-op.
(cd ../src && go run ./cmd/gengrammar)

echo
echo "Generated. Next: tooling/publish-grammar.sh to commit, push, and repoint zed-luna."
