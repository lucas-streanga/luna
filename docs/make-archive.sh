#!/bin/bash
# Archive the current directory into luna-spec.zip

set -euo pipefail

OUTPUT="luna-spec.zip"

# Remove an existing archive so we don't append to a stale one
rm -f "$OUTPUT"

# Zip the current directory's contents recursively
zip -r "$OUTPUT" . -x "$OUTPUT"

echo "Created $OUTPUT"
