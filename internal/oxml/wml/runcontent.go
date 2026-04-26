package wml

import (
	"encoding/xml"
	"fmt"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
	"github.com/theAstiv/docx-go/internal/oxml/registry"
)

// RunContentElement is the marker interface for elements that can appear as
// children of CT_R (the schema group EG_RunInnerContent).
type RunContentElement interface {
	MarshalXML(e *xml.Encoder, start xml.StartElement) error
	UnmarshalXML(d *xml.Decoder, start xml.StartElement) error
	isRunContent()
}

// RawRunContent captures unrecognised run children for pass-through.
type RawRunContent struct {
	E *raw.RawElement
}

func (*RawRunContent) isRunContent() {}

func (rc *RawRunContent) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if rc.E == nil {
		return nil
	}
	return rc.E.MarshalXML(e, start)
}

func (rc *RawRunContent) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	rc.E = new(raw.RawElement)
	return rc.E.UnmarshalXML(d, start)
}

// ---------------------------------------------------------------------------
// CT_Text — w:t
// ---------------------------------------------------------------------------

// CT_Text is the w:t element: a run of text.
type CT_Text struct {
	// Space is the xml:space attribute value ("preserve" or "").
	Space   string
	Content string
}

func (*CT_Text) isRunContent() {}

func (t *CT_Text) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:t")
	if t.Space != "" {
		start.Attr = append(start.Attr, xml.Attr{
			Name:  xml.Name{Space: ns.XML, Local: "space"},
			Value: t.Space,
		})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if err := e.EncodeToken(xml.CharData(t.Content)); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func (t *CT_Text) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	t.Space = ""
	t.Content = ""
	for _, a := range start.Attr {
		if a.Name.Local == "space" {
			t.Space = a.Value
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("wml: CT_Text: %w", err)
		}
		switch v := tok.(type) {
		case xml.CharData:
			t.Content += string(v)
		case xml.EndElement:
			return nil
		case xml.StartElement:
			if err := d.Skip(); err != nil {
				return fmt.Errorf("wml: CT_Text: %w", err)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// CT_Tab — w:tab
// ---------------------------------------------------------------------------

// CT_Tab is the w:tab element: a tab character in a run.
type CT_Tab struct{}

func (*CT_Tab) isRunContent() {}

func (t *CT_Tab) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:tab")
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func (t *CT_Tab) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return d.Skip()
}

// ---------------------------------------------------------------------------
// CT_Br — w:br
// ---------------------------------------------------------------------------

// CT_Br is the w:br element: a break.
// Type values: "page", "column", "textWrapping" (default).
// Clear values: "none", "left", "right", "all" (only for textWrapping).
type CT_Br struct {
	Type  string
	Clear string
}

func (*CT_Br) isRunContent() {}

func (b *CT_Br) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:br")
	if b.Type != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: ns.Qn("w:type"), Value: b.Type})
	}
	if b.Clear != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: ns.Qn("w:clear"), Value: b.Clear})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func (b *CT_Br) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	b.Type = wAttr(start.Attr, "type")
	b.Clear = wAttr(start.Attr, "clear")
	return d.Skip()
}

// ---------------------------------------------------------------------------
// CT_Cr — w:cr
// ---------------------------------------------------------------------------

// CT_Cr is the w:cr element: a carriage return (line break within a paragraph).
type CT_Cr struct{}

func (*CT_Cr) isRunContent() {}

func (c *CT_Cr) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:cr")
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func (c *CT_Cr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return d.Skip()
}

// ---------------------------------------------------------------------------
// CT_NoBreakHyphen — w:noBreakHyphen
// ---------------------------------------------------------------------------

// CT_NoBreakHyphen is the w:noBreakHyphen element: a non-breaking hyphen.
type CT_NoBreakHyphen struct{}

func (*CT_NoBreakHyphen) isRunContent() {}

func (h *CT_NoBreakHyphen) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:noBreakHyphen")
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func (h *CT_NoBreakHyphen) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return d.Skip()
}

// ---------------------------------------------------------------------------
// CT_SoftHyphen — w:softHyphen
// ---------------------------------------------------------------------------

