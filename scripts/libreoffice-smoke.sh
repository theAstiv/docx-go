#!/usr/bin/env bash
# Convert every round-tripped .docx in testdata/golden/docx/ to PDF using
# LibreOffice headless in Docker.  A successful conversion is the smoke
# signal that LibreOffice can open the file without producing repair errors.
#
# Builds the image on first run; subsequent runs reuse the cached image.
#
# Usage: scripts/libreoffice-smoke.sh
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${REPO_ROOT}"

# Docker Desktop on Windows accepts Windows-style paths (C:/...) for volume
# mounts; Git Bash's /c/... form does not always translate cleanly, so use
# cygpath when it is available.  On Linux/macOS this is a no-op.
to_docker_path() {
  if command -v cygpath >/dev/null 2>&1; then
    cygpath -m "$1"
  else
    echo "$1"
  fi
}

IMAGE=docx-go-libreoffice

if ! docker image inspect "${IMAGE}" >/dev/null 2>&1; then
  echo "Building ${IMAGE}…"
  docker build -t "${IMAGE}" tools/libreoffice
fi

if [[ ! -d testdata/golden/docx ]] || [[ -z $(ls testdata/golden/docx/*.docx 2>/dev/null) ]]; then
  echo "No round-tripped .docx files in testdata/golden/docx/."
  echo "Run scripts/save-fixtures.sh first."
  exit 1
fi

OUTDIR="testdata/golden/pdf"
mkdir -p "${OUTDIR}"

# Mount the whole testdata/golden tree so soffice can read all .docx files
# in a single container invocation; this is much faster than spawning one
# container per file (LibreOffice startup dominates).
# The glob /files/*.docx must expand inside the container, so override the
# entrypoint with bash -c.
DOCX_HOST=$(to_docker_path "$(pwd)/testdata/golden/docx")
PDF_HOST=$(to_docker_path "$(pwd)/${OUTDIR}")

docker run --rm \
  -v "${DOCX_HOST}:/files:ro" \
  -v "${PDF_HOST}:/out" \
  --entrypoint bash \
  "${IMAGE}" \
  -c 'soffice --headless --nologo --norestore --nofirststartwizard --convert-to pdf --outdir /out /files/*.docx'

failed=0
for f in testdata/golden/docx/*.docx; do
  base=$(basename "$f" .docx)
  pdf="${OUTDIR}/${base}.pdf"
  if [[ -f "${pdf}" ]]; then
    echo "OK  ${base}"
  else
    echo "FAIL ${base} (no PDF produced)"
    ((failed++))
  fi
done

echo ""
if [[ $failed -gt 0 ]]; then
  echo "libreoffice-smoke: $failed file(s) failed to convert"
  exit 1
fi
echo "libreoffice-smoke: all $(ls testdata/golden/docx/*.docx | wc -l) file(s) converted successfully"
