package docx_test

import (
	"os"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

func tw(v int64) *docx.Twip { t := docx.Twip(v); return &t }

func TestTable_BordersRoundTrip(t *testing.T) {
	d := docx.New()
	tbl := d.Body().AddTable(2, 2, docx.TableOptions{Style: "TableGrid"})
	single := &docx.Border{Style: docx.BorderSingle, Width: 8, Color: "1F3864"}
	dotted := &docx.Border{Style: docx.BorderDotted, Width: 4, Space: 1}
	tbl.SetBorders(docx.BorderOptions{Top: single, Bottom: single, InsideH: dotted})

	got := tbl.Borders()
	if got.Top == nil || got.Top.Width != 8 || got.Top.Color != "1F3864" || got.Top.Style != docx.BorderSingle {
		t.Errorf("Top = %+v", got.Top)
	}
	if got.Left != nil || got.InsideV != nil {
		t.Error("edges that were not set should stay nil")
	}

	d2 := reopen(t, d)
	for tb := range d2.Body().Tables() {
		b := tb.Borders()
		if b.Bottom == nil || b.Bottom.Width != 8 {
			t.Errorf("Bottom after round-trip = %+v", b.Bottom)
		}
		if b.InsideH == nil || b.InsideH.Style != docx.BorderDotted || b.InsideH.Space != 1 {
			t.Errorf("InsideH after round-trip = %+v", b.InsideH)
		}
		if tb.Style() != "TableGrid" {
			t.Errorf("table style lost: %q", tb.Style())
		}
	}

	tbl.ClearBorders()
	if b := tbl.Borders(); b.Top != nil {
		t.Error("ClearBorders should remove the override")
	}
}

func TestTable_PropertyOrderIsSchemaOrder(t *testing.T) {
	d := docx.New()
	tbl := d.Body().AddTable(1, 1, docx.TableOptions{})
	// Set in reverse schema order on purpose, style last.
	tbl.SetCellMargins(docx.CellMargins{Top: tw(0), Left: tw(108), Bottom: tw(0), Right: tw(108)})
	tbl.SetLayout(docx.LayoutFixed)
	tbl.SetBorders(docx.BorderOptions{Top: &docx.Border{Width: 4}})
	tbl.SetWidth(docx.WidthPercent(docx.Percent(100)))
	tbl.SetStyle("TableGrid")

	xml := string(partXML(t, d, "word/document.xml"))
	order := []string{"tblStyle", "tblW", "tblBorders", "tblLayout", "tblCellMar"}
	last := -1
	for _, name := range order {
		i := strings.Index(xml, "<"+name)
		if i < 0 {
			t.Fatalf("missing <%s>: %s", name, xml)
		}
		if i < last {
			t.Errorf("<%s> out of schema order: %s", name, xml)
		}
		last = i
	}
}

func TestTable_WidthKinds(t *testing.T) {
	d := docx.New()
	tbl := d.Body().AddTable(1, 1, docx.TableOptions{})
	if _, ok := tbl.Width(); ok {
		t.Error("new table should have no width")
	}
	tbl.SetWidth(docx.WidthTwips(docx.Inches(6)))
	if w, ok := tbl.Width(); !ok || w.Kind() != docx.WidthKindTwips || w.Twips() != docx.Inches(6) {
		t.Errorf("twips width = %+v ok=%v", w, ok)
	}
	tbl.SetWidth(docx.WidthPercent(docx.Percent(50)))
	if w, _ := tbl.Width(); w.Kind() != docx.WidthKindPercent || w.Percent() != docx.Percent(50) {
		t.Errorf("percent width = %+v", w)
	}
	tbl.SetWidth(docx.WidthAuto())
	if w, ok := tbl.Width(); !ok || w.Kind() != docx.WidthKindAuto {
		t.Errorf("auto width = %+v ok=%v", w, ok)
	}

	c := firstCell(tbl)
	c.SetWidth(docx.WidthPercent(docx.Percent(25)))
	d2 := reopen(t, d)
	for tb := range d2.Body().Tables() {
		w, ok := firstCell(tb).Width()
		if !ok || w.Kind() != docx.WidthKindPercent || w.Percent() != docx.Percent(25) {
			t.Errorf("cell width after round-trip = %+v ok=%v", w, ok)
		}
	}
}

func TestTable_LayoutAndMargins(t *testing.T) {
	d := docx.New()
	tbl := d.Body().AddTable(1, 1, docx.TableOptions{})
	tbl.SetLayout(docx.LayoutFixed)
	tbl.SetCellMargins(docx.CellMargins{Left: tw(200), Right: tw(200)})

	d2 := reopen(t, d)
	for tb := range d2.Body().Tables() {
		if tb.Layout() != docx.LayoutFixed {
			t.Errorf("Layout = %q", tb.Layout())
		}
		m := tb.CellMargins()
		if m.Left == nil || *m.Left != 200 || m.Right == nil || *m.Right != 200 {
			t.Errorf("margins = %+v", m)
		}
		if m.Top != nil || m.Bottom != nil {
			t.Error("unset margin sides should stay nil")
		}
	}

	tbl.SetLayout("")
	if tbl.Layout() != "" {
		t.Error("empty layout should clear the override")
	}
}

func TestTable_ClearingDoesNotCreateProperties(t *testing.T) {
	d := docx.New()
	tbl := d.Body().AddTable(1, 1, docx.TableOptions{})
	tbl.ClearBorders()
	tbl.SetLayout("")
	row := firstRow(t, tbl)
	row.SetCantSplit(nil)
	row.SetHeader(false)
	c := firstCell(tbl)
	c.SetShading("")

	xml := string(partXML(t, d, "word/document.xml"))
	for _, name := range []string{"<tblPr", "<trPr", "<tcPr"} {
		if strings.Contains(xml, name) {
			t.Errorf("clearing unset properties created %s: %s", name, xml)
		}
	}
}

func TestRow_HeaderAndCantSplit(t *testing.T) {
	d := docx.New()
	tbl := d.Body().AddTable(2, 1, docx.TableOptions{})
	row := firstRow(t, tbl)
	row.SetHeader(true)
	row.SetCantSplit(docx.On())
	if !row.IsHeader() {
		t.Error("IsHeader should be true")
	}

	d2 := reopen(t, d)
	for tb := range d2.Body().Tables() {
		r := firstRow(t, tb)
		if !r.IsHeader() {
			t.Error("header flag lost")
		}
		if cs := r.CantSplit(); cs == nil || !*cs {
			t.Errorf("CantSplit = %v", cs)
		}
	}
	row.SetHeader(false)
	if row.IsHeader() {
		t.Error("SetHeader(false) should clear the flag")
	}
}

func TestCell_BordersAndShading(t *testing.T) {
	d := docx.New()
	tbl := d.Body().AddTable(1, 1, docx.TableOptions{})
	c := firstCell(tbl)
	c.SetSpan(1, 2) // gridSpan first, so width and borders must still sort around it
	c.SetWidth(docx.WidthTwips(2000))
	c.SetShading("FFC7CE")
	c.SetBorders(docx.BorderOptions{
		Top:   &docx.Border{Style: docx.BorderDouble, Width: 6},
		TL2BR: &docx.Border{Style: docx.BorderSingle, Width: 4},
	})

	d2 := reopen(t, d)
	for tb := range d2.Body().Tables() {
		cc := firstCell(tb)
		if cc.Shading() != "FFC7CE" {
			t.Errorf("Shading = %q", cc.Shading())
		}
		b := cc.Borders()
		if b.Top == nil || b.Top.Style != docx.BorderDouble || b.TL2BR == nil {
			t.Errorf("cell borders = %+v", b)
		}
		if cc.ColSpan() != 2 {
			t.Errorf("ColSpan = %d", cc.ColSpan())
		}
	}

	xml := string(partXML(t, d2, "word/document.xml"))
	order := []string{"tcW", "gridSpan", "tcBorders", "shd"}
	last := -1
	for _, name := range order {
		i := strings.Index(xml, "<"+name)
		if i < 0 {
			t.Fatalf("missing <%s>: %s", name, xml)
		}
		if i < last {
			t.Errorf("<%s> out of schema order: %s", name, xml)
		}
		last = i
	}

	c.SetShading("")
	if c.Shading() != "" {
		t.Error("empty fill should clear shading")
	}
}

func TestCell_LegacyWidthOptionStillWorks(t *testing.T) {
	d := docx.New()
	tbl := d.Body().AddTable(1, 1, docx.TableOptions{})
	var r *docx.Row
	for rr := range tbl.Rows() {
		r = rr
	}
	cell := r.AddCellWithOptions(docx.CellOptions{ColSpan: 2, Width: 1500})
	w, ok := cell.Width()
	if !ok || w.Kind() != docx.WidthKindTwips || w.Twips() != 1500 {
		t.Errorf("width from CellOptions = %+v ok=%v", w, ok)
	}
	// tcW is written before gridSpan even though the option sets span first.
	xml := string(partXML(t, d, "word/document.xml"))
	if strings.Index(xml, "<tcW") > strings.Index(xml, "<gridSpan") {
		t.Errorf("tcW must precede gridSpan: %s", xml)
	}
}

func TestTable_FixtureBordersMarginsShading(t *testing.T) {
	read := func(name string) *docx.Document {
		data, err := os.ReadFile("../testdata/fixtures/poi/" + name)
		if err != nil {
			t.Fatal(err)
		}
		d, err := docx.OpenBytes(data)
		if err != nil {
			t.Fatal(err)
		}
		return d
	}

	d := read("table-indent.docx")
	var bordered, withMargins int
	for tb := range d.Body().Tables() {
		if tb.CellMargins().Left != nil {
			withMargins++
		}
		for r := range tb.Rows() {
			for c := range r.Cells() {
				if c.Borders().Top != nil || c.Borders().Left != nil {
					bordered++
				}
			}
		}
	}
	if bordered == 0 || withMargins == 0 {
		t.Errorf("table-indent: bordered cells %d, tables with margins %d; both should be > 0", bordered, withMargins)
	}

	d = read("table-alignment.docx")
	shaded := 0
	for tb := range d.Body().Tables() {
		for r := range tb.Rows() {
			for c := range r.Cells() {
				if c.Shading() != "" {
					shaded++
				}
			}
		}
	}
	if shaded == 0 {
		t.Error("table-alignment: no shaded cells read")
	}
}
