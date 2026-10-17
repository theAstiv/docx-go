package docx

import (
	"iter"
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// ── Option structs ────────────────────────────────────────────────────────

// TableOptions configures a new table created by Body.AddTable.
type TableOptions struct {
	// Style is the table style ID (e.g. "TableGrid", "PlainTable1").
	// Empty string leaves the table unstyled.
	Style string
	// ColWidth specifies the width of each column in twips.
	// If nil or shorter than cols, remaining columns get no explicit width.
	ColWidth []Twip
}

// RowOptions configures a new row created by Table.AddRow.
type RowOptions struct {
	// Header marks the row as a repeating table header row.
	Header bool
}

// CellOptions configures a new cell created by Row.AddCell.
type CellOptions struct {
	// Width sets the cell width in twips via w:tcW w:type="dxa".
	// Zero means no explicit width is set.
	Width Twip
	// ColSpan sets gridSpan (column span). Values ≤ 1 are treated as 1.
	ColSpan int
}

// ── VMergeState ──────────────────────────────────────────────────────────

// VMergeState describes the vertical merge state of a cell.
type VMergeState int

const (
	// VMergeNone means the cell has no vMerge attribute.
	VMergeNone VMergeState = iota
	// VMergeStart marks the first cell of a vertical merge group (w:vMerge w:val="restart").
	VMergeStart
	// VMergeContinue marks a continuation cell in a vertical merge group (w:vMerge with no val).
	VMergeContinue
)

// ── Table ─────────────────────────────────────────────────────────────────

// Table is a proxy for a w:tbl element.
type Table struct {
	tbl *wml.CT_Tbl
	st  *story
}

// Rows returns an iterator over all CT_Tr rows in the table (skipping raw passthrough elements).
func (t *Table) Rows() iter.Seq[*Row] {
	return func(yield func(*Row) bool) {
		for _, child := range t.tbl.Children {
			tr, ok := child.(*wml.CT_Tr)
			if !ok {
				continue
			}
			if !yield(&Row{tr: tr, st: t.st}) {
				return
			}
		}
	}
}

// AddRow appends a new empty row and returns its proxy.
func (t *Table) AddRow() *Row {
	return t.AddRowWithOptions(RowOptions{})
}

// AddRowWithOptions appends a new row with options and returns its proxy.
func (t *Table) AddRowWithOptions(opts RowOptions) *Row {
	tr := new(wml.CT_Tr)
	if opts.Header {
		b := true
		trpr := new(wml.CT_TrPr)
		trpr.SetHeader(&b)
		tr.TrPr = trpr
	}
	t.tbl.Children = append(t.tbl.Children, tr)
	return &Row{tr: tr, st: t.st}
}

// Style returns the table style ID, or "" if none.
func (t *Table) Style() string {
	if t.tbl.TblPr == nil {
		return ""
	}
	return t.tbl.TblPr.StyleID()
}

// SetStyle sets the table style ID. An empty string clears the style.
func (t *Table) SetStyle(id string) {
	if t.tbl.TblPr == nil {
		t.tbl.TblPr = new(wml.CT_TblPr)
	}
	t.tbl.TblPr.SetStyleID(id)
}

// ── Row ───────────────────────────────────────────────────────────────────

// Row is a proxy for a w:tr element.
type Row struct {
	tr *wml.CT_Tr
	st *story
}

// Cells returns an iterator over all CT_Tc cells in the row.
func (r *Row) Cells() iter.Seq[*Cell] {
	return func(yield func(*Cell) bool) {
		for _, child := range r.tr.Children {
			tc, ok := child.(*wml.CT_Tc)
			if !ok {
				continue
			}
			if !yield(&Cell{tc: tc, st: r.st}) {
				return
			}
		}
	}
}

// AddCell appends a new cell with an empty paragraph and returns its proxy.
func (r *Row) AddCell() *Cell {
	return r.AddCellWithOptions(CellOptions{})
}

// AddCellWithOptions appends a new cell with options and returns its proxy.
func (r *Row) AddCellWithOptions(opts CellOptions) *Cell {
	tc := new(wml.CT_Tc)
	// Cells always need at least one trailing paragraph.
	tc.Children = []wml.BlockElement{new(wml.CT_P)}
	if opts.ColSpan > 1 || opts.Width > 0 {
		tcpr := new(wml.CT_TcPr)
		if opts.ColSpan > 1 {
			n := int64(opts.ColSpan)
			tcpr.SetGridSpan(&n)
		}
		if opts.Width > 0 {
			tcpr.SetWidth(int64(opts.Width))
		}
		tc.TcPr = tcpr
	}
	r.tr.Children = append(r.tr.Children, tc)
	return &Cell{tc: tc, st: r.st}
}

// IsHeader returns whether this row is marked as a repeating table header.
func (r *Row) IsHeader() bool {
	if r.tr.TrPr == nil {
		return false
	}
	b := r.tr.TrPr.IsHeader()
	return b != nil && *b
}

// ── Cell ──────────────────────────────────────────────────────────────────

// Cell is a proxy for a w:tc element.
type Cell struct {
	tc *wml.CT_Tc
	st *story
}

// Paragraphs returns an iterator over paragraph children of the cell.
func (c *Cell) Paragraphs() iter.Seq[*Paragraph] {
	return func(yield func(*Paragraph) bool) {
		for _, child := range c.tc.Children {
			p, ok := child.(*wml.CT_P)
			if !ok {
				continue
			}
			if !yield(&Paragraph{p: p, st: c.st}) {
				return
			}
		}
	}
}

// Tables returns an iterator over nested table children of the cell.
func (c *Cell) Tables() iter.Seq[*Table] {
	return func(yield func(*Table) bool) {
		for _, child := range c.tc.Children {
			tbl, ok := child.(*wml.CT_Tbl)
			if !ok {
				continue
			}
			if !yield(&Table{tbl: tbl, st: c.st}) {
				return
			}
		}
	}
}

// AddParagraph appends a paragraph before the terminal paragraph and returns its proxy.
// OOXML requires a terminal w:p as the last cell child; this method inserts
// the new paragraph before any existing terminal paragraph.
func (c *Cell) AddParagraph() *Paragraph {
	p := new(wml.CT_P)
	// Insert before the last child if it is the terminal paragraph; otherwise append.
	n := len(c.tc.Children)
	if n > 0 {
		if _, ok := c.tc.Children[n-1].(*wml.CT_P); ok {
			// Splice before the terminal paragraph.
			c.tc.Children = append(c.tc.Children, nil)
			copy(c.tc.Children[n:], c.tc.Children[n-1:])
			c.tc.Children[n-1] = p
			return &Paragraph{p: p, st: c.st}
		}
	}
	c.tc.Children = append(c.tc.Children, p)
	return &Paragraph{p: p, st: c.st}
}

// SetSpan sets the column span (gridSpan) and optionally marks the cell as the
// start of a vertical merge group. cols ≤ 1 clears the gridSpan attribute.
// rows ≤ 1 does not alter the vMerge state.
func (c *Cell) SetSpan(rows, cols int) {
	if c.tc.TcPr == nil && (cols > 1 || rows > 1) {
		c.tc.TcPr = new(wml.CT_TcPr)
	}
	if cols > 1 {
		n := int64(cols)
		c.tc.TcPr.SetGridSpan(&n)
	} else if cols == 1 && c.tc.TcPr != nil {
		c.tc.TcPr.SetGridSpan(nil)
	}
	if rows > 1 {
		c.tc.TcPr.SetVMerge(wml.VMergeRestart)
	}
}

// SetVMerge sets the vertical merge state of this cell.
func (c *Cell) SetVMerge(state VMergeState) {
	switch state {
	case VMergeNone:
		if c.tc.TcPr != nil {
			c.tc.TcPr.ClearVMerge()
		}
	case VMergeStart:
		c.tcpr().SetVMerge(wml.VMergeRestart)
	case VMergeContinue:
		c.tcpr().SetVMerge("")
	}
}

// VMerge returns the vertical merge state of this cell.
func (c *Cell) VMerge() VMergeState {
	if c.tc.TcPr == nil {
		return VMergeNone
	}
	val, present := c.tc.TcPr.VMerge()
	if !present {
		return VMergeNone
	}
	if val == wml.VMergeRestart {
		return VMergeStart
	}
	return VMergeContinue
}

// ColSpan returns the column span (gridSpan) of this cell. Returns 1 if not set.
func (c *Cell) ColSpan() int {
	if c.tc.TcPr == nil {
		return 1
	}
	n := c.tc.TcPr.GridSpan()
	if n == nil {
		return 1
	}
	return int(*n)
}

// Text returns the concatenated text of all paragraphs in the cell joined
// by newlines. Trailing empty paragraphs (the structurally required terminal
// paragraph) are excluded so callers see only user-visible content.
func (c *Cell) Text() string {
	var parts []string
	for _, child := range c.tc.Children {
		if p, ok := child.(*wml.CT_P); ok {
			parts = append(parts, p.Text())
		}
	}
	for len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return strings.Join(parts, "\n")
}

// tcpr lazily creates the TcPr element when a setter needs it.
func (c *Cell) tcpr() *wml.CT_TcPr {
	if c.tc.TcPr == nil {
		c.tc.TcPr = new(wml.CT_TcPr)
	}
	return c.tc.TcPr
}

// ── Body.AddTable ─────────────────────────────────────────────────────────

// addTable is called from body.go to create and append a CT_Tbl.
func buildTable(rows, cols int, opts TableOptions) *wml.CT_Tbl {
	tbl := new(wml.CT_Tbl)
	// Table properties.
	if opts.Style != "" {
		tblPr := new(wml.CT_TblPr)
		tblPr.SetStyleID(opts.Style)
		tbl.TblPr = tblPr
	}
	// Column grid.
	if cols > 0 {
		grid := new(wml.CT_TblGrid)
		for i := range cols {
			col := new(wml.CT_GridCol)
			if i < len(opts.ColWidth) && opts.ColWidth[i] > 0 {
				w := int64(opts.ColWidth[i])
				col.W = &w
			}
			grid.Cols = append(grid.Cols, col)
		}
		tbl.TblGrid = grid
	}
	// Rows.
	for range rows {
		tr := new(wml.CT_Tr)
		for range cols {
			tc := new(wml.CT_Tc)
			tc.Children = []wml.BlockElement{new(wml.CT_P)}
			tr.Children = append(tr.Children, tc)
		}
		tbl.Children = append(tbl.Children, tr)
	}
	return tbl
}
