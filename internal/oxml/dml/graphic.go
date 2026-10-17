package dml

import (
	"encoding/xml"
	"fmt"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
	"github.com/theAstiv/docx-go/internal/oxml/registry"
)

// PictureURI is the a:graphicData/@uri value that marks a picture payload.
const PictureURI = "http://schemas.openxmlformats.org/drawingml/2006/picture"

// CT_GraphicalObjectData is the a:graphicData element: the payload of a
// graphic frame. Pictures are typed; any other payload (charts, diagrams,
// shapes) is preserved raw in Rest.
type CT_GraphicalObjectData struct {
	Uri string
	// ExtraAttrs preserves other attributes.
	ExtraAttrs []xml.Attr
	Pic        *CT_Picture
	Rest       []*raw.RawElement
}

// MarshalXML emits <a:graphicData>.
func (g *CT_GraphicalObjectData) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("a:graphicData")
	start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "uri"}, Value: g.Uri})
	start.Attr = append(start.Attr, g.ExtraAttrs...)
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("dml: graphicData start: %w", err)
	}
	if g.Pic != nil {
		if err := g.Pic.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	if err := marshalRaw(e, g.Rest); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML reads an a:graphicData element.
func (g *CT_GraphicalObjectData) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	g.Uri, g.ExtraAttrs, g.Pic, g.Rest = "", nil, nil, nil
	for _, a := range start.Attr {
		switch {
		case isNSDecl(a):
		case a.Name.Space == "" && a.Name.Local == "uri":
			g.Uri = a.Value
		default:
			g.ExtraAttrs = append(g.ExtraAttrs, a)
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("dml: graphicData: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name == ns.Qn("pic:pic") && g.Pic == nil {
				g.Pic = new(CT_Picture)
				if err := g.Pic.UnmarshalXML(d, t); err != nil {
					return err
				}
				continue
			}
			re, err := readRaw(d, t)
			if err != nil {
				return err
			}
			g.Rest = append(g.Rest, re)
		case xml.EndElement:
			return nil
		}
	}
}

// CT_GraphicalObject is the a:graphic element.
type CT_GraphicalObject struct {
	// Attrs preserves non-namespace-declaration attributes.
	Attrs       []xml.Attr
	GraphicData *CT_GraphicalObjectData
	Rest        []*raw.RawElement
}

// MarshalXML emits <a:graphic>.
func (g *CT_GraphicalObject) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("a:graphic")
	start.Attr = append(start.Attr, g.Attrs...)
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("dml: graphic start: %w", err)
	}
	if g.GraphicData != nil {
		if err := g.GraphicData.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	if err := marshalRaw(e, g.Rest); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML reads an a:graphic element.
func (g *CT_GraphicalObject) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	g.Attrs, g.GraphicData, g.Rest = nil, nil, nil
	for _, a := range start.Attr {
		if !isNSDecl(a) {
			g.Attrs = append(g.Attrs, a)
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("dml: graphic: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name == ns.Qn("a:graphicData") && g.GraphicData == nil {
				g.GraphicData = new(CT_GraphicalObjectData)
				if err := g.GraphicData.UnmarshalXML(d, t); err != nil {
					return err
				}
				continue
			}
			re, err := readRaw(d, t)
			if err != nil {
				return err
			}
			g.Rest = append(g.Rest, re)
		case xml.EndElement:
			return nil
		}
	}
}

// Picture returns the pic:pic payload, or nil when the graphic holds
// something else.
func (g *CT_GraphicalObject) Picture() *CT_Picture {
	if g == nil || g.GraphicData == nil {
		return nil
	}
	return g.GraphicData.Pic
}

func init() {
	registry.Register(ns.Qn("a:graphic"), func() registry.Element { return new(CT_GraphicalObject) })
	registry.Register(ns.Qn("a:graphicData"), func() registry.Element { return new(CT_GraphicalObjectData) })
}
