package wml

import (
	"encoding/xml"
	"fmt"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
	"github.com/theAstiv/docx-go/internal/oxml/registry"
)

// CT_PPr is the w:pPr element: paragraph properties.
//
// Most children are stored in raw in their original order for round-trip
// fidelity. SectPr is extracted as a typed field so callers can create and
// inspect embedded section breaks without touching the raw bytes.
type CT_PPr struct {
	// raw holds all w:pPr child elements except w:sectPr, in document order.
	raw []*raw.RawElement
	// SectPr is the embedded section-break element (w:p > w:pPr > w:sectPr).
	// Present only on the last paragraph of a non-final section; nil otherwise.
	SectPr *CT_SectPr
}

// MarshalXML emits <w:pPr> with raw children, inserting SectPr (xs:sequence
// position 35) before any element at a higher position (e.g. w:pPrChange=36).
func (p *CT_PPr) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:pPr")
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("wml: CT_PPr start: %w", err)
	}
	sectPrEmitted := false
	for _, child := range p.raw {
		// Inject SectPr before any xs:sequence element whose position > 35.
		if !sectPrEmitted && p.SectPr != nil {
			if pos, ok := pPrChildSeqPos[child.QName]; ok && pos > 35 {
				if err := p.SectPr.MarshalXML(e, xml.StartElement{}); err != nil {
					return err
				}
				sectPrEmitted = true
			}
		}
		if err := child.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	if !sectPrEmitted && p.SectPr != nil {
		if err := p.SectPr.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML reads a w:pPr element, capturing all children. w:sectPr is
// parsed into the typed SectPr field; all other children go into raw.
func (p *CT_PPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	p.raw = p.raw[:0]
	p.SectPr = nil
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("wml: CT_PPr: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name == ns.Qn("w:sectPr") {
				p.SectPr = new(CT_SectPr)
				if err := p.SectPr.UnmarshalXML(d, t); err != nil {
					return err
				}
			} else {
				re := new(raw.RawElement)
				if err := re.UnmarshalXML(d, t); err != nil {
					return err
				}
				p.raw = append(p.raw, re)
			}
		case xml.EndElement:
			return nil
		}
	}
}

// ---------------------------------------------------------------------------
// Style
// ---------------------------------------------------------------------------

// StyleID returns the paragraph style ID (w:pStyle/@w:val), or "".
func (p *CT_PPr) StyleID() string {
	return rawGetStr(p.raw, ns.Qn("w:pStyle"))
}

// SetStyleID sets the paragraph style. An empty string removes the element.
func (p *CT_PPr) SetStyleID(s string) {
	rawSetStr(&p.raw, ns.Qn("w:pStyle"), s)
}

// ---------------------------------------------------------------------------
// Alignment (justification)
// ---------------------------------------------------------------------------

// Alignment returns the w:jc/@w:val, or "".
func (p *CT_PPr) Alignment() string {
	return rawGetStr(p.raw, ns.Qn("w:jc"))
}

// SetAlignment sets the paragraph alignment. An empty string removes it.
func (p *CT_PPr) SetAlignment(a string) {
	rawSetStr(&p.raw, ns.Qn("w:jc"), a)
}

// ---------------------------------------------------------------------------
// Outline level
// ---------------------------------------------------------------------------

// OutlineLevel returns the w:outlineLvl/@w:val, or nil.
func (p *CT_PPr) OutlineLevel() *int64 {
	return rawGetInt64Attr(p.raw, ns.Qn("w:outlineLvl"), "val")
}

// SetOutlineLevel sets the outline level (0-8). nil removes it.
func (p *CT_PPr) SetOutlineLevel(lvl *int64) {
	rawSetInt64Attr(&p.raw, ns.Qn("w:outlineLvl"), ns.Qn("w:val"), lvl)
}

// ---------------------------------------------------------------------------
// OnOff flags
// ---------------------------------------------------------------------------

// KeepNext returns the w:keepNext tri-state.
func (p *CT_PPr) KeepNext() *bool { return rawGetOnOff(p.raw, ns.Qn("w:keepNext")) }

// SetKeepNext sets w:keepNext. nil removes the element.
func (p *CT_PPr) SetKeepNext(b *bool) { rawSetOnOff(&p.raw, ns.Qn("w:keepNext"), b) }

