package docx_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

func TestDocumentText_Layout(t *testing.T) {
	d := docx.New()
	b := d.Body()
	b.AddParagraph().AddRun("Title")
	p := b.AddParagraph()
	p.AddRun("a")
	r := p.AddRun("b")
	r.AddTab()
	r.AddBreak(docx.BreakLine)
	tbl := b.AddTable(2, 2, docx.TableOptions{})
	texts := []string{"r1c1", "r1c2", "r2c1", "r2c2"}
	i := 0
	for row := range tbl.Rows() {
		for c := range row.Cells() {
			c.SetText(texts[i])
			i++
		}
	}
	b.AddParagraph().AddRun("End")

	want := "Title\nab\t\n\nr1c1\tr1c2\nr2c1\tr2c2\nEnd\n"
	if got := d.Text(); got != want {
		t.Errorf("Document.Text:\n got  %q\nwant %q", got, want)
	}
	if got := b.Text(); got != want {
		t.Errorf("Body.Text differs from Document.Text: %q", got)
	}
}

func TestHeaderFooterText(t *testing.T) {
	d := docx.New()
	d.Body().AddParagraph().AddRun("body")
	sec := d.LastSection()
	h := sec.SetHeader(docx.HeaderDefault)
	h.SetText("Acme report")
	f := sec.SetFooter(docx.FooterDefault)
	f.SetText("Page ").AddPageNumber()

	if got := h.Text(); got != "Acme report\n" {
		t.Errorf("Header.Text = %q", got)
	}
	if got := f.Text(); got != "Page 1\n" {
		t.Errorf("Footer.Text = %q", got)
	}
	if strings.Contains(d.Text(), "Acme") {
		t.Errorf("Document.Text includes header text: %q", d.Text())
	}
}

// Every top-level paragraph's text appears, in order, in Document.Text for
// each fixture, and header/footer text extraction does not fail.
func TestDocumentText_Fixtures(t *testing.T) {
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
				d, err := docx.OpenBytes(data)
				if err != nil {
					t.Skipf("not openable: %v", err)
				}
				text := d.Text()
				pos := 0
				for p := range d.Body().Paragraphs() {
					pt := p.Text()
					i := strings.Index(text[pos:], pt+"\n")
					if i < 0 {
						t.Fatalf("paragraph %q not found in order in Document.Text", pt)
					}
					pos += i + len(pt) + 1
				}
				sec := d.LastSection()
				for _, typ := range []docx.HeaderType{docx.HeaderDefault, docx.HeaderFirst, docx.HeaderEven} {
					if h := sec.Header(typ); h != nil {
						_ = h.Text()
					}
					if f := sec.Footer(typ); f != nil {
						_ = f.Text()
					}
				}
			})
		}
	}
	if n == 0 {
		t.Fatal("no fixtures found")
	}
}
