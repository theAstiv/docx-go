package roundtrip

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"reflect"
	"testing"

	"github.com/theAstiv/docx-go/internal/canon"
)

// Element is the interface every oxml type must satisfy.
type Element interface {
	MarshalXML(e *xml.Encoder, start xml.StartElement) error
	UnmarshalXML(d *xml.Decoder, start xml.StartElement) error
}

// Verify asserts that marshalling v, unmarshalling the result, and marshalling
// again produces byte-equal output under structural canonicalisation.
// The start element used for the first marshal is passed as startName.
func Verify[T Element](t *testing.T, v T, startName xml.Name) {
	t.Helper()

	first, err := marshalElement(v, startName)
	if err != nil {
		t.Fatalf("roundtrip: first marshal: %v", err)
	}

	v2, err := unmarshalElement[T](first, startName)
	if err != nil {
		t.Fatalf("roundtrip: unmarshal: %v", err)
	}

	second, err := marshalElement(v2, startName)
	if err != nil {
		t.Fatalf("roundtrip: second marshal: %v", err)
	}

	eq, err := canon.Equal(first, second)
	if err != nil {
		t.Fatalf("roundtrip: canonicalise: %v", err)
	}
	if !eq {
		cf, _ := canon.XML(first)
		cs, _ := canon.XML(second)
		t.Errorf("round-trip not equal\nfirst:  %s\nsecond: %s", cf, cs)
	}
}

// VerifyBytes is like Verify but accepts pre-marshalled XML bytes as input.
// Useful when the fixture XML is loaded from a file.
func VerifyBytes[T Element](t *testing.T, xmlBytes []byte, startName xml.Name) {
	t.Helper()

	v, err := unmarshalElement[T](xmlBytes, startName)
	if err != nil {
		t.Fatalf("roundtrip: unmarshal: %v", err)
	}
	second, err := marshalElement(v, startName)
	if err != nil {
		t.Fatalf("roundtrip: re-marshal: %v", err)
	}
	eq, err := canon.Equal(xmlBytes, second)
	if err != nil {
		t.Fatalf("roundtrip: canonicalise: %v", err)
	}
	if !eq {
		cf, _ := canon.XML(xmlBytes)
		cs, _ := canon.XML(second)
		t.Errorf("VerifyBytes round-trip not equal\ninput:  %s\noutput: %s", cf, cs)
	}
}

func marshalElement(v Element, name xml.Name) ([]byte, error) {
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	start := xml.StartElement{Name: name}
	if err := v.MarshalXML(enc, start); err != nil {
		return nil, fmt.Errorf("MarshalXML: %w", err)
	}
	if err := enc.Flush(); err != nil {
		return nil, fmt.Errorf("flush: %w", err)
	}
	return buf.Bytes(), nil
}

// unmarshalElement allocates a fresh T, feeds it the XML, and returns it.
// T must be a pointer type (e.g. *CT_Foo).  The zero value of a pointer is
// nil, so we use reflect.New to allocate the pointed-to struct.
func unmarshalElement[T Element](xmlBytes []byte, _ xml.Name) (T, error) {
	var zero T
	// Allocate a non-nil instance via reflection so pointer receivers work.
	typ := reflect.TypeOf(zero)
	if typ != nil && typ.Kind() == reflect.Ptr {
		v := reflect.New(typ.Elem())
		zero = v.Interface().(T)
	}
	dec := xml.NewDecoder(bytes.NewReader(xmlBytes))
	tok, err := dec.Token()
	if err != nil {
		return zero, fmt.Errorf("read start token: %w", err)
	}
	start, ok := tok.(xml.StartElement)
	if !ok {
		return zero, fmt.Errorf("expected StartElement, got %T", tok)
	}
	if err := zero.UnmarshalXML(dec, start); err != nil {
		return zero, fmt.Errorf("UnmarshalXML: %w", err)
	}
	return zero, nil
}
