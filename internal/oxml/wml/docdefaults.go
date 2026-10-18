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

	// bare records that the wrapper was read without its inner element
	// (<w:rPrDefault/>). RPr/PPr are then non-nil but empty, and are written
	// back as a bare wrapper until a property is set on them.
	rprBare, pprBare bool
	// rprExtras and pprExtras hold unknown children of the wrapper itself,
	// written back inside it rather than promoted to docDefaults.
	rprExtras, pprExtras []*raw.RawElement
}

// MarshalXML emits <w:docDefaults>.
func (dd *CT_DocDefaults) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:docDefaults")
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("wml: CT_DocDefaults start: %w", err)
	}
	if dd.RPr != nil {
		if err := marshalWrapped(e, ns.Qn("w:rPrDefault"), dd.RPr, dd.rprBare && dd.RPr.isEmpty(), dd.rprExtras); err != nil {
			return err
		}
	}
	if dd.PPr != nil {
		if err := marshalWrapped(e, ns.Qn("w:pPrDefault"), dd.PPr, dd.pprBare && dd.PPr.isEmpty(), dd.pprExtras); err != nil {
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

// marshalWrapped emits <wrapper>child extras...</wrapper>. With omitChild the
// child is left out, reproducing a wrapper that had none.
func marshalWrapped(e *xml.Encoder, wrapper xml.Name, child xml.Marshaler, omitChild bool, extras []*raw.RawElement) error {
	ws := xml.StartElement{Name: wrapper}
	if err := e.EncodeToken(ws); err != nil {
		return fmt.Errorf("wml: %s start: %w", wrapper.Local, err)
	}
	if !omitChild {
		if err := child.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	for _, x := range extras {
		if err := x.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: wrapper})
}

// UnmarshalXML reads a w:docDefaults element.
func (dd *CT_DocDefaults) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	dd.RPr = nil
	dd.PPr = nil
	dd.extras = nil
	dd.rprBare, dd.pprBare = false, false
	dd.rprExtras, dd.pprExtras = nil, nil
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
				ok, err := unmarshalWrapped(d, ns.Qn("w:rPr"), rpr, &dd.rprExtras)
				if err != nil {
					return err
				}
				dd.RPr = rpr
				dd.rprBare = !ok
			case ns.Qn("w:pPrDefault"):
				ppr := new(CT_PPr)
				ok, err := unmarshalWrapped(d, ns.Qn("w:pPr"), ppr, &dd.pprExtras)
				if err != nil {
					return err
				}
				dd.PPr = ppr
				dd.pprBare = !ok
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
