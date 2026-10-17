package docx_test

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

func reopen(t *testing.T, d *docx.Document) *docx.Document {
	t.Helper()
	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save: %v", err)
	}
	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}
	return d2
}

// partXML saves d and returns the raw bytes of one package part.
func partXML(t *testing.T, d *docx.Document, name string) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("zip: %v", err)
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
		if err != nil {
			t.Fatal(err)
		}
		if err := rc.Close(); err != nil {
			t.Fatal(err)
		}
		return b
	}
	t.Fatalf("part %s not found", name)
	return nil
}

func twipP(v int64) *docx.Twip { t := docx.Twip(v); return &t }

func TestStyles_EnsureBuiltin_AddsDependencies(t *testing.T) {
	d := docx.New()
	h := d.Styles().EnsureBuiltin(docx.StyleHeading1)
	if h == nil {
		t.Fatal("EnsureBuiltin(Heading1) = nil")
	}
	if h.BasedOn() != "Normal" || h.Name() != "heading 1" || h.Type() != docx.StyleTypeParagraph {
		t.Errorf("Heading1 = based %q name %q type %q", h.BasedOn(), h.Name(), h.Type())
	}
	if d.Styles().ByID("Normal") == nil {
		t.Error("Normal should have been created as a dependency")
	}

	link := d.Styles().EnsureBuiltin(docx.StyleHyperlink)
	if link.Type() != docx.StyleTypeCharacter || d.Styles().ByID("DefaultParagraphFont") == nil {
		t.Error("Hyperlink should pull in DefaultParagraphFont")
	}
	if d.Styles().EnsureBuiltin(docx.StyleTableGrid) == nil || d.Styles().ByID("TableNormal") == nil {
		t.Error("TableGrid should pull in TableNormal")
	}
	if d.Styles().EnsureBuiltin("NotAStyle") != nil {
		t.Error("unknown built-in should return nil")
	}
}

func TestStyles_EnsureBuiltin_KeepsExistingEdits(t *testing.T) {
	d := docx.New()
	h := d.Styles().EnsureBuiltin(docx.StyleHeading1)
	h.Font().SetSizePt(40)

	again := d.Styles().EnsureBuiltin(docx.StyleHeading1)
	if got := again.Font().SizePt(); got != 40 {
		t.Errorf("size after second EnsureBuiltin = %v, want 40 (edit preserved)", got)
	}
	n := 0
	for s := range d.Styles().All() {
		if s.ID() == "Heading1" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("Heading1 appears %d times, want 1", n)
	}
}

func TestStyles_AllBuiltinsRoundTrip(t *testing.T) {
	d := docx.New()
	all := []docx.BuiltinStyle{
		docx.StyleNormal, docx.StyleTitle, docx.StyleHeading1, docx.StyleHeading2,
		docx.StyleHeading3, docx.StyleHyperlink, docx.StyleTableGrid, docx.StyleTOC1,
		docx.StyleTOC2, docx.StyleTOC3, docx.StyleCaption,
	}
	for _, b := range all {
		if d.Styles().EnsureBuiltin(b) == nil {
			t.Fatalf("EnsureBuiltin(%s) = nil", b)
		}
	}
	d2 := reopen(t, d)
	for _, b := range all {
		if d2.Styles().ByID(string(b)) == nil {
			t.Fatalf("%s lost on round-trip", b)
		}
	}
	if got := d2.Styles().ByID("TableGrid").BasedOn(); got != "TableNormal" {
		t.Errorf("TableGrid basedOn = %q", got)
	}
	if got := d2.Styles().ByID("Heading2").Font().SizePt(); got != 13 {
		t.Errorf("Heading2 size = %v, want 13", got)
	}
}

