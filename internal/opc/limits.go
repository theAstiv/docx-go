package opc

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
)

// Resource limits applied when a package is read. A .docx is untrusted input:
// without them a small file can exhaust memory (a zip bomb) or overflow the
// stack of the recursive element decoders (deeply nested elements).
const (
	// MaxEntries is the most ZIP entries a package may hold.
	MaxEntries = 10_000

	// MaxEntrySize is the largest uncompressed size of one ZIP entry.
	MaxEntrySize int64 = 512 << 20

	// MaxTotalSize is the largest uncompressed size of all entries together.
	MaxTotalSize int64 = 1 << 30

	// MaxXMLDepth is the deepest element nesting an XML part may have.
	// encoding/xml's own depth cap does not apply to the Token loops the
	// element decoders use, so without it a small, highly compressed file
	// overflows the stack. Word nests tables at most 63 levels, but the POI
	// corpus holds a 15,005-level document (deep-table-cell.docx) that must
	// still open. Templating keeps per-level location paths, so a much
	// higher cap (100,000) exhausts memory there.
	MaxXMLDepth = 20_000
)

// limits is the set of limits readZip applies; tests use smaller ones.
type limits struct {
	entries      int
	entry, total int64
	depth        int
}

var defaultLimits = limits{entries: MaxEntries, entry: MaxEntrySize, total: MaxTotalSize, depth: MaxXMLDepth}

// ErrLimitExceeded is returned (wrapped) by Open when the package exceeds one
// of the resource limits.
var ErrLimitExceeded = errors.New("opc: resource limit exceeded")

// looksLikeXML reports whether data may be parsed as XML by a later layer:
// after an optional UTF-8 byte-order mark and whitespace, it starts with '<'.
// The part name cannot decide this: the officeDocument relationship and the
// content types choose which part is parsed, so the main part may be called
// word/doc.dat. Binary parts (images) never start with '<'; an SVG image
// does, and is scanned too, which is harmless.
func looksLikeXML(data []byte) bool {
	data = bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF"))
	data = bytes.TrimLeft(data, " \t\r\n")
	return len(data) > 0 && data[0] == '<'
}

// checkXMLDepth reports an ErrLimitExceeded error when data nests elements
// deeper than max. Malformed XML stops the scan without an error: the parser
// that reads the part reports it.
func checkXMLDepth(name string, data []byte, max int) error {
	d := xml.NewDecoder(bytes.NewReader(data))
	depth := 0
	for {
		tok, err := d.RawToken()
		if err != nil {
			return nil //nolint:nilerr // malformed XML is the part parser's error to report
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
			if depth > max {
				return fmt.Errorf("%w: %s nests elements deeper than %d", ErrLimitExceeded, name, max)
			}
		case xml.EndElement:
			depth--
		}
	}
}
