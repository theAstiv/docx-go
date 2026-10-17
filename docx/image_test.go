package docx_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

const emuPx = 9525 // EMU per pixel at 96 DPI

func pngBytes(t *testing.T, w, h int, shade uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: shade, G: uint8(x), B: uint8(y), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zipEntries(t *testing.T, d *docx.Document) map[string][]byte {
	t.Helper()
	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		if _, err := b.ReadFrom(rc); err != nil {
			t.Fatal(err)
		}
		if err := rc.Close(); err != nil {
			t.Fatal(err)
		}
		out[f.Name] = b.Bytes()
	}
	return out
}

func mediaNames(entries map[string][]byte) []string {
	var out []string
	for name := range entries {
		if strings.HasPrefix(name, "word/media/") {
			out = append(out, name)
		}
	}
	return out
}

// firstImage returns the first image found in the body's top-level paragraphs.
func firstImage(d *docx.Document) *docx.Image {
	for p := range d.Body().Paragraphs() {
		for r := range p.Runs() {
			for img := range r.Images() {
				return img
			}
		}
	}
	return nil
}

func TestImage_DefaultSizeIs96DPI(t *testing.T) {
	d := docx.New()
	run := d.Body().AddParagraph().AddRun("")
	img, err := run.AddImage(bytes.NewReader(pngBytes(t, 40, 20, 10)), docx.ImageOptions{AltText: "grid"})
	if err != nil {
		t.Fatalf("AddImage: %v", err)
	}
	w, h := img.Size()
	if w != 40*emuPx || h != 20*emuPx {
		t.Errorf("size = %d x %d, want %d x %d", w, h, 40*emuPx, 20*emuPx)
	}
	if img.AltText() != "grid" {
		t.Errorf("AltText = %q", img.AltText())
	}
}

func TestImage_AspectRatioFromOneDimension(t *testing.T) {
	d := docx.New()
	run := d.Body().AddParagraph().AddRun("")
	data := pngBytes(t, 200, 100, 0)

	byWidth, err := run.AddImage(bytes.NewReader(data), docx.ImageOptions{Width: docx.EMUInches(2)})
	if err != nil {
		t.Fatal(err)
	}
	if w, h := byWidth.Size(); w != docx.EMUInches(2) || h != docx.EMUInches(1) {
		t.Errorf("by width = %d x %d, want 2in x 1in", w, h)
	}
	byHeight, err := run.AddImage(bytes.NewReader(data), docx.ImageOptions{Height: docx.EMUInches(3)})
	if err != nil {
		t.Fatal(err)
	}
	if w, h := byHeight.Size(); w != docx.EMUInches(6) || h != docx.EMUInches(3) {
		t.Errorf("by height = %d x %d, want 6in x 3in", w, h)
	}
	both, err := run.AddImage(bytes.NewReader(data), docx.ImageOptions{Width: 1000, Height: 5000})
	if err != nil {
		t.Fatal(err)
	}
	if w, h := both.Size(); w != 1000 || h != 5000 {
		t.Errorf("explicit = %d x %d", w, h)
	}
}

func TestImage_RoundTripPreservesBytesAndMetadata(t *testing.T) {
	d := docx.New()
	original := pngBytes(t, 64, 48, 77)
	run := d.Body().AddParagraph().AddRun("")
	if _, err := run.AddImage(bytes.NewReader(original), docx.ImageOptions{AltText: "A gradient", Title: "Fig. 1"}); err != nil {
		t.Fatal(err)
	}

	d2 := reopen(t, d)
	img := firstImage(d2)
	if img == nil {
		t.Fatal("image lost on round-trip")
	}
	got, err := img.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Error("image bytes changed on round-trip")
	}
	if w, h := img.Size(); w != 64*emuPx || h != 48*emuPx {
		t.Errorf("size after round-trip = %d x %d", w, h)
	}
	if img.AltText() != "A gradient" || img.Title() != "Fig. 1" {
		t.Errorf("alt/title = %q/%q", img.AltText(), img.Title())
	}

	// The media part itself is byte-identical and typed in [Content_Types].xml.
	entries := zipEntries(t, d2)
	media := mediaNames(entries)
	if len(media) != 1 || !bytes.Equal(entries[media[0]], original) {
		t.Fatalf("media parts = %v", media)
	}
	if !strings.Contains(string(entries["[Content_Types].xml"]), `image/png`) {
		t.Errorf("content types lack image/png: %s", entries["[Content_Types].xml"])
	}
	if !strings.Contains(string(entries["word/_rels/document.xml.rels"]), "media/image1.png") {
		t.Errorf("document rels lack the image: %s", entries["word/_rels/document.xml.rels"])
	}
}

func TestImage_EmptyAltTextIsWrittenNotOmitted(t *testing.T) {
	d := docx.New()
	run := d.Body().AddParagraph().AddRun("")
	if _, err := run.AddImage(bytes.NewReader(pngBytes(t, 4, 4, 1)), docx.ImageOptions{}); err != nil {
		t.Fatal(err)
	}
	xml := string(partXML(t, d, "word/document.xml"))
	if !strings.Contains(xml, `descr=""`) {
		t.Errorf("empty alt text should be written as descr=\"\": %s", xml)
	}
	d2 := reopen(t, d)
	if img := firstImage(d2); img == nil || img.AltText() != "" {
		t.Error("decorative image lost or gained alt text")
	}
}

