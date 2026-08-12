#!/bin/bash
# Archive the current directory into luna-spec.zip, respecting .gitignore
set -euo pipefail
OUTPUT="luna-spec.zip"
rm -f "$OUTPUT"
# List all files git tracks or would keep (excludes ignored via every nested .gitignore),
# NUL-delimited so spaces/newlines in names are safe.
#
# -y stores symlinks as symlinks instead of following them. `src/specs -> ../specs`
# exists so the spec is reachable from inside the Go module (lexer-testing-plan §10);
# without -y, zip would resolve it and the whole spec tree would be archived twice.
git ls-files --cached --others --exclude-standard -z \
  | xargs -0 --no-run-if-empty zip -qy "$OUTPUT"
echo "Created $OUTPUT"
