package parts

import (
	"bytes"
	"encoding/xml"
	"fmt"

	"github.com/theAstiv/docx-go/internal/nsfix"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

const (
	// ContentTypeSettings is the MIME type for word/settings.xml.
	ContentTypeSettings = "application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"
	// PathSettings is the canonical OPC path for the settings part.
	PathSettings = "word/settings.xml"
)

// SettingsPart owns the word/settings.xml part of a .docx package.
type SettingsPart struct {
	ct *wml.CT_Settings
}

// NewSettingsPart creates a SettingsPart with an empty settings element.
func NewSettingsPart() *SettingsPart {
	sp := new(SettingsPart)
	sp.ct = new(wml.CT_Settings)
	return sp
}

// Parse reads raw XML bytes and populates the SettingsPart.
func (sp *SettingsPart) Parse(data []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("parts: settings: read root token: %w", err)
		}
		start, ok := tok.(xml.StartElement)
		if ok {
			sp.ct = new(wml.CT_Settings)
			return sp.ct.UnmarshalXML(dec, start)
		}
	}
}

// CT returns the underlying CT_Settings for direct oxml access.
func (sp *SettingsPart) CT() *wml.CT_Settings {
	return sp.ct
}

// Marshal serialises the SettingsPart to XML bytes.
func (sp *SettingsPart) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	if err := sp.ct.MarshalXML(enc, xml.StartElement{}); err != nil {
		return nil, fmt.Errorf("parts: settings: marshal: %w", err)
	}
	if err := enc.Flush(); err != nil {
		return nil, fmt.Errorf("parts: settings: flush: %w", err)
	}
	fixed, err := nsfix.Fix(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("parts: settings: nsfix: %w", err)
	}
	return fixed, nil
}
