#!/usr/bin/env bash
# Validate all .docx files in testdata/golden/docx/ and testdata/fixtures/
# against the OOXML schema using docxlint (OOXMLValidatorCLI or Docker).
#
# Fails (non-zero exit) if any schema errors are found; summarises warnings.
#
# Usage: scripts/validate-corpus.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${REPO_ROOT}"

shopt -s nullglob
docx_files=(
  testdata/golden/docx/*.docx
  testdata/fixtures/word/*.docx
  testdata/fixtures/libreoffice/*.docx
  testdata/fixtures/gdocs/*.docx
)

if [[ ${#docx_files[@]} -eq 0 ]]; then
  echo "No .docx files found — corpus is empty."
  exit 0
fi

echo "Validating ${#docx_files[@]} file(s)..."
go run "${REPO_ROOT}/cmd/docxlint" "${docx_files[@]}"
