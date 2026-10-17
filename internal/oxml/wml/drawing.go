package wml

import (
	"encoding/xml"
	"fmt"

	"github.com/theAstiv/docx-go/internal/oxml/dml"
	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
	"github.com/theAstiv/docx-go/internal/oxml/registry"
)

// CT_Drawing is the w:drawing run child: a DrawingML object.
//
// Inline drawings (wp:inline) are typed, so callers can read and edit the
// size, alt text and image reference. Anything else, notably floating
// wp:anchor drawings, is preserved raw in Rest until anchored images are
// modelled.
type CT_Drawing struct {
	Inline *dml.CT_Inline
	Rest   []*raw.RawElement
}

func (*CT_Drawing) isRunContent() {}

// MarshalXML emits <w:drawing>.
func (d *CT_Drawing) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:drawing")
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("wml: CT_Drawing start: %w", err)
	}
	if d.Inline != nil {
		if err := d.Inline.MarshalXML(e, xml.StartElement{}); err != nil {
			return fmt.Errorf("wml: CT_Drawing inline: %w", err)
		}
	}
	if err := rawMarshalChildren(e, d.Rest); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML reads a w:drawing element.
func (d *CT_Drawing) UnmarshalXML(dec *xml.Decoder, _ xml.StartElement) error {
	d.Inline = nil
	d.Rest = nil
	for {
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("wml: CT_Drawing: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name == ns.Qn("wp:inline") && d.Inline == nil {
				d.Inline = new(dml.CT_Inline)
				if err := d.Inline.UnmarshalXML(dec, t); err != nil {
					return fmt.Errorf("wml: CT_Drawing: %w", err)
				}
				continue
			}
			re := new(raw.RawElement)
			if err := re.UnmarshalXML(dec, t); err != nil {
				return fmt.Errorf("wml: CT_Drawing: %w", err)
			}
			d.Rest = append(d.Rest, re)
		case xml.EndElement:
			return nil
		}
	}
}

// IsInline reports whether the drawing is a modelled inline picture frame.
func (d *CT_Drawing) IsInline() bool { return d.Inline != nil }

// ExtentEMU returns the displayed size in EMU; ok is false for drawings that
// are not modelled inline frames.
func (d *CT_Drawing) ExtentEMU() (cx, cy int64, ok bool) {
	if d.Inline == nil || d.Inline.Extent == nil {
		return 0, 0, false
	}
	return d.Inline.Extent.Cx, d.Inline.Extent.Cy, true
}

// SetExtentEMU sets the displayed size of an inline drawing, keeping the
// picture's own transform in step. It does nothing for other drawings.
func (d *CT_Drawing) SetExtentEMU(cx, cy int64) {
	if d.Inline != nil {
		d.Inline.SetExtentEMU(cx, cy)
	}
}

// DocPrID returns the wp:docPr id of the drawing, looking inside a raw
// wp:anchor when the drawing is not inline.
func (d *CT_Drawing) DocPrID() (int64, bool) {
	if d.Inline != nil && d.Inline.DocPr != nil {
		return d.Inline.DocPr.ID, true
	}
	for _, anchor := range d.Rest {
		for _, k := range rawKids(anchor) {
			if k.QName == ns.Qn("wp:docPr") {
				if n := parseInt64Ptr(attrLocal(k.Attrs, "id")); n != nil {
					return *n, true
				}
			}
		}
	}
	return 0, false
}

