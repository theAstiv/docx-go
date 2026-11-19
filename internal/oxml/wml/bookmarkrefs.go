package wml

import (
	"encoding/xml"
	"regexp"
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
)

// refField matches the bookmark a REF, PAGEREF or NOTEREF field points at.
// Field names are case-insensitive; the name may be quoted.
var refField = regexp.MustCompile(`(?i)\b(?:REF|PAGEREF|NOTEREF)\s+"?([^\s"\\]+)`)

// ReferencedBookmarks returns the names of the bookmarks blocks point at:
// internal hyperlinks (w:anchor) and REF, PAGEREF and NOTEREF fields, in
// typed and raw content alike. Field instructions are joined per paragraph,
// since Word often splits one across runs.
func ReferencedBookmarks(blocks []BlockElement) map[string]bool {
	r := refScanner{names: map[string]bool{}, instr: strings.Builder{}}
	r.blocks(blocks)
	return r.names
}

type refScanner struct {
	names map[string]bool
	instr strings.Builder // field instructions of the current paragraph
}

func (r *refScanner) blocks(blocks []BlockElement) {
	for _, b := range blocks {
		switch el := b.(type) {
		case *CT_P:
			r.inlines(el.Children)
			r.flush()
		case *CT_Tbl:
			for _, row := range el.Children {
				switch x := row.(type) {
				case *CT_Tr:
					for _, c := range x.Children {
						switch cell := c.(type) {
						case *CT_Tc:
							r.blocks(cell.Children)
						case *RawTableCell:
							r.raw(cell.E)
							r.flush()
						}
					}
				case *RawTableRow:
					r.raw(x.E)
					r.flush()
				}
			}
		case *RawBlock:
			r.raw(el.E)
			r.flush()
		}
	}
}

func (r *refScanner) inlines(children []InlineElement) {
	for _, c := range children {
		switch el := c.(type) {
		case *CT_Hyperlink:
			if el.Anchor != "" {
				r.names[el.Anchor] = true
			}
			r.inlines(el.Children)
		case *CT_R:
			for _, rc := range el.Children {
				switch x := rc.(type) {
				case *CT_FldChar:
					if x.FldCharType == FldCharBegin {
						r.instr.WriteByte(' ') // keep fields apart
					}
				case *CT_InstrText:
					r.instr.WriteString(x.Content)
				case *RawRunContent:
					r.raw(x.E)
				}
			}
		case *RawInline:
			r.raw(el.E)
		}
	}
}

func (r *refScanner) raw(re *raw.RawElement) {
	if re == nil {
		return
	}
	switch re.QName {
	case ns.Qn("w:hyperlink"):
		if a := attrNS(re.Attrs, ns.W, "anchor"); a != "" {
			r.names[a] = true
		}
	case ns.Qn("w:fldSimple"):
		r.instr.WriteByte(' ')
		r.instr.WriteString(attrNS(re.Attrs, ns.W, "instr"))
	case ns.Qn("w:fldChar"):
		r.instr.WriteByte(' ')
	case ns.Qn("w:instrText"):
		for _, c := range re.Children {
			if cd, ok := c.(xml.CharData); ok {
				r.instr.Write(cd)
			}
		}
	}
	for _, k := range rawKids(re) {
		r.raw(k)
	}
	if re.QName == ns.Qn("w:p") {
		r.flush()
	}
}

// flush reads the bookmark names out of the paragraph's instructions.
func (r *refScanner) flush() {
	if r.instr.Len() == 0 {
		return
	}
	for _, m := range refField.FindAllStringSubmatch(r.instr.String(), -1) {
		r.names[m[1]] = true
	}
	r.instr.Reset()
}
