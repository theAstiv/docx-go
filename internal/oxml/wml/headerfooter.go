package wml

import (
	"encoding/xml"
	"fmt"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
	"github.com/theAstiv/docx-go/internal/oxml/registry"
)

// ── CT_Hdr ────────────────────────────────────────────────────────────────────

// CT_Hdr is the root element of a header part (w:hdr).
// It holds block-level content identical to the document body but without a
// trailing sectPr.
type CT_Hdr struct {
	Children []BlockElement
}

// MarshalXML encodes the header root element.
func (h *CT_Hdr) MarshalXML(e *xml.Encoder, _ xml.StartElement) error {
	start := xml.StartElement{Name: ns.Qn("w:hdr")}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for _, child := range h.Children {
		if err := child.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: ns.Qn("w:hdr")})
}

// UnmarshalXML reads a w:hdr element.
func (h *CT_Hdr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	h.Children = h.Children[:0]
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("CT_Hdr: %w", err)
		}
		switch t := tok.(type) {
		case xml.EndElement:
			return nil
		case xml.StartElement:
			child, err := unmarshalBlockElement(d, t)
			if err != nil {
				return err
			}
			h.Children = append(h.Children, child)
		}
	}
}

// ── CT_Ftr ────────────────────────────────────────────────────────────────────

// CT_Ftr is the root element of a footer part (w:ftr).
type CT_Ftr struct {
	Children []BlockElement
}

// MarshalXML encodes the footer root element.
func (f *CT_Ftr) MarshalXML(e *xml.Encoder, _ xml.StartElement) error {
	start := xml.StartElement{Name: ns.Qn("w:ftr")}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for _, child := range f.Children {
		if err := child.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: ns.Qn("w:ftr")})
}

// UnmarshalXML reads a w:ftr element.
func (f *CT_Ftr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	f.Children = f.Children[:0]
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("CT_Ftr: %w", err)
		}
		switch t := tok.(type) {
		case xml.EndElement:
			return nil
		case xml.StartElement:
			child, err := unmarshalBlockElement(d, t)
			if err != nil {
				return err
			}
			f.Children = append(f.Children, child)
		}
	}
}

// unmarshalBlockElement dispatches a StartElement to the correct BlockElement type.
// It is used by both CT_Hdr and CT_Ftr (and mirrors the logic in CT_Body.UnmarshalXML).
func unmarshalBlockElement(d *xml.Decoder, t xml.StartElement) (BlockElement, error) {
	switch t.Name {
	case ns.Qn("w:p"):
		p := new(CT_P)
		if err := p.UnmarshalXML(d, t); err != nil {
			return nil, err
		}
		return p, nil
	case ns.Qn("w:tbl"):
		tbl := new(CT_Tbl)
		if err := tbl.UnmarshalXML(d, t); err != nil {
			return nil, err
		}
		return tbl, nil
	default:
		r := new(raw.RawElement) //nolint:exhaustruct // zero-value decode target; UnmarshalXML populates all fields
		if err := r.UnmarshalXML(d, t); err != nil {
			return nil, err
		}
		return &RawBlock{E: r}, nil
	}
}

// ── CT_HdrFtrRef ──────────────────────────────────────────────────────────────

// HdrFtrType is the w:type attribute value for a header/footer reference.
type HdrFtrType string

const (
	// HdrFtrDefault is the default (continuous) header/footer type.
	HdrFtrDefault HdrFtrType = "default"
	// HdrFtrFirst is the first-page header/footer type.
	HdrFtrFirst HdrFtrType = "first"
	// HdrFtrEven is the even-page header/footer type.
	HdrFtrEven HdrFtrType = "even"
)

// CT_HdrFtrRef represents a w:headerReference or w:footerReference element.
type CT_HdrFtrRef struct {
	Type  HdrFtrType
	RelID string // r:id
}

// ── CT_SectPr typed accessors ─────────────────────────────────────────────────

// PgSzW returns the page width in twips from w:pgSz @w:w, or nil.
func (s *CT_SectPr) PgSzW() *int64 {
	return rawGetInt64Attr(s.raw, ns.Qn("w:pgSz"), "w")
}

// PgSzH returns the page height in twips from w:pgSz @w:h, or nil.
func (s *CT_SectPr) PgSzH() *int64 {
	return rawGetInt64Attr(s.raw, ns.Qn("w:pgSz"), "h")
}

// SetPgSz sets the w:pgSz element with the given width and height in twips.
func (s *CT_SectPr) SetPgSz(w, h int64) {
	rawSetInt64Attr(&s.raw, ns.Qn("w:pgSz"), ns.Qn("w:w"), &w)
	rawSetInt64Attr(&s.raw, ns.Qn("w:pgSz"), ns.Qn("w:h"), &h)
}

// PgMarTop returns the top margin in twips, or nil.
func (s *CT_SectPr) PgMarTop() *int64 {
	return rawGetInt64Attr(s.raw, ns.Qn("w:pgMar"), "top")
}

// PgMarBottom returns the bottom margin in twips, or nil.
func (s *CT_SectPr) PgMarBottom() *int64 {
	return rawGetInt64Attr(s.raw, ns.Qn("w:pgMar"), "bottom")
}

