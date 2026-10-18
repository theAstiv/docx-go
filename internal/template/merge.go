package template

import (
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
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

type pieceKind int

const (
	// pieceText is a w:t; its text is part of the flattened string.
	pieceText pieceKind = iota
	// pieceBarrier is one barrier character in the flattened string.
	pieceBarrier
	// pieceDrop is zero-length and removed when it sits inside a tag.
	pieceDrop
	// pieceMove is zero-length and moved before the tag when inside one.
	pieceMove
)

// piece is one item of a flattened inline sequence.
type piece struct {
	kind pieceKind
	// start and end are byte offsets in the flattened text.
	start, end int
	// inline is the index of the inline child holding the piece.
	inline int
	// content is the index in the run's children; -1 for inline-level pieces.
	content int
}

// flatten turns an inline sequence into text plus the pieces it came from.
// Text in skip (a table of contents) becomes a boundary: it is not template
// text, and a tag cannot reach across it.
func flatten(children []wml.InlineElement, skip skipSet) (string, []piece) {
	goBack := map[int64]bool{}
	for _, c := range children {
		if bs, ok := c.(*wml.CT_BookmarkStart); ok && bs.Name == "_GoBack" {
			goBack[bs.ID] = true
		}
	}
	var sb strings.Builder
	var pieces []piece
	add := func(kind pieceKind, inline, content int, text string) {
		start := sb.Len()
		sb.WriteString(text)
		pieces = append(pieces, piece{kind: kind, start: start, end: sb.Len(), inline: inline, content: content})
	}
	boundary := string(barrierBoundary)
	for i, c := range children {
		switch el := c.(type) {
		case *wml.CT_R:
			for k, rc := range el.Children {
				switch r := rc.(type) {
				case *wml.CT_Text:
					if skip[r] {
						add(pieceBarrier, i, k, boundary)
					} else {
						add(pieceText, i, k, r.Content)
					}
				case *wml.CT_Tab, *wml.CT_Br, *wml.CT_Cr, *wml.CT_NoBreakHyphen, *wml.CT_SoftHyphen, *wml.CT_Sym:
					add(pieceBarrier, i, k, string(barrierBreak))
				case *wml.RawRunContent:
					if r.E != nil && r.E.QName == ns.Qn("w:lastRenderedPageBreak") {
						add(pieceDrop, i, k, "")
					} else {
						add(pieceBarrier, i, k, boundary)
					}
				default: // field characters, instructions, drawings
					add(pieceBarrier, i, k, boundary)
				}
			}
		case *wml.CT_BookmarkStart:
			if el.Name == "_GoBack" {
				add(pieceMove, i, -1, "")
			} else {
				add(pieceBarrier, i, -1, boundary)
			}
		case *wml.CT_BookmarkEnd:
			if goBack[el.ID] {
				add(pieceMove, i, -1, "")
			} else {
				add(pieceBarrier, i, -1, boundary)
			}
		case *wml.RawInline:
			if el.E != nil && el.E.QName == ns.Qn("w:proofErr") {
				add(pieceDrop, i, -1, "")
			} else {
				add(pieceBarrier, i, -1, boundary)
			}
		default: // hyperlinks (normalized on their own) and anything else
			add(pieceBarrier, i, -1, boundary)
		}
	}
	return sb.String(), pieces
}

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
	text, _ := flatten(children, skip)
	if !strings.Contains(text, "{{") {
		return children, bad
	}
	tags, lexBad := lexTags(text)
	bad = append(bad, lexBad...)
	for _, t := range tags {
		_, pieces := flatten(children, skip)
		children = mergeTag(children, pieces, t)
	}
	return children, bad
}

// mergeTag collapses the text of one tag into the w:t holding its "{{".
func mergeTag(children []wml.InlineElement, pieces []piece, t tag) []wml.InlineElement {
	first, last := -1, -1
	for i, p := range pieces {
		if p.kind == pieceText && p.start < t.end && p.end > t.start {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 0 || first == last {
		return children // already in one w:t
	}

	textAt := func(p piece) *wml.CT_Text {
		run, _ := children[p.inline].(*wml.CT_R)
		txt, _ := run.Children[p.content].(*wml.CT_Text)
		return txt
	}
	fp, lp := pieces[first], pieces[last]
	ft, lt := textAt(fp), textAt(lp)
	ft.Content = ft.Content[:t.start-fp.start] + t.text
	lt.Content = lt.Content[t.end-lp.start:]
	preserveSpace(ft)
	preserveSpace(lt)

	type key struct{ inline, content int }
	dropContent := map[key]bool{}
	dropInline := map[int]bool{}
	touched := map[int]bool{}
	var moved []wml.InlineElement
	for i := first + 1; i <= last; i++ {
		p := pieces[i]
		switch p.kind {
		case pieceText:
			if i < last {
				textAt(p).Content = ""
			}
			if textAt(p).Content == "" {
				dropContent[key{p.inline, p.content}] = true
			}
			touched[p.inline] = true
		case pieceDrop:
			if p.content >= 0 {
				dropContent[key{p.inline, p.content}] = true
				touched[p.inline] = true
			} else {
				dropInline[p.inline] = true
			}
		case pieceMove:
			moved = append(moved, children[p.inline])
			dropInline[p.inline] = true
		case pieceBarrier:
			// Unreachable: a well-formed tag contains no barrier.
		}
	}

	out := make([]wml.InlineElement, 0, len(children))
	for i, c := range children {
		if i == fp.inline {
			out = append(out, moved...)
		}
		if dropInline[i] {
			continue
		}
		if run, ok := c.(*wml.CT_R); ok && touched[i] {
			kept := run.Children[:0:0]
			for k, rc := range run.Children {
				if !dropContent[key{i, k}] {
					kept = append(kept, rc)
				}
			}
			run.Children = kept
			if len(kept) == 0 && i != fp.inline {
				continue // the run held only text that moved into the tag
			}
		}
		out = append(out, c)
	}
	return out
}

// preserveSpace marks text whose edges are whitespace, which XML would
// otherwise let a reader collapse. It never removes an existing mark.
func preserveSpace(t *wml.CT_Text) {
	if t.Content == "" {
		return
	}
	if strings.TrimSpace(t.Content[:1]) == "" || strings.TrimSpace(t.Content[len(t.Content)-1:]) == "" {
		t.Space = "preserve"
	}
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
