package wml

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"math"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/raw"
)

// BookmarkRef identifies one bookmarkStart found while scanning.
type BookmarkRef struct {
	ID   int64
	Name string
	// Start is the typed element when the bookmark sits in a plain paragraph;
	// nil when it was found inside raw content (a content control, w:ins,
	// w:smartTag, a block-level bookmark, ...), which has no typed proxy.
	Start *CT_BookmarkStart
}

// IDScan collects the identifiers that must stay unique across a document.
type IDScan struct {
	// DocPr holds every wp:docPr id.
	DocPr []int64
	// Bookmarks holds every w:bookmarkStart.
	Bookmarks []BookmarkRef
	// StoreItemIDs holds the w:storeItemID of every w:dataBinding: the
	// custom XML parts (and document properties) content controls show.
	StoreItemIDs []string
	// Refs holds every footnote, endnote and comment reference (Kind
	// MarkFootnoteRef, MarkEndnoteRef or MarkCommentRef).
	Refs []Mark
}

// Referenced reports whether the scan holds a reference of kind with id.
func (s *IDScan) Referenced(kind MarkKind, id int64) bool {
	for _, r := range s.Refs {
		if r.Kind == kind && r.ID == id {
			return true
		}
	}
	return false
}

// ScanPart adds the ids found in a part kept as bytes (footnotes, endnotes,
// comments) to s, reading it as a token stream: wp:docPr ids, bookmarks,
// data bindings and note references, wherever they are nested. The bytes
// are not modelled, so bookmarks found here carry no typed element.
func ScanPart(data []byte, s *IDScan) error {
	d := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		scanStart(se.Name, se.Attr, s)
	}
}

// scanStart records the ids one element holds, for scanRaw and ScanPart.
func scanStart(name xml.Name, attrs []xml.Attr, s *IDScan) {
	switch name {
	case ns.Qn("wp:docPr"):
		if n := parseInt64Ptr(attrLocal(attrs, "id")); n != nil {
			s.DocPr = append(s.DocPr, *n)
		}
	case ns.Qn("w:bookmarkStart"):
		ref := BookmarkRef{ID: 0, Name: attrNS(attrs, ns.W, "name"), Start: nil}
		if n := parseInt64Ptr(attrNS(attrs, ns.W, "id")); n != nil {
			ref.ID = *n
		}
		s.Bookmarks = append(s.Bookmarks, ref)
	case ns.Qn("w:dataBinding"), ns.Qn("w15:dataBinding"):
		if id := attrAnyNS(attrs, "storeItemID"); id != "" {
			s.StoreItemIDs = append(s.StoreItemIDs, id)
		}
	default:
		if k, ok := refNames[name]; ok {
			if n := parseInt64Ptr(attrNS(attrs, ns.W, "id")); n != nil {
				s.Refs = append(s.Refs, Mark{Kind: k, ID: *n, FldCharType: ""})
			}
		}
	}
}

// DocPrAllocator returns a function that hands out wp:docPr ids, one per
// call, all above every id in s. Scan once, then allocate many: rescanning
// every story per new drawing is quadratic in a loop that adds hundreds.
func (s *IDScan) DocPrAllocator() func() int64 {
	return NewIDAllocator(s.DocPr, 1, MaxDocPrID)
}

// MaxDocPrID is the largest wp:docPr id: the attribute is xsd:unsignedInt.
const MaxDocPrID = math.MaxUint32

// MaxBookmarkID is the largest bookmark id docx-go hands out: w:id is
// ST_DecimalNumber, which Word reads as a 32-bit signed integer.
const MaxBookmarkID = math.MaxInt32

// NewIDAllocator returns a function that hands out ids in [lo, hi] that are
// not in used, one per call: above the largest used id while that fits,
// then the smallest free ones. Ids outside [lo, hi] are ignored, so a
// document holding an id at the top of the range (or beyond it) does not
// make the allocator overflow or leave the range. Once every id in the
// range is taken it returns hi+1, which no real document reaches.
func NewIDAllocator(used []int64, lo, hi int64) func() int64 {
	taken := make(map[int64]bool, len(used))
	next := lo
	for _, id := range used {
		if id < lo || id > hi {
			continue
		}
		taken[id] = true
		if id >= next {
			next = id + 1
		}
	}
	wrapped := false
	return func() int64 {
		if next > hi && !wrapped {
			wrapped, next = true, lo
		}
		if wrapped {
			for next <= hi && taken[next] {
				next++
			}
		}
		id := next
		taken[id] = true
		next++
		return id
	}
}

