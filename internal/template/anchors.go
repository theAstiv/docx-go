package template

import (
	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// Block and row ranges drop whole paragraphs and rows: the marker
// paragraphs, the branch the data does not take, an empty loop's body.
// Inline ranges check every cut with textmerge.CanDelete; for blocks the
// same promises are kept in two steps:
//
//   - the structure check (checkBlockDrop) refuses a range whose branches
//     or marker paragraphs hold part of a field, or whose marker paragraphs
//     hold a drawing or object (it would be deleted with the tag);
//   - after execution (cleanStoryAnchors), a bookmark or comment-range
//     anchor whose partner a dropped block took is removed too, as
//     docx Remove does, and a removed bookmark that a cross-reference
//     points at is reported like an inline one.

// storyAnchors is what a story held before execution.
type storyAnchors struct {
	// lone are the anchors that already lacked a partner (left alone).
	lone map[wml.MarkKind]map[int64]bool
	// names are the bookmark names.
	names map[string]bool
}

func anchorsOf(blocks []wml.BlockElement) storyAnchors {
	return storyAnchors{lone: loneAnchors(blocks), names: bookmarkNames(blocks)}
}

// loneAnchors returns, by kind, the anchors in blocks whose partner is not
// there.
func loneAnchors(blocks []wml.BlockElement) map[wml.MarkKind]map[int64]bool {
	var marks []wml.Mark
	for _, b := range blocks {
		marks = append(marks, wml.MarksIn(b)...)
	}
	// OrphanedPartners is keyed by the missing partner's kind; the anchor
	// that is present has the other kind.
	out := map[wml.MarkKind]map[int64]bool{}
	for kind, ids := range wml.OrphanedPartners(marks) {
		out[partnerKind(kind)] = ids
	}
	return out
}

func partnerKind(k wml.MarkKind) wml.MarkKind {
	switch k {
	case wml.MarkBookmarkStart:
		return wml.MarkBookmarkEnd
	case wml.MarkBookmarkEnd:
		return wml.MarkBookmarkStart
	case wml.MarkCommentStart:
		return wml.MarkCommentEnd
	case wml.MarkCommentEnd:
		return wml.MarkCommentStart
	}
	return k
}

func bookmarkNames(blocks []wml.BlockElement) map[string]bool {
	scan := new(wml.IDScan)
	wml.ScanBlocks(blocks, scan)
	names := make(map[string]bool, len(scan.Bookmarks))
	for _, b := range scan.Bookmarks {
		names[b.Name] = true
	}
	return names
}

// cleanStoryAnchors removes the anchors execution left without a partner
// and records the bookmarks it removed, so warnRemovedReferences can report
// the ones a cross-reference points at.
func (en *engine) cleanStoryAnchors(blocks *[]wml.BlockElement, before storyAnchors, loc Location) {
	lone := loneAnchors(*blocks)
	for kind, ids := range lone {
		for id := range ids {
			if before.lone[kind][id] {
				delete(ids, id)
			}
		}
	}
	wml.RemoveAnchors(blocks, lone)

	known := map[string]bool{}
	for _, b := range en.removedBookmarks {
		known[b.name] = true
	}
	after := bookmarkNames(*blocks)
	for name := range before.names {
		if !after[name] && !known[name] {
			en.removedBookmarks = append(en.removedBookmarks, removedBookmark{name: name, loc: loc, open: tag{}})
		}
	}
}

// checkBlockDrop reports, in the structure check, a block range whose
// dropped content would leave something broken: a branch or marker
// paragraph holding part of a field, or a marker paragraph holding a
// drawing or object. branchesOK says whether the branches hold whole fields
// (blockPlan.fieldsOK).
func (en *engine) checkBlockDrop(branchesOK bool, markers []*wml.CT_P, loc Location, open tag) {
	if !branchesOK {
		en.fail(loc, open, ErrSplitAcrossBoundary, blockFieldDetail)
		return
	}
	for _, p := range markers {
		if p == nil {
			continue
		}
		if !wml.FieldsBalanced(wml.MarksIn(p)) {
			en.fail(loc, open, ErrSplitAcrossBoundary, blockFieldDetail)
			return
		}
		if holdsGraphic(p.Children) {
			en.fail(loc, open, ErrMisplacedBlock, markerGraphicDetail)
			return
		}
	}
}

// checkRowDrop is checkBlockDrop for row ranges, whose marker tags are cut
// from the edge cells rather than dropped with a paragraph.
func (en *engine) checkRowDrop(segments [][]wml.TableRowElement, loc Location, open tag) {
	for _, seg := range segments {
		var marks []wml.Mark
		for _, r := range seg {
			marks = append(marks, wml.MarksIn(r)...)
		}
		if !wml.FieldsBalanced(marks) {
			en.fail(loc, open, ErrSplitAcrossBoundary, blockFieldDetail)
			return
		}
	}
}

const (
	blockFieldDetail = "each branch of a block, and each block tag's paragraph, must hold whole fields: " +
		"a field (a table of contents, say) that starts inside it must end inside it too"
	markerGraphicDetail = "a block tag's paragraph is removed with the tag, so it cannot also hold a drawing or object; " +
		"move the drawing to another paragraph"
)

// graphicNames are the elements that show something without text.
var graphicNames = map[string]bool{
	"drawing": true, "pict": true, "object": true, "AlternateContent": true,
}

func isGraphic(re *raw.RawElement) bool {
	if re == nil {
		return false
	}
	switch re.QName.Space {
	case ns.W, ns.MC:
		return graphicNames[re.QName.Local]
	}
	return false
}

// holdsGraphic reports whether inline content holds a drawing, VML picture,
// embedded object or alternate content.
func holdsGraphic(children []wml.InlineElement) bool {
	for _, c := range children {
		switch el := c.(type) {
		case *wml.CT_R:
			for _, rc := range el.Children {
				switch x := rc.(type) {
				case *wml.CT_Drawing:
					return true
				case *wml.RawRunContent:
					if rawHoldsGraphic(x.E) {
						return true
					}
				}
			}
		case *wml.CT_Hyperlink:
			if holdsGraphic(el.Children) {
				return true
			}
		case *wml.RawInline:
			if rawHoldsGraphic(el.E) {
				return true
			}
		}
	}
	return false
}

func rawHoldsGraphic(re *raw.RawElement) bool {
	if re == nil {
		return false
	}
	if isGraphic(re) {
		return true
	}
	for _, k := range re.Children {
		if e, ok := k.(*raw.RawElement); ok && rawHoldsGraphic(e) {
			return true
		}
	}
	return false
}
