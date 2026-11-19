package textmerge

import (
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// CanDelete reports whether DeleteRange can remove flat[start:end]. flat
// and pieces must come from Flatten(children, …) on the current children.
//
// A range can hold text, tabs, breaks, symbols, hyphens and drawings, which
// are removed with it, and bookmarks whose start and end both lie inside.
// It cannot hold a field character or instruction, a hyperlink, a content
// control, a tracked change or other raw inline, text Flatten marked as
// skipped (a table of contents), or only one end of a bookmark: removing
// part of those would corrupt the document.
func CanDelete(children []wml.InlineElement, pieces []Piece, start, end int) bool {
	for _, p := range inside(pieces, start, end) {
		if p.Kind != PieceBarrier {
			continue
		}
		if p.Content >= 0 {
			run, _ := children[p.Inline].(*wml.CT_R)
			if run == nil || !deletableContent(run.Children[p.Content]) {
				return false
			}
			continue
		}
		if _, ok := bookmarkPartnerInside(children, pieces, p, start, end); !ok {
			return false
		}
	}
	return true
}

// DeleteRange removes flat[start:end] from children: the text in it, the
// run content and bookmark pairs CanDelete allows, and runs it leaves empty.
// Zero-length items Flatten marked as dropped are removed when they sit
// strictly inside the range; the _GoBack bookmark is kept. It returns false,
// and leaves children as they are, when CanDelete is false. Offsets before
// start are unchanged, so several ranges can be deleted right to left,
// flattening again before each.
func DeleteRange(children []wml.InlineElement, pieces []Piece, start, end int) ([]wml.InlineElement, bool) {
	if start >= end {
		return children, true
	}
	if !CanDelete(children, pieces, start, end) {
		return children, false
	}
	type key struct{ inline, content int }
	dropContent := map[key]bool{}
	dropInline := map[int]bool{}
	touched := map[int]bool{}
	for _, p := range inside(pieces, start, end) {
		switch p.Kind {
		case PieceText:
			run, _ := children[p.Inline].(*wml.CT_R)
			txt, _ := run.Children[p.Content].(*wml.CT_Text)
			ls, le := max(start, p.Start)-p.Start, min(end, p.End)-p.Start
			txt.Content = txt.Content[:ls] + txt.Content[le:]
			PreserveSpace(txt)
			if txt.Content == "" {
				dropContent[key{p.Inline, p.Content}] = true
			}
			touched[p.Inline] = true
		case PieceBarrier, PieceDrop:
			if p.Content >= 0 {
				dropContent[key{p.Inline, p.Content}] = true
				touched[p.Inline] = true
			} else {
				dropInline[p.Inline] = true
			}
		case PieceMove:
			// The _GoBack bookmark is harmless where it is.
		}
	}
	out := make([]wml.InlineElement, 0, len(children))
	for i, c := range children {
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
			if len(kept) == 0 {
				continue // everything the run held was in the range
			}
		}
		out = append(out, c)
	}
	return out, true
}

// inside returns the pieces a range removes: text overlapping it, barriers
// within it, and zero-length pieces strictly inside it (one at an edge
// belongs to the neighbouring text).
func inside(pieces []Piece, start, end int) []Piece {
	var out []Piece
	for _, p := range pieces {
		switch {
		case p.Start == p.End:
			if p.Start > start && p.Start < end {
				out = append(out, p)
			}
		case p.Start < end && p.End > start:
			if p.Kind == PieceText || (p.Start >= start && p.End <= end) {
				out = append(out, p)
			}
		}
	}
	return out
}

// deletableContent reports whether run content may be removed with the
// text around it.
func deletableContent(rc wml.RunContentElement) bool {
	switch rc.(type) {
	case *wml.CT_Tab, *wml.CT_Br, *wml.CT_Cr, *wml.CT_NoBreakHyphen, *wml.CT_SoftHyphen, *wml.CT_Sym, *wml.CT_Drawing:
		return true
	}
	return false
}

// bookmarkPartnerInside returns, for an inline-level barrier that is a
// bookmark start or end, the index of its partner when that also lies in
// the range. ok is false for any other inline barrier, or a bookmark whose
// partner is outside the range.
func bookmarkPartnerInside(children []wml.InlineElement, pieces []Piece, p Piece, start, end int) (int, bool) {
	var id int64
	var wantStart bool
	switch el := children[p.Inline].(type) {
	case *wml.CT_BookmarkStart:
		id, wantStart = el.ID, false
	case *wml.CT_BookmarkEnd:
		id, wantStart = el.ID, true
	default:
		return 0, false
	}
	for _, q := range inside(pieces, start, end) {
		if q.Kind != PieceBarrier || q.Content >= 0 {
			continue
		}
		switch el := children[q.Inline].(type) {
		case *wml.CT_BookmarkStart:
			if wantStart && el.ID == id {
				return q.Inline, true
			}
		case *wml.CT_BookmarkEnd:
			if !wantStart && el.ID == id {
				return q.Inline, true
			}
		}
	}
	return 0, false
}
