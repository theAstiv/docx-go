package docx

import (
	"slices"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// ── Tab stops ─────────────────────────────────────────────────────────────

// TabAlign is how text aligns at a tab stop (w:tab/@w:val).
type TabAlign string

// Tab stop alignments. TabAlignClear is not a stop: it cancels a stop the
// paragraph would inherit from its style at the same position.
const (
	TabAlignLeft    TabAlign = wml.TabLeft
	TabAlignCenter  TabAlign = wml.TabCenter
	TabAlignRight   TabAlign = wml.TabRight
	TabAlignDecimal TabAlign = wml.TabDecimal
	TabAlignBar     TabAlign = wml.TabBar
	TabAlignClear   TabAlign = wml.TabClear
)

// TabLeader is the character that fills the space before a tab stop
// (w:tab/@w:leader).
type TabLeader string

// Tab leaders. The zero value writes no leader (blank space).
const (
	TabLeaderNone       TabLeader = ""
	TabLeaderDot        TabLeader = "dot"
	TabLeaderHyphen     TabLeader = "hyphen"
	TabLeaderUnderscore TabLeader = "underscore"
	TabLeaderHeavy      TabLeader = "heavy"
	TabLeaderMiddleDot  TabLeader = "middleDot"
)

// TabStop is one custom tab stop. Position is measured from the paragraph's
// left indent; Align "" means TabAlignLeft.
type TabStop struct {
	Position Twip
	Align    TabAlign
	Leader   TabLeader
}

// TabStops returns the paragraph's own tab stops in position order,
// including TabAlignClear entries; nil when it has none. Stops inherited
// from the style are not included.
func (f *ParagraphFormat) TabStops() []TabStop {
	if f.cur() == nil {
		return nil
	}
	var out []TabStop
	for _, t := range f.cur().Tabs() {
		out = append(out, TabStop{Position: Twip(t.Pos), Align: TabAlign(t.Val), Leader: TabLeader(t.Leader)})
	}
	return out
}

// SetTabStops replaces the paragraph's tab stops; they are written sorted by
// position. nil or an empty slice removes them all.
func (f *ParagraphFormat) SetTabStops(stops []TabStop) {
	if len(stops) == 0 && f.cur() == nil {
		return
	}
	sorted := slices.Clone(stops)
	slices.SortStableFunc(sorted, func(a, b TabStop) int {
		switch {
		case a.Position < b.Position:
			return -1
		case a.Position > b.Position:
			return 1
		}
		return 0
	})
	tabs := make([]wml.Tab, 0, len(sorted))
	for _, s := range sorted {
		tabs = append(tabs, wml.Tab{Val: string(s.Align), Pos: int64(s.Position), Leader: string(s.Leader)})
	}
	f.ppr().SetTabs(tabs)
}

// AddTabStop adds a tab stop, replacing any of the paragraph's own stops at
// the same position.
func (f *ParagraphFormat) AddTabStop(s TabStop) {
	stops := slices.DeleteFunc(f.TabStops(), func(t TabStop) bool { return t.Position == s.Position })
	f.SetTabStops(append(stops, s))
}

// RemoveTabStop removes the paragraph's own tab stop at pos, if any. A stop
// the style defines at that position applies again; use ClearTabStop to
// cancel it.
func (f *ParagraphFormat) RemoveTabStop(pos Twip) {
	stops := f.TabStops()
	kept := slices.DeleteFunc(slices.Clone(stops), func(t TabStop) bool { return t.Position == pos })
	if len(kept) != len(stops) {
		f.SetTabStops(kept)
	}
}

// ClearTabStop cancels the tab stop at pos for this paragraph, including one
// inherited from its style, by writing a TabAlignClear entry there.
func (f *ParagraphFormat) ClearTabStop(pos Twip) {
	f.AddTabStop(TabStop{Position: pos, Align: TabAlignClear, Leader: TabLeaderNone})
}

// ── Borders ───────────────────────────────────────────────────────────────

// ParagraphBorderOptions selects the edges of a paragraph border. Between is
// drawn between consecutive paragraphs that have the same borders (otherwise
// they share one box); Bar is a vertical line beside the paragraph. A nil
// edge is left out.
type ParagraphBorderOptions struct {
	Top, Left, Bottom, Right *Border
	Between, Bar             *Border
}

// SetBorders replaces the paragraph's borders with the edges in o. Setting
// the zero ParagraphBorderOptions writes an empty border set, which
// overrides borders from the style; use ClearBorders to inherit them.
func (f *ParagraphFormat) SetBorders(o ParagraphBorderOptions) {
	b := new(wml.ParagraphBorders)
	b.Top, b.Left, b.Bottom, b.Right = borderToWML(o.Top), borderToWML(o.Left), borderToWML(o.Bottom), borderToWML(o.Right)
	b.Between, b.Bar = borderToWML(o.Between), borderToWML(o.Bar)
	f.ppr().SetBorders(b)
}

// Borders returns the paragraph's own borders; the zero value when none are
// set.
func (f *ParagraphFormat) Borders() ParagraphBorderOptions {
	var o ParagraphBorderOptions
	if f.cur() == nil {
		return o
	}
	b := f.cur().Borders()
	if b == nil {
		return o
	}
	o.Top, o.Left, o.Bottom, o.Right = borderFromWML(b.Top), borderFromWML(b.Left), borderFromWML(b.Bottom), borderFromWML(b.Right)
	o.Between, o.Bar = borderFromWML(b.Between), borderFromWML(b.Bar)
	return o
}

// ClearBorders removes the paragraph's own borders, so the style's apply.
func (f *ParagraphFormat) ClearBorders() {
	if f.cur() != nil {
		f.cur().SetBorders(nil)
	}
}

// ── Shading ───────────────────────────────────────────────────────────────

// SetShading fills the paragraph background with a solid colour given as
// hex RRGGBB. An empty string removes the shading.
func (f *ParagraphFormat) SetShading(fill string) {
	if fill == "" {
		if f.cur() != nil {
			f.cur().SetShading(nil)
		}
		return
	}
	s := new(wml.Shading)
	s.Val, s.Color, s.Fill = "clear", "auto", fill
	f.ppr().SetShading(s)
}

// Shading returns the paragraph fill colour as hex RRGGBB, or "" for none
// or automatic.
func (f *ParagraphFormat) Shading() string {
	if f.cur() == nil {
		return ""
	}
	s := f.cur().Shading()
	if s == nil || s.Fill == "auto" {
		return ""
	}
	return s.Fill
}
