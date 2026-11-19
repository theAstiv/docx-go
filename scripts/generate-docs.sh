#!/usr/bin/env bash
# Write the documents docx-go itself creates to tmp/generated/, for the
# checks that need real output rather than re-saved fixtures:
# validate-generated.sh (OOXML schema) and libreoffice-smoke.sh.
#
# Generated documents:
#   - docx/validation_docs_test.go  (TestGenerateValidationDocs)
#   - examples/report
#   - examples/edit, applied to the examples/report output
#   - examples/template_report: the template itself and the filled report
#     (templating must not turn a valid template into an invalid document)
#   - the Word-authored templates (testdata/fixtures/word/template_*.docx),
#     filled from testdata/template/word-templates.json, except
#     template_images.docx, which TestWordTemplates_Images fills
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
  go run ./examples/edit "${OUT}/example-report.docx" "${OUT}/example-edited.docx" >/dev/null
  cp examples/template_report/template.docx "${OUT}/example-template.docx"
  go run ./examples/template_report "${OUT}/example-template-report.docx"
  go build -o "${REPO_ROOT}/tmp/template_report" ./examples/template_report
  # template_images.docx needs image values, which JSON cannot carry:
  # TestWordTemplates_Images fills it and writes filled-template_images.docx.
  DOCX_VALIDATION_DIR="${OUT}" go test -count=1 -run 'TestWordTemplates_Images$' ./docx/template
  for tpl in testdata/fixtures/word/template_*.docx; do
    [[ "$(basename "${tpl}")" == template_images.docx ]] && continue
    "${REPO_ROOT}/tmp/template_report" -template "${tpl}" \
      -data testdata/template/word-templates.json \
      "${OUT}/filled-$(basename "${tpl}")"
  done
} >&2

shopt -s nullglob
files=("${OUT}"/*.docx)
# Guard against a generator that silently wrote nothing (a vacuous pass).
expected=18
if [[ ${#files[@]} -ne ${expected} ]]; then
  echo "expected ${expected} generated documents, found ${#files[@]}: ${files[*]:-none}" >&2
  exit 1
fi
echo "${OUT}"
