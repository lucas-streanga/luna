#!/bin/bash
# Archive the current directory into luna-spec.zip, respecting .gitignore
set -euo pipefail
OUTPUT="luna-spec.zip"
rm -f "$OUTPUT"
# List all files git tracks or would keep (excludes ignored via every nested .gitignore),
# NUL-delimited so spaces/newlines in names are safe.
git ls-files --cached --others --exclude-standard -z \
  | xargs -0 --no-run-if-empty zip -q "$OUTPUT"
echo "Created $OUTPUT"
