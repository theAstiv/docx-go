package template_test

import (
	"bytes"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/docx"
	"github.com/theAstiv/docx-go/docx/template"
	"github.com/theAstiv/docx-go/internal/canon"
)

// TestWordTemplates_Images fills template_images.docx, typed in Word (P3c.6):
// {{image}} tags split by spell-check marks, an edited letter and formatting
// toggled mid-tag, in the body, the default header, a row loop and both
// branches of inline conditions. JSON cannot carry images, so the data is
// built here rather than read from word-templates.json, and generate-docs.sh
// gets the filled document from this test (DOCX_VALIDATION_DIR).
func TestWordTemplates_Images(t *testing.T) {
	src, err := os.ReadFile("../../testdata/fixtures/word/template_images.docx")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := docx.OpenBytes(src)
	if err != nil {
		t.Fatal(err)
	}
	logo := pngBytes(t, 40, 20, color.Black)
	signature := pngBytes(t, 60, 20, color.RGBA{B: 255, A: 255})
	badge := pngBytes(t, 20, 20, color.RGBA{R: 255, A: 255})
	photo := pngBytes(t, 24, 32, color.Gray{Y: 128})
	clientLogo := pngBytes(t, 30, 30, color.RGBA{G: 255, A: 255})
	data := map[string]any{
		"logo":        template.Image{Data: logo, Width: docx.Inches(1).EMU(), AltText: "Company logo"},
		"signature":   &template.Image{Data: signature, AltText: "Signature", Title: "Signed"},
		"badge":       template.Image{Data: badge, AltText: "Badge"},
		"client_logo": template.Image{Data: clientLogo, Height: docx.Inches(0.5).EMU(), AltText: "Client logo"},
		"show_badge":  true,
		// no_badge is missing: it sits in the dropped {{#unless}} branch.
		"team": []any{
			map[string]any{"name": "A. Author", "photo": template.Image{Data: photo, AltText: "Author"}},
			map[string]any{"name": "R. Reviewer", "photo": template.Image{Data: photo, AltText: "Reviewer"}},
		},
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
		if err := os.WriteFile(filepath.Join(dir, "filled-template_images.docx"), buf.Bytes(), 0o600); err != nil {
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

	// Body: logo, signature, badge and one photo per team member, in order.
	var alts []string
	for _, img := range images(filled) {
		alts = append(alts, img.AltText())
	}
	if got, want := strings.Join(alts, ", "), "Company logo, Signature, Badge, Author, Reviewer"; got != want {
		t.Errorf("body images = %s, want %s", got, want)
	}
	for _, img := range images(filled) {
		if img.AltText() != "Author" {
			continue
		}
		if b, err := img.Bytes(); err != nil || !bytes.Equal(b, photo) {
			t.Errorf("photo bytes: %v", err)
		}
	}
	// The signature tag's run was bold from "image sig" on, but the run
	// holding "{{" was not: the picture takes the first run's formatting.
	for p := range filled.Body().Paragraphs() {
		for r := range p.Runs() {
			for img := range r.Images() {
				if img.AltText() == "Signature" {
					if b := r.Font().Bold(); b != nil && *b {
						t.Error("signature picture took the bold of a later piece of its tag")
					}
				}
			}
		}
	}

	// Header: the client logo, in the header Word typed into.
	n := 0
	for p := range filled.LastSection().Header(docx.HeaderDefault).Paragraphs() {
		for r := range p.Runs() {
			for img := range r.Images() {
				n++
				w, h := img.Size()
				if img.AltText() != "Client logo" || w != docx.Inches(0.5).EMU() || h != docx.Inches(0.5).EMU() {
					t.Errorf("header image %q is %d x %d, want Client logo at 0.5in square", img.AltText(), w, h)
				}
			}
		}
	}
	if n != 1 {
		t.Errorf("%d header images, want 1", n)
	}

	// Five distinct images; the two team photos share one media part.
	var media []string
	for _, name := range zipNames(t, buf.Bytes()) {
		if strings.HasPrefix(name, "word/media/") {
			media = append(media, name)
		}
	}
	if len(media) != 5 {
		t.Errorf("media parts = %v, want 5", media)
	}

	parts := zipParts(t, buf.Bytes())
	var all strings.Builder
	for _, name := range sortedKeys(parts) {
		all.WriteString(xmlText(parts[name]))
		canon.Assert(t, "word-templates/template_images/"+strings.TrimSuffix(name, ".xml"), parts[name])
	}
	for _, w := range []string{"Logo:  (company mark)", "Signed: ", "Badge: ", "A. Author", "R. Reviewer", " Confidential"} {
		if !strings.Contains(all.String(), w) {
			t.Errorf("filled document lacks %q", w)
		}
	}
	for _, w := range []string{"{{", "No badge"} {
		if strings.Contains(all.String(), w) {
			t.Errorf("filled document still has %q", w)
		}
	}
}
