package docx

import "github.com/theAstiv/docx-go/internal/oxml/wml"

// A table created without column widths gets them the way Word sizes an
// inserted table: the text width of the place holding it, shared evenly by
// the columns that have no width. Without widths Word lays out a table of
// empty cells as a sliver (CLAUDE.md C17).

const (
	// cellMargins is Word's default left plus right cell margin (2 × 0.075").
	cellMargins Twip = 2 * 108
	// minColumnShare is the narrowest share a missing column gets from the
	// width left over (0.5"); below it the columns share the whole width.
	minColumnShare Twip = 720
)

// defaultSection is the page New() writes (wml.DefaultSectPr); its values
// fill in what a section leaves out.
var defaultSection = wml.DefaultSectPr()

// defaultTextWidth is the text width of defaultSection.
var defaultTextWidth = Twip(*defaultSection.PgSzW() - *defaultSection.PgMarLeft() - *defaultSection.PgMarRight())

// sectionTextWidth returns the width of one text column of a section: the
// page width less the left, right and gutter margins, divided among its
// text columns (w:cols). Missing page values come from DefaultSectPr.
func sectionTextWidth(sp *wml.CT_SectPr) Twip {
	w, left, right := *defaultSection.PgSzW(), *defaultSection.PgMarLeft(), *defaultSection.PgMarRight()
	var gutter int64
	num, space, colWidths := 1, int64(720), []int64(nil)
	if sp != nil {
		if v := sp.PgSzW(); v != nil {
			w = *v
		}
		if v := sp.PgMarLeft(); v != nil {
			left = *v
		}
		if v := sp.PgMarRight(); v != nil {
			right = *v
		}
		if v := sp.PgMarGutter(); v != nil && *v > 0 {
			gutter = *v
		}
		num, space, colWidths = sp.TextColumns()
	}
	text := w - left - right - gutter
	switch {
	case len(colWidths) > 0 && colWidths[0] > 0:
		// Unequal columns: Word sizes a table to the column it is in; the
		// first is the best guess without layout.
		text = colWidths[0]
	case num > 1:
		text = (text - space*int64(num-1)) / int64(num)
	}
	if text > 0 {
		return Twip(text)
	}
	return defaultTextWidth
}

// textWidthAt returns the width a table inserted into list at index i may
// fill:
//   - in the body, the text column of the section it falls in (the next
//     paragraph-level section break at or after i, else the body's final
//     section);
//   - in a table cell, the cell's width less the default cell margins: its
//     w:tcW when that is fixed (dxa), else the width of the grid columns it
//     spans;
//   - in a header or footer, the first section that uses that part;
//   - anywhere else, the body's final section.
func (s *story) textWidthAt(list *[]wml.BlockElement, i int) Twip {
	body := s.body()
	final := defaultTextWidth
	if body != nil {
		final = sectionTextWidth(body.SectPr)
	}
	if list == nil {
		return final
	}
	if s.isBody(list) {
		for _, b := range (*list)[min(max(i, 0), len(*list)):] {
			if p, ok := b.(*wml.CT_P); ok && p.PPr != nil && p.PPr.SectPr != nil {
				return sectionTextWidth(p.PPr.SectPr)
			}
		}
		return final
	}
	if loc := s.cellHolding(list); loc != nil {
		if w := loc.width(); w > cellMargins {
			return w - cellMargins
		}
		return final
	}
	if sp := s.sectionUsingPart(); sp != nil {
		return sectionTextWidth(sp)
	}
	return final
}

func (s *story) body() *wml.CT_Body {
	if s == nil || s.pkg == nil || s.pkg.DocPart == nil {
		return nil
	}
	return s.pkg.DocPart.Body()
}

// sections returns the document's sections in order: each paragraph-level
// section break, then the body's final section.
func (s *story) sections() []*wml.CT_SectPr {
	body := s.body()
	if body == nil {
		return nil
	}
	var out []*wml.CT_SectPr
	for _, b := range body.Children {
		if p, ok := b.(*wml.CT_P); ok && p.PPr != nil && p.PPr.SectPr != nil {
			out = append(out, p.PPr.SectPr)
		}
	}
	if body.SectPr != nil {
		out = append(out, body.SectPr)
	}
	return out
}

