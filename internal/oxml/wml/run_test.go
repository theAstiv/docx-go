package wml_test

import (
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/roundtrip"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

var rName = ns.Qn("w:r")

func TestCT_R_RoundTrip_Empty(t *testing.T) {
	roundtrip.Verify[*wml.CT_R](t, &wml.CT_R{}, rName)
}

func TestCT_R_RoundTrip_WithText(t *testing.T) {
	r := &wml.CT_R{}
	r.Children = append(r.Children, &wml.CT_Text{Content: "hello", Space: "preserve"})
	roundtrip.Verify[*wml.CT_R](t, r, rName)
}

func TestCT_R_RoundTrip_WithRPr(t *testing.T) {
	r := &wml.CT_R{}
	r.RPr = &wml.CT_RPr{}
	r.RPr.SetBold(boolPtr(true))
	r.RPr.SetItalic(boolPtr(false))
	r.RPr.SetColorVal("FF0000")
	r.Children = append(r.Children, &wml.CT_Text{Content: "bold red"})
	roundtrip.Verify[*wml.CT_R](t, r, rName)
}

func TestCT_R_RoundTrip_AllRunContent(t *testing.T) {
	r := &wml.CT_R{}
	r.Children = append(r.Children,
		&wml.CT_Text{Content: "a"},
		&wml.CT_Tab{},
		&wml.CT_Br{Type: "page"},
		&wml.CT_Cr{},
		&wml.CT_NoBreakHyphen{},
		&wml.CT_SoftHyphen{},
		&wml.CT_Sym{Font: "Symbol", Char: "F0B7"},
		&wml.CT_FldChar{FldCharType: "begin"},
		&wml.CT_InstrText{Content: " DATE ", Space: "preserve"},
		&wml.CT_FldChar{FldCharType: "separate"},
		&wml.CT_Text{Content: "2026"},
		&wml.CT_FldChar{FldCharType: "end"},
	)
	roundtrip.Verify[*wml.CT_R](t, r, rName)
}

func boolPtr(b bool) *bool { return &b }
