#!/usr/bin/env bash
# Save every .docx fixture through docx-go and write the result to
# testdata/golden/docx/.  The saved files are the inputs for downstream
# OOXML validation and LibreOffice smoke testing.
#
# Usage: scripts/save-fixtures.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${REPO_ROOT}"

mkdir -p testdata/golden/docx tmp

# Build once so we don't pay the go run compilation cost per fixture (~20×
# speedup on large corpora).
DOCXSAVE_BIN="${REPO_ROOT}/tmp/docxsave"
echo "Building docxsave…"
go build -o "${DOCXSAVE_BIN}" "${REPO_ROOT}/cmd/docxsave"

# globstar is a bash 4+ feature; macOS ships bash 3.2 which silently ignores it.
if ! shopt -s globstar 2>/dev/null; then
  echo "save-fixtures.sh requires bash 4+ (globstar); macOS users: brew install bash" >&2
  exit 2
fi
shopt -s nullglob
fixture_files=(testdata/fixtures/**/*.docx)

if [[ ${#fixture_files[@]} -eq 0 ]]; then
  echo "No fixtures found under testdata/fixtures/ — nothing to save."
  exit 0
fi

failed=0
for f in "${fixture_files[@]}"; do
  base=$(basename "$f" .docx)
  # Prefix with parent dir name to avoid basename collisions across corpora.
  parent=$(basename "$(dirname "$f")")
  out="testdata/golden/docx/${parent}__${base}.docx"
  if "${DOCXSAVE_BIN}" "$f" "$out"; then
    echo "OK  $f → $out"
  else
    echo "FAIL $f"
    ((failed++))
  fi
done

echo ""
if [[ $failed -gt 0 ]]; then
  echo "save-fixtures: $failed file(s) failed to save"
  exit 1
fi
echo "save-fixtures: ${#fixture_files[@]} file(s) saved to testdata/golden/docx/"
