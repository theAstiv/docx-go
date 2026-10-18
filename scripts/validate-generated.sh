#!/usr/bin/env bash
# Validate documents that docx-go itself creates against the OOXML schema.
#
# validate-corpus.sh --delta only re-saves existing fixtures, so it cannot see
# defects in content docx-go writes (a missing required attribute, children in
# the wrong order, a Strict-only attribute). This script validates the
# documents scripts/generate-docs.sh writes and requires ZERO schema errors:
# freshly generated output has no pre-existing errors to excuse.
#
# Usage: scripts/validate-generated.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${REPO_ROOT}"

OUT="$(bash scripts/generate-docs.sh)"

shopt -s nullglob
files=("${OUT}"/*.docx)
echo "Validating ${#files[@]} generated file(s)…"
go run ./cmd/docxlint "${files[@]}"
