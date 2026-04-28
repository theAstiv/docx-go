package wml_test

import (
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/roundtrip"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

var rprName = ns.Qn("w:rPr")

func TestCT_RPr_RoundTrip_Empty(t *testing.T) {
	roundtrip.Verify[*wml.CT_RPr](t, &wml.CT_RPr{}, rprName)
}

func TestCT_RPr_RoundTrip_BoldItalic(t *testing.T) {
	r := &wml.CT_RPr{}
	r.SetBold(boolPtr(true))
	r.SetItalic(boolPtr(true))
	roundtrip.Verify[*wml.CT_RPr](t, r, rprName)
}

func TestCT_RPr_RoundTrip_BoldOff(t *testing.T) {
	r := &wml.CT_RPr{}
	r.SetBold(boolPtr(false))
	roundtrip.Verify[*wml.CT_RPr](t, r, rprName)
}

func TestCT_RPr_RoundTrip_ColorAndSize(t *testing.T) {
	r := &wml.CT_RPr{}
	r.SetColorVal("FF0000")
	hp := int64(28)
	r.SetSzHalfPt(&hp)
	roundtrip.Verify[*wml.CT_RPr](t, r, rprName)
}

func TestCT_RPr_RoundTrip_Fonts(t *testing.T) {
	r := &wml.CT_RPr{}
	r.SetFontASCII("Calibri")
	r.SetFontHAnsi("Calibri")
	r.SetFontEastAsia("SimSun")
	roundtrip.Verify[*wml.CT_RPr](t, r, rprName)
}

func TestCT_RPr_RoundTrip_Underline(t *testing.T) {
	r := &wml.CT_RPr{}
	r.SetUnderlineStyle("single")
	r.SetUnderlineColor("0000FF")
	roundtrip.Verify[*wml.CT_RPr](t, r, rprName)
}

func TestCT_RPr_Bold_Accessors(t *testing.T) {
	r := &wml.CT_RPr{}
	if r.Bold() != nil {
		t.Error("Bold() on empty should be nil")
	}
	r.SetBold(boolPtr(true))
	if b := r.Bold(); b == nil || !*b {
		t.Error("Bold() after SetBold(true) should be true")
	}
	r.SetBold(boolPtr(false))
	if b := r.Bold(); b == nil || *b {
		t.Error("Bold() after SetBold(false) should be false")
	}
	r.SetBold(nil)
	if r.Bold() != nil {
		t.Error("Bold() after SetBold(nil) should be nil")
	}
}

func TestCT_RPr_StyleID_Accessors(t *testing.T) {
	r := &wml.CT_RPr{}
	if r.StyleID() != "" {
		t.Error("StyleID() on empty should be empty")
	}
	r.SetStyleID("Strong")
	if got := r.StyleID(); got != "Strong" {
		t.Errorf("StyleID() = %q, want %q", got, "Strong")
	}
}
