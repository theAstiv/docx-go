package template_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/docx"
	"github.com/theAstiv/docx-go/docx/template"
)

// TestExecute_PreservesFixtures runs every fixture in the corpus through
// Execute with no data and checks the saved file is byte-identical to a
// plain open and save: executing a template must not touch content that
// holds no tags (Phase 3b exit criterion E2).
func TestExecute_PreservesFixtures(t *testing.T) {
	files, err := filepath.Glob("../../testdata/fixtures/*/*.docx")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no fixtures found")
	}
	checked := 0
	for _, f := range files {
		if strings.HasPrefix(filepath.Base(f), "~$") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if hasTags(t, f, data) {
			continue // a template fixture; TestWordTemplates covers it
		}
		plain := openSave(t, f, data, nil)
		filled := openSave(t, f, data, func(doc *docx.Document) {
			if _, err := template.Execute(doc, nil, template.Options{Mode: template.Lenient}); err != nil {
				t.Errorf("%s: Execute: %v", f, err)
			}
		})
		if !bytes.Equal(plain, filled) {
			t.Errorf("%s: Execute changed a document that has no tags", f)
		}
		checked++
	}
	t.Logf("checked %d fixtures", checked)
}

// hasTags reports whether a fixture holds template tags.
func hasTags(t *testing.T, name string, data []byte) bool {
	t.Helper()
	doc, err := docx.OpenBytes(data)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	ps, _ := template.Placeholders(doc)
	return len(ps) > 0
}

func openSave(t *testing.T, name string, data []byte, mutate func(*docx.Document)) []byte {
	t.Helper()
	doc, err := docx.OpenBytes(data)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if mutate != nil {
		mutate(doc)
	}
	var buf bytes.Buffer
	if err := doc.Save(&buf); err != nil {
		t.Fatalf("%s: save: %v", name, err)
	}
	return buf.Bytes()
}
