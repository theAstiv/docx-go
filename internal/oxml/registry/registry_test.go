package registry_test

import (
	"bytes"
	"encoding/xml"
	"slices"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/registry"
)

// toyElement is a minimal registered type used only in tests.
type toyElement struct {
	Val string
}

func (t *toyElement) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	t.Val = ""
	for _, a := range start.Attr {
		if a.Name.Local == "val" {
			t.Val = a.Value
		}
	}
	return d.Skip()
}

func (t *toyElement) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if t.Val != "" {
		start.Attr = append(start.Attr, xml.Attr{
			Name:  xml.Name{Local: "val"},
			Value: t.Val,
		})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

var toyName = xml.Name{
	Space: "http://example.com/test",
	Local: "toy",
}

func init() {
	registry.Register(toyName, func() registry.Element { return &toyElement{} })
}

func TestRegistry_New_Registered(t *testing.T) {
	el := registry.New(toyName)
	if el == nil {
		t.Fatal("New returned nil for registered type")
	}
	if _, ok := el.(*toyElement); !ok {
		t.Fatalf("New returned wrong type: %T", el)
	}
}

func TestRegistry_New_Unknown(t *testing.T) {
	unknown := xml.Name{Space: "http://example.com/test", Local: "unknown"}
	if registry.New(unknown) != nil {
		t.Error("New should return nil for unregistered type")
	}
}

func TestRegistry_RoundTrip(t *testing.T) {
	const src = `<toy xmlns="http://example.com/test" val="hello"/>`
	dec := xml.NewDecoder(strings.NewReader(src))
	tok, err := dec.Token()
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	start, ok := tok.(xml.StartElement)
	if !ok {
		t.Fatalf("expected StartElement")
	}

	el := registry.New(start.Name)
	if el == nil {
		t.Fatal("New returned nil")
	}
	if err := el.UnmarshalXML(dec, start); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	if err := el.MarshalXML(enc, start); err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := enc.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	if !strings.Contains(buf.String(), `val="hello"`) {
		t.Errorf("round-trip lost attribute: %s", buf.String())
	}
}

func TestRegistry_Registered_ContainsToy(t *testing.T) {
	names := registry.Registered()
	if !slices.Contains(names, toyName) {
		t.Errorf("toyName not found in Registered(); got %v", names)
	}
}
