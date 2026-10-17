package wml_test

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/roundtrip"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

func indexOrder(t *testing.T, xmlText string, names ...string) {
	t.Helper()
	last := -1
	for _, n := range names {
		i := strings.Index(xmlText, "<"+n)
		if i < 0 {
			t.Fatalf("missing <%s> in %s", n, xmlText)
		}
		if i < last {
			t.Errorf("<%s> out of schema order in %s", n, xmlText)
		}
		last = i
	}
}

func TestCT_TblPr_TypedAccessors_SchemaOrder(t *testing.T) {
	p := new(wml.CT_TblPr)
	four := int64(4)
	p.SetCellMargins(&wml.CellMargins{Left: &four})
	p.SetLayout("fixed")
	p.SetShading(&wml.Shading{Fill: "EEEEEE"})
	p.SetBorders(&wml.Borders{Top: &wml.Border{Val: "single", Sz: &four}})
	p.SetWidth(&wml.TblWidth{Type: wml.WidthPct, W: 5000})
	p.SetStyleID("TableGrid")

	out, err := xml.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	indexOrder(t, string(out), "tblStyle", "tblW", "tblBorders", "shd", "tblLayout", "tblCellMar")
	roundtrip.Verify[*wml.CT_TblPr](t, p, ns.Qn("w:tblPr"))

	if w := p.Width(); w == nil || w.Type != wml.WidthPct || w.W != 5000 {
		t.Errorf("Width = %+v", w)
	}
	if b := p.Borders(); b == nil || b.Top == nil || *b.Top.Sz != 4 || b.Left != nil {
		t.Errorf("Borders = %+v", b)
	}
	if p.Layout() != "fixed" || p.StyleID() != "TableGrid" {
		t.Errorf("layout/style = %q/%q", p.Layout(), p.StyleID())
	}
	if s := p.Shading(); s == nil || s.Fill != "EEEEEE" || s.Val != "clear" {
		t.Errorf("Shading = %+v", s)
	}

	p.SetWidth(nil)
	p.SetBorders(nil)
	p.SetLayout("")
	p.SetCellMargins(nil)
	p.SetShading(nil)
	if p.Width() != nil || p.Borders() != nil || p.Layout() != "" || p.CellMargins() != nil || p.Shading() != nil {
		t.Error("nil/empty setters should remove the children")
	}
}

func TestCT_TblPr_SetKeepsUnknownNeighbours(t *testing.T) {
	src := `<w:tblPr xmlns:w="` + ns.W + `"><w:tblStyle w:val="X"/><w:jc w:val="center"/>` +
		`<w:tblInd w:w="108" w:type="dxa"/><w:tblLook w:val="04A0"/></w:tblPr>`
	var p wml.CT_TblPr
	if err := xml.Unmarshal([]byte(src), &p); err != nil {
		t.Fatal(err)
	}
	four := int64(4)
	p.SetBorders(&wml.Borders{Top: &wml.Border{Val: "single", Sz: &four}})
	p.SetLayout("autofit")

	out, _ := xml.Marshal(&p)
	// jc, tblInd and tblLook survive, and the new children land between them.
	indexOrder(t, string(out), "tblStyle", "jc", "tblInd", "tblBorders", "tblLayout", "tblLook")
}

func TestCT_TcPr_TypedAccessors_SchemaOrder(t *testing.T) {
	p := new(wml.CT_TcPr)
	two := int64(2)
	p.SetShading(&wml.Shading{Fill: "FF0000"})
	p.SetBorders(&wml.Borders{Top: &wml.Border{Val: "double"}, TL2BR: &wml.Border{Val: "single"}})
	p.SetVMerge(wml.VMergeRestart)
	p.SetGridSpan(&two)
	p.SetWidth(1440)

	out, err := xml.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	indexOrder(t, string(out), "tcW", "gridSpan", "vMerge", "tcBorders", "shd")
	roundtrip.Verify[*wml.CT_TcPr](t, p, ns.Qn("w:tcPr"))

	if p.Width() != 1440 {
		t.Errorf("Width = %d", p.Width())
	}
	if w := p.WidthSpec(); w == nil || w.Type != wml.WidthDxa {
		t.Errorf("WidthSpec = %+v", w)
	}
	if b := p.Borders(); b == nil || b.TL2BR == nil || b.Top.Val != "double" {
		t.Errorf("Borders = %+v", b)
	}
	if gs := p.GridSpan(); gs == nil || *gs != 2 {
		t.Errorf("GridSpan = %v", gs)
	}

	// Replacing width does not duplicate the element.
	p.SetWidthSpec(&wml.TblWidth{Type: wml.WidthAuto})
	out, _ = xml.Marshal(p)
	if strings.Count(string(out), "<tcW") != 1 {
		t.Errorf("tcW duplicated: %s", out)
	}
}

func TestTblWidth_ParsesPercentSuffix(t *testing.T) {
	src := `<w:tcPr xmlns:w="` + ns.W + `"><w:tcW w:w="50%" w:type="pct"/></w:tcPr>`
	var p wml.CT_TcPr
	if err := xml.Unmarshal([]byte(src), &p); err != nil {
		t.Fatal(err)
	}
	if w := p.WidthSpec(); w == nil || w.Type != wml.WidthPct || w.W != 2500 {
		t.Errorf("WidthSpec = %+v, want pct 2500 (50%% in fiftieths)", w)
	}
	roundtrip.VerifyBytes[*wml.CT_TcPr](t, []byte(src), ns.Qn("w:tcPr"))
}

func TestCT_TrPr_CantSplit(t *testing.T) {
	p := new(wml.CT_TrPr)
	yes := true
	p.SetCantSplit(&yes)
	if v := p.CantSplit(); v == nil || !*v {
		t.Errorf("CantSplit = %v", v)
	}
	roundtrip.Verify[*wml.CT_TrPr](t, p, ns.Qn("w:trPr"))
	p.SetCantSplit(nil)
	if p.CantSplit() != nil {
		t.Error("nil should remove cantSplit")
	}
}
