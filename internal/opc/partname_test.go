package opc

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"
)

const (
	ctXML   = `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="application/xml"/></Types>`
	rootRel = `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"/>`
)

// zipList builds a ZIP from name/data pairs in order, so duplicates are kept.
func zipList(t *testing.T, pairs ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i := 0; i+1 < len(pairs); i += 2 {
		w, err := zw.Create(pairs[i])
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(pairs[i+1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestOpen_RefusesInvalidPartNames(t *testing.T) {
	for _, name := range []string{
		"/word/document.xml", `word\document.xml`, "word/../x.xml", "word/./x.xml",
		"word//x.xml", "word/x.", "../evil.xml",
	} {
		data := zipList(t, contentTypesName, ctXML, "_rels/.rels", rootRel, name, "<x/>")
		if _, err := OpenBytes(data); !errors.Is(err, ErrPartName) {
			t.Errorf("%q: err = %v, want ErrPartName", name, err)
		}
	}
}

func TestOpen_RefusesDuplicateParts(t *testing.T) {
	for _, pair := range [][2]string{
		{"word/a.xml", "word/a.xml"},
		{"word/a.xml", "Word/A.XML"},
	} {
		data := zipList(t, contentTypesName, ctXML, "_rels/.rels", rootRel, pair[0], "<a/>", pair[1], "<b/>")
		if _, err := OpenBytes(data); !errors.Is(err, ErrPartName) {
			t.Errorf("%v: err = %v, want ErrPartName", pair, err)
		}
	}
}

// Folder entries are not parts and are skipped.
func TestOpen_SkipsFolderEntries(t *testing.T) {
	data := zipList(t, "word/", "", contentTypesName, ctXML, "_rels/.rels", rootRel, "word/a.xml", "<a/>")
	pkg, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := pkg.PartPaths(); len(got) != 1 || got[0] != "word/a.xml" {
		t.Errorf("parts = %v", got)
	}
}

func TestRelsFileNames(t *testing.T) {
	cases := []struct {
		name   string
		isRels bool
		source string
	}{
		{"_rels/.rels", true, ""},
		{"_rels/x.xml.rels", true, "x.xml"},
		{"word/_rels/document.xml.rels", true, "word/document.xml"},
		{"customXml/_rels/item1.xml.rels", true, "customXml/item1.xml"},
		{"word/foo.rels", false, ""},
		{"foo.rels", false, ""},
		{"word/rels/x.rels", false, ""},
	}
	for _, tc := range cases {
		if got := isRelsFile(tc.name); got != tc.isRels {
			t.Errorf("isRelsFile(%q) = %v", tc.name, got)
			continue
		}
		if tc.isRels {
			if got := relsPathToPartPath(tc.name); got != tc.source {
				t.Errorf("relsPathToPartPath(%q) = %q, want %q", tc.name, got, tc.source)
			}
			if got := relsPath(tc.source); got != tc.name {
				t.Errorf("relsPath(%q) = %q, want %q", tc.source, got, tc.name)
			}
		}
	}
}

// A .rels file outside a _rels folder is an ordinary part, kept as it was.
// A malformed relationships part of a part nothing reads does not fail
// Open; it is reported by RelsErr and written back unchanged.
func TestOpen_RelsEdgeCases(t *testing.T) {
	const bad = `not xml`
	data := zipList(t, contentTypesName, ctXML, "_rels/.rels", rootRel,
		"word/foo.rels", "opaque", "word/x.xml", "<x/>", "word/_rels/x.xml.rels", bad)
	pkg, err := OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if string(pkg.Part("word/foo.rels")) != "opaque" {
		t.Error("word/foo.rels was not kept as a part")
	}
	if pkg.RelsErr("word/x.xml") == nil {
		t.Error("RelsErr(word/x.xml) = nil for a malformed relationships part")
	}
	var buf bytes.Buffer
	if err := pkg.Save(&buf); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, f := range zr.File {
		found[f.Name] = true
	}
	for _, want := range []string{"word/foo.rels", "word/_rels/x.xml.rels"} {
		if !found[want] {
			t.Errorf("%s missing after save; have %v", want, found)
		}
	}
	// Writing relationships for that part replaces the malformed set.
	pkg.Rels("word/x.xml").Add(RelTypeImage, "media/a.png", "")
	buf.Reset()
	if err := pkg.Save(&buf); err != nil {
		t.Fatal(err)
	}
	if pkg.RelsErr("word/x.xml") != nil || strings.Contains(string(pkg.Part("word/_rels/x.xml.rels")), bad) {
		t.Error("RelsErr still set after the set was replaced")
	}
}

func TestContentTypes_CaseInsensitive(t *testing.T) {
	ct := &ContentTypes{}
	ct.SetDefault("png", "image/png")
	ct.SetDefault("PNG", "image/png")
	if len(ct.Defaults) != 1 {
		t.Errorf("Defaults = %v, want one entry", ct.Defaults)
	}
	ct.SetOverride("/word/document.xml", "a")
	ct.SetOverride("/Word/Document.xml", "b")
	if len(ct.Overrides) != 1 || ct.ContentTypeFor("/WORD/document.XML") != "b" {
		t.Errorf("Overrides = %v", ct.Overrides)
	}
	if got := ct.ContentTypeFor("/word/media/image1.PNG"); got != "image/png" {
		t.Errorf("ContentTypeFor(.PNG) = %q", got)
	}
}

// A huge numeric rId neither overflows the counter nor causes a duplicate.
func TestRels_HugeRID(t *testing.T) {
	r := &Rels{}
	r.AddWithID("rId9223372036854775807", RelTypeImage, "a.png", "")
	r.AddWithID("rId0", RelTypeImage, "b.png", "")
	seen := map[string]bool{"rId9223372036854775807": true, "rId0": true}
	for range 3 {
		id := r.Add(RelTypeImage, "c.png", "")
		if seen[id] || strings.Contains(id, "-") {
			t.Fatalf("Add returned %q", id)
		}
		seen[id] = true
	}
}
