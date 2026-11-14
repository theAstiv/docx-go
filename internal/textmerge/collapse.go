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
// later ranges stay valid; callers that collapse several ranges must
// re-flatten between calls because piece indexes change.
//
// It returns false, and leaves children as they are, when a barrier lies
// inside the range: text cannot be merged across a tab, field, hyperlink
// edge or similar.
func Collapse(children []wml.InlineElement, pieces []Piece, flat string, start, end int) ([]wml.InlineElement, bool) {
	first, last := -1, -1
	for i, p := range pieces {
		if p.Kind == PieceText && p.Start < end && p.End > start {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 0 || first == last {
		return children, true // empty, or already in one w:t
	}
	for i := first + 1; i < last; i++ {
		if pieces[i].Kind == PieceBarrier {
			return children, false
		}
	}

	textAt := func(p Piece) *wml.CT_Text {
		run, _ := children[p.Inline].(*wml.CT_R)
		txt, _ := run.Children[p.Content].(*wml.CT_Text)
		return txt
	}
	fp, lp := pieces[first], pieces[last]
	ft, lt := textAt(fp), textAt(lp)
	ft.Content = ft.Content[:start-fp.Start] + flat[start:end]
	lt.Content = lt.Content[end-lp.Start:]
	PreserveSpace(ft)
	PreserveSpace(lt)

	type key struct{ inline, content int }
	dropContent := map[key]bool{}
	dropInline := map[int]bool{}
	touched := map[int]bool{}
	var moved []wml.InlineElement
	for i := first + 1; i <= last; i++ {
		p := pieces[i]
		switch p.Kind {
		case PieceText:
			if i < last {
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
			moved = append(moved, children[p.Inline])
			dropInline[p.Inline] = true
		case PieceBarrier:
			// Unreachable: rejected above.
		}
	}

	out := make([]wml.InlineElement, 0, len(children))
	for i, c := range children {
		if i == fp.Inline {
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
			if len(kept) == 0 && i != fp.Inline {
				continue // the run held only text that moved into the range
			}
		}
		out = append(out, c)
	}
	return out, true
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