func TestImage_SetSizeKeepsPictureTransformInStep(t *testing.T) {
	d := docx.New()
	run := d.Body().AddParagraph().AddRun("")
	img, err := run.AddImage(bytes.NewReader(pngBytes(t, 10, 10, 3)), docx.ImageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	img.SetSize(123456, 654321)
	img.SetSize(0, 5) // ignored: must be positive
	img.SetAltText("new alt")

	xml := string(partXML(t, d, "word/document.xml"))
	// Both wp:extent and pic:spPr/a:xfrm/a:ext carry the new size.
	if strings.Count(xml, `cx="123456"`) != 2 || strings.Count(xml, `cy="654321"`) != 2 {
		t.Errorf("extent and transform out of step: %s", xml)
	}
	d2 := reopen(t, d)
	got := firstImage(d2)
	if w, h := got.Size(); w != 123456 || h != 654321 || got.AltText() != "new alt" {
		t.Errorf("after round-trip: %d x %d alt %q", w, h, got.AltText())
	}
}

func TestImage_DuplicateBytesStoredOnce(t *testing.T) {
	d := docx.New()
	logo := pngBytes(t, 16, 16, 200)
	other := pngBytes(t, 16, 16, 100)
	p := d.Body().AddParagraph()
	for _, data := range [][]byte{logo, logo, other, logo} {
		if _, err := p.AddRun("").AddImage(bytes.NewReader(data), docx.ImageOptions{AltText: "x"}); err != nil {
			t.Fatal(err)
		}
	}
	entries := zipEntries(t, d)
	if media := mediaNames(entries); len(media) != 2 {
		t.Errorf("media parts = %v, want 2 (logo once, other once)", media)
	}
	rels := string(entries["word/_rels/document.xml.rels"])
	if n := strings.Count(rels, "relationships/image"); n != 2 {
		t.Errorf("%d image relationships, want 2: %s", n, rels)
	}
}

func TestImage_DocPrIDsUniqueAcrossStories(t *testing.T) {
	d := docx.New()
	data := pngBytes(t, 8, 8, 9)
	add := func(r *docx.Run) {
		if _, err := r.AddImage(bytes.NewReader(data), docx.ImageOptions{AltText: "x"}); err != nil {
			t.Fatal(err)
		}
	}
	add(d.Body().AddParagraph().AddRun(""))
	hdr := d.LastSection().SetHeader(docx.HeaderDefault)
	add(hdr.AddParagraph().AddRun(""))
	tbl := d.Body().AddTable(1, 1, docx.TableOptions{})
	add(firstCell(tbl).AddParagraph().AddRun(""))
	hl, err := d.Body().AddParagraph().AddHyperlink("https://example.com", docx.HyperlinkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	add(hl.AddRun(""))

	d2 := reopen(t, d)
	seen := map[string]bool{}
	for name, part := range zipEntries(t, d2) {
		if name != "word/document.xml" && !strings.HasPrefix(name, "word/header") {
			continue
		}
		for _, id := range extractDocPrIDs(string(part)) {
			if seen[id] {
				t.Errorf("docPr id %s used twice", id)
			}
			seen[id] = true
		}
	}
	if len(seen) != 4 {
		t.Errorf("found %d distinct docPr ids, want 4: %v", len(seen), seen)
	}
}

func extractDocPrIDs(xml string) []string {
	var ids []string
	for rest := xml; ; {
		i := strings.Index(rest, "<docPr")
		if i < 0 {
			return ids
		}
		rest = rest[i:]
		end := strings.Index(rest, ">")
		tag := rest[:end]
		if j := strings.Index(tag, ` id="`); j >= 0 {
			v := tag[j+5:]
			ids = append(ids, v[:strings.Index(v, `"`)])
		} else if j := strings.Index(tag, `:id="`); j >= 0 {
			v := tag[j+5:]
			ids = append(ids, v[:strings.Index(v, `"`)])
		}
		rest = rest[end:]
	}
}

func TestImage_InHeaderUsesHeaderRels(t *testing.T) {
	d := docx.New()
	data := pngBytes(t, 12, 12, 5)
	hdr := d.LastSection().SetHeader(docx.HeaderDefault)
	if _, err := hdr.AddParagraph().AddRun("").AddImage(bytes.NewReader(data), docx.ImageOptions{AltText: "logo"}); err != nil {
		t.Fatal(err)
	}
	entries := zipEntries(t, d)
	if !strings.Contains(string(entries["word/_rels/header1.xml.rels"]), "media/image1.png") {
		t.Errorf("header rels lack the image: %v", string(entries["word/_rels/header1.xml.rels"]))
	}
	if strings.Contains(string(entries["word/_rels/document.xml.rels"]), "media/") {
		t.Error("header image leaked into document rels")
	}

	d2 := reopen(t, d)
	hdr2 := d2.LastSection().Header(docx.HeaderDefault)
	var got []byte
	for p := range hdr2.Paragraphs() {
		for r := range p.Runs() {
			for img := range r.Images() {
				b, err := img.Bytes()
				if err != nil {
					t.Fatal(err)
				}
				got = b
			}
		}
	}
	if !bytes.Equal(got, data) {
		t.Error("header image bytes not preserved")
	}
}

func TestImage_JPEGAndGIF(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 30, 15))
	var jb, gb bytes.Buffer
	if err := jpeg.Encode(&jb, img, nil); err != nil {
		t.Fatal(err)
	}
	pal := image.NewPaletted(image.Rect(0, 0, 30, 15), color.Palette{color.Black, color.White})
	if err := gif.Encode(&gb, pal, nil); err != nil {
		t.Fatal(err)
	}
	d := docx.New()
	p := d.Body().AddParagraph()
	for _, data := range [][]byte{jb.Bytes(), gb.Bytes()} {
		im, err := p.AddRun("").AddImage(bytes.NewReader(data), docx.ImageOptions{AltText: "x"})
		if err != nil {
			t.Fatal(err)
		}
		if w, _ := im.Size(); w != 30*emuPx {
			t.Errorf("width = %d", w)
		}
	}
	entries := zipEntries(t, d)
	names := strings.Join(mediaNames(entries), ",")
	if !strings.Contains(names, ".jpeg") || !strings.Contains(names, ".gif") {
		t.Errorf("media names = %s", names)
	}
	ct := string(entries["[Content_Types].xml"])
	if !strings.Contains(ct, "image/jpeg") || !strings.Contains(ct, "image/gif") {
		t.Errorf("content types = %s", ct)
	}
}

