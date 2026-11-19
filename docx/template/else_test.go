package template_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/theAstiv/docx-go/docx"
	"github.com/theAstiv/docx-go/docx/template"
)

func TestElse_Paragraphs(t *testing.T) {
	tests := []struct {
		name  string
		paras []string
		data  map[string]any
		want  string
	}{
		{
			name:  "if true",
			paras: []string{"a", "{{#if x}}", "then", "{{else}}", "else", "{{/if}}", "z"},
			data:  map[string]any{"x": true},
			want:  "a\nthen\nz",
		},
		{
			name:  "if false",
			paras: []string{"a", "{{#if x}}", "then", "{{else}}", "else", "{{/if}}", "z"},
			data:  map[string]any{"x": false},
			want:  "a\nelse\nz",
		},
		{
			name:  "if missing key is false",
			paras: []string{"{{#if x}}", "then", "{{ else }}", "else", "{{/if}}"},
			data:  map[string]any{},
			want:  "else",
		},
		{
			name:  "unless",
			paras: []string{"{{#unless x}}", "then", "{{else}}", "else", "{{/unless}}"},
			data:  map[string]any{"x": true},
			want:  "else",
		},
		{
			name:  "empty then branch",
			paras: []string{"{{#if x}}", "{{else}}", "else", "{{/if}}"},
			data:  map[string]any{"x": false},
			want:  "else",
		},
		{
			name:  "each with items",
			paras: []string{"{{#each items}}", "- {{.}}", "{{else}}", "none", "{{/each}}"},
			data:  map[string]any{"items": []any{"a", "b"}},
			want:  "- a\n- b",
		},
		{
			name:  "each empty",
			paras: []string{"{{#each items}}", "- {{.}}", "{{else}}", "none", "{{/each}}"},
			data:  map[string]any{"items": []any{}},
			want:  "none",
		},
		{
			name:  "each null",
			paras: []string{"{{#each items}}", "- {{.}}", "{{else}}", "none", "{{/each}}"},
			data:  map[string]any{"items": nil},
			want:  "none",
		},
		{
			// With an else, a missing list is "no items", not a missing key.
			name:  "each missing",
			paras: []string{"{{#each items}}", "- {{.}}", "{{else}}", "none for {{who}}", "{{/each}}"},
			data:  map[string]any{"who": "me"},
			want:  "none for me",
		},
		{
			// An inner else belongs to the inner block; the outer block has none.
			name: "nested",
			paras: []string{
				"{{#each threats}}", "{{.id}}",
				"{{#if .high}}", "high", "{{else}}", "low", "{{/if}}",
				"{{#each .fixes}}", "fix {{.}}", "{{else}}", "no fix", "{{/each}}",
				"{{/each}}",
			},
			data: map[string]any{"threats": threats},
			want: "T1\nhigh\nfix MFA\nfix Signed tokens\nT2\nlow\nno fix\nT3\nhigh\nfix Audit log",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := docx.New()
			paras(doc, tt.paras...)
			mustExecute(t, doc, tt.data)
			if got := bodyText(t, doc); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// Without an else, a missing list is still a missing key in Strict mode.
func TestElse_EachMissingWithoutElse(t *testing.T) {
	doc := docx.New()
	paras(doc, "{{#each items}}", "- {{.}}", "{{/each}}")
	if _, err := template.Execute(doc, map[string]any{}, template.Options{}); !errors.Is(err, template.ErrMissingKey) {
		t.Errorf("err = %v, want ErrMissingKey", err)
	}
}

// A list that is not a list fails even with an else.
func TestElse_EachNotAList(t *testing.T) {
	doc := docx.New()
	paras(doc, "{{#each items}}", "- {{.}}", "{{else}}", "none", "{{/each}}")
	if _, err := template.Execute(doc, map[string]any{"items": "x"}, template.Options{}); !errors.Is(err, template.ErrUnsupportedValue) {
		t.Errorf("err = %v, want ErrUnsupportedValue", err)
	}
}

func TestElse_Rows(t *testing.T) {
	rows := [][]string{
		{"ID", "Title"},
		{"{{#each items}}{{.id}}", "{{.title}}"},
		{"{{else}}No items", "—{{/each}}"},
		{"Total", "{{n}}"},
	}
	tests := []struct {
		name  string
		items any
		want  []string
	}{
		{"items", []any{map[string]any{"id": "1", "title": "One"}, map[string]any{"id": "2", "title": "Two"}},
			[]string{"ID | Title", "1 | One", "2 | Two", "Total | 0"}},
		{"empty", []any{}, []string{"ID | Title", "No items | —", "Total | 0"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := docx.New()
			table(doc, rows)
			mustExecute(t, doc, map[string]any{"items": tt.items, "n": 0})
			got := tableTexts(reopen(t, doc).Body())
			if len(got) != 1 || strings.Join(got[0], "\n") != strings.Join(tt.want, "\n") {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// The else branch may span several rows, and the close may sit in a later
// row than the else.
func TestElse_RowsMultiRowBranches(t *testing.T) {
	rows := [][]string{
		{"{{#if ok}}A", "a"},
		{"B", "b"},
		{"{{else}}C", "c"},
		{"D", "d{{/if}}"},
	}
	for _, tt := range []struct {
		ok   bool
		want string
	}{
		{true, "A | a\nB | b"},
		{false, "C | c\nD | d"},
	} {
		doc := docx.New()
		table(doc, rows)
		mustExecute(t, doc, map[string]any{"ok": tt.ok})
		got := tableTexts(reopen(t, doc).Body())
		if len(got) != 1 || strings.Join(got[0], "\n") != tt.want {
			t.Errorf("ok=%v: got %q, want %q", tt.ok, got, tt.want)
		}
	}
}

// A paragraph range with an else inside one cell, at the cell's edges, is a
// range of that cell, not a row range.
func TestElse_ParagraphRangeInsideCell(t *testing.T) {
	doc := docx.New()
	tbl := doc.Body().AddTable(1, 1, docx.TableOptions{})
	for row := range tbl.Rows() {
		for cell := range row.Cells() {
			cell.SetText("{{#if x}}")
			cell.AddParagraph().AddRun("yes")
			cell.AddParagraph().AddRun("{{else}}")
			cell.AddParagraph().AddRun("no")
			cell.AddParagraph().AddRun("{{/if}}")
		}
	}
	mustExecute(t, doc, map[string]any{"x": false})
	got := tableTexts(reopen(t, doc).Body())
	if len(got) != 1 || len(got[0]) != 1 || got[0][0] != "no" {
		t.Errorf("got %q, want [[no]]", got)
	}
}

func TestElse_Errors(t *testing.T) {
	tests := []struct {
		name  string
		paras []string
		want  error
	}{
		{"stray else", []string{"a", "{{else}}", "b"}, template.ErrMisplacedBlock},
		{"second else", []string{"{{#if x}}", "a", "{{else}}", "b", "{{else}}", "c", "{{/if}}"}, template.ErrUnbalancedBlock},
		{"else inside text", []string{"{{#if x}}", "a {{else}} b", "{{/if}}"}, template.ErrMisplacedBlock},
		{"else after close", []string{"{{#if x}}", "a", "{{/if}}", "{{else}}"}, template.ErrMisplacedBlock},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := docx.New()
			paras(doc, tt.paras...)
			if _, err := template.Execute(doc, map[string]any{"x": true}, template.Options{}); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

// An else in the row that opens a row range is in the middle of the row's
// text, so it is misplaced.
func TestElse_RowElseInOpenRow(t *testing.T) {
	doc := docx.New()
	table(doc, [][]string{{"{{#if x}}a", "{{else}}b{{/if}}"}})
	if _, err := template.Execute(doc, map[string]any{"x": true}, template.Options{}); !errors.Is(err, template.ErrMisplacedBlock) {
		t.Errorf("err = %v, want ErrMisplacedBlock", err)
	}
}

// {{.else}} still names a data key inside a loop.
func TestElse_DotElseIsAValue(t *testing.T) {
	doc := docx.New()
	paras(doc, "{{#each items}}", "{{.else}}", "{{/each}}")
	mustExecute(t, doc, map[string]any{"items": []any{map[string]any{"else": "kept"}}})
	if got := bodyText(t, doc); got != "kept" {
		t.Errorf("got %q, want %q", got, "kept")
	}
}

func TestElse_Placeholders(t *testing.T) {
	doc := docx.New()
	paras(doc, "{{#if x}}", "a", "{{else}}", "b", "{{/if}}")
	ps, err := template.Placeholders(doc)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, p := range ps {
		kinds = append(kinds, p.Kind+":"+p.Path)
	}
	if got, want := strings.Join(kinds, " "), "if:x else: end:"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
