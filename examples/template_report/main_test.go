package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"flag"
	"image/png"
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
	// Compare the parts, not the archive bytes: compressed output differs
	// between Go releases (Go 1.27 changed it), and the committed file need
	// not change when only the compressor did.
	got, want := zipEntries(t, buf.Bytes()), zipEntries(t, defaultTemplate)
	if len(got) != len(want) {
		t.Fatalf("template.docx is out of date (%d parts, want %d): go test ./examples/template_report -update-template", len(got), len(want))
	}
	for name, data := range got {
		// The example draws its images with image/png, whose compressed
		// bytes change with the compressor too: compare pixels.
		if strings.HasSuffix(name, ".png") && samePixels(t, data, want[name]) {
			continue
		}
		if !bytes.Equal(data, want[name]) {
			t.Fatalf("template.docx is out of date (%s differs): go test ./examples/template_report -update-template", name)
		}
	}
}

// samePixels reports whether two PNGs decode to the same image.
func samePixels(t *testing.T, a, b []byte) bool {
	t.Helper()
	ia, err := png.Decode(bytes.NewReader(a))
	if err != nil {
		return false
	}
	ib, err := png.Decode(bytes.NewReader(b))
	if err != nil || ia.Bounds() != ib.Bounds() {
		return false
	}
	r := ia.Bounds()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			r1, g1, b1, a1 := ia.At(x, y).RGBA()
			r2, g2, b2, a2 := ib.At(x, y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				return false
			}
		}
	}
	return true
}

// zipEntries returns the uncompressed contents of every entry, by name.
func zipEntries(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		out[f.Name] = b
	}
	return out
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

// With no gaps, the {{else}} of {{#each gaps}} shows instead of the loop.
func TestFill_NoGaps(t *testing.T) {
	var values map[string]any
	if err := json.Unmarshal(defaultData, &values); err != nil {
		t.Fatal(err)
	}
	values["gaps"] = []any{}
	data, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	doc, _, err := fill(defaultTemplate, data)
	if err != nil {
		t.Fatal(err)
	}
	text := doc.Text()
	if !strings.Contains(text, "No gaps were found.") || strings.Contains(text, "Recommended actions") {
		t.Errorf("body = %q", text)
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
		// Formatters.
		"Assessment date: 17 October 2026 · Assessor: A. Assessor",
		"Estimated remediation budget: ₹450,000",
		// Rich value: 58 of 100 is "Partly ready", with a link.
		"Verdict: Partly ready (see the Ministry's data protection pages)",
		// Inline condition.
		"G-01 · Consent is bundled with the terms of service (critical)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("body lacks %q", want)
		}
	}
	if strings.Contains(text, "by email only (critical)") {
		t.Error("the inline condition kept its text for a gap that is not critical")
	}
	if strings.Contains(text, "No gaps were found") {
		t.Error("the {{else}} of {{#each gaps}} was kept")
	}
	if n := strings.Count(text, "Critical: fix before processing resumes."); n != 2 {
		t.Errorf("%d critical notes, want 2 (G-01 and G-02)", n)
	}
	if n := strings.Count(text, "Priority: fix within the next quarter."); n != 1 {
		t.Errorf("%d priority notes from {{else}}, want 1 (G-03)", n)
	}
	var bodyImages, links int
	for p := range r.Body().Paragraphs() {
		for run := range p.Runs() {
			for range run.Images() {
				bodyImages++
			}
		}
		for range p.Hyperlinks() {
			links++
		}
	}
	if bodyImages != 1 || links != 1 {
		t.Errorf("body has %d images and %d links, want the client logo and the verdict's link", bodyImages, links)
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
