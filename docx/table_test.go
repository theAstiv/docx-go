package docx_test

import (
	"bytes"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

func TestBody_AddTable_Basic(t *testing.T) {
	d := docx.New()
	tbl := d.Body().AddTable(2, 3, docx.TableOptions{})
	if tbl == nil {
		t.Fatal("AddTable() returned nil")
	}

	var rowCount int
	for row := range tbl.Rows() {
		rowCount++
		var cellCount int
		for range row.Cells() {
			cellCount++
		}
		if cellCount != 3 {
			t.Errorf("row has %d cells, want 3", cellCount)
		}
	}
	if rowCount != 2 {
		t.Errorf("table has %d rows, want 2", rowCount)
	}
}

func TestBody_AddTable_WithStyle(t *testing.T) {
	d := docx.New()
	tbl := d.Body().AddTable(1, 2, docx.TableOptions{Style: "TableGrid"})
	if got := tbl.Style(); got != "TableGrid" {
		t.Errorf("Style() = %q, want %q", got, "TableGrid")
	}
}

func TestBody_Tables_Iterator(t *testing.T) {
	d := docx.New()
	d.Body().AddParagraph().AddRun("before")
	d.Body().AddTable(1, 1, docx.TableOptions{})
	d.Body().AddTable(2, 2, docx.TableOptions{})
	d.Body().AddParagraph().AddRun("after")

	var count int
	for range d.Body().Tables() {
		count++
	}
	if count != 2 {
		t.Errorf("Tables() yielded %d tables, want 2", count)
	}

	// Paragraphs iterator should skip tables.
	var paraTexts []string
	for p := range d.Body().Paragraphs() {
		paraTexts = append(paraTexts, p.Text())
	}
	if len(paraTexts) != 2 {
		t.Errorf("Paragraphs() yielded %d items, want 2", len(paraTexts))
	}
}

func TestTable_AddRow(t *testing.T) {
	d := docx.New()
	tbl := d.Body().AddTable(1, 2, docx.TableOptions{})

	// Add a second row dynamically.
	row2 := tbl.AddRow()
	if row2 == nil {
		t.Fatal("AddRow() returned nil")
	}
	row2.AddCell()
	row2.AddCell()

	var rowCount int
	for range tbl.Rows() {
		rowCount++
	}
	if rowCount != 2 {
		t.Errorf("after AddRow table has %d rows, want 2", rowCount)
	}
}

func TestTable_AddRow_Header(t *testing.T) {
	d := docx.New()
	tbl := d.Body().AddTable(0, 2, docx.TableOptions{})
	hdr := tbl.AddRowWithOptions(docx.RowOptions{Header: true})
	if !hdr.IsHeader() {
		t.Error("AddRowWithOptions(Header:true).IsHeader() should be true")
	}
	body := tbl.AddRow()
	if body.IsHeader() {
		t.Error("AddRow().IsHeader() should be false")
	}
}

func TestCell_Text(t *testing.T) {
	d := docx.New()
	tbl := d.Body().AddTable(1, 2, docx.TableOptions{})
	var cells []*docx.Cell
	for row := range tbl.Rows() {
		for cell := range row.Cells() {
			cells = append(cells, cell)
		}
	}
	if len(cells) < 2 {
		t.Fatal("expected at least 2 cells")
	}
	// Write text into cells via Paragraphs.
	for p := range cells[0].Paragraphs() {
		p.AddRun("hello")
	}
	for p := range cells[1].Paragraphs() {
		p.AddRun("world")
	}
	if got := cells[0].Text(); got != "hello" {
		t.Errorf("cell[0].Text() = %q, want %q", got, "hello")
	}
	if got := cells[1].Text(); got != "world" {
		t.Errorf("cell[1].Text() = %q, want %q", got, "world")
	}
}

func TestCell_AddParagraph(t *testing.T) {
	d := docx.New()
	tbl := d.Body().AddTable(1, 1, docx.TableOptions{})
	var cell *docx.Cell
	for row := range tbl.Rows() {
		for c := range row.Cells() {
			cell = c
		}
	}
	// A new cell holds one empty paragraph; AddParagraph appends after it,
	// as Body.AddParagraph does.
	p1 := cell.AddParagraph()
	p1.AddRun("first")
	p2 := cell.AddParagraph()
	p2.AddRun("second")

	if got := cell.Text(); got != "\nfirst\nsecond" {
		t.Errorf("cell.Text() = %q, want %q", got, "\nfirst\nsecond")
	}
}

func TestCell_SetText(t *testing.T) {
	d := docx.New()
	tbl := d.Body().AddTable(1, 1, docx.TableOptions{})
	var cell *docx.Cell
	for row := range tbl.Rows() {
		for c := range row.Cells() {
			cell = c
		}
	}
	for p := range cell.Paragraphs() {
		p.Format().SetAlignment(docx.AlignCenter)
	}
	cell.AddParagraph().AddRun("old")

	cell.SetText("new")
	ps := collectParagraphs(cell)
	if len(ps) != 1 || ps[0].Text() != "new" {
		t.Fatalf("paragraphs = %d, text %q; want one paragraph %q", len(ps), cell.Text(), "new")
	}
	if ps[0].Format().Alignment() != docx.AlignCenter {
		t.Error("SetText dropped the first paragraph's formatting")
	}

	cell.SetText("")
	if ps := collectParagraphs(cell); len(ps) != 1 || ps[0].Text() != "" {
		t.Errorf("SetText(\"\") left %d paragraphs, text %q; want one empty paragraph", len(ps), cell.Text())
	}
}

func collectParagraphs(c *docx.Cell) []*docx.Paragraph {
	var out []*docx.Paragraph
	for p := range c.Paragraphs() {
		out = append(out, p)
	}
	return out
}

func TestCell_ColSpan(t *testing.T) {
	d := docx.New()
	tbl := d.Body().AddTable(1, 3, docx.TableOptions{})

	var cells []*docx.Cell
	for row := range tbl.Rows() {
		for cell := range row.Cells() {
			cells = append(cells, cell)
		}
	}

	if got := cells[0].ColSpan(); got != 1 {
		t.Errorf("ColSpan() on fresh cell = %d, want 1", got)
	}
	cells[0].SetSpan(1, 2)
	if got := cells[0].ColSpan(); got != 2 {
		t.Errorf("ColSpan() after SetSpan(1,2) = %d, want 2", got)
	}
}

func TestCell_VMerge(t *testing.T) {
	d := docx.New()
	tbl := d.Body().AddTable(2, 1, docx.TableOptions{})

	var rows []*docx.Row
	for row := range tbl.Rows() {
		rows = append(rows, row)
	}

	var cells0, cells1 []*docx.Cell
	for c := range rows[0].Cells() {
		cells0 = append(cells0, c)
	}
	for c := range rows[1].Cells() {
		cells1 = append(cells1, c)
	}

	if cells0[0].VMerge() != docx.VMergeNone {
		t.Error("VMerge() on fresh cell should be VMergeNone")
	}

	cells0[0].SetVMerge(docx.VMergeStart)
	if got := cells0[0].VMerge(); got != docx.VMergeStart {
		t.Errorf("VMerge() after SetVMerge(Start) = %v, want VMergeStart", got)
	}

	cells1[0].SetVMerge(docx.VMergeContinue)
	if got := cells1[0].VMerge(); got != docx.VMergeContinue {
		t.Errorf("VMerge() after SetVMerge(Continue) = %v, want VMergeContinue", got)
	}
}

func TestTable_SaveRoundTrip(t *testing.T) {
	d := docx.New()
	tbl := d.Body().AddTable(2, 2, docx.TableOptions{Style: "TableGrid"})

	// Fill cells with text.
	row0 := firstRow(t, tbl)
	fillRowCells(row0, "A", "B")
	row1 := nthRow(t, tbl, 1)
	fillRowCells(row1, "C", "D")

	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes() error: %v", err)
	}

	var tables []*docx.Table
	for tbl2 := range d2.Body().Tables() {
		tables = append(tables, tbl2)
	}
	if len(tables) != 1 {
		t.Fatalf("reopened doc has %d tables, want 1", len(tables))
	}
	if got := tables[0].Style(); got != "TableGrid" {
		t.Errorf("Style() after round-trip = %q, want %q", got, "TableGrid")
	}

	var rowCount int
	for row := range tables[0].Rows() {
		var cellTexts []string
		for cell := range row.Cells() {
			cellTexts = append(cellTexts, cell.Text())
		}
		if len(cellTexts) != 2 {
			t.Errorf("row %d has %d cells, want 2", rowCount, len(cellTexts))
		}
		rowCount++
	}
	if rowCount != 2 {
		t.Errorf("reopened table has %d rows, want 2", rowCount)
	}
}

