package template_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/docx"
	"github.com/theAstiv/docx-go/docx/template"
	"github.com/theAstiv/docx-go/internal/canon"
)

// filledParts are the parts a filled template can change: stories, plus the
// document properties and custom XML that data-bound content controls read.
var filledParts = regexp.MustCompile(`^(word/(document|header\d+|footer\d+)\.xml|docProps/core\.xml|customXml/item\d+\.xml)$`)

// TestWordTemplates fills the templates made in Microsoft Word itself
// (testdata/fixtures/word/template_*.docx, PLAN-3B P3b.8.1): tags Word split
// across runs, spell-check marks, revision ids, a table of contents, a
// built-in cover page with data-bound content controls. Every tag must be
// filled, and the expected text must come out.
func TestWordTemplates(t *testing.T) {
	raw, err := os.ReadFile("../../testdata/template/word-templates.json")
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		file    string
		want    []string // text that must appear in some filled part
		notWant []string // text that must not appear anywhere
	}{
		{"template_scalars", []string{
			"Acme Health & Co", "Pune", "411001", "12 MG Road", "R. Reviewer", "Partially ready",
			"58", "the full findings", "Data Protection Office", "author@example.com", "1.2",
			"Consent and breach response need work.",
		}, nil},
		{"template_header_footer", []string{
			"DPDPA Gap Assessment", "Acme Health & Co", "A. Author", "18 October 2026", "GA-2026-017",
		}, nil},
		{"template_sections", []string{
			"G-01", "Consent bundled with terms", "One acceptance covers the terms and all purposes.",
			"Separate consent from the terms.", "Ask per purpose.", "G-02", "Track requests in a queue.",
			"S. Assessor",
		}, []string{"No gaps were found"}},
		{"template_table_loop", []string{
			"Spoofing", "Tampering", "Repudiation", "High", "MFA for admins", "Audit logging", "Done",
			"Dec 2026", "S. Assessor", "Reviewed on site",
		}, nil},
		{"template_cover_sdt", []string{
			"DPDPA Gap Assessment", "Acme Health & Co", "A. Author", "18 October 2026",
		}, nil},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			src, err := os.ReadFile("../../testdata/fixtures/word/" + c.file + ".docx")
			if err != nil {
				t.Fatal(err)
			}
			doc, err := docx.OpenBytes(src)
			if err != nil {
				t.Fatal(err)
			}
			// The recommended call for any template with a TOC or page
			// fields: Word refreshes them from the filled text on open.
			doc.SetUpdateFieldsOnOpen(true)
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
			filled, err := docx.OpenBytes(buf.Bytes())
			if err != nil {
				t.Fatalf("reopen: %v", err)
			}
			ps, err := template.Placeholders(filled)
			if err != nil || len(ps) != 0 {
				t.Errorf("after filling: %d tags left (%v), err %v", len(ps), ps, err)
			}

			parts := zipParts(t, buf.Bytes())
			var all strings.Builder
			for _, name := range sortedKeys(parts) {
				all.WriteString(xmlText(parts[name]))
				all.WriteString("\n")
				canon.Assert(t, "word-templates/"+c.file+"/"+strings.TrimSuffix(name, ".xml"), parts[name])
			}
			for _, w := range c.want {
				if !strings.Contains(all.String(), w) {
					t.Errorf("filled document lacks %q", w)
				}
			}
			for _, w := range c.notWant {
				if strings.Contains(all.String(), w) {
					t.Errorf("filled document still has %q", w)
				}
			}
		})
	}
}

// A TOC that repeats heading tags is left for Word to rebuild, so Execute
// warns unless Word is told to update fields on open.
func TestWordTemplates_TOCWarning(t *testing.T) {
	src, err := os.ReadFile("../../testdata/fixtures/word/template_sections.docx")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../testdata/template/word-templates.json")
	if err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	doc, err := docx.OpenBytes(src)
	if err != nil {
		t.Fatal(err)
	}
	warnings, err := template.Execute(doc, data, template.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0].Err.Error(), "SetUpdateFieldsOnOpen") ||
		warnings[0].Location.Part != "word/document.xml" {
		t.Errorf("warnings = %v, want one table-of-contents warning", warnings)
	}
}

// The cover page's data-bound controls read docProps/core.xml and a custom
// XML item; Word re-syncs them on open, so those parts must be filled too.
func TestWordTemplates_CoverBoundData(t *testing.T) {
	src, err := os.ReadFile("../../testdata/fixtures/word/template_cover_sdt.docx")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := docx.OpenBytes(src)
	if err != nil {
		t.Fatal(err)
	}
	data := map[string]any{
		"report":     map[string]any{"title": "Title & More"},
		"client":     map[string]any{"name": "Client"},
		"author":     map[string]any{"name": "Author"},
		"assessment": map[string]any{"date": "18 October 2026"},
	}
	if _, err := template.Execute(doc, data, template.Options{}); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := doc.Save(&buf); err != nil {
		t.Fatal(err)
	}
	parts := zipParts(t, buf.Bytes())
	core := string(parts["docProps/core.xml"])
	for _, want := range []string{"<dc:title>Title &amp; More</dc:title>", "<dc:subject>Client</dc:subject>"} {
		if !strings.Contains(core, want) {
			t.Errorf("core.xml lacks %s:\n%s", want, core)
		}
	}
	if strings.Contains(string(parts["customXml/item1.xml"]), "{{") {
		t.Errorf("custom XML still holds a tag:\n%s", parts["customXml/item1.xml"])
	}
	// Only text changed: the original prefixes and declarations survive.
	orig := zipParts(t, src)["docProps/core.xml"]
	if strings.Count(core, "xmlns:") != strings.Count(string(orig), "xmlns:") || !strings.Contains(core, "<cp:coreProperties") {
		t.Errorf("core.xml structure changed:\n%s", core)
	}
}

// zipParts returns the filled-template parts of a .docx.
func zipParts(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	for _, f := range zr.File {
		if !filledParts.MatchString(f.Name) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(rc)
		if err != nil {
			t.Fatal(err)
		}
		if err := rc.Close(); err != nil {
			t.Fatal(err)
		}
		out[f.Name] = b
	}
	return out
}

var xmlTag = regexp.MustCompile(`<[^>]*>`)

// xmlText returns the text content of an XML part, roughly: tags removed,
// the common entities decoded. Enough to look for filled values.
func xmlText(b []byte) string {
	s := xmlTag.ReplaceAllString(string(b), "")
	return strings.NewReplacer("&amp;", "&", "&lt;", "<", "&gt;", ">", "&quot;", `"`, "&apos;", "'", "&#39;", "'").Replace(s)
}

func sortedKeys(m map[string][]byte) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
