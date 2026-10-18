package wml

import (
	"encoding/xml"
	"fmt"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
	"github.com/theAstiv/docx-go/internal/oxml/registry"
)

// CT_RPr is the w:rPr element: run properties.
//
// All children are stored in raw in their original order so that
// marshal → unmarshal → marshal produces canonically-equal output.
// Typed accessors read/write through the raw slice.
type CT_RPr struct {
	raw []*raw.RawElement
}

// MarshalXML emits <w:rPr> with all raw children in their original order.
func (r *CT_RPr) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:rPr")
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("wml: CT_RPr start: %w", err)
	}
	if err := rawMarshalChildren(e, orderBySeq(r.raw, rPrChildSeqPos)); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// isEmpty reports whether the element has no children.
func (r *CT_RPr) isEmpty() bool { return len(r.raw) == 0 }

// UnmarshalXML reads a w:rPr element, capturing all children as raw.
func (r *CT_RPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	var err error
	r.raw, err = rawUnmarshalChildren(d)
	return err
}

// ---------------------------------------------------------------------------
// OnOff flags
// ---------------------------------------------------------------------------

func (r *CT_RPr) Bold() *bool           { return rawGetOnOff(r.raw, ns.Qn("w:b")) }
func (r *CT_RPr) SetBold(b *bool)       { rawSetOnOff(&r.raw, ns.Qn("w:b"), b) }
func (r *CT_RPr) BoldCS() *bool         { return rawGetOnOff(r.raw, ns.Qn("w:bCs")) }
func (r *CT_RPr) SetBoldCS(b *bool)     { rawSetOnOff(&r.raw, ns.Qn("w:bCs"), b) }
func (r *CT_RPr) Italic() *bool         { return rawGetOnOff(r.raw, ns.Qn("w:i")) }
func (r *CT_RPr) SetItalic(b *bool)     { rawSetOnOff(&r.raw, ns.Qn("w:i"), b) }
func (r *CT_RPr) ItalicCS() *bool       { return rawGetOnOff(r.raw, ns.Qn("w:iCs")) }
func (r *CT_RPr) SetItalicCS(b *bool)   { rawSetOnOff(&r.raw, ns.Qn("w:iCs"), b) }
func (r *CT_RPr) Caps() *bool           { return rawGetOnOff(r.raw, ns.Qn("w:caps")) }
func (r *CT_RPr) SetCaps(b *bool)       { rawSetOnOff(&r.raw, ns.Qn("w:caps"), b) }
func (r *CT_RPr) SmallCaps() *bool      { return rawGetOnOff(r.raw, ns.Qn("w:smallCaps")) }
func (r *CT_RPr) SetSmallCaps(b *bool)  { rawSetOnOff(&r.raw, ns.Qn("w:smallCaps"), b) }
func (r *CT_RPr) Strike() *bool         { return rawGetOnOff(r.raw, ns.Qn("w:strike")) }
func (r *CT_RPr) SetStrike(b *bool)     { rawSetOnOff(&r.raw, ns.Qn("w:strike"), b) }
func (r *CT_RPr) DStrike() *bool        { return rawGetOnOff(r.raw, ns.Qn("w:dstrike")) }
func (r *CT_RPr) SetDStrike(b *bool)    { rawSetOnOff(&r.raw, ns.Qn("w:dstrike"), b) }
func (r *CT_RPr) Vanish() *bool         { return rawGetOnOff(r.raw, ns.Qn("w:vanish")) }
func (r *CT_RPr) SetVanish(b *bool)     { rawSetOnOff(&r.raw, ns.Qn("w:vanish"), b) }
func (r *CT_RPr) WebHidden() *bool      { return rawGetOnOff(r.raw, ns.Qn("w:webHidden")) }
func (r *CT_RPr) SetWebHidden(b *bool)  { rawSetOnOff(&r.raw, ns.Qn("w:webHidden"), b) }
func (r *CT_RPr) Outline() *bool        { return rawGetOnOff(r.raw, ns.Qn("w:outline")) }
func (r *CT_RPr) SetOutline(b *bool)    { rawSetOnOff(&r.raw, ns.Qn("w:outline"), b) }
func (r *CT_RPr) Shadow() *bool         { return rawGetOnOff(r.raw, ns.Qn("w:shadow")) }
func (r *CT_RPr) SetShadow(b *bool)     { rawSetOnOff(&r.raw, ns.Qn("w:shadow"), b) }
func (r *CT_RPr) Emboss() *bool         { return rawGetOnOff(r.raw, ns.Qn("w:emboss")) }
func (r *CT_RPr) SetEmboss(b *bool)     { rawSetOnOff(&r.raw, ns.Qn("w:emboss"), b) }
func (r *CT_RPr) Imprint() *bool        { return rawGetOnOff(r.raw, ns.Qn("w:imprint")) }
func (r *CT_RPr) SetImprint(b *bool)    { rawSetOnOff(&r.raw, ns.Qn("w:imprint"), b) }
func (r *CT_RPr) RTL() *bool            { return rawGetOnOff(r.raw, ns.Qn("w:rtl")) }
func (r *CT_RPr) SetRTL(b *bool)        { rawSetOnOff(&r.raw, ns.Qn("w:rtl"), b) }
func (r *CT_RPr) NoProof() *bool        { return rawGetOnOff(r.raw, ns.Qn("w:noProof")) }
func (r *CT_RPr) SetNoProof(b *bool)    { rawSetOnOff(&r.raw, ns.Qn("w:noProof"), b) }
func (r *CT_RPr) SpecVanish() *bool     { return rawGetOnOff(r.raw, ns.Qn("w:specVanish")) }
func (r *CT_RPr) SetSpecVanish(b *bool) { rawSetOnOff(&r.raw, ns.Qn("w:specVanish"), b) }

