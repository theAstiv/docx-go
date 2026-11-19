package docx_test

import (
	"archive/zip"
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

// gridWidths returns the width of every cell in the table's first row.
func gridWidths(t *testing.T, tbl *docx.Table) []docx.Twip {
	t.Helper()
	var out []docx.Twip
	for row := range tbl.Rows() {
		for c := range row.Cells() {
			w, ok := c.Width()
			if !ok || w.Kind() != docx.WidthKindTwips {
				t.Fatalf("cell without a twips width: %+v", w)
			}
			out = append(out, w.Twips())
		}
		break
	}
	return out
}

func TestTableDefaultWidths(t *testing.T) {
	a4 := func(d *docx.Document) {
		s := d.LastSection()
		s.SetPageSize(11906, 16838) // A4
		s.SetMargins(docx.PageMargins{Top: 1440, Right: 1134, Bottom: 1440, Left: 1134})
	}
	tests := []struct {
		name  string
		setup func(*docx.Document)
		opts  docx.TableOptions
		cols  int
		want  []docx.Twip
	}{
		// New()'s Letter page: 12240 less margins 1800 + 1440 = 9000.
		{"letter, unsized", nil, docx.TableOptions{}, 3, []docx.Twip{3000, 3000, 3000}},
		// A4: 11906 less 2 × 1134 = 9638.
		{"A4, unsized", a4, docx.TableOptions{}, 2, []docx.Twip{4819, 4819}},
		{"partly sized", nil, docx.TableOptions{ColWidth: []docx.Twip{5000}}, 3, []docx.Twip{5000, 2000, 2000}},
		{"zero entry is unsized", nil, docx.TableOptions{ColWidth: []docx.Twip{0, 3000}}, 2, []docx.Twip{6000, 3000}},
		// The given widths fill the page: the rest get an even share of it.
		{"over-full", nil, docx.TableOptions{ColWidth: []docx.Twip{8000, 2000}}, 3, []docx.Twip{8000, 2000, 3000}},
		{"fully sized unchanged", nil, docx.TableOptions{ColWidth: []docx.Twip{100, 200}}, 2, []docx.Twip{100, 200}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := docx.New()
			if tt.setup != nil {
				tt.setup(d)
			}
			got := gridWidths(t, d.Body().AddTable(2, tt.cols, tt.opts))
			if !slices.Equal(got, tt.want) {
				t.Errorf("widths = %v, want %v", got, tt.want)
			}
		})
	}
}

// A table takes the width of the section it falls in: one inserted before
// a section break uses that section's page, one after it the next.
func TestTableDefaultWidths_Sections(t *testing.T) {
	d := docx.New()
	first := d.Body().AddParagraph()
	first.AddRun("narrow section")
	sec := first.AddSectionBreak()
	sec.SetPageSize(8000, 12000)
	sec.SetMargins(docx.PageMargins{Top: 1000, Right: 1000, Bottom: 1000, Left: 1000})
	before := d.Body().AddParagraph()
	before.AddRun("in the last section")

	// Before the break paragraph (inserted after a paragraph that precedes
	// it): 8000 − 2000 = 6000.
	lead := first.InsertParagraphBefore("lead")
	if got := gridWidths(t, lead.InsertTableAfter(1, 2, docx.TableOptions{})); !slices.Equal(got, []docx.Twip{3000, 3000}) {
		t.Errorf("in the first section: %v, want [3000 3000]", got)
	}
	// At the end of the body: the final section (Letter, 9000).
	if got := gridWidths(t, d.Body().AddTable(1, 2, docx.TableOptions{})); !slices.Equal(got, []docx.Twip{4500, 4500}) {
		t.Errorf("in the last section: %v, want [4500 4500]", got)
	}
	// After the break paragraph: the next (final) section.
	if got := gridWidths(t, first.InsertTableAfter(1, 1, docx.TableOptions{})); !slices.Equal(got, []docx.Twip{9000}) {
		t.Errorf("after the break: %v, want [9000]", got)
	}
}

// In a cell with a fixed width, a nested table fills that width less the
// default cell margins; in a header, the final section's text width.
func TestTableDefaultWidths_CellAndHeader(t *testing.T) {
	d := docx.New()
	outer := d.Body().AddTable(1, 2, docx.TableOptions{ColWidth: []docx.Twip{4216, 4784}})
	var cell *docx.Cell
	for row := range outer.Rows() {
		for c := range row.Cells() {
			cell = c
			break
		}
	}
	var p *docx.Paragraph
	for para := range cell.Paragraphs() {
		p = para
	}
	if got := gridWidths(t, p.InsertTableAfter(1, 2, docx.TableOptions{})); !slices.Equal(got, []docx.Twip{2000, 2000}) {
		t.Errorf("in a 4216-twip cell: %v, want [2000 2000]", got)
	}
	cell.SetWidth(docx.WidthPercent(50))
	if got := gridWidths(t, p.InsertTableAfter(1, 1, docx.TableOptions{})); !slices.Equal(got, []docx.Twip{4000}) {
		t.Errorf("in a percentage-width cell: %v, want its grid column less margins, [4000]", got)
	}
	h := d.LastSection().SetHeader(docx.HeaderDefault)
	if got := gridWidths(t, h.AddTable(1, 3, docx.TableOptions{})); !slices.Equal(got, []docx.Twip{3000, 3000, 3000}) {
		t.Errorf("in a header: %v, want [3000 3000 3000]", got)
	}
}

