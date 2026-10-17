package wml

import (
	"encoding/xml"
	"fmt"
	"strconv"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
	"github.com/theAstiv/docx-go/internal/oxml/registry"
)

// ── Row-level element interface ───────────────────────────────────────────

// TableRowElement is implemented by CT_Tr and RawTableRow.
// It covers everything that can appear between a w:tblPr/w:tblGrid and
// the closing </w:tbl> — in practice rows and passthrough bookmarks.
type TableRowElement interface {
	MarshalXML(e *xml.Encoder, start xml.StartElement) error
	UnmarshalXML(d *xml.Decoder, start xml.StartElement) error
	isTableRow()
}

// RawTableRow captures an unknown element between table rows.
type RawTableRow struct{ E *raw.RawElement }

func (*RawTableRow) isTableRow() {}

// MarshalXML delegates to the wrapped RawElement.
func (r *RawTableRow) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if r.E == nil {
		return nil
	}
	return r.E.MarshalXML(e, start)
}

// UnmarshalXML captures the element into the wrapped RawElement.
func (r *RawTableRow) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	r.E = new(raw.RawElement)
	return r.E.UnmarshalXML(d, start)
}

// ── Cell-level element interface ──────────────────────────────────────────

// TableCellElement is implemented by CT_Tc and RawTableCell.
type TableCellElement interface {
	MarshalXML(e *xml.Encoder, start xml.StartElement) error
	UnmarshalXML(d *xml.Decoder, start xml.StartElement) error
	isTableCell()
}

// RawTableCell captures an unknown element inside a table row.
type RawTableCell struct{ E *raw.RawElement }

func (*RawTableCell) isTableCell() {}

// MarshalXML delegates to the wrapped RawElement.
func (r *RawTableCell) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if r.E == nil {
		return nil
	}
	return r.E.MarshalXML(e, start)
}

// UnmarshalXML captures the element into the wrapped RawElement.
func (r *RawTableCell) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	r.E = new(raw.RawElement)
	return r.E.UnmarshalXML(d, start)
}

// ── CT_Tbl ────────────────────────────────────────────────────────────────

// CT_Tbl is the w:tbl element.
//
// Children holds rows (CT_Tr) and passthrough elements (RawTableRow) in
// document order, preserving the full OOXML row sequence.
type CT_Tbl struct {
	TblPr    *CT_TblPr
	TblGrid  *CT_TblGrid
	Children []TableRowElement
}

func (*CT_Tbl) isBlock() {}

// MarshalXML emits <w:tbl>.
func (t *CT_Tbl) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:tbl")
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("wml: CT_Tbl start: %w", err)
	}
	if t.TblPr != nil {
		if err := t.TblPr.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	if t.TblGrid != nil {
		if err := t.TblGrid.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	for _, child := range t.Children {
		if err := child.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML reads a w:tbl element.
func (t *CT_Tbl) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	t.TblPr = nil
	t.TblGrid = nil
	t.Children = t.Children[:0]
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("wml: CT_Tbl: %w", err)
		}
		switch te := tok.(type) {
		case xml.StartElement:
			switch te.Name {
			case ns.Qn("w:tblPr"):
				t.TblPr = new(CT_TblPr)
				if err := t.TblPr.UnmarshalXML(d, te); err != nil {
					return err
				}
			case ns.Qn("w:tblGrid"):
				t.TblGrid = new(CT_TblGrid)
				if err := t.TblGrid.UnmarshalXML(d, te); err != nil {
					return err
				}
			case ns.Qn("w:tr"):
				row := new(CT_Tr)
				if err := row.UnmarshalXML(d, te); err != nil {
					return err
				}
				t.Children = append(t.Children, row)
			default:
				rr := new(RawTableRow)
				if err := rr.UnmarshalXML(d, te); err != nil {
					return err
				}
				t.Children = append(t.Children, rr)
			}
		case xml.EndElement:
			return nil
		}
	}
}

// ── CT_TblPr ─────────────────────────────────────────────────────────────

// CT_TblPr is the w:tblPr element: table properties.
//
// All children are preserved in raw for order-safe round-tripping.
// Typed accessors expose the most commonly modified properties.
type CT_TblPr struct {
	raw []*raw.RawElement
}

