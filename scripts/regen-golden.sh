#!/usr/bin/env bash
# Regenerate all golden XML snapshots under testdata/golden/xml/.
# Run this when a canonicalisation change intentionally changes the output.
#
# Usage: scripts/regen-golden.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${REPO_ROOT}"

echo "Regenerating golden files..."
go test -update ./... 2>&1 | grep -v "^?" || true
echo "Done. Review the diff with: git diff testdata/golden/"
