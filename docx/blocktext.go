package docx

import "github.com/theAstiv/docx-go/internal/oxml/wml"

// setBlocksText replaces the content of a block container (a cell, header or
// footer) with one paragraph holding text, and returns it. The first
// paragraph's properties (style, alignment, …) are kept so the text keeps
// the container's look; its runs and every other block go.
func setBlocksText(children *[]wml.BlockElement, text string, st *story) *Paragraph {
	var p *wml.CT_P
	for _, b := range *children {
		if first, ok := b.(*wml.CT_P); ok {
			p = first
			break
		}
	}
	if p == nil {
		p = new(wml.CT_P)
	}
	var marks []wml.Mark
	for _, b := range *children {
		marks = append(marks, wml.MarksIn(b)...)
	}
	p.Children = nil
	*children = []wml.BlockElement{p}
	// The blocks dropped here may hold lists other proxies hint at.
	st.detach()
	// Same cleanup as Remove: partners of dropped bookmark and comment
	// anchors go too, and the id cache forgets dropped ids.
	st.afterRemoval(marks)
	para := &Paragraph{p: p, st: st, parent: children, gen: st.stamp()}
	if text != "" {
		para.AddRun(text)
	}
	return para
}

// appendBlock appends b and, when b is not a paragraph, an empty paragraph
// after it: Word expects a cell, header or footer to end with one.
func appendBlock(children *[]wml.BlockElement, b wml.BlockElement) {
	*children = append(*children, b)
	if _, ok := b.(*wml.CT_P); !ok {
		*children = append(*children, new(wml.CT_P))
	}
}
