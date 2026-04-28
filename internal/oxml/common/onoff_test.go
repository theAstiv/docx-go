package common_test

import (
	"bytes"
	"encoding/xml"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/common"
)

var wNS = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"

func bName(local string) xml.Name { return xml.Name{Space: wNS, Local: local} }

// unmarshalOnOff parses a single CT_OnOff element from src.
func unmarshalOnOff(t *testing.T, src string) common.OnOffValue {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(src))
	tok, err := dec.Token()
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	start, ok := tok.(xml.StartElement)
	if !ok {
		t.Fatalf("expected StartElement, got %T", tok)
	}
	var v common.OnOffValue
	if err := v.UnmarshalXML(dec, start); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return v
}

// marshalOnOff encodes v as a single CT_OnOff element with the given name.
func marshalOnOff(t *testing.T, v common.OnOffValue, name xml.Name) string {
	t.Helper()
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	if err := v.MarshalXML(enc, xml.StartElement{Name: name}); err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := enc.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	return buf.String()
}

// ---- Decode tests -----------------------------------------------------------

func TestOnOff_Decode_Bare(t *testing.T) {
	v := unmarshalOnOff(t, `<w:b xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"/>`)
	if !v.Set || !v.True {
		t.Errorf("bare element: want Set=true True=true, got %+v", v)
	}
}

func TestOnOff_Decode_Val1(t *testing.T) {
	v := unmarshalOnOff(t, `<w:b xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" w:val="1"/>`)
	if !v.Set || !v.True {
		t.Errorf("val=1: want Set=true True=true, got %+v", v)
	}
}

func TestOnOff_Decode_ValTrue(t *testing.T) {
	v := unmarshalOnOff(t, `<w:b xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" w:val="true"/>`)
	if !v.Set || !v.True {
		t.Errorf("val=true: want Set=true True=true, got %+v", v)
	}
}

func TestOnOff_Decode_ValOn(t *testing.T) {
	v := unmarshalOnOff(t, `<w:b xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" w:val="on"/>`)
	if !v.Set || !v.True {
		t.Errorf("val=on: want Set=true True=true, got %+v", v)
	}
}

func TestOnOff_Decode_Val0(t *testing.T) {
	v := unmarshalOnOff(t, `<w:b xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" w:val="0"/>`)
	if !v.Set || v.True {
		t.Errorf("val=0: want Set=true True=false, got %+v", v)
	}
}

func TestOnOff_Decode_ValFalse(t *testing.T) {
	v := unmarshalOnOff(t, `<w:b xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" w:val="false"/>`)
	if !v.Set || v.True {
		t.Errorf("val=false: want Set=true True=false, got %+v", v)
	}
}

func TestOnOff_Decode_ValOff(t *testing.T) {
	v := unmarshalOnOff(t, `<w:b xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" w:val="off"/>`)
	if !v.Set || v.True {
		t.Errorf("val=off: want Set=true True=false, got %+v", v)
	}
}

// ECMA-376 capital variants.
func TestOnOff_Decode_ValCapitals(t *testing.T) {
	cases := []struct {
		val      string
		wantTrue bool
	}{
		{"True", true}, {"False", false}, {"On", true}, {"Off", false},
	}
	for _, tc := range cases {
		src := `<w:b xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" w:val="` + tc.val + `"/>`
		v := unmarshalOnOff(t, src)
		if !v.Set || v.True != tc.wantTrue {
			t.Errorf("val=%q: want True=%v, got %+v", tc.val, tc.wantTrue, v)
		}
	}
}

// ---- Encode tests -----------------------------------------------------------

func TestOnOff_Encode_Unset(t *testing.T) {
	out := marshalOnOff(t, common.OnOffUnset, bName("b"))
	if out != "" {
		t.Errorf("unset: expected empty output, got %q", out)
	}
}

func TestOnOff_Encode_True(t *testing.T) {
	out := marshalOnOff(t, common.OnOffTrue, bName("b"))
	if !strings.Contains(out, "<") {
		t.Errorf("true: expected XML element, got %q", out)
	}
	if strings.Contains(out, "val") {
		t.Errorf("true: unexpected val attribute in %q", out)
	}
}

func TestOnOff_Encode_False(t *testing.T) {
	out := marshalOnOff(t, common.OnOffFalse, bName("b"))
	if !strings.Contains(out, `val="false"`) {
		t.Errorf("false: expected val=\"false\" in %q", out)
	}
}

// ---- ToBool / FromBool ------------------------------------------------------

func TestOnOff_ToBool_Nil(t *testing.T) {
	if common.OnOffUnset.ToBool() != nil {
		t.Error("unset ToBool should be nil")
	}
}

func TestOnOff_ToBool_True(t *testing.T) {
	b := common.OnOffTrue.ToBool()
	if b == nil || !*b {
		t.Errorf("OnOffTrue.ToBool() = %v, want &true", b)
	}
}

func TestOnOff_ToBool_False(t *testing.T) {
	b := common.OnOffFalse.ToBool()
	if b == nil || *b {
		t.Errorf("OnOffFalse.ToBool() = %v, want &false", b)
	}
}

func TestOnOff_FromBool_Nil(t *testing.T) {
	v := common.FromBool(nil)
	if v.Set {
		t.Error("FromBool(nil).Set should be false")
	}
}

func TestOnOff_FromBool_True(t *testing.T) {
	tr := true
	v := common.FromBool(&tr)
	if !v.Set || !v.True {
		t.Errorf("FromBool(&true) = %+v, want {Set:true True:true}", v)
	}
}
