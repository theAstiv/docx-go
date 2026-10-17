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
// content this package does not model. It rescans on every call; callers that
// add many items should keep that cost in mind.
func (s *story) scan() *wml.IDScan {
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
