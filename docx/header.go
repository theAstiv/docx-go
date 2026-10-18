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

// story returns the story of this header's part.
func (h *Header) story() *story {
	for _, e := range h.pkg.Headers {
		if e.Part.CT() == h.ct {
			return &story{pkg: h.pkg, src: e.Path()}
		}
	}
	return nil
}

// Paragraphs returns an iterator over all paragraph children of the header.
func (h *Header) Paragraphs() iter.Seq[*Paragraph] {
	return func(yield func(*Paragraph) bool) {
		for _, child := range h.ct.Children {
			p, ok := child.(*wml.CT_P)
			if !ok {
				continue
			}
			if !yield(&Paragraph{p: p, st: h.story()}) {
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
			if !yield(&Table{tbl: tbl, st: h.story()}) {
				return
			}
		}
	}
}

// AddParagraph appends a paragraph to the header and returns its proxy, like
// Body.AddParagraph. A new header already holds one empty paragraph: fill it
// with SetText or the first element of Paragraphs, or this adds a second.
func (h *Header) AddParagraph() *Paragraph {
	p := new(wml.CT_P)
	h.ct.Children = append(h.ct.Children, p)
	return &Paragraph{p: p, st: h.story()}
}

// SetText replaces the header's content with one paragraph holding text and
// returns that paragraph, to add runs, images or fields to. The first
// paragraph's formatting is kept. SetText("") gives the header's own
// paragraph, empty, to build on.
func (h *Header) SetText(text string) *Paragraph {
	return setBlocksText(&h.ct.Children, text, h.story())
}

// AddTable appends a new table to the header and returns its proxy. An
// empty paragraph follows it, because Word wants w:hdr to end with w:p; a
// paragraph added afterwards comes after that empty one (as in python-docx).
func (h *Header) AddTable(rows, cols int, opts TableOptions) *Table {
	if rows < 1 {
		rows = 1
	}
	if cols < 1 {
		cols = 1
	}
	tbl := buildTable(rows, cols, opts)
	appendBlock(&h.ct.Children, tbl)
	return &Table{tbl: tbl, st: h.story()}
}