func TestImage_Errors(t *testing.T) {
	d := docx.New()
	run := d.Body().AddParagraph().AddRun("")
	good := pngBytes(t, 4, 4, 1)

	cases := []struct {
		name string
		data []byte
		opts docx.ImageOptions
		want error
	}{
		{"svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"/>`), docx.ImageOptions{}, docx.ErrUnsupportedImage},
		{"svg with prolog", []byte("\xef\xbb\xbf<?xml version=\"1.0\"?>\n<svg/>"), docx.ImageOptions{}, docx.ErrUnsupportedImage},
		{"garbage", []byte("definitely not an image"), docx.ImageOptions{}, docx.ErrUnsupportedImage},
		{"empty", nil, docx.ImageOptions{}, docx.ErrUnsupportedImage},
		{"negative width", good, docx.ImageOptions{Width: -1}, docx.ErrInvalidImageSize},
		{"negative height", good, docx.ImageOptions{Height: -5}, docx.ErrInvalidImageSize},
	}
	for _, c := range cases {
		if _, err := run.AddImage(bytes.NewReader(c.data), c.opts); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
	// Failures must leave the document untouched: no media, no drawing.
	entries := zipEntries(t, d)
	if len(mediaNames(entries)) != 0 {
		t.Errorf("failed AddImage left media parts: %v", mediaNames(entries))
	}
	if strings.Contains(string(partXML(t, d, "word/document.xml")), "drawing") {
		t.Error("failed AddImage left a drawing in the document")
	}
}

func TestImage_FixtureExistingImagesSurviveAndIDsStayUnique(t *testing.T) {
	data, err := os.ReadFile("../testdata/fixtures/python-docx/having-images.docx")
	if err != nil {
		t.Fatal(err)
	}
	d, err := docx.OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	before := mediaNames(zipEntries(t, d))
	if len(before) == 0 {
		t.Fatal("fixture has no media parts")
	}

	// Existing pictures are readable through the proxy.
	var found int
	for p := range d.Body().Paragraphs() {
		for r := range p.Runs() {
			for img := range r.Images() {
				found++
				if w, h := img.Size(); w <= 0 || h <= 0 {
					t.Errorf("fixture image size = %d x %d", w, h)
				}
				if b, err := img.Bytes(); err != nil || len(b) == 0 {
					t.Errorf("fixture image bytes: %d, %v", len(b), err)
				}
			}
		}
	}
	if found == 0 {
		t.Fatal("no inline pictures found in fixture")
	}

	// Adding a new image allocates an unused docPr id and keeps the old parts.
	newData := pngBytes(t, 5, 5, 42)
	if _, err := d.Body().AddParagraph().AddRun("").AddImage(bytes.NewReader(newData), docx.ImageOptions{AltText: "new"}); err != nil {
		t.Fatal(err)
	}
	entries := zipEntries(t, reopen(t, d))
	after := mediaNames(entries)
	if len(after) != len(before)+1 {
		t.Errorf("media parts %d -> %d, want one more", len(before), len(after))
	}
	seen := map[string]bool{}
	for _, id := range extractDocPrIDs(string(entries["word/document.xml"])) {
		if seen[id] {
			t.Errorf("docPr id %s duplicated after AddImage", id)
		}
		seen[id] = true
	}
}
