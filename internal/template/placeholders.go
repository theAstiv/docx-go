package template

import (
	"fmt"
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
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
	return PlaceholdersAll(stories, nil)
}

// PlaceholdersAll is Placeholders that also lists the tags in the parts
// data-bound content controls read from, after the stories' tags.
func PlaceholdersAll(stories []Story, boundParts map[string][]byte) ([]Placeholder, []Issue, error) {
	out, issues, err := storyPlaceholders(stories)
	if err != nil {
		return nil, nil, err
	}
	ps, is := xmlPartPlaceholders(boundParts)
	return append(out, ps...), append(issues, is...), nil
}

func storyPlaceholders(stories []Story) ([]Placeholder, []Issue, error) {
	var out []Placeholder
	var issues []Issue
	for _, s := range stories {
		blocks, err := cloneBlocks(*s.Blocks)
		if err != nil {
			return nil, nil, fmt.Errorf("template: copy %s: %w", s.Part, err)
		}
		loc := Location{Part: s.Part}
		skip, normIssues := normalizeStory(blocks, loc)
		issues = append(issues, normIssues...)
		c := &collector{out: nil, issues: nil, skip: skip, err: nil}
		c.blocks(blocks, loc)
		if c.err != nil {
			return nil, nil, c.err
		}
		out = append(out, c.out...)
		issues = append(issues, c.issues...)
	}
	return out, issues, nil
}

// collector gathers the tags of typed and raw content in document order.
type collector struct {
	out    []Placeholder
	issues []Issue
	// skip holds text that is not template text (tables of contents).
	skip skipSet
	err  error
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

// raw collects the tags of raw content. Its paragraphs are typed and
// normalized as Execute does, so a split tag there is found, or reported.
func (c *collector) raw(re *raw.RawElement, loc Location) {
	if re == nil || c.err != nil {
		return
	}
	if re.QName == ns.Qn("w:t") {
		c.text(rawText(re), loc)
		return
	}
	err := walkRawParagraphs(re, loc, false, func(paras []*wml.CT_P, ploc Location) {
		for t := range tocSkipsParagraphs(paras) {
			c.skip[t] = true
		}
		for i, p := range paras {
			ploc := ploc.child("p", i)
			var bad []badTag
			p.Children, bad = normalizeInlines(p.Children, c.skip)
			for _, b := range bad {
				c.issues = append(c.issues, issueErr(ploc, b.text, b.err, b.detail))
			}
			c.inlines(p.Children, ploc)
		}
	}, func(t *raw.RawElement) { c.text(rawText(t), loc) })
	if err != nil {
		c.err = err
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
					if !c.skip[r] {
						c.text(r.Content, loc)
					}
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
