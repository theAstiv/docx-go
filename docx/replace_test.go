package docx_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

func TestReplace_Everywhere(t *testing.T) {
	d := docx.New()
	d.Body().AddParagraph().AddRun("ACME and acme and ACME")
	tbl := d.Body().AddTable(1, 1, docx.TableOptions{})
	cells(rows(tbl)[0])[0].SetText("cell ACME")
	sec := d.LastSection()
	sec.SetHeader(docx.HeaderDefault).SetText("ACME header")
	sec.SetFooter(docx.FooterDefault).SetText("ACME footer")

	res := d.Replace("ACME", "Globex", docx.ReplaceOptions{})
	if res.Count != 5 || len(res.Skipped) != 0 {
		t.Fatalf("result = %+v, want 5 replaced", res)
	}
	if got := d.Text(); got != "Globex and acme and Globex\ncell Globex\n" {
		t.Errorf("body = %q", got)
	}
	if h := sec.Header(docx.HeaderDefault).Text(); h != "Globex header\n" {
		t.Errorf("header = %q", h)
	}

	d2 := docx.New()
	d2.Body().AddParagraph().AddRun("x")
	s2 := d2.LastSection()
	s2.SetHeader(docx.HeaderDefault).SetText("x")
	s2.SetFooter(docx.FooterDefault).SetText("x")
	if res := d2.Replace("x", "y", docx.ReplaceOptions{SkipHeaders: true, SkipFooters: true}); res.Count != 1 {
		t.Errorf("SkipHeaders/SkipFooters: %d replaced, want 1", res.Count)
	}
}

// Word splits text across runs; the match is still found, and the
// replacement takes the first run's formatting.
func TestReplace_SplitRuns(t *testing.T) {
	frag := `<w:p` + nsDecl + `><w:r><w:rPr><w:b/></w:rPr><w:t>Dear Cl</w:t></w:r><w:proofErr w:type="spellStart"/>` +
		`<w:r><w:t>ie</w:t></w:r><w:proofErr w:type="spellEnd"/><w:r><w:rPr><w:i/></w:rPr><w:t xml:space="preserve">nt, hi</w:t></w:r></w:p>`
	d := injectIntoBody(t, docx.New(), frag)
	res := d.Replace("Client", "Mr Smith", docx.ReplaceOptions{})
	if res.Count != 1 {
		t.Fatalf("result = %+v", res)
	}
	p := paras(d)[0]
	if p.Text() != "Dear Mr Smith, hi" {
		t.Errorf("text = %q", p.Text())
	}
	first := slices.Collect(p.Runs())[0]
	if first.Text() != "Dear Mr Smith" || first.Font().Bold() == nil || !*first.Font().Bold() {
		t.Errorf("first run %q (bold %v); want the replacement in the bold run", first.Text(), first.Font().Bold())
	}
	if got := reopen(t, d).Text(); got != "Dear Mr Smith, hi\n" {
		t.Errorf("after reopen: %q", got)
	}
}

func TestReplace_BreaksAndTabs(t *testing.T) {
	d := docx.New()
	d.Body().AddParagraph().AddRun("Address: ADDR.")
	d.Replace("ADDR", "1 Main St\r\nSpringfield\tUSA", docx.ReplaceOptions{})
	if got := d.Text(); got != "Address: 1 Main St\nSpringfield\tUSA.\n" {
		t.Errorf("Text = %q", got)
	}
}

func TestReplace_IgnoreCaseAndLimit(t *testing.T) {
	d := docx.New()
	d.Body().AddParagraph().AddRun("Ärger, äRGER, ÄRGER, Ärger")
	if res := d.Replace("ärger", "x", docx.ReplaceOptions{IgnoreCase: true, Limit: 3}); res.Count != 3 || len(res.Skipped) != 0 {
		t.Fatalf("result = %+v", res)
	}
	if got := d.Text(); got != "x, x, x, Ärger\n" {
		t.Errorf("Text = %q", got)
	}
	if res := d.Replace("ärger", "x", docx.ReplaceOptions{}); res.Count != 0 {
		t.Error("case-sensitive search matched a different case")
	}
}

