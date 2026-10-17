package wml_test

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/roundtrip"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

var tblName = ns.Qn("w:tbl")
var trName = ns.Qn("w:tr")
var tcName = ns.Qn("w:tc")
var tblPrName = ns.Qn("w:tblPr")
var trPrName = ns.Qn("w:trPr")
var tcPrName = ns.Qn("w:tcPr")
var tblGridName = ns.Qn("w:tblGrid")
var gridColName = ns.Qn("w:gridCol")

// ── CT_TblPr ─────────────────────────────────────────────────────────────

func TestCT_TblPr_RoundTrip_Empty(t *testing.T) {
	roundtrip.Verify[*wml.CT_TblPr](t, &wml.CT_TblPr{}, tblPrName)
}

func TestCT_TblPr_RoundTrip_WithStyle(t *testing.T) {
	p := &wml.CT_TblPr{}
	p.SetStyleID("TableGrid")
	roundtrip.Verify[*wml.CT_TblPr](t, p, tblPrName)
}

func TestCT_TblPr_StyleID_Accessors(t *testing.T) {
	p := &wml.CT_TblPr{}
	if got := p.StyleID(); got != "" {
		t.Errorf("StyleID() on empty = %q, want empty", got)
	}
	p.SetStyleID("TableGrid")
	if got := p.StyleID(); got != "TableGrid" {
		t.Errorf("StyleID() = %q, want %q", got, "TableGrid")
	}
	p.SetStyleID("")
	if got := p.StyleID(); got != "" {
		t.Errorf("StyleID() after clear = %q, want empty", got)
	}
}

// ── CT_TrPr ──────────────────────────────────────────────────────────────

func TestCT_TrPr_RoundTrip_Empty(t *testing.T) {
	roundtrip.Verify[*wml.CT_TrPr](t, &wml.CT_TrPr{}, trPrName)
}

func TestCT_TrPr_RoundTrip_Header(t *testing.T) {
	p := &wml.CT_TrPr{}
	p.SetHeader(boolPtr(true))
	roundtrip.Verify[*wml.CT_TrPr](t, p, trPrName)
}

func TestCT_TrPr_IsHeader_Accessors(t *testing.T) {
	p := &wml.CT_TrPr{}
	if p.IsHeader() != nil {
		t.Error("IsHeader() on empty should be nil")
	}
	p.SetHeader(boolPtr(true))
	if got := p.IsHeader(); got == nil || !*got {
		t.Errorf("IsHeader() = %v, want true", got)
	}
	p.SetHeader(nil)
	if p.IsHeader() != nil {
		t.Error("IsHeader() after nil set should be nil")
	}
}

// ── CT_TcPr ──────────────────────────────────────────────────────────────

func TestCT_TcPr_RoundTrip_Empty(t *testing.T) {
	roundtrip.Verify[*wml.CT_TcPr](t, &wml.CT_TcPr{}, tcPrName)
}

func TestCT_TcPr_RoundTrip_GridSpan(t *testing.T) {
	p := &wml.CT_TcPr{}
	n := int64(3)
	p.SetGridSpan(&n)
	roundtrip.Verify[*wml.CT_TcPr](t, p, tcPrName)
}

func TestCT_TcPr_RoundTrip_VMerge(t *testing.T) {
	p := &wml.CT_TcPr{}
	p.SetVMerge(wml.VMergeRestart)
	roundtrip.Verify[*wml.CT_TcPr](t, p, tcPrName)
}

func TestCT_TcPr_RoundTrip_VMergeContinue(t *testing.T) {
	p := &wml.CT_TcPr{}
	p.SetVMerge("")
	roundtrip.Verify[*wml.CT_TcPr](t, p, tcPrName)
}

func TestCT_TcPr_GridSpan_Accessors(t *testing.T) {
	p := &wml.CT_TcPr{}
	if p.GridSpan() != nil {
		t.Error("GridSpan() on empty should be nil")
	}
	n := int64(2)
	p.SetGridSpan(&n)
	if got := p.GridSpan(); got == nil || *got != 2 {
		t.Errorf("GridSpan() = %v, want 2", got)
	}
	p.SetGridSpan(nil)
	if p.GridSpan() != nil {
		t.Error("GridSpan() after nil set should be nil")
	}
}

