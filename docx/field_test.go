package docx_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

func collectFields(p *docx.Paragraph) []*docx.Field {
	var out []*docx.Field
	for f := range p.Fields() {
		out = append(out, f)
	}
	return out
}

func TestField_AddFieldRoundTrip(t *testing.T) {
	d := docx.New()
	p := d.Body().AddParagraph()
	p.AddRun("Report ")
	f := p.AddField(`REF intro \h`, "Introduction")
	p.AddRun(" end")

	if f.Instruction() != `REF intro \h` || f.Code() != "REF" || f.CachedResult() != "Introduction" {
		t.Errorf("new field = %q / %q / %q", f.Instruction(), f.Code(), f.CachedResult())
	}

	d2 := reopen(t, d)
	for para := range d2.Body().Paragraphs() {
		fields := collectFields(para)
		if len(fields) != 1 {
			t.Fatalf("%d fields after round-trip, want 1", len(fields))
		}
		got := fields[0]
		if got.Instruction() != `REF intro \h` || got.CachedResult() != "Introduction" {
			t.Errorf("field = %q / %q", got.Instruction(), got.CachedResult())
		}
		// The cached result is visible text; the instruction is not.
		if para.Text() != "Report Introduction end" {
			t.Errorf("paragraph text = %q", para.Text())
		}
	}
}

func TestField_PageNumberOfCount(t *testing.T) {
	d := docx.New()
	ftr := d.LastSection().SetFooter(docx.FooterDefault)
	p := ftr.AddParagraph()
	p.AddRun("Page ")
	p.AddPageNumber()
	p.AddRun(" of ")
	p.AddPageCount()

	d2 := reopen(t, d)
	ftr2 := d2.LastSection().Footer(docx.FooterDefault)
	if ftr2 == nil {
		t.Fatal("footer lost")
	}
	found := false
	for para := range ftr2.Paragraphs() {
		fields := collectFields(para)
		if len(fields) == 0 {
			continue
		}
		found = true
		if len(fields) != 2 || fields[0].Code() != "PAGE" || fields[1].Code() != "NUMPAGES" {
			t.Errorf("footer fields = %d, codes %q %q", len(fields), fields[0].Code(), fields[len(fields)-1].Code())
		}
		if para.Text() != "Page 1 of 1" {
			t.Errorf("footer text = %q", para.Text())
		}
	}
	if !found {
		t.Fatal("no footer paragraph with fields")
	}
}

func TestField_NoFieldsInPlainParagraph(t *testing.T) {
	d := docx.New()
	p := d.Body().AddParagraph()
	p.AddRun("plain")
	if got := collectFields(p); len(got) != 0 {
		t.Errorf("got %d fields in a plain paragraph", len(got))
	}
}

func TestField_FixtureFieldCodes(t *testing.T) {
	data, err := os.ReadFile("../testdata/fixtures/poi/FieldCodes.docx")
	if err != nil {
		t.Fatal(err)
	}
	d, err := docx.OpenBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	var codes []string
	for p := range d.Body().Paragraphs() {
		for f := range p.Fields() {
			codes = append(codes, f.Code())
		}
	}
	if len(codes) < 2 || codes[0] != "AUTHOR" || codes[1] != "CREATEDATE" {
		t.Errorf("fixture field codes = %v, want AUTHOR then CREATEDATE", codes)
	}
	// Survives a round-trip with the same instructions.
	d2 := reopen(t, d)
	n := 0
	for p := range d2.Body().Paragraphs() {
		for range p.Fields() {
			n++
		}
	}
	if n != len(codes) {
		t.Errorf("fields after round-trip = %d, want %d", n, len(codes))
	}
}

func TestDocument_AddTOC(t *testing.T) {
	d := docx.New()
	d.Styles().EnsureBuiltin(docx.StyleHeading1)
	d.Body().AddParagraph().AddRun("Intro")

	f, err := d.AddTOC(docx.TOCOptions{Hyperlinks: true})
	if err != nil {
		t.Fatalf("AddTOC: %v", err)
	}
	if f.Instruction() != `TOC \o "1-3" \h \z \u` {
		t.Errorf("instruction = %q", f.Instruction())
	}
	if !strings.Contains(f.CachedResult(), "update table of contents") {
		t.Errorf("placeholder = %q", f.CachedResult())
	}
	for _, id := range []string{"TOC1", "TOC2", "TOC3", "Hyperlink"} {
		if d.Styles().ByID(id) == nil {
			t.Errorf("style %s not ensured", id)
		}
	}

	d2 := reopen(t, d)
	found := false
	for p := range d2.Body().Paragraphs() {
		for fld := range p.Fields() {
			if fld.Code() == "TOC" {
				found = true
			}
		}
	}
	if !found {
		t.Error("TOC field lost on round-trip")
	}
}

func TestDocument_AddTOC_Levels(t *testing.T) {
	d := docx.New()
	f, err := d.AddTOC(docx.TOCOptions{Levels: "2-4"})
	if err != nil {
		t.Fatal(err)
	}
	if f.Instruction() != `TOC \o "2-4" \z \u` {
		t.Errorf("instruction = %q", f.Instruction())
	}
	if d.Styles().ByID("Hyperlink") != nil {
		t.Error("Hyperlink style should not be added when Hyperlinks is false")
	}
	for _, bad := range []string{"3", "1-", "0-3", "4-2", "1-10", "a-b", "1,3"} {
		if _, err := d.AddTOC(docx.TOCOptions{Levels: bad}); !errors.Is(err, docx.ErrInvalidTOCLevels) {
			t.Errorf("Levels %q err = %v, want ErrInvalidTOCLevels", bad, err)
		}
	}
	// Rejected calls must not add paragraphs: only the first TOC exists.
	n := 0
	for p := range d.Body().Paragraphs() {
		for range p.Fields() {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d fields in body, want 1", n)
	}
}

func TestDocument_UpdateFieldsOnOpen(t *testing.T) {
	d := docx.New()
	if d.UpdateFieldsOnOpen() {
		t.Error("expected false on a new document")
	}
	d.SetUpdateFieldsOnOpen(true)
	if !d.UpdateFieldsOnOpen() {
		t.Error("expected true after SetUpdateFieldsOnOpen(true)")
	}
	d2 := reopen(t, d)
	if !d2.UpdateFieldsOnOpen() {
		t.Error("updateFields not persisted")
	}
	d2.SetUpdateFieldsOnOpen(false)
	if d2.UpdateFieldsOnOpen() {
		t.Error("expected false after SetUpdateFieldsOnOpen(false)")
	}
}
