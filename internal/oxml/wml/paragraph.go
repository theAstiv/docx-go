package wml

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/registry"
)

// CT_P is the w:p element: a paragraph.
//
// PPr holds the optional paragraph properties.
// Children holds the paragraph's inline content (runs, hyperlinks,
// bookmarks, etc.) in document order.
type CT_P struct {
	PPr      *CT_PPr
	Children []InlineElement
	// Attrs holds non-xmlns attributes declared on the w:p element itself
	// (e.g. w:rsidR, w14:paraId) for round-trip fidelity.
	Attrs []xml.Attr
}

func (*CT_P) isBlock() {}

// MarshalXML emits <w:p>.
func (p *CT_P) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:p")
	start.Attr = append(start.Attr, p.Attrs...)
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("wml: CT_P start: %w", err)
	}
	if p.PPr != nil {
		if err := p.PPr.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	for _, child := range p.Children {
		if err := child.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML reads a w:p element.
func (p *CT_P) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	p.PPr = nil
	p.Children = p.Children[:0]
	p.Attrs = elementAttrs(start)
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("wml: CT_P: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name {
			case ns.Qn("w:pPr"):
				p.PPr = new(CT_PPr)
				if err := p.PPr.UnmarshalXML(d, t); err != nil {
					return err
				}
			case ns.Qn("w:r"):
				r := new(CT_R)
				if err := r.UnmarshalXML(d, t); err != nil {
					return err
				}
				p.Children = append(p.Children, r)
			case ns.Qn("w:hyperlink"):
				hl := new(CT_Hyperlink)
				if err := hl.UnmarshalXML(d, t); err != nil {
					return err
				}
				p.Children = append(p.Children, hl)
			case ns.Qn("w:bookmarkStart"):
				bs := new(CT_BookmarkStart)
				if err := bs.UnmarshalXML(d, t); err != nil {
					return err
				}
				p.Children = append(p.Children, bs)
			case ns.Qn("w:bookmarkEnd"):
				be := new(CT_BookmarkEnd)
				if err := be.UnmarshalXML(d, t); err != nil {
					return err
				}
				p.Children = append(p.Children, be)
			default:
				ri := new(RawInline)
				if err := ri.UnmarshalXML(d, t); err != nil {
					return err
				}
				p.Children = append(p.Children, ri)
			}
		case xml.EndElement:
			return nil
		}
	}
}

// Text returns the visible text of the paragraph by walking all w:t
// descendants in document order, following the rendering rules in CLAUDE.md.
//
// Tabs render as \t, breaks and carriage-returns as \n.
// Field instructions (w:instrText) are excluded; field results are included.
// w:del runs (tracked deletions) are excluded; w:ins runs are included.
// Unknown elements (RawInline) contribute nothing.
func (p *CT_P) Text() string {
	var sb strings.Builder
	textFromInlines(p.Children, &sb, false)
	return sb.String()
}

// textFromInlines recursively collects visible text from a run of InlineElements.
// inField tracks whether we're between w:fldChar begin and separate (instruction phase).
func textFromInlines(children []InlineElement, sb *strings.Builder, inField bool) bool {
	for _, child := range children {
		switch c := child.(type) {
		case *CT_R:
			inField = textFromRunContent(c.Children, sb, inField)
		case *CT_Hyperlink:
			inField = textFromInlines(c.Children, sb, inField)
		case *CT_BookmarkStart, *CT_BookmarkEnd, *RawInline:
			// No visible text.
		}
	}
	return inField
}

