package ns

import (
	"encoding/xml"
	"fmt"
	"maps"
	"strings"
)

// Namespace URIs for all namespaces used in OOXML / OPC.
const (
	W      = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	W14    = "http://schemas.microsoft.com/office/word/2010/wordml"
	W15    = "http://schemas.microsoft.com/office/word/2012/wordml"
	W16CID = "http://schemas.microsoft.com/office/word/2016/wordml/cid"
	W16SE  = "http://schemas.microsoft.com/office/word/2015/wordml/symex"
	W16    = "http://schemas.microsoft.com/office/word/2018/wordml"

	R   = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	M   = "http://schemas.openxmlformats.org/officeDocument/2006/math"
	MC  = "http://schemas.openxmlformats.org/markup-compatibility/2006"
	WP  = "http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
	A   = "http://schemas.openxmlformats.org/drawingml/2006/main"
	PIC = "http://schemas.openxmlformats.org/drawingml/2006/picture"
	O   = "urn:schemas-microsoft-com:office:office"
	V   = "urn:schemas-microsoft-com:vml"
	XML = "http://www.w3.org/XML/1998/namespace"
	WPS = "http://schemas.microsoft.com/office/word/2010/wordprocessingShape"
	WPG = "http://schemas.microsoft.com/office/word/2010/wordprocessingGroup"
	W10 = "urn:schemas-microsoft-com:office:word"
	VT  = "http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes"
)

// prefixTable maps namespace URI → canonical prefix.
// This is the single source of truth; all packages call Qn() rather than
// constructing xml.Name literals with URI strings.
var prefixTable = map[string]string{
	W:      "w",
	W14:    "w14",
	W15:    "w15",
	W16CID: "w16cid",
	W16SE:  "w16se",
	W16:    "w16",
	R:      "r",
	M:      "m",
	MC:     "mc",
	WP:     "wp",
	A:      "a",
	PIC:    "pic",
	O:      "o",
	V:      "v",
	XML:    "xml",
	WPS:    "wps",
	WPG:    "wpg",
	W10:    "w10",
	VT:     "vt",
}

// uriTable maps canonical prefix → namespace URI (inverse of prefixTable).
var uriTable map[string]string

func init() {
	uriTable = make(map[string]string, len(prefixTable))
	for uri, pfx := range prefixTable {
		uriTable[pfx] = uri
	}
}

// Qn parses a "prefix:localName" string and returns the Clark-notation
// xml.Name using the canonical URI for that prefix.  Panics on unknown
// prefixes so mistakes surface at init time, not at runtime.
func Qn(prefixedName string) xml.Name {
	pfx, local, hasPfx := strings.Cut(prefixedName, ":")
	if !hasPfx {
		// No prefix — local name only (e.g. "id").
		return xml.Name{Local: prefixedName}
	}
	uri, ok := uriTable[pfx]
	if !ok {
		panic(fmt.Sprintf("ns.Qn: unknown prefix %q in %q", pfx, prefixedName))
	}
	return xml.Name{Space: uri, Local: local}
}

// Prefix returns the canonical prefix for a namespace URI, or "" if unknown.
func Prefix(uri string) string {
	return prefixTable[uri]
}

// URI returns the namespace URI for a canonical prefix, or "" if unknown.
func URI(prefix string) string {
	return uriTable[prefix]
}

// All returns a copy of the prefix→URI table for use in namespace declarations.
func All() map[string]string {
	out := make(map[string]string, len(uriTable))
	maps.Copy(out, uriTable)
	return out
}
