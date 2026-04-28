package example_test

import (
	"encoding/xml"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/example"
	"github.com/theAstiv/docx-go/internal/oxml/roundtrip"
)

const exampleNS = "https://example.com/docx-go/oxml/example"

var fooName = xml.Name{Space: exampleNS, Local: "foo"}

func TestCT_ExampleFoo_RoundTrip_Empty(t *testing.T) {
	roundtrip.Verify[*example.CT_ExampleFoo](t, &example.CT_ExampleFoo{}, fooName)
}

func TestCT_ExampleFoo_RoundTrip_WithVal(t *testing.T) {
	roundtrip.Verify[*example.CT_ExampleFoo](t, &example.CT_ExampleFoo{Val: "hello"}, fooName)
}

func TestCT_ExampleFoo_RoundTrip_SpecialChars(t *testing.T) {
	roundtrip.Verify[*example.CT_ExampleFoo](t, &example.CT_ExampleFoo{Val: "<>&\"'"}, fooName)
}