// KeepLines returns the w:keepLines tri-state.
func (p *CT_PPr) KeepLines() *bool { return rawGetOnOff(p.raw, ns.Qn("w:keepLines")) }

// SetKeepLines sets w:keepLines.
func (p *CT_PPr) SetKeepLines(b *bool) { rawSetOnOff(&p.raw, ns.Qn("w:keepLines"), b) }

// PageBreakBefore returns the w:pageBreakBefore tri-state.
func (p *CT_PPr) PageBreakBefore() *bool {
	return rawGetOnOff(p.raw, ns.Qn("w:pageBreakBefore"))
}

// SetPageBreakBefore sets w:pageBreakBefore.
func (p *CT_PPr) SetPageBreakBefore(b *bool) {
	rawSetOnOff(&p.raw, ns.Qn("w:pageBreakBefore"), b)
}

// WidowControl returns the w:widowControl tri-state.
func (p *CT_PPr) WidowControl() *bool { return rawGetOnOff(p.raw, ns.Qn("w:widowControl")) }

// SetWidowControl sets w:widowControl.
func (p *CT_PPr) SetWidowControl(b *bool) { rawSetOnOff(&p.raw, ns.Qn("w:widowControl"), b) }

// SuppressLineNumbers returns the w:suppressLineNumbers tri-state.
func (p *CT_PPr) SuppressLineNumbers() *bool {
	return rawGetOnOff(p.raw, ns.Qn("w:suppressLineNumbers"))
}

// SetSuppressLineNumbers sets w:suppressLineNumbers.
func (p *CT_PPr) SetSuppressLineNumbers(b *bool) {
	rawSetOnOff(&p.raw, ns.Qn("w:suppressLineNumbers"), b)
}

// SuppressAutoHyphens returns the w:suppressAutoHyphens tri-state.
func (p *CT_PPr) SuppressAutoHyphens() *bool {
	return rawGetOnOff(p.raw, ns.Qn("w:suppressAutoHyphens"))
}

// SetSuppressAutoHyphens sets w:suppressAutoHyphens.
func (p *CT_PPr) SetSuppressAutoHyphens(b *bool) {
	rawSetOnOff(&p.raw, ns.Qn("w:suppressAutoHyphens"), b)
}

// Bidi returns the w:bidi tri-state.
func (p *CT_PPr) Bidi() *bool { return rawGetOnOff(p.raw, ns.Qn("w:bidi")) }

// SetBidi sets w:bidi.
func (p *CT_PPr) SetBidi(b *bool) { rawSetOnOff(&p.raw, ns.Qn("w:bidi"), b) }

// Kinsoku returns the w:kinsoku tri-state.
func (p *CT_PPr) Kinsoku() *bool { return rawGetOnOff(p.raw, ns.Qn("w:kinsoku")) }

// SetKinsoku sets w:kinsoku.
func (p *CT_PPr) SetKinsoku(b *bool) { rawSetOnOff(&p.raw, ns.Qn("w:kinsoku"), b) }

// WordWrap returns the w:wordWrap tri-state.
func (p *CT_PPr) WordWrap() *bool { return rawGetOnOff(p.raw, ns.Qn("w:wordWrap")) }

// SetWordWrap sets w:wordWrap.
func (p *CT_PPr) SetWordWrap(b *bool) { rawSetOnOff(&p.raw, ns.Qn("w:wordWrap"), b) }

// OverflowPunct returns the w:overflowPunct tri-state.
func (p *CT_PPr) OverflowPunct() *bool { return rawGetOnOff(p.raw, ns.Qn("w:overflowPunct")) }

// SetOverflowPunct sets w:overflowPunct.
func (p *CT_PPr) SetOverflowPunct(b *bool) { rawSetOnOff(&p.raw, ns.Qn("w:overflowPunct"), b) }

// TopLinePunct returns the w:topLinePunct tri-state.
func (p *CT_PPr) TopLinePunct() *bool { return rawGetOnOff(p.raw, ns.Qn("w:topLinePunct")) }

