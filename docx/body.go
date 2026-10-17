package docx

import (
	"iter"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
	"github.com/theAstiv/docx-go/internal/parts"
)

// Body is a proxy for the w:body element, providing access to the document's
// block-level content.
type Body struct {
	docPart *parts.DocumentPart
	st      *story
}

func (b *Body) ctBody() *wml.CT_Body {
	return b.docPart.Body()
}

// Paragraphs returns an iterator over all top-level paragraphs in the body.
// Tables and other block elements are skipped; use Children() for full access.
func (b *Body) Paragraphs() iter.Seq[*Paragraph] {
	return func(yield func(*Paragraph) bool) {
		body := b.ctBody()
		if body == nil {
			return
		}
		for _, child := range body.Children {
			p, ok := child.(*wml.CT_P)
			if !ok {
				continue
			}
			if !yield(&Paragraph{p: p, st: b.st}) {
				return
			}
		}
	}
}

// AddParagraph appends a new empty paragraph to the body and returns its proxy.
func (b *Body) AddParagraph() *Paragraph {
	p := new(wml.CT_P)
	body := b.ctBody()
	// Insert before the trailing sectPr (which is not a child but a field),
	// so we simply append to Children.
	body.Children = append(body.Children, p)
	return &Paragraph{p: p, st: b.st}
}

// Tables returns an iterator over all top-level tables in the body.
// Paragraphs and other block elements are skipped.
func (b *Body) Tables() iter.Seq[*Table] {
	return func(yield func(*Table) bool) {
		body := b.ctBody()
		if body == nil {
			return
		}
		for _, child := range body.Children {
			tbl, ok := child.(*wml.CT_Tbl)
			if !ok {
				continue
			}
			if !yield(&Table{tbl: tbl, st: b.st}) {
				return
			}
		}
	}
}

// AddTable appends a new table with the given dimensions and returns its proxy.
// rows and cols must be ≥ 1. Each cell is pre-populated with an empty paragraph.
func (b *Body) AddTable(rows, cols int, opts TableOptions) *Table {
	if rows < 1 {
		rows = 1
	}
	if cols < 1 {
		cols = 1
	}
	tbl := buildTable(rows, cols, opts)
	b.ctBody().Children = append(b.ctBody().Children, tbl)
	return &Table{tbl: tbl, st: b.st}
}
