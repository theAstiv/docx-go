package textmerge

import (
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// Collapse moves the text of flat[start:end] into the w:t that holds its
// first byte, so the range can be read or rewritten in one place. flat and
// pieces must come from Flatten(children, …) on the current children.
//
// Inside the range, PieceDrop items are removed, PieceMove items (the
// _GoBack bookmark) are moved before the run holding the range, and runs
// left empty are removed. The flattened text is unchanged, so offsets of
// later ranges stay valid; piece indexes do change, so callers that
// collapse several ranges use CollapseAll or re-flatten between calls.
//
// It returns false, and leaves children as they are, when a barrier lies
// inside the range: text cannot be merged across a tab, field, hyperlink
// edge or similar.
func Collapse(children []wml.InlineElement, pieces []Piece, flat string, start, end int) ([]wml.InlineElement, bool) {
	out, ok := CollapseAll(children, pieces, flat, []Range{{Start: start, End: end}})
	return out, ok[0]
}

// CollapseAll is Collapse for several ranges at once, in one pass over
// children: ranges must be sorted and must not overlap. ok[i] reports
// whether ranges[i] was collapsed; a range holding a barrier is left as it
// is and the others are still collapsed. Collapsing one range at a time
// would re-flatten the paragraph for each, which is quadratic in a
// paragraph of many split tags.
func CollapseAll(children []wml.InlineElement, pieces []Piece, flat string, ranges []Range) ([]wml.InlineElement, []bool) {
	type span struct{ first, last int }
	spans := make([]span, len(ranges))
	ok := make([]bool, len(ranges))
	pi := 0
	for k, r := range ranges {
		spans[k] = span{-1, -1}
		// Piece ends never decrease, so the pieces that end at or before
		// this range's start (none of which overlap it or any later range)
		// are skipped once, across all ranges.
		for pi < len(pieces) && pieces[pi].End <= r.Start {
			pi++
		}
		first, last := -1, -1
		for j := pi; j < len(pieces) && pieces[j].Start < r.End; j++ {
			if pieces[j].Kind == PieceText && pieces[j].End > r.Start {
				if first < 0 {
					first = j
				}
				last = j
			}
		}
		ok[k] = true
		if first < 0 || first == last {
			continue // empty, or already in one w:t
		}
		for i := first + 1; i < last; i++ {
			if pieces[i].Kind == PieceBarrier {
				ok[k] = false
			}
		}
		if ok[k] {
			spans[k] = span{first, last}
		}
	}

	textAt := func(p Piece) *wml.CT_Text {
		run, _ := children[p.Inline].(*wml.CT_R)
		txt, _ := run.Children[p.Content].(*wml.CT_Text)
		return txt
	}
	type key struct{ inline, content int }
	dropContent := map[key]bool{}
	dropInline := map[int]bool{}
	touched := map[int]bool{}
	firstInline := map[int]bool{}
	moved := map[int][]wml.InlineElement{}
	// Right to left: a w:t can hold the end of one range and the start of
	// the next, and editing the later range first keeps the earlier one's
	// offsets into that w:t valid (it only keeps text before the later
	// range's start).
	for k := len(ranges) - 1; k >= 0; k-- {
		sp := spans[k]
		if sp.first < 0 {
			continue
		}
		start, end := ranges[k].Start, ranges[k].End
		fp, lp := pieces[sp.first], pieces[sp.last]
		ft, lt := textAt(fp), textAt(lp)
		ft.Content = ft.Content[:start-fp.Start] + flat[start:end]
		lt.Content = lt.Content[end-lp.Start:]
		PreserveSpace(ft)
		PreserveSpace(lt)
		firstInline[fp.Inline] = true
		for i := sp.first + 1; i <= sp.last; i++ {
			p := pieces[i]
			switch p.Kind {
			case PieceText:
				if i < sp.last {
					textAt(p).Content = ""
				}
				if textAt(p).Content == "" {
					dropContent[key{p.Inline, p.Content}] = true
				}
				touched[p.Inline] = true
			case PieceDrop:
				if p.Content >= 0 {
					dropContent[key{p.Inline, p.Content}] = true
					touched[p.Inline] = true
				} else {
					dropInline[p.Inline] = true
				}
			case PieceMove:
				moved[fp.Inline] = append(moved[fp.Inline], children[p.Inline])
				dropInline[p.Inline] = true
			case PieceBarrier:
				// Unreachable: such ranges have no span.
			}
		}
	}
	if len(touched) == 0 && len(dropInline) == 0 {
		return children, ok
	}

	out := make([]wml.InlineElement, 0, len(children))
	for i, c := range children {
		out = append(out, moved[i]...)
		if dropInline[i] {
			continue
		}
		if run, isRun := c.(*wml.CT_R); isRun && touched[i] {
			kept := run.Children[:0:0]
			for k, rc := range run.Children {
				if !dropContent[key{i, k}] {
					kept = append(kept, rc)
				}
			}
			run.Children = kept
			if len(kept) == 0 && !firstInline[i] {
				continue // the run held only text that moved into a range
			}
		}
		out = append(out, c)
	}
	return out, ok
}

// PreserveSpace marks text whose edges are whitespace, which XML would
// otherwise let a reader collapse. It never removes an existing mark.
func PreserveSpace(t *wml.CT_Text) {
	if t.Content == "" {
		return
	}
	if strings.TrimSpace(t.Content[:1]) == "" || strings.TrimSpace(t.Content[len(t.Content)-1:]) == "" {
		t.Space = "preserve"
	}
}
