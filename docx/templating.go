package docx

import (
	"errors"
	"fmt"

	"github.com/theAstiv/docx-go/docx/internal/bridge"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
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
	bridge.AddImage = templateImage
	bridge.Snapshot = func(d any) (func(), error) {
		doc, ok := d.(*Document)
		if !ok || doc == nil || doc.pkg == nil {
			return func() {}, nil
		}
		s, err := doc.pkg.Snapshot()
		if err != nil {
			return nil, fmt.Errorf("docx: %w", err)
		}
		return func() { doc.pkg.Restore(s) }, nil
	}
}

// templateImage places a template image value: the same validation, sizing
// and storage as Run.AddImage, with the wp:docPr id the template engine
// allocated.
func templateImage(d any, part string, img itemplate.ImageValue, docPrID int64) (*wml.CT_Drawing, error) {
	doc, ok := d.(*Document)
	if !ok || doc == nil || doc.pkg == nil {
		return nil, errors.New("docx: no document to add the image to")
	}
	opts := ImageOptions{Width: EMU(img.Width), Height: EMU(img.Height), AltText: img.AltText, Title: img.Title}
	return newPicture(doc.pkg, part, img.Data, opts, func() int64 { return docPrID })
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
