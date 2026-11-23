package docx_test

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

// paras returns the top-level body paragraphs.
func paras(d *docx.Document) []*docx.Paragraph {
	return slices.Collect(d.Body().Paragraphs())
}

func rows(t *docx.Table) []*docx.Row { return slices.Collect(t.Rows()) }

func cells(r *docx.Row) []*docx.Cell { return slices.Collect(r.Cells()) }

func threeParas() *docx.Document {
	d := docx.New()
	for _, s := range []string{"A", "B", "C"} {
		d.Body().AddParagraph().AddRun(s)
	}
	return d
}

func TestParagraphRemove_Body(t *testing.T) {
	d := threeParas()
	b := paras(d)[1]
	if err := b.Remove(); err != nil {
		t.Fatal(err)
	}
	if got := d.Text(); got != "A\nC\n" {
		t.Errorf("Text = %q", got)
	}
	if err := b.Remove(); !errors.Is(err, docx.ErrNotInDocument) {
		t.Errorf("second Remove = %v, want ErrNotInDocument", err)
	}
	if got := reopen(t, d).Text(); got != "A\nC\n" {
		t.Errorf("after reopen: %q", got)
	}
}

// A proxy found by a fresh walk (no parent hint) can still be removed.
func TestParagraphRemove_RemoveWhileIterating(t *testing.T) {
	d := docx.New()
	for i := range 6 {
		d.Body().AddParagraph().AddRun(string(rune('a' + i)))
	}
	for i, p := range paras(d) {
		if i%2 == 1 {
			if err := p.Remove(); err != nil {
				t.Fatal(err)
			}
		}
	}
	if got := d.Text(); got != "a\nc\ne\n" {
		t.Errorf("Text = %q", got)
	}
}

func TestParagraphRemove_CellKeepsAParagraph(t *testing.T) {
	d := docx.New()
	tbl := d.Body().AddTable(1, 2, docx.TableOptions{})
	c := cells(rows(tbl)[0])[0]
	p := c.SetText("only")
	if err := p.Remove(); err != nil {
		t.Fatal(err)
	}
	ps := slices.Collect(c.Paragraphs())
	if len(ps) != 1 || c.Text() != "" {
		t.Errorf("cell has %d paragraphs, text %q; want one empty paragraph", len(ps), c.Text())
	}
}

func TestParagraphRemove_HeaderEndsWithParagraph(t *testing.T) {
	d := docx.New()
	h := d.LastSection().SetHeader(docx.HeaderDefault)
	h.SetText("top")
	h.AddTable(1, 1, docx.TableOptions{})
	ps := slices.Collect(h.Paragraphs())
	last := ps[len(ps)-1] // the empty paragraph AddTable put after the table
	if err := last.Remove(); err != nil {
		t.Fatal(err)
	}
	hdr := string(partXML(t, d, "word/header1.xml"))
	if !strings.HasSuffix(strings.TrimSpace(hdr), "</tbl><p></p></hdr>") {
		t.Errorf("header does not end with a paragraph:\n%s", hdr)
	}
}

func TestParagraphRemove_SectionBreak(t *testing.T) {
	d := threeParas()
	p := paras(d)[1]
	p.AddSectionBreak()
	if err := p.Remove(); !errors.Is(err, docx.ErrSectionBreak) {
		t.Fatalf("Remove = %v, want ErrSectionBreak", err)
	}
	if len(paras(d)) != 3 {
		t.Error("paragraph removed despite the error")
	}
}