func TestCT_TcPr_VMerge_Accessors(t *testing.T) {
	p := &wml.CT_TcPr{}
	if _, ok := p.VMerge(); ok {
		t.Error("VMerge() on empty should not be present")
	}
	p.SetVMerge(wml.VMergeRestart)
	if val, ok := p.VMerge(); !ok || val != wml.VMergeRestart {
		t.Errorf("VMerge() = (%q, %v), want (restart, true)", val, ok)
	}
	p.SetVMerge("") // continuation
	if val, ok := p.VMerge(); !ok || val != "" {
		t.Errorf("VMerge() continuation = (%q, %v), want ('', true)", val, ok)
	}
	p.ClearVMerge()
	if _, ok := p.VMerge(); ok {
		t.Error("VMerge() after ClearVMerge should not be present")
	}
}

// ── CT_GridCol ────────────────────────────────────────────────────────────

func TestCT_GridCol_RoundTrip_Nil(t *testing.T) {
	roundtrip.Verify[*wml.CT_GridCol](t, &wml.CT_GridCol{}, gridColName)
}

func TestCT_GridCol_RoundTrip_Width(t *testing.T) {
	w := int64(1440)
	roundtrip.Verify[*wml.CT_GridCol](t, &wml.CT_GridCol{W: &w}, gridColName)
}

// ── CT_TblGrid ────────────────────────────────────────────────────────────

func TestCT_TblGrid_RoundTrip_Empty(t *testing.T) {
	roundtrip.Verify[*wml.CT_TblGrid](t, &wml.CT_TblGrid{}, tblGridName)
}

func TestCT_TblGrid_RoundTrip_TwoCols(t *testing.T) {
	w1, w2 := int64(1440), int64(2880)
	g := &wml.CT_TblGrid{
		Cols: []*wml.CT_GridCol{
			{W: &w1},
			{W: &w2},
		},
	}
	roundtrip.Verify[*wml.CT_TblGrid](t, g, tblGridName)
}

// ── CT_Tr ─────────────────────────────────────────────────────────────────

func TestCT_Tr_RoundTrip_Empty(t *testing.T) {
	roundtrip.Verify[*wml.CT_Tr](t, &wml.CT_Tr{}, trName)
}

func TestCT_Tr_RoundTrip_OneCell(t *testing.T) {
	cell := &wml.CT_Tc{
		Children: []wml.BlockElement{&wml.CT_P{}},
	}
	row := &wml.CT_Tr{
		Children: []wml.TableCellElement{cell},
	}
	roundtrip.Verify[*wml.CT_Tr](t, row, trName)
}

// ── CT_Tc ─────────────────────────────────────────────────────────────────

func TestCT_Tc_RoundTrip_Empty(t *testing.T) {
	roundtrip.Verify[*wml.CT_Tc](t, &wml.CT_Tc{}, tcName)
}

func TestCT_Tc_RoundTrip_WithParagraph(t *testing.T) {
	cell := &wml.CT_Tc{
		Children: []wml.BlockElement{&wml.CT_P{}},
	}
	roundtrip.Verify[*wml.CT_Tc](t, cell, tcName)
}

// ── CT_Tbl ────────────────────────────────────────────────────────────────

func TestCT_Tbl_RoundTrip_Empty(t *testing.T) {
	roundtrip.Verify[*wml.CT_Tbl](t, &wml.CT_Tbl{}, tblName)
}

func TestCT_Tbl_RoundTrip_OneByOne(t *testing.T) {
	cell := &wml.CT_Tc{
		Children: []wml.BlockElement{&wml.CT_P{}},
	}
	row := &wml.CT_Tr{
		Children: []wml.TableCellElement{cell},
	}
	tbl := &wml.CT_Tbl{
		Children: []wml.TableRowElement{row},
	}
	roundtrip.Verify[*wml.CT_Tbl](t, tbl, tblName)
}

