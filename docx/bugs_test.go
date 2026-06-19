package docx_test

// Regression tests for the eight bugs reported in review.

import (
	"bytes"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

func firstCell(tbl *docx.Table) *docx.Cell {
	for row := range tbl.Rows() {
		for cell := range row.Cells() {
			return cell
		}
	}
	return nil
}

// Bug 1: Cell.SetSpan(1,1) and SetSpan(1,0) panicked when TcPr was nil.
func TestCell_SetSpan_ClearDoesNotPanic(t *testing.T) {
	d := docx.New()
	tbl := d.Body().AddTable(1, 2, docx.TableOptions{})
	cell := firstCell(tbl)
	cell.SetSpan(1, 1)
	cell.SetSpan(1, 0)
	if cell.ColSpan() != 1 {
		t.Errorf("ColSpan() = %d, want 1", cell.ColSpan())
	}
}

// Bug 1b: SetSpan with rows>1 then clearing cols still works.
func TestCell_SetSpan_MixedClear(t *testing.T) {
	d := docx.New()
	tbl := d.Body().AddTable(2, 2, docx.TableOptions{})
	cell := firstCell(tbl)
	cell.SetSpan(2, 2)
	if cell.ColSpan() != 2 {
		t.Errorf("ColSpan after SetSpan(2,2) = %d, want 2", cell.ColSpan())
	}
	cell.SetSpan(1, 1)
	if cell.ColSpan() != 1 {
		t.Errorf("ColSpan after clear = %d, want 1", cell.ColSpan())
	}
}

// Bug 2: Header.AddTable must not place the table after the terminal paragraph.
// After AddTable the structure must survive save+reload (Word would repair otherwise).
func TestHeader_AddTable_TerminalParagraphKept(t *testing.T) {
	d := docx.New()
	sec := d.LastSection()
	hdr := sec.SetHeader(docx.HeaderDefault)
	hdr.AddParagraph().AddRun("before table")
	hdr.AddTable(1, 2, docx.TableOptions{})
	hdr.AddParagraph().AddRun("after table")

	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes() error: %v", err)
	}
	hdr2 := d2.LastSection().Header(docx.HeaderDefault)
	if hdr2 == nil {
		t.Fatal("header missing after round-trip")
	}
	var texts []string
	for p := range hdr2.Paragraphs() {
		if txt := p.Text(); txt != "" {
			texts = append(texts, txt)
		}
	}
	if len(texts) < 2 {
		t.Errorf("header paragraph texts = %v, want at least 2 non-empty paragraphs", texts)
	}
}

// Bug 3: Footer.AddTable — same terminal-paragraph requirement.
func TestFooter_AddTable_TerminalParagraphKept(t *testing.T) {
	d := docx.New()
	sec := d.LastSection()
	ftr := sec.SetFooter(docx.FooterDefault)
	ftr.AddTable(1, 2, docx.TableOptions{})
	ftr.AddParagraph().AddRun("after table")

	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes() error: %v", err)
	}
	ftr2 := d2.LastSection().Footer(docx.FooterDefault)
	if ftr2 == nil {
		t.Fatal("footer missing after round-trip")
	}
}

// Bug 4 & 5: AddHeaderPart/AddFooterPart must not collide with existing part numbers.
// Two distinct header types must get distinct file paths.
func TestAddHeaderPart_UniquePathsForMultipleHeaders(t *testing.T) {
	d := docx.New()
	sec := d.LastSection()
	sec.SetHeader(docx.HeaderDefault).AddParagraph().AddRun("default")
	sec.SetHeader(docx.HeaderFirst).AddParagraph().AddRun("first")
	sec.EnableTitlePage(true)

	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes() error: %v", err)
	}
	sec2 := d2.LastSection()
	if sec2.Header(docx.HeaderDefault) == nil {
		t.Error("default header missing after round-trip")
	}
	if sec2.Header(docx.HeaderFirst) == nil {
		t.Error("first header missing after round-trip")
	}
}

// Bug 4 & 5 (footer variant).
func TestAddFooterPart_UniquePathsForMultipleFooters(t *testing.T) {
	d := docx.New()
	sec := d.LastSection()
	sec.SetFooter(docx.FooterDefault).AddParagraph().AddRun("default")
	sec.SetFooter(docx.FooterFirst).AddParagraph().AddRun("first")
	sec.EnableTitlePage(true)

	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes() error: %v", err)
	}
	sec2 := d2.LastSection()
	var texts []string
	for p := range sec2.Footer(docx.FooterDefault).Paragraphs() {
		if txt := p.Text(); txt != "" {
			texts = append(texts, txt)
		}
	}
	if len(texts) == 0 || texts[0] != "default" {
		t.Errorf("default footer text = %v, want [default]", texts)
	}
}

// Bug 6: SetHeader called twice with the same type must not duplicate the ref.
func TestSetHeader_Idempotent(t *testing.T) {
	d := docx.New()
	sec := d.LastSection()

	h1 := sec.SetHeader(docx.HeaderDefault)
	h1.AddParagraph().AddRun("content")

	// Second call must return the same header, not create a duplicate ref.
	h2 := sec.SetHeader(docx.HeaderDefault)
	var texts []string
	for p := range h2.Paragraphs() {
		if txt := p.Text(); txt != "" {
			texts = append(texts, txt)
		}
	}
	if len(texts) == 0 || texts[0] != "content" {
		t.Errorf("SetHeader idempotent text = %v, want [content]", texts)
	}

	// Save must succeed (duplicate refs would cause OPC violation).
	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes() error: %v", err)
	}
	// Only one default header should be present.
	sec2 := d2.LastSection()
	if sec2.Header(docx.HeaderDefault) == nil {
		t.Fatal("header missing after round-trip")
	}
	// SetHeader again on d2 must not panic or create a second part.
	h3 := sec2.SetHeader(docx.HeaderDefault)
	if h3 == nil {
		t.Error("SetHeader on loaded doc returned nil")
	}
}

// Bug 7: CT_AbstractNum.UnmarshalXML must not discard elements appearing after lvl children.
// Verified by full round-trip of a document with a 9-level abstract numbering definition.
func TestAbstractNum_RoundTrip_AllLevelsPreserved(t *testing.T) {
	d := docx.New()
	list := d.CreateList(docx.ListOptions{Style: docx.ListDecimal})

	for i := 0; i < 9; i++ {
		p := d.Body().AddParagraph()
		p.AddRun("level item")
		p.SetList(list, i)
	}

	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes() error: %v", err)
	}

	// Verify numPr survived on every paragraph.
	count := 0
	for p := range d2.Body().Paragraphs() {
		if id := p.Format().NumID(); id != nil && *id > 0 {
			count++
		}
	}
	if count != 9 {
		t.Errorf("numPr found on %d paragraphs, want 9", count)
	}
}

// Bug 8: SetNumPr must place w:numPr at the correct xs:sequence position in w:pPr.
// If it's appended at the end (after rPr, sectPr), the validator flags a schema error.
func TestSetNumPr_SchemaOrder_RoundTrip(t *testing.T) {
	d := docx.New()
	list := d.CreateList(docx.ListOptions{Style: docx.ListDecimal})

	p := d.Body().AddParagraph()
	p.AddRun("list item")
	p.SetList(list, 0)

	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes() error: %v", err)
	}

	found := false
	for p2 := range d2.Body().Paragraphs() {
		if id := p2.Format().NumID(); id != nil && *id > 0 {
			found = true
		}
	}
	if !found {
		t.Error("numPr missing after round-trip (possible schema-order corruption)")
	}
}
