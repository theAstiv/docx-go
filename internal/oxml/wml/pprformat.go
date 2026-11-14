package wml

import (
	"strconv"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
)

// Paragraph borders, shading and tab stops (w:pBdr, w:shd, w:tabs). Like the
// other w:pPr properties they are stored as raw children; these accessors
// read and write them, and pPrChildSeqPos puts them in schema order.

// ── Borders ───────────────────────────────────────────────────────────────

// ParagraphBorders is w:pBdr (CT_PBdr). Between is drawn between paragraphs
// that share the same border settings; Bar is a vertical line beside the
// paragraph. A nil edge is not written.
type ParagraphBorders struct {
	Top, Left, Bottom, Right *Border
	Between, Bar             *Border
}

// Borders returns the paragraph borders, or nil when w:pBdr is absent. The
// ISO 29500 names w:start / w:end are read as Left / Right.
func (p *CT_PPr) Borders() *ParagraphBorders {
	re := rawFindChild(p.raw, ns.Qn("w:pBdr"))
	if re == nil {
		return nil
	}
	b := new(ParagraphBorders)
	for _, k := range rawKids(re) {
		switch k.QName.Local {
		case "top":
			b.Top = borderFromRaw(k)
		case "left", "start":
			b.Left = borderFromRaw(k)
		case "bottom":
			b.Bottom = borderFromRaw(k)
		case "right", "end":
			b.Right = borderFromRaw(k)
		case "between":
			b.Between = borderFromRaw(k)
		case "bar":
			b.Bar = borderFromRaw(k)
		}
	}
	return b
}

// SetBorders replaces w:pBdr with the edges in b, in schema order (top,
// left, bottom, right, between, bar). nil removes it.
func (p *CT_PPr) SetBorders(b *ParagraphBorders) {
	if b == nil {
		rawRemoveChild(&p.raw, ns.Qn("w:pBdr"))
		return
	}
	parent := rawEl("pBdr")
	add := func(name string, e *Border) {
		if e != nil {
			parent.Children = append(parent.Children, borderToRaw(name, e))
		}
	}
	add("top", b.Top)
	add("left", b.Left)
	add("bottom", b.Bottom)
	add("right", b.Right)
	add("between", b.Between)
	add("bar", b.Bar)
	rawPutOrdered(&p.raw, parent, pPrChildSeqPos)
}

// ── Shading ───────────────────────────────────────────────────────────────

// Shading returns the paragraph w:shd, or nil when absent.
func (p *CT_PPr) Shading() *Shading {
	if re := rawFindChild(p.raw, ns.Qn("w:shd")); re != nil {
		return shadingFromRaw(re)
	}
	return nil
}

// SetShading sets the paragraph w:shd. nil removes it.
func (p *CT_PPr) SetShading(s *Shading) {
	if s == nil {
		rawRemoveChild(&p.raw, ns.Qn("w:shd"))
		return
	}
	rawPutOrdered(&p.raw, shadingToRaw(s), pPrChildSeqPos)
}

// ── Tabs ──────────────────────────────────────────────────────────────────

// Tab stop alignments (ST_TabJc). TabClear removes a tab stop the paragraph
// would otherwise inherit from its style at the same position.
const (
	TabLeft    = "left"
	TabCenter  = "center"
	TabRight   = "right"
	TabDecimal = "decimal"
	TabBar     = "bar"
	TabNum     = "num"
	TabClear   = "clear"
)

// Tab is one w:tabs/w:tab. Pos is in twips from the paragraph's leading
// indent (it may be negative); Leader is "" when absent.
type Tab struct {
	Val    string
	Pos    int64
	Leader string
}

// Tabs returns the paragraph's tab stops in document order, or nil when
// w:tabs is absent. The ISO 29500 values start / end are read as left /
// right. A stop with an unreadable position is skipped.
func (p *CT_PPr) Tabs() []Tab {
	re := rawFindChild(p.raw, ns.Qn("w:tabs"))
	if re == nil {
		return nil
	}
	var out []Tab
	for _, k := range rawKids(re) {
		if k.QName != ns.Qn("w:tab") {
			continue
		}
		pos := parseInt64Ptr(wAttr(k.Attrs, "pos"))
		if pos == nil {
			continue
		}
		val := wAttr(k.Attrs, "val")
		switch val {
		case "start":
			val = TabLeft
		case "end":
			val = TabRight
		}
		out = append(out, Tab{Val: val, Pos: *pos, Leader: wAttr(k.Attrs, "leader")})
	}
	return out
}

// SetTabs replaces w:tabs with tabs, in the order given. An empty slice
// removes w:tabs: the schema requires at least one w:tab inside it. Val
// defaults to left.
func (p *CT_PPr) SetTabs(tabs []Tab) {
	if len(tabs) == 0 {
		rawRemoveChild(&p.raw, ns.Qn("w:tabs"))
		return
	}
	parent := rawEl("tabs")
	for _, t := range tabs {
		val := t.Val
		if val == "" {
			val = TabLeft
		}
		el := rawEl("tab", "val", val)
		if t.Leader != "" {
			setAttrInSlice(&el.Attrs, ns.Qn("w:leader"), t.Leader)
		}
		setAttrInSlice(&el.Attrs, ns.Qn("w:pos"), strconv.FormatInt(t.Pos, 10))
		parent.Children = append(parent.Children, el)
	}
	rawPutOrdered(&p.raw, parent, pPrChildSeqPos)
}
