package dml

import (
	"encoding/xml"
	"fmt"
	"strconv"

	"github.com/theAstiv/docx-go/internal/oxml/raw"
)

// isNSDecl reports whether a is a namespace declaration. The encoder re-emits
// these from xml.Name.Space, so storing them would double-declare on marshal.
func isNSDecl(a xml.Attr) bool {
	return a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns")
}

// attrInt64 parses an integer attribute value; absent or malformed → 0.
func attrInt64(s string) int64 {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// attrStr returns the value of the attribute with the given local name and
// namespace, and whether it was present.
func attrStr(attrs []xml.Attr, space, local string) (string, bool) {
	for _, a := range attrs {
		if a.Name.Local == local && a.Name.Space == space {
			return a.Value, true
		}
	}
	return "", false
}

// marshalRaw writes each raw child in order.
func marshalRaw(e *xml.Encoder, children []*raw.RawElement) error {
	for _, c := range children {
		if err := c.MarshalXML(e, xml.StartElement{}); err != nil {
			return fmt.Errorf("dml: raw child: %w", err)
		}
	}
	return nil
}

// readRaw captures the element starting at start as a RawElement.
func readRaw(d *xml.Decoder, start xml.StartElement) (*raw.RawElement, error) {
	re := new(raw.RawElement)
	if err := re.UnmarshalXML(d, start); err != nil {
		return nil, fmt.Errorf("dml: capture %v: %w", start.Name, err)
	}
	return re, nil
}

// emptyElement writes <name attrs/> as a start/end pair.
func emptyElement(e *xml.Encoder, start xml.StartElement) error {
	if err := e.EncodeToken(start); err != nil {
		return fmt.Errorf("dml: %s start: %w", start.Name.Local, err)
	}
	if err := e.EncodeToken(xml.EndElement{Name: start.Name}); err != nil {
		return fmt.Errorf("dml: %s end: %w", start.Name.Local, err)
	}
	return nil
}
