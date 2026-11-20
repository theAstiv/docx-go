package docx_test

import (
	"errors"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

// Every parser of package XML refuses text and DOCTYPE declarations before
// the root element (opc.RootElement). They used to skip any token there, so
// "a<w:document…>" opened as a normal document; together with a depth check
// that only looked at parts starting with '<', that let a nesting bomb
// through. Each part kind goes through the shared check.
func TestOpen_RefusesTextBeforeRoot(t *testing.T) {
	good := map[string]string{
		"[Content_Types].xml":          contentTypes,
		"_rels/.rels":                  rels(rel("rId1", relDoc, "word/document.xml")),
		"word/document.xml":            documentXML("hello", ""),
		"word/_rels/document.xml.rels": rels(rel("rId1", relStyles, "styles.xml")),
		"word/styles.xml":              stylesXML("Mine"),
	}
	if _, err := docx.OpenBytes(zipFiles(t, good)); err != nil {
		t.Fatalf("the well-formed package does not open: %v", err)
	}
	for _, part := range []string{"[Content_Types].xml", "_rels/.rels", "word/document.xml", "word/_rels/document.xml.rels", "word/styles.xml"} {
		for lead, desc := range map[string]string{"a": "text", " ": "no-break space", "<!DOCTYPE x>": "doctype"} {
			t.Run(part+"/"+desc, func(t *testing.T) {
				files := map[string]string{}
				for k, v := range good {
					files[k] = v
				}
				files[part] = lead + files[part]
				if _, err := docx.OpenBytes(zipFiles(t, files)); !errors.Is(err, docx.ErrMalformedPart) {
					t.Errorf("err = %v, want ErrMalformedPart", err)
				}
			})
		}
	}
}
