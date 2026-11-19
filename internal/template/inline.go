package template

import (
	"sort"

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

// pairedInText reports whether the block tag at index i of tags (block tags
// only) pairs inside the same text, and returns its partner's index.
func pairedInText(tags []tag, i int) (int, bool) {
	roots, _ := inlinePairs(tags)
	var find func([]*inlinePair) (int, bool)
	find = func(ps []*inlinePair) (int, bool) {
		for _, p := range ps {
			if p.open == i {
				return p.close, true
			}
			if p.close == i {
				return p.open, true
			}
			if j, ok := find(p.children); ok {
				return j, true
			}
		}
		return 0, false
	}
	return find(roots)
}

const (
	inlineEachDetail = "{{#each}} cannot repeat part of a paragraph; put it alone in a paragraph or at a row's edges"
	// inlineBoundaryDetail explains a range DeleteRange refused.
	inlineBoundaryDetail = "an inline block cannot remove part of a field, hyperlink, content control or tracked change, " +
		"or one end of a bookmark; move the tags, or put them alone in their own paragraphs"
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
	if !en.planInline(roots, tags, children, pieces, sc, loc, &cuts) {
		return children
	}
	// Right to left: a deletion leaves the offsets before it unchanged.
	sort.Slice(cuts, func(a, b int) bool { return cuts[a].start > cuts[b].start })
	for _, c := range cuts {
		_, pieces = textmerge.Flatten(children, en.skip)
		var ok bool
		children, ok = textmerge.DeleteRange(children, pieces, c.start, c.end)
		if !ok {
			// planInline checked every cut against the same pieces.
			en.fail(loc, c.open, ErrSplitAcrossBoundary, inlineBoundaryDetail)
			return children
		}
	}
	return children
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
		case en.condition(open, sc):
			chosen = keepThen
		default:
			chosen = keepElse
		}
		for _, c := range chosen {
			if !textmerge.CanDelete(children, pieces, c.start, c.end) {
				en.fail(loc, open, ErrSplitAcrossBoundary, inlineBoundaryDetail)
				return false
			}
		}
		if en.check {
			ok = en.planInline(r.children, tags, children, pieces, sc, loc, cuts) && ok
			continue
		}
		*cuts = append(*cuts, chosen...)
		// Ranges inside the kept branch are expanded too; those in the
		// removed branch go with it.
		lo, hi := open.end, thenEnd
		if !en.condition(open, sc) {
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
