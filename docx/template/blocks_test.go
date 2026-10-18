package template_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/theAstiv/docx-go/docx"
	"github.com/theAstiv/docx-go/docx/template"
)

// paras adds one paragraph per text to the body.
func paras(doc *docx.Document, texts ...string) {
	for _, s := range texts {
		doc.Body().AddParagraph().AddRun(s)
	}
}

// table adds a table whose cells hold the given texts, one paragraph each.
func table(doc *docx.Document, rows [][]string) {
	tbl := doc.Body().AddTable(len(rows), len(rows[0]), docx.TableOptions{})
	r := 0
	for row := range tbl.Rows() {
		c := 0
		for cell := range row.Cells() {
			for p := range cell.Paragraphs() {
				p.AddRun(rows[r][c])
				break
			}
			c++
		}
		r++
	}
}

// tableTexts returns every table's cell texts, row by row, joined by " | ".
func tableTexts(b *docx.Body) [][]string {
	var out [][]string
	for tbl := range b.Tables() {
		var rows []string
		for row := range tbl.Rows() {
			var cells []string
			for cell := range row.Cells() {
				cells = append(cells, cell.Text())
			}
			rows = append(rows, strings.Join(cells, " | "))
		}
		out = append(out, rows)
	}
	return out
}

func mustExecute(t *testing.T, doc *docx.Document, data map[string]any) []template.Issue {
	t.Helper()
	warnings, err := template.Execute(doc, data, template.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return warnings
}

func bodyText(t *testing.T, doc *docx.Document) string {
	t.Helper()
	return strings.Join(texts(reopen(t, doc).Body()), "\n")
}

var threats = []any{
	map[string]any{"id": "T1", "title": "Spoofing", "high": true, "fixes": []any{"MFA", "Signed tokens"}},
	map[string]any{"id": "T2", "title": "Tampering", "high": false, "fixes": []any{}},
	map[string]any{"id": "T3", "title": "Repudiation", "high": true, "fixes": []any{"Audit log"}},
}

func TestBlocks_IfUnless(t *testing.T) {
	doc := docx.New()
	paras(doc, "start", "{{#if yes}}", "shown", "{{/if}}", "{{#if no}}", "hidden", "{{/if}}",
		"{{#unless no}}", "unless shown", "{{/unless}}", "{{#if absent}}", "absent hidden", "{{/if}}", "end")
	mustExecute(t, doc, map[string]any{"yes": "x", "no": 0})
	if got, want := bodyText(t, doc), "start\nshown\nunless shown\nend"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBlocks_EachParagraphs(t *testing.T) {
	doc := docx.New()
	paras(doc, "Threats:", "{{#each threats}}", "{{.id}}: {{.title}}",
		"{{#if .high}}", "  high severity", "{{/if}}",
		"{{#each .fixes}}", "  fix: {{.}}", "{{/each}}",
		"{{/each}}", "{{#each none}}", "never", "{{/each}}", "done")
	mustExecute(t, doc, map[string]any{"threats": threats, "none": []any{}})
	want := strings.Join([]string{
		"Threats:",
		"T1: Spoofing", "  high severity", "  fix: MFA", "  fix: Signed tokens",
		"T2: Tampering",
		"T3: Repudiation", "  high severity", "  fix: Audit log",
		"done",
	}, "\n")
	if got := bodyText(t, doc); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestBlocks_RowLoop(t *testing.T) {
	doc := docx.New()
	table(doc, [][]string{
		{"ID", "Title"},
		{"{{#each threats}}{{.id}}", "{{.title}}{{/each}}"},
		{"Total", "{{count}}"},
	})
	mustExecute(t, doc, map[string]any{"threats": threats, "count": 3})
	got := tableTexts(reopen(t, doc).Body())
	want := []string{"ID | Title", "T1 | Spoofing", "T2 | Tampering", "T3 | Repudiation", "Total | 3"}
	if len(got) != 1 || strings.Join(got[0], "\n") != strings.Join(want, "\n") {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBlocks_MultiRowLoopAndRowIf(t *testing.T) {
	doc := docx.New()
	table(doc, [][]string{
		{"{{#each threats}}{{.id}}", "{{.title}}"},
		{"{{#if .high}}!", "high{{/if}}"},
		{"-", "-{{/each}}"},
	})
	mustExecute(t, doc, map[string]any{"threats": threats})
	got := strings.Join(tableTexts(reopen(t, doc).Body())[0], "\n")
	want := strings.Join([]string{
		"T1 | Spoofing", "! | high", "- | -",
		"T2 | Tampering", "- | -",
		"T3 | Repudiation", "! | high", "- | -",
	}, "\n")
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// A block tag alone in its paragraph and closed inside the same cell is a
// paragraph range of that cell, even in the first and last cell of a row.
func TestBlocks_ParagraphRangeInsideCell(t *testing.T) {
	doc := docx.New()
	tbl := doc.Body().AddTable(1, 1, docx.TableOptions{})
	// Cell.AddParagraph inserts before the cell's last paragraph, so the
	// cell's own paragraph, written last, ends the cell.
	for row := range tbl.Rows() {
		for cell := range row.Cells() {
			last := collect(cell.Paragraphs())[0]
			cell.AddParagraph().AddRun("{{#each items}}")
			cell.AddParagraph().AddRun("- {{.}}")
			last.AddRun("{{/each}}")
		}
	}
	mustExecute(t, doc, map[string]any{"items": []string{"a", "b"}})
	if got := tableTexts(reopen(t, doc).Body())[0][0]; got != "- a\n- b" {
		t.Errorf("cell = %q, want %q", got, "- a\n- b")
	}
}

func TestBlocks_EmptyLoopRemovesTable(t *testing.T) {
	doc := docx.New()
	paras(doc, "before")
	table(doc, [][]string{{"{{#each rows}}{{.a}}{{/each}}"}})
	paras(doc, "after")
	warnings := mustExecute(t, doc, map[string]any{"rows": []any{}})
	if n := len(tableTexts(doc.Body())); n != 0 {
		t.Errorf("%d tables left, want 0", n)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0].Err.Error(), "table was removed") {
		t.Errorf("warnings = %v", warnings)
	}
	if got := bodyText(t, doc); got != "before\nafter" {
		t.Errorf("body = %q", got)
	}
}

// A cell whose only paragraph is removed by a range still ends with a
// paragraph, as the schema requires.
func TestBlocks_CellKeepsAParagraph(t *testing.T) {
	doc := docx.New()
	tbl := doc.Body().AddTable(1, 2, docx.TableOptions{})
	for row := range tbl.Rows() {
		n := 0
		for cell := range row.Cells() {
			if n == 1 {
				last := collect(cell.Paragraphs())[0]
				cell.AddParagraph().AddRun("{{#if no}}")
				cell.AddParagraph().AddRun("x")
				last.AddRun("{{/if}}")
			}
			n++
		}
	}
	mustExecute(t, doc, nil)
	for row := range reopen(t, doc).Body().Tables() {
		for r := range row.Rows() {
			for c := range r.Cells() {
				if len(collect(c.Paragraphs())) == 0 {
					t.Error("a cell has no paragraph")
				}
			}
		}
	}
}

func collect[T any](seq func(func(T) bool)) []T {
	var out []T
	for v := range seq {
		out = append(out, v)
	}
	return out
}

func TestBlocks_Errors(t *testing.T) {
	cases := []struct {
		name  string
		paras []string
		data  map[string]any
		want  error
		count int
	}{
		{"unclosed", []string{"{{#if a}}", "x"}, nil, template.ErrUnbalancedBlock, 1},
		{"orphan close", []string{"x", "{{/each}}"}, nil, template.ErrUnbalancedBlock, 1},
		{"mismatched", []string{"{{#if a}}", "{{/each}}"}, nil, template.ErrUnbalancedBlock, 1},
		{"misplaced", []string{"text {{#if a}} more"}, nil, template.ErrMisplacedBlock, 1},
		// The orphan close stops expansion at this level; the misplaced
		// open is still reported.
		{"misplaced and orphan", []string{"text {{#if a}} more", "{{/if}}"}, nil, template.ErrMisplacedBlock, 2},
		{"missing list", []string{"{{#each nope}}", "x", "{{/each}}"}, nil, template.ErrMissingKey, 1},
		{"not a list", []string{"{{#each s}}", "x", "{{/each}}"}, map[string]any{"s": "str"}, template.ErrUnsupportedValue, 1},
		{"missing field per item", []string{"{{#each threats}}", "{{.severity}}", "{{/each}}"},
			map[string]any{"threats": threats}, template.ErrMissingKey, 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := docx.New()
			paras(doc, c.paras...)
			before := save(t, doc)
			_, err := template.Execute(doc, c.data, template.Options{})
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
			var te *template.Error
			if errors.As(err, &te) && c.count > 0 && len(te.Issues) != c.count {
				t.Errorf("%d issues, want %d: %v", len(te.Issues), c.count, err)
			}
			if !bytes.Equal(before, save(t, doc)) {
				t.Error("document changed after a failed Execute")
			}
		})
	}
}

func TestBlocks_MissingFieldNamesItem(t *testing.T) {
	doc := docx.New()
	paras(doc, "{{#each threats}}", "{{.severity}}", "{{/each}}")
	_, err := template.Execute(doc, map[string]any{"threats": threats}, template.Options{})
	if err == nil || !strings.Contains(err.Error(), "threats[2]") {
		t.Errorf("error does not name the item: %v", err)
	}
}

func TestBlocks_SectionBreakInRange(t *testing.T) {
	doc := docx.New()
	paras(doc, "{{#if a}}")
	doc.Body().AddParagraph().AddSectionBreak()
	paras(doc, "{{/if}}")
	if _, err := template.Execute(doc, map[string]any{"a": true}, template.Options{}); !errors.Is(err, template.ErrSectionBreakInBlock) {
		t.Errorf("err = %v, want ErrSectionBreakInBlock", err)
	}
}

// Copies keep ids unique: bookmarks stay on the first copy only, drawings
// get fresh wp:docPr ids.
func TestBlocks_CopiesKeepIDsUnique(t *testing.T) {
	doc := docx.New()
	paras(doc, "{{#each threats}}")
	p := doc.Body().AddParagraph()
	p.AddRun("{{.id}}")
	if _, err := p.AddBookmark("threat"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.AddRun("").AddImage(bytes.NewReader(tinyPNG), docx.ImageOptions{AltText: "icon"}); err != nil {
		t.Fatal(err)
	}
	paras(doc, "{{/each}}")
	mustExecute(t, doc, map[string]any{"threats": threats})

	xml := documentXML(t, save(t, doc))
	if n := strings.Count(xml, "<bookmarkStart"); n != 1 {
		t.Errorf("%d bookmarkStart elements, want 1", n)
	}
	// The encoder may write namespace declarations before id.
	ids := regexp.MustCompile(`<docPr[^>]*\sid="(\d+)"`).FindAllStringSubmatch(xml, -1)
	seen := map[string]bool{}
	for _, m := range ids {
		if seen[m[1]] {
			t.Errorf("duplicate docPr id %s", m[1])
		}
		seen[m[1]] = true
	}
	if len(ids) != 3 {
		t.Errorf("%d drawings, want 3", len(ids))
	}
}

func TestBlocks_LargeRowLoop(t *testing.T) {
	doc := docx.New()
	table(doc, [][]string{{"{{#each rows}}{{.a}}", "{{.b}}", "{{.c}}{{/each}}"}})
	rows := make([]any, 200)
	for i := range rows {
		rows[i] = map[string]any{"a": i, "b": fmt.Sprint("row ", i), "c": i % 2}
	}
	start := time.Now()
	mustExecute(t, doc, map[string]any{"rows": rows})
	elapsed := time.Since(start)
	if n := len(tableTexts(doc.Body())[0]); n != 200 {
		t.Errorf("%d rows, want 200", n)
	}
	// A smoke bound, generous for slow CI machines and the race detector.
	if elapsed > 5*time.Second {
		t.Errorf("200-row loop took %v", elapsed)
	}
	t.Logf("200-row loop: %v", elapsed)
}

// tinyPNG is a 1×1 PNG.
var tinyPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
	0x42, 0x60, 0x82,
}

func documentXML(t *testing.T, docxBytes []byte) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(docxBytes), int64(len(docxBytes)))
	if err != nil {
		t.Fatal(err)
	}
	f, err := zr.Open("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return string(b)
}
