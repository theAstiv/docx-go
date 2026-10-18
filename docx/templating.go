package docx

import (
	"github.com/theAstiv/docx-go/docx/internal/bridge"
	"github.com/theAstiv/docx-go/internal/parts"
	itemplate "github.com/theAstiv/docx-go/internal/template"
)

func init() {
	bridge.Stories = templateStories
}

// templateStories hands docx/template the document's stories: the body,
// then every header and footer part.
func templateStories(d any) []itemplate.Story {
	doc, ok := d.(*Document)
	if !ok || doc == nil || doc.pkg == nil {
		return nil
	}
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
