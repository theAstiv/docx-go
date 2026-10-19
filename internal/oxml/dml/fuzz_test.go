package dml_test

import (
	"bytes"
	"encoding/xml"
	"regexp"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/dml"
	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/roundtrip"
)

// xmlName is the XML 1.0 Name production restricted to ASCII, which is all the
// seeds use. encoding/xml's decoder accepts some names outside it (such as
// ".extent"), which its own second pass then rejects; that is a stdlib quirk,
// not a defect of these types, so such inputs are skipped.
var xmlName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.\-]*$`)

// wellNamed reports whether every element and attribute name in data is a
// valid XML name.
func wellNamed(data []byte) bool {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			return true // parse errors are handled by the caller
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if !xmlName.MatchString(t.Name.Local) {
				return false
			}
			for _, a := range t.Attr {
				if !xmlName.MatchString(a.Name.Local) {
					return false
				}
			}
		case xml.EndElement:
			if !xmlName.MatchString(t.Name.Local) {
				return false
			}
		}
	}
}

// startOf returns the decoder positioned after the first start element.
func startOf(data []byte) (*xml.Decoder, xml.StartElement, bool) {
	if !wellNamed(data) {
		return nil, xml.StartElement{}, false
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, xml.StartElement{}, false
		}
		if se, ok := tok.(xml.StartElement); ok {
			return dec, se, true
		}
	}
}

// FuzzCT_Inline checks that any wp:inline that parses also round-trips.
func FuzzCT_Inline(f *testing.F) {
	f.Add([]byte(wordInline))
	f.Add([]byte(`<wp:inline xmlns:wp="` + ns.WP + `"><wp:extent cx="1" cy="1"/><wp:docPr id="1" name="x"/></wp:inline>`))
	f.Add([]byte(`<wp:inline xmlns:wp="` + ns.WP + `" xmlns:a="` + ns.A + `"><a:graphic/></wp:inline>`))
	f.Add([]byte(`<wp:inline xmlns:wp="` + ns.WP + `"><wp:unknown a="b"><x/></wp:unknown></wp:inline>`))
	f.Add([]byte(`<wp:inline xmlns:wp="` + ns.WP + `"/>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		if !roundtrip.NamespaceWellFormed(data) {
			return // not valid namespaced XML; nothing Word could write
		}
		dec, se, ok := startOf(data)
		if !ok || se.Name != ns.Qn("wp:inline") {
			return
		}
		v := new(dml.CT_Inline)
		if err := v.UnmarshalXML(dec, se); err != nil {
			return
		}
		roundtrip.Verify(t, v, ns.Qn("wp:inline"))
	})
}

// FuzzCT_Picture checks that any pic:pic that parses also round-trips.
func FuzzCT_Picture(f *testing.F) {
	pic := `xmlns:pic="` + ns.PIC + `" xmlns:a="` + ns.A + `" xmlns:r="` + ns.R + `"`
	f.Add([]byte(`<pic:pic ` + pic + `><pic:nvPicPr><pic:cNvPr id="0" name="a.png"/><pic:cNvPicPr/></pic:nvPicPr>` +
		`<pic:blipFill><a:blip r:embed="rId1"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill>` +
		`<pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="10" cy="10"/></a:xfrm></pic:spPr></pic:pic>`))
	f.Add([]byte(`<pic:pic ` + pic + `><pic:blipFill><a:blip r:link="rId2"/></pic:blipFill></pic:pic>`))
	f.Add([]byte(`<pic:pic ` + pic + `><pic:spPr bwMode="auto"/><pic:style><a:lnRef idx="1"/></pic:style></pic:pic>`))
	f.Add([]byte(`<pic:pic ` + pic + `/>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		if !roundtrip.NamespaceWellFormed(data) {
			return // not valid namespaced XML; nothing Word could write
		}
		dec, se, ok := startOf(data)
		if !ok || se.Name != ns.Qn("pic:pic") {
			return
		}
		v := new(dml.CT_Picture)
		if err := v.UnmarshalXML(dec, se); err != nil {
			return
		}
		roundtrip.Verify(t, v, ns.Qn("pic:pic"))
	})
}
