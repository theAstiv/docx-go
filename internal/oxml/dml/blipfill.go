package dml

import (
	"encoding/xml"
	"fmt"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
	"github.com/theAstiv/docx-go/internal/oxml/registry"
)

// CT_BlipFillProperties is the pic:blipFill element of a picture.
//
// Blip is the image reference. Rest holds the crop (a:srcRect) and the fill
// mode (a:stretch or a:tile), which follow the blip in the schema sequence.
type CT_BlipFillProperties struct {
	// Attrs preserves dpi, rotWithShape.
	Attrs []xml.Attr
	Blip  *CT_Blip
	Rest  []*raw.RawElement
}

// MarshalXML emits <pic:blipFill>.
func (f *CT_BlipFillProperties) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("pic:blipFill")
	start.Attr = append(start.Attr, f.Attrs...)
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("dml: blipFill start: %w", err)
	}
	if f.Blip != nil {
		if err := f.Blip.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	if err := marshalRaw(e, f.Rest); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML reads a pic:blipFill element.
func (f *CT_BlipFillProperties) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	f.Attrs, f.Blip, f.Rest = nil, nil, nil
	for _, a := range start.Attr {
		if !isNSDecl(a) {
			f.Attrs = append(f.Attrs, a)
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("dml: blipFill: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name == ns.Qn("a:blip") && f.Blip == nil {
				f.Blip = new(CT_Blip)
				if err := f.Blip.UnmarshalXML(d, t); err != nil {
					return err
				}
				continue
			}
			re, err := readRaw(d, t)
			if err != nil {
				return err
			}
			f.Rest = append(f.Rest, re)
		case xml.EndElement:
			return nil
		}
	}
}

func init() {
	registry.Register(ns.Qn("pic:blipFill"), func() registry.Element { return new(CT_BlipFillProperties) })
}