// StyleID returns the w:tblStyle val, or "" if absent.
func (p *CT_TblPr) StyleID() string { return rawGetStr(p.raw, ns.Qn("w:tblStyle")) }

// SetStyleID sets or clears the table style.
func (p *CT_TblPr) SetStyleID(s string) {
	if s == "" {
		rawRemoveChild(&p.raw, ns.Qn("w:tblStyle"))
		return
	}
	rawPutOrdered(&p.raw, rawEl("tblStyle", "val", s), tblPrChildSeqPos)
}

// MarshalXML emits <w:tblPr>.
func (p *CT_TblPr) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:tblPr")
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("wml: CT_TblPr start: %w", err)
	}
	if err := rawMarshalChildren(e, p.raw); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML reads a w:tblPr element.
func (p *CT_TblPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	var err error
	p.raw, err = rawUnmarshalChildren(d)
	return err
}

// ── CT_TblGrid ────────────────────────────────────────────────────────────

// CT_TblGrid is the w:tblGrid element: column width definitions.
type CT_TblGrid struct {
	Cols []*CT_GridCol
	// raw captures unknown children (e.g. w:tblGridChange for revision tracking).
	raw []*raw.RawElement
}

// MarshalXML emits <w:tblGrid>.
func (g *CT_TblGrid) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:tblGrid")
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("wml: CT_TblGrid start: %w", err)
	}
	for _, col := range g.Cols {
		if err := col.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	if err := rawMarshalChildren(e, g.raw); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML reads a w:tblGrid element.
func (g *CT_TblGrid) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	g.Cols = g.Cols[:0]
	g.raw = nil
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("wml: CT_TblGrid: %w", err)
		}
		switch te := tok.(type) {
		case xml.StartElement:
			if te.Name == ns.Qn("w:gridCol") {
				col := new(CT_GridCol)
				if err := col.UnmarshalXML(d, te); err != nil {
					return err
				}
				g.Cols = append(g.Cols, col)
			} else {
				re := new(raw.RawElement)
				if err := re.UnmarshalXML(d, te); err != nil {
					return err
				}
				g.raw = append(g.raw, re)
			}
		case xml.EndElement:
			return nil
		}
	}
}

// ── CT_GridCol ────────────────────────────────────────────────────────────

// CT_GridCol is the w:gridCol element: a single column width in twips.
type CT_GridCol struct {
	W *int64 // nil = absent (column width determined by table layout)
}

// MarshalXML emits <w:gridCol>.
func (c *CT_GridCol) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:gridCol")
	if c.W != nil {
		start.Attr = append(start.Attr, xml.Attr{
			Name:  ns.Qn("w:w"),
			Value: strconv.FormatInt(*c.W, 10),
		})
	}
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("wml: CT_GridCol start: %w", err)
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML reads a w:gridCol element.
func (c *CT_GridCol) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	s := wAttr(start.Attr, "w")
	if s != "" {
		n, err := strconv.ParseInt(s, 10, 64)
		if err == nil {
			c.W = &n
		}
	}
	return d.Skip()
}

// ── CT_Tr ─────────────────────────────────────────────────────────────────

// CT_Tr is the w:tr element: a table row.
//
// Children holds cells (CT_Tc) and passthrough elements (RawTableCell) in
// document order. Attrs preserves revision-tracking and other w:tr attributes.
type CT_Tr struct {
	TrPr     *CT_TrPr
	Children []TableCellElement
	// Attrs holds non-xmlns attributes declared on the w:tr element itself
	// (e.g. w:rsidR, w:rsidTr) for round-trip fidelity.
	Attrs []xml.Attr
}

func (*CT_Tr) isTableRow() {}

