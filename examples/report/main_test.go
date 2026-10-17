package main

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

// TestReport_AllPhase3aFeaturesSurviveRoundTrip is the Phase 3a exit gate for
// the example: it builds the report, saves it, reopens the bytes, and checks
// every feature through the public API.
func TestReport_AllPhase3aFeaturesSurviveRoundTrip(t *testing.T) {
	d, err := build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("save: %v", err)
	}
	r, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}

	// Styles.
	for _, id := range []string{"Title", "Heading1", "Heading2", "TableGrid", "Caption", "TOC1", "Hyperlink"} {
		if r.Styles().ByID(id) == nil {
			t.Errorf("style %s missing", id)
		}
	}
	if got := r.Styles().ByID("Heading1").Font().Color(); got != "1F3864" {
		t.Errorf("Heading1 colour = %q", got)
	}

	// Bookmarks and links.
	for _, name := range []string{"sec_summary", "sec_findings"} {
		if r.Bookmark(name) == nil {
			t.Errorf("bookmark %s missing", name)
		}
	}
	var internal, external, toc int
	for p := range r.Body().Paragraphs() {
		for h := range p.Hyperlinks() {
			switch {
			case h.Anchor() == "sec_findings" && h.URL() == "":
				internal++
			case h.URL() == "https://www.first.org/cvss/":
				external++
			}
		}
		for f := range p.Fields() {
			if f.Code() == "TOC" {
				toc++
			}
		}
	}
	if internal != 1 || external != 1 || toc != 1 {
		t.Errorf("internal links %d, external links %d, TOC fields %d; want 1 each", internal, external, toc)
	}

	// Table: repeating header row, borders, shaded severity cells.
	var tbl *docx.Table
	for tb := range r.Body().Tables() {
		tbl = tb
	}
	if tbl == nil {
		t.Fatal("findings table missing")
	}
	var rows []*docx.Row
	for row := range tbl.Rows() {
		rows = append(rows, row)
	}
	if len(rows) != len(findings)+1 || !rows[0].IsHeader() {
		t.Errorf("rows = %d, header row = %v", len(rows), rows[0].IsHeader())
	}
	if b := tbl.Borders(); b.Top == nil || b.InsideV == nil || b.Top.Color != "808080" {
		t.Errorf("table borders = %+v", b)
	}
	if tbl.Layout() != docx.LayoutFixed {
		t.Errorf("layout = %q", tbl.Layout())
	}
	if w, ok := tbl.Width(); !ok || w.Kind() != docx.WidthKindPercent {
		t.Errorf("table width = %+v ok=%v", w, ok)
	}
	for i, f := range findings {
		cells := cellsOf(rows[i+1])
		if got := cells[2].Shading(); got != severityFill[f.severity] {
			t.Errorf("%s severity fill = %q, want %q", f.id, got, severityFill[f.severity])
		}
	}

	// Image: alt text, size, and a decodable PNG.
	var imgs []*docx.Image
	for p := range r.Body().Paragraphs() {
		for run := range p.Runs() {
			for im := range run.Images() {
				imgs = append(imgs, im)
			}
		}
	}
	if len(imgs) != 1 {
		t.Fatalf("%d images, want 1", len(imgs))
	}
	if imgs[0].AltText() == "" {
		t.Error("image has no alt text")
	}
	w, h := imgs[0].Size()
	if w != docx.Inches(4).EMU() || h != docx.Inches(2).EMU() {
		t.Errorf("image size = %d x %d EMU, want 4in x 2in", w, h)
	}
	raw, err := imgs[0].Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(raw)); err != nil {
		t.Errorf("stored image does not decode: %v", err)
	}

	// Footer: Page X of Y, with no footer on the title page.
	sec := r.LastSection()
	if !sec.TitlePg() {
		t.Error("title page footer suppression lost")
	}
	ftr := sec.Footer(docx.FooterDefault)
	if ftr == nil {
		t.Fatal("footer missing")
	}
	var codes []string
	for p := range ftr.Paragraphs() {
		for f := range p.Fields() {
			codes = append(codes, f.Code())
		}
	}
	if len(codes) != 2 || codes[0] != "PAGE" || codes[1] != "NUMPAGES" {
		t.Errorf("footer fields = %v", codes)
	}

	if !r.UpdateFieldsOnOpen() {
		t.Error("updateFields on open lost")
	}
}