func TestTable_FixtureRoundTrip_Tables(t *testing.T) {
	// The fixture round-trip harness in roundtrip_test.go covers real .docx
	// files including table-heavy ones from the Apache POI corpus.
	// This test verifies that a document with both paragraphs and tables
	// survives Save → OpenBytes with no content loss.
	d := docx.New()
	d.Body().AddParagraph().AddRun("intro")
	tbl := d.Body().AddTable(1, 2, docx.TableOptions{})
	for row := range tbl.Rows() {
		cells := make([]*docx.Cell, 0, 2)
		for c := range row.Cells() {
			cells = append(cells, c)
		}
		if len(cells) >= 1 {
			for p := range cells[0].Paragraphs() {
				p.AddRun("left")
			}
		}
		if len(cells) >= 2 {
			for p := range cells[1].Paragraphs() {
				p.AddRun("right")
			}
		}
	}
	d.Body().AddParagraph().AddRun("outro")

	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes() error: %v", err)
	}

	// Verify paragraph order is preserved (intro, outro visible; table in between).
	var paraTexts []string
	for p := range d2.Body().Paragraphs() {
		paraTexts = append(paraTexts, p.Text())
	}
	if len(paraTexts) != 2 || paraTexts[0] != "intro" || paraTexts[1] != "outro" {
		t.Errorf("paragraphs after round-trip = %v, want [intro outro]", paraTexts)
	}

	var tables []*docx.Table
	for tbl2 := range d2.Body().Tables() {
		tables = append(tables, tbl2)
	}
	if len(tables) != 1 {
		t.Fatalf("tables after round-trip = %d, want 1", len(tables))
	}
}

// ── helpers ───────────────────────────────────────────────────────────────

func firstRow(t *testing.T, tbl *docx.Table) *docx.Row {
	t.Helper()
	return nthRow(t, tbl, 0)
}

func nthRow(t *testing.T, tbl *docx.Table, n int) *docx.Row {
	t.Helper()
	var i int
	for row := range tbl.Rows() {
		if i == n {
			return row
		}
		i++
	}
	t.Fatalf("table has fewer than %d rows", n+1)
	return nil
}

func fillRowCells(row *docx.Row, texts ...string) {
	i := 0
	for cell := range row.Cells() {
		if i < len(texts) {
			for p := range cell.Paragraphs() {
				p.AddRun(texts[i])
			}
		}
		i++
	}
}
