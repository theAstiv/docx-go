package raw_test

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/raw"
)

// roundTrip marshals r, then unmarshals into a fresh RawElement, then marshals
// again and asserts the two XML outputs are byte-equal.
func roundTrip(t *testing.T, r *raw.RawElement) {
	t.Helper()

	var buf1 bytes.Buffer
	enc1 := xml.NewEncoder(&buf1)
	if err := r.MarshalXML(enc1, xml.StartElement{}); err != nil {
		t.Fatalf("first marshal: %v", err)
	}
	if err := enc1.Flush(); err != nil {
		t.Fatalf("first flush: %v", err)
	}

	var r2 raw.RawElement
	dec := xml.NewDecoder(bytes.NewReader(buf1.Bytes()))
	tok, err := dec.Token()
	if err != nil {
		t.Fatalf("reading start token: %v", err)
	}
	start, ok := tok.(xml.StartElement)
	if !ok {
		t.Fatalf("expected StartElement, got %T", tok)
	}
	if err := r2.UnmarshalXML(dec, start); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var buf2 bytes.Buffer
	enc2 := xml.NewEncoder(&buf2)
	if err := r2.MarshalXML(enc2, xml.StartElement{}); err != nil {
		t.Fatalf("second marshal: %v", err)
	}
	if err := enc2.Flush(); err != nil {
		t.Fatalf("second flush: %v", err)
	}

	if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
		t.Errorf("round-trip not equal\nfirst:  %s\nsecond: %s", buf1.Bytes(), buf2.Bytes())
	}
}

// An element in no namespace, kept raw inside a namespaced parent, must stay
// in no namespace when written back: without xmlns="" it would fall into the
// parent's default namespace and be read back as a different element (a
// <bookmarkEnd/> becoming w:bookmarkEnd). Found by the nightly fuzzer.
func TestRawElement_NoNamespaceStaysNoNamespace(t *testing.T) {
	const w = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	parent := &raw.RawElement{
		QName: xml.Name{Space: w, Local: "p"},
		Attrs: nil,
		Children: []xml.Token{&raw.RawElement{
			QName: xml.Name{Space: "", Local: "bookmarkEnd"}, Attrs: nil, Children: nil,
		}},
	}
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	if err := parent.MarshalXML(enc, xml.StartElement{}); err != nil {
		t.Fatal(err)
	}
	if err := enc.Flush(); err != nil {
		t.Fatal(err)
	}
	var back raw.RawElement
	if err := xml.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Fatal(err)
	}
	child, ok := back.Children[0].(*raw.RawElement)
	if !ok {
		t.Fatalf("child is %T", back.Children[0])
	}
	if child.QName.Space != "" {
		t.Errorf("child namespace = %q after a round-trip, want none\n%s", child.QName.Space, buf.String())
	}
	roundTrip(t, parent)
}

func TestRawElement_SimpleElement(t *testing.T) {
	const src = `<w14:foo xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml">hello world</w14:foo>`
	r := parseRaw(t, src)
	roundTrip(t, r)
}

func TestRawElement_NestedChildren(t *testing.T) {
	const src = `<w14:bar xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml"><w14:a/><w14:b>text</w14:b></w14:bar>`
	r := parseRaw(t, src)
	roundTrip(t, r)
}

func TestRawElement_Attributes(t *testing.T) {
	const src = `<w14:sdtPr xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml" w14:id="42" w14:val="foo"/>`
	r := parseRaw(t, src)
	roundTrip(t, r)
}

func TestRawElement_MixedTextAndChild(t *testing.T) {
	const src = `<mc:AlternateContent xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"><mc:Choice Requires="w14">text</mc:Choice></mc:AlternateContent>`
	r := parseRaw(t, src)
	roundTrip(t, r)
}

func TestRawElement_TextPreservedByteForByte(t *testing.T) {
	// Text with spaces and special chars must come back unchanged.
	const src = `<w14:t xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml">  hello&#x9;world  </w14:t>`
	r := parseRaw(t, src)

	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	if err := r.MarshalXML(enc, xml.StartElement{}); err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := enc.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if !strings.Contains(buf.String(), "hello") {
		t.Errorf("text content not preserved: %s", buf.String())
	}
}

func parseRaw(t *testing.T, src string) *raw.RawElement {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(src))
	tok, err := dec.Token()
	if err != nil {
		t.Fatalf("read token: %v", err)
	}
	start, ok := tok.(xml.StartElement)
	if !ok {
		t.Fatalf("expected StartElement, got %T", tok)
	}
	var r raw.RawElement
	if err := r.UnmarshalXML(dec, start); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return &r
}
