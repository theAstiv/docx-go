package template_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"regexp"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/docx"
	"github.com/theAstiv/docx-go/docx/template"
)

// pngBytes returns a w×h PNG of one colour.
func pngBytes(t *testing.T, w, h int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// images returns every inline picture in the body's paragraphs and tables.
func images(doc *docx.Document) []*docx.Image {
	var out []*docx.Image
	collect := func(p *docx.Paragraph) {
		for r := range p.Runs() {
			for img := range r.Images() {
				out = append(out, img)
			}
		}
	}
	for p := range doc.Body().Paragraphs() {
		collect(p)
	}
	for tbl := range doc.Body().Tables() {
		for row := range tbl.Rows() {
			for cell := range row.Cells() {
				for p := range cell.Paragraphs() {
					collect(p)
				}
			}
		}
	}
	return out
}

// zipNames returns the entry names of a saved package.
func zipNames(t *testing.T, data []byte) []string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	return names
}

func TestImage_Body(t *testing.T) {
	logo := pngBytes(t, 40, 20, color.Black)
	doc := docx.New()
	p := doc.Body().AddParagraph()
	p.AddRun("Logo: ")
	p.AddRun("{{image logo}}").Font().SetBold(docx.On())
	p.AddRun(" end")
	mustExecute(t, doc, map[string]any{
		"logo": template.Image{Data: logo, Width: docx.Inches(2).EMU(), AltText: "Company logo", Title: "Acme"},
	})
	re := reopen(t, doc)
	if got := bodyText(t, doc); got != "Logo:  end" {
		t.Errorf("text = %q", got)
	}
	imgs := images(re)
	if len(imgs) != 1 {
		t.Fatalf("%d images, want 1", len(imgs))
	}
	w, h := imgs[0].Size()
	if w != docx.Inches(2).EMU() || h != docx.Inches(1).EMU() {
		t.Errorf("size = %d x %d, want 2in x 1in (aspect ratio kept)", w, h)
	}
	if imgs[0].AltText() != "Company logo" || imgs[0].Title() != "Acme" {
		t.Errorf("alt %q, title %q", imgs[0].AltText(), imgs[0].Title())
	}
	if data, err := imgs[0].Bytes(); err != nil || !bytes.Equal(data, logo) {
		t.Errorf("bytes: %v", err)
	}
	// The picture sits in the tag's run, so it keeps the run's formatting.
	for para := range re.Body().Paragraphs() {
		for r := range para.Runs() {
			for range r.Images() {
				if b := r.Font().Bold(); b == nil || !*b {
					t.Error("the picture's run lost the tag run's bold")
				}
			}
		}
	}
}

// An image in a header gets its relationship on the header part.
func TestImage_Header(t *testing.T) {
	doc := docx.New()
	doc.Body().AddParagraph().AddRun("body")
	hdr := doc.LastSection().SetHeader(docx.HeaderDefault)
	hdr.SetText("{{image logo}}")
	logo := pngBytes(t, 8, 8, color.White)
	mustExecute(t, doc, map[string]any{"logo": &template.Image{Data: logo, AltText: "logo"}})
	out := save(t, doc)
	if !strings.Contains(strings.Join(zipNames(t, out), " "), "word/_rels/header1.xml.rels") {
		t.Error("no header relationships part")
	}
	re := reopen(t, doc)
	h := re.LastSection().Header(docx.HeaderDefault)
	n := 0
	for p := range h.Paragraphs() {
		for r := range p.Runs() {
			for img := range r.Images() {
				n++
				if data, err := img.Bytes(); err != nil || !bytes.Equal(data, logo) {
					t.Errorf("header image bytes: %v", err)
				}
			}
		}
	}
	if n != 1 {
		t.Errorf("%d header images, want 1", n)
	}
}

// docPrID matches a drawing id however the encoder prefixed the element.
var docPrID = regexp.MustCompile(`<(?:wp:)?docPr\b[^>]*\sid="(\d+)"`)

// Images in a loop: one per item, unique drawing ids, identical bytes
// stored once.
func TestImage_Loop(t *testing.T) {
	red, blue := pngBytes(t, 4, 4, color.RGBA{R: 255, A: 255}), pngBytes(t, 4, 4, color.RGBA{B: 255, A: 255})
	doc := docx.New()
	doc.Body().AddParagraph().AddRun("before")
	r := doc.Body().AddParagraph().AddRun("")
	if _, err := r.AddImage(bytes.NewReader(red), docx.ImageOptions{AltText: "existing"}); err != nil {
		t.Fatal(err)
	}
	table(doc, [][]string{{"{{#each items}}{{.name}}", "{{image .pic}}{{/each}}"}})
	items := []any{
		map[string]any{"name": "a", "pic": template.Image{Data: red, AltText: "red"}},
		map[string]any{"name": "b", "pic": template.Image{Data: blue, AltText: "blue"}},
		map[string]any{"name": "c", "pic": template.Image{Data: red, AltText: "red again"}},
	}
	mustExecute(t, doc, map[string]any{"items": items})
	if n := len(images(reopen(t, doc))); n != 4 {
		t.Errorf("%d images, want 4", n)
	}
	out := save(t, doc)
	media := 0
	for _, name := range zipNames(t, out) {
		if strings.HasPrefix(name, "word/media/") {
			media++
		}
	}
	if media != 2 {
		t.Errorf("%d media parts, want 2 (red stored once)", media)
	}
	zr, _ := zip.NewReader(bytes.NewReader(out), int64(len(out)))
	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		_, err = buf.ReadFrom(rc)
		if cerr := rc.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		for _, m := range docPrID.FindAllStringSubmatch(buf.String(), -1) {
			if seen[m[1]] {
				t.Errorf("duplicate wp:docPr id %s", m[1])
			}
			seen[m[1]] = true
		}
		if len(seen) != 4 {
			t.Errorf("%d drawing ids, want 4", len(seen))
		}
	}
}

