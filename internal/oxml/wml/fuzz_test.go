package wml_test

import (
	"bytes"
	"encoding/xml"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/roundtrip"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

const wNS = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"`

// FuzzCT_P verifies that any CT_P that unmarshals without error also
// round-trips cleanly: marshal(parse(input)) must be stable under a second
// marshal-parse cycle.
func FuzzCT_P(f *testing.F) {
	add := func(s string) { f.Add([]byte(s)) }

	add(`<w:p ` + wNS + `></w:p>`)
	add(`<w:p ` + wNS + `><w:pPr><w:jc w:val="center"/></w:pPr></w:p>`)
	add(`<w:p ` + wNS + `><w:r><w:t xml:space="preserve">hello</w:t></w:r></w:p>`)
	add(`<w:p ` + wNS + `><w:pPr><w:spacing w:before="240" w:after="120"/></w:pPr>` +
		`<w:r><w:rPr><w:b/></w:rPr><w:t>bold</w:t></w:r></w:p>`)
	add(`<w:p ` + wNS + ` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<w:hyperlink r:id="rId1"><w:r><w:t>link</w:t></w:r></w:hyperlink></w:p>`)
	add(`<w:p ` + wNS + `>` +
		`<w:bookmarkStart w:id="0" w:name="intro"/>` +
		`<w:r><w:t>anchored</w:t></w:r>` +
		`<w:bookmarkEnd w:id="0"/>` +
		`</w:p>`)

	f.Fuzz(func(t *testing.T, data []byte) {
		if !roundtrip.NamespaceWellFormed(data) {
			return // not valid namespaced XML; nothing Word could write
		}
		v := new(wml.CT_P)
		dec := xml.NewDecoder(bytes.NewReader(data))
		tok, err := dec.Token()
		if err != nil {
			return
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			return
		}
		if err := v.UnmarshalXML(dec, se); err != nil {
			return
		}
		roundtrip.Verify(t, v, ns.Qn("w:p"))
	})
}

// FuzzCT_R verifies that any CT_R that unmarshals without error also
// round-trips cleanly.
func FuzzCT_R(f *testing.F) {
	add := func(s string) { f.Add([]byte(s)) }

	add(`<w:r ` + wNS + `></w:r>`)
	add(`<w:r ` + wNS + `><w:rPr><w:b/></w:rPr></w:r>`)
	add(`<w:r ` + wNS + `><w:rPr><w:b/><w:i/><w:color w:val="4472C4"/></w:rPr>` +
		`<w:t xml:space="preserve">styled text</w:t></w:r>`)
	add(`<w:r ` + wNS + `><w:tab/></w:r>`)
	add(`<w:r ` + wNS + `><w:br w:type="page"/></w:r>`)
	add(`<w:r ` + wNS + `><w:rPr><w:color w:val="FF0000"/></w:rPr>` +
		`<w:sym w:font="Wingdings" w:char="F0B7"/></w:r>`)
	add(`<w:r ` + wNS + `><w:fldChar w:fldCharType="begin"/></w:r>`)
	add(`<w:r ` + wNS + `><w:instrText xml:space="preserve"> HYPERLINK "https://example.com" </w:instrText></w:r>`)
	add(`<w:r ` + wNS + `><w:noBreakHyphen/></w:r>`)

	f.Fuzz(func(t *testing.T, data []byte) {
		if !roundtrip.NamespaceWellFormed(data) {
			return // not valid namespaced XML; nothing Word could write
		}
		v := new(wml.CT_R)
		dec := xml.NewDecoder(bytes.NewReader(data))
		tok, err := dec.Token()
		if err != nil {
			return
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			return
		}
		if err := v.UnmarshalXML(dec, se); err != nil {
			return
		}
		roundtrip.Verify(t, v, ns.Qn("w:r"))
	})
}
