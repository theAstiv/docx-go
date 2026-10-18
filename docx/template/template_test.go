package template_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/docx"
	"github.com/theAstiv/docx-go/docx/template"
)

// save returns doc's bytes.
func save(t *testing.T, doc *docx.Document) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := doc.Save(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// reopen saves and reopens doc, so assertions see what a reader would.
func reopen(t *testing.T, doc *docx.Document) *docx.Document {
	t.Helper()
	out, err := docx.OpenBytes(save(t, doc))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func texts(b *docx.Body) []string {
	var out []string
	for p := range b.Paragraphs() {
		out = append(out, p.Text())
	}
	return out
}

func cellText(b *docx.Body, row, col int) string {
	r := 0
	for tbl := range b.Tables() {
		for rw := range tbl.Rows() {
			c := 0
			for cell := range rw.Cells() {
				if r == row && c == col {
					return cell.Text()
				}
				c++
			}
			r++
		}
	}
	return ""
}

func TestExecute_Values(t *testing.T) {
	doc := docx.New()
	body := doc.Body()
	body.AddParagraph().AddRun("Dear {{client.name}},")
	p := body.AddParagraph()
	p.AddRun("Score: {{")
	p.AddRun("score}} of {{max}}") // split across two runs
	body.AddParagraph().AddRun("{{notes}}")
	body.AddParagraph().AddRun("{{special}}")
	tbl := body.AddTable(1, 1, docx.TableOptions{})
	for row := range tbl.Rows() {
		for cell := range row.Cells() {
			for cp := range cell.Paragraphs() {
				cp.AddRun("{{client.city}}")
			}
		}
	}
	doc.LastSection().SetHeader(docx.HeaderDefault).AddParagraph().AddRun("Report for {{client.name}}")

	var data map[string]any
	if err := json.Unmarshal([]byte(`{
		"client": {"name": "Acme & Sons <Ltd>", "city": "Pune"},
		"score": 3, "max": 4.5,
		"notes": "line one\nline two\tend",
		"special": "\"quotes\" & 'apostrophes'"
	}`), &data); err != nil {
		t.Fatal(err)
	}
	warnings, err := template.Execute(doc, data, template.Options{Mode: template.Strict})
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	got := reopen(t, doc)
	want := []string{
		"Dear Acme & Sons <Ltd>,",
		"Score: 3 of 4.5",
		"line one\nline two\tend",
		`"quotes" & 'apostrophes'`,
	}
	gotTexts := texts(got.Body())
	for i, w := range want {
		if i >= len(gotTexts) || gotTexts[i] != w {
			t.Errorf("paragraph %d = %q, want %q", i, at(gotTexts, i), w)
		}
	}
	if c := cellText(got.Body(), 0, 0); c != "Pune" {
		t.Errorf("table cell = %q, want Pune", c)
	}
	// A new header also holds an empty paragraph, so look for ours.
	var header []string
	for hp := range got.LastSection().Header(docx.HeaderDefault).Paragraphs() {
		header = append(header, hp.Text())
	}
	if !strings.Contains(strings.Join(header, "\n"), "Report for Acme & Sons <Ltd>") {
		t.Errorf("header = %q", header)
	}
}

func at(s []string, i int) string {
	if i < len(s) {
		return s[i]
	}
	return "<missing>"
}

// Strict mode reports every missing key in one error and leaves the
// document byte-for-byte unchanged.
func TestExecute_StrictMissingKeys(t *testing.T) {
	doc := docx.New()
	doc.Body().AddParagraph().AddRun("{{a}} {{b.c}}")
	tbl := doc.Body().AddTable(1, 1, docx.TableOptions{})
	for row := range tbl.Rows() {
		for cell := range row.Cells() {
			cell.AddParagraph().AddRun("{{d}}")
		}
	}
	doc.LastSection().SetHeader(docx.HeaderDefault).AddParagraph().AddRun("{{e}}")
	before := save(t, doc)

	_, err := template.Execute(doc, map[string]any{"a": "x", "b": map[string]any{}}, template.Options{})
	var te *template.Error
	if !errors.As(err, &te) {
		t.Fatalf("err = %v, want *template.Error", err)
	}
	if !errors.Is(err, template.ErrMissingKey) {
		t.Errorf("errors.Is(err, ErrMissingKey) = false")
	}
	var tags []string
	for _, is := range te.Issues {
		tags = append(tags, is.Tag+"@"+is.Location.Part)
	}
	want := []string{"{{b.c}}@word/document.xml", "{{d}}@word/document.xml", "{{e}}@word/header1.xml"}
	if strings.Join(tags, " ") != strings.Join(want, " ") {
		t.Errorf("issues = %v, want %v\n%v", tags, want, err)
	}
	if after := save(t, doc); !bytes.Equal(before, after) {
		t.Error("document changed after a failed Execute")
	}
}

func TestExecute_Lenient(t *testing.T) {
	doc := docx.New()
	doc.Body().AddParagraph().AddRun("[{{missing}}]")
	warnings, err := template.Execute(doc, nil, template.Options{Mode: template.Lenient})
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !errors.Is(warnings[0].Err, template.ErrMissingKey) || warnings[0].Severity != template.SeverityWarning {
		t.Errorf("warnings = %v, want one missing-key warning", warnings)
	}
	if got := texts(reopen(t, doc).Body()); got[0] != "[]" {
		t.Errorf("text = %q, want []", got[0])
	}
}

// Structural errors fail in Lenient mode too.
func TestExecute_LenientStillFailsOnBadTags(t *testing.T) {
	doc := docx.New()
	doc.Body().AddParagraph().AddRun("{{1bad}}")
	_, err := template.Execute(doc, nil, template.Options{Mode: template.Lenient})
	if !errors.Is(err, template.ErrMalformedTag) {
		t.Errorf("err = %v, want ErrMalformedTag", err)
	}
}

type stringer struct{}

func (stringer) String() string { return "via Stringer" }

func TestExecute_ValueTypes(t *testing.T) {
	cases := []struct {
		value any
		want  string
		err   error
	}{
		{"s", "s", nil},
		{true, "true", nil},
		{42, "42", nil},
		{int64(-7), "-7", nil},
		{1e21, "1000000000000000000000", nil},
		{0.1, "0.1", nil},
		{json.Number("12.50"), "12.50", nil},
		{nil, "", nil},
		{stringer{}, "via Stringer", nil},
		{map[string]any{"a": 1}, "", template.ErrUnsupportedValue},
		{[]any{1}, "", template.ErrUnsupportedValue},
	}
	for _, c := range cases {
		doc := docx.New()
		doc.Body().AddParagraph().AddRun("{{v}}")
		_, err := template.Execute(doc, map[string]any{"v": c.value}, template.Options{})
		if c.err != nil {
			if !errors.Is(err, c.err) {
				t.Errorf("%#v: err = %v, want %v", c.value, err, c.err)
			}
			continue
		}
		if err != nil {
			t.Errorf("%#v: %v", c.value, err)
			continue
		}
		if got := texts(doc.Body())[0]; got != c.want {
			t.Errorf("%#v: text = %q, want %q", c.value, got, c.want)
		}
	}
}

func TestExecute_RelativePathOutsideLoop(t *testing.T) {
	doc := docx.New()
	doc.Body().AddParagraph().AddRun("{{.field}}")
	if _, err := template.Execute(doc, map[string]any{}, template.Options{}); !errors.Is(err, template.ErrMissingKey) {
		t.Errorf("err = %v, want ErrMissingKey", err)
	}
}

func TestNilDocument(t *testing.T) {
	if _, err := template.Execute(nil, nil, template.Options{}); err == nil {
		t.Error("Execute(nil) returned no error")
	}
	if _, err := template.Placeholders(nil); err == nil {
		t.Error("Placeholders(nil) returned no error")
	}
}

func TestPlaceholders(t *testing.T) {
	doc := docx.New()
	doc.Body().AddParagraph().AddRun("{{a}} and {{")
	doc.Body().AddParagraph().AddRun("{{#each items}}")
	doc.Body().AddParagraph().AddRun("{{.name}}")
	doc.Body().AddParagraph().AddRun("{{/each}} {{1bad}}")
	before := save(t, doc)

	ps, err := template.Placeholders(doc)
	if !errors.Is(err, template.ErrMalformedTag) {
		t.Errorf("err = %v, want ErrMalformedTag for {{1bad}} and the unclosed {{", err)
	}
	var got []string
	for _, p := range ps {
		got = append(got, p.Kind+":"+p.Path)
	}
	if want := "value:a each:items value:.name end:"; strings.Join(got, " ") != want {
		t.Errorf("placeholders = %q, want %q", strings.Join(got, " "), want)
	}
	if !bytes.Equal(before, save(t, doc)) {
		t.Error("Placeholders modified the document")
	}
}
