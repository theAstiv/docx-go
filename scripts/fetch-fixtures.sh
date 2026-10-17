#!/usr/bin/env bash
# fetch-fixtures.sh — download external fixture documents for round-trip testing.
#
# Run once to populate testdata/fixtures/ from upstream sources.
# Downloaded files are committed to the repo so CI requires no network access.
#
# Sources and licences:
#   python-docx  MIT          https://github.com/python-openxml/python-docx
#   Apache POI   Apache 2.0   https://github.com/apache/poi
#
# Usage: bash scripts/fetch-fixtures.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FIXTURES="$REPO_ROOT/testdata/fixtures"

fetch() {
    local url="$1"
    local dest="$2"
    mkdir -p "$(dirname "$dest")"
    if [[ -f "$dest" ]]; then
        echo "  skip (exists): $(basename "$dest")"
        return
    fi
    echo "  fetch: $(basename "$dest")"
    curl -fsSL --retry 3 "$url" -o "$dest"
}

# ---------------------------------------------------------------------------
# python-docx (MIT)
# https://github.com/python-openxml/python-docx/tree/master/tests/test_files
# ---------------------------------------------------------------------------
echo "=== python-docx (MIT) ==="
PY_RAW="https://raw.githubusercontent.com/python-openxml/python-docx/master/tests/test_files"
PY_DEST="$FIXTURES/python-docx"

fetch "$PY_RAW/test.docx"              "$PY_DEST/test.docx"
fetch "$PY_RAW/having-images.docx"     "$PY_DEST/having-images.docx"
fetch "$PY_RAW/blk-inner-content.docx" "$PY_DEST/blk-inner-content.docx"
fetch "$PY_RAW/sct-inner-content.docx" "$PY_DEST/sct-inner-content.docx"

# ---------------------------------------------------------------------------
# Apache POI (Apache 2.0)
# https://github.com/apache/poi/tree/trunk/test-data/document
# ---------------------------------------------------------------------------
echo "=== Apache POI (Apache 2.0) ==="
POI_RAW="https://raw.githubusercontent.com/apache/poi/trunk/test-data/document"
POI_DEST="$FIXTURES/poi"

# Basic paragraphs and styles
fetch "$POI_RAW/SampleDoc.docx"               "$POI_DEST/SampleDoc.docx"
fetch "$POI_RAW/heading123.docx"              "$POI_DEST/heading123.docx"
fetch "$POI_RAW/capitalized.docx"             "$POI_DEST/capitalized.docx"
fetch "$POI_RAW/Styles.docx"                  "$POI_DEST/Styles.docx"

# Paragraph formatting
fetch "$POI_RAW/bug-paragraph-alignment.docx" "$POI_DEST/bug-paragraph-alignment.docx"
fetch "$POI_RAW/emptyPPr.docx"                "$POI_DEST/emptyPPr.docx"
fetch "$POI_RAW/WithTabs.docx"                "$POI_DEST/WithTabs.docx"

# Content that exercises RawElement passthrough (not yet modelled in Phase 1)
fetch "$POI_RAW/FieldCodes.docx"              "$POI_DEST/FieldCodes.docx"
fetch "$POI_RAW/bookmarks.docx"               "$POI_DEST/bookmarks.docx"
fetch "$POI_RAW/Numbering.docx"               "$POI_DEST/Numbering.docx"
fetch "$POI_RAW/headerFooter.docx"            "$POI_DEST/headerFooter.docx"

# Phase 2 additions — tables (Apache 2.0)
fetch "$POI_RAW/TestTableCellAlign.docx"      "$POI_DEST/TestTableCellAlign.docx"
fetch "$POI_RAW/TestTableColumns.docx"        "$POI_DEST/TestTableColumns.docx"
fetch "$POI_RAW/table-alignment.docx"         "$POI_DEST/table-alignment.docx"
fetch "$POI_RAW/table-indent.docx"            "$POI_DEST/table-indent.docx"
fetch "$POI_RAW/deep-table-cell.docx"         "$POI_DEST/deep-table-cell.docx"

# Phase 2 additions — lists/numbering (Apache 2.0)
fetch "$POI_RAW/ComplexNumberedLists.docx"    "$POI_DEST/ComplexNumberedLists.docx"
fetch "$POI_RAW/NumberingWOverrides.docx"     "$POI_DEST/NumberingWOverrides.docx"
fetch "$POI_RAW/NumberingWithOutOfOrderId.docx" "$POI_DEST/NumberingWithOutOfOrderId.docx"

# Phase 2 additions — multi-section / headers (Apache 2.0)
fetch "$POI_RAW/DiffFirstPageHeadFoot.docx"   "$POI_DEST/DiffFirstPageHeadFoot.docx"
fetch "$POI_RAW/PageSpecificHeadFoot.docx"    "$POI_DEST/PageSpecificHeadFoot.docx"

# Phase 3a additions — images, hyperlinks, shaded/bordered tables (Apache 2.0)
fetch "$POI_RAW/VariousPictures.docx"         "$POI_DEST/VariousPictures.docx"
fetch "$POI_RAW/61745.docx"                   "$POI_DEST/61745.docx"
fetch "$POI_RAW/56392.docx"                   "$POI_DEST/56392.docx"
fetch "$POI_RAW/headerPic.docx"               "$POI_DEST/headerPic.docx"
fetch "$POI_RAW/WithGIF.docx"                 "$POI_DEST/WithGIF.docx"
fetch "$POI_RAW/61991.docx"                   "$POI_DEST/61991.docx"
fetch "$POI_RAW/TestDocument.docx"            "$POI_DEST/TestDocument.docx"
fetch "$POI_RAW/bug57031.docx"                "$POI_DEST/bug57031.docx"
fetch "$POI_RAW/drawing.docx"                 "$POI_DEST/drawing.docx"
fetch "$POI_RAW/IllustrativeCases.docx"       "$POI_DEST/IllustrativeCases.docx"

# ---------------------------------------------------------------------------
echo ""
total=$(find "$FIXTURES" -name '*.docx' ! -name '~$*' | wc -l)
echo "Done. $total fixtures total in testdata/fixtures/."
