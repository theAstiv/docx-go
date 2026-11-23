#!/usr/bin/env bash
# Regenerate all golden XML snapshots under testdata/golden/xml/.
# Run this when a canonicalisation change intentionally changes the output.
#
# Usage: scripts/regen-golden.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${REPO_ROOT}"

echo "Regenerating golden files..."
# The -update flag is defined by internal/canon, so only test binaries that
# import it accept it: run exactly those packages, and let the test run's
# own exit status decide (a failure must not pass silently).
mapfile -t pkgs < <(go list -f '{{.ImportPath}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./... |
  awk '/github.com\/theAstiv\/docx-go\/internal\/canon( |$)/ { print $1 }')
if [[ ${#pkgs[@]} -eq 0 ]]; then
  echo "no package uses golden files" >&2
  exit 1
fi
go test -count=1 "${pkgs[@]}" -args -update
echo "Done. Review the diff with: git diff testdata/golden/"
