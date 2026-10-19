package roundtrip

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
)

// NamespaceWellFormed reports whether data is well-formed under XML
// Namespaces as far as encoding/xml's decoder lets one tell: every element
// and attribute name has a local part that is an NCName (so not "0" from
// "x:0"), and every prefix is declared.
//
// encoding/xml accepts both mistakes, and the encoder cannot write them back
// ("x:0" becomes the invalid "<0>"). Word never produces such input, and the
// round-trip promise covers valid documents, so fuzz targets skip it.
func NamespaceWellFormed(data []byte) bool {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return true
		}
		if err != nil {
			return false
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if !nameOK(se.Name) {
			return false
		}
		for _, a := range se.Attr {
			if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
				continue
			}
			if !nameOK(a.Name) {
				return false
			}
		}
	}
}

// nameOK checks one resolved name. The decoder leaves an undeclared prefix
// as the name's Space; a declared namespace is an absolute URI, which always
// contains ':' (the xml namespace too).
func nameOK(n xml.Name) bool {
	if n.Space != "" && !strings.Contains(n.Space, ":") {
		return false // undeclared prefix
	}
	return isNCName(n.Local)
}

// isNCName reports whether s is a valid NCName for the ASCII range, which is
// what fuzzers produce: a letter or '_' first, then letters, digits, '.',
// '-' or '_'. Non-ASCII letters are accepted.
func isNCName(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		switch {
		case c == '_', c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c > 0x7F:
		case i > 0 && (c == '-' || c == '.' || c >= '0' && c <= '9'):
		default:
			return false
		}
	}
	return true
}
