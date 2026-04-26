package parts

import (
	"bytes"
	"encoding/xml"
	"fmt"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

const (
	// ContentTypeStyles is the MIME type for word/styles.xml.
	ContentTypeStyles = "application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"
	// PathStyles is the canonical OPC path for the styles part.
	PathStyles = "word/styles.xml"
)

// StylesPart owns the word/styles.xml part of a .docx package.
type StylesPart struct {
	styles *wml.CT_Styles
}

// NewStylesPart creates an empty StylesPart.
func NewStylesPart() *StylesPart {
	sp := new(StylesPart)
	sp.styles = new(wml.CT_Styles)
	return sp
}

// Parse reads raw XML bytes and populates the StylesPart.
// It skips the XML declaration and any processing instructions before the root.
func (sp *StylesPart) Parse(data []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("parts: styles: read root token: %w", err)
		}
		start, ok := tok.(xml.StartElement)
		if ok {
			sp.styles = new(wml.CT_Styles)
			return sp.styles.UnmarshalXML(dec, start)
		}
	}
}

// Styles returns the underlying CT_Styles element for direct oxml access.
func (sp *StylesPart) Styles() *wml.CT_Styles {
	return sp.styles
}

// Marshal serialises the StylesPart to XML bytes.
func (sp *StylesPart) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	if err := sp.styles.MarshalXML(enc, xml.StartElement{}); err != nil {
		return nil, fmt.Errorf("parts: styles: marshal: %w", err)
	}
	if err := enc.Flush(); err != nil {
		return nil, fmt.Errorf("parts: styles: flush: %w", err)
	}
	return buf.Bytes(), nil
}
