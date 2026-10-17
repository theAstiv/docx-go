#!/usr/bin/env bash
# Validate .docx fixtures against the OOXML schema.
#
# Default (strict) mode: validate the standard corpus globs as-is.
# Exits non-zero if any schema error is found.  Use this when you already have
# round-tripped golden files present (e.g. after running save-fixtures.sh).
#
# --delta mode: for each fixture, round-trip it through docxsave and fail only
# if the round-trip introduced NEW schema errors (delta > 0).  Pre-existing
# errors in the original fixture are expected and do not count against the PR.
# This is the mode used by CI, where golden files are not committed.
#
# Usage:
#   scripts/validate-corpus.sh           # strict — any error = failure
#   scripts/validate-corpus.sh --delta   # CI-safe delta comparison
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${REPO_ROOT}"

DELTA_MODE=false
if [[ "${1:-}" == "--delta" ]]; then
  DELTA_MODE=true
fi

shopt -s nullglob

# ── Strict mode ──────────────────────────────────────────────────────────────
if [[ "${DELTA_MODE}" == "false" ]]; then
  docx_files=(
    testdata/golden/docx/*.docx
    testdata/fixtures/word/*.docx
    testdata/fixtures/poi/*.docx
    testdata/fixtures/python-docx/*.docx
  )
  if [[ ${#docx_files[@]} -eq 0 ]]; then
    echo "No .docx files found — corpus is empty."
    exit 0
  fi
  echo "Validating ${#docx_files[@]} file(s)..."
  go run "${REPO_ROOT}/cmd/docxlint" "${docx_files[@]}"
  exit 0
fi

# ── Delta mode ────────────────────────────────────────────────────────────────
fixture_files=(
  testdata/fixtures/word/*.docx
  testdata/fixtures/poi/*.docx
  testdata/fixtures/python-docx/*.docx
)

if [[ ${#fixture_files[@]} -eq 0 ]]; then
  echo "No fixture files found under testdata/fixtures/ — nothing to validate."
  exit 0
fi

# count_errors runs docxlint and returns the number of ERROR lines.
# The docxlint exit code is ignored; only the count matters.
count_errors() {
  go run "${REPO_ROOT}/cmd/docxlint" "$@" 2>&1 | grep -c '^ERROR' || true
}

# Build docxsave once so we don't recompile per fixture.
mkdir -p tmp testdata/golden/docx
echo "Building docxsave…"
go build -o tmp/docxsave "${REPO_ROOT}/cmd/docxsave"

echo "Delta validation: ${#fixture_files[@]} fixture(s)…"
failed=0
for f in "${fixture_files[@]}"; do
  base=$(basename "$f" .docx)
  parent=$(basename "$(dirname "$f")")
  out="testdata/golden/docx/${parent}__${base}.docx"

  if ! tmp/docxsave "$f" "$out" 2>/dev/null; then
    echo "FAIL (save error)  $f"
    failed=$((failed + 1))
    continue
  fi

  orig_errors=$(count_errors "$f")
  rt_errors=$(count_errors "$out")

  if [[ "${rt_errors}" -gt "${orig_errors}" ]]; then
    echo "REGRESSION  $f  (${orig_errors} pre-existing → ${rt_errors} after round-trip)"
    go run "${REPO_ROOT}/cmd/docxlint" "$out" 2>&1 | grep '^ERROR' || true
    failed=$((failed + 1))
  else
    echo "OK  $f  (${orig_errors} pre-existing, delta=0)"
  fi
done

echo ""
if [[ $failed -gt 0 ]]; then
  echo "validate-corpus: ${failed} regression(s) — round-trip introduced new schema errors"
  exit 1
fi
echo "validate-corpus: all ${#fixture_files[@]} file(s) round-trip without new errors"
