package opc_test

import (
	"bytes"
	"testing"

	"github.com/theAstiv/docx-go/internal/opc"
)

// minimalDocxParts returns the minimum set of parts needed to build a valid
// empty OPC package that could be opened as a .docx.
func minimalDocxParts() map[string][]byte {
	return map[string][]byte{
		"word/document.xml": []byte(
			`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
				`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
				`<w:body><w:p/><w:sectPr/></w:body></w:document>`),
	}
}

func buildMinimalPackage(t *testing.T) *opc.Package {
	t.Helper()
	// Use the exported Open path by first saving an empty package and reading
	// it back, so we exercise the full round-trip.
	pkg := opc.NewPackage()
	pkg.ContentTypes.SetDefault("rels", "application/vnd.openxmlformats-package.relationships+xml")
	pkg.ContentTypes.SetDefault("xml", "application/xml")
	pkg.ContentTypes.SetOverride("/word/document.xml",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml")

	for name, data := range minimalDocxParts() {
		pkg.SetPart(name, data)
	}
	pkg.Rels("").Add(opc.RelTypeOfficeDocument, "word/document.xml", "")
	return pkg
}

func TestPackage_RoundTrip_Empty(t *testing.T) {
	pkg := opc.NewPackage()
	pkg.ContentTypes.SetDefault("rels", "application/vnd.openxmlformats-package.relationships+xml")

	var buf bytes.Buffer
	if err := pkg.Save(&buf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	pkg2, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}
	if len(pkg2.PartPaths()) != 0 {
		t.Errorf("expected no parts, got %v", pkg2.PartPaths())
	}
}

func TestPackage_RoundTrip_MinimalDocx(t *testing.T) {
	pkg := buildMinimalPackage(t)

	var buf bytes.Buffer
	if err := pkg.Save(&buf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	pkg2, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes after save: %v", err)
	}

	doc := pkg2.Part("word/document.xml")
	if doc == nil {
		t.Fatal("word/document.xml not found after round-trip")
	}
	if !bytes.Contains(doc, []byte("w:document")) {
		t.Errorf("document content corrupted: %s", doc)
	}
}

func TestPackage_ContentType_Override(t *testing.T) {
	pkg := buildMinimalPackage(t)
	ct := pkg.ContentTypes.ContentTypeFor("/word/document.xml")
	if ct == "" {
		t.Error("Override content type not found for /word/document.xml")
	}
}

func TestPackage_Rels_RoundTrip(t *testing.T) {
	pkg := buildMinimalPackage(t)
	rels := pkg.Rels("")
	all := rels.All()
	if len(all) != 1 {
		t.Fatalf("expected 1 root relationship, got %d", len(all))
	}
	if all[0].Type != opc.RelTypeOfficeDocument {
		t.Errorf("wrong rel type: %s", all[0].Type)
	}

	var buf bytes.Buffer
	if err := pkg.Save(&buf); err != nil {
		t.Fatalf("Save: %v", err)
	}

	pkg2, err := opc.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}
	rels2 := pkg2.Rels("")
	all2 := rels2.All()
	if len(all2) != 1 {
		t.Fatalf("after round-trip: expected 1 root relationship, got %d", len(all2))
	}
	if all2[0].Target != "word/document.xml" {
		t.Errorf("after round-trip: wrong target: %s", all2[0].Target)
	}
}

func TestPackage_DeterministicOutput(t *testing.T) {
	pkg := buildMinimalPackage(t)

	var buf1, buf2 bytes.Buffer
	if err := pkg.Save(&buf1); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if err := pkg.Save(&buf2); err != nil {
		t.Fatalf("second save: %v", err)
	}
	if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
		t.Error("two saves of the same package produced different bytes")
	}
}
