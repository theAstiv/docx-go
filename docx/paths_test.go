package docx_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

// Packages whose parts are not at the canonical paths, or whose
// relationships name them differently (absolute or case-variant targets, a
// main part not named document.xml, unreferenced header parts). Each used to
// be opened wrong, or saved with edits lost or real parts overwritten.

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

const (
	relFooter    = `http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer`
	relSettings  = `http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings`
	relNumbering = `http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering`
	relCore      = `http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties`
)

// Two typed roles naming one part (a header relationship to the main
// document, say) were each parsed as their own type and saved in a fixed
// order, so the last writer replaced the part: the main document became a
// <w:hdr> (v0.5.0 review H2). Open now refuses the package.
func TestPaths_TypedRolesShareAPart(t *testing.T) {
	base := func(docRels []string, pkgRels ...string) map[string]string {
		return map[string]string{
			"[Content_Types].xml":          contentTypes,
			"_rels/.rels":                  rels(append([]string{rel("rId1", relDoc, "word/document.xml")}, pkgRels...)...),
			"word/document.xml":            documentXML("hello", ""),
			"word/_rels/document.xml.rels": rels(append([]string{rel("rId1", relStyles, "styles.xml")}, docRels...)...),
			"word/styles.xml":              stylesXML("Mine"),
			"word/header1.xml":             headerXML("head"),
		}
	}
	cases := []struct {
		name  string
		files map[string]string
	}{
		{"header is the document", base([]string{rel("rId2", relHeader, "document.xml")})},
		{"header is the document, other case", base([]string{rel("rId2", relHeader, "Document.XML")})},
		{"footer is the document", base([]string{rel("rId2", relFooter, "/word/document.xml")})},
		{"header and footer share a part", base([]string{rel("rId2", relHeader, "header1.xml"), rel("rId3", relFooter, "header1.xml")})},
		{"settings are the styles", base([]string{rel("rId2", relSettings, "styles.xml")})},
		{"numbering is the document", base([]string{rel("rId2", relNumbering, "document.xml")})},
		{"core properties are the document", base(nil, rel("rId2", relCore, "word/document.xml"))},
		{"styles are the document", func() map[string]string {
			f := base(nil)
			f["word/_rels/document.xml.rels"] = rels(rel("rId1", relStyles, "document.xml"))
			return f
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := docx.OpenBytes(zipFiles(t, tc.files))
			if !errors.Is(err, docx.ErrMalformedPart) {
				t.Fatalf("err = %v, want ErrMalformedPart", err)
			}
		})
	}
}

