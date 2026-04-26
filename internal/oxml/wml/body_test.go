package wml_test

import (
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/roundtrip"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

var bodyName = ns.Qn("w:body")

func TestCT_Body_RoundTrip_Empty(t *testing.T) {
	roundtrip.Verify[*wml.CT_Body](t, &wml.CT_Body{}, bodyName)
}

func TestCT_Body_RoundTrip_WithParagraphs(t *testing.T) {
	b := &wml.CT_Body{}
	p1 := &wml.CT_P{}
	r := &wml.CT_R{}
	r.Children = append(r.Children, &wml.CT_Text{Content: "paragraph one"})
	p1.Children = append(p1.Children, r)
	p2 := &wml.CT_P{}
	b.Children = append(b.Children, p1, p2)
	b.SectPr = &wml.CT_SectPr{}
	roundtrip.Verify[*wml.CT_Body](t, b, bodyName)
}

var docName = ns.Qn("w:document")

func TestCT_Document_RoundTrip_Empty(t *testing.T) {
	d := &wml.CT_Document{}
	d.Body = &wml.CT_Body{}
	roundtrip.Verify[*wml.CT_Document](t, d, docName)
}

func TestCT_Document_RoundTrip_WithContent(t *testing.T) {
	d := &wml.CT_Document{}
	d.Body = &wml.CT_Body{}
	p := &wml.CT_P{}
	r := &wml.CT_R{}
	r.Children = append(r.Children, &wml.CT_Text{Content: "doc content"})
	p.Children = append(p.Children, r)
	d.Body.Children = append(d.Body.Children, p)
	d.Body.SectPr = &wml.CT_SectPr{}
	roundtrip.Verify[*wml.CT_Document](t, d, docName)
}

var sectPrName = ns.Qn("w:sectPr")

func TestCT_SectPr_RoundTrip_Empty(t *testing.T) {
	roundtrip.Verify[*wml.CT_SectPr](t, &wml.CT_SectPr{}, sectPrName)
}
