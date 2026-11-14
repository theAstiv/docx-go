package docx_test

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/theAstiv/docx-go/docx"
)

// zipEntry returns the bytes of one entry of a saved .docx, or nil.
func zipEntry(t *testing.T, data []byte, name string) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(rc)
		if cerr := rc.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	return nil
}

func save(t *testing.T, d *docx.Document) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestCoreProperties_NewDocument(t *testing.T) {
	d := docx.New()
	cp := d.CoreProperties()
	if cp.Title() != "" || cp.Author() != "" || cp.Revision() != 0 {
		t.Fatal("new document reports properties")
	}
	if _, ok := cp.Created(); ok {
		t.Fatal("new document reports a creation time")
	}
	// Clearing a property that is not set must not create the part.
	cp.SetTitle("")
	cp.SetCreated(time.Time{})
	if zipEntry(t, save(t, d), "docProps/core.xml") != nil {
		t.Fatal("clearing an unset property created docProps/core.xml")
	}

	ist := time.FixedZone("IST", 5*3600+1800)
	cp.SetTitle("Gap assessment")
	cp.SetSubject("DPDPA")
	cp.SetAuthor("A. Author")
	cp.SetKeywords("privacy; dpia")
	cp.SetDescription("Draft for review")
	cp.SetCategory("Report")
	cp.SetLastModifiedBy("B. Editor")
	cp.SetRevision(4)
	cp.SetCreated(time.Date(2026, 11, 14, 15, 0, 0, 0, ist))
	cp.SetModified(time.Date(2026, 11, 14, 16, 30, 59, 999, ist))

	out := save(t, d)
	if ct := string(zipEntry(t, out, "[Content_Types].xml")); !strings.Contains(ct, `PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"`) {
		t.Errorf("content types lack core.xml override:\n%s", ct)
	}
	if rels := string(zipEntry(t, out, "_rels/.rels")); !strings.Contains(rels, "metadata/core-properties") {
		t.Errorf("package rels lack core-properties:\n%s", rels)
	}

	re := mustOpen(t, out).CoreProperties()
	for _, c := range []struct{ name, got, want string }{
		{"Title", re.Title(), "Gap assessment"},
		{"Subject", re.Subject(), "DPDPA"},
		{"Author", re.Author(), "A. Author"},
		{"Keywords", re.Keywords(), "privacy; dpia"},
		{"Description", re.Description(), "Draft for review"},
		{"Category", re.Category(), "Report"},
		{"LastModifiedBy", re.LastModifiedBy(), "B. Editor"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
	if re.Revision() != 4 {
		t.Errorf("Revision = %d", re.Revision())
	}
	if c, ok := re.Created(); !ok || !c.Equal(time.Date(2026, 11, 14, 9, 30, 0, 0, time.UTC)) {
		t.Errorf("Created = %v, %v", c, ok)
	}
	if m, ok := re.Modified(); !ok || !m.Equal(time.Date(2026, 11, 14, 11, 0, 59, 0, time.UTC)) {
		t.Errorf("Modified = %v, %v (want truncated to the second, in UTC)", m, ok)
	}

	// Removing a property.
	re.SetTitle("")
	re.SetRevision(0)
	if re.Title() != "" || re.Revision() != 0 {
		t.Error("properties not removed")
	}
}

// Opening and saving without touching the properties keeps core.xml
// byte-identical; a change keeps the properties that were not touched.
func TestCoreProperties_Fixtures(t *testing.T) {
	files, _ := filepath.Glob("../testdata/fixtures/word/*.docx")
	n := 0
	for _, path := range files {
		data, err := os.ReadFile(path) // #nosec G304 -- test fixture path
		if err != nil {
			t.Fatal(err)
		}
		orig := zipEntry(t, data, "docProps/core.xml")
		if orig == nil {
			continue
		}
		n++
		t.Run(filepath.Base(path), func(t *testing.T) {
			d := mustOpen(t, data)
			before := d.CoreProperties()
			author, title := before.Author(), before.Title()
			if got := zipEntry(t, save(t, d), "docProps/core.xml"); !bytes.Equal(got, orig) {
				t.Fatal("core.xml changed although no property was set")
			}
			before.SetCategory("Checked")
			re := mustOpen(t, save(t, d)).CoreProperties()
			if re.Category() != "Checked" || re.Author() != author || re.Title() != title {
				t.Errorf("after edit: category %q, author %q (want %q), title %q (want %q)",
					re.Category(), re.Author(), author, re.Title(), title)
			}
		})
	}
	if n == 0 {
		t.Fatal("no Word fixture has docProps/core.xml")
	}
}
