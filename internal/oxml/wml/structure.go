package wml

import (
	"encoding/xml"
	"slices"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
)

// Support for removing content (docx structural editing): finding the range
// anchors and field characters inside a subtree, and deleting the partners a
// removal leaves behind. Both reach typed and raw content alike.

// MarkKind classifies a Mark.
type MarkKind int

// Mark kinds.
const (
	MarkBookmarkStart MarkKind = iota
	MarkBookmarkEnd
	MarkCommentStart
	MarkCommentEnd
	MarkFldChar
	MarkMoveFromStart
	MarkMoveFromEnd
	MarkMoveToStart
	MarkMoveToEnd
	MarkPermStart
	MarkPermEnd
	// MarkFootnoteRef, MarkEndnoteRef and MarkCommentRef are references to
	// a note or comment held in another part; they have no partner in the
	// story.
	MarkFootnoteRef
	MarkEndnoteRef
	MarkCommentRef
)

// Mark is a range anchor, note reference or field character found in
// content.
type Mark struct {
	Kind MarkKind
	// ID is the w:id of an anchor or reference; 0 for field characters. A
	// w:permStart/w:permEnd id is a string in the schema; one that is not a
	// number is not reported.
	ID int64
	// FldCharType is "begin", "separate" or "end" for MarkFldChar.
	FldCharType string
}

// anchorNames maps the anchor element names to their kinds.
var anchorNames = map[xml.Name]MarkKind{
	ns.Qn("w:bookmarkStart"):      MarkBookmarkStart,
	ns.Qn("w:bookmarkEnd"):        MarkBookmarkEnd,
	ns.Qn("w:commentRangeStart"):  MarkCommentStart,
	ns.Qn("w:commentRangeEnd"):    MarkCommentEnd,
	ns.Qn("w:moveFromRangeStart"): MarkMoveFromStart,
	ns.Qn("w:moveFromRangeEnd"):   MarkMoveFromEnd,
	ns.Qn("w:moveToRangeStart"):   MarkMoveToStart,
	ns.Qn("w:moveToRangeEnd"):     MarkMoveToEnd,
	ns.Qn("w:permStart"):          MarkPermStart,
	ns.Qn("w:permEnd"):            MarkPermEnd,
}

// refNames maps the note and comment reference element names to their
// kinds.
var refNames = map[xml.Name]MarkKind{
	ns.Qn("w:footnoteReference"): MarkFootnoteRef,
	ns.Qn("w:endnoteReference"):  MarkEndnoteRef,
	ns.Qn("w:commentReference"):  MarkCommentRef,
}

// anchorPairs lists each range kind's start and end.
var anchorPairs = [][2]MarkKind{
	{MarkBookmarkStart, MarkBookmarkEnd},
	{MarkCommentStart, MarkCommentEnd},
	{MarkMoveFromStart, MarkMoveFromEnd},
	{MarkMoveToStart, MarkMoveToEnd},
	{MarkPermStart, MarkPermEnd},
}

// PartnerKind returns the kind of an anchor's other end: the end for a
// start and the start for an end. Other kinds are returned unchanged.
func PartnerKind(k MarkKind) MarkKind {
	for _, p := range anchorPairs {
		switch k {
		case p[0]:
			return p[1]
		case p[1]:
			return p[0]
		}
	}
	return k
}

// MarksIn returns the marks inside node, in document order. node is a
// *CT_P, *CT_Tbl, *CT_Tr, *CT_R, or a block, inline, row or cell element of
// any kind; other values have none.
func MarksIn(node any) []Mark {
	var m markCollector
	m.node(node)
	return m.out
}

type markCollector struct{ out []Mark }