func TestReplace_Skipped(t *testing.T) {
	frag := // split by a bookmark
		`<w:p` + nsDecl + `><w:r><w:t>Hel</w:t></w:r><w:bookmarkStart w:id="1" w:name="b"/><w:r><w:t>lo</w:t></w:r><w:bookmarkEnd w:id="1"/></w:p>` +
			// inside a tracked insertion
			`<w:p` + nsDecl + `><w:ins w:id="2" w:author="a"><w:r><w:t>Hello</w:t></w:r></w:ins><w:r><w:t xml:space="preserve"> Hello</w:t></w:r></w:p>` +
			// inside a block content control
			`<w:sdt` + nsDecl + `><w:sdtContent><w:p><w:r><w:t>Hello sdt</w:t></w:r></w:p></w:sdtContent></w:sdt>` +
			// inside a table of contents
			`<w:p` + nsDecl + `><w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText> TOC </w:instrText></w:r>` +
			`<w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:t>Hello toc</w:t></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>`
	d := injectIntoBody(t, docx.New(), frag)
	res := d.Replace("Hello", "Bye", docx.ReplaceOptions{})
	if res.Count != 1 {
		t.Errorf("replaced %d, want 1 (the plain run)", res.Count)
	}
	var skipped []string
	for _, s := range res.Skipped {
		if s.Part != "word/document.xml" || s.Count != 1 {
			t.Errorf("skip %+v", s)
		}
		skipped = append(skipped, s.Text)
	}
	want := []string{"Hello", "Hello Hello", "Hello sdt", "Hello toc"} // text before replacing
	if !slices.Equal(skipped, want) {
		t.Errorf("skipped %q, want %q", skipped, want)
	}
}

// Field results are replaced; instructions are never searched.
func TestReplace_Fields(t *testing.T) {
	d := docx.New()
	p := d.Body().AddParagraph()
	p.AddField("DATE", "DATE")
	res := d.Replace("DATE", "today", docx.ReplaceOptions{})
	if res.Count != 1 || p.Text() != "today" {
		t.Errorf("result %+v, text %q", res, p.Text())
	}
	var instr []string
	for f := range p.Fields() {
		instr = append(instr, f.Code())
	}
	if !slices.Equal(instr, []string{"DATE"}) {
		t.Errorf("field codes = %q, want the instruction untouched", instr)
	}
}

func TestParagraphReplace_Scoped(t *testing.T) {
	d := threeParas()
	ps := paras(d)
	ps[1].AddRun(" A")
	if res := ps[1].Replace("A", "z", docx.ReplaceOptions{}); res.Count != 1 {
		t.Errorf("%+v", res)
	}
	if got := d.Text(); got != "A\nB z\nC\n" {
		t.Errorf("Text = %q", got)
	}
}

func TestReplace_RejectsUnsearchable(t *testing.T) {
	d := threeParas()
	for _, old := range []string{"", "A\tB", "A\nB"} {
		if res := d.Replace(old, "x", docx.ReplaceOptions{}); res.Count != 0 || res.Skipped != nil {
			t.Errorf("Replace(%q) = %+v", old, res)
		}
	}
	if d.Text() != "A\nB\nC\n" {
		t.Error("document changed")
	}
}

// In every fixture, each body paragraph with nothing skipped reads exactly
// as strings.ReplaceAll of its old text, and the result reopens.
func TestReplace_FixturesProperty(t *testing.T) {
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
				var before []string
				for _, p := range paras(d) {
					before = append(before, p.Text())
				}
				res := d.Replace("e", "[E]", docx.ReplaceOptions{SkipHeaders: true, SkipFooters: true})
				skipped := map[string]bool{}
				for _, s := range res.Skipped {
					skipped[s.Text] = true
				}
				total := 0
				for i, p := range paras(d) {
					want := strings.ReplaceAll(before[i], "e", "[E]")
					if skipped[before[i]] {
						continue
					}
					total += strings.Count(before[i], "e")
					if got := p.Text(); got != want {
						t.Errorf("paragraph %d:\n got  %q\nwant %q", i, got, want)
					}
				}
				if res.Count < total {
					t.Errorf("Count %d < matches in unskipped body paragraphs %d", res.Count, total)
				}
				_ = reopen(t, d)
			})
		}
	}
	if n == 0 {
		t.Fatal("no fixtures")
	}
}
