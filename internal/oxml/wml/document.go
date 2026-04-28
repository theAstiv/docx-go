package wml

import (
	"encoding/xml"
	"fmt"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
	"github.com/theAstiv/docx-go/internal/oxml/registry"
)

// CT_Document is the w:document element: the root of document.xml.
//
// Attrs captures non-namespace attributes (e.g. mc:Ignorable) for round-trip
// fidelity. rawRest preserves any siblings of w:body (e.g. w:background) that
// this type does not model. Body holds the document body.
type CT_Document struct {
	Attrs   []xml.Attr
	rawRest []*raw.RawElement
	Body    *CT_Body
}

// MarshalXML emits <w:document>.
func (d *CT_Document) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:document")
	start.Attr = append(start.Attr, d.Attrs...)
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("wml: CT_Document start: %w", err)
	}
	for _, child := range d.rawRest {
		if err := child.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	if d.Body != nil {
		if err := d.Body.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML reads a w:document element.
func (d *CT_Document) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	d.Attrs = d.Attrs[:0]
	d.rawRest = nil
	d.Body = nil
	// Preserve xmlns:prefix declarations (Space=="xmlns") so the nsfix
	// post-processor can reconstruct them on the output root element.
	// The default namespace (Space=="", Local=="xmlns") is intentionally
	// dropped because encoding/xml auto-generates it from the element name,
	// and including it in Attrs would produce a duplicate xmlns= attribute.
	for _, a := range start.Attr {
		if a.Name.Space == "" && a.Name.Local == "xmlns" {
			continue
		}
		d.Attrs = append(d.Attrs, a)
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("wml: CT_Document: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name == ns.Qn("w:body") {
				d.Body = new(CT_Body)
				if err := d.Body.UnmarshalXML(dec, t); err != nil {
					return err
				}
			} else {
				re := new(raw.RawElement)
				if err := re.UnmarshalXML(dec, t); err != nil {
					return err
				}
				d.rawRest = append(d.rawRest, re)
			}
		case xml.EndElement:
			return nil
		}
	}
}

func init() {
	registry.Register(ns.Qn("w:document"), func() registry.Element { return new(CT_Document) })
}