// MarshalXML emits <w:tr>.
func (r *CT_Tr) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:tr")
	start.Attr = append(start.Attr, r.Attrs...)
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("wml: CT_Tr start: %w", err)
	}
	if r.TrPr != nil {
		if err := r.TrPr.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	for _, cell := range r.Children {
		if err := cell.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML reads a w:tr element.
func (r *CT_Tr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	r.TrPr = nil
	r.Children = r.Children[:0]
	r.Attrs = r.Attrs[:0]
	for _, a := range start.Attr {
		if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
			continue
		}
		r.Attrs = append(r.Attrs, a)
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("wml: CT_Tr: %w", err)
		}
		switch te := tok.(type) {
		case xml.StartElement:
			switch te.Name {
			case ns.Qn("w:trPr"):
				r.TrPr = new(CT_TrPr)
				if err := r.TrPr.UnmarshalXML(d, te); err != nil {
					return err
				}
			case ns.Qn("w:tc"):
				cell := new(CT_Tc)
				if err := cell.UnmarshalXML(d, te); err != nil {
					return err
				}
				r.Children = append(r.Children, cell)
			default:
				rc := new(RawTableCell)
				if err := rc.UnmarshalXML(d, te); err != nil {
					return err
				}
				r.Children = append(r.Children, rc)
			}
		case xml.EndElement:
			return nil
		}
	}
}

// ── CT_TrPr ───────────────────────────────────────────────────────────────

// CT_TrPr is the w:trPr element: table row properties.
//
// All children are preserved in raw. IsHeader/SetHeader expose the
// w:tblHeader flag; everything else is passed through.
type CT_TrPr struct {
	raw []*raw.RawElement
}

// IsHeader returns whether the row is a table header (w:tblHeader), or nil if absent.
func (p *CT_TrPr) IsHeader() *bool { return rawGetOnOff(p.raw, ns.Qn("w:tblHeader")) }

// SetHeader marks or unmarks the row as a repeating table header.
func (p *CT_TrPr) SetHeader(b *bool) { rawSetOnOff(&p.raw, ns.Qn("w:tblHeader"), b) }

// MarshalXML emits <w:trPr>.
func (p *CT_TrPr) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:trPr")
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("wml: CT_TrPr start: %w", err)
	}
	if err := rawMarshalChildren(e, p.raw); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML reads a w:trPr element.
func (p *CT_TrPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	var err error
	p.raw, err = rawUnmarshalChildren(d)
	return err
}

// ── CT_Tc ─────────────────────────────────────────────────────────────────

// CT_Tc is the w:tc element: a table cell.
//
// Children holds block-level content (CT_P, CT_Tbl, RawBlock) in document
// order. The OOXML schema requires at least one w:p as the terminal child;
// callers must ensure this invariant.
type CT_Tc struct {
	TcPr     *CT_TcPr
	Children []BlockElement
}

func (*CT_Tc) isTableCell() {}

