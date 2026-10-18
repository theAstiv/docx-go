package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/docx"
	"github.com/theAstiv/docx-go/docx/template"
	"github.com/theAstiv/docx-go/internal/canon"
)

// updateTemplate is not "-update": that flag belongs to the golden files
// (internal/canon).
var updateTemplate = flag.Bool("update-template", false, "rewrite template.docx from buildTemplate")

// TestTemplateUpToDate keeps the committed template.docx in step with
// buildTemplate. Run with -update-template after changing it.
func TestTemplateUpToDate(t *testing.T) {
	d, err := buildTemplate()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatal(err)
	}
	if *updateTemplate {
		if err := os.WriteFile("template.docx", buf.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	if !bytes.Equal(buf.Bytes(), defaultTemplate) {
		t.Fatal("template.docx is out of date: go test ./examples/template_report -update-template")
	}
}

// TestFill_Golden snapshots the filled body and header, canonicalised, so a
// change in how runs, formatting or attributes come out is seen in review.
// Regenerate with -update.
func TestFill_Golden(t *testing.T) {
	doc, _, err := fill(defaultTemplate, defaultData)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := doc.Save(&buf); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"word/document.xml", "word/header1.xml"} {
		f, err := zr.Open(part)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(f)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		canon.Assert(t, "examples/template_report/"+strings.TrimSuffix(part, ".xml"), data)
	}
}

// The template lists what the data must provide.
func TestTemplate_Placeholders(t *testing.T) {
	doc, err := docx.OpenBytes(defaultTemplate)
	if err != nil {
		t.Fatal(err)
	}
	ps, err := template.Placeholders(doc)
	if err != nil {
		t.Fatal(err)
	}
	var header bool
	for _, p := range ps {
		if p.Path == "client.name" && strings.HasPrefix(p.Location.Part, "word/header") {
			header = true
		}
	}
	if !header {
		t.Errorf("no {{client.name}} found in the header: %v", ps)
	}
}

func TestFill(t *testing.T) {
	doc, warnings, err := fill(defaultTemplate, defaultData)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings: %v", warnings)
	}
	var buf bytes.Buffer
	if err := doc.Save(&buf); err != nil {
		t.Fatal(err)
	}
	r, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	var body []string
	for p := range r.Body().Paragraphs() {
		body = append(body, p.Text())
	}
	text := strings.Join(body, "\n")
	for _, want := range []string{
		"Prepared for Example Health Pvt. Ltd., Pune",
		"Overall readiness: 58 of 100",
		"Consent capture and breach response need work before the next audit.\nRetention",
		"G-01 · Consent is bundled with the terms of service",
		"Record consent with a timestamp and the notice version shown.",
		"G-03 · Erasure requests are handled by email only",
		"Address G-01 and G-02 first.\tA follow-up review",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("body lacks %q", want)
		}
	}
	if strings.Contains(text, "No gaps were found") {
		t.Error("the {{#unless gaps}} block was kept")
	}
	if n := strings.Count(text, "Critical: fix before processing resumes."); n != 2 {
		t.Errorf("%d critical notes, want 2 (G-01 and G-02)", n)
	}

	// Scorecard: the header row plus one row per area.
	var rows []string
	for tbl := range r.Body().Tables() {
		for row := range tbl.Rows() {
			var cells []string
			for c := range row.Cells() {
				cells = append(cells, c.Text())
			}
			rows = append(rows, strings.Join(cells, " | "))
		}
	}
	if len(rows) != 6 || rows[0] != "Area | Status | Score" || rows[4] != "Breach notification | Missing | 4" {
		t.Errorf("scorecard rows = %q", rows)
	}

	// Header: the client's name, the logo kept.
	var header []string
	for p := range r.LastSection().Header(docx.HeaderDefault).Paragraphs() {
		header = append(header, p.Text())
		for run := range p.Runs() {
			for range run.Images() {
				header = append(header, "[logo]")
			}
		}
	}
	h := strings.Join(header, " ")
	if !strings.Contains(h, "Example Health Pvt. Ltd. · DPDPA gap assessment") || !strings.Contains(h, "[logo]") {
		t.Errorf("header = %q", h)
	}

	if strings.Contains(text+h+strings.Join(rows, ""), "{{") {
		t.Error("tags left in the output")
	}
}
