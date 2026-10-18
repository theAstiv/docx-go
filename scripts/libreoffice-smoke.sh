#!/usr/bin/env bash
# Convert .docx files to PDF with LibreOffice headless in Docker. A file that
# converts is the smoke signal that LibreOffice can open it; a file that
# produces no PDF (or hangs) fails.
#
# This is a weak check: LibreOffice opens much that Word would repair. It
# complements, not replaces, the OOXML validator and opening files in Word.
#
# Builds the image on first run; subsequent runs reuse the cached image.
#
# Usage: scripts/libreoffice-smoke.sh [dir ...]
#   Each dir holds .docx files. Default: testdata/golden/docx (fixtures
#   re-saved by scripts/save-fixtures.sh). CI also passes tmp/generated
#   (scripts/generate-docs.sh).
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

dirs=("$@")
if [[ ${#dirs[@]} -eq 0 ]]; then
  dirs=(testdata/golden/docx)
fi

shopt -s nullglob
failed=0
total=0
for dir in "${dirs[@]}"; do
  files=("${dir}"/*.docx)
  if [[ ${#files[@]} -eq 0 ]]; then
    echo "No .docx files in ${dir} (run scripts/save-fixtures.sh or scripts/generate-docs.sh first)." >&2
    exit 1
  fi
  outdir="tmp/pdf/$(basename "${dir}")"
  rm -rf "${outdir}"
  mkdir -p "${outdir}"

  # One container per directory: LibreOffice start-up dominates, so convert
  # every file in one soffice call. The glob must expand inside the
  # container, hence bash -c. timeout guards against a file that hangs it.
  # --infilter forces the Word 2007+ import filter: without it LibreOffice
  # falls back to plain-text import and "converts" a file that is not a
  # .docx at all, so a broken file would pass.
  docker run --rm \
    -v "$(to_docker_path "$(pwd)/${dir}"):/files:ro" \
    -v "$(to_docker_path "$(pwd)/${outdir}"):/out" \
    --entrypoint bash \
    "${IMAGE}" \
    -c 'timeout 900 soffice --headless --nologo --norestore --nofirststartwizard --infilter="MS Word 2007 XML" --convert-to pdf --outdir /out /files/*.docx' \
    || echo "soffice exited non-zero for ${dir}; checking which files converted" >&2

  for f in "${files[@]}"; do
    base=$(basename "$f" .docx)
    total=$((total + 1))
    if [[ -s "${outdir}/${base}.pdf" ]]; then
      echo "OK   ${dir}/${base}"
    else
      echo "FAIL ${dir}/${base} (no PDF produced)"
      failed=$((failed + 1))
    fi
  done
done

echo ""
if [[ ${failed} -gt 0 ]]; then
  echo "libreoffice-smoke: ${failed} of ${total} file(s) failed to convert"
  exit 1
fi
echo "libreoffice-smoke: all ${total} file(s) converted"
