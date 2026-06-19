package parts

import (
	"bytes"
	"encoding/xml"
	"fmt"

	"github.com/theAstiv/docx-go/internal/nsfix"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

const (
	// ContentTypeFooter is the MIME type for word/footer*.xml.
	ContentTypeFooter = "application/vnd.openxmlformats-officedocument.wordprocessingml.footer+xml"
)

// FooterPart owns a word/footer{N}.xml part of a .docx package.
type FooterPart struct {
	ct *wml.CT_Ftr
}

// NewFooterPart creates a FooterPart with an empty default paragraph.
func NewFooterPart() *FooterPart {
	fp := new(FooterPart)
	fp.ct = new(wml.CT_Ftr)
	fp.ct.Children = append(fp.ct.Children, new(wml.CT_P))
	return fp
}

// Parse reads raw XML bytes and populates the FooterPart.
func (fp *FooterPart) Parse(data []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("parts: footer: read root token: %w", err)
		}
		start, ok := tok.(xml.StartElement)
		if ok {
			fp.ct = new(wml.CT_Ftr)
			return fp.ct.UnmarshalXML(dec, start)
		}
	}
}

// CT returns the underlying CT_Ftr for direct oxml access.
func (fp *FooterPart) CT() *wml.CT_Ftr {
	return fp.ct
}

// Marshal serialises the FooterPart to XML bytes.
func (fp *FooterPart) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	if err := fp.ct.MarshalXML(enc, xml.StartElement{}); err != nil {
		return nil, fmt.Errorf("parts: footer: marshal: %w", err)
	}
	if err := enc.Flush(); err != nil {
		return nil, fmt.Errorf("parts: footer: flush: %w", err)
	}
	fixed, err := nsfix.Fix(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("parts: footer: nsfix: %w", err)
	}
	return fixed, nil
}
