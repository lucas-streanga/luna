#!/usr/bin/env bash
#
# check.sh: run everything under src/.
#
#   ./check.sh              format, vet, tests, lint  (the gate; seconds)
#   ./check.sh --fuzz 30    ... plus 30s on each fuzz target
#   ./check.sh --mutate     ... plus the mutation harness (minutes)
#   ./check.sh --ambiguity  ... plus the exhaustive grammar search (a minute)
#   ./check.sh --all        the gate, fuzzing, mutation, and the grammar search
#
# Runs from anywhere. Every step runs even after an earlier one fails, so one
# invocation reports everything wrong rather than the first thing wrong; the
# summary at the end is the verdict and the exit status follows it.
#
# Three deliberate choices, all about checks that pass by not running:
#
#   -race and -shuffle=on, per lexer-testing-plan §7. The driver is the first
#   concurrent code here, and a suite that cannot see a data race is worse than no
#   suite once goroutines exist. Shuffling catches tests that depend on each other.
#
#   -count=1 always. Go caches test results, and a cached pass over a spec file
#   the test never opened is a green suite that checked nothing. This repo has
#   the src/specs and src/tooling symlinks precisely so the corpus is inside the
#   module and therefore tracked -- but the cache is not worth trusting for a
#   deliberate full run.
#
#   A missing tool is a FAILURE, not a skip. golangci-lint absent and silently
#   passed over is the same fail-open the -count=1 note describes, one level up.

set -uo pipefail
cd "$(dirname "$0")"

fuzz_seconds=0
mutate=0
ambiguity=0

while [ $# -gt 0 ]; do
  case "$1" in
    --fuzz)      fuzz_seconds="${2:-30}"; shift 2 ;;
    --mutate)    mutate=1; shift ;;
    --ambiguity) ambiguity=1; shift ;;
    --all)       fuzz_seconds=30; mutate=1; ambiguity=1; shift ;;
    -h|--help) sed -n '2,9p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "check.sh: unknown argument '$1'" >&2; exit 2 ;;
  esac
done

failed=()
step() {
  local name="$1"; shift
  printf '\n\033[1m== %s\033[0m\n' "$name"
  if "$@"; then
    return 0
  fi
  failed+=("$name")
  return 1
}

# gofmt -l prints unformatted files and exits 0 either way, so emptiness is the
# assertion; without this it would pass on every file it disliked.
check_fmt() {
  local out
  out="$(gofmt -l .)" || return 1
  [ -z "$out" ] && return 0
  echo "not gofmt'd:"
  echo "$out"
  return 1
}

require() {
  command -v "$1" >/dev/null 2>&1 && return 0
  echo "$1 is not installed; this check cannot run and is not being skipped" >&2
  return 1
}

run_lint() { require golangci-lint && golangci-lint run ./...; }

# One target at a time is a `go test -fuzz` restriction, not a choice: it takes a
# single pattern and a single package. Seeded corpus entries still run in the
# ordinary test step above -- this is the part that generates new input, which
# nothing else in the suite does.
run_fuzz() {
  local pkg target found=0
  while read -r pkg; do
    while read -r target; do
      [ -z "$target" ] && continue
      found=1
      echo "--- $target in $pkg (${fuzz_seconds}s)"
      go test -run='^$' -fuzz="^${target}\$" -fuzztime="${fuzz_seconds}s" "$pkg" || return 1
    done < <(go test -list='Fuzz.*' "$pkg" 2>/dev/null | grep '^Fuzz' || true)
  done < <(go list ./...)

  [ "$found" -eq 1 ] && return 0
  echo "no fuzz targets found; this check reached nothing" >&2
  return 1
}

step "gofmt"  check_fmt
step "go vet" go vet ./...
step "go test" go test -race -shuffle=on -count=1 ./...
step "golangci-lint" run_lint

[ "$fuzz_seconds" -gt 0 ] && step "fuzz (${fuzz_seconds}s per target)" run_fuzz
[ "$mutate" -eq 1 ] && step "mutation" go run ./cmd/mutate

# Opt-in for a reason the other two do not share: this one is a proof over a fixed
# grammar, so its answer moves only when specs/build/grammar.md does. Re-running it
# every commit re-establishes what the last run established. The gate keeps the
# three-token sweep (internal/ebnf), which is the part that can regress.
[ "$ambiguity" -eq 1 ] && step "ambiguity" go run ./cmd/ambiguity -sweep

printf '\n'
if [ ${#failed[@]} -eq 0 ]; then
  printf '\033[32mall checks passed\033[0m\n'
  exit 0
fi
printf '\033[31m%d failed:\033[0m %s\n' "${#failed[@]}" "${failed[*]}"
exit 1
