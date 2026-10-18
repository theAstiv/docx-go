package template

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// storyPart matches the parts whose content the engine normalizes.
var storyPart = regexp.MustCompile(`^word/(document|header\d+|footer\d+)\.xml$`)

// TestNormalize_CorpusKeepsText normalizes every paragraph of every story in
// the fixture corpus and checks that no paragraph's text changes and the
// result still marshals. Real documents are where unexpected run content
// shows up first.
func TestNormalize_CorpusKeepsText(t *testing.T) {
	files, err := filepath.Glob("../../testdata/fixtures/*/*.docx")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no fixtures found")
	}
	paragraphs := 0
	for _, f := range files {
		if strings.HasPrefix(filepath.Base(f), "~$") {
			continue // Word lock file
		}
		for name, data := range storyParts(t, f) {
			blocks := parseStory(t, name, data)
			var before []string
			eachParagraph(blocks, func(p *wml.CT_P) { before = append(before, p.Text()) })
			normalizeBlocks(blocks, Location{Part: name})
			i := 0
			eachParagraph(blocks, func(p *wml.CT_P) {
				if after := p.Text(); after != before[i] {
					t.Errorf("%s %s paragraph %d: text changed\n before %q\n after  %q", f, name, i, before[i], after)
				}
				i++
			})
			paragraphs += i
			for _, b := range blocks {
				if _, err := xml.Marshal(b); err != nil {
					t.Errorf("%s %s: marshal after normalize: %v", f, name, err)
				}
			}
		}
	}
	t.Logf("normalized %d paragraphs in %d fixtures", paragraphs, len(files))
}

func storyParts(t *testing.T, path string) map[string][]byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	out := map[string][]byte{}
	for _, zf := range zr.File {
		if !storyPart.MatchString(zf.Name) {
			continue
		}
		rc, err := zf.Open()
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
		out[zf.Name] = b
	}
	return out
}

func parseStory(t *testing.T, name string, data []byte) []wml.BlockElement {
	t.Helper()
	switch {
	case strings.Contains(name, "header"):
		var h wml.CT_Hdr
		if err := xml.Unmarshal(data, &h); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return h.Children
	case strings.Contains(name, "footer"):
		var f wml.CT_Ftr
		if err := xml.Unmarshal(data, &f); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return f.Children
	}
	var d wml.CT_Document
	if err := xml.Unmarshal(data, &d); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if d.Body == nil {
		return nil
	}
	return d.Body.Children
}

// eachParagraph visits every typed paragraph in blocks, including table cells.
func eachParagraph(blocks []wml.BlockElement, fn func(*wml.CT_P)) {
	for _, b := range blocks {
		switch el := b.(type) {
		case *wml.CT_P:
			fn(el)
		case *wml.CT_Tbl:
			for _, r := range el.Children {
				if row, ok := r.(*wml.CT_Tr); ok {
					for _, c := range row.Children {
						if cell, ok := c.(*wml.CT_Tc); ok {
							eachParagraph(cell.Children, fn)
						}
					}
				}
			}
		}
	}
}
