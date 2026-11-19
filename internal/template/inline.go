package template

import (
	"fmt"
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
	"github.com/theAstiv/docx-go/internal/textmerge"
)

// Inline block ranges (P3c.10, CLAUDE.md C13): {{#if x}}text{{/if}} inside
// one paragraph. A paragraph whose whole text is one block tag is a marker
// paragraph (blocks.go); otherwise the block tags of each inline container
// (the paragraph's own children, each hyperlink's children) that pair inside
// it are inline ranges, and the rest are misplaced. Inline ranges keep or
// remove text, not blocks: the paragraph stays.

// inlinePair is one inline range: indexes into a container's block tags.
type inlinePair struct {
	open, els, close int // els is -1 without {{else}}
	// children are the ranges directly inside this one.
	children []*inlinePair
}

// leftover is a block tag that is not part of an inline range, with the
// error to report for it.
type leftover struct {
	t        tag
	sentinel error
	detail   string
}

// inlinePairs pairs block tags (in text order) with a stack. Unlike pair, a
// tag that does not fit is set aside, not fatal: the pairs that do form are
// still ranges. It returns the outermost ranges and the tags left over.
func inlinePairs(tags []tag) ([]*inlinePair, []leftover) {
	var roots []*inlinePair
	var stack []*inlinePair
	var left []leftover
	misplaced := func(t tag) {
		left = append(left, leftover{t: t, sentinel: ErrMisplacedBlock, detail: misplacedDetail})
	}
	for i, t := range tags {
		switch t.kind {
		case tagOpen:
			stack = append(stack, &inlinePair{open: i, els: -1, close: -1})
		case tagElse:
			switch {
			case len(stack) == 0:
				misplaced(t)
			case stack[len(stack)-1].els >= 0:
				top := stack[len(stack)-1]
				left = append(left, leftover{t: t, sentinel: ErrUnbalancedBlock,
					detail: "a second {{else}} for " + tags[top.open].text})
			default:
				stack[len(stack)-1].els = i
			}
		case tagClose:
			if len(stack) == 0 || tags[stack[len(stack)-1].open].block != t.block {
				misplaced(t)
				continue
			}
			p := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			p.close = i
			if len(stack) == 0 {
				roots = append(roots, p)
			} else {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, p)
			}
		}
	}
	for _, p := range stack {
		misplaced(tags[p.open])
		if p.els >= 0 {
			misplaced(tags[p.els])
		}
		// Ranges closed inside an unclosed one are still ranges.
		roots = append(roots, p.children...)
	}
	return roots, left
}

// blockTagsOf returns the block tags among tags.
func blockTagsOf(tags []tag) []tag {
	var out []tag
	for _, t := range tags {
		if isBlockTag(t) {
			out = append(out, t)
		}
	}
	return out
}

// inlineAtEdge reports whether the block tag at a row edge, the first tag
// of the row's first paragraph (first) or the last of its last, is part of
// an inline range and so not a row-range edge (C13). The exception is a
// range without {{else}} that fills a one-paragraph row on its own: that
// stays a one-row row range, as before inline ranges existed. (A row range
// cannot hold an {{else}} in its opening row, so with one the range is
// inline.)
func inlineAtEdge(row *wml.CT_Tr, p *wml.CT_P, first bool) bool {
	text := p.Text()
	all, _ := lexTags(text)
	tags := blockTagsOf(all)
	if len(tags) == 0 {
		return false
	}
	edge := 0
	if !first {
		edge = len(tags) - 1
	}
	roots, _ := inlinePairs(tags)
	// An edge tag that pairs is an outermost range: nothing comes before
	// (or after) it in the text.
	var r *inlinePair
	for _, c := range roots {
		if c.open == edge || c.close == edge {
			r = c
		}
	}
	if r == nil {
		return false
	}
	start := len(text) - len(strings.TrimLeft(text, " \t"))
	end := len(strings.TrimRight(text, " \t"))
	wholeRow := p == edgeParagraph(row, true) && p == edgeParagraph(row, false) &&
		r.els < 0 && tags[r.open].start == start && tags[r.close].end == end
	return !wholeRow
}

const (
	inlineEachDetail = "{{#each}} cannot repeat part of a paragraph; put it alone in a paragraph or at a row's edges"
	// inlineBoundaryDetail explains a range CanDelete refused.
	inlineBoundaryDetail = "an inline block can remove whole fields, hyperlinks and content controls, " +
		"but not part of a field, one end of a bookmark or comment, or a footnote, endnote or comment reference; " +
		"move the tags, or put them alone in their own paragraphs"
)

