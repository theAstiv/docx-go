#!/usr/bin/env bash
# Write the documents docx-go itself creates to tmp/generated/, for the
# checks that need real output rather than re-saved fixtures:
# validate-generated.sh (OOXML schema) and libreoffice-smoke.sh.
#
# Generated documents:
#   - docx/validation_docs_test.go  (TestGenerateValidationDocs)
#   - examples/report
#   - examples/template_report: the template itself and the filled report
#     (templating must not turn a valid template into an invalid document)
#
# Usage: scripts/generate-docs.sh
# Prints nothing on stdout but the output directory, so callers can capture it.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${REPO_ROOT}"

OUT="${REPO_ROOT}/tmp/generated"
rm -rf "${OUT}"
mkdir -p "${OUT}"

{
  echo "Generating documents…"
  # go test runs inside ./docx, so the output directory must be absolute.
  DOCX_VALIDATION_DIR="${OUT}" go test -count=1 -run TestGenerateValidationDocs ./docx
  go run ./examples/report "${OUT}/example-report.docx"
  cp examples/template_report/template.docx "${OUT}/example-template.docx"
  go run ./examples/template_report "${OUT}/example-template-report.docx"
} >&2

shopt -s nullglob
files=("${OUT}"/*.docx)
# Guard against a generator that silently wrote nothing (a vacuous pass).
expected=6
if [[ ${#files[@]} -ne ${expected} ]]; then
  echo "expected ${expected} generated documents, found ${#files[@]}: ${files[*]:-none}" >&2
  exit 1
fi
echo "${OUT}"