// SetTopLinePunct sets w:topLinePunct.
func (p *CT_PPr) SetTopLinePunct(b *bool) { rawSetOnOff(&p.raw, ns.Qn("w:topLinePunct"), b) }

// AutoSpaceDE returns the w:autoSpaceDE tri-state.
func (p *CT_PPr) AutoSpaceDE() *bool { return rawGetOnOff(p.raw, ns.Qn("w:autoSpaceDE")) }

// SetAutoSpaceDE sets w:autoSpaceDE.
func (p *CT_PPr) SetAutoSpaceDE(b *bool) { rawSetOnOff(&p.raw, ns.Qn("w:autoSpaceDE"), b) }

// AutoSpaceDN returns the w:autoSpaceDN tri-state.
func (p *CT_PPr) AutoSpaceDN() *bool { return rawGetOnOff(p.raw, ns.Qn("w:autoSpaceDN")) }

// SetAutoSpaceDN sets w:autoSpaceDN.
func (p *CT_PPr) SetAutoSpaceDN(b *bool) { rawSetOnOff(&p.raw, ns.Qn("w:autoSpaceDN"), b) }

// ContextualSpacing returns the w:contextualSpacing tri-state.
func (p *CT_PPr) ContextualSpacing() *bool {
	return rawGetOnOff(p.raw, ns.Qn("w:contextualSpacing"))
}

// SetContextualSpacing sets w:contextualSpacing.
func (p *CT_PPr) SetContextualSpacing(b *bool) {
	rawSetOnOff(&p.raw, ns.Qn("w:contextualSpacing"), b)
}

// MirrorIndents returns the w:mirrorIndents tri-state.
func (p *CT_PPr) MirrorIndents() *bool { return rawGetOnOff(p.raw, ns.Qn("w:mirrorIndents")) }

// SetMirrorIndents sets w:mirrorIndents.
func (p *CT_PPr) SetMirrorIndents(b *bool) { rawSetOnOff(&p.raw, ns.Qn("w:mirrorIndents"), b) }

// ---------------------------------------------------------------------------
// Spacing (w:spacing)
// ---------------------------------------------------------------------------

// SpacingBefore returns w:spacing/@w:before in twips, or nil.
func (p *CT_PPr) SpacingBefore() *int64 {
	return rawGetInt64Attr(p.raw, ns.Qn("w:spacing"), "before")
}

// SetSpacingBefore sets w:spacing/@w:before. nil removes the attribute.
func (p *CT_PPr) SetSpacingBefore(twips *int64) {
	rawSetInt64Attr(&p.raw, ns.Qn("w:spacing"), ns.Qn("w:before"), twips)
}

// SpacingAfter returns w:spacing/@w:after in twips, or nil.
func (p *CT_PPr) SpacingAfter() *int64 {
	return rawGetInt64Attr(p.raw, ns.Qn("w:spacing"), "after")
}

// SetSpacingAfter sets w:spacing/@w:after. nil removes the attribute.
func (p *CT_PPr) SetSpacingAfter(twips *int64) {
	rawSetInt64Attr(&p.raw, ns.Qn("w:spacing"), ns.Qn("w:after"), twips)
}

// SpacingLine returns w:spacing/@w:line, or nil.
// The unit depends on SpacingLineRule: 240ths of a line for "auto",
// twips for "exact" or "atLeast".
func (p *CT_PPr) SpacingLine() *int64 {
	return rawGetInt64Attr(p.raw, ns.Qn("w:spacing"), "line")
}

// SetSpacingLine sets w:spacing/@w:line.
func (p *CT_PPr) SetSpacingLine(v *int64) {
	rawSetInt64Attr(&p.raw, ns.Qn("w:spacing"), ns.Qn("w:line"), v)
}

// SpacingLineRule returns w:spacing/@w:lineRule ("auto", "exact", "atLeast"), or "".
func (p *CT_PPr) SpacingLineRule() string {
	return rawGetAttrOnElement(p.raw, ns.Qn("w:spacing"), "lineRule")
}

// SetSpacingLineRule sets w:spacing/@w:lineRule.
func (p *CT_PPr) SetSpacingLineRule(rule string) {
	if rule == "" {
		rawRemoveAttrFromElement(&p.raw, ns.Qn("w:spacing"), "lineRule")
		return
	}
	rawSetAttrOnElement(&p.raw, ns.Qn("w:spacing"), ns.Qn("w:lineRule"), rule)
}

