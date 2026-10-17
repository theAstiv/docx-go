package wml

import (
	"encoding/xml"
	"strconv"
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
)

// This file holds the value types and schema-ordered accessors for the table
// formatting properties of w:tblPr, w:tcPr and w:trPr. The property containers
// keep their children as raw elements (lossless round-trip); the accessors
// decode a child into a small value struct on read and replace the whole child
// at its xs:sequence position on write.

// tblPrChildSeqPos is the xs:sequence order of w:tblPr children.
var tblPrChildSeqPos = map[xml.Name]int{
	ns.Qn("w:tblStyle"):            1,
	ns.Qn("w:tblpPr"):              2,
	ns.Qn("w:tblOverlap"):          3,
	ns.Qn("w:bidiVisual"):          4,
	ns.Qn("w:tblStyleRowBandSize"): 5,
	ns.Qn("w:tblStyleColBandSize"): 6,
	ns.Qn("w:tblW"):                7,
	ns.Qn("w:jc"):                  8,
	ns.Qn("w:tblCellSpacing"):      9,
	ns.Qn("w:tblInd"):              10,
	ns.Qn("w:tblBorders"):          11,
	ns.Qn("w:shd"):                 12,
	ns.Qn("w:tblLayout"):           13,
	ns.Qn("w:tblCellMar"):          14,
	ns.Qn("w:tblLook"):             15,
	ns.Qn("w:tblCaption"):          16,
	ns.Qn("w:tblDescription"):      17,
	ns.Qn("w:tblPrChange"):         18,
}

// tcPrChildSeqPos is the xs:sequence order of w:tcPr children.
var tcPrChildSeqPos = map[xml.Name]int{
	ns.Qn("w:cnfStyle"):      1,
	ns.Qn("w:tcW"):           2,
	ns.Qn("w:gridSpan"):      3,
	ns.Qn("w:hMerge"):        4,
	ns.Qn("w:vMerge"):        5,
	ns.Qn("w:tcBorders"):     6,
	ns.Qn("w:shd"):           7,
	ns.Qn("w:noWrap"):        8,
	ns.Qn("w:tcMar"):         9,
	ns.Qn("w:textDirection"): 10,
	ns.Qn("w:tcFitText"):     11,
	ns.Qn("w:vAlign"):        12,
	ns.Qn("w:hideMark"):      13,
	ns.Qn("w:headers"):       14,
	ns.Qn("w:cellIns"):       15,
	ns.Qn("w:cellDel"):       16,
	ns.Qn("w:cellMerge"):     17,
	ns.Qn("w:tcPrChange"):    18,
}

// rawPutOrdered replaces any child named el.QName with el, placing it at its
// position in seq.
func rawPutOrdered(children *[]*raw.RawElement, el *raw.RawElement, seq map[xml.Name]int) {
	rawRemoveChild(children, el.QName)
	rawInsertOrdered(children, el, seq)
}

// rawFindChild returns the first child with the given name, or nil.
func rawFindChild(children []*raw.RawElement, qname xml.Name) *raw.RawElement {
	for _, c := range children {
		if c.QName == qname {
			return c
		}
	}
	return nil
}

// rawKids returns the element children of re.
func rawKids(re *raw.RawElement) []*raw.RawElement {
	var out []*raw.RawElement
	for _, c := range re.Children {
		if k, ok := c.(*raw.RawElement); ok {
			out = append(out, k)
		}
	}
	return out
}

