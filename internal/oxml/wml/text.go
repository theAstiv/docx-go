package wml

import (
	"encoding/xml"
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
)

// Plain-text extraction for block lists (a body, header, footer or cell).
//
// Layout: every paragraph ends with "\n". A table is written row by row:
// the cells of a row are separated by "\t" and each row ends with "\n". A
// cell's own paragraphs are joined with "\n" (trailing empty paragraphs
// dropped, as Word requires one), so a cell holding several paragraphs or a
// nested table spans lines. Content the typed model keeps raw is included:
// block-level content controls (w:sdt), custom XML, tracked insertions,
// smart tags and the like. Tracked deletions, field instructions, content-
// control properties, drawings and mc:Fallback copies are not.

// BlocksText returns the plain text of blocks.
func BlocksText(blocks []BlockElement) string {
	var sb strings.Builder
	for _, seg := range blockSegments(blocks) {
		sb.WriteString(seg)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// Text returns the cell's paragraphs (and nested tables) joined with "\n",
// without the trailing empty paragraphs.
func (tc *CT_Tc) Text() string { return cellText(tc.Children) }

// blockSegments returns one string per paragraph and per table in blocks,
// without terminators.
func blockSegments(blocks []BlockElement) []string {
	var segs []string
	for _, b := range blocks {
		switch el := b.(type) {
		case *CT_P:
			segs = append(segs, el.Text())
		case *CT_Tbl:
			segs = append(segs, tableText(el))
		case *RawBlock:
			if el.E != nil {
				segs = rawBlockSegments(el.E, segs)
			}
		}
	}
	return segs
}

func cellText(blocks []BlockElement) string {
	return joinCell(blockSegments(blocks))
}

// joinCell joins a cell's segments, dropping trailing empty ones.
func joinCell(segs []string) string {
	for len(segs) > 0 && segs[len(segs)-1] == "" {
		segs = segs[:len(segs)-1]
	}
	return strings.Join(segs, "\n")
}

func tableText(t *CT_Tbl) string {
	var rows []string
	for _, r := range t.Children {
		switch row := r.(type) {
		case *CT_Tr:
			rows = append(rows, rowText(row))
		case *RawTableRow:
			if row.E != nil {
				rows = rawRows(row.E, rows)
			}
		}
	}
	return strings.Join(rows, "\n")
}

func rowText(tr *CT_Tr) string {
	var cells []string
	for _, c := range tr.Children {
		switch cell := c.(type) {
		case *CT_Tc:
			cells = append(cells, cell.Text())
		case *RawTableCell:
			if cell.E != nil {
				cells = rawCells(cell.E, cells)
			}
		}
	}
	return strings.Join(cells, "\t")
}

// ── raw content ───────────────────────────────────────────────────────────

// rawSkip lists raw elements whose subtree holds no visible text, or text
// that must not be reported: deleted content, properties, drawings, and the
// fallback copy of mc:AlternateContent.
var rawSkip = map[xml.Name]bool{
	ns.Qn("w:del"):          true,
	ns.Qn("w:moveFrom"):     true,
	ns.Qn("w:pPr"):          true,
	ns.Qn("w:rPr"):          true,
	ns.Qn("w:sectPr"):       true,
	ns.Qn("w:tblPr"):        true,
	ns.Qn("w:tblGrid"):      true,
	ns.Qn("w:trPr"):         true,
	ns.Qn("w:tcPr"):         true,
	ns.Qn("w:sdtPr"):        true,
	ns.Qn("w:sdtEndPr"):     true,
	ns.Qn("w:customXmlPr"):  true,
	ns.Qn("w:smartTagPr"):   true,
	ns.Qn("w:drawing"):      true,
	ns.Qn("w:pict"):         true,
	ns.Qn("w:object"):       true,
	ns.Qn("mc:Fallback"):    true,
	ns.Qn("w:instrText"):    true,
	ns.Qn("w:delInstrText"): true,
}

func rawElems(re *raw.RawElement) []*raw.RawElement {
	var out []*raw.RawElement
	for _, k := range re.Children {
		if c, ok := k.(*raw.RawElement); ok {
			out = append(out, c)
		}
	}
	return out
}

// rawBlockSegments appends the segments of a raw block-level element: a
// paragraph, a table, or a container (w:sdt, w:customXml, …) holding them.
func rawBlockSegments(re *raw.RawElement, segs []string) []string {
	switch {
	case rawSkip[re.QName]:
		return segs
	case re.QName == ns.Qn("w:p"):
		var w textWriter
		rawInlineText(re.Children, &w)
		return append(segs, w.sb.String())
	case re.QName == ns.Qn("w:tbl"):
		var rows []string
		for _, c := range rawElems(re) {
			rows = rawRows(c, rows)
		}
		return append(segs, strings.Join(rows, "\n"))
	}
	for _, c := range rawElems(re) {
		segs = rawBlockSegments(c, segs)
	}
	return segs
}

// rawRows appends the rows found in a raw table child: a w:tr, or a
// container (a row-level w:sdt) holding rows.
func rawRows(re *raw.RawElement, rows []string) []string {
	switch {
	case rawSkip[re.QName]:
		return rows
	case re.QName == ns.Qn("w:tr"):
		var cells []string
		for _, c := range rawElems(re) {
			cells = rawCells(c, cells)
		}
		return append(rows, strings.Join(cells, "\t"))
	}
	for _, c := range rawElems(re) {
		rows = rawRows(c, rows)
	}
	return rows
}

// rawCells appends the cells found in a raw row child: a w:tc, or a
// container (a cell-level w:sdt) holding cells.
func rawCells(re *raw.RawElement, cells []string) []string {
	switch {
	case rawSkip[re.QName]:
		return cells
	case re.QName == ns.Qn("w:tc"):
		var segs []string
		for _, c := range rawElems(re) {
			segs = rawBlockSegments(c, segs)
		}
		return append(cells, joinCell(segs))
	}
	for _, c := range rawElems(re) {
		cells = rawCells(c, cells)
	}
	return cells
}

// rawInlineText writes the text of raw paragraph content (runs, and
// containers such as w:ins, w:smartTag, w:sdt and w:fldSimple holding
// them) into w, with the rules of textFromRunContent.
func rawInlineText(children []xml.Token, w *textWriter) {
	for _, k := range children {
		el, ok := k.(*raw.RawElement)
		if !ok || rawSkip[el.QName] {
			continue
		}
		if el.QName == ns.Qn("w:r") {
			rawRunText(el, w)
			continue
		}
		rawInlineText(el.Children, w)
	}
}

// rawRunText writes the text of a raw w:r into w.
func rawRunText(r *raw.RawElement, w *textWriter) {
	for _, c := range rawElems(r) {
		switch c.QName {
		case ns.Qn("w:t"):
			for _, tok := range c.Children {
				if cd, ok := tok.(xml.CharData); ok {
					w.write(string(cd))
				}
			}
		case ns.Qn("w:tab"):
			w.write("\t")
		case ns.Qn("w:br"), ns.Qn("w:cr"):
			w.write("\n")
		case ns.Qn("w:noBreakHyphen"):
			w.write("‑") // NON-BREAKING HYPHEN
		case ns.Qn("w:softHyphen"):
			w.write("­") // SOFT HYPHEN
		case ns.Qn("w:fldChar"):
			w.fldChar(rawAttr(c, "fldCharType"))
		}
	}
}

// rawAttr returns the value of the attribute with the given local name.
func rawAttr(re *raw.RawElement, local string) string {
	for _, a := range re.Attrs {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}
