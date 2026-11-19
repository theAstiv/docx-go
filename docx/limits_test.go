package docx_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/docx"
	"github.com/theAstiv/docx-go/docx/template"
)

// withDocumentXML returns a new document's package with word/document.xml
// replaced by body (the content of w:body).
func withDocumentXML(t *testing.T, body string) []byte {
	t.Helper()
	var src bytes.Buffer
	if err := docx.New().Save(&src); err != nil {
		t.Fatal(err)
	}
	return rezip(t, src.Bytes(), map[string]string{"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` +
		body + `</w:body></w:document>`})
}

// rezip copies the ZIP in data, replacing the entries in repl.
func rezip(t *testing.T, data []byte, repl map[string]string) []byte {
	t.Helper()
	entries := readZip(t, data)
	for name, r := range repl {
		entries[name] = []byte(r)
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for name, b := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(b); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// TestOpenDepthLimit checks that XML nested right up to the limit opens,
// extracts text, is searched, templated and saved without exhausting the stack, and one level more
// is refused with ErrLimitExceeded.
func TestOpenDepthLimit(t *testing.T) {
	const limit = 20_000
	// document, body, p and r are 4 levels.
	unknown := func(n int) string {
		return `<w:p><w:r><w:t>x</w:t></w:r>` + strings.Repeat(`<u:a xmlns:u="urn:u">`, n) + strings.Repeat(`</u:a>`, n) + `</w:p>`
	}
	// Nested tables: tbl, tr, tc are 3 levels each, the innermost cell holds a
	// paragraph with a run with text (3 levels).
	tables := func(n int) string {
		return strings.Repeat(`<w:tbl><w:tr><w:tc>`, n) + `<w:p><w:r><w:t>x</w:t></w:r></w:p>` +
			strings.Repeat(`</w:tc></w:tr></w:tbl><w:p/>`, n)
	}
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{"unknown at limit", unknown(limit - 3), false},
		{"unknown over limit", unknown(limit - 2), true},
		{"tables at limit", tables((limit - 5) / 3), false},
		{"tables over limit", tables((limit-5)/3 + 1), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := docx.OpenBytes(withDocumentXML(t, tt.body))
			if tt.wantErr {
				if !errors.Is(err, docx.ErrLimitExceeded) {
					t.Fatalf("err = %v, want ErrLimitExceeded", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(d.Text(), "x") {
				t.Errorf("Text() = %q, want it to contain x", d.Text())
			}
			d.Replace("x", "y", docx.ReplaceOptions{})
			if _, err := template.Execute(d, map[string]any{}, template.Options{}); err != nil {
				t.Fatalf("template.Execute: %v", err)
			}
			var buf bytes.Buffer
			if err := d.Save(&buf); err != nil {
				t.Fatal(err)
			}
			if _, err := docx.OpenBytes(buf.Bytes()); err != nil {
				t.Fatalf("reopen: %v", err)
			}
		})
	}
}
