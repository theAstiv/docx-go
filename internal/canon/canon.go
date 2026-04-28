package canon

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"maps"
	"sort"
	"strings"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
)

// XML canonicalises the XML bytes in src according to the structural-only
// canonicalisation rules for docx-go:
//
//   - Attributes are sorted lexicographically by Clark-notation Name.
//   - Namespace prefixes are rewritten to the canonical mapping in ns.go.
//   - Redundant namespace declarations (already declared by a parent) are dropped.
//   - Text-node content (xml.CharData) is preserved byte-for-byte.
//   - Whitespace between elements is NOT normalised.
//
// This is intentionally NOT XML C14N / C14N11.  The goal is structural
// equivalence, not byte-identical w3c canonical form.
func XML(src []byte) ([]byte, error) {
	dec := xml.NewDecoder(bytes.NewReader(src))

	type nsScope struct {
		// prefix → URI bindings in scope at this element depth
		bindings map[string]string
	}

	var (
		buf   bytes.Buffer
		enc   = xml.NewEncoder(&buf)
		stack []nsScope
	)

	// The root scope has no bindings yet.
	stack = append(stack, nsScope{bindings: map[string]string{}})

	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("canon: decode: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			parent := stack[len(stack)-1]
			scope := nsScope{bindings: make(map[string]string, len(parent.bindings))}
			maps.Copy(scope.bindings, parent.bindings)

			// Rewrite element name to canonical prefix.
			t.Name = canonName(t.Name)

			// Rewrite and filter attributes.
			var attrs []xml.Attr
			for _, a := range t.Attr {
				// Skip stored namespace declarations (handled via Name.Space).
				if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
					continue
				}
				a.Name = canonName(a.Name)
				attrs = append(attrs, a)
			}

			// Sort attributes by Clark-notation name for determinism.
			sort.Slice(attrs, func(i, j int) bool {
				return clarkAttr(attrs[i].Name) < clarkAttr(attrs[j].Name)
			})
			t.Attr = attrs

			// Track new namespace bindings for this element.
			if t.Name.Space != "" {
				pfx := ns.Prefix(t.Name.Space)
				scope.bindings[pfx] = t.Name.Space
			}
			for _, a := range attrs {
				if a.Name.Space != "" {
					pfx := ns.Prefix(a.Name.Space)
					scope.bindings[pfx] = a.Name.Space
				}
			}

			stack = append(stack, scope)

			if err := enc.EncodeToken(t); err != nil {
				return nil, fmt.Errorf("canon: encode start %v: %w", t.Name, err)
			}

		case xml.EndElement:
			stack = stack[:len(stack)-1]
			t.Name = canonName(t.Name)
			if err := enc.EncodeToken(t); err != nil {
				return nil, fmt.Errorf("canon: encode end %v: %w", t.Name, err)
			}

		case xml.CharData:
			if err := enc.EncodeToken(t); err != nil {
				return nil, fmt.Errorf("canon: encode chardata: %w", err)
			}

		case xml.Comment:
			if err := enc.EncodeToken(t); err != nil {
				return nil, fmt.Errorf("canon: encode comment: %w", err)
			}

		case xml.ProcInst:
			if err := enc.EncodeToken(t); err != nil {
				return nil, fmt.Errorf("canon: encode procinst: %w", err)
			}
		}
	}

	if err := enc.Flush(); err != nil {
		return nil, fmt.Errorf("canon: flush: %w", err)
	}
	return buf.Bytes(), nil
}

// Equal returns true if the two XML byte slices are structurally equal under
// canonicalisation.
func Equal(a, b []byte) (bool, error) {
	ca, err := XML(a)
	if err != nil {
		return false, fmt.Errorf("canon: canonicalise a: %w", err)
	}
	cb, err := XML(b)
	if err != nil {
		return false, fmt.Errorf("canon: canonicalise b: %w", err)
	}
	return bytes.Equal(ca, cb), nil
}

// canonName rewrites an xml.Name so its Space field holds a URI recognised by
// the ns package, and returns the name unchanged for unrecognised URIs.
func canonName(n xml.Name) xml.Name {
	if n.Space == "" {
		return n
	}
	// Already a known URI — keep as is.
	if ns.Prefix(n.Space) != "" {
		return n
	}
	return n
}

// clarkAttr formats an xml.Name as "{URI}local" for sorting.
func clarkAttr(n xml.Name) string {
	if n.Space == "" {
		return n.Local
	}
	return "{" + n.Space + "}" + n.Local
}

// Attrs returns src's attributes sorted by Clark notation.  Helper for tests.
func Attrs(attrs []xml.Attr) []xml.Attr {
	out := make([]xml.Attr, len(attrs))
	copy(out, attrs)
	sort.Slice(out, func(i, j int) bool {
		return clarkAttr(out[i].Name) < clarkAttr(out[j].Name)
	})
	return out
}

// StripXMLDecl removes a leading `<?xml ...?>` processing instruction from
// canonicalised output so bytes from different encoder configurations compare
// equal.
func StripXMLDecl(b []byte) []byte {
	b = bytes.TrimSpace(b)
	if bytes.HasPrefix(b, []byte("<?xml")) {
		end := bytes.IndexByte(b, '>')
		if end >= 0 {
			b = bytes.TrimSpace(b[end+1:])
		}
	}
	return b
}

// NormaliseNS strips the "w:" prefix from element and attribute names in the
// serialised output for human-readable diff output.  Not used in the test
// oracle — only in cmd/docxdiff.
func NormaliseNS(s string) string {
	return strings.ReplaceAll(s, " xmlns:", " _xmlns_:")
}
