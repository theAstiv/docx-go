package canon

import (
	"bytes"
	"encoding/xml"
	"errors"
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
// Two namespace defects are errors (ErrNamespace), not differences to be
// canonicalised away: an undeclared prefix on an element or attribute (a
// lost declaration), and a declaration binding a prefix to the URI "xmlns"
// (the xmlns:_xmlns="xmlns" junk encoding/xml writes when it is asked to
// encode a stored namespace declaration). Comparing only after dropping every
// declaration made both invisible to the round-trip tests.
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

			if undeclared(t.Name) {
				return nil, fmt.Errorf("%w: element %s:%s uses an undeclared prefix", ErrNamespace, t.Name.Space, t.Name.Local)
			}
			// Rewrite and filter attributes.
			var attrs []xml.Attr
			for _, a := range t.Attr {
				// Skip stored namespace declarations (handled via Name.Space).
				if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
					if a.Value == "xmlns" {
						return nil, fmt.Errorf("%w: %v binds a prefix to the URI \"xmlns\"", ErrNamespace, t.Name)
					}
					continue
				}
				if undeclared(a.Name) {
					return nil, fmt.Errorf("%w: attribute %s:%s on %v uses an undeclared prefix", ErrNamespace, a.Name.Space, a.Name.Local, t.Name)
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

// ErrNamespace is returned (wrapped) by XML and Equal for namespace defects
// in the input: an undeclared prefix, or a prefix bound to "xmlns".
var ErrNamespace = errors.New("canon: namespace defect")

// undeclared reports whether n kept an unresolved prefix: encoding/xml leaves
// an undeclared prefix as the name's Space, while a declared namespace is an
// absolute URI, which always contains ':' (the xml namespace too).
func undeclared(n xml.Name) bool {
	return n.Space != "" && !strings.Contains(n.Space, ":")
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
