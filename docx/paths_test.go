package docx_test

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

// Packages whose parts are not at the canonical paths, or whose
// relationships name them differently (review 2026-11-19, P0 #7-#9 and the
// header numbering finding). Each was opened wrong, or saved with edits lost
// or real parts overwritten.

const (
	pathsWNS   = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"`
	pathsRelNS = `http://schemas.openxmlformats.org/package/2006/relationships`
	relDoc     = `http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument`
	relStyles  = `http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles`
	relHeader  = `http://schemas.openxmlformats.org/officeDocument/2006/relationships/header`
)

// zipFiles builds a ZIP from name → content.
func zipFiles(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func rels(rs ...string) string {
	return `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="` + pathsRelNS + `">` + strings.Join(rs, "") + `</Relationships>`
}

func rel(id, typ, target string) string {
	return `<Relationship Id="` + id + `" Type="` + typ + `" Target="` + target + `"/>`
}

const contentTypes = `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
	`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
	`<Default Extension="xml" ContentType="application/xml"/></Types>`

func documentXML(text string, sect string) string {
	return `<w:document ` + pathsWNS + ` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><w:body>` +
		`<w:p><w:r><w:t>` + text + `</w:t></w:r></w:p><w:sectPr>` + sect + `</w:sectPr></w:body></w:document>`
}

func stylesXML(id string) string {
	return `<w:styles ` + pathsWNS + `><w:style w:type="paragraph" w:styleId="` + id + `"><w:name w:val="` + id + `"/></w:style></w:styles>`
}

func headerXML(text string) string {
	return `<w:hdr ` + pathsWNS + `><w:p><w:r><w:t>` + text + `</w:t></w:r></w:p></w:hdr>`
}

func saveBytes(t *testing.T, d *docx.Document) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// An absolute officeDocument target, as System.IO.Packaging and the Open XML
// SDK write it, failed to open.
func TestPaths_AbsoluteOfficeDocumentTarget(t *testing.T) {
	data := zipFiles(t, map[string]string{
		"[Content_Types].xml":          contentTypes,
		"_rels/.rels":                  rels(rel("rId1", relDoc, "/word/document.xml")),
		"word/document.xml":            documentXML("hello", ""),
		"word/_rels/document.xml.rels": rels(rel("rId1", relStyles, "/word/styles.xml")),
		"word/styles.xml":              stylesXML("Mine"),
	})
	d, err := docx.OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.Text(), "hello") {
		t.Errorf("Text() = %q", d.Text())
	}
	if d.Styles().ByID("Mine") == nil {
		t.Error("styles behind an absolute target were not loaded")
	}
}

// A main part at word/document2.xml was saved to word/document.xml, leaving
// the real part unchanged: every edit was lost.
func TestPaths_NonCanonicalMainPart(t *testing.T) {
	data := zipFiles(t, map[string]string{
		"[Content_Types].xml":           contentTypes,
		"_rels/.rels":                   rels(rel("rId1", relDoc, "word/document2.xml")),
		"word/document2.xml":            documentXML("hello", ""),
		"word/_rels/document2.xml.rels": rels(rel("rId1", relStyles, "styles.xml")),
		"word/styles.xml":               stylesXML("Mine"),
	})
	d, err := docx.OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	d.Body().AddParagraph().AddRun("edited")
	d.LastSection().SetHeader(docx.HeaderDefault).AddParagraph().AddRun("head")
	out := saveBytes(t, d)
	entries := readZip(t, out)
	if _, ok := entries["word/document.xml"]; ok {
		t.Error("an orphan word/document.xml was written")
	}
	d2, err := docx.OpenBytes(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d2.Text(), "edited") {
		t.Errorf("edit lost: Text() = %q", d2.Text())
	}
	if h := d2.LastSection().Header(docx.HeaderDefault); h == nil || !strings.Contains(h.Text(), "head") {
		t.Error("the header's relationship was not attached to the main part")
	}
}