func TestParagraphRemove_BookmarkAndCommentPartners(t *testing.T) {
	frag := `<w:p` + nsDecl + `><w:bookmarkStart w:id="7" w:name="Span"/><w:commentRangeStart w:id="3"/><w:r><w:t>one</w:t></w:r></w:p>` +
		`<w:p` + nsDecl + `><w:r><w:t>two</w:t></w:r><w:bookmarkEnd w:id="7"/><w:commentRangeEnd w:id="3"/></w:p>` +
		`<w:p` + nsDecl + `><w:bookmarkStart w:id="8" w:name="Whole"/><w:r><w:t>three</w:t></w:r><w:bookmarkEnd w:id="8"/></w:p>`
	d := injectIntoBody(t, docx.New(), frag)
	if d.Bookmark("Span") == nil {
		t.Fatal("fixture bookmark missing")
	}
	ps := paras(d)
	if err := ps[0].Remove(); err != nil {
		t.Fatal(err)
	}
	if d.Bookmark("Span") != nil {
		t.Error("Bookmark still listed after its start was removed")
	}
	doc := string(partXML(t, d, "word/document.xml"))
	for _, gone := range [][2]string{{"bookmarkEnd", "7"}, {"commentRangeEnd", "3"}} {
		if hasElemID(doc, gone[0], gone[1]) {
			t.Errorf("partner %s %s left behind:\n%s", gone[0], gone[1], doc)
		}
	}
	if !hasElemID(doc, "bookmarkEnd", "8") || !hasElemID(doc, "bookmarkStart", "8") || d.Bookmark("Whole") == nil {
		t.Errorf("an unrelated bookmark was removed:\n%s", doc)
	}
}

// hasElemID reports whether xml holds an element with the given local name
// and id attribute, whatever prefixes the encoder chose.
func hasElemID(xml, local, id string) bool {
	return regexp.MustCompile(`<(\w+:)?` + local + `\b[^>]*\b(\w+:)?id="` + id + `"`).MatchString(xml)
}

// A field that spans paragraphs (a table of contents) can only be removed
// whole: its first and last paragraph refuse, the ones it passes through
// do not.
func TestParagraphRemove_UnbalancedField(t *testing.T) {
	frag := `<w:p` + nsDecl + `><w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText> TOC \o "1-3" </w:instrText></w:r>` +
		`<w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:t>Intro</w:t></w:r></w:p>` +
		`<w:p` + nsDecl + `><w:r><w:t>Scope</w:t></w:r></w:p>` +
		`<w:p` + nsDecl + `><w:r><w:t>End</w:t></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>`
	d := injectIntoBody(t, docx.New(), frag)
	ps := paras(d)
	if err := ps[0].Remove(); !errors.Is(err, docx.ErrUnbalancedField) {
		t.Errorf("first paragraph: %v, want ErrUnbalancedField", err)
	}
	if err := ps[2].Remove(); !errors.Is(err, docx.ErrUnbalancedField) {
		t.Errorf("last paragraph: %v, want ErrUnbalancedField", err)
	}
	if err := ps[1].Remove(); err != nil {
		t.Errorf("middle paragraph: %v", err)
	}

	// A paragraph holding whole fields is removable.
	d2 := docx.New()
	p := d2.Body().AddParagraph()
	p.AddRun("Page ")
	p.AddPageNumber()
	if err := p.Remove(); err != nil {
		t.Errorf("paragraph with a whole field: %v", err)
	}
}

func TestTableRemove(t *testing.T) {
	d := threeParas()
	tbl := paras(d)[0].InsertTableAfter(2, 2, docx.TableOptions{})
	if err := tbl.Remove(); err != nil {
		t.Fatal(err)
	}
	if got := d.Text(); got != "A\nB\nC\n" {
		t.Errorf("Text = %q", got)
	}
	if err := tbl.Remove(); !errors.Is(err, docx.ErrNotInDocument) {
		t.Errorf("second Remove = %v", err)
	}
}

