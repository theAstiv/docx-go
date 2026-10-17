package docx_test

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

// TestGenerateValidationDocs writes documents that use every Phase 3a feature
// to the directory named by DOCX_VALIDATION_DIR, for scripts/validate-
// generated.sh to run through the OOXML schema validator. Fixture-based
// validation only re-saves existing files; this is what checks content docx-go
// itself creates. Without the variable the test is skipped.
func TestGenerateValidationDocs(t *testing.T) {
	dir := os.Getenv("DOCX_VALIDATION_DIR")
	if dir == "" {
		t.Skip("DOCX_VALIDATION_DIR not set")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	write := func(name string, d *docx.Document) {
		t.Helper()
		var buf bytes.Buffer
		if err := d.Save(&buf); err != nil {
			t.Fatalf("%s: save: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	write("gen-baseline.docx", baselineDoc())
	write("gen-features.docx", featuresDoc(t))
	write("gen-formatting-order.docx", formattingOrderDoc())
}

// formattingOrderDoc sets every paragraph and run property in reverse schema
// order, on a paragraph, a run, and a style. A setter that appends instead of
// inserting at its xs:sequence position produces a schema error here.
func formattingOrderDoc() *docx.Document {
	d := docx.New()
	st := d.Styles()
	st.EnsureBuiltin(docx.StyleHeading1)

	applyPara := func(f *docx.ParagraphFormat) {
		line := docx.LineSpacing240(276)
		f.SetMirrorIndents(docx.On())
		f.SetContextualSpacing(docx.On())
		f.SetHangingIndent(tw(100))
		f.SetFirstLineIndent(nil)
		f.SetRightIndent(tw(200))
		f.SetLeftIndent(tw(300))
		f.SetLineSpacingAuto(line)
		after, before := docx.Twip(120), docx.Twip(240)
		f.SetSpaceAfter(&after)
		f.SetSpaceBefore(&before)
		f.SetAlignment(docx.AlignJustify)
		lvl := int64(1)
		f.SetOutlineLevel(&lvl)
		f.SetSuppressLineNumbers(docx.On())
		f.SetWidowControl(docx.On())
		f.SetPageBreakBefore(docx.On())
		f.SetKeepLines(docx.On())
		f.SetKeepNext(docx.On())
		f.SetStyle("Heading1")
	}
	applyFont := func(f *docx.RunFont) {
		f.SetVertAlign(docx.VertAlignSuperscript)
		f.SetUnderline(docx.UnderlineSingle)
		f.SetColor("336699")
		f.SetSizePt(13)
		f.SetStrike(docx.On())
		f.SetItalic(docx.On())
		f.SetBold(docx.On())
		f.SetName("Arial")
		f.SetStyle("Strong")
	}

	p := d.Body().AddParagraph()
	applyPara(p.Format())
	r := p.AddRun("ordered")
	applyFont(r.Font())

	s := st.AddParagraphStyle("Probe", "Probe", docx.StyleOptions{})
	applyPara(s.Format())
	applyFont(s.Font())
	// A style's own pPr/rPr cannot reference another style.
	s.Format().SetStyle("")
	s.Font().SetStyle("")
	st.DocDefaultsFont().SetSizePt(10)
	st.DocDefaultsFormat().SetKeepNext(docx.On())
	return d
}

func baselineDoc() *docx.Document {
	d := docx.New()
	d.Body().AddParagraph().AddRun("Hello")
	return d
}

func jpegBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 24, 12)), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func gifBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	pal := image.NewPaletted(image.Rect(0, 0, 16, 16), color.Palette{color.Black, color.White})
	if err := gif.Encode(&buf, pal, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// featuresDoc exercises: styles (built-in, custom paragraph/character/table,
// docDefaults), bookmarks, internal and external links, images in the body, a
// table cell, a header and a hyperlink, PNG/JPEG/GIF, formatted tables, and
// fields (generic, PAGE/NUMPAGES, TOC, updateFields).
func featuresDoc(t *testing.T) *docx.Document {
	d := docx.New()
	body := d.Body()
	st := d.Styles()

	for _, b := range []docx.BuiltinStyle{
		docx.StyleNormal, docx.StyleTitle, docx.StyleHeading1, docx.StyleHeading2, docx.StyleHeading3,
		docx.StyleHyperlink, docx.StyleTableGrid, docx.StyleTOC1, docx.StyleTOC2, docx.StyleTOC3, docx.StyleCaption,
	} {
		st.EnsureBuiltin(b)
	}
	prio := int64(20)
	callout := st.AddParagraphStyle("Callout", "Callout", docx.StyleOptions{
		BasedOn: "Normal", Next: "Callout", UIPriority: &prio, QuickStyle: true,
	})
	callout.Font().SetBold(docx.On())
	callout.Font().SetColor("C00000")
	callout.Format().SetKeepNext(docx.On())
	st.AddCharacterStyle("Code", "Code", docx.StyleOptions{BasedOn: "DefaultParagraphFont"}).Font().SetName("Consolas")
	st.AddTableStyle("Report", "Report", docx.StyleOptions{BasedOn: "TableNormal"})
	st.DocDefaultsFont().SetSizePt(11)
	after := docx.Pt(6)
	st.DocDefaultsFormat().SetSpaceAfter(&after)

	// Headings with bookmarks, a TOC, links.
	if _, err := d.AddTOC(docx.TOCOptions{Hyperlinks: true}); err != nil {
		t.Fatal(err)
	}
	h := body.AddParagraph()
	h.Format().SetStyle("Heading1")
	h.AddRun("Introduction")
	if _, err := h.AddBookmark("intro"); err != nil {
		t.Fatal(err)
	}
	p := body.AddParagraph()
	p.Format().SetStyle("Callout")
	p.AddRun("See ")
	il, err := p.AddInternalLink("intro", docx.HyperlinkOptions{Tooltip: "back"})
	if err != nil {
		t.Fatal(err)
	}
	il.AddRun("the introduction")
	p.AddRun(" and ")
	el, err := p.AddHyperlink("https://example.com/a?b=1&c=2", docx.HyperlinkOptions{History: docx.Off()})
	if err != nil {
		t.Fatal(err)
	}
	el.AddRun("an external page")
	p.AddField(`REF intro \h`, "Introduction")

	// Images: PNG, JPEG, GIF in the body, one inside a hyperlink.
	pic := body.AddParagraph()
	for _, data := range [][]byte{pngBytes(t, 40, 20, 5), jpegBytes(t), gifBytes(t)} {
		if _, err := pic.AddRun("").AddImage(bytes.NewReader(data), docx.ImageOptions{AltText: "sample", Title: "t"}); err != nil {
			t.Fatal(err)
		}
	}
	linked, err := body.AddParagraph().AddHyperlink("https://example.com/img", docx.HyperlinkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := linked.AddRun("").AddImage(bytes.NewReader(pngBytes(t, 8, 8, 9)), docx.ImageOptions{AltText: "linked"}); err != nil {
		t.Fatal(err)
	}

	// Formatted table: borders, widths in each unit, margins, layout, shading,
	// merged cell, header and cantSplit rows, an image in a cell.
	tbl := body.AddTable(3, 3, docx.TableOptions{Style: "TableGrid", ColWidth: []docx.Twip{2000, 3000, 3000}})
	tbl.SetWidth(docx.WidthPercent(docx.Percent(100)))
	tbl.SetLayout(docx.LayoutFixed)
	edge := &docx.Border{Style: docx.BorderSingle, Width: 4, Color: "808080"}
	tbl.SetBorders(docx.BorderOptions{Top: edge, Left: edge, Bottom: edge, Right: edge, InsideH: edge, InsideV: edge})
	pad := docx.Pt(3)
	tbl.SetCellMargins(docx.CellMargins{Top: &pad, Left: &pad, Bottom: &pad, Right: &pad})
	var rows []*docx.Row
	for r := range tbl.Rows() {
		rows = append(rows, r)
	}
	rows[0].SetHeader(true)
	rows[0].SetCantSplit(docx.On())
	var c00, c01, c11 *docx.Cell
	for i, c := range cellsIn(rows[0]) {
		c.SetShading("1F3864")
		if i == 0 {
			c00 = c
		}
		if i == 1 {
			c01 = c
		}
	}
	c00.SetWidth(docx.WidthTwips(2000))
	c01.SetWidth(docx.WidthPercent(docx.Percent(40)))
	cellsIn(rows[0])[2].SetWidth(docx.WidthAuto())
	c00.SetBorders(docx.BorderOptions{
		Top:   &docx.Border{Style: docx.BorderDouble, Width: 6},
		TL2BR: &docx.Border{Style: docx.BorderSingle, Width: 4},
	})
	c11 = cellsIn(rows[1])[1]
	c11.SetSpan(1, 2)
	if _, err := c11.AddParagraph().AddRun("").AddImage(bytes.NewReader(pngBytes(t, 10, 10, 3)), docx.ImageOptions{AltText: "in cell"}); err != nil {
		t.Fatal(err)
	}

	// Header with an image and a link; footer with Page X of Y.
	hdr := d.LastSection().SetHeader(docx.HeaderDefault)
	hp := hdr.AddParagraph()
	if _, err := hp.AddRun("").AddImage(bytes.NewReader(pngBytes(t, 20, 10, 1)), docx.ImageOptions{AltText: "logo"}); err != nil {
		t.Fatal(err)
	}
	hl, err := hp.AddHyperlink("https://example.com/h", docx.HyperlinkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	hl.AddRun("home")
	ftr := d.LastSection().SetFooter(docx.FooterDefault)
	fp := ftr.AddParagraph()
	fp.AddRun("Page ")
	fp.AddPageNumber()
	fp.AddRun(" of ")
	fp.AddPageCount()

	d.SetUpdateFieldsOnOpen(true)
	return d
}

func cellsIn(r *docx.Row) []*docx.Cell {
	var out []*docx.Cell
	for c := range r.Cells() {
		out = append(out, c)
	}
	return out
}
