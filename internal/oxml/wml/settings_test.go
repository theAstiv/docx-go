package wml_test

import (
	"bytes"
	"encoding/xml"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/ns"
	"github.com/theAstiv/docx-go/internal/oxml/roundtrip"
	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

func TestCT_Settings_RoundTrip(t *testing.T) {
	input := `<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" mc:Ignorable="w14"><w:evenAndOddHeaders/><w:defaultTabStop w:val="720"/><w:compat><w:compatSetting w:name="compatibilityMode" w:uri="http://schemas.microsoft.com/office/word" w:val="15"/></w:compat></w:settings>`
	roundtrip.VerifyBytes[*wml.CT_Settings](t, []byte(input), ns.Qn("w:settings"))
}

func TestCT_Settings_EvenAndOddHeaders(t *testing.T) {
	s := new(wml.CT_Settings)
	if s.EvenAndOddHeaders() != nil {
		t.Fatal("expected nil before setting")
	}
	tr := true
	s.SetEvenAndOddHeaders(&tr)
	got := s.EvenAndOddHeaders()
	if got == nil || !*got {
		t.Fatalf("expected true after set, got %v", got)
	}
	s.SetEvenAndOddHeaders(nil)
	if s.EvenAndOddHeaders() != nil {
		t.Fatal("expected nil after clear")
	}
}

func TestCT_Settings_DefaultTabStop(t *testing.T) {
	s := new(wml.CT_Settings)
	if s.DefaultTabStop() != nil {
		t.Fatal("expected nil before setting")
	}
	v := int64(720)
	s.SetDefaultTabStop(&v)
	got := s.DefaultTabStop()
	if got == nil || *got != 720 {
		t.Fatalf("expected 720 after set, got %v", got)
	}
	s.SetDefaultTabStop(nil)
	if s.DefaultTabStop() != nil {
		t.Fatal("expected nil after clear")
	}
}

func TestCT_Settings_UnknownChildrenPassThrough(t *testing.T) {
	input := `<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:unknown1/><w:defaultTabStop w:val="360"/><w:unknown2 w:attr="x"/></w:settings>`
	s := new(wml.CT_Settings)
	dec := xml.NewDecoder(bytes.NewReader([]byte(input)))
	var start xml.StartElement
	for {
		tok, err := dec.Token()
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		if se, ok := tok.(xml.StartElement); ok {
			start = se
			break
		}
	}
	if err := s.UnmarshalXML(dec, start); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	v := s.DefaultTabStop()
	if v == nil || *v != 360 {
		t.Fatalf("DefaultTabStop: got %v, want 360", v)
	}
	// Re-marshal and verify unknown children are present.
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	if err := s.MarshalXML(enc, xml.StartElement{}); err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := enc.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	out := buf.String()
	// Check element names and the attribute value (prefix may change due to
	// encoding/xml namespace mangling; nsfix corrects this at the parts layer).
	for _, want := range []string{"unknown1", "unknown2", `attr="x"`} {
		if !bytes.Contains(buf.Bytes(), []byte(want)) {
			t.Errorf("output missing %q: %s", want, out)
		}
	}
}
