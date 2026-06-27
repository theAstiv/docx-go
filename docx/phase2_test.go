package docx_test

// Phase 2 exit gate tests — tables, lists, multi-section.
// These complement the generic TestRoundTrip_Fixtures harness with
// Phase-2-specific structural assertions: table row/cell counts, numbering
// ID preservation, and section geometry (page size, TitlePg flag).

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

// TestMutationFixture_TableStructure opens a table fixture, mutates a cell
// paragraph, saves, reopens, and verifies table structure (row/cell count)
// is unchanged.
func TestMutationFixture_TableStructure(t *testing.T) {
	const path = "../testdata/fixtures/poi/TestTableCellAlign.docx"
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		t.Skipf("fixture not found: %s", path)
	}
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	d, err := docx.OpenBytes(data)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// Count original table structure.
	type tableShape struct{ rows, cols int }
	origShapes := collectTableShapes(d)
	if len(origShapes) == 0 {
		t.Skip("fixture has no tables")
	}

	// Mutation: append a paragraph to the document body.
	d.Body().AddParagraph().AddRun("phase2 marker")

	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("save: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}

	rtShapes := collectTableShapes(d2)
	if len(rtShapes) != len(origShapes) {
		t.Fatalf("table count changed: original=%d round-tripped=%d",
			len(origShapes), len(rtShapes))
	}
	for i, orig := range origShapes {
		rt := rtShapes[i]
		if rt.rows != orig.rows {
			t.Errorf("table[%d] row count changed: %d → %d", i, orig.rows, rt.rows)
		}
		if rt.cols != orig.cols {
			t.Errorf("table[%d] col count changed: %d → %d", i, orig.cols, rt.cols)
		}
	}
}

func collectTableShapes(d *docx.Document) []struct{ rows, cols int } {
	var out []struct{ rows, cols int }
	for tbl := range d.Body().Tables() {
		var rows, maxCols int
		for row := range tbl.Rows() {
			rows++
			var cells int
			for range row.Cells() {
				cells++
			}
			if cells > maxCols {
				maxCols = cells
			}
		}
		out = append(out, struct{ rows, cols int }{rows, maxCols})
	}
	return out
}

// TestMutationFixture_SectionGeometry opens a multi-section fixture that uses a
// first-page header, mutates a paragraph, saves, reopens, and verifies that
// the TitlePg flag and page geometry are preserved.
func TestMutationFixture_SectionGeometry(t *testing.T) {
	const path = "../testdata/fixtures/poi/DiffFirstPageHeadFoot.docx"
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		t.Skipf("fixture not found: %s", path)
	}
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	d, err := docx.OpenBytes(data)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	sec := d.LastSection()
	origTitlePg := sec.TitlePg()
	origWidth := sec.PageWidth()
	origHeight := sec.PageHeight()

	// Mutation: add a paragraph.
	d.Body().AddParagraph().AddRun("section geometry marker")

	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("save: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}

	sec2 := d2.LastSection()
	if sec2.TitlePg() != origTitlePg {
		t.Errorf("TitlePg changed: %v → %v", origTitlePg, sec2.TitlePg())
	}
	if origWidth != nil {
		if got := sec2.PageWidth(); got == nil || *got != *origWidth {
			t.Errorf("PageWidth changed: %v → %v", origWidth, got)
		}
	}
	if origHeight != nil {
		if got := sec2.PageHeight(); got == nil || *got != *origHeight {
			t.Errorf("PageHeight changed: %v → %v", origHeight, got)
		}
	}
}

// TestMutationFixture_NumberingPreserved opens a complex numbering fixture,
// counts paragraphs with list IDs, mutates (adds a plain paragraph), saves,
// reopens, and verifies that the numbered paragraph count and IDs are unchanged.
func TestMutationFixture_NumberingPreserved(t *testing.T) {
	const path = "../testdata/fixtures/poi/ComplexNumberedLists.docx"
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		t.Skipf("fixture not found: %s", path)
	}
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	d, err := docx.OpenBytes(data)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	var origNumIDs []int64
	for p := range d.Body().Paragraphs() {
		if id := p.Format().NumID(); id != nil {
			origNumIDs = append(origNumIDs, *id)
		}
	}
	if len(origNumIDs) == 0 {
		t.Skip("fixture has no numbered paragraphs")
	}

	// Mutation: add a plain paragraph (not a list item).
	d.Body().AddParagraph().AddRun("plain after list")

	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("save: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}

	var rtNumIDs []int64
	for p := range d2.Body().Paragraphs() {
		if id := p.Format().NumID(); id != nil {
			rtNumIDs = append(rtNumIDs, *id)
		}
	}
	if len(rtNumIDs) != len(origNumIDs) {
		t.Fatalf("numbered paragraph count changed: %d → %d", len(origNumIDs), len(rtNumIDs))
	}
	for i := range origNumIDs {
		if rtNumIDs[i] != origNumIDs[i] {
			t.Errorf("para[%d] numId changed: %d → %d", i, origNumIDs[i], rtNumIDs[i])
		}
	}
}
