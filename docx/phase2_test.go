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

// TestExitGate_TableWithMerges creates a 3×3 table from scratch with a
// column-span cell (row 0 col 0 spans 2 cols) and a vertical merge group
// (col 2 rows 1–2), saves, reopens, and verifies that both merge attributes
// survived the round-trip unchanged.
func TestExitGate_TableWithMerges(t *testing.T) {
	d := docx.New()
	tbl := d.Body().AddTable(3, 3, docx.TableOptions{})

	// Collect rows for indexed access.
	var rows []*docx.Row
	for r := range tbl.Rows() {
		rows = append(rows, r)
	}

	// Row 0 col 0: span 2 columns.
	var cells0 []*docx.Cell
	for c := range rows[0].Cells() {
		cells0 = append(cells0, c)
	}
	cells0[0].SetSpan(1, 2)

	// Row 1 col 2: start vertical merge group.
	var cells1 []*docx.Cell
	for c := range rows[1].Cells() {
		cells1 = append(cells1, c)
	}
	cells1[2].SetVMerge(docx.VMergeStart)

	// Row 2 col 2: continue vertical merge group.
	var cells2 []*docx.Cell
	for c := range rows[2].Cells() {
		cells2 = append(cells2, c)
	}
	cells2[2].SetVMerge(docx.VMergeContinue)

	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}

	var rows2 []*docx.Row
	for tbl2 := range d2.Body().Tables() {
		for r := range tbl2.Rows() {
			rows2 = append(rows2, r)
		}
	}
	if len(rows2) < 3 {
		t.Fatalf("expected 3 rows after round-trip, got %d", len(rows2))
	}

	var rtCells0 []*docx.Cell
	for c := range rows2[0].Cells() {
		rtCells0 = append(rtCells0, c)
	}
	if rtCells0[0].ColSpan() != 2 {
		t.Errorf("row0 col0 ColSpan = %d, want 2", rtCells0[0].ColSpan())
	}

	var rtCells1, rtCells2 []*docx.Cell
	for c := range rows2[1].Cells() {
		rtCells1 = append(rtCells1, c)
	}
	for c := range rows2[2].Cells() {
		rtCells2 = append(rtCells2, c)
	}
	if rtCells1[2].VMerge() != docx.VMergeStart {
		t.Errorf("row1 col2 VMerge = %v, want VMergeStart", rtCells1[2].VMerge())
	}
	if rtCells2[2].VMerge() != docx.VMergeContinue {
		t.Errorf("row2 col2 VMerge = %v, want VMergeContinue", rtCells2[2].VMerge())
	}
}

// TestExitGate_ListRestart creates a 5-item numbered list where items 4–5 use
// RestartAt(0, 1) to reset the counter, saves, reopens, and verifies that the
// restart paragraphs received a different numId from the original paragraphs.
func TestExitGate_ListRestart(t *testing.T) {
	d := docx.New()
	list := d.CreateList(docx.ListOptions{Style: docx.ListDecimal})

	// Items 1–3: use original list.
	for range 3 {
		p := d.Body().AddParagraph()
		p.SetList(list, 0)
	}

	// Items 4–5: restart at 1.
	restarted := list.RestartAt(0, 1)
	for range 2 {
		p := d.Body().AddParagraph()
		p.SetList(restarted, 0)
	}

	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}

	var numIDs []int64
	for p := range d2.Body().Paragraphs() {
		if id := p.Format().NumID(); id != nil {
			numIDs = append(numIDs, *id)
		}
	}
	if len(numIDs) < 5 {
		t.Fatalf("expected at least 5 numbered paragraphs, got %d", len(numIDs))
	}
	origID := numIDs[0]
	restartID := numIDs[3]
	if origID == restartID {
		t.Errorf("expected different numIds for original vs restarted list, both got %d", origID)
	}
	// Items 1–3 share origID; items 4–5 share restartID.
	for i := 0; i < 3; i++ {
		if numIDs[i] != origID {
			t.Errorf("item[%d] numId = %d, want %d", i, numIDs[i], origID)
		}
	}
	for i := 3; i < 5; i++ {
		if numIDs[i] != restartID {
			t.Errorf("item[%d] numId = %d, want %d", i, numIDs[i], restartID)
		}
	}
}

// TestExitGate_MultiSection splits a document into two sections by embedding a
// w:sectPr in a paragraph's pPr, saves, reopens, and verifies that both
// sections retain their page geometry.
func TestExitGate_MultiSection(t *testing.T) {
	d := docx.New()

	// Section 1: A4 (11906 × 16838 twips) — distinct from DefaultSectPr (US Letter).
	p1 := d.Body().AddParagraph()
	p1.AddRun("section one content")
	sec1 := p1.AddSectionBreak()
	sec1.SetPageSize(11906, 16838)

	// Section 2 (final): Legal (12240 × 20160 twips).
	p2 := d.Body().AddParagraph()
	p2.AddRun("section two content")
	d.LastSection().SetPageSize(12240, 20160)

	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}

	// Verify section 1 geometry via the paragraph's embedded pPr.sectPr.
	var paras []*docx.Paragraph
	for p := range d2.Body().Paragraphs() {
		paras = append(paras, p)
	}
	if len(paras) < 2 {
		t.Fatalf("expected at least 2 paragraphs, got %d", len(paras))
	}
	// Verify section 1 geometry: the paragraph must carry an embedded sectPr
	// with A4 dimensions.  SectionBreak() returns nil if the sectPr was not
	// persisted, which would fail the nil check below.
	sec1rt := paras[0].SectionBreak()
	if sec1rt == nil {
		t.Fatal("section 1 paragraph lost its embedded sectPr after round-trip")
	}
	if w := sec1rt.PageWidth(); w == nil || *w != 11906 {
		t.Errorf("section1 PageWidth = %v, want 11906 (A4)", w)
	}
	if h := sec1rt.PageHeight(); h == nil || *h != 16838 {
		t.Errorf("section1 PageHeight = %v, want 16838 (A4)", h)
	}

	// Verify section 2 (final) geometry: Legal page.
	sec2 := d2.LastSection()
	if w := sec2.PageWidth(); w == nil || *w != 12240 {
		t.Errorf("section2 PageWidth = %v, want 12240 (Legal)", w)
	}
	if h := sec2.PageHeight(); h == nil || *h != 20160 {
		t.Errorf("section2 PageHeight = %v, want 20160 (Legal)", h)
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
