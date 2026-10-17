package wml_test

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/roundtrip"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

func newSpec() wml.InlinePictureSpec {
	return wml.InlinePictureSpec{
		RID: "rId5", FileName: "image1.png", CX: 952500, CY: 476250,
		DocPrID: 3, AltText: "diagram", Title: "",
	}
}

func TestNewInlinePicture_RoundTripAndAccessors(t *testing.T) {
	d := wml.NewInlinePicture(newSpec())
	roundtrip.Verify[*wml.CT_Drawing](t, d, ns.Qn("w:drawing"))

	if !d.IsInline() {
		t.Fatal("IsInline = false")
	}
	if cx, cy, ok := d.ExtentEMU(); !ok || cx != 952500 || cy != 476250 {
		t.Errorf("ExtentEMU = %d, %d, %v", cx, cy, ok)
	}
	if id, ok := d.DocPrID(); !ok || id != 3 {
		t.Errorf("DocPrID = %d, %v", id, ok)
	}
	if d.EmbedRID() != "rId5" || d.AltText() != "diagram" || d.Title() != "" {
		t.Errorf("rid/alt/title = %q/%q/%q", d.EmbedRID(), d.AltText(), d.Title())
	}
	d.SetTitle("T")
	d.SetAltText("")
	if d.Title() != "T" || d.AltText() != "" {
		t.Errorf("after set: title %q alt %q", d.Title(), d.AltText())
	}
	d.SetTitle("")
	if d.Title() != "" {
		t.Error("SetTitle(\"\") should remove the title")
	}
}

// TestNewInlinePicture_SchemaRequiredAttributes pins the attributes that the
// schema requires and that a hand-built tree can silently omit. A missing
// a:prstGeom/@prst was a real defect: Word may repair such a file.
func TestNewInlinePicture_SchemaRequiredAttributes(t *testing.T) {
	out, err := xml.Marshal(wml.NewInlinePicture(newSpec()))
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	for _, want := range []string{
		`prst="rect"`,                // a:prstGeom
		`cx="952500"`, `cy="476250"`, // wp:extent and a:ext
		`name="Picture 3"`, // wp:docPr
		`id="3"`,           // wp:docPr
		`noChangeAspect="1"`,
		`uri="http://schemas.openxmlformats.org/drawingml/2006/picture"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}
	// The geometry element must carry prst on the element itself.
	i := strings.Index(got, "<prstGeom")
	if i < 0 || !strings.Contains(got[i:strings.Index(got[i:], ">")+i], `prst="rect"`) {
		t.Errorf("a:prstGeom lacks prst: %s", got)
	}
}

func TestCT_Drawing_AnchorStaysRawAndDocPrIDFound(t *testing.T) {
	src := `<w:drawing xmlns:w="` + ns.W + `" xmlns:wp="` + ns.WP + `">` +
		`<wp:anchor distT="0" simplePos="0"><wp:simplePos x="0" y="0"/>` +
		`<wp:docPr id="12" name="Floating"/></wp:anchor></w:drawing>`
	roundtrip.VerifyBytes[*wml.CT_Drawing](t, []byte(src), ns.Qn("w:drawing"))

	var d wml.CT_Drawing
	if err := xml.Unmarshal([]byte(src), &d); err != nil {
		t.Fatal(err)
	}
	if d.IsInline() {
		t.Error("anchor must not be reported inline")
	}
	if id, ok := d.DocPrID(); !ok || id != 12 {
		t.Errorf("DocPrID from raw anchor = %d, %v", id, ok)
	}
	if _, _, ok := d.ExtentEMU(); ok {
		t.Error("ExtentEMU should be unavailable for an anchor")
	}
	d.SetExtentEMU(1, 2) // no-op, must not panic
	d.SetAltText("x")    // no-op
	if d.AltText() != "" {
		t.Error("alt text on a raw anchor should stay empty")
	}
}

func TestCT_R_DocPrIDs_IncludesRawAlternateContent(t *testing.T) {
	src := `<w:r xmlns:w="` + ns.W + `" xmlns:wp="` + ns.WP + `" xmlns:mc="` + ns.MC + `">` +
		`<mc:AlternateContent><mc:Choice Requires="wps"><w:drawing><wp:inline>` +
		`<wp:docPr id="40" name="Shape"/></wp:inline></w:drawing></mc:Choice></mc:AlternateContent></w:r>`
	var r wml.CT_R
	if err := xml.Unmarshal([]byte(src), &r); err != nil {
		t.Fatal(err)
	}
	ids := r.DocPrIDs()
	if len(ids) != 1 || ids[0] != 40 {
		t.Errorf("DocPrIDs = %v, want [40] (found inside mc:AlternateContent)", ids)
	}
}

func TestCT_R_WithDrawing_RoundTripThroughRun(t *testing.T) {
	r := new(wml.CT_R)
	r.Children = append(r.Children, wml.NewInlinePicture(newSpec()))
	roundtrip.Verify[*wml.CT_R](t, r, ns.Qn("w:r"))
	out, err := xml.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "drawing") || !strings.Contains(string(out), "inline") {
		t.Errorf("drawing missing from run: %s", out)
	}
	if r.Text() != "" {
		t.Errorf("a drawing contributes no text, got %q", r.Text())
	}
}
