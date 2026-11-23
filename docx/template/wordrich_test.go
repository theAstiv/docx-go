package template_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/docx"
	"github.com/theAstiv/docx-go/docx/template"
	"github.com/theAstiv/docx-go/internal/canon"
)

// richRun is one run of a filled paragraph, with the URL of the hyperlink
// holding it ("" outside a hyperlink).
type richRun struct {
	run *docx.Run
	url string
}

// richRuns maps the text of each run in p, direct or inside a hyperlink, to
// the run. Runs with the same text keep the first.
func richRuns(p *docx.Paragraph) map[string]richRun {
	out := map[string]richRun{}
	add := func(r *docx.Run, url string) {
		if _, dup := out[r.Text()]; !dup {
			out[r.Text()] = richRun{run: r, url: url}
		}
	}
	for r := range p.Runs() {
		add(r, "")
	}
	for h := range p.Hyperlinks() {
		for r := range h.Runs() {
			add(r, h.URL())
		}
	}
	return out
}

// TestWordTemplates_Rich fills template_rich.docx, typed in Word:
// rich-value tags split by spell-check marks and edited letters, in runs
// Word made italic, bold or coloured, two in one run, one inside a
// hyperlink Word made, one in a row loop and one in the default header.
// Rich values cannot come from JSON, so the data is built here, and
// generate-docs.sh gets the filled document from this test
// (DOCX_VALIDATION_DIR).
func TestWordTemplates_Rich(t *testing.T) {
	src, err := os.ReadFile("../../testdata/fixtures/word/template_rich.docx")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := docx.OpenBytes(src)
	if err != nil {
		t.Fatal(err)
	}
	rich := func(runs ...template.RichRun) template.Rich { return template.Rich{Runs: runs} }
	data := map[string]any{
		"status": rich(template.RichRun{Text: "On track", Bold: docx.On(), Color: "2E7D32"}),
		"owner":  rich(template.RichRun{Text: "A. Author", Bold: docx.On()}, template.RichRun{Text: " (IT)", Italic: docx.On()}),
		"ref":    rich(template.RichRun{Text: "the policy", Link: "https://example.com/policy"}),
		// Misspelt on purpose, as typed in Word, so spell-check splits the tag.
		"sumary": rich(template.RichRun{Text: "Consent needs work.\nRetention\tfine", Italic: docx.On()}), //nolint:misspell // matches the {{sumary}} tag typed in Word
		// Inside the hyperlink Word made: rich runs, but no links.
		"link_text": rich(template.RichRun{Text: "full", Bold: docx.On()}, template.RichRun{Text: " report"}),
		"mitigations": []any{
			map[string]any{"control": "MFA", "state": rich(template.RichRun{Text: "Open", Color: "C62828"})},
			map[string]any{"control": "Logging", "state": "Done"},
		},
		"doc_title": rich(template.RichRun{Text: "Gap Assessment", Link: "https://example.com/gap"}),
	}
	warnings, err := template.Execute(doc, data, template.Options{Mode: template.Strict})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings: %v", warnings)
	}

	var buf bytes.Buffer
	if err := doc.Save(&buf); err != nil {
		t.Fatal(err)
	}
	if dir := os.Getenv("DOCX_VALIDATION_DIR"); dir != "" {
		if err := os.WriteFile(filepath.Join(dir, "filled-template_rich.docx"), buf.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	filled, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if ps, err := template.Placeholders(filled); err != nil || len(ps) != 0 {
		t.Errorf("after filling: %d tags left (%v), err %v", len(ps), ps, err)
	}

	var paras []*docx.Paragraph
	for p := range filled.Body().Paragraphs() {
		paras = append(paras, p)
	}
	wantText := []string{
		"Status: On track today.",
		"Owner: A. Author (IT), see the policy.",
		"Summary: Consent needs work.\nRetention\tfine",
		"Read the full report online.",
	}
	if len(paras) < len(wantText) {
		t.Fatalf("%d paragraphs, want at least %d", len(paras), len(wantText))
	}
	for i, w := range wantText {
		if got := paras[i].Text(); got != w {
			t.Errorf("paragraph %d = %q, want %q", i, got, w)
		}
	}

	check := func(what string, ok bool) {
		t.Helper()
		if !ok {
			t.Errorf("%s", what)
		}
	}
	need := func(m map[string]richRun, text string) (richRun, bool) {
		t.Helper()
		r, ok := m[text]
		if !ok {
			t.Errorf("no run %q", text)
		}
		return r, ok
	}
	// Word made the whole {{status}} tag italic: the rich run keeps that and
	// adds its own bold and colour; the text around the tag is not italic.
	st := richRuns(paras[0])
	if r, ok := need(st, "On track"); ok {
		f := r.run.Font()
		check("On track: not bold", isOn(f.Bold()))
		check("On track: lost the tag's italic", isOn(f.Italic()))
		check("On track: colour "+f.Color(), f.Color() == "2E7D32")
	}
	if r, ok := st[" today."]; ok {
		check("text after the tag took the tag's italic", !isOn(r.run.Font().Italic()))
	}

	// Two rich tags in one run Word split; the second is a link.
	ow := richRuns(paras[1])
	if r, ok := need(ow, "A. Author"); ok {
		check("A. Author: not bold", isOn(r.run.Font().Bold()))
	}
	if r, ok := need(ow, " (IT)"); ok {
		check("(IT): not italic", isOn(r.run.Font().Italic()))
		check("(IT): took the bold of the run before it", !isOn(r.run.Font().Bold()))
	}
	if r, ok := need(ow, "the policy"); ok {
		check("the policy: link "+r.url, r.url == "https://example.com/policy")
	}

	// The tag's first piece was bold ("{{sum"): the rich run is bold and
	// italic.
	for _, r := range richRuns(paras[2]) {
		if strings.Contains(r.run.Text(), "Consent") {
			check("summary: not bold and italic", isOn(r.run.Font().Bold()) && isOn(r.run.Font().Italic()))
		}
	}

	// Inside Word's hyperlink: the runs stay in that hyperlink, with its URL.
	hl := richRuns(paras[3])
	if r, ok := need(hl, "full"); ok {
		check("full: not bold", isOn(r.run.Font().Bold()))
		check("full: left the hyperlink ("+r.url+")", r.url == "https://example.com/report")
	}
	n := 0
	for range paras[3].Hyperlinks() {
		n++
	}
	check("the paragraph with Word's hyperlink now has more than one hyperlink", n == 1)

	// Row loop: a rich state overrides the cell's navy; a plain one keeps it.
	var states []string
	for tbl := range filled.Body().Tables() {
		for row := range tbl.Rows() {
			var cells []*docx.Cell
			for c := range row.Cells() {
				cells = append(cells, c)
			}
			for p := range cells[len(cells)-1].Paragraphs() {
				for r := range p.Runs() {
					if r.Text() != "" {
						states = append(states, r.Text()+"="+r.Font().Color())
					}
				}
			}
		}
	}
	if got, want := strings.Join(states, " "), "State= Open=C62828 Done=000080"; got != want {
		t.Errorf("state cells = %q, want %q", got, want)
	}

	// Header: a link whose run keeps the bold of "{{doc_".
	hn := 0
	for p := range filled.LastSection().Header(docx.HeaderDefault).Paragraphs() {
		for h := range p.Hyperlinks() {
			hn++
			check("header link "+h.URL(), h.URL() == "https://example.com/gap" && h.Text() == "Gap Assessment")
			for r := range h.Runs() {
				check("header link run lost the tag's bold", isOn(r.Font().Bold()))
			}
		}
		check("header text "+p.Text(), p.Text() == "Gap Assessment - Confidential")
	}
	check("header has no link", hn == 1)

	parts := zipParts(t, buf.Bytes())
	for _, name := range sortedKeys(parts) {
		if strings.Contains(string(parts[name]), "{{") {
			t.Errorf("%s still holds a tag", name)
		}
		canon.Assert(t, "word-templates/template_rich/"+strings.TrimSuffix(name, ".xml"), parts[name])
	}
}