func (m *markCollector) node(node any) {
	switch el := node.(type) {
	case *CT_P:
		m.inlines(el.Children)
	case *CT_Tbl:
		for _, r := range el.Children {
			m.node(r)
		}
	case *CT_Tr:
		for _, c := range el.Children {
			m.node(c)
		}
	case *CT_Tc:
		for _, b := range el.Children {
			m.node(b)
		}
	case *CT_R:
		m.run(el)
	case *CT_Hyperlink:
		m.inlines(el.Children)
	case *CT_BookmarkStart:
		m.out = append(m.out, Mark{Kind: MarkBookmarkStart, ID: el.ID, FldCharType: ""})
	case *CT_BookmarkEnd:
		m.out = append(m.out, Mark{Kind: MarkBookmarkEnd, ID: el.ID, FldCharType: ""})
	case *RawBlock:
		m.raw(el.E)
	case *RawInline:
		m.raw(el.E)
	case *RawTableRow:
		m.raw(el.E)
	case *RawTableCell:
		m.raw(el.E)
	}
}

func (m *markCollector) inlines(children []InlineElement) {
	for _, c := range children {
		m.node(c)
	}
}

func (m *markCollector) run(r *CT_R) {
	for _, c := range r.Children {
		switch el := c.(type) {
		case *CT_FldChar:
			m.out = append(m.out, Mark{Kind: MarkFldChar, ID: 0, FldCharType: el.FldCharType})
		case *CT_Drawing:
			// A text box in a drawing holds paragraphs of its own.
			for _, re := range drawingRaws(el) {
				m.raw(re)
			}
		case *RawRunContent:
			m.raw(el.E)
		}
	}
}

func (m *markCollector) raw(re *raw.RawElement) {
	if re == nil {
		return
	}
	k, anchor := anchorNames[re.QName]
	if !anchor {
		k, anchor = refNames[re.QName]
	}
	if anchor {
		if id := parseInt64Ptr(attrNS(re.Attrs, ns.W, "id")); id != nil {
			m.out = append(m.out, Mark{Kind: k, ID: *id, FldCharType: ""})
		}
	} else if re.QName == ns.Qn("w:fldChar") {
		m.out = append(m.out, Mark{Kind: MarkFldChar, ID: 0, FldCharType: attrNS(re.Attrs, ns.W, "fldCharType")})
	}
	for _, k := range rawKids(re) {
		m.raw(k)
	}
}

// FieldsBalanced reports whether the field characters in marks form whole
// fields: every begin is closed by an end, no end or separate appears
// outside a field, and no field is left open.
func FieldsBalanced(marks []Mark) bool {
	depth := 0
	for _, mk := range marks {
		if mk.Kind != MarkFldChar {
			continue
		}
		switch mk.FldCharType {
		case FldCharBegin:
			depth++
		case FldCharSeparate:
			if depth == 0 {
				return false
			}
		case FldCharEnd:
			if depth == 0 {
				return false
			}
			depth--
		}
	}
	return depth == 0
}

// OrphanedPartners returns, per anchor kind, the ids whose start or end is
// in marks but whose partner is not: removing the content those marks came
// from leaves the partners dangling elsewhere. The result is keyed by the
// kind of the partner to delete.
func OrphanedPartners(marks []Mark) map[MarkKind]map[int64]bool {
	seen := map[MarkKind]map[int64]bool{}
	for _, mk := range marks {
		if PartnerKind(mk.Kind) == mk.Kind {
			continue // a field character or a reference
		}
		if seen[mk.Kind] == nil {
			seen[mk.Kind] = map[int64]bool{}
		}
		seen[mk.Kind][mk.ID] = true
	}
	out := map[MarkKind]map[int64]bool{}
	add := func(kind MarkKind, id int64) {
		if out[kind] == nil {
			out[kind] = map[int64]bool{}
		}
		out[kind][id] = true
	}
	for _, p := range anchorPairs {
		start, end := p[0], p[1]
		for id := range seen[start] {
			if !seen[end][id] {
				add(end, id)
			}
		}
		for id := range seen[end] {
			if !seen[start][id] {
				add(start, id)
			}
		}
	}
	return out
}

// Refs returns, by kind (MarkFootnoteRef, MarkEndnoteRef, MarkCommentRef),
// the ids of the note and comment references in marks.
func Refs(marks []Mark) map[MarkKind]map[int64]bool {
	out := map[MarkKind]map[int64]bool{}
	for _, mk := range marks {
		switch mk.Kind {
		case MarkFootnoteRef, MarkEndnoteRef, MarkCommentRef:
			if out[mk.Kind] == nil {
				out[mk.Kind] = map[int64]bool{}
			}
			out[mk.Kind][mk.ID] = true
		}
	}
	return out
}

