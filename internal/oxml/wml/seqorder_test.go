package wml_test

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

func TestOrderBySeq_PropertyContainersAreWrittenInSchemaOrder(t *testing.T) {
	// Children deliberately out of order.
	ppr := `<w:pPr xmlns:w="` + ns.W + `"><w:jc w:val="center"/><w:contextualSpacing/><w:ind w:left="10"/>` +
		`<w:keepNext/><w:pStyle w:val="X"/></w:pPr>`
	var p wml.CT_PPr
	if err := xml.Unmarshal([]byte(ppr), &p); err != nil {
		t.Fatal(err)
	}
	out, _ := xml.Marshal(&p)
	indexOrder(t, string(out), "pStyle", "keepNext", "ind", "contextualSpacing", "jc")

	rpr := `<w:rPr xmlns:w="` + ns.W + `"><w:u w:val="single"/><w:sz w:val="24"/><w:color w:val="FF0000"/>` +
		`<w:b/><w:rStyle w:val="S"/></w:rPr>`
	var r wml.CT_RPr
	if err := xml.Unmarshal([]byte(rpr), &r); err != nil {
		t.Fatal(err)
	}
	out, _ = xml.Marshal(&r)
	indexOrder(t, string(out), "rStyle", "b", "color", "sz", "u")

	// An extension element stays attached to the element before it: here the
	// custom element follows w:sz, so it sorts with sz and ahead of w:u.
	ext := `<w:rPr xmlns:w="` + ns.W + `" xmlns:x="urn:x"><w:u w:val="single"/><w:sz w:val="24"/><x:custom/></w:rPr>`
	var e wml.CT_RPr
	if err := xml.Unmarshal([]byte(ext), &e); err != nil {
		t.Fatal(err)
	}
	out, _ = xml.Marshal(&e)
	indexOrder(t, string(out), "sz", "custom", "u")

	// An already-ordered container is written back unchanged.
	ordered := `<w:rPr xmlns:w="` + ns.W + `"><w:b/><w:sz w:val="24"/><w:u w:val="single"/></w:rPr>`
	var o wml.CT_RPr
	if err := xml.Unmarshal([]byte(ordered), &o); err != nil {
		t.Fatal(err)
	}
	out, _ = xml.Marshal(&o)
	indexOrder(t, string(out), "b", "sz", "u")
}

func TestSectPr_AddHeaderRefLeadsTheElement(t *testing.T) {
	s := wml.DefaultSectPr() // pgSz, pgMar
	s.SetTitlePg(true)
	s.AddFooterRef(wml.HdrFtrDefault, "rId2")
	s.AddHeaderRef(wml.HdrFtrDefault, "rId1")
	out, err := xml.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	if h, f, pg := strings.Index(got, "headerReference"), strings.Index(got, "footerReference"), strings.Index(got, "<pgSz"); h < 0 || f < 0 || h > pg || f > pg {
		t.Errorf("header/footer references must precede pgSz: %s", got)
	}
	indexOrder(t, got, "pgSz", "pgMar", "titlePg")
}
