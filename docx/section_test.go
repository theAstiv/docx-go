package docx_test

import (
	"bytes"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

func TestDocument_LastSection_Default(t *testing.T) {
	d := docx.New()
	sec := d.LastSection()
	if sec == nil {
		t.Fatal("LastSection() returned nil")
	}
	// New documents have a default sectPr with US Letter page size.
	if sec.PageWidth() == nil {
		t.Error("PageWidth() should not be nil on a default section")
	}
}

func TestSection_SetPageSize(t *testing.T) {
	d := docx.New()
	sec := d.LastSection()

	// A4 page: 11906 × 16838 twips.
	sec.SetPageSize(11906, 16838)

	if got := sec.PageWidth(); got == nil || *got != 11906 {
		t.Errorf("PageWidth() = %v, want 11906", got)
	}
	if got := sec.PageHeight(); got == nil || *got != 16838 {
		t.Errorf("PageHeight() = %v, want 16838", got)
	}

	// Verify round-trip.
	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes() error: %v", err)
	}
	sec2 := d2.LastSection()
	if got := sec2.PageWidth(); got == nil || *got != 11906 {
		t.Errorf("PageWidth() after round-trip = %v, want 11906", got)
	}
	if got := sec2.PageHeight(); got == nil || *got != 16838 {
		t.Errorf("PageHeight() after round-trip = %v, want 16838", got)
	}
}

func TestSection_SetMargins(t *testing.T) {
	d := docx.New()
	sec := d.LastSection()
	sec.SetMargins(docx.PageMargins{
		Top:    720,
		Right:  720,
		Bottom: 720,
		Left:   720,
	})

	m := sec.Margins()
	if m.Top != 720 || m.Left != 720 {
		t.Errorf("Margins() = %+v, want all 720", m)
	}

	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes() error: %v", err)
	}
	m2 := d2.LastSection().Margins()
	if m2.Top != 720 {
		t.Errorf("Margins().Top after round-trip = %d, want 720", m2.Top)
	}
}

func TestSection_Header_SetAndGet(t *testing.T) {
	d := docx.New()
	sec := d.LastSection()

	// SetHeader creates the header part.
	hdr := sec.SetHeader(docx.HeaderDefault)
	if hdr == nil {
		t.Fatal("SetHeader() returned nil")
	}

	// Add content to the header.
	p := hdr.AddParagraph()
	p.AddRun("page header text")

	// Header() returns the same header.
	hdr2 := sec.Header(docx.HeaderDefault)
	if hdr2 == nil {
		t.Fatal("Header() returned nil after SetHeader")
	}

	// Round-trip.
	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes() error: %v", err)
	}

	sec2 := d2.LastSection()
	hdr3 := sec2.Header(docx.HeaderDefault)
	if hdr3 == nil {
		t.Fatal("Header() returned nil after round-trip")
	}

	var texts []string
	for p2 := range hdr3.Paragraphs() {
		if t2 := p2.Text(); t2 != "" {
			texts = append(texts, t2)
		}
	}
	if len(texts) == 0 || texts[0] != "page header text" {
		t.Errorf("header paragraph texts = %v, want [page header text]", texts)
	}
}

func TestSection_Footer_SetAndGet(t *testing.T) {
	d := docx.New()
	sec := d.LastSection()

	ftr := sec.SetFooter(docx.FooterDefault)
	if ftr == nil {
		t.Fatal("SetFooter() returned nil")
	}
	ftr.AddParagraph().AddRun("page footer")

	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes() error: %v", err)
	}
	sec2 := d2.LastSection()
	ftr2 := sec2.Footer(docx.FooterDefault)
	if ftr2 == nil {
		t.Fatal("Footer() returned nil after round-trip")
	}
	var texts []string
	for p := range ftr2.Paragraphs() {
		if t := p.Text(); t != "" {
			texts = append(texts, t)
		}
	}
	if len(texts) == 0 || texts[0] != "page footer" {
		t.Errorf("footer texts = %v, want [page footer]", texts)
	}
}

func TestSection_TitlePg(t *testing.T) {
	d := docx.New()
	sec := d.LastSection()

	if sec.TitlePg() {
		t.Error("TitlePg() on new section should be false")
	}
	sec.EnableTitlePage(true)
	if !sec.TitlePg() {
		t.Error("TitlePg() after EnableTitlePage(true) should be true")
	}

	// First-page header.
	hdr := sec.SetHeader(docx.HeaderFirst)
	hdr.AddParagraph().AddRun("first page header")

	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes() error: %v", err)
	}
	sec2 := d2.LastSection()
	if !sec2.TitlePg() {
		t.Error("TitlePg() after round-trip should be true")
	}
	hdr2 := sec2.Header(docx.HeaderFirst)
	if hdr2 == nil {
		t.Fatal("first-page header should survive round-trip")
	}
}

func TestSection_MultipleHeaderTypes(t *testing.T) {
	d := docx.New()
	sec := d.LastSection()

	sec.SetHeader(docx.HeaderDefault).AddParagraph().AddRun("default header")
	sec.SetHeader(docx.HeaderFirst).AddParagraph().AddRun("first header")
	sec.EnableTitlePage(true)

	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes() error: %v", err)
	}

	sec2 := d2.LastSection()
	def := sec2.Header(docx.HeaderDefault)
	first := sec2.Header(docx.HeaderFirst)
	if def == nil || first == nil {
		t.Fatalf("expected both default and first headers, got def=%v first=%v", def, first)
	}
	wantDef, wantFirst := "default header", "first header"
	for p := range def.Paragraphs() {
		if got := p.Text(); got != "" && got != wantDef {
			t.Errorf("default header paragraph = %q, want %q", got, wantDef)
		}
	}
	for p := range first.Paragraphs() {
		if got := p.Text(); got != "" && got != wantFirst {
			t.Errorf("first header paragraph = %q, want %q", got, wantFirst)
		}
	}
}

func TestSection_FixtureRoundTrip(t *testing.T) {
	// A document with body content, a default header/footer, and custom page size.
	d := docx.New()
	d.Body().AddParagraph().AddRun("document body")

	sec := d.LastSection()
	sec.SetPageSize(12240, 15840) // US Letter
	sec.SetMargins(docx.PageMargins{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440})
	sec.SetHeader(docx.HeaderDefault).AddParagraph().AddRun("header content")
	sec.SetFooter(docx.FooterDefault).AddParagraph().AddRun("footer content")

	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes() error: %v", err)
	}

	// Verify body.
	var bodyTexts []string
	for p := range d2.Body().Paragraphs() {
		if t := p.Text(); t != "" {
			bodyTexts = append(bodyTexts, t)
		}
	}
	if len(bodyTexts) == 0 || bodyTexts[0] != "document body" {
		t.Errorf("body paragraphs = %v, want [document body]", bodyTexts)
	}

	// Verify page geometry.
	sec2 := d2.LastSection()
	if got := sec2.PageWidth(); got == nil || *got != 12240 {
		t.Errorf("PageWidth() = %v, want 12240", got)
	}

	// Verify header/footer.
	hdr := sec2.Header(docx.HeaderDefault)
	ftr := sec2.Footer(docx.FooterDefault)
	if hdr == nil || ftr == nil {
		t.Fatalf("header or footer nil after round-trip")
	}
}
