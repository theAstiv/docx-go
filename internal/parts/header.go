package parts

import (
	"bytes"
	"encoding/xml"
	"fmt"

	"github.com/theAstiv/docx-go/internal/nsfix"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

const (
	// ContentTypeHeader is the MIME type for word/header*.xml.
	ContentTypeHeader = "application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"
)

// HeaderPart owns a word/header{N}.xml part of a .docx package.
type HeaderPart struct {
	ct *wml.CT_Hdr
}

// NewHeaderPart creates a HeaderPart with an empty default paragraph.
func NewHeaderPart() *HeaderPart {
	hp := new(HeaderPart)
	hp.ct = new(wml.CT_Hdr)
	hp.ct.Children = append(hp.ct.Children, new(wml.CT_P))
	return hp
}

// Parse reads raw XML bytes and populates the HeaderPart.
func (hp *HeaderPart) Parse(data []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("parts: header: read root token: %w", err)
		}
		start, ok := tok.(xml.StartElement)
		if ok {
			hp.ct = new(wml.CT_Hdr)
			return hp.ct.UnmarshalXML(dec, start)
		}
	}
}

// CT returns the underlying CT_Hdr for direct oxml access.
func (hp *HeaderPart) CT() *wml.CT_Hdr {
	return hp.ct
}

// Marshal serialises the HeaderPart to XML bytes.
func (hp *HeaderPart) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	if err := hp.ct.MarshalXML(enc, xml.StartElement{}); err != nil {
		return nil, fmt.Errorf("parts: header: marshal: %w", err)
	}
	if err := enc.Flush(); err != nil {
		return nil, fmt.Errorf("parts: header: flush: %w", err)
	}
	fixed, err := nsfix.Fix(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("parts: header: nsfix: %w", err)
	}
	return fixed, nil
}