// CT_SoftHyphen is the w:softHyphen element: a soft (optional) hyphen.
type CT_SoftHyphen struct{}

func (*CT_SoftHyphen) isRunContent() {}

func (h *CT_SoftHyphen) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:softHyphen")
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func (h *CT_SoftHyphen) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return d.Skip()
}

// ---------------------------------------------------------------------------
// CT_Sym — w:sym
// ---------------------------------------------------------------------------

// CT_Sym is the w:sym element: a symbol character.
type CT_Sym struct {
	Font string
	Char string // hex character code, e.g. "F0B7"
}

func (*CT_Sym) isRunContent() {}

func (s *CT_Sym) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:sym")
	if s.Font != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: ns.Qn("w:font"), Value: s.Font})
	}
	if s.Char != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: ns.Qn("w:char"), Value: s.Char})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func (s *CT_Sym) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	s.Font = wAttr(start.Attr, "font")
	s.Char = wAttr(start.Attr, "char")
	return d.Skip()
}

// ---------------------------------------------------------------------------
// CT_FldChar — w:fldChar
// ---------------------------------------------------------------------------

// CT_FldChar is the w:fldChar element: a field character (begin/separate/end).
// rawRest captures child elements (w:ffData, w:fldData, w:numberingChange, etc.)
// so that form-field data and other children survive round-trips.
type CT_FldChar struct {
	// FldCharType: "begin", "separate", or "end".
	FldCharType string
	rawRest     []*raw.RawElement
}

func (*CT_FldChar) isRunContent() {}

func (f *CT_FldChar) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:fldChar")
	if f.FldCharType != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: ns.Qn("w:fldCharType"), Value: f.FldCharType})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if err := rawMarshalChildren(e, f.rawRest); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func (f *CT_FldChar) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	f.FldCharType = wAttr(start.Attr, "fldCharType")
	var err error
	f.rawRest, err = rawUnmarshalChildren(d)
	return err
}

// ---------------------------------------------------------------------------
// CT_InstrText — w:instrText
// ---------------------------------------------------------------------------

// CT_InstrText is the w:instrText element: field instruction text.
type CT_InstrText struct {
	Space   string
	Content string
}

func (*CT_InstrText) isRunContent() {}

func (t *CT_InstrText) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:instrText")
	if t.Space != "" {
		start.Attr = append(start.Attr, xml.Attr{
			Name:  xml.Name{Space: ns.XML, Local: "space"},
			Value: t.Space,
		})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if err := e.EncodeToken(xml.CharData(t.Content)); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func (t *CT_InstrText) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	t.Space = ""
	t.Content = ""
	for _, a := range start.Attr {
		if a.Name.Local == "space" {
			t.Space = a.Value
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("wml: CT_InstrText: %w", err)
		}
		switch v := tok.(type) {
		case xml.CharData:
			t.Content += string(v)
		case xml.EndElement:
			return nil
		case xml.StartElement:
			if err := d.Skip(); err != nil {
				return fmt.Errorf("wml: CT_InstrText: %w", err)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Registry — run content types registered for the roundtrip audit
// ---------------------------------------------------------------------------

func init() {
	registry.Register(ns.Qn("w:t"), func() registry.Element { return new(CT_Text) })
	registry.Register(ns.Qn("w:tab"), func() registry.Element { return new(CT_Tab) })
	registry.Register(ns.Qn("w:br"), func() registry.Element { return new(CT_Br) })
	registry.Register(ns.Qn("w:cr"), func() registry.Element { return new(CT_Cr) })
	registry.Register(ns.Qn("w:noBreakHyphen"), func() registry.Element { return new(CT_NoBreakHyphen) })
	registry.Register(ns.Qn("w:softHyphen"), func() registry.Element { return new(CT_SoftHyphen) })
	registry.Register(ns.Qn("w:sym"), func() registry.Element { return new(CT_Sym) })
	registry.Register(ns.Qn("w:fldChar"), func() registry.Element { return new(CT_FldChar) })
	registry.Register(ns.Qn("w:instrText"), func() registry.Element { return new(CT_InstrText) })
}
