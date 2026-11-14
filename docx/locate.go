package docx

import "github.com/theAstiv/docx-go/internal/oxml/wml"

// Proxies hold their element, not its container. Removing or inserting
// next to an element needs the container, so proxies carry a parent hint
// (the slice they were found in) and the functions below confirm it by
// identity, falling back to a walk over every story. A stale hint (the
// element was moved or removed) is never trusted.

// roots returns the top-level block lists of every story: the body, then
// each header and footer.
func (s *story) roots() []*[]wml.BlockElement {
	if s == nil || s.pkg == nil {
		return nil
	}
	var out []*[]wml.BlockElement
	if s.pkg.DocPart != nil {
		if body := s.pkg.DocPart.Body(); body != nil {
			out = append(out, &body.Children)
		}
	}
	for _, h := range s.pkg.Headers {
		out = append(out, &h.Part.CT().Children)
	}
	for _, f := range s.pkg.Footers {
		out = append(out, &f.Part.CT().Children)
	}
	return out
}

// isBody reports whether list is the document body's block list.
func (s *story) isBody(list *[]wml.BlockElement) bool {
	if s == nil || s.pkg == nil || s.pkg.DocPart == nil {
		return false
	}
	body := s.pkg.DocPart.Body()
	return body != nil && list == &body.Children
}

// indexOfBlock returns the position of el in list, or -1.
func indexOfBlock(list []wml.BlockElement, el wml.BlockElement) int {
	for i, b := range list {
		if b == el {
			return i
		}
	}
	return -1
}

// locateBlock finds the block list holding el and its index there.
func (s *story) locateBlock(el wml.BlockElement, hint *[]wml.BlockElement) (*[]wml.BlockElement, int, bool) {
	if hint != nil {
		if i := indexOfBlock(*hint, el); i >= 0 {
			return hint, i, true
		}
	}
	for _, root := range s.roots() {
		if list, i, ok := findBlock(root, el); ok {
			return list, i, true
		}
	}
	return nil, -1, false
}

// findBlock searches list and the cells of its tables, depth first.
func findBlock(list *[]wml.BlockElement, el wml.BlockElement) (*[]wml.BlockElement, int, bool) {
	if i := indexOfBlock(*list, el); i >= 0 {
		return list, i, true
	}
	for _, b := range *list {
		tbl, ok := b.(*wml.CT_Tbl)
		if !ok {
			continue
		}
		for _, r := range tbl.Children {
			row, ok := r.(*wml.CT_Tr)
			if !ok {
				continue
			}
			for _, c := range row.Children {
				if cell, ok := c.(*wml.CT_Tc); ok {
					if l, i, found := findBlock(&cell.Children, el); found {
						return l, i, true
					}
				}
			}
		}
	}
	return nil, -1, false
}

// locateInline finds the inline list (a paragraph's or hyperlink's
// children) holding el and its index there.
func (s *story) locateInline(el wml.InlineElement, hint *[]wml.InlineElement) (*[]wml.InlineElement, int, bool) {
	if hint != nil {
		if i := indexOfInline(*hint, el); i >= 0 {
			return hint, i, true
		}
	}
	for _, root := range s.roots() {
		if list, i, ok := findInlineInBlocks(*root, el); ok {
			return list, i, true
		}
	}
	return nil, -1, false
}

func indexOfInline(list []wml.InlineElement, el wml.InlineElement) int {
	for i, c := range list {
		if c == el {
			return i
		}
	}
	return -1
}

func findInlineInBlocks(blocks []wml.BlockElement, el wml.InlineElement) (*[]wml.InlineElement, int, bool) {
	for _, b := range blocks {
		switch blk := b.(type) {
		case *wml.CT_P:
			if l, i, ok := findInline(&blk.Children, el); ok {
				return l, i, true
			}
		case *wml.CT_Tbl:
			for _, r := range blk.Children {
				row, ok := r.(*wml.CT_Tr)
				if !ok {
					continue
				}
				for _, c := range row.Children {
					if cell, ok := c.(*wml.CT_Tc); ok {
						if l, i, found := findInlineInBlocks(cell.Children, el); found {
							return l, i, true
						}
					}
				}
			}
		}
	}
	return nil, -1, false
}

func findInline(list *[]wml.InlineElement, el wml.InlineElement) (*[]wml.InlineElement, int, bool) {
	if i := indexOfInline(*list, el); i >= 0 {
		return list, i, true
	}
	for _, c := range *list {
		if h, ok := c.(*wml.CT_Hyperlink); ok {
			if l, i, found := findInline(&h.Children, el); found {
				return l, i, true
			}
		}
	}
	return nil, -1, false
}

// blocksAfter returns the blocks that follow target in its container (the
// body, a header, a footer or a table cell), or nil when target is not found.
func (s *story) blocksAfter(target *wml.CT_P, hint *[]wml.BlockElement) []wml.BlockElement {
	list, i, ok := s.locateBlock(target, hint)
	if !ok {
		return nil
	}
	return (*list)[i+1:]
}