// textFromRunContent collects text from run inner content.
func textFromRunContent(children []RunContentElement, sb *strings.Builder, inField bool) bool {
	for _, child := range children {
		switch c := child.(type) {
		case *CT_Text:
			if !inField {
				sb.WriteString(c.Content)
			}
		case *CT_Tab:
			if !inField {
				sb.WriteByte('\t')
			}
		case *CT_Br, *CT_Cr:
			if !inField {
				sb.WriteByte('\n')
			}
		case *CT_FldChar:
			switch c.FldCharType {
			case "begin":
				inField = true
			case "separate":
				// Switch from instruction phase to result phase — stop suppressing text.
				inField = false
			case "end":
				// Field result phase is done.
			}
		case *CT_InstrText:
			// Instruction text — always suppressed.
		case *CT_NoBreakHyphen:
			if !inField {
				sb.WriteRune('‑') // U+2011 NON-BREAKING HYPHEN
			}
		case *CT_SoftHyphen:
			if !inField {
				sb.WriteRune('­') // U+00AD SOFT HYPHEN
			}
		case *CT_Sym:
			// Symbol glyphs require font+char resolution; contribute nothing for now.
		case *RawRunContent:
			// Unknown content — contribute nothing.
		}
	}
	return inField
}

// ---------------------------------------------------------------------------
// CT_Hyperlink — w:hyperlink
// ---------------------------------------------------------------------------

// CT_Hyperlink is the w:hyperlink element: a hyperlink container.
// RID names a hyperlink relationship of the part that holds the element;
// resolving it is the parts layer's job.
type CT_Hyperlink struct {
	// RID is the r:id relationship target for external links.
	RID string
	// Anchor is the w:anchor target for internal bookmark links.
	Anchor string
	// ExtraAttrs captures other attributes for round-trip fidelity.
	ExtraAttrs []xml.Attr
	// Children are the runs and other inline elements inside the hyperlink.
	Children []InlineElement
}

func (*CT_Hyperlink) isInline() {}

func (h *CT_Hyperlink) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:hyperlink")
	if h.RID != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: ns.Qn("r:id"), Value: h.RID})
	}
	if h.Anchor != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: ns.Qn("w:anchor"), Value: h.Anchor})
	}
	start.Attr = append(start.Attr, h.ExtraAttrs...)
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for _, child := range h.Children {
		if err := child.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func (h *CT_Hyperlink) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	h.RID = ""
	h.Anchor = ""
	h.ExtraAttrs = nil
	h.Children = nil
	for _, a := range start.Attr {
		if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
			continue
		}
		switch {
		case a.Name.Space == ns.R && a.Name.Local == "id":
			h.RID = a.Value
		case a.Name.Space == ns.W && a.Name.Local == "anchor":
			h.Anchor = a.Value
		default:
			h.ExtraAttrs = append(h.ExtraAttrs, a)
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("wml: CT_Hyperlink: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name {
			case ns.Qn("w:r"):
				r := new(CT_R)
				if err := r.UnmarshalXML(d, t); err != nil {
					return err
				}
				h.Children = append(h.Children, r)
			case ns.Qn("w:bookmarkStart"):
				bs := new(CT_BookmarkStart)
				if err := bs.UnmarshalXML(d, t); err != nil {
					return err
				}
				h.Children = append(h.Children, bs)
			case ns.Qn("w:bookmarkEnd"):
				be := new(CT_BookmarkEnd)
				if err := be.UnmarshalXML(d, t); err != nil {
					return err
				}
				h.Children = append(h.Children, be)
			default:
				ri := new(RawInline)
				if err := ri.UnmarshalXML(d, t); err != nil {
					return err
				}
				h.Children = append(h.Children, ri)
			}
		case xml.EndElement:
			return nil
		}
	}
}

// Tooltip returns the w:tooltip attribute, or "".
func (h *CT_Hyperlink) Tooltip() string { return wAttr(h.ExtraAttrs, "tooltip") }

// SetTooltip sets w:tooltip. An empty string removes it.
func (h *CT_Hyperlink) SetTooltip(s string) {
	if s == "" {
		removeAttrFromSlice(&h.ExtraAttrs, "tooltip")
		return
	}
	setAttrInSlice(&h.ExtraAttrs, ns.Qn("w:tooltip"), s)
}

// History returns the w:history tri-state: nil when absent.
func (h *CT_Hyperlink) History() *bool {
	for _, a := range h.ExtraAttrs {
		if a.Name.Local == "history" {
			v := parseOnOffVal(a.Value)
			return &v
		}
	}
	return nil
}

