package wml

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
)

// Cloneable is any element with paired XML methods: block, inline, row and
// cell elements, and their raw passthrough wrappers.
type Cloneable interface {
	MarshalXML(e *xml.Encoder, start xml.StartElement) error
	UnmarshalXML(d *xml.Decoder, start xml.StartElement) error
}

// Clone returns a deep copy of el, which must be a non-nil pointer.
//
// It marshals el and unmarshals the bytes into a new value of the same
// concrete type. The round-trip rule (every type's Marshal and Unmarshal are
// inverse) makes this exactly as faithful as a save and reopen, including
// for raw content, with no per-type copy code to keep in sync.
func Clone[T Cloneable](el T) (T, error) {
	return CloneWith(el, nil)
}

// CloneFixup lists the changes a copy needs so it can sit in the same
// document as its original. Word requires these identifiers to be unique,
// and duplicates are a known cause of its "unreadable content" warning.
//
// The fix-ups run on the copy's token stream, so they reach typed and raw
// content alike: paragraphs inside content controls and text boxes, and
// bookmarks between table rows.
type CloneFixup struct {
	// StripParaIDs removes w14:paraId and w14:textId (optional attributes
	// that must be unique when present).
	StripParaIDs bool
	// DropBookmarks removes w:bookmarkStart and w:bookmarkEnd.
	DropBookmarks bool
	// DropCommentAnchors removes w:commentRangeStart, w:commentRangeEnd and
	// w:commentReference; a comment cannot be anchored twice.
	DropCommentAnchors bool
	// NextDocPrID, when set, gives every wp:docPr a fresh id.
	NextDocPrID func() int64

	// DroppedComments counts the comment anchors removed so far.
	DroppedComments int
	// NoteRefs counts footnote and endnote references copied so far; each
	// copy shows the same note again, which callers may want to report.
	NoteRefs int
}

// CloneWith is Clone with fix-ups applied to the copy. A nil fx applies none.
func CloneWith[T Cloneable](el T, fx *CloneFixup) (T, error) {
	var zero T
	rv := reflect.ValueOf(el)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return zero, fmt.Errorf("wml: Clone: need a non-nil pointer, got %T", el)
	}

	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	if err := el.MarshalXML(enc, xml.StartElement{}); err != nil {
		return zero, fmt.Errorf("wml: Clone %T: %w", el, err)
	}
	if err := enc.Flush(); err != nil {
		return zero, fmt.Errorf("wml: Clone %T: %w", el, err)
	}
	src := buf.Bytes()
	if fx != nil && len(src) > 0 {
		var err error
		if src, err = fx.apply(src); err != nil {
			return zero, fmt.Errorf("wml: Clone %T: %w", el, err)
		}
	}

	out, ok := reflect.New(rv.Type().Elem()).Interface().(T)
	if !ok {
		return zero, fmt.Errorf("wml: Clone: cannot allocate %T", el)
	}
	if len(src) == 0 {
		// A raw wrapper with no element marshals to nothing; so does its copy.
		return out, nil
	}
	dec := xml.NewDecoder(bytes.NewReader(src))
	for {
		tok, err := dec.Token()
		if err != nil {
			return zero, fmt.Errorf("wml: Clone %T: %w", el, err)
		}
		if start, isStart := tok.(xml.StartElement); isStart {
			if err := out.UnmarshalXML(dec, start); err != nil {
				return zero, fmt.Errorf("wml: Clone %T: %w", el, err)
			}
			return out, nil
		}
	}
}

// apply rewrites one marshalled element per the fix-ups.
func (fx *CloneFixup) apply(src []byte) ([]byte, error) {
	dec := xml.NewDecoder(bytes.NewReader(src))
	var out bytes.Buffer
	enc := xml.NewEncoder(&out)
	skip := 0 // depth inside a dropped element
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("fix-up: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if skip > 0 || fx.drops(t.Name) {
				skip++
				continue
			}
			if t.Name == ns.Qn("w:footnoteReference") || t.Name == ns.Qn("w:endnoteReference") {
				fx.NoteRefs++
			}
			tok = fx.fixStart(t)
		case xml.EndElement:
			if skip > 0 {
				skip--
				continue
			}
		default:
			if skip > 0 {
				continue
			}
		}
		if err := enc.EncodeToken(xml.CopyToken(tok)); err != nil {
			return nil, fmt.Errorf("fix-up: %w", err)
		}
	}
	if err := enc.Flush(); err != nil {
		return nil, fmt.Errorf("fix-up: %w", err)
	}
	return out.Bytes(), nil
}

// drops reports whether an element is removed from the copy, counting
// dropped comment anchors.
func (fx *CloneFixup) drops(name xml.Name) bool {
	switch name {
	case ns.Qn("w:bookmarkStart"), ns.Qn("w:bookmarkEnd"):
		return fx.DropBookmarks
	case ns.Qn("w:commentRangeStart"), ns.Qn("w:commentRangeEnd"), ns.Qn("w:commentReference"):
		if fx.DropCommentAnchors {
			if name.Local == "commentReference" {
				fx.DroppedComments++
			}
			return true
		}
	}
	return false
}

// fixStart rewrites one start tag: namespace declarations go (the encoder
// re-emits them), paraIds go when asked, wp:docPr gets a fresh id.
func (fx *CloneFixup) fixStart(t xml.StartElement) xml.StartElement {
	attrs := make([]xml.Attr, 0, len(t.Attr))
	for _, a := range t.Attr {
		switch {
		case a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns"):
			continue
		case fx.StripParaIDs && a.Name.Space == ns.W14 && (a.Name.Local == "paraId" || a.Name.Local == "textId"):
			continue
		case fx.NextDocPrID != nil && t.Name == ns.Qn("wp:docPr") && a.Name.Space == "" && a.Name.Local == "id":
			a.Value = strconv.FormatInt(fx.NextDocPrID(), 10)
		}
		attrs = append(attrs, a)
	}
	t.Attr = attrs
	return t
}