// withFinalSection saves d, replaces its body-level w:sectPr with the
// children given (raw WordprocessingML, w: prefix), and reopens it.
func withFinalSection(t *testing.T, d *docx.Document, children string) *docx.Document {
	t.Helper()
	var buf bytes.Buffer
	if err := d.Save(&buf); err != nil {
		t.Fatal(err)
	}
	entries := readZip(t, buf.Bytes())
	doc := string(entries["word/document.xml"])
	i, j := strings.LastIndex(doc, "<sectPr"), strings.LastIndex(doc, "</sectPr>")
	if i < 0 || j < i {
		t.Fatalf("no body sectPr in %s", doc)
	}
	entries["word/document.xml"] = []byte(doc[:i] + `<sectPr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		children + doc[j:])
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for name, data := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	r, err := docx.OpenBytes(out.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// Widths that nearly fill the page do not leave sliver columns.
func TestTableDefaultWidths_NearlyFull(t *testing.T) {
	d := docx.New()
	got := gridWidths(t, d.Body().AddTable(1, 3, docx.TableOptions{ColWidth: []docx.Twip{4500, 4490}}))
	if !slices.Equal(got, []docx.Twip{4500, 4490, 3000}) {
		t.Errorf("widths = %v, want [4500 4490 3000] (no 10-twip column)", got)
	}
}

// Text columns and the gutter narrow the width a table fills.
func TestTableDefaultWidths_ColumnsAndGutter(t *testing.T) {
	const page = `<w:pgSz w:w="12240" w:h="15840"/>`
	tests := []struct {
		name, sect string
		want       docx.Twip // width of a one-column table
	}{
		// 12240 − 2 × 1440 = 9360; two columns 720 apart: (9360 − 720) / 2.
		{"two columns", page + `<w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720" w:gutter="0"/><w:cols w:num="2" w:space="720"/>`, 4320},
		// Default space between columns is 720.
		{"three columns, default space", page + `<w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720" w:gutter="0"/><w:cols w:num="3"/>`, 2640},
		{"unequal columns: the first", page + `<w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720" w:gutter="0"/><w:cols w:num="2" w:space="360" w:equalWidth="0"><w:col w:w="3000" w:space="360"/><w:col w:w="6000"/></w:cols>`, 3000},
		{"gutter", page + `<w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720" w:gutter="720"/>`, 8640},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := withFinalSection(t, docx.New(), tt.sect)
			if got := gridWidths(t, d.Body().AddTable(1, 1, docx.TableOptions{})); !slices.Equal(got, []docx.Twip{tt.want}) {
				t.Errorf("width = %v, want [%d]", got, tt.want)
			}
		})
	}
}

// A header table takes the width of the first section that uses the
// header, not the final section's.
func TestTableDefaultWidths_HeaderOfEarlierSection(t *testing.T) {
	d := docx.New()
	p := d.Body().AddParagraph()
	p.AddRun("landscape section")
	sec := p.AddSectionBreak()
	sec.SetPageSize(15840, 12240)
	sec.SetMargins(docx.PageMargins{Top: 1440, Right: 1440, Bottom: 1440, Left: 1440})
	landscape := sec.SetHeader(docx.HeaderDefault)
	d.Body().AddParagraph().AddRun("portrait section")
	portrait := d.LastSection().SetHeader(docx.HeaderDefault)

	if got := gridWidths(t, landscape.AddTable(1, 2, docx.TableOptions{})); !slices.Equal(got, []docx.Twip{6480, 6480}) {
		t.Errorf("landscape header: %v, want [6480 6480] (15840 − 2880 shared)", got)
	}
	if got := gridWidths(t, portrait.AddTable(1, 1, docx.TableOptions{})); !slices.Equal(got, []docx.Twip{9000}) {
		t.Errorf("portrait header: %v, want [9000]", got)
	}
}

// A nested table in a cell without a fixed width fills the grid columns
// the cell spans, not the page.
func TestTableDefaultWidths_NestedInSpannedAutoCell(t *testing.T) {
	d := docx.New()
	outer := d.Body().AddTable(1, 3, docx.TableOptions{ColWidth: []docx.Twip{1000, 2216, 3000}})
	var cells []*docx.Cell
	for row := range outer.Rows() {
		cells = slices.Collect(row.Cells())
	}
	cells[1].SetSpan(1, 2) // covers grid columns 1 and 2: 5216
	cells[1].SetWidth(docx.WidthAuto())
	var p *docx.Paragraph
	for para := range cells[1].Paragraphs() {
		p = para
	}
	if got := gridWidths(t, p.InsertTableAfter(1, 2, docx.TableOptions{})); !slices.Equal(got, []docx.Twip{2500, 2500}) {
		t.Errorf("widths = %v, want [2500 2500] (5216 − 216 shared)", got)
	}
}
