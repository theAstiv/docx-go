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
	//
	// A naive bytes.ReplaceAll is NOT safe: inside elements that declare a
	// different default namespace (e.g. <mc:AlternateContent> uses xmlns="MC"),
	// W-namespace children correctly emit xmlns="ROOT_NS" to restore the root
	// default.  Stripping those would silently move the element into the MC
	// namespace.  We therefore walk the child bytes with xml.Decoder, tracking
	// the default-namespace stack, and strip xmlns="ROOT_NS" only when it is
	// genuinely redundant (current default already IS ROOT_NS).
	rest := src[rootEnd+1:]
	if se.Name.Space != "" {
		rest = stripRedundantRootNS(rest, se.Name.Space)
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

// stripRedundantRootNS removes xmlns="rootNS" from child-element start tags,
// but ONLY when the current XML default namespace is already rootNS.  Inside
// elements that declare a different default namespace (e.g. mc:AlternateContent
// uses xmlns="MC"), W-namespace descendants correctly re-declare xmlns="rootNS"
// to restore the root default; those declarations must be preserved.
//
// The walk uses xml.Decoder so it correctly handles nested scopes and
// self-closing elements.  InputOffset() gives byte offsets into src so that
// unmodified regions are copied verbatim.
func stripRedundantRootNS(src []byte, rootNS string) []byte {
	target := []byte(` xmlns="` + rootNS + `"`)
	if !bytes.Contains(src, target) {
		return src
	}

	dec := xml.NewDecoder(bytes.NewReader(src))
	// defaultNS tracks the active default namespace at each element depth.
	// The root element (not in src) already declared rootNS as default.
	defaultNS := []string{rootNS}

	var out bytes.Buffer
	out.Grow(len(src))
	written := int64(0) // bytes of src already written to out

	for {
		startOff := dec.InputOffset() // start of the next token (after prev token)
		tok, err := dec.Token()
		if err != nil {
			break
		}
		endOff := dec.InputOffset() // end of tok

		switch t := tok.(type) {
		case xml.StartElement:
			// Determine the default namespace declared BY this element (if any).
			elementDefaultNS := defaultNS[len(defaultNS)-1]
			for _, a := range t.Attr {
				if a.Name.Space == "" && a.Name.Local == "xmlns" {
					elementDefaultNS = a.Value
				}
			}
			// xmlns="rootNS" is redundant only when the incoming default is
			// already rootNS AND the element is also declaring rootNS.
			currentDefault := defaultNS[len(defaultNS)-1]
			if currentDefault == rootNS && elementDefaultNS == rootNS {
				tagBytes := src[startOff:endOff]
				modified := bytes.Replace(tagBytes, target, nil, 1)
				out.Write(src[written:startOff])
				out.Write(modified)
				written = endOff
			}
			defaultNS = append(defaultNS, elementDefaultNS)

		case xml.EndElement:
			if len(defaultNS) > 1 {
				defaultNS = defaultNS[:len(defaultNS)-1]
			}
		}
	}
	out.Write(src[written:])
	return out.Bytes()
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