func parseInt64Ptr(s string) *int64 {
	if s == "" {
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return &n
}

// ── Border ────────────────────────────────────────────────────────────────

// Border is one border edge (CT_Border). Sz is in eighths of a point and Space
// in points; both are nil when absent. Color is "auto" or a hex RRGGBB value.
type Border struct {
	Val   string
	Sz    *int64
	Space *int64
	Color string
}

// Borders is the set of edges of w:tblBorders or w:tcBorders. A nil edge is
// not written. TL2BR and TR2BL (diagonals) exist only on cells and are ignored
// for tables.
type Borders struct {
	Top, Left, Bottom, Right *Border
	InsideH, InsideV         *Border
	TL2BR, TR2BL             *Border
}

func borderToRaw(local string, b *Border) *raw.RawElement {
	val := b.Val
	if val == "" {
		val = "single"
	}
	color := b.Color
	if color == "" {
		color = "auto"
	}
	re := rawEl(local, "val", val)
	if b.Sz != nil {
		setAttrInSlice(&re.Attrs, ns.Qn("w:sz"), strconv.FormatInt(*b.Sz, 10))
	}
	if b.Space != nil {
		setAttrInSlice(&re.Attrs, ns.Qn("w:space"), strconv.FormatInt(*b.Space, 10))
	}
	setAttrInSlice(&re.Attrs, ns.Qn("w:color"), color)
	return re
}

func borderFromRaw(re *raw.RawElement) *Border {
	b := new(Border)
	b.Val = wAttr(re.Attrs, "val")
	b.Sz = parseInt64Ptr(wAttr(re.Attrs, "sz"))
	b.Space = parseInt64Ptr(wAttr(re.Attrs, "space"))
	b.Color = wAttr(re.Attrs, "color")
	return b
}

// bordersToRaw builds a w:tblBorders (cell=false) or w:tcBorders (cell=true).
func bordersToRaw(local string, b *Borders, cell bool) *raw.RawElement {
	parent := rawEl(local)
	add := func(name string, e *Border) {
		if e != nil {
			parent.Children = append(parent.Children, borderToRaw(name, e))
		}
	}
	add("top", b.Top)
	add("left", b.Left)
	add("bottom", b.Bottom)
	add("right", b.Right)
	add("insideH", b.InsideH)
	add("insideV", b.InsideV)
	if cell {
		add("tl2br", b.TL2BR)
		add("tr2bl", b.TR2BL)
	}
	return parent
}

func bordersFromRaw(re *raw.RawElement) *Borders {
	b := new(Borders)
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
		case "insideH":
			b.InsideH = borderFromRaw(k)
		case "insideV":
			b.InsideV = borderFromRaw(k)
		case "tl2br":
			b.TL2BR = borderFromRaw(k)
		case "tr2bl":
			b.TR2BL = borderFromRaw(k)
		}
	}
	return b
}

// ── Shading ───────────────────────────────────────────────────────────────

// Shading is w:shd. Val is the pattern ("clear" for a solid Fill), Color the
// pattern colour and Fill the background; colours are "auto" or hex RRGGBB.
type Shading struct {
	Val, Color, Fill string
}

func shadingToRaw(s *Shading) *raw.RawElement {
	val, color, fill := s.Val, s.Color, s.Fill
	if val == "" {
		val = "clear"
	}
	if color == "" {
		color = "auto"
	}
	if fill == "" {
		fill = "auto"
	}
	return rawEl("shd", "val", val, "color", color, "fill", fill)
}

func shadingFromRaw(re *raw.RawElement) *Shading {
	s := new(Shading)
	s.Val = wAttr(re.Attrs, "val")
	s.Color = wAttr(re.Attrs, "color")
	s.Fill = wAttr(re.Attrs, "fill")
	return s
}

// ── Widths ────────────────────────────────────────────────────────────────

// Table width types for TblWidth.Type.
const (
	WidthDxa  = "dxa"  // W is twips
	WidthPct  = "pct"  // W is fiftieths of a percent
	WidthAuto = "auto" // W is ignored
	WidthNil  = "nil"  // W is ignored
)

// TblWidth is CT_TblWidth (w:tblW, w:tcW): a measurement and its unit.
type TblWidth struct {
	Type string
	W    int64
}

