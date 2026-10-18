package wml

import (
	"encoding/xml"

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
}

// DocPrAllocator returns a function that hands out wp:docPr ids, one per
// call, all above every id in s. Scan once, then allocate many: rescanning
// every story per new drawing is quadratic in a loop that adds hundreds.
func (s *IDScan) DocPrAllocator() func() int64 {
	next := int64(1)
	for _, id := range s.DocPr {
		if id >= next {
			next = id + 1
		}
	}
	return func() int64 {
		id := next
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
	if in := d.Inline; in != nil {
		if in.DocPr != nil {
			s.DocPr = append(s.DocPr, in.DocPr.ID)
			scanRawAll(in.DocPr.Children, s)
		}
		scanRawOne(in.EffectExtent, s)
		scanRawOne(in.CNvGraphicFramePr, s)
		scanRawAll(in.Extras, s)
		if g := in.Graphic; g != nil {
			scanRawAll(g.Rest, s)
			if gd := g.GraphicData; gd != nil {
				scanRawAll(gd.Rest, s)
				if pic := gd.Pic; pic != nil {
					scanRawOne(pic.NvPicPr, s)
					scanRawAll(pic.Extras, s)
					if pic.SpPr != nil {
						scanRawAll(pic.SpPr.Children, s)
					}
					if pic.BlipFill != nil {
						scanRawAll(pic.BlipFill.Rest, s)
						if pic.BlipFill.Blip != nil {
							scanRawAll(pic.BlipFill.Blip.Children, s)
						}
					}
				}
			}
		}
	}
	scanRawAll(d.Rest, s)
}

func scanRawOne(re *raw.RawElement, s *IDScan) {
	if re != nil {
		scanRaw(re, s)
	}
}

func scanRawAll(res []*raw.RawElement, s *IDScan) {
	for _, re := range res {
		scanRaw(re, s)
	}
}

// scanRaw walks a raw subtree for wp:docPr and w:bookmarkStart.
func scanRaw(re *raw.RawElement, s *IDScan) {
	switch re.QName {
	case ns.Qn("wp:docPr"):
		if n := parseInt64Ptr(attrLocal(re.Attrs, "id")); n != nil {
			s.DocPr = append(s.DocPr, *n)
		}
	case ns.Qn("w:bookmarkStart"):
		ref := new(BookmarkRef)
		ref.Name = attrNS(re.Attrs, ns.W, "name")
		if n := parseInt64Ptr(attrNS(re.Attrs, ns.W, "id")); n != nil {
			ref.ID = *n
		}
		s.Bookmarks = append(s.Bookmarks, *ref)
	}
	for _, k := range rawKids(re) {
		scanRaw(k, s)
	}
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
