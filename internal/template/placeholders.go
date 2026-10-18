package template

import (
	"fmt"
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/raw"
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
		c := &collector{}
		c.blocks(blocks, loc)
		out = append(out, c.out...)
	}
	return out, issues, nil
}

// collector gathers the tags of typed and raw content in document order.
type collector struct {
	out []Placeholder
}

func (c *collector) text(s string, loc Location) {
	if !strings.Contains(s, "{{") {
		return
	}
	tags, _ := lexTags(s)
	for _, t := range tags {
		c.out = append(c.out, Placeholder{Kind: kindName(t), Path: t.path, Tag: t.text, Location: loc})
	}
}

func (c *collector) raw(re *raw.RawElement, loc Location) {
	for _, s := range rawTexts(re) {
		c.text(s, loc)
	}
}

func (c *collector) blocks(blocks []wml.BlockElement, loc Location) {
	locs := blockLocs(blocks, loc)
	for i, b := range blocks {
		switch el := b.(type) {
		case *wml.CT_P:
			c.inlines(el.Children, locs[i])
		case *wml.CT_Tbl:
			c.table(el, locs[i])
		case *wml.RawBlock:
			c.raw(el.E, locs[i])
		}
	}
}

func (c *collector) table(t *wml.CT_Tbl, loc Location) {
	nr := 0
	for _, r := range t.Children {
		switch row := r.(type) {
		case *wml.RawTableRow:
			c.raw(row.E, loc)
		case *wml.CT_Tr:
			rloc := loc.child("tr", nr)
			nc := 0
			for _, cl := range row.Children {
				switch cell := cl.(type) {
				case *wml.RawTableCell:
					c.raw(cell.E, rloc)
				case *wml.CT_Tc:
					c.blocks(cell.Children, rloc.child("tc", nc))
					nc++
				}
			}
			nr++
		}
	}
}

func (c *collector) inlines(children []wml.InlineElement, loc Location) {
	for _, ch := range children {
		switch el := ch.(type) {
		case *wml.CT_R:
			for _, rc := range el.Children {
				switch r := rc.(type) {
				case *wml.CT_Text:
					c.text(r.Content, loc)
				case *wml.RawRunContent:
					c.raw(r.E, loc)
				case *wml.CT_Drawing:
					for _, re := range r.Rest {
						c.raw(re, loc)
					}
					if in := r.Inline; in != nil && in.Graphic != nil && in.Graphic.GraphicData != nil {
						for _, re := range in.Graphic.GraphicData.Rest {
							c.raw(re, loc)
						}
					}
				}
			}
		case *wml.CT_Hyperlink:
			c.inlines(el.Children, loc)
		case *wml.RawInline:
			c.raw(el.E, loc)
		}
	}
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
