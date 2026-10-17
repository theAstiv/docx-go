package dml

import (
	"encoding/xml"
	"strconv"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/registry"
)

// CT_PositiveSize2D is the wp:extent element: the size of a drawing in EMU.
type CT_PositiveSize2D struct {
	Cx, Cy int64
	// ExtraAttrs preserves attributes other than cx and cy.
	ExtraAttrs []xml.Attr
}

// MarshalXML emits <wp:extent>.
func (s *CT_PositiveSize2D) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("wp:extent")
	start.Attr = append(start.Attr,
		xml.Attr{Name: xml.Name{Local: "cx"}, Value: strconv.FormatInt(s.Cx, 10)},
		xml.Attr{Name: xml.Name{Local: "cy"}, Value: strconv.FormatInt(s.Cy, 10)},
	)
	start.Attr = append(start.Attr, s.ExtraAttrs...)
	return emptyElement(e, start)
}

// UnmarshalXML reads a wp:extent element.
func (s *CT_PositiveSize2D) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	s.Cx, s.Cy, s.ExtraAttrs = 0, 0, nil
	for _, a := range start.Attr {
		switch {
		case isNSDecl(a):
		case a.Name.Space == "" && a.Name.Local == "cx":
			s.Cx = attrInt64(a.Value)
		case a.Name.Space == "" && a.Name.Local == "cy":
			s.Cy = attrInt64(a.Value)
		default:
			s.ExtraAttrs = append(s.ExtraAttrs, a)
		}
	}
	return d.Skip()
}

func init() {
	registry.Register(ns.Qn("wp:extent"), func() registry.Element { return new(CT_PositiveSize2D) })
}
