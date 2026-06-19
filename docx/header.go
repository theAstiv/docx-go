package docx

import (
	"iter"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
	"github.com/theAstiv/docx-go/internal/parts"
)

// Header is a proxy for a w:hdr part, providing the same content API as Body.
type Header struct {
	pkg *parts.Package
	ct  *wml.CT_Hdr
}

// Paragraphs returns an iterator over all paragraph children of the header.
func (h *Header) Paragraphs() iter.Seq[*Paragraph] {
	return func(yield func(*Paragraph) bool) {
		for _, child := range h.ct.Children {
			p, ok := child.(*wml.CT_P)
			if !ok {
				continue
			}
			if !yield(&Paragraph{p: p}) {
				return
			}
		}
	}
}

// Tables returns an iterator over all table children of the header.
func (h *Header) Tables() iter.Seq[*Table] {
	return func(yield func(*Table) bool) {
		for _, child := range h.ct.Children {
			tbl, ok := child.(*wml.CT_Tbl)
			if !ok {
				continue
			}
			if !yield(&Table{tbl: tbl}) {
				return
			}
		}
	}
}

// AddParagraph appends a new paragraph to the header and returns its proxy.
// If the header's last child is an empty paragraph, the new paragraph is
// inserted before it to preserve the OOXML-required trailing paragraph.
func (h *Header) AddParagraph() *Paragraph {
	p := new(wml.CT_P)
	n := len(h.ct.Children)
	// Insert before the last paragraph if it appears to be a structural terminal.
	if n > 0 {
		if _, ok := h.ct.Children[n-1].(*wml.CT_P); ok {
			h.ct.Children = append(h.ct.Children, nil)
			copy(h.ct.Children[n:], h.ct.Children[n-1:])
			h.ct.Children[n-1] = p
			return &Paragraph{p: p}
		}
	}
	h.ct.Children = append(h.ct.Children, p)
	return &Paragraph{p: p}
}

// AddTable appends a new table to the header and returns its proxy.
// The table is inserted before the OOXML-required trailing paragraph so
// that w:hdr always ends with w:p.
func (h *Header) AddTable(rows, cols int, opts TableOptions) *Table {
	if rows < 1 {
		rows = 1
	}
	if cols < 1 {
		cols = 1
	}
	tbl := buildTable(rows, cols, opts)
	n := len(h.ct.Children)
	if n > 0 {
		if _, ok := h.ct.Children[n-1].(*wml.CT_P); ok {
			h.ct.Children = append(h.ct.Children, nil)
			copy(h.ct.Children[n:], h.ct.Children[n-1:])
			h.ct.Children[n-1] = tbl
			return &Table{tbl: tbl}
		}
	}
	h.ct.Children = append(h.ct.Children, tbl)
	return &Table{tbl: tbl}
}
