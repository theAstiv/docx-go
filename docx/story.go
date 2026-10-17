package docx

import (
	"github.com/theAstiv/docx-go/internal/oxml/wml"
	"github.com/theAstiv/docx-go/internal/parts"
)

// story identifies the part that holds a piece of content (the main document,
// a header or a footer). Proxies carry it so that features which touch the
// package, such as hyperlink relationships and document-wide id allocation,
// can find the right relationships and see every other story.
type story struct {
	pkg *parts.Package
	// src is the OPC path of the part that owns the content's relationships.
	src string
}

// docStory returns the story of the main document part.
func docStory(pkg *parts.Package) *story {
	return &story{pkg: pkg, src: parts.PathDocument}
}

// walkParagraphs calls fn for every paragraph in every story of the package
// (body, headers, footers), descending into tables.
func (s *story) walkParagraphs(fn func(*wml.CT_P)) {
	if s.pkg.DocPart != nil {
		if body := s.pkg.DocPart.Body(); body != nil {
			walkBlocks(body.Children, fn)
		}
	}
	for _, h := range s.pkg.Headers {
		walkBlocks(h.Part.CT().Children, fn)
	}
	for _, f := range s.pkg.Footers {
		walkBlocks(f.Part.CT().Children, fn)
	}
}

// nextDocPrID returns an id one above the largest wp:docPr id in any story, or
// 1 for a document without drawings. Drawing ids must be unique document-wide.
func (s *story) nextDocPrID() int64 {
	next := int64(1)
	s.walkParagraphs(func(p *wml.CT_P) {
		forEachRun(p.Children, func(r *wml.CT_R) {
			for _, id := range r.DocPrIDs() {
				if id >= next {
					next = id + 1
				}
			}
		})
	})
	return next
}

// forEachRun calls fn for every run among children, including runs inside
// hyperlinks.
func forEachRun(children []wml.InlineElement, fn func(*wml.CT_R)) {
	for _, c := range children {
		switch el := c.(type) {
		case *wml.CT_R:
			fn(el)
		case *wml.CT_Hyperlink:
			forEachRun(el.Children, fn)
		}
	}
}

// walkBlocks calls fn for every paragraph in blocks, descending into tables
// and their cells.
func walkBlocks(blocks []wml.BlockElement, fn func(*wml.CT_P)) {
	for _, b := range blocks {
		switch el := b.(type) {
		case *wml.CT_P:
			fn(el)
		case *wml.CT_Tbl:
			for _, r := range el.Children {
				tr, ok := r.(*wml.CT_Tr)
				if !ok {
					continue
				}
				for _, c := range tr.Children {
					if tc, ok := c.(*wml.CT_Tc); ok {
						walkBlocks(tc.Children, fn)
					}
				}
			}
		}
	}
}
