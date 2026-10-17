package wml_test

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

func TestTblWidth_PercentRoundsNotTruncates(t *testing.T) {
	// 33.33% is 1666.5 fiftieths; truncation gave 1666, rounding gives 1667.
	src := `<w:tcPr xmlns:w="` + ns.W + `"><w:tcW w:w="33.33%" w:type="pct"/></w:tcPr>`
	var p wml.CT_TcPr
	if err := xml.Unmarshal([]byte(src), &p); err != nil {
		t.Fatal(err)
	}
	if w := p.WidthSpec(); w == nil || w.W != 1667 {
		t.Errorf("WidthSpec = %+v, want 1667", w)
	}
}

func TestSettings_UpdateFieldsPrecedesMathPrAndSchemaLibrary(t *testing.T) {
	src := `<w:settings xmlns:w="` + ns.W + `" xmlns:m="` + ns.M + `" xmlns:sl="http://schemas.openxmlformats.org/schemaLibrary/2006/main">` +
		`<w:zoom w:percent="100"/><m:mathPr/><sl:schemaLibrary/></w:settings>`
	var s wml.CT_Settings
	if err := xml.Unmarshal([]byte(src), &s); err != nil {
		t.Fatal(err)
	}
	on := true
	s.SetUpdateFields(&on)
	out, err := xml.Marshal(&s)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	upd, math, lib := strings.Index(got, "<updateFields"), strings.Index(got, "<mathPr"), strings.Index(got, "<schemaLibrary")
	if upd < 0 || upd > math || upd > lib {
		t.Errorf("updateFields must precede mathPr and schemaLibrary: %s", got)
	}
}
