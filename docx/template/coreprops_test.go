package template_test

import (
	"testing"

	"github.com/theAstiv/docx-go/docx"
	"github.com/theAstiv/docx-go/docx/template"
)

// Core properties set through the API before Execute are filled like any
// other bound part, and are not lost when templating rewrites the part.
func TestExecute_CoreProperties(t *testing.T) {
	doc := docx.New()
	doc.Body().AddParagraph().AddRun("{{report.title}}")
	cp := doc.CoreProperties()
	cp.SetTitle("{{report.title}}")
	cp.SetAuthor("Fixed author")

	data := map[string]any{"report": map[string]any{"title": "Q4 gap assessment"}}
	if _, err := template.Execute(doc, data, template.Options{}); err != nil {
		t.Fatal(err)
	}
	if got := cp.Title(); got != "Q4 gap assessment" {
		t.Errorf("Title = %q", got)
	}
	if got := cp.Author(); got != "Fixed author" {
		t.Errorf("Author = %q", got)
	}

	// A change after Execute is written on Save.
	cp.SetCategory("Report")
	out := save(t, doc)
	re, err := docx.OpenBytes(out)
	if err != nil {
		t.Fatal(err)
	}
	if re.CoreProperties().Title() != "Q4 gap assessment" || re.CoreProperties().Category() != "Report" {
		t.Errorf("after save: title %q, category %q", re.CoreProperties().Title(), re.CoreProperties().Category())
	}
}
