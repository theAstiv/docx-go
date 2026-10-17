package wml

import (
	"encoding/xml"
	"fmt"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
	"github.com/theAstiv/docx-go/internal/oxml/registry"
)

// CT_DocDefaults is the w:docDefaults element: document-wide default run and
// paragraph properties.
//
// Only the two schema children are modelled: w:rPrDefault/w:rPr and
// w:pPrDefault/w:pPr. Anything else (extension children) is preserved in
// extras and emitted after the typed children.
type CT_DocDefaults struct {
	// RPr is the content of w:rPrDefault; nil when the wrapper is absent.
	RPr *CT_RPr
	// PPr is the content of w:pPrDefault; nil when the wrapper is absent.
	PPr    *CT_PPr
	extras []*raw.RawElement
}

// MarshalXML emits <w:docDefaults>.
func (dd *CT_DocDefaults) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:docDefaults")
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("wml: CT_DocDefaults start: %w", err)
	}
	if dd.RPr != nil {
		if err := marshalWrapped(e, ns.Qn("w:rPrDefault"), dd.RPr); err != nil {
			return err
		}
	}
	if dd.PPr != nil {
		if err := marshalWrapped(e, ns.Qn("w:pPrDefault"), dd.PPr); err != nil {
			return err
		}
	}
	for _, x := range dd.extras {
		if err := x.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// marshalWrapped emits <wrapper>child</wrapper>.
func marshalWrapped(e *xml.Encoder, wrapper xml.Name, child xml.Marshaler) error {
	ws := xml.StartElement{Name: wrapper}
	if err := e.EncodeToken(ws); err != nil {
		return fmt.Errorf("wml: %s start: %w", wrapper.Local, err)
	}
	if err := child.MarshalXML(e, xml.StartElement{}); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: wrapper})
}

// UnmarshalXML reads a w:docDefaults element.
func (dd *CT_DocDefaults) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	dd.RPr = nil
	dd.PPr = nil
	dd.extras = nil
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("wml: CT_DocDefaults: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name {
			case ns.Qn("w:rPrDefault"):
				rpr := new(CT_RPr)
				ok, err := unmarshalWrapped(d, ns.Qn("w:rPr"), rpr, &dd.extras)
				if err != nil {
					return err
				}
				if ok {
					dd.RPr = rpr
				} else {
					dd.RPr = new(CT_RPr)
				}
			case ns.Qn("w:pPrDefault"):
				ppr := new(CT_PPr)
				ok, err := unmarshalWrapped(d, ns.Qn("w:pPr"), ppr, &dd.extras)
				if err != nil {
					return err
				}
				if ok {
					dd.PPr = ppr
				} else {
					dd.PPr = new(CT_PPr)
				}
			default:
				re := new(raw.RawElement)
				if err := re.UnmarshalXML(d, t); err != nil {
					return err
				}
				dd.extras = append(dd.extras, re)
			}
		case xml.EndElement:
			return nil
		}
	}
}

// unmarshalWrapped reads the body of a wrapper element, decoding the child
// named want into dst. Other children are appended to extras. It reports
// whether want was found.
func unmarshalWrapped(d *xml.Decoder, want xml.Name, dst xml.Unmarshaler, extras *[]*raw.RawElement) (bool, error) {
	found := false
	for {
		tok, err := d.Token()
		if err != nil {
			return false, fmt.Errorf("wml: docDefaults wrapper: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name == want && !found {
				if err := dst.UnmarshalXML(d, t); err != nil {
					return false, err
				}
				found = true
				continue
			}
			re := new(raw.RawElement)
			if err := re.UnmarshalXML(d, t); err != nil {
				return false, err
			}
			*extras = append(*extras, re)
		case xml.EndElement:
			return found, nil
		}
	}
}

func init() {
	registry.Register(ns.Qn("w:docDefaults"), func() registry.Element { return new(CT_DocDefaults) })
}