// PgMarLeft returns the left margin in twips, or nil.
func (s *CT_SectPr) PgMarLeft() *int64 {
	return rawGetInt64Attr(s.raw, ns.Qn("w:pgMar"), "left")
}

// PgMarRight returns the right margin in twips, or nil.
func (s *CT_SectPr) PgMarRight() *int64 {
	return rawGetInt64Attr(s.raw, ns.Qn("w:pgMar"), "right")
}

// SetPgMargins sets all margins on w:pgMar in twips.
func (s *CT_SectPr) SetPgMargins(top, right, bottom, left int64) {
	rawSetInt64Attr(&s.raw, ns.Qn("w:pgMar"), ns.Qn("w:top"), &top)
	rawSetInt64Attr(&s.raw, ns.Qn("w:pgMar"), ns.Qn("w:right"), &right)
	rawSetInt64Attr(&s.raw, ns.Qn("w:pgMar"), ns.Qn("w:bottom"), &bottom)
	rawSetInt64Attr(&s.raw, ns.Qn("w:pgMar"), ns.Qn("w:left"), &left)
}

// TitlePg returns true if the first-page header/footer is enabled (w:titlePg).
func (s *CT_SectPr) TitlePg() bool {
	b := rawGetOnOff(s.raw, ns.Qn("w:titlePg"))
	return b != nil && *b
}

// SetTitlePg enables or disables the first-page header/footer (w:titlePg).
func (s *CT_SectPr) SetTitlePg(enable bool) {
	if enable {
		t := true
		rawSetOnOff(&s.raw, ns.Qn("w:titlePg"), &t)
	} else {
		rawRemoveChild(&s.raw, ns.Qn("w:titlePg"))
	}
}

// HeaderRefs returns all w:headerReference elements in this sectPr.
func (s *CT_SectPr) HeaderRefs() []CT_HdrFtrRef {
	return hdrFtrRefs(s.raw, ns.Qn("w:headerReference"))
}

// FooterRefs returns all w:footerReference elements in this sectPr.
func (s *CT_SectPr) FooterRefs() []CT_HdrFtrRef {
	return hdrFtrRefs(s.raw, ns.Qn("w:footerReference"))
}

// AddHeaderRef appends a w:headerReference element.
func (s *CT_SectPr) AddHeaderRef(typ HdrFtrType, relID string) {
	s.raw = append(s.raw, buildHdrFtrRef(ns.Qn("w:headerReference"), typ, relID))
}

// AddFooterRef appends a w:footerReference element.
func (s *CT_SectPr) AddFooterRef(typ HdrFtrType, relID string) {
	s.raw = append(s.raw, buildHdrFtrRef(ns.Qn("w:footerReference"), typ, relID))
}

// RemoveHeaderRef removes any existing w:headerReference with the given type.
func (s *CT_SectPr) RemoveHeaderRef(typ HdrFtrType) {
	removeHdrFtrRef(&s.raw, ns.Qn("w:headerReference"), typ)
}

// RemoveFooterRef removes any existing w:footerReference with the given type.
func (s *CT_SectPr) RemoveFooterRef(typ HdrFtrType) {
	removeHdrFtrRef(&s.raw, ns.Qn("w:footerReference"), typ)
}

// hdrFtrRefs extracts all header/footer references of the given element QName.
func hdrFtrRefs(children []*raw.RawElement, qname xml.Name) []CT_HdrFtrRef {
	var out []CT_HdrFtrRef
	for _, r := range children {
		if r.QName != qname {
			continue
		}
		ref := CT_HdrFtrRef{ //nolint:exhaustruct // Type and RelID populated below; missing fields are zero
			Type:  HdrFtrDefault,
			RelID: "",
		}
		for _, a := range r.Attrs {
			switch a.Name.Local {
			case "type":
				ref.Type = HdrFtrType(a.Value)
			case "id":
				ref.RelID = a.Value
			}
		}
		out = append(out, ref)
	}
	return out
}

// buildHdrFtrRef creates a raw element for a header or footer reference.
func buildHdrFtrRef(qname xml.Name, typ HdrFtrType, relID string) *raw.RawElement {
	el := new(raw.RawElement) //nolint:exhaustruct // populated immediately below
	el.QName = qname
	el.Attrs = []xml.Attr{
		{Name: ns.Qn("w:type"), Value: string(typ)},
		{Name: xml.Name{Space: "http://schemas.openxmlformats.org/officeDocument/2006/relationships", Local: "id"}, Value: relID},
	}
	return el
}

// removeHdrFtrRef removes the first reference of the given type from the raw slice.
func removeHdrFtrRef(children *[]*raw.RawElement, qname xml.Name, typ HdrFtrType) {
	for i, r := range *children {
		if r.QName != qname {
			continue
		}
		for _, a := range r.Attrs {
			if a.Name.Local == "type" && HdrFtrType(a.Value) == typ {
				*children = append((*children)[:i], (*children)[i+1:]...)
				return
			}
		}
	}
}

func init() {
	registry.Register(ns.Qn("w:hdr"), func() registry.Element { return new(CT_Hdr) })
	registry.Register(ns.Qn("w:ftr"), func() registry.Element { return new(CT_Ftr) })
}
