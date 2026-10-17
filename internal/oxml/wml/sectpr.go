package wml

import (
	"encoding/xml"
	"fmt"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
	"github.com/theAstiv/docx-go/internal/oxml/registry"
)

// CT_SectPr is the w:sectPr element: section properties.
//
// All children are preserved in raw for order-safe round-tripping.
// Phase 2 will add typed accessors for page size, margins, and
// header/footer references.
type CT_SectPr struct {
	// Attributes on the sectPr element itself (e.g. rsidR, rsidSect).
	Attrs []xml.Attr
	// raw holds all child elements in document order.
	raw []*raw.RawElement
}

// MarshalXML emits <w:sectPr> with all captured children.
func (s *CT_SectPr) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:sectPr")
	start.Attr = append(start.Attr, s.Attrs...)
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("wml: CT_SectPr start: %w", err)
	}
	if err := rawMarshalChildren(e, orderBySeq(s.raw, sectPrChildSeqPos)); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML reads a w:sectPr element.
func (s *CT_SectPr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	s.Attrs = make([]xml.Attr, 0, len(start.Attr))
	for _, a := range start.Attr {
		if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
			continue
		}
		s.Attrs = append(s.Attrs, a)
	}
	var err error
	s.raw, err = rawUnmarshalChildren(d)
	return err
}

// DefaultSectPr returns a CT_SectPr pre-populated with US Letter page size
// (8.5" × 11") and standard 1" margins (1.25" left to match Word's default).
// Without explicit pgSz/pgMar Word applies locale-dependent defaults, which
// produces different page geometries on different machines.
func DefaultSectPr() *CT_SectPr {
	s := new(CT_SectPr)
	pgSz := new(raw.RawElement)
	pgSz.QName = ns.Qn("w:pgSz")
	pgSz.Attrs = []xml.Attr{
		{Name: ns.Qn("w:w"), Value: "12240"}, // 8.5" × 1440 twips/in
		{Name: ns.Qn("w:h"), Value: "15840"}, // 11"  × 1440 twips/in
	}
	pgMar := new(raw.RawElement)
	pgMar.QName = ns.Qn("w:pgMar")
	pgMar.Attrs = []xml.Attr{
		{Name: ns.Qn("w:top"), Value: "1440"},
		{Name: ns.Qn("w:right"), Value: "1440"},
		{Name: ns.Qn("w:bottom"), Value: "1440"},
		{Name: ns.Qn("w:left"), Value: "1800"}, // 1.25" — Word's default left margin
		{Name: ns.Qn("w:header"), Value: "720"},
		{Name: ns.Qn("w:footer"), Value: "720"},
		{Name: ns.Qn("w:gutter"), Value: "0"},
	}
	s.raw = []*raw.RawElement{pgSz, pgMar}
	return s
}

func init() {
	registry.Register(ns.Qn("w:sectPr"), func() registry.Element { return new(CT_SectPr) })
}