// ---------------------------------------------------------------------------
// Style reference
// ---------------------------------------------------------------------------

// StyleID returns the run style ID (w:rStyle/@w:val), or "".
func (r *CT_RPr) StyleID() string { return rawGetStr(r.raw, ns.Qn("w:rStyle")) }

// SetStyleID sets the run style. An empty string removes the element.
func (r *CT_RPr) SetStyleID(s string) { rawSetStr(&r.raw, ns.Qn("w:rStyle"), s) }

// ---------------------------------------------------------------------------
// Font references (w:rFonts)
// ---------------------------------------------------------------------------

// FontASCII returns w:rFonts/@w:ascii.
func (r *CT_RPr) FontASCII() string {
	return rawGetAttrOnElement(r.raw, ns.Qn("w:rFonts"), "ascii")
}

// SetFontASCII sets w:rFonts/@w:ascii.
func (r *CT_RPr) SetFontASCII(name string) {
	if name == "" {
		rawRemoveAttrFromElement(&r.raw, ns.Qn("w:rFonts"), "ascii")
		return
	}
	rawSetAttrOnElement(&r.raw, ns.Qn("w:rFonts"), ns.Qn("w:ascii"), name)
}

// FontHAnsi returns w:rFonts/@w:hAnsi.
func (r *CT_RPr) FontHAnsi() string {
	return rawGetAttrOnElement(r.raw, ns.Qn("w:rFonts"), "hAnsi")
}

// SetFontHAnsi sets w:rFonts/@w:hAnsi.
func (r *CT_RPr) SetFontHAnsi(name string) {
	if name == "" {
		rawRemoveAttrFromElement(&r.raw, ns.Qn("w:rFonts"), "hAnsi")
		return
	}
	rawSetAttrOnElement(&r.raw, ns.Qn("w:rFonts"), ns.Qn("w:hAnsi"), name)
}

// FontEastAsia returns w:rFonts/@w:eastAsia.
func (r *CT_RPr) FontEastAsia() string {
	return rawGetAttrOnElement(r.raw, ns.Qn("w:rFonts"), "eastAsia")
}

// SetFontEastAsia sets w:rFonts/@w:eastAsia.
func (r *CT_RPr) SetFontEastAsia(name string) {
	if name == "" {
		rawRemoveAttrFromElement(&r.raw, ns.Qn("w:rFonts"), "eastAsia")
		return
	}
	rawSetAttrOnElement(&r.raw, ns.Qn("w:rFonts"), ns.Qn("w:eastAsia"), name)
}