func widthToRaw(local string, w *TblWidth) *raw.RawElement {
	typ := w.Type
	if typ == "" {
		typ = WidthDxa
	}
	v := w.W
	if typ == WidthAuto || typ == WidthNil {
		v = 0
	}
	return rawEl(local, "w", strconv.FormatInt(v, 10), "type", typ)
}

func widthFromRaw(re *raw.RawElement) *TblWidth {
	w := new(TblWidth)
	w.Type = wAttr(re.Attrs, "type")
	if w.Type == "" {
		w.Type = WidthDxa
	}
	s := wAttr(re.Attrs, "w")
	if num, ok := strings.CutSuffix(s, "%"); ok {
		// ST_MeasurementOrPercent allows "50%"; normalise to fiftieths.
		if f, err := strconv.ParseFloat(num, 64); err == nil {
			w.W = int64(f * 50)
			w.Type = WidthPct
		}
		return w
	}
	if n := parseInt64Ptr(s); n != nil {
		w.W = *n
	}
	return w
}

// ── Cell margins ──────────────────────────────────────────────────────────

// CellMargins is w:tblCellMar: default cell padding in twips. A nil side is
// not written.
type CellMargins struct {
	Top, Left, Bottom, Right *int64
}

func cellMarginsToRaw(local string, m *CellMargins) *raw.RawElement {
	parent := rawEl(local)
	add := func(name string, v *int64) {
		if v != nil {
			parent.Children = append(parent.Children,
				rawEl(name, "w", strconv.FormatInt(*v, 10), "type", WidthDxa))
		}
	}
	add("top", m.Top)
	add("left", m.Left)
	add("bottom", m.Bottom)
	add("right", m.Right)
	return parent
}

func cellMarginsFromRaw(re *raw.RawElement) *CellMargins {
	m := new(CellMargins)
	for _, k := range rawKids(re) {
		v := parseInt64Ptr(wAttr(k.Attrs, "w"))
		switch k.QName.Local {
		case "top":
			m.Top = v
		case "left", "start":
			m.Left = v
		case "bottom":
			m.Bottom = v
		case "right", "end":
			m.Right = v
		}
	}
	return m
}

// ── CT_TblPr accessors ────────────────────────────────────────────────────

// Width returns w:tblW, or nil when absent.
func (p *CT_TblPr) Width() *TblWidth {
	if re := rawFindChild(p.raw, ns.Qn("w:tblW")); re != nil {
		return widthFromRaw(re)
	}
	return nil
}

// SetWidth sets w:tblW. nil removes it.
func (p *CT_TblPr) SetWidth(w *TblWidth) {
	if w == nil {
		rawRemoveChild(&p.raw, ns.Qn("w:tblW"))
		return
	}
	rawPutOrdered(&p.raw, widthToRaw("tblW", w), tblPrChildSeqPos)
}

// Borders returns w:tblBorders, or nil when absent.
func (p *CT_TblPr) Borders() *Borders {
	if re := rawFindChild(p.raw, ns.Qn("w:tblBorders")); re != nil {
		return bordersFromRaw(re)
	}
	return nil
}

// SetBorders replaces w:tblBorders. nil removes it.
func (p *CT_TblPr) SetBorders(b *Borders) {
	if b == nil {
		rawRemoveChild(&p.raw, ns.Qn("w:tblBorders"))
		return
	}
	rawPutOrdered(&p.raw, bordersToRaw("tblBorders", b, false), tblPrChildSeqPos)
}

// Shading returns the table-level w:shd, or nil when absent.
func (p *CT_TblPr) Shading() *Shading {
	if re := rawFindChild(p.raw, ns.Qn("w:shd")); re != nil {
		return shadingFromRaw(re)
	}
	return nil
}

// SetShading sets the table-level w:shd. nil removes it.
func (p *CT_TblPr) SetShading(s *Shading) {
	if s == nil {
		rawRemoveChild(&p.raw, ns.Qn("w:shd"))
		return
	}
	rawPutOrdered(&p.raw, shadingToRaw(s), tblPrChildSeqPos)
}

