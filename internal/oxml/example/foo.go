// Package example contains the Phase-0 exit-gate toy registered type.
// It demonstrates the full Register → MarshalXML → UnmarshalXML → round-trip
// pattern that every real oxml type must follow.
// This package is not part of the public API and may be removed after Phase 1.
package example

import (
	"encoding/xml"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/registry"
)

// CT_ExampleFoo is a minimal synthetic oxml type used by the exit-gate test.
// It mirrors a hypothetical element <eg:foo eg:val="…"/> in the example namespace.
type CT_ExampleFoo struct {
	Val string
}

// XMLName returns the Clark-notation name for the element.
func (f *CT_ExampleFoo) XMLName() xml.Name { return exampleFooName }

// MarshalXML emits <eg:foo eg:val="…"/>.
func (f *CT_ExampleFoo) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = exampleFooName
	if f.Val != "" {
		start.Attr = append(start.Attr, xml.Attr{
			Name:  xml.Name{Space: exampleNS, Local: "val"},
			Value: f.Val,
		})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// UnmarshalXML reads <eg:foo eg:val="…"/>.
func (f *CT_ExampleFoo) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	f.Val = ""
	for _, a := range start.Attr {
		if a.Name.Local == "val" {
			f.Val = a.Value
		}
	}
	return d.Skip()
}

const exampleNS = "https://example.com/docx-go/oxml/example"

var exampleFooName = xml.Name{Space: exampleNS, Local: "foo"}

func init() {
	// Register with the global registry so the roundtripaudit can find it.
	registry.Register(exampleFooName, func() registry.Element {
		return &CT_ExampleFoo{} //nolint:exhaustruct_v5 // registry factory returns zero-value; UnmarshalXML populates fields
	})
	// Teach the ns package about this namespace so canon works.
	// (In real types the namespace is already in ns/ns.go.)
	_ = ns.Qn // keep import used; real types call ns.Qn in their child maps
}
