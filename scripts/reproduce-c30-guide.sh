#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

engine="$root/dist/engine/attestara"
if [[ ! -x "$engine" ]]; then
  printf 'Build the CLI with npm run setup before running this example.\n' >&2
  exit 3
fi

workspace="$(mktemp -d)"
trap 'rm -rf -- "$workspace"' EXIT
mkdir -p "$workspace/fixture"
cp -R -- "$root/tests/fixtures/attestara/C30/." "$workspace/fixture/"

run_case() {
  local label="$1"
  local expected="$2"
  shift 2
  local actual=0
  if "$@" >"$workspace/$label.stdout" 2>"$workspace/$label.stderr"; then
    actual=0
  else
    actual=$?
  fi
  printf '%s: expected_exit=%s actual_exit=%s\n' "$label" "$expected" "$actual"
  if [[ "$actual" -ne "$expected" ]]; then
    printf 'C30 regression: %s returned an unexpected exit status.\n' "$label" >&2
    return 1
  fi
}

printf 'source_revision=%s\n' "$(git rev-parse HEAD)"
printf 'fixture=tests/fixtures/attestara/C30 (disposable copy)\n'

run_case evaluate 0 "$engine" evaluate \
  --input "$workspace/fixture/manifest.json" \
  --files "$workspace/fixture" \
  --policy "$workspace/fixture/rule.json" \
  --evidence "$workspace/fixture/evidence.json" \
  --freshness fresh \
  --store "$workspace/store" \
  --operation-id c30-guide-check

run_case stored_verify 0 "$engine" verify \
  --store "$workspace/store" \
  --operation-id c30-guide-check

run_case signed_verify 0 "$engine" verify \
  --input "$workspace/fixture/manifest.json" \
  --files "$workspace/fixture" \
  --passport "$workspace/evaluate.stdout" \
  --signature "$workspace/fixture/signature.json" \
  --trust "$workspace/fixture/trust.json"

printf 'synthetic change to referenced file\n' >>"$workspace/fixture/subject.txt"

run_case tampered_bytes 2 "$engine" verify \
  --input "$workspace/fixture/manifest.json" \
  --files "$workspace/fixture" \
  --passport "$workspace/evaluate.stdout" \
  --signature "$workspace/fixture/signature.json" \
  --trust "$workspace/fixture/trust.json"

printf 'C30 synthetic regression completed; no original fixtures were modified.\n'
