package wml_test

import (
	"encoding/xml"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/roundtrip"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

func TestCT_PPr_BordersShadingTabs_SchemaOrder(t *testing.T) {
	p := new(wml.CT_PPr)
	four, one := int64(4), int64(1)
	// Set in reverse schema order; marshal must sort them.
	p.SetAlignment("center")
	p.SetIndStart(&four)
	p.SetSpacingAfter(&four)
	p.SetTabs([]wml.Tab{{Val: wml.TabRight, Pos: 9000, Leader: "dot"}, {Val: "", Pos: 720, Leader: ""}})
	p.SetShading(&wml.Shading{Fill: "EEEEEE"})
	p.SetBorders(&wml.ParagraphBorders{
		Bar:     &wml.Border{Val: "single", Sz: &four},
		Between: &wml.Border{Val: "dotted", Sz: &four, Space: &one},
		Top:     &wml.Border{Val: "single", Sz: &four, Color: "FF0000"},
	})
	p.SetStyleID("Quote")

	out, err := xml.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	indexOrder(t, s, "pStyle", "pBdr", "shd", "tabs", "spacing", "ind", "jc")
	indexOrder(t, s, "top", "between", "bar") // CT_PBdr order
	roundtrip.Verify[*wml.CT_PPr](t, p, ns.Qn("w:pPr"))

	b := p.Borders()
	if b == nil || b.Top == nil || b.Top.Color != "FF0000" || b.Between == nil || *b.Between.Space != 1 || b.Bar == nil || b.Left != nil {
		t.Errorf("Borders = %+v", b)
	}
	if sh := p.Shading(); sh == nil || sh.Fill != "EEEEEE" || sh.Val != "clear" || sh.Color != "auto" {
		t.Errorf("Shading = %+v", sh)
	}
	tabs := p.Tabs()
	if len(tabs) != 2 || tabs[0] != (wml.Tab{Val: wml.TabRight, Pos: 9000, Leader: "dot"}) || tabs[1] != (wml.Tab{Val: wml.TabLeft, Pos: 720, Leader: ""}) {
		t.Errorf("Tabs = %+v", tabs)
	}

	p.SetBorders(nil)
	p.SetShading(nil)
	p.SetTabs(nil)
	if p.Borders() != nil || p.Shading() != nil || p.Tabs() != nil {
		t.Error("nil setters should remove the children")
	}
}

func TestCT_PPr_ReadsISONames(t *testing.T) {
	src := `<w:pPr xmlns:w="` + ns.W + `"><w:pBdr><w:start w:val="single"/><w:end w:val="double"/></w:pBdr>` +
		`<w:tabs><w:tab w:val="start" w:pos="100"/><w:tab w:val="end" w:pos="-200"/><w:tab w:val="left" w:pos="bad"/></w:tabs></w:pPr>`
	var p wml.CT_PPr
	if err := xml.Unmarshal([]byte(src), &p); err != nil {
		t.Fatal(err)
	}
	if b := p.Borders(); b == nil || b.Left == nil || b.Left.Val != "single" || b.Right == nil || b.Right.Val != "double" {
		t.Errorf("Borders = %+v", b)
	}
	tabs := p.Tabs()
	if len(tabs) != 2 || tabs[0].Val != wml.TabLeft || tabs[1].Val != wml.TabRight || tabs[1].Pos != -200 {
		t.Errorf("Tabs = %+v", tabs)
	}
}
