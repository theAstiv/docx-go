// Package nsfix post-processes XML output to work around encoding/xml
// namespace declaration bugs (golang/go#9519, golang/go#13400).
//
// The encoder represents xmlns:foo="uri" attributes (stored as
// xml.Attr{Name:{Space:"xmlns",Local:foo}}) as the two-attribute sequence:
//
//	xmlns:_xmlns="xmlns"  _xmlns:foo="uri"
//
// and auto-generates throw-away prefixes (e.g. "_", "ns1") for any
// namespace-qualified attribute whose prefix is not already in scope.
//
// Fix corrects these by: (1) decoding the mangled root element through
// xml.Decoder — which correctly recovers the original xml.Attr values —
// and (2) re-emitting the root element opening tag as raw bytes with proper
// xmlns:prefix="uri" declarations using canonical prefixes from ns.Prefix().
// Child element bytes are passed through unchanged.
//
// The root element name is intentionally kept unprefixed (matching
// encoding/xml's closing tag) with the root namespace declared as the XML
// default namespace (xmlns="..."), so the closing tag is not disturbed.
package nsfix

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"sort"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
)

// Fix repairs namespace declarations produced by encoding/xml bugs #9519/#13400.
// It returns corrected XML bytes; src is not modified.
func Fix(src []byte) ([]byte, error) {
	rootStart := findRootStart(src)
	if rootStart < 0 {
		return src, nil
	}

	// xml.Decoder correctly recovers the original attribute semantics from the
	// mangled output:
	//   xmlns:_xmlns="xmlns" _xmlns:foo="uri"  → {Space:"xmlns", Local:"foo", Value:"uri"}
	//   xmlns:X="mc-uri"  X:Ignorable="val"    → {Space:"mc-uri", Local:"Ignorable", Value:"val"}
	dec := xml.NewDecoder(bytes.NewReader(src[rootStart:]))
	tok, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("nsfix: parse root element: %w", err)
	}
	se, ok := tok.(xml.StartElement)
	if !ok {
		return nil, fmt.Errorf("nsfix: expected StartElement at root, got %T", tok)
	}

	rootEnd := findTagEnd(src, rootStart)
	if rootEnd < 0 {
		return nil, fmt.Errorf("nsfix: unterminated root element tag")
	}

	var out bytes.Buffer
	out.Grow(len(src) + 512)
	out.Write(src[:rootStart])
	buildTag(&out, se)

	// Strip redundant default-namespace re-declarations from child elements.
	// encoding/xml repeats xmlns="ROOT_NS" on every child element in that
	// namespace.  When a child element also has namespace-qualified attributes
	// (e.g. main:asciiTheme) the encoder additionally emits xmlns:main="ROOT_NS"
	// for the prefix — giving the same URI two bindings on one element, which
	// .NET's XmlDocument rejects as a duplicate namespace declaration.
	// Removing the redundant xmlns="ROOT_NS" from child content is safe because
	// children inherit the root element's default-namespace declaration.
	rest := src[rootEnd+1:]
	if se.Name.Space != "" {
		redundant := []byte(` xmlns="` + se.Name.Space + `"`)
		rest = bytes.ReplaceAll(rest, redundant, nil)
	}
	out.Write(rest)
	return out.Bytes(), nil
}

// findRootStart returns the byte index of the '<' that opens the root element,
// skipping any <?...?> processing instructions or <!...> declarations.
func findRootStart(src []byte) int {
	for i := 0; i+1 < len(src); i++ {
		if src[i] == '<' && src[i+1] != '?' && src[i+1] != '!' {
			return i
		}
	}
	return -1
}

// findTagEnd returns the index of the '>' that closes the opening tag
// starting at src[start], correctly skipping quoted attribute values.
func findTagEnd(src []byte, start int) int {
	quote := byte(0)
	for i := start; i < len(src); i++ {
		b := src[i]
		switch {
		case quote != 0:
			if b == quote {
				quote = 0
			}
		case b == '"' || b == '\'':
			quote = b
		case b == '>':
			return i
		}
	}
	return -1
}

