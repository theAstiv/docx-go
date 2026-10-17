package dml_test

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/dml"
	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/roundtrip"
)

const nsDecls = ` xmlns:wp="` + ns.WP + `" xmlns:a="` + ns.A + `" xmlns:pic="` + ns.PIC + `" xmlns:r="` + ns.R + `"`

// wordInline is an inline picture in the shape Word writes it, including an
// extension list with an SVG fallback blip and a crop, which must survive.
const wordInline = `<wp:inline distT="0" distB="0" distL="0" distR="0"` + nsDecls + `>` +
	`<wp:extent cx="1905000" cy="952500"/>` +
	`<wp:effectExtent l="0" t="0" r="0" b="0"/>` +
	`<wp:docPr id="7" name="Picture 7" descr="A bar chart" title="Q3"/>` +
	`<wp:cNvGraphicFramePr><a:graphicFrameLocks noChangeAspect="1"/></wp:cNvGraphicFramePr>` +
	`<a:graphic><a:graphicData uri="` + dml.PictureURI + `">` +
	`<pic:pic><pic:nvPicPr><pic:cNvPr id="0" name="chart.png"/><pic:cNvPicPr/></pic:nvPicPr>` +
	`<pic:blipFill dpi="0" rotWithShape="1"><a:blip r:embed="rId9" cstate="print">` +
	`<a:extLst><a:ext uri="{96DAC541-7B7A-43D3-8B79-37D633B846F1}"><asvg:svgBlip xmlns:asvg="http://schemas.microsoft.com/office/drawing/2016/SVG/main" r:embed="rId10"/></a:ext></a:extLst>` +
	`</a:blip><a:srcRect l="1000" r="1000"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill>` +
	`<pic:spPr bwMode="auto"><a:xfrm><a:off x="0" y="0"/><a:ext cx="1905000" cy="952500"/></a:xfrm>` +
	`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr>` +
	`</pic:pic></a:graphicData></a:graphic></wp:inline>`

func TestCT_Inline_RoundTrip_WordSample(t *testing.T) {
	roundtrip.VerifyBytes[*dml.CT_Inline](t, []byte(wordInline), ns.Qn("wp:inline"))

	var in dml.CT_Inline
	if err := xml.Unmarshal([]byte(wordInline), &in); err != nil {
		t.Fatal(err)
	}
	if in.Extent == nil || in.Extent.Cx != 1905000 || in.Extent.Cy != 952500 {
		t.Errorf("Extent = %+v", in.Extent)
	}
	if in.DocPr == nil || in.DocPr.ID != 7 || in.DocPr.Descr == nil || *in.DocPr.Descr != "A bar chart" {
		t.Errorf("DocPr = %+v", in.DocPr)
	}
	pic := in.Picture()
	if pic == nil || pic.EmbedRID() != "rId9" {
		t.Fatalf("Picture/EmbedRID = %v", pic)
	}
	if len(pic.BlipFill.Blip.Children) != 1 {
		t.Errorf("extLst on the blip not captured: %d children", len(pic.BlipFill.Blip.Children))
	}
	if len(pic.BlipFill.Rest) != 2 {
		t.Errorf("srcRect + stretch not captured: %d", len(pic.BlipFill.Rest))
	}
}

func TestCT_Inline_SetExtentKeepsTransformInStep(t *testing.T) {
	var in dml.CT_Inline
	if err := xml.Unmarshal([]byte(wordInline), &in); err != nil {
		t.Fatal(err)
	}
	in.SetExtentEMU(111, 222)
	cx, cy, ok := in.Picture().SpPr.ExtentEMU()
	if !ok || cx != 111 || cy != 222 || in.Extent.Cx != 111 || in.Extent.Cy != 222 {
		t.Errorf("transform %d x %d (ok=%v), extent %+v", cx, cy, ok, in.Extent)
	}
	out, err := xml.Marshal(&in)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(out), `cx="111"`) != 2 {
		t.Errorf("both extents should carry cx=111: %s", out)
	}
}

func TestCT_Inline_UnknownChildrenAndAttrsKept(t *testing.T) {
	src := `<wp:inline distT="1" wp14:anchorId="0A1B2C3D" xmlns:wp14="` + ns.WP14 + `"` + nsDecls + `>` +
		`<wp:extent cx="1" cy="2"/><wp:docPr id="1" name="x"/>` +
		`<wp:futureThing foo="bar"><a:b/></wp:futureThing></wp:inline>`
	roundtrip.VerifyBytes[*dml.CT_Inline](t, []byte(src), ns.Qn("wp:inline"))
}

func TestCT_Inline_NonPictureGraphicStaysRaw(t *testing.T) {
	src := `<wp:inline` + nsDecls + `><wp:extent cx="1" cy="1"/><wp:docPr id="2" name="Chart 2"/>` +
		`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart">` +
		`<c:chart xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" r:id="rId5"/>` +
		`</a:graphicData></a:graphic></wp:inline>`
	roundtrip.VerifyBytes[*dml.CT_Inline](t, []byte(src), ns.Qn("wp:inline"))
	var in dml.CT_Inline
	if err := xml.Unmarshal([]byte(src), &in); err != nil {
		t.Fatal(err)
	}
	if in.Picture() != nil {
		t.Error("a chart must not be reported as a picture")
	}
	if len(in.Graphic.GraphicData.Rest) != 1 {
		t.Errorf("chart payload not preserved raw: %d", len(in.Graphic.GraphicData.Rest))
	}
}

