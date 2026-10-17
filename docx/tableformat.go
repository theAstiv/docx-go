package docx

import (
	"math"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// toInt32 narrows v, reporting false when it does not fit.
func toInt32(v int64) (int32, bool) {
	if v > math.MaxInt32 || v < math.MinInt32 {
		return 0, false
	}
	return int32(v), true
}

// ── Borders ───────────────────────────────────────────────────────────────

// BorderStyle is the line style of a border (ST_Border).
type BorderStyle string

// Common border styles. Other ST_Border values read from existing documents
// are preserved as-is.
const (
	BorderNone   BorderStyle = "nil"
	BorderSingle BorderStyle = "single"
	BorderThick  BorderStyle = "thick"
	BorderDouble BorderStyle = "double"
	BorderDotted BorderStyle = "dotted"
	BorderDashed BorderStyle = "dashed"
	BorderDotDot BorderStyle = "dotDash"
)

// Border describes one border edge.
type Border struct {
	// Style is the line style; "" means BorderSingle.
	Style BorderStyle
	// Width is the line width in eighths of a point (EighthPoint(4) = 0.5 pt).
	Width EighthPoint
	// Space is the distance between the border and the text, in points.
	Space int
	// Color is a hex RRGGBB value; "" means automatic.
	Color string
}

// BorderOptions selects the edges to set. A nil edge is left out, so setting
// options replaces the element's borders with exactly the edges given. TL2BR
// and TR2BL (diagonals) apply to cells only and are ignored for tables.
type BorderOptions struct {
	Top, Left, Bottom, Right *Border
	InsideH, InsideV         *Border
	TL2BR, TR2BL             *Border
}

func borderToWML(b *Border) *wml.Border {
	if b == nil {
		return nil
	}
	out := new(wml.Border)
	out.Val = string(b.Style)
	// A zero Width is left unset rather than written as sz="0", which Word
	// does not accept as a visible line.
	if b.Width > 0 {
		sz := int64(b.Width)
		out.Sz = &sz
	}
	space := int64(b.Space)
	out.Space = &space
	out.Color = b.Color
	return out
}

func borderFromWML(b *wml.Border) *Border {
	if b == nil {
		return nil
	}
	out := new(Border)
	out.Style = BorderStyle(b.Val)
	if b.Sz != nil {
		// A width outside int32 is nonsense in a document; leave Width unset.
		if v, ok := toInt32(*b.Sz); ok {
			out.Width = EighthPoint(v)
		}
	}
	if b.Space != nil {
		out.Space = int(*b.Space)
	}
	if b.Color != "auto" {
		out.Color = b.Color
	}
	return out
}

func bordersToWML(o BorderOptions) *wml.Borders {
	b := new(wml.Borders)
	b.Top, b.Left, b.Bottom, b.Right = borderToWML(o.Top), borderToWML(o.Left), borderToWML(o.Bottom), borderToWML(o.Right)
	b.InsideH, b.InsideV = borderToWML(o.InsideH), borderToWML(o.InsideV)
	b.TL2BR, b.TR2BL = borderToWML(o.TL2BR), borderToWML(o.TR2BL)
	return b
}

func bordersFromWML(b *wml.Borders) BorderOptions {
	var o BorderOptions
	if b == nil {
		return o
	}
	o.Top, o.Left, o.Bottom, o.Right = borderFromWML(b.Top), borderFromWML(b.Left), borderFromWML(b.Bottom), borderFromWML(b.Right)
	o.InsideH, o.InsideV = borderFromWML(b.InsideH), borderFromWML(b.InsideV)
	o.TL2BR, o.TR2BL = borderFromWML(b.TL2BR), borderFromWML(b.TR2BL)
	return o
}

// ── Widths ────────────────────────────────────────────────────────────────

// TableWidthKind says which unit a TableWidth is expressed in.
type TableWidthKind int

// Width kinds. The zero value is WidthKindAuto.
const (
	WidthKindAuto TableWidthKind = iota
	WidthKindTwips
	WidthKindPercent
)

// TableWidth is a table or cell width: automatic, an absolute size, or a
// percentage of the available width.
type TableWidth struct {
	kind  TableWidthKind
	twips Twip
	pct   FiftiethPercent
}

// WidthAuto sizes the table or cell from its content.
func WidthAuto() TableWidth { return TableWidth{} }

// WidthTwips is an absolute width.
func WidthTwips(t Twip) TableWidth { return TableWidth{kind: WidthKindTwips, twips: t} }

// WidthPercent is a percentage of the available width (Percent(50) is half).
func WidthPercent(p FiftiethPercent) TableWidth { return TableWidth{kind: WidthKindPercent, pct: p} }

// Kind reports the unit of the width.
func (w TableWidth) Kind() TableWidthKind { return w.kind }

// Twips returns the absolute width; 0 unless Kind is WidthKindTwips.
func (w TableWidth) Twips() Twip { return w.twips }

// Percent returns the relative width; 0 unless Kind is WidthKindPercent.
func (w TableWidth) Percent() FiftiethPercent { return w.pct }

func widthToWML(w TableWidth) *wml.TblWidth {
	out := new(wml.TblWidth)
	switch w.kind {
	case WidthKindTwips:
		out.Type, out.W = wml.WidthDxa, int64(w.twips)
	case WidthKindPercent:
		out.Type, out.W = wml.WidthPct, int64(w.pct)
	default:
		out.Type = wml.WidthAuto
	}
	return out
}

// widthFromWML converts a stored width; ok is false when none is set.
func widthFromWML(w *wml.TblWidth) (TableWidth, bool) {
	if w == nil {
		return TableWidth{}, false
	}
	switch w.Type {
	case wml.WidthDxa:
		return WidthTwips(Twip(w.W)), true
	case wml.WidthPct:
		v, ok := toInt32(w.W)
		if !ok {
			return TableWidth{}, false
		}
		return WidthPercent(FiftiethPercent(v)), true
	default:
		return WidthAuto(), true
	}
}

// ── Layout, margins, shading ──────────────────────────────────────────────

// TableLayout selects how column widths are computed.
type TableLayout string

// Table layouts.
const (
	LayoutAutofit TableLayout = "autofit"
	LayoutFixed   TableLayout = "fixed"
)

// CellMargins is default cell padding. A nil side is left unset.
type CellMargins struct {
	Top, Left, Bottom, Right *Twip
}

func marginsToWML(m CellMargins) *wml.CellMargins {
	out := new(wml.CellMargins)
	out.Top, out.Left, out.Bottom, out.Right = twipPtr(m.Top), twipPtr(m.Left), twipPtr(m.Bottom), twipPtr(m.Right)
	return out
}

// ── Table ─────────────────────────────────────────────────────────────────

func (t *Table) tblpr() *wml.CT_TblPr {
	if t.tbl.TblPr == nil {
		t.tbl.TblPr = new(wml.CT_TblPr)
	}
	return t.tbl.TblPr
}

// SetBorders replaces the table borders with the edges in o. Setting the zero
// BorderOptions writes an empty border set; use ClearBorders to remove it.
func (t *Table) SetBorders(o BorderOptions) { t.tblpr().SetBorders(bordersToWML(o)) }

// Borders returns the table borders; the zero value when none are set.
func (t *Table) Borders() BorderOptions {
	if t.tbl.TblPr == nil {
		return BorderOptions{}
	}
	return bordersFromWML(t.tbl.TblPr.Borders())
}

// ClearBorders removes the table border override, so the table style applies.
func (t *Table) ClearBorders() {
	if t.tbl.TblPr != nil {
		t.tbl.TblPr.SetBorders(nil)
	}
}

// SetWidth sets the preferred table width.
func (t *Table) SetWidth(w TableWidth) { t.tblpr().SetWidth(widthToWML(w)) }

// Width returns the preferred table width; ok is false when none is set.
func (t *Table) Width() (w TableWidth, ok bool) {
	if t.tbl.TblPr == nil {
		return TableWidth{}, false
	}
	return widthFromWML(t.tbl.TblPr.Width())
}

// SetCellMargins sets the default cell padding for the whole table.
func (t *Table) SetCellMargins(m CellMargins) { t.tblpr().SetCellMargins(marginsToWML(m)) }

// CellMargins returns the table's default cell padding; sides that are not
// set are nil.
func (t *Table) CellMargins() CellMargins {
	var m CellMargins
	if t.tbl.TblPr == nil {
		return m
	}
	if src := t.tbl.TblPr.CellMargins(); src != nil {
		m.Top, m.Left, m.Bottom, m.Right = twipFromInt64(src.Top), twipFromInt64(src.Left), twipFromInt64(src.Bottom), twipFromInt64(src.Right)
	}
	return m
}

// SetLayout selects fixed or autofit column sizing. An empty layout removes
// the override.
func (t *Table) SetLayout(l TableLayout) {
	if l == "" && t.tbl.TblPr == nil {
		return
	}
	t.tblpr().SetLayout(string(l))
}

// Layout returns the column sizing mode, or "" when unset.
func (t *Table) Layout() TableLayout {
	if t.tbl.TblPr == nil {
		return ""
	}
	return TableLayout(t.tbl.TblPr.Layout())
}

// ── Row ───────────────────────────────────────────────────────────────────

func (r *Row) trpr() *wml.CT_TrPr {
	if r.tr.TrPr == nil {
		r.tr.TrPr = new(wml.CT_TrPr)
	}
	return r.tr.TrPr
}

// SetHeader marks the row as a repeating header row (true) or clears the
// mark (false).
func (r *Row) SetHeader(header bool) {
	if !header {
		if r.tr.TrPr != nil {
			r.tr.TrPr.SetHeader(nil)
		}
		return
	}
	r.trpr().SetHeader(On())
}

// CantSplit returns the keep-row-on-one-page tri-state; nil when unset.
func (r *Row) CantSplit() *bool {
	if r.tr.TrPr == nil {
		return nil
	}
	return r.tr.TrPr.CantSplit()
}

// SetCantSplit keeps the row from breaking across pages. nil clears it.
func (r *Row) SetCantSplit(b *bool) {
	if b == nil && r.tr.TrPr == nil {
		return
	}
	r.trpr().SetCantSplit(b)
}

// ── Cell ──────────────────────────────────────────────────────────────────

// SetBorders replaces the cell borders with the edges in o (diagonals
// included).
func (c *Cell) SetBorders(o BorderOptions) { c.tcpr().SetBorders(bordersToWML(o)) }

// Borders returns the cell borders; the zero value when none are set.
func (c *Cell) Borders() BorderOptions {
	if c.tc.TcPr == nil {
		return BorderOptions{}
	}
	return bordersFromWML(c.tc.TcPr.Borders())
}

// SetWidth sets the preferred cell width.
func (c *Cell) SetWidth(w TableWidth) { c.tcpr().SetWidthSpec(widthToWML(w)) }

// Width returns the preferred cell width; ok is false when none is set.
func (c *Cell) Width() (w TableWidth, ok bool) {
	if c.tc.TcPr == nil {
		return TableWidth{}, false
	}
	return widthFromWML(c.tc.TcPr.WidthSpec())
}

// SetShading fills the cell background with a solid colour given as hex
// RRGGBB. An empty string removes the shading.
func (c *Cell) SetShading(fill string) {
	if fill == "" {
		if c.tc.TcPr != nil {
			c.tc.TcPr.SetShading(nil)
		}
		return
	}
	s := new(wml.Shading)
	s.Val, s.Color, s.Fill = "clear", "auto", fill
	c.tcpr().SetShading(s)
}

// Shading returns the cell fill colour as hex RRGGBB, or "" for none or
// automatic.
func (c *Cell) Shading() string {
	if c.tc.TcPr == nil {
		return ""
	}
	s := c.tc.TcPr.Shading()
	if s == nil || s.Fill == "auto" {
		return ""
	}
	return s.Fill
}
