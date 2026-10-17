package wml_test

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/roundtrip"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

func TestCT_Hyperlink_TooltipHistory(t *testing.T) {
	h := new(wml.CT_Hyperlink)
	h.RID = "rId7"
	h.SetTooltip("tip")
	h.SetHistory(boolPtr(true))
	if h.Tooltip() != "tip" {
		t.Errorf("Tooltip = %q", h.Tooltip())
	}
	if v := h.History(); v == nil || !*v {
		t.Errorf("History = %v, want &true", v)
	}
	roundtrip.Verify[*wml.CT_Hyperlink](t, h, ns.Qn("w:hyperlink"))

	h.SetHistory(boolPtr(false))
	if v := h.History(); v == nil || *v {
		t.Errorf("History = %v, want &false", v)
	}
	h.SetHistory(nil)
	h.SetTooltip("")
	if h.History() != nil || h.Tooltip() != "" {
		t.Error("clearing should remove the attributes")
	}
}

func TestCT_Hyperlink_ParsesTooltipAndText(t *testing.T) {
	src := `<w:hyperlink xmlns:w="` + ns.W + `" xmlns:r="` + ns.R + `" r:id="rId1" w:tooltip="T" w:history="1">` +
		`<w:r><w:t>one </w:t></w:r><w:r><w:t>two</w:t></w:r></w:hyperlink>`
	roundtrip.VerifyBytes[*wml.CT_Hyperlink](t, []byte(src), ns.Qn("w:hyperlink"))

	var h wml.CT_Hyperlink
	if err := xml.Unmarshal([]byte(src), &h); err != nil {
		t.Fatal(err)
	}
	if h.RID != "rId1" || h.Tooltip() != "T" || h.Text() != "one two" {
		t.Errorf("rid/tooltip/text = %q/%q/%q", h.RID, h.Tooltip(), h.Text())
	}
	if v := h.History(); v == nil || !*v {
		t.Errorf("History = %v", v)
	}
}

func TestCT_BookmarkStart_DropsNamespaceDeclarations(t *testing.T) {
	src := `<w:bookmarkStart xmlns:w="` + ns.W + `" xmlns:x="urn:x" w:id="3" w:name="n"/>`
	var b wml.CT_BookmarkStart
	if err := xml.Unmarshal([]byte(src), &b); err != nil {
		t.Fatal(err)
	}
	out, err := xml.Marshal(&b)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "xmlns:x") || strings.Contains(string(out), "_xmlns") {
		t.Errorf("namespace declaration leaked into output: %s", out)
	}
	if b.ID != 3 || b.Name != "n" {
		t.Errorf("id/name = %d/%q", b.ID, b.Name)
	}
}
