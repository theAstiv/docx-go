package docx

import "github.com/theAstiv/docx-go/internal/oxml/wml"

// A table created without column widths gets them the way Word sizes an
// inserted table: the text width (page width minus the left and right
// margins) of the section holding it, shared evenly by the columns that
// have no width. Without widths Word lays out a table of empty cells as a
// sliver (CLAUDE.md C17).

// cellMargins is Word's default left plus right cell margin (2 × 0.075").
const cellMargins Twip = 2 * 108

// defaultSection is the page New() writes (wml.DefaultSectPr); its values
// fill in what a section leaves out.
var defaultSection = wml.DefaultSectPr()

// defaultTextWidth is the text width of defaultSection.
var defaultTextWidth = Twip(*defaultSection.PgSzW() - *defaultSection.PgMarLeft() - *defaultSection.PgMarRight())

// sectionTextWidth returns the width between a section's left and right
// margins, taking missing values from DefaultSectPr.
func sectionTextWidth(sp *wml.CT_SectPr) Twip {
	w, left, right := *defaultSection.PgSzW(), *defaultSection.PgMarLeft(), *defaultSection.PgMarRight()
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
	}
	if tw := Twip(w - left - right); tw > 0 {
		return tw
	}
	return defaultTextWidth
}

// textWidthAt returns the width a table inserted into list at index i may
// fill: in the body, the text width of the section it falls in (the next
// paragraph-level section break at or after i, else the body's final
// section); in a table cell with a fixed width, that width less the
// default cell margins; anywhere else (headers, footers, other cells), the
// body's final section.
func (s *story) textWidthAt(list *[]wml.BlockElement, i int) Twip {
	var body *wml.CT_Body
	if s != nil && s.pkg != nil && s.pkg.DocPart != nil {
		body = s.pkg.DocPart.Body()
	}
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
	if cell := s.cellHolding(list); cell != nil && cell.TcPr != nil {
		if ws := cell.TcPr.WidthSpec(); ws != nil && ws.Type == wml.WidthDxa && Twip(ws.W) > cellMargins {
			return Twip(ws.W) - cellMargins
		}
	}
	return final
}

// cellHolding returns the table cell whose block list is list, or nil.
func (s *story) cellHolding(list *[]wml.BlockElement) *wml.CT_Tc {
	for _, root := range s.roots() {
		if c := findCell(*root, list); c != nil {
			return c
		}
	}
	return nil
}

func findCell(blocks []wml.BlockElement, list *[]wml.BlockElement) *wml.CT_Tc {
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
			for _, c := range row.Children {
				cell, ok := c.(*wml.CT_Tc)
				if !ok {
					continue
				}
				if &cell.Children == list {
					return cell
				}
				if found := findCell(cell.Children, list); found != nil {
					return found
				}
			}
		}
	}
	return nil
}

// columnWidths returns the width of each of cols columns: the given width
// where there is one, and an even share of what is left of width for the
// others. When the given widths already fill width, each missing column
// gets width / cols.
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
	if share <= 0 {
		share = width / Twip(cols)
	}
	for i := range out {
		if out[i] == 0 {
			out[i] = share
		}
	}
	return out
}
