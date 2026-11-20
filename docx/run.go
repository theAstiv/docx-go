package docx

import (
	"math"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// Run is a proxy for the w:r element: a styled run of text.
type Run struct {
	r  *wml.CT_R
	st *story
	// parent is the inline list (a paragraph's or hyperlink's children) the
	// run was found in or added to: a hint for Remove (locate.go).
	parent *[]wml.InlineElement
	// gen is story.stamp() when parent was recorded (locate.go).
	gen uint64
}

// Text returns the visible text of this run, using the same rendering rules as
// Paragraph.Text(): tabs as \t, breaks as \n, field results included, instructions excluded.
func (r *Run) Text() string {
	return r.r.Text()
}

// SetText replaces the run's text content. Existing w:t elements are removed and
// replaced; other run content (w:br, w:tab, w:sym, etc.) is preserved in place.
func (r *Run) SetText(text string) {
	// In-place filter: keep non-CT_Text children (write index always ≤ read index).
	filtered := r.r.Children[:0]
	for _, child := range r.r.Children {
		if _, ok := child.(*wml.CT_Text); !ok {
			filtered = append(filtered, child)
		}
	}
	r.r.Children = filtered
	if text != "" {
		t := new(wml.CT_Text)
		t.Content = text
		t.Space = "preserve"
		r.r.Children = append(r.r.Children, t)
	}
}

// AddBreak appends a w:br element to this run.
// Use BreakLine (or BreakTextWrapping for the explicit form) for a soft line break,
// BreakPage for a page break, BreakColumn for a column break.
func (r *Run) AddBreak(kind BreakType) {
	br := new(wml.CT_Br)
	br.Type = string(kind)
	r.r.Children = append(r.r.Children, br)
}

// AddTab appends a w:tab element to this run, advancing to the next tab stop:
// one set with ParagraphFormat.SetTabStops or AddTabStop, one from the
// paragraph style, or else the next multiple of the document's default tab
// stop (Document.SetDefaultTabStop).
func (r *Run) AddTab() {
	r.r.Children = append(r.r.Children, new(wml.CT_Tab))
}

// Font returns a RunFont proxy for reading and writing run formatting.
// Property elements (CT_RPr) are created lazily only when a setter is called.
func (r *Run) Font() *RunFont {
	return &RunFont{slot: &r.r.RPr}
}

// ---------------------------------------------------------------------------
// RunFont — run character formatting proxy
// ---------------------------------------------------------------------------

// RunFont is a proxy for w:rPr, providing typed access to character
// formatting properties. Mutations write through to the underlying CT_RPr.
//
// The proxy binds to a *CT_RPr slot rather than a run, so the same type
// serves runs (CT_R.RPr) and styles (CT_Style.RPr).
type RunFont struct {
	slot **wml.CT_RPr
}

// cur returns the current CT_RPr, or nil when none has been created.
func (f *RunFont) cur() *wml.CT_RPr { return *f.slot }

func (f *RunFont) rpr() *wml.CT_RPr {
	if *f.slot == nil {
		*f.slot = new(wml.CT_RPr)
	}
	return *f.slot
}

// edit returns the CT_RPr a setter writes to. A setter that clears a
// property (clear is true) on a run without w:rPr has nothing to remove: it
// gets a throwaway element, so it does not add an empty <w:rPr/>.
func (f *RunFont) edit(clear bool) *wml.CT_RPr {
	if clear && *f.slot == nil {
		return new(wml.CT_RPr)
	}
	return f.rpr()
}

// Bold returns the Bold tri-state, or nil (inherit from style).
func (f *RunFont) Bold() *bool {
	if f.cur() == nil {
		return nil
	}
	return f.cur().Bold()
}

// SetBold sets Bold. nil removes the element (inherit from style).
func (f *RunFont) SetBold(b *bool) { f.edit(b == nil).SetBold(b) }

// Italic returns the Italic tri-state, or nil.
func (f *RunFont) Italic() *bool {
	if f.cur() == nil {
		return nil
	}
	return f.cur().Italic()
}

// SetItalic sets Italic.
func (f *RunFont) SetItalic(b *bool) { f.edit(b == nil).SetItalic(b) }

// Underline returns the underline style, or "".
func (f *RunFont) Underline() UnderlineStyle {
	if f.cur() == nil {
		return ""
	}
	return UnderlineStyle(f.cur().UnderlineStyle())
}

// SetUnderline sets the underline style. An empty string removes underline.
func (f *RunFont) SetUnderline(style UnderlineStyle) {
	f.edit(style == "").SetUnderlineStyle(string(style))
}

// Strike returns the strikethrough tri-state, or nil.
func (f *RunFont) Strike() *bool {
	if f.cur() == nil {
		return nil
	}
	return f.cur().Strike()
}

// SetStrike sets strikethrough.
func (f *RunFont) SetStrike(b *bool) { f.edit(b == nil).SetStrike(b) }

// SizePt returns the font size in points (half-points / 2), or 0 if not set.
// Use SizeHalfPt for half-point precision.
func (f *RunFont) SizePt() float64 {
	hp := f.SizeHalfPt()
	if hp == nil {
		return 0
	}
	return float64(*hp) / 2
}

// SizeHalfPt returns the font size in half-points, or nil if not set.
func (f *RunFont) SizeHalfPt() *int64 {
	if f.cur() == nil {
		return nil
	}
	return f.cur().SzHalfPt()
}

// SetSizePt sets the font size in points, rounded to the nearest half-point.
func (f *RunFont) SetSizePt(pt float64) {
	hp := int64(math.Round(pt * 2))
	f.rpr().SetSzHalfPt(&hp)
}

// SetSizeHalfPt sets the font size in half-points. nil removes it.
func (f *RunFont) SetSizeHalfPt(hp *int64) { f.edit(hp == nil).SetSzHalfPt(hp) }

// Color returns the hex RGB color string (e.g. "FF0000"), or "".
func (f *RunFont) Color() string {
	if f.cur() == nil {
		return ""
	}
	return f.cur().ColorVal()
}

// SetColor sets the hex RGB color string. An empty string removes color.
func (f *RunFont) SetColor(hex string) { f.edit(hex == "").SetColorVal(hex) }

// Style returns the character style ID, or "".
func (f *RunFont) Style() string {
	if f.cur() == nil {
		return ""
	}
	return f.cur().StyleID()
}

// SetStyle sets the character style ID. An empty string removes it.
func (f *RunFont) SetStyle(s string) { f.edit(s == "").SetStyleID(s) }

// VertAlign returns the vertical alignment ("superscript", "subscript", or "").
func (f *RunFont) VertAlign() VertAlignValue {
	if f.cur() == nil {
		return ""
	}
	return VertAlignValue(f.cur().VertAlign())
}

// SetVertAlign sets vertical alignment.
func (f *RunFont) SetVertAlign(v VertAlignValue) { f.edit(v == "").SetVertAlign(string(v)) }

// Name returns the font family (w:rFonts/@w:ascii), or "" if not set.
func (f *RunFont) Name() string {
	if f.cur() == nil {
		return ""
	}
	return f.cur().FontASCII()
}

// SetName sets the font family for Latin text (the w:ascii and w:hAnsi
// attributes of w:rFonts). An empty string clears both.
func (f *RunFont) SetName(name string) {
	if name == "" && f.cur() == nil {
		return
	}
	f.rpr().SetFontASCII(name)
	f.rpr().SetFontHAnsi(name)
}

// Caps returns the AllCaps tri-state, or nil.
func (f *RunFont) Caps() *bool {
	if f.cur() == nil {
		return nil
	}
	return f.cur().Caps()
}

// SetCaps sets AllCaps.
func (f *RunFont) SetCaps(b *bool) { f.edit(b == nil).SetCaps(b) }

// SmallCaps returns the SmallCaps tri-state, or nil.
func (f *RunFont) SmallCaps() *bool {
	if f.cur() == nil {
		return nil
	}
	return f.cur().SmallCaps()
}

// SetSmallCaps sets SmallCaps.
func (f *RunFont) SetSmallCaps(b *bool) { f.edit(b == nil).SetSmallCaps(b) }

// Highlight returns the highlight color name (e.g. "yellow", "none"), or "".
func (f *RunFont) Highlight() string {
	if f.cur() == nil {
		return ""
	}
	return f.cur().HighlightColor()
}

// SetHighlight sets the highlight color name. An empty string removes it.
func (f *RunFont) SetHighlight(color string) { f.edit(color == "").SetHighlightColor(color) }

// ASCII returns the ASCII font face name, or "".
func (f *RunFont) ASCII() string {
	if f.cur() == nil {
		return ""
	}
	return f.cur().FontASCII()
}

// SetASCII sets the ASCII font face name. An empty string removes it.
func (f *RunFont) SetASCII(name string) { f.edit(name == "").SetFontASCII(name) }

// HAnsi returns the High-ANSI font face name, or "".
func (f *RunFont) HAnsi() string {
	if f.cur() == nil {
		return ""
	}
	return f.cur().FontHAnsi()
}

// SetHAnsi sets the High-ANSI font face name. An empty string removes it.
func (f *RunFont) SetHAnsi(name string) { f.edit(name == "").SetFontHAnsi(name) }