// FontCS returns w:rFonts/@w:cs.
func (r *CT_RPr) FontCS() string {
	return rawGetAttrOnElement(r.raw, ns.Qn("w:rFonts"), "cs")
}

// SetFontCS sets w:rFonts/@w:cs.
func (r *CT_RPr) SetFontCS(name string) {
	if name == "" {
		rawRemoveAttrFromElement(&r.raw, ns.Qn("w:rFonts"), "cs")
		return
	}
	rawSetAttrOnElement(&r.raw, ns.Qn("w:rFonts"), ns.Qn("w:cs"), name)
}

// ---------------------------------------------------------------------------
// Font size (w:sz, w:szCs)
// ---------------------------------------------------------------------------

// SzHalfPt returns w:sz/@w:val in half-points, or nil.
func (r *CT_RPr) SzHalfPt() *int64 {
	return rawGetInt64Attr(r.raw, ns.Qn("w:sz"), "val")
}

// SetSzHalfPt sets w:sz/@w:val. nil removes the element.
func (r *CT_RPr) SetSzHalfPt(hp *int64) {
	rawSetInt64Attr(&r.raw, ns.Qn("w:sz"), ns.Qn("w:val"), hp)
}

// SzCSHalfPt returns w:szCs/@w:val in half-points, or nil.
func (r *CT_RPr) SzCSHalfPt() *int64 {
	return rawGetInt64Attr(r.raw, ns.Qn("w:szCs"), "val")
}

// SetSzCSHalfPt sets w:szCs/@w:val. nil removes the element.
func (r *CT_RPr) SetSzCSHalfPt(hp *int64) {
	rawSetInt64Attr(&r.raw, ns.Qn("w:szCs"), ns.Qn("w:val"), hp)
}

// ---------------------------------------------------------------------------
// Color (w:color)
// ---------------------------------------------------------------------------

// ColorVal returns w:color/@w:val (hex RGB or "auto"), or "".
func (r *CT_RPr) ColorVal() string {
	return rawGetAttrOnElement(r.raw, ns.Qn("w:color"), "val")
}

// SetColorVal sets w:color/@w:val. An empty string removes the element.
func (r *CT_RPr) SetColorVal(hex string) {
	if hex == "" {
		rawRemoveChild(&r.raw, ns.Qn("w:color"))
		return
	}
	rawSetAttrOnElement(&r.raw, ns.Qn("w:color"), ns.Qn("w:val"), hex)
}

// ---------------------------------------------------------------------------
// Highlight (w:highlight)
// ---------------------------------------------------------------------------

// HighlightColor returns w:highlight/@w:val (e.g. "yellow", "none"), or "".
func (r *CT_RPr) HighlightColor() string {
	return rawGetAttrOnElement(r.raw, ns.Qn("w:highlight"), "val")
}

// SetHighlightColor sets w:highlight/@w:val. An empty string removes the element.
func (r *CT_RPr) SetHighlightColor(color string) {
	if color == "" {
		rawRemoveChild(&r.raw, ns.Qn("w:highlight"))
		return
	}
	rawSetAttrOnElement(&r.raw, ns.Qn("w:highlight"), ns.Qn("w:val"), color)
}

// ---------------------------------------------------------------------------
// Underline (w:u)
// ---------------------------------------------------------------------------

// UnderlineStyle returns w:u/@w:val ("single", "double", "none", …), or "".
func (r *CT_RPr) UnderlineStyle() string {
	return rawGetAttrOnElement(r.raw, ns.Qn("w:u"), "val")
}

// SetUnderlineStyle sets w:u/@w:val. An empty string removes the element.
func (r *CT_RPr) SetUnderlineStyle(style string) {
	if style == "" {
		rawRemoveChild(&r.raw, ns.Qn("w:u"))
		return
	}
	rawSetAttrOnElement(&r.raw, ns.Qn("w:u"), ns.Qn("w:val"), style)
}

// UnderlineColor returns w:u/@w:color, or "".
func (r *CT_RPr) UnderlineColor() string {
	return rawGetAttrOnElement(r.raw, ns.Qn("w:u"), "color")
}