// buildTag writes a corrected root element opening tag to w.
//
// The element name is kept unprefixed (matching encoding/xml's closing tag)
// and the root namespace is declared as the XML default namespace (xmlns="...").
// All other namespace declarations are emitted with canonical prefixes.
func buildTag(w *bytes.Buffer, se xml.StartElement) {
	// Build uri → canonical-prefix for non-root namespaces.
	// The root element's own namespace is declared as the default namespace
	// (xmlns="...") so that it matches the encoding/xml-generated closing tag
	// which has no prefix.
	uriToPrefix := make(map[string]string, 32)

	for _, a := range se.Attr {
		if a.Name.Space != "xmlns" {
			continue
		}
		uri := a.Value
		// Skip the fake "xmlns" URI artifact introduced by the encoder mangling.
		if uri == "" || uri == "xmlns" {
			continue
		}
		// Skip the root element's own namespace — it is covered by xmlns="...".
		if uri == se.Name.Space {
			continue
		}
		if _, exists := uriToPrefix[uri]; !exists {
			pfx := ns.Prefix(uri)
			if pfx == "" {
				pfx = a.Name.Local // keep original prefix if not in our canonical map
			}
			uriToPrefix[uri] = pfx
		}
	}

	// Element name has no prefix (matches encoding/xml's closing </localname> tag).
	w.WriteByte('<')
	w.WriteString(se.Name.Local)

	// Declare root element's namespace as the default namespace.
	if se.Name.Space != "" {
		w.WriteString(` xmlns="`)
		writeEscaped(w, se.Name.Space)
		w.WriteByte('"')
	}

	// Emit xmlns:prefix="uri" for all other namespaces, sorted by prefix.
	type pair struct{ pfx, uri string }
	decls := make([]pair, 0, len(uriToPrefix))
	for uri, pfx := range uriToPrefix {
		decls = append(decls, pair{pfx, uri})
	}
	sort.Slice(decls, func(i, j int) bool { return decls[i].pfx < decls[j].pfx })
	for _, d := range decls {
		writeNSDecl(w, d.pfx, d.uri)
	}

	// Emit regular (non-xmlns) attributes.
	for _, a := range se.Attr {
		if a.Name.Space == "xmlns" {
			continue
		}
		// Skip the default-namespace declaration {Space:"", Local:"xmlns"} —
		// it is already emitted as xmlns="se.Name.Space" above; emitting it
		// again here would produce a duplicate xmlns attribute.
		if a.Name.Space == "" && a.Name.Local == "xmlns" {
			continue
		}
		var name string
		if a.Name.Space != "" {
			pfx, ok := uriToPrefix[a.Name.Space]
			if !ok {
				// Namespace not yet declared — determine prefix and add declaration.
				pfx = ns.Prefix(a.Name.Space)
				if pfx == "" {
					pfx = fmt.Sprintf("ns%d", len(uriToPrefix))
				}
				uriToPrefix[a.Name.Space] = pfx
				writeNSDecl(w, pfx, a.Name.Space)
			}
			name = pfx + ":" + a.Name.Local
		} else {
			name = a.Name.Local
		}
		w.WriteByte(' ')
		w.WriteString(name)
		w.WriteString(`="`)
		writeEscaped(w, a.Value)
		w.WriteByte('"')
	}

	w.WriteByte('>')
}

func writeNSDecl(w *bytes.Buffer, pfx, uri string) {
	// Never re-declare the implicit xml namespace.
	if pfx == "xml" {
		return
	}
	w.WriteString(` xmlns:`)
	w.WriteString(pfx)
	w.WriteString(`="`)
	writeEscaped(w, uri)
	w.WriteByte('"')
}

func writeEscaped(w *bytes.Buffer, s string) {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&':
			w.WriteString("&amp;")
		case '"':
			w.WriteString("&quot;")
		case '<':
			w.WriteString("&lt;")
		default:
			w.WriteByte(s[i])
		}
	}
}
