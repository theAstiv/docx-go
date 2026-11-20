package opc_test

import (
	"bytes"
	"encoding/xml"
	"errors"
	"testing"

	"github.com/theAstiv/docx-go/internal/opc"
)

// RootElement accepts only the declaration, processing instructions,
// comments and whitespace before the root. The part parsers used to skip
// every token, so "a<w:document…>" was read as a document.
func TestRootElement(t *testing.T) {
	const root = `<w:document xmlns:w="urn:w"/>`
	ok := map[string]string{
		"root only":            root,
		"declaration":          `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + root,
		"declaration and line": `<?xml version="1.0"?>` + "\r\n" + root,
		"comment and pi":       `<?xml version="1.0"?><!-- c --><?mso-application progid="Word.Document"?>` + "\n\t " + root,
		"byte-order mark":      "\xEF\xBB\xBF" + `<?xml version="1.0"?>` + root,
	}
	for name, src := range ok {
		t.Run(name, func(t *testing.T) {
			start, err := opc.RootElement(xml.NewDecoder(bytes.NewReader([]byte(src))))
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if start.Name.Local != "document" {
				t.Errorf("root = %v, want document", start.Name)
			}
		})
	}
	bad := map[string]string{
		"text":                 "a" + root,
		"text after the decl":  `<?xml version="1.0"?>x` + root,
		"no-break space":       " " + root,
		"ideographic space":    "　" + root,
		"text after a comment": `<!-- c -->.` + root,
		"doctype":              `<!DOCTYPE document>` + root,
	}
	for name, src := range bad {
		t.Run(name, func(t *testing.T) {
			_, err := opc.RootElement(xml.NewDecoder(bytes.NewReader([]byte(src))))
			if !errors.Is(err, opc.ErrMalformedPart) {
				t.Errorf("err = %v, want ErrMalformedPart", err)
			}
		})
	}
	t.Run("no root", func(t *testing.T) {
		if _, err := opc.RootElement(xml.NewDecoder(bytes.NewReader([]byte(`<?xml version="1.0"?>`)))); err == nil {
			t.Error("err = nil for a part without a root element")
		}
	})
}