func TestCT_Tbl_RoundTrip_WithGridAndStyle(t *testing.T) {
	w1, w2 := int64(1440), int64(1440)
	tblPr := &wml.CT_TblPr{}
	tblPr.SetStyleID("TableGrid")
	cell1 := &wml.CT_Tc{Children: []wml.BlockElement{&wml.CT_P{}}}
	cell2 := &wml.CT_Tc{Children: []wml.BlockElement{&wml.CT_P{}}}
	row := &wml.CT_Tr{Children: []wml.TableCellElement{cell1, cell2}}
	tbl := &wml.CT_Tbl{
		TblPr: tblPr,
		TblGrid: &wml.CT_TblGrid{
			Cols: []*wml.CT_GridCol{{W: &w1}, {W: &w2}},
		},
		Children: []wml.TableRowElement{row},
	}
	roundtrip.Verify[*wml.CT_Tbl](t, tbl, tblName)
}

func TestCT_Tbl_RoundTrip_MergedCells(t *testing.T) {
	tcPr1 := &wml.CT_TcPr{}
	tcPr1.SetVMerge(wml.VMergeRestart)
	tcPr2 := &wml.CT_TcPr{}
	tcPr2.SetVMerge("")
	span := int64(2)
	tcPr3 := &wml.CT_TcPr{}
	tcPr3.SetGridSpan(&span)
	row1 := &wml.CT_Tr{Children: []wml.TableCellElement{
		&wml.CT_Tc{TcPr: tcPr1, Children: []wml.BlockElement{&wml.CT_P{}}},
		&wml.CT_Tc{TcPr: tcPr3, Children: []wml.BlockElement{&wml.CT_P{}}},
	}}
	row2 := &wml.CT_Tr{Children: []wml.TableCellElement{
		&wml.CT_Tc{TcPr: tcPr2, Children: []wml.BlockElement{&wml.CT_P{}}},
		&wml.CT_Tc{Children: []wml.BlockElement{&wml.CT_P{}}},
	}}
	tbl := &wml.CT_Tbl{Children: []wml.TableRowElement{row1, row2}}
	roundtrip.Verify[*wml.CT_Tbl](t, tbl, tblName)
}

func TestCT_Tbl_RoundTrip_NestedTable(t *testing.T) {
	innerCell := &wml.CT_Tc{Children: []wml.BlockElement{&wml.CT_P{}}}
	innerRow := &wml.CT_Tr{Children: []wml.TableCellElement{innerCell}}
	innerTbl := &wml.CT_Tbl{Children: []wml.TableRowElement{innerRow}}
	outerCell := &wml.CT_Tc{Children: []wml.BlockElement{innerTbl, &wml.CT_P{}}}
	outerRow := &wml.CT_Tr{Children: []wml.TableCellElement{outerCell}}
	tbl := &wml.CT_Tbl{Children: []wml.TableRowElement{outerRow}}
	roundtrip.Verify[*wml.CT_Tbl](t, tbl, tblName)
}

func TestCT_Tr_TblPrExStaysBeforeTrPr(t *testing.T) {
	src := `<w:tr xmlns:w="` + ns.W + `"><w:tblPrEx><w:tblCellMar><w:top w:w="0" w:type="dxa"/></w:tblCellMar></w:tblPrEx>` +
		`<w:trPr><w:jc w:val="center"/></w:trPr><w:tc><w:p/></w:tc></w:tr>`
	roundtrip.VerifyBytes[*wml.CT_Tr](t, []byte(src), ns.Qn("w:tr"))

	var tr wml.CT_Tr
	if err := xml.Unmarshal([]byte(src), &tr); err != nil {
		t.Fatal(err)
	}
	out, err := xml.Marshal(&tr)
	if err != nil {
		t.Fatal(err)
	}
	if ex, pr := strings.Index(string(out), "tblPrEx"), strings.Index(string(out), "trPr"); ex < 0 || pr < 0 || ex > pr {
		t.Errorf("tblPrEx must precede trPr (xs:sequence): %s", out)
	}
}
