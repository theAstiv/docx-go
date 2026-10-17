package dml

import (
	"encoding/xml"
	"fmt"
	"strconv"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
	"github.com/theAstiv/docx-go/internal/oxml/registry"
)

// CT_NonVisualDrawingProps is the wp:docPr element: identity and accessibility
// text of a drawing.
//
// Descr and Title are pointers so that an absent attribute and an explicitly
// empty one (descr="" marks a decorative image) stay distinct.
type CT_NonVisualDrawingProps struct {
	ID    int64
	Name  string
	Descr *string
	Title *string
	// ExtraAttrs preserves other attributes (hidden, ...).
	ExtraAttrs []xml.Attr
	// Children preserves a:hlinkClick, a:hlinkHover, a:extLst and the like.
	Children []*raw.RawElement
}

// MarshalXML emits <wp:docPr>.
func (p *CT_NonVisualDrawingProps) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("wp:docPr")
	start.Attr = append(start.Attr,
		xml.Attr{Name: xml.Name{Local: "id"}, Value: strconv.FormatInt(p.ID, 10)},
		xml.Attr{Name: xml.Name{Local: "name"}, Value: p.Name},
	)
	if p.Descr != nil {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "descr"}, Value: *p.Descr})
	}
	if p.Title != nil {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "title"}, Value: *p.Title})
	}
	start.Attr = append(start.Attr, p.ExtraAttrs...)
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("dml: docPr start: %w", err)
	}
	if err := marshalRaw(e, p.Children); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML reads a wp:docPr element.
func (p *CT_NonVisualDrawingProps) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	p.ID, p.Name, p.Descr, p.Title, p.ExtraAttrs, p.Children = 0, "", nil, nil, nil, nil
	for _, a := range start.Attr {
		switch {
		case isNSDecl(a):
		case a.Name.Space == "" && a.Name.Local == "id":
			p.ID = attrInt64(a.Value)
		case a.Name.Space == "" && a.Name.Local == "name":
			p.Name = a.Value
		case a.Name.Space == "" && a.Name.Local == "descr":
			v := a.Value
			p.Descr = &v
		case a.Name.Space == "" && a.Name.Local == "title":
			v := a.Value
			p.Title = &v
		default:
			p.ExtraAttrs = append(p.ExtraAttrs, a)
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("dml: docPr: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			re, err := readRaw(d, t)
			if err != nil {
				return err
			}
			p.Children = append(p.Children, re)
		case xml.EndElement:
			return nil
		}
	}
}

func init() {
	registry.Register(ns.Qn("wp:docPr"), func() registry.Element { return new(CT_NonVisualDrawingProps) })
}