func TestStyles_AddCustomStyles(t *testing.T) {
	d := docx.New()
	prio := int64(5)
	s := d.Styles().AddParagraphStyle("Callout", "Callout", docx.StyleOptions{
		BasedOn: "Normal", Next: "Callout", UIPriority: &prio, QuickStyle: true,
	})
	s.Format().SetSpaceBefore(twipP(240))
	s.Font().SetBold(docx.On())
	s.Font().SetColor("C00000")

	c := d.Styles().AddCharacterStyle("Code", "Code", docx.StyleOptions{})
	tb := d.Styles().AddTableStyle("Report", "Report", docx.StyleOptions{BasedOn: "TableNormal"})
	if c.Type() != docx.StyleTypeCharacter || tb.Type() != docx.StyleTypeTable {
		t.Error("wrong style types")
	}

	// Existing id returns the existing style without overwriting.
	if dup := d.Styles().AddParagraphStyle("Callout", "Other", docx.StyleOptions{}); dup.Name() != "Callout" {
		t.Errorf("duplicate id replaced style, name = %q", dup.Name())
	}

	d2 := reopen(t, d)
	got := d2.Styles().ByID("Callout")
	if got == nil {
		t.Fatal("Callout lost on round-trip")
	}
	if got.BasedOn() != "Normal" || got.Next() != "Callout" {
		t.Errorf("basedOn/next = %q/%q", got.BasedOn(), got.Next())
	}
	if p := got.UIPriority(); p == nil || *p != 5 {
		t.Errorf("UIPriority = %v", p)
	}
	if q := got.QuickStyle(); q == nil || !*q {
		t.Errorf("QuickStyle = %v", q)
	}
	if b := got.Font().Bold(); b == nil || !*b {
		t.Errorf("Bold = %v", b)
	}
	if sb := got.Format().SpaceBefore(); sb == nil || *sb != 240 {
		t.Errorf("SpaceBefore = %v", sb)
	}
}

func TestStyles_GettersDoNotCreateProperties(t *testing.T) {
	d := docx.New()
	s := d.Styles().AddParagraphStyle("Plain", "Plain", docx.StyleOptions{})
	_ = s.Format().Alignment()
	_ = s.Font().Bold()
	_ = s.Format().SpaceBefore()
	x := partXML(t, d, "word/styles.xml")
	if bytes.Contains(x, []byte("<w:pPr")) || bytes.Contains(x, []byte("<w:rPr")) {
		t.Errorf("getters created property elements: %s", x)
	}
}

func TestStyles_DocDefaults(t *testing.T) {
	d := docx.New()
	d.Styles().DocDefaultsFont().SetSizePt(11)
	d.Styles().DocDefaultsFormat().SetSpaceAfter(twipP(160))
	d2 := reopen(t, d)
	if got := d2.Styles().DocDefaultsFont().SizePt(); got != 11 {
		t.Errorf("default size = %v, want 11", got)
	}
	if sa := d2.Styles().DocDefaultsFormat().SpaceAfter(); sa == nil || *sa != 160 {
		t.Errorf("default space after = %v", sa)
	}
}

// TestExitGate_StyleEdit is the Phase 3a style criterion: changing Heading1's
// font size is reflected through every Heading1 paragraph after reopen.
func TestExitGate_StyleEdit(t *testing.T) {
	data, err := os.ReadFile("../testdata/fixtures/poi/heading123.docx")
	if err != nil {
		t.Fatal(err)
	}
	d, err := docx.OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	h1 := d.Styles().ByID("Heading1")
	if h1 == nil {
		t.Fatal("fixture has no Heading1 style")
	}
	h1.Font().SetSizePt(30)

	d2 := reopen(t, d)
	n := 0
	for p := range d2.Body().Paragraphs() {
		if p.Format().Style() != "Heading1" {
			continue
		}
		n++
		if got := d2.Styles().ByID(p.Format().Style()).Font().SizePt(); got != 30 {
			t.Errorf("Heading1 paragraph resolves to %v pt, want 30", got)
		}
	}
	if n == 0 {
		t.Fatal("fixture has no Heading1 paragraphs")
	}
}