// SetHistory sets w:history (written as "1" or "0"). nil removes it.
func (h *CT_Hyperlink) SetHistory(b *bool) {
	if b == nil {
		removeAttrFromSlice(&h.ExtraAttrs, "history")
		return
	}
	v := "0"
	if *b {
		v = "1"
	}
	setAttrInSlice(&h.ExtraAttrs, ns.Qn("w:history"), v)
}

// Text returns the visible text inside the hyperlink, using the same rules
// as CT_P.Text.
func (h *CT_Hyperlink) Text() string {
	var sb strings.Builder
	textFromInlines(h.Children, &sb, false)
	return sb.String()
}

// ---------------------------------------------------------------------------
// CT_BookmarkStart — w:bookmarkStart
// ---------------------------------------------------------------------------

// CT_BookmarkStart is the w:bookmarkStart element.
// ExtraAttrs preserves attributes beyond id and name (e.g. w:colFirst,
// w:colLast, w:displacedByCustomXml) for round-trip fidelity.
type CT_BookmarkStart struct {
	ID         int64
	Name       string
	ExtraAttrs []xml.Attr
}

func (*CT_BookmarkStart) isInline() {}

func (b *CT_BookmarkStart) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:bookmarkStart")
	start.Attr = append(start.Attr,
		xml.Attr{Name: ns.Qn("w:id"), Value: strconv.FormatInt(b.ID, 10)},
		xml.Attr{Name: ns.Qn("w:name"), Value: b.Name},
	)
	start.Attr = append(start.Attr, b.ExtraAttrs...)
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func (b *CT_BookmarkStart) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	b.ExtraAttrs = nil
	for _, a := range start.Attr {
		// Namespace declarations are re-emitted by the encoder; keeping them
		// here would double-declare on the next marshal.
		if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
			continue
		}
		switch a.Name.Local {
		case "name":
			b.Name = a.Value
		case "id":
			n, err := strconv.ParseInt(a.Value, 10, 64)
			if err != nil {
				return fmt.Errorf("wml: CT_BookmarkStart: invalid id %q: %w", a.Value, err)
			}
			b.ID = n
		default:
			b.ExtraAttrs = append(b.ExtraAttrs, a)
		}
	}
	return d.Skip()
}

// ---------------------------------------------------------------------------
// CT_BookmarkEnd — w:bookmarkEnd
// ---------------------------------------------------------------------------

// CT_BookmarkEnd is the w:bookmarkEnd element.
// ExtraAttrs preserves attributes beyond id (e.g. w:displacedByCustomXml).
type CT_BookmarkEnd struct {
	ID         int64
	ExtraAttrs []xml.Attr
}

func (*CT_BookmarkEnd) isInline() {}

func (b *CT_BookmarkEnd) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:bookmarkEnd")
	start.Attr = append(start.Attr,
		xml.Attr{Name: ns.Qn("w:id"), Value: strconv.FormatInt(b.ID, 10)},
	)
	start.Attr = append(start.Attr, b.ExtraAttrs...)
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

func (b *CT_BookmarkEnd) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	b.ExtraAttrs = nil
	for _, a := range start.Attr {
		if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
			continue
		}
		if a.Name.Local == "id" {
			n, err := strconv.ParseInt(a.Value, 10, 64)
			if err != nil {
				return fmt.Errorf("wml: CT_BookmarkEnd: invalid id %q: %w", a.Value, err)
			}
			b.ID = n
		} else {
			b.ExtraAttrs = append(b.ExtraAttrs, a)
		}
	}
	return d.Skip()
}

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------

func init() {
	registry.Register(ns.Qn("w:p"), func() registry.Element { return new(CT_P) })
	registry.Register(ns.Qn("w:hyperlink"), func() registry.Element { return new(CT_Hyperlink) })
	registry.Register(ns.Qn("w:bookmarkStart"), func() registry.Element { return new(CT_BookmarkStart) })
	registry.Register(ns.Qn("w:bookmarkEnd"), func() registry.Element { return new(CT_BookmarkEnd) })
}