// SetUnderlineColor sets w:u/@w:color.
func (r *CT_RPr) SetUnderlineColor(hex string) {
	if hex == "" {
		rawRemoveAttrFromElement(&r.raw, ns.Qn("w:u"), "color")
		return
	}
	rawSetAttrOnElement(&r.raw, ns.Qn("w:u"), ns.Qn("w:color"), hex)
}

// ---------------------------------------------------------------------------
// Vertical alignment (w:vertAlign)
// ---------------------------------------------------------------------------

// VertAlign returns w:vertAlign/@w:val ("superscript", "subscript", "baseline"), or "".
func (r *CT_RPr) VertAlign() string {
	return rawGetAttrOnElement(r.raw, ns.Qn("w:vertAlign"), "val")
}

// SetVertAlign sets w:vertAlign/@w:val. An empty string removes the element.
func (r *CT_RPr) SetVertAlign(align string) {
	if align == "" {
		rawRemoveChild(&r.raw, ns.Qn("w:vertAlign"))
		return
	}
	rawSetAttrOnElement(&r.raw, ns.Qn("w:vertAlign"), ns.Qn("w:val"), align)
}

// ---------------------------------------------------------------------------
// Language (w:lang)
// ---------------------------------------------------------------------------

// LangVal returns w:lang/@w:val (BCP-47 tag), or "".
func (r *CT_RPr) LangVal() string {
	return rawGetAttrOnElement(r.raw, ns.Qn("w:lang"), "val")
}

// SetLangVal sets w:lang/@w:val. An empty string removes the attribute.
func (r *CT_RPr) SetLangVal(tag string) {
	if tag == "" {
		rawRemoveAttrFromElement(&r.raw, ns.Qn("w:lang"), "val")
		return
	}
	rawSetAttrOnElement(&r.raw, ns.Qn("w:lang"), ns.Qn("w:val"), tag)
}

// ---------------------------------------------------------------------------
// Character spacing (w:spacing) and text scale (w:w)
// ---------------------------------------------------------------------------

// CharSpacingTwips returns w:spacing/@w:val in twips, or nil.
func (r *CT_RPr) CharSpacingTwips() *int64 {
	return rawGetInt64Attr(r.raw, ns.Qn("w:spacing"), "val")
}

// SetCharSpacingTwips sets w:spacing/@w:val. nil removes the element.
func (r *CT_RPr) SetCharSpacingTwips(twips *int64) {
	rawSetInt64Attr(&r.raw, ns.Qn("w:spacing"), ns.Qn("w:val"), twips)
}

// TextScalePct returns w:w/@w:val (character width scale %), or nil.
func (r *CT_RPr) TextScalePct() *int64 {
	return rawGetInt64Attr(r.raw, ns.Qn("w:w"), "val")
}

// SetTextScalePct sets w:w/@w:val. nil removes the element.
func (r *CT_RPr) SetTextScalePct(pct *int64) {
	rawSetInt64Attr(&r.raw, ns.Qn("w:w"), ns.Qn("w:val"), pct)
}

// ---------------------------------------------------------------------------
// Kerning (w:kern)
// ---------------------------------------------------------------------------

// KernHalfPt returns w:kern/@w:val in half-points, or nil.
func (r *CT_RPr) KernHalfPt() *int64 {
	return rawGetInt64Attr(r.raw, ns.Qn("w:kern"), "val")
}

// SetKernHalfPt sets w:kern/@w:val. nil removes the element.
func (r *CT_RPr) SetKernHalfPt(hp *int64) {
	rawSetInt64Attr(&r.raw, ns.Qn("w:kern"), ns.Qn("w:val"), hp)
}

// ---------------------------------------------------------------------------
// Position (w:position — raised/lowered text in half-points)
// ---------------------------------------------------------------------------

// PositionHalfPt returns w:position/@w:val, or nil.
func (r *CT_RPr) PositionHalfPt() *int64 {
	return rawGetInt64Attr(r.raw, ns.Qn("w:position"), "val")
}

// SetPositionHalfPt sets w:position/@w:val. nil removes the element.
func (r *CT_RPr) SetPositionHalfPt(hp *int64) {
	rawSetInt64Attr(&r.raw, ns.Qn("w:position"), ns.Qn("w:val"), hp)
}

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------

func init() {
	registry.Register(ns.Qn("w:rPr"), func() registry.Element { return new(CT_RPr) })
}
