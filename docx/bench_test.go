package docx_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/theAstiv/docx-go/docx"
)

// Benchmarks for the PR perf gate (bench.yml): the paths a typical program
// takes through the library, on real fixtures and generated content.

var benchFixtures = []string{
	"../testdata/fixtures/poi/ComplexNumberedLists.docx",
	"../testdata/fixtures/poi/IllustrativeCases.docx",
	"../testdata/fixtures/word/template_cover_sdt.docx",
}

func readBenchFixtures(b *testing.B) [][]byte {
	b.Helper()
	var out [][]byte
	for _, f := range benchFixtures {
		data, err := os.ReadFile(f)
		if err != nil {
			b.Fatal(err)
		}
		out = append(out, data)
	}
	return out
}

func BenchmarkOpenSave(b *testing.B) {
	files := readBenchFixtures(b)
	b.ReportAllocs()
	for range b.N {
		for _, data := range files {
			d, err := docx.OpenBytes(data)
			if err != nil {
				b.Fatal(err)
			}
			if err := d.Save(io.Discard); err != nil {
				b.Fatal(err)
			}
		}
	}
}

// bigDocument builds 2,000 formatted paragraphs and a 200-row table.
func bigDocument() *docx.Document {
	d := docx.New()
	for i := range 2000 {
		p := d.Body().AddParagraph()
		r := p.AddRun(fmt.Sprintf("Paragraph %d with some text to measure.", i))
		if i%3 == 0 {
			r.Font().SetBold(docx.On())
		}
	}
	tbl := d.Body().AddTable(200, 4, docx.TableOptions{})
	for row := range tbl.Rows() {
		for cell := range row.Cells() {
			cell.SetText("cell")
		}
	}
	return d
}

func BenchmarkBuildAndSave(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		var buf bytes.Buffer
		if err := bigDocument().Save(&buf); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkText(b *testing.B) {
	d := bigDocument()
	b.ReportAllocs()
	for range b.N {
		_ = d.Text()
	}
}

func BenchmarkReplace(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		b.StopTimer()
		d := bigDocument()
		b.StartTimer()
		d.Replace("some text", "other words", docx.ReplaceOptions{})
	}
}