// ---------------------------------------------------------------------------
// Indentation (w:ind)
// ---------------------------------------------------------------------------

// IndStart returns w:ind/@w:start (or @w:left for legacy), or nil.
func (p *CT_PPr) IndStart() *int64 {
	v := rawGetInt64Attr(p.raw, ns.Qn("w:ind"), "start")
	if v != nil {
		return v
	}
	return rawGetInt64Attr(p.raw, ns.Qn("w:ind"), "left")
}

// SetIndStart sets w:ind/@w:start.
func (p *CT_PPr) SetIndStart(twips *int64) {
	rawSetInt64Attr(&p.raw, ns.Qn("w:ind"), ns.Qn("w:start"), twips)
}

// IndEnd returns w:ind/@w:end (or @w:right for legacy), or nil.
func (p *CT_PPr) IndEnd() *int64 {
	v := rawGetInt64Attr(p.raw, ns.Qn("w:ind"), "end")
	if v != nil {
		return v
	}
	return rawGetInt64Attr(p.raw, ns.Qn("w:ind"), "right")
}

// SetIndEnd sets w:ind/@w:end.
func (p *CT_PPr) SetIndEnd(twips *int64) {
	rawSetInt64Attr(&p.raw, ns.Qn("w:ind"), ns.Qn("w:end"), twips)
}

// IndFirstLine returns w:ind/@w:firstLine, or nil.
func (p *CT_PPr) IndFirstLine() *int64 {
	return rawGetInt64Attr(p.raw, ns.Qn("w:ind"), "firstLine")
}

// SetIndFirstLine sets w:ind/@w:firstLine.
func (p *CT_PPr) SetIndFirstLine(twips *int64) {
	rawSetInt64Attr(&p.raw, ns.Qn("w:ind"), ns.Qn("w:firstLine"), twips)
}

// IndHanging returns w:ind/@w:hanging, or nil.
func (p *CT_PPr) IndHanging() *int64 {
	return rawGetInt64Attr(p.raw, ns.Qn("w:ind"), "hanging")
}

// SetIndHanging sets w:ind/@w:hanging.
func (p *CT_PPr) SetIndHanging(twips *int64) {
	rawSetInt64Attr(&p.raw, ns.Qn("w:ind"), ns.Qn("w:hanging"), twips)
}

// ---------------------------------------------------------------------------
// Numbering reference (w:numPr)
// ---------------------------------------------------------------------------

// NumID returns the numId value from w:numPr, or nil if absent.
func (p *CT_PPr) NumID() *int64 {
	for _, child := range p.raw {
		if child.QName == ns.Qn("w:numPr") {
			nested := rawElementsFromTokens(child.Children)
			return rawGetInt64Attr(nested, ns.Qn("w:numId"), "val")
		}
	}
	return nil
}

// NumILvl returns the ilvl (list level, 0-based) from w:numPr, or nil.
func (p *CT_PPr) NumILvl() *int64 {
	for _, child := range p.raw {
		if child.QName == ns.Qn("w:numPr") {
			nested := rawElementsFromTokens(child.Children)
			return rawGetInt64Attr(nested, ns.Qn("w:ilvl"), "val")
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// rawGetAttrOnElement reads a named attribute from the first child matching qname.
func rawGetAttrOnElement(children []*raw.RawElement, qname xml.Name, attrLocal string) string {
	for _, child := range children {
		if child.QName == qname {
			return wAttr(child.Attrs, attrLocal)
		}
	}
	return ""
}

// rawElementsFromTokens extracts *raw.RawElement entries from a []xml.Token slice.
// Used to read nested children stored inside a raw.RawElement.Children.
func rawElementsFromTokens(tokens []xml.Token) []*raw.RawElement {
	var out []*raw.RawElement
	for _, tok := range tokens {
		if re, ok := tok.(*raw.RawElement); ok {
			out = append(out, re)
		}
	}
	return out
}

func init() {
	registry.Register(ns.Qn("w:pPr"), func() registry.Element { return new(CT_PPr) })
}
