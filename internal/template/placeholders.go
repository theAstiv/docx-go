package template

import (
	"fmt"
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// Placeholder is one tag found in a template.
type Placeholder struct {
	// Kind is "value", "each", "if", "unless", "end" or "image".
	Kind string
	// Path is the data path; empty for "end".
	Path string
	// Tag is the tag as written.
	Tag      string
	Location Location
}

// Placeholders lists every well-formed tag in the stories, in document
// order, and reports malformed ones as issues. The stories are not modified.
func Placeholders(stories []Story) ([]Placeholder, []Issue, error) {
	var out []Placeholder
	var issues []Issue
	for _, s := range stories {
		blocks, err := cloneBlocks(*s.Blocks)
		if err != nil {
			return nil, nil, fmt.Errorf("template: copy %s: %w", s.Part, err)
		}
		loc := Location{Part: s.Part}
		issues = append(issues, normalizeBlocks(blocks, loc)...)
		walkParagraphs(blocks, loc, func(p *wml.CT_P, ploc Location) {
			for _, txt := range paragraphTexts(p.Children) {
				if !strings.Contains(txt.Content, "{{") {
					continue
				}
				tags, _ := lexTags(txt.Content)
				for _, t := range tags {
					out = append(out, Placeholder{Kind: kindName(t), Path: t.path, Tag: t.text, Location: ploc})
				}
			}
		})
	}
	return out, issues, nil
}

func kindName(t tag) string {
	switch t.kind {
	case tagOpen:
		return t.block
	case tagClose:
		return "end"
	case tagImage:
		return "image"
	}
	return "value"
}

// walkParagraphs calls fn for every typed paragraph in blocks, including
// those in table cells, with its location.
func walkParagraphs(blocks []wml.BlockElement, loc Location, fn func(*wml.CT_P, Location)) {
	var np, nt int
	for _, b := range blocks {
		switch el := b.(type) {
		case *wml.CT_P:
			fn(el, loc.child("p", np))
			np++
		case *wml.CT_Tbl:
			tloc := loc.child("tbl", nt)
			nr := 0
			for _, r := range el.Children {
				row, ok := r.(*wml.CT_Tr)
				if !ok {
					continue
				}
				rloc := tloc.child("tr", nr)
				nc := 0
				for _, c := range row.Children {
					if cell, ok := c.(*wml.CT_Tc); ok {
						walkParagraphs(cell.Children, rloc.child("tc", nc), fn)
						nc++
					}
				}
				nr++
			}
			nt++
		}
	}
}

// paragraphTexts returns the w:t elements of a paragraph in document order,
// including those inside hyperlinks.
func paragraphTexts(children []wml.InlineElement) []*wml.CT_Text {
	var out []*wml.CT_Text
	for _, c := range children {
		switch el := c.(type) {
		case *wml.CT_R:
			for _, rc := range el.Children {
				if t, ok := rc.(*wml.CT_Text); ok {
					out = append(out, t)
				}
			}
		case *wml.CT_Hyperlink:
			out = append(out, paragraphTexts(el.Children)...)
		}
	}
	return out
}
