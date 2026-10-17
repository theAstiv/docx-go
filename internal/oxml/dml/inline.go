package dml

import (
	"encoding/xml"
	"fmt"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
	"github.com/theAstiv/docx-go/internal/oxml/registry"
)

// CT_Inline is the wp:inline element: a drawing laid out in the text flow.
//
// Extent, DocPr and Graphic are typed. EffectExtent and CNvGraphicFramePr stay
// raw; Extras holds anything else and is emitted last.
type CT_Inline struct {
	// Attrs preserves distT, distB, distL, distR and any other attribute.
	Attrs             []xml.Attr
	Extent            *CT_PositiveSize2D
	EffectExtent      *raw.RawElement
	DocPr             *CT_NonVisualDrawingProps
	CNvGraphicFramePr *raw.RawElement
	Graphic           *CT_GraphicalObject
	Extras            []*raw.RawElement
}

// MarshalXML emits <wp:inline> with children in schema order.
func (i *CT_Inline) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("wp:inline")
	start.Attr = append(start.Attr, i.Attrs...)
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("dml: inline start: %w", err)
	}
	if i.Extent != nil {
		if err := i.Extent.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	if i.EffectExtent != nil {
		if err := i.EffectExtent.MarshalXML(e, xml.StartElement{}); err != nil {
			return fmt.Errorf("dml: inline effectExtent: %w", err)
		}
	}
	if i.DocPr != nil {
		if err := i.DocPr.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	if i.CNvGraphicFramePr != nil {
		if err := i.CNvGraphicFramePr.MarshalXML(e, xml.StartElement{}); err != nil {
			return fmt.Errorf("dml: inline cNvGraphicFramePr: %w", err)
		}
	}
	if i.Graphic != nil {
		if err := i.Graphic.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	if err := marshalRaw(e, i.Extras); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML reads a wp:inline element.
func (i *CT_Inline) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	i.Attrs, i.Extent, i.EffectExtent, i.DocPr, i.CNvGraphicFramePr, i.Graphic, i.Extras =
		nil, nil, nil, nil, nil, nil, nil
	for _, a := range start.Attr {
		if !isNSDecl(a) {
			i.Attrs = append(i.Attrs, a)
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("dml: inline: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if err := i.readChild(d, t); err != nil {
				return err
			}
		case xml.EndElement:
			return nil
		}
	}
}

func (i *CT_Inline) readChild(d *xml.Decoder, t xml.StartElement) error {
	switch {
	case t.Name == ns.Qn("wp:extent") && i.Extent == nil:
		i.Extent = new(CT_PositiveSize2D)
		return i.Extent.UnmarshalXML(d, t)
	case t.Name == ns.Qn("wp:effectExtent") && i.EffectExtent == nil:
		re, err := readRaw(d, t)
		i.EffectExtent = re
		return err
	case t.Name == ns.Qn("wp:docPr") && i.DocPr == nil:
		i.DocPr = new(CT_NonVisualDrawingProps)
		return i.DocPr.UnmarshalXML(d, t)
	case t.Name == ns.Qn("wp:cNvGraphicFramePr") && i.CNvGraphicFramePr == nil:
		re, err := readRaw(d, t)
		i.CNvGraphicFramePr = re
		return err
	case t.Name == ns.Qn("a:graphic") && i.Graphic == nil:
		i.Graphic = new(CT_GraphicalObject)
		return i.Graphic.UnmarshalXML(d, t)
	default:
		re, err := readRaw(d, t)
		if err != nil {
			return err
		}
		i.Extras = append(i.Extras, re)
		return nil
	}
}

// Picture returns the inline's pic:pic payload, or nil.
func (i *CT_Inline) Picture() *CT_Picture { return i.Graphic.Picture() }

// SetExtentEMU sets the displayed size in EMU. The picture's own transform
// (pic:spPr/a:xfrm/a:ext) is kept in step, because Word renders from the
// transform and a mismatch distorts or crops the image.
func (i *CT_Inline) SetExtentEMU(cx, cy int64) {
	if i.Extent == nil {
		i.Extent = new(CT_PositiveSize2D)
	}
	i.Extent.Cx, i.Extent.Cy = cx, cy
	if pic := i.Picture(); pic != nil && pic.SpPr != nil {
		pic.SpPr.SetExtentEMU(cx, cy)
	}
}

func init() {
	registry.Register(ns.Qn("wp:inline"), func() registry.Element { return new(CT_Inline) })
}
