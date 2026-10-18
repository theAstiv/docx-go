package docx

import (
	"testing"

	"github.com/theAstiv/docx-go/internal/oxml/wml"
)

// addRawRuns appends runs to p the way Word lays out one half of a field that
// crosses a paragraph boundary.
func addRawRuns(p *Paragraph, runs ...*wml.CT_R) {
	for _, r := range runs {
		p.p.Children = append(p.p.Children, r)
	}
}

// firstCell appends a 1x1 table to the body and returns its cell.
func firstCell(d *Document) *Cell {
	for r := range d.Body().AddTable(1, 1, TableOptions{}).Rows() {
		for c := range r.Cells() {
			return c
		}
	}
	return nil
}

func fieldResults(p *Paragraph) (codes, results []string) {
	for f := range p.Fields() {
		codes = append(codes, f.Code())
		results = append(results, f.CachedResult())
	}
	return codes, results
}

// A Word-made TOC: begin/instr/separate and the first entry in one paragraph,
// more entries (with nested PAGEREF fields) after it, end in the last one.
func newSpanningTOC(t *testing.T) (d *Document, first, mid, last *Paragraph) {
	t.Helper()
	d = New()
	runs := wml.NewFieldRuns(`TOC \o "1-3"`, "")
	first = d.Body().AddParagraph()
	addRawRuns(first, runs[0], runs[1], runs[2])
	first.AddRun("Heading one")

	mid = d.Body().AddParagraph()
	mid.AddRun("two")
	mid.AddField("PAGEREF a", "3")

	last = d.Body().AddParagraph()
	last.AddRun("three")
	addRawRuns(last, runs[4])
	return d, first, mid, last
}

func TestParagraphFields_SpansParagraphs(t *testing.T) {
	_, first, mid, last := newSpanningTOC(t)

	codes, results := fieldResults(first)
	if len(codes) != 1 || codes[0] != "TOC" {
		t.Fatalf("first paragraph fields = %v, want [TOC]", codes)
	}
	if want := "Heading one\ntwo3\nthree"; results[0] != want {
		t.Errorf("TOC result = %q, want %q", results[0], want)
	}

	// The middle paragraph reports only the field that lives wholly in it.
	codes, _ = fieldResults(mid)
	if len(codes) != 1 || codes[0] != "PAGEREF" {
		t.Errorf("middle paragraph fields = %v, want [PAGEREF]", codes)
	}
	// The closing paragraph holds a stray end marker and no field.
	if codes, _ = fieldResults(last); len(codes) != 0 {
		t.Errorf("closing paragraph fields = %v, want none", codes)
	}
}

func TestParagraphFields_SpansParagraphsInTableCell(t *testing.T) {
	d := New()
	runs := wml.NewFieldRuns("DATE", "")
	cell := firstCell(d)
	p1 := cell.SetText("")
	addRawRuns(p1, runs[0], runs[1], runs[2])
	p1.AddRun("today")
	p2 := cell.AddParagraph()
	addRawRuns(p2, runs[4])

	codes, results := fieldResults(p1)
	if len(codes) != 1 || codes[0] != "DATE" || results[0] != "today\n" {
		t.Errorf("cell fields = %v / %q, want [DATE] / %q", codes, results, "today\n")
	}
}

func TestParagraphFields_UnterminatedNotReported(t *testing.T) {
	d := New()
	runs := wml.NewFieldRuns("PAGE", "")
	p := d.Body().AddParagraph()
	addRawRuns(p, runs[0], runs[1], runs[2])
	d.Body().AddParagraph().AddRun("no end marker anywhere")

	if codes, _ := fieldResults(p); len(codes) != 0 {
		t.Errorf("unterminated field reported: %v", codes)
	}
}

func TestParagraphFields_StopsAtTable(t *testing.T) {
	d := New()
	runs := wml.NewFieldRuns("PAGE", "")
	p := d.Body().AddParagraph()
	addRawRuns(p, runs[0], runs[1], runs[2])
	d.Body().AddTable(1, 1, TableOptions{})
	addRawRuns(d.Body().AddParagraph(), runs[4])

	if codes, _ := fieldResults(p); len(codes) != 0 {
		t.Errorf("field running into a table reported: %v", codes)
	}
}

func TestParagraphFields_EarlyBreakFromSpan(t *testing.T) {
	_, first, _, _ := newSpanningTOC(t)
	n := 0
	for range first.Fields() {
		n++
		break
	}
	if n != 1 {
		t.Errorf("iterated %d fields before break, want 1", n)
	}
}
