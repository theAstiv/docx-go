package docx_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

func TestParagraphFormat_TabStops(t *testing.T) {
	d := docx.New()
	p := d.Body().AddParagraph()
	f := p.Format()

	f.AddTabStop(docx.TabStop{Position: docx.Twip(9000), Align: docx.TabAlignRight, Leader: docx.TabLeaderDot})
	f.AddTabStop(docx.TabStop{Position: docx.Twip(720)})
	f.AddTabStop(docx.TabStop{Position: docx.Twip(4500), Align: docx.TabAlignCenter})
	f.AddTabStop(docx.TabStop{Position: docx.Twip(720), Align: docx.TabAlignDecimal}) // replaces 720
	f.ClearTabStop(docx.Twip(2000))

	want := []docx.TabStop{
		{Position: 720, Align: docx.TabAlignDecimal},
		{Position: 2000, Align: docx.TabAlignClear},
		{Position: 4500, Align: docx.TabAlignCenter},
		{Position: 9000, Align: docx.TabAlignRight, Leader: docx.TabLeaderDot},
	}
	re := mustOpen(t, save(t, d))
	var got []docx.TabStop
	for rp := range re.Body().Paragraphs() {
		got = rp.Format().TabStops()
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("after reopen:\n got  %+v\nwant %+v", got, want)
	}

	f.RemoveTabStop(docx.Twip(2000))
	f.RemoveTabStop(docx.Twip(1)) // absent: no-op
	if n := len(f.TabStops()); n != 3 {
		t.Errorf("%d stops after RemoveTabStop, want 3", n)
	}
	f.SetTabStops(nil)
	if f.TabStops() != nil {
		t.Error("SetTabStops(nil) kept stops")
	}
}

// Clearing on a paragraph without w:pPr must not create one.
func TestParagraphFormat_ClearersDoNotCreatePPr(t *testing.T) {
	d := docx.New()
	f := d.Body().AddParagraph().Format()
	f.SetTabStops(nil)
	f.RemoveTabStop(docx.Twip(720))
	f.ClearBorders()
	f.SetShading("")
	if f.TabStops() != nil || f.Shading() != "" || f.Borders() != (docx.ParagraphBorderOptions{}) {
		t.Fatal("getters report values on an empty paragraph")
	}
	doc := string(zipEntry(t, save(t, d), "word/document.xml"))
	if strings.Contains(doc, "pPr") {
		t.Errorf("clearing created w:pPr:\n%s", doc)
	}
}

func TestParagraphFormat_BordersAndShading(t *testing.T) {
	d := docx.New()
	f := d.Body().AddParagraph().Format()
	line := &docx.Border{Style: docx.BorderSingle, Width: 8, Space: 4, Color: "1F3864"}
	f.SetBorders(docx.ParagraphBorderOptions{Top: line, Bottom: line, Between: &docx.Border{Style: docx.BorderDotted, Width: 4}})
	f.SetShading("F2F2F2")

	rf := firstParagraph(t, mustOpen(t, save(t, d))).Format()
	b := rf.Borders()
	if b.Top == nil || *b.Top != *line || b.Bottom == nil || b.Between == nil || b.Between.Style != docx.BorderDotted || b.Left != nil || b.Bar != nil {
		t.Errorf("Borders after reopen = %+v", b)
	}
	if rf.Shading() != "F2F2F2" {
		t.Errorf("Shading = %q", rf.Shading())
	}
	rf.ClearBorders()
	rf.SetShading("")
	if rf.Borders() != (docx.ParagraphBorderOptions{}) || rf.Shading() != "" {
		t.Error("ClearBorders / SetShading(\"\") did not remove")
	}
}

// The same ParagraphFormat proxy serves styles.
func TestParagraphFormat_OnStyle(t *testing.T) {
	d := docx.New()
	st := d.Styles().EnsureBuiltin(docx.StyleHeading1)
	st.Format().SetBorders(docx.ParagraphBorderOptions{Bottom: &docx.Border{Style: docx.BorderSingle, Width: 6}})
	st.Format().AddTabStop(docx.TabStop{Position: 9360, Align: docx.TabAlignRight})

	rst := mustOpen(t, save(t, d)).Styles().EnsureBuiltin(docx.StyleHeading1)
	if b := rst.Format().Borders(); b.Bottom == nil || b.Bottom.Width != 6 {
		t.Errorf("style borders after reopen = %+v", b)
	}
	if ts := rst.Format().TabStops(); len(ts) != 1 || ts[0].Position != 9360 || ts[0].Align != docx.TabAlignRight {
		t.Errorf("style tab stops after reopen = %+v", ts)
	}
}

func firstParagraph(t *testing.T, d *docx.Document) *docx.Paragraph {
	t.Helper()
	for p := range d.Body().Paragraphs() {
		return p
	}
	t.Fatal("no paragraph")
	return nil
}
