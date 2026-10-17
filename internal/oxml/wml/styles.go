package wml

import (
	"encoding/xml"
	"fmt"
	"strconv"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
	"github.com/theAstiv/docx-go/internal/oxml/registry"
)

// CT_Styles is the w:styles element: the root of styles.xml.
//
// Attrs captures element attributes for round-trip fidelity.
// w:docDefaults is decoded into DocDefaults and named styles into Styles;
// latentStyles and unknown children are preserved as raw children in rawRest.
//
// Ordering note: MarshalXML always emits rawRest before Styles. This matches
// the canonical Word ordering (docDefaults → latentStyles → style…). Documents
// with interleaved ordering will have their order normalised on round-trip.
// Phase 2 can improve this by recording each child's insertion index.
type CT_Styles struct {
	Attrs []xml.Attr
	// DocDefaults is the typed w:docDefaults child; nil when absent.
	DocDefaults *CT_DocDefaults
	Styles      []*CT_Style
	rawRest     []*raw.RawElement // latentStyles and unknown children
}

// MarshalXML emits <w:styles>.
func (s *CT_Styles) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:styles")
	start.Attr = append(start.Attr, s.Attrs...)
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("wml: CT_Styles start: %w", err)
	}
	if s.DocDefaults != nil {
		if err := s.DocDefaults.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	// Emit raw non-style children next (latentStyles, etc.)
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
	s.DocDefaults = nil
	s.rawRest = nil
	for _, a := range start.Attr {
		if a.Name.Space == "" && a.Name.Local == "xmlns" {
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
			} else if t.Name == ns.Qn("w:docDefaults") && s.DocDefaults == nil {
				dd := new(CT_DocDefaults)
				if err := dd.UnmarshalXML(d, t); err != nil {
					return err
				}
				s.DocDefaults = dd
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
// the paragraph and run properties for the style. Children are split into
// rawRestPre (before w:pPr in the xs:sequence) and rawRestPost (after w:rPr)
// so that MarshalXML can emit them in schema-correct order.
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
	// rawRestPre preserves children that appear before w:pPr in the xs:sequence
	// (w:aliases, w:basedOn, w:next, w:link, w:uiPriority, w:rsid, etc.).
	rawRestPre []*raw.RawElement
	// rawRestPost preserves children that appear after w:rPr in the xs:sequence
	// (w:tblPr, w:trPr, w:tcPr, and unknown future elements).
	rawRestPost []*raw.RawElement
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
	// xs:sequence: rawRestPre (aliases, basedOn, next, link, uiPriority, rsid, …)
	// then pPr (17), rPr (18), then rawRestPost (tblPr 19, trPr 20, tcPr 21, …).
	for _, child := range s.rawRestPre {
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
	for _, child := range s.rawRestPost {
		if err := child.MarshalXML(e, xml.StartElement{}); err != nil {
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
	s.rawRestPre = nil
	s.rawRestPost = nil
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
	// pprSeen tracks whether we have already consumed a w:pPr or w:rPr child.
	// Children seen before pPr go to rawRestPre; those after rPr go to rawRestPost.
	pprSeen := false
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
				pprSeen = true
				s.PPr = new(CT_PPr)
				if err := s.PPr.UnmarshalXML(d, t); err != nil {
					return err
				}
			case ns.Qn("w:rPr"):
				pprSeen = true
				s.RPr = new(CT_RPr)
				if err := s.RPr.UnmarshalXML(d, t); err != nil {
					return err
				}
			default:
				re := new(raw.RawElement)
				if err := re.UnmarshalXML(d, t); err != nil {
					return err
				}
				if pprSeen {
					s.rawRestPost = append(s.rawRestPost, re)
				} else {
					s.rawRestPre = append(s.rawRestPre, re)
				}
			}
		case xml.EndElement:
			return nil
		}
	}
}

// ---------------------------------------------------------------------------
// Typed accessors for the children that precede w:pPr
// ---------------------------------------------------------------------------

// styleChildSeqPos maps w:style child names that precede w:pPr to their
// xs:sequence position. Unknown names sort to the tail of rawRestPre.
var styleChildSeqPos = map[xml.Name]int{
	ns.Qn("w:aliases"):        1,
	ns.Qn("w:basedOn"):        2,
	ns.Qn("w:next"):           3,
	ns.Qn("w:link"):           4,
	ns.Qn("w:autoRedefine"):   5,
	ns.Qn("w:hidden"):         6,
	ns.Qn("w:uiPriority"):     7,
	ns.Qn("w:semiHidden"):     8,
	ns.Qn("w:unhideWhenUsed"): 9,
	ns.Qn("w:qFormat"):        10,
	ns.Qn("w:locked"):         11,
}

// setPreStr sets (or, for "", removes) a single-w:val child of rawRestPre,
// inserting new elements at their schema position.
func (s *CT_Style) setPreStr(qname xml.Name, value string) {
	rawRemoveChild(&s.rawRestPre, qname)
	if value == "" {
		return
	}
	re := new(raw.RawElement)
	re.QName = qname
	re.Attrs = []xml.Attr{{Name: ns.Qn("w:val"), Value: value}}
	rawInsertOrdered(&s.rawRestPre, re, styleChildSeqPos)
}

// setPreOnOff sets (or, for nil, removes) an OnOff child of rawRestPre.
func (s *CT_Style) setPreOnOff(qname xml.Name, b *bool) {
	rawRemoveChild(&s.rawRestPre, qname)
	if b == nil {
		return
	}
	re := new(raw.RawElement)
	re.QName = qname
	if !*b {
		re.Attrs = []xml.Attr{{Name: ns.Qn("w:val"), Value: "false"}}
	}
	rawInsertOrdered(&s.rawRestPre, re, styleChildSeqPos)
}

// BasedOn returns w:basedOn/@w:val, or "".
func (s *CT_Style) BasedOn() string { return rawGetStr(s.rawRestPre, ns.Qn("w:basedOn")) }

// SetBasedOn sets w:basedOn. An empty string removes it.
func (s *CT_Style) SetBasedOn(id string) { s.setPreStr(ns.Qn("w:basedOn"), id) }

// Next returns w:next/@w:val, or "".
func (s *CT_Style) Next() string { return rawGetStr(s.rawRestPre, ns.Qn("w:next")) }

// SetNext sets w:next. An empty string removes it.
func (s *CT_Style) SetNext(id string) { s.setPreStr(ns.Qn("w:next"), id) }

// Link returns w:link/@w:val, or "".
func (s *CT_Style) Link() string { return rawGetStr(s.rawRestPre, ns.Qn("w:link")) }

// SetLink sets w:link. An empty string removes it.
func (s *CT_Style) SetLink(id string) { s.setPreStr(ns.Qn("w:link"), id) }

// UIPriority returns w:uiPriority/@w:val, or nil when absent.
func (s *CT_Style) UIPriority() *int64 {
	return rawGetInt64Attr(s.rawRestPre, ns.Qn("w:uiPriority"), "val")
}

// SetUIPriority sets w:uiPriority. nil removes it.
func (s *CT_Style) SetUIPriority(v *int64) {
	rawRemoveChild(&s.rawRestPre, ns.Qn("w:uiPriority"))
	if v == nil {
		return
	}
	s.setPreStr(ns.Qn("w:uiPriority"), strconv.FormatInt(*v, 10))
}

// SemiHidden returns the w:semiHidden tri-state.
func (s *CT_Style) SemiHidden() *bool { return rawGetOnOff(s.rawRestPre, ns.Qn("w:semiHidden")) }

// SetSemiHidden sets w:semiHidden. nil removes it.
func (s *CT_Style) SetSemiHidden(b *bool) { s.setPreOnOff(ns.Qn("w:semiHidden"), b) }

// UnhideWhenUsed returns the w:unhideWhenUsed tri-state.
func (s *CT_Style) UnhideWhenUsed() *bool {
	return rawGetOnOff(s.rawRestPre, ns.Qn("w:unhideWhenUsed"))
}

// SetUnhideWhenUsed sets w:unhideWhenUsed. nil removes it.
func (s *CT_Style) SetUnhideWhenUsed(b *bool) { s.setPreOnOff(ns.Qn("w:unhideWhenUsed"), b) }

// QFormat returns the w:qFormat tri-state.
func (s *CT_Style) QFormat() *bool { return rawGetOnOff(s.rawRestPre, ns.Qn("w:qFormat")) }

// SetQFormat sets w:qFormat. nil removes it.
func (s *CT_Style) SetQFormat(b *bool) { s.setPreOnOff(ns.Qn("w:qFormat"), b) }

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------

func init() {
	registry.Register(ns.Qn("w:styles"), func() registry.Element { return new(CT_Styles) })
	registry.Register(ns.Qn("w:style"), func() registry.Element { return new(CT_Style) })
}