// Layout returns w:tblLayout/@w:type ("fixed" or "autofit"), or "".
func (p *CT_TblPr) Layout() string {
	return rawGetAttrOnElement(p.raw, ns.Qn("w:tblLayout"), "type")
}

// SetLayout sets w:tblLayout. An empty string removes it.
func (p *CT_TblPr) SetLayout(layout string) {
	if layout == "" {
		rawRemoveChild(&p.raw, ns.Qn("w:tblLayout"))
		return
	}
	rawPutOrdered(&p.raw, rawEl("tblLayout", "type", layout), tblPrChildSeqPos)
}

// CellMargins returns w:tblCellMar, or nil when absent.
func (p *CT_TblPr) CellMargins() *CellMargins {
	if re := rawFindChild(p.raw, ns.Qn("w:tblCellMar")); re != nil {
		return cellMarginsFromRaw(re)
	}
	return nil
}

// SetCellMargins replaces w:tblCellMar. nil removes it.
func (p *CT_TblPr) SetCellMargins(m *CellMargins) {
	if m == nil {
		rawRemoveChild(&p.raw, ns.Qn("w:tblCellMar"))
		return
	}
	rawPutOrdered(&p.raw, cellMarginsToRaw("tblCellMar", m), tblPrChildSeqPos)
}

// ── CT_TcPr accessors ─────────────────────────────────────────────────────

// WidthSpec returns w:tcW, or nil when absent.
func (p *CT_TcPr) WidthSpec() *TblWidth {
	if re := rawFindChild(p.raw, ns.Qn("w:tcW")); re != nil {
		return widthFromRaw(re)
	}
	return nil
}

// SetWidthSpec sets w:tcW. nil removes it.
func (p *CT_TcPr) SetWidthSpec(w *TblWidth) {
	if w == nil {
		rawRemoveChild(&p.raw, ns.Qn("w:tcW"))
		return
	}
	rawPutOrdered(&p.raw, widthToRaw("tcW", w), tcPrChildSeqPos)
}

// Borders returns w:tcBorders, or nil when absent.
func (p *CT_TcPr) Borders() *Borders {
	if re := rawFindChild(p.raw, ns.Qn("w:tcBorders")); re != nil {
		return bordersFromRaw(re)
	}
	return nil
}

// SetBorders replaces w:tcBorders. nil removes it.
func (p *CT_TcPr) SetBorders(b *Borders) {
	if b == nil {
		rawRemoveChild(&p.raw, ns.Qn("w:tcBorders"))
		return
	}
	rawPutOrdered(&p.raw, bordersToRaw("tcBorders", b, true), tcPrChildSeqPos)
}

// Shading returns the cell w:shd, or nil when absent.
func (p *CT_TcPr) Shading() *Shading {
	if re := rawFindChild(p.raw, ns.Qn("w:shd")); re != nil {
		return shadingFromRaw(re)
	}
	return nil
}

// SetShading sets the cell w:shd. nil removes it.
func (p *CT_TcPr) SetShading(s *Shading) {
	if s == nil {
		rawRemoveChild(&p.raw, ns.Qn("w:shd"))
		return
	}
	rawPutOrdered(&p.raw, shadingToRaw(s), tcPrChildSeqPos)
}

// ── CT_TrPr accessors ─────────────────────────────────────────────────────

// CantSplit returns the w:cantSplit tri-state; nil when absent.
func (p *CT_TrPr) CantSplit() *bool { return rawGetOnOff(p.raw, ns.Qn("w:cantSplit")) }

// SetCantSplit sets w:cantSplit (keep the row on one page). nil removes it.
// CT_TrPrBase is an unordered choice, so position does not matter.
func (p *CT_TrPr) SetCantSplit(b *bool) { rawSetOnOff(&p.raw, ns.Qn("w:cantSplit"), b) }
