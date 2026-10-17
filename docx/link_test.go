package docx_test

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

func TestBookmark_AddAndRoundTrip(t *testing.T) {
	d := docx.New()
	p := d.Body().AddParagraph()
	p.AddRun("Intro")
	bm, err := p.AddBookmark("intro")
	if err != nil {
		t.Fatalf("AddBookmark: %v", err)
	}
	if bm.Name() != "intro" {
		t.Errorf("Name = %q", bm.Name())
	}

	d2 := reopen(t, d)
	if d2.Bookmark("intro") == nil {
		t.Fatal("bookmark lost on round-trip")
	}
	if d2.Bookmark("missing") != nil {
		t.Error("Bookmark(missing) should be nil")
	}
	// Text is untouched by the wrapping markers.
	for para := range d2.Body().Paragraphs() {
		if para.Text() != "Intro" {
			t.Errorf("paragraph text = %q", para.Text())
		}
	}
}

func TestBookmark_NameRules(t *testing.T) {
	d := docx.New()
	p := d.Body().AddParagraph()
	long := "a" + strings.Repeat("b", 40) // 41 characters

	cases := []struct {
		name string
		want error
	}{
		{"", docx.ErrInvalidBookmarkName},
		{"1abc", docx.ErrInvalidBookmarkName},
		{"has space", docx.ErrInvalidBookmarkName},
		{"dash-ed", docx.ErrInvalidBookmarkName},
		{long, docx.ErrInvalidBookmarkName},
		{"_Toc123", docx.ErrReservedBookmarkName},
	}
	for _, c := range cases {
		if _, err := p.AddBookmark(c.name); !errors.Is(err, c.want) {
			t.Errorf("AddBookmark(%q) err = %v, want %v", c.name, err, c.want)
		}
	}
	// Valid edge cases: exactly 40 characters, digits and underscores after the first letter.
	for _, ok := range []string{"a" + strings.Repeat("b", 39), "sec_1_2", "Überblick"} {
		q := d.Body().AddParagraph()
		if _, err := q.AddBookmark(ok); err != nil {
			t.Errorf("AddBookmark(%q) unexpected error: %v", ok, err)
		}
	}
	// A failed call must not have modified the paragraph.
	if n := countBookmarks(d); n != 3 {
		t.Errorf("document has %d bookmarks, want 3", n)
	}
}

func countBookmarks(d *docx.Document) int {
	n := 0
	for range d.Bookmarks() {
		n++
	}
	return n
}

func TestBookmark_DuplicateAcrossStories(t *testing.T) {
	d := docx.New()
	hdr := d.LastSection().SetHeader(docx.HeaderDefault)
	hp := hdr.AddParagraph()
	if _, err := hp.AddBookmark("shared"); err != nil {
		t.Fatal(err)
	}
	bp := d.Body().AddParagraph()
	if _, err := bp.AddBookmark("shared"); !errors.Is(err, docx.ErrDuplicateBookmark) {
		t.Errorf("duplicate across header/body: err = %v", err)
	}
	if _, err := bp.AddBookmark("other"); err != nil {
		t.Fatal(err)
	}
	if countBookmarks(d) != 2 {
		t.Errorf("want 2 bookmarks, got %d", countBookmarks(d))
	}
}