// A typed part whose root element is not the one its relationship type
// calls for (a header relationship to a footer part) is refused rather than
// parsed as the wrong type and written back over itself.
func TestPaths_WrongRootElement(t *testing.T) {
	ftr := `<w:ftr ` + pathsWNS + `><w:p/></w:ftr>`
	cases := []struct {
		name, relType, target, part string
	}{
		{"header holding a footer", relHeader, "header1.xml", ftr},
		{"footer holding a header", relFooter, "footer1.xml", headerXML("x")},
		{"settings holding styles", relSettings, "settings.xml", stylesXML("x")},
		{"numbering in another namespace", relNumbering, "numbering.xml", `<numbering xmlns="urn:x"/>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := zipFiles(t, map[string]string{
				"[Content_Types].xml":          contentTypes,
				"_rels/.rels":                  rels(rel("rId1", relDoc, "word/document.xml")),
				"word/document.xml":            documentXML("hello", ""),
				"word/_rels/document.xml.rels": rels(rel("rId1", relStyles, "styles.xml"), rel("rId2", tc.relType, tc.target)),
				"word/styles.xml":              stylesXML("Mine"),
				"word/" + tc.target:            tc.part,
			})
			_, err := docx.OpenBytes(data)
			if !errors.Is(err, docx.ErrMalformedPart) {
				t.Fatalf("err = %v, want ErrMalformedPart", err)
			}
		})
	}
	t.Run("main part not a document", func(t *testing.T) {
		data := zipFiles(t, map[string]string{
			"[Content_Types].xml": contentTypes,
			"_rels/.rels":         rels(rel("rId1", relDoc, "word/styles.xml")),
			"word/styles.xml":     stylesXML("Mine"),
		})
		if _, err := docx.OpenBytes(data); !errors.Is(err, docx.ErrMalformedPart) {
			t.Fatalf("err = %v, want ErrMalformedPart", err)
		}
	})
}

func relMode(id, typ, target, mode string) string {
	return `<Relationship Id="` + id + `" Type="` + typ + `" Target="` + target + `" TargetMode="` + mode + `"/>`
}

func coreXML(title string) string {
	return `<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" ` +
		`xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>` + title + `</dc:title></cp:coreProperties>`
}

// checkEntryNames fails unless every ZIP entry is a valid part name: rooted
// inside the package, no empty, "." or ".." segment, and no entry that is a
// folder of another.
func checkEntryNames(t *testing.T, entries map[string][]byte) {
	t.Helper()
	for name := range entries {
		if name == "" || strings.HasPrefix(name, "/") || strings.Contains(name, `\`) {
			t.Errorf("invalid entry %q", name)
			continue
		}
		for _, seg := range strings.Split(name, "/") {
			if seg == "" || seg == "." || seg == ".." {
				t.Errorf("invalid entry %q", name)
			}
		}
		for other := range entries {
			if strings.HasPrefix(strings.ToLower(other), strings.ToLower(name)+"/") {
				t.Errorf("entry %q is a folder of %q", name, other)
			}
		}
	}
}

// countRels counts the relationships of type typ in a saved .rels part.
func countRels(relsXML []byte, typ string) int {
	return strings.Count(string(relsXML), `Type="`+typ+`"`)
}

// A relationship target that climbs out of the package, is empty, names a
// folder or is a URI became the name the part was saved under: Save wrote
// "../../evil.xml" or an entry "word" beside the word/ folder, and the
// output no longer opened (v0.5.0 review H3). Such a target names no part:
// the styles relationship is replaced by one to a fresh part.
func TestPaths_StylesTargetOutsidePackage(t *testing.T) {
	for _, target := range []string{"../../../evil.xml", "", ".", "..", "http://x/y", "/"} {
		t.Run(target, func(t *testing.T) {
			data := zipFiles(t, map[string]string{
				"[Content_Types].xml":          contentTypes,
				"_rels/.rels":                  rels(rel("rId1", relDoc, "word/document.xml")),
				"word/document.xml":            documentXML("hello", ""),
				"word/_rels/document.xml.rels": rels(rel("rId1", relStyles, target)),
			})
			d, err := docx.OpenBytes(data)
			if err != nil {
				t.Fatal(err)
			}
			out := saveBytes(t, d)
			entries := readZip(t, out)
			checkEntryNames(t, entries)
			if n := countRels(entries["word/_rels/document.xml.rels"], relStyles); n != 1 {
				t.Errorf("%d styles relationships, want 1", n)
			}
			if _, ok := entries["word/styles.xml"]; !ok {
				t.Error("no word/styles.xml written")
			}
			if _, err := docx.OpenBytes(out); err != nil {
				t.Fatalf("reopen: %v", err)
			}
		})
	}
}

// A core properties relationship with an external, URI or escaping target
// made EditCoreProps create a part named "http:/x/y" or "../x" (review H3).
func TestPaths_CorePropsTargetOutsidePackage(t *testing.T) {
	for _, r := range []string{
		relMode("rId2", relCore, "http://x/y", "External"),
		rel("rId2", relCore, "http://x/y"),
		rel("rId2", relCore, "../x"),
		rel("rId2", relCore, "docProps"),
	} {
		t.Run(r, func(t *testing.T) {
			data := zipFiles(t, map[string]string{
				"[Content_Types].xml":          contentTypes,
				"_rels/.rels":                  rels(rel("rId1", relDoc, "word/document.xml"), r),
				"word/document.xml":            documentXML("hello", ""),
				"word/_rels/document.xml.rels": rels(rel("rId1", relStyles, "styles.xml")),
				"word/styles.xml":              stylesXML("Mine"),
				"docProps/app.xml":             `<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties"/>`,
			})
			d, err := docx.OpenBytes(data)
			if err != nil {
				t.Fatal(err)
			}
			if got := d.CoreProperties().Title(); got != "" {
				t.Errorf("Title() = %q from a target outside the package", got)
			}
			d.CoreProperties().SetTitle("New")
			out := saveBytes(t, d)
			entries := readZip(t, out)
			checkEntryNames(t, entries)
			if n := countRels(entries["_rels/.rels"], relCore); n != 1 {
				t.Errorf("%d core properties relationships, want 1", n)
			}
			d2, err := docx.OpenBytes(out)
			if err != nil {
				t.Fatalf("reopen: %v", err)
			}
			if got := d2.CoreProperties().Title(); got != "New" {
				t.Errorf("Title() after reopen = %q, want New", got)
			}
		})
	}
}

