#!/usr/bin/env bash
# Structural diff between two .docx files.
# Uses the project's own canonicaliser — agrees with the round-trip test oracle.
# Does NOT use xmllint --c14n11 (that normalises whitespace, which would mask
# text-corruption bugs).
#
# Usage: scripts/docx-diff.sh a.docx b.docx
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: $0 a.docx b.docx" >&2
  exit 1
fi

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

go run "${REPO_ROOT}/cmd/docxdiff" "$1" "$2" | less -R