// ScanBlocks adds the ids found in blocks to s. It reaches everything, not
// just plain paragraphs: tables and their cells, hyperlinks, and all raw
// content (w:sdt, w:ins, w:del, w:smartTag, w:customXml, mc:AlternateContent),
// so a freshly allocated id or name never collides with content this package
// does not model.
func ScanBlocks(blocks []BlockElement, s *IDScan) {
	for _, b := range blocks {
		switch el := b.(type) {
		case *CT_P:
			scanInlines(el.Children, s)
		case *CT_Tbl:
			scanTable(el, s)
		case *RawBlock:
			if el.E != nil {
				scanRaw(el.E, s)
			}
		}
	}
}

func scanTable(t *CT_Tbl, s *IDScan) {
	for _, r := range t.Children {
		switch row := r.(type) {
		case *CT_Tr:
			for _, c := range row.Children {
				switch cell := c.(type) {
				case *CT_Tc:
					ScanBlocks(cell.Children, s)
				case *RawTableCell:
					if cell.E != nil {
						scanRaw(cell.E, s)
					}
				}
			}
		case *RawTableRow:
			if row.E != nil {
				scanRaw(row.E, s)
			}
		}
	}
}

func scanInlines(children []InlineElement, s *IDScan) {
	for _, c := range children {
		switch el := c.(type) {
		case *CT_R:
			scanRun(el, s)
		case *CT_Hyperlink:
			scanInlines(el.Children, s)
		case *CT_BookmarkStart:
			s.Bookmarks = append(s.Bookmarks, BookmarkRef{ID: el.ID, Name: el.Name, Start: el})
		case *RawInline:
			if el.E != nil {
				scanRaw(el.E, s)
			}
		}
	}
}

// scanRun collects ids from a run's drawings and raw content.
func scanRun(r *CT_R, s *IDScan) {
	for _, c := range r.Children {
		switch el := c.(type) {
		case *CT_Drawing:
			scanDrawing(el, s)
		case *RawRunContent:
			if el.E != nil {
				scanRaw(el.E, s)
			}
		}
	}
}

// scanDrawing collects ids from a drawing: its own wp:docPr and everything
// nested in its raw parts. A text box (wps:txbx) inside a drawing can hold
// further pictures and bookmarks, in an inline drawing's graphic data just as
// in an anchored drawing, so every raw bucket is walked, not only docPr.
func scanDrawing(d *CT_Drawing, s *IDScan) {
	if in := d.Inline; in != nil && in.DocPr != nil {
		s.DocPr = append(s.DocPr, in.DocPr.ID)
	}
	for _, re := range drawingRaws(d) {
		scanRaw(re, s)
	}
}

// drawingRaws returns every raw part of a drawing, in document order: an
// anchored drawing (wp:anchor) as a whole, and the raw pieces of a typed
// inline picture. Text boxes, with their paragraphs, pictures and anchors,
// live in these.
func drawingRaws(d *CT_Drawing) []*raw.RawElement {
	var out []*raw.RawElement
	one := func(re *raw.RawElement) {
		if re != nil {
			out = append(out, re)
		}
	}
	all := func(res []*raw.RawElement) {
		for _, re := range res {
			one(re)
		}
	}
	if in := d.Inline; in != nil {
		if in.DocPr != nil {
			all(in.DocPr.Children)
		}
		one(in.EffectExtent)
		one(in.CNvGraphicFramePr)
		all(in.Extras)
		if g := in.Graphic; g != nil {
			all(g.Rest)
			if gd := g.GraphicData; gd != nil {
				all(gd.Rest)
				if pic := gd.Pic; pic != nil {
					one(pic.NvPicPr)
					all(pic.Extras)
					if pic.SpPr != nil {
						all(pic.SpPr.Children)
					}
					if pic.BlipFill != nil {
						all(pic.BlipFill.Rest)
						if pic.BlipFill.Blip != nil {
							all(pic.BlipFill.Blip.Children)
						}
					}
				}
			}
		}
	}
	all(d.Rest)
	return out
}

// scanRaw walks a raw subtree for the ids scanStart records.
func scanRaw(re *raw.RawElement, s *IDScan) {
	scanStart(re.QName, re.Attrs, s)
	for _, k := range rawKids(re) {
		scanRaw(k, s)
	}
}

// attrAnyNS returns the value of the first attribute with the given local
// name, in any namespace.
func attrAnyNS(attrs []xml.Attr, local string) string {
	for _, a := range attrs {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

// attrNS returns the value of the attribute with the given namespace and local
// name, tolerating documents that wrote it without a namespace.
func attrNS(attrs []xml.Attr, space, local string) string {
	for _, a := range attrs {
		if a.Name.Local == local && (a.Name.Space == space || a.Name.Space == "") {
			return a.Value
		}
	}
	return ""
}
