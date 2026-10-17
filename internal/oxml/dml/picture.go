package dml

import (
	"encoding/xml"
	"fmt"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
	"github.com/theAstiv/docx-go/internal/oxml/registry"
)

// CT_Picture is the pic:pic element: a picture.
//
// NvPicPr (pic:nvPicPr) is kept raw. BlipFill and SpPr are typed because the
// image reference and the transform size are what callers edit. Extras holds
// any further child (pic:style, extensions) and follows SpPr.
type CT_Picture struct {
	// Attrs preserves attributes on pic:pic (none are defined by the schema).
	Attrs    []xml.Attr
	NvPicPr  *raw.RawElement
	BlipFill *CT_BlipFillProperties
	SpPr     *CT_ShapeProperties
	Extras   []*raw.RawElement
}

// MarshalXML emits <pic:pic>.
func (p *CT_Picture) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("pic:pic")
	start.Attr = append(start.Attr, p.Attrs...)
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("dml: pic start: %w", err)
	}
	if p.NvPicPr != nil {
		if err := p.NvPicPr.MarshalXML(e, xml.StartElement{}); err != nil {
			return fmt.Errorf("dml: pic nvPicPr: %w", err)
		}
	}
	if p.BlipFill != nil {
		if err := p.BlipFill.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	if p.SpPr != nil {
		if err := p.SpPr.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	if err := marshalRaw(e, p.Extras); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML reads a pic:pic element.
func (p *CT_Picture) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	p.Attrs, p.NvPicPr, p.BlipFill, p.SpPr, p.Extras = nil, nil, nil, nil, nil
	for _, a := range start.Attr {
		if !isNSDecl(a) {
			p.Attrs = append(p.Attrs, a)
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("dml: pic: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch {
			case t.Name == ns.Qn("pic:nvPicPr") && p.NvPicPr == nil:
				re, err := readRaw(d, t)
				if err != nil {
					return err
				}
				p.NvPicPr = re
			case t.Name == ns.Qn("pic:blipFill") && p.BlipFill == nil:
				p.BlipFill = new(CT_BlipFillProperties)
				if err := p.BlipFill.UnmarshalXML(d, t); err != nil {
					return err
				}
			case t.Name == ns.Qn("pic:spPr") && p.SpPr == nil:
				p.SpPr = new(CT_ShapeProperties)
				if err := p.SpPr.UnmarshalXML(d, t); err != nil {
					return err
				}
			default:
				re, err := readRaw(d, t)
				if err != nil {
					return err
				}
				p.Extras = append(p.Extras, re)
			}
		case xml.EndElement:
			return nil
		}
	}
}

// EmbedRID returns the relationship ID of the embedded image, or "".
func (p *CT_Picture) EmbedRID() string {
	if p == nil || p.BlipFill == nil || p.BlipFill.Blip == nil {
		return ""
	}
	return p.BlipFill.Blip.Embed
}

func init() {
	registry.Register(ns.Qn("pic:pic"), func() registry.Element { return new(CT_Picture) })
}
