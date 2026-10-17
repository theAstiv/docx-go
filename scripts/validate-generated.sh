#!/usr/bin/env bash
# Validate documents that docx-go itself creates against the OOXML schema.
#
# validate-corpus.sh --delta only re-saves existing fixtures, so it cannot see
# defects in content docx-go writes (a missing required attribute, children in
# the wrong order, a Strict-only attribute). This script builds documents that
# use every feature and requires ZERO schema errors: freshly generated output
# has no pre-existing errors to excuse.
#
# Generated documents:
#   - docx/validation_docs_test.go  (TestGenerateValidationDocs)
#   - examples/report
#
# Usage: scripts/validate-generated.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${REPO_ROOT}"

OUT="${REPO_ROOT}/tmp/generated"
rm -rf "${OUT}"
mkdir -p "${OUT}"

echo "Generating documents…"
# go test runs inside ./docx, so the output directory must be absolute.
DOCX_VALIDATION_DIR="${OUT}" go test -count=1 -run TestGenerateValidationDocs ./docx
go run ./examples/report "${OUT}/example-report.docx"

shopt -s nullglob
files=("${OUT}"/*.docx)
# Guard against a generator that silently wrote nothing (a vacuous pass).
expected=4
if [[ ${#files[@]} -ne ${expected} ]]; then
  echo "expected ${expected} generated documents, found ${#files[@]}: ${files[*]:-none}" >&2
  exit 1
fi
echo "Validating ${#files[@]} generated file(s)…"
go run ./cmd/docxlint "${files[@]}"
