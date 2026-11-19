package textmerge

import (
	"sort"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// Range is a span [Start, End) of flattened text.
type Range struct{ Start, End int }

// CanDelete reports whether DeleteRanges can remove flat[start:end]. flat
// and pieces must come from Flatten(children, …) on the current children.
//
// A range can remove text and whole elements: tabs, breaks, symbols,
// hyphens, drawings, hyperlinks, content controls, tracked changes and
// complete fields. It cannot leave anything broken behind, so it refuses a
// range that holds only part of a field (a field character or instruction
// without the rest of its field), one end of a bookmark or comment range,
// a footnote, endnote or comment reference (the note or comment would be
// left without its anchor), or text Flatten was told to skip.
func CanDelete(children []wml.InlineElement, pieces []Piece, start, end int) bool {
	var marks []wml.Mark
	depth := 0 // field nesting among the removed run content, in order
	for _, p := range inside(pieces, start, end) {
		if p.Kind != PieceBarrier {
			continue
		}
		if p.Content < 0 {
			el := children[p.Inline]
			if hasReference(el) {
				return false
			}
			marks = append(marks, wml.MarksIn(el)...)
			continue
		}
		run, _ := children[p.Inline].(*wml.CT_R)
		if run == nil {
			return false
		}
		switch rc := run.Children[p.Content].(type) {
		case *wml.CT_Text:
			return false // text Flatten was told to skip (a table of contents)
		case *wml.CT_FldChar:
			marks = append(marks, wml.Mark{Kind: wml.MarkFldChar, ID: 0, FldCharType: rc.FldCharType})
			switch rc.FldCharType {
			case wml.FldCharBegin:
				depth++
			case wml.FldCharEnd:
				depth--
			}
		case *wml.CT_InstrText:
			if depth <= 0 {
				return false // an instruction whose field begins outside the range
			}
		case *wml.RawRunContent:
			if rc.E == nil || isReference(rc.E) || hasReference(&wml.RawInline{E: rc.E}) {
				return false
			}
			if rc.E.QName == ns.Qn("w:instrText") && depth <= 0 {
				return false
			}
			marks = append(marks, wml.MarksIn(&wml.RawInline{E: rc.E})...)
		}
	}
	return wml.FieldsBalanced(marks) && len(wml.OrphanedPartners(marks)) == 0
}

// DeleteRange removes flat[start:end] from children; see DeleteRanges.
func DeleteRange(children []wml.InlineElement, pieces []Piece, start, end int) ([]wml.InlineElement, bool) {
	return DeleteRanges(children, pieces, []Range{{start, end}})
}

// DeleteRanges removes several disjoint ranges of flattened text from
// children in one pass: their text, the elements CanDelete allows, and runs
// left empty. Zero-length items Flatten marked as dropped are removed when
// they sit strictly inside a range; the _GoBack bookmark is kept. pieces
// must come from Flatten(children, …). It returns false, and leaves
// children as they are, when CanDelete refuses any of the ranges.
func DeleteRanges(children []wml.InlineElement, pieces []Piece, ranges []Range) ([]wml.InlineElement, bool) {
	rs := make([]Range, 0, len(ranges))
	for _, r := range ranges {
		if r.Start < r.End {
			rs = append(rs, r)
		}
	}
	if len(rs) == 0 {
		return children, true
	}
	for _, r := range rs {
		if !CanDelete(children, pieces, r.Start, r.End) {
			return children, false
		}
	}
	// Right to left: trimming a w:t leaves the offsets before the cut valid.
	sort.Slice(rs, func(a, b int) bool { return rs[a].Start > rs[b].Start })

	type key struct{ inline, content int }
	dropContent := map[key]bool{}
	dropInline := map[int]bool{}
	touched := map[int]bool{}
	var trimmed []*wml.CT_Text
	for _, r := range rs {
		for _, p := range inside(pieces, r.Start, r.End) {
			switch p.Kind {
			case PieceText:
				run, _ := children[p.Inline].(*wml.CT_R)
				txt, _ := run.Children[p.Content].(*wml.CT_Text)
				ls, le := max(r.Start, p.Start)-p.Start, min(r.End, p.End)-p.Start
				txt.Content = txt.Content[:ls] + txt.Content[le:]
				trimmed = append(trimmed, txt)
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
	}
	for _, t := range trimmed {
		PreserveSpace(t)
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
				continue // everything the run held was in a range
			}
		}
		out = append(out, c)
	}
	return out, true
}

// RemovedBookmarks returns the names of the bookmarks a deletion of
// flat[start:end] removes, including those inside removed hyperlinks and
// content controls. _GoBack is never removed.
func RemovedBookmarks(children []wml.InlineElement, pieces []Piece, start, end int) []string {
	var names []string
	for _, p := range inside(pieces, start, end) {
		if p.Kind != PieceBarrier || p.Content >= 0 {
			continue
		}
		names = append(names, bookmarkNames(children[p.Inline])...)
	}
	return names
}

func bookmarkNames(el wml.InlineElement) []string {
	switch x := el.(type) {
	case *wml.CT_BookmarkStart:
		return []string{x.Name}
	case *wml.CT_Hyperlink:
		var out []string
		for _, c := range x.Children {
			out = append(out, bookmarkNames(c)...)
		}
		return out
	case *wml.RawInline:
		var out []string
		walkRaw(x.E, func(re *raw.RawElement) {
			if re.QName == ns.Qn("w:bookmarkStart") {
				for _, a := range re.Attrs {
					if a.Name.Local == "name" {
						out = append(out, a.Value)
					}
				}
			}
		})
		return out
	}
	return nil
}

// inside returns the pieces a range removes: text overlapping it, barriers
// within it, and zero-length pieces strictly inside it (one at an edge
// belongs to the neighbouring text). Pieces are in offset order, so it
// starts at the first piece that can overlap.
func inside(pieces []Piece, start, end int) []Piece {
	i := sort.Search(len(pieces), func(i int) bool { return pieces[i].End >= start })
	var out []Piece
	for ; i < len(pieces) && pieces[i].Start <= end; i++ {
		p := pieces[i]
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

// referenceNames are anchors of content held in another part; removing one
// leaves its footnote, endnote or comment without a place in the text.
var referenceNames = map[string]bool{
	"footnoteReference": true, "endnoteReference": true, "commentReference": true,
}

func isReference(re *raw.RawElement) bool {
	return re != nil && re.QName.Space == ns.W && referenceNames[re.QName.Local]
}

// hasReference reports whether an inline element holds a note or comment
// reference anywhere inside it.
func hasReference(el wml.InlineElement) bool {
	found := false
	var inlines func([]wml.InlineElement)
	visit := func(re *raw.RawElement) {
		walkRaw(re, func(e *raw.RawElement) {
			if isReference(e) {
				found = true
			}
		})
	}
	inlines = func(children []wml.InlineElement) {
		for _, c := range children {
			switch x := c.(type) {
			case *wml.CT_R:
				for _, rc := range x.Children {
					if r, ok := rc.(*wml.RawRunContent); ok {
						visit(r.E)
					}
				}
			case *wml.CT_Hyperlink:
				inlines(x.Children)
			case *wml.RawInline:
				visit(x.E)
			}
		}
	}
	inlines([]wml.InlineElement{el})
	return found
}

// walkRaw calls fn on re and every raw element below it.
func walkRaw(re *raw.RawElement, fn func(*raw.RawElement)) {
	if re == nil {
		return
	}
	fn(re)
	for _, c := range re.Children {
		if k, ok := c.(*raw.RawElement); ok {
			walkRaw(k, fn)
		}
	}
}