// expandInline expands the inline ranges of a typed paragraph's containers
// and reports the block tags that are not part of one. It is not used in
// raw content, where block tags are not supported.
func (en *engine) expandInline(children []wml.InlineElement, sc *scope, loc Location) []wml.InlineElement {
	for _, c := range children {
		if h, ok := c.(*wml.CT_Hyperlink); ok {
			h.Children = en.expandInline(h.Children, sc, loc)
		}
	}
	flat, pieces := textmerge.Flatten(children, en.skip)
	all, _ := lexTags(flat) // malformed tags were reported by normalize
	tags := blockTagsOf(all)
	if len(tags) == 0 {
		return children
	}
	roots, left := inlinePairs(tags)
	for _, l := range left {
		en.fail(loc, l.t, l.sentinel, l.detail)
	}
	var cuts []cut
	if !en.planInline(roots, tags, children, pieces, sc, loc, &cuts) || len(cuts) == 0 {
		return children
	}
	ranges := make([]textmerge.Range, len(cuts))
	for i, c := range cuts {
		ranges[i] = textmerge.Range{Start: c.start, End: c.end}
		for _, name := range textmerge.RemovedBookmarks(children, pieces, c.start, c.end) {
			en.removedBookmarks = append(en.removedBookmarks, removedBookmark{name: name, loc: loc, open: c.open})
		}
	}
	out, ok := textmerge.DeleteRanges(children, pieces, ranges)
	if !ok {
		// Unreachable: planInline checked every cut against these pieces.
		en.fail(loc, cuts[0].open, ErrSplitAcrossBoundary, inlineBoundaryDetail)
	}
	return out
}

// removedBookmark is a bookmark an inline range removed.
type removedBookmark struct {
	name string
	loc  Location
	open tag
}

// warnRemovedReferences warns about removed bookmarks that a cross-reference
// or internal link in the filled stories still points at: Word shows
// "Error! Reference source not found." in their place.
func (en *engine) warnRemovedReferences(stories [][]wml.BlockElement) {
	if len(en.removedBookmarks) == 0 {
		return
	}
	refs := map[string]bool{}
	for _, blocks := range stories {
		for name := range wml.ReferencedBookmarks(blocks) {
			refs[name] = true
		}
	}
	for _, b := range en.removedBookmarks {
		if refs[b.name] {
			en.warn(b.loc, b.open, fmt.Sprintf("removed bookmark %q, which a cross-reference or internal link points at; "+
				"Word shows \"Error! Reference source not found.\" there", b.name))
		}
	}
}

// cut is a range of flattened text to delete, and the range it belongs to.
type cut struct {
	start, end int
	open       tag
}

// planInline decides the cuts for ranges and the ranges inside the branches
// they keep. In the structure check it keeps no cuts but checks both
// outcomes of every range, so a range that could not be removed is reported
// whatever the data. It returns false after reporting a problem.
func (en *engine) planInline(ranges []*inlinePair, tags []tag, children []wml.InlineElement,
	pieces []textmerge.Piece, sc *scope, loc Location, cuts *[]cut) bool {
	ok := true
	for _, r := range ranges {
		open, closeTag := tags[r.open], tags[r.close]
		if open.block == "each" {
			en.fail(loc, open, ErrMisplacedBlock, inlineEachDetail)
			ok = false
			continue
		}
		cond := !en.check && en.condition(open, sc)
		// thenEnd is where the then branch stops; elseStart where the else
		// branch starts (both the close tag's start without an else).
		thenEnd, elseStart := closeTag.start, closeTag.start
		if r.els >= 0 {
			thenEnd, elseStart = tags[r.els].start, tags[r.els].end
		}
		keepThen := []cut{{open.start, open.end, open}, {thenEnd, closeTag.end, open}}
		keepElse := []cut{{open.start, elseStart, open}, {closeTag.start, closeTag.end, open}}
		var chosen []cut
		switch {
		case en.check:
			chosen = append(keepThen, keepElse...)
		case cond:
			chosen = keepThen
		default:
			chosen = keepElse
		}
		refused := false
		for _, c := range chosen {
			if !textmerge.CanDelete(children, pieces, c.start, c.end) {
				refused = true
			}
		}
		if refused {
			// Keep going: every refused range is reported in one run.
			en.fail(loc, open, ErrSplitAcrossBoundary, inlineBoundaryDetail)
			ok = false
			continue
		}
		if en.check {
			ok = en.planInline(r.children, tags, children, pieces, sc, loc, cuts) && ok
			continue
		}
		*cuts = append(*cuts, chosen...)
		// Ranges inside the kept branch are expanded too; those in the
		// removed branch go with it.
		lo, hi := open.end, thenEnd
		if !cond {
			lo, hi = elseStart, closeTag.start
		}
		var kept []*inlinePair
		for _, c := range r.children {
			if tags[c.open].start >= lo && tags[c.close].end <= hi {
				kept = append(kept, c)
			}
		}
		ok = en.planInline(kept, tags, children, pieces, sc, loc, cuts) && ok
	}
	return ok
}
