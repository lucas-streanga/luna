#!/usr/bin/env bash
#
# reorg-luna-spec.sh
# Reorganize the flat Luna spec into folders, and fix index.md's path column.
#
# Safe by design:
#   - inter-spec cross-references are name-based ("tables §2.1"), NOT path-based,
#     so moving files breaks no references; only index.md's path column changes.
#   - uses `git mv` when inside a git repo (preserves history), else plain `mv`.
#   - idempotent: re-running skips already-moved files and won't double-prefix paths.
#   - dry-run mode prints the plan without touching anything.
#   - verifies every .md file is accounted for before moving (nothing left behind).
#
# Usage:
#   ./reorg-luna-spec.sh            # apply
#   ./reorg-luna-spec.sh --dry-run  # show what would happen, change nothing
#
# Run it from the spec root (the directory that contains index.md).

set -euo pipefail

DRY_RUN=0
[[ "${1:-}" == "--dry-run" || "${1:-}" == "-n" ]] && DRY_RUN=1

# --- must be run from the spec root -----------------------------------------
if [[ ! -f index.md ]]; then
  echo "error: index.md not found here; run this from the spec root." >&2
  exit 1
fi

# --- folder -> files ---------------------------------------------------------
declare -A LAYOUT=(
  [overview]="high-level-overview.md types.md"
  [types]="type.md never.md undefined.md int.md double.md bool.md numeric-tower.md \
           strings.md bytes.md regex.md command.md secret.md tables.md table-api.md \
           views.md stream.md stream-api.md stringBuilder.md enum.md functions.md"
  [declarations]="protocols.md constraints.md errors.md capabilities.md attributes.md"
  [type-operations]="as.md is.md match.md equality.md conversion.md reflection.md"
  [bindings]="variables.md destructuring.md spread.md wildcard.md \
              optional-access-and-coalescing.md"
  [expressions]="operators.md numeric-operators.md control-flow.md pipeline.md \
                 defer.md range.md"
  [concurrency]="concurrency.md exec.md"
  [build]="modules.md compiler.md incremental-compilation-build-cache.md tooling.md"
  [internals]="internal-representation-of-variables.md \
               internal-representation-of-strings.md"
)

# --- completeness check: every .md on disk (except index.md) must be mapped ---
declare -A MAPPED=()
for dir in "${!LAYOUT[@]}"; do
  for f in ${LAYOUT[$dir]}; do MAPPED["$f"]="$dir"; done
done

missing_on_disk=0
for f in "${!MAPPED[@]}"; do
  if [[ ! -f "$f" && ! -f "${MAPPED[$f]}/$f" ]]; then
    echo "WARN: mapped file not found on disk: $f" >&2
    missing_on_disk=1
  fi
done

unmapped=0
shopt -s nullglob
for f in *.md; do
  [[ "$f" == "index.md" ]] && continue
  if [[ -z "${MAPPED[$f]:-}" ]]; then
    echo "WARN: on-disk file not in layout (would be left in root): $f" >&2
    unmapped=1
  fi
done
shopt -u nullglob

if (( unmapped || missing_on_disk )); then
  echo "note: resolve the warnings above (edit LAYOUT) before applying." >&2
  (( DRY_RUN )) || { echo "aborting."; exit 1; }
fi

# --- choose mover ------------------------------------------------------------
if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  MOVER="git mv"
else
  MOVER="mv"
fi
echo "using: $MOVER  (dry-run: $DRY_RUN)"
echo

# --- move files --------------------------------------------------------------
for dir in "${!LAYOUT[@]}"; do
  (( DRY_RUN )) || mkdir -p "$dir"
  for f in ${LAYOUT[$dir]}; do
    if [[ -f "$f" ]]; then
      echo "move  $f -> $dir/$f"
      (( DRY_RUN )) || $MOVER "$f" "$dir/$f"
    elif [[ -f "$dir/$f" ]]; then
      echo "skip  $f (already in $dir/)"
    fi
  done
done

# --- update index.md's path column: `file.md` -> `dir/file.md` ---------------
# Backtick-delimited and dot-escaped so the match is exact; already-prefixed
# entries (`dir/file.md`) no longer start with a backtick before the name, so
# re-running never double-prefixes.
echo
echo "updating index.md paths..."
for dir in "${!LAYOUT[@]}"; do
  for f in ${LAYOUT[$dir]}; do
    esc=$(printf '%s' "$f" | sed 's/\./\\./g')      # escape dots for the regex
    if (( DRY_RUN )); then
      grep -q "\`${f}\`" index.md 2>/dev/null && echo "  would rewrite \`$f\` -> \`$dir/$f\`"
    else
      sed -i "s#\`${esc}\`#\`${dir}/${f}\`#g" index.md
    fi
  done
done

echo
if (( DRY_RUN )); then
  echo "dry-run complete; nothing changed."
else
  echo "done. index.md at root; all other specs under their folders."
  echo "inter-spec references are name-based and were not touched."
fi
