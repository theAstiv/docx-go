package wml

import (
	"encoding/xml"
	"fmt"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/registry"
)

// CT_Body is the w:body element.
//
// Children holds all block-level content (paragraphs, tables, etc.) in
// document order. SectPr holds the final body-level section properties.
type CT_Body struct {
	Children []BlockElement
	SectPr   *CT_SectPr
}

// MarshalXML emits <w:body>.
func (b *CT_Body) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:body")
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("wml: CT_Body start: %w", err)
	}
	for _, child := range b.Children {
		if err := child.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	if b.SectPr != nil {
		if err := b.SectPr.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML reads a w:body element.
func (b *CT_Body) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	b.Children = b.Children[:0]
	b.SectPr = nil
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("wml: CT_Body: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name {
			case ns.Qn("w:p"):
				p := new(CT_P)
				if err := p.UnmarshalXML(d, t); err != nil {
					return err
				}
				b.Children = append(b.Children, p)
			case ns.Qn("w:tbl"):
				tbl := new(CT_Tbl)
				if err := tbl.UnmarshalXML(d, t); err != nil {
					return err
				}
				b.Children = append(b.Children, tbl)
			case ns.Qn("w:sectPr"):
				b.SectPr = new(CT_SectPr)
				if err := b.SectPr.UnmarshalXML(d, t); err != nil {
					return err
				}
			default:
				rb := new(RawBlock)
				if err := rb.UnmarshalXML(d, t); err != nil {
					return err
				}
				b.Children = append(b.Children, rb)
			}
		case xml.EndElement:
			return nil
		}
	}
}

func init() {
	registry.Register(ns.Qn("w:body"), func() registry.Element { return new(CT_Body) })
}
