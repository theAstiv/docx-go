package opc

import (
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
)

// ErrMalformedPart is returned (wrapped) when an XML part holds something
// other than markup before its root element.
var ErrMalformedPart = errors.New("opc: malformed XML part")

// prologSpace holds the characters allowed between markup before the root:
// XML whitespace (#x20, #x9, #xD, #xA) and a byte-order mark, which
// encoding/xml passes through as text.
const prologSpace = " \t\r\n" + "\xEF\xBB\xBF" // the last three bytes: U+FEFF in UTF-8

// RootElement reads tokens from dec up to the root element and returns its
// start tag, leaving dec positioned inside it.
//
// Before the root it accepts only what a well-formed XML document may hold
// there: the XML declaration and other processing instructions, comments,
// and whitespace. Anything else is ErrMalformedPart:
//
//   - text ("a<w:document…>", or one led by a no-break space). encoding/xml
//     reports it as an ordinary token, and the parsers used to skip it, so a
//     part that is not well-formed XML was read as if it were;
//   - a DOCTYPE declaration. Open Packaging Conventions forbid DTDs in
//     package XML, and Word never writes one.
//
// Every part parser starts here, so they all agree on which bytes are a part.
func RootElement(dec *xml.Decoder) (xml.StartElement, error) {
	for {
		tok, err := dec.Token()
		if err != nil {
			return xml.StartElement{}, fmt.Errorf("read root token: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			return t, nil
		case xml.ProcInst, xml.Comment:
			// The XML declaration, other processing instructions, comments.
		case xml.CharData:
			if strings.Trim(string(t), prologSpace) != "" {
				return xml.StartElement{}, fmt.Errorf("%w: text before the root element", ErrMalformedPart)
			}
		case xml.Directive:
			return xml.StartElement{}, fmt.Errorf("%w: DOCTYPE or other declaration before the root element", ErrMalformedPart)
		default:
			return xml.StartElement{}, fmt.Errorf("%w: unexpected %T before the root element", ErrMalformedPart, tok)
		}
	}
}