// RemoveAnchors deletes the anchors named in ids (by kind, then w:id) from
// blocks, wherever they are: in paragraphs and hyperlinks, between blocks,
// rows and cells, and inside raw content.
func RemoveAnchors(blocks *[]BlockElement, ids map[MarkKind]map[int64]bool) {
	if len(ids) == 0 {
		return
	}
	r := anchorRemover{ids: ids}
	r.blocks(blocks)
}

type anchorRemover struct {
	ids map[MarkKind]map[int64]bool
}

// rawMatch reports whether a raw element is an anchor to remove.
func (r anchorRemover) rawMatch(re *raw.RawElement) bool {
	if re == nil {
		return false
	}
	k, ok := anchorNames[re.QName]
	if !ok {
		return false
	}
	id := parseInt64Ptr(attrNS(re.Attrs, ns.W, "id"))
	return id != nil && r.ids[k][*id]
}

func (r anchorRemover) blocks(blocks *[]BlockElement) {
	*blocks = slices.DeleteFunc(*blocks, func(b BlockElement) bool {
		rb, ok := b.(*RawBlock)
		return ok && r.rawMatch(rb.E)
	})
	for _, b := range *blocks {
		switch el := b.(type) {
		case *CT_P:
			r.inlines(&el.Children)
		case *CT_Tbl:
			r.table(el)
		case *RawBlock:
			r.raw(el.E)
		}
	}
}

func (r anchorRemover) table(t *CT_Tbl) {
	t.Children = slices.DeleteFunc(t.Children, func(row TableRowElement) bool {
		rr, ok := row.(*RawTableRow)
		return ok && r.rawMatch(rr.E)
	})
	for _, row := range t.Children {
		switch el := row.(type) {
		case *CT_Tr:
			el.Children = slices.DeleteFunc(el.Children, func(c TableCellElement) bool {
				rc, ok := c.(*RawTableCell)
				return ok && r.rawMatch(rc.E)
			})
			for _, c := range el.Children {
				switch cell := c.(type) {
				case *CT_Tc:
					r.blocks(&cell.Children)
				case *RawTableCell:
					r.raw(cell.E)
				}
			}
		case *RawTableRow:
			r.raw(el.E)
		}
	}
}

func (r anchorRemover) inlines(children *[]InlineElement) {
	*children = slices.DeleteFunc(*children, func(c InlineElement) bool {
		switch el := c.(type) {
		case *CT_BookmarkStart:
			return r.ids[MarkBookmarkStart][el.ID]
		case *CT_BookmarkEnd:
			return r.ids[MarkBookmarkEnd][el.ID]
		case *RawInline:
			return r.rawMatch(el.E)
		}
		return false
	})
	for _, c := range *children {
		switch el := c.(type) {
		case *CT_R:
			r.run(el)
		case *CT_Hyperlink:
			r.inlines(&el.Children)
		case *RawInline:
			r.raw(el.E)
		}
	}
}

// run removes matching anchors nested in a run: in text boxes of its
// drawings and in raw run content (mc:AlternateContent, w:pict, w:object).
func (r anchorRemover) run(run *CT_R) {
	for _, c := range run.Children {
		switch el := c.(type) {
		case *CT_Drawing:
			for _, re := range drawingRaws(el) {
				r.raw(re)
			}
		case *RawRunContent:
			r.raw(el.E)
		}
	}
}

// raw removes matching anchors from a raw subtree.
func (r anchorRemover) raw(re *raw.RawElement) {
	if re == nil {
		return
	}
	re.Children = slices.DeleteFunc(re.Children, func(tok xml.Token) bool {
		k, ok := tok.(*raw.RawElement)
		return ok && r.rawMatch(k)
	})
	for _, k := range rawKids(re) {
		r.raw(k)
	}
}
