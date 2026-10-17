package wml

import (
	"encoding/xml"
	"fmt"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
)

// CT_Settings is the root element of word/settings.xml (w:settings).
//
// A handful of commonly-needed properties are exposed via typed accessors;
// everything else passes through in the raw child list so round-trips are
// lossless.
type CT_Settings struct {
	// Attrs preserves the root-element namespace declarations (xmlns:*) so
	// nsfix can reconstruct them when marshalling, mirroring CT_Document.
	Attrs []xml.Attr
	raw   []*raw.RawElement
}

// EvenAndOddHeaders returns whether separate even/odd headers are enabled.
// nil → not set (Word treats absent as false).
func (s *CT_Settings) EvenAndOddHeaders() *bool {
	return rawGetOnOff(s.raw, ns.Qn("w:evenAndOddHeaders"))
}

// SetEvenAndOddHeaders enables or disables separate even/odd headers.
// nil removes the element (inherits Word's default = false).
func (s *CT_Settings) SetEvenAndOddHeaders(b *bool) {
	rawSetOnOff(&s.raw, ns.Qn("w:evenAndOddHeaders"), b)
}

// DefaultTabStop returns the default tab-stop width in twips, or nil if not set.
func (s *CT_Settings) DefaultTabStop() *int64 {
	return rawGetInt64Attr(s.raw, ns.Qn("w:defaultTabStop"), "val")
}

// SetDefaultTabStop sets the default tab-stop width in twips.
// nil removes the element.
func (s *CT_Settings) SetDefaultTabStop(v *int64) {
	rawSetInt64Attr(&s.raw, ns.Qn("w:defaultTabStop"), ns.Qn("w:val"), v)
}

// settingsAfterUpdateFields lists the w:settings children that follow
// w:updateFields in the CT_Settings xs:sequence. Everything else precedes it.
var settingsAfterUpdateFields = map[xml.Name]bool{
	ns.Qn("w:hdrShapeDefaults"):           true,
	ns.Qn("w:footnotePr"):                 true,
	ns.Qn("w:endnotePr"):                  true,
	ns.Qn("w:compat"):                     true,
	ns.Qn("w:docVars"):                    true,
	ns.Qn("w:rsids"):                      true,
	ns.Qn("w:mathPr"):                     true,
	ns.Qn("w:attachedSchema"):             true,
	ns.Qn("w:themeFontLang"):              true,
	ns.Qn("w:clrSchemeMapping"):           true,
	ns.Qn("w:doNotIncludeSubdocsInStats"): true,
	ns.Qn("w:doNotAutoCompressPictures"):  true,
	ns.Qn("w:forceUpgrade"):               true,
	ns.Qn("w:captions"):                   true,
	ns.Qn("w:readModeInkLockDown"):        true,
	ns.Qn("w:smartTagType"):               true,
	ns.Qn("w:schemaLibrary"):              true,
	ns.Qn("w:shapeDefaults"):              true,
	ns.Qn("w:doNotEmbedSmartTags"):        true,
	ns.Qn("w:decimalSymbol"):              true,
	ns.Qn("w:listSeparator"):              true,
}

// UpdateFields returns the w:updateFields tri-state. nil → not set.
func (s *CT_Settings) UpdateFields() *bool {
	return rawGetOnOff(s.raw, ns.Qn("w:updateFields"))
}

// SetUpdateFields sets w:updateFields, which asks Word to refresh all fields
// (TOC, page numbers, ...) when the document opens. nil removes the element.
//
// A new element is inserted at its xs:sequence position, before w:compat,
// w:rsids and the other children that follow it.
func (s *CT_Settings) SetUpdateFields(b *bool) {
	name := ns.Qn("w:updateFields")
	rawRemoveChild(&s.raw, name)
	if b == nil {
		return
	}
	re := new(raw.RawElement)
	re.QName = name
	if !*b {
		re.Attrs = []xml.Attr{{Name: ns.Qn("w:val"), Value: "false"}}
	}
	for i, child := range s.raw {
		if settingsAfterUpdateFields[child.QName] {
			s.raw = append(s.raw, nil)
			copy(s.raw[i+1:], s.raw[i:])
			s.raw[i] = re
			return
		}
	}
	s.raw = append(s.raw, re)
}

// MarshalXML encodes w:settings.
func (s *CT_Settings) MarshalXML(e *xml.Encoder, _ xml.StartElement) error {
	start := xml.StartElement{Name: ns.Qn("w:settings"), Attr: s.Attrs}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if err := rawMarshalChildren(e, s.raw); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: ns.Qn("w:settings")})
}

// UnmarshalXML reads a w:settings element.
func (s *CT_Settings) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	s.Attrs = s.Attrs[:0]
	s.raw = s.raw[:0]
	for _, a := range start.Attr {
		// Drop the bare default-namespace declaration — the encoder re-emits
		// it automatically from Name.Space, so storing it produces a duplicate.
		if a.Name.Space == "" && a.Name.Local == "xmlns" {
			continue
		}
		s.Attrs = append(s.Attrs, a)
	}
	children, err := rawUnmarshalChildren(d)
	if err != nil {
		return fmt.Errorf("CT_Settings: %w", err)
	}
	s.raw = children
	return nil
}
