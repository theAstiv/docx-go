package dml

import (
	"encoding/xml"
	"fmt"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
	"github.com/theAstiv/docx-go/internal/oxml/registry"
)

// CT_Blip is the a:blip element: a reference to an image part.
//
// Embed is the r:embed relationship ID of an image stored in the package;
// Link is r:link for an externally linked image. Effects and the extension
// list (a14, asvg:svgBlip, ...) are preserved as raw children.
type CT_Blip struct {
	Embed string
	Link  string
	// ExtraAttrs preserves other attributes (cstate, ...).
	ExtraAttrs []xml.Attr
	Children   []*raw.RawElement
}

// MarshalXML emits <a:blip>.
func (b *CT_Blip) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("a:blip")
	if b.Embed != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: ns.Qn("r:embed"), Value: b.Embed})
	}
	if b.Link != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: ns.Qn("r:link"), Value: b.Link})
	}
	start.Attr = append(start.Attr, b.ExtraAttrs...)
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("dml: blip start: %w", err)
	}
	if err := marshalRaw(e, b.Children); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML reads an a:blip element.
func (b *CT_Blip) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	b.Embed, b.Link, b.ExtraAttrs, b.Children = "", "", nil, nil
	for _, a := range start.Attr {
		switch {
		case isNSDecl(a):
		case a.Name.Space == ns.R && a.Name.Local == "embed":
			b.Embed = a.Value
		case a.Name.Space == ns.R && a.Name.Local == "link":
			b.Link = a.Value
		default:
			b.ExtraAttrs = append(b.ExtraAttrs, a)
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("dml: blip: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			re, err := readRaw(d, t)
			if err != nil {
				return err
			}
			b.Children = append(b.Children, re)
		case xml.EndElement:
			return nil
		}
	}
}

func init() {
	registry.Register(ns.Qn("a:blip"), func() registry.Element { return new(CT_Blip) })
}
