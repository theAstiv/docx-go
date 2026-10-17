package wml_test

import (
	"encoding/xml"
	"strings"
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

func TestCT_Style_Accessors_SchemaOrder(t *testing.T) {
	s := &wml.CT_Style{StyleID: "Heading1", Type: "paragraph", Name: "heading 1"}
	s.PPr = &wml.CT_PPr{}
	s.PPr.SetKeepNext(boolPtr(true))
	// Set out of schema order on purpose.
	s.SetQFormat(boolPtr(true))
	s.SetUIPriority(int64Ptr(9))
	s.SetLink("Heading1Char")
	s.SetNext("Normal")
	s.SetBasedOn("Normal")
	s.SetUnhideWhenUsed(boolPtr(true))
	s.SetSemiHidden(boolPtr(false))

	out, err := xml.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	order := []string{"basedOn", "next", "link", "uiPriority", "semiHidden", "unhideWhenUsed", "qFormat", "pPr"}
	last := -1
	for _, name := range order {
		i := strings.Index(got, "<"+name)
		if i < 0 {
			t.Fatalf("missing <%s> in %s", name, got)
		}
		if i < last {
			t.Errorf("<%s> out of schema order in %s", name, got)
		}
		last = i
	}

	if s.BasedOn() != "Normal" || s.Next() != "Normal" || s.Link() != "Heading1Char" {
		t.Errorf("string accessors: %q %q %q", s.BasedOn(), s.Next(), s.Link())
	}
	if p := s.UIPriority(); p == nil || *p != 9 {
		t.Errorf("UIPriority = %v, want 9", p)
	}
	if v := s.SemiHidden(); v == nil || *v {
		t.Errorf("SemiHidden = %v, want &false", v)
	}
	roundtrip.Verify[*wml.CT_Style](t, s, styleName)

	s.SetBasedOn("")
	s.SetUIPriority(nil)
	s.SetQFormat(nil)
	if s.BasedOn() != "" || s.UIPriority() != nil || s.QFormat() != nil {
		t.Error("clearing accessors should remove the elements")
	}
}

func TestCT_Style_UnknownPreChildKept(t *testing.T) {
	src := `<w:style xmlns:w="` + ns.W + `" w:type="paragraph" w:styleId="X">` +
		`<w:name w:val="X"/><w:rsid w:val="00AB12CD"/><w:pPr/></w:style>`
	roundtrip.VerifyBytes[*wml.CT_Style](t, []byte(src), styleName)
}

func TestCT_Styles_DocDefaults_RoundTrip(t *testing.T) {
	src := `<w:styles xmlns:w="` + ns.W + `"><w:docDefaults>` +
		`<w:rPrDefault><w:rPr><w:rFonts w:ascii="Calibri"/><w:sz w:val="22"/></w:rPr></w:rPrDefault>` +
		`<w:pPrDefault><w:pPr><w:spacing w:after="160" w:line="259" w:lineRule="auto"/></w:pPr></w:pPrDefault>` +
		`</w:docDefaults><w:latentStyles w:count="1"/></w:styles>`
	roundtrip.VerifyBytes[*wml.CT_Styles](t, []byte(src), stylesName)

	var s wml.CT_Styles
	if err := xml.Unmarshal([]byte(src), &s); err != nil {
		t.Fatal(err)
	}
	if s.DocDefaults == nil || s.DocDefaults.RPr == nil || s.DocDefaults.PPr == nil {
		t.Fatalf("DocDefaults not typed: %+v", s.DocDefaults)
	}
	if got := s.DocDefaults.RPr.FontASCII(); got != "Calibri" {
		t.Errorf("default font = %q, want Calibri", got)
	}
}