func TestCT_NonVisualDrawingProps_DescrAbsentVersusEmpty(t *testing.T) {
	absent := `<wp:docPr xmlns:wp="` + ns.WP + `" id="1" name="a"/>`
	empty := `<wp:docPr xmlns:wp="` + ns.WP + `" id="1" name="a" descr=""/>`
	roundtrip.VerifyBytes[*dml.CT_NonVisualDrawingProps](t, []byte(absent), ns.Qn("wp:docPr"))
	roundtrip.VerifyBytes[*dml.CT_NonVisualDrawingProps](t, []byte(empty), ns.Qn("wp:docPr"))

	var a, e dml.CT_NonVisualDrawingProps
	if err := xml.Unmarshal([]byte(absent), &a); err != nil {
		t.Fatal(err)
	}
	if err := xml.Unmarshal([]byte(empty), &e); err != nil {
		t.Fatal(err)
	}
	if a.Descr != nil {
		t.Error("absent descr should be nil")
	}
	if e.Descr == nil || *e.Descr != "" {
		t.Error("descr=\"\" should be a non-nil empty string")
	}
	out, _ := xml.Marshal(&e)
	if !strings.Contains(string(out), `descr=""`) {
		t.Errorf("empty descr dropped on marshal: %s", out)
	}
}

func TestCT_NonVisualDrawingProps_ExtrasKept(t *testing.T) {
	src := `<wp:docPr xmlns:wp="` + ns.WP + `" xmlns:a="` + ns.A + `" id="3" name="n" hidden="1">` +
		`<a:hlinkClick r:id="rId1" xmlns:r="` + ns.R + `"/></wp:docPr>`
	roundtrip.VerifyBytes[*dml.CT_NonVisualDrawingProps](t, []byte(src), ns.Qn("wp:docPr"))
}

func TestCT_PositiveSize2D_RoundTrip(t *testing.T) {
	s := &dml.CT_PositiveSize2D{Cx: 914400, Cy: 457200}
	roundtrip.Verify[*dml.CT_PositiveSize2D](t, s, ns.Qn("wp:extent"))
}

func TestCT_Blip_RoundTrip(t *testing.T) {
	b := &dml.CT_Blip{Embed: "rId4"}
	roundtrip.Verify[*dml.CT_Blip](t, b, ns.Qn("a:blip"))
	linked := `<a:blip xmlns:a="` + ns.A + `" xmlns:r="` + ns.R + `" r:link="rId2" cstate="screen"/>`
	roundtrip.VerifyBytes[*dml.CT_Blip](t, []byte(linked), ns.Qn("a:blip"))
}

func TestCT_BlipFill_Spppr_Picture_Graphic_RoundTrip(t *testing.T) {
	roundtrip.Verify[*dml.CT_BlipFillProperties](t, &dml.CT_BlipFillProperties{Blip: &dml.CT_Blip{Embed: "rId1"}}, ns.Qn("pic:blipFill"))
	roundtrip.Verify[*dml.CT_ShapeProperties](t, &dml.CT_ShapeProperties{}, ns.Qn("pic:spPr"))
	roundtrip.Verify[*dml.CT_Picture](t, &dml.CT_Picture{}, ns.Qn("pic:pic"))
	roundtrip.Verify[*dml.CT_GraphicalObject](t, &dml.CT_GraphicalObject{}, ns.Qn("a:graphic"))
	gd := &dml.CT_GraphicalObjectData{Uri: dml.PictureURI}
	roundtrip.Verify[*dml.CT_GraphicalObjectData](t, gd, ns.Qn("a:graphicData"))
}

func TestCT_ShapeProperties_ExtentWithoutTransform(t *testing.T) {
	var sp dml.CT_ShapeProperties
	if _, _, ok := sp.ExtentEMU(); ok {
		t.Error("no xfrm: ExtentEMU should report ok=false")
	}
	sp.SetExtentEMU(5, 6) // must not panic or invent a transform
	if _, _, ok := sp.ExtentEMU(); ok {
		t.Error("SetExtentEMU must not create a transform")
	}
}

func TestZeroValuesMarshalWithoutPanic(t *testing.T) {
	// Mirrors the registry audit: a zero value of each type must marshal.
	for _, el := range []interface {
		MarshalXML(*xml.Encoder, xml.StartElement) error
	}{
		new(dml.CT_Inline), new(dml.CT_PositiveSize2D), new(dml.CT_NonVisualDrawingProps),
		new(dml.CT_Blip), new(dml.CT_BlipFillProperties), new(dml.CT_ShapeProperties),
		new(dml.CT_Picture), new(dml.CT_GraphicalObject), new(dml.CT_GraphicalObjectData),
	} {
		if _, err := xml.Marshal(el); err != nil {
			t.Errorf("%T: %v", el, err)
		}
	}
}