// attrLocal returns the value of the unqualified attribute named local.
func attrLocal(attrs []xml.Attr, local string) string {
	for _, a := range attrs {
		if a.Name.Space == "" && a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

// DocPrIDs returns the wp:docPr ids of every drawing in the run, including
// ones inside raw content such as mc:AlternateContent, so id allocation never
// collides with a drawing this package does not model.
func (r *CT_R) DocPrIDs() []int64 {
	var ids []int64
	for _, c := range r.Children {
		switch el := c.(type) {
		case *CT_Drawing:
			if id, ok := el.DocPrID(); ok {
				ids = append(ids, id)
			}
			for _, re := range el.Rest {
				collectDocPrIDs(re, &ids)
			}
		case *RawRunContent:
			if el.E != nil {
				collectDocPrIDs(el.E, &ids)
			}
		}
	}
	return ids
}

// collectDocPrIDs appends the id of every wp:docPr in the raw tree.
func collectDocPrIDs(re *raw.RawElement, ids *[]int64) {
	if re.QName == ns.Qn("wp:docPr") {
		if n := parseInt64Ptr(attrLocal(re.Attrs, "id")); n != nil {
			*ids = append(*ids, *n)
		}
	}
	for _, k := range rawKids(re) {
		collectDocPrIDs(k, ids)
	}
}

// Name returns the wp:docPr name of an inline drawing, or "".
func (d *CT_Drawing) Name() string {
	if d.Inline == nil || d.Inline.DocPr == nil {
		return ""
	}
	return d.Inline.DocPr.Name
}

// AltText returns the wp:docPr description of an inline drawing, or "".
func (d *CT_Drawing) AltText() string {
	if d.Inline == nil || d.Inline.DocPr == nil || d.Inline.DocPr.Descr == nil {
		return ""
	}
	return *d.Inline.DocPr.Descr
}

// SetAltText sets wp:docPr/@descr. An empty string is written as descr=""
// (marking the image decorative), not removed.
func (d *CT_Drawing) SetAltText(s string) {
	if d.Inline == nil || d.Inline.DocPr == nil {
		return
	}
	d.Inline.DocPr.Descr = &s
}

// Title returns the wp:docPr title of an inline drawing, or "".
func (d *CT_Drawing) Title() string {
	if d.Inline == nil || d.Inline.DocPr == nil || d.Inline.DocPr.Title == nil {
		return ""
	}
	return *d.Inline.DocPr.Title
}

// SetTitle sets wp:docPr/@title. An empty string removes the attribute.
func (d *CT_Drawing) SetTitle(s string) {
	if d.Inline == nil || d.Inline.DocPr == nil {
		return
	}
	if s == "" {
		d.Inline.DocPr.Title = nil
		return
	}
	d.Inline.DocPr.Title = &s
}

// EmbedRID returns the relationship ID of the embedded image, or "".
func (d *CT_Drawing) EmbedRID() string {
	if d.Inline == nil {
		return ""
	}
	return d.Inline.Picture().EmbedRID()
}

// InlinePictureSpec describes a new inline picture for NewInlinePicture.
type InlinePictureSpec struct {
	// RID is the image relationship ID the picture embeds.
	RID string
	// FileName names the picture (pic:cNvPr/@name), usually the media file name.
	FileName string
	// CX and CY are the displayed size in EMU.
	CX, CY int64
	// DocPrID is the document-unique drawing id.
	DocPrID int64
	// AltText is written as wp:docPr/@descr (empty marks the image decorative).
	AltText string
	// Title is written as wp:docPr/@title when non-empty.
	Title string
}

// dmlEl builds a raw DrawingML element: element name from a prefixed name,
// unqualified attributes from alternating key, value arguments.
func dmlEl(name string, kv ...string) *raw.RawElement {
	re := new(raw.RawElement)
	re.QName = ns.Qn(name)
	for i := 0; i+1 < len(kv); i += 2 {
		re.Attrs = append(re.Attrs, xml.Attr{Name: xml.Name{Local: kv[i]}, Value: kv[i+1]})
	}
	return re
}

func dmlParent(name string, kids ...*raw.RawElement) *raw.RawElement {
	re := dmlEl(name)
	for _, k := range kids {
		re.Children = append(re.Children, k)
	}
	return re
}

// NewInlinePicture builds a w:drawing holding a picture in the form Word
// writes for an inserted inline image.
func NewInlinePicture(spec InlinePictureSpec) *CT_Drawing {
	cx, cy := fmt.Sprint(spec.CX), fmt.Sprint(spec.CY)

	pic := new(dml.CT_Picture)
	pic.NvPicPr = dmlParent("pic:nvPicPr",
		dmlEl("pic:cNvPr", "id", "0", "name", spec.FileName),
		dmlEl("pic:cNvPicPr"))
	pic.BlipFill = new(dml.CT_BlipFillProperties)
	pic.BlipFill.Blip = new(dml.CT_Blip)
	pic.BlipFill.Blip.Embed = spec.RID
	pic.BlipFill.Rest = []*raw.RawElement{
		dmlParent("a:stretch", dmlEl("a:fillRect")),
	}
	pic.SpPr = new(dml.CT_ShapeProperties)
	pic.SpPr.Children = []*raw.RawElement{
		dmlParent("a:xfrm",
			dmlEl("a:off", "x", "0", "y", "0"),
			dmlEl("a:ext", "cx", cx, "cy", cy)),
		dmlParent("a:prstGeom", dmlEl("a:avLst")),
	}
	// The pic prefix is declared on pic:pic in Word's output; the encoder adds
	// the declaration itself.

	gd := new(dml.CT_GraphicalObjectData)
	gd.Uri = dml.PictureURI
	gd.Pic = pic
	graphic := new(dml.CT_GraphicalObject)
	graphic.GraphicData = gd

	docPr := new(dml.CT_NonVisualDrawingProps)
	docPr.ID = spec.DocPrID
	docPr.Name = fmt.Sprintf("Picture %d", spec.DocPrID)
	alt := spec.AltText
	docPr.Descr = &alt
	if spec.Title != "" {
		title := spec.Title
		docPr.Title = &title
	}

	in := new(dml.CT_Inline)
	in.Attrs = []xml.Attr{
		{Name: xml.Name{Local: "distT"}, Value: "0"},
		{Name: xml.Name{Local: "distB"}, Value: "0"},
		{Name: xml.Name{Local: "distL"}, Value: "0"},
		{Name: xml.Name{Local: "distR"}, Value: "0"},
	}
	in.Extent = new(dml.CT_PositiveSize2D)
	in.Extent.Cx, in.Extent.Cy = spec.CX, spec.CY
	in.EffectExtent = dmlEl("wp:effectExtent", "l", "0", "t", "0", "r", "0", "b", "0")
	in.DocPr = docPr
	in.CNvGraphicFramePr = dmlParent("wp:cNvGraphicFramePr",
		dmlEl("a:graphicFrameLocks", "noChangeAspect", "1"))
	in.Graphic = graphic

	d := new(CT_Drawing)
	d.Inline = in
	return d
}

func init() {
	registry.Register(ns.Qn("w:drawing"), func() registry.Element { return new(CT_Drawing) })
}
