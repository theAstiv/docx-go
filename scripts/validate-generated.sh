#!/usr/bin/env bash
# Validate documents that docx-go itself creates against the OOXML schema.
#
# validate-corpus.sh --delta only re-saves existing fixtures, so it cannot see
# defects in content docx-go writes (a missing required attribute, children in
# the wrong order, a Strict-only attribute). This script validates the
# documents scripts/generate-docs.sh writes:
#
#   - documents docx-go builds from scratch must have ZERO schema errors:
#     there are no pre-existing errors to excuse;
#   - Word-authored templates filled by docx-go (filled-template_*.docx) must
#     have no MORE errors than the template they came from. Word writes some
#     post-2007 markup (w:tblLook/@w:noHBand) that the validator rejects in the
#     template itself; filling must not add to it.
#
# Usage: scripts/validate-generated.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${REPO_ROOT}"

OUT="$(bash scripts/generate-docs.sh)"

DOCXLINT="${REPO_ROOT}/tmp/docxlint"
go build -o "${DOCXLINT}" ./cmd/docxlint

shopt -s nullglob
scratch=()
filled=()
for f in "${OUT}"/*.docx; do
  case "$(basename "$f")" in
    filled-*) filled+=("$f") ;;
    *) scratch+=("$f") ;;
  esac
done

echo "Validating ${#scratch[@]} document(s) built from scratch (zero errors)…"
"${DOCXLINT}" "${scratch[@]}"

# errors prints the number of schema errors docxlint reports for one file.
errors() {
  "${DOCXLINT}" "$1" 2>&1 | grep -c '^ERROR' || true
}

echo "Validating ${#filled[@]} filled template(s) (no new errors)…"
failed=0
for f in "${filled[@]}"; do
  src="testdata/fixtures/word/$(basename "$f" | sed 's/^filled-//')"
  got=$(errors "$f")
  base=$(errors "${src}")
  if [[ ${got} -gt ${base} ]]; then
    echo "FAIL $(basename "$f"): ${got} schema errors, the template has ${base}"
    "${DOCXLINT}" "$f" 2>&1 | grep '^ERROR' | head -20
    failed=$((failed + 1))
  else
    echo "OK   $(basename "$f") (${got} errors, template ${base})"
  fi
done
if [[ ${failed} -gt 0 ]]; then
  echo "validate-generated: ${failed} filled template(s) gained schema errors"
  exit 1
fi
echo "validate-generated: all documents pass"