func TestBookmark_IDsUniqueAcrossStories(t *testing.T) {
	d := docx.New()
	ftr := d.LastSection().SetFooter(docx.FooterDefault)
	if _, err := ftr.AddParagraph().AddBookmark("f1"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Body().AddParagraph().AddBookmark("b1"); err != nil {
		t.Fatal(err)
	}
	tbl := d.Body().AddTable(1, 1, docx.TableOptions{})
	var cell *docx.Cell
	for r := range tbl.Rows() {
		for c := range r.Cells() {
			cell = c
		}
	}
	if _, err := cell.AddParagraph().AddBookmark("c1"); err != nil {
		t.Fatal(err)
	}

	d2 := reopen(t, d)
	seen := map[string]bool{}
	for _, name := range []string{"f1", "b1", "c1"} {
		if d2.Bookmark(name) == nil {
			t.Fatalf("bookmark %s missing after round-trip", name)
		}
		seen[name] = true
	}
	// Distinct ids appear in the XML.
	body := string(partXML(t, d2, "word/document.xml"))
	if !strings.Contains(body, `:id="1"`) || !strings.Contains(body, `:id="2"`) {
		t.Errorf("expected ids 1 and 2 in body: %s", body)
	}
	if strings.Contains(body, "_xmlns") {
		t.Errorf("re-saved bookmarks carry stray namespace declarations: %s", body)
	}
}

func TestBookmark_FixtureExistingAndNewID(t *testing.T) {
	data, err := os.ReadFile("../testdata/fixtures/poi/bookmarks.docx")
	if err != nil {
		t.Fatal(err)
	}
	d, err := docx.OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if d.Bookmark("poi") == nil || d.Bookmark("xwpf") == nil {
		t.Fatal("fixture bookmarks poi/xwpf not found")
	}
	before := countBookmarks(d)
	if _, err := d.Body().AddParagraph().AddBookmark("added"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Body().AddParagraph().AddBookmark("poi"); !errors.Is(err, docx.ErrDuplicateBookmark) {
		t.Errorf("dup of fixture bookmark: %v", err)
	}
	d2 := reopen(t, d)
	if got := countBookmarks(d2); got != before+1 {
		t.Errorf("bookmarks = %d, want %d", got, before+1)
	}
}

func TestHyperlink_ExternalRoundTrip(t *testing.T) {
	d := docx.New()
	p := d.Body().AddParagraph()
	p.AddRun("See ")
	hl, err := p.AddHyperlink("https://example.com/a?b=1&c=2", docx.HyperlinkOptions{
		Tooltip: "Example", History: docx.Off(),
	})
	if err != nil {
		t.Fatalf("AddHyperlink: %v", err)
	}
	hl.AddRun("the site")

	if d.Styles().ByID("Hyperlink") == nil {
		t.Error("Hyperlink style should be ensured")
	}

	d2 := reopen(t, d)
	var found *docx.Hyperlink
	for para := range d2.Body().Paragraphs() {
		for h := range para.Hyperlinks() {
			found = h
		}
		if para.Text() != "See the site" {
			t.Errorf("paragraph text = %q", para.Text())
		}
	}
	if found == nil {
		t.Fatal("hyperlink lost on round-trip")
	}
	if found.URL() != "https://example.com/a?b=1&c=2" {
		t.Errorf("URL = %q (must be stored exactly as given)", found.URL())
	}
	if found.Text() != "the site" || found.Tooltip() != "Example" || found.Anchor() != "" {
		t.Errorf("text/tooltip/anchor = %q/%q/%q", found.Text(), found.Tooltip(), found.Anchor())
	}
	for r := range found.Runs() {
		if r.Font().Style() != "Hyperlink" {
			t.Errorf("run style = %q, want Hyperlink", r.Font().Style())
		}
	}

	rels := string(partXML(t, d2, "word/_rels/document.xml.rels"))
	if !strings.Contains(rels, `TargetMode="External"`) {
		t.Errorf("rels lack External hyperlink: %s", rels)
	}
	doc := string(partXML(t, d2, "word/document.xml"))
	if !strings.Contains(doc, `:history="0"`) {
		t.Errorf("history attribute missing: %s", doc)
	}
}

func TestHyperlink_SharedRelationshipForSameURL(t *testing.T) {
	d := docx.New()
	p := d.Body().AddParagraph()
	for range 3 {
		hl, err := p.AddHyperlink("https://example.com", docx.HyperlinkOptions{})
		if err != nil {
			t.Fatal(err)
		}
		hl.AddRun("x")
	}
	rels := string(partXML(t, d, "word/_rels/document.xml.rels"))
	if n := strings.Count(rels, `TargetMode="External"`); n != 1 {
		t.Errorf("%d external relationships, want 1: %s", n, rels)
	}
}

func TestHyperlink_URLValidation(t *testing.T) {
	d := docx.New()
	p := d.Body().AddParagraph()
	for _, bad := range []string{"", "example.com", "/relative/path", "javascript:alert(1)", "JavaScript:alert(1)", "data:text/html,x", "vbscript:x", "http://a b"} {
		if _, err := p.AddHyperlink(bad, docx.HyperlinkOptions{}); !errors.Is(err, docx.ErrInvalidURL) {
			t.Errorf("AddHyperlink(%q) err = %v, want ErrInvalidURL", bad, err)
		}
	}
	for _, good := range []string{"https://example.com", "http://x.y/z#frag", "mailto:a@b.co", "ftp://host/file"} {
		if _, err := p.AddHyperlink(good, docx.HyperlinkOptions{}); err != nil {
			t.Errorf("AddHyperlink(%q) unexpected error: %v", good, err)
		}
	}
	var n int
	for range p.Hyperlinks() {
		n++
	}
	if n != 4 {
		t.Errorf("%d hyperlinks present, want 4 (rejected ones must not be added)", n)
	}
}

func TestHyperlink_InternalLink(t *testing.T) {
	d := docx.New()
	target := d.Body().AddParagraph()
	target.AddRun("Target")
	if _, err := target.AddBookmark("target"); err != nil {
		t.Fatal(err)
	}
	p := d.Body().AddParagraph()
	hl, err := p.AddInternalLink("target", docx.HyperlinkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	hl.AddRun("go to target")
	if _, err := p.AddInternalLink("", docx.HyperlinkOptions{}); !errors.Is(err, docx.ErrEmptyAnchor) {
		t.Errorf("empty anchor err = %v", err)
	}

	d2 := reopen(t, d)
	found := false
	for para := range d2.Body().Paragraphs() {
		for h := range para.Hyperlinks() {
			found = true
			if h.Anchor() != "target" || h.URL() != "" || h.Text() != "go to target" {
				t.Errorf("anchor/url/text = %q/%q/%q", h.Anchor(), h.URL(), h.Text())
			}
		}
	}
	if !found {
		t.Fatal("internal link lost")
	}
	rels := string(partXML(t, d2, "word/_rels/document.xml.rels"))
	if strings.Contains(rels, "hyperlink") {
		t.Errorf("internal link must not create a relationship: %s", rels)
	}
}

func TestHyperlink_InHeaderUsesHeaderRels(t *testing.T) {
	d := docx.New()
	hdr := d.LastSection().SetHeader(docx.HeaderDefault)
	p := hdr.AddParagraph()
	hl, err := p.AddHyperlink("https://example.com/h", docx.HyperlinkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	hl.AddRun("header link")

	d2 := reopen(t, d)
	hdr2 := d2.LastSection().Header(docx.HeaderDefault)
	if hdr2 == nil {
		t.Fatal("header lost")
	}
	var url string
	for para := range hdr2.Paragraphs() {
		for h := range para.Hyperlinks() {
			url = h.URL()
		}
	}
	if url != "https://example.com/h" {
		t.Errorf("header link URL = %q", url)
	}
	hrels := string(partXML(t, d2, "word/_rels/header1.xml.rels"))
	if !strings.Contains(hrels, "https://example.com/h") {
		t.Errorf("header rels missing link: %s", hrels)
	}
	drels := string(partXML(t, d2, "word/_rels/document.xml.rels"))
	if strings.Contains(drels, "example.com/h") {
		t.Errorf("header link leaked into document rels: %s", drels)
	}
}

func TestHyperlink_FixtureLinksResolve(t *testing.T) {
	// A document opened from disk must resolve existing links and keep them.
	var buf bytes.Buffer
	d := docx.New()
	p := d.Body().AddParagraph()
	hl, _ := p.AddHyperlink("https://example.com/keep", docx.HyperlinkOptions{})
	hl.AddRun("keep")
	if err := d.Save(&buf); err != nil {
		t.Fatal(err)
	}
	d2 := reopen(t, mustOpen(t, buf.Bytes()))
	n := 0
	for para := range d2.Body().Paragraphs() {
		for h := range para.Hyperlinks() {
			n++
			if h.URL() != "https://example.com/keep" {
				t.Errorf("URL after second round-trip = %q", h.URL())
			}
		}
	}
	if n != 1 {
		t.Errorf("%d links, want 1", n)
	}
}

func mustOpen(t *testing.T, data []byte) *docx.Document {
	t.Helper()
	d, err := docx.OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	return d
}
