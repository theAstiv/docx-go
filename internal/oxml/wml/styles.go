package wml

import (
	"encoding/xml"
	"fmt"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
	"github.com/theAstiv/docx-go/internal/oxml/registry"
)

// CT_Styles is the w:styles element: the root of styles.xml.
//
// Attrs captures element attributes for round-trip fidelity.
// DocDefaults and latentStyles are preserved as raw children in rawRest;
// named styles are decoded into Styles for typed access.
//
// Ordering note: MarshalXML always emits rawRest before Styles. This matches
// the canonical Word ordering (docDefaults → latentStyles → style…). Documents
// with interleaved ordering will have their order normalised on round-trip.
// Phase 2 can improve this by recording each child's insertion index.
type CT_Styles struct {
	Attrs   []xml.Attr
	Styles  []*CT_Style
	rawRest []*raw.RawElement // docDefaults, latentStyles, and unknown children
}

// MarshalXML emits <w:styles>.
func (s *CT_Styles) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:styles")
	start.Attr = append(start.Attr, s.Attrs...)
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("wml: CT_Styles start: %w", err)
	}
	// Emit raw non-style children first (docDefaults, latentStyles, etc.)
	for _, child := range s.rawRest {
		if err := child.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	for _, style := range s.Styles {
		if err := style.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML reads a w:styles element.
func (s *CT_Styles) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	s.Attrs = s.Attrs[:0]
	s.Styles = nil
	s.rawRest = nil
	for _, a := range start.Attr {
		if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
			continue
		}
		s.Attrs = append(s.Attrs, a)
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("wml: CT_Styles: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name == ns.Qn("w:style") {
				st := new(CT_Style)
				if err := st.UnmarshalXML(d, t); err != nil {
					return err
				}
				s.Styles = append(s.Styles, st)
			} else {
				re := new(raw.RawElement)
				if err := re.UnmarshalXML(d, t); err != nil {
					return err
				}
				s.rawRest = append(s.rawRest, re)
			}
		case xml.EndElement:
			return nil
		}
	}
}

// ByID returns the style with the given ID, or nil.
func (s *CT_Styles) ByID(id string) *CT_Style {
	for _, st := range s.Styles {
		if st.StyleID == id {
			return st
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// CT_Style — w:style
// ---------------------------------------------------------------------------

// CT_Style is the w:style element: a named style definition.
//
// StyleID and Type are the two discriminating attributes. PPr and RPr hold
// the paragraph and run properties for the style. All other children are
// preserved as raw for round-trip fidelity.
type CT_Style struct {
	// StyleID is the w:styleId attribute value.
	StyleID string
	// Type is the w:type attribute value ("paragraph", "character", "table", "numbering").
	Type string
	// Default indicates this is the document default for its type.
	Default bool
	// CustomStyle is the w:customStyle attribute.
	CustomStyle bool
	// Name is the human-readable w:name/@w:val.
	Name string
	// PPr holds paragraph properties for paragraph-type styles.
	PPr *CT_PPr
	// RPr holds run properties for character/paragraph styles.
	RPr *CT_RPr
	// rawRest preserves unmodelled children (w:basedOn, w:next, w:link, etc.)
	rawRest []*raw.RawElement
}

// MarshalXML emits <w:style>.
func (s *CT_Style) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:style")
	if s.Type != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: ns.Qn("w:type"), Value: s.Type})
	}
	if s.StyleID != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: ns.Qn("w:styleId"), Value: s.StyleID})
	}
	if s.Default {
		start.Attr = append(start.Attr, xml.Attr{Name: ns.Qn("w:default"), Value: "1"})
	}
	if s.CustomStyle {
		start.Attr = append(start.Attr, xml.Attr{Name: ns.Qn("w:customStyle"), Value: "1"})
	}
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("wml: CT_Style start: %w", err)
	}
	if s.Name != "" {
		nameStart := xml.StartElement{Name: ns.Qn("w:name")}
		nameStart.Attr = []xml.Attr{{Name: ns.Qn("w:val"), Value: s.Name}}
		if err := e.EncodeToken(nameStart); err != nil {
			return err
		}
		if err := e.EncodeToken(xml.EndElement{Name: nameStart.Name}); err != nil {
			return err
		}
	}
	for _, child := range s.rawRest {
		if err := child.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	if s.PPr != nil {
		if err := s.PPr.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	if s.RPr != nil {
		if err := s.RPr.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML reads a w:style element.
func (s *CT_Style) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	s.StyleID = ""
	s.Type = ""
	s.Default = false
	s.CustomStyle = false
	s.Name = ""
	s.PPr = nil
	s.RPr = nil
	s.rawRest = nil
	for _, a := range start.Attr {
		switch {
		case a.Name.Local == "styleId":
			s.StyleID = a.Value
		case a.Name.Local == "type":
			s.Type = a.Value
		case a.Name.Local == "default" && (a.Value == "1" || a.Value == "true"):
			s.Default = true
		case a.Name.Local == "customStyle" && (a.Value == "1" || a.Value == "true"):
			s.CustomStyle = true
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("wml: CT_Style: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name {
			case ns.Qn("w:name"):
				s.Name = wAttr(t.Attr, "val")
				if err := d.Skip(); err != nil {
					return err
				}
			case ns.Qn("w:pPr"):
				s.PPr = new(CT_PPr)
				if err := s.PPr.UnmarshalXML(d, t); err != nil {
					return err
				}
			case ns.Qn("w:rPr"):
				s.RPr = new(CT_RPr)
				if err := s.RPr.UnmarshalXML(d, t); err != nil {
					return err
				}
			default:
				re := new(raw.RawElement)
				if err := re.UnmarshalXML(d, t); err != nil {
					return err
				}
				s.rawRest = append(s.rawRest, re)
			}
		case xml.EndElement:
			return nil
		}
	}
}

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------

func init() {
	registry.Register(ns.Qn("w:styles"), func() registry.Element { return new(CT_Styles) })
	registry.Register(ns.Qn("w:style"), func() registry.Element { return new(CT_Style) })
}
