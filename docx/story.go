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
	return &story{pkg: pkg, src: pkg.DocPath()}
}

// scan collects the document-wide identifiers (drawing ids, bookmarks) from
// every story: body, headers and footers. It reaches into tables, hyperlinks
// and all raw content such as content controls (w:sdt), tracked changes,
// smart tags and mc:AlternateContent, so allocation never collides with
// content this package does not model.
//
// The result is cached on the package, so a loop of AddImage or AddBookmark
// calls scans once. The cache is only ever extended by the allocators below
// (noteDocPr, noteBookmark). Anything that can add ids behind its back, such
// as template execution, calls invalidateIDs; so do the Remove methods, so
// that Document.Bookmarks stops listing a removed bookmark.
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
	// One visit per part: a header two relationships share would otherwise
	// report each of its bookmarks twice.
	for _, h := range s.pkg.UniqueHeaders() {
		wml.ScanBlocks(h.Part.CT().Children, scan)
	}
	for _, f := range s.pkg.UniqueFooters() {
		wml.ScanBlocks(f.Part.CT().Children, scan)
	}
	return scan
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
