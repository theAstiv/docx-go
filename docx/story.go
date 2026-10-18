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

// scan collects the document-wide identifiers (drawing ids, bookmarks) from
// every story: body, headers and footers. It reaches into tables, hyperlinks
// and all raw content such as content controls (w:sdt), tracked changes,
// smart tags and mc:AlternateContent, so allocation never collides with
// content this package does not model.
//
// The result is cached on the package, so a loop of AddImage or AddBookmark
// calls scans once. The cache is only ever extended by the allocators below
// (noteDocPr, noteBookmark); removing content leaves it stale in the safe
// direction (ids are skipped, never reused). Anything that can add ids behind
// its back, such as template execution, calls invalidateIDs.
func (s *story) scan() *wml.IDScan {
	if s.pkg.IDCache == nil {
		s.pkg.IDCache = s.freshScan()
	}
	return s.pkg.IDCache
}

// invalidateIDs drops the cached scan; the next scan walks the stories again.
func (s *story) invalidateIDs() { s.pkg.IDCache = nil }

// noteDocPr records an allocated wp:docPr id in the cache.
func (s *story) noteDocPr(id int64) {
	sc := s.scan()
	sc.DocPr = append(sc.DocPr, id)
}

// noteBookmark records a created bookmark in the cache.
func (s *story) noteBookmark(bs *wml.CT_BookmarkStart) {
	sc := s.scan()
	sc.Bookmarks = append(sc.Bookmarks, wml.BookmarkRef{ID: bs.ID, Name: bs.Name, Start: bs})
}

// freshScan walks every story, ignoring the cache.
func (s *story) freshScan() *wml.IDScan {
	scan := new(wml.IDScan)
	if s.pkg.DocPart != nil {
		if body := s.pkg.DocPart.Body(); body != nil {
			wml.ScanBlocks(body.Children, scan)
		}
	}
	for _, h := range s.pkg.Headers {
		wml.ScanBlocks(h.Part.CT().Children, scan)
	}
	for _, f := range s.pkg.Footers {
		wml.ScanBlocks(f.Part.CT().Children, scan)
	}
	return scan
}

// blocksAfter returns the blocks that follow target in its container (the
// body, a header, a footer or a table cell), or nil when target is not found.
func (s *story) blocksAfter(target *wml.CT_P) []wml.BlockElement {
	var roots [][]wml.BlockElement
	if s.pkg.DocPart != nil {
		if body := s.pkg.DocPart.Body(); body != nil {
			roots = append(roots, body.Children)
		}
	}
	for _, h := range s.pkg.Headers {
		roots = append(roots, h.Part.CT().Children)
	}
	for _, f := range s.pkg.Footers {
		roots = append(roots, f.Part.CT().Children)
	}
	for _, blocks := range roots {
		if after, ok := blocksAfterIn(blocks, target); ok {
			return after
		}
	}
	return nil
}

func blocksAfterIn(blocks []wml.BlockElement, target *wml.CT_P) ([]wml.BlockElement, bool) {
	for i, b := range blocks {
		switch el := b.(type) {
		case *wml.CT_P:
			if el == target {
				return blocks[i+1:], true
			}
		case *wml.CT_Tbl:
			for _, r := range el.Children {
				row, ok := r.(*wml.CT_Tr)
				if !ok {
					continue
				}
				for _, c := range row.Children {
					if cell, ok := c.(*wml.CT_Tc); ok {
						if after, found := blocksAfterIn(cell.Children, target); found {
							return after, true
						}
					}
				}
			}
		}
	}
	return nil, false
}

// nextDocPrID returns an id one above the largest wp:docPr id in any story, or
// 1 for a document without drawings. Drawing ids must be unique document-wide.
func (s *story) nextDocPrID() int64 {
	next := int64(1)
	for _, id := range s.scan().DocPr {
		if id >= next {
			next = id + 1
		}
	}
	return next
}