// Core properties were looked up by exact name: with target
// docProps/Core.xml and entry docProps/core.xml they read as empty, and an
// edit saved a second part, so the output failed to open (review M3).
func TestPaths_CorePropsTargetCase(t *testing.T) {
	data := zipFiles(t, map[string]string{
		"[Content_Types].xml":          contentTypes,
		"_rels/.rels":                  rels(rel("rId1", relDoc, "word/document.xml"), rel("rId2", relCore, "docProps/Core.xml")),
		"word/document.xml":            documentXML("hello", ""),
		"word/_rels/document.xml.rels": rels(rel("rId1", relStyles, "styles.xml")),
		"word/styles.xml":              stylesXML("Mine"),
		"docProps/core.xml":            coreXML("Old"),
	})
	d, err := docx.OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := d.CoreProperties().Title(); got != "Old" {
		t.Fatalf("Title() = %q, want Old", got)
	}
	d.CoreProperties().SetTitle("New")
	out := saveBytes(t, d)
	n := 0
	for name := range readZip(t, out) {
		if strings.EqualFold(name, "docProps/core.xml") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d core properties parts, want 1", n)
	}
	d2, err := docx.OpenBytes(out)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := d2.CoreProperties().Title(); got != "New" {
		t.Errorf("Title() after reopen = %q, want New", got)
	}
}

// Typed-part relationships with TargetMode="External" were loaded as parts
// inside the package (review L14). They name nothing: the part beside them
// is left alone, and a part created for the role replaces the relationship.
func TestPaths_ExternalTypedRelationship(t *testing.T) {
	settings := `<w:settings ` + pathsWNS + `><w:zoom w:percent="42"/></w:settings>`
	data := zipFiles(t, map[string]string{
		"[Content_Types].xml": contentTypes,
		"_rels/.rels":         rels(rel("rId1", relDoc, "word/document.xml")),
		"word/document.xml":   documentXML("hello", ""),
		"word/_rels/document.xml.rels": rels(relMode("rId1", relStyles, "styles.xml", "External"),
			relMode("rId2", relSettings, "settings.xml", "External")),
		"word/styles.xml":   stylesXML("Mine"),
		"word/settings.xml": settings,
	})
	d, err := docx.OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	if d.Styles().ByID("Mine") != nil {
		t.Error("styles behind an external relationship were loaded")
	}
	d.SetEvenAndOddHeaders(true) // creates a settings part
	out := saveBytes(t, d)
	entries := readZip(t, out)
	checkEntryNames(t, entries)
	if got := string(entries["word/styles.xml"]); !strings.Contains(got, "Mine") {
		t.Errorf("word/styles.xml was overwritten:\n%s", got)
	}
	if got := string(entries["word/settings.xml"]); got != settings {
		t.Errorf("word/settings.xml was overwritten:\n%s", got)
	}
	docRels := entries["word/_rels/document.xml.rels"]
	if strings.Contains(string(docRels), "External") {
		t.Errorf("external typed relationships kept beside the new parts:\n%s", docRels)
	}
	if countRels(docRels, relStyles) != 1 || countRels(docRels, relSettings) != 1 {
		t.Errorf("want one styles and one settings relationship:\n%s", docRels)
	}
	if _, err := docx.OpenBytes(out); err != nil {
		t.Fatalf("reopen: %v", err)
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
