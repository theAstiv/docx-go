package docx_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

// phase3aFixtures are the real-world documents that exercise Phase 3a
// features: inline and anchored images (body and header), hyperlinks, and
// shaded or bordered tables.
var phase3aFixtures = []string{
	"python-docx/having-images.docx",
	"poi/VariousPictures.docx",
	"poi/61745.docx",
	"poi/56392.docx",
	"poi/headerPic.docx",
	"poi/WithGIF.docx",
	"poi/61991.docx",
	"poi/TestDocument.docx",
	"poi/bug57031.docx",
	"poi/drawing.docx",
	"poi/IllustrativeCases.docx",
	"poi/bookmarks.docx",
	"poi/table-indent.docx",
	"poi/table-alignment.docx",
}

func readZip(t *testing.T, data []byte) map[string][]byte {
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

// Element and attribute matchers that ignore namespace prefixes, because the
// source documents use w:/wp:/a: while docx-go writes default namespaces.
var (
	reDrawing   = regexp.MustCompile(`<(?:\w+:)?drawing[\s/>]`)
	reInline    = regexp.MustCompile(`<(?:\w+:)?inline[\s/>]`)
	reAnchor    = regexp.MustCompile(`<(?:\w+:)?anchor[\s/>]`)
	reHyperlink = regexp.MustCompile(`<(?:\w+:)?hyperlink[\s/>]`)
	reBookmark  = regexp.MustCompile(`<(?:\w+:)?bookmarkStart[\s/>]`)
	reTable     = regexp.MustCompile(`<(?:\w+:)?tbl[\s/>]`)
	reShading   = regexp.MustCompile(`<(?:\w+:)?shd[\s/>]`)
	reDocPrID   = regexp.MustCompile(`<(?:\w+:)?docPr\s[^>]*?\bid="(\d+)"`)
	reEmbed     = regexp.MustCompile(`<(?:\w+:)?blip\s[^>]*?\b(?:\w+:)?embed="([^"]+)"`)
	reBMName    = regexp.MustCompile(`<(?:\w+:)?bookmarkStart\s[^>]*?\b(?:\w+:)?name="([^"]*)"`)
)

func count(re *regexp.Regexp, data []byte) int { return len(re.FindAll(data, -1)) }

func submatches(re *regexp.Regexp, data []byte) []string {
	var out []string
	for _, m := range re.FindAllSubmatch(data, -1) {
		out = append(out, string(m[1]))
	}
	sort.Strings(out)
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// relTargets returns the sorted "type|target|mode" triples of a .rels part,
// ignoring relationship ids (which may be renumbered).
func relTargets(data []byte) []string {
	re := regexp.MustCompile(`<Relationship\s[^>]*>`)
	attr := func(tag, name string) string {
		m := regexp.MustCompile(`\b` + name + `="([^"]*)"`).FindStringSubmatch(tag)
		if m == nil {
			return ""
		}
		return m[1]
	}
	var out []string
	for _, tag := range re.FindAllString(string(data), -1) {
		out = append(out, attr(tag, "Type")+"|"+attr(tag, "Target")+"|"+attr(tag, "TargetMode"))
	}
	sort.Strings(out)
	return out
}

func isStoryPart(name string) bool {
	return name == "word/document.xml" ||
		(strings.HasPrefix(name, "word/header") && strings.HasSuffix(name, ".xml")) ||
		(strings.HasPrefix(name, "word/footer") && strings.HasSuffix(name, ".xml"))
}

// TestExitGate_ImageRoundTripKeepsBytesAndAspectRatio is the Phase 3a image
// criterion: the stored file is byte-identical after a round-trip and the
// displayed size keeps the pixel aspect ratio exactly.
func TestExitGate_ImageRoundTripKeepsBytesAndAspectRatio(t *testing.T) {
	const pxW, pxH = 300, 200
	data := pngBytes(t, pxW, pxH, 33)

	d := docx.New()
	run := d.Body().AddParagraph().AddRun("")
	if _, err := run.AddImage(bytes.NewReader(data), docx.ImageOptions{
		Width: docx.Inches(3).EMU(), AltText: "ratio check",
	}); err != nil {
		t.Fatal(err)
	}

	img := firstImage(reopen(t, reopen(t, d))) // two save/reopen cycles
	if img == nil {
		t.Fatal("image lost")
	}
	got, err := img.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Error("image bytes changed across two round-trips")
	}
	w, h := img.Size()
	if int64(w)*pxH != int64(h)*pxW {
		t.Errorf("aspect ratio drifted: %d x %d EMU is not %d:%d", w, h, pxW, pxH)
	}
	if w != docx.Inches(3).EMU() || h != docx.Inches(2).EMU() {
		t.Errorf("size = %d x %d, want 3in x 2in", w, h)
	}
}

// TestIDs_RespectTextBoxContentInsideDrawings covers ids that sit inside a
// drawing's own content: a text box (wps:txbx) in an anchored drawing and one
// in the graphic data of a typed inline drawing, each holding a bookmark and a
// nested picture. Scanning only a drawing's own docPr would miss all of them.
func TestIDs_RespectTextBoxContentInsideDrawings(t *testing.T) {
	const decls = ` xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"` +
		` xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"` +
		` xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"` +
		` xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape"`
	box := func(bookmark string, bmID int, picID int) string {
		return `<wps:wsp><wps:txbx><w:txbxContent><w:p><w:bookmarkStart w:id="` + itoa(bmID) + `" w:name="` + bookmark + `"/>` +
			`<w:r><w:drawing><wp:inline><wp:extent cx="1" cy="1"/><wp:docPr id="` + itoa(picID) + `" name="Nested"/></wp:inline></w:drawing></w:r>` +
			`</w:p></w:txbxContent></wps:txbx></wps:wsp>`
	}
	anchored := `<w:p` + decls + `><w:r><w:drawing><wp:anchor><wp:docPr id="20" name="Box"/>` +
		`<a:graphic><a:graphicData uri="x">` + box("InAnchorBox", 3, 21) + `</a:graphicData></a:graphic></wp:anchor></w:drawing></w:r></w:p>`
	inline := `<w:p` + decls + `><w:r><w:drawing><wp:inline><wp:extent cx="1" cy="1"/><wp:docPr id="30" name="Box2"/>` +
		`<a:graphic><a:graphicData uri="x">` + box("InInlineBox", 4, 31) + `</a:graphicData></a:graphic></wp:inline></w:drawing></w:r></w:p>`

	d := injectIntoBody(t, docx.New(), anchored+inline)

	for _, name := range []string{"InAnchorBox", "InInlineBox"} {
		if d.Bookmark(name) == nil {
			t.Errorf("bookmark %s inside a drawing text box is not visible", name)
		}
	}
	p := d.Body().AddParagraph()
	for _, name := range []string{"InAnchorBox", "InInlineBox"} {
		if _, err := p.AddBookmark(name); !errors.Is(err, docx.ErrDuplicateBookmark) {
			t.Errorf("duplicate of %s: err = %v, want ErrDuplicateBookmark", name, err)
		}
	}
	if _, err := p.AddRun("").AddImage(bytes.NewReader(pngBytes(t, 4, 4, 1)), docx.ImageOptions{AltText: "n"}); err != nil {
		t.Fatal(err)
	}
	ids := extractDocPrIDs(string(partXML(t, d, "word/document.xml")))
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Errorf("docPr id %s used twice: %v", id, ids)
		}
		seen[id] = true
	}
	if len(ids) != 5 {
		t.Errorf("docPr ids = %v, want 5 distinct (20, 21, 30, 31 and a new one)", ids)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

// TestPhase3a_Fixtures_CoverTheFeatures guards the structural test against
// passing vacuously: the corpus must really contain inline and anchored
// drawings, a header image, hyperlinks, bookmarks, shading and media files.
func TestPhase3a_Fixtures_CoverTheFeatures(t *testing.T) {
	var drawings, inlines, anchors, links, marks, shades, media, headerDrawings int
	for _, rel := range phase3aFixtures {
		orig, err := os.ReadFile(filepath.Join("..", "testdata", "fixtures", filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		for name, b := range readZip(t, orig) {
			switch {
			case strings.HasPrefix(name, "word/media/"):
				media++
			case isStoryPart(name):
				drawings += count(reDrawing, b)
				inlines += count(reInline, b)
				anchors += count(reAnchor, b)
				links += count(reHyperlink, b)
				marks += count(reBookmark, b)
				shades += count(reShading, b)
				if strings.HasPrefix(name, "word/header") {
					headerDrawings += count(reDrawing, b)
				}
			}
		}
	}
	for label, n := range map[string]int{
		"drawings": drawings, "inline pictures": inlines, "anchored pictures": anchors,
		"hyperlinks": links, "bookmarks": marks, "shading": shades, "media files": media,
		"drawings in headers": headerDrawings,
	} {
		if n == 0 {
			t.Errorf("fixture corpus has no %s; the structural test would pass vacuously", label)
		}
	}
}

// TestPhase3a_Fixtures_StructuralRoundTrip opens each fixture, saves it
// unchanged, and checks what Phase 3a is responsible for keeping: media files
// byte-equal, the same drawings (inline and anchored) with the same ids and
// image references, the same hyperlinks, bookmarks and tables, and the same
// relationship targets. Paragraph-text equality alone would miss a dropped
// image or link.
func TestPhase3a_Fixtures_StructuralRoundTrip(t *testing.T) {
	for _, rel := range phase3aFixtures {
		t.Run(rel, func(t *testing.T) {
			t.Parallel()
			orig, err := os.ReadFile(filepath.Join("..", "testdata", "fixtures", filepath.FromSlash(rel)))
			if err != nil {
				t.Fatal(err)
			}
			d, err := docx.OpenBytes(orig)
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if err := d.Save(&buf); err != nil {
				t.Fatal(err)
			}
			before, after := readZip(t, orig), readZip(t, buf.Bytes())

			for name, b := range before {
				got, ok := after[name]
				if !ok {
					t.Errorf("part %s missing after save", name)
					continue
				}
				switch {
				case strings.HasPrefix(name, "word/media/"):
					if !bytes.Equal(b, got) {
						t.Errorf("media %s changed", name)
					}
				case isStoryPart(name):
					for label, re := range map[string]*regexp.Regexp{
						"drawing": reDrawing, "inline": reInline, "anchor": reAnchor,
						"hyperlink": reHyperlink, "bookmark": reBookmark,
						"table": reTable, "shading": reShading,
					} {
						if want, have := count(re, b), count(re, got); want != have {
							t.Errorf("%s: %d <%s> before, %d after", name, want, label, have)
						}
					}
					if want, have := submatches(reDocPrID, b), submatches(reDocPrID, got); !equalStrings(want, have) {
						t.Errorf("%s: docPr ids %v -> %v", name, want, have)
					}
					if want, have := submatches(reEmbed, b), submatches(reEmbed, got); !equalStrings(want, have) {
						t.Errorf("%s: blip embed ids %v -> %v", name, want, have)
					}
					if want, have := submatches(reBMName, b), submatches(reBMName, got); !equalStrings(want, have) {
						t.Errorf("%s: bookmark names %v -> %v", name, want, have)
					}
				case strings.HasSuffix(name, ".rels") && strings.HasPrefix(name, "word/_rels/"):
					if want, have := relTargets(b), relTargets(got); !equalStrings(want, have) {
						t.Errorf("%s: relationships changed\n  before %v\n  after  %v", name, want, have)
					}
				}
			}
		})
	}
}

// injectIntoBody saves d, inserts xmlFragment before the body-level sectPr of
// word/document.xml, and reopens the result. It lets tests plant content the
// package keeps raw (content controls, tracked changes, smart tags).
func injectIntoBody(t *testing.T, d *docx.Document, xmlFragment string) *docx.Document {
	t.Helper()
	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatal(err)
	}
	entries := readZip(t, buf.Bytes())
	doc := string(entries["word/document.xml"])
	i := strings.LastIndex(doc, "<sectPr")
	if i < 0 {
		t.Fatalf("no body sectPr in %s", doc)
	}
	entries["word/document.xml"] = []byte(doc[:i] + xmlFragment + doc[i:])

	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for name, data := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	r, err := docx.OpenBytes(out.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

const (
	nsDecl = ` xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"` +
		` xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"`
	// A cover page in a content control: a drawing with docPr id 1 and a
	// bookmark "Cover" with id 0 that live only inside raw w:sdt content.
	coverSDT = `<w:sdt` + nsDecl + `><w:sdtPr><w:alias w:val="Cover"/></w:sdtPr><w:sdtContent>` +
		`<w:p><w:bookmarkStart w:id="0" w:name="Cover"/><w:r><w:drawing><wp:inline>` +
		`<wp:extent cx="1" cy="1"/><wp:docPr id="1" name="Logo"/></wp:inline></w:drawing></w:r>` +
		`<w:bookmarkEnd w:id="0"/></w:p></w:sdtContent></w:sdt>`
	// Raw inline content in an ordinary paragraph: a smart tag, and a tracked
	// insertion, each hiding a bookmark and a drawing id.
	inlineRaw = `<w:p` + nsDecl + `><w:smartTag w:uri="urn:x" w:element="x"><w:bookmarkStart w:id="5" w:name="InSmartTag"/>` +
		`<w:r><w:t>x</w:t></w:r></w:smartTag>` +
		`<w:ins w:id="9" w:author="a"><w:r><w:drawing><wp:inline><wp:extent cx="1" cy="1"/>` +
		`<wp:docPr id="7" name="Tracked"/></wp:inline></w:drawing></w:r></w:ins></w:p>`
)

// TestIDs_RespectContentInsideRawContainers is the regression test for ids and
// names that live inside content controls, smart tags and tracked changes.
// Allocating over them produced duplicate drawing ids (a known cause of Word's
// "unreadable content" warning) and let a duplicate bookmark name through.
func TestIDs_RespectContentInsideRawContainers(t *testing.T) {
	d := injectIntoBody(t, docx.New(), coverSDT+inlineRaw)

	// Existing raw bookmarks are visible.
	if d.Bookmark("Cover") == nil || d.Bookmark("InSmartTag") == nil {
		t.Fatal("bookmarks inside raw content are not visible through the Document API")
	}

	p := d.Body().AddParagraph()
	if _, err := p.AddBookmark("Cover"); !errors.Is(err, docx.ErrDuplicateBookmark) {
		t.Errorf("duplicate of a bookmark inside an sdt: err = %v, want ErrDuplicateBookmark", err)
	}
	if _, err := p.AddBookmark("InSmartTag"); !errors.Is(err, docx.ErrDuplicateBookmark) {
		t.Errorf("duplicate of a bookmark inside a smartTag: err = %v", err)
	}
	if _, err := p.AddBookmark("Fresh"); err != nil {
		t.Fatal(err)
	}

	img, err := p.AddRun("").AddImage(bytes.NewReader(pngBytes(t, 4, 4, 1)), docx.ImageOptions{AltText: "new"})
	if err != nil {
		t.Fatal(err)
	}
	_ = img

	xml := string(partXML(t, d, "word/document.xml"))
	ids := extractDocPrIDs(xml)
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Errorf("docPr id %s is used twice: %v", id, ids)
		}
		seen[id] = true
	}
	if len(ids) != 3 {
		t.Errorf("docPr ids = %v, want 3 distinct (1, 7 and a new one)", ids)
	}

	// The new bookmark must not reuse id 0 or 5 either.
	marks := regexp.MustCompile(`<(?:\w+:)?bookmarkStart\s[^>]*?\bid="(\d+)"`)
	bm := map[string]bool{}
	for _, m := range marks.FindAllStringSubmatch(xml, -1) {
		if bm[m[1]] {
			t.Errorf("bookmark id %s used twice in %s", m[1], xml)
		}
		bm[m[1]] = true
	}
	if len(bm) != 3 {
		t.Errorf("bookmark ids = %v, want 3 distinct", bm)
	}
}
