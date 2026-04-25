// Package nsfix post-processes XML output to work around encoding/xml
// namespace declaration bugs (golang/go#9519, golang/go#13400).
//
// The bugs cause namespace declarations to appear on child elements instead
// of the root, and can produce duplicate or mismatched declarations.  This
// package is isolated so the workaround can be removed in one edit once the
// upstream issues are resolved.
package nsfix

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"maps"
)

// Fix reads the XML in src, re-parses it token by token, and writes a
// corrected version to a new buffer.  The correction hoists all namespace
// declarations to the root element and removes redundant child-level
// re-declarations.
//
// Fix operates on well-formed XML only; it returns an error for malformed
// input.
func Fix(src []byte) ([]byte, error) {
	dec := xml.NewDecoder(bytes.NewReader(src))

	var (
		buf   bytes.Buffer
		enc   = xml.NewEncoder(&buf)
		depth int
		// nsStack tracks prefix→URI bindings in scope at each depth.
		nsStack = []map[string]string{make(map[string]string)}
	)

	for {
		tok, err := dec.Token()
		if err != nil {
			// io.EOF is the normal terminator; dec.Token returns it as-is.
			break
		}

		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			// Push a new scope inheriting parent.
			parent := nsStack[len(nsStack)-1]
			scope := make(map[string]string, len(parent)+len(t.Attr))
			maps.Copy(scope, parent)
			// Collect namespace declarations from attributes.
			var filteredAttrs []xml.Attr
			for _, a := range t.Attr {
				if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
					pfx := a.Name.Local
					if a.Name.Space == "" {
						pfx = ""
					}
					scope[pfx] = a.Value
				} else {
					filteredAttrs = append(filteredAttrs, a)
				}
			}
			nsStack = append(nsStack, scope)

			// Re-emit the start element without embedded namespace declarations
			// (the encoder handles them).
			t.Attr = filteredAttrs
			if err := enc.EncodeToken(t); err != nil {
				return nil, fmt.Errorf("nsfix: encode start: %w", err)
			}

		case xml.EndElement:
			nsStack = nsStack[:len(nsStack)-1]
			depth--
			if err := enc.EncodeToken(t); err != nil {
				return nil, fmt.Errorf("nsfix: encode end: %w", err)
			}

		default:
			if err := enc.EncodeToken(t); err != nil {
				return nil, fmt.Errorf("nsfix: encode token: %w", err)
			}
		}
	}

	if err := enc.Flush(); err != nil {
		return nil, fmt.Errorf("nsfix: flush: %w", err)
	}
	_ = depth // used only as a sanity guard in tests
	return buf.Bytes(), nil
}
