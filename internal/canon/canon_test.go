package canon_test

import (
	"errors"
	"testing"

	"github.com/theAstiv/docx-go/internal/canon"
)

func TestXML_AttrSort(t *testing.T) {
	// Attributes in different orders should canonicalise to equal output.
	a := []byte(`<w:rPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" w:b="1" w:i="1"/>`)
	b := []byte(`<w:rPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" w:i="1" w:b="1"/>`)
	eq, err := canon.Equal(a, b)
	if err != nil {
		t.Fatalf("Equal: %v", err)
	}
	if !eq {
		ca, _ := canon.XML(a)
		cb, _ := canon.XML(b)
		t.Errorf("attribute-reordered elements should be equal\na: %s\nb: %s", ca, cb)
	}
}

func TestXML_TextPreservedByteForByte(t *testing.T) {
	// Text with leading/trailing spaces must come back unchanged.
	src := []byte(`<w:t xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">  hello world  </w:t>`)
	out, err := canon.XML(src)
	if err != nil {
		t.Fatalf("XML: %v", err)
	}
	// The text content must not be trimmed.
	outStr := string(out)
	if outStr == "" {
		t.Fatal("canonical output is empty")
	}
	// Verify "  hello world  " (with spaces) is present somewhere in output.
	found := false
	for i := range outStr {
		if i+len("  hello world  ") <= len(outStr) &&
			outStr[i:i+len("  hello world  ")] == "  hello world  " {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("text spaces not preserved: %s", out)
	}
}

func TestXML_DifferentText_NotEqual(t *testing.T) {
	a := []byte(`<w:t xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">hello</w:t>`)
	b := []byte(`<w:t xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">world</w:t>`)
	eq, err := canon.Equal(a, b)
	if err != nil {
		t.Fatalf("Equal: %v", err)
	}
	if eq {
		t.Error("elements with different text should not be equal")
	}
}

// Namespace defects are errors, not differences canonicalised away: the
// round-trip oracle compared outputs only after dropping every declaration,
// so encoder junk and lost declarations both passed.
func TestXML_NamespaceDefects(t *testing.T) {
	const w = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"`
	for name, src := range map[string]string{
		"undeclared element prefix":   `<w:p ` + w + `><w14:x/></w:p>`,
		"undeclared attribute prefix": `<w:p ` + w + `><w:jc main:val="center"/></w:p>`,
		"xmlns junk":                  `<w:p ` + w + ` xmlns:_xmlns="xmlns" _xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml"/>`,
	} {
		if _, err := canon.XML([]byte(src)); !errors.Is(err, canon.ErrNamespace) {
			t.Errorf("%s: err = %v, want ErrNamespace", name, err)
		}
	}
}

func TestXML_SameStructureDifferentNSDecl(t *testing.T) {
	// Same element, namespace declared at different levels — should be equal.
	a := []byte(`<w:p xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:r/></w:p>`)
	b := []byte(`<w:p xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:r xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"/></w:p>`)
	eq, err := canon.Equal(a, b)
	if err != nil {
		t.Fatalf("Equal: %v", err)
	}
	if !eq {
		ca, _ := canon.XML(a)
		cb, _ := canon.XML(b)
		t.Errorf("same structure with redundant ns decl should be equal\na: %s\nb: %s", ca, cb)
	}
}
