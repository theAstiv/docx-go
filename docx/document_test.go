package docx_test

import (
	"bytes"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

func TestNew_EmptyDocument(t *testing.T) {
	d := docx.New()
	if d == nil {
		t.Fatal("New() returned nil")
	}
	issues := d.Validate()
	if len(issues) > 0 {
		t.Errorf("new document has validation issues: %v", issues)
	}
}

func TestDocument_On_Off_NoSharedState(t *testing.T) {
	a := docx.On()
	b := docx.On()
	if a == b {
		t.Error("On() must return distinct pointers each call")
	}
	*a = false
	if !*b {
		t.Error("mutating one On() pointer must not affect another")
	}
}

func TestDocument_AddParagraph(t *testing.T) {
	d := docx.New()
	body := d.Body()
	para := body.AddParagraph()
	if para == nil {
		t.Fatal("AddParagraph() returned nil")
	}
	run := para.AddRun("hello world")
	if run == nil {
		t.Fatal("AddRun() returned nil")
	}
	if got := run.Text(); got != "hello world" {
		t.Errorf("run.Text() = %q, want %q", got, "hello world")
	}
	if got := para.Text(); got != "hello world" {
		t.Errorf("para.Text() = %q, want %q", got, "hello world")
	}
}

func TestDocument_Paragraphs_Iterator(t *testing.T) {
	d := docx.New()
	body := d.Body()
	body.AddParagraph().AddRun("first")
	body.AddParagraph().AddRun("second")
	body.AddParagraph().AddRun("third")

	var texts []string
	for p := range body.Paragraphs() {
		texts = append(texts, p.Text())
	}
	if len(texts) != 3 {
		t.Fatalf("got %d paragraphs, want 3", len(texts))
	}
	for i, want := range []string{"first", "second", "third"} {
		if texts[i] != want {
			t.Errorf("para[%d].Text() = %q, want %q", i, texts[i], want)
		}
	}
}

func TestDocument_SaveRoundTrip(t *testing.T) {
	d := docx.New()
	body := d.Body()
	p := body.AddParagraph()
	p.AddRun("round-trip test")
	p.Format().SetStyle("Normal")
	p.Format().SetAlignment(docx.AlignLeft)

	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("Save() produced empty output")
	}

	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes() error: %v", err)
	}

	var texts []string
	for p2 := range d2.Body().Paragraphs() {
		texts = append(texts, p2.Text())
	}
	if len(texts) != 1 {
		t.Fatalf("reopened document has %d paragraphs, want 1", len(texts))
	}
	if texts[0] != "round-trip test" {
		t.Errorf("para text after round-trip = %q, want %q", texts[0], "round-trip test")
	}
}

func TestDocument_FormatProxy_WriteThrough(t *testing.T) {
	d := docx.New()
	p := d.Body().AddParagraph()

	f := p.Format()
	f.SetStyle("Heading1")
	f.SetAlignment(docx.AlignCenter)
	v := int64(3)
	f.SetOutlineLevel(&v)

	// Read back via a fresh proxy call.
	f2 := p.Format()
	if got := f2.Style(); got != "Heading1" {
		t.Errorf("Style() = %q, want %q", got, "Heading1")
	}
	if got := f2.Alignment(); got != docx.AlignCenter {
		t.Errorf("Alignment() = %q, want %q", got, docx.AlignCenter)
	}
	if got := f2.OutlineLevel(); got == nil || *got != 3 {
		t.Errorf("OutlineLevel() = %v, want 3", got)
	}
}

func TestDocument_RunFont_WriteThrough(t *testing.T) {
	d := docx.New()
	run := d.Body().AddParagraph().AddRun("styled")

	font := run.Font()
	font.SetBold(docx.On())
	font.SetColor("FF0000")
	font.SetSizePt(14)

	font2 := run.Font()
	if b := font2.Bold(); b == nil || !*b {
		t.Error("Bold() should be true")
	}
	if c := font2.Color(); c != "FF0000" {
		t.Errorf("Color() = %q, want %q", c, "FF0000")
	}
	if sz := font2.SizeHalfPt(); sz == nil || *sz != 28 {
		t.Errorf("SizeHalfPt() = %v, want 28", sz)
	}
}

func TestDocument_SaveAndReopen_FormattingPreserved(t *testing.T) {
	d := docx.New()
	p := d.Body().AddParagraph()
	p.Format().SetStyle("Heading1")
	run := p.AddRun("formatted")
	run.Font().SetBold(docx.On())
	run.Font().SetSizePt(16)

	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	d2, err := docx.OpenBytes(buf.Bytes())
	if err != nil {
		t.Fatalf("OpenBytes() error: %v", err)
	}

	var paras []*docx.Paragraph
	for p2 := range d2.Body().Paragraphs() {
		paras = append(paras, p2)
	}
	if len(paras) == 0 {
		t.Fatal("no paragraphs after round-trip")
	}

	pf := paras[0].Format()
	if got := pf.Style(); got != "Heading1" {
		t.Errorf("Style() after round-trip = %q, want %q", got, "Heading1")
	}

	var runs []*docx.Run
	for r := range paras[0].Runs() {
		runs = append(runs, r)
	}
	if len(runs) == 0 {
		t.Fatal("no runs after round-trip")
	}
	rf := runs[0].Font()
	if b := rf.Bold(); b == nil || !*b {
		t.Error("Bold should be preserved after round-trip")
	}
	if sz := rf.SizeHalfPt(); sz == nil || *sz != 32 {
		t.Errorf("SizeHalfPt() after round-trip = %v, want 32", sz)
	}
}