func TestRunRemove(t *testing.T) {
	d := docx.New()
	p := d.Body().AddParagraph()
	p.AddRun("a")
	b := p.AddRun("b")
	p.AddRun("c")
	if err := b.Remove(); err != nil {
		t.Fatal(err)
	}
	if p.Text() != "ac" {
		t.Errorf("Text = %q", p.Text())
	}
	if err := b.Remove(); !errors.Is(err, docx.ErrNotInDocument) {
		t.Errorf("second Remove = %v", err)
	}

	// A run in a hyperlink.
	h, err := p.AddHyperlink("https://example.com", docx.HyperlinkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	h.AddRun("x")
	link := h.AddRun("y")
	if err := link.Remove(); err != nil || h.Text() != "x" {
		t.Errorf("hyperlink run: err %v, text %q", err, h.Text())
	}

	// A run holding one field character.
	q := d.Body().AddParagraph()
	q.AddPageNumber()
	first := slices.Collect(q.Runs())[0]
	if err := first.Remove(); !errors.Is(err, docx.ErrUnbalancedField) {
		t.Errorf("field run: %v, want ErrUnbalancedField", err)
	}
}

func TestRowRemove(t *testing.T) {
	d := docx.New()
	tbl := d.Body().AddTable(3, 2, docx.TableOptions{})
	for i, r := range rows(tbl) {
		cells(r)[0].SetText(string(rune('1' + i)))
	}
	rs := rows(tbl)
	if err := rs[1].Remove(); err != nil {
		t.Fatal(err)
	}
	if got := d.Text(); got != "1\t\n3\t\n" {
		t.Errorf("Text = %q", got)
	}
	if err := rs[1].Remove(); !errors.Is(err, docx.ErrNotInDocument) {
		t.Errorf("second Remove = %v", err)
	}
	if err := rs[0].Remove(); err != nil {
		t.Fatal(err)
	}
	if err := rs[2].Remove(); !errors.Is(err, docx.ErrLastRow) {
		t.Errorf("last row: %v, want ErrLastRow", err)
	}
}

// Removing the row where a vertical merge starts passes the start down.
func TestRowRemove_VerticalMerge(t *testing.T) {
	d := docx.New()
	tbl := d.Body().AddTable(3, 2, docx.TableOptions{})
	rs := rows(tbl)
	cells(rs[0])[1].SetVMerge(docx.VMergeStart)
	cells(rs[1])[1].SetVMerge(docx.VMergeContinue)
	cells(rs[2])[1].SetVMerge(docx.VMergeContinue)
	if err := rs[0].Remove(); err != nil {
		t.Fatal(err)
	}
	rs = rows(tbl)
	if got := cells(rs[0])[1].VMerge(); got != docx.VMergeStart {
		t.Errorf("new first row: %v, want VMergeStart", got)
	}
	if got := cells(rs[1])[1].VMerge(); got != docx.VMergeContinue {
		t.Errorf("second row: %v, want VMergeContinue", got)
	}
	if got := cells(rs[0])[0].VMerge(); got != docx.VMergeNone {
		t.Errorf("unmerged column changed: %v", got)
	}
}

func TestInsertRow(t *testing.T) {
	d := docx.New()
	tbl := d.Body().AddTable(2, 2, docx.TableOptions{})
	rs := rows(tbl)
	ref := cells(rs[1])
	ref[0].SetShading("D9E2F3")
	ref[0].SetWidth(docx.WidthTwips(2400))
	ref[1].SetBorders(docx.BorderOptions{Top: &docx.Border{Style: docx.BorderSingle, Width: 4}})
	ref[0].Paragraphs()(func(p *docx.Paragraph) bool { p.Format().SetAlignment(docx.AlignCenter); return false })
	ref[0].SetText("old")

	nr := tbl.InsertRow(1, docx.RowOptions{})
	rs = rows(tbl)
	if len(rs) != 3 {
		t.Fatalf("%d rows", len(rs))
	}
	nc := cells(rs[1])
	if len(nc) != 2 || nc[0].Text() != "" || nc[0].Shading() != "D9E2F3" {
		t.Errorf("new row cells: %d, text %q, shading %q", len(nc), nc[0].Text(), nc[0].Shading())
	}
	if w, ok := nc[0].Width(); !ok || w.Twips() != 2400 {
		t.Errorf("width = %v, %v", w, ok)
	}
	if nc[1].Borders().Top == nil {
		t.Error("borders not copied")
	}
	for p := range nc[0].Paragraphs() {
		if p.Format().Alignment() != docx.AlignCenter {
			t.Errorf("paragraph formatting not copied: %q", p.Format().Alignment())
		}
	}
	if err := nr.Remove(); err != nil {
		t.Errorf("new row is removable: %v", err)
	}

	// Clamped, header option.
	hdr := tbl.InsertRow(-5, docx.RowOptions{Header: true})
	if !hdr.IsHeader() || rows(tbl)[0] == nil || !rows(tbl)[0].IsHeader() {
		t.Error("InsertRow(-5) did not insert a header row first")
	}
	tbl.InsertRow(99, docx.RowOptions{})
	if n := len(rows(tbl)); n != 4 {
		t.Errorf("%d rows after appending, want 4", n)
	}
	if rows(tbl)[3].IsHeader() {
		t.Error("appended row copied the header mark")
	}
}

// Inserting inside a vertically merged block extends it.
func TestInsertRow_ExtendsVerticalMerge(t *testing.T) {
	d := docx.New()
	tbl := d.Body().AddTable(2, 2, docx.TableOptions{})
	rs := rows(tbl)
	cells(rs[0])[0].SetVMerge(docx.VMergeStart)
	cells(rs[1])[0].SetVMerge(docx.VMergeContinue)
	tbl.InsertRow(1, docx.RowOptions{})
	got := []docx.VMergeState{}
	for _, r := range rows(tbl) {
		got = append(got, cells(r)[0].VMerge())
	}
	want := []docx.VMergeState{docx.VMergeStart, docx.VMergeContinue, docx.VMergeContinue}
	if !slices.Equal(got, want) {
		t.Errorf("merge states %v, want %v", got, want)
	}
	// Inserting above the start does not join the merge.
	tbl.InsertRow(0, docx.RowOptions{})
	if s := cells(rows(tbl)[0])[0].VMerge(); s != docx.VMergeNone {
		t.Errorf("row above the merge: %v", s)
	}
}

func TestInsertParagraphs(t *testing.T) {
	d := threeParas()
	b := paras(d)[1]
	b.InsertParagraphBefore("b0")
	b.InsertParagraphAfter("b1").InsertParagraphAfter("b2")
	b.InsertParagraphAfter("")
	if got := d.Text(); got != "A\nb0\nB\n\nb1\nb2\nC\n" {
		t.Errorf("Text = %q", got)
	}

	tbl := paras(d)[0].InsertTableAfter(1, 1, docx.TableOptions{})
	tbl.InsertParagraphBefore("before")
	tbl.InsertParagraphAfter("after")
	if got := d.Text(); !strings.HasPrefix(got, "A\nbefore\n\nafter\nb0") {
		t.Errorf("Text = %q", got)
	}

	// Detached: the new paragraph is returned but not attached.
	if err := b.Remove(); err != nil {
		t.Fatal(err)
	}
	before := d.Text()
	if np := b.InsertParagraphAfter("ghost"); np == nil || np.Text() != "ghost" {
		t.Error("insert next to a removed paragraph returned nothing")
	}
	if d.Text() != before {
		t.Error("insert next to a removed paragraph changed the document")
	}
}

func TestInsertTableAfter_InCell(t *testing.T) {
	d := docx.New()
	tbl := d.Body().AddTable(1, 1, docx.TableOptions{})
	c := cells(rows(tbl)[0])[0]
	p := c.SetText("x")
	p.InsertTableAfter(1, 2, docx.TableOptions{})
	doc := string(partXML(t, d, "word/document.xml"))
	if !strings.Contains(doc, "</tbl><p></p></tc>") {
		t.Errorf("nested table is not followed by a paragraph:\n%s", doc)
	}
}

// Fields still finds a table of contents that spans paragraphs, whether the
// paragraph proxy carries a container hint or not.
func TestFields_SpanningAfterRefactor(t *testing.T) {
	frag := `<w:p` + nsDecl + `><w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText> TOC </w:instrText></w:r>` +
		`<w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:t>One</w:t></w:r></w:p>` +
		`<w:p` + nsDecl + `><w:r><w:t>Two</w:t></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>`
	d := injectIntoBody(t, docx.New(), frag)
	var got []string
	for f := range paras(d)[0].Fields() {
		got = append(got, f.CachedResult())
	}
	if len(got) != 1 || got[0] != "One\nTwo" {
		t.Errorf("fields = %q", got)
	}
}

// Every fixture survives removing every other body paragraph that can be
// removed, and reopens with the remaining text intact.
func TestRemove_Fixtures(t *testing.T) {
	dirs := []string{"../testdata/fixtures/word", "../testdata/fixtures/poi", "../testdata/fixtures/python-docx"}
	n := 0
	for _, dir := range dirs {
		files, _ := filepath.Glob(filepath.Join(dir, "*.docx"))
		for _, path := range files {
			n++
			t.Run(filepath.Base(path), func(t *testing.T) {
				data, err := os.ReadFile(path) // #nosec G304 -- test fixture path
				if err != nil {
					t.Fatal(err)
				}
				d := mustOpen(t, data)
				var kept []string
				for i, p := range paras(d) {
					if i%2 == 1 {
						err := p.Remove()
						if err == nil {
							continue
						}
						if !errors.Is(err, docx.ErrSectionBreak) && !errors.Is(err, docx.ErrUnbalancedField) {
							t.Fatalf("paragraph %d: %v", i, err)
						}
					}
					kept = append(kept, p.Text())
				}
				var got []string
				for p := range reopen(t, d).Body().Paragraphs() {
					got = append(got, p.Text())
				}
				if !slices.Equal(got, kept) {
					t.Errorf("after reopen: %d paragraphs, want %d", len(got), len(kept))
				}
			})
		}
	}
	if n == 0 {
		t.Fatal("no fixtures")
	}
}

// SetText drops the container's content with the same cleanup as Remove:
// the outside partner of a dropped anchor goes, and the id cache forgets
// the bookmarks that were inside.
func TestCellSetText_CleansUpAnchors(t *testing.T) {
	frag := `<w:tbl` + nsDecl + `><w:tblPr/><w:tblGrid><w:gridCol w:w="2000"/></w:tblGrid><w:tr><w:tc><w:tcPr><w:tcW w:w="2000" w:type="dxa"/></w:tcPr>` +
		`<w:p><w:bookmarkStart w:id="7" w:name="Span"/><w:r><w:t>one</w:t></w:r></w:p>` +
		`<w:p><w:bookmarkStart w:id="8" w:name="Inner"/><w:r><w:t>two</w:t></w:r><w:bookmarkEnd w:id="8"/></w:p>` +
		`</w:tc></w:tr></w:tbl>` +
		`<w:p` + nsDecl + `><w:r><w:t>after</w:t></w:r><w:bookmarkEnd w:id="7"/></w:p>`
	d := injectIntoBody(t, docx.New(), frag)
	if d.Bookmark("Span") == nil || d.Bookmark("Inner") == nil {
		t.Fatal("fixture bookmarks missing")
	}
	n := 0
	for range d.Bookmarks() { // fill the id cache
		n++
	}
	tbl := slices.Collect(d.Body().Tables())[0]
	cells(rows(tbl)[0])[0].SetText("new")
	if d.Bookmark("Span") != nil || d.Bookmark("Inner") != nil {
		t.Error("dropped bookmarks still listed")
	}
	for range d.Bookmarks() {
		t.Error("Bookmarks lists a dropped bookmark")
	}
	doc := string(partXML(t, d, "word/document.xml"))
	if hasElemID(doc, "bookmarkEnd", "7") {
		t.Errorf("orphan bookmarkEnd left behind:\n%s", doc)
	}
	if !strings.Contains(d.Text(), "after") {
		t.Errorf("text after the table lost: %q", d.Text())
	}
}

// SetMargins on a section without w:pgMar writes the attributes the schema
// requires besides the four margins.
func TestSectionSetMargins_RequiredAttrs(t *testing.T) {
	frag := `<w:p` + nsDecl + `><w:pPr><w:sectPr><w:pgSz w:w="12240" w:h="15840"/></w:sectPr></w:pPr></w:p>`
	d := injectIntoBody(t, docx.New(), frag)
	sec := paras(d)[0].SectionBreak()
	sec.SetMargins(docx.PageMargins{Top: 1000, Right: 1000, Bottom: 1000, Left: 1000})
	doc := string(partXML(t, d, "word/document.xml"))
	m := regexp.MustCompile(`<(\w+:)?pgMar\b[^>]*>`).FindString(doc)
	for _, a := range []string{"top", "right", "bottom", "left", "header", "footer", "gutter"} {
		if !regexp.MustCompile(`\b(\w+:)?` + a + `="`).MatchString(m) {
			t.Errorf("pgMar lacks %s: %s", a, m)
		}
	}
}
