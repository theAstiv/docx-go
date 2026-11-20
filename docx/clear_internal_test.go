package docx

import (
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// A setter called with its clearing value on a run or paragraph that has
// no properties must not create an empty w:rPr / w:pPr (CLAUDE.md
// anti-pattern: "SetXxx(nil) allocating an empty <w:pPr/>").
func TestClearingSettersDoNotAllocate(t *testing.T) {
	fontSetters := map[string]func(*RunFont){
		"SetBold":       func(f *RunFont) { f.SetBold(nil) },
		"SetItalic":     func(f *RunFont) { f.SetItalic(nil) },
		"SetUnderline":  func(f *RunFont) { f.SetUnderline("") },
		"SetStrike":     func(f *RunFont) { f.SetStrike(nil) },
		"SetSizeHalfPt": func(f *RunFont) { f.SetSizeHalfPt(nil) },
		"SetColor":      func(f *RunFont) { f.SetColor("") },
		"SetStyle":      func(f *RunFont) { f.SetStyle("") },
		"SetVertAlign":  func(f *RunFont) { f.SetVertAlign("") },
		"SetName":       func(f *RunFont) { f.SetName("") },
		"SetCaps":       func(f *RunFont) { f.SetCaps(nil) },
		"SetSmallCaps":  func(f *RunFont) { f.SetSmallCaps(nil) },
		"SetHighlight":  func(f *RunFont) { f.SetHighlight("") },
		"SetASCII":      func(f *RunFont) { f.SetASCII("") },
		"SetHAnsi":      func(f *RunFont) { f.SetHAnsi("") },
	}
	for name, set := range fontSetters {
		var rpr *wml.CT_RPr
		set(&RunFont{slot: &rpr})
		if rpr != nil {
			t.Errorf("RunFont.%s(clear) created w:rPr", name)
		}
	}

	formatSetters := map[string]func(*ParagraphFormat){
		"SetStyle":               func(f *ParagraphFormat) { f.SetStyle("") },
		"SetAlignment":           func(f *ParagraphFormat) { f.SetAlignment("") },
		"SetOutlineLevel":        func(f *ParagraphFormat) { f.SetOutlineLevel(nil) },
		"SetSpaceBefore":         func(f *ParagraphFormat) { f.SetSpaceBefore(nil) },
		"SetSpaceAfter":          func(f *ParagraphFormat) { f.SetSpaceAfter(nil) },
		"SetLeftIndent":          func(f *ParagraphFormat) { f.SetLeftIndent(nil) },
		"SetRightIndent":         func(f *ParagraphFormat) { f.SetRightIndent(nil) },
		"SetFirstLineIndent":     func(f *ParagraphFormat) { f.SetFirstLineIndent(nil) },
		"SetHangingIndent":       func(f *ParagraphFormat) { f.SetHangingIndent(nil) },
		"ClearLineSpacing":       func(f *ParagraphFormat) { f.ClearLineSpacing() },
		"SetKeepNext":            func(f *ParagraphFormat) { f.SetKeepNext(nil) },
		"SetKeepLines":           func(f *ParagraphFormat) { f.SetKeepLines(nil) },
		"SetPageBreakBefore":     func(f *ParagraphFormat) { f.SetPageBreakBefore(nil) },
		"SetContextualSpacing":   func(f *ParagraphFormat) { f.SetContextualSpacing(nil) },
		"SetWidowControl":        func(f *ParagraphFormat) { f.SetWidowControl(nil) },
		"SetSuppressLineNumbers": func(f *ParagraphFormat) { f.SetSuppressLineNumbers(nil) },
		"SetMirrorIndents":       func(f *ParagraphFormat) { f.SetMirrorIndents(nil) },
		"SetTabStops":            func(f *ParagraphFormat) { f.SetTabStops(nil) },
		"RemoveTabStop":          func(f *ParagraphFormat) { f.RemoveTabStop(720) },
		"ClearBorders":           func(f *ParagraphFormat) { f.ClearBorders() },
		"SetShading":             func(f *ParagraphFormat) { f.SetShading("") },
	}
	for name, set := range formatSetters {
		var ppr *wml.CT_PPr
		set(&ParagraphFormat{slot: &ppr})
		if ppr != nil {
			t.Errorf("ParagraphFormat.%s(clear) created w:pPr", name)
		}
	}
}

// The clearing path still clears an existing property.
func TestClearingSettersStillClear(t *testing.T) {
	var rpr *wml.CT_RPr
	f := &RunFont{slot: &rpr}
	f.SetColor("FF0000")
	f.SetColor("")
	if f.Color() != "" {
		t.Errorf("Color = %q after clearing", f.Color())
	}
	var ppr *wml.CT_PPr
	pf := &ParagraphFormat{slot: &ppr}
	pf.SetAlignment(AlignCenter)
	pf.SetAlignment("")
	if pf.Alignment() != "" {
		t.Errorf("Alignment = %q after clearing", pf.Alignment())
	}
}