// MarshalXML emits <w:tc>.
func (c *CT_Tc) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:tc")
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("wml: CT_Tc start: %w", err)
	}
	if c.TcPr != nil {
		if err := c.TcPr.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	for _, child := range c.Children {
		if err := child.MarshalXML(e, xml.StartElement{}); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML reads a w:tc element.
func (c *CT_Tc) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	c.TcPr = nil
	c.Children = c.Children[:0]
	for {
		tok, err := d.Token()
		if err != nil {
			return fmt.Errorf("wml: CT_Tc: %w", err)
		}
		switch te := tok.(type) {
		case xml.StartElement:
			switch te.Name {
			case ns.Qn("w:tcPr"):
				c.TcPr = new(CT_TcPr)
				if err := c.TcPr.UnmarshalXML(d, te); err != nil {
					return err
				}
			case ns.Qn("w:p"):
				p := new(CT_P)
				if err := p.UnmarshalXML(d, te); err != nil {
					return err
				}
				c.Children = append(c.Children, p)
			case ns.Qn("w:tbl"):
				tbl := new(CT_Tbl)
				if err := tbl.UnmarshalXML(d, te); err != nil {
					return err
				}
				c.Children = append(c.Children, tbl)
			default:
				rb := new(RawBlock)
				if err := rb.UnmarshalXML(d, te); err != nil {
					return err
				}
				c.Children = append(c.Children, rb)
			}
		case xml.EndElement:
			return nil
		}
	}
}

// ── CT_TcPr ───────────────────────────────────────────────────────────────

// CT_TcPr is the w:tcPr element: table cell properties.
//
// Typed accessors expose gridSpan and vMerge (the two properties most
// commonly manipulated programmatically). Borders, shading, width, and
// other children are preserved in raw.
type CT_TcPr struct {
	raw []*raw.RawElement
}

// GridSpan returns the column span count from w:gridSpan, or nil if absent.
func (p *CT_TcPr) GridSpan() *int64 {
	return rawGetInt64Attr(p.raw, ns.Qn("w:gridSpan"), "val")
}

// SetGridSpan sets or clears the column span (w:gridSpan).
func (p *CT_TcPr) SetGridSpan(n *int64) {
	if n == nil {
		rawRemoveChild(&p.raw, ns.Qn("w:gridSpan"))
		return
	}
	rawPutOrdered(&p.raw, rawEl("gridSpan", "val", strconv.FormatInt(*n, 10)), tcPrChildSeqPos)
}

// VMergeRestart is the w:val for the first cell of a vertical merge group.
const VMergeRestart = "restart"

// VMerge returns the vMerge state: (VMergeRestart, true) for the start of
// a merge group, ("", true) for a continuation cell, ("", false) if absent.
func (p *CT_TcPr) VMerge() (val string, present bool) {
	for _, child := range p.raw {
		if child.QName == ns.Qn("w:vMerge") {
			return wAttr(child.Attrs, "val"), true
		}
	}
	return "", false
}

// SetVMerge sets the vMerge element. Use VMergeRestart to start a merge
// group; use "" to mark a continuation cell.
func (p *CT_TcPr) SetVMerge(val string) {
	for _, child := range p.raw {
		if child.QName == ns.Qn("w:vMerge") {
			if val == VMergeRestart {
				setAttrInSlice(&child.Attrs, ns.Qn("w:val"), val)
			} else {
				removeAttrFromSlice(&child.Attrs, "val")
			}
			return
		}
	}
	re := new(raw.RawElement)
	re.QName = ns.Qn("w:vMerge")
	if val == VMergeRestart {
		re.Attrs = []xml.Attr{{Name: ns.Qn("w:val"), Value: val}}
	}
	rawInsertOrdered(&p.raw, re, tcPrChildSeqPos)
}

// ClearVMerge removes the w:vMerge element.
func (p *CT_TcPr) ClearVMerge() { rawRemoveChild(&p.raw, ns.Qn("w:vMerge")) }

// SetWidth sets the cell width as a dxa (twips) measurement via w:tcW.
func (p *CT_TcPr) SetWidth(twips int64) {
	p.SetWidthSpec(&TblWidth{Type: WidthDxa, W: twips})
}

// Width returns the w:tcW magnitude (twips for dxa), or 0 if absent. Use
// WidthSpec when the unit matters.
func (p *CT_TcPr) Width() int64 {
	if w := p.WidthSpec(); w != nil {
		return w.W
	}
	return 0
}

// MarshalXML emits <w:tcPr>.
func (p *CT_TcPr) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = ns.Qn("w:tcPr")
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("wml: CT_TcPr start: %w", err)
	}
	if err := rawMarshalChildren(e, p.raw); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML reads a w:tcPr element.
func (p *CT_TcPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	var err error
	p.raw, err = rawUnmarshalChildren(d)
	return err
}

// ── Registry ──────────────────────────────────────────────────────────────

func init() {
	registry.Register(ns.Qn("w:tbl"), func() registry.Element { return new(CT_Tbl) })
	registry.Register(ns.Qn("w:tblPr"), func() registry.Element { return new(CT_TblPr) })
	registry.Register(ns.Qn("w:tblGrid"), func() registry.Element { return new(CT_TblGrid) })
	registry.Register(ns.Qn("w:gridCol"), func() registry.Element { return new(CT_GridCol) })
	registry.Register(ns.Qn("w:tr"), func() registry.Element { return new(CT_Tr) })
	registry.Register(ns.Qn("w:trPr"), func() registry.Element { return new(CT_TrPr) })
	registry.Register(ns.Qn("w:tc"), func() registry.Element { return new(CT_Tc) })
	registry.Register(ns.Qn("w:tcPr"), func() registry.Element { return new(CT_TcPr) })
}
