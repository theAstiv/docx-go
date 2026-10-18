package docx

import (
	"github.com/theAstiv/docx-go/docx/internal/bridge"
	"github.com/theAstiv/docx-go/internal/parts"
	itemplate "github.com/theAstiv/docx-go/internal/template"
)

func init() {
	bridge.Stories = templateStories
	bridge.UnparsedParts = func(d any) map[string][]byte {
		doc, ok := d.(*Document)
		if !ok || doc == nil || doc.pkg == nil {
			return nil
		}
		return doc.pkg.UnparsedStoryParts()
	}
	bridge.BoundParts = func(d any) map[string][]byte {
		doc, ok := d.(*Document)
		if !ok || doc == nil || doc.pkg == nil {
			return nil
		}
		return doc.pkg.BoundDataParts()
	}
	bridge.SetPart = func(d any, path string, data []byte) {
		if doc, ok := d.(*Document); ok && doc != nil && doc.pkg != nil {
			doc.pkg.SetPartData(path, data)
		}
	}
}

// templateStories hands docx/template the document's stories: the body,
// then every header and footer part.
func templateStories(d any) []itemplate.Story {
	doc, ok := d.(*Document)
	if !ok || doc == nil || doc.pkg == nil {
		return nil
	}
	// Execution hands out writable trees and may add bookmarks and drawings,
	// so the cached id scan can no longer be trusted.
	doc.pkg.IDCache = nil
	var out []itemplate.Story
	if doc.pkg.DocPart != nil {
		if body := doc.pkg.DocPart.Body(); body != nil {
			out = append(out, itemplate.Story{Part: parts.PathDocument, Blocks: &body.Children})
		}
	}
	for _, h := range doc.pkg.Headers {
		out = append(out, itemplate.Story{Part: h.Path(), Blocks: &h.Part.CT().Children})
	}
	for _, f := range doc.pkg.Footers {
		out = append(out, itemplate.Story{Part: f.Path(), Blocks: &f.Part.CT().Children})
	}
	return out
}
