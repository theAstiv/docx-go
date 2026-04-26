package wml

import (
	"encoding/xml"

	"github.com/theAstiv/docx-go/internal/oxml/raw"
)

// BlockElement is the marker interface for elements that can appear as
// direct children of CT_Body (paragraphs, tables, structured document tags, etc.).
type BlockElement interface {
	MarshalXML(e *xml.Encoder, start xml.StartElement) error
	UnmarshalXML(d *xml.Decoder, start xml.StartElement) error
	isBlock()
}

// RawBlock wraps a RawElement so unknown body-level elements pass through
// unchanged. It satisfies BlockElement.
type RawBlock struct {
	E *raw.RawElement
}

func (*RawBlock) isBlock() {}

// MarshalXML delegates to the wrapped RawElement.
func (rb *RawBlock) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if rb.E == nil {
		return nil
	}
	return rb.E.MarshalXML(e, start)
}

// UnmarshalXML captures the element into the wrapped RawElement.
func (rb *RawBlock) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	rb.E = new(raw.RawElement)
	return rb.E.UnmarshalXML(d, start)
}
