package textmerge

import (
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// Barrier characters stand in for non-text content in flattened text.
// Neither can occur in XML text, so they never collide with document content.
const (
	// BarrierBreak marks a tab, line break or other run content that text
	// can sit next to but not be merged across.
	BarrierBreak = '\x00'
	// BarrierBoundary marks a field character, bookmark, hyperlink edge,
	// content control or other element text must not be merged across.
	BarrierBoundary = '\x01'
)

// PieceKind classifies one item of a flattened inline sequence.
type PieceKind int

const (
	// PieceText is a w:t; its text is part of the flattened string.
	PieceText PieceKind = iota
	// PieceBarrier is one barrier character in the flattened string.
	PieceBarrier
	// PieceDrop is zero-length and removed when it sits inside a merged range.
	PieceDrop
	// PieceMove is zero-length and moved before a merged range it sits inside.
	PieceMove
)

// Piece is one item of a flattened inline sequence.
type Piece struct {
	Kind PieceKind
	// Start and End are byte offsets in the flattened text.
	Start, End int
	// Inline is the index of the inline child holding the piece.
	Inline int
	// Content is the index in the run's children; -1 for inline-level pieces.
	Content int
}

// Flatten turns an inline sequence into text plus the pieces it came from.
//
// Text runs contribute their text. Tabs, breaks, hyphens and symbols become
// BarrierBreak; field characters, instructions, drawings, bookmarks,
// hyperlinks and other raw inlines become BarrierBoundary. Spell-check marks
// (w:proofErr) and w:lastRenderedPageBreak are PieceDrop, and Word's _GoBack
// bookmark is PieceMove. Text in skip (a table of contents) becomes a
// boundary. Hyperlink children are not descended into; flatten them on
// their own.
func Flatten(children []wml.InlineElement, skip SkipSet) (string, []Piece) {
	goBack := map[int64]bool{}
	for _, c := range children {
		if bs, ok := c.(*wml.CT_BookmarkStart); ok && bs.Name == "_GoBack" {
			goBack[bs.ID] = true
		}
	}
	var sb strings.Builder
	var pieces []Piece
	add := func(kind PieceKind, inline, content int, text string) {
		start := sb.Len()
		sb.WriteString(text)
		pieces = append(pieces, Piece{Kind: kind, Start: start, End: sb.Len(), Inline: inline, Content: content})
	}
	boundary := string(BarrierBoundary)
	for i, c := range children {
		switch el := c.(type) {
		case *wml.CT_R:
			for k, rc := range el.Children {
				switch r := rc.(type) {
				case *wml.CT_Text:
					if skip[r] {
						add(PieceBarrier, i, k, boundary)
					} else {
						add(PieceText, i, k, r.Content)
					}
				case *wml.CT_Tab, *wml.CT_Br, *wml.CT_Cr, *wml.CT_NoBreakHyphen, *wml.CT_SoftHyphen, *wml.CT_Sym:
					add(PieceBarrier, i, k, string(BarrierBreak))
				case *wml.RawRunContent:
					if r.E != nil && r.E.QName == ns.Qn("w:lastRenderedPageBreak") {
						add(PieceDrop, i, k, "")
					} else {
						add(PieceBarrier, i, k, boundary)
					}
				default: // field characters, instructions, drawings
					add(PieceBarrier, i, k, boundary)
				}
			}
		case *wml.CT_BookmarkStart:
			if el.Name == "_GoBack" {
				add(PieceMove, i, -1, "")
			} else {
				add(PieceBarrier, i, -1, boundary)
			}
		case *wml.CT_BookmarkEnd:
			if goBack[el.ID] {
				add(PieceMove, i, -1, "")
			} else {
				add(PieceBarrier, i, -1, boundary)
			}
		case *wml.RawInline:
			if el.E != nil && el.E.QName == ns.Qn("w:proofErr") {
				add(PieceDrop, i, -1, "")
			} else {
				add(PieceBarrier, i, -1, boundary)
			}
		default: // hyperlinks (flattened on their own) and anything else
			add(PieceBarrier, i, -1, boundary)
		}
	}
	return sb.String(), pieces
}
