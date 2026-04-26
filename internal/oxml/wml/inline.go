package wml

import (
	"encoding/xml"

	"github.com/theAstiv/docx-go/internal/oxml/raw"
)

// InlineElement is the marker interface for elements that can appear as
// direct children of CT_P: runs, hyperlinks, bookmarks, etc.
type InlineElement interface {
	MarshalXML(e *xml.Encoder, start xml.StartElement) error
	UnmarshalXML(d *xml.Decoder, start xml.StartElement) error
	isInline()
}

// RawInline wraps a RawElement so unknown paragraph-level inline elements
// pass through unchanged. It satisfies InlineElement.
type RawInline struct {
	E *raw.RawElement
}

func (*RawInline) isInline() {}

// MarshalXML delegates to the wrapped RawElement.
func (ri *RawInline) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if ri.E == nil {
		return nil
	}
	return ri.E.MarshalXML(e, start)
}

// UnmarshalXML captures the element into the wrapped RawElement.
func (ri *RawInline) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	ri.E = new(raw.RawElement)
	return ri.E.UnmarshalXML(d, start)
}
