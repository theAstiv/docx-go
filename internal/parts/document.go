package parts

import (
	"bytes"
	"encoding/xml"
	"fmt"

	"github.com/theAstiv/docx-go/internal/nsfix"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

const (
	// ContentTypeDocument is the MIME type for word/document.xml.
	ContentTypeDocument = "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"
	// PathDocument is the canonical OPC path for the main document part.
	PathDocument = "word/document.xml"
)

// DocumentPart owns the word/document.xml part of a .docx package.
// It bridges the opc (raw bytes) layer and the docx (proxy) layer.
type DocumentPart struct {
	doc *wml.CT_Document
}

// NewDocumentPart creates a DocumentPart with a minimal empty document.
// The body always has a trailing w:sectPr (OOXML requirement; missing it
// causes Word to synthesize defaults and can silently alter the document).
func NewDocumentPart() *DocumentPart {
	dp := new(DocumentPart)
	dp.doc = new(wml.CT_Document)
	dp.doc.Body = new(wml.CT_Body)
	dp.doc.Body.SectPr = wml.DefaultSectPr()
	return dp
}

// Parse reads raw XML bytes and populates the DocumentPart.
// It skips the XML declaration and any processing instructions before the root.
func (dp *DocumentPart) Parse(data []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("parts: document: read root token: %w", err)
		}
		start, ok := tok.(xml.StartElement)
		if ok {
			dp.doc = new(wml.CT_Document)
			return dp.doc.UnmarshalXML(dec, start)
		}
		// Skip xml.ProcInst (<?xml...?>) and xml.Comment before the root element.
	}
}

// Body returns the CT_Body element for direct oxml access.
// The docx layer uses this to build paragraph/run proxies.
func (dp *DocumentPart) Body() *wml.CT_Body {
	if dp.doc == nil {
		return nil
	}
	return dp.doc.Body
}

// Marshal serialises the DocumentPart to XML bytes suitable for storage
// as word/document.xml in the OPC ZIP.
func (dp *DocumentPart) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	if err := dp.doc.MarshalXML(enc, xml.StartElement{}); err != nil {
		return nil, fmt.Errorf("parts: document: marshal: %w", err)
	}
	if err := enc.Flush(); err != nil {
		return nil, fmt.Errorf("parts: document: flush: %w", err)
	}
	fixed, err := nsfix.Fix(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("parts: document: nsfix: %w", err)
	}
	return fixed, nil
}
