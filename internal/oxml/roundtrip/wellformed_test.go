package roundtrip_test

import (
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/roundtrip"
)

func TestNamespaceWellFormed(t *testing.T) {
	const w = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"`
	cases := map[string]bool{
		`<w:p ` + w + `><w:r/></w:p>`:                          true,
		`<w:p ` + w + `><bookmarkEnd/></w:p>`:                  true, // no namespace is allowed
		`<w:p ` + w + ` w:rsidR="00AB" xml:space="preserve"/>`: true,
		`<w:p ` + w + `><A:0 xmlns:A="urn:a"/></w:p>`:          false, // local part "0"
		`<w:p ` + w + ` x:0="1" xmlns:x="urn:x"/>`:             false, // attribute local part "0"
		`<w:p A000=""><bookmarkEnd/></w:p>`:                    false, // undeclared prefix w
		`<w:p ` + w + `><w:r>`:                                 false, // not well formed
	}
	for in, want := range cases {
		if got := roundtrip.NamespaceWellFormed([]byte(in)); got != want {
			t.Errorf("NamespaceWellFormed(%s) = %v, want %v", in, got, want)
		}
	}
}