// A styles target that differs in case from the ZIP entry was not found, and
// the real word/styles.xml was overwritten with defaults.
func TestPaths_StylesTargetCase(t *testing.T) {
	data := zipFiles(t, map[string]string{
		"[Content_Types].xml":          contentTypes,
		"_rels/.rels":                  rels(rel("rId1", relDoc, "word/document.xml")),
		"word/document.xml":            documentXML("hello", ""),
		"word/_rels/document.xml.rels": rels(rel("rId1", relStyles, "Styles.xml")),
		"word/styles.xml":              stylesXML("Mine"),
	})
	d, err := docx.OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if d.Styles().ByID("Mine") == nil {
		t.Fatal("styles were not loaded")
	}
	entries := readZip(t, saveBytes(t, d))
	n := 0
	for name, b := range entries {
		if strings.EqualFold(name, "word/styles.xml") {
			n++
			if !strings.Contains(string(b), "Mine") {
				t.Errorf("%s lost its styles", name)
			}
		}
	}
	if n != 1 {
		t.Errorf("%d styles parts, want 1", n)
	}
}

// Without a styles relationship, the defaults were written over an
// unreferenced word/styles.xml.
func TestPaths_OrphanStylesNotOverwritten(t *testing.T) {
	data := zipFiles(t, map[string]string{
		"[Content_Types].xml": contentTypes,
		"_rels/.rels":         rels(rel("rId1", relDoc, "word/document.xml")),
		"word/document.xml":   documentXML("hello", ""),
		"word/styles.xml":     stylesXML("Orphan"),
	})
	d, err := docx.OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	out := saveBytes(t, d)
	if got := string(readZip(t, out)["word/styles.xml"]); !strings.Contains(got, "Orphan") {
		t.Errorf("word/styles.xml was overwritten:\n%s", got)
	}
	if _, err := docx.OpenBytes(out); err != nil {
		t.Fatal(err)
	}
}

// Header numbering looked only at referenced headers, so a new header
// overwrote an unreferenced word/header1.xml.
func TestPaths_NewHeaderAvoidsUnreferencedPart(t *testing.T) {
	data := zipFiles(t, map[string]string{
		"[Content_Types].xml":          contentTypes,
		"_rels/.rels":                  rels(rel("rId1", relDoc, "word/document.xml")),
		"word/document.xml":            documentXML("hello", ""),
		"word/_rels/document.xml.rels": rels(rel("rId1", relStyles, "styles.xml")),
		"word/styles.xml":              stylesXML("Mine"),
		"word/header1.xml":             headerXML("orphan"),
	})
	d, err := docx.OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	d.LastSection().SetHeader(docx.HeaderDefault).AddParagraph().AddRun("new")
	entries := readZip(t, saveBytes(t, d))
	if got := string(entries["word/header1.xml"]); !strings.Contains(got, "orphan") {
		t.Errorf("word/header1.xml was overwritten:\n%s", got)
	}
}

// Two relationships to one header part gave two trees, and the last one
// saved won; an edit through the first was lost. Replace must also count the
// shared header once.
func TestPaths_SharedHeaderPart(t *testing.T) {
	sect := `<w:headerReference w:type="default" r:id="rId2"/><w:headerReference w:type="first" r:id="rId3"/>`
	data := zipFiles(t, map[string]string{
		"[Content_Types].xml": contentTypes,
		"_rels/.rels":         rels(rel("rId1", relDoc, "word/document.xml")),
		"word/document.xml":   documentXML("hello", sect),
		"word/_rels/document.xml.rels": rels(rel("rId1", relStyles, "styles.xml"),
			rel("rId2", relHeader, "header1.xml"), rel("rId3", relHeader, "header1.xml")),
		"word/styles.xml":  stylesXML("Mine"),
		"word/header1.xml": headerXML("shared"),
	})
	d, err := docx.OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if res := d.Replace("shared", "changed", docx.ReplaceOptions{}); res.Count != 1 {
		t.Errorf("Replace count = %d, want 1 (one part)", res.Count)
	}
	d.LastSection().Header(docx.HeaderDefault).AddParagraph().AddRun("added")
	got := string(readZip(t, saveBytes(t, d))["word/header1.xml"])
	if !strings.Contains(got, "changed") || !strings.Contains(got, "added") {
		t.Errorf("edits through the first reference were lost:\n%s", got)
	}
}
