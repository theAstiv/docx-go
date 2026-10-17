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

// AddParagraph appends a new paragraph to the footer and returns its proxy.
func (f *Footer) AddParagraph() *Paragraph {
	p := new(wml.CT_P)
	n := len(f.ct.Children)
	if n > 0 {
		if _, ok := f.ct.Children[n-1].(*wml.CT_P); ok {
			f.ct.Children = append(f.ct.Children, nil)
			copy(f.ct.Children[n:], f.ct.Children[n-1:])
			f.ct.Children[n-1] = p
			return &Paragraph{p: p, st: f.story()}
		}
	}
	f.ct.Children = append(f.ct.Children, p)
	return &Paragraph{p: p, st: f.story()}
}

// AddTable appends a new table to the footer and returns its proxy.
// The table is inserted before the OOXML-required trailing paragraph so
// that w:ftr always ends with w:p.
func (f *Footer) AddTable(rows, cols int, opts TableOptions) *Table {
	if rows < 1 {
		rows = 1
	}
	if cols < 1 {
		cols = 1
	}
	tbl := buildTable(rows, cols, opts)
	n := len(f.ct.Children)
	if n > 0 {
		if _, ok := f.ct.Children[n-1].(*wml.CT_P); ok {
			f.ct.Children = append(f.ct.Children, nil)
			copy(f.ct.Children[n:], f.ct.Children[n-1:])
			f.ct.Children[n-1] = tbl
			return &Table{tbl: tbl, st: f.story()}
		}
	}
	f.ct.Children = append(f.ct.Children, tbl)
	return &Table{tbl: tbl, st: f.story()}
}
