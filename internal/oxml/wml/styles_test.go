package wml_test

import (
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/roundtrip"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

var stylesName = ns.Qn("w:styles")
var styleName = ns.Qn("w:style")

func TestCT_Styles_RoundTrip_Empty(t *testing.T) {
	roundtrip.Verify[*wml.CT_Styles](t, &wml.CT_Styles{}, stylesName)
}

func TestCT_Styles_RoundTrip_WithStyles(t *testing.T) {
	s := &wml.CT_Styles{}
	norm := &wml.CT_Style{
		StyleID: "Normal",
		Type:    "paragraph",
		Default: true,
		Name:    "Normal",
	}
	heading := &wml.CT_Style{
		StyleID: "Heading1",
		Type:    "paragraph",
		Name:    "heading 1",
	}
	heading.PPr = &wml.CT_PPr{}
	heading.PPr.SetOutlineLevel(int64Ptr(0))
	s.Styles = append(s.Styles, norm, heading)
	roundtrip.Verify[*wml.CT_Styles](t, s, stylesName)
}

func TestCT_Style_RoundTrip_Empty(t *testing.T) {
	roundtrip.Verify[*wml.CT_Style](t, &wml.CT_Style{}, styleName)
}

func TestCT_Style_RoundTrip_ParagraphStyle(t *testing.T) {
	s := &wml.CT_Style{
		StyleID: "Heading1",
		Type:    "paragraph",
		Name:    "heading 1",
	}
	s.PPr = &wml.CT_PPr{}
	s.PPr.SetOutlineLevel(int64Ptr(0))
	s.PPr.SetKeepNext(boolPtr(true))
	s.RPr = &wml.CT_RPr{}
	s.RPr.SetBold(boolPtr(true))
	roundtrip.Verify[*wml.CT_Style](t, s, styleName)
}

func TestCT_Style_RoundTrip_CharacterStyle(t *testing.T) {
	s := &wml.CT_Style{
		StyleID: "Strong",
		Type:    "character",
		Name:    "Strong",
	}
	s.RPr = &wml.CT_RPr{}
	s.RPr.SetBold(boolPtr(true))
	roundtrip.Verify[*wml.CT_Style](t, s, styleName)
}

func TestCT_Styles_ByID(t *testing.T) {
	s := &wml.CT_Styles{}
	s.Styles = append(s.Styles,
		&wml.CT_Style{StyleID: "Normal", Type: "paragraph"},
		&wml.CT_Style{StyleID: "Heading1", Type: "paragraph"},
	)
	if got := s.ByID("Normal"); got == nil {
		t.Error("ByID(Normal) should find the style")
	}
	if got := s.ByID("Missing"); got != nil {
		t.Error("ByID(Missing) should return nil")
	}
}

func int64Ptr(v int64) *int64 { return &v }
