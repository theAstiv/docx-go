package parts

import (
	"bytes"
	"encoding/xml"
	"fmt"

	"github.com/theAstiv/docx-go/internal/nsfix"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

const (
	// ContentTypeNumbering is the MIME type for word/numbering.xml.
	ContentTypeNumbering = "application/vnd.openxmlformats-officedocument.wordprocessingml.numbering+xml"
	// PathNumbering is the canonical OPC path for the numbering part.
	PathNumbering = "word/numbering.xml"
)

// NumberingPart owns the word/numbering.xml part of a .docx package.
type NumberingPart struct {
	ct *wml.CT_Numbering
}

// NewNumberingPart creates an empty NumberingPart.
func NewNumberingPart() *NumberingPart {
	np := new(NumberingPart)
	np.ct = new(wml.CT_Numbering)
	return np
}

// Parse reads raw XML bytes and populates the NumberingPart.
func (np *NumberingPart) Parse(data []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("parts: numbering: read root token: %w", err)
		}
		start, ok := tok.(xml.StartElement)
		if ok {
			np.ct = new(wml.CT_Numbering)
			return np.ct.UnmarshalXML(dec, start)
		}
	}
}

// CT returns the underlying CT_Numbering for direct oxml access.
func (np *NumberingPart) CT() *wml.CT_Numbering {
	return np.ct
}

// Marshal serialises the NumberingPart to XML bytes.
func (np *NumberingPart) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	if err := np.ct.MarshalXML(enc, xml.StartElement{}); err != nil {
		return nil, fmt.Errorf("parts: numbering: marshal: %w", err)
	}
	if err := enc.Flush(); err != nil {
		return nil, fmt.Errorf("parts: numbering: flush: %w", err)
	}
	fixed, err := nsfix.Fix(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("parts: numbering: nsfix: %w", err)
	}
	return fixed, nil
}
