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
			if !yield(&Paragraph{p: p}) {
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
	return &Paragraph{p: p}
}
