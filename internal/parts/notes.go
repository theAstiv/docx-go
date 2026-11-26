package parts

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strconv"
	"strings"

	"github.com/theAstiv/docx-go/internal/opc"
	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// NoteKind names one of the parts that hold notes or comments for the main
// document. This package keeps them as bytes: it scans them for ids and
// removes whole notes, but does not model their content.
type NoteKind int

// Note kinds.
const (
	Footnotes NoteKind = iota
	Endnotes
	Comments
)

// noteKinds lists every NoteKind, in story order.
var noteKinds = []NoteKind{Footnotes, Endnotes, Comments}

// relType returns the relationship type the main document names the part by.
func (k NoteKind) relType() string {
	switch k {
	case Endnotes:
		return opc.RelTypeEndnotes
	case Comments:
		return opc.RelTypeComments
	}
	return opc.RelTypeFootnotes
}

// child returns the name of the part's note elements (the root's children).
func (k NoteKind) child() xml.Name {
	switch k {
	case Endnotes:
		return ns.Qn("w:endnote")
	case Comments:
		return ns.Qn("w:comment")
	}
	return ns.Qn("w:footnote")
}

// NotePath returns the stored path of the part of the given kind that the
// main document's relationships name, or "" when there is none.
func (p *Package) NotePath(kind NoteKind) string {
	if p.docPath == "" {
		return ""
	}
	for _, r := range p.pkg.Rels(p.docPath).ByType(kind.relType()) {
		if name, ok := p.findTarget(p.docPath, r); ok {
			return name
		}
	}
	return ""
}

// ScanNotes adds the ids held in the footnotes, endnotes and comments parts
// (drawing ids, bookmarks, data bindings, references) to scan. A part that
// is not well-formed contributes what was read before the error.
func (p *Package) ScanNotes(scan *wml.IDScan) {
	for _, kind := range noteKinds {
		if path := p.NotePath(kind); path != "" {
			_ = wml.ScanPart(p.pkg.Part(path), scan) // partial results are kept
		}
	}
}

// RemoveNotes deletes the notes (or comments) of the given kind whose w:id
// is in ids from their part. Only the bytes of the removed elements change;
// the rest of the part is kept as it was. It does nothing when the part is
// missing or not well-formed: a note left without a reference is harmless,
// a broken part is not.
func (p *Package) RemoveNotes(kind NoteKind, ids map[int64]bool) {
	if len(ids) == 0 {
		return
	}
	path := p.NotePath(kind)
	if path == "" {
		return
	}
	data := p.pkg.Part(path)
	cuts, err := noteSpans(data, kind.child(), ids)
	if err != nil || len(cuts) == 0 {
		return
	}
	out := make([]byte, 0, len(data))
	last := 0
	for _, c := range cuts {
		out = append(out, data[last:c[0]]...)
		last = c[1]
	}
	out = append(out, data[last:]...)
	p.pkg.SetPart(path, out)
}

// noteSpans returns the byte ranges [start, end) of the root's children
// named child whose w:id is in ids, in order.
func noteSpans(data []byte, child xml.Name, ids map[int64]bool) ([][2]int, error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	var cuts [][2]int
	depth := 0
	for {
		start := d.InputOffset()
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			return cuts, nil
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if id, ok := noteID(t.Attr); ok && depth == 1 && t.Name == child && ids[id] {
				if err := d.Skip(); err != nil {
					return nil, err
				}
				end := d.InputOffset()
				if start < 0 || end < start || end > int64(len(data)) {
					return nil, errors.New("note offsets outside the part")
				}
				cuts = append(cuts, [2]int{int(start), int(end)})
				continue
			}
			depth++
		case xml.EndElement:
			depth--
		}
	}
}

// noteID returns a note's w:id and whether it has a numeric one.
func noteID(attrs []xml.Attr) (int64, bool) {
	for _, a := range attrs {
		if a.Name.Local == "id" && (a.Name.Space == ns.W || a.Name.Space == "") {
			n, err := strconv.ParseInt(strings.TrimSpace(a.Value), 10, 64)
			return n, err == nil
		}
	}
	return 0, false
}