// A failed Execute that already added an image leaves the document exactly
// as it was: no media part, relationship or content type remains.
func TestImage_FailedExecuteIsAtomic(t *testing.T) {
	doc := docx.New()
	paras(doc, "{{image logo}}", "{{missing}}")
	before := save(t, doc)
	_, err := template.Execute(doc, map[string]any{"logo": template.Image{Data: pngBytes(t, 2, 2, color.Black), AltText: "x"}}, template.Options{})
	if !errors.Is(err, template.ErrMissingKey) {
		t.Fatalf("err = %v, want ErrMissingKey", err)
	}
	if after := save(t, doc); !bytes.Equal(after, before) {
		t.Errorf("a failed Execute changed the package: %v", zipNames(t, after))
	}
	// And the document still works: a successful run afterwards.
	mustExecute(t, doc, map[string]any{"logo": template.Image{Data: pngBytes(t, 2, 2, color.Black), AltText: "x"}, "missing": "ok"})
	if n := len(images(reopen(t, doc))); n != 1 {
		t.Errorf("%d images after the second run, want 1", n)
	}
}

func TestImage_Errors(t *testing.T) {
	good := pngBytes(t, 2, 2, color.Black)
	tests := []struct {
		name  string
		paras []string
		data  map[string]any
		want  error
	}{
		{"no alt text", []string{"{{image p}}"}, map[string]any{"p": template.Image{Data: good}}, template.ErrUnsupportedValue},
		{"not an image", []string{"{{image p}}"}, map[string]any{"p": "logo.png"}, template.ErrUnsupportedValue},
		{"bad data", []string{"{{image p}}"}, map[string]any{"p": template.Image{Data: []byte("nope"), AltText: "x"}}, template.ErrUnsupportedValue},
		{"svg", []string{"{{image p}}"}, map[string]any{"p": template.Image{Data: []byte("<svg/>"), AltText: "x"}}, template.ErrUnsupportedValue},
		{"negative size", []string{"{{image p}}"}, map[string]any{"p": template.Image{Data: good, Width: -1, AltText: "x"}}, template.ErrUnsupportedValue},
		{"nil pointer", []string{"{{image p}}"}, map[string]any{"p": (*template.Image)(nil)}, template.ErrUnsupportedValue},
		{"image in a value tag", []string{"{{p}}"}, map[string]any{"p": template.Image{Data: good, AltText: "x"}}, template.ErrUnsupportedValue},
		{"missing key", []string{"{{image p}}"}, map[string]any{}, template.ErrMissingKey},
		{"formatter on an image tag", []string{"{{image p | upper}}"}, map[string]any{}, template.ErrMalformedTag},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := docx.New()
			paras(doc, tt.paras...)
			before := save(t, doc)
			if _, err := template.Execute(doc, tt.data, template.Options{}); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
			if after := save(t, doc); !bytes.Equal(after, before) {
				t.Error("a failed Execute changed the package")
			}
		})
	}
}

// In Lenient mode a missing image is a warning and nothing is placed.
func TestImage_LenientMissing(t *testing.T) {
	doc := docx.New()
	paras(doc, "[{{image p}}]")
	warnings, err := template.Execute(doc, map[string]any{}, template.Options{Mode: template.Lenient})
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !errors.Is(warnings[0].Err, template.ErrMissingKey) {
		t.Errorf("warnings = %v", warnings)
	}
	if got := bodyText(t, doc); got != "[]" || len(images(reopen(t, doc))) != 0 {
		t.Errorf("text %q", got)
	}
}

// An image tag in a branch the data drops adds nothing to the package.
func TestImage_DroppedBranch(t *testing.T) {
	doc := docx.New()
	paras(doc, "{{#if x}}", "{{image p}}", "{{/if}}", "{{#if x}}inline {{image p}}{{/if}}")
	mustExecute(t, doc, map[string]any{"x": false, "p": template.Image{Data: pngBytes(t, 2, 2, color.Black), AltText: "x"}})
	for _, name := range zipNames(t, save(t, doc)) {
		if strings.HasPrefix(name, "word/media/") {
			t.Errorf("media part %s added for a dropped image", name)
		}
	}
}
