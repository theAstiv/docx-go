package docx

import (
	"iter"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
	"github.com/theAstiv/docx-go/internal/parts"
)

// Footer is a proxy for a w:ftr part, providing the same content API as Body.
type Footer struct {
	pkg *parts.Package
	ct  *wml.CT_Ftr
}

// story returns the story of this footer's part.
func (f *Footer) story() *story {
	for _, e := range f.pkg.Footers {
		if e.Part.CT() == f.ct {
			return &story{pkg: f.pkg, src: e.Path()}
		}
	}
	return nil
}

// Paragraphs returns an iterator over all paragraph children of the footer.
func (f *Footer) Paragraphs() iter.Seq[*Paragraph] {
	return func(yield func(*Paragraph) bool) {
		for _, child := range f.ct.Children {
			p, ok := child.(*wml.CT_P)
			if !ok {
				continue
			}
			if !yield(&Paragraph{p: p, st: f.story()}) {
				return
			}
		}
	}
}

// Tables returns an iterator over all table children of the footer.
func (f *Footer) Tables() iter.Seq[*Table] {
	return func(yield func(*Table) bool) {
		for _, child := range f.ct.Children {
			tbl, ok := child.(*wml.CT_Tbl)
			if !ok {
				continue
			}
			if !yield(&Table{tbl: tbl, st: f.story()}) {
				return
			}
		}
	}
}

// AddParagraph appends a paragraph to the footer and returns its proxy, like
// Body.AddParagraph. A new footer already holds one empty paragraph: fill it
// with SetText or the first element of Paragraphs, or this adds a second.
func (f *Footer) AddParagraph() *Paragraph {
	p := new(wml.CT_P)
	f.ct.Children = append(f.ct.Children, p)
	return &Paragraph{p: p, st: f.story()}
}

// SetText replaces the footer's content with one paragraph holding text and
// returns that paragraph, to add runs, images or fields to. The first
// paragraph's formatting is kept. SetText("") gives the footer's own
// paragraph, empty, to build on.
func (f *Footer) SetText(text string) *Paragraph {
	return setBlocksText(&f.ct.Children, text, f.story())
}

// AddTable appends a new table to the footer and returns its proxy. An
// empty paragraph follows it, because Word wants w:ftr to end with w:p; a
// paragraph added afterwards comes after that empty one (as in python-docx).
func (f *Footer) AddTable(rows, cols int, opts TableOptions) *Table {
	if rows < 1 {
		rows = 1
	}
	if cols < 1 {
		cols = 1
	}
	tbl := buildTable(rows, cols, opts)
	appendBlock(&f.ct.Children, tbl)
	return &Table{tbl: tbl, st: f.story()}
}
