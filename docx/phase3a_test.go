package docx_test

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