// sectionUsingPart returns the first section that references the story's
// part as a header or footer, or nil when the story is not one (or no
// section references it).
func (s *story) sectionUsingPart() *wml.CT_SectPr {
	if s == nil || s.pkg == nil {
		return nil
	}
	relID, header := "", false
	for _, h := range s.pkg.Headers {
		if h.Path() == s.src {
			relID, header = h.RelID, true
		}
	}
	for _, f := range s.pkg.Footers {
		if f.Path() == s.src {
			relID = f.RelID
		}
	}
	if relID == "" {
		return nil
	}
	for _, sp := range s.sections() {
		refs := sp.FooterRefs()
		if header {
			refs = sp.HeaderRefs()
		}
		for _, r := range refs {
			if r.RelID == relID {
				return sp
			}
		}
	}
	return nil
}

// cellLoc is a table cell with the table and column it sits in.
type cellLoc struct {
	cell *wml.CT_Tc
	tbl  *wml.CT_Tbl
	col  int // first grid column the cell covers
}

// width returns the cell's width: its w:tcW when fixed (dxa), else the
// widths of the grid columns it spans, or 0 when neither is known.
func (c *cellLoc) width() Twip {
	if c.cell.TcPr != nil {
		if ws := c.cell.TcPr.WidthSpec(); ws != nil && ws.Type == wml.WidthDxa && ws.W > 0 {
			return Twip(ws.W)
		}
	}
	if c.tbl.TblGrid == nil {
		return 0
	}
	span := gridSpanOf(c.cell)
	var w Twip
	for k := c.col; k < c.col+span && k < len(c.tbl.TblGrid.Cols); k++ {
		if gc := c.tbl.TblGrid.Cols[k]; gc != nil && gc.W != nil {
			w += Twip(*gc.W)
		}
	}
	return w
}

// cellHolding finds the table cell whose block list is list.
func (s *story) cellHolding(list *[]wml.BlockElement) *cellLoc {
	for _, root := range s.roots() {
		if c := findCell(*root, list); c != nil {
			return c
		}
	}
	return nil
}

func findCell(blocks []wml.BlockElement, list *[]wml.BlockElement) *cellLoc {
	for _, b := range blocks {
		tbl, ok := b.(*wml.CT_Tbl)
		if !ok {
			continue
		}
		for _, r := range tbl.Children {
			row, ok := r.(*wml.CT_Tr)
			if !ok {
				continue
			}
			col := 0
			for _, c := range row.Children {
				cell, ok := c.(*wml.CT_Tc)
				if !ok {
					continue
				}
				if &cell.Children == list {
					return &cellLoc{cell: cell, tbl: tbl, col: col}
				}
				if found := findCell(cell.Children, list); found != nil {
					return found
				}
				col += gridSpanOf(cell)
			}
		}
	}
	return nil
}

// maxGridColumns is the most columns a Word table can have; a w:gridSpan
// beyond it (only a broken or hostile document has one) is clamped to it.
const maxGridColumns = 63

// gridSpanOf returns how many grid columns a cell covers: its w:gridSpan,
// at least 1 and at most maxGridColumns.
func gridSpanOf(cell *wml.CT_Tc) int {
	if cell.TcPr == nil {
		return 1
	}
	n := cell.TcPr.GridSpan()
	switch {
	case n == nil || *n <= 1:
		return 1
	case *n >= maxGridColumns:
		return maxGridColumns
	default:
		return int(*n)
	}
}

// columnWidths returns the width of each of cols columns: the given width
// where there is one, and an even share of what is left of width for the
// others. When that share would be narrower than minColumnShare (the given
// widths fill or nearly fill width), each missing column gets width / cols.
func columnWidths(cols int, given []Twip, width Twip) []Twip {
	out := make([]Twip, cols)
	var used Twip
	missing := 0
	for i := range cols {
		if i < len(given) && given[i] > 0 {
			out[i] = given[i]
			used += given[i]
		} else {
			missing++
		}
	}
	if missing == 0 {
		return out
	}
	share := (width - used) / Twip(missing)
	if share < minColumnShare {
		share = width / Twip(cols)
	}
	for i := range out {
		if out[i] == 0 {
			out[i] = share
		}
	}
	return out
}
