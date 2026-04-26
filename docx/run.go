package docx

import (
	"math"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// Run is a proxy for the w:r element: a styled run of text.
type Run struct {
	r *wml.CT_R
}

// Text returns the visible text of this run, using the same rendering rules as
// Paragraph.Text(): tabs as \t, breaks as \n, field results included, instructions excluded.
func (r *Run) Text() string {
	return r.r.Text()
}

// SetText replaces the run's text content. If the run already has a single
// w:t element it is updated; otherwise all existing text elements are replaced.
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

// Font returns a RunFont proxy for reading and writing run formatting.
// Property elements (CT_RPr) are created lazily only when a setter is called.
func (r *Run) Font() *RunFont {
	return &RunFont{r: r.r}
}

// ---------------------------------------------------------------------------
// RunFont — run character formatting proxy
// ---------------------------------------------------------------------------

// RunFont is a proxy for w:rPr, providing typed access to character
// formatting properties. Mutations write through to the underlying CT_RPr.
type RunFont struct {
	r *wml.CT_R
}

func (f *RunFont) rpr() *wml.CT_RPr {
	if f.r.RPr == nil {
		f.r.RPr = new(wml.CT_RPr)
	}
	return f.r.RPr
}

// Bold returns the Bold tri-state, or nil (inherit from style).
func (f *RunFont) Bold() *bool {
	if f.r.RPr == nil {
		return nil
	}
	return f.r.RPr.Bold()
}

// SetBold sets Bold. nil removes the element (inherit from style).
func (f *RunFont) SetBold(b *bool) { f.rpr().SetBold(b) }

// Italic returns the Italic tri-state, or nil.
func (f *RunFont) Italic() *bool {
	if f.r.RPr == nil {
		return nil
	}
	return f.r.RPr.Italic()
}

// SetItalic sets Italic.
func (f *RunFont) SetItalic(b *bool) { f.rpr().SetItalic(b) }

// Underline returns the underline style, or "".
func (f *RunFont) Underline() UnderlineStyle {
	if f.r.RPr == nil {
		return ""
	}
	return UnderlineStyle(f.r.RPr.UnderlineStyle())
}

// SetUnderline sets the underline style. An empty string removes underline.
func (f *RunFont) SetUnderline(style UnderlineStyle) { f.rpr().SetUnderlineStyle(string(style)) }

// Strike returns the strikethrough tri-state, or nil.
func (f *RunFont) Strike() *bool {
	if f.r.RPr == nil {
		return nil
	}
	return f.r.RPr.Strike()
}

// SetStrike sets strikethrough.
func (f *RunFont) SetStrike(b *bool) { f.rpr().SetStrike(b) }

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
	if f.r.RPr == nil {
		return nil
	}
	return f.r.RPr.SzHalfPt()
}

// SetSizePt sets the font size in points, rounded to the nearest half-point.
func (f *RunFont) SetSizePt(pt float64) {
	hp := int64(math.Round(pt * 2))
	f.rpr().SetSzHalfPt(&hp)
}

// SetSizeHalfPt sets the font size in half-points. nil removes it.
func (f *RunFont) SetSizeHalfPt(hp *int64) { f.rpr().SetSzHalfPt(hp) }

// Color returns the hex RGB color string (e.g. "FF0000"), or "".
func (f *RunFont) Color() string {
	if f.r.RPr == nil {
		return ""
	}
	return f.r.RPr.ColorVal()
}

// SetColor sets the hex RGB color string. An empty string removes color.
func (f *RunFont) SetColor(hex string) { f.rpr().SetColorVal(hex) }

// Style returns the character style ID, or "".
func (f *RunFont) Style() string {
	if f.r.RPr == nil {
		return ""
	}
	return f.r.RPr.StyleID()
}

// SetStyle sets the character style ID. An empty string removes it.
func (f *RunFont) SetStyle(s string) { f.rpr().SetStyleID(s) }

// VertAlign returns the vertical alignment ("superscript", "subscript", or "").
func (f *RunFont) VertAlign() VertAlignValue {
	if f.r.RPr == nil {
		return ""
	}
	return VertAlignValue(f.r.RPr.VertAlign())
}

// SetVertAlign sets vertical alignment.
func (f *RunFont) SetVertAlign(v VertAlignValue) { f.rpr().SetVertAlign(string(v)) }

// Caps returns the AllCaps tri-state, or nil.
func (f *RunFont) Caps() *bool {
	if f.r.RPr == nil {
		return nil
	}
	return f.r.RPr.Caps()
}

// SetCaps sets AllCaps.
func (f *RunFont) SetCaps(b *bool) { f.rpr().SetCaps(b) }

// SmallCaps returns the SmallCaps tri-state, or nil.
func (f *RunFont) SmallCaps() *bool {
	if f.r.RPr == nil {
		return nil
	}
	return f.r.RPr.SmallCaps()
}

// SetSmallCaps sets SmallCaps.
func (f *RunFont) SetSmallCaps(b *bool) { f.rpr().SetSmallCaps(b) }

// Highlight returns the highlight color name (e.g. "yellow", "none"), or "".
func (f *RunFont) Highlight() string {
	if f.r.RPr == nil {
		return ""
	}
	return f.r.RPr.HighlightColor()
}

// SetHighlight sets the highlight color name. An empty string removes it.
func (f *RunFont) SetHighlight(color string) { f.rpr().SetHighlightColor(color) }

// ASCII returns the ASCII font face name, or "".
func (f *RunFont) ASCII() string {
	if f.r.RPr == nil {
		return ""
	}
	return f.r.RPr.FontASCII()
}

// SetASCII sets the ASCII font face name. An empty string removes it.
func (f *RunFont) SetASCII(name string) { f.rpr().SetFontASCII(name) }

// HAnsi returns the High-ANSI font face name, or "".
func (f *RunFont) HAnsi() string {
	if f.r.RPr == nil {
		return ""
	}
	return f.r.RPr.FontHAnsi()
}

// SetHAnsi sets the High-ANSI font face name. An empty string removes it.
func (f *RunFont) SetHAnsi(name string) { f.rpr().SetFontHAnsi(name) }
