package template

import (
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
	"github.com/theAstiv/docx-go/internal/textmerge"
)

// Word splits text into runs whenever formatting, spell-check state or
// revision ids change, so a tag the author typed in one go often arrives as
// "{{cli" + "ent.na" + "me}}" in three runs. normalize rewrites each
// paragraph so every well-formed tag sits inside one w:t, keeping the
// formatting of the run that holds its opening "{{".
//
// Between the pieces of one tag:
//   - w:proofErr (spell-check marks) and w:lastRenderedPageBreak are dropped;
//   - the _GoBack bookmark (Word's last-edit marker) is moved before the tag;
//   - anything else (field characters, other bookmarks, hyperlink edges,
//     content controls, tracked changes, drawings) makes the tag an
//     ErrSplitAcrossBoundary issue and it is left as it is.
//
// Merging only moves text between w:t elements and removes zero-length
// markers, so the paragraph's text, and every tag offset, is unchanged.
// The flatten and collapse steps live in internal/textmerge, shared with
// docx.Replace.

// normalizeInlines merges split tags in children (recursing into
// hyperlinks) and returns the new sequence and the malformed tags found.
func normalizeInlines(children []wml.InlineElement, skip skipSet) ([]wml.InlineElement, []badTag) {
	var bad []badTag
	for _, c := range children {
		if h, ok := c.(*wml.CT_Hyperlink); ok {
			var hb []badTag
			h.Children, hb = normalizeInlines(h.Children, skip)
			bad = append(bad, hb...)
		}
	}
	text, _ := textmerge.Flatten(children, skip)
	if !strings.Contains(text, "{{") {
		return children, bad
	}
	tags, lexBad := lexTags(text)
	bad = append(bad, lexBad...)
	if len(tags) == 0 {
		return children, bad
	}
	// One pass for every tag: collapsing them one by one re-flattens the
	// paragraph each time, quadratic in a paragraph of many split tags. A
	// well-formed tag holds no barrier (lexTags rejects those), so every
	// range collapses.
	flat, pieces := textmerge.Flatten(children, skip)
	ranges := make([]textmerge.Range, len(tags))
	for i, t := range tags {
		ranges[i] = textmerge.Range{Start: t.start, End: t.end}
	}
	children, _ = textmerge.CollapseAll(children, pieces, flat, ranges)
	return children, bad
}

// normalizeStory normalizes a story's blocks. It returns the text to leave
// alone (tables of contents, see tocskip.go) and an issue per malformed tag.
func normalizeStory(blocks []wml.BlockElement, loc Location) (skipSet, []Issue) {
	skip := tocSkips(blocks)
	return skip, normalizeBlocks(blocks, loc, skip)
}

// normalizeBlocks normalizes every paragraph in blocks, recursing into
// tables, and returns an issue per malformed tag. Raw blocks (content
// controls and the like) are left to the raw pass.
func normalizeBlocks(blocks []wml.BlockElement, loc Location, skip skipSet) []Issue {
	var issues []Issue
	var np, nt int
	for _, b := range blocks {
		switch el := b.(type) {
		case *wml.CT_P:
			var bad []badTag
			el.Children, bad = normalizeInlines(el.Children, skip)
			ploc := loc.child("p", np)
			for _, bt := range bad {
				issues = append(issues, issueErr(ploc, bt.text, bt.err, bt.detail))
			}
			np++
		case *wml.CT_Tbl:
			issues = append(issues, normalizeTable(el, loc.child("tbl", nt), skip)...)
			nt++
		}
	}
	return issues
}

func normalizeTable(t *wml.CT_Tbl, loc Location, skip skipSet) []Issue {
	var issues []Issue
	nr := 0
	for _, r := range t.Children {
		row, ok := r.(*wml.CT_Tr)
		if !ok {
			continue
		}
		rloc := loc.child("tr", nr)
		nc := 0
		for _, c := range row.Children {
			if cell, ok := c.(*wml.CT_Tc); ok {
				issues = append(issues, normalizeBlocks(cell.Children, rloc.child("tc", nc), skip)...)
				nc++
			}
		}
		nr++
	}
	return issues
}
