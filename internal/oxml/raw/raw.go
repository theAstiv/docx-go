package raw

import (
	"encoding/xml"
	"fmt"
)

// RawElement captures an unrecognised XML element as a token stream.
//
// Promise: same QName, attribute set, child order, and text-node bytes on
// round-trip.  Namespace prefixes may be re-canonicalised because
// encoding/xml does not expose source byte offsets.
type RawElement struct {
	QName    xml.Name
	Attrs    []xml.Attr
	Children []xml.Token // xml.CharData, xml.Comment, xml.ProcInst, or *RawElement
}

// UnmarshalXML consumes tokens until the matching EndElement.  Nested
// unknown child elements are recursively wrapped as *RawElement.
// CharData bytes are cloned because the decoder reuses its buffer.
func (r *RawElement) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	r.QName = start.Name
	// Namespace-declaration attributes (Space=="xmlns") must not be stored —
	// encoding/xml re-emits them automatically from xml.Name.Space fields.
	// Storing them causes double-declaration corruption on re-encode.
	r.Attrs = r.Attrs[:0]
	for _, a := range start.Attr {
		if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
			continue
		}
		r.Attrs = append(r.Attrs, a)
	}
	r.Children = r.Children[:0]

	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("raw: reading children of %v: %w", r.QName, err)
		}
		switch t := tok.(type) {
		case xml.EndElement:
			return nil
		case xml.StartElement:
			child := &RawElement{} //nolint:exhaustruct_v5 // zero-value decode target; UnmarshalXML populates all fields
			if err := child.UnmarshalXML(d, t); err != nil {
				return err
			}
			r.Children = append(r.Children, child)
		case xml.CharData:
			cp := make(xml.CharData, len(t))
			copy(cp, t)
			r.Children = append(r.Children, cp)
		case xml.Comment:
			cp := make(xml.Comment, len(t))
			copy(cp, t)
			r.Children = append(r.Children, cp)
		case xml.ProcInst:
			r.Children = append(r.Children, xml.ProcInst{
				Target: t.Target,
				Inst:   append([]byte(nil), t.Inst...),
			})
		}
	}
}

// MarshalXML replays the captured token stream.
func (r *RawElement) MarshalXML(e *xml.Encoder, _ xml.StartElement) error {
	start := xml.StartElement{Name: r.QName, Attr: r.Attrs}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for _, child := range r.Children {
		switch c := child.(type) {
		case *RawElement:
			if err := c.MarshalXML(e, xml.StartElement{}); err != nil {
				return err
			}
		case xml.CharData:
			if err := e.EncodeToken(c); err != nil {
				return err
			}
		case xml.Comment:
			if err := e.EncodeToken(c); err != nil {
				return err
			}
		case xml.ProcInst:
			if err := e.EncodeToken(c); err != nil {
				return err
			}
		}
	}
	return e.EncodeToken(xml.EndElement{Name: r.QName})
}
